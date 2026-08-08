pub mod command;
pub mod dns;
pub mod error;
pub mod packet;
pub mod path;
pub mod platform;
pub mod relay_frame;
pub mod runtime;
pub mod signal_quality;
pub mod state;

pub use command::{
    AssignedIpPayload, ClientCommand, ClientMessageNoticePayload, DeviceSessionPayload,
    TrafficStatsPayload,
};
pub use dns::resolver_response_for_query;
pub use error::{ClientCoreError, ClientCoreResult};
pub use packet::{
    acl_allows_egress_packet, acl_allows_ingress_packet, icmp_echo_reply_for_request,
    ipv4_destination, ipv4_protocol, ipv4_source, ipv4_transport_checksum_valid,
    normalize_ipv4_transport_checksums, normalize_virtual_ip, relay_peer_index_for_packet,
};
pub use path::{
    compare_path_candidates, live_path_quality_is_better, live_path_quality_score,
    mark_path_ready_for_node, mark_path_ready_for_nodes, mark_peer_path_probe_success,
    normalize_relay_transport, path_candidate_score, path_should_upgrade, preferred_path_order,
    relay_path_kind_for_transport, select_active_path, selected_runtime_paths,
    sort_path_candidates, update_peer_active_path, PathCandidate, PathKind, PathPolicy,
    PathQualitySample, PathQualityTracker, PathState, PathTracker, PeerPathConfig, PeerPathRuntime,
};
pub use platform::{
    node_config_path_rank, AndroidNetworkEvent, AndroidNetworkEventType,
    AndroidSocketProtectionReason, AndroidSocketProtectionRequest, AndroidVpnConsentRequest,
    AndroidVpnPermissionState, AndroidVpnSessionConfig, NetworkRuntimeState, NodeConfig,
    PlatformAclPeer, PlatformAclPolicy, PlatformAclRule, PlatformDeviceNetworkConfig,
    PlatformDiagnosticCheck, PlatformNetwork, PlatformNetworkConfig, PlatformNetworkDiagnostics,
    PlatformResolverConfig, PlatformResolverRecord, PlatformResolverZone, RelayDataPlaneConfig,
    RelayPeerSession, RelayTicket, RouteSpec, SLAN_DNS_SERVICE_IP,
};
pub use relay_frame::relay_frame_is_replayed;
pub use runtime::ClientRuntime;
pub use signal_quality::{assess_signal_quality, SignalQualityAssessment};
pub use state::ClientViewState;
