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
    let local_prefix_len = interface_prefix_len(bootstrap);
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
            addresses: vec![format!("{}/{}", local_virtual_ip, local_prefix_len)],
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

fn interface_prefix_len(bootstrap: &BootstrapConfig) -> u8 {
    let network_id = bootstrap
        .network_map
        .as_ref()
        .map(|map| map.network_id.as_str());
    let cidr = network_id
        .and_then(|target| {
            bootstrap
                .networks
                .iter()
                .find(|network| network.network_id == target)
        })
        .or_else(|| bootstrap.networks.first())
        .map(|network| network.cidr.as_str());
    cidr.and_then(prefix_len_from_cidr).unwrap_or(32)
}

fn prefix_len_from_cidr(cidr: &str) -> Option<u8> {
    let (_, prefix) = cidr.trim().rsplit_once('/')?;
    let parsed = prefix.trim().parse::<u8>().ok()?;
    (parsed <= 32).then_some(parsed)
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

#[cfg(test)]
mod tests {
    use super::*;
    use slan_app_core::{
        ControlPlaneConfig, Device, DnsConfig, Network, NetworkMap, Peer, RelayConfig,
        WireGuardKeyPair,
    };

    #[test]
    fn build_tunnel_config_uses_network_cidr_prefix_for_interface_address() {
        let bootstrap = BootstrapConfig {
            device: Device {
                device_id: "dev-1".into(),
                name: "device".into(),
                platform: "windows".into(),
                status: "active".into(),
                virtual_ip: Some("10.0.0.2".into()),
                public_key: Some("pub".into()),
                mqtt: None,
            },
            networks: vec![Network {
                network_id: "net-1".into(),
                name: "default".into(),
                description: None,
                cidr: "10.0.0.0/16".into(),
                default_subnet_id: None,
                subnets: vec![],
                members: vec![],
            }],
            control_plane: ControlPlaneConfig {
                ws_url: "mqtt://127.0.0.1:1883".into(),
                session_token: None,
                heartbeat_seconds: 15,
            },
            stun_servers: vec![],
            relay: RelayConfig {
                default_cluster_id: "local".into(),
                countries: vec![],
            },
            derp_map: None,
            network_map: Some(NetworkMap {
                self_user_id: "user-1".into(),
                self_device_id: "dev-1".into(),
                self_node_id: "node-1".into(),
                network_id: "net-1".into(),
                revision: 1,
                heartbeat_seconds: 15,
                stun_servers: vec![],
                peers: vec![],
                routes: vec![],
                relay_regions: vec![],
                dns: DnsConfig {
                    servers: vec![],
                    search_domains: vec![],
                },
                mtu: Some(1280),
            }),
        };
        let peer = Peer {
            node_id: "peer-1".into(),
            device_id: "peer-dev-1".into(),
            public_key: "peer-pub".into(),
            status: "online".into(),
            relay_allowed: true,
            virtual_ips: vec!["10.0.0.3".into()],
            endpoints: vec![],
            allowed_routes: vec![],
        };

        let config = build_tunnel_config(
            &ActivePath::Relay {
                peer_node_id: "peer-1".into(),
            },
            &bootstrap,
            &peer,
            WireGuardKeyPair {
                public_key: "pub".into(),
                private_key: "priv".into(),
            },
        )
        .expect("tunnel config");

        assert_eq!(config.wireguard_interface.addresses, vec!["10.0.0.2/16"]);
    }
}
