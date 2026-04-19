use serde::{Deserialize, Serialize};

use crate::DerpNodeMeta;

/// 单次 DERP 探测样本。
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ProbeSample {
    pub rtt_ms: u32,
    pub timed_out: bool,
    pub packet_loss_ppm: u32,
    pub sampled_at_ms: u64,
}

/// 单个 DERP 连接状态。
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum DerpLinkState {
    Connecting,
    Ready,
    Suspect,
    Failed,
    Closed,
}

/// DERP 健康快照。
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct DerpHealth {
    pub rtt_ms_ewma: u32,
    pub loss_ppm: u32,
    pub timeout_count: u32,
    pub consecutive_failures: u32,
    pub last_probe_at_ms: u64,
    pub last_recv_at_ms: u64,
    pub score: u32,
}

/// 单个 DERP 连接快照。
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct DerpLinkSnapshot {
    pub meta: DerpNodeMeta,
    pub state: DerpLinkState,
    pub health: DerpHealth,
    pub is_active: bool,
}

/// DERP 连接池状态。
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct DerpPoolState {
    pub cluster_id: String,
    pub session_id: String,
    pub active_node_id: Option<String>,
    pub links: Vec<DerpLinkSnapshot>,
    pub switch_epoch: u64,
}

/// 路径切换原因。
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub enum SwitchReason {
    BootstrapPreferred,
    ProbeTimeout,
    HighRtt,
    HighLoss,
    SendFailure,
    Manual,
}

/// DERP active 切换事件。
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct DerpSwitchEvent {
    pub cluster_id: String,
    pub from_node_id: Option<String>,
    pub to_node_id: String,
    pub reason: SwitchReason,
    pub happened_at_ms: u64,
}
