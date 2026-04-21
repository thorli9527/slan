use std::sync::Mutex;

use slan_app_core::WireGuardKeyPair;
use tunnel::{TunnelConfig, TunnelManager};

use crate::facade::{DataPlaneError, DataPlaneErrorCode};
use crate::key_provider::TunnelKeyProvider;
use crate::snapshot::AppCoreSnapshot;

pub fn ensure_tunnel_key_pair(
    state: &Mutex<AppCoreSnapshot>,
    tunnel_key_provider: &dyn TunnelKeyProvider,
    device_id: &str,
    peer_node_id: &str,
    created_at_ms: u64,
) -> Result<WireGuardKeyPair, String> {
    let mut state = state
        .lock()
        .map_err(|_| "app core state poisoned".to_string())?;
    let current_node_id = state.current_node.as_ref().map(|node| node.node_id.clone());
    let existing = state.tunnel_key_material.clone();
    let material = match existing {
        Some(material)
            if material.device_id.as_deref() == Some(device_id)
                && material.node_id == current_node_id =>
        {
            material
        }
        _ => {
            let material = tunnel_key_provider.generate(
                device_id,
                current_node_id.as_deref(),
                peer_node_id,
                created_at_ms,
            )?;
            state.tunnel_key_material = Some(material.clone());
            material
        }
    };
    Ok(material.key_pair)
}

pub fn replace_tunnel<T: TunnelManager>(
    current_tunnel_peer_virtual_ip: &Mutex<Option<String>>,
    tunnel_manager: &T,
    config: Option<TunnelConfig>,
) -> Result<(), String> {
    let mut current_tunnel = current_tunnel_peer_virtual_ip
        .lock()
        .map_err(|_| "app core tunnel state poisoned".to_string())?;
    let next_peer_virtual_ip = config
        .as_ref()
        .map(|config| config.peer_virtual_ip.as_str());
    if let Some(current_peer_virtual_ip) = current_tunnel.as_deref() {
        if Some(current_peer_virtual_ip) != next_peer_virtual_ip {
            tunnel_manager.close(current_peer_virtual_ip)?;
            *current_tunnel = None;
        }
    }
    if let Some(config) = config {
        if current_tunnel.as_deref() != Some(config.peer_virtual_ip.as_str()) {
            tunnel_manager.establish(&config)?;
            *current_tunnel = Some(config.peer_virtual_ip);
        }
    }
    Ok(())
}

pub fn remember_data_plane_error(
    state: &Mutex<AppCoreSnapshot>,
    error: &DataPlaneError,
) -> Result<(), DataPlaneError> {
    let mut state = state
        .lock()
        .map_err(|_| DataPlaneError::new(DataPlaneErrorCode::Unknown, "app core state poisoned"))?;
    state.last_data_plane_error = Some(error.clone());
    Ok(())
}
