use control_mqtt_client::{
    ControlMqttActiveNetworkEnabled, ControlMqttClient, ControlMqttConfig, ControlMqttConnectPlan,
    ControlMqttConnectionStateReport, ControlMqttDeviceIPReassigned, ControlMqttEvent,
    ControlMqttPathHealthReport,
};
use controller_client::{
    ControllerClient, CreateNetworkRequest, DeactivateNetworkRequest, DeviceNetworkStateRequest,
    JoinNetworkByKeyRequest, JoinNetworkRequest, LoginRequest, RefreshTokenRequest,
    RegisterDeviceRequest, RegisterNodeRequest, RegisterRequest, RelayTicketRequest,
    SwitchNetworkRequest, UpdateAttachmentRemarkRequest,
};
use p2p::{P2PConnector, PeerCandidate};
use relay_client::{DerpPool, PathManager, RelayClient};
use slan_app_core::{
    ActivePath, AllowedIp, BootstrapConfig, ConnectionPath, ConnectionState, DerpCluster, Device,
    Endpoint, Network, NetworkAssignment, NetworkJoinResult, NetworkMap, Node, Peer, RelayTicket,
    Session, TunnelTransport, WireGuardInterfaceConfig, WireGuardKeyPair, WireGuardPeerConfig,
};
use std::sync::Mutex;
use tunnel::{TunnelConfig, TunnelManager};

use crate::connect_runtime::{finalize_connected_path, resolve_connect_context};
use crate::diagnostics::{classify_path_manager_error, now_ms};
use crate::facade::{
    AppCoreFacade, ControlConnectPlanView, ControlPathOptionView, ControlStatusView,
    DataPlaneError, DataPlaneErrorCode, DataPlaneProbe,
};
use crate::key_provider::{InMemoryTunnelKeyProvider, TunnelKeyProvider};
use crate::probe_runtime::{poll_reply_until_timeout, probe_health_from_active_path};
use crate::snapshot::AppCoreSnapshot;
use crate::snapshot_updates::{
    record_probe_result, update_connected_snapshot, update_disconnected_snapshot,
};
use crate::state_helpers::{ensure_tunnel_key_pair, remember_data_plane_error, replace_tunnel};
use crate::tunnel_runtime::{
    build_tunnel_config, build_tunnel_runtime, connection_state_from_active_path,
};

pub struct DefaultAppCoreFacade<C, P, R, D, M, T>
where
    C: ControllerClient,
    P: P2PConnector,
    R: RelayClient,
    D: DerpPool,
    M: PathManager,
    T: TunnelManager,
{
    controller: C,
    p2p: P,
    relay: R,
    derp_pool: D,
    path_manager: M,
    tunnel_manager: T,
    tunnel_key_provider: Box<dyn TunnelKeyProvider>,
    state: Mutex<AppCoreSnapshot>,
    control_mqtt: Mutex<Option<ControlMqttClient>>,
    current_tunnel_peer_virtual_ip: Mutex<Option<String>>,
    probe_sequence: Mutex<u64>,
    default_probe_reply_timeout_ms: u64,
}

