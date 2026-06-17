use anyhow::Result;
use serde::{Deserialize, Serialize};

use crate::{PathPolicy, PeerPathConfig, PeerPathRuntime};

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
    pub dns_servers: Vec<String>,
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

/// Android VPN 会话配置，同时作为跨平台 PlatformNetworkConfig 的当前别名。
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
    pub dns_servers: Vec<String>,
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

/// 当前平台网络配置类型；后续各平台分化时可替换为枚举或专用结构。
pub type PlatformNetworkConfig = AndroidVpnSessionConfig;

/// 单个网络配置摘要，用于平台层了解本设备在各网络内的配置规模。
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct PlatformDeviceNetworkConfig {
    pub network_id: String,
    pub device_id: String,
    #[serde(default)]
    pub network_name: Option<String>,
    #[serde(default)]
    pub network_code: Option<String>,
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
    pub dns_record_count: usize,
    #[serde(default)]
    pub security_rule_count: usize,
    #[serde(default)]
    pub relay_candidate_count: usize,
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
    /// 配置 DNS。
    fn configure_dns(&self, dns_servers: &[String]) -> Result<()>;
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
}

#[cfg(test)]
mod tests {
    use super::*;

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
                resolved_peer_virtual_ips: vec!["100.64.0.2".to_string()],
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
            path_policy: PathPolicy::default(),
            peer_paths: Vec::new(),
            relay_mtu: Some(1280),
            max_frame_payload: Some(1200),
            acl_policies: vec![acl_policy],
            sessions: vec![RelayPeerSession {
                session_id: "session-1".to_string(),
                peer_node_id: "node-peer".to_string(),
                peer_virtual_ips: vec!["100.64.0.2".to_string()],
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
    fn platform_network_config_round_trips_acl_policies() {
        let acl_policy = test_acl_policy();
        let config = AndroidVpnSessionConfig {
            session_name: "SLAN".to_string(),
            virtual_ip: "100.64.0.1".to_string(),
            prefix_len: 32,
            network_configs: Vec::new(),
            dns_servers: vec!["100.64.0.53".to_string()],
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

        let value = serde_json::to_value(&config).expect("serialize platform config");
        assert!(value.get("aclPolicies").is_some());
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
            serde_json::from_value(value).expect("deserialize platform config");
        assert_eq!(decoded.acl_policies, vec![acl_policy.clone()]);
        assert_eq!(
            decoded
                .relay_data_plane
                .expect("relay data plane should decode")
                .acl_policies,
            vec![acl_policy]
        );
    }

    #[test]
    fn platform_network_config_defaults_missing_acl_policies_to_empty() {
        let decoded: AndroidVpnSessionConfig = serde_json::from_value(serde_json::json!({
            "sessionName": "SLAN",
            "virtualIp": "100.64.0.1",
            "prefixLen": 32,
            "dnsServers": [],
            "routes": [],
            "mtu": null,
            "relayEndpointId": null,
            "relayTransport": null,
            "relayAddress": null
        }))
        .expect("deserialize minimal platform config");

        assert!(decoded.acl_policies.is_empty());
        assert!(decoded.relay_data_plane.is_none());
    }
}
