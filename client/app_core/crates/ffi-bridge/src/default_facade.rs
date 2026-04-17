use std::sync::Mutex;

use controller_client::{
    ControllerClient, CreateNetworkRequest, LoginRequest, RegisterDeviceRequest,
    RegisterNodeRequest, RegisterRequest, RelayTicketRequest,
};
use p2p::{P2PConnector, PeerCandidate};
use relay_client::{DerpPool, RelayClient};
use slan_app_core::{
    BootstrapConfig, ConnectionPath, ConnectionState, DerpCluster, Device, Network, Node,
    RelayTicket, Session,
};

use crate::facade::AppCoreFacade;

#[derive(Default)]
struct FacadeState {
    session: Option<Session>,
    current_device: Option<Device>,
    current_node: Option<Node>,
    current_bootstrap: Option<BootstrapConfig>,
    current_network_id: Option<String>,
    connection_state: Option<ConnectionState>,
}

pub struct DefaultAppCoreFacade<C, P, R, D>
where
    C: ControllerClient,
    P: P2PConnector,
    R: RelayClient,
    D: DerpPool,
{
    controller: C,
    p2p: P,
    relay: R,
    derp_pool: D,
    state: Mutex<FacadeState>,
}

impl<C, P, R, D> DefaultAppCoreFacade<C, P, R, D>
where
    C: ControllerClient,
    P: P2PConnector,
    R: RelayClient,
    D: DerpPool,
{
    pub fn new(controller: C, p2p: P, relay: R, derp_pool: D) -> Self {
        Self {
            controller,
            p2p,
            relay,
            derp_pool,
            state: Mutex::new(FacadeState::default()),
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
}

impl<C, P, R, D> AppCoreFacade for DefaultAppCoreFacade<C, P, R, D>
where
    C: ControllerClient,
    P: P2PConnector,
    R: RelayClient,
    D: DerpPool,
{
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
        let session = self.controller.login(LoginRequest { email, password })?;
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
        Ok(device)
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
        Ok(node)
    }

    fn list_networks(&self) -> Result<Vec<Network>, String> {
        let access_token = self.with_access_token()?;
        self.controller.list_networks(&access_token)
    }

    fn create_network(&self, name: String, cidr: String) -> Result<Network, String> {
        let access_token = self.with_access_token()?;
        self.controller
            .create_network(&access_token, CreateNetworkRequest { name, cidr })
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
        Ok(bootstrap)
    }

    fn issue_relay_ticket(
        &self,
        network_id: String,
        src_node_id: String,
        dst_node_id: String,
        reason: String,
    ) -> Result<RelayTicket, String> {
        self.issue_relay_ticket_with_options(
            network_id,
            src_node_id,
            dst_node_id,
            None,
            vec![],
            reason,
        )
    }

    fn connect(&self, network_id: String, peer_node_id: String) -> Result<ConnectionState, String> {
        let (bootstrap, src_node_id) = {
            let state = self
                .state
                .lock()
                .map_err(|_| "app core state poisoned".to_string())?;
            (
                state.current_bootstrap.clone(),
                state.current_node.as_ref().map(|node| node.node_id.clone()),
            )
        };
        let bootstrap = bootstrap
            .ok_or_else(|| "missing bootstrap config, call bootstrap first".to_string())?;
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
            .ok_or_else(|| format!("peer node not found in network map: {}", peer_node_id))?;

        if let Some(endpoint) = peer.endpoints.first() {
            let p2p_state = self.p2p.connect(&PeerCandidate {
                peer_node_id: peer.node_id.clone(),
                endpoint: endpoint.address.clone(),
                candidate_type: endpoint.endpoint_type.clone(),
            })?;
            if matches!(p2p_state, ConnectionState::Connected(ConnectionPath::P2P)) {
                let mut state = self
                    .state
                    .lock()
                    .map_err(|_| "app core state poisoned".to_string())?;
                state.connection_state = Some(p2p_state.clone());
                return Ok(p2p_state);
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
                let ticket = self.issue_relay_ticket(
                    network_id,
                    src_node_id,
                    peer.node_id.clone(),
                    "p2p_failed".to_string(),
                )?;
                self.relay.connect(&ticket)?
            }
        };
        let mut state = self
            .state
            .lock()
            .map_err(|_| "app core state poisoned".to_string())?;
        state.connection_state = Some(fallback_state.clone());
        Ok(fallback_state)
    }

    fn disconnect(&self) -> Result<(), String> {
        let mut state = self
            .state
            .lock()
            .map_err(|_| "app core state poisoned".to_string())?;
        state.connection_state = Some(ConnectionState::Disconnected);
        Ok(())
    }
}