impl<C, P, R, D, M, T> DefaultAppCoreFacade<C, P, R, D, M, T>
where
    C: ControllerClient,
    P: P2PConnector,
    R: RelayClient,
    D: DerpPool,
    M: PathManager,
    T: TunnelManager,
{
    const DEFAULT_PROBE_REPLY_TIMEOUT_MS: u64 = 25;
    const PROBE_REPLY_POLL_INTERVAL_MS: u64 = 2;

    pub fn new(
        controller: C,
        p2p: P,
        relay: R,
        derp_pool: D,
        path_manager: M,
        tunnel_manager: T,
    ) -> Self {
        Self::new_with_tunnel_key_provider(
            controller,
            p2p,
            relay,
            derp_pool,
            path_manager,
            tunnel_manager,
            Box::new(InMemoryTunnelKeyProvider),
        )
    }

    pub fn new_with_tunnel_key_provider(
        controller: C,
        p2p: P,
        relay: R,
        derp_pool: D,
        path_manager: M,
        tunnel_manager: T,
        tunnel_key_provider: Box<dyn TunnelKeyProvider>,
    ) -> Self {
        Self {
            controller,
            p2p,
            relay,
            derp_pool,
            path_manager,
            tunnel_manager,
            tunnel_key_provider,
            state: Mutex::new(AppCoreSnapshot::default()),
            control_mqtt: Mutex::new(None),
            current_tunnel_peer_virtual_ip: Mutex::new(None),
            probe_sequence: Mutex::new(0),
            default_probe_reply_timeout_ms: Self::DEFAULT_PROBE_REPLY_TIMEOUT_MS,
        }
    }

    pub fn new_with_probe_reply_timeout(
        controller: C,
        p2p: P,
        relay: R,
        derp_pool: D,
        path_manager: M,
        tunnel_manager: T,
        default_probe_reply_timeout_ms: u64,
    ) -> Self {
        Self::new_with_probe_reply_timeout_and_key_provider(
            controller,
            p2p,
            relay,
            derp_pool,
            path_manager,
            tunnel_manager,
            default_probe_reply_timeout_ms,
            Box::new(InMemoryTunnelKeyProvider),
        )
    }

    pub fn new_with_probe_reply_timeout_and_key_provider(
        controller: C,
        p2p: P,
        relay: R,
        derp_pool: D,
        path_manager: M,
        tunnel_manager: T,
        default_probe_reply_timeout_ms: u64,
        tunnel_key_provider: Box<dyn TunnelKeyProvider>,
    ) -> Self {
        Self {
            controller,
            p2p,
            relay,
            derp_pool,
            path_manager,
            tunnel_manager,
            tunnel_key_provider,
            state: Mutex::new(AppCoreSnapshot::default()),
            control_mqtt: Mutex::new(None),
            current_tunnel_peer_virtual_ip: Mutex::new(None),
            probe_sequence: Mutex::new(0),
            default_probe_reply_timeout_ms,
        }
    }

    fn with_access_token(&self) -> Result<String, String> {
        self.state
            .lock()
            .map_err(|_| "app core state poisoned".to_string())?
            .session
            .as_ref()
            .map(|session| session.access_token.clone())
            .ok_or_else(|| "missing session, login first".to_string())
    }

    fn current_node_id(&self) -> Result<String, String> {
        self.state
            .lock()
            .map_err(|_| "app core state poisoned".to_string())?
            .current_node
            .as_ref()
            .map(|node| node.node_id.clone())
            .ok_or_else(|| "missing current node, register node first".to_string())
    }

    fn issue_relay_ticket_with_options(
        &self,
        network_id: String,
        src_node_id: String,
        dst_node_id: String,
        derp_cluster_id: Option<String>,
        preferred_derp_node_ids: Vec<String>,
        reason: String,
        relay_region_id: Option<String>,
    ) -> Result<RelayTicket, String> {
        let access_token = self.with_access_token()?;
        self.controller.issue_relay_ticket(
            &access_token,
            RelayTicketRequest {
                network_id,
                src_node_id,
                dst_node_id,
                derp_cluster_id,
                preferred_derp_node_ids,
                reason,
                relay_region_id,
            },
        )
    }

    fn select_derp_cluster(
        &self,
        bootstrap: &BootstrapConfig,
    ) -> Result<Option<DerpCluster>, String> {
        let Some(derp_map) = &bootstrap.derp_map else {
            return Ok(None);
        };
        Ok(derp_map
            .clusters
            .iter()
            .find(|cluster| !cluster.nodes.is_empty())
            .cloned())
    }

    fn connect_via_derp_pool(
        &self,
        bootstrap: &BootstrapConfig,
        network_id: String,
        src_node_id: String,
        dst_node_id: String,
    ) -> Result<Option<ConnectionState>, String> {
        let Some(cluster) = self.select_derp_cluster(bootstrap)? else {
            return Ok(None);
        };
        let preferred_derp_node_ids = cluster
            .nodes
            .iter()
            .map(|node| node.node_id.clone())
            .collect::<Vec<_>>();
        let ticket = self.issue_relay_ticket_with_options(
            network_id,
            src_node_id,
            dst_node_id,
            Some(cluster.cluster_id.clone()),
            preferred_derp_node_ids,
            "p2p_failed".to_string(),
            None,
        )?;
        self.derp_pool
            .install_cluster(&cluster.cluster_id, cluster.nodes.clone(), ticket)?;
        self.derp_pool
            .warm_up(usize::from(cluster.recommended_fanout.max(1)))?;
        self.derp_pool.tick_health_check()?;
        let _ = self.derp_pool.maybe_switch()?;
        let _active_link = self
            .derp_pool
            .active_link()
            .ok_or_else(|| "derp pool has no active link after warm up".to_string())?;
        Ok(Some(ConnectionState::Connected(ConnectionPath::Derp)))
    }

    pub fn cached_node_id(&self) -> Result<String, String> {
        self.current_node_id()
    }

    pub fn snapshot(&self) -> Result<AppCoreSnapshot, String> {
        self.state
            .lock()
            .map_err(|_| "app core state poisoned".to_string())
            .map(|state| state.clone())
    }

    pub fn restore_snapshot(&self, snapshot: AppCoreSnapshot) -> Result<(), String> {
        let mut control_mqtt = self
            .control_mqtt
            .lock()
            .map_err(|_| "app core control mqtt state poisoned".to_string())?;
        *control_mqtt = None;
        let mut current_tunnel = self
            .current_tunnel_peer_virtual_ip
            .lock()
            .map_err(|_| "app core tunnel state poisoned".to_string())?;
        *current_tunnel = snapshot.tunnel_peer_virtual_ip.clone();
        let mut state = self
            .state
            .lock()
            .map_err(|_| "app core state poisoned".to_string())?;
        *state = snapshot;
        Ok(())
    }

    fn build_tunnel_config(
        &self,
        active_path: &ActivePath,
        bootstrap: &BootstrapConfig,
        peer: &slan_app_core::Peer,
    ) -> Result<Option<TunnelConfig>, String> {
        let key_pair = ensure_tunnel_key_pair(
            &self.state,
            self.tunnel_key_provider.as_ref(),
            bootstrap.device.device_id.as_str(),
            &peer.node_id,
            now_ms(),
        )?;
        Ok(build_tunnel_config(active_path, bootstrap, peer, key_pair))
    }

    fn build_local_network_tunnel_config(
        &self,
        network: &Network,
        device: &Device,
    ) -> TunnelConfig {
        let local_virtual_ip = local_virtual_ip_for(network, device).unwrap_or_default();
        let peer_virtual_ip =
            peer_virtual_ip_for(network, device.device_id.as_str(), &local_virtual_ip);
        let key_pair = WireGuardKeyPair {
            public_key: device
                .public_key
                .clone()
                .filter(|value| !value.trim().is_empty())
                .unwrap_or_else(|| "debug-public-key".to_string()),
            private_key: "debug-private-key".to_string(),
        };
        TunnelConfig {
            transport: TunnelTransport::Relay,
            local_virtual_ip: local_virtual_ip.clone(),
            peer_virtual_ip: peer_virtual_ip.clone(),
            wireguard_interface: WireGuardInterfaceConfig {
                interface_name: Some("SLAN LAN Adapter".to_string()),
                key_pair,
                listen_port: Some(51820),
                mtu: Some(1280),
                addresses: vec![format!(
                    "{}/{}",
                    local_virtual_ip,
                    prefix_len_from_cidr(network.cidr.as_str()).unwrap_or(32)
                )],
                dns_servers: vec![],
                peers: vec![],
            },
            wireguard_peer: WireGuardPeerConfig {
                peer_node_id: None,
                public_key: "peer-debug-public-key".to_string(),
                preshared_key: None,
                endpoint: Some(default_tunnel_endpoint()),
                allowed_ips: vec![AllowedIp {
                    cidr: format!("{peer_virtual_ip}/32"),
                }],
                persistent_keepalive_seconds: None,
            },
        }
    }

    fn next_probe_id(&self, sampled_at_ms: u64) -> Result<String, String> {
        let mut sequence = self
            .probe_sequence
            .lock()
            .map_err(|_| "app core probe state poisoned".to_string())?;
        *sequence += 1;
        Ok(format!("probe-{sampled_at_ms}-{}", *sequence))
    }

    fn control_mqtt_config(&self) -> Result<ControlMqttConfig, String> {
        let state = self
            .state
            .lock()
            .map_err(|_| "app core state poisoned".to_string())?;
        let bootstrap = state
            .current_bootstrap
            .as_ref()
            .ok_or_else(|| "missing bootstrap config, call bootstrap first".to_string())?;
        let network_map = bootstrap.network_map.as_ref().ok_or_else(|| {
            "missing network map in bootstrap config, call bootstrap first".to_string()
        })?;
        let session_token = bootstrap
            .control_plane
            .session_token
            .clone()
            .ok_or_else(|| "missing control session token in bootstrap config".to_string())?;
        let node = state
            .current_node
            .as_ref()
            .ok_or_else(|| "missing current node, register node first".to_string())?;
        let user_id = state
            .session
            .as_ref()
            .map(|session| session.user_id.clone())
            .unwrap_or_else(|| network_map.self_user_id.clone());
        let access_token = state
            .session
            .as_ref()
            .map(|session| session.access_token.clone())
            .ok_or_else(|| "missing session access token".to_string())?;
        let device_id = state
            .current_device
            .as_ref()
            .map(|device| device.device_id.clone())
            .unwrap_or_else(|| network_map.self_device_id.clone());
        let mqtt = state
            .current_device
            .as_ref()
            .and_then(|device| device.mqtt.clone())
            .ok_or_else(|| "missing MQTT credential in current device".to_string())?;

        Ok(ControlMqttConfig {
            access_token,
            session_token,
            user_id,
            device_id,
            node_id: node.node_id.clone(),
            node_public_key: node.node_public_key.clone(),
            network_id: state
                .current_network_id
                .clone()
                .unwrap_or_else(|| network_map.network_id.clone()),
            capabilities: node.capabilities.clone(),
            mqtt,
        })
    }

    fn connect_plan_for_peer(
        &self,
        peer_node_id: &str,
    ) -> Result<Option<ControlMqttConnectPlan>, String> {
        let state = self
            .state
            .lock()
            .map_err(|_| "app core state poisoned".to_string())?;
        Ok(state.current_connect_plans.get(peer_node_id).cloned())
    }

    fn infer_control_report_peer_node_id(state: &AppCoreSnapshot) -> Option<String> {
        match state.active_path.as_ref() {
            Some(ActivePath::P2P { peer_node_id }) | Some(ActivePath::Relay { peer_node_id }) => {
                Some(peer_node_id.clone())
            }
            _ if state.current_connect_plans.len() == 1 => {
                state.current_connect_plans.keys().next().cloned()
            }
            _ => None,
        }
    }

    fn control_report_endpoint_for_peer(
        state: &AppCoreSnapshot,
        peer_node_id: &str,
    ) -> Option<String> {
        state
            .current_connect_plans
            .get(peer_node_id)
            .and_then(|plan| plan.paths.first())
            .map(|path| path.endpoint.clone())
    }

    fn emit_control_observations(
        client: &mut ControlMqttClient,
        state: &AppCoreSnapshot,
        config: &ControlMqttConfig,
    ) -> Result<(), String> {
        let Some(peer_node_id) = Self::infer_control_report_peer_node_id(state) else {
            return Ok(());
        };
        let endpoint = Self::control_report_endpoint_for_peer(state, &peer_node_id);
        if let Some(connection_state) = state.connection_state.as_ref() {
            let (path, status, reason) = match connection_state {
                ConnectionState::Disconnected => ("none".to_string(), "closed".to_string(), None),
                ConnectionState::Connecting => {
                    ("connecting".to_string(), "connecting".to_string(), None)
                }
                ConnectionState::Connected(ConnectionPath::P2P) => {
                    ("p2p".to_string(), "connected".to_string(), None)
                }
                ConnectionState::Connected(ConnectionPath::Relay) => {
                    ("relay".to_string(), "connected".to_string(), None)
                }
                ConnectionState::Connected(ConnectionPath::Derp) => {
                    ("derp".to_string(), "connected".to_string(), None)
                }
                ConnectionState::Failed(reason) => (
                    "failed".to_string(),
                    "failed".to_string(),
                    Some(reason.clone()),
                ),
            };
            let (observed_rtt_ms, packet_loss_ppm, path_score, derp_node_id) = state
                .last_probe
                .as_ref()
                .map(|probe| {
                    (
                        probe.observed_rtt_ms,
                        probe.packet_loss_ppm,
                        probe.path_score,
                        probe.derp_node_id.clone(),
                    )
                })
                .unwrap_or((None, None, None, None));
            client.send_connection_state(&ControlMqttConnectionStateReport {
                network_id: config.network_id.clone(),
                peer_node_id: peer_node_id.clone(),
                path,
                state: status,
                reason,
                observed_rtt_ms,
                packet_loss_ppm,
                path_score,
                derp_node_id,
            })?;
        }

        if let Some(probe) = state.last_probe.as_ref() {
            let path_type = match &probe.active_path {
                ActivePath::None => "none".to_string(),
                ActivePath::P2P { .. } => "p2p".to_string(),
                ActivePath::Relay { .. } => "relay".to_string(),
                ActivePath::Derp { .. } => "derp".to_string(),
            };
            client.send_path_health_report(&ControlMqttPathHealthReport {
                network_id: config.network_id.clone(),
                peer_node_id,
                path_type,
                endpoint,
                derp_node_id: probe.derp_node_id.clone(),
                observed_rtt_ms: probe.observed_rtt_ms,
                packet_loss_ppm: probe.packet_loss_ppm,
                path_score: probe.path_score,
                sampled_at_ms: probe.sampled_at_ms,
            })?;
        }
        Ok(())
    }

    fn ordered_peer_endpoints(peer: &Peer, plan: Option<&ControlMqttConnectPlan>) -> Vec<Endpoint> {
        let mut endpoints = peer.endpoints.clone();
        let Some(plan) = plan else {
            return endpoints;
        };
        endpoints.sort_by_key(|endpoint| {
            plan.paths
                .iter()
                .position(|path| {
                    path.endpoint == endpoint.address
                        && !path.path_type.contains("relay")
                        && !path.path_type.contains("derp")
                })
                .unwrap_or(usize::MAX)
        });
        endpoints
    }

    fn relay_ticket_from_plan(
        &self,
        network_id: String,
        src_node_id: String,
        peer_node_id: String,
        plan: Option<&ControlMqttConnectPlan>,
    ) -> Result<RelayTicket, String> {
        if let Some(ticket) = plan.and_then(|plan| plan.relay_ticket.clone()) {
            return Ok(ticket);
        }
        self.issue_relay_ticket_with_options(
            network_id,
            src_node_id,
            peer_node_id,
            plan.and_then(|plan| {
                let value = plan.derp_cluster_id.trim();
                if value.is_empty() {
                    None
                } else {
                    Some(value.to_string())
                }
            }),
            plan.map(|plan| plan.preferred_derp_node_ids.clone())
                .unwrap_or_default(),
            "p2p_failed".to_string(),
            None,
        )
    }

    fn clear_local_network_runtime(&self) -> Result<(), String> {
        replace_tunnel(
            &self.current_tunnel_peer_virtual_ip,
            &self.tunnel_manager,
            None,
        )?;
        let _ = self.tunnel_manager.stop_local_dns();
        if let Ok(mut control_mqtt) = self.control_mqtt.lock() {
            *control_mqtt = None;
        }
        let mut state = self
            .state
            .lock()
            .map_err(|_| "app core state poisoned".to_string())?;
        state.current_network_id = None;
        state.current_connect_plans.clear();
        state.connection_state = Some(ConnectionState::Disconnected);
        state.active_path = None;
        state.tunnel_peer_virtual_ip = None;
        state.tunnel_runtime = None;
        if let Some(bootstrap) = state.current_bootstrap.as_mut() {
            bootstrap.network_map = None;
        }
        Ok(())
    }
}

