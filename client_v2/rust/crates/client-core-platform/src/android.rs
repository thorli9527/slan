//! Android platform bridge for SLAN mesh networking.
//!
//! Android may still need protected system network APIs, but the product
//! language and app-level boundary stay as mesh networking.

use anyhow::{bail, Result};
use client_core::{NetworkRuntimeState, PlatformNetwork, RouteSpec};

#[derive(Debug, Clone, Default)]
pub struct AndroidPlatformNetwork;

impl PlatformNetwork for AndroidPlatformNetwork {
    fn install_adapter(&self) -> Result<()> {
        bail!("android mesh network backend is not implemented yet")
    }

    fn configure_ip(&self, _virtual_ip: &str) -> Result<()> {
        bail!("android mesh network backend is not implemented yet")
    }

    fn configure_routes(&self, _routes: &[RouteSpec]) -> Result<()> {
        bail!("android mesh network backend is not implemented yet")
    }

    fn configure_dns(&self, _dns_servers: &[String]) -> Result<()> {
        bail!("android mesh network backend is not implemented yet")
    }

    fn disable_network(&self) -> Result<()> {
        Ok(())
    }

    fn read_runtime_state(&self) -> Result<NetworkRuntimeState> {
        Ok(NetworkRuntimeState::default())
    }
}
