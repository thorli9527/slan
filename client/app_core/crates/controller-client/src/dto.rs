use serde::{Deserialize, Serialize};
use slan_app_core::{
    AccessPolicy, BootstrapConfig, ControlPlaneConfig, DerpCluster, DerpMap, DerpNodeMeta,
    DerpTransport, Device, DnsConfig, Endpoint, MqttCredential, Network, NetworkAssignment,
    NetworkJoinResult, NetworkMap, NetworkMember, Node, Peer, RelayCity, RelayCluster, RelayConfig,
    RelayCountry, RelayEndpoint, RelayNode, RelayRegion, RelayTicket, Route, Session, Subnet,
};

use crate::api::{
    CreateNetworkRequest, JoinNetworkByKeyRequest, LoginRequest, RefreshTokenRequest,
    RegisterDeviceRequest, RegisterNodeRequest, RegisterRequest, RelayTicketRequest,
    UpdateAttachmentRemarkRequest, UpdateNetworkDNSRequest,
};

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ErrorResponseDto {
    pub code: String,
    pub message: String,
}

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
    #[serde(skip_serializing_if = "Option::is_none")]
    pub device_id: Option<String>,
}

impl From<LoginRequest> for LoginRequestDto {
    fn from(value: LoginRequest) -> Self {
        Self {
            email: value.email,
            password: value.password,
            device_id: value.device_id.filter(|value| !value.trim().is_empty()),
        }
    }
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct RefreshTokenRequestDto {
    pub refresh_token: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub device_id: Option<String>,
}

impl From<RefreshTokenRequest> for RefreshTokenRequestDto {
    fn from(value: RefreshTokenRequest) -> Self {
        Self {
            refresh_token: value.refresh_token,
            device_id: value.device_id.filter(|value| !value.trim().is_empty()),
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
    #[serde(default)]
    pub owner_email: Option<String>,
    pub platform: String,
    #[serde(default)]
    pub machine_id: Option<String>,
    #[serde(default)]
    pub status: String,
    #[serde(default)]
    pub current_virtual_ip: Option<String>,
    #[serde(default)]
    pub link_status: Option<String>,
    #[serde(default)]
    pub connectivity_protocol: Option<String>,
    #[serde(default)]
    pub joined_at: Option<i64>,
    #[serde(default)]
    pub membership_status: Option<String>,
    #[serde(default)]
    pub network_role: Option<String>,
    #[serde(default)]
    pub created_at: Option<i64>,
    #[serde(default)]
    pub virtual_ip: Option<String>,
    #[serde(default)]
    pub public_key: Option<String>,
    #[serde(default)]
    pub network_ids: Vec<String>,
    #[serde(default)]
    pub mqtt: Option<MqttCredentialDto>,
    #[serde(default)]
    pub network_state: Option<DeviceNetworkStateDto>,
}

impl From<DeviceDto> for Device {
    fn from(value: DeviceDto) -> Self {
        Self {
            device_id: value.device_id,
            name: value.name,
            platform: value.platform,
            status: value.status,
            virtual_ip: value.current_virtual_ip.or(value.virtual_ip),
            public_key: value.public_key,
            mqtt: value.mqtt.map(Into::into),
        }
    }
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ListDevicesResponseDto {
    #[serde(default)]
    pub items: Vec<DeviceDto>,
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct DeviceNetworkStateDto {
    pub device_id: String,
    pub network_id: String,
    pub control_reachable: bool,
    pub network_online: bool,
    pub tunnel_up: bool,
    pub last_probe_ok: bool,
    #[serde(default)]
    pub virtual_ip: Option<String>,
    pub last_seen_at: i64,
    pub updated_at: i64,
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct MqttCredentialDto {
    pub broker_url: String,
    pub client_id: String,
    pub username: String,
    pub password: String,
    pub topic_prefix: String,
    #[serde(default)]
    pub expires_at: Option<i64>,
}

impl From<MqttCredentialDto> for MqttCredential {
    fn from(value: MqttCredentialDto) -> Self {
        Self {
            broker_url: value.broker_url,
            client_id: value.client_id,
            username: value.username,
            password: value.password,
            topic_prefix: value.topic_prefix,
            expires_at: value.expires_at,
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
    #[serde(skip_serializing_if = "Option::is_none")]
    pub description: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub cidr: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub allocation_start_ip: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub allocation_end_ip: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub bind_device_id: Option<String>,
}

impl From<CreateNetworkRequest> for CreateNetworkRequestDto {
    fn from(value: CreateNetworkRequest) -> Self {
        Self {
            name: value.name,
            description: value.description.filter(|value| !value.trim().is_empty()),
            cidr: value.cidr.filter(|value| !value.trim().is_empty()),
            allocation_start_ip: value
                .allocation_start_ip
                .filter(|value| !value.trim().is_empty()),
            allocation_end_ip: value
                .allocation_end_ip
                .filter(|value| !value.trim().is_empty()),
            bind_device_id: value
                .bind_device_id
                .filter(|value| !value.trim().is_empty()),
        }
    }
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct UpdateNetworkDNSRequestDto {
    pub servers: Vec<String>,
    pub search_domains: Vec<String>,
    pub wildcards: Vec<String>,
}

impl From<UpdateNetworkDNSRequest> for UpdateNetworkDNSRequestDto {
    fn from(value: UpdateNetworkDNSRequest) -> Self {
        Self {
            servers: value.servers,
            search_domains: value.search_domains,
            wildcards: value.wildcards,
        }
    }
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct JoinNetworkRequestDto {
    pub device_id: String,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct SwitchNetworkRequestDto {
    pub device_id: String,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct DeactivateNetworkRequestDto {
    pub device_id: String,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct DeviceNetworkStateRequestDto {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub device_id: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub network_id: Option<String>,
    pub control_reachable: bool,
    pub network_online: bool,
    pub tunnel_up: bool,
    pub last_probe_ok: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub virtual_ip: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub reported_at: Option<i64>,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct JoinNetworkByKeyRequestDto {
    pub join_key: String,
    pub device_id: String,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct UpdateAttachmentRemarkRequestDto {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub remark: Option<String>,
}

impl From<UpdateAttachmentRemarkRequest> for UpdateAttachmentRemarkRequestDto {
    fn from(value: UpdateAttachmentRemarkRequest) -> Self {
        Self {
            remark: value.remark.filter(|value| !value.trim().is_empty()),
        }
    }
}

impl From<JoinNetworkByKeyRequest> for JoinNetworkByKeyRequestDto {
    fn from(value: JoinNetworkByKeyRequest) -> Self {
        Self {
            join_key: value.join_key,
            device_id: value.device_id,
        }
    }
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct NetworkDto {
    pub network_id: String,
    pub name: String,
    #[serde(default)]
    pub description: Option<String>,
    #[serde(default)]
    pub default_subnet_id: Option<String>,
    #[serde(default)]
    pub cidr: String,
    #[serde(default)]
    pub default_subnet_cidr: Option<String>,
    #[serde(default)]
    pub join_key_configured: Option<bool>,
    #[serde(default)]
    pub subnets: Vec<SubnetDto>,
    #[serde(default)]
    pub members: Vec<NetworkMemberDto>,
}

impl From<NetworkDto> for Network {
    fn from(value: NetworkDto) -> Self {
        let cidr = if !value.cidr.is_empty() {
            value.cidr.clone()
        } else if let Some(default_subnet_cidr) = value.default_subnet_cidr.clone() {
            default_subnet_cidr
        } else {
            value
                .subnets
                .iter()
                .find(|subnet| subnet.is_default)
                .map(|subnet| subnet.cidr.clone())
                .or_else(|| value.subnets.first().map(|subnet| subnet.cidr.clone()))
                .unwrap_or_default()
        };
        Self {
            network_id: value.network_id,
            name: value.name,
            description: value.description,
            cidr,
            default_subnet_id: value.default_subnet_id,
            subnets: value.subnets.into_iter().map(Into::into).collect(),
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
    pub member: NetworkMemberDto,
    pub attachment: SubnetAttachmentDto,
}

impl From<NetworkJoinResultDto> for NetworkJoinResult {
    fn from(value: NetworkJoinResultDto) -> Self {
        Self {
            network_id: value.attachment.network_id,
            device_id: value.attachment.device_id,
            member_id: value.member.member_id,
            attachment_id: Some(value.attachment.attachment_id),
            virtual_ip: value.attachment.virtual_ip,
        }
    }
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct NetworkAssignmentDto {
    pub attachment_id: String,
    pub network_id: String,
    pub subnet_id: String,
    pub device_id: String,
    pub device_name: String,
    pub user_id: String,
    pub user_email: String,
    pub role: String,
    #[serde(default)]
    pub remark: Option<String>,
    #[serde(default)]
    pub virtual_ip: Option<String>,
    #[serde(default)]
    pub status: Option<String>,
}

impl From<NetworkAssignmentDto> for NetworkAssignment {
    fn from(value: NetworkAssignmentDto) -> Self {
        Self {
            attachment_id: value.attachment_id,
            network_id: value.network_id,
            subnet_id: value.subnet_id,
            device_id: value.device_id,
            device_name: value.device_name,
            user_id: value.user_id,
            user_email: value.user_email,
            role: value.role,
            remark: value.remark,
            virtual_ip: value.virtual_ip,
            status: value.status,
        }
    }
}

#[allow(dead_code)]
#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SubnetAttachmentDto {
    pub attachment_id: String,
    pub network_id: String,
    pub subnet_id: String,
    pub device_id: String,
    #[serde(default)]
    pub virtual_ip: Option<String>,
    #[serde(default)]
    pub remark: Option<String>,
    #[serde(default)]
    pub status: String,
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct SubnetDto {
    #[serde(default)]
    pub subnet_id: Option<String>,
    #[serde(default)]
    pub network_id: Option<String>,
    #[serde(default)]
    pub name: Option<String>,
    pub cidr: String,
    #[serde(default)]
    pub remark: Option<String>,
    #[serde(default)]
    pub gateway_ip: Option<String>,
    #[serde(default)]
    pub allocation_start_ip: Option<String>,
    #[serde(default)]
    pub allocation_end_ip: Option<String>,
    #[serde(default)]
    pub is_default: bool,
    #[serde(default)]
    pub status: Option<String>,
}

impl From<SubnetDto> for Subnet {
    fn from(value: SubnetDto) -> Self {
        Self {
            subnet_id: value.subnet_id,
            network_id: value.network_id.unwrap_or_default(),
            name: value.name,
            cidr: value.cidr,
            remark: value.remark,
            gateway_ip: value.gateway_ip,
            allocation_start_ip: value.allocation_start_ip,
            allocation_end_ip: value.allocation_end_ip,
            is_default: value.is_default,
            status: value.status,
        }
    }
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
    #[serde(default)]
    pub control_session_id: Option<String>,
    #[serde(default)]
    pub session_token: Option<String>,
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

    fn try_from(mut value: BootstrapResponseDto) -> Result<Self, Self::Error> {
        let device_attachments = value.device.attachments.clone();
        let mut device: Device = value.device.device.into();
        if device.virtual_ip.as_deref().unwrap_or("").trim().is_empty() {
            device.virtual_ip = device_attachments
                .iter()
                .filter(|attachment| !is_disabled_attachment_status(&attachment.status))
                .find_map(|attachment| attachment.virtual_ip.clone())
                .filter(|value| !value.trim().is_empty());
        }
        for network in &mut value.networks {
            for member in &mut network.members {
                if member.device_id != device.device_id {
                    continue;
                }
                let attachment = device_attachments.iter().find(|attachment| {
                    attachment.device_id == member.device_id
                        && attachment.network_id == network.network_id
                        && member
                            .attachment_id
                            .as_deref()
                            .map(|id| id == attachment.attachment_id)
                            .unwrap_or(true)
                });
                if let Some(attachment) = attachment {
                    if !attachment.status.trim().is_empty() {
                        member.status = Some(attachment.status.clone());
                    }
                    member.attachment_id = Some(attachment.attachment_id.clone());
                    member.virtual_ip = attachment.virtual_ip.clone();
                }
            }
        }
        let mut control_plane: ControlPlaneConfig = value.control_plane.into();
        if control_plane.session_token.is_none() {
            control_plane.session_token = value.session_token;
        }
        Ok(Self {
            device,
            networks: value.networks.into_iter().map(Into::into).collect(),
            control_plane,
            stun_servers: value.stun_servers,
            relay: value.relay.into(),
            derp_map: value.derp_map.map(TryInto::try_into).transpose()?,
            network_map: value.network_map.map(TryInto::try_into).transpose()?,
        })
    }
}

fn is_disabled_attachment_status(status: &str) -> bool {
    matches!(
        status.trim().to_ascii_lowercase().as_str(),
        "disabled" | "suspended" | "rejected"
    )
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct BootstrapDeviceDto {
    pub device: DeviceDto,
    #[serde(default)]
    pub attachments: Vec<SubnetAttachmentDto>,
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ControlPlaneConfigDto {
    pub ws_url: String,
    #[serde(default)]
    pub session_token: Option<String>,
    #[serde(default = "default_heartbeat_seconds")]
    pub heartbeat_seconds: u32,
}

impl From<ControlPlaneConfigDto> for ControlPlaneConfig {
    fn from(value: ControlPlaneConfigDto) -> Self {
        Self {
            ws_url: value.ws_url,
            session_token: value.session_token,
            heartbeat_seconds: value.heartbeat_seconds,
        }
    }
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RelayConfigDto {
    pub default_cluster_id: String,
    #[serde(default)]
    pub countries: Vec<RelayCountryDto>,
}

impl From<RelayConfigDto> for RelayConfig {
    fn from(value: RelayConfigDto) -> Self {
        Self {
            default_cluster_id: value.default_cluster_id,
            countries: value.countries.into_iter().map(Into::into).collect(),
        }
    }
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RelayCountryDto {
    pub country_code: String,
    pub country_name: String,
    #[serde(default)]
    pub cities: Vec<RelayCityDto>,
}

impl From<RelayCountryDto> for RelayCountry {
    fn from(value: RelayCountryDto) -> Self {
        Self {
            country_code: value.country_code,
            country_name: value.country_name,
            cities: value.cities.into_iter().map(Into::into).collect(),
        }
    }
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RelayCityDto {
    pub city_code: String,
    pub city_name: String,
    #[serde(default)]
    pub clusters: Vec<RelayClusterDto>,
}

impl From<RelayCityDto> for RelayCity {
    fn from(value: RelayCityDto) -> Self {
        Self {
            city_code: value.city_code,
            city_name: value.city_name,
            clusters: value.clusters.into_iter().map(Into::into).collect(),
        }
    }
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RelayClusterDto {
    pub cluster_id: String,
    pub cluster_name: String,
    #[serde(default)]
    pub nodes: Vec<RelayNodeDto>,
}

impl From<RelayClusterDto> for RelayCluster {
    fn from(value: RelayClusterDto) -> Self {
        Self {
            cluster_id: value.cluster_id,
            cluster_name: value.cluster_name,
            nodes: value.nodes.into_iter().map(Into::into).collect(),
        }
    }
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RelayNodeDto {
    pub node_id: String,
    pub transport: String,
    pub address: String,
    #[serde(default)]
    pub priority: u32,
    #[serde(default)]
    pub tags: Vec<String>,
}

impl From<RelayNodeDto> for RelayNode {
    fn from(value: RelayNodeDto) -> Self {
        Self {
            node_id: value.node_id,
            transport: value.transport,
            address: value.address,
            priority: value.priority,
            tags: value.tags,
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
    #[serde(default)]
    pub wildcards: Vec<String>,
}

impl From<DnsConfigDto> for DnsConfig {
    fn from(value: DnsConfigDto) -> Self {
        Self {
            servers: value.servers,
            search_domains: value.search_domains,
            wildcards: value.wildcards,
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
    pub policy: AccessPolicyDto,
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
            policy: value.policy.into(),
            mtu: value.mtu,
        })
    }
}

#[derive(Debug, Clone, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct AccessPolicyDto {
    #[serde(default)]
    pub plan_code: Option<String>,
    #[serde(default)]
    pub max_active_devices: Option<u32>,
    #[serde(default)]
    pub bandwidth_limit_mbps: Option<u32>,
    #[serde(default)]
    pub relay_bandwidth_limit_kbps: Option<u32>,
    #[serde(default)]
    pub p2p_unlimited: bool,
    #[serde(default)]
    pub dns_available: bool,
}

impl From<AccessPolicyDto> for AccessPolicy {
    fn from(value: AccessPolicyDto) -> Self {
        Self {
            plan_code: value.plan_code,
            max_active_devices: value.max_active_devices,
            bandwidth_limit_mbps: value.bandwidth_limit_mbps,
            relay_bandwidth_limit_kbps: value.relay_bandwidth_limit_kbps,
            p2p_unlimited: value.p2p_unlimited,
            dns_available: value.dns_available,
        }
    }
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct NetworkMemberDto {
    #[serde(default)]
    pub member_id: Option<String>,
    #[serde(default)]
    pub network_id: Option<String>,
    pub device_id: String,
    #[serde(default)]
    pub role: String,
    #[serde(default)]
    pub created_at: Option<i64>,
    #[serde(default)]
    pub status: Option<String>,
    #[serde(default)]
    pub attachment_id: Option<String>,
    #[serde(default)]
    pub virtual_ip: Option<String>,
    #[serde(default)]
    pub remark: Option<String>,
}

impl From<NetworkMemberDto> for NetworkMember {
    fn from(value: NetworkMemberDto) -> Self {
        Self {
            member_id: value.member_id,
            network_id: value.network_id,
            attachment_id: value.attachment_id,
            device_id: value.device_id,
            role: value.role,
            status: value.status,
            virtual_ip: value.virtual_ip,
            remark: value.remark,
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
    #[serde(rename = "type")]
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
    pub country_code: Option<String>,
    #[serde(default)]
    pub country_name: Option<String>,
    #[serde(default)]
    pub city_code: Option<String>,
    #[serde(default)]
    pub city_name: Option<String>,
    #[serde(default)]
    pub cluster_id: Option<String>,
    #[serde(default)]
    pub cluster_name: Option<String>,
    #[serde(default)]
    pub endpoints: Vec<RelayEndpointDto>,
}

impl From<RelayRegionDto> for RelayRegion {
    fn from(value: RelayRegionDto) -> Self {
        Self {
            region_id: value.region_id,
            region_name: value.region_name,
            country_code: value.country_code,
            country_name: value.country_name,
            city_code: value.city_code,
            city_name: value.city_name,
            cluster_id: value.cluster_id,
            cluster_name: value.cluster_name,
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
    #[serde(default)]
    pub cluster_name: Option<String>,
    pub region_id: String,
    pub region_name: String,
    #[serde(default)]
    pub country_code: Option<String>,
    #[serde(default)]
    pub country_name: Option<String>,
    #[serde(default)]
    pub city_code: Option<String>,
    #[serde(default)]
    pub city_name: Option<String>,
    #[serde(default = "default_recommended_fanout")]
    pub recommended_fanout: u8,
    #[serde(default)]
    pub nodes: Vec<DerpNodeDto>,
}

impl TryFrom<DerpClusterDto> for DerpCluster {
    type Error = String;

    fn try_from(value: DerpClusterDto) -> Result<Self, Self::Error> {
        let cluster_id = value.cluster_id;
        let cluster_name = value.cluster_name;
        let region_id = value.region_id;
        let region_name = value.region_name;
        let country_code = value.country_code;
        let country_name = value.country_name;
        let city_code = value.city_code;
        let city_name = value.city_name;
        let nodes = value
            .nodes
            .into_iter()
            .map(|node| {
                node.into_meta(
                    cluster_id.clone(),
                    region_id.clone(),
                    country_code.clone(),
                    country_name.clone(),
                    city_code.clone(),
                    city_name.clone(),
                )
            })
            .collect();
        Ok(Self {
            cluster_id,
            cluster_name,
            region_id,
            region_name,
            country_code,
            country_name,
            city_code,
            city_name,
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
    fn into_meta(
        self,
        cluster_id: String,
        region_id: String,
        country_code: Option<String>,
        country_name: Option<String>,
        city_code: Option<String>,
        city_name: Option<String>,
    ) -> DerpNodeMeta {
        DerpNodeMeta {
            cluster_id,
            region_id,
            country_code,
            country_name,
            city_code,
            city_name,
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
    #[serde(skip_serializing_if = "Option::is_none")]
    pub derp_cluster_id: Option<String>,
    #[serde(skip_serializing_if = "Vec::is_empty")]
    pub preferred_derp_node_ids: Vec<String>,
    pub reason: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub relay_region_id: Option<String>,
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
            relay_region_id: value.relay_region_id,
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
    pub country_code: Option<String>,
    #[serde(default)]
    pub city_code: Option<String>,
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
            country_code: value.country_code,
            city_code: value.city_code,
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
