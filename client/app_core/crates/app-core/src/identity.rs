/// 用户会话模型。
#[derive(Debug, Clone)]
pub struct Session {
    pub user_id: String,
    pub access_token: String,
    pub refresh_token: Option<String>,
    pub expires_in: i64,
    pub device_id: Option<String>,
}

/// 设备模型。
#[derive(Debug, Clone)]
pub struct Device {
    pub device_id: String,
    pub name: String,
    pub platform: String,
    pub status: String,
    pub virtual_ip: Option<String>,
    pub public_key: Option<String>,
}

/// 节点模型。
#[derive(Debug, Clone)]
pub struct Node {
    pub node_id: String,
    pub device_id: String,
    pub node_public_key: String,
    pub network_ids: Vec<String>,
    pub capabilities: Vec<String>,
}
