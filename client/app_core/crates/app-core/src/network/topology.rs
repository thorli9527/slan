use serde::{Deserialize, Serialize};

use super::{DnsConfig, RelayRegion};

/// 网络成员模型。
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct NetworkMember {
    pub member_id: Option<String>,
    pub network_id: Option<String>,
    pub attachment_id: Option<String>,
    pub device_id: String,
    pub role: String,
    pub status: Option<String>,
    pub virtual_ip: Option<String>,
    pub remark: Option<String>,
}

/// 网络模型。
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Network {
    pub network_id: String,
    pub name: String,
    pub description: Option<String>,
    pub cidr: String,
    pub default_subnet_id: Option<String>,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub subnets: Vec<Subnet>,
    pub members: Vec<NetworkMember>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Subnet {
    pub subnet_id: Option<String>,
    pub network_id: String,
    pub name: Option<String>,
    pub cidr: String,
    pub remark: Option<String>,
    pub gateway_ip: Option<String>,
    pub allocation_start_ip: Option<String>,
    pub allocation_end_ip: Option<String>,
    pub is_default: bool,
    pub status: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct NetworkJoinResult {
    pub network_id: String,
    pub device_id: String,
    pub member_id: Option<String>,
    pub attachment_id: Option<String>,
    pub virtual_ip: Option<String>,
}

/// 控制面下发的节点候选端点。
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct NetworkAssignment {
    pub attachment_id: String,
    pub network_id: String,
    pub subnet_id: String,
    pub device_id: String,
    pub device_name: String,
    pub user_id: String,
    pub user_email: String,
    pub role: String,
    pub remark: Option<String>,
    pub virtual_ip: Option<String>,
    pub status: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Endpoint {
    pub endpoint_type: String,
    pub address: String,
    pub updated_at: i64,
}

/// 控制面视角下的对等节点。
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Peer {
    pub node_id: String,
    pub device_id: String,
    pub public_key: String,
    pub status: String,
    pub relay_allowed: bool,
    pub virtual_ips: Vec<String>,
    pub endpoints: Vec<Endpoint>,
    pub allowed_routes: Vec<String>,
}

/// 控制面下发的逻辑路由。
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Route {
    pub cidr: String,
    pub via_node_id: String,
    pub metric: Option<String>,
}

/// 控制面下发的网络地图。
#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct AccessPolicy {
    pub plan_code: Option<String>,
    pub max_active_devices: Option<u32>,
    pub bandwidth_limit_mbps: Option<u32>,
    pub relay_bandwidth_limit_kbps: Option<u32>,
    pub p2p_unlimited: bool,
    pub dns_available: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct NetworkMap {
    pub self_user_id: String,
    pub self_device_id: String,
    pub self_node_id: String,
    pub network_id: String,
    pub revision: u64,
    pub heartbeat_seconds: u32,
    pub stun_servers: Vec<String>,
    pub peers: Vec<Peer>,
    pub routes: Vec<Route>,
    pub relay_regions: Vec<RelayRegion>,
    pub dns: DnsConfig,
    #[serde(default)]
    pub policy: AccessPolicy,
    pub mtu: Option<u32>,
}
