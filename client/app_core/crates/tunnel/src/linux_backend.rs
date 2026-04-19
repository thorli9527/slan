use crate::linux_adapter::{
    LinuxDriverDiagnostics, LinuxExecutionBackend, LinuxExecutionMode, LinuxKernelCommandExecutor,
    LinuxKernelWireGuardAdapter,
};
use crate::linux_mapper::LinuxKernelWireGuardConfigMapper;
use crate::linux_stats::LinuxKernelWireGuardStatsMapper;
use crate::{TunnelBackend, TunnelConfig};

/// Linux kernel WireGuard backend 骨架。
///
/// 当前阶段：
/// - 用 mapper 固定 Rust -> Linux kernel WireGuard 配置映射
/// - 用 adapter 固定 Rust -> Linux tunnel driver 调用边界
/// - 用 stats mapper 固定 Linux runtime -> 统一 stats 模型
///
/// 后续接真实 netlink / wgctrl 时，只需要替换 adapter 内部实现。
#[derive(Default)]
pub struct LinuxKernelWireGuardBackend {
    adapter: LinuxKernelWireGuardAdapter,
}

impl LinuxKernelWireGuardBackend {
    pub fn new() -> Self {
        Self::default()
    }

    pub fn new_with_executor(executor: Box<dyn LinuxKernelCommandExecutor>) -> Self {
        Self {
            adapter: LinuxKernelWireGuardAdapter::new_with_executor(
                executor,
                LinuxExecutionMode::DryRun,
                LinuxExecutionBackend::Shell,
            ),
        }
    }

    pub fn planned_peer_ips(&self) -> Vec<String> {
        self.adapter.planned_peer_ips()
    }

    pub fn interface_name(&self) -> Option<String> {
        self.adapter
            .interface_runtime()
            .and_then(|runtime| runtime.interface_name)
    }

    pub fn is_up(&self) -> bool {
        self.adapter
            .interface_runtime()
            .map(|runtime| runtime.is_up)
            .unwrap_or(false)
    }

    pub fn diagnostics(&self) -> LinuxDriverDiagnostics {
        self.adapter.diagnostics()
    }

    pub fn recent_commands(&self) -> Vec<crate::linux_adapter::LinuxCommandSpec> {
        self.adapter.recent_commands()
    }
}

impl TunnelBackend for LinuxKernelWireGuardBackend {
    fn apply_interface_config(
        &self,
        interface: &slan_app_core::WireGuardInterfaceConfig,
    ) -> Result<(), String> {
        LinuxKernelWireGuardConfigMapper::validate_interface(interface)?;
        let mapped = LinuxKernelWireGuardConfigMapper::map_interface(interface);
        let runtime = LinuxKernelWireGuardStatsMapper::initial_runtime(&mapped);
        self.adapter.apply_interface(mapped, runtime)
    }

    fn apply_peer_config(
        &self,
        peer_virtual_ip: &str,
        peer: &slan_app_core::WireGuardPeerConfig,
    ) -> Result<(), String> {
        LinuxKernelWireGuardConfigMapper::validate_peer(peer_virtual_ip, peer)?;
        let mapped = LinuxKernelWireGuardConfigMapper::map_peer(peer);
        self.adapter.apply_peer(peer_virtual_ip, mapped)
    }

    fn bring_up(&self) -> Result<(), String> {
        self.adapter.bring_up()
    }

    fn bring_down(&self) -> Result<(), String> {
        self.adapter.bring_down()
    }

    fn establish(&self, config: &TunnelConfig) -> Result<(), String> {
        LinuxKernelWireGuardConfigMapper::validate_config(config)?;
        self.apply_interface_config(&config.wireguard_interface)?;
        self.apply_peer_config(&config.peer_virtual_ip, &config.wireguard_peer)?;
        self.bring_up()
    }

    fn remove_peer(&self, peer_virtual_ip: &str) -> Result<(), String> {
        self.adapter.remove_peer(peer_virtual_ip)
    }

