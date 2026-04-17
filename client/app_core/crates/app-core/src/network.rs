/// 网络成员模型。
#[derive(Debug, Clone)]
pub struct NetworkMember {
    pub device_id: String,
    pub role: String,
    pub virtual_ip: Option<String>,
}

/// 网络模型。
#[derive(Debug, Clone)]
pub struct Network {
    pub network_id: String,
    pub name: String,
    pub cidr: String,
    pub members: Vec<NetworkMember>,
}

/// 控制面下发的节点候选端点。
#[derive(Debug, Clone)]
pub struct Endpoint {
    pub endpoint_type: String,
    pub address: String,
    pub updated_at: i64,
}

/// 控制面视角下的对等节点。
#[derive(Debug, Clone)]
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
#[derive(Debug, Clone)]
pub struct Route {
    pub cidr: String,
    pub via_node_id: String,
    pub metric: Option<String>,
}

/// DNS 配置。
#[derive(Debug, Clone)]
pub struct DnsConfig {
    pub servers: Vec<String>,
    pub search_domains: Vec<String>,
}

/// Relay 区域内的具体入口点。
#[derive(Debug, Clone)]
pub struct RelayEndpoint {
    pub endpoint_id: String,
    pub transport: String,
    pub address: String,
}

/// 控制面下发的 relay 区域。
#[derive(Debug, Clone)]
pub struct RelayRegion {
    pub region_id: String,
    pub region_name: String,
    pub endpoints: Vec<RelayEndpoint>,
}

/// 控制面下发的网络地图。
#[derive(Debug, Clone)]
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
    pub mtu: Option<u32>,
}

/// 控制面运行配置。
#[derive(Debug, Clone)]
pub struct ControlPlaneConfig {
    pub ws_url: String,
    pub heartbeat_seconds: u32,
}

/// Relay 配置。
#[derive(Debug, Clone)]
pub struct RelayConfig {
    pub region: String,
    pub udp_endpoint: String,
    pub tcp_endpoint: Option<String>,
}
