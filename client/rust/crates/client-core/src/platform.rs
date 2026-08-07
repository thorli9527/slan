use anyhow::Result;
use serde::{Deserialize, Serialize};

use crate::{PathCandidate, PathPolicy, PeerPathConfig, PeerPathRuntime};

/// Stable host-local DNS service address intercepted by each SLAN data plane.
/// It is never allocated to devices and never leaves the local tunnel.
pub const SLAN_DNS_SERVICE_IP: &str = "10.0.0.53";

/// 平台路由配置，描述需要写入系统路由表或 VPN 配置的目标网段。
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RouteSpec {
    /// 目标网段或特殊目标，例如 mesh。
    pub destination: String,
    /// 可选网关地址。
    pub gateway: Option<String>,
}

/// 平台网络运行状态，是 UI 与核心状态同步的输入。
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct NetworkRuntimeState {
    /// 虚拟网卡或 VPN adapter 是否存在。
    pub adapter_present: bool,
    /// 虚拟网络是否处于启用状态。
    pub network_enabled: bool,
    /// 当前平台读到的虚拟 IP。
    pub virtual_ip: Option<String>,
    /// 当前数据面活跃路径。
    #[serde(default)]
    pub active_path: Option<crate::PathKind>,
    /// 各 peer 的路径运行状态。
    #[serde(default)]
    pub peer_paths: Vec<PeerPathRuntime>,
}

/// 平台网络诊断结果，用于 UI 或日志展示网络配置状态。
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct PlatformNetworkDiagnostics {
    #[serde(default)]
    pub platform: String,
    #[serde(default)]
    pub adapter_present: bool,
    #[serde(default)]
    pub adapter_name: Option<String>,
    #[serde(default)]
    pub admin_status: Option<String>,
    #[serde(default)]
    pub interface_index: Option<u32>,
    #[serde(default)]
    pub virtual_ip: Option<String>,
    #[serde(default)]
    pub mtu: Option<u32>,
    #[serde(default)]
    pub mss: Option<u32>,
    #[serde(default)]
    pub resolver_servers: Vec<String>,
    #[serde(default)]
    pub resolver_search_domains: Vec<String>,
    #[serde(default)]
    pub resolver_split_domains: Vec<String>,
    #[serde(default)]
    pub routes: Vec<String>,
    #[serde(default)]
    pub checks: Vec<PlatformDiagnosticCheck>,
}

/// 单项平台诊断检查结果。
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct PlatformDiagnosticCheck {
    pub name: String,
    pub ok: bool,
    pub message: Option<String>,
}

/// Android VPN 权限状态。
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum AndroidVpnPermissionState {
    /// 已获得 VpnService 授权。
    Granted,
    /// 需要用户在系统弹窗中确认授权。
    NeedsUserConsent,
}

