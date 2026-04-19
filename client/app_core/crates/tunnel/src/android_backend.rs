use crate::{TunnelBackend, TunnelConfig};

/// Android `com.wireguard.android:tunnel` backend 骨架。
pub struct AndroidWireGuardTunnelBackend;

impl AndroidWireGuardTunnelBackend {
    pub fn new() -> Self {
        Self
    }

    fn unsupported() -> Result<(), String> {
        Err("android wireguard tunnel backend not implemented".to_string())
    }
}

impl Default for AndroidWireGuardTunnelBackend {
    fn default() -> Self {
        Self::new()
    }
}

impl TunnelBackend for AndroidWireGuardTunnelBackend {
    fn apply_peer_config(
        &self,
        _peer_virtual_ip: &str,
        _peer: &slan_app_core::WireGuardPeerConfig,
    ) -> Result<(), String> {
        Self::unsupported()
    }

    fn remove_peer(&self, _peer_virtual_ip: &str) -> Result<(), String> {
        Self::unsupported()
    }

    fn establish(&self, _config: &TunnelConfig) -> Result<(), String> {
        Self::unsupported()
    }
}
