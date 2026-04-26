use std::collections::HashMap;

use control_mqtt_client::ControlMqttConnectPlan;
use serde::{Deserialize, Serialize};
use slan_app_core::{
    ActivePath, BootstrapConfig, ConnectionState, Device, Node, Session, TunnelKeyMaterial,
};

use crate::facade::{DataPlaneError, DataPlaneProbe, TunnelRuntimeView};

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct AppCoreSnapshot {
    pub session: Option<Session>,
    pub current_device: Option<Device>,
    pub current_node: Option<Node>,
    pub tunnel_key_material: Option<TunnelKeyMaterial>,
    pub current_bootstrap: Option<BootstrapConfig>,
    pub current_network_id: Option<String>,
    pub connection_state: Option<ConnectionState>,
    #[serde(default)]
    pub current_connect_plans: HashMap<String, ControlMqttConnectPlan>,
    pub active_path: Option<ActivePath>,
    pub tunnel_peer_virtual_ip: Option<String>,
    pub tunnel_runtime: Option<TunnelRuntimeView>,
    pub last_data_plane_error: Option<DataPlaneError>,
    pub last_probe: Option<DataPlaneProbe>,
}