/// Android VPN 授权请求信息。
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AndroidVpnConsentRequest {
    pub request_id: String,
    pub message: Option<String>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct PlatformResolverConfig {
    /// 需要写入平台网络栈的 resolver 服务器。
    #[serde(default)]
    pub servers: Vec<String>,
    /// 平台侧搜索域。
    #[serde(default)]
    pub search_domains: Vec<String>,
    /// 平台侧 split-horizon 域。
    #[serde(default)]
    pub split_domains: Vec<String>,
    /// 当本地 authoritative 数据不足时是否允许回退系统 resolver。
    #[serde(default)]
    pub fallback_to_system_resolvers: bool,
}

/// Rust 内部平台网络配置。
///
/// 这份结构既用于 Android/iOS 启动参数，也用于 FFI、本地 resolver responder、
/// macOS 数据面等内部模块。Flutter 层消费的是 service 额外裁剪后的 DTO，
/// 不应假定这里的全部字段都会跨层暴露。
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AndroidVpnSessionConfig {
    /// 系统 VPN 会话名称。
    pub session_name: String,
    /// 本机虚拟 IP。
    pub virtual_ip: String,
    /// 虚拟 IP 前缀长度。
    pub prefix_len: u8,
    #[serde(default)]
    pub network_configs: Vec<PlatformDeviceNetworkConfig>,
    #[serde(default)]
    pub resolver: PlatformResolverConfig,
    #[serde(default)]
    pub resolver_zones: Vec<PlatformResolverZone>,
    #[serde(default)]
    pub resolver_records: Vec<PlatformResolverRecord>,
    pub routes: Vec<RouteSpec>,
    pub mtu: Option<u16>,
    pub relay_endpoint_id: Option<String>,
    pub relay_transport: Option<String>,
    pub relay_address: Option<String>,
    #[serde(default)]
    pub acl_policies: Vec<PlatformAclPolicy>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub relay_data_plane: Option<RelayDataPlaneConfig>,
}

/// 当前 Rust 内部平台网络配置类型；后续各平台分化时可替换为枚举或专用结构。
pub type PlatformNetworkConfig = AndroidVpnSessionConfig;

/// 单个网络配置摘要，供 Rust 内部平台层评估配置规模。
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct PlatformDeviceNetworkConfig {
    pub network_id: String,
    pub device_id: String,
    #[serde(default)]
    pub network_name: Option<String>,
    #[serde(default)]
    pub intra_group_policy: Option<String>,
    #[serde(default)]
    pub network_created_at: Option<i64>,
    #[serde(default)]
    pub config_version: Option<i64>,
    #[serde(default)]
    pub global_ip: Option<String>,
    #[serde(default)]
    pub global_name: Option<String>,
    #[serde(default)]
    pub peer_count: usize,
    #[serde(default)]
    pub resolver_record_count: usize,
    #[serde(default)]
    pub security_rule_count: usize,
    #[serde(default)]
    pub relay_candidate_count: usize,
}

/// Rust 内部本地 resolver zone 配置。
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct PlatformResolverZone {
    pub zone_id: String,
    pub network_id: String,
    pub zone_name: String,
}

/// Rust 内部本地 resolver 记录配置，目前供隧道内 resolver responder 使用。
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct PlatformResolverRecord {
    pub record_id: String,
    pub zone_id: String,
    pub network_id: String,
    pub name: String,
    #[serde(default)]
    pub fqdn: Option<String>,
    #[serde(default)]
    pub record_type: String,
    #[serde(default)]
    pub target_device_id: Option<String>,
    #[serde(default)]
    pub target_ip: Option<String>,
    #[serde(default)]
    pub cname: Option<String>,
    #[serde(default)]
    pub port: Option<String>,
    #[serde(default)]
    pub ttl: Option<i64>,
}

/// 客户端数据面 ACL 策略，来自服务端安全组与规则。
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct PlatformAclPolicy {
    pub network_id: String,
    #[serde(default)]
    pub rules: Vec<PlatformAclRule>,
}

/// 客户端数据面 ACL 规则，保留服务端字段并补充 device 规则的解析结果。
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct PlatformAclRule {
    pub rule_id: String,
    pub security_group_id: String,
    #[serde(default)]
    pub direction: String,
    #[serde(default)]
    pub priority: i64,
    #[serde(default)]
    pub action: String,
    #[serde(default)]
    pub protocol: String,
    #[serde(default)]
    pub port_from: i64,
    #[serde(default)]
    pub port_to: i64,
    #[serde(default)]
    pub peer_type: String,
    #[serde(default)]
    pub peer_value: String,
    #[serde(default)]
    pub source_type: String,
    #[serde(default)]
    pub source_value: String,
    #[serde(default)]
    pub enabled: bool,
    #[serde(default)]
    pub resolved_peer_node_id: Option<String>,
    #[serde(default)]
    pub resolved_peer_virtual_ips: Vec<String>,
}

/// ACL 匹配时使用的 peer 上下文。
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct PlatformAclPeer {
    #[serde(default)]
    pub peer_node_id: Option<String>,
    #[serde(default)]
    pub peer_virtual_ips: Vec<String>,
}

/// Relay 数据面配置，描述本机如何通过 relay/DERP 与 peer 建立转发会话。
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RelayDataPlaneConfig {
    /// 是否启用 relay 数据面。
    pub enabled: bool,
    /// relay 传输类型，例如 udp。
    pub transport: String,
    /// relay 服务端地址。
    pub relay_address: String,
    /// 本机节点 ID。
    pub local_node_id: String,
    /// 所属虚拟网络 ID。
    pub network_id: String,
    /// Direct UDP 固定监听端口；随机模式下为 0。
    #[serde(default = "default_direct_udp_port")]
    pub direct_udp_port: u16,
    /// 是否让操作系统为 Direct UDP 分配随机端口。
    #[serde(default)]
    pub randomize_direct_udp_port: bool,
    /// Server-managed direct and relay infrastructure nodes.
    #[serde(default)]
    pub node_configs: Vec<NodeConfig>,
    #[serde(default)]
    pub path_policy: PathPolicy,
    #[serde(default)]
    pub peer_paths: Vec<PeerPathConfig>,
    #[serde(default)]
    pub relay_mtu: Option<u16>,
    #[serde(default)]
    pub max_frame_payload: Option<u16>,
    #[serde(default)]
    pub acl_policies: Vec<PlatformAclPolicy>,
    pub sessions: Vec<RelayPeerSession>,
}

impl RelayDataPlaneConfig {
    /// Compare only fields that change packet routing or transport setup.
    /// Candidate health is runtime observation and must not restart the tunnel.
    pub fn data_plane_equivalent(&self, other: &Self) -> bool {
        self.data_plane_change_fields(other).is_empty()
    }

    /// Return the routing domains that require a data-plane reconfiguration.
    pub fn data_plane_change_fields(&self, other: &Self) -> Vec<&'static str> {
        let mut fields = Vec::new();
        if self.enabled != other.enabled {
            fields.push("enabled");
        }
        if self.transport != other.transport {
            fields.push("transport");
        }
        if self.relay_address != other.relay_address {
            fields.push("relay_address");
        }
        if self.local_node_id != other.local_node_id {
            fields.push("local_node_id");
        }
        if self.network_id != other.network_id {
            fields.push("network_id");
        }
        if self.direct_udp_port != other.direct_udp_port {
            fields.push("direct_udp_port");
        }
        if self.randomize_direct_udp_port != other.randomize_direct_udp_port {
            fields.push("randomize_direct_udp_port");
        }
        if self.node_configs != other.node_configs {
            fields.push("node_configs");
        }
        if self.path_policy != other.path_policy {
            fields.push("path_policy");
        }
        if !peer_path_transport_config_matches(&self.peer_paths, &other.peer_paths) {
            fields.push("peer_paths");
        }
        if self.relay_mtu != other.relay_mtu {
            fields.push("relay_mtu");
        }
        if self.max_frame_payload != other.max_frame_payload {
            fields.push("max_frame_payload");
        }
        if self.acl_policies != other.acl_policies {
            fields.push("acl_policies");
        }
        if self.sessions != other.sessions {
            fields.push("sessions");
        }
        fields
    }
}

