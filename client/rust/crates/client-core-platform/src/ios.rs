//! iOS platform bridge for SLAN mesh networking.
//!
//! iOS owns the packet data plane in the NetworkExtension target. This Rust
//! bridge mirrors Android's control-plane cache so service status, diagnostics,
//! and selected peer paths remain available to the shared client core.

use std::sync::{Mutex, OnceLock};

use anyhow::{anyhow, Result};
use client_core::{
    NetworkRuntimeState, PeerPathRuntime, PlatformDiagnosticCheck, PlatformNetwork,
    PlatformNetworkDiagnostics, PlatformResolverConfig, RelayDataPlaneConfig, RouteSpec,
};

const HOST_INTERFACE_PREFIX_LEN: u8 = 32;

/// IosPlatformNetwork 是 iOS 的 PlatformNetwork 实现。
///
/// 真正的包数据面由 NetworkExtension PacketTunnel 目标承载；这里缓存控制面
/// 下发的网络配置，并向共享核心返回运行状态和诊断信息。
#[derive(Debug, Clone, Default)]
pub struct IosPlatformNetwork;

/// IosCachedNetworkConfig 是主 App/Rust 核心侧缓存的 iOS 网络配置摘要。
#[derive(Debug, Clone, Default)]
struct IosCachedNetworkConfig {
    installed: bool,
    virtual_ip: Option<String>,
    prefix_len: Option<u8>,
    resolver_servers: Vec<String>,
    resolver_search_domains: Vec<String>,
    resolver_split_domains: Vec<String>,
    routes: Vec<RouteSpec>,
    relay_config: Option<RelayDataPlaneConfig>,
}

static IOS_CONFIG: OnceLock<Mutex<IosCachedNetworkConfig>> = OnceLock::new();

fn cached_config() -> &'static Mutex<IosCachedNetworkConfig> {
    IOS_CONFIG.get_or_init(|| Mutex::new(IosCachedNetworkConfig::default()))
}

impl PlatformNetwork for IosPlatformNetwork {
    fn install_adapter(&self) -> Result<()> {
        let mut config = cached_config()
            .lock()
            .map_err(|_| anyhow!("ios network config lock poisoned"))?;
        config.installed = true;
        Ok(())
    }

    fn configure_ip(&self, virtual_ip: &str, prefix_len: u8) -> Result<()> {
        let value = virtual_ip
            .trim()
            .split_once('/')
            .map_or(virtual_ip.trim(), |(ip, _)| ip.trim());
        if value.is_empty() {
            anyhow::bail!("ios virtual IP is empty");
        }
        let mut config = cached_config()
            .lock()
            .map_err(|_| anyhow!("ios network config lock poisoned"))?;
        config.virtual_ip = Some(value.to_string());
        config.prefix_len = Some(if prefix_len == 0 {
            HOST_INTERFACE_PREFIX_LEN
        } else {
            prefix_len
        });
        Ok(())
    }

    fn configure_routes(&self, routes: &[RouteSpec]) -> Result<()> {
        let mut config = cached_config()
            .lock()
            .map_err(|_| anyhow!("ios network config lock poisoned"))?;
        config.routes = routes.to_vec();
        Ok(())
    }

    fn configure_resolver(&self, resolver: &PlatformResolverConfig) -> Result<()> {
        let mut config = cached_config()
            .lock()
            .map_err(|_| anyhow!("ios network config lock poisoned"))?;
        config.resolver_servers = resolver
            .servers
            .iter()
            .map(|value| value.trim())
            .filter(|value| !value.is_empty())
            .map(str::to_string)
            .collect();
        config.resolver_search_domains = resolver.search_domains.clone();
        config.resolver_split_domains = resolver.split_domains.clone();
        Ok(())
    }

    fn configure_relay(&self, relay_config: Option<&RelayDataPlaneConfig>) -> Result<()> {
        let mut config = cached_config()
            .lock()
            .map_err(|_| anyhow!("ios network config lock poisoned"))?;
        config.relay_config = relay_config.cloned();
        Ok(())
    }

    fn disable_network(&self) -> Result<()> {
        let mut config = cached_config()
            .lock()
            .map_err(|_| anyhow!("ios network config lock poisoned"))?;
        *config = IosCachedNetworkConfig::default();
        Ok(())
    }

