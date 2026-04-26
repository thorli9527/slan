use controller_client::{
    ControllerClient, CreateNetworkRequest, DeactivateNetworkRequest, JoinNetworkByKeyRequest,
    JoinNetworkByOwnerEmailRequest, JoinNetworkRequest, LoginRequest, RefreshTokenRequest,
    RegisterDeviceRequest, RegisterNodeRequest, RegisterRequest, RelayTicketRequest,
    UpdateAttachmentRemarkRequest, UpdateNetworkDNSRequest,
};
use ffi_bridge::{DefaultAppCoreFacade, JsonAppCoreFacade};
use p2p::{P2PConnector, PeerCandidate};
use relay_client::{DerpPool, PathManager, PathManagerError, RelayClient, RelayClientError};
use serde_json::json;
use slan_app_core::{
    ActivePath, BootstrapConfig, ConnectionPath, ConnectionState, ControlPlaneConfig,
    DerpLinkSnapshot, DerpNodeMeta, DerpPoolState, DerpSwitchEvent, Device, DnsConfig, Endpoint,
    Network, NetworkAssignment, NetworkJoinResult, NetworkMap, Node, Peer, RelayCity, RelayCluster,
    RelayConfig, RelayCountry, RelayNode, RelayTicket, Session, SwitchReason,
};
use tunnel::{TunnelConfig, TunnelManager};

mod support;

struct FakeController;

impl ControllerClient for FakeController {
    fn register(&self, _req: RegisterRequest) -> Result<Session, String> {
        Ok(Session {
            user_id: "user-1".into(),
            access_token: "token-1".into(),
            refresh_token: Some("refresh-1".into()),
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

    fn refresh(&self, _req: RefreshTokenRequest) -> Result<Session, String> {
        self.register(RegisterRequest {
            email: String::new(),
            password: String::new(),
        })
    }

    fn register_device(
        &self,
        _access_token: &str,
        req: RegisterDeviceRequest,
    ) -> Result<Device, String> {
        Ok(Device {
            device_id: req.machine_id,
            name: req.name,
            platform: req.platform,
            status: "online".into(),
            virtual_ip: Some("100.64.0.10".into()),
            public_key: Some(req.public_key),
            mqtt: None,
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
        Ok(vec![Network {
            network_id: "net-1".into(),
            name: "home".into(),
            cidr: "100.64.0.0/24".into(),
            members: vec![],
        }])
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

    fn update_network_dns(
        &self,
        _access_token: &str,
        req: UpdateNetworkDNSRequest,
    ) -> Result<Network, String> {
        Ok(Network {
            network_id: req.network_id,
            name: "home".into(),
            cidr: "100.64.0.0/24".into(),
            members: vec![],
        })
    }

    fn join_network(
        &self,
        _access_token: &str,
        req: JoinNetworkRequest,
    ) -> Result<NetworkJoinResult, String> {
        Ok(join_result(req.network_id, req.device_id))
    }

    fn join_network_by_owner_email(
        &self,
        _access_token: &str,
        _req: JoinNetworkByOwnerEmailRequest,
    ) -> Result<NetworkJoinResult, String> {
        Ok(join_result("net-1".into(), _req.device_id))
    }

    fn join_network_by_key(
        &self,
        _access_token: &str,
        _req: JoinNetworkByKeyRequest,
    ) -> Result<NetworkJoinResult, String> {
        Ok(join_result("net-1".into(), _req.device_id))
    }

    fn update_attachment_remark(
        &self,
        _access_token: &str,
        req: UpdateAttachmentRemarkRequest,
    ) -> Result<NetworkAssignment, String> {
        Ok(network_assignment(
            req.network_id,
            req.attachment_id,
            req.remark,
        ))
    }

    fn activate_network(
        &self,
        _access_token: &str,
        req: JoinNetworkRequest,
    ) -> Result<NetworkJoinResult, String> {
        Ok(join_result(req.network_id, req.device_id))
    }

    fn deactivate_network(
        &self,
        _access_token: &str,
        _req: DeactivateNetworkRequest,
    ) -> Result<(), String> {
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
                virtual_ip: Some("100.64.0.10".into()),
                public_key: Some("device-pk".into()),
                mqtt: None,
            },
            networks: vec![Network {
                network_id: network_id.into(),
                name: "home".into(),
                cidr: "100.64.0.0/24".into(),
                members: vec![],
            }],
            control_plane: ControlPlaneConfig {
                ws_url: support::DEV_CONTROL_WS_URL.into(),
                session_token: Some("control-session-token".into()),
                heartbeat_seconds: 15,
            },
            stun_servers: vec![support::DEV_STUN_SERVER.into()],
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
                            nodes: vec![RelayNode {
                                node_id: "relay-cn-local-udp".into(),
                                transport: "udp".into(),
                                address: support::DEV_RELAY_UDP_ADDRESS.into(),
                                priority: 10,
                                tags: vec![],
                            }],
                        }],
                    }],
                }],
            },
            derp_map: None,
            network_map: Some(NetworkMap {
                self_user_id: "user-1".into(),
                self_device_id: "dev-1".into(),
                self_node_id: node_id.into(),
                network_id: network_id.into(),
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
        Ok(RelayTicket {
            ticket_id: "ticket-1".into(),
            network_id: req.network_id,
            session_id: "session-1".into(),
            src_node_id: req.src_node_id,
            dst_node_id: req.dst_node_id,
            derp_cluster_id: Some("cn-local-a".into()),
            country_code: Some("CN".into()),
            city_code: Some("local".into()),
            allowed_derp_node_ids: vec!["relay-cn-local-udp".into()],
            relay_url: support::DEV_RELAY_UDP_URL.into(),
            expires_at: "2099-01-01T00:00:00Z".into(),
            session_key: None,
            signature: "sig".into(),
        })
    }
}