fn peer_path_transport_config_matches(left: &[PeerPathConfig], right: &[PeerPathConfig]) -> bool {
    left.len() == right.len()
        && left.iter().all(|left| {
            right.iter().any(|right| {
                left.peer_node_id == right.peer_node_id
                    && unordered_strings_match(&left.peer_virtual_ips, &right.peer_virtual_ips)
                    && path_candidate_transport_config_matches(&left.candidates, &right.candidates)
            })
        })
}

fn path_candidate_transport_config_matches(
    left: &[PathCandidate],
    right: &[PathCandidate],
) -> bool {
    left.len() == right.len()
        && left.iter().all(|left| {
            right.iter().any(|right| {
                left.kind == right.kind
                    && left.endpoint_id == right.endpoint_id
                    && left.address == right.address
                    && left.session_id == right.session_id
                    && left.transport == right.transport
            })
        })
}

fn unordered_strings_match(left: &[String], right: &[String]) -> bool {
    left.len() == right.len() && left.iter().all(|value| right.contains(value))
}

fn default_direct_udp_port() -> u16 {
    41642
}

/// Server-managed infrastructure node used for direct discovery or relay.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct NodeConfig {
    #[serde(default)]
    pub node_id: String,
    pub connection_type: String,
    pub transport: String,
    pub path_kind: String,
    pub address: String,
    #[serde(default)]
    pub country_code: String,
    #[serde(default)]
    pub city_code: String,
    #[serde(default)]
    pub priority: u16,
}

