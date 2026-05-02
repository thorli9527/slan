//! Linux platform bridge for SLAN mesh networking.

use anyhow::{bail, Result};
use client_core::{NetworkRuntimeState, PlatformNetwork, RouteSpec};

#[derive(Debug, Clone, Default)]
pub struct LinuxPlatformNetwork;

impl PlatformNetwork for LinuxPlatformNetwork {
    fn install_adapter(&self) -> Result<()> {
        bail!("linux mesh network backend is not implemented yet")
    }

    fn configure_ip(&self, _virtual_ip: &str, _prefix_len: u8) -> Result<()> {
        bail!("linux mesh network backend is not implemented yet")
    }

    fn configure_routes(&self, _routes: &[RouteSpec]) -> Result<()> {
        bail!("linux mesh network backend is not implemented yet")
    }

    fn configure_dns(&self, _dns_servers: &[String]) -> Result<()> {
        bail!("linux mesh network backend is not implemented yet")
    }

    fn disable_network(&self) -> Result<()> {
        Ok(())
    }

    fn read_runtime_state(&self) -> Result<NetworkRuntimeState> {
        Ok(NetworkRuntimeState::default())
    }
}
