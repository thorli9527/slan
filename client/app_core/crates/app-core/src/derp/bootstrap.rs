use serde::{Deserialize, Serialize};

use crate::network::ControlPlaneConfig;
use crate::{DerpMap, Device, Network, NetworkMap, RelayConfig};

/// 客户端启动配置。
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct BootstrapConfig {
    pub device: Device,
    pub networks: Vec<Network>,
    pub control_plane: ControlPlaneConfig,
    pub stun_servers: Vec<String>,
    pub relay: RelayConfig,
    pub derp_map: Option<DerpMap>,
    pub network_map: Option<NetworkMap>,
}
