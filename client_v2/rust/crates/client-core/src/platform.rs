use anyhow::Result;
use serde::{Deserialize, Serialize};

use crate::{PathPolicy, PeerPathConfig, PeerPathRuntime};

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RouteSpec {
    pub destination: String,
    pub gateway: Option<String>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct NetworkRuntimeState {
    pub adapter_present: bool,
    pub network_enabled: bool,
    pub virtual_ip: Option<String>,
    #[serde(default)]
    pub active_path: Option<crate::PathKind>,
    #[serde(default)]
    pub peer_paths: Vec<PeerPathRuntime>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct PlatformNetworkDiagnostics {
    #[serde(default)]
    pub platform: String,
    #[serde(default)]
    pub adapter_present: bool,
    #[serde(default)]
    pub adapter_name: Option<String>,
    #[serde(default)]
    pub admin_status: Option<String>,
    #[serde(default)]
    pub interface_index: Option<u32>,
    #[serde(default)]
    pub virtual_ip: Option<String>,
    #[serde(default)]
    pub mtu: Option<u32>,
    #[serde(default)]
    pub mss: Option<u32>,
    #[serde(default)]
    pub dns_servers: Vec<String>,
    #[serde(default)]
    pub routes: Vec<String>,
    #[serde(default)]
    pub checks: Vec<PlatformDiagnosticCheck>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct PlatformDiagnosticCheck {
    pub name: String,
    pub ok: bool,
    pub message: Option<String>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum AndroidVpnPermissionState {
    Granted,
    NeedsUserConsent,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AndroidVpnConsentRequest {
    pub callback_id: String,
    pub message: Option<String>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AndroidVpnSessionConfig {
    pub session_name: String,
    pub virtual_ip: String,
    pub prefix_len: u8,
    pub dns_servers: Vec<String>,
    pub routes: Vec<RouteSpec>,
    pub mtu: Option<u16>,
    pub relay_endpoint_id: Option<String>,
    pub relay_transport: Option<String>,
    pub relay_address: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub relay_data_plane: Option<RelayDataPlaneConfig>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RelayDataPlaneConfig {
    pub enabled: bool,
    pub transport: String,
    pub relay_address: String,
    pub local_node_id: String,
    pub network_id: String,
    #[serde(default)]
    pub path_policy: PathPolicy,
    #[serde(default)]
    pub peer_paths: Vec<PeerPathConfig>,
    #[serde(default)]
    pub relay_mtu: Option<u16>,
    #[serde(default)]
    pub max_frame_payload: Option<u16>,
    pub sessions: Vec<RelayPeerSession>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RelayPeerSession {
    pub session_id: String,
    pub peer_node_id: String,
    #[serde(default)]
    pub peer_virtual_ips: Vec<String>,
    pub ticket: RelayTicket,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RelayTicket {
    pub ticket_id: String,
    pub network_id: String,
    pub session_id: String,
    pub src_node_id: String,
    pub dst_node_id: String,
    #[serde(default)]
    pub derp_cluster_id: Option<String>,
    #[serde(default)]
    pub country_code: Option<String>,
    #[serde(default)]
    pub city_code: Option<String>,
    #[serde(default)]
    pub allowed_derp_node_ids: Vec<String>,
    pub relay_url: String,
    pub expires_at: String,
    #[serde(default)]
    pub session_key: String,
    pub signature: String,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AndroidSocketProtectionRequest {
    pub socket_fd: i32,
    pub reason: AndroidSocketProtectionReason,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum AndroidSocketProtectionReason {
    ControlPlane,
    Mqtt,
    RelayProbe,
    RelayTransport,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AndroidNetworkEvent {
    pub event_type: AndroidNetworkEventType,
    pub message: Option<String>,
    pub runtime_state: Option<NetworkRuntimeState>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum AndroidNetworkEventType {
    PermissionRequired,
    PermissionGranted,
    VpnStarted,
    VpnStopped,
    VpnRevoked,
    ConnectivityChanged,
    RelayChanged,
    Error,
}

pub trait PlatformNetwork {
    fn install_adapter(&self) -> Result<()>;
    fn configure_ip(&self, virtual_ip: &str, prefix_len: u8) -> Result<()>;
    fn configure_routes(&self, routes: &[RouteSpec]) -> Result<()>;
    fn configure_dns(&self, dns_servers: &[String]) -> Result<()>;
    fn configure_relay(&self, _config: Option<&RelayDataPlaneConfig>) -> Result<()> {
        Ok(())
    }
    fn diagnostics(&self) -> Result<PlatformNetworkDiagnostics> {
        Ok(PlatformNetworkDiagnostics::default())
    }
    fn disable_network(&self) -> Result<()>;
    fn read_runtime_state(&self) -> Result<NetworkRuntimeState>;
}
