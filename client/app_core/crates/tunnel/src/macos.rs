use crate::macos_adapter::MacosWireGuardKitAdapter;
use crate::macos_mapper::MacosWireGuardKitConfigMapper;
use crate::macos_stats::MacosWireGuardKitStatsMapper;
use crate::{TunnelBackend, TunnelBackendDiagnostics, TunnelConfig};

/// macOS WireGuardKit backend 骨架。
///
/// 当前阶段：
/// - 用 mapper 固定 Rust -> WireGuardKit 配置映射
/// - 用 adapter 固定 Rust -> 原生 backend 调用边界
/// - 用 stats mapper 固定原生 runtime -> 统一 stats 模型
///
/// 后续接真实 WireGuardKit 时，只需要替换 adapter 内部实现。
#[derive(Default)]
pub struct MacosWireGuardKitBackend {
    adapter: MacosWireGuardKitAdapter,
}

impl MacosWireGuardKitBackend {
    pub fn new() -> Self {
        Self::default()
    }

    pub fn planned_peer_ips(&self) -> Vec<String> {
        self.adapter.planned_peer_ips()
    }
}

impl TunnelBackend for MacosWireGuardKitBackend {
    fn apply_interface_config(
        &self,
        interface: &slan_app_core::WireGuardInterfaceConfig,
    ) -> Result<(), String> {
        MacosWireGuardKitConfigMapper::validate_interface(interface)?;
        let mapped = MacosWireGuardKitConfigMapper::map_interface(interface);
        let runtime = MacosWireGuardKitStatsMapper::initial_runtime(&mapped);
        self.adapter.apply_interface(runtime)
    }

    fn apply_peer_config(
        &self,
        peer_virtual_ip: &str,
        peer: &slan_app_core::WireGuardPeerConfig,
    ) -> Result<(), String> {
        MacosWireGuardKitConfigMapper::validate_peer(peer)?;
        let mapped = MacosWireGuardKitConfigMapper::map_peer(peer);
        self.adapter.apply_peer(peer_virtual_ip, mapped)
    }

    fn establish(&self, config: &TunnelConfig) -> Result<(), String> {
        MacosWireGuardKitConfigMapper::validate_config(config)?;
        self.apply_interface_config(&config.wireguard_interface)?;
        self.apply_peer_config(&config.peer_virtual_ip, &config.wireguard_peer)
    }

    fn remove_peer(&self, peer_virtual_ip: &str) -> Result<(), String> {
        self.adapter.remove_peer(peer_virtual_ip)
    }

    fn runtime_stats(
        &self,
        peer_virtual_ip: &str,
    ) -> Result<Option<slan_app_core::WireGuardRuntimeStats>, String> {
        let _ = self
            .adapter
            .interface_runtime()
            .ok_or_else(|| "macos adapter interface runtime unavailable".to_string())?;
        let peer_virtual_ip = peer_virtual_ip.to_string();
        let peer_exists = self
            .adapter
            .planned_peer_ips()
            .into_iter()
            .any(|planned| planned == peer_virtual_ip);
        if !peer_exists {
            return Ok(None);
        }
        Ok(Some(MacosWireGuardKitStatsMapper::map_runtime_stats(
            slan_app_core::TunnelTransport::Relay,
            String::new(),
            None,
        )))
    }

    fn diagnostics(&self) -> TunnelBackendDiagnostics {
        let mut diagnostics = TunnelBackendDiagnostics::new("macos-wireguardkit");
        diagnostics.execution_mode = Some("system-extension");
        diagnostics.execution_backend = Some("wireguardkit");
        diagnostics.planned_peer_count = self.planned_peer_ips().len();
        diagnostics
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use slan_app_core::{
        AllowedIp, TunnelTransport, WireGuardInterfaceConfig, WireGuardKeyPair, WireGuardPeerConfig,
    };

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
                dns_servers: vec!["1.1.1.1".into()],
                peers: vec![],
            },
            wireguard_peer: WireGuardPeerConfig {
                peer_node_id: None,
                public_key: "peer-pk".into(),
                preshared_key: None,
                endpoint: Some("203.0.113.10:51820".into()),
                allowed_ips: vec![AllowedIp {
                    cidr: "100.64.0.2/32".into(),
                }],
                persistent_keepalive_seconds: None,
            },
        }
    }

    #[test]
    fn macos_backend_tracks_planned_peers() {
        let backend = MacosWireGuardKitBackend::new();
        backend.establish(&sample_config()).unwrap();

        assert_eq!(backend.planned_peer_ips(), vec!["100.64.0.2"]);
        let diagnostics = <MacosWireGuardKitBackend as TunnelBackend>::diagnostics(&backend);
        assert_eq!(diagnostics.name, "macos-wireguardkit");
        assert_eq!(diagnostics.execution_backend, Some("wireguardkit"));
        assert_eq!(diagnostics.planned_peer_count, 1);
    }

    #[test]
    fn macos_backend_rejects_missing_peer_public_key() {
        let backend = MacosWireGuardKitBackend::new();
        let mut config = sample_config();
        config.wireguard_peer.public_key.clear();

        let err = backend.establish(&config).unwrap_err();

        assert_eq!(err, "macos backend requires non-empty peer public key");
        assert!(backend.planned_peer_ips().is_empty());
    }

    #[test]
    fn macos_backend_rejects_same_local_and_peer_ip() {
        let backend = MacosWireGuardKitBackend::new();
        let mut config = sample_config();
        config.peer_virtual_ip = config.local_virtual_ip.clone();

        let err = backend.establish(&config).unwrap_err();

        assert_eq!(
            err,
            "macos backend requires distinct local and peer virtual ip"
        );
        assert!(backend.planned_peer_ips().is_empty());
    }

    #[test]
    fn macos_backend_rejects_missing_allowed_ips() {
        let backend = MacosWireGuardKitBackend::new();
        let mut config = sample_config();
        config.wireguard_peer.allowed_ips.clear();

        let err = backend.establish(&config).unwrap_err();

        assert_eq!(err, "macos backend requires at least one allowed ip");
        assert!(backend.planned_peer_ips().is_empty());
    }

    #[test]
    fn macos_backend_closes_planned_peer() {
        let backend = MacosWireGuardKitBackend::new();
        backend.establish(&sample_config()).unwrap();

        backend.close("100.64.0.2").unwrap();

        assert!(backend.planned_peer_ips().is_empty());
    }
}
