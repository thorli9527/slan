use std::sync::Mutex;

use slan_app_core::{ActivePath, ConnectionState};

use crate::facade::{DataPlaneError, DataPlaneProbe, TunnelRuntimeView};
use crate::snapshot::AppCoreSnapshot;

pub fn update_connected_snapshot(
    state: &Mutex<AppCoreSnapshot>,
    connection_state: ConnectionState,
    active_path: ActivePath,
    tunnel_peer_virtual_ip: Option<String>,
    tunnel_runtime: Option<TunnelRuntimeView>,
) -> Result<(), String> {
    let mut state = state
        .lock()
        .map_err(|_| "app core state poisoned".to_string())?;
    state.connection_state = Some(connection_state);
    state.active_path = Some(active_path);
    state.tunnel_peer_virtual_ip = tunnel_peer_virtual_ip;
    state.tunnel_runtime = tunnel_runtime;
    state.last_data_plane_error = None;
    state.last_probe = None;
    Ok(())
}

pub fn update_disconnected_snapshot(state: &Mutex<AppCoreSnapshot>) -> Result<(), String> {
    let mut state = state
        .lock()
        .map_err(|_| "app core state poisoned".to_string())?;
    state.connection_state = Some(ConnectionState::Disconnected);
    state.active_path = None;
    state.tunnel_peer_virtual_ip = None;
    state.tunnel_runtime = None;
    state.last_data_plane_error = None;
    state.last_probe = None;
    Ok(())
}

pub fn record_probe_result(
    state: &Mutex<AppCoreSnapshot>,
    probe: DataPlaneProbe,
) -> Result<DataPlaneProbe, DataPlaneError> {
    let mut state = state.lock().map_err(|_| {
        DataPlaneError::new(
            crate::facade::DataPlaneErrorCode::Unknown,
            "app core state poisoned",
        )
    })?;
    state.last_data_plane_error = None;
    state.last_probe = Some(probe.clone());
    Ok(probe)
}
