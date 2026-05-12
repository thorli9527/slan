use std::io::{Read, Write};
use std::net::TcpStream;
use std::path::PathBuf;
use std::time::Duration;

const DEFAULT_IO_TIMEOUT: Duration = Duration::from_secs(5);
const MQTT_QOS_AT_MOST_ONCE: u8 = 0;
const MQTT_QOS_EXACTLY_ONCE: u8 = 2;

#[derive(Debug, Clone)]
pub struct ThinMqttCredential {
    pub broker_url: String,
    pub client_id: String,
    pub username: String,
    pub password: String,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ThinMqttQoS {
    AtMostOnce,
    ExactlyOnce,
}

impl ThinMqttQoS {
    fn delivery_qos(self) -> MqttDeliveryQoS {
        match self {
            Self::AtMostOnce => MqttDeliveryQoS::AtMostOnce,
            Self::ExactlyOnce => MqttDeliveryQoS::ExactlyOnce,
        }
    }
}

#[derive(Debug, Clone)]
pub struct ThinMqttPublish {
    pub topic: String,
    pub payload: Vec<u8>,
    pub qos: u8,
    pub packet_id: Option<u16>,
    publish: ParsedMqttPublish,
}

#[derive(Debug)]
pub struct ThinControlMqttClient {
    stream: TcpStream,
    next_packet_id: u16,
}

impl ThinControlMqttClient {
    pub fn connect(credential: &ThinMqttCredential, subscribe_topic: &str) -> Result<Self, String> {
        Self::connect_with_subscription_suffix(credential, subscribe_topic, "v2-worker")
    }

    pub fn connect_with_subscription_suffix(
        credential: &ThinMqttCredential,
        subscribe_topic: &str,
        client_suffix: &str,
    ) -> Result<Self, String> {
        let mut client = Self::connect_without_subscription(credential, client_suffix)?;
        client.subscribe(subscribe_topic)?;
        Ok(client)
    }

    pub fn connect_without_subscription(
        credential: &ThinMqttCredential,
        client_suffix: &str,
    ) -> Result<Self, String> {
        let endpoint = parse_mqtt_url(&credential.broker_url)?;
        let mut stream = TcpStream::connect(endpoint.authority.as_str())
            .map_err(|err| format!("connect control mqtt {}: {err}", endpoint.authority))?;
        stream
            .set_read_timeout(Some(DEFAULT_IO_TIMEOUT))
            .map_err(|err| format!("set read timeout: {err}"))?;
        stream
            .set_write_timeout(Some(DEFAULT_IO_TIMEOUT))
            .map_err(|err| format!("set write timeout: {err}"))?;

        let suffix = client_suffix.trim_matches('-').trim();
        let client_id = if suffix.is_empty() {
            credential.client_id.clone()
        } else {
            format!("{}-{suffix}", credential.client_id)
        };
        let connect = mqtt_connect_packet(&client_id, &credential.username, &credential.password)?;
        stream
            .write_all(&connect)
            .map_err(|err| format!("write mqtt connect: {err}"))?;
        stream
            .flush()
            .map_err(|err| format!("flush mqtt connect: {err}"))?;
        read_mqtt_connack(&mut stream)?;

        Ok(Self {
            stream,
            next_packet_id: 2,
        })
    }

    pub fn subscribe(&mut self, topic_filter: &str) -> Result<(), String> {
        let packet_id = self.next_publish_packet_id();
        let subscribe = mqtt_subscribe_packet(packet_id, topic_filter)?;
        self.stream
            .write_all(&subscribe)
            .map_err(|err| format!("write mqtt subscribe: {err}"))?;
        self.stream
            .flush()
            .map_err(|err| format!("flush mqtt subscribe: {err}"))?;
        read_mqtt_suback(&mut self.stream, packet_id)
    }

