use std::sync::{Arc, Mutex};

use controller_client::{
    ControllerClient, CreateNetworkRequest, JoinNetworkRequest, LoginRequest,
    RegisterDeviceRequest, RegisterNodeRequest, RegisterRequest, RelayTicketRequest,
};
use ffi_bridge::{AppCoreFacade, DefaultAppCoreFacade};
use p2p::{P2PConnector, PeerCandidate};
use relay_client::{DerpPool, PathManager, RelayClient};
use slan_app_core::{
    ActivePath, BootstrapConfig, ConnectionPath, ConnectionState, ControlPlaneConfig, DerpCluster,
    DerpHealth, DerpLinkSnapshot, DerpLinkState, DerpMap, DerpNodeMeta, DerpPoolState,
    DerpSwitchEvent, DerpTransport, Device, DnsConfig, Endpoint, Network, NetworkMap, Node, Peer,
    RelayCity, RelayCluster, RelayConfig, RelayCountry, RelayNode, RelayTicket, Session,
    SwitchReason,
};
use tunnel::{TunnelConfig, TunnelManager};

struct FakeController {
    relay_reason: Mutex<Option<String>>,
    with_derp_map: bool,
    device_virtual_ip: Option<String>,
}

impl FakeController {
    fn new(with_derp_map: bool) -> Self {
        Self {
            relay_reason: Mutex::new(None),
            with_derp_map,
            device_virtual_ip: Some("100.64.0.10".into()),
        }
    }

    fn without_device_virtual_ip(with_derp_map: bool) -> Self {
        Self {
            relay_reason: Mutex::new(None),
            with_derp_map,
            device_virtual_ip: None,
        }
    }
}

impl ControllerClient for FakeController {
    fn register(&self, _req: RegisterRequest) -> Result<Session, String> {
        Ok(Session {
            user_id: "user-1".into(),
            access_token: "token-1".into(),
            refresh_token: None,
            expires_in: 3600,
            device_id: None,
        })
    }

    fn login(&self, _req: LoginRequest) -> Result<Session, String> {
        self.register(RegisterRequest {
            email: String::new(),
            password: String::new(),
        })
    }

    fn register_device(
        &self,
        _access_token: &str,
        _req: RegisterDeviceRequest,
    ) -> Result<Device, String> {
        Ok(Device {
            device_id: "dev-1".into(),
            name: "mac".into(),
            platform: "macos".into(),
            status: "online".into(),
            virtual_ip: None,
            public_key: Some("device-pk".into()),
        })
    }

    fn register_node(&self, _access_token: &str, req: RegisterNodeRequest) -> Result<Node, String> {
        Ok(Node {
            node_id: req.node_id,
            device_id: req.device_id,
            node_public_key: req.node_public_key,
            network_ids: vec!["net-1".into()],
            capabilities: req.capabilities,
        })
    }

    fn list_networks(&self, _access_token: &str) -> Result<Vec<Network>, String> {
        Ok(vec![])
    }

    fn create_network(
        &self,
        _access_token: &str,
        req: CreateNetworkRequest,
    ) -> Result<Network, String> {
        Ok(Network {
            network_id: "net-1".into(),
            name: req.name,
            cidr: req.cidr,
            members: vec![],
        })
    }

    fn join_network(&self, _access_token: &str, _req: JoinNetworkRequest) -> Result<(), String> {
        Ok(())
    }

