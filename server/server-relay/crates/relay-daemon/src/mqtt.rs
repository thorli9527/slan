use std::{
    io::{Read, Write},
    net::TcpStream,
    time::{Duration, SystemTime, UNIX_EPOCH},
};

use hmac::{Hmac, Mac};
use serde::Serialize;
use sha2::Sha256;

use crate::config::RelayMqttConfig;

type HmacSha256 = Hmac<Sha256>;

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct RelayHeartbeatPayload {
    pub node_id: String,
    pub cluster_id: Option<String>,
    pub country_code: Option<String>,
    pub city_code: Option<String>,
    pub transport: String,
    pub address: String,
    pub healthy: bool,
    pub active_sessions: usize,
    pub reported_at_ms: u64,
}

pub fn publish_relay_heartbeat(
    config: &RelayMqttConfig,
    payload: &RelayHeartbeatPayload,
) -> Result<(), String> {
    if !config.enabled {
        return Ok(());
    }
    let credential = relay_credential(config)?;
    let endpoint = parse_mqtt_url(&config.broker_url)?;
    let mut stream = TcpStream::connect(endpoint.as_str())
        .map_err(|err| format!("connect relay mqtt {endpoint}: {err}"))?;
    stream
        .set_read_timeout(Some(Duration::from_secs(5)))
        .map_err(|err| format!("set mqtt read timeout: {err}"))?;
    stream
        .set_write_timeout(Some(Duration::from_secs(5)))
        .map_err(|err| format!("set mqtt write timeout: {err}"))?;

    let connect = mqtt_connect_packet(
        &format!("{}-heartbeat", credential.client_id),
        &credential.username,
        &credential.password,
    )?;
    stream
        .write_all(&connect)
        .map_err(|err| format!("write mqtt connect: {err}"))?;
    stream
        .flush()
        .map_err(|err| format!("flush mqtt connect: {err}"))?;
    read_mqtt_connack(&mut stream)?;

    let body = serde_json::to_vec(payload).map_err(|err| format!("encode heartbeat: {err}"))?;
    let publish = mqtt_publish_packet(&credential.topic, &body)?;
    stream
        .write_all(&publish)
        .map_err(|err| format!("write mqtt publish: {err}"))?;
    stream
        .flush()
        .map_err(|err| format!("flush mqtt publish: {err}"))?;
    Ok(())
}

pub fn current_timestamp_ms() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|duration| duration.as_millis() as u64)
        .unwrap_or_default()
}

struct RelayMqttCredential {
    client_id: String,
    username: String,
    password: String,
    topic: String,
}

fn relay_credential(config: &RelayMqttConfig) -> Result<RelayMqttCredential, String> {
    let node_id = config.node_id.trim();
    if node_id.is_empty() {
        return Err("relay mqtt node_id is required".to_string());
    }
    let username_prefix = config.username_prefix.trim();
    if username_prefix.is_empty() {
        return Err("relay mqtt username_prefix is required".to_string());
    }
    let expires_at =
        current_timestamp_seconds() + config.credential_ttl_seconds.unwrap_or(86_400).max(60);
    let id = format!("relay/{node_id}");
    let client_id = format!("{username_prefix}-{id}");
    let username = format!("{username_prefix}/{id}/{expires_at}");
    let password = mqtt_password(&config.password_secret, &client_id, &username, &id)?;
    let topic = format!(
        "{}/relays/{node_id}/heartbeat",
        trim_topic(&config.topic_prefix)
    );
    Ok(RelayMqttCredential {
        client_id,
        username,
        password,
        topic,
    })
}

fn mqtt_password(
    secret: &str,
    client_id: &str,
    username: &str,
    id: &str,
) -> Result<String, String> {
    let mut mac = HmacSha256::new_from_slice(secret.as_bytes()).map_err(|err| err.to_string())?;
    mac.update(client_id.as_bytes());
    mac.update(&[0]);
    mac.update(username.as_bytes());
    mac.update(&[0]);
    mac.update(id.as_bytes());
    Ok(to_hex(&mac.finalize().into_bytes()))
}

fn current_timestamp_seconds() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|duration| duration.as_secs())
        .unwrap_or_default()
}

fn parse_mqtt_url(url: &str) -> Result<String, String> {
    let stripped = url
        .trim()
        .strip_prefix("mqtt://")
        .ok_or_else(|| "only mqtt:// relay heartbeat broker URLs are supported".to_string())?;
    let authority = stripped.split('/').next().unwrap_or(stripped).trim();
    if authority.is_empty() {
        return Err("relay mqtt broker authority is empty".to_string());
    }
    if authority.contains(':') {
        Ok(authority.to_string())
    } else {
        Ok(format!("{authority}:1883"))
    }
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

fn read_mqtt_connack(stream: &mut impl Read) -> Result<(), String> {
    let mut header = [0_u8; 1];
    stream
        .read_exact(&mut header)
        .map_err(|err| format!("read mqtt connack header: {err}"))?;
    let remaining = read_mqtt_remaining_length(stream)?;
    let mut body = vec![0_u8; remaining];
    stream
        .read_exact(&mut body)
        .map_err(|err| format!("read mqtt connack body: {err}"))?;
    if header[0] != 0x20 || body.len() != 2 || body[1] != 0 {
        return Err("mqtt broker rejected relay heartbeat connection".to_string());
    }
    Ok(())
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

fn trim_topic(topic: &str) -> String {
    let value = topic.trim().trim_matches('/');
    if value.is_empty() {
        "slan".to_string()
    } else {
        value.to_string()
    }
}

fn to_hex(bytes: &[u8]) -> String {
    const HEX: &[u8; 16] = b"0123456789abcdef";
    let mut out = String::with_capacity(bytes.len() * 2);
    for byte in bytes {
        out.push(HEX[(byte >> 4) as usize] as char);
        out.push(HEX[(byte & 0x0f) as usize] as char);
    }
    out
}
