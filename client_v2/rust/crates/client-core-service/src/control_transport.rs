use anyhow::Result;
use client_core::ClientViewState;
use serde::Deserialize;
use serde_json::Value;

use crate::{
    control_plane::MqttCredential,
    control_tasks::{ControlTask, ControlTaskStatus},
    PersistedSession,
};

#[derive(Debug, Clone, serde::Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ControlTransportStatus {
    pub mqtt_credential_ready: bool,
    pub control_session_ready: bool,
    pub ready: bool,
    pub missing: Vec<String>,
    pub mqtt_expires_at: Option<i64>,
    pub active_network_id: Option<String>,
    pub device_id: Option<String>,
}

#[derive(Debug, Clone, serde::Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ControlTransportPlan {
    pub heartbeat_qos: MqttQos,
    pub control_qos: MqttQos,
    pub heartbeat_topic: Option<String>,
    pub runtime_state_topic: Option<String>,
    pub downstream_control_topic: Option<String>,
    pub upstream_control_ack_topic: Option<String>,
}

#[derive(Debug, Clone, serde::Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ControlTransportOutbox {
    pub messages: Vec<ControlTransportMessage>,
}

#[derive(Debug, Clone, serde::Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ControlTransportMessage {
    pub id: String,
    pub topic: String,
    pub qos: MqttQos,
    pub kind: ControlTransportMessageKind,
    pub payload: Value,
    pub ack_task_id: Option<String>,
}

#[derive(Debug, Clone, Copy, serde::Serialize)]
pub enum ControlTransportMessageKind {
    #[serde(rename = "heartbeat")]
    Heartbeat,
    #[serde(rename = "runtimeState")]
    RuntimeState,
    #[serde(rename = "controlAck")]
    ControlAck,
}

#[derive(Debug, Clone, Copy, serde::Serialize)]
pub enum MqttQos {
    #[serde(rename = "qos0")]
    QoS0,
    #[serde(rename = "qos2")]
    QoS2,
}

#[derive(Debug, Clone, Deserialize, serde::Serialize)]
#[serde(rename_all = "camelCase")]
pub struct DownstreamControlMessage {
    pub action: String,
    #[serde(default)]
    pub message_id: Option<String>,
    #[serde(default)]
    pub delivery_id: Option<String>,
    #[serde(default)]
    pub require_ui_refresh: bool,
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
struct DownstreamEnvelope {
    #[serde(rename = "type")]
    message_type: String,
    #[serde(default)]
    message_id: Option<String>,
    #[serde(default)]
    payload: Value,
}

#[derive(Debug, Clone)]
pub struct AcceptedDownstreamControlMessage {
    pub action: String,
    pub delivery_id: String,
    pub require_ui_refresh: bool,
}

#[derive(Debug, Clone, serde::Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ControlTaskAck {
    pub task_id: String,
    pub delivery_id: String,
    pub action: String,
    pub status: ControlTaskAckStatus,
    pub error: Option<String>,
    pub processed_at_ms: u64,
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct PublishedControlTransportMessage {
    pub id: String,
}

#[derive(Debug, Clone, Deserialize, serde::Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ControlTransportOutboxRequest {
    #[serde(default = "default_true")]
    pub include_heartbeat: bool,
    #[serde(default = "default_true")]
    pub include_runtime_state: bool,
    #[serde(default = "default_true")]
    pub include_control_acks: bool,
}

#[derive(Debug, Clone, serde::Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ControlTransportCadence {
    pub ack_flush_interval_ms: u64,
    pub heartbeat_interval_ms: u64,
    pub runtime_state_interval_ms: u64,
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ControlTransportTickRequest {
    #[serde(default)]
    pub now_ms: Option<u64>,
    #[serde(default)]
    pub last_ack_flush_ms: Option<u64>,
    #[serde(default)]
    pub last_heartbeat_ms: Option<u64>,
    #[serde(default)]
    pub last_runtime_state_ms: Option<u64>,
}

#[derive(Debug, Clone, serde::Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ControlTransportTickPlan {
    pub now_ms: u64,
    pub outbox: ControlTransportOutboxRequest,
    pub next_ack_flush_due_ms: u64,
    pub next_heartbeat_due_ms: u64,
    pub next_runtime_state_due_ms: u64,
}

#[derive(Debug, Clone, Copy, serde::Serialize)]
pub enum ControlTaskAckStatus {
    #[serde(rename = "succeeded")]
    Succeeded,
    #[serde(rename = "failed")]
    Failed,
}