fn join_result(network_id: String, device_id: String) -> NetworkJoinResult {
    NetworkJoinResult {
        network_id,
        device_id,
        member_id: Some("member-1".into()),
        attachment_id: Some("attach-1".into()),
        virtual_ip: Some("100.64.0.10".into()),
    }
}

fn network_assignment(
    network_id: String,
    attachment_id: String,
    remark: Option<String>,
) -> NetworkAssignment {
    NetworkAssignment {
        attachment_id,
        network_id,
        subnet_id: "subnet-1".into(),
        device_id: "dev-1".into(),
        device_name: "device-1".into(),
        user_id: "user-1".into(),
        user_email: "user@example.com".into(),
        role: "member".into(),
        remark,
        virtual_ip: Some("100.64.0.10".into()),
        status: Some("active".into()),
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

struct FakeRelay;

impl RelayClient for FakeRelay {
    fn connect(&self, _ticket: &RelayTicket) -> Result<ConnectionState, RelayClientError> {
        Ok(ConnectionState::Connected(ConnectionPath::Relay))
    }

    fn send_transport_packet(&self, _packet: &[u8]) -> Result<(), RelayClientError> {
        Ok(())
    }

    fn poll_transport_packet(&self) -> Result<Option<Vec<u8>>, RelayClientError> {
        Ok(None)
    }
}

struct FakeDerpPool;

impl DerpPool for FakeDerpPool {
    fn install_cluster(
        &self,
        _cluster_id: &str,
        _nodes: Vec<DerpNodeMeta>,
        _ticket: RelayTicket,
    ) -> Result<(), String> {
        Ok(())
    }

    fn warm_up(&self, _fanout: usize) -> Result<(), String> {
        Ok(())
    }

    fn active_link(&self) -> Option<DerpLinkSnapshot> {
        None
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
            cluster_id: String::new(),
            session_id: String::new(),
            active_node_id: None,
            links: vec![],
            switch_epoch: 0,
        }
    }

    fn force_switch(
        &self,
        target_node_id: &str,
        reason: SwitchReason,
    ) -> Result<DerpSwitchEvent, String> {
        Ok(DerpSwitchEvent {
            cluster_id: String::new(),
            from_node_id: None,
            to_node_id: target_node_id.to_string(),
            reason,
            happened_at_ms: 0,
        })
    }

    fn close(&self) -> Result<(), String> {
        Ok(())
    }
}

struct NoopPathManager {
    current_path: Mutex<ActivePath>,
}

impl Default for NoopPathManager {
    fn default() -> Self {
        Self {
            current_path: Mutex::new(ActivePath::None),
        }
    }
}

impl PathManager for NoopPathManager {
    fn on_p2p_failed(&self, peer_node_id: &str, _reason: &str) -> Result<(), PathManagerError> {
        *self.current_path.lock().unwrap() = ActivePath::Relay {
            peer_node_id: peer_node_id.to_string(),
        };
        Ok(())
    }

    fn on_p2p_recovered(&self, peer_node_id: &str) -> Result<(), PathManagerError> {
        *self.current_path.lock().unwrap() = ActivePath::P2P {
            peer_node_id: peer_node_id.to_string(),
        };
        Ok(())
    }

    fn current_path(&self) -> ActivePath {
        self.current_path.lock().unwrap().clone()
    }

    fn send_transport_packet(&self, _packet: &[u8]) -> Result<(), PathManagerError> {
        Ok(())
    }

    fn poll_transport_packet(&self) -> Result<Option<Vec<u8>>, PathManagerError> {
        Ok(None)
    }
}

struct FailingPathManager;

impl PathManager for FailingPathManager {
    fn on_p2p_failed(&self, _peer_node_id: &str, _reason: &str) -> Result<(), PathManagerError> {
        Ok(())
    }

    fn on_p2p_recovered(&self, _peer_node_id: &str) -> Result<(), PathManagerError> {
        Ok(())
    }

    fn current_path(&self) -> ActivePath {
        ActivePath::Relay {
            peer_node_id: "peer-1".into(),
        }
    }

    fn send_transport_packet(&self, _packet: &[u8]) -> Result<(), PathManagerError> {
        Err(PathManagerError::message(
            "timed out waiting for probe reply",
        ))
    }

    fn poll_transport_packet(&self) -> Result<Option<Vec<u8>>, PathManagerError> {
        Ok(None)
    }
}

struct UnsupportedPathManager;

impl PathManager for UnsupportedPathManager {
    fn on_p2p_failed(&self, _peer_node_id: &str, _reason: &str) -> Result<(), PathManagerError> {
        Ok(())
    }

    fn on_p2p_recovered(&self, _peer_node_id: &str) -> Result<(), PathManagerError> {
        Ok(())
    }

    fn current_path(&self) -> ActivePath {
        ActivePath::None
    }

    fn send_transport_packet(&self, _packet: &[u8]) -> Result<(), PathManagerError> {
        Err(PathManagerError::message(
            "unsupported active path selected for send",
        ))
    }

    fn poll_transport_packet(&self) -> Result<Option<Vec<u8>>, PathManagerError> {
        Ok(None)
    }
}

struct NoopTunnelManager;

impl TunnelManager for NoopTunnelManager {
    fn establish(&self, _config: &TunnelConfig) -> Result<(), String> {
        Ok(())
    }

    fn close(&self, _peer_virtual_ip: &str) -> Result<(), String> {
        Ok(())
    }
}

#[test]
fn json_facade_serializes_bootstrap_and_relay_ticket() {
    let facade = JsonAppCoreFacade::new(DefaultAppCoreFacade::new(
        FakeController,
        FakeP2P,
        FakeRelay,
        FakeDerpPool,
        NoopPathManager::default(),
        NoopTunnelManager,
    ));

    facade
        .invoke(
            "register",
            json!({
                "email": "user@example.com",
                "password": "secret"
            }),
        )
        .unwrap();

    let bootstrap = facade
        .invoke(
            "bootstrap",
            json!({
                "nodeId": "node-1",
                "networkId": "net-1"
            }),
        )
        .unwrap();
    let ticket = facade
        .invoke(
            "issueRelayTicket",
            json!({
                "networkId": "net-1",
                "srcNodeId": "node-1",
                "dstNodeId": "peer-1",
                "reason": "p2p_failed"
            }),
        )
        .unwrap();

    assert_eq!(bootstrap["relay"]["defaultClusterId"], "cn-local-a");
    assert_eq!(
        bootstrap["relay"]["countries"][0]["cities"][0]["clusters"][0]["nodes"][0]["nodeId"],
        "relay-cn-local-udp"
    );
    assert_eq!(ticket["derpClusterId"], "cn-local-a");
    assert_eq!(ticket["allowedDerpNodeIds"][0], "relay-cn-local-udp");
}

#[test]
fn json_facade_serializes_switch_network() {
    let facade = JsonAppCoreFacade::new(DefaultAppCoreFacade::new(
        FakeController,
        FakeP2P,
        FakeRelay,
        FakeDerpPool,
        NoopPathManager::default(),
        NoopTunnelManager,
    ));

    facade
        .invoke(
            "register",
            json!({
                "email": "user@example.com",
                "password": "secret"
            }),
        )
        .unwrap();

    let switched = facade
        .invoke(
            "switchNetwork",
            json!({
                "networkId": "net-1",
                "deviceId": "dev-1"
            }),
        )
        .unwrap();

    assert_eq!(switched["networkId"], "net-1");
    assert_eq!(switched["deviceId"], "dev-1");
    assert_eq!(switched["attachmentId"], "attach-1");
}

#[test]
fn json_facade_serializes_connection_state_for_bridge() {
    let facade = JsonAppCoreFacade::new(DefaultAppCoreFacade::new(
        FakeController,
        FakeP2P,
        FakeRelay,
        FakeDerpPool,
        NoopPathManager::default(),
        NoopTunnelManager,
    ));

    facade
        .invoke(
            "register",
            json!({
                "email": "user@example.com",
                "password": "secret"
            }),
        )
        .unwrap();
    facade
        .invoke(
            "registerNode",
            json!({
                "deviceId": "dev-1",
                "nodeId": "node-1",
                "nodePublicKey": "node-pk",
                "capabilities": ["relay"]
            }),
        )
        .unwrap();
    facade
        .invoke(
            "bootstrap",
            json!({
                "nodeId": "node-1",
                "networkId": "net-1"
            }),
        )
        .unwrap();

    let state = facade
        .invoke(
            "connect",
            json!({
                "networkId": "net-1",
                "peerNodeId": "peer-1"
            }),
        )
        .unwrap();

    assert_eq!(state["status"], "connected");
    assert_eq!(state["path"], "relay");
}

#[test]
fn json_facade_serializes_send_result_for_bridge() {
    let facade = JsonAppCoreFacade::new(DefaultAppCoreFacade::new(
        FakeController,
        FakeP2P,
        FakeRelay,
        FakeDerpPool,
        NoopPathManager::default(),
        NoopTunnelManager,
    ));

    facade
        .invoke(
            "register",
            json!({
                "email": "user@example.com",
                "password": "secret"
            }),
        )
        .unwrap();
    facade
        .invoke(
            "registerNode",
            json!({
                "deviceId": "dev-1",
                "nodeId": "node-1",
                "nodePublicKey": "node-pk",
                "capabilities": ["relay"]
            }),
        )
        .unwrap();
    facade
        .invoke(
            "bootstrap",
            json!({
                "nodeId": "node-1",
                "networkId": "net-1"
            }),
        )
        .unwrap();
    facade
        .invoke(
            "connect",
            json!({
                "networkId": "net-1",
                "peerNodeId": "peer-1"
            }),
        )
        .unwrap();

    let send = facade
        .invoke(
            "send",
            json!({
                "payload": "hello"
            }),
        )
        .unwrap();

    assert_eq!(send["bytesSent"], 5);
}

#[test]
fn json_facade_surfaces_typed_send_errors() {
    let facade = JsonAppCoreFacade::new(DefaultAppCoreFacade::new(
        FakeController,
        FakeP2P,
        FakeRelay,
        FakeDerpPool,
        FailingPathManager,
        NoopTunnelManager,
    ));

    let error = facade
        .invoke(
            "send",
            json!({
                "payload": "hello"
            }),
        )
        .unwrap_err();

    assert!(error.starts_with("send_timeout:"));
}

#[test]
fn json_facade_surfaces_send_unsupported_errors() {
    let facade = JsonAppCoreFacade::new(DefaultAppCoreFacade::new(
        FakeController,
        FakeP2P,
        FakeRelay,
        FakeDerpPool,
        UnsupportedPathManager,
        NoopTunnelManager,
    ));

    let error = facade
        .invoke(
            "send",
            json!({
                "payload": "hello"
            }),
        )
        .unwrap_err();

    assert!(error.starts_with("send_unsupported_path:"));
}

#[test]
fn json_facade_surfaces_probe_unsupported_errors() {
    let facade = JsonAppCoreFacade::new(DefaultAppCoreFacade::new(
        FakeController,
        FakeP2P,
        FakeRelay,
        FakeDerpPool,
        UnsupportedPathManager,
        NoopTunnelManager,
    ));

    let error = facade
        .invoke(
            "probe",
            json!({
                "payload": "hello"
            }),
        )
        .unwrap_err();

    assert!(error.starts_with("probe_unsupported_path:"));
}

#[test]
fn json_facade_serializes_probe_result_for_bridge() {
    let facade = JsonAppCoreFacade::new(DefaultAppCoreFacade::new(
        FakeController,
        FakeP2P,
        FakeRelay,
        FakeDerpPool,
        NoopPathManager::default(),
        NoopTunnelManager,
    ));

    facade
        .invoke(
            "register",
            json!({
                "email": "user@example.com",
                "password": "secret"
            }),
        )
        .unwrap();
    facade
        .invoke(
            "registerNode",
            json!({
                "deviceId": "dev-1",
                "nodeId": "node-1",
                "nodePublicKey": "node-pk",
                "capabilities": ["relay"]
            }),
        )
        .unwrap();
    facade
        .invoke(
            "bootstrap",
            json!({
                "nodeId": "node-1",
                "networkId": "net-1"
            }),
        )
        .unwrap();
    facade
        .invoke(
            "connect",
            json!({
                "networkId": "net-1",
                "peerNodeId": "peer-1"
            }),
        )
        .unwrap();

    let probe = facade
        .invoke(
            "probe",
            json!({
                "payload": "hello"
            }),
        )
        .unwrap();

    assert!(probe["probeId"]
        .as_str()
        .expect("probeId string")
        .starts_with("probe-"));
    assert!(probe["sampledAtMs"].as_u64().expect("sampledAtMs u64") > 0);
    assert_eq!(probe["bytesSent"], 5);
    assert_eq!(probe["replyObserved"], false);
    assert_eq!(probe["replyBytesReceived"], serde_json::Value::Null);
    assert_eq!(probe["replySampledAtMs"], serde_json::Value::Null);
    assert_eq!(probe["replyRttMs"], serde_json::Value::Null);
    assert_eq!(probe["activePath"]["relay"]["peer_node_id"], "peer-1");
    assert_eq!(probe["tunnelPeerVirtualIp"], "100.64.0.2");
    assert_eq!(probe["observedRttMs"], serde_json::Value::Null);
    assert_eq!(probe["packetLossPpm"], serde_json::Value::Null);
    assert_eq!(probe["pathScore"], serde_json::Value::Null);
    assert_eq!(probe["derpClusterId"], serde_json::Value::Null);
    assert_eq!(probe["derpNodeId"], serde_json::Value::Null);
}

#[test]
fn json_facade_accepts_probe_timeout_override() {
    let facade = JsonAppCoreFacade::new(DefaultAppCoreFacade::new(
        FakeController,
        FakeP2P,
        FakeRelay,
        FakeDerpPool,
        NoopPathManager::default(),
        NoopTunnelManager,
    ));

    facade
        .invoke(
            "register",
            json!({
                "email": "user@example.com",
                "password": "secret"
            }),
        )
        .unwrap();
    facade
        .invoke(
            "registerNode",
            json!({
                "deviceId": "dev-1",
                "nodeId": "node-1",
                "nodePublicKey": "node-pk",
                "capabilities": ["relay"]
            }),
        )
        .unwrap();
    facade
        .invoke(
            "bootstrap",
            json!({
                "nodeId": "node-1",
                "networkId": "net-1"
            }),
        )
        .unwrap();
    facade
        .invoke(
            "connect",
            json!({
                "networkId": "net-1",
                "peerNodeId": "peer-1"
            }),
        )
        .unwrap();

    let probe = facade
        .invoke(
            "probe",
            json!({
                "payload": "hello",
                "probeTimeoutMs": 0
            }),
        )
        .unwrap();

    assert_eq!(probe["replyObserved"], false);
}
use std::sync::Mutex;
