use std::sync::Mutex;

use controller_client::{
    ControllerClient, CreateNetworkRequest, JoinNetworkRequest, LoginRequest,
    RegisterDeviceRequest, RegisterNodeRequest, RegisterRequest, RelayTicketRequest,
};
use ffi_bridge::{AppCoreFacade, DefaultAppCoreFacade};
use p2p::{P2PConnector, PeerCandidate};
use relay_client::{DerpPool, RelayClient};
use slan_app_core::{
    BootstrapConfig, ConnectionPath, ConnectionState, ControlPlaneConfig, DerpCluster, DerpHealth,
    DerpLinkSnapshot, DerpLinkState, DerpMap, DerpNodeMeta, DerpPoolState, DerpSwitchEvent,
    DerpTransport, Device, DnsConfig, Endpoint, Network, NetworkMap, Node, Peer, RelayConfig,
    RelayTicket, Session, SwitchReason,
};

struct FakeController {
    relay_reason: Mutex<Option<String>>,
    with_derp_map: bool,
}

impl FakeController {
    fn new(with_derp_map: bool) -> Self {
        Self {
            relay_reason: Mutex::new(None),
            with_derp_map,
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
                virtual_ip: None,
                public_key: Some("device-pk".into()),
            },
            networks: vec![],
            control_plane: ControlPlaneConfig {
                ws_url: "ws://127.0.0.1:8080/control/ws".into(),
                heartbeat_seconds: 15,
            },
            stun_servers: vec!["stun:127.0.0.1:3478".into()],
            relay: RelayConfig {
                region: "local".into(),
                udp_endpoint: "127.0.0.1:9000".into(),
                tcp_endpoint: None,
            },
            derp_map: self.with_derp_map.then(|| DerpMap {
                probe_interval_seconds: 5,
                clusters: vec![DerpCluster {
                    cluster_id: "cluster-ap-east".into(),
                    region_id: "ap-east".into(),
                    region_name: "Asia Pacific East".into(),
                    recommended_fanout: 2,
                    nodes: vec![
                        DerpNodeMeta {
                            cluster_id: "cluster-ap-east".into(),
                            region_id: "ap-east".into(),
                            node_id: "tokyo".into(),
                            host: "tokyo.derp.local".into(),
                            port: 9000,
                            transport: DerpTransport::Udp,
                            priority: 10,
                            tags: vec![],
                        },
                        DerpNodeMeta {
                            cluster_id: "cluster-ap-east".into(),
                            region_id: "ap-east".into(),
                            node_id: "singapore".into(),
                            host: "singapore.derp.local".into(),
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
            derp_cluster_id: None,
            allowed_derp_node_ids: vec![],
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
}

struct FakeRelay;

impl RelayClient for FakeRelay {
    fn connect(&self, _ticket: &RelayTicket) -> Result<ConnectionState, String> {
        Ok(ConnectionState::Connected(ConnectionPath::Relay))
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
                cluster_id: "cluster-ap-east".into(),
                region_id: "ap-east".into(),
                node_id: "tokyo".into(),
                host: "tokyo.derp.local".into(),
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

    fn send_via_active(&self, _packet: &[u8]) -> Result<(), String> {
        Ok(())
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

    fn send_via_active(&self, _packet: &[u8]) -> Result<(), String> {
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

#[test]
fn connect_prefers_derp_pool_after_p2p_failure() {
    let facade = DefaultAppCoreFacade::new(
        FakeController::new(true),
        FakeP2P,
        FakeRelay,
        FakeDerpPool::default(),
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
    let facade = DefaultAppCoreFacade::new(
        FakeController::new(false),
        FakeP2P,
        FakeRelay,
        DisabledDerpPool,
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
