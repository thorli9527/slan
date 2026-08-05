use client_core::PlatformNetworkDiagnostics;
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct PersistedRelayCandidate {
    pub(crate) endpoint_id: String,
    pub(crate) transport: String,
    pub(crate) address: String,
    #[serde(default)]
    pub(crate) country_code: Option<String>,
    #[serde(default)]
    pub(crate) city_code: Option<String>,
    #[serde(default)]
    pub(crate) region_id: Option<String>,
    #[serde(default)]
    pub(crate) cluster_id: Option<String>,
    #[serde(default)]
    pub(crate) reachable_hint: bool,
    #[serde(default)]
    pub(crate) observed_rtt_ms_hint: Option<u32>,
    #[serde(default)]
    pub(crate) path_score_hint: Option<u32>,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct RelayCandidateSelection {
    pub(crate) endpoint_id: String,
    pub(crate) transport: String,
    pub(crate) address: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub(crate) country_code: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub(crate) city_code: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub(crate) region_id: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub(crate) cluster_id: Option<String>,
    pub(crate) configured_priority: u32,
    pub(crate) reachable: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub(crate) rtt_ms: Option<u32>,
    pub(crate) path_score: u32,
    pub(crate) selected: bool,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct RelayCandidateListResponse {
    pub(crate) network_id: Option<String>,
    pub(crate) refreshed: bool,
    pub(crate) candidates: Vec<RelayCandidateSelection>,
    pub(crate) best: Option<RelayCandidateSelection>,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct PathDiagnoseResponse {
    pub(crate) network_id: Option<String>,
    pub(crate) health: PathDiagnoseHealth,
    pub(crate) active_path_type: String,
    pub(crate) active_path_counts: Vec<PathDiagnosePathCount>,
    pub(crate) peer_paths: Vec<client_core::PeerPathRuntime>,
    pub(crate) relay: Option<PathDiagnoseRelay>,
    pub(crate) direct_candidates: Vec<PathDiagnoseDirectCandidate>,
    pub(crate) relay_candidates: Vec<RelayCandidateSelection>,
    pub(crate) mtu: PathDiagnoseMtu,
    pub(crate) resolver: PathDiagnoseResolver,
    pub(crate) platform: PlatformNetworkDiagnostics,
    pub(crate) export_path: Option<String>,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct PathDiagnoseHealth {
    pub(crate) status: String,
    pub(crate) reasons: Vec<PathDiagnoseHealthReason>,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct PathDiagnoseHealthReason {
    pub(crate) code: String,
    pub(crate) severity: String,
    pub(crate) message: String,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct PathDiagnosePathCount {
    pub(crate) path_type: String,
    pub(crate) count: usize,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct PathDiagnoseRelay {
    pub(crate) address: String,
    pub(crate) transport: Option<String>,
    pub(crate) active_path: Option<String>,
    pub(crate) requested_relay_session_count: u32,
    pub(crate) relay_session_count: u32,
    pub(crate) attached_peer_session_count: u32,
    pub(crate) attached_transport_count: u32,
    pub(crate) ticket_expires_at: Option<String>,
    pub(crate) ticket_expires_in_ms: Option<i64>,
    pub(crate) ticket_renew_due: bool,
    pub(crate) relay_attach_failures: u64,
    pub(crate) last_relay_attach_error: Option<String>,
    pub(crate) peers: Vec<PathDiagnoseRelayPeer>,
    pub(crate) relay_mtu: Option<u16>,
    pub(crate) max_frame_payload: Option<u16>,
    pub(crate) tun_packets_sent: u64,
    pub(crate) relay_packets_received: u64,
    pub(crate) relay_error_responses: u64,
    pub(crate) relay_config_hash_mismatches: u64,
    pub(crate) last_relay_error: Option<String>,
    pub(crate) failures: u64,
    pub(crate) unroutable_tun_packets: u64,
    pub(crate) last_unroutable_destination: Option<String>,
    pub(crate) oversized_tun_packets: u64,
    pub(crate) last_oversized_tun_packet_size: Option<u32>,
    pub(crate) started_at_ms: u64,
    pub(crate) last_tun_packet_at_ms: Option<u64>,
    pub(crate) last_relay_packet_at_ms: Option<u64>,
    pub(crate) last_relay_keepalive_at_ms: Option<u64>,
    pub(crate) updated_at_ms: u64,
    pub(crate) stale: bool,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct PathDiagnoseRelayPeer {
    pub(crate) peer_node_id: String,
    pub(crate) session_id: String,
    pub(crate) peer_virtual_ips: Vec<String>,
    pub(crate) attached: bool,
    pub(crate) attach_error: Option<String>,
    pub(crate) tun_packets_sent: u64,
    pub(crate) relay_packets_received: u64,
    pub(crate) relay_errors: u64,
    pub(crate) last_relay_error: Option<String>,
    pub(crate) last_send_path: Option<String>,
    pub(crate) path_downgrades: u64,
    pub(crate) path_upgrades: u64,
    pub(crate) last_path_change: Option<String>,
    pub(crate) replayed_frames: u64,
    pub(crate) config_hash_mismatches: u64,
    pub(crate) last_rx_seq: u64,
    pub(crate) send_failures: u64,
    pub(crate) receive_failures: u64,
    pub(crate) wintun_write_failures: u64,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct PathDiagnoseDirectCandidate {
    pub(crate) peer_node_id: String,
    pub(crate) path_type: String,
    pub(crate) endpoint_type: String,
    pub(crate) address: String,
    pub(crate) updated_at: i64,
    pub(crate) reachable: bool,
    pub(crate) rtt_ms: Option<u32>,
}

#[derive(Debug, Default, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct PathDiagnoseMtu {
    pub(crate) relay_mtu: Option<u16>,
    pub(crate) max_frame_payload: Option<u16>,
    pub(crate) policy_scope: Option<String>,
    pub(crate) policy_path_type: Option<String>,
    pub(crate) actual_mtu_checked: bool,
    pub(crate) actual_mtu_ok: Option<bool>,
    pub(crate) actual_mss_checked: bool,
    pub(crate) actual_mss_ok: Option<bool>,
    pub(crate) note: Option<String>,
}

#[derive(Debug, Default, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct PathDiagnoseResolver {
    pub(crate) expected_servers: Vec<String>,
    pub(crate) actual_servers: Vec<String>,
    pub(crate) checked: bool,
    pub(crate) ok: Option<bool>,
    pub(crate) missing_servers: Vec<String>,
    pub(crate) extra_servers: Vec<String>,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct RelayRuntimeStats {
    pub(crate) relay_address: String,
    #[serde(default)]
    pub(crate) relay_transport: Option<String>,
    #[serde(default)]
    pub(crate) active_path: Option<String>,
    #[serde(default)]
    pub(crate) requested_relay_session_count: u32,
    #[serde(default)]
    pub(crate) relay_session_count: u32,
    #[serde(default)]
    pub(crate) attached_peer_session_count: u32,
    #[serde(default)]
    pub(crate) attached_transport_count: u32,
    #[serde(default)]
    pub(crate) ticket_expires_at: Option<String>,
    #[serde(default)]
    pub(crate) ticket_expires_in_ms: Option<i64>,
    #[serde(default)]
    pub(crate) ticket_renew_due: bool,
    #[serde(default)]
    pub(crate) relay_attach_failures: u64,
    #[serde(default)]
    pub(crate) last_relay_attach_error: Option<String>,
    #[serde(default)]
    pub(crate) peers: Vec<RelayRuntimePeerStats>,
    #[serde(default)]
    pub(crate) relay_mtu: Option<u16>,
    #[serde(default)]
    pub(crate) max_frame_payload: Option<u16>,
    pub(crate) tun_packets_sent: u64,
    pub(crate) relay_packets_received: u64,
    pub(crate) relay_decode_failures: u64,
    #[serde(default)]
    pub(crate) relay_config_hash_mismatches: u64,
    #[serde(default)]
    pub(crate) relay_error_responses: u64,
    #[serde(default)]
    pub(crate) last_relay_error: Option<String>,
    #[serde(default)]
    pub(crate) relay_send_failures: u64,
    #[serde(default)]
    pub(crate) relay_receive_failures: u64,
    pub(crate) unroutable_tun_packets: u64,
    #[serde(default)]
    pub(crate) last_unroutable_destination: Option<String>,
    #[serde(default)]
    pub(crate) oversized_tun_packets: u64,
    #[serde(default)]
    pub(crate) last_oversized_tun_packet_size: Option<u32>,
    pub(crate) wintun_write_failures: u64,
    #[serde(default)]
    pub(crate) started_at_ms: u64,
    #[serde(default)]
    pub(crate) last_tun_packet_at_ms: Option<u64>,
    #[serde(default)]
    pub(crate) last_relay_packet_at_ms: Option<u64>,
    #[serde(default)]
    pub(crate) last_relay_keepalive_at_ms: Option<u64>,
    pub(crate) updated_at_ms: u64,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct RelayRuntimePeerStats {
    #[serde(default)]
    pub(crate) peer_node_id: String,
    #[serde(default)]
    pub(crate) session_id: String,
    #[serde(default)]
    pub(crate) peer_virtual_ips: Vec<String>,
    #[serde(default)]
    pub(crate) attached: bool,
    #[serde(default)]
    pub(crate) attach_error: Option<String>,
    #[serde(default)]
    pub(crate) tun_packets_sent: u64,
    #[serde(default)]
    pub(crate) relay_packets_received: u64,
    #[serde(default)]
    pub(crate) relay_errors: u64,
    #[serde(default)]
    pub(crate) last_relay_error: Option<String>,
    #[serde(default)]
    pub(crate) last_send_path: Option<String>,
    #[serde(default)]
    pub(crate) path_downgrades: u64,
    #[serde(default)]
    pub(crate) path_upgrades: u64,
    #[serde(default)]
    pub(crate) last_path_change: Option<String>,
    #[serde(default)]
    pub(crate) replayed_frames: u64,
    #[serde(default)]
    pub(crate) config_hash_mismatches: u64,
    #[serde(default)]
    pub(crate) last_rx_seq: u64,
    #[serde(default)]
    pub(crate) send_failures: u64,
    #[serde(default)]
    pub(crate) receive_failures: u64,
    #[serde(default)]
    pub(crate) wintun_write_failures: u64,
}

pub(crate) struct RelayPayloadPolicy {
    pub(crate) relay_mtu: u16,
    pub(crate) max_frame_payload: u16,
}
