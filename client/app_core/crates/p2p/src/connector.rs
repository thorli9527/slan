use slan_app_core::ConnectionState;

use crate::PeerCandidate;

/// P2P 连接器接口。
pub trait P2PConnector: Send + Sync {
    /// 尝试基于候选信息建立点对点连接。
    fn connect(&self, peer: &PeerCandidate) -> Result<ConnectionState, String>;
}