    fn bootstrap(
        &self,
        _access_token: &str,
        node_id: &str,
        network_id: &str,
    ) -> Result<BootstrapConfig, String> {
        Ok(BootstrapConfig {
            device: Device {
                device_id: "dev-1".into(),
                name: "mac".into(),
                platform: "macos".into(),
                status: "online".into(),
                virtual_ip: self.device_virtual_ip.clone(),
                public_key: Some("device-pk".into()),
            },
            networks: vec![],
            control_plane: ControlPlaneConfig {
                ws_url: "ws://127.0.0.1:8080/control/ws".into(),
                heartbeat_seconds: 15,
            },
            stun_servers: vec!["stun:127.0.0.1:3478".into()],
            relay: RelayConfig {
                default_cluster_id: "cn-local-a".into(),
                countries: vec![RelayCountry {
                    country_code: "CN".into(),
                    country_name: "China".into(),
                    cities: vec![RelayCity {
                        city_code: "local".into(),
                        city_name: "Local".into(),
                        clusters: vec![RelayCluster {
                            cluster_id: "cn-local-a".into(),
                            cluster_name: "CN Local A".into(),
                            nodes: vec![
                                RelayNode {
                                    node_id: "relay-cn-local-udp".into(),
                                    transport: "udp".into(),
                                    address: "127.0.0.1:9000".into(),
                                    priority: 10,
                                    tags: vec![],
                                },
                                RelayNode {
                                    node_id: "relay-cn-local-tcp".into(),
                                    transport: "tcp".into(),
                                    address: "127.0.0.1:9001".into(),
                                    priority: 20,
                                    tags: vec![],
                                },
                            ],
                        }],
                    }],
                }],
            },
            derp_map: self.with_derp_map.then(|| DerpMap {
                probe_interval_seconds: 5,
                clusters: vec![DerpCluster {
                    cluster_id: "cn-local-a".into(),
                    cluster_name: Some("CN Local A".into()),
                    region_id: "local".into(),
                    region_name: "Local".into(),
                    country_code: Some("CN".into()),
                    country_name: Some("China".into()),
                    city_code: Some("local".into()),
                    city_name: Some("Local".into()),
                    recommended_fanout: 2,
                    nodes: vec![
                        DerpNodeMeta {
                            cluster_id: "cn-local-a".into(),
                            region_id: "local".into(),
                            country_code: Some("CN".into()),
                            country_name: Some("China".into()),
                            city_code: Some("local".into()),
                            city_name: Some("Local".into()),
                            node_id: "relay-cn-local-udp".into(),
                            host: "127.0.0.1".into(),
                            port: 9000,
                            transport: DerpTransport::Udp,
                            priority: 10,
                            tags: vec![],
                        },
                        DerpNodeMeta {
                            cluster_id: "cn-local-a".into(),
                            region_id: "local".into(),
                            country_code: Some("CN".into()),
                            country_name: Some("China".into()),
                            city_code: Some("local".into()),
                            city_name: Some("Local".into()),
                            node_id: "relay-cn-local-tcp".into(),
                            host: "127.0.0.1".into(),
                            port: 9001,
                            transport: DerpTransport::Udp,
                            priority: 20,
                            tags: vec![],
                        },
                    ],
                }],
            }),
            network_map: Some(NetworkMap {
                self_user_id: "user-1".into(),
                self_device_id: "dev-1".into(),
                self_node_id: node_id.to_string(),
                network_id: network_id.to_string(),
                revision: 1,
                heartbeat_seconds: 15,
                stun_servers: vec![],
                peers: vec![Peer {
                    node_id: "peer-1".into(),
                    device_id: "dev-2".into(),
                    public_key: "peer-pk".into(),
                    status: "online".into(),
                    relay_allowed: true,
                    virtual_ips: vec!["100.64.0.2".into()],
                    endpoints: vec![Endpoint {
                        endpoint_type: "udp".into(),
                        address: "198.51.100.10:41641".into(),
                        updated_at: 1,
                    }],
                    allowed_routes: vec![],
                }, Peer {
                    node_id: "peer-2".into(),
                    device_id: "dev-3".into(),
                    public_key: "peer2-pk".into(),
                    status: "online".into(),
                    relay_allowed: true,
                    virtual_ips: vec!["100.64.0.3".into()],
                    endpoints: vec![Endpoint {
                        endpoint_type: "udp".into(),
                        address: "198.51.100.11:41641".into(),
                        updated_at: 1,
                    }],
                    allowed_routes: vec![],
                }],
                routes: vec![],
                relay_regions: vec![],
                dns: DnsConfig {
                    servers: vec![],
                    search_domains: vec![],
                },
                mtu: Some(1280),
            }),
        })
    }

