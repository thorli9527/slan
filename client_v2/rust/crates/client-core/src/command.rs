use serde::{Deserialize, Serialize};

use crate::platform::NetworkRuntimeState;

/// 设备登录成功后由服务端或平台层传入客户端核心的认证载荷。
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct AuthPayload {
    /// 用户访问令牌，用于后续调用控制面接口。
    pub access_token: String,
    /// 可选刷新令牌。
    pub refresh_token: Option<String>,
    /// 当前登录用户 ID。
    pub user_id: String,
    /// UI 展示用用户名称或邮箱。
    pub user_label: String,
    /// 当前设备 ID，部分登录阶段可能为空。
    pub device_id: Option<String>,
    /// 当前活跃网络 ID。
    #[serde(default)]
    pub active_network_id: Option<String>,
    /// 服务端分配给设备的虚拟 IP。
    pub virtual_ip: Option<String>,
    /// access token 剩余有效期，单位秒。
    pub expires_in: Option<u64>,
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

/// 密码登录命令的请求载荷。
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct PasswordLoginPayload {
    pub email: String,
    pub password: String,
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
    /// 打开 Web Console 的客户端登录入口。
    OpenClientLogin,
    /// 请求使用邮箱密码登录。
    LoginWithPassword(PasswordLoginPayload),
    /// 应用设备登录完成后的用户认证信息。
    ApplyDeviceUserLogin(AuthPayload),
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
    Logout,
    /// 主动刷新平台运行状态。
    Refresh,
    /// 请求打开 Web 控制台。
    OpenWebConsole,
}
