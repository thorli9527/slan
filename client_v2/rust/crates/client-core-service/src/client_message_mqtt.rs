use std::{thread, time::Duration};

use anyhow::{bail, Context, Result};
use control_mqtt_client::{ThinControlMqttClient, ThinMqttCredential, ThinMqttQoS};
use serde_json::Value;

use crate::{control_plane::MqttCredential, session_store::current_timestamp_ms};

pub(crate) const CLIENT_PING_BODY_PREFIX: &str = "SLAN_PING:";
pub(crate) const CLIENT_PONG_BODY_PREFIX: &str = "SLAN_PONG:";

pub(crate) fn publish_client_message(
    mqtt: &MqttCredential,
    network_id: &str,
    from_device_id: &str,
    target_device_id: &str,
    body: &str,
    metadata: Option<&Value>,
) -> Result<Value> {
    let network_id = network_id.trim();
    let from_device_id = from_device_id.trim();
    let target_device_id = target_device_id.trim();
    let body = body.trim();
    if network_id.is_empty()
        || from_device_id.is_empty()
        || target_device_id.is_empty()
        || body.is_empty()
    {
        bail!("networkId, fromDeviceId, targetDeviceId and body are required");
    }

    let message_id = format!("client-msg-{}-{target_device_id}", current_timestamp_ms());
    let payload = serde_json::json!({
        "messageId": message_id,
        "networkId": network_id,
        "fromDeviceId": from_device_id,
        "targetDeviceId": target_device_id,
        "body": body,
        "metadata": metadata.cloned().unwrap_or_else(|| serde_json::json!({})),
    });
    let envelope = serde_json::json!({
        "type": "client_message",
        "messageId": message_id,
        "payload": payload,
    });
    let body = serde_json::to_vec(&envelope).context("encode client message mqtt envelope")?;
    let topic = network_broadcast_topic(mqtt, network_id);
    eprintln!(
        "client-core-service publishing client_message messageId={} networkId={} fromDeviceId={} targetDeviceId={} topic={}",
        message_id, network_id, from_device_id, target_device_id, topic
    );
    let credential = ThinMqttCredential {
        broker_url: mqtt.broker_url.clone(),
        client_id: mqtt.client_id.clone(),
        username: mqtt.username.clone(),
        password: mqtt.password.clone(),
    };
    let mut errors = Vec::new();
    for attempt in 1..=3 {
        let suffix = format!("v2-publisher-{}-{attempt}", current_timestamp_ms());
        match publish_once(&credential, &suffix, &topic, &body) {
            Ok(()) => {
                eprintln!(
                    "client-core-service published client_message messageId={} topic={} qos=1",
                    message_id, topic
                );
                return Ok(serde_json::json!({
                    "messageId": message_id,
                    "transport": "mqtt",
                    "qos": 1,
                    "topic": topic,
                }));
            }
            Err(error) => {
                eprintln!(
                    "client-core-service publish client_message retry attempt={} messageId={} topic={} error={:#}",
                    attempt, message_id, topic, error
                );
                errors.push(format!("attempt {attempt}: {error:#}"));
                thread::sleep(Duration::from_millis(250 * attempt));
            }
        }
    }
    bail!("publish client message mqtt failed: {}", errors.join("; "));
}

fn network_broadcast_topic(mqtt: &MqttCredential, network_id: &str) -> String {
    let prefix = mqtt_root_topic_prefix(mqtt);
    format!("{prefix}/networks/{}/broadcast", network_id.trim())
}

fn mqtt_root_topic_prefix(mqtt: &MqttCredential) -> String {
    let prefix = mqtt.topic_prefix.trim().trim_matches('/').to_string();
    let Some(device_id) = mqtt_device_id(mqtt) else {
        return prefix;
    };
    let suffix = format!("/devices/{device_id}");
    if prefix.ends_with(&suffix) {
        return prefix[..prefix.len() - suffix.len()].to_string();
    }
    prefix
}

fn mqtt_device_id(mqtt: &MqttCredential) -> Option<&str> {
    mqtt.topic_prefix
        .trim()
        .trim_matches('/')
        .strip_prefix("slan/devices/")
        .filter(|value| !value.is_empty() && !value.contains('/'))
}

#[cfg(test)]
pub(crate) fn publish_client_message_topic_for_test(
    mqtt: &MqttCredential,
    network_id: &str,
) -> String {
    network_broadcast_topic(mqtt, network_id)
}

fn publish_once(
    credential: &ThinMqttCredential,
    client_suffix: &str,
    topic: &str,
    body: &[u8],
) -> Result<()> {
    let mut client = ThinControlMqttClient::connect_without_subscription(credential, client_suffix)
        .map_err(|err| anyhow::anyhow!(err))
        .context("connect mqtt publisher")?;
    client
        .publish(topic, body, ThinMqttQoS::AtLeastOnce)
        .map_err(|err| anyhow::anyhow!(err))
        .context("publish client message mqtt")
}

pub(crate) fn parse_client_ping_body(body: &str) -> Option<(String, u64)> {
    let value = body.trim();
    let rest = value.strip_prefix(CLIENT_PING_BODY_PREFIX)?;
    let mut parts = rest.split(':');
    let ping_id = parts.next()?.trim();
    let sent_at_ms = parts.next()?.trim().parse::<u64>().ok()?;
    if ping_id.is_empty() {
        return None;
    }
    Some((ping_id.to_string(), sent_at_ms))
}

pub(crate) fn build_client_pong_body(ping_id: &str, sent_at_ms: u64, replied_at_ms: u64) -> String {
    format!("{CLIENT_PONG_BODY_PREFIX}{ping_id}:{sent_at_ms}:{replied_at_ms}")
}
