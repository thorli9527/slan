use std::sync::Mutex;

use slan_app_core::{ActivePath, BootstrapConfig, ConnectionState, Peer};
use tunnel::{TunnelConfig, TunnelManager};

use crate::snapshot::AppCoreSnapshot;
use crate::snapshot_updates::update_connected_snapshot;
use crate::state_helpers::replace_tunnel;
use crate::tunnel_runtime::build_tunnel_runtime;

pub struct ConnectContext {
    pub bootstrap: BootstrapConfig,
    pub src_node_id: String,
    pub peer: Peer,
}

pub fn resolve_connect_context(
    state: &Mutex<AppCoreSnapshot>,
    peer_node_id: &str,
) -> Result<ConnectContext, String> {
    let (bootstrap, src_node_id) = {
        let state = state
            .lock()
            .map_err(|_| "app core state poisoned".to_string())?;
        (
            state.current_bootstrap.clone(),
            state.current_node.as_ref().map(|node| node.node_id.clone()),
        )
    };
    let bootstrap =
        bootstrap.ok_or_else(|| "missing bootstrap config, call bootstrap first".to_string())?;
    let src_node_id =
        src_node_id.ok_or_else(|| "missing current node, register node first".to_string())?;
    let network_map = bootstrap
        .network_map
        .clone()
        .ok_or_else(|| "missing network map in bootstrap config".to_string())?;
    let peer = network_map
        .peers
        .into_iter()
        .find(|peer| peer.node_id == peer_node_id)
        .ok_or_else(|| format!("peer node not found in network map: {peer_node_id}"))?;
    Ok(ConnectContext {
        bootstrap,
        src_node_id,
        peer,
    })
}

pub fn finalize_connected_path<T: TunnelManager>(
    state: &Mutex<AppCoreSnapshot>,
    current_tunnel_peer_virtual_ip: &Mutex<Option<String>>,
    tunnel_manager: &T,
    connection_state: ConnectionState,
    active_path: ActivePath,
    tunnel_config: Option<TunnelConfig>,
) -> Result<(), String> {
    let tunnel_runtime = tunnel_config.as_ref().map(build_tunnel_runtime);
    replace_tunnel(
        current_tunnel_peer_virtual_ip,
        tunnel_manager,
        tunnel_config,
    )?;
    let tunnel_peer_virtual_ip = current_tunnel_peer_virtual_ip
        .lock()
        .map_err(|_| "app core tunnel state poisoned".to_string())?
        .clone();
    update_connected_snapshot(
        state,
        connection_state,
        active_path,
        tunnel_peer_virtual_ip,
        tunnel_runtime,
    )
}
