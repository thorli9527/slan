use slan_app_core::{TunnelTransport, WireGuardRuntimeStats};

use crate::macos_mapper::MacosWireGuardKitInterfacePlan;

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct MacosWireGuardKitRuntime {
    pub interface_name: Option<String>,
    pub addresses: Vec<String>,
    pub dns_servers: Vec<String>,
}

pub struct MacosWireGuardKitStatsMapper;

impl MacosWireGuardKitStatsMapper {
    pub fn initial_runtime(interface: &MacosWireGuardKitInterfacePlan) -> MacosWireGuardKitRuntime {
        MacosWireGuardKitRuntime {
            interface_name: interface.interface_name.clone(),
            addresses: interface.addresses.clone(),
            dns_servers: interface.dns_servers.clone(),
        }
    }

    pub fn map_runtime_stats(
        transport: TunnelTransport,
        peer_public_key: String,
        selected_endpoint: Option<String>,
    ) -> WireGuardRuntimeStats {
        WireGuardRuntimeStats {
            transport,
            peer_public_key,
            selected_endpoint,
            latest_handshake_at_ms: None,
            bytes_received: 0,
            bytes_sent: 0,
        }
    }
}