    fn issue_relay_ticket(
        &self,
        _access_token: &str,
        req: RelayTicketRequest,
    ) -> Result<RelayTicket, String> {
        *self.relay_reason.lock().unwrap() = Some(req.reason);
        Ok(RelayTicket {
            ticket_id: "ticket-1".into(),
            network_id: req.network_id,
            session_id: "session-1".into(),
            src_node_id: req.src_node_id,
            dst_node_id: req.dst_node_id,
            derp_cluster_id: Some("cn-local-a".into()),
            country_code: Some("CN".into()),
            city_code: Some("local".into()),
            allowed_derp_node_ids: vec!["relay-cn-local-udp".into(), "relay-cn-local-tcp".into()],
            relay_url: "udp://127.0.0.1:9000".into(),
            expires_at: "2099-01-01T00:00:00Z".into(),
            session_key: None,
            signature: "sig".into(),
        })
    }
}

struct FakeP2P;

impl P2PConnector for FakeP2P {
    fn connect(&self, _peer: &PeerCandidate) -> Result<ConnectionState, String> {
        Ok(ConnectionState::Failed("timeout".into()))
    }

    fn send_transport_packet(&self, _packet: &[u8]) -> Result<(), String> {
        Ok(())
    }

    fn poll_transport_packet(&self) -> Result<Option<Vec<u8>>, String> {
        Ok(None)
    }
}

struct SuccessfulP2P;

impl P2PConnector for SuccessfulP2P {
    fn connect(&self, _peer: &PeerCandidate) -> Result<ConnectionState, String> {
        Ok(ConnectionState::Connected(ConnectionPath::P2P))
    }

    fn send_transport_packet(&self, _packet: &[u8]) -> Result<(), String> {
        Ok(())
    }

    fn poll_transport_packet(&self) -> Result<Option<Vec<u8>>, String> {
        Ok(None)
    }
}

struct FakeRelay;

impl RelayClient for FakeRelay {
    fn connect(&self, _ticket: &RelayTicket) -> Result<ConnectionState, String> {
        Ok(ConnectionState::Connected(ConnectionPath::Relay))
    }

    fn send_transport_packet(&self, _packet: &[u8]) -> Result<(), String> {
        Ok(())
    }

    fn poll_transport_packet(&self) -> Result<Option<Vec<u8>>, String> {
        Ok(None)
    }
}

#[derive(Default)]
struct FakeDerpPool {
    install_calls: Mutex<Vec<(String, Vec<String>, String)>>,
}

impl DerpPool for FakeDerpPool {
    fn install_cluster(
        &self,
        cluster_id: &str,
        nodes: Vec<DerpNodeMeta>,
        ticket: RelayTicket,
    ) -> Result<(), String> {
        self.install_calls.lock().unwrap().push((
            cluster_id.to_string(),
            nodes.into_iter().map(|node| node.node_id).collect(),
            ticket.session_id,
        ));
        Ok(())
    }

    fn warm_up(&self, _fanout: usize) -> Result<(), String> {
        Ok(())
    }

    fn active_link(&self) -> Option<DerpLinkSnapshot> {
        Some(DerpLinkSnapshot {
            meta: DerpNodeMeta {
                cluster_id: "cn-local-a".into(),
                region_id: "local".into(),
                country_code: Some("CN".into()),
                country_name: Some("China".into()),
                city_code: Some("local".into()),
                city_name: Some("Local".into()),
                node_id: "relay-cn-local-udp".into(),
                host: "127.0.0.1".into(),
                port: 9000,
                transport: DerpTransport::Udp,
                priority: 10,
                tags: vec![],
            },
            state: DerpLinkState::Ready,
            health: DerpHealth {
                rtt_ms_ewma: 10,
                loss_ppm: 0,
                timeout_count: 0,
                consecutive_failures: 0,
                last_probe_at_ms: 1,
                last_recv_at_ms: 1,
                score: 100,
            },
            is_active: true,
        })
    }

    fn send_transport_packet_via_active(&self, _packet: &[u8]) -> Result<(), String> {
        Ok(())
    }

    fn poll_transport_packet_via_active(&self) -> Result<Option<Vec<u8>>, String> {
        Ok(None)
    }

    fn tick_health_check(&self) -> Result<(), String> {
        Ok(())
    }

    fn maybe_switch(&self) -> Result<Option<DerpSwitchEvent>, String> {
        Ok(None)
    }

    fn state(&self) -> DerpPoolState {
        DerpPoolState {
            cluster_id: "cluster-ap-east".into(),
            session_id: "session-1".into(),
            active_node_id: Some("tokyo".into()),
            links: vec![],
            switch_epoch: 1,
        }
    }