impl NodeConfig {
    /// Validate the canonical server-managed node tuple.
    pub fn is_valid(&self) -> bool {
        !self.node_id.trim().is_empty()
            && !self.address.trim().is_empty()
            && matches!(
                (
                    self.connection_type.as_str(),
                    self.transport.as_str(),
                    self.path_kind.as_str(),
                ),
                ("direct", "udp", "direct_udp")
                    | ("relay", "udp", "relay_udp")
                    | ("relay", "tcp", "relay_tcp")
            )
    }

    /// Return the fixed infrastructure path order.
    pub fn path_rank(&self) -> u8 {
        node_config_path_rank(&self.path_kind)
    }

    /// Whether this node can discover the public endpoint of a direct UDP socket.
    pub fn is_direct_udp_discovery(&self) -> bool {
        self.connection_type == "direct"
            && self.transport == "udp"
            && self.path_kind == "direct_udp"
    }
}

/// Return the canonical order for server-managed infrastructure path names.
pub fn node_config_path_rank(path_kind: &str) -> u8 {
    match path_kind {
        "direct_udp" => 0,
        "relay_udp" => 1,
        "relay_tcp" => 2,
        _ => u8::MAX,
    }
}

/// 本机到单个 peer 的 relay 会话配置。
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RelayPeerSession {
    pub session_id: String,
    pub peer_node_id: String,
    #[serde(default)]
    pub peer_virtual_ips: Vec<String>,
    pub ticket: RelayTicket,
}

/// 服务端签发的 relay/DERP 授权票据。
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RelayTicket {
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
    pub session_key: String,
    pub signature: String,
}

/// Android socket protect 请求，要求平台把指定 fd 排除在 VPN 路由之外。
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AndroidSocketProtectionRequest {
    pub socket_fd: i32,
    pub reason: AndroidSocketProtectionReason,
}

/// Android socket protect 的业务原因。
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum AndroidSocketProtectionReason {
    ControlPlane,
    Mqtt,
    RelayProbe,
    RelayTransport,
}

/// Android 平台层推给核心/UI 的网络事件。
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AndroidNetworkEvent {
    pub event_type: AndroidNetworkEventType,
    pub message: Option<String>,
    pub runtime_state: Option<NetworkRuntimeState>,
}

/// Android 网络事件类型。
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum AndroidNetworkEventType {
    PermissionRequired,
    PermissionGranted,
    VpnStarted,
    VpnStopped,
    VpnRevoked,
    ConnectivityChanged,
    RelayChanged,
    Error,
}

