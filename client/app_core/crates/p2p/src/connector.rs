use std::sync::Arc;

use slan_app_core::ConnectionState;

use crate::PeerCandidate;

/// P2P 连接器接口。
pub trait P2PConnector: Send + Sync {
    /// 尝试基于候选信息建立点对点连接。
    fn connect(&self, peer: &PeerCandidate) -> Result<ConnectionState, String>;
    /// 通过已建立的 P2P 路径发送 transport packet。
    fn send_transport_packet(&self, packet: &[u8]) -> Result<(), String>;
    /// 非阻塞轮询接收已建立 P2P 路径上的 transport packet。
    fn poll_transport_packet(&self) -> Result<Option<Vec<u8>>, String>;
}

impl<T> P2PConnector for Arc<T>
where
    T: P2PConnector,
{
    fn connect(&self, peer: &PeerCandidate) -> Result<ConnectionState, String> {
        self.as_ref().connect(peer)
    }

    fn send_transport_packet(&self, packet: &[u8]) -> Result<(), String> {
        self.as_ref().send_transport_packet(packet)
    }

    fn poll_transport_packet(&self) -> Result<Option<Vec<u8>>, String> {
        self.as_ref().poll_transport_packet()
    }
}
