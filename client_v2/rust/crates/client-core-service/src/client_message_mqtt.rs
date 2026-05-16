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
    let topic = format!("{}/control/up", mqtt.topic_prefix.trim_end_matches('/'));
    let mut client = ThinControlMqttClient::connect_without_subscription(
        &ThinMqttCredential {
            broker_url: mqtt.broker_url.clone(),
            client_id: mqtt.client_id.clone(),
            username: mqtt.username.clone(),
            password: mqtt.password.clone(),
        },
        "v2-publisher",
    )
    .map_err(|err| anyhow::anyhow!(err))
    .context("connect mqtt publisher")?;
    client
        .publish(&topic, &body, ThinMqttQoS::ExactlyOnce)
        .map_err(|err| anyhow::anyhow!(err))
        .context("publish client message mqtt")?;
    Ok(serde_json::json!({
        "messageId": message_id,
        "transport": "mqtt",
        "qos": 2,
        "topic": topic,
    }))
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
