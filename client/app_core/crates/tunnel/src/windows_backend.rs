use std::collections::HashMap;
use std::sync::Mutex;
use std::time::{SystemTime, UNIX_EPOCH};

use slan_app_core::{
    TunnelTransport, WireGuardInterfaceConfig, WireGuardPeerConfig, WireGuardRuntimeStats,
};

use crate::{TunnelBackend, TunnelConfig};

#[derive(Debug, Clone)]
struct WindowsInterfaceRuntime {
    interface_name: String,
    local_virtual_ip: String,
    is_up: bool,
}

#[derive(Debug, Clone)]
struct WindowsPeerRuntime {
    peer: WireGuardPeerConfig,
    latest_handshake_at_ms: Option<u64>,
}

/// Windows embeddable tunnel backend。
///
/// 当前阶段先补齐一个可工作的本地 backend：
/// - 完整保存 interface / peer 运行态
/// - 支持 bring up / bring down / runtime stats
/// - 保持 dry-run 语义，不依赖 Wintun/WireGuardNT
///
/// 后续接入真实 Wintun 时，只需要替换内部状态同步与驱动调用。
#[derive(Default)]
pub struct WindowsEmbeddableServiceBackend {
    interface: Mutex<Option<WindowsInterfaceRuntime>>,
    peers: Mutex<HashMap<String, WindowsPeerRuntime>>,
}

impl WindowsEmbeddableServiceBackend {
    pub fn new() -> Self {
        Self::default()
    }

    pub fn planned_peer_ips(&self) -> Vec<String> {
        self.peers
            .lock()
            .map(|peers| peers.keys().cloned().collect())
            .unwrap_or_default()
    }

    pub fn interface_name(&self) -> Option<String> {
        self.interface
            .lock()
            .ok()
            .and_then(|state| state.as_ref().map(|runtime| runtime.interface_name.clone()))
    }

    pub fn is_up(&self) -> bool {
        self.interface
            .lock()
            .ok()
            .and_then(|state| state.as_ref().map(|runtime| runtime.is_up))
            .unwrap_or(false)
    }

    fn validate_interface(interface: &WireGuardInterfaceConfig) -> Result<(), String> {
        if interface
            .key_pair
            .public_key
            .trim()
            .is_empty()
        {
            return Err("windows backend requires non-empty wireguard public key".to_string());
        }
        if interface
            .key_pair
            .private_key
            .trim()
            .is_empty()
        {
            return Err("windows backend requires non-empty wireguard private key".to_string());
        }
        if interface.addresses.is_empty() {
            return Err("windows backend requires at least one wireguard interface address".to_string());
        }
        Ok(())
    }

    fn validate_peer(peer_virtual_ip: &str, peer: &WireGuardPeerConfig) -> Result<(), String> {
        if peer_virtual_ip.trim().is_empty() {
            return Err("windows backend requires non-empty peer virtual ip".to_string());
        }
        if peer.public_key.trim().is_empty() {
            return Err("windows backend requires non-empty peer public key".to_string());
        }
        if peer.allowed_ips.is_empty() {
            return Err("windows backend requires at least one allowed ip".to_string());
        }
        Ok(())
    }

    fn validate_config(config: &TunnelConfig) -> Result<(), String> {
        if config.local_virtual_ip.trim().is_empty() {
            return Err("windows backend requires non-empty local virtual ip".to_string());
        }
        if config.peer_virtual_ip.trim().is_empty() {
            return Err("windows backend requires non-empty peer virtual ip".to_string());
        }
        if config.local_virtual_ip == config.peer_virtual_ip {
            return Err("windows backend requires distinct local and peer virtual ip".to_string());
        }
        Self::validate_interface(&config.wireguard_interface)?;
        Self::validate_peer(&config.peer_virtual_ip, &config.wireguard_peer)
    }

    fn current_timestamp_ms() -> u64 {
        SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .map(|duration| duration.as_millis() as u64)
            .unwrap_or_default()
    }
}

impl TunnelBackend for WindowsEmbeddableServiceBackend {
    fn apply_interface_config(&self, interface: &WireGuardInterfaceConfig) -> Result<(), String> {
        Self::validate_interface(interface)?;
        let interface_name = interface
            .interface_name
            .clone()
            .filter(|value| !value.trim().is_empty())
            .unwrap_or_else(|| "slan0".to_string());
        let local_virtual_ip = interface
            .addresses
            .first()
            .map(|address| address.split('/').next().unwrap_or_default().to_string())
            .filter(|value| !value.trim().is_empty())
            .ok_or_else(|| "windows backend requires at least one wireguard interface address".to_string())?;
        let mut state = self
            .interface
            .lock()
            .map_err(|_| "windows backend interface state poisoned".to_string())?;
        *state = Some(WindowsInterfaceRuntime {
            interface_name,
            local_virtual_ip,
            is_up: false,
        });
        Ok(())
    }

    fn apply_peer_config(
        &self,
        peer_virtual_ip: &str,
        peer: &WireGuardPeerConfig,
    ) -> Result<(), String> {
        Self::validate_peer(peer_virtual_ip, peer)?;
        let mut peers = self
            .peers
            .lock()
            .map_err(|_| "windows backend peer state poisoned".to_string())?;
        peers.insert(
            peer_virtual_ip.to_string(),
            WindowsPeerRuntime {
                peer: peer.clone(),
                latest_handshake_at_ms: None,
            },
        );
        Ok(())
    }

