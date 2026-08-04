use client_core::{PlatformAclPolicy, PlatformResolverConfig, RelayDataPlaneConfig, RouteSpec};
use serde::Serialize;

/// 返回给 Flutter / 移动端原生插件的最小平台网络配置。
///
/// Rust 内部运行时仍可保留更完整的 resolver zone / record 结构；这里仅保留
/// 移动端实际启动 VPN / PacketTunnel 所需的最小 resolver 配置与路由信息。
#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct MobilePlatformNetworkConfig {
    pub(crate) session_name: String,
    pub(crate) virtual_ip: String,
    pub(crate) prefix_len: u8,
    #[serde(default)]
    pub(crate) network_configs: Vec<MobilePlatformDeviceNetworkConfig>,
    /// 透传给原生平台数据面的最小 resolver 配置。
    #[serde(default)]
    pub(crate) resolver: PlatformResolverConfig,
    #[serde(default)]
    pub(crate) routes: Vec<RouteSpec>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub(crate) mtu: Option<u16>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub(crate) relay_endpoint_id: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub(crate) relay_transport: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub(crate) relay_address: Option<String>,
    #[serde(default)]
    pub(crate) acl_policies: Vec<PlatformAclPolicy>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub(crate) relay_data_plane: Option<RelayDataPlaneConfig>,
}

/// 返回给 Flutter 的网络摘要。
///
/// 不再暴露 resolver record 规模之类只属于 Rust 内部 runtime 的统计。
#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct MobilePlatformDeviceNetworkConfig {
    pub(crate) network_id: String,
    pub(crate) device_id: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub(crate) network_name: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub(crate) intra_group_policy: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub(crate) network_created_at: Option<i64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub(crate) config_version: Option<i64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub(crate) global_ip: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub(crate) global_name: Option<String>,
    #[serde(default)]
    pub(crate) peer_count: usize,
    #[serde(default)]
    pub(crate) security_rule_count: usize,
    #[serde(default)]
    pub(crate) relay_candidate_count: usize,
}