/// PlatformNetwork 抽象各操作系统的虚拟网卡/VPN/TUN 数据面操作。
pub trait PlatformNetwork {
    /// 安装或准备虚拟网卡。
    fn install_adapter(&self) -> Result<()>;
    /// 配置本机虚拟 IP。
    fn configure_ip(&self, virtual_ip: &str, prefix_len: u8) -> Result<()>;
    /// 配置系统或 VPN 路由。
    fn configure_routes(&self, routes: &[RouteSpec]) -> Result<()>;
    /// 配置平台 resolver。
    fn configure_resolver(&self, resolver: &PlatformResolverConfig) -> Result<()>;
    /// 配置本地 authoritative resolver 的 zone/record 映射。
    fn configure_resolver_map(
        &self,
        _resolver_zones: &[PlatformResolverZone],
        _resolver_records: &[PlatformResolverRecord],
    ) -> Result<()> {
        Ok(())
    }
    /// 配置 relay 数据面；不支持的平台可使用默认空实现。
    fn configure_relay(&self, _config: Option<&RelayDataPlaneConfig>) -> Result<()> {
        Ok(())
    }
    /// 读取平台诊断信息；不支持的平台返回默认空诊断。
    fn diagnostics(&self) -> Result<PlatformNetworkDiagnostics> {
        Ok(PlatformNetworkDiagnostics::default())
    }
    /// 禁用虚拟网络并清理平台配置。
    fn disable_network(&self) -> Result<()>;
    /// 读取平台当前网络运行状态。
    fn read_runtime_state(&self) -> Result<NetworkRuntimeState>;
    /// 标记网络已启用（更新平台特定的运行时状态缓存）。
    /// 默认空实现，Windows 平台需要更新 Wintun 运行时缓存。
    fn mark_network_enabled(&self, _virtual_ip: &str) -> Result<()> {
        Ok(())
    }
    /// 验证适配器是否实际配置了指定的 IP 地址。
    /// 默认返回 true（跳过验证），Windows 平台会实际检查适配器 IP。
    fn verify_adapter_ip(&self, _virtual_ip: &str) -> Result<bool> {
        Ok(true)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::{PathCandidate, PathKind, PathState};

    fn test_acl_policy() -> PlatformAclPolicy {
        PlatformAclPolicy {
            network_id: "network-1".to_string(),
            rules: vec![PlatformAclRule {
                rule_id: "rule-1".to_string(),
                security_group_id: "sg-1".to_string(),
                direction: "egress".to_string(),
                priority: 100,
                action: "deny".to_string(),
                protocol: "icmp".to_string(),
                port_from: 0,
                port_to: 0,
                peer_type: "device".to_string(),
                peer_value: "device-peer".to_string(),
                source_type: "device".to_string(),
                source_value: "device-peer".to_string(),
                enabled: true,
                resolved_peer_node_id: Some("node-peer".to_string()),
                resolved_peer_virtual_ips: vec!["10.0.0.2".to_string()],
            }],
        }
    }

    fn test_relay_config(acl_policy: PlatformAclPolicy) -> RelayDataPlaneConfig {
        RelayDataPlaneConfig {
            enabled: true,
            transport: "udp".to_string(),
            relay_address: "47.245.40.231:39000".to_string(),
            local_node_id: "node-local".to_string(),
            network_id: "network-1".to_string(),
            direct_udp_port: 41642,
            randomize_direct_udp_port: false,
            node_configs: Vec::new(),
            path_policy: PathPolicy::default(),
            peer_paths: Vec::new(),
            relay_mtu: Some(1280),
            max_frame_payload: Some(1200),
            acl_policies: vec![acl_policy],
            sessions: vec![RelayPeerSession {
                session_id: "session-1".to_string(),
                peer_node_id: "node-peer".to_string(),
                peer_virtual_ips: vec!["10.0.0.2".to_string()],
                ticket: RelayTicket {
                    ticket_id: "ticket-1".to_string(),
                    network_id: "network-1".to_string(),
                    session_id: "session-1".to_string(),
                    src_node_id: "node-local".to_string(),
                    dst_node_id: "node-peer".to_string(),
                    derp_cluster_id: None,
                    country_code: None,
                    city_code: None,
                    allowed_derp_node_ids: Vec::new(),
                    relay_url: "udp://47.245.40.231:39000".to_string(),
                    expires_at: "2026-06-05T00:00:00Z".to_string(),
                    session_key: "session-key".to_string(),
                    signature: "signature".to_string(),
                },
            }],
        }
    }

    #[test]
    fn internal_platform_network_config_round_trips_acl_policies() {
        let acl_policy = test_acl_policy();
        let config = AndroidVpnSessionConfig {
            session_name: "SLAN".to_string(),
            virtual_ip: "10.0.0.1".to_string(),
            prefix_len: 32,
            network_configs: Vec::new(),
            resolver: PlatformResolverConfig {
                servers: vec!["10.0.0.53".to_string()],
                search_domains: vec!["test.lan".to_string()],
                split_domains: vec!["test.lan".to_string()],
                fallback_to_system_resolvers: false,
            },
            resolver_zones: vec![PlatformResolverZone {
                zone_id: "zone-1".to_string(),
                network_id: "network-1".to_string(),
                zone_name: "test.lan".to_string(),
            }],
            resolver_records: vec![PlatformResolverRecord {
                record_id: "record-1".to_string(),
                zone_id: "zone-1".to_string(),
                network_id: "network-1".to_string(),
                name: "mac".to_string(),
                fqdn: Some("mac.test.lan".to_string()),
                record_type: "A".to_string(),
                target_ip: Some("10.0.0.2".to_string()),
                ttl: Some(60),
                ..PlatformResolverRecord::default()
            }],
            routes: vec![RouteSpec {
                destination: "mesh".to_string(),
                gateway: None,
            }],
            mtu: Some(1280),
            relay_endpoint_id: Some("relay-1".to_string()),
            relay_transport: Some("udp".to_string()),
            relay_address: Some("47.245.40.231:39000".to_string()),
            acl_policies: vec![acl_policy.clone()],
            relay_data_plane: Some(test_relay_config(acl_policy.clone())),
        };

        let value = serde_json::to_value(&config).expect("serialize internal platform config");
        assert!(value.get("aclPolicies").is_some());
        assert_eq!(
            value
                .pointer("/dnsZones/0/zoneName")
                .or_else(|| value.pointer("/resolverZones/0/zoneName"))
                .and_then(|value| value.as_str()),
            Some("test.lan")
        );
        assert_eq!(
            value
                .pointer("/dnsRecords/0/fqdn")
                .or_else(|| value.pointer("/resolverRecords/0/fqdn"))
                .and_then(|value| value.as_str()),
            Some("mac.test.lan")
        );
        assert!(value.pointer("/aclPolicies/0/securityGroups").is_none());
        assert_eq!(
            value
                .pointer("/aclPolicies/0/rules/0/sourceType")
                .and_then(|value| value.as_str()),
            Some("device")
        );
        assert!(value
            .get("relayDataPlane")
            .and_then(|relay| relay.get("aclPolicies"))
            .is_some());

        let decoded: AndroidVpnSessionConfig =
            serde_json::from_value(value).expect("deserialize internal platform config");
        assert_eq!(decoded.acl_policies, vec![acl_policy.clone()]);
        assert_eq!(decoded.resolver.servers, vec!["10.0.0.53".to_string()]);
        assert_eq!(
            decoded.resolver.search_domains,
            vec!["test.lan".to_string()]
        );
        assert_eq!(decoded.resolver_zones.len(), 1);
        assert_eq!(decoded.resolver_records.len(), 1);
        assert_eq!(
            decoded
                .relay_data_plane
                .expect("relay data plane should decode")
                .acl_policies,
            vec![acl_policy]
        );
    }

    #[test]
    fn internal_platform_network_config_defaults_missing_acl_policies_to_empty() {
        let decoded: AndroidVpnSessionConfig = serde_json::from_value(serde_json::json!({
            "sessionName": "SLAN",
            "virtualIp": "10.0.0.1",
            "prefixLen": 32,
            "resolver": {
                "servers": [],
                "searchDomains": [],
                "splitDomains": [],
                "fallbackToSystemResolvers": false
            },
            "resolverZones": [],
            "resolverRecords": [],
            "routes": [],
            "mtu": null,
            "relayEndpointId": null,
            "relayTransport": null,
            "relayAddress": null
        }))
        .expect("deserialize minimal internal platform config");

        assert!(decoded.acl_policies.is_empty());
        assert!(decoded.resolver_zones.is_empty());
        assert!(decoded.resolver_records.is_empty());
        assert!(decoded.relay_data_plane.is_none());
    }

    #[test]
    fn relay_data_plane_equivalence_ignores_candidate_health_only() {
        let mut current = test_relay_config(test_acl_policy());
        current.peer_paths = vec![PeerPathConfig {
            peer_node_id: "node-peer".to_string(),
            peer_virtual_ips: vec!["10.0.0.2".to_string()],
            candidates: vec![PathCandidate {
                kind: PathKind::LanUdp,
                state: PathState::Probing,
                endpoint_id: None,
                address: Some("192.168.1.2:41642".to_string()),
                session_id: None,
                transport: Some("udp".to_string()),
                rtt_ms: None,
                path_score: None,
                last_ok_at_ms: None,
                last_error: None,
            }],
        }];
        let mut refreshed = current.clone();
        refreshed.peer_paths[0].candidates[0].state = PathState::Ready;
        refreshed.peer_paths[0].candidates[0].rtt_ms = Some(12);
        refreshed.peer_paths[0].candidates[0].path_score = Some(8);
        refreshed.peer_paths[0].candidates[0].last_ok_at_ms = Some(1234);
        assert!(current.data_plane_equivalent(&refreshed));

        refreshed.peer_paths[0].candidates[0].address = Some("192.168.1.3:41642".to_string());
        assert!(!current.data_plane_equivalent(&refreshed));
    }

    #[test]
    fn relay_data_plane_equivalence_ignores_peer_and_candidate_order() {
        let mut current = test_relay_config(test_acl_policy());
        let candidate = |kind, address: &str| PathCandidate {
            kind,
            state: PathState::Standby,
            endpoint_id: None,
            address: Some(address.to_string()),
            session_id: None,
            transport: Some("udp".to_string()),
            rtt_ms: None,
            path_score: None,
            last_ok_at_ms: None,
            last_error: None,
        };
        current.peer_paths = vec![
            PeerPathConfig {
                peer_node_id: "node-a".to_string(),
                peer_virtual_ips: vec!["10.0.0.2".to_string(), "10.0.0.3".to_string()],
                candidates: vec![
                    candidate(PathKind::LanUdp, "192.168.1.2:41642"),
                    candidate(PathKind::DirectUdp, "203.0.113.2:41642"),
                ],
            },
            PeerPathConfig {
                peer_node_id: "node-b".to_string(),
                peer_virtual_ips: vec!["10.0.0.4".to_string()],
                candidates: vec![candidate(PathKind::DirectUdp, "203.0.113.4:41642")],
            },
        ];
        let mut reordered = current.clone();
        reordered.peer_paths.reverse();
        reordered.peer_paths[1].peer_virtual_ips.reverse();
        reordered.peer_paths[1].candidates.reverse();

        assert!(current.data_plane_equivalent(&reordered));
    }

    #[test]
    fn node_config_accepts_only_canonical_path_tuples() {
        let direct = NodeConfig {
            node_id: "punch-1".to_string(),
            connection_type: "direct".to_string(),
            transport: "udp".to_string(),
            path_kind: "direct_udp".to_string(),
            address: "203.0.113.1:3478".to_string(),
            country_code: "US".to_string(),
            city_code: "5391959".to_string(),
            priority: 100,
        };
        assert!(direct.is_valid());
        assert!(direct.is_direct_udp_discovery());
        assert_eq!(direct.path_rank(), 0);

        let mut invalid = direct;
        invalid.connection_type = "relay".to_string();
        assert!(!invalid.is_valid());
        assert!(!invalid.is_direct_udp_discovery());
    }
}
