use serde::{Deserialize, Serialize};

/// WireGuard 允许通过隧道宣告的地址范围。
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct AllowedIp {
    pub cidr: String,
}

/// WireGuard 使用的本地密钥对。
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct WireGuardKeyPair {
    pub public_key: String,
    pub private_key: String,
}

/// 客户端本地持有的 tunnel key material。
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct TunnelKeyMaterial {
    pub device_id: Option<String>,
    pub node_id: Option<String>,
    pub key_pair: WireGuardKeyPair,
    pub created_at_ms: u64,
}

/// WireGuard 对端配置。
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct WireGuardPeerConfig {
    pub peer_node_id: Option<String>,
    pub public_key: String,
    pub preshared_key: Option<String>,
    pub endpoint: Option<String>,
    pub allowed_ips: Vec<AllowedIp>,
    pub persistent_keepalive_seconds: Option<u16>,
}

/// WireGuard 接口配置。
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct WireGuardInterfaceConfig {
    pub interface_name: Option<String>,
    pub key_pair: WireGuardKeyPair,
    pub listen_port: Option<u16>,
    pub mtu: Option<u16>,
    pub addresses: Vec<String>,
    pub dns_servers: Vec<String>,
    #[serde(default)]
    pub peers: Vec<WireGuardPeerConfig>,
}

/// Tunnel 当前承载 WireGuard transport 的底层路径。
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum TunnelTransport {
    P2P,
    Relay,
    Derp,
}

/// WireGuard 运行时观测。
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct WireGuardRuntimeStats {
    pub transport: TunnelTransport,
    pub peer_public_key: String,
    pub selected_endpoint: Option<String>,
    pub latest_handshake_at_ms: Option<u64>,
    pub bytes_received: u64,
    pub bytes_sent: u64,
}
