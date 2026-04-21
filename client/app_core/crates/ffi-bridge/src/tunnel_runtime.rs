use slan_app_core::{
    ActivePath, AllowedIp, BootstrapConfig, ConnectionPath, ConnectionState, Peer, TunnelTransport,
    WireGuardInterfaceConfig, WireGuardKeyPair, WireGuardPeerConfig,
};
use tunnel::TunnelConfig;

use crate::facade::{TunnelRuntimeView, TunnelState};

pub fn connection_state_from_active_path(path: ActivePath) -> ConnectionState {
    match path {
        ActivePath::None => ConnectionState::Disconnected,
        ActivePath::P2P { .. } => ConnectionState::Connected(ConnectionPath::P2P),
        ActivePath::Relay { .. } => ConnectionState::Connected(ConnectionPath::Relay),
        ActivePath::Derp { .. } => ConnectionState::Connected(ConnectionPath::Derp),
    }
}

fn transport_from_active_path(path: &ActivePath) -> Option<TunnelTransport> {
    match path {
        ActivePath::P2P { .. } => Some(TunnelTransport::P2P),
        ActivePath::Relay { .. } => Some(TunnelTransport::Relay),
        ActivePath::Derp { .. } => Some(TunnelTransport::Derp),
        ActivePath::None => None,
    }
}

pub fn build_tunnel_config(
    active_path: &ActivePath,
    bootstrap: &BootstrapConfig,
    peer: &Peer,
    key_pair: WireGuardKeyPair,
) -> Option<TunnelConfig> {
    let transport = transport_from_active_path(active_path)?;
    let local_virtual_ip = bootstrap.device.virtual_ip.clone()?;
    let peer_virtual_ip = peer.virtual_ips.first().cloned()?;
    Some(TunnelConfig {
        transport,
        local_virtual_ip: local_virtual_ip.clone(),
        peer_virtual_ip: peer_virtual_ip.clone(),
        wireguard_interface: WireGuardInterfaceConfig {
            interface_name: None,
            key_pair,
            listen_port: None,
            mtu: bootstrap
                .network_map
                .as_ref()
                .and_then(|map| map.mtu)
                .map(|mtu| mtu as u16),
            addresses: vec![format!("{}/32", local_virtual_ip)],
            dns_servers: bootstrap
                .network_map
                .as_ref()
                .map(|map| map.dns.servers.clone())
                .unwrap_or_default(),
            peers: vec![],
        },
        wireguard_peer: WireGuardPeerConfig {
            peer_node_id: Some(peer.node_id.clone()),
            public_key: peer.public_key.clone(),
            preshared_key: None,
            endpoint: peer
                .endpoints
                .first()
                .map(|endpoint| endpoint.address.clone()),
            allowed_ips: vec![AllowedIp {
                cidr: format!("{}/32", peer_virtual_ip),
            }],
            persistent_keepalive_seconds: None,
        },
    })
}

pub fn build_tunnel_runtime(config: &TunnelConfig) -> TunnelRuntimeView {
    TunnelRuntimeView {
        state: TunnelState::Configured,
        transport: config.transport.clone(),
        peer_virtual_ip: config.peer_virtual_ip.clone(),
        peer_public_key: config.wireguard_peer.public_key.clone(),
        selected_endpoint: config.wireguard_peer.endpoint.clone(),
        interface_name: config.wireguard_interface.interface_name.clone(),
    }
}
