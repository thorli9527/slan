#![allow(clippy::items_after_test_module)]

use std::{
    collections::BTreeSet,
    fs,
    net::{TcpStream, ToSocketAddrs, UdpSocket},
    path::PathBuf,
    time::{Duration, Instant},
};

use anyhow::Result;
use client_core::{assess_signal_quality, normalize_relay_transport, ClientViewState};
use client_core_platform::direct_udp::current_direct_udp_endpoint_report;
use serde::Deserialize;
use serde_json::Value;

use crate::{
    control_plane::MqttCredential,
    control_tasks::{ControlTask, ControlTaskStatus},
    relay_models::PersistedRelayCandidate,
    session_store::app_data_dir,
    session_store::PersistedSession,
    time_utils::ticket_timing_with_window,
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
    pub upstream_control_topic: Option<String>,
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
    #[serde(rename = "pathHealth")]
    PathHealth,
    #[serde(rename = "endpointReport")]
    EndpointReport,
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
    #[serde(default)]
    pub include_path_health: bool,
    #[serde(default = "default_true")]
    pub include_control_acks: bool,
}

#[derive(Debug, Clone, serde::Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ControlTransportCadence {
    pub ack_flush_interval_ms: u64,
    pub heartbeat_interval_ms: u64,
    pub runtime_state_interval_ms: u64,
    pub path_health_interval_ms: u64,
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
    #[serde(default)]
    pub last_path_health_ms: Option<u64>,
}

