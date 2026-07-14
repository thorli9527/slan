use std::{
    env,
    net::{IpAddr, Ipv4Addr, Ipv6Addr, SocketAddr},
};

use anyhow::Result;
use client_core::{NetworkRuntimeState, PlatformNetwork, PlatformResolverConfig, RouteSpec};

pub mod direct_udp;

#[cfg(target_os = "android")]
pub mod android;
#[cfg(target_os = "ios")]
pub mod ios;
#[cfg(target_os = "linux")]
pub mod linux;
#[cfg(target_os = "macos")]
pub mod macos;
#[cfg(target_os = "windows")]
pub mod windows;

#[cfg(target_os = "android")]
pub use android::AndroidPlatformNetwork as PlatformNetworkImpl;
#[cfg(target_os = "ios")]
pub use ios::IosPlatformNetwork as PlatformNetworkImpl;
#[cfg(target_os = "linux")]
pub use linux::LinuxPlatformNetwork as PlatformNetworkImpl;
#[cfg(target_os = "macos")]
pub use macos::MacosPlatformNetwork as PlatformNetworkImpl;
#[cfg(target_os = "windows")]
pub use windows::WindowsPlatformNetwork as PlatformNetworkImpl;

#[cfg(not(any(
    target_os = "windows",
    target_os = "macos",
    target_os = "linux",
    target_os = "android",
    target_os = "ios"
)))]
pub type PlatformNetworkImpl = NoopPlatformNetwork;

#[derive(Debug, Clone, Default)]
pub struct NoopPlatformNetwork;

pub fn effective_resolver_servers(configured: &[String]) -> Vec<String> {
    if let Some(local) = local_dns_override_server() {
        return vec![local];
    }
    configured
        .iter()
        .map(|value| value.trim().to_string())
        .filter(|value| !value.is_empty())
        .collect()
}

fn local_dns_override_server() -> Option<String> {
    let bind = env::var("SLAN_LOCAL_DNS_BIND").ok()?;
    let bind = bind.trim();
    if bind.is_empty() {
        return None;
    }
    let addr: SocketAddr = bind.parse().ok()?;
    if addr.port() != 53 {
        return None;
    }
    let ip = match addr.ip() {
        IpAddr::V4(ip) if ip.is_unspecified() => IpAddr::V4(Ipv4Addr::LOCALHOST),
        IpAddr::V6(ip) if ip.is_unspecified() => IpAddr::V6(Ipv6Addr::LOCALHOST),
        ip => ip,
    };
    Some(ip.to_string())
}

#[cfg(test)]
mod tests {
    use super::effective_resolver_servers;
    use std::sync::{Mutex, OnceLock};

    fn env_lock() -> std::sync::MutexGuard<'static, ()> {
        static LOCK: OnceLock<Mutex<()>> = OnceLock::new();
        LOCK.get_or_init(|| Mutex::new(()))
            .lock()
            .expect("env test mutex poisoned")
    }

    #[test]
    fn effective_resolver_servers_prefers_local_override_on_port_53() {
        let _lock = env_lock();
        std::env::set_var("SLAN_LOCAL_DNS_BIND", "127.0.0.1:53");
        let actual = effective_resolver_servers(&["8.8.8.8".to_string(), "1.1.1.1".to_string()]);
        assert_eq!(actual, vec!["127.0.0.1".to_string()]);
        std::env::remove_var("SLAN_LOCAL_DNS_BIND");
    }

    #[test]
    fn effective_resolver_servers_ignores_non_53_local_override() {
        let _lock = env_lock();
        std::env::set_var("SLAN_LOCAL_DNS_BIND", "127.0.0.1:53535");
        let actual = effective_resolver_servers(&["8.8.8.8".to_string(), "".to_string()]);
        assert_eq!(actual, vec!["8.8.8.8".to_string()]);
        std::env::remove_var("SLAN_LOCAL_DNS_BIND");
    }
}

impl PlatformNetwork for NoopPlatformNetwork {
    fn install_adapter(&self) -> Result<()> {
        anyhow::bail!("network operations are not implemented on this platform yet")
    }

    fn configure_ip(&self, _virtual_ip: &str, _prefix_len: u8) -> Result<()> {
        anyhow::bail!("network operations are not implemented on this platform yet")
    }

    fn configure_routes(&self, _routes: &[RouteSpec]) -> Result<()> {
        anyhow::bail!("network operations are not implemented on this platform yet")
    }

    fn configure_resolver(&self, _resolver: &PlatformResolverConfig) -> Result<()> {
        anyhow::bail!("network operations are not implemented on this platform yet")
    }

    fn disable_network(&self) -> Result<()> {
        Ok(())
    }

    fn read_runtime_state(&self) -> Result<NetworkRuntimeState> {
        Ok(NetworkRuntimeState::default())
    }
}