    fn runtime_stats(
        &self,
        peer_virtual_ip: &str,
    ) -> Result<Option<slan_app_core::WireGuardRuntimeStats>, String> {
        let interface = self
            .adapter
            .interface_runtime()
            .ok_or_else(|| "linux adapter interface runtime unavailable".to_string())?;
        let Some(peer_runtime) = self.adapter.peer_runtime(peer_virtual_ip) else {
            return Ok(None);
        };
        let transport = if interface.is_up {
            slan_app_core::TunnelTransport::P2P
        } else {
            slan_app_core::TunnelTransport::Relay
        };
        Ok(Some(LinuxKernelWireGuardStatsMapper::map_runtime_stats(
            transport,
            &peer_runtime.peer,
        )))
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::linux_adapter::{LinuxCommandSpec, LinuxKernelCommandExecutor};
    use slan_app_core::{
        AllowedIp, TunnelTransport, WireGuardInterfaceConfig, WireGuardKeyPair,
        WireGuardPeerConfig,
    };
    use std::sync::Mutex;

    #[derive(Default)]
    struct RecordingLinuxCommandExecutor {
        commands: Mutex<Vec<LinuxCommandSpec>>,
    }

    impl LinuxKernelCommandExecutor for RecordingLinuxCommandExecutor {
        fn run(&self, spec: &LinuxCommandSpec) -> Result<(), String> {
            self.commands
                .lock()
                .map_err(|_| "recording linux command executor poisoned".to_string())?
                .push(spec.clone());
            Ok(())
        }
    }

    fn test_backend() -> LinuxKernelWireGuardBackend {
        LinuxKernelWireGuardBackend::new_with_executor(Box::new(
            RecordingLinuxCommandExecutor::default(),
        ))
    }

    fn sample_config() -> TunnelConfig {
        TunnelConfig {
            transport: TunnelTransport::Relay,
            local_virtual_ip: "100.64.0.10".into(),
            peer_virtual_ip: "100.64.0.2".into(),
            wireguard_interface: WireGuardInterfaceConfig {
                interface_name: Some("wg0".into()),
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
                peer_node_id: Some("peer-1".into()),
                public_key: "peer-pk".into(),
                preshared_key: None,
                endpoint: Some("198.51.100.10:51820".into()),
                allowed_ips: vec![AllowedIp {
                    cidr: "100.64.0.2/32".into(),
                }],
                persistent_keepalive_seconds: Some(15),
            },
        }
    }

    #[test]
    fn linux_backend_tracks_established_peer_and_interface() {
        let backend = test_backend();
        backend.establish(&sample_config()).unwrap();

        assert_eq!(backend.planned_peer_ips(), vec!["100.64.0.2"]);
        assert_eq!(backend.interface_name().as_deref(), Some("wg0"));
        assert!(backend.is_up());
        assert_eq!(backend.diagnostics().execution_mode, LinuxExecutionMode::DryRun);
        assert_eq!(
            backend.diagnostics().execution_backend,
            LinuxExecutionBackend::Shell
        );
        assert!(!backend.recent_commands().is_empty());
    }

    #[test]
    fn linux_backend_reports_runtime_stats() {
        let backend = test_backend();
        backend.establish(&sample_config()).unwrap();

        let runtime = backend.runtime_stats("100.64.0.2").unwrap().unwrap();

        assert_eq!(runtime.transport, TunnelTransport::P2P);
        assert_eq!(runtime.selected_endpoint.as_deref(), Some("198.51.100.10:51820"));
    }

    #[test]
    fn linux_backend_rejects_invalid_config() {
        let backend = test_backend();
        let mut config = sample_config();
        config.wireguard_peer.public_key.clear();

        let err = backend.establish(&config).unwrap_err();

        assert_eq!(err, "linux backend requires non-empty peer public key");
        assert!(backend.planned_peer_ips().is_empty());
    }

    #[test]
    fn linux_backend_can_bring_down_and_remove_peer() {
        let backend = test_backend();
        backend.establish(&sample_config()).unwrap();

        backend.bring_down().unwrap();
        backend.close("100.64.0.2").unwrap();

        assert!(!backend.is_up());
        assert!(backend.planned_peer_ips().is_empty());
    }
}
