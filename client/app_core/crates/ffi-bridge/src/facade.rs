use serde::{Deserialize, Serialize};
use slan_app_core::{
    ActivePath, BootstrapConfig, ConnectionState, Device, Network, NetworkAssignment,
    NetworkJoinResult, Node, RelayTicket, Session, TunnelTransport,
};

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct DataPlaneProbe {
    pub probe_id: String,
    pub sampled_at_ms: u64,
    pub active_path: ActivePath,
    pub bytes_sent: usize,
    pub reply_observed: bool,
    pub reply_bytes_received: Option<usize>,
    pub reply_sampled_at_ms: Option<u64>,
    pub reply_rtt_ms: Option<u64>,
    pub tunnel_peer_virtual_ip: Option<String>,
    pub observed_rtt_ms: Option<u32>,
    pub packet_loss_ppm: Option<u32>,
    pub path_score: Option<u32>,
    pub derp_cluster_id: Option<String>,
    pub derp_node_id: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum TunnelState {
    Disconnected,
    Configured,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct TunnelRuntimeView {
    pub state: TunnelState,
    pub transport: TunnelTransport,
    pub peer_virtual_ip: String,
    pub peer_public_key: String,
    pub selected_endpoint: Option<String>,
    pub interface_name: Option<String>,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum DataPlaneErrorCode {
    Timeout,
    Transport,
    RelayAuth,
    RelaySession,
    RelayProtocol,
    UnsupportedPath,
    Unknown,
}

impl DataPlaneErrorCode {
    pub fn as_str(self) -> &'static str {
        match self {
            Self::Timeout => "probe_timeout",
            Self::Transport => "probe_transport_error",
            Self::RelayAuth => "probe_relay_auth_error",
            Self::RelaySession => "probe_relay_session_error",
            Self::RelayProtocol => "probe_relay_protocol_error",
            Self::UnsupportedPath => "probe_unsupported_path",
            Self::Unknown => "probe_failed",
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct DataPlaneError {
    pub code: DataPlaneErrorCode,
    pub message: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ControlStatusView {
    pub status: String,
    pub ws_url: Option<String>,
    pub heartbeat_seconds: Option<u32>,
    pub session_token_present: bool,
    pub network_map_present: bool,
    pub network_id: Option<String>,
    pub node_id: Option<String>,
    pub device_id: Option<String>,
    pub peer_count: usize,
    pub connect_plan_count: usize,
    pub connect_plans: Vec<ControlConnectPlanView>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ControlConnectPlanView {
    pub peer_node_id: String,
    pub prefer_direct: bool,
    pub path_count: usize,
    pub preferred_path: Option<ControlPathOptionView>,
    pub derp_cluster_id: Option<String>,
    pub preferred_derp_node_ids: Vec<String>,
    pub relay_ticket_id: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ControlPathOptionView {
    pub path_type: String,
    pub endpoint: String,
    pub priority: i32,
}

impl DataPlaneError {
    pub fn new(code: DataPlaneErrorCode, message: impl Into<String>) -> Self {
        Self {
            code,
            message: message.into(),
        }
    }
}

impl std::fmt::Display for DataPlaneError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        f.write_str(&self.message)
    }
}

pub trait AppCoreFacade: Send + Sync {
    fn register(&self, email: String, password: String) -> Result<Session, String>;
    fn login(&self, email: String, password: String) -> Result<Session, String>;
    fn refresh_session(
        &self,
        refresh_token: String,
        device_id: Option<String>,
    ) -> Result<Session, String>;
    fn register_device(
        &self,
        name: String,
        platform: String,
        machine_id: String,
        public_key: String,
    ) -> Result<Device, String>;
    fn register_node(
        &self,
        device_id: String,
        node_id: String,
        node_public_key: String,
        capabilities: Vec<String>,
    ) -> Result<Node, String>;
    fn list_networks(&self) -> Result<Vec<Network>, String>;
    fn create_network(
        &self,
        name: String,
        cidr: Option<String>,
        expected_devices: Option<u32>,
        gateway_ip: Option<String>,
        allocation_start_ip: Option<String>,
        allocation_end_ip: Option<String>,
    ) -> Result<Network, String>;
    fn join_network(
        &self,
        network_id: String,
        device_id: String,
    ) -> Result<NetworkJoinResult, String>;
    fn join_network_by_owner_email(
        &self,
        owner_email: String,
        device_id: String,
    ) -> Result<NetworkJoinResult, String>;
    fn join_network_by_key(
        &self,
        join_key: String,
        device_id: String,
    ) -> Result<NetworkJoinResult, String>;
    fn update_attachment_remark(
        &self,
        network_id: String,
        attachment_id: String,
        remark: Option<String>,
    ) -> Result<NetworkAssignment, String>;
    fn activate_network(
        &self,
        network_id: String,
        device_id: String,
    ) -> Result<NetworkJoinResult, String>;
    fn switch_network(
        &self,
        network_id: String,
        device_id: String,
    ) -> Result<NetworkJoinResult, String> {
        self.activate_network(network_id, device_id)
    }
    fn deactivate_network(&self, network_id: String, device_id: String) -> Result<(), String>;
    fn set_device_network_state(
        &self,
        device_id: String,
        network_id: String,
        control_reachable: bool,
        network_online: bool,
        tunnel_up: bool,
        last_probe_ok: bool,
        virtual_ip: Option<String>,
        reported_at: Option<i64>,
    ) -> Result<(), String>;
    fn bootstrap(&self, node_id: String, network_id: String) -> Result<BootstrapConfig, String>;
    fn control_sync(&self) -> Result<BootstrapConfig, String>;
    fn control_status(&self) -> Result<ControlStatusView, String>;
    fn issue_relay_ticket(
        &self,
        network_id: String,
        src_node_id: String,
        dst_node_id: String,
        derp_cluster_id: Option<String>,
        preferred_derp_node_ids: Vec<String>,
        reason: String,
        relay_region_id: Option<String>,
    ) -> Result<RelayTicket, String>;
    fn connect(&self, network_id: String, peer_node_id: String) -> Result<ConnectionState, String>;
    fn probe_with_timeout(
        &self,
        packet: Vec<u8>,
        reply_timeout_ms: Option<u64>,
    ) -> Result<DataPlaneProbe, DataPlaneError>;
    fn probe(&self, packet: Vec<u8>) -> Result<DataPlaneProbe, DataPlaneError> {
        self.probe_with_timeout(packet, None)
    }
    fn send(&self, packet: Vec<u8>) -> Result<usize, DataPlaneError>;
    fn disconnect(&self) -> Result<(), String>;
}