impl<C, P, R, D, M, T> AppCoreFacade for DefaultAppCoreFacade<C, P, R, D, M, T>
where
    C: ControllerClient,
    P: P2PConnector,
    R: RelayClient,
    D: DerpPool,
    M: PathManager,
    T: TunnelManager,
{
    fn snapshot(&self) -> Result<AppCoreSnapshot, String> {
        DefaultAppCoreFacade::snapshot(self)
    }

    fn restore_snapshot(&self, snapshot: AppCoreSnapshot) -> Result<(), String> {
        DefaultAppCoreFacade::restore_snapshot(self, snapshot)
    }

    fn restore_session(&self, session: Session) -> Result<(), String> {
        let mut state = self
            .state
            .lock()
            .map_err(|_| "app core state poisoned".to_string())?;
        state.session = Some(session);
        Ok(())
    }

    fn register(&self, email: String, password: String) -> Result<Session, String> {
        let session = self
            .controller
            .register(RegisterRequest { email, password })?;
        let mut state = self
            .state
            .lock()
            .map_err(|_| "app core state poisoned".to_string())?;
        state.session = Some(session.clone());
        Ok(session)
    }

    fn login(&self, email: String, password: String) -> Result<Session, String> {
        let session = self.controller.login(LoginRequest {
            email,
            password,
            device_id: None,
        })?;
        let mut state = self
            .state
            .lock()
            .map_err(|_| "app core state poisoned".to_string())?;
        state.session = Some(session.clone());
        Ok(session)
    }

    fn refresh_session(
        &self,
        refresh_token: String,
        device_id: Option<String>,
    ) -> Result<Session, String> {
        let session = self.controller.refresh(RefreshTokenRequest {
            refresh_token,
            device_id,
        })?;
        let mut state = self
            .state
            .lock()
            .map_err(|_| "app core state poisoned".to_string())?;
        state.session = Some(session.clone());
        Ok(session)
    }

    fn register_device(
        &self,
        name: String,
        platform: String,
        machine_id: String,
        public_key: String,
    ) -> Result<Device, String> {
        let access_token = self.with_access_token()?;
        let device = self.controller.register_device(
            &access_token,
            RegisterDeviceRequest {
                name,
                platform,
                machine_id,
                public_key,
            },
        )?;
        let mut state = self
            .state
            .lock()
            .map_err(|_| "app core state poisoned".to_string())?;
        state.current_device = Some(device.clone());
        state.tunnel_key_material = None;
        Ok(device)
    }

    fn list_devices(&self) -> Result<Vec<Device>, String> {
        let access_token = self.with_access_token()?;
        let devices = self.controller.list_devices(&access_token)?;
        let session_device_id = {
            let state = self
                .state
                .lock()
                .map_err(|_| "app core state poisoned".to_string())?;
            state.session.as_ref().and_then(|session| {
                session
                    .device_id
                    .as_ref()
                    .map(|device_id| device_id.trim().to_string())
                    .filter(|device_id| !device_id.is_empty())
            })
        };
        let selected = session_device_id
            .as_deref()
            .and_then(|device_id| {
                devices
                    .iter()
                    .find(|device| device.device_id == device_id)
                    .cloned()
            })
            .or_else(|| devices.first().cloned());
        if let Some(device) = selected {
            let mut state = self
                .state
                .lock()
                .map_err(|_| "app core state poisoned".to_string())?;
            state.current_device = Some(device);
        }
        Ok(devices)
    }

    fn register_node(
        &self,
        device_id: String,
        node_id: String,
        node_public_key: String,
        capabilities: Vec<String>,
    ) -> Result<Node, String> {
        let access_token = self.with_access_token()?;
        let node = self.controller.register_node(
            &access_token,
            RegisterNodeRequest {
                device_id,
                node_id,
                node_public_key,
                capabilities,
            },
        )?;
        let mut state = self
            .state
            .lock()
            .map_err(|_| "app core state poisoned".to_string())?;
        state.current_node = Some(node.clone());
        state.tunnel_key_material = None;
        Ok(node)
    }

    fn list_networks(&self) -> Result<Vec<Network>, String> {
        let access_token = self.with_access_token()?;
        self.controller.list_networks(&access_token)
    }

    fn create_network(
        &self,
        name: String,
        cidr: Option<String>,
        allocation_start_ip: Option<String>,
        allocation_end_ip: Option<String>,
    ) -> Result<Network, String> {
        let access_token = self.with_access_token()?;
        self.controller.create_network(
            &access_token,
            CreateNetworkRequest {
                name,
                cidr,
                description: None,
                allocation_start_ip,
                allocation_end_ip,
                bind_device_id: None,
            },
        )
    }

    fn join_network(
        &self,
        network_id: String,
        device_id: String,
    ) -> Result<NetworkJoinResult, String> {
        let access_token = self.with_access_token()?;
        self.controller.join_network(
            &access_token,
            JoinNetworkRequest {
                network_id,
                device_id,
            },
        )
    }

    fn join_network_by_key(
        &self,
        join_key: String,
        device_id: String,
    ) -> Result<NetworkJoinResult, String> {
        let access_token = self.with_access_token()?;
        self.controller.join_network_by_key(
            &access_token,
            JoinNetworkByKeyRequest {
                join_key,
                device_id,
            },
        )
    }

    fn update_attachment_remark(
        &self,
        network_id: String,
        attachment_id: String,
        remark: Option<String>,
    ) -> Result<NetworkAssignment, String> {
        let access_token = self.with_access_token()?;
        self.controller.update_attachment_remark(
            &access_token,
            UpdateAttachmentRemarkRequest {
                network_id,
                attachment_id,
                remark,
            },
        )
    }

    fn activate_network(
        &self,
        network_id: String,
        device_id: String,
    ) -> Result<NetworkJoinResult, String> {
        let access_token = self.with_access_token()?;
        let joined = self.controller.activate_network(
            &access_token,
            JoinNetworkRequest {
                network_id: network_id.clone(),
                device_id,
            },
        )?;
        let mut state = self
            .state
            .lock()
            .map_err(|_| "app core state poisoned".to_string())?;
        state.current_network_id = Some(network_id);
        if let Some(virtual_ip) = joined.virtual_ip.as_ref() {
            if let Some(device) = state.current_device.as_mut() {
                device.virtual_ip = Some(virtual_ip.clone());
            }
        }
        Ok(joined)
    }

    fn switch_network(
        &self,
        network_id: String,
        device_id: String,
    ) -> Result<NetworkJoinResult, String> {
        let access_token = self.with_access_token()?;
        let joined = self.controller.switch_network(
            &access_token,
            SwitchNetworkRequest {
                network_id: network_id.clone(),
                device_id,
            },
        )?;
        let mut state = self
            .state
            .lock()
            .map_err(|_| "app core state poisoned".to_string())?;
        state.current_network_id = Some(network_id);
        if let Some(virtual_ip) = joined.virtual_ip.as_ref() {
            if let Some(device) = state.current_device.as_mut() {
                device.virtual_ip = Some(virtual_ip.clone());
            }
        }
        Ok(joined)
    }

    fn deactivate_network(&self, network_id: String, device_id: String) -> Result<(), String> {
        let access_token = self.with_access_token()?;
        self.controller.deactivate_network(
            &access_token,
            DeactivateNetworkRequest {
                network_id: network_id.clone(),
                device_id,
            },
        )?;
        let mut state = self
            .state
            .lock()
            .map_err(|_| "app core state poisoned".to_string())?;
        if state.current_network_id.as_deref() == Some(network_id.as_str()) {
            state.current_network_id = None;
            state.current_bootstrap = None;
            state.current_connect_plans.clear();
            state.connection_state = Some(ConnectionState::Disconnected);
            state.active_path = None;
            state.tunnel_peer_virtual_ip = None;
        }
        Ok(())
    }

    fn set_device_network_state(
        &self,
        device_id: String,
        network_id: String,
        control_reachable: bool,
        network_online: bool,
        tunnel_up: bool,
        last_probe_ok: bool,
        virtual_ip: Option<String>,
        reported_at: Option<i64>,
    ) -> Result<(), String> {
        let access_token = self.with_access_token()?;
        self.controller.set_device_network_state(
            &access_token,
            DeviceNetworkStateRequest {
                device_id,
                network_id,
                control_reachable,
                network_online,
                tunnel_up,
                last_probe_ok,
                virtual_ip,
                reported_at,
            },
        )
    }

    fn report_device_network_state(&self) -> Result<(), String> {
        if let Err(err) = self.control_sync() {
            if is_remote_network_disabled_error(&err) {
                self.clear_local_network_runtime()?;
                return Ok(());
            }
        }
        let access_token = self.with_access_token()?;
        let (device, network_id, virtual_ip, tunnel_online, last_probe_ok) = {
            let state = self
                .state
                .lock()
                .map_err(|_| "app core state poisoned".to_string())?
                .clone();
            let device = match state.current_device.clone() {
                Some(device) => device,
                None => {
                    let devices = self.controller.list_devices(&access_token)?;
                    let session_device_id = state.session.as_ref().and_then(|session| {
                        session
                            .device_id
                            .as_ref()
                            .map(|device_id| device_id.trim().to_string())
                            .filter(|device_id| !device_id.is_empty())
                    });
                    let device = session_device_id
                        .as_deref()
                        .and_then(|device_id| {
                            devices
                                .iter()
                                .find(|device| device.device_id == device_id)
                                .cloned()
                        })
                        .or_else(|| devices.first().cloned())
                        .ok_or_else(|| "missing current device".to_string())?;
                    let mut live_state = self
                        .state
                        .lock()
                        .map_err(|_| "app core state poisoned".to_string())?;
                    live_state.current_device = Some(device.clone());
                    device
                }
            };
            let network_id = state.current_network_id.clone().or_else(|| {
                state
                    .current_bootstrap
                    .as_ref()
                    .and_then(|bootstrap| bootstrap.network_map.as_ref())
                    .map(|network_map| network_map.network_id.clone())
            });
            let tunnel_online = state.tunnel_runtime.is_some();
            let last_probe_ok = state
                .last_probe
                .as_ref()
                .map(|probe| probe.reply_observed)
                .unwrap_or(tunnel_online);
            (
                device,
                network_id,
                state.current_device.and_then(|device| device.virtual_ip),
                tunnel_online,
                last_probe_ok,
            )
        };
        let (network_id, virtual_ip) = match network_id {
            Some(network_id) => (network_id, virtual_ip),
            None => {
                let networks = self.controller.list_networks(&access_token)?;
                let network = match resolve_target_network(&networks, None, Some(&device.device_id))
                {
                    Ok(network) => network,
                    Err(_) => return Ok(()),
                };
                let virtual_ip = local_virtual_ip_for(&network, &device).or(virtual_ip);
                let mut state = self
                    .state
                    .lock()
                    .map_err(|_| "app core state poisoned".to_string())?;
                state.current_network_id = Some(network.network_id.clone());
                if let Some(current_device) = state.current_device.as_mut() {
                    if current_device.virtual_ip.is_none() {
                        current_device.virtual_ip = virtual_ip.clone();
                    }
                }
                (network.network_id, virtual_ip)
            }
        };
        self.controller.set_device_network_state(
            &access_token,
            DeviceNetworkStateRequest {
                device_id: device.device_id,
                network_id,
                control_reachable: true,
                network_online: tunnel_online,
                tunnel_up: tunnel_online,
                last_probe_ok,
                virtual_ip,
                reported_at: Some((now_ms() / 1000) as i64),
            },
        )
    }

    fn enable_local_network(&self, network_id: Option<String>) -> Result<BootstrapConfig, String> {
        let access_token = self.with_access_token()?;
        let networks = self.controller.list_networks(&access_token)?;
        let device = {
            let state = self
                .state
                .lock()
                .map_err(|_| "app core state poisoned".to_string())?;
            state.current_device.clone()
        };
        let device = match device {
            Some(device) => device,
            None => {
                let devices = self.controller.list_devices(&access_token)?;
                let session_device_id = {
                    let state = self
                        .state
                        .lock()
                        .map_err(|_| "app core state poisoned".to_string())?;
                    state.session.as_ref().and_then(|session| {
                        session
                            .device_id
                            .as_ref()
                            .map(|device_id| device_id.trim().to_string())
                            .filter(|device_id| !device_id.is_empty())
                    })
                };
                let device = session_device_id
                    .as_deref()
                    .and_then(|device_id| {
                        devices
                            .iter()
                            .find(|device| device.device_id == device_id)
                            .cloned()
                    })
                    .or_else(|| devices.first().cloned())
                    .ok_or_else(|| "missing current device, register device first".to_string())?;
                let mut state = self
                    .state
                    .lock()
                    .map_err(|_| "app core state poisoned".to_string())?;
                state.current_device = Some(device.clone());
                device
            }
        };
        let target_network = resolve_target_network(
            &networks,
            network_id.as_deref(),
            Some(device.device_id.as_str()),
        )?;
        let node = {
            let existing_node = self
                .state
                .lock()
                .map_err(|_| "app core state poisoned".to_string())?
                .current_node
                .clone();
            match existing_node {
                Some(node) if node.device_id == device.device_id => node,
                _ => self.controller.register_node(
                    &access_token,
                    RegisterNodeRequest {
                        device_id: device.device_id.clone(),
                        node_id: format!("node-{}", now_ms()),
                        node_public_key: format!("node-key-{}", now_ms()),
                        capabilities: vec!["desktop".to_string()],
                    },
                )?,
            }
        };
        let joined = self
            .controller
            .activate_network(
                &access_token,
                JoinNetworkRequest {
                    network_id: target_network.network_id.clone(),
                    device_id: device.device_id.clone(),
                },
            )
            .map_err(|err| format!("enable local network activate failed: {err}"))?;
        let mut bootstrap = self
            .controller
            .bootstrap(&access_token, &node.node_id, &target_network.network_id)
            .map_err(|err| format!("enable local network bootstrap failed: {err}"))?;
        let mut active_network = bootstrap
            .networks
            .iter()
            .find(|network| network.network_id == target_network.network_id)
            .cloned()
            .unwrap_or(target_network);
        let mut active_device = bootstrap.device.clone();
        if active_device.device_id.trim().is_empty() {
            active_device.device_id = device.device_id.clone();
        }
        if active_device.name.trim().is_empty() {
            active_device.name = device.name.clone();
        }
        if active_device.platform.trim().is_empty() {
            active_device.platform = device.platform.clone();
        }
        if active_device.status.trim().is_empty() {
            active_device.status = device.status.clone();
        }
        if active_device.mqtt.is_none() {
            active_device.mqtt = device.mqtt.clone();
        }
        if active_device
            .virtual_ip
            .as_deref()
            .unwrap_or("")
            .trim()
            .is_empty()
        {
            active_device.virtual_ip = device.virtual_ip.clone();
        }
        if active_device
            .virtual_ip
            .as_deref()
            .unwrap_or("")
            .trim()
            .is_empty()
        {
            active_device.virtual_ip = joined.virtual_ip.clone();
        }
        if let Some(virtual_ip) = active_device
            .virtual_ip
            .clone()
            .filter(|value| !value.trim().is_empty())
        {
            for member in &mut active_network.members {
                if member.device_id == active_device.device_id {
                    member.virtual_ip = Some(virtual_ip.clone());
                }
            }
            for network in &mut bootstrap.networks {
                if network.network_id != active_network.network_id {
                    continue;
                }
                for member in &mut network.members {
                    if member.device_id == active_device.device_id {
                        member.virtual_ip = Some(virtual_ip.clone());
                    }
                }
            }
        }
        bootstrap.device = active_device.clone();
        if local_virtual_ip_for(&active_network, &active_device).is_none() {
            let _ = self.tunnel_manager.stop_local_dns();
            let mut state = self
                .state
                .lock()
                .map_err(|_| "app core state poisoned".to_string())?;
            state.current_node = Some(node);
            state.current_device = Some(active_device);
            state.current_bootstrap = Some(bootstrap.clone());
            state.current_network_id = Some(active_network.network_id);
            state.current_connect_plans.clear();
            state.tunnel_peer_virtual_ip = None;
            state.tunnel_runtime = None;
            return Ok(bootstrap);
        }
        let mut config = self.build_local_network_tunnel_config(&active_network, &active_device);
        let dns_records = dns_records_from_bootstrap(&bootstrap);
        if !dns_records.is_empty() {
            config.wireguard_interface.dns_servers = vec!["127.0.0.1".to_string()];
        }
        replace_tunnel(
            &self.current_tunnel_peer_virtual_ip,
            &self.tunnel_manager,
            Some(config.clone()),
        )
        .map_err(|err| format!("enable local network replace tunnel failed: {err}"))?;
        if dns_records.is_empty() {
            let _ = self.tunnel_manager.stop_local_dns();
        } else {
            self.tunnel_manager
                .start_local_dns(dns_records)
                .map_err(|err| format!("enable local network start dns failed: {err}"))?;
        }
        {
            let mut state = self
                .state
                .lock()
                .map_err(|_| "app core state poisoned".to_string())?;
            state.current_node = Some(node);
            state.current_device = Some(active_device.clone());
            state.current_bootstrap = Some(bootstrap.clone());
            state.current_network_id = Some(active_network.network_id.clone());
            state.current_connect_plans.clear();
            state.tunnel_peer_virtual_ip = Some(config.peer_virtual_ip.clone());
            state.tunnel_runtime = Some(build_tunnel_runtime(&config));
        }
        self.controller
            .set_device_network_state(
                &access_token,
                DeviceNetworkStateRequest {
                    device_id: active_device.device_id.clone(),
                    network_id: active_network.network_id.clone(),
                    control_reachable: true,
                    network_online: true,
                    tunnel_up: true,
                    last_probe_ok: true,
                    virtual_ip: active_device.virtual_ip.clone(),
                    reported_at: Some((now_ms() / 1000) as i64),
                },
            )
            .map_err(|err| format!("enable local network report state failed: {err}"))?;
        Ok(bootstrap)
    }

    fn disable_local_network(&self, network_id: Option<String>) -> Result<(), String> {
        let access_token = self.with_access_token()?;
        let (device, current_network_id, peer_virtual_ip) = {
            let state = self
                .state
                .lock()
                .map_err(|_| "app core state poisoned".to_string())?;
            (
                state.current_device.clone(),
                state.current_network_id.clone().or_else(|| {
                    state
                        .current_bootstrap
                        .as_ref()
                        .and_then(|bootstrap| bootstrap.network_map.as_ref())
                        .map(|network_map| network_map.network_id.clone())
                }),
                state.tunnel_peer_virtual_ip.clone(),
            )
        };
        if let Some(peer_virtual_ip) = peer_virtual_ip {
            replace_tunnel(
                &self.current_tunnel_peer_virtual_ip,
                &self.tunnel_manager,
                None,
            )?;
            let _ = self.tunnel_manager.stop_local_dns();
            let mut state = self
                .state
                .lock()
                .map_err(|_| "app core state poisoned".to_string())?;
            if state.tunnel_peer_virtual_ip.as_deref() == Some(peer_virtual_ip.as_str()) {
                state.tunnel_peer_virtual_ip = None;
                state.tunnel_runtime = None;
            }
        }
        let target_network_id = network_id
            .filter(|value| !value.trim().is_empty())
            .or(current_network_id);
        if let (Some(device), Some(network_id)) = (device.as_ref(), target_network_id.as_ref()) {
            let _ = self.controller.set_device_network_state(
                &access_token,
                DeviceNetworkStateRequest {
                    device_id: device.device_id.clone(),
                    network_id: network_id.clone(),
                    control_reachable: true,
                    network_online: false,
                    tunnel_up: false,
                    last_probe_ok: false,
                    virtual_ip: device.virtual_ip.clone(),
                    reported_at: Some((now_ms() / 1000) as i64),
                },
            );
        }
        let mut state = self
            .state
            .lock()
            .map_err(|_| "app core state poisoned".to_string())?;
        state.current_bootstrap = None;
        state.current_network_id = None;
        state.current_connect_plans.clear();
        state.tunnel_peer_virtual_ip = None;
        state.tunnel_runtime = None;
        Ok(())
    }

    fn ensure_local_dns(&self) -> Result<(), String> {
        let (bootstrap, network, device, records) = {
            let state = self
                .state
                .lock()
                .map_err(|_| "app core state poisoned".to_string())?;
            let bootstrap = state
                .current_bootstrap
                .clone()
                .ok_or_else(|| "missing bootstrap config, call bootstrap first".to_string())?;
            let network_id = state
                .current_network_id
                .clone()
                .or_else(|| {
                    bootstrap
                        .network_map
                        .as_ref()
                        .map(|network_map| network_map.network_id.clone())
                })
                .ok_or_else(|| "missing current network".to_string())?;
            let network = bootstrap
                .networks
                .iter()
                .find(|network| network.network_id == network_id)
                .cloned()
                .ok_or_else(|| "missing active network in bootstrap".to_string())?;
            let device = state
                .current_device
                .clone()
                .unwrap_or_else(|| bootstrap.device.clone());
            let records = dns_records_from_bootstrap(&bootstrap);
            (bootstrap, network, device, records)
        };
        if local_virtual_ip_for(&network, &device).is_none() {
            let _ = self.tunnel_manager.stop_local_dns();
            return Ok(());
        }
        if records.is_empty() {
            let _ = self.tunnel_manager.stop_local_dns();
            return Ok(());
        }

        let mut config = self.build_local_network_tunnel_config(&network, &device);
        config.wireguard_interface.dns_servers = vec!["127.0.0.1".to_string()];
        replace_tunnel(
            &self.current_tunnel_peer_virtual_ip,
            &self.tunnel_manager,
            Some(config.clone()),
        )?;
        self.tunnel_manager.start_local_dns(records)?;
        let mut state = self
            .state
            .lock()
            .map_err(|_| "app core state poisoned".to_string())?;
        state.current_bootstrap = Some(bootstrap);
        state.current_network_id = Some(network.network_id);
        state.current_device = Some(device);
        state.tunnel_peer_virtual_ip = Some(config.peer_virtual_ip.clone());
        state.tunnel_runtime = Some(build_tunnel_runtime(&config));
        Ok(())
    }

    fn bootstrap(&self, node_id: String, network_id: String) -> Result<BootstrapConfig, String> {
        let access_token = self.with_access_token()?;
        let bootstrap = self
            .controller
            .bootstrap(&access_token, &node_id, &network_id)?;
        let mut state = self
            .state
            .lock()
            .map_err(|_| "app core state poisoned".to_string())?;
        state.current_bootstrap = Some(bootstrap.clone());
        state.current_network_id = Some(network_id);
        state.current_connect_plans.clear();
        Ok(bootstrap)
    }

    fn control_sync(&self) -> Result<BootstrapConfig, String> {
        let config = match self.control_mqtt_config() {
            Ok(config) => config,
            Err(error)
                if error.contains("missing MQTT credential")
                    || error.contains("missing control session token") =>
            {
                let access_token = self.with_access_token()?;
                let (node_id, network_id) = {
                    let state = self
                        .state
                        .lock()
                        .map_err(|_| "app core state poisoned".to_string())?;
                    let node_id = state
                        .current_node
                        .as_ref()
                        .map(|node| node.node_id.clone())
                        .or_else(|| {
                            state
                                .current_bootstrap
                                .as_ref()
                                .and_then(|bootstrap| bootstrap.network_map.as_ref())
                                .map(|network_map| network_map.self_node_id.clone())
                        })
                        .ok_or_else(|| "missing current node, register node first".to_string())?;
                    let network_id = state
                        .current_network_id
                        .clone()
                        .or_else(|| {
                            state
                                .current_bootstrap
                                .as_ref()
                                .and_then(|bootstrap| bootstrap.network_map.as_ref())
                                .map(|network_map| network_map.network_id.clone())
                        })
                        .ok_or_else(|| "missing current network".to_string())?;
                    (node_id, network_id)
                };
                let bootstrap = self
                    .controller
                    .bootstrap(&access_token, &node_id, &network_id)?;
                let current_device_id = {
                    let state = self
                        .state
                        .lock()
                        .map_err(|_| "app core state poisoned".to_string())?;
                    state
                        .current_device
                        .as_ref()
                        .map(|device| device.device_id.clone())
                        .unwrap_or_else(|| bootstrap.device.device_id.clone())
                };
                if bootstrap_device_attachment_disabled(&bootstrap, &network_id, &current_device_id)
                {
                    replace_tunnel(
                        &self.current_tunnel_peer_virtual_ip,
                        &self.tunnel_manager,
                        None,
                    )?;
                    let _ = self.tunnel_manager.stop_local_dns();
                    update_disconnected_snapshot(&self.state)?;
                    if let Ok(mut control_mqtt) = self.control_mqtt.lock() {
                        *control_mqtt = None;
                    }
                    let mut state = self
                        .state
                        .lock()
                        .map_err(|_| "app core state poisoned".to_string())?;
                    state.current_bootstrap = Some(bootstrap.clone());
                    state.current_network_id = None;
                    state.current_connect_plans.clear();
                    if let Some(current_bootstrap) = state.current_bootstrap.as_mut() {
                        current_bootstrap.network_map = None;
                    }
                    return Ok(bootstrap);
                }
                let mut state = self
                    .state
                    .lock()
                    .map_err(|_| "app core state poisoned".to_string())?;
                state.current_bootstrap = Some(bootstrap.clone());
                state.current_network_id = Some(network_id);
                return Ok(bootstrap);
            }
            Err(error) => return Err(error),
        };
        let current_revision = {
            let state = self
                .state
                .lock()
                .map_err(|_| "app core state poisoned".to_string())?;
            state
                .current_bootstrap
                .as_ref()
                .and_then(|bootstrap| bootstrap.network_map.as_ref())
                .map(|network_map| network_map.revision)
                .unwrap_or(0)
        };
        let mut control_mqtt = self
            .control_mqtt
            .lock()
            .map_err(|_| "app core control mqtt state poisoned".to_string())?;
        let bootstrap = if let Some(client) = control_mqtt.as_mut() {
            match client.ping(now_ms() as i64).and_then(|_| {
                let snapshot = self
                    .state
                    .lock()
                    .map_err(|_| "app core state poisoned".to_string())?
                    .clone();
                Self::emit_control_observations(client, &snapshot, &config)?;
                client.request_network_map_since(&config.network_id, current_revision)
            }) {
                Ok(mut network_map) => {
                    let mut pending_connect_plans = {
                        let state = self
                            .state
                            .lock()
                            .map_err(|_| "app core state poisoned".to_string())?;
                        state.current_connect_plans.clone()
                    };
                    let mut device_ip_updates = Vec::new();
                    let mut active_network_enabled = None;
                    for event in
                        client.drain_pending_events(std::time::Duration::from_millis(200), 32)?
                    {
                        apply_control_mqtt_event(
                            &mut network_map,
                            &mut pending_connect_plans,
                            &mut device_ip_updates,
                            &mut active_network_enabled,
                            event,
                        );
                    }
                    ControlMqttBootstrapUpdate {
                        bootstrap_override: None,
                        network_map,
                        heartbeat_seconds: 0,
                        connect_plans: pending_connect_plans,
                        device_ip_updates,
                        active_network_enabled,
                    }
                }
                Err(_) => {
                    let mut client = ControlMqttClient::connect(&config)?;
                    let mut bootstrap = client.bootstrap_session(&config)?;
                    let snapshot = self
                        .state
                        .lock()
                        .map_err(|_| "app core state poisoned".to_string())?
                        .clone();
                    Self::emit_control_observations(&mut client, &snapshot, &config)?;
                    let mut pending_connect_plans = std::collections::HashMap::new();
                    let mut device_ip_updates = Vec::new();
                    let mut active_network_enabled = None;
                    for event in
                        client.drain_pending_events(std::time::Duration::from_millis(200), 32)?
                    {
                        apply_control_mqtt_event(
                            &mut bootstrap.network_map,
                            &mut pending_connect_plans,
                            &mut device_ip_updates,
                            &mut active_network_enabled,
                            event,
                        );
                    }
                    *control_mqtt = Some(client);
                    ControlMqttBootstrapUpdate {
                        bootstrap_override: None,
                        network_map: bootstrap.network_map,
                        heartbeat_seconds: bootstrap.ack.heartbeat_seconds,
                        connect_plans: pending_connect_plans,
                        device_ip_updates,
                        active_network_enabled,
                    }
                }
            }
        } else {
            let mut client = ControlMqttClient::connect(&config)?;
            let mut bootstrap = client.bootstrap_session(&config)?;
            let snapshot = self
                .state
                .lock()
                .map_err(|_| "app core state poisoned".to_string())?
                .clone();
            Self::emit_control_observations(&mut client, &snapshot, &config)?;
            let mut pending_connect_plans = std::collections::HashMap::new();
            let mut device_ip_updates = Vec::new();
            let mut active_network_enabled = None;
            for event in client.drain_pending_events(std::time::Duration::from_millis(200), 32)? {
                apply_control_mqtt_event(
                    &mut bootstrap.network_map,
                    &mut pending_connect_plans,
                    &mut device_ip_updates,
                    &mut active_network_enabled,
                    event,
                );
            }
            *control_mqtt = Some(client);
            ControlMqttBootstrapUpdate {
                bootstrap_override: None,
                network_map: bootstrap.network_map,
                heartbeat_seconds: bootstrap.ack.heartbeat_seconds,
                connect_plans: pending_connect_plans,
                device_ip_updates,
                active_network_enabled,
            }
        };
        let bootstrap = if let Some(enabled) = bootstrap.active_network_enabled.as_ref() {
            let current_network_id = {
                let state = self
                    .state
                    .lock()
                    .map_err(|_| "app core state poisoned".to_string())?;
                state.current_network_id.clone()
            };
            if current_network_id.as_deref() != Some(enabled.network_id.as_str()) {
                let access_token = self.with_access_token()?;
                let device_id = {
                    let state = self
                        .state
                        .lock()
                        .map_err(|_| "app core state poisoned".to_string())?;
                    state
                        .current_device
                        .as_ref()
                        .map(|device| device.device_id.clone())
                        .ok_or_else(|| {
                            "missing current device, register device first".to_string()
                        })?
                };
                self.controller.activate_network(
                    &access_token,
                    JoinNetworkRequest {
                        network_id: enabled.network_id.clone(),
                        device_id,
                    },
                )?;
                let refreshed = self.controller.bootstrap(
                    &access_token,
                    &config.node_id,
                    &enabled.network_id,
                )?;
                ControlMqttBootstrapUpdate {
                    bootstrap_override: Some(refreshed.clone()),
                    network_map: refreshed.network_map.clone().ok_or_else(|| {
                        "missing network map after active network enable".to_string()
                    })?,
                    heartbeat_seconds: refreshed.control_plane.heartbeat_seconds,
                    connect_plans: std::collections::HashMap::new(),
                    device_ip_updates: vec![],
                    active_network_enabled: Some(enabled.clone()),
                }
            } else {
                bootstrap
            }
        } else {
            bootstrap
        };
        let (
            updated_bootstrap,
            tunnel_reapply,
            local_tunnel_reapply,
            should_disable_current_network,
        ) = {
            let mut state = self
                .state
                .lock()
                .map_err(|_| "app core state poisoned".to_string())?;
            let active_path = state.active_path.clone();
            let current_device_id = state
                .current_device
                .as_ref()
                .map(|device| device.device_id.clone());
            let current_bootstrap = state
                .current_bootstrap
                .as_mut()
                .ok_or_else(|| "missing bootstrap config, call bootstrap first".to_string())?;
            if let Some(override_bootstrap) = bootstrap.bootstrap_override.clone() {
                *current_bootstrap = override_bootstrap;
            }
            let override_device = current_bootstrap.device.clone();
            let previous_network_map = current_bootstrap.network_map.clone();
            let dns_changed =
                dns_config_changed(previous_network_map.as_ref(), &bootstrap.network_map);
            if bootstrap.heartbeat_seconds > 0 {
                current_bootstrap.control_plane.heartbeat_seconds = bootstrap.heartbeat_seconds;
            }
            current_bootstrap.network_map = Some(bootstrap.network_map.clone());
            let mut should_reapply_tunnel = dns_changed;
            let mut updated_current_device_virtual_ip: Option<String> = None;
            for update in &bootstrap.device_ip_updates {
                if update.network_id != bootstrap.network_map.network_id {
                    continue;
                }
                for network in &mut current_bootstrap.networks {
                    if network.network_id != update.network_id {
                        continue;
                    }
                    for member in &mut network.members {
                        if member.device_id == update.device_id {
                            member.virtual_ip = if update.virtual_ip.trim().is_empty() {
                                None
                            } else {
                                Some(update.virtual_ip.clone())
                            };
                        }
                    }
                }
                if current_bootstrap.device.device_id == update.device_id {
                    current_bootstrap.device.virtual_ip = if update.virtual_ip.trim().is_empty() {
                        None
                    } else {
                        Some(update.virtual_ip.clone())
                    };
                    should_reapply_tunnel = true;
                }
                if current_device_id.as_deref() == Some(update.device_id.as_str()) {
                    updated_current_device_virtual_ip = Some(update.virtual_ip.clone());
                }
            }
            let current_device_attachment_disabled = current_device_id
                .as_deref()
                .and_then(|device_id| {
                    current_bootstrap
                        .networks
                        .iter()
                        .find(|network| network.network_id == bootstrap.network_map.network_id)
                        .and_then(|network| {
                            network
                                .members
                                .iter()
                                .find(|member| member.device_id == device_id)
                        })
                        .and_then(|member| member.status.as_deref())
                })
                .map(is_disabled_member_status)
                .unwrap_or(false);
            let should_disable_current_network = current_device_attachment_disabled
                || updated_current_device_virtual_ip
                    .as_ref()
                    .map(|virtual_ip| virtual_ip.trim().is_empty())
                    .unwrap_or(false);
            let updated_bootstrap = current_bootstrap.clone();
            let local_tunnel_reapply = if should_reapply_tunnel {
                let current_device = updated_bootstrap.device.clone();
                updated_bootstrap
                    .networks
                    .iter()
                    .find(|network| network.network_id == bootstrap.network_map.network_id)
                    .cloned()
                    .map(|network| (network, current_device))
            } else {
                None
            };
            let tunnel_reapply = match active_path.clone() {
                Some(ActivePath::P2P { ref peer_node_id })
                | Some(ActivePath::Relay { ref peer_node_id }) => {
                    let peer_update = bootstrap
                        .device_ip_updates
                        .iter()
                        .find(|update| {
                            updated_bootstrap
                                .network_map
                                .as_ref()
                                .and_then(|network_map| {
                                    network_map
                                        .peers
                                        .iter()
                                        .find(|peer| peer.node_id == *peer_node_id)
                                        .map(|peer| peer.device_id.as_str())
                                })
                                .or_else(|| {
                                    previous_network_map.as_ref().and_then(|network_map| {
                                        network_map
                                            .peers
                                            .iter()
                                            .find(|peer| peer.node_id == *peer_node_id)
                                            .map(|peer| peer.device_id.as_str())
                                    })
                                })
                                == Some(update.device_id.as_str())
                        })
                        .cloned();
                    let peer = updated_bootstrap
                        .network_map
                        .as_ref()
                        .and_then(|network_map| {
                            network_map
                                .peers
                                .iter()
                                .find(|peer| peer.node_id == *peer_node_id)
                                .cloned()
                        })
                        .or_else(|| {
                            previous_network_map.as_ref().and_then(|network_map| {
                                network_map
                                    .peers
                                    .iter()
                                    .find(|peer| peer.node_id == *peer_node_id)
                                    .cloned()
                            })
                        })
                        .map(|mut peer| {
                            if let Some(update) = peer_update.as_ref() {
                                peer.virtual_ips = vec![update.virtual_ip.clone()];
                            }
                            peer
                        });
                    if let Some(peer) = peer {
                        if peer_update.is_some()
                            || bootstrap
                                .device_ip_updates
                                .iter()
                                .any(|update| update.device_id == peer.device_id)
                        {
                            should_reapply_tunnel = true;
                        }
                        if should_reapply_tunnel {
                            Some((
                                active_path.clone().unwrap_or(ActivePath::None),
                                updated_bootstrap.clone(),
                                peer,
                            ))
                        } else {
                            None
                        }
                    } else {
                        None
                    }
                }
                _ => None,
            };
            if let Some(virtual_ip) = updated_current_device_virtual_ip {
                if let Some(device) = state.current_device.as_mut() {
                    device.virtual_ip = if virtual_ip.trim().is_empty() {
                        None
                    } else {
                        Some(virtual_ip)
                    };
                }
            } else if bootstrap.bootstrap_override.is_some() {
                state.current_device = Some(override_device);
            }
            if should_disable_current_network {
                state.current_network_id = None;
                state.current_connect_plans.clear();
                if let Some(current_bootstrap) = state.current_bootstrap.as_mut() {
                    current_bootstrap.network_map = None;
                }
            } else {
                state.current_network_id = Some(bootstrap.network_map.network_id.clone());
                state.current_connect_plans = bootstrap.connect_plans.clone();
            }
            (
                updated_bootstrap,
                if should_disable_current_network {
                    None
                } else {
                    tunnel_reapply
                },
                if should_disable_current_network {
                    None
                } else {
                    local_tunnel_reapply
                },
                should_disable_current_network,
            )
        };
        if should_disable_current_network {
            replace_tunnel(
                &self.current_tunnel_peer_virtual_ip,
                &self.tunnel_manager,
                None,
            )?;
            let _ = self.tunnel_manager.stop_local_dns();
            update_disconnected_snapshot(&self.state)?;
            if let Ok(mut control_mqtt) = self.control_mqtt.lock() {
                *control_mqtt = None;
            }
            return Ok(updated_bootstrap);
        }
        match tunnel_reapply {
            Some((active_path, bootstrap, peer)) => {
                if let Some(config) = self.build_tunnel_config(&active_path, &bootstrap, &peer)? {
                    replace_tunnel(
                        &self.current_tunnel_peer_virtual_ip,
                        &self.tunnel_manager,
                        Some(config.clone()),
                    )?;
                    let dns_records = dns_records_from_bootstrap(&bootstrap);
                    if dns_records.is_empty() {
                        let _ = self.tunnel_manager.stop_local_dns();
                    } else {
                        self.tunnel_manager.start_local_dns(dns_records)?;
                    }
                    let mut state = self
                        .state
                        .lock()
                        .map_err(|_| "app core state poisoned".to_string())?;
                    state.tunnel_peer_virtual_ip = Some(config.peer_virtual_ip.clone());
                    state.tunnel_runtime = Some(build_tunnel_runtime(&config));
                }
            }
            None => {
                let Some((network, device)) = local_tunnel_reapply else {
                    return Ok(updated_bootstrap);
                };
                if local_virtual_ip_for(&network, &device).is_none() {
                    let _ = self.tunnel_manager.stop_local_dns();
                    let mut state = self
                        .state
                        .lock()
                        .map_err(|_| "app core state poisoned".to_string())?;
                    state.tunnel_peer_virtual_ip = None;
                    state.tunnel_runtime = None;
                    return Ok(updated_bootstrap);
                }
                let mut config = self.build_local_network_tunnel_config(&network, &device);
                let dns_records = dns_records_from_bootstrap(&updated_bootstrap);
                if !dns_records.is_empty() {
                    config.wireguard_interface.dns_servers = vec!["127.0.0.1".to_string()];
                }
                replace_tunnel(
                    &self.current_tunnel_peer_virtual_ip,
                    &self.tunnel_manager,
                    Some(config.clone()),
                )?;
                if dns_records.is_empty() {
                    let _ = self.tunnel_manager.stop_local_dns();
                } else {
                    self.tunnel_manager.start_local_dns(dns_records)?;
                }
                let mut state = self
                    .state
                    .lock()
                    .map_err(|_| "app core state poisoned".to_string())?;
                state.tunnel_peer_virtual_ip = Some(config.peer_virtual_ip.clone());
                state.tunnel_runtime = Some(build_tunnel_runtime(&config));
            }
        }
        Ok(updated_bootstrap)
    }

    fn control_status(&self) -> Result<ControlStatusView, String> {
        let state = self
            .state
            .lock()
            .map_err(|_| "app core state poisoned".to_string())?;
        let Some(bootstrap) = state.current_bootstrap.as_ref() else {
            return Ok(ControlStatusView {
                status: "none".to_string(),
                ws_url: None,
                heartbeat_seconds: None,
                session_token_present: false,
                network_map_present: false,
                network_id: state.current_network_id.clone(),
                node_id: state.current_node.as_ref().map(|node| node.node_id.clone()),
                device_id: state
                    .current_device
                    .as_ref()
                    .map(|device| device.device_id.clone()),
                peer_count: 0,
                connect_plan_count: state.current_connect_plans.len(),
                connect_plans: vec![],
            });
        };

        let network_map = bootstrap.network_map.as_ref();
        let mut connect_plans = state
            .current_connect_plans
            .iter()
            .map(|(peer_node_id, plan)| ControlConnectPlanView {
                peer_node_id: peer_node_id.clone(),
                prefer_direct: plan.prefer_direct,
                path_count: plan.paths.len(),
                preferred_path: plan.paths.first().map(|path| ControlPathOptionView {
                    path_type: path.path_type.clone(),
                    endpoint: path.endpoint.clone(),
                    priority: path.priority,
                }),
                derp_cluster_id: if plan.derp_cluster_id.trim().is_empty() {
                    None
                } else {
                    Some(plan.derp_cluster_id.clone())
                },
                preferred_derp_node_ids: plan.preferred_derp_node_ids.clone(),
                relay_ticket_id: plan
                    .relay_ticket
                    .as_ref()
                    .map(|ticket| ticket.ticket_id.clone()),
            })
            .collect::<Vec<_>>();
        connect_plans.sort_by(|left, right| left.peer_node_id.cmp(&right.peer_node_id));

        Ok(ControlStatusView {
            status: "configured".to_string(),
            ws_url: Some(bootstrap.control_plane.ws_url.clone()),
            heartbeat_seconds: Some(bootstrap.control_plane.heartbeat_seconds),
            session_token_present: bootstrap.control_plane.session_token.is_some(),
            network_map_present: network_map.is_some(),
            network_id: state
                .current_network_id
                .clone()
                .or_else(|| network_map.map(|map| map.network_id.clone())),
            node_id: state
                .current_node
                .as_ref()
                .map(|node| node.node_id.clone())
                .or_else(|| network_map.map(|map| map.self_node_id.clone())),
            device_id: state
                .current_device
                .as_ref()
                .map(|device| device.device_id.clone())
                .or_else(|| network_map.map(|map| map.self_device_id.clone())),
            peer_count: network_map.map(|map| map.peers.len()).unwrap_or(0),
            connect_plan_count: connect_plans.len(),
            connect_plans,
        })
    }

    fn issue_relay_ticket(
        &self,
        network_id: String,
        src_node_id: String,
        dst_node_id: String,
        derp_cluster_id: Option<String>,
        preferred_derp_node_ids: Vec<String>,
        reason: String,
        relay_region_id: Option<String>,
    ) -> Result<RelayTicket, String> {
        self.issue_relay_ticket_with_options(
            network_id,
            src_node_id,
            dst_node_id,
            derp_cluster_id,
            preferred_derp_node_ids,
            reason,
            relay_region_id,
        )
    }

    fn connect(&self, network_id: String, peer_node_id: String) -> Result<ConnectionState, String> {
        let context = resolve_connect_context(&self.state, &peer_node_id)?;
        let bootstrap = context.bootstrap;
        let src_node_id = context.src_node_id;
        let peer = context.peer;
        let connect_plan = self.connect_plan_for_peer(&peer.node_id)?;

        let ordered_endpoints = Self::ordered_peer_endpoints(&peer, connect_plan.as_ref());
        if let Some(endpoint) = ordered_endpoints.first() {
            let p2p_state = self.p2p.connect(&PeerCandidate {
                peer_node_id: peer.node_id.clone(),
                endpoint: endpoint.address.clone(),
                candidate_type: endpoint.endpoint_type.clone(),
            })?;
            if matches!(p2p_state, ConnectionState::Connected(ConnectionPath::P2P)) {
                self.path_manager
                    .on_p2p_recovered(&peer.node_id)
                    .map_err(|err| err.to_string())?;
                let active_path = self.path_manager.current_path();
                let tunnel_config = self.build_tunnel_config(&active_path, &bootstrap, &peer)?;
                finalize_connected_path(
                    &self.state,
                    &self.current_tunnel_peer_virtual_ip,
                    &self.tunnel_manager,
                    p2p_state.clone(),
                    active_path,
                    tunnel_config,
                )?;
                return Ok(p2p_state);
            }
            if let ConnectionState::Failed(reason) = &p2p_state {
                self.path_manager
                    .on_p2p_failed(&peer.node_id, reason)
                    .map_err(|err| err.to_string())?;
            }
        }

        if !peer.relay_allowed {
            return Err(format!(
                "peer {} does not allow relay fallback",
                peer.node_id
            ));
        }

        let fallback_state = match self.connect_via_derp_pool(
            &bootstrap,
            network_id.clone(),
            src_node_id.clone(),
            peer.node_id.clone(),
        )? {
            Some(state) => state,
            None => {
                let ticket = self.relay_ticket_from_plan(
                    network_id,
                    src_node_id,
                    peer.node_id.clone(),
                    connect_plan.as_ref(),
                )?;
                let relay_state = self.relay.connect(&ticket).map_err(|err| err.to_string())?;
                if matches!(
                    relay_state,
                    ConnectionState::Connected(ConnectionPath::Relay)
                ) {
                    self.path_manager
                        .on_p2p_failed(&peer.node_id, "relay_fallback")
                        .map_err(|err| err.to_string())?;
                }
                relay_state
            }
        };
        if matches!(
            fallback_state,
            ConnectionState::Connected(ConnectionPath::Derp)
        ) {
            self.path_manager
                .on_p2p_failed(&peer.node_id, "derp_fallback")
                .map_err(|err| err.to_string())?;
        }
        let managed_state = connection_state_from_active_path(self.path_manager.current_path());
        if matches!(managed_state, ConnectionState::Connected(_)) {
            let active_path = self.path_manager.current_path();
            let tunnel_config = self.build_tunnel_config(&active_path, &bootstrap, &peer)?;
            finalize_connected_path(
                &self.state,
                &self.current_tunnel_peer_virtual_ip,
                &self.tunnel_manager,
                managed_state.clone(),
                active_path,
                tunnel_config,
            )?;
            return Ok(managed_state);
        }
        update_connected_snapshot(
            &self.state,
            managed_state.clone(),
            self.path_manager.current_path(),
            None,
            None,
        )?;
        Ok(managed_state)
    }

    fn probe_with_timeout(
        &self,
        packet: Vec<u8>,
        reply_timeout_ms: Option<u64>,
    ) -> Result<DataPlaneProbe, DataPlaneError> {
        let active_path = self.path_manager.current_path();
        self.path_manager
            .send_transport_packet(&packet)
            .map_err(classify_path_manager_error)
            .inspect_err(|error| {
                let _ = remember_data_plane_error(&self.state, error);
            })?;
        let sampled_at_ms = now_ms();
        let probe_id = self
            .next_probe_id(sampled_at_ms)
            .map_err(|err| DataPlaneError::new(DataPlaneErrorCode::Unknown, err))?;
        let reply = poll_reply_until_timeout(
            &self.path_manager,
            reply_timeout_ms.unwrap_or(self.default_probe_reply_timeout_ms),
            Self::PROBE_REPLY_POLL_INTERVAL_MS,
        )
        .map_err(classify_path_manager_error)?;
        let (observed_rtt_ms, packet_loss_ppm, path_score, derp_cluster_id, derp_node_id) =
            probe_health_from_active_path(&self.derp_pool, &active_path);
        let tunnel_peer_virtual_ip = self
            .current_tunnel_peer_virtual_ip
            .lock()
            .map_err(|_| {
                DataPlaneError::new(
                    DataPlaneErrorCode::Unknown,
                    "app core tunnel state poisoned",
                )
            })?
            .clone();
        let (reply_observed, reply_bytes_received, reply_sampled_at_ms, reply_rtt_ms) = match reply
        {
            Some((reply_len, reply_sampled_at_ms)) => (
                true,
                Some(reply_len),
                Some(reply_sampled_at_ms),
                Some(reply_sampled_at_ms.saturating_sub(sampled_at_ms)),
            ),
            None => (false, None, None, None),
        };
        let probe = DataPlaneProbe {
            probe_id,
            sampled_at_ms,
            active_path,
            bytes_sent: packet.len(),
            reply_observed,
            reply_bytes_received,
            reply_sampled_at_ms,
            reply_rtt_ms,
            tunnel_peer_virtual_ip,
            observed_rtt_ms,
            packet_loss_ppm,
            path_score,
            derp_cluster_id,
            derp_node_id,
        };
        record_probe_result(&self.state, probe)
    }

    fn send(&self, packet: Vec<u8>) -> Result<usize, DataPlaneError> {
        self.probe(packet).map(|probe| probe.bytes_sent)
    }

    fn disconnect(&self) -> Result<(), String> {
        let mut control_mqtt = self
            .control_mqtt
            .lock()
            .map_err(|_| "app core control mqtt state poisoned".to_string())?;
        *control_mqtt = None;
        let peer_virtual_ip = self
            .current_tunnel_peer_virtual_ip
            .lock()
            .map_err(|_| "app core tunnel state poisoned".to_string())?
            .take();
        if let Some(peer_virtual_ip) = peer_virtual_ip {
            self.tunnel_manager.close(&peer_virtual_ip)?;
        }
        let _ = self.tunnel_manager.stop_local_dns();
        let mut state = self
            .state
            .lock()
            .map_err(|_| "app core state poisoned".to_string())?;
        state.session = None;
        state.current_device = None;
        state.current_node = None;
        state.current_bootstrap = None;
        state.current_network_id = None;
        state.current_connect_plans.clear();
        drop(state);
        update_disconnected_snapshot(&self.state)
    }
}

