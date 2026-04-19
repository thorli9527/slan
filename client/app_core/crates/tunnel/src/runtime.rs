use crate::{InMemoryTunnelBackend, TunnelBackend, TunnelConfig, TunnelManager};

/// 基于底层 tunnel backend 的通用 manager。
pub struct SystemTunnelManager<B> {
    backend: B,
}

impl<B> SystemTunnelManager<B> {
    pub fn new(backend: B) -> Self {
        Self { backend }
    }

    pub fn backend(&self) -> &B {
        &self.backend
    }
}

impl<B> TunnelManager for SystemTunnelManager<B>
where
    B: TunnelBackend,
{
    fn establish(&self, config: &TunnelConfig) -> Result<(), String> {
        self.backend.establish(config)
    }

    fn close(&self, peer_virtual_ip: &str) -> Result<(), String> {
        self.backend.close(peer_virtual_ip)
    }
}

/// 当前默认的内存 manager，保持既有外部接线不变。
pub struct InMemoryTunnelManager {
    inner: SystemTunnelManager<InMemoryTunnelBackend>,
}

impl Default for InMemoryTunnelManager {
    fn default() -> Self {
        Self {
            inner: SystemTunnelManager::new(InMemoryTunnelBackend::default()),
        }
    }
}

impl InMemoryTunnelManager {
    pub fn established_peer_ips(&self) -> Vec<String> {
        self.inner.backend().established_peer_ips()
    }
}

impl TunnelManager for InMemoryTunnelManager {
    fn establish(&self, config: &TunnelConfig) -> Result<(), String> {
        self.inner.establish(config)
    }

    fn close(&self, peer_virtual_ip: &str) -> Result<(), String> {
        self.inner.close(peer_virtual_ip)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use slan_app_core::{
        AllowedIp, TunnelTransport, WireGuardInterfaceConfig, WireGuardKeyPair,
        WireGuardPeerConfig,
    };
    use std::sync::Mutex;

    fn sample_config() -> TunnelConfig {
        TunnelConfig {
            transport: TunnelTransport::Relay,
            local_virtual_ip: "100.64.0.10".into(),
            peer_virtual_ip: "100.64.0.2".into(),
            wireguard_interface: WireGuardInterfaceConfig {
                interface_name: Some("utun9".into()),
                key_pair: WireGuardKeyPair {
                    public_key: "self-pk".into(),
                    private_key: "self-sk".into(),
                },
                listen_port: Some(51820),
                mtu: Some(1280),
                addresses: vec!["100.64.0.10/32".into()],
                dns_servers: vec![],
                peers: vec![],
            },
            wireguard_peer: WireGuardPeerConfig {
                peer_node_id: None,
                public_key: "peer-pk".into(),
                preshared_key: None,
                endpoint: None,
                allowed_ips: vec![AllowedIp {
                    cidr: "100.64.0.2/32".into(),
                }],
                persistent_keepalive_seconds: None,
            },
        }
    }

    struct RecordingTunnelBackend {
        established: Mutex<Vec<String>>,
        closed: Mutex<Vec<String>>,
    }

    impl RecordingTunnelBackend {
        fn new() -> Self {
            Self {
                established: Mutex::new(Vec::new()),
                closed: Mutex::new(Vec::new()),
            }
        }
    }

    impl TunnelBackend for RecordingTunnelBackend {
        fn apply_peer_config(
            &self,
            peer_virtual_ip: &str,
            _peer: &WireGuardPeerConfig,
        ) -> Result<(), String> {
            self.established
                .lock()
                .unwrap()
                .push(peer_virtual_ip.to_string());
            Ok(())
        }

        fn remove_peer(&self, peer_virtual_ip: &str) -> Result<(), String> {
            self.closed.lock().unwrap().push(peer_virtual_ip.to_string());
            Ok(())
        }
    }

    #[test]
    fn system_tunnel_manager_delegates_to_backend() {
        let manager = SystemTunnelManager::new(RecordingTunnelBackend::new());
        let config = sample_config();

        manager.establish(&config).unwrap();
        manager.close("100.64.0.2").unwrap();

        assert_eq!(
            manager.backend().established.lock().unwrap().clone(),
            vec!["100.64.0.2"]
        );
        assert_eq!(
            manager.backend().closed.lock().unwrap().clone(),
            vec!["100.64.0.2"]
        );
    }

    #[test]
    fn tunnel_manager_tracks_established_peers() {
        let manager = InMemoryTunnelManager::default();
        manager.establish(&sample_config()).unwrap();

        assert_eq!(manager.established_peer_ips(), vec!["100.64.0.2"]);
    }

    #[test]
    fn tunnel_manager_closes_peer() {
        let manager = InMemoryTunnelManager::default();
        manager.establish(&sample_config()).unwrap();
        manager.close("100.64.0.2").unwrap();

        assert!(manager.established_peer_ips().is_empty());
    }
}