    fn bring_up(&self) -> Result<(), String> {
        let mut interface = self
            .interface
            .lock()
            .map_err(|_| "windows backend interface state poisoned".to_string())?;
        let Some(runtime) = interface.as_mut() else {
            return Err("windows backend interface runtime unavailable".to_string());
        };
        runtime.is_up = true;
        drop(interface);

        let handshake_at_ms = Self::current_timestamp_ms();
        let mut peers = self
            .peers
            .lock()
            .map_err(|_| "windows backend peer state poisoned".to_string())?;
        for peer in peers.values_mut() {
            peer.latest_handshake_at_ms = Some(handshake_at_ms);
        }
        Ok(())
    }

    fn bring_down(&self) -> Result<(), String> {
        let mut interface = self
            .interface
            .lock()
            .map_err(|_| "windows backend interface state poisoned".to_string())?;
        let Some(runtime) = interface.as_mut() else {
            return Err("windows backend interface runtime unavailable".to_string());
        };
        runtime.is_up = false;
        Ok(())
    }

    fn establish(&self, config: &TunnelConfig) -> Result<(), String> {
        Self::validate_config(config)?;
        self.apply_interface_config(&config.wireguard_interface)?;
        self.apply_peer_config(&config.peer_virtual_ip, &config.wireguard_peer)?;
        self.bring_up()
    }

    fn remove_peer(&self, peer_virtual_ip: &str) -> Result<(), String> {
        let mut peers = self
            .peers
            .lock()
            .map_err(|_| "windows backend peer state poisoned".to_string())?;
        peers.remove(peer_virtual_ip);
        Ok(())
    }

    fn runtime_stats(
        &self,
        peer_virtual_ip: &str,
    ) -> Result<Option<WireGuardRuntimeStats>, String> {
        let interface = self
            .interface
            .lock()
            .map_err(|_| "windows backend interface state poisoned".to_string())?;
        let Some(interface_runtime) = interface.as_ref() else {
            return Err("windows backend interface runtime unavailable".to_string());
        };
        let _ = &interface_runtime.local_virtual_ip;
        let peers = self
            .peers
            .lock()
            .map_err(|_| "windows backend peer state poisoned".to_string())?;
        let Some(peer_runtime) = peers.get(peer_virtual_ip) else {
            return Ok(None);
        };
        Ok(Some(WireGuardRuntimeStats {
            transport: if interface_runtime.is_up {
                TunnelTransport::P2P
            } else {
                TunnelTransport::Relay
            },
            peer_public_key: peer_runtime.peer.public_key.clone(),
            selected_endpoint: peer_runtime.peer.endpoint.clone(),
            latest_handshake_at_ms: peer_runtime.latest_handshake_at_ms,
            bytes_received: if interface_runtime.is_up { 1 } else { 0 },
            bytes_sent: if interface_runtime.is_up { 1 } else { 0 },
        }))
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use slan_app_core::{
        AllowedIp, TunnelTransport, WireGuardInterfaceConfig, WireGuardKeyPair,
        WireGuardPeerConfig,
    };

    fn sample_config() -> TunnelConfig {
        TunnelConfig {
            transport: TunnelTransport::Relay,
            local_virtual_ip: "10.12.0.2".into(),
            peer_virtual_ip: "10.12.0.3".into(),
            wireguard_interface: WireGuardInterfaceConfig {
                interface_name: Some("slan0".into()),
                key_pair: WireGuardKeyPair {
                    public_key: "self-pk".into(),
                    private_key: "self-sk".into(),
                },
                listen_port: Some(51820),
                mtu: Some(1280),
                addresses: vec!["10.12.0.2/32".into()],
                dns_servers: vec![],
                peers: vec![],
            },
            wireguard_peer: WireGuardPeerConfig {
                peer_node_id: Some("peer-1".into()),
                public_key: "peer-pk".into(),
                preshared_key: None,
                endpoint: Some("198.51.100.20:51820".into()),
                allowed_ips: vec![AllowedIp {
                    cidr: "10.12.0.3/32".into(),
                }],
                persistent_keepalive_seconds: Some(15),
            },
        }
    }

    #[test]
    fn windows_backend_tracks_established_peer_and_interface() {
        let backend = WindowsEmbeddableServiceBackend::new();
        backend.establish(&sample_config()).unwrap();

        assert_eq!(backend.planned_peer_ips(), vec!["10.12.0.3"]);
        assert_eq!(backend.interface_name().as_deref(), Some("slan0"));
        assert!(backend.is_up());
    }

    #[test]
    fn windows_backend_reports_runtime_stats() {
        let backend = WindowsEmbeddableServiceBackend::new();
        backend.establish(&sample_config()).unwrap();

        let runtime = backend.runtime_stats("10.12.0.3").unwrap().unwrap();

        assert_eq!(runtime.transport, TunnelTransport::P2P);
        assert_eq!(runtime.selected_endpoint.as_deref(), Some("198.51.100.20:51820"));
        assert_eq!(runtime.peer_public_key, "peer-pk");
    }

    #[test]
    fn windows_backend_rejects_invalid_config() {
        let backend = WindowsEmbeddableServiceBackend::new();
        let mut config = sample_config();
        config.wireguard_peer.public_key.clear();

        let err = backend.establish(&config).unwrap_err();

        assert_eq!(err, "windows backend requires non-empty peer public key");
        assert!(backend.planned_peer_ips().is_empty());
    }

    #[test]
    fn windows_backend_can_bring_down_and_remove_peer() {
        let backend = WindowsEmbeddableServiceBackend::new();
        backend.establish(&sample_config()).unwrap();

        backend.bring_down().unwrap();
        backend.close("10.12.0.3").unwrap();

        assert!(!backend.is_up());
        assert!(backend.planned_peer_ips().is_empty());
    }
}
