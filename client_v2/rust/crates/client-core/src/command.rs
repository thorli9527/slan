use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AuthPayload {
    pub access_token: String,
    pub refresh_token: Option<String>,
    pub user_id: String,
    pub user_label: String,
    pub device_id: Option<String>,
    pub virtual_ip: Option<String>,
    pub expires_in: Option<u64>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AssignedIpPayload {
    pub virtual_ip: String,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(tag = "type", content = "payload", rename_all = "camelCase")]
pub enum ClientCommand {
    LoginWithBrowser,
    ApplyAuthCallback(AuthPayload),
    EnableNetwork,
    DisableNetwork,
    SyncAssignedIp(AssignedIpPayload),
    Logout,
    Refresh,
    OpenWebConsole,
}
