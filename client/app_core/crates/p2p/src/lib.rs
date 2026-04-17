//! P2P 连接抽象定义。

mod candidate;
mod connector;

pub use candidate::PeerCandidate;
pub use connector::P2PConnector;