struct ControlMqttBootstrapUpdate {
    bootstrap_override: Option<BootstrapConfig>,
    network_map: slan_app_core::NetworkMap,
    heartbeat_seconds: u32,
    connect_plans: std::collections::HashMap<String, ControlMqttConnectPlan>,
    device_ip_updates: Vec<ControlMqttDeviceIPReassigned>,
    active_network_enabled: Option<ControlMqttActiveNetworkEnabled>,
}

fn resolve_target_network(
    networks: &[Network],
    requested_network_id: Option<&str>,
    device_id: Option<&str>,
) -> Result<Network, String> {
    let requested = requested_network_id
        .map(str::trim)
        .filter(|value| !value.is_empty());
    if let Some(network_id) = requested {
        return networks
            .iter()
            .find(|network| network.network_id == network_id)
            .cloned()
            .ok_or_else(|| format!("network not found: {network_id}"));
    }
    if let Some(device_id) = device_id {
        if let Some(network) = networks.iter().find(|network| {
            network
                .members
                .iter()
                .any(|member| member.device_id == device_id)
        }) {
            return Ok(network.clone());
        }
    }
    networks
        .first()
        .cloned()
        .ok_or_else(|| "no network is available".to_string())
}

fn local_virtual_ip_for(network: &Network, device: &Device) -> Option<String> {
    device
        .virtual_ip
        .clone()
        .filter(|value| !value.trim().is_empty())
        .or_else(|| {
            network
                .members
                .iter()
                .find(|member| member.device_id == device.device_id)
                .and_then(|member| member.virtual_ip.clone())
                .filter(|value| !value.trim().is_empty())
        })
}

