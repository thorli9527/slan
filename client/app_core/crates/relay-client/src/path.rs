use slan_app_core::ActivePath;

/// 路径管理能力。
pub trait PathManager: Send + Sync {
    /// 通知 P2P 失败并进入 relay/derp 选择。
    fn on_p2p_failed(&self, peer_node_id: &str, reason: &str) -> Result<(), String>;
    /// 通知 P2P 恢复。
    fn on_p2p_recovered(&self, peer_node_id: &str) -> Result<(), String>;
    /// 查询当前 active 路径。
    fn current_path(&self) -> ActivePath;
    /// 通过当前 active 路径发送。
    fn send(&self, packet: &[u8]) -> Result<(), String>;
}
