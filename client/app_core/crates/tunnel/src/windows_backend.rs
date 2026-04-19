use crate::{TunnelBackend, TunnelConfig};

/// Windows embeddable-dll-service backend 骨架。
pub struct WindowsEmbeddableServiceBackend;

impl WindowsEmbeddableServiceBackend {
    pub fn new() -> Self {
        Self
    }

    fn unsupported() -> Result<(), String> {
        Err("windows embeddable wireguard backend not implemented".to_string())
    }
}

impl Default for WindowsEmbeddableServiceBackend {
    fn default() -> Self {
        Self::new()
    }
}

impl TunnelBackend for WindowsEmbeddableServiceBackend {
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
