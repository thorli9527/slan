//! AppCore 共享领域模型。
//!
//! 这一层不关心具体实现方式，只定义 Flutter、控制面客户端、P2P、
//! relay 和隧道管理之间共享的数据结构。
mod connection;
mod derp;
mod identity;
mod network;
mod wireguard;

pub use connection::{ActivePath, ConnectionPath, ConnectionState};
pub use derp::{
    BootstrapConfig, DerpCluster, DerpHealth, DerpLinkSnapshot, DerpLinkState, DerpMap,
    DerpNodeMeta, DerpPoolState, DerpSwitchEvent, DerpTransport, ProbeSample, RelayTicket,
    SwitchReason,
};
pub use identity::{Device, MqttCredential, Node, Session};
pub use network::{
    ControlPlaneConfig, DnsConfig, Endpoint, Network, NetworkAssignment, NetworkJoinResult,
    NetworkMap, NetworkMember, Peer, RelayCity, RelayCluster, RelayConfig, RelayCountry,
    RelayEndpoint, RelayNode, RelayRegion, Route,
};
pub use wireguard::{
    AllowedIp, TunnelKeyMaterial, TunnelTransport, WireGuardInterfaceConfig, WireGuardKeyPair,
    WireGuardPeerConfig, WireGuardRuntimeStats,
};
