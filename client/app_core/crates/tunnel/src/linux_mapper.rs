use slan_app_core::{WireGuardInterfaceConfig, WireGuardPeerConfig};

use crate::TunnelConfig;

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct LinuxKernelInterfacePlan {
    pub interface_name: Option<String>,
    pub private_key: String,
    pub listen_port: Option<u16>,
    pub mtu: Option<u16>,
    pub addresses: Vec<String>,
    pub dns_servers: Vec<String>,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct LinuxKernelPeerPlan {
    pub peer_node_id: Option<String>,
    pub public_key: String,
    pub preshared_key: Option<String>,
    pub endpoint: Option<String>,
    pub allowed_ips: Vec<String>,
    pub persistent_keepalive_seconds: Option<u16>,
}

pub struct LinuxKernelWireGuardConfigMapper;

impl LinuxKernelWireGuardConfigMapper {
    pub fn validate_config(config: &TunnelConfig) -> Result<(), String> {
        Self::validate_interface(&config.wireguard_interface)?;
        Self::validate_peer(&config.peer_virtual_ip, &config.wireguard_peer)?;
        if config.local_virtual_ip.trim().is_empty() {
            return Err("linux backend requires non-empty local virtual ip".to_string());
        }
        if config.peer_virtual_ip.trim().is_empty() {
            return Err("linux backend requires non-empty peer virtual ip".to_string());
        }
        if config.local_virtual_ip == config.peer_virtual_ip {
            return Err("linux backend requires distinct local and peer virtual ip".to_string());
        }
        Ok(())
    }

    pub fn validate_interface(interface: &WireGuardInterfaceConfig) -> Result<(), String> {
        if interface.key_pair.public_key.trim().is_empty() {
            return Err("linux backend requires non-empty interface public key".to_string());
        }
        if interface.key_pair.private_key.trim().is_empty() {
            return Err("linux backend requires non-empty interface private key".to_string());
        }
        if interface.addresses.is_empty() {
            return Err("linux backend requires at least one interface address".to_string());
        }
        Ok(())
    }

    pub fn validate_peer(
        peer_virtual_ip: &str,
        peer: &WireGuardPeerConfig,
    ) -> Result<(), String> {
        if peer_virtual_ip.trim().is_empty() {
            return Err("linux backend requires non-empty peer virtual ip".to_string());
        }
        if peer.public_key.trim().is_empty() {
            return Err("linux backend requires non-empty peer public key".to_string());
        }
        if peer.allowed_ips.is_empty() {
            return Err("linux backend requires at least one allowed ip".to_string());
        }
        Ok(())
    }

    pub fn map_interface(interface: &WireGuardInterfaceConfig) -> LinuxKernelInterfacePlan {
        LinuxKernelInterfacePlan {
            interface_name: interface.interface_name.clone(),
            private_key: interface.key_pair.private_key.clone(),
            listen_port: interface.listen_port,
            mtu: interface.mtu,
            addresses: interface.addresses.clone(),
            dns_servers: interface.dns_servers.clone(),
        }
    }

    pub fn map_peer(peer: &WireGuardPeerConfig) -> LinuxKernelPeerPlan {
        LinuxKernelPeerPlan {
            peer_node_id: peer.peer_node_id.clone(),
            public_key: peer.public_key.clone(),
            preshared_key: peer.preshared_key.clone(),
            endpoint: peer.endpoint.clone(),
            allowed_ips: peer.allowed_ips.iter().map(|ip| ip.cidr.clone()).collect(),
            persistent_keepalive_seconds: peer.persistent_keepalive_seconds,
        }
    }
}