pub fn control_transport_status(session: &PersistedSession) -> ControlTransportStatus {
    let mut missing = Vec::new();
    let mqtt_credential_ready = mqtt_credential_ready(session.mqtt.as_ref(), &mut missing);
    let control_session_ready =
        required_field(session.device_id.as_deref(), "deviceId", &mut missing)
            && required_field(
                session.active_network_id.as_deref(),
                "activeNetworkId",
                &mut missing,
            );
    ControlTransportStatus {
        mqtt_credential_ready,
        control_session_ready,
        ready: mqtt_credential_ready && control_session_ready,
        missing,
        mqtt_expires_at: session
            .mqtt
            .as_ref()
            .and_then(|credential| credential.expires_at),
        active_network_id: session.active_network_id.clone(),
        device_id: session.device_id.clone(),
    }
}

pub fn control_transport_cadence() -> ControlTransportCadence {
    ControlTransportCadence {
        ack_flush_interval_ms: 1_000,
        heartbeat_interval_ms: 30_000,
        runtime_state_interval_ms: 10_000,
    }
}

pub fn control_transport_tick_plan(
    request: ControlTransportTickRequest,
    fallback_now_ms: u64,
) -> ControlTransportTickPlan {
    let cadence = control_transport_cadence();
    let now_ms = request.now_ms.unwrap_or(fallback_now_ms);
    let include_control_acks = due(
        now_ms,
        request.last_ack_flush_ms,
        cadence.ack_flush_interval_ms,
    );
    let include_heartbeat = due(
        now_ms,
        request.last_heartbeat_ms,
        cadence.heartbeat_interval_ms,
    );
    let include_runtime_state = due(
        now_ms,
        request.last_runtime_state_ms,
        cadence.runtime_state_interval_ms,
    );
    ControlTransportTickPlan {
        now_ms,
        outbox: ControlTransportOutboxRequest {
            include_heartbeat,
            include_runtime_state,
            include_control_acks,
        },
        next_ack_flush_due_ms: next_due_ms(
            now_ms,
            request.last_ack_flush_ms,
            cadence.ack_flush_interval_ms,
        ),
        next_heartbeat_due_ms: next_due_ms(
            now_ms,
            request.last_heartbeat_ms,
            cadence.heartbeat_interval_ms,
        ),
        next_runtime_state_due_ms: next_due_ms(
            now_ms,
            request.last_runtime_state_ms,
            cadence.runtime_state_interval_ms,
        ),
    }
}

pub fn control_transport_plan(session: &PersistedSession) -> ControlTransportPlan {
    let topic_prefix = session
        .mqtt
        .as_ref()
        .map(|credential| credential.topic_prefix.trim())
        .filter(|value| !value.is_empty());
    ControlTransportPlan {
        heartbeat_qos: MqttQos::QoS0,
        control_qos: MqttQos::QoS2,
        heartbeat_topic: topic_prefix.map(|prefix| format!("{prefix}/heartbeat")),
        runtime_state_topic: topic_prefix.map(|prefix| format!("{prefix}/runtime-state")),
        downstream_control_topic: topic_prefix.map(|prefix| format!("{prefix}/control/down")),
        upstream_control_ack_topic: topic_prefix.map(|prefix| format!("{prefix}/control/ack")),
    }
}

pub fn control_transport_outbox(
    session: &PersistedSession,
    state: &ClientViewState,
    acks: Vec<ControlTaskAck>,
    reported_at_ms: u64,
    include_heartbeat: bool,
    include_runtime_state: bool,
) -> ControlTransportOutbox {
    let plan = control_transport_plan(session);
    let mut messages = Vec::new();
    if include_heartbeat {
        if let Some(topic) = plan.heartbeat_topic {
            messages.push(ControlTransportMessage {
                id: format!("heartbeat-{reported_at_ms}"),
                topic,
                qos: plan.heartbeat_qos,
                kind: ControlTransportMessageKind::Heartbeat,
                ack_task_id: None,
                payload: serde_json::json!({
                    "deviceId": session.device_id.clone(),
                    "activeNetworkId": session.active_network_id.clone(),
                    "signedIn": state.signed_in,
                    "reportedAtMs": reported_at_ms,
                }),
            });
        }
    }
    if include_runtime_state {
        if let Some(topic) = plan.runtime_state_topic {
            messages.push(ControlTransportMessage {
                id: format!("runtime-state-{reported_at_ms}"),
                topic,
                qos: plan.heartbeat_qos,
                kind: ControlTransportMessageKind::RuntimeState,
                ack_task_id: None,
                payload: serde_json::json!({
                    "deviceId": session.device_id.clone(),
                    "activeNetworkId": session.active_network_id.clone(),
                    "networkEnabled": state.network_enabled,
                    "virtualIp": state.virtual_ip.clone(),
                    "syncing": state.syncing,
                    "error": state.error.clone(),
                    "reportedAtMs": reported_at_ms,
                }),
            });
        }
    }
    if let Some(topic) = plan.upstream_control_ack_topic {
        for ack in acks {
            messages.push(ControlTransportMessage {
                id: transport_ack_message_id(&ack.task_id),
                topic: topic.clone(),
                qos: plan.control_qos,
                kind: ControlTransportMessageKind::ControlAck,
                ack_task_id: Some(ack.task_id.clone()),
                payload: serde_json::to_value(ack).unwrap_or_else(|_| serde_json::json!({})),
            });
        }
    }
    ControlTransportOutbox { messages }
}

