use serde::Deserialize;
use slan_app_core::Session;

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AuthArgs {
    pub email: String,
    pub password: String,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RefreshSessionArgs {
    pub refresh_token: String,
    #[serde(default)]
    pub device_id: Option<String>,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RestoreSessionArgs {
    #[serde(flatten)]
    pub session: Session,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RegisterDeviceArgs {
    pub name: String,
    pub platform: String,
    pub machine_id: String,
    pub public_key: String,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RegisterNodeArgs {
    pub device_id: String,
    pub node_id: String,
    pub node_public_key: String,
    #[serde(default)]
    pub capabilities: Vec<String>,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct CreateNetworkArgs {
    pub name: String,
    #[serde(default)]
    pub cidr: Option<String>,
    #[serde(default)]
    pub allocation_start_ip: Option<String>,
    #[serde(default)]
    pub allocation_end_ip: Option<String>,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct JoinNetworkArgs {
    pub network_id: String,
    pub device_id: String,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct DeviceNetworkStateArgs {
    pub device_id: String,
    pub network_id: String,
    pub control_reachable: bool,
    pub network_online: bool,
    pub tunnel_up: bool,
    pub last_probe_ok: bool,
    #[serde(default)]
    pub virtual_ip: Option<String>,
    #[serde(default)]
    pub reported_at: Option<i64>,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct LocalNetworkArgs {
    #[serde(default)]
    pub network_id: Option<String>,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct JoinNetworkByKeyArgs {
    pub join_key: String,
    pub device_id: String,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct UpdateAttachmentRemarkArgs {
    pub network_id: String,
    pub attachment_id: String,
    #[serde(default)]
    pub remark: Option<String>,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct BootstrapArgs {
    pub node_id: String,
    pub network_id: String,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RelayTicketArgs {
    pub network_id: String,
    pub src_node_id: String,
    pub dst_node_id: String,
    #[serde(default)]
    pub derp_cluster_id: Option<String>,
    #[serde(default)]
    pub preferred_derp_node_ids: Vec<String>,
    pub reason: String,
    #[serde(default)]
    pub relay_region_id: Option<String>,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ConnectArgs {
    pub network_id: String,
    pub peer_node_id: String,
}

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SendArgs {
    pub payload: String,
    #[serde(default)]
    pub probe_timeout_ms: Option<u64>,
}
