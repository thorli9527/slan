use serde::{Deserialize, Serialize};

/// DERP 传输类型。
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum DerpTransport {
    Udp,
    Tcp,
    Quic,
}

/// 单个 DERP 节点元数据。
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct DerpNodeMeta {
    pub cluster_id: String,
    pub region_id: String,
    pub country_code: Option<String>,
    pub country_name: Option<String>,
    pub city_code: Option<String>,
    pub city_name: Option<String>,
    pub node_id: String,
    pub host: String,
    pub port: u16,
    pub transport: DerpTransport,
    pub priority: u32,
    pub tags: Vec<String>,
}

/// 一个 DERP 集群。
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct DerpCluster {
    pub cluster_id: String,
    pub cluster_name: Option<String>,
    pub region_id: String,
    pub region_name: String,
    pub country_code: Option<String>,
    pub country_name: Option<String>,
    pub city_code: Option<String>,
    pub city_name: Option<String>,
    pub recommended_fanout: u8,
    pub nodes: Vec<DerpNodeMeta>,
}

/// 控制面下发的 DERP 视图。
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct DerpMap {
    pub probe_interval_seconds: u32,
    pub clusters: Vec<DerpCluster>,
}