pub fn normalize_downstream_control_message(
    message: DownstreamControlMessage,
) -> Result<AcceptedDownstreamControlMessage> {
    let action = message.action.trim().to_string();
    if action.is_empty() {
        anyhow::bail!("downstream control action is empty");
    }
    let delivery_id = message
        .delivery_id
        .or(message.message_id)
        .map(|value| value.trim().to_string())
        .filter(|value| !value.is_empty())
        .ok_or_else(|| anyhow::anyhow!("downstream control deliveryId/messageId is required"))?;
    Ok(AcceptedDownstreamControlMessage {
        action,
        delivery_id,
        require_ui_refresh: message.require_ui_refresh,
    })
}

pub fn normalize_downstream_control_value(
    value: Value,
    self_device_id: Option<&str>,
) -> Result<Option<AcceptedDownstreamControlMessage>> {
    if value.get("type").and_then(Value::as_str).is_some() {
        return normalize_downstream_envelope(value, self_device_id);
    }
    let message: DownstreamControlMessage =
        serde_json::from_value(value).map_err(|err| anyhow::anyhow!("{err}"))?;
    normalize_downstream_control_message(message).map(Some)
}

fn normalize_downstream_envelope(
    value: Value,
    self_device_id: Option<&str>,
) -> Result<Option<AcceptedDownstreamControlMessage>> {
    let envelope: DownstreamEnvelope = serde_json::from_value(value)?;
    match envelope.message_type.trim() {
        "device_network_disabled" => {
            let device_id = envelope
                .payload
                .get("deviceId")
                .and_then(Value::as_str)
                .unwrap_or_default()
                .trim();
            if !message_targets_self(device_id, self_device_id) {
                return Ok(None);
            }
            Ok(Some(AcceptedDownstreamControlMessage {
                action: "disableNetwork".to_string(),
                delivery_id: envelope_delivery_id(
                    envelope.message_id,
                    "device-network-disabled",
                    &envelope.payload,
                ),
                require_ui_refresh: true,
            }))
        }
        "device_ip_reassigned" => {
            let device_id = envelope
                .payload
                .get("deviceId")
                .and_then(Value::as_str)
                .unwrap_or_default()
                .trim();
            if !message_targets_self(device_id, self_device_id) {
                return Ok(None);
            }
            let virtual_ip = envelope
                .payload
                .get("virtualIp")
                .and_then(Value::as_str)
                .unwrap_or_default()
                .trim();
            if !virtual_ip.is_empty() {
                return Ok(None);
            }
            Ok(Some(AcceptedDownstreamControlMessage {
                action: "disableNetwork".to_string(),
                delivery_id: envelope_delivery_id(
                    envelope.message_id,
                    "device-ip-cleared",
                    &envelope.payload,
                ),
                require_ui_refresh: true,
            }))
        }
        _ => Ok(None),
    }
}

fn message_targets_self(device_id: &str, self_device_id: Option<&str>) -> bool {
    let Some(self_device_id) = self_device_id else {
        return false;
    };
    !device_id.is_empty() && device_id == self_device_id.trim()
}

fn envelope_delivery_id(message_id: Option<String>, prefix: &str, payload: &Value) -> String {
    message_id
        .map(|value| value.trim().to_string())
        .filter(|value| !value.is_empty())
        .unwrap_or_else(|| {
            let network_id = payload
                .get("networkId")
                .and_then(Value::as_str)
                .unwrap_or_default();
            let device_id = payload
                .get("deviceId")
                .and_then(Value::as_str)
                .unwrap_or_default();
            let attachment_id = payload
                .get("attachmentId")
                .and_then(Value::as_str)
                .unwrap_or_default();
            format!("{prefix}-{network_id}-{device_id}-{attachment_id}")
        })
}

