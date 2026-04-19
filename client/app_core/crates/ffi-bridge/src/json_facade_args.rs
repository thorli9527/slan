use serde::Deserialize;

#[derive(Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AuthArgs {
    pub email: String,
    pub password: String,
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
    pub cidr: String,
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
    pub reason: String,
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
