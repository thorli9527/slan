use crate::TunnelConfig;

/// 隧道管理能力。
pub trait TunnelManager: Send + Sync {
    /// 建立到对端的隧道。
    fn establish(&self, config: &TunnelConfig) -> Result<(), String>;
    /// 按对端虚拟 IP 关闭隧道。
    fn close(&self, peer_virtual_ip: &str) -> Result<(), String>;
}
