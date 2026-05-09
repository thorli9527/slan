use serde::{Deserialize, Serialize};

use crate::platform::NetworkRuntimeState;

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
    #[serde(default)]
    pub prefix_len: Option<u8>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct PasswordLoginPayload {
    pub email: String,
    pub password: String,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ClientMessageNoticePayload {
    pub message_id: Option<String>,
    pub from_device_id: Option<String>,
    pub body: Option<String>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RelayPolicyNoticePayload {
    pub policy_id: Option<String>,
    pub updated_at_ms: Option<u64>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct TrafficStatsPayload {
    pub tx_bytes: u64,
    pub rx_bytes: u64,
    pub tx_bytes_per_minute: u64,
    pub rx_bytes_per_minute: u64,
    pub updated_at_ms: u64,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(tag = "type", content = "payload", rename_all = "camelCase")]
pub enum ClientCommand {
    LoginWithBrowser,
    LoginWithPassword(PasswordLoginPayload),
    ApplyAuthCallback(AuthPayload),
    EnableNetwork,
    DisableNetwork,
    SyncAssignedIp(AssignedIpPayload),
    ApplyPlatformRuntimeState(NetworkRuntimeState),
    ApplyTrafficStats(TrafficStatsPayload),
    ApplyClientMessage(ClientMessageNoticePayload),
    ApplyRelayPolicyNotice(RelayPolicyNoticePayload),
    Logout,
    Refresh,
    OpenWebConsole,
}