    fn force_switch(
        &self,
        target_node_id: &str,
        reason: SwitchReason,
    ) -> Result<DerpSwitchEvent, String> {
        Ok(DerpSwitchEvent {
            cluster_id: "cluster-ap-east".into(),
            from_node_id: Some("tokyo".into()),
            to_node_id: target_node_id.to_string(),
            reason,
            happened_at_ms: 1,
        })
    }

    fn close(&self) -> Result<(), String> {
        Ok(())
    }
}

struct DisabledDerpPool;

impl DerpPool for DisabledDerpPool {
    fn install_cluster(
        &self,
        _cluster_id: &str,
        _nodes: Vec<DerpNodeMeta>,
        _ticket: RelayTicket,
    ) -> Result<(), String> {
        Err("derp disabled".into())
    }

    fn warm_up(&self, _fanout: usize) -> Result<(), String> {
        Err("derp disabled".into())
    }

    fn active_link(&self) -> Option<DerpLinkSnapshot> {
        None
    }

    fn send_transport_packet_via_active(&self, _packet: &[u8]) -> Result<(), String> {
        Err("derp disabled".into())
    }

    fn poll_transport_packet_via_active(&self) -> Result<Option<Vec<u8>>, String> {
        Err("derp disabled".into())
    }

    fn tick_health_check(&self) -> Result<(), String> {
        Err("derp disabled".into())
    }

    fn maybe_switch(&self) -> Result<Option<DerpSwitchEvent>, String> {
        Err("derp disabled".into())
    }

    fn state(&self) -> DerpPoolState {
        DerpPoolState {
            cluster_id: String::new(),
            session_id: String::new(),
            active_node_id: None,
            links: vec![],
            switch_epoch: 0,
        }
    }

    fn force_switch(
        &self,
        _target_node_id: &str,
        _reason: SwitchReason,
    ) -> Result<DerpSwitchEvent, String> {
        Err("derp disabled".into())
    }

    fn close(&self) -> Result<(), String> {
        Ok(())
    }
}

struct RecordingPathManager {
    events: Mutex<Vec<String>>,
    current_path: Mutex<ActivePath>,
    queued_reply: Mutex<Option<Vec<u8>>>,
    pending_empty_polls: Mutex<usize>,
}

impl RecordingPathManager {
    fn with_current_path(path: ActivePath) -> Self {
        Self {
            events: Mutex::new(vec![]),
            current_path: Mutex::new(path),
            queued_reply: Mutex::new(None),
            pending_empty_polls: Mutex::new(0),
        }
    }

    fn with_reply_after_polls(self, polls: usize, reply: Vec<u8>) -> Self {
        *self.pending_empty_polls.lock().unwrap() = polls;
        *self.queued_reply.lock().unwrap() = Some(reply);
        self
    }
}

impl Default for RecordingPathManager {
    fn default() -> Self {
        Self {
            events: Mutex::new(vec![]),
            current_path: Mutex::new(ActivePath::None),
            queued_reply: Mutex::new(None),
            pending_empty_polls: Mutex::new(0),
        }
    }
}

impl PathManager for RecordingPathManager {
    fn on_p2p_failed(&self, peer_node_id: &str, reason: &str) -> Result<(), String> {
        self.events
            .lock()
            .unwrap()
            .push(format!("failed:{peer_node_id}:{reason}"));
        Ok(())
    }

    fn on_p2p_recovered(&self, peer_node_id: &str) -> Result<(), String> {
        self.events
            .lock()
            .unwrap()
            .push(format!("recovered:{peer_node_id}"));
        Ok(())
    }

    fn current_path(&self) -> ActivePath {
        self.current_path.lock().unwrap().clone()
    }

    fn send_transport_packet(&self, _packet: &[u8]) -> Result<(), String> {
        Ok(())
    }

    fn poll_transport_packet(&self) -> Result<Option<Vec<u8>>, String> {
        let mut pending_empty_polls = self.pending_empty_polls.lock().unwrap();
        if *pending_empty_polls > 0 {
            *pending_empty_polls -= 1;
            return Ok(None);
        }
        Ok(self.queued_reply.lock().unwrap().take())
    }
}

