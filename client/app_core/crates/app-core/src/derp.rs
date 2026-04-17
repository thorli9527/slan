use crate::network::ControlPlaneConfig;
use crate::{Device, Network, NetworkMap, RelayConfig};

/// DERP 传输类型。
#[derive(Debug, Clone)]
pub enum DerpTransport {
    Udp,
    Tcp,
    Quic,
}

/// 单个 DERP 节点元数据。
#[derive(Debug, Clone)]
pub struct DerpNodeMeta {
    pub cluster_id: String,
    pub region_id: String,
    pub node_id: String,
    pub host: String,
    pub port: u16,
    pub transport: DerpTransport,
    pub priority: u32,
    pub tags: Vec<String>,
}

/// 一个 DERP 集群。
#[derive(Debug, Clone)]
pub struct DerpCluster {
    pub cluster_id: String,
    pub region_id: String,
    pub region_name: String,
    pub recommended_fanout: u8,
    pub nodes: Vec<DerpNodeMeta>,
}

/// 控制面下发的 DERP 视图。
#[derive(Debug, Clone)]
pub struct DerpMap {
    pub probe_interval_seconds: u32,
    pub clusters: Vec<DerpCluster>,
}

/// 客户端启动配置。
#[derive(Debug, Clone)]
pub struct BootstrapConfig {
    pub device: Device,
    pub networks: Vec<Network>,
    pub control_plane: ControlPlaneConfig,
    pub stun_servers: Vec<String>,
    pub relay: RelayConfig,
    pub derp_map: Option<DerpMap>,
    pub network_map: Option<NetworkMap>,
}

/// Relay 票据。
#[derive(Debug, Clone)]
pub struct RelayTicket {
    pub ticket_id: String,
    pub network_id: String,
    pub session_id: String,
    pub src_node_id: String,
    pub dst_node_id: String,
    pub derp_cluster_id: Option<String>,
    pub allowed_derp_node_ids: Vec<String>,
    pub relay_url: String,
    pub expires_at: String,
    pub session_key: Option<String>,
    pub signature: String,
}

/// 单次 DERP 探测样本。
#[derive(Debug, Clone)]
pub struct ProbeSample {
    pub rtt_ms: u32,
    pub timed_out: bool,
    pub packet_loss_ppm: u32,
    pub sampled_at_ms: u64,
}

/// 单个 DERP 连接状态。
#[derive(Debug, Clone)]
pub enum DerpLinkState {
    Connecting,
    Ready,
    Suspect,
    Failed,
    Closed,
}

/// DERP 健康快照。
#[derive(Debug, Clone)]
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
#[derive(Debug, Clone)]
pub struct DerpLinkSnapshot {
    pub meta: DerpNodeMeta,
    pub state: DerpLinkState,
    pub health: DerpHealth,
    pub is_active: bool,
}

/// DERP 连接池状态。
#[derive(Debug, Clone)]
pub struct DerpPoolState {
    pub cluster_id: String,
    pub session_id: String,
    pub active_node_id: Option<String>,
    pub links: Vec<DerpLinkSnapshot>,
    pub switch_epoch: u64,
}

/// 路径切换原因。
#[derive(Debug, Clone)]
pub enum SwitchReason {
    BootstrapPreferred,
    ProbeTimeout,
    HighRtt,
    HighLoss,
    SendFailure,
    Manual,
}

/// DERP active 切换事件。
#[derive(Debug, Clone)]
pub struct DerpSwitchEvent {
    pub cluster_id: String,
    pub from_node_id: Option<String>,
    pub to_node_id: String,
    pub reason: SwitchReason,
    pub happened_at_ms: u64,
}
