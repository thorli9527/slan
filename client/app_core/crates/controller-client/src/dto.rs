use serde::{Deserialize, Serialize};
use slan_app_core::{
    BootstrapConfig, ControlPlaneConfig, DerpCluster, DerpMap, DerpNodeMeta, DerpTransport,
    Device, DnsConfig, Endpoint, Network, NetworkMap, NetworkMember, Node, Peer, RelayConfig,
    RelayEndpoint, RelayRegion, RelayTicket, Route, Session,
};

use crate::api::{
    CreateNetworkRequest, LoginRequest, RegisterDeviceRequest, RegisterNodeRequest,
    RegisterRequest, RelayTicketRequest,
};

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct RegisterRequestDto {
    pub email: String,
    pub password: String,
}

impl From<RegisterRequest> for RegisterRequestDto {
    fn from(value: RegisterRequest) -> Self {
        Self {
            email: value.email,
            password: value.password,
        }
    }
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct LoginRequestDto {
    pub email: String,
    pub password: String,
}

impl From<LoginRequest> for LoginRequestDto {
    fn from(value: LoginRequest) -> Self {
        Self {
            email: value.email,
            password: value.password,
        }
    }
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AuthResponseDto {
    pub user_id: String,
    pub access_token: String,
    #[serde(default)]
    pub refresh_token: Option<String>,
    #[serde(default)]
    pub expires_in: i64,
    #[serde(default)]
    pub device_id: Option<String>,
}

impl TryFrom<AuthResponseDto> for Session {
    type Error = String;