#[derive(Default)]
struct RecordingTunnelManager {
    established: Mutex<Vec<String>>,
    closed: Mutex<Vec<String>>,
}

impl TunnelManager for RecordingTunnelManager {
    fn establish(&self, config: &TunnelConfig) -> Result<(), String> {
        self.established
            .lock()
            .unwrap()
            .push(format!(
                "{}->{}:{}",
                config.local_virtual_ip,
                config.peer_virtual_ip,
                config.wireguard_peer.public_key
            ));
        Ok(())
    }

    fn close(&self, peer_virtual_ip: &str) -> Result<(), String> {
        self.closed.lock().unwrap().push(peer_virtual_ip.to_string());
        Ok(())
    }
}

#[test]
fn connect_prefers_derp_pool_after_p2p_failure() {
    let path_manager = Arc::new(RecordingPathManager::with_current_path(ActivePath::Derp {
        cluster_id: "cn-local-a".into(),
        node_id: "relay-cn-local-udp".into(),
    }));
    let tunnel_manager = Arc::new(RecordingTunnelManager::default());
    let facade = DefaultAppCoreFacade::new(
        FakeController::new(true),
        FakeP2P,
        FakeRelay,
        FakeDerpPool::default(),
        path_manager.clone(),
        tunnel_manager,
    );

    facade
        .register("a@example.com".into(), "secret".into())
        .unwrap();
    facade
        .register_node(
            "dev-1".into(),
            "node-1".into(),
            "node-pk".into(),
            vec!["relay".into()],
        )
        .unwrap();
    facade.bootstrap("node-1".into(), "net-1".into()).unwrap();

    let state = facade.connect("net-1".into(), "peer-1".into()).unwrap();
    assert!(matches!(
        state,
        ConnectionState::Connected(ConnectionPath::Derp)
    ));
}

#[test]
fn connect_falls_back_to_relay_when_derp_is_unavailable() {
    let path_manager = Arc::new(RecordingPathManager::with_current_path(ActivePath::Relay {
        peer_node_id: "peer-1".into(),
    }));
    let tunnel_manager = Arc::new(RecordingTunnelManager::default());
    let facade = DefaultAppCoreFacade::new(
        FakeController::new(false),
        FakeP2P,
        FakeRelay,
        DisabledDerpPool,
        path_manager.clone(),
        tunnel_manager,
    );

    facade
        .register("a@example.com".into(), "secret".into())
        .unwrap();
    facade
        .register_node(
            "dev-1".into(),
            "node-1".into(),
            "node-pk".into(),
            vec!["relay".into()],
        )
        .unwrap();
    facade.bootstrap("node-1".into(), "net-1".into()).unwrap();

    let state = facade.connect("net-1".into(), "peer-1".into()).unwrap();
    assert!(matches!(
        state,
        ConnectionState::Connected(ConnectionPath::Relay)
    ));
}

#[test]
fn connect_notifies_path_manager_on_derp_fallback() {
    let path_manager = Arc::new(RecordingPathManager::with_current_path(ActivePath::Derp {
        cluster_id: "cn-local-a".into(),
        node_id: "relay-cn-local-udp".into(),
    }));
    let tunnel_manager = Arc::new(RecordingTunnelManager::default());
    let facade = DefaultAppCoreFacade::new(
        FakeController::new(true),
        FakeP2P,
        FakeRelay,
        FakeDerpPool::default(),
        path_manager.clone(),
        tunnel_manager,
    );

    facade
        .register("a@example.com".into(), "secret".into())
        .unwrap();
    facade
        .register_node(
            "dev-1".into(),
            "node-1".into(),
            "node-pk".into(),
            vec!["relay".into()],
        )
        .unwrap();
    facade.bootstrap("node-1".into(), "net-1".into()).unwrap();

    let state = facade.connect("net-1".into(), "peer-1".into()).unwrap();
    assert!(matches!(
        state,
        ConnectionState::Connected(ConnectionPath::Derp)
    ));

    let events = path_manager.events.lock().unwrap().clone();
    assert_eq!(events, vec!["failed:peer-1:timeout", "failed:peer-1:derp_fallback"]);
}