    pub fn publish(&mut self, topic: &str, payload: &[u8], qos: ThinMqttQoS) -> Result<(), String> {
        let qos = qos.delivery_qos();
        let packet_id = if qos == MqttDeliveryQoS::ExactlyOnce {
            Some(self.next_publish_packet_id())
        } else {
            None
        };
        let packet = mqtt_publish_packet(topic, payload, qos, packet_id)?;
        self.stream
            .write_all(&packet)
            .map_err(|err| format!("write mqtt publish: {err}"))?;
        self.stream
            .flush()
            .map_err(|err| format!("flush mqtt publish: {err}"))?;
        if let Some(packet_id) = packet_id {
            complete_mqtt_qos2_publish(&mut self.stream, packet_id)?;
        }
        Ok(())
    }

    pub fn read_publish(&mut self, timeout: Duration) -> Result<Option<ThinMqttPublish>, String> {
        self.stream
            .set_read_timeout(Some(timeout))
            .map_err(|err| format!("set thin mqtt read timeout: {err}"))?;
        let publish = match read_mqtt_publish(&mut self.stream) {
            Ok(publish) => publish,
            Err(err) if is_control_mqtt_timeout_error(&err) => {
                let _ = self.stream.set_read_timeout(Some(DEFAULT_IO_TIMEOUT));
                return Ok(None);
            }
            Err(err) => {
                let _ = self.stream.set_read_timeout(Some(DEFAULT_IO_TIMEOUT));
                return Err(err);
            }
        };
        self.stream
            .set_read_timeout(Some(DEFAULT_IO_TIMEOUT))
            .map_err(|err| format!("reset thin mqtt read timeout: {err}"))?;
        persist_mqtt_downstream_inbox(&publish.topic, &publish.payload)?;
        Ok(Some(ThinMqttPublish {
            topic: publish.topic.clone(),
            payload: publish.payload.clone(),
            qos: publish.qos,
            packet_id: publish.packet_id,
            publish,
        }))
    }

    pub fn ack_publish(&mut self, publish: &ThinMqttPublish) -> Result<(), String> {
        ack_mqtt_publish(&mut self.stream, &publish.publish)
    }

    pub fn ping(&mut self) -> Result<(), String> {
        self.stream
            .write_all(&[0xc0, 0x00])
            .map_err(|err| format!("write mqtt ping request: {err}"))?;
        self.stream
            .flush()
            .map_err(|err| format!("flush mqtt ping request: {err}"))
    }