    fn try_from(value: AuthResponseDto) -> Result<Self, Self::Error> {
        if value.user_id.trim().is_empty() || value.access_token.trim().is_empty() {
            return Err("invalid auth response".to_string());
        }
        Ok(Self {
            user_id: value.user_id,
            access_token: value.access_token,
            refresh_token: value.refresh_token,
            expires_in: value.expires_in,
            device_id: value.device_id,
        })
    }
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct RegisterDeviceRequestDto {
    pub name: String,
    pub platform: String,
    pub machine_id: String,
    pub public_key: String,
}

impl From<RegisterDeviceRequest> for RegisterDeviceRequestDto {
    fn from(value: RegisterDeviceRequest) -> Self {
        Self {
            name: value.name,
            platform: value.platform,
            machine_id: value.machine_id,
            public_key: value.public_key,
        }
    }
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct DeviceDto {
    pub device_id: String,
    pub name: String,
    pub platform: String,
    #[serde(default)]
    pub status: String,
    #[serde(default)]
    pub virtual_ip: Option<String>,
    #[serde(default)]
    pub public_key: Option<String>,
}

impl From<DeviceDto> for Device {
    fn from(value: DeviceDto) -> Self {
        Self {
            device_id: value.device_id,
            name: value.name,
            platform: value.platform,
            status: value.status,
            virtual_ip: value.virtual_ip,
            public_key: value.public_key,
        }
    }
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct RegisterNodeRequestDto {
    pub device_id: String,
    pub node_id: String,
    pub node_public_key: String,
    pub capabilities: Vec<String>,
}

impl From<RegisterNodeRequest> for RegisterNodeRequestDto {
    fn from(value: RegisterNodeRequest) -> Self {
        Self {
            device_id: value.device_id,
            node_id: value.node_id,
            node_public_key: value.node_public_key,
            capabilities: value.capabilities,
        }
    }
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct NodeDto {
    pub node_id: String,
    pub device_id: String,
    pub node_public_key: String,
    #[serde(default)]
    pub network_ids: Vec<String>,
    #[serde(default)]
    pub capabilities: Vec<String>,
}

impl From<NodeDto> for Node {
    fn from(value: NodeDto) -> Self {
        Self {
            node_id: value.node_id,
            device_id: value.device_id,
            node_public_key: value.node_public_key,
            network_ids: value.network_ids,
            capabilities: value.capabilities,
        }
    }
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct CreateNetworkRequestDto {
    pub name: String,
    pub cidr: String,
}

impl From<CreateNetworkRequest> for CreateNetworkRequestDto {
    fn from(value: CreateNetworkRequest) -> Self {
        Self {
            name: value.name,
            cidr: value.cidr,
        }
    }
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct JoinNetworkRequestDto {
    pub device_id: String,
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct NetworkDto {
    pub network_id: String,
    pub name: String,
    pub cidr: String,
    #[serde(default)]
    pub members: Vec<NetworkMemberDto>,
}

impl From<NetworkDto> for Network {
    fn from(value: NetworkDto) -> Self {
        Self {
            network_id: value.network_id,
            name: value.name,
            cidr: value.cidr,
            members: value.members.into_iter().map(Into::into).collect(),
        }
    }
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ListNetworksResponseDto {
    pub items: Vec<NetworkDto>,
}

#[allow(dead_code)]
#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct NetworkJoinResultDto {
    pub network_id: String,
    pub device_id: String,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct BootstrapRequestDto {
    pub node_id: String,
    pub network_id: String,
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct BootstrapResponseDto {
    pub device: BootstrapDeviceDto,
    #[serde(default)]
    pub networks: Vec<NetworkDto>,
    pub control_plane: ControlPlaneConfigDto,
    #[serde(default)]
    pub stun_servers: Vec<String>,
    pub relay: RelayConfigDto,
    #[serde(default)]
    pub derp_map: Option<DerpMapDto>,
    #[serde(default)]
    pub network_map: Option<NetworkMapDto>,
}

impl TryFrom<BootstrapResponseDto> for BootstrapConfig {
    type Error = String;

    fn try_from(value: BootstrapResponseDto) -> Result<Self, Self::Error> {
        Ok(Self {
            device: value.device.device.into(),
            networks: value.networks.into_iter().map(Into::into).collect(),
            control_plane: value.control_plane.into(),
            stun_servers: value.stun_servers,
            relay: value.relay.into(),
            derp_map: value.derp_map.map(TryInto::try_into).transpose()?,
            network_map: value.network_map.map(TryInto::try_into).transpose()?,
        })
    }
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct BootstrapDeviceDto {
    pub device: DeviceDto,
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ControlPlaneConfigDto {
    pub ws_url: String,
    #[serde(default = "default_heartbeat_seconds")]
    pub heartbeat_seconds: u32,
}

impl From<ControlPlaneConfigDto> for ControlPlaneConfig {
    fn from(value: ControlPlaneConfigDto) -> Self {
        Self {
            ws_url: value.ws_url,
            heartbeat_seconds: value.heartbeat_seconds,
        }
    }
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RelayConfigDto {
    pub region: String,
    pub udp_endpoint: String,
    #[serde(default)]
    pub tcp_endpoint: Option<String>,
}

impl From<RelayConfigDto> for RelayConfig {
    fn from(value: RelayConfigDto) -> Self {
        Self {
            region: value.region,
            udp_endpoint: value.udp_endpoint,
            tcp_endpoint: value.tcp_endpoint,
        }
    }
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct DnsConfigDto {
    #[serde(default)]
    pub servers: Vec<String>,
    #[serde(default)]
    pub search_domains: Vec<String>,
}

impl From<DnsConfigDto> for DnsConfig {
    fn from(value: DnsConfigDto) -> Self {
        Self {
            servers: value.servers,
            search_domains: value.search_domains,
        }
    }
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct NetworkMapDto {
    pub self_user_id: String,
    pub self_device_id: String,
    pub self_node_id: String,
    pub network_id: String,
    pub revision: u64,
    #[serde(default = "default_heartbeat_seconds")]
    pub heartbeat_seconds: u32,
    #[serde(default)]
    pub stun_servers: Vec<String>,
    #[serde(default)]
    pub peers: Vec<PeerDto>,
    #[serde(default)]
    pub routes: Vec<RouteDto>,
    #[serde(default)]
    pub relay_regions: Vec<RelayRegionDto>,
    pub dns: DnsConfigDto,
    #[serde(default)]
    pub mtu: Option<u32>,
}

impl TryFrom<NetworkMapDto> for NetworkMap {
    type Error = String;

    fn try_from(value: NetworkMapDto) -> Result<Self, Self::Error> {
        Ok(Self {
            self_user_id: value.self_user_id,
            self_device_id: value.self_device_id,
            self_node_id: value.self_node_id,
            network_id: value.network_id,
            revision: value.revision,
            heartbeat_seconds: value.heartbeat_seconds,
            stun_servers: value.stun_servers,
            peers: value.peers.into_iter().map(Into::into).collect(),
            routes: value.routes.into_iter().map(Into::into).collect(),
            relay_regions: value.relay_regions.into_iter().map(Into::into).collect(),
            dns: value.dns.into(),
            mtu: value.mtu,
        })
    }
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct NetworkMemberDto {
    pub device_id: String,
    #[serde(default)]
    pub role: String,
    #[serde(default)]
    pub virtual_ip: Option<String>,
}

impl From<NetworkMemberDto> for NetworkMember {
    fn from(value: NetworkMemberDto) -> Self {
        Self {
            device_id: value.device_id,
            role: value.role,
            virtual_ip: value.virtual_ip,
        }
    }
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct PeerDto {
    pub node_id: String,
    pub device_id: String,
    pub public_key: String,
    #[serde(default)]
    pub status: String,
    #[serde(default)]
    pub relay_allowed: bool,
    #[serde(default)]
    pub virtual_ips: Vec<String>,
    #[serde(default)]
    pub endpoints: Vec<EndpointDto>,
    #[serde(default)]
    pub allowed_routes: Vec<String>,
}

impl From<PeerDto> for Peer {
    fn from(value: PeerDto) -> Self {
        Self {
            node_id: value.node_id,
            device_id: value.device_id,
            public_key: value.public_key,
            status: value.status,
            relay_allowed: value.relay_allowed,
            virtual_ips: value.virtual_ips,
            endpoints: value.endpoints.into_iter().map(Into::into).collect(),
            allowed_routes: value.allowed_routes,
        }
    }
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct EndpointDto {
    pub endpoint_type: String,
    pub address: String,
    #[serde(default)]
    pub updated_at: i64,
}

impl From<EndpointDto> for Endpoint {
    fn from(value: EndpointDto) -> Self {
        Self {
            endpoint_type: value.endpoint_type,
            address: value.address,
            updated_at: value.updated_at,
        }
    }
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RouteDto {
    pub cidr: String,
    pub via_node_id: String,
    #[serde(default)]
    pub metric: Option<String>,
}

impl From<RouteDto> for Route {
    fn from(value: RouteDto) -> Self {
        Self {
            cidr: value.cidr,
            via_node_id: value.via_node_id,
            metric: value.metric,
        }
    }
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RelayRegionDto {
    pub region_id: String,
    pub region_name: String,
    #[serde(default)]
    pub endpoints: Vec<RelayEndpointDto>,
}

impl From<RelayRegionDto> for RelayRegion {
    fn from(value: RelayRegionDto) -> Self {
        Self {
            region_id: value.region_id,
            region_name: value.region_name,
            endpoints: value.endpoints.into_iter().map(Into::into).collect(),
        }
    }
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RelayEndpointDto {
    pub endpoint_id: String,
    pub transport: String,
    pub address: String,
}

impl From<RelayEndpointDto> for RelayEndpoint {
    fn from(value: RelayEndpointDto) -> Self {
        Self {
            endpoint_id: value.endpoint_id,
            transport: value.transport,
            address: value.address,
        }
    }
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct DerpMapDto {
    #[serde(default = "default_probe_interval_seconds")]
    pub probe_interval_seconds: u32,
    #[serde(default)]
    pub clusters: Vec<DerpClusterDto>,
}

impl TryFrom<DerpMapDto> for DerpMap {
    type Error = String;

    fn try_from(value: DerpMapDto) -> Result<Self, Self::Error> {
        Ok(Self {
            probe_interval_seconds: value.probe_interval_seconds,
            clusters: value
                .clusters
                .into_iter()
                .map(TryInto::try_into)
                .collect::<Result<Vec<_>, _>>()?,
        })
    }
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct DerpClusterDto {
    pub cluster_id: String,
    pub region_id: String,
    pub region_name: String,
    #[serde(default = "default_recommended_fanout")]
    pub recommended_fanout: u8,
    #[serde(default)]
    pub nodes: Vec<DerpNodeDto>,
}

impl TryFrom<DerpClusterDto> for DerpCluster {
    type Error = String;

    fn try_from(value: DerpClusterDto) -> Result<Self, Self::Error> {
        let cluster_id = value.cluster_id;
        let region_id = value.region_id;
        let region_name = value.region_name;
        let nodes = value
            .nodes
            .into_iter()
            .map(|node| node.into_meta(cluster_id.clone(), region_id.clone()))
            .collect();
        Ok(Self {
            cluster_id,
            region_id,
            region_name,
            recommended_fanout: value.recommended_fanout,
            nodes,
        })
    }
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct DerpNodeDto {
    pub node_id: String,
    pub host: String,
    pub port: u16,
    pub transport: String,
    #[serde(default)]
    pub priority: u32,
    #[serde(default)]
    pub tags: Vec<String>,
}

impl DerpNodeDto {
    fn into_meta(self, cluster_id: String, region_id: String) -> DerpNodeMeta {
        DerpNodeMeta {
            cluster_id,
            region_id,
            node_id: self.node_id,
            host: self.host,
            port: self.port,
            transport: parse_derp_transport(&self.transport),
            priority: self.priority,
            tags: self.tags,
        }
    }
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct RelayTicketRequestDto {
    pub network_id: String,
    pub src_node_id: String,
    pub dst_node_id: String,
    pub derp_cluster_id: Option<String>,
    pub preferred_derp_node_ids: Vec<String>,
    pub reason: String,
}

impl From<RelayTicketRequest> for RelayTicketRequestDto {
    fn from(value: RelayTicketRequest) -> Self {
        Self {
            network_id: value.network_id,
            src_node_id: value.src_node_id,
            dst_node_id: value.dst_node_id,
            derp_cluster_id: value.derp_cluster_id,
            preferred_derp_node_ids: value.preferred_derp_node_ids,
            reason: value.reason,
        }
    }
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RelayTicketDto {
    pub ticket_id: String,
    pub network_id: String,
    pub session_id: String,
    pub src_node_id: String,
    pub dst_node_id: String,
    #[serde(default)]
    pub derp_cluster_id: Option<String>,
    #[serde(default)]
    pub allowed_derp_node_ids: Vec<String>,
    pub relay_url: String,
    pub expires_at: String,
    #[serde(default)]
    pub session_key: Option<String>,
    pub signature: String,
}

impl From<RelayTicketDto> for RelayTicket {
    fn from(value: RelayTicketDto) -> Self {
        Self {
            ticket_id: value.ticket_id,
            network_id: value.network_id,
            session_id: value.session_id,
            src_node_id: value.src_node_id,
            dst_node_id: value.dst_node_id,
            derp_cluster_id: value.derp_cluster_id,
            allowed_derp_node_ids: value.allowed_derp_node_ids,
            relay_url: value.relay_url,
            expires_at: value.expires_at,
            session_key: value.session_key,
            signature: value.signature,
        }
    }
}

fn parse_derp_transport(raw: &str) -> DerpTransport {
    match raw {
        "tcp" => DerpTransport::Tcp,
        "quic" => DerpTransport::Quic,
        _ => DerpTransport::Udp,
    }
}

fn default_heartbeat_seconds() -> u32 {
    15
}

fn default_probe_interval_seconds() -> u32 {
    5
}

fn default_recommended_fanout() -> u8 {
    2
}
