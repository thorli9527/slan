use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ClientViewState {
    pub signed_in: bool,
    pub user_label: Option<String>,
    pub device_id: Option<String>,
    pub auth_callback_id: Option<String>,
    pub virtual_ip: Option<String>,
    pub network_enabled: bool,
    pub syncing: bool,
    pub sync_reason: Option<String>,
    pub switch_enabled: bool,
    pub notice: Option<String>,
    pub error: Option<String>,
    #[serde(default)]
    pub last_client_message_id: Option<String>,
    #[serde(default)]
    pub last_client_message_from_device_id: Option<String>,
    #[serde(default)]
    pub last_client_message_body: Option<String>,
    #[serde(default)]
    pub last_relay_policy_id: Option<String>,
    #[serde(default)]
    pub last_relay_policy_updated_at_ms: Option<u64>,
    #[serde(default)]
    pub traffic_tx_bytes: Option<u64>,
    #[serde(default)]
    pub traffic_rx_bytes: Option<u64>,
    #[serde(default)]
    pub traffic_tx_bytes_per_minute: Option<u64>,
    #[serde(default)]
    pub traffic_rx_bytes_per_minute: Option<u64>,
    #[serde(default)]
    pub traffic_updated_at_ms: Option<u64>,
}

impl Default for ClientViewState {
    fn default() -> Self {
        Self {
            signed_in: false,
            user_label: None,
            device_id: None,
            auth_callback_id: None,
            virtual_ip: None,
            network_enabled: false,
            syncing: false,
            sync_reason: None,
            switch_enabled: true,
            notice: None,
            error: None,
            last_client_message_id: None,
            last_client_message_from_device_id: None,
            last_client_message_body: None,
            last_relay_policy_id: None,
            last_relay_policy_updated_at_ms: None,
            traffic_tx_bytes: None,
            traffic_rx_bytes: None,
            traffic_tx_bytes_per_minute: None,
            traffic_rx_bytes_per_minute: None,
            traffic_updated_at_ms: None,
        }
    }
}
