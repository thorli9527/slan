//! P2P 连接抽象定义。

mod candidate;
mod connector;
mod runtime;

pub use candidate::PeerCandidate;
pub use connector::P2PConnector;
pub use runtime::SocketP2PConnector;