pub fn downstream_task_ack(task: &ControlTask) -> Option<ControlTaskAck> {
    let status = match task.status {
        ControlTaskStatus::Succeeded => ControlTaskAckStatus::Succeeded,
        ControlTaskStatus::Failed => ControlTaskAckStatus::Failed,
        _ => return None,
    };
    Some(ControlTaskAck {
        task_id: task.id.clone(),
        delivery_id: task.delivery_id.clone()?,
        action: task.action.as_str().to_string(),
        status,
        error: task.error.clone(),
        processed_at_ms: task.updated_at_ms,
    })
}

pub fn ack_task_id_from_transport_message_id(message_id: &str) -> Option<String> {
    message_id
        .trim()
        .strip_prefix(CONTROL_ACK_MESSAGE_ID_PREFIX)
        .map(str::to_string)
        .filter(|value| !value.is_empty())
}

fn transport_ack_message_id(task_id: &str) -> String {
    format!("{CONTROL_ACK_MESSAGE_ID_PREFIX}{task_id}")
}

const CONTROL_ACK_MESSAGE_ID_PREFIX: &str = "control-ack-";

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn device_disabled_envelope_targets_self_as_disable_task() {
        let accepted = normalize_downstream_control_value(
            serde_json::json!({
                "type": "device_network_disabled",
                "messageId": "msg-1",
                "payload": {
                    "networkId": "net-1",
                    "deviceId": "dev-1",
                    "attachmentId": "att-1"
                }
            }),
            Some("dev-1"),
        )
        .expect("normalize")
        .expect("accepted");

        assert_eq!(accepted.action, "disableNetwork");
        assert_eq!(accepted.delivery_id, "msg-1");
        assert!(accepted.require_ui_refresh);
    }

    #[test]
    fn device_disabled_envelope_for_peer_is_ignored() {
        let accepted = normalize_downstream_control_value(
            serde_json::json!({
                "type": "device_network_disabled",
                "messageId": "msg-1",
                "payload": {
                    "networkId": "net-1",
                    "deviceId": "dev-other"
                }
            }),
            Some("dev-1"),
        )
        .expect("normalize");

        assert!(accepted.is_none());
    }

    #[test]
    fn direct_downstream_task_still_normalizes() {
        let accepted = normalize_downstream_control_value(
            serde_json::json!({
                "action": "disableNetwork",
                "deliveryId": "delivery-1",
                "requireUiRefresh": true
            }),
            Some("dev-1"),
        )
        .expect("normalize")
        .expect("accepted");

        assert_eq!(accepted.action, "disableNetwork");
        assert_eq!(accepted.delivery_id, "delivery-1");
        assert!(accepted.require_ui_refresh);
    }
}

fn mqtt_credential_ready(credential: Option<&MqttCredential>, missing: &mut Vec<String>) -> bool {
    let Some(credential) = credential else {
        missing.push("mqtt".to_string());
        return false;
    };
    let mut ready = true;
    ready &= required_field(Some(&credential.broker_url), "mqtt.brokerUrl", missing);
    ready &= required_field(Some(&credential.client_id), "mqtt.clientId", missing);
    ready &= required_field(Some(&credential.username), "mqtt.username", missing);
    ready &= required_field(Some(&credential.password), "mqtt.password", missing);
    ready &= required_field(Some(&credential.topic_prefix), "mqtt.topicPrefix", missing);
    ready
}

fn required_field(value: Option<&str>, name: &str, missing: &mut Vec<String>) -> bool {
    if value
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .is_some()
    {
        return true;
    }
    missing.push(name.to_string());
    false
}

fn default_true() -> bool {
    true
}

fn due(now_ms: u64, last_ms: Option<u64>, interval_ms: u64) -> bool {
    last_ms
        .map(|last_ms| now_ms.saturating_sub(last_ms) >= interval_ms)
        .unwrap_or(true)
}

fn next_due_ms(now_ms: u64, last_ms: Option<u64>, interval_ms: u64) -> u64 {
    match last_ms {
        Some(last_ms) if now_ms.saturating_sub(last_ms) < interval_ms => last_ms + interval_ms,
        _ => now_ms,
    }
}