fn peer_virtual_ip_for(network: &Network, device_id: &str, local_virtual_ip: &str) -> String {
    network
        .members
        .iter()
        .find(|member| {
            member.device_id != device_id
                && member
                    .virtual_ip
                    .as_ref()
                    .map(|value| !value.trim().is_empty())
                    .unwrap_or(false)
        })
        .and_then(|member| member.virtual_ip.clone())
        .unwrap_or_else(|| {
            if local_virtual_ip == "10.0.0.2" {
                "10.0.0.3".to_string()
            } else {
                "10.0.0.2".to_string()
            }
        })
}

fn prefix_len_from_cidr(cidr: &str) -> Option<u8> {
    let (_, prefix) = cidr.trim().rsplit_once('/')?;
    let parsed = prefix.trim().parse::<u8>().ok()?;
    (parsed <= 32).then_some(parsed)
}

fn default_tunnel_endpoint() -> String {
    std::env::var("SLAN_DEV_TUNNEL_ENDPOINT")
        .ok()
        .map(|value| value.trim().to_string())
        .filter(|value| !value.is_empty())
        .unwrap_or_else(|| "203.0.113.10:51820".to_string())
}

fn is_remote_network_disabled_error(error: &str) -> bool {
    let normalized = error.to_ascii_lowercase();
    normalized.contains("forbidden")
        && (normalized.contains("no active network attachment")
            || normalized.contains("has no active network attachment")
            || normalized.contains("device unavailable"))
}