#[test]
fn connect_notifies_path_manager_on_relay_fallback() {
    let path_manager = Arc::new(RecordingPathManager::with_current_path(ActivePath::Relay {
        peer_node_id: "peer-1".into(),
    }));
    let tunnel_manager = Arc::new(RecordingTunnelManager::default());
    let facade = DefaultAppCoreFacade::new(
        FakeController::new(false),
        FakeP2P,
        FakeRelay,
        DisabledDerpPool,
        path_manager.clone(),
        tunnel_manager,
    );

    facade
        .register("a@example.com".into(), "secret".into())
        .unwrap();
    facade
        .register_node(
            "dev-1".into(),
            "node-1".into(),
            "node-pk".into(),
            vec!["relay".into()],
        )
        .unwrap();
    facade.bootstrap("node-1".into(), "net-1".into()).unwrap();

    let state = facade.connect("net-1".into(), "peer-1".into()).unwrap();
    assert!(matches!(
        state,
        ConnectionState::Connected(ConnectionPath::Relay)
    ));

    let events = path_manager.events.lock().unwrap().clone();
    assert_eq!(
        events,
        vec!["failed:peer-1:timeout", "failed:peer-1:relay_fallback"]
    );
}

#[test]
fn connect_notifies_path_manager_on_p2p_recovery() {
    let path_manager = Arc::new(RecordingPathManager::with_current_path(ActivePath::P2P {
        peer_node_id: "peer-1".into(),
    }));
    let tunnel_manager = Arc::new(RecordingTunnelManager::default());
    let facade = DefaultAppCoreFacade::new(
        FakeController::new(true),
        SuccessfulP2P,
        FakeRelay,
        FakeDerpPool::default(),
        path_manager.clone(),
        tunnel_manager,
    );

    facade
        .register("a@example.com".into(), "secret".into())
        .unwrap();
    facade
        .register_node(
            "dev-1".into(),
            "node-1".into(),
            "node-pk".into(),
            vec!["relay".into()],
        )
        .unwrap();
    facade.bootstrap("node-1".into(), "net-1".into()).unwrap();

    let state = facade.connect("net-1".into(), "peer-1".into()).unwrap();
    assert!(matches!(
        state,
        ConnectionState::Connected(ConnectionPath::P2P)
    ));

    let events = path_manager.events.lock().unwrap().clone();
    assert_eq!(events, vec!["recovered:peer-1"]);
}

#[test]
fn connect_establishes_and_disconnect_closes_tunnel_when_ips_are_available() {
    let path_manager = Arc::new(RecordingPathManager::with_current_path(ActivePath::Relay {
        peer_node_id: "peer-1".into(),
    }));
    let tunnel_manager = Arc::new(RecordingTunnelManager::default());
    let facade = DefaultAppCoreFacade::new(
        FakeController::new(false),
        FakeP2P,
        FakeRelay,
        DisabledDerpPool,
        path_manager,
        tunnel_manager.clone(),
    );

    facade
        .register("a@example.com".into(), "secret".into())
        .unwrap();
    facade
        .register_node(
            "dev-1".into(),
            "node-1".into(),
            "node-pk".into(),
            vec!["relay".into()],
        )
        .unwrap();
    facade.bootstrap("node-1".into(), "net-1".into()).unwrap();
    facade.connect("net-1".into(), "peer-1".into()).unwrap();
    facade.disconnect().unwrap();

    assert_eq!(
        tunnel_manager.established.lock().unwrap().clone(),
        vec!["100.64.0.10->100.64.0.2:peer-pk"]
    );
    assert_eq!(
        tunnel_manager.closed.lock().unwrap().clone(),
        vec!["100.64.0.2"]
    );
}

