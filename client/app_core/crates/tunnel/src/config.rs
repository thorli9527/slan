use slan_app_core::{TunnelTransport, WireGuardInterfaceConfig, WireGuardPeerConfig};

/// 隧道建立所需配置。
///
#[derive(Debug, Clone)]
pub struct TunnelConfig {
    pub transport: TunnelTransport,
    pub local_virtual_ip: String,
    pub peer_virtual_ip: String,
    pub wireguard_interface: WireGuardInterfaceConfig,
    pub wireguard_peer: WireGuardPeerConfig,
}
