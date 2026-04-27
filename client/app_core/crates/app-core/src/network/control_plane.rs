use serde::{Deserialize, Serialize};

/// DNS 配置。
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct DnsConfig {
    pub servers: Vec<String>,
    pub search_domains: Vec<String>,
    #[serde(default)]
    pub wildcards: Vec<String>,
}

/// 控制面运行配置。
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ControlPlaneConfig {
    pub ws_url: String,
    #[serde(default)]
    pub session_token: Option<String>,
    pub heartbeat_seconds: u32,
}
