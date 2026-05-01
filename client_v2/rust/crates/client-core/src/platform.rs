use anyhow::Result;
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RouteSpec {
    pub destination: String,
    pub gateway: Option<String>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct NetworkRuntimeState {
    pub adapter_present: bool,
    pub network_enabled: bool,
    pub virtual_ip: Option<String>,
}

pub trait PlatformNetwork {
    fn install_adapter(&self) -> Result<()>;
    fn configure_ip(&self, virtual_ip: &str) -> Result<()>;
    fn configure_routes(&self, routes: &[RouteSpec]) -> Result<()>;
    fn configure_dns(&self, dns_servers: &[String]) -> Result<()>;
    fn disable_network(&self) -> Result<()>;
    fn read_runtime_state(&self) -> Result<NetworkRuntimeState>;
}
