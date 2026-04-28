use std::collections::VecDeque;
use std::fmt;
use std::io::{Read, Write};
use std::net::TcpStream;
use std::time::Duration;

use serde::{Deserialize, Serialize};
use serde_json::Value;
use slan_app_core::{
    AccessPolicy, DnsConfig, Endpoint, MqttCredential, NetworkMap, Peer, RelayEndpoint,
    RelayRegion, RelayTicket, Route,
};

const DEFAULT_IO_TIMEOUT: Duration = Duration::from_secs(5);

#[derive(Debug, Clone)]
pub struct ControlMqttConfig {
    pub access_token: String,
    pub session_token: String,
    pub user_id: String,
    pub device_id: String,
    pub node_id: String,
    pub node_public_key: String,
    pub network_id: String,
    pub capabilities: Vec<String>,
    pub mqtt: MqttCredential,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct NodeHelloAck {
    pub control_session_id: String,
    pub heartbeat_seconds: u32,
    pub network_revision: u64,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct Pong {
    pub timestamp: i64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ControlSessionBootstrap {
    pub ack: NodeHelloAck,
    pub network_map: NetworkMap,
}

#[derive(Debug, Clone)]
pub enum ControlMqttEvent {
    PeerUpdate(ControlMqttPeerUpdate),
    PeerRemove(ControlMqttPeerRemove),
    ConnectPlan(ControlMqttConnectPlan),
    NetworkRestartRequired(ControlMqttNetworkRestartRequired),
    DeviceIPReassigned(ControlMqttDeviceIPReassigned),
    ActiveNetworkEnabled(ControlMqttActiveNetworkEnabled),
}

#[derive(Debug, Clone)]
pub struct ControlMqttPeerUpdate {
    pub network_id: String,
    pub revision: u64,
    pub peer: Peer,
}

#[derive(Debug, Clone)]
pub struct ControlMqttPeerRemove {
    pub network_id: String,
    pub revision: u64,
    pub peer_node_id: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct ControlMqttActiveNetworkEnabled {
    pub user_id: String,
    pub network_id: String,
    #[serde(default)]
    pub reason: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct ControlMqttNetworkRestartRequired {
    pub network_id: String,
    #[serde(default)]
    pub revision: u64,
    #[serde(default)]
    pub reason: String,
    #[serde(default)]
    pub default_subnet_cidr: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct ControlMqttDeviceIPReassigned {
    pub network_id: String,
    pub device_id: String,
    pub attachment_id: String,
    pub virtual_ip: String,
    #[serde(default)]
    pub reason: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ControlMqttConnectPlan {
    pub peer_node_id: String,
    pub prefer_direct: bool,
    #[serde(default)]
    pub paths: Vec<ControlMqttPathOption>,
    #[serde(default)]
    pub derp_cluster_id: String,
    #[serde(default)]
    pub preferred_derp_node_ids: Vec<String>,
    #[serde(default)]
    pub relay_ticket: Option<RelayTicket>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ControlMqttPathOption {
    pub path_type: String,
    pub endpoint: String,
    pub priority: i32,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ControlMqttConnectionStateReport {
    pub network_id: String,
    pub peer_node_id: String,
    pub path: String,
    pub state: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub reason: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub observed_rtt_ms: Option<u32>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub packet_loss_ppm: Option<u32>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub path_score: Option<u32>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub derp_node_id: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ControlMqttPathHealthReport {
    pub network_id: String,
    pub peer_node_id: String,
    pub path_type: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub endpoint: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub derp_node_id: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub observed_rtt_ms: Option<u32>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub packet_loss_ppm: Option<u32>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub path_score: Option<u32>,
    pub sampled_at_ms: u64,
}

#[derive(Debug)]
pub struct ControlMqttClient {
    stream: TcpStream,
    up_topic: String,
    source_node_id: String,
    network_id: String,
    queued_events: VecDeque<ControlMqttEvent>,
}

impl ControlMqttClient {
    pub fn connect(config: &ControlMqttConfig) -> Result<Self, String> {
        let endpoint = parse_mqtt_url(&config.mqtt.broker_url)?;
        let mut stream = TcpStream::connect(endpoint.authority.as_str())
            .map_err(|err| format!("connect control mqtt {}: {err}", endpoint.authority))?;
        stream
            .set_read_timeout(Some(DEFAULT_IO_TIMEOUT))
            .map_err(|err| format!("set read timeout: {err}"))?;
        stream
            .set_write_timeout(Some(DEFAULT_IO_TIMEOUT))
            .map_err(|err| format!("set write timeout: {err}"))?;

        let connect = mqtt_connect_packet(
            &format!("{}-control", config.mqtt.client_id),
            &config.mqtt.username,
            &config.mqtt.password,
        )?;
        stream
            .write_all(&connect)
            .map_err(|err| format!("write mqtt connect: {err}"))?;
        stream
            .flush()
            .map_err(|err| format!("flush mqtt connect: {err}"))?;
        read_mqtt_connack(&mut stream)?;

        let down_topic = format!(
            "{}/control/down",
            config.mqtt.topic_prefix.trim_end_matches('/')
        );
        let subscribe = mqtt_subscribe_packet(1, &down_topic)?;
        stream
            .write_all(&subscribe)
            .map_err(|err| format!("write mqtt subscribe: {err}"))?;
        stream
            .flush()
            .map_err(|err| format!("flush mqtt subscribe: {err}"))?;
        read_mqtt_suback(&mut stream, 1)?;

        Ok(Self {
            stream,
            up_topic: format!(
                "{}/control/up",
                config.mqtt.topic_prefix.trim_end_matches('/')
            ),
            source_node_id: config.node_id.clone(),
            network_id: config.network_id.clone(),
            queued_events: VecDeque::new(),
        })
    }

    pub fn bootstrap_session(
        &mut self,
        config: &ControlMqttConfig,
    ) -> Result<ControlSessionBootstrap, String> {
        let hello = Envelope {
            msg_type: "node_hello".to_string(),
            request_id: Some("node-hello".to_string()),
            message_id: None,
            source_node_id: None,
            network_id: None,
            payload: serde_json::to_value(NodeHelloPayload::from(config.clone()))
                .map_err(|err| format!("encode node_hello payload: {err}"))?,
        };
        self.send_envelope(&hello)?;

        let ack_env = self.read_response_envelope("node_hello_ack")?;
        if ack_env.msg_type != "node_hello_ack" {
            return Err(format!(
                "unexpected control mqtt response: {}",
                ack_env.msg_type
            ));
        }
        let ack: NodeHelloAck = serde_json::from_value(ack_env.payload)
            .map_err(|err| format!("decode node_hello_ack: {err}"))?;

        let network_env = self.read_response_envelope("network_map_response")?;
        if network_env.msg_type != "network_map_response" {
            return Err(format!(
                "unexpected control mqtt response after node_hello_ack: {}",
                network_env.msg_type
            ));
        }
        let network_map = decode_network_map_response(network_env.payload)?;

        Ok(ControlSessionBootstrap { ack, network_map })
    }

    pub fn request_network_map(&mut self, network_id: &str) -> Result<NetworkMap, String> {
        self.request_network_map_since(network_id, 0)
    }

    pub fn request_network_map_since(
        &mut self,
        network_id: &str,
        last_revision: u64,
    ) -> Result<NetworkMap, String> {
        let request = Envelope {
            msg_type: "network_map_request".to_string(),
            request_id: Some("network-map".to_string()),
            message_id: None,
            source_node_id: None,
            network_id: None,
            payload: serde_json::json!({
                "networkId": network_id,
                "lastRevision": last_revision,
            }),
        };
        self.send_envelope(&request)?;
        let response = self.read_response_envelope("network_map_response")?;
        if response.msg_type != "network_map_response" {
            return Err(format!(
                "unexpected network map response: {}",
                response.msg_type
            ));
        }
        decode_network_map_response(response.payload)
    }

    pub fn ping(&mut self, timestamp: i64) -> Result<Pong, String> {
        let request = Envelope {
            msg_type: "ping".to_string(),
            request_id: Some("ping".to_string()),
            message_id: None,
            source_node_id: None,
            network_id: None,
            payload: serde_json::json!({
                "timestamp": timestamp,
            }),
        };
        self.send_envelope(&request)?;
        let response = self.read_response_envelope("pong")?;
        if response.msg_type != "pong" {
            return Err(format!(
                "unexpected control mqtt response to ping: {}",
                response.msg_type
            ));
        }
        serde_json::from_value(response.payload).map_err(|err| format!("decode pong: {err}"))
    }

    pub fn send_connection_state(
        &mut self,
        report: &ControlMqttConnectionStateReport,
    ) -> Result<(), String> {
        let request = Envelope {
            msg_type: "connection_state".to_string(),
            request_id: Some("connection-state".to_string()),
            message_id: None,
            source_node_id: None,
            network_id: None,
            payload: serde_json::to_value(report)
                .map_err(|err| format!("encode connection_state payload: {err}"))?,
        };
        self.send_envelope(&request)
    }

    pub fn send_path_health_report(
        &mut self,
        report: &ControlMqttPathHealthReport,
    ) -> Result<(), String> {
        let request = Envelope {
            msg_type: "path_health_report".to_string(),
            request_id: Some("path-health".to_string()),
            message_id: None,
            source_node_id: None,
            network_id: None,
            payload: serde_json::to_value(report)
                .map_err(|err| format!("encode path_health_report payload: {err}"))?,
        };
        self.send_envelope(&request)
    }

    pub fn read_event(&mut self) -> Result<ControlMqttEvent, String> {
        if let Some(event) = self.queued_events.pop_front() {
            return Ok(event);
        }
        let response = self.read_envelope()?;
        if response.msg_type == "error" {
            return Err(format_control_error(response.payload));
        }
        decode_control_mqtt_event(response)
    }

    pub fn drain_pending_events(
        &mut self,
        timeout: Duration,
        limit: usize,
    ) -> Result<Vec<ControlMqttEvent>, String> {
        self.stream
            .set_read_timeout(Some(timeout))
            .map_err(|err| format!("set drain read timeout: {err}"))?;
        let mut events = Vec::new();
        for _ in 0..limit {
            match self.read_event() {
                Ok(event) => events.push(event),
                Err(err) if is_control_mqtt_timeout_error(&err) => break,
                Err(err) => {
                    self.stream
                        .set_read_timeout(Some(DEFAULT_IO_TIMEOUT))
                        .map_err(|reset_err| {
                            format!("reset read timeout after error: {reset_err}")
                        })?;
                    return Err(err);
                }
            }
        }
        self.stream
            .set_read_timeout(Some(DEFAULT_IO_TIMEOUT))
            .map_err(|err| format!("reset read timeout: {err}"))?;
        Ok(events)
    }

    fn send_envelope(&mut self, envelope: &Envelope) -> Result<(), String> {
        let mut envelope = envelope.clone();
        if envelope.source_node_id.is_none() {
            envelope.source_node_id = Some(self.source_node_id.clone());
        }
        if envelope.network_id.is_none() {
            envelope.network_id = Some(self.network_id.clone());
        }
        let payload =
            serde_json::to_vec(&envelope).map_err(|err| format!("encode mqtt envelope: {err}"))?;
        let packet = mqtt_publish_packet(&self.up_topic, &payload)?;
        self.stream
            .write_all(&packet)
            .map_err(|err| format!("write mqtt publish: {err}"))?;
        self.stream
            .flush()
            .map_err(|err| format!("flush mqtt publish: {err}"))?;
        Ok(())
    }

    fn read_envelope(&mut self) -> Result<Envelope, String> {
        let (_topic, payload) = read_mqtt_publish(&mut self.stream)?;
        serde_json::from_slice::<Envelope>(&payload)
            .map_err(|err| format!("decode mqtt envelope: {err}"))
    }

    fn read_response_envelope(&mut self, expected_type: &str) -> Result<Envelope, String> {
        loop {
            let response = self.read_envelope()?;
            if response.msg_type == "error" {
                return Err(format_control_error(response.payload));
            }
            if response.msg_type == expected_type {
                return Ok(response);
            }
            match decode_control_mqtt_event(response) {
                Ok(event) => self.queued_events.push_back(event),
                Err(_) => {
                    return Err(format!(
                        "unexpected control mqtt response while waiting for {expected_type}"
                    ));
                }
            }
        }
    }
}

fn decode_control_mqtt_event(response: Envelope) -> Result<ControlMqttEvent, String> {
    match response.msg_type.as_str() {
        "peer_update" => {
            let update: PeerUpdateWire = serde_json::from_value(response.payload)
                .map_err(|err| format!("decode peer_update: {err}"))?;
            Ok(ControlMqttEvent::PeerUpdate(update.into()))
        }
        "peer_remove" => {
            let remove: PeerRemoveWire = serde_json::from_value(response.payload)
                .map_err(|err| format!("decode peer_remove: {err}"))?;
            Ok(ControlMqttEvent::PeerRemove(remove.into()))
        }
        "connect_plan" => {
            let plan: ControlMqttConnectPlan = serde_json::from_value(response.payload)
                .map_err(|err| format!("decode connect_plan: {err}"))?;
            Ok(ControlMqttEvent::ConnectPlan(plan))
        }
        "network_restart_required" => {
            let restart: ControlMqttNetworkRestartRequired =
                serde_json::from_value(response.payload)
                    .map_err(|err| format!("decode network_restart_required: {err}"))?;
            Ok(ControlMqttEvent::NetworkRestartRequired(restart))
        }
        "device_ip_reassigned" => {
            let updated: ControlMqttDeviceIPReassigned =
                serde_json::from_value(response.payload)
                    .map_err(|err| format!("decode device_ip_reassigned: {err}"))?;
            Ok(ControlMqttEvent::DeviceIPReassigned(updated))
        }
        "active_network_enabled" => {
            let enabled: ControlMqttActiveNetworkEnabled = serde_json::from_value(response.payload)
                .map_err(|err| format!("decode active_network_enabled: {err}"))?;
            Ok(ControlMqttEvent::ActiveNetworkEnabled(enabled))
        }
        other => Err(format!("unexpected control mqtt event: {other}")),
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct Envelope {
    #[serde(rename = "type")]
    msg_type: String,
    #[serde(default)]
    request_id: Option<String>,
    #[serde(default)]
    message_id: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    source_node_id: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    network_id: Option<String>,
    #[serde(default)]
    payload: Value,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
struct NodeHelloPayload {
    user_id: String,
    device_id: String,
    node_id: String,
    network_id: String,
    session_token: String,
    node_public_key: String,
    capabilities: Vec<String>,
}

impl From<ControlMqttConfig> for NodeHelloPayload {
    fn from(value: ControlMqttConfig) -> Self {
        Self {
            user_id: value.user_id,
            device_id: value.device_id,
            node_id: value.node_id,
            network_id: value.network_id,
            session_token: value.session_token,
            node_public_key: value.node_public_key,
            capabilities: value.capabilities,
        }
    }
}

#[derive(Debug, Clone)]
struct MqttEndpoint {
    authority: String,
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
    variable.push(0x04);
    variable.push(0x02 | 0x80 | 0x40);
    variable.extend_from_slice(&30_u16.to_be_bytes());
    mqtt_write_string(&mut variable, client_id)?;
    mqtt_write_string(&mut variable, username)?;
    mqtt_write_string(&mut variable, password)?;
    let mut packet = vec![0x10];
    packet.extend_from_slice(&mqtt_remaining_length(variable.len())?);
    packet.extend_from_slice(&variable);
    Ok(packet)
}

fn mqtt_publish_packet(topic: &str, payload: &[u8]) -> Result<Vec<u8>, String> {
    let mut variable = Vec::new();
    mqtt_write_string(&mut variable, topic)?;
    variable.extend_from_slice(payload);
    let mut packet = vec![0x30];
    packet.extend_from_slice(&mqtt_remaining_length(variable.len())?);
    packet.extend_from_slice(&variable);
    Ok(packet)
}

fn mqtt_subscribe_packet(packet_id: u16, topic_filter: &str) -> Result<Vec<u8>, String> {
    let mut variable = Vec::new();
    variable.extend_from_slice(&packet_id.to_be_bytes());
    mqtt_write_string(&mut variable, topic_filter)?;
    variable.push(0x00);
    let mut packet = vec![0x82];
    packet.extend_from_slice(&mqtt_remaining_length(variable.len())?);
    packet.extend_from_slice(&variable);
    Ok(packet)
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
    if packet.header != 0x20 || packet.body.len() != 2 || packet.body[1] != 0 {
        return Err("mqtt broker rejected connection".to_string());
    }
    Ok(())
}

fn read_mqtt_suback(stream: &mut TcpStream, packet_id: u16) -> Result<(), String> {
    let packet = read_mqtt_packet(stream)?;
    if packet.header & 0xf0 != 0x90 || packet.body.len() < 3 {
        return Err("mqtt subscribe rejected".to_string());
    }
    if u16::from_be_bytes([packet.body[0], packet.body[1]]) != packet_id {
        return Err("mqtt subscribe packet id mismatch".to_string());
    }
    if packet.body[2..].iter().any(|code| *code == 0x80) {
        return Err("mqtt subscribe rejected".to_string());
    }
    Ok(())
}

fn read_mqtt_publish(stream: &mut TcpStream) -> Result<(String, Vec<u8>), String> {
    loop {
        let packet = read_mqtt_packet(stream)?;
        match packet.header & 0xf0 {
            0x30 => return parse_mqtt_publish(packet.header, &packet.body),
            0xd0 => continue,
            0xc0 => {
                stream
                    .write_all(&[0xd0, 0x00])
                    .map_err(|err| format!("write mqtt ping response: {err}"))?;
            }
            _ => continue,
        }
    }
}

struct MqttPacket {
    header: u8,
    body: Vec<u8>,
}

fn read_mqtt_packet(stream: &mut TcpStream) -> Result<MqttPacket, String> {
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

fn read_mqtt_remaining_length(stream: &mut TcpStream) -> Result<usize, String> {
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

fn parse_mqtt_publish(header: u8, body: &[u8]) -> Result<(String, Vec<u8>), String> {
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
    if qos > 0 {
        if body.len() < payload_offset + 2 {
            return Err("mqtt publish packet id truncated".to_string());
        }
        payload_offset += 2;
    }
    Ok((topic, body[payload_offset..].to_vec()))
}

fn format_control_error(payload: Value) -> String {
    if let Ok(error) = serde_json::from_value::<ControlError>(payload.clone()) {
        if !error.message.trim().is_empty() {
            return format!("{}: {}", error.code, error.message);
        }
        return error.code;
    }
    payload.to_string()
}

fn is_control_mqtt_timeout_error(error: &str) -> bool {
    let lower = error.to_ascii_lowercase();
    lower.contains("timed out")
        || lower.contains("would block")
        || lower.contains("resource temporarily unavailable")
        || lower.contains("os error 35")
}

#[derive(Debug, Clone, Serialize, Deserialize)]
struct ControlError {
    #[serde(default)]
    code: String,
    #[serde(default)]
    message: String,
}

fn decode_network_map_response(payload: Value) -> Result<NetworkMap, String> {
    let response: NetworkMapResponse = serde_json::from_value(payload)
        .map_err(|err| format!("decode network_map_response: {err}"))?;
    Ok(response.map.into())
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
struct NetworkMapResponse {
    map: NetworkMapWire,
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
struct PeerUpdateWire {
    network_id: String,
    revision: u64,
    peer: PeerWire,
}

impl From<PeerUpdateWire> for ControlMqttPeerUpdate {
    fn from(value: PeerUpdateWire) -> Self {
        Self {
            network_id: value.network_id,
            revision: value.revision,
            peer: value.peer.into(),
        }
    }
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
struct PeerRemoveWire {
    network_id: String,
    revision: u64,
    peer_node_id: String,
}

impl From<PeerRemoveWire> for ControlMqttPeerRemove {
    fn from(value: PeerRemoveWire) -> Self {
        Self {
            network_id: value.network_id,
            revision: value.revision,
            peer_node_id: value.peer_node_id,
        }
    }
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
struct NetworkMapWire {
    self_user_id: String,
    self_device_id: String,
    self_node_id: String,
    network_id: String,
    revision: u64,
    heartbeat_seconds: u32,
    #[serde(default)]
    stun_servers: Vec<String>,
    #[serde(default)]
    peers: Vec<PeerWire>,
    #[serde(default)]
    routes: Vec<RouteWire>,
    #[serde(default)]
    relay_regions: Vec<RelayRegionWire>,
    dns: DnsConfigWire,
    #[serde(default)]
    policy: AccessPolicyWire,
    #[serde(default)]
    mtu: Option<u32>,
}

impl From<NetworkMapWire> for NetworkMap {
    fn from(value: NetworkMapWire) -> Self {
        Self {
            self_user_id: value.self_user_id,
            self_device_id: value.self_device_id,
            self_node_id: value.self_node_id,
            network_id: value.network_id,
            revision: value.revision,
            heartbeat_seconds: value.heartbeat_seconds,
            stun_servers: value.stun_servers,
            peers: value.peers.into_iter().map(Into::into).collect(),
            routes: value.routes.into_iter().map(Into::into).collect(),
            relay_regions: value.relay_regions.into_iter().map(Into::into).collect(),
            dns: value.dns.into(),
            policy: value.policy.into(),
            mtu: value.mtu,
        }
    }
}

#[derive(Debug, Clone, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
struct AccessPolicyWire {
    #[serde(default)]
    plan_code: Option<String>,
    #[serde(default)]
    max_active_devices: Option<u32>,
    #[serde(default)]
    bandwidth_limit_mbps: Option<u32>,
    #[serde(default)]
    relay_bandwidth_limit_kbps: Option<u32>,
    #[serde(default)]
    p2p_unlimited: bool,
    #[serde(default)]
    dns_available: bool,
}

impl From<AccessPolicyWire> for AccessPolicy {
    fn from(value: AccessPolicyWire) -> Self {
        Self {
            plan_code: value.plan_code,
            max_active_devices: value.max_active_devices,
            bandwidth_limit_mbps: value.bandwidth_limit_mbps,
            relay_bandwidth_limit_kbps: value.relay_bandwidth_limit_kbps,
            p2p_unlimited: value.p2p_unlimited,
            dns_available: value.dns_available,
        }
    }
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
struct PeerWire {
    node_id: String,
    device_id: String,
    public_key: String,
    status: String,
    relay_allowed: bool,
    #[serde(default)]
    virtual_ips: Vec<String>,
    #[serde(default)]
    endpoints: Vec<EndpointWire>,
    #[serde(default)]
    allowed_routes: Vec<String>,
}

impl From<PeerWire> for Peer {
    fn from(value: PeerWire) -> Self {
        Self {
            node_id: value.node_id,
            device_id: value.device_id,
            public_key: value.public_key,
            status: value.status,
            relay_allowed: value.relay_allowed,
            virtual_ips: value.virtual_ips,
            endpoints: value.endpoints.into_iter().map(Into::into).collect(),
            allowed_routes: value.allowed_routes,
        }
    }
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
struct EndpointWire {
    #[serde(rename = "type")]
    endpoint_type: String,
    address: String,
    updated_at: i64,
}

impl From<EndpointWire> for Endpoint {
    fn from(value: EndpointWire) -> Self {
        Self {
            endpoint_type: value.endpoint_type,
            address: value.address,
            updated_at: value.updated_at,
        }
    }
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
struct RouteWire {
    cidr: String,
    via_node_id: String,
    #[serde(default)]
    metric: Option<String>,
}

impl From<RouteWire> for Route {
    fn from(value: RouteWire) -> Self {
        Self {
            cidr: value.cidr,
            via_node_id: value.via_node_id,
            metric: value.metric,
        }
    }
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
struct DnsConfigWire {
    #[serde(default)]
    servers: Vec<String>,
    #[serde(default)]
    search_domains: Vec<String>,
    #[serde(default)]
    wildcards: Vec<String>,
}

impl From<DnsConfigWire> for DnsConfig {
    fn from(value: DnsConfigWire) -> Self {
        Self {
            servers: value.servers,
            search_domains: value.search_domains,
            wildcards: value.wildcards,
        }
    }
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
struct RelayRegionWire {
    region_id: String,
    region_name: String,
    #[serde(default)]
    country_code: String,
    #[serde(default)]
    country_name: String,
    #[serde(default)]
    city_code: String,
    #[serde(default)]
    city_name: String,
    #[serde(default)]
    cluster_id: String,
    #[serde(default)]
    cluster_name: String,
    #[serde(default)]
    endpoints: Vec<RelayEndpointWire>,
}

impl From<RelayRegionWire> for RelayRegion {
    fn from(value: RelayRegionWire) -> Self {
        Self {
            region_id: value.region_id,
            region_name: value.region_name,
            country_code: if value.country_code.is_empty() {
                None
            } else {
                Some(value.country_code)
            },
            country_name: if value.country_name.is_empty() {
                None
            } else {
                Some(value.country_name)
            },
            city_code: if value.city_code.is_empty() {
                None
            } else {
                Some(value.city_code)
            },
            city_name: if value.city_name.is_empty() {
                None
            } else {
                Some(value.city_name)
            },
            cluster_id: if value.cluster_id.is_empty() {
                None
            } else {
                Some(value.cluster_id)
            },
            cluster_name: if value.cluster_name.is_empty() {
                None
            } else {
                Some(value.cluster_name)
            },
            endpoints: value.endpoints.into_iter().map(Into::into).collect(),
        }
    }
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
struct RelayEndpointWire {
    endpoint_id: String,
    transport: String,
    address: String,
}

impl From<RelayEndpointWire> for RelayEndpoint {
    fn from(value: RelayEndpointWire) -> Self {
        Self {
            endpoint_id: value.endpoint_id,
            transport: value.transport,
            address: value.address,
        }
    }
}

impl fmt::Display for ControlSessionBootstrap {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(
            f,
            "control session {} heartbeat {} peers {}",
            self.ack.control_session_id,
            self.ack.heartbeat_seconds,
            self.network_map.peers.len()
        )
    }
}
