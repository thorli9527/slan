use slan_app_core::{TunnelTransport, WireGuardRuntimeStats};

use crate::linux_mapper::{LinuxKernelInterfacePlan, LinuxKernelPeerPlan};

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct LinuxKernelRuntime {
    pub interface_name: Option<String>,
    pub addresses: Vec<String>,
    pub dns_servers: Vec<String>,
    pub is_up: bool,
}

pub struct LinuxKernelWireGuardStatsMapper;

impl LinuxKernelWireGuardStatsMapper {
    pub fn initial_runtime(interface: &LinuxKernelInterfacePlan) -> LinuxKernelRuntime {
        LinuxKernelRuntime {
            interface_name: interface.interface_name.clone(),
            addresses: interface.addresses.clone(),
            dns_servers: interface.dns_servers.clone(),
            is_up: false,
        }
    }

    pub fn map_runtime_stats(
        transport: TunnelTransport,
        peer: &LinuxKernelPeerPlan,
    ) -> WireGuardRuntimeStats {
        WireGuardRuntimeStats {
            transport,
            peer_public_key: peer.public_key.clone(),
            selected_endpoint: peer.endpoint.clone(),
            latest_handshake_at_ms: None,
            bytes_received: 0,
            bytes_sent: 0,
        }
    }
}
