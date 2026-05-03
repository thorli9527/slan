pub mod command;
pub mod error;
pub mod packet;
pub mod path;
pub mod platform;
pub mod relay_frame;
pub mod runtime;
pub mod state;

pub use command::{AssignedIpPayload, AuthPayload, ClientCommand};
pub use error::{ClientCoreError, ClientCoreResult};
pub use packet::{
    ipv4_destination, ipv4_source, normalize_virtual_ip, relay_peer_index_for_packet,
};
pub use path::{
    mark_path_ready_for_node, mark_path_ready_for_nodes, mark_peer_path_probe_success,
    normalize_relay_transport, path_should_upgrade, preferred_path_order,
    relay_path_kind_for_transport, select_active_path, selected_runtime_paths,
    update_peer_active_path, PathCandidate, PathKind, PathPolicy, PathState, PathTracker,
    PeerPathConfig, PeerPathRuntime,
};
pub use platform::{
    AndroidNetworkEvent, AndroidNetworkEventType, AndroidSocketProtectionReason,
    AndroidSocketProtectionRequest, AndroidVpnConsentRequest, AndroidVpnPermissionState,
    AndroidVpnSessionConfig, NetworkRuntimeState, PlatformDiagnosticCheck, PlatformNetwork,
    PlatformNetworkDiagnostics, RelayDataPlaneConfig, RelayPeerSession, RelayTicket, RouteSpec,
};
pub use relay_frame::relay_frame_is_replayed;
pub use runtime::ClientRuntime;
pub use state::ClientViewState;