    fn read_runtime_state(&self) -> Result<NetworkRuntimeState> {
        let config = cached_config()
            .lock()
            .map_err(|_| anyhow!("ios network config lock poisoned"))?;
        Ok(NetworkRuntimeState {
            adapter_present: config.installed,
            network_enabled: config.installed && config.virtual_ip.is_some(),
            virtual_ip: config.virtual_ip.clone(),
            active_path: config
                .relay_config
                .as_ref()
                .filter(|relay| relay.enabled && !relay.sessions.is_empty())
                .and_then(|relay| client_core::relay_path_kind_for_transport(&relay.transport)),
            peer_paths: config
                .relay_config
                .as_ref()
                .map(|relay| {
                    let peer_paths = relay
                        .peer_paths
                        .iter()
                        .map(|path| PeerPathRuntime {
                            peer_node_id: path.peer_node_id.clone(),
                            peer_virtual_ips: path.peer_virtual_ips.clone(),
                            active_path: None,
                            candidates: path.candidates.clone(),
                        })
                        .collect::<Vec<_>>();
                    client_core::selected_runtime_paths(&relay.path_policy, peer_paths)
                })
                .unwrap_or_default(),
        })
    }

    fn diagnostics(&self) -> Result<PlatformNetworkDiagnostics> {
        let config = cached_config()
            .lock()
            .map_err(|_| anyhow!("ios network config lock poisoned"))?
            .clone();
        let adapter_present = config.installed;
        let network_enabled = config.installed && config.virtual_ip.is_some();
        Ok(PlatformNetworkDiagnostics {
            platform: "ios".to_string(),
            adapter_present,
            adapter_name: Some("NEPacketTunnelProvider".to_string()),
            admin_status: Some(if network_enabled {
                "enabled".to_string()
            } else {
                "disabled".to_string()
            }),
            interface_index: None,
            virtual_ip: config.virtual_ip.clone(),
            mtu: config
                .relay_config
                .as_ref()
                .and_then(|relay| relay.relay_mtu)
                .map(u32::from),
            mss: None,
            resolver_servers: config.resolver_servers.clone(),
            resolver_search_domains: config.resolver_search_domains.clone(),
            resolver_split_domains: config.resolver_split_domains.clone(),
            routes: config
                .routes
                .iter()
                .map(|route| route.destination.clone())
                .collect(),
            checks: ios_diagnostic_checks(&config),
        })
    }
}

fn ios_diagnostic_checks(config: &IosCachedNetworkConfig) -> Vec<PlatformDiagnosticCheck> {
    let mut checks = Vec::new();
    checks.push(PlatformDiagnosticCheck {
        name: "packetTunnelConfig".to_string(),
        ok: config.installed,
        message: Some(if config.installed {
            "iOS PacketTunnel config prepared".to_string()
        } else {
            "iOS PacketTunnel config is not prepared".to_string()
        }),
    });
    checks.push(PlatformDiagnosticCheck {
        name: "virtualIp".to_string(),
        ok: config.virtual_ip.is_some(),
        message: config.virtual_ip.clone(),
    });
    if let Some(relay) = &config.relay_config {
        checks.push(PlatformDiagnosticCheck {
            name: "relayDataPlane".to_string(),
            ok: relay.enabled
                && !relay.sessions.is_empty()
                && client_core::relay_path_kind_for_transport(&relay.transport).is_some(),
            message: Some(format!(
                "transport={} sessions={}",
                relay.transport,
                relay.sessions.len()
            )),
        });
    }
    checks
}

#[cfg(test)]
mod tests {
    use super::IosPlatformNetwork;
    use client_core::{PlatformNetwork, PlatformResolverConfig};

    #[test]
    fn diagnostics_include_resolver_search_and_split_domains() {
        let platform = IosPlatformNetwork;
        platform.install_adapter().expect("install adapter");
        platform
            .configure_resolver(&PlatformResolverConfig {
                servers: vec!["10.0.0.53".to_string()],
                search_domains: vec!["corp.lan".to_string()],
                split_domains: vec!["mesh.local".to_string()],
                fallback_to_system_resolvers: false,
            })
            .expect("configure resolver");

        let diagnostics = platform.diagnostics().expect("diagnostics");
        assert_eq!(diagnostics.resolver_servers, vec!["10.0.0.53".to_string()]);
        assert_eq!(
            diagnostics.resolver_search_domains,
            vec!["corp.lan".to_string()]
        );
        assert_eq!(
            diagnostics.resolver_split_domains,
            vec!["mesh.local".to_string()]
        );

        platform.disable_network().expect("disable network");
    }
}
