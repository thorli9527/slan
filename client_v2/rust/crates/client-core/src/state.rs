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
        }
    }
}
