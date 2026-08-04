use serde::{Deserialize, Serialize};

use crate::platform::NetworkRuntimeState;

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct DeviceSessionPayload {
    pub device_id: Option<String>,
    pub virtual_ip: Option<String>,
}

/// 服务端下发的虚拟 IP 同步载荷。
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AssignedIpPayload {
    /// 虚拟 IP，可带 CIDR 后缀，核心会归一化为纯 IP。
    pub virtual_ip: String,
    /// 前缀长度，未提供时平台层按默认值处理。
    #[serde(default)]
    pub prefix_len: Option<u8>,
}

/// 客户端间消息通知载荷。
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ClientMessageNoticePayload {
    pub message_id: Option<String>,
    pub from_device_id: Option<String>,
    pub body: Option<String>,
}

/// 平台数据面流量统计载荷。
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct TrafficStatsPayload {
    pub tx_bytes: u64,
    pub rx_bytes: u64,
    pub tx_bytes_per_minute: u64,
    pub rx_bytes_per_minute: u64,
    pub updated_at_ms: u64,
}

/// Flutter/UI/平台插件向 client-core 派发的统一命令。
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(tag = "type", content = "payload", rename_all = "camelCase")]
pub enum ClientCommand {
    /// 将已持久化的设备会话应用到运行时。
    ApplyDeviceSession(DeviceSessionPayload),
    /// 启用本机虚拟网络。
    EnableNetwork,
    /// 禁用本机虚拟网络。
    DisableNetwork,
    /// 同步服务端分配的虚拟 IP。
    SyncAssignedIp(AssignedIpPayload),
    /// 同步平台层读取到的网络运行状态。
    ApplyPlatformRuntimeState(NetworkRuntimeState),
    /// 同步流量统计。
    ApplyTrafficStats(TrafficStatsPayload),
    /// 同步客户端间消息通知。
    ApplyClientMessage(ClientMessageNoticePayload),
    /// 登出并清理本地网络状态。
    DeactivateDevice,
    /// 主动刷新平台运行状态。
    Refresh,
}
