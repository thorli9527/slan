use serde::{Deserialize, Serialize};

/// ClientViewState 是 Flutter/UI 层直接消费的客户端状态快照。
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ClientViewState {
    /// 当前是否存在已激活的设备会话。
    pub activated: bool,
    /// 当前设备 ID。
    pub device_id: Option<String>,
    /// 当前设备虚拟 IP。
    pub virtual_ip: Option<String>,
    /// 虚拟网络是否启用。
    pub network_enabled: bool,
    /// 是否正在执行平台同步操作。
    pub syncing: bool,
    /// 当前同步原因，供 UI 展示加载状态。
    pub sync_reason: Option<String>,
    /// UI 网络开关是否允许点击。
    pub switch_enabled: bool,
    /// 一次性提示消息。
    pub notice: Option<String>,
    /// 最近一次错误消息。
    pub error: Option<String>,
    /// 最近收到的客户端间消息 ID。
    #[serde(default)]
    pub last_client_message_id: Option<String>,
    /// 最近收到消息的发送设备 ID。
    #[serde(default)]
    pub last_client_message_from_device_id: Option<String>,
    /// 最近收到的消息正文。
    #[serde(default)]
    pub last_client_message_body: Option<String>,
    /// 累计发送字节数。
    #[serde(default)]
    pub traffic_tx_bytes: Option<u64>,
    /// 累计接收字节数。
    #[serde(default)]
    pub traffic_rx_bytes: Option<u64>,
    /// 每分钟发送字节速率。
    #[serde(default)]
    pub traffic_tx_bytes_per_minute: Option<u64>,
    /// 每分钟接收字节速率。
    #[serde(default)]
    pub traffic_rx_bytes_per_minute: Option<u64>,
    /// 流量统计更新时间，Unix 毫秒。
    #[serde(default)]
    pub traffic_updated_at_ms: Option<u64>,
    /// 当前数据面综合信号分数（0-100）。
    #[serde(default)]
    pub signal_score: Option<u8>,
    /// 当前数据面信号等级：excellent/good/fair/poor/offline。
    #[serde(default)]
    pub signal_quality: Option<String>,
    /// 当前优选路径类型。
    #[serde(default)]
    pub signal_path: Option<String>,
}

impl Default for ClientViewState {
    fn default() -> Self {
        Self {
            activated: false,
            device_id: None,
            virtual_ip: None,
            network_enabled: false,
            syncing: false,
            sync_reason: None,
            switch_enabled: true,
            notice: None,
            error: None,
            last_client_message_id: None,
            last_client_message_from_device_id: None,
            last_client_message_body: None,
            traffic_tx_bytes: None,
            traffic_rx_bytes: None,
            traffic_tx_bytes_per_minute: None,
            traffic_rx_bytes_per_minute: None,
            traffic_updated_at_ms: None,
            signal_score: None,
            signal_quality: Some("offline".to_string()),
            signal_path: None,
        }
    }
}
