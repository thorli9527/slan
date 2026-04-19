use serde::{Deserialize, Serialize};

/// Relay 区域内的具体入口点。
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RelayEndpoint {
    pub endpoint_id: String,
    pub transport: String,
    pub address: String,
}

/// 控制面下发的 relay 区域。
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RelayRegion {
    pub region_id: String,
    pub region_name: String,
    pub country_code: Option<String>,
    pub country_name: Option<String>,
    pub city_code: Option<String>,
    pub city_name: Option<String>,
    pub cluster_id: Option<String>,
    pub cluster_name: Option<String>,
    pub endpoints: Vec<RelayEndpoint>,
}

/// Relay 配置。
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RelayConfig {
    pub default_cluster_id: String,
    pub countries: Vec<RelayCountry>,
}

/// Relay 国家层。
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RelayCountry {
    pub country_code: String,
    pub country_name: String,
    pub cities: Vec<RelayCity>,
}

/// Relay 城市层。
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RelayCity {
    pub city_code: String,
    pub city_name: String,
    pub clusters: Vec<RelayCluster>,
}

/// Relay 集群层。
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RelayCluster {
    pub cluster_id: String,
    pub cluster_name: String,
    pub nodes: Vec<RelayNode>,
}

/// Relay 节点层。
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RelayNode {
    pub node_id: String,
    pub transport: String,
    pub address: String,
    pub priority: u32,
    pub tags: Vec<String>,
}