#[test]
fn reconnect_replaces_existing_tunnel_for_new_peer() {
    let path_manager = Arc::new(RecordingPathManager::with_current_path(ActivePath::Relay {
        peer_node_id: "peer-1".into(),
    }));
    let tunnel_manager = Arc::new(RecordingTunnelManager::default());
    let facade = DefaultAppCoreFacade::new(
        FakeController::new(false),
        FakeP2P,
        FakeRelay,
        DisabledDerpPool,
        path_manager.clone(),
        tunnel_manager.clone(),
    );

    facade
        .register("a@example.com".into(), "secret".into())
        .unwrap();
    facade
        .register_node(
            "dev-1".into(),
            "node-1".into(),
            "node-pk".into(),
            vec!["relay".into()],
        )
        .unwrap();
    facade.bootstrap("node-1".into(), "net-1".into()).unwrap();
    facade.connect("net-1".into(), "peer-1".into()).unwrap();
    *path_manager.current_path.lock().unwrap() = ActivePath::Relay {
        peer_node_id: "peer-2".into(),
    };
    facade.connect("net-1".into(), "peer-2".into()).unwrap();

    assert_eq!(
        tunnel_manager.established.lock().unwrap().clone(),
        vec![
            "100.64.0.10->100.64.0.2:peer-pk",
            "100.64.0.10->100.64.0.3:peer2-pk",
        ]
    );
    assert_eq!(
        tunnel_manager.closed.lock().unwrap().clone(),
        vec!["100.64.0.2"]
    );
}

#[test]
fn connect_skips_tunnel_when_virtual_ips_are_missing() {
    let path_manager = Arc::new(RecordingPathManager::with_current_path(ActivePath::Relay {
        peer_node_id: "peer-1".into(),
    }));
    let tunnel_manager = Arc::new(RecordingTunnelManager::default());
    let facade = DefaultAppCoreFacade::new(
        FakeController::without_device_virtual_ip(false),
        FakeP2P,
        FakeRelay,
        DisabledDerpPool,
        path_manager,
        tunnel_manager.clone(),
    );

    facade
        .register("a@example.com".into(), "secret".into())
        .unwrap();
    facade
        .register_node(
            "dev-1".into(),
            "node-1".into(),
            "node-pk".into(),
            vec!["relay".into()],
        )
        .unwrap();
    facade.bootstrap("node-1".into(), "net-1".into()).unwrap();
    facade.connect("net-1".into(), "peer-1".into()).unwrap();
    facade.disconnect().unwrap();

    assert!(tunnel_manager.established.lock().unwrap().is_empty());
    assert!(tunnel_manager.closed.lock().unwrap().is_empty());
}

#[test]
fn probe_updates_snapshot_and_disconnect_clears_last_probe() {
    let path_manager = Arc::new(RecordingPathManager::with_current_path(ActivePath::Relay {
        peer_node_id: "peer-1".into(),
    }));
    let tunnel_manager = Arc::new(RecordingTunnelManager::default());
    let facade = DefaultAppCoreFacade::new(
        FakeController::new(false),
        FakeP2P,
        FakeRelay,
        DisabledDerpPool,
        path_manager,
        tunnel_manager,
    );

    facade
        .register("a@example.com".into(), "secret".into())
        .unwrap();
    facade
        .register_node(
            "dev-1".into(),
            "node-1".into(),
            "node-pk".into(),
            vec!["relay".into()],
        )
        .unwrap();
    facade.bootstrap("node-1".into(), "net-1".into()).unwrap();
    facade.connect("net-1".into(), "peer-1".into()).unwrap();

    let probe = facade.probe(b"hello".to_vec()).unwrap();
    let snapshot = facade.snapshot().unwrap();
    let last_probe = snapshot.last_probe.expect("last probe");
    assert_eq!(last_probe.probe_id, probe.probe_id);
    assert_eq!(last_probe.sampled_at_ms, probe.sampled_at_ms);
    assert_eq!(last_probe.bytes_sent, 5);
    assert!(!last_probe.reply_observed);
    assert!(last_probe.reply_bytes_received.is_none());
    assert!(last_probe.reply_sampled_at_ms.is_none());
    assert!(last_probe.reply_rtt_ms.is_none());
    assert!(last_probe.observed_rtt_ms.is_none());
    assert!(last_probe.packet_loss_ppm.is_none());
    assert!(last_probe.path_score.is_none());
    assert!(last_probe.derp_cluster_id.is_none());
    assert!(last_probe.derp_node_id.is_none());
    match last_probe.active_path {
        ActivePath::Relay { peer_node_id } => assert_eq!(peer_node_id, "peer-1"),
        _ => panic!("expected relay active path"),
    }
    assert_eq!(last_probe.tunnel_peer_virtual_ip.as_deref(), Some("100.64.0.2"));

    facade.disconnect().unwrap();
    let disconnected = facade.snapshot().unwrap();
    assert!(disconnected.last_probe.is_none());
}

