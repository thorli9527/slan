use anyhow::Result;
use client_core::{NetworkRuntimeState, PlatformNetwork, RouteSpec};

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

    fn configure_dns(&self, _dns_servers: &[String]) -> Result<()> {
        anyhow::bail!("network operations are not implemented on this platform yet")
    }

    fn disable_network(&self) -> Result<()> {
        Ok(())
    }

    fn read_runtime_state(&self) -> Result<NetworkRuntimeState> {
        Ok(NetworkRuntimeState::default())
    }
}