fn is_disabled_member_status(status: &str) -> bool {
    matches!(
        status.trim().to_ascii_lowercase().as_str(),
        "disabled" | "suspended" | "rejected"
    )
}

fn bootstrap_device_attachment_disabled(
    bootstrap: &BootstrapConfig,
    network_id: &str,
    device_id: &str,
) -> bool {
    bootstrap
        .networks
        .iter()
        .find(|network| network.network_id == network_id)
        .and_then(|network| {
            network
                .members
                .iter()
                .find(|member| member.device_id == device_id)
        })
        .and_then(|member| member.status.as_deref())
        .map(is_disabled_member_status)
        .unwrap_or(false)
}

fn dns_records_from_bootstrap(bootstrap: &BootstrapConfig) -> Vec<(String, String)> {
    bootstrap
        .network_map
        .as_ref()
        .map(|network_map| {
            network_map
                .dns
                .wildcards
                .iter()
                .filter_map(|wildcard| parse_dns_wildcard(wildcard))
                .collect()
        })
        .unwrap_or_default()
}

fn dns_config_changed(previous: Option<&NetworkMap>, next: &NetworkMap) -> bool {
    let Some(previous) = previous else {
        return !next.dns.servers.is_empty()
            || !next.dns.search_domains.is_empty()
            || !next.dns.wildcards.is_empty();
    };
    previous.dns.servers != next.dns.servers
        || previous.dns.search_domains != next.dns.search_domains
        || previous.dns.wildcards != next.dns.wildcards
}