    fn next_publish_packet_id(&mut self) -> u16 {
        let packet_id = self.next_packet_id;
        self.next_packet_id = if self.next_packet_id == u16::MAX {
            2
        } else {
            self.next_packet_id + 1
        };
        packet_id
    }
}

#[derive(Debug, Clone)]
struct MqttEndpoint {
    authority: String,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum MqttDeliveryQoS {
    AtMostOnce,
    ExactlyOnce,
}

impl MqttDeliveryQoS {
    fn packet_qos(self) -> u8 {
        match self {
            Self::AtMostOnce => MQTT_QOS_AT_MOST_ONCE,
            Self::ExactlyOnce => MQTT_QOS_EXACTLY_ONCE,
        }
    }
}

fn parse_mqtt_url(url: &str) -> Result<MqttEndpoint, String> {
    let trimmed = url.trim();
    let stripped = trimmed
        .strip_prefix("mqtt://")
        .ok_or_else(|| "only mqtt:// control broker URLs are currently supported".to_string())?;
    let authority = stripped.split('/').next().unwrap_or(stripped).trim();
    if authority.is_empty() {
        return Err("invalid mqtt broker url".to_string());
    }
    let authority = if authority.contains(':') {
        authority.to_string()
    } else {
        format!("{authority}:1883")
    };
    Ok(MqttEndpoint { authority })
}

fn mqtt_connect_packet(client_id: &str, username: &str, password: &str) -> Result<Vec<u8>, String> {
    let mut variable = Vec::new();
    mqtt_write_string(&mut variable, "MQTT")?;
    variable.push(0x05);
    variable.push(0x02 | 0x80 | 0x40);
    variable.extend_from_slice(&30_u16.to_be_bytes());
    variable.push(0x00);
    mqtt_write_string(&mut variable, client_id)?;
    mqtt_write_string(&mut variable, username)?;
    mqtt_write_string(&mut variable, password)?;
    let mut packet = vec![0x10];
    packet.extend_from_slice(&mqtt_remaining_length(variable.len())?);
    packet.extend_from_slice(&variable);
    Ok(packet)
}

fn mqtt_publish_packet(
    topic: &str,
    payload: &[u8],
    qos: MqttDeliveryQoS,
    packet_id: Option<u16>,
) -> Result<Vec<u8>, String> {
    let mut variable = Vec::new();
    mqtt_write_string(&mut variable, topic)?;
    if qos == MqttDeliveryQoS::ExactlyOnce {
        let packet_id =
            packet_id.ok_or_else(|| "mqtt qos2 publish missing packet id".to_string())?;
        variable.extend_from_slice(&packet_id.to_be_bytes());
    }
    variable.push(0x00);
    variable.extend_from_slice(payload);
    let mut packet = vec![0x30 | (qos.packet_qos() << 1)];
    packet.extend_from_slice(&mqtt_remaining_length(variable.len())?);
    packet.extend_from_slice(&variable);
    Ok(packet)
}

fn mqtt_subscribe_packet(packet_id: u16, topic_filter: &str) -> Result<Vec<u8>, String> {
    let mut variable = Vec::new();
    variable.extend_from_slice(&packet_id.to_be_bytes());
    variable.push(0x00);
    mqtt_write_string(&mut variable, topic_filter)?;
    variable.push(MQTT_QOS_EXACTLY_ONCE);
    let mut packet = vec![0x82];
    packet.extend_from_slice(&mqtt_remaining_length(variable.len())?);
    packet.extend_from_slice(&variable);
    Ok(packet)
}

fn complete_mqtt_qos2_publish(stream: &mut TcpStream, packet_id: u16) -> Result<(), String> {
    loop {
        let packet = read_mqtt_packet(stream)?;
        match packet.header & 0xf0 {
            0x50 => {
                let pubrec_id = read_packet_id(&packet.body, "pubrec")?;
                if pubrec_id != packet_id {
                    return Err("mqtt pubrec packet id mismatch".to_string());
                }
                write_mqtt_packet_id(stream, 0x62, packet_id, "pubrel")?;
                break;
            }
            0xc0 => write_ping_response(stream)?,
            0xd0 => {}
            _ => {}
        }
    }

    loop {
        let packet = read_mqtt_packet(stream)?;
        match packet.header & 0xf0 {
            0x70 => {
                let pubcomp_id = read_packet_id(&packet.body, "pubcomp")?;
                if pubcomp_id != packet_id {
                    return Err("mqtt pubcomp packet id mismatch".to_string());
                }
                return Ok(());
            }
            0xc0 => write_ping_response(stream)?,
            0xd0 => {}
            _ => {}
        }
    }
}

fn mqtt_write_string(buf: &mut Vec<u8>, value: &str) -> Result<(), String> {
    let len = value.as_bytes().len();
    if len > u16::MAX as usize {
        return Err("mqtt string too long".to_string());
    }
    buf.extend_from_slice(&(len as u16).to_be_bytes());
    buf.extend_from_slice(value.as_bytes());
    Ok(())
}

fn mqtt_remaining_length(mut length: usize) -> Result<Vec<u8>, String> {
    if length > 268_435_455 {
        return Err("mqtt remaining length out of range".to_string());
    }
    let mut out = Vec::new();
    loop {
        let mut digit = (length % 128) as u8;
        length /= 128;
        if length > 0 {
            digit |= 128;
        }
        out.push(digit);
        if length == 0 {
            return Ok(out);
        }
    }
}

fn read_mqtt_connack(stream: &mut TcpStream) -> Result<(), String> {
    let packet = read_mqtt_packet(stream)?;
    if packet.header != 0x20 || packet.body.len() < 2 || packet.body[1] != 0 {
        return Err("mqtt broker rejected connection".to_string());
    }
    Ok(())
}

fn read_mqtt_suback(reader: &mut impl Read, packet_id: u16) -> Result<(), String> {
    let packet = read_mqtt_packet(reader)?;
    if packet.header & 0xf0 != 0x90 || packet.body.len() < 3 {
        return Err("mqtt subscribe rejected".to_string());
    }
    if u16::from_be_bytes([packet.body[0], packet.body[1]]) != packet_id {
        return Err("mqtt subscribe packet id mismatch".to_string());
    }
    let (property_len, property_bytes) = mqtt_decode_remaining_length(&packet.body, 2)
        .ok_or_else(|| "mqtt suback properties truncated".to_string())?;
    let cursor = 2 + property_bytes + property_len;
    if cursor >= packet.body.len() {
        return Err("mqtt subscribe rejected".to_string());
    }
    let codes = &packet.body[cursor..];
    if codes.iter().any(|code| *code >= 0x80) {
        return Err(format!("mqtt subscribe rejected codes={}", mqtt_hex(codes)));
    }
    Ok(())
}

fn mqtt_hex(bytes: &[u8]) -> String {
    bytes
        .iter()
        .map(|byte| format!("{byte:02x}"))
        .collect::<Vec<_>>()
        .join("")
}

fn read_mqtt_publish(stream: &mut TcpStream) -> Result<ParsedMqttPublish, String> {
    loop {
        let packet = read_mqtt_packet(stream)?;
        match packet.header & 0xf0 {
            0x30 => return parse_mqtt_publish(packet.header, &packet.body),
            0xd0 => continue,
            0xc0 => write_ping_response(stream)?,
            _ => continue,
        }
    }
}

#[derive(Debug, Clone)]
struct ParsedMqttPublish {
    topic: String,
    packet_id: Option<u16>,
    qos: u8,
    payload: Vec<u8>,
}

struct MqttPacket {
    header: u8,
    body: Vec<u8>,
}

fn read_mqtt_packet(stream: &mut impl Read) -> Result<MqttPacket, String> {
    let mut header = [0_u8; 1];
    stream
        .read_exact(&mut header)
        .map_err(|err| format!("read mqtt packet header: {err}"))?;
    let remaining = read_mqtt_remaining_length(stream)?;
    let mut body = vec![0_u8; remaining];
    stream
        .read_exact(&mut body)
        .map_err(|err| format!("read mqtt packet payload: {err}"))?;
    Ok(MqttPacket {
        header: header[0],
        body,
    })
}

fn read_mqtt_remaining_length(stream: &mut impl Read) -> Result<usize, String> {
    let mut multiplier = 1_usize;
    let mut value = 0_usize;
    for _ in 0..4 {
        let mut encoded = [0_u8; 1];
        stream
            .read_exact(&mut encoded)
            .map_err(|err| format!("read mqtt remaining length: {err}"))?;
        value += ((encoded[0] & 127) as usize) * multiplier;
        if encoded[0] & 128 == 0 {
            return Ok(value);
        }
        multiplier *= 128;
    }
    Err("malformed mqtt remaining length".to_string())
}

fn parse_mqtt_publish(header: u8, body: &[u8]) -> Result<ParsedMqttPublish, String> {
    if body.len() < 2 {
        return Err("mqtt publish too short".to_string());
    }
    let topic_len = u16::from_be_bytes([body[0], body[1]]) as usize;
    if body.len() < 2 + topic_len {
        return Err("mqtt publish topic truncated".to_string());
    }
    let topic = String::from_utf8(body[2..2 + topic_len].to_vec())
        .map_err(|err| format!("mqtt publish topic utf8: {err}"))?;
    let mut payload_offset = 2 + topic_len;
    let qos = (header >> 1) & 0x03;
    let packet_id = if qos > 0 {
        if body.len() < payload_offset + 2 {
            return Err("mqtt publish packet id truncated".to_string());
        }
        let packet_id = u16::from_be_bytes([body[payload_offset], body[payload_offset + 1]]);
        payload_offset += 2;
        Some(packet_id)
    } else {
        None
    };
    let (property_len, property_bytes) = mqtt_decode_remaining_length(body, payload_offset)
        .ok_or_else(|| "mqtt publish properties truncated".to_string())?;
    payload_offset += property_bytes + property_len;
    if payload_offset > body.len() {
        return Err("mqtt publish properties truncated".to_string());
    }
    Ok(ParsedMqttPublish {
        topic,
        packet_id,
        qos,
        payload: body[payload_offset..].to_vec(),
    })
}

fn mqtt_decode_remaining_length(data: &[u8], start: usize) -> Option<(usize, usize)> {
    let mut multiplier = 1_usize;
    let mut value = 0_usize;
    let mut index = start;
    while index < data.len() {
        let encoded = data[index];
        value += ((encoded & 127) as usize) * multiplier;
        index += 1;
        if encoded & 128 == 0 {
            return Some((value, index - start));
        }
        multiplier *= 128;
        if multiplier > 128 * 128 * 128 {
            return None;
        }
    }
    None
}

fn ack_mqtt_publish(stream: &mut TcpStream, publish: &ParsedMqttPublish) -> Result<(), String> {
    match (publish.qos, publish.packet_id) {
        (0, _) => Ok(()),
        (1, Some(packet_id)) => write_mqtt_packet_id(stream, 0x40, packet_id, "puback"),
        (2, Some(packet_id)) => {
            write_mqtt_packet_id(stream, 0x50, packet_id, "pubrec")?;
            loop {
                let packet = read_mqtt_packet(stream)?;
                match packet.header & 0xf0 {
                    0x60 => {
                        let pubrel_id = read_packet_id(&packet.body, "pubrel")?;
                        if pubrel_id == packet_id {
                            return write_mqtt_packet_id(stream, 0x70, packet_id, "pubcomp");
                        }
                    }
                    0xc0 => write_ping_response(stream)?,
                    0xd0 => {}
                    _ => {}
                }
            }
        }
        (3, _) => Err("invalid mqtt publish qos 3".to_string()),
        _ => Err("mqtt publish missing packet id".to_string()),
    }
}

fn read_packet_id(body: &[u8], packet_name: &str) -> Result<u16, String> {
    if body.len() < 2 {
        return Err(format!("mqtt {packet_name} packet id truncated"));
    }
    Ok(u16::from_be_bytes([body[0], body[1]]))
}

fn write_mqtt_packet_id(
    stream: &mut TcpStream,
    header: u8,
    packet_id: u16,
    packet_name: &str,
) -> Result<(), String> {
    let mut packet = vec![header, 0x02];
    packet.extend_from_slice(&packet_id.to_be_bytes());
    stream
        .write_all(&packet)
        .map_err(|err| format!("write mqtt {packet_name}: {err}"))?;
    stream
        .flush()
        .map_err(|err| format!("flush mqtt {packet_name}: {err}"))
}

fn write_ping_response(stream: &mut TcpStream) -> Result<(), String> {
    stream
        .write_all(&[0xd0, 0x00])
        .map_err(|err| format!("write mqtt ping response: {err}"))
}

fn persist_mqtt_downstream_inbox(topic: &str, payload: &[u8]) -> Result<(), String> {
    if is_heartbeat_message(topic, payload) {
        return Ok(());
    }
    let path = mqtt_downstream_inbox_path();
    if let Some(parent) = path.parent() {
        std::fs::create_dir_all(parent).map_err(|err| {
            format!(
                "create mqtt downstream inbox directory {}: {err}",
                parent.display()
            )
        })?;
    }
    let mut entries = std::fs::read_to_string(&path).unwrap_or_default();
    if entries.trim().is_empty() {
        entries.push_str("<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<mqttDownstreamInbox>\n");
    } else if let Some(index) = entries.rfind("</mqttDownstreamInbox>") {
        entries.truncate(index);
    } else {
        entries.push('\n');
    }
    let payload_text = String::from_utf8_lossy(payload);
    entries.push_str("  <message");
    entries.push_str(&format!(" receivedAtMs=\"{}\"", current_timestamp_ms()));
    entries.push_str(&format!(" topic=\"{}\"", xml_escape(topic)));
    entries.push_str(&format!(" payload=\"{}\"", xml_escape(&payload_text)));
    entries.push_str(" />\n</mqttDownstreamInbox>\n");
    std::fs::write(&path, entries)
        .map_err(|err| format!("write mqtt downstream inbox {}: {err}", path.display()))
}

fn is_heartbeat_message(topic: &str, payload: &[u8]) -> bool {
    topic
        .split('/')
        .any(|segment| segment.eq_ignore_ascii_case("heartbeat"))
        || is_heartbeat_payload(payload)
}

fn is_heartbeat_payload(payload: &[u8]) -> bool {
    let payload_text = String::from_utf8_lossy(payload);
    let normalized: String = payload_text
        .chars()
        .filter(|ch| !ch.is_ascii_whitespace())
        .flat_map(char::to_lowercase)
        .collect();
    normalized.contains("\"type\":\"ping\"")
        || normalized.contains("\"type\":\"pong\"")
        || normalized.contains("\"kind\":\"heartbeat\"")
}

fn mqtt_downstream_inbox_path() -> PathBuf {
    if let Ok(configured) = std::env::var("SLAN_MQTT_DOWNSTREAM_INBOX_FILE") {
        let trimmed = configured.trim();
        if !trimmed.is_empty() {
            return PathBuf::from(trimmed);
        }
    }
    #[cfg(target_os = "windows")]
    if let Ok(program_data) = std::env::var("ProgramData") {
        let trimmed = program_data.trim();
        if !trimmed.is_empty() {
            return PathBuf::from(trimmed).join("SLAN").join("mqtt-inbox.xml");
        }
    }
    #[cfg(target_os = "windows")]
    return PathBuf::from(r"C:\ProgramData\SLAN\mqtt-inbox.xml");
    #[cfg(not(target_os = "windows"))]
    std::env::temp_dir().join("slan-mqtt-inbox.xml")
}

fn current_timestamp_ms() -> u64 {
    std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .map(|duration| duration.as_millis() as u64)
        .unwrap_or_default()
}

fn xml_escape(value: &str) -> String {
    value
        .replace('&', "&amp;")
        .replace('"', "&quot;")
        .replace('<', "&lt;")
        .replace('>', "&gt;")
}

fn is_control_mqtt_timeout_error(error: &str) -> bool {
    let lower = error.to_ascii_lowercase();
    lower.contains("timed out")
        || lower.contains("would block")
        || lower.contains("try again")
        || lower.contains("resource temporarily unavailable")
        || lower.contains("os error 11")
        || lower.contains("os error 35")
        || lower.contains("os error 10060")
}

#[cfg(test)]
mod tests {
    use super::{is_heartbeat_message, is_heartbeat_payload};

    #[test]
    fn heartbeat_topic_is_not_persisted() {
        assert!(is_heartbeat_message(
            "slan/devices/device-1/heartbeat",
            br#"{"reportedAtMs":1}"#
        ));
    }

    #[test]
    fn ping_and_pong_payloads_are_not_persisted() {
        assert!(is_heartbeat_payload(
            br#"{"type":"ping","payload":{"timestamp":1}}"#
        ));
        assert!(is_heartbeat_payload(
            br#"{"type": "pong", "payload": {"timestamp": 1}}"#
        ));
    }

    #[test]
    fn non_heartbeat_payload_is_persisted() {
        assert!(!is_heartbeat_message(
            "slan/device-1/control/down",
            br#"{"type":"node_hello_ack","payload":{"heartbeatSeconds":15}}"#
        ));
    }
}
