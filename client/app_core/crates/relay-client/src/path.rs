use std::sync::Arc;

use slan_app_core::ActivePath;

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct PathManagerError {
    pub code: Option<String>,
    pub message: String,
}

impl PathManagerError {
    pub fn new(code: Option<String>, message: impl Into<String>) -> Self {
        Self {
            code,
            message: message.into(),
        }
    }

    pub fn message(message: impl Into<String>) -> Self {
        Self::new(None, message)
    }
}

impl std::fmt::Display for PathManagerError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self.code.as_deref() {
            Some(code) if !code.trim().is_empty() => write!(f, "[{code}] {}", self.message),
            _ => f.write_str(&self.message),
        }
    }
}

impl std::error::Error for PathManagerError {}

impl From<String> for PathManagerError {
    fn from(value: String) -> Self {
        Self::message(value)
    }
}

/// 路径管理能力。
pub trait PathManager: Send + Sync {
    /// 通知 P2P 失败并进入 relay/derp 选择。
    fn on_p2p_failed(&self, peer_node_id: &str, reason: &str) -> Result<(), PathManagerError>;
    /// 通知 P2P 恢复。
    fn on_p2p_recovered(&self, peer_node_id: &str) -> Result<(), PathManagerError>;
    /// 查询当前 active 路径。
    fn current_path(&self) -> ActivePath;
    /// 通过当前 active 路径发送 transport packet。
    fn send_transport_packet(&self, packet: &[u8]) -> Result<(), PathManagerError>;
    /// 非阻塞轮询当前 active 路径上的 transport packet。
    fn poll_transport_packet(&self) -> Result<Option<Vec<u8>>, PathManagerError>;
}

impl<T> PathManager for Arc<T>
where
    T: PathManager,
{
    fn on_p2p_failed(&self, peer_node_id: &str, reason: &str) -> Result<(), PathManagerError> {
        self.as_ref().on_p2p_failed(peer_node_id, reason)
    }

    fn on_p2p_recovered(&self, peer_node_id: &str) -> Result<(), PathManagerError> {
        self.as_ref().on_p2p_recovered(peer_node_id)
    }

    fn current_path(&self) -> ActivePath {
        self.as_ref().current_path()
    }

    fn send_transport_packet(&self, packet: &[u8]) -> Result<(), PathManagerError> {
        self.as_ref().send_transport_packet(packet)
    }

    fn poll_transport_packet(&self) -> Result<Option<Vec<u8>>, PathManagerError> {
        self.as_ref().poll_transport_packet()
    }
}