fn parse_dns_wildcard(value: &str) -> Option<(String, String)> {
    let (host, ip) = value.split_once('=')?;
    let mut host = host.trim().to_ascii_lowercase();
    if host.starts_with('*') && !host.starts_with("*.") {
        host = format!("*.{}", host.trim_start_matches('*').trim_start_matches('.'));
    }
    let ip = ip.trim().to_string();
    if host.is_empty() || ip.is_empty() {
        return None;
    }
    Some((host, ip))
}

fn apply_control_mqtt_event(
    network_map: &mut slan_app_core::NetworkMap,
    connect_plans: &mut std::collections::HashMap<String, ControlMqttConnectPlan>,
    device_ip_updates: &mut Vec<ControlMqttDeviceIPReassigned>,
    active_network_enabled: &mut Option<ControlMqttActiveNetworkEnabled>,
    event: ControlMqttEvent,
) {
    match event {
        ControlMqttEvent::PeerUpdate(update) => {
            network_map.revision = network_map.revision.max(update.revision);
            if let Some(existing) = network_map
                .peers
                .iter_mut()
                .find(|peer| peer.node_id == update.peer.node_id)
            {
                *existing = update.peer;
            } else {
                network_map.peers.push(update.peer);
            }
        }
        ControlMqttEvent::PeerRemove(remove) => {
            network_map.revision = network_map.revision.max(remove.revision);
            network_map
                .peers
                .retain(|peer| peer.node_id != remove.peer_node_id);
            connect_plans.remove(&remove.peer_node_id);
        }
        ControlMqttEvent::ConnectPlan(plan) => {
            connect_plans.insert(plan.peer_node_id.clone(), plan);
        }
        ControlMqttEvent::NetworkRestartRequired(_restart) => {}
        ControlMqttEvent::DeviceIPReassigned(update) => {
            if update.network_id == network_map.network_id {
                for peer in network_map
                    .peers
                    .iter_mut()
                    .filter(|peer| peer.device_id == update.device_id)
                {
                    peer.virtual_ips = vec![update.virtual_ip.clone()];
                }
            }
            device_ip_updates.push(update);
        }
        ControlMqttEvent::ActiveNetworkEnabled(enabled) => {
            *active_network_enabled = Some(enabled);
        }
    }
}