#[derive(Debug, Clone, serde::Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ControlTransportTickPlan {
    pub now_ms: u64,
    pub outbox: ControlTransportOutboxRequest,
    pub next_ack_flush_due_ms: u64,
    pub next_heartbeat_due_ms: u64,
    pub next_runtime_state_due_ms: u64,
    pub next_path_health_due_ms: u64,
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
        required_field(session.device_id.as_deref(), "deviceId", &mut missing);
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
        path_health_interval_ms: 600_000,
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
    let include_path_health = due(
        now_ms,
        request.last_path_health_ms,
        cadence.path_health_interval_ms,
    );
    ControlTransportTickPlan {
        now_ms,
        outbox: ControlTransportOutboxRequest {
            include_heartbeat,
            include_runtime_state,
            include_path_health,
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
        next_path_health_due_ms: next_due_ms(
            now_ms,
            request.last_path_health_ms,
            cadence.path_health_interval_ms,
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
        upstream_control_topic: topic_prefix.map(|prefix| format!("{prefix}/control/up")),
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
    include_path_health: bool,
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
        if let Some(topic) = plan.upstream_control_topic.clone() {
            messages.extend(endpoint_report_messages(
                session,
                &topic,
                reported_at_ms,
                plan.control_qos,
            ));
        }
    }
    if include_path_health {
        if let Some(topic) = plan.upstream_control_topic.clone() {
            messages.extend(relay_path_health_messages(
                session,
                &topic,
                reported_at_ms,
                plan.control_qos,
            ));
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

fn endpoint_report_messages(
    session: &PersistedSession,
    topic: &str,
    reported_at_ms: u64,
    qos: MqttQos,
) -> Vec<ControlTransportMessage> {
    let Some(endpoint_report) = load_direct_udp_endpoint_report(reported_at_ms) else {
        return Vec::new();
    };
    let updated_at = i64::try_from(reported_at_ms / 1_000).unwrap_or(i64::MAX);

    let mut endpoints = vec![serde_json::json!({
        "type": endpoint_report.endpoint_type.clone(),
        "address": endpoint_report.endpoint.clone(),
        "updatedAt": endpoint_report.updated_at().unwrap_or(updated_at)
    })];
    if !endpoint_report.lan_endpoint.is_empty()
        && endpoints[0]
            .get("address")
            .and_then(serde_json::Value::as_str)
            != Some(endpoint_report.lan_endpoint.as_str())
    {
        endpoints.push(serde_json::json!({
            "type": "lan_udp",
            "address": endpoint_report.lan_endpoint.clone(),
            "updatedAt": endpoint_report.updated_at().unwrap_or(updated_at)
        }));
    }

    endpoint_report_network_ids(session)
        .into_iter()
        .map(|network_id| {
            let message_id = format!("endpoint-report-{network_id}-{reported_at_ms}");
            ControlTransportMessage {
                id: message_id.clone(),
                topic: topic.to_string(),
                qos,
                kind: ControlTransportMessageKind::EndpointReport,
                ack_task_id: None,
                payload: serde_json::json!({
                    "type": "endpoint_report",
                    "requestId": message_id,
                    "networkId": network_id,
                    "payload": {
                        "networkId": network_id,
                        "nodeId": session.self_node_id.clone().unwrap_or_default(),
                        "natType": endpoint_report.nat_type,
                        "endpoints": endpoints
                    }
                }),
            }
        })
        .collect()
}

fn endpoint_report_network_ids(session: &PersistedSession) -> Vec<String> {
    let mut network_ids = session
        .network_ids
        .iter()
        .map(|value| value.trim())
        .filter(|value| !value.is_empty())
        .map(str::to_string)
        .collect::<BTreeSet<_>>();
    if let Some(network_id) = session
        .active_network_id
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
    {
        network_ids.insert(network_id.to_string());
    }
    network_ids.into_iter().collect()
}

pub(crate) fn pending_endpoint_report_messages(
    session: &PersistedSession,
    reported_at_ms: u64,
) -> Vec<ControlTransportMessage> {
    let plan = control_transport_plan(session);
    let Some(topic) = plan.upstream_control_topic else {
        return Vec::new();
    };
    endpoint_report_messages(session, &topic, reported_at_ms, plan.control_qos)
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
struct DirectUdpEndpointReport {
    endpoint: String,
    #[serde(default)]
    endpoint_type: String,
    #[serde(default)]
    lan_endpoint: String,
    #[serde(default)]
    nat_type: String,
    #[serde(default)]
    updated_at_ms: Option<u64>,
}

impl DirectUdpEndpointReport {
    fn normalized(mut self, fallback_updated_at_ms: u64) -> Option<Self> {
        self.endpoint = self.endpoint.trim().to_string();
        self.endpoint_type = self.endpoint_type.trim().to_string();
        self.nat_type = self.nat_type.trim().to_string();
        if self.endpoint.is_empty() {
            return None;
        }
        if self.endpoint_type.is_empty() {
            self.endpoint_type = "lan_udp".to_string();
        }
        if self.nat_type.is_empty() {
            self.nat_type = "unknown".to_string();
        }
        if self.updated_at_ms.is_none() {
            self.updated_at_ms = Some(fallback_updated_at_ms);
        }
        Some(self)
    }

    fn updated_at(&self) -> Option<i64> {
        self.updated_at_ms
            .map(|value| i64::try_from(value / 1_000).unwrap_or(i64::MAX))
    }
}

fn load_direct_udp_endpoint_report(now_ms: u64) -> Option<DirectUdpEndpointReport> {
    if let Some(report) = current_direct_udp_endpoint_report() {
        return DirectUdpEndpointReport {
            endpoint: report.endpoint,
            endpoint_type: report.endpoint_type,
            lan_endpoint: report.lan_endpoint,
            nat_type: report.nat_type,
            updated_at_ms: Some(report.updated_at_ms),
        }
        .normalized(now_ms)
        .filter(|report| {
            report
                .updated_at_ms
                .is_some_and(|updated_at_ms| now_ms.saturating_sub(updated_at_ms) <= 5 * 60 * 1_000)
        });
    }
    let payload = fs::read(direct_udp_endpoint_file_path()).ok()?;
    let report = serde_json::from_slice::<DirectUdpEndpointReport>(&payload).ok()?;
    let report = report.normalized(now_ms)?;
    if let Some(updated_at_ms) = report.updated_at_ms {
        if now_ms.saturating_sub(updated_at_ms) > 5 * 60 * 1_000 {
            return None;
        }
    }
    Some(report)
}

fn direct_udp_endpoint_file_path() -> PathBuf {
    if let Some(dir) = std::env::var_os("ProgramData") {
        return PathBuf::from(dir)
            .join("SLAN")
            .join("client-v2-direct-udp-endpoint.json");
    }
    if let Some(dir) = std::env::var_os("SLAN_STATE_DIR") {
        return PathBuf::from(dir).join("client-v2-direct-udp-endpoint.json");
    }
    app_data_dir()
        .join("SLAN")
        .join("client-v2-direct-udp-endpoint.json")
}

fn relay_path_health_messages(
    session: &PersistedSession,
    topic: &str,
    reported_at_ms: u64,
    qos: MqttQos,
) -> Vec<ControlTransportMessage> {
    let Some(network_id) = session
        .active_network_id
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
    else {
        return Vec::new();
    };
    let relay_runtime_stats = load_relay_runtime_stats(reported_at_ms);
    let mut messages = session
        .relay_candidates
        .iter()
        .take(12)
        .filter(|candidate| !candidate.endpoint_id.trim().is_empty())
        .map(|candidate| {
            let sample = probe_relay_candidate(
                candidate,
                relay_runtime_stats
                    .as_ref()
                    .filter(|stats| stats.relay_address == candidate.address),
            );
            let path_type = relay_path_type_for_transport(candidate.transport.as_str());
            let signal = assess_signal_quality(
                true,
                Some(&path_type),
                sample.observed_rtt_ms,
                sample.packet_loss_ppm,
            );
            ControlTransportMessage {
                id: format!("path-health-{}-{reported_at_ms}", candidate.endpoint_id),
                topic: topic.to_string(),
                qos,
                kind: ControlTransportMessageKind::PathHealth,
                ack_task_id: None,
                payload: serde_json::json!({
                    "type": "path_health_report",
                    "requestId": format!("path-health-{reported_at_ms}"),
                    "networkId": network_id,
                    "payload": {
                        "networkId": network_id,
                        "pathType": path_type,
                        "relayTransport": normalize_relay_transport(&candidate.transport)
                            .unwrap_or(candidate.transport.as_str()),
                        "endpoint": candidate.address,
                        "derpNodeId": candidate.endpoint_id,
                        "observedRttMs": sample.observed_rtt_ms,
                        "packetLossPpm": sample.packet_loss_ppm,
                        "pathScore": sample.path_score,
                        "signalScore": signal.score,
                        "signalQuality": signal.quality,
                        "sourceCountryCode": device_country_code(),
                        "relayCountryCode": candidate.country_code,
                        "crossCountry": cross_country(device_country_code().as_deref(), candidate.country_code.as_deref()),
                        "relayMtu": sample.relay_mtu,
                        "maxFramePayload": sample.max_frame_payload,
                        "sampledAtMs": reported_at_ms
                    }
                }),
            }
        })
        .collect::<Vec<_>>();
    if let Some(stats) = relay_runtime_stats.as_ref() {
        messages.extend(peer_runtime_path_health_messages(
            network_id,
            topic,
            reported_at_ms,
            qos,
            stats,
        ));
    }
    messages
}

fn peer_runtime_path_health_messages(
    network_id: &str,
    topic: &str,
    reported_at_ms: u64,
    qos: MqttQos,
    stats: &RelayRuntimeStats,
) -> Vec<ControlTransportMessage> {
    stats
        .peers
        .iter()
        .filter(|peer| !peer.peer_node_id.trim().is_empty())
        .map(|peer| {
            let path_type = peer
                .last_send_path
                .as_deref()
                .map(str::trim)
                .filter(|value| !value.is_empty())
                .or(stats.active_path.as_deref())
                .unwrap_or("unknown");
            let packet_loss_ppm = peer_packet_loss_ppm(peer);
            let signal = assess_signal_quality(true, Some(path_type), None, packet_loss_ppm);
            ControlTransportMessage {
                id: format!("peer-path-health-{}-{reported_at_ms}", peer.peer_node_id),
                topic: topic.to_string(),
                qos,
                kind: ControlTransportMessageKind::PathHealth,
                ack_task_id: None,
                payload: serde_json::json!({
                    "type": "path_health_report",
                    "requestId": format!("peer-path-health-{reported_at_ms}"),
                    "networkId": network_id,
                    "payload": {
                        "networkId": network_id,
                        "peerNodeId": peer.peer_node_id,
                        "pathType": path_type,
                        "activePath": path_type,
                        "relayTransport": relay_transport_from_path(path_type)
                            .or(stats.relay_transport.as_deref()),
                        "endpoint": stats.relay_address,
                        "packetLossPpm": packet_loss_ppm,
                        "pathScore": peer_path_score(peer),
                        "signalScore": signal.score,
                        "signalQuality": signal.quality,
                        "relayMtu": stats.relay_mtu,
                        "maxFramePayload": stats.max_frame_payload,
                        "ticketExpiresAt": stats.ticket_expires_at,
                        "ticketExpiresInMs": stats.ticket_expires_in_ms,
                        "ticketRenewDue": stats.ticket_renew_due,
                        "pathDowngrades": peer.path_downgrades,
                        "pathUpgrades": peer.path_upgrades,
                        "lastPathChange": peer.last_path_change,
                        "sampledAtMs": reported_at_ms
                    }
                }),
            }
        })
        .collect()
}

struct RelayPathHealthSample {
    observed_rtt_ms: Option<u32>,
    packet_loss_ppm: Option<u32>,
    path_score: Option<u32>,
    relay_mtu: Option<u32>,
    max_frame_payload: Option<u32>,
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
struct RelayRuntimeStats {
    relay_address: String,
    #[serde(default)]
    relay_transport: Option<String>,
    #[serde(default)]
    active_path: Option<String>,
    #[serde(default)]
    peers: Vec<RelayRuntimePeerStats>,
    tun_packets_sent: u64,
    relay_packets_received: u64,
    relay_decode_failures: u64,
    unroutable_tun_packets: u64,
    #[serde(default)]
    oversized_tun_packets: u64,
    wintun_write_failures: u64,
    #[serde(default)]
    relay_mtu: Option<u32>,
    #[serde(default)]
    max_frame_payload: Option<u32>,
    #[serde(default)]
    ticket_expires_at: Option<String>,
    #[serde(default)]
    ticket_expires_in_ms: Option<i64>,
    #[serde(default)]
    ticket_renew_due: bool,
    updated_at_ms: u64,
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
struct RelayRuntimePeerStats {
    #[serde(default)]
    peer_node_id: String,
    #[serde(default)]
    tun_packets_sent: u64,
    #[serde(default)]
    relay_packets_received: u64,
    #[serde(default)]
    last_send_path: Option<String>,
    #[serde(default)]
    path_downgrades: u64,
    #[serde(default)]
    path_upgrades: u64,
    #[serde(default)]
    last_path_change: Option<String>,
    #[serde(default)]
    send_failures: u64,
    #[serde(default)]
    receive_failures: u64,
    #[serde(default)]
    wintun_write_failures: u64,
}

fn probe_relay_candidate(
    candidate: &PersistedRelayCandidate,
    runtime_stats: Option<&RelayRuntimeStats>,
) -> RelayPathHealthSample {
    let transport = candidate.transport.trim().to_ascii_lowercase();
    if transport == "udp" {
        if let Some(rtt_ms) = probe_udp_ping_rtt_ms(&candidate.address) {
            let packet_loss_ppm = runtime_stats.and_then(runtime_packet_loss_ppm).unwrap_or(0);
            let penalty = packet_loss_ppm / 1000;
            return RelayPathHealthSample {
                observed_rtt_ms: Some(rtt_ms),
                packet_loss_ppm: Some(packet_loss_ppm),
                path_score: Some(
                    rtt_ms
                        .saturating_add(30)
                        .saturating_add(penalty)
                        .min(10_000),
                ),
                relay_mtu: runtime_stats.and_then(|stats| stats.relay_mtu),
                max_frame_payload: runtime_stats.and_then(|stats| stats.max_frame_payload),
            };
        }
        return RelayPathHealthSample {
            observed_rtt_ms: None,
            packet_loss_ppm: Some(1_000_000),
            path_score: Some(10_000),
            relay_mtu: runtime_stats.and_then(|stats| stats.relay_mtu),
            max_frame_payload: runtime_stats.and_then(|stats| stats.max_frame_payload),
        };
    }
    if normalize_relay_transport(&transport) == Some("derp_tcp_tls_443") {
        if let Some(rtt_ms) = probe_tcp_connect_rtt_ms(&candidate.address) {
            return RelayPathHealthSample {
                observed_rtt_ms: Some(rtt_ms),
                packet_loss_ppm: Some(0),
                path_score: Some(rtt_ms.saturating_add(100).min(10_500)),
                relay_mtu: runtime_stats.and_then(|stats| stats.relay_mtu),
                max_frame_payload: runtime_stats.and_then(|stats| stats.max_frame_payload),
            };
        }
        return RelayPathHealthSample {
            observed_rtt_ms: None,
            packet_loss_ppm: Some(1_000_000),
            path_score: Some(10_500),
            relay_mtu: runtime_stats.and_then(|stats| stats.relay_mtu),
            max_frame_payload: runtime_stats.and_then(|stats| stats.max_frame_payload),
        };
    }
    RelayPathHealthSample {
        observed_rtt_ms: None,
        packet_loss_ppm: None,
        path_score: Some(11_000),
        relay_mtu: runtime_stats.and_then(|stats| stats.relay_mtu),
        max_frame_payload: runtime_stats.and_then(|stats| stats.max_frame_payload),
    }
}

fn relay_path_type_for_transport(transport: &str) -> String {
    match normalize_relay_transport(transport).unwrap_or(transport.trim()) {
        "udp" => "relay_udp",
        "derp_tcp_tls_443" => "derp_tcp_tls_443",
        _ => "relay",
    }
    .to_string()
}

fn relay_transport_from_path(path_type: &str) -> Option<&'static str> {
    match path_type.trim() {
        "relay_udp" => Some("udp"),
        "derp_tcp_tls_443" => Some("derp_tcp_tls_443"),
        _ => None,
    }
}

fn probe_udp_ping_rtt_ms(address: &str) -> Option<u32> {
    let socket_addr = address
        .to_socket_addrs()
        .ok()
        .and_then(|mut values| values.next())?;
    let socket = UdpSocket::bind("0.0.0.0:0").ok()?;
    socket
        .set_read_timeout(Some(Duration::from_millis(750)))
        .ok()?;
    socket
        .set_write_timeout(Some(Duration::from_millis(750)))
        .ok()?;
    socket.connect(socket_addr).ok()?;
    let started = Instant::now();
    socket.send(br#"{"kind":"ping"}"#).ok()?;

    let mut response = [0_u8; 512];
    let len = socket.recv(&mut response).ok()?;
    let value = serde_json::from_slice::<Value>(&response[..len]).ok()?;
    if value.get("kind").and_then(Value::as_str) != Some("pong") {
        return None;
    }
    Some(started.elapsed().as_millis().min(u32::MAX as u128) as u32)
}

fn probe_tcp_connect_rtt_ms(address: &str) -> Option<u32> {
    let socket_addr = address
        .trim()
        .trim_start_matches("derp://")
        .trim_start_matches("derp+tcp+tls://")
        .trim_start_matches("derp_tcp_tls_443://")
        .to_socket_addrs()
        .ok()
        .and_then(|mut values| values.next())?;
    let started = Instant::now();
    TcpStream::connect_timeout(&socket_addr, Duration::from_millis(750)).ok()?;
    Some(started.elapsed().as_millis().min(u32::MAX as u128) as u32)
}

fn load_relay_runtime_stats(now_ms: u64) -> Option<RelayRuntimeStats> {
    let path = relay_stats_file_path();
    let payload = fs::read(path).ok()?;
    let mut stats = serde_json::from_slice::<RelayRuntimeStats>(&payload).ok()?;
    if now_ms.saturating_sub(stats.updated_at_ms) > 15 * 60 * 1000 {
        return None;
    }
    refresh_relay_runtime_ticket_timing(&mut stats, now_ms);
    Some(stats)
}

fn refresh_relay_runtime_ticket_timing(stats: &mut RelayRuntimeStats, now_ms: u64) {
    let timing = ticket_timing_with_window(
        now_ms,
        stats.ticket_expires_at.as_deref(),
        RELAY_TICKET_RENEW_WINDOW_MS,
    );
    stats.ticket_expires_in_ms = timing.expires_in_ms;
    stats.ticket_renew_due = timing.renew_due;
}

fn relay_stats_file_path() -> PathBuf {
    if let Some(dir) = std::env::var_os("ProgramData") {
        return PathBuf::from(dir)
            .join("SLAN")
            .join("client-v2-relay-stats.json");
    }
    if let Some(dir) = std::env::var_os("SLAN_STATE_DIR") {
        return PathBuf::from(dir).join("client-v2-relay-stats.json");
    }
    app_data_dir()
        .join("SLAN")
        .join("client-v2-relay-stats.json")
}

fn runtime_packet_loss_ppm(stats: &RelayRuntimeStats) -> Option<u32> {
    let failures = stats
        .unroutable_tun_packets
        .saturating_add(stats.relay_decode_failures)
        .saturating_add(stats.oversized_tun_packets)
        .saturating_add(stats.wintun_write_failures);
    let total = stats
        .tun_packets_sent
        .saturating_add(stats.relay_packets_received)
        .saturating_add(failures);
    if total == 0 {
        return None;
    }
    let ppm = failures.saturating_mul(1_000_000) / total;
    Some(ppm.min(1_000_000) as u32)
}

fn peer_packet_loss_ppm(peer: &RelayRuntimePeerStats) -> Option<u32> {
    let failures = peer
        .send_failures
        .saturating_add(peer.receive_failures)
        .saturating_add(peer.wintun_write_failures);
    let total = peer
        .tun_packets_sent
        .saturating_add(peer.relay_packets_received)
        .saturating_add(failures);
    if total == 0 {
        return None;
    }
    Some((failures.saturating_mul(1_000_000) / total).min(1_000_000) as u32)
}

fn peer_path_score(peer: &RelayRuntimePeerStats) -> Option<u32> {
    let loss = peer_packet_loss_ppm(peer).unwrap_or(0);
    let switch_penalty = peer
        .path_downgrades
        .saturating_add(peer.path_upgrades)
        .saturating_mul(25)
        .min(5_000) as u32;
    Some(
        (100_u32)
            .saturating_add(loss / 1_000)
            .saturating_add(switch_penalty)
            .min(10_000),
    )
}

fn device_country_code() -> Option<String> {
    std::env::var("SLAN_DEVICE_COUNTRY_CODE")
        .ok()
        .map(|value| value.trim().to_ascii_uppercase())
        .filter(|value| !value.is_empty())
}

fn cross_country(source: Option<&str>, relay: Option<&str>) -> Option<bool> {
    let source = source.map(str::trim).filter(|value| !value.is_empty())?;
    let relay = relay.map(str::trim).filter(|value| !value.is_empty())?;
    Some(!source.eq_ignore_ascii_case(relay))
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
const RELAY_TICKET_RENEW_WINDOW_MS: u64 = 5 * 60 * 1000;

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn endpoint_reports_target_every_session_network() {
        let mut session = PersistedSession::empty();
        session.active_network_id = Some("net-primary".to_string());
        session.network_ids = vec![
            "net-shared".to_string(),
            "net-primary".to_string(),
            "net-shared".to_string(),
        ];

        assert_eq!(
            endpoint_report_network_ids(&session),
            vec!["net-primary".to_string(), "net-shared".to_string()]
        );
    }

    #[test]
    fn prelogin_session_is_ready_for_downstream_device_user_login_succeeded() {
        let mut session = PersistedSession::prelogin(
            "dev-1",
            Some(MqttCredential {
                broker_url: "mqtt://127.0.0.1:1883".to_string(),
                client_id: "client-1".to_string(),
                username: "user".to_string(),
                password: "pass".to_string(),
                topic_prefix: "slan/devices/dev-1".to_string(),
                expires_at: None,
            }),
        );
        session.active_network_id = None;

        let status = control_transport_status(&session);

        assert!(status.ready);
        assert!(status.control_session_ready);
        assert_eq!(status.missing, Vec::<String>::new());
    }

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

    #[test]
    fn path_health_tick_runs_every_ten_minutes() {
        let first = control_transport_tick_plan(
            ControlTransportTickRequest {
                now_ms: Some(600_000),
                last_ack_flush_ms: Some(600_000),
                last_heartbeat_ms: Some(600_000),
                last_runtime_state_ms: Some(600_000),
                last_path_health_ms: None,
            },
            600_000,
        );
        assert!(first.outbox.include_path_health);

        let early = control_transport_tick_plan(
            ControlTransportTickRequest {
                now_ms: Some(1_000_000),
                last_ack_flush_ms: Some(1_000_000),
                last_heartbeat_ms: Some(1_000_000),
                last_runtime_state_ms: Some(1_000_000),
                last_path_health_ms: Some(600_000),
            },
            1_000_000,
        );
        assert!(!early.outbox.include_path_health);

        let due = control_transport_tick_plan(
            ControlTransportTickRequest {
                now_ms: Some(1_200_000),
                last_ack_flush_ms: Some(1_200_000),
                last_heartbeat_ms: Some(1_200_000),
                last_runtime_state_ms: Some(1_200_000),
                last_path_health_ms: Some(600_000),
            },
            1_200_000,
        );
        assert!(due.outbox.include_path_health);
    }

    #[test]
    fn path_health_outbox_uses_control_up_envelope() {
        let _lock = crate::test_env_lock();
        let state_dir = std::env::temp_dir().join(format!(
            "slan-control-transport-path-health-{}",
            std::time::SystemTime::now()
                .duration_since(std::time::UNIX_EPOCH)
                .unwrap()
                .as_millis()
        ));
        std::fs::create_dir_all(&state_dir).unwrap();
        let previous_state_dir = std::env::var_os("SLAN_STATE_DIR");
        std::env::set_var("SLAN_STATE_DIR", &state_dir);
        let mut session = PersistedSession::empty();
        session.active_network_id = Some("net-1".to_string());
        session.mqtt = Some(MqttCredential {
            broker_url: "mqtt://127.0.0.1:1883".to_string(),
            client_id: "client-1".to_string(),
            username: "user".to_string(),
            password: "pass".to_string(),
            topic_prefix: "slan/devices/dev-1".to_string(),
            expires_at: None,
        });
        session.relay_candidates = vec![PersistedRelayCandidate {
            endpoint_id: "relay-cn-tcp".to_string(),
            transport: "udp".to_string(),
            address: "127.0.0.1:9000".to_string(),
            country_code: Some("CN".to_string()),
            region_id: Some("sha".to_string()),
            cluster_id: Some("cn-a".to_string()),
            reachable_hint: false,
            observed_rtt_ms_hint: None,
            path_score_hint: None,
            selected_hint: false,
        }];

        let outbox = control_transport_outbox(
            &session,
            &ClientViewState::default(),
            Vec::new(),
            1_000,
            false,
            false,
            true,
        );

        assert_eq!(outbox.messages.len(), 1);
        assert_eq!(outbox.messages[0].topic, "slan/devices/dev-1/control/up");
        assert!(matches!(
            outbox.messages[0].kind,
            ControlTransportMessageKind::PathHealth
        ));
        assert_eq!(
            outbox.messages[0]
                .payload
                .get("type")
                .and_then(Value::as_str),
            Some("path_health_report")
        );
        assert_eq!(
            outbox.messages[0]
                .payload
                .pointer("/payload/derpNodeId")
                .and_then(Value::as_str),
            Some("relay-cn-tcp")
        );
        assert_eq!(
            outbox.messages[0]
                .payload
                .pointer("/payload/pathType")
                .and_then(Value::as_str),
            Some("relay_udp")
        );
        assert_eq!(
            outbox.messages[0]
                .payload
                .pointer("/payload/relayTransport")
                .and_then(Value::as_str),
            Some("udp")
        );
        if let Some(value) = previous_state_dir {
            std::env::set_var("SLAN_STATE_DIR", value);
        } else {
            std::env::remove_var("SLAN_STATE_DIR");
        }
    }

    #[test]
    fn peer_runtime_path_health_reports_path_switch_metadata() {
        let stats = RelayRuntimeStats {
            relay_address: "127.0.0.1:9000".to_string(),
            relay_transport: Some("udp".to_string()),
            active_path: Some("relay_udp".to_string()),
            peers: vec![RelayRuntimePeerStats {
                peer_node_id: "node-peer".to_string(),
                tun_packets_sent: 90,
                relay_packets_received: 10,
                last_send_path: Some("direct_udp".to_string()),
                path_downgrades: 2,
                path_upgrades: 1,
                last_path_change: Some("relay_udp -> direct_udp after probe success".to_string()),
                send_failures: 5,
                receive_failures: 3,
                wintun_write_failures: 2,
            }],
            tun_packets_sent: 0,
            relay_packets_received: 0,
            relay_decode_failures: 0,
            unroutable_tun_packets: 0,
            oversized_tun_packets: 0,
            wintun_write_failures: 0,
            relay_mtu: Some(1280),
            max_frame_payload: Some(1200),
            ticket_expires_at: Some("2026-05-03T10:01:30Z".to_string()),
            ticket_expires_in_ms: Some(90_000),
            ticket_renew_due: true,
            updated_at_ms: 1_000,
        };

        let messages = peer_runtime_path_health_messages(
            "net-1",
            "slan/devices/dev-1/control/up",
            2_000,
            MqttQos::QoS2,
            &stats,
        );

        assert_eq!(messages.len(), 1);
        assert_eq!(
            messages[0]
                .payload
                .pointer("/payload/peerNodeId")
                .and_then(Value::as_str),
            Some("node-peer")
        );
        assert_eq!(
            messages[0]
                .payload
                .pointer("/payload/activePath")
                .and_then(Value::as_str),
            Some("direct_udp")
        );
        assert_eq!(
            messages[0]
                .payload
                .pointer("/payload/relayTransport")
                .and_then(Value::as_str),
            Some("udp")
        );
        assert_eq!(
            messages[0]
                .payload
                .pointer("/payload/pathDowngrades")
                .and_then(Value::as_u64),
            Some(2)
        );
        assert_eq!(
            messages[0]
                .payload
                .pointer("/payload/pathUpgrades")
                .and_then(Value::as_u64),
            Some(1)
        );
        assert_eq!(
            messages[0]
                .payload
                .pointer("/payload/lastPathChange")
                .and_then(Value::as_str),
            Some("relay_udp -> direct_udp after probe success")
        );
        assert_eq!(
            messages[0]
                .payload
                .pointer("/payload/ticketExpiresInMs")
                .and_then(Value::as_i64),
            Some(90_000)
        );
        assert_eq!(
            messages[0]
                .payload
                .pointer("/payload/ticketRenewDue")
                .and_then(Value::as_bool),
            Some(true)
        );
    }

    #[test]
    fn refresh_relay_runtime_ticket_timing_overwrites_stale_derived_values() {
        let now_ms = crate::time_utils::parse_rfc3339_utc_ms("2026-05-03T10:00:00Z").unwrap();
        let mut stats = RelayRuntimeStats {
            relay_address: "127.0.0.1:9000".to_string(),
            relay_transport: Some("udp".to_string()),
            active_path: Some("relay_udp".to_string()),
            peers: Vec::new(),
            tun_packets_sent: 0,
            relay_packets_received: 0,
            relay_decode_failures: 0,
            unroutable_tun_packets: 0,
            oversized_tun_packets: 0,
            wintun_write_failures: 0,
            relay_mtu: None,
            max_frame_payload: None,
            ticket_expires_at: Some("2026-05-03T10:01:30Z".to_string()),
            ticket_expires_in_ms: Some(1),
            ticket_renew_due: false,
            updated_at_ms: now_ms,
        };

        refresh_relay_runtime_ticket_timing(&mut stats, now_ms);

        assert_eq!(stats.ticket_expires_in_ms, Some(90_000));
        assert!(stats.ticket_renew_due);
    }

    #[test]
    fn control_map_relay_candidates_are_extracted_for_persistence() {
        let candidates =
            crate::relay_candidates::extract_persisted_relay_candidates_from_control_map(
                &serde_json::json!({
                    "relayRegions": [
                        {
                            "countryCode": "CN",
                            "regionId": "sha",
                            "clusterId": "cn-a",
                            "endpoints": [
                                {
                                    "endpointId": "relay-cn-udp",
                                    "transport": "udp",
                                    "address": "127.0.0.1:9000"
                                }
                            ]
                        }
                    ]
                }),
            );

        assert_eq!(candidates.len(), 1);
        assert_eq!(candidates[0].endpoint_id, "relay-cn-udp");
        assert_eq!(candidates[0].country_code.as_deref(), Some("CN"));
        assert_eq!(candidates[0].cluster_id.as_deref(), Some("cn-a"));
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
