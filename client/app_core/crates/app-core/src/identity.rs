use serde::{Deserialize, Serialize};

/// 用户会话模型。
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Session {
    pub user_id: String,
    pub access_token: String,
    pub refresh_token: Option<String>,
    pub expires_in: i64,
    pub device_id: Option<String>,
}

/// 设备模型。
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Device {
    pub device_id: String,
    pub name: String,
    pub platform: String,
    pub status: String,
    pub virtual_ip: Option<String>,
    pub public_key: Option<String>,
    pub mqtt: Option<MqttCredential>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct MqttCredential {
    pub broker_url: String,
    pub client_id: String,
    pub username: String,
    pub password: String,
    pub topic_prefix: String,
    pub expires_at: Option<i64>,
}

/// 节点模型。
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Node {
    pub node_id: String,
    pub device_id: String,
    pub node_public_key: String,
    pub network_ids: Vec<String>,
    pub capabilities: Vec<String>,
}