#[test]
fn probe_reports_derp_health_when_active_path_is_derp() {
    let path_manager = Arc::new(RecordingPathManager::with_current_path(ActivePath::Derp {
        cluster_id: "cn-local-a".into(),
        node_id: "relay-cn-local-udp".into(),
    }));
    let tunnel_manager = Arc::new(RecordingTunnelManager::default());
    let facade = DefaultAppCoreFacade::new(
        FakeController::new(true),
        FakeP2P,
        FakeRelay,
        FakeDerpPool::default(),
        path_manager,
        tunnel_manager,
    );

    facade
        .register("a@example.com".into(), "secret".into())
        .unwrap();
    facade
        .register_node(
            "dev-1".into(),
            "node-1".into(),
            "node-pk".into(),
            vec!["relay".into()],
        )
        .unwrap();
    facade.bootstrap("node-1".into(), "net-1".into()).unwrap();
    facade.connect("net-1".into(), "peer-1".into()).unwrap();

    let probe = facade.probe(b"hello".to_vec()).unwrap();
    assert_eq!(probe.observed_rtt_ms, Some(10));
    assert_eq!(probe.packet_loss_ppm, Some(0));
    assert_eq!(probe.path_score, Some(100));
    assert_eq!(probe.derp_cluster_id.as_deref(), Some("cn-local-a"));
    assert_eq!(probe.derp_node_id.as_deref(), Some("relay-cn-local-udp"));
    assert!(!probe.reply_observed);
    assert!(probe.reply_bytes_received.is_none());
    assert!(probe.reply_sampled_at_ms.is_none());
    assert!(probe.reply_rtt_ms.is_none());
}

#[test]
fn probe_reports_reply_observation_when_packet_is_received() {
    let path_manager = Arc::new(
        RecordingPathManager::with_current_path(ActivePath::Relay {
            peer_node_id: "peer-1".into(),
        })
        .with_reply_after_polls(2, b"world".to_vec()),
    );
    let tunnel_manager = Arc::new(RecordingTunnelManager::default());
    let facade = DefaultAppCoreFacade::new(
        FakeController::new(false),
        FakeP2P,
        FakeRelay,
        DisabledDerpPool,
        path_manager,
        tunnel_manager,
    );

    facade
        .register("a@example.com".into(), "secret".into())
        .unwrap();
    facade
        .register_node(
            "dev-1".into(),
            "node-1".into(),
            "node-pk".into(),
            vec!["relay".into()],
        )
        .unwrap();
    facade.bootstrap("node-1".into(), "net-1".into()).unwrap();
    facade.connect("net-1".into(), "peer-1".into()).unwrap();

    let probe = facade.probe(b"hello".to_vec()).unwrap();
    assert!(probe.reply_observed);
    assert_eq!(probe.reply_bytes_received, Some(5));
    assert!(probe.reply_sampled_at_ms.is_some());
    assert!(probe.reply_rtt_ms.is_some());
}

#[test]
fn probe_respects_custom_reply_timeout_override() {
    let path_manager = Arc::new(
        RecordingPathManager::with_current_path(ActivePath::Relay {
            peer_node_id: "peer-1".into(),
        })
        .with_reply_after_polls(2, b"world".to_vec()),
    );
    let tunnel_manager = Arc::new(RecordingTunnelManager::default());
    let facade = DefaultAppCoreFacade::new(
        FakeController::new(false),
        FakeP2P,
        FakeRelay,
        DisabledDerpPool,
        path_manager,
        tunnel_manager,
    );

    facade
        .register("a@example.com".into(), "secret".into())
        .unwrap();
    facade
        .register_node(
            "dev-1".into(),
            "node-1".into(),
            "node-pk".into(),
            vec!["relay".into()],
        )
        .unwrap();
    facade.bootstrap("node-1".into(), "net-1".into()).unwrap();
    facade.connect("net-1".into(), "peer-1".into()).unwrap();

    let probe = facade.probe_with_timeout(b"hello".to_vec(), Some(0)).unwrap();
    assert!(!probe.reply_observed);
    assert!(probe.reply_bytes_received.is_none());
    assert!(probe.reply_sampled_at_ms.is_none());
    assert!(probe.reply_rtt_ms.is_none());
}
