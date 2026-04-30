use std::sync::Mutex;
use std::time::Duration;

use controller_client::{
    ControllerClient, HttpControllerClient, HttpRequest, HttpResponse, JoinNetworkByKeyRequest,
    JoinNetworkRequest, JsonHttpTransport, LoginRequest, RelayTicketRequest, SwitchNetworkRequest,
    TcpJsonHttpTransport,
};
use serde_json::Value;

mod support;

#[derive(Default)]
struct RecordingTransport {
    last_request: Mutex<Option<HttpRequest>>,
    next_response: Mutex<Option<HttpResponse>>,
}

impl RecordingTransport {
    fn respond_with(&self, status: u16, body_json: &str) {
        *self.next_response.lock().unwrap() = Some(HttpResponse {
            status,
            body_json: body_json.as_bytes().to_vec(),
        });
    }

    fn take_request(&self) -> HttpRequest {
        self.last_request
            .lock()
            .unwrap()
            .take()
            .expect("missing recorded request")
    }
}

impl JsonHttpTransport for RecordingTransport {
    fn send(&self, request: HttpRequest) -> Result<HttpResponse, String> {
        *self.last_request.lock().unwrap() = Some(request);
        self.next_response
            .lock()
            .unwrap()
            .take()
            .ok_or_else(|| "missing fake response".to_string())
    }
}

#[test]
fn relay_ticket_request_serializes_derp_fields() {
    let transport = RecordingTransport::default();
    transport.respond_with(
        200,
        r#"{
            "ticketId":"ticket-1",
            "networkId":"net-1",
            "sessionId":"session-1",
            "srcNodeId":"node-a",
            "dstNodeId":"node-b",
            "derpClusterId":"cn-local-a",
            "countryCode":"CN",
            "cityCode":"local",
            "allowedDerpNodeIds":["relay-cn-local-udp","relay-cn-local-tcp"],
            "relayUrl":"udp://127.0.0.1:9000",
            "expiresAt":"2099-01-01T00:00:00Z",
            "signature":"sig"
        }"#,
    );

    let client = HttpControllerClient::new(support::DEV_CONTROL_BASE_URL, transport);
    client
        .issue_relay_ticket(
            "token-1",
            RelayTicketRequest {
                network_id: "net-1".into(),
                src_node_id: "node-a".into(),
                dst_node_id: "node-b".into(),
                derp_cluster_id: Some("cn-local-a".into()),
                preferred_derp_node_ids: vec![
                    "relay-cn-local-udp".into(),
                    "relay-cn-local-tcp".into(),
                ],
                reason: "timeout".into(),
                relay_region_id: None,
            },
        )
        .unwrap();

    let request = client.transport.take_request();
    assert_eq!(
        request.path,
        format!("{}/relay/tickets", support::DEV_CONTROL_BASE_URL)
    );
    let body: Value = serde_json::from_slice(&request.body_json.unwrap()).unwrap();
    assert_eq!(body["derpClusterId"], "cn-local-a");
    assert_eq!(body["preferredDerpNodeIds"][0], "relay-cn-local-udp");
    assert_eq!(body["preferredDerpNodeIds"][1], "relay-cn-local-tcp");
}

#[test]
fn login_request_serializes_optional_device_id() {
    let transport = RecordingTransport::default();
    transport.respond_with(
        200,
        r#"{
            "userId":"user-1",
            "accessToken":"token-1",
            "expiresIn":3600,
            "deviceId":"dev-1"
        }"#,
    );

    let client = HttpControllerClient::new(support::DEV_CONTROL_BASE_URL, transport);
    let session = client
        .login(LoginRequest {
            email: "user@example.com".into(),
            password: "password123".into(),
            device_id: Some("dev-1".into()),
        })
        .unwrap();

    let request = client.transport.take_request();
    let body: Value = serde_json::from_slice(&request.body_json.unwrap()).unwrap();
    assert_eq!(body["deviceId"], "dev-1");
    assert_eq!(session.device_id.as_deref(), Some("dev-1"));
}

#[test]
fn join_network_accepts_member_attachment_response() {
    let transport = RecordingTransport::default();
    transport.respond_with(
        200,
        r#"{
            "member":{
                "memberId":"member-1",
                "networkId":"net-1",
                "deviceId":"dev-1",
                "role":"member",
                "status":"active"
            },
            "attachment":{
                "attachmentId":"att-1",
                "networkId":"net-1",
                "subnetId":"subnet-1",
                "deviceId":"dev-1",
                "virtualIp":"100.64.0.2",
                "status":"active"
            }
        }"#,
    );

    let client = HttpControllerClient::new(support::DEV_CONTROL_BASE_URL, transport);
    client
        .join_network(
            "token-1",
            JoinNetworkRequest {
                network_id: "net-1".into(),
                device_id: "dev-1".into(),
            },
        )
        .unwrap();

    let request = client.transport.take_request();
    assert_eq!(
        request.path,
        format!("{}/networks/net-1/join", support::DEV_CONTROL_BASE_URL)
    );
}

#[test]
fn join_by_key_accepts_network_member_attachment_response() {
    let transport = RecordingTransport::default();
    transport.respond_with(
        200,
        r#"{
            "network":{
                "networkId":"net-1",
                "name":"home",
                "defaultSubnetCidr":"100.64.0.0/24"
            },
            "member":{
                "memberId":"member-1",
                "networkId":"net-1",
                "deviceId":"dev-1",
                "role":"member",
                "status":"active"
            },
            "attachment":{
                "attachmentId":"att-1",
                "networkId":"net-1",
                "subnetId":"subnet-1",
                "deviceId":"dev-1",
                "virtualIp":"100.64.0.2",
                "status":"active"
            }
        }"#,
    );

    let client = HttpControllerClient::new(support::DEV_CONTROL_BASE_URL, transport);
    let joined = client
        .join_network_by_key(
            "token-1",
            JoinNetworkByKeyRequest {
                join_key: "invite-1".into(),
                device_id: "dev-1".into(),
            },
        )
        .unwrap();

    let request = client.transport.take_request();
    assert_eq!(
        request.path,
        format!("{}/networks/join-by-key", support::DEV_CONTROL_BASE_URL)
    );
    assert_eq!(joined.network_id, "net-1");
    assert_eq!(joined.attachment_id.as_deref(), Some("att-1"));
    assert_eq!(joined.virtual_ip.as_deref(), Some("100.64.0.2"));
}

#[test]
fn switch_network_posts_to_switch_endpoint() {
    let transport = RecordingTransport::default();
    transport.respond_with(
        200,
        r#"{
            "member":{
                "memberId":"member-1",
                "networkId":"net-1",
                "deviceId":"dev-1",
                "role":"member",
                "status":"active"
            },
            "attachment":{
                "attachmentId":"att-1",
                "networkId":"net-1",
                "subnetId":"subnet-1",
                "deviceId":"dev-1",
                "virtualIp":"100.64.0.2",
                "status":"active"
            }
        }"#,
    );

    let client = HttpControllerClient::new(support::DEV_CONTROL_BASE_URL, transport);
    let switched = client
        .switch_network(
            "token-1",
            SwitchNetworkRequest {
                network_id: "net-1".into(),
                device_id: "dev-1".into(),
            },
        )
        .unwrap();

    let request = client.transport.take_request();
    assert_eq!(
        request.path,
        format!("{}/networks/net-1/switch", support::DEV_CONTROL_BASE_URL)
    );
    let body: Value = serde_json::from_slice(&request.body_json.unwrap()).unwrap();
    assert_eq!(body["deviceId"], "dev-1");
    assert_eq!(switched.network_id, "net-1");
    assert_eq!(switched.attachment_id.as_deref(), Some("att-1"));
}

#[test]
fn bootstrap_response_parses_derp_map() {
    let transport = RecordingTransport::default();
    transport.respond_with(
        200,
        r#"{
            "device": {"device":{"deviceId":"dev-1","name":"mac","platform":"macos","status":"online","publicKey":"pk"}},
            "networks": [],
            "controlPlane": {"wsUrl":"mqtt://127.0.0.1:1883","heartbeatSeconds":15},
            "stunServers": ["stun:127.0.0.1:3478"],
            "relay": {
                "defaultClusterId":"cn-local-a",
                "countries":[{
                    "countryCode":"CN",
                    "countryName":"China",
                    "cities":[{
                        "cityCode":"local",
                        "cityName":"Local",
                        "clusters":[{
                            "clusterId":"cn-local-a",
                            "clusterName":"CN Local A",
                            "nodes":[
                                {"nodeId":"relay-cn-local-udp","transport":"udp","address":"127.0.0.1:9000","priority":10},
                                {"nodeId":"relay-cn-local-tcp","transport":"tcp","address":"127.0.0.1:9001","priority":20}
                            ]
                        }]
                    }]
                }]
            },
            "derpMap": {
                "probeIntervalSeconds": 5,
                "clusters": [{
                    "clusterId": "cn-local-a",
                    "clusterName": "CN Local A",
                    "regionId": "local",
                    "regionName": "Local",
                    "countryCode": "CN",
                    "countryName": "China",
                    "cityCode": "local",
                    "cityName": "Local",
                    "recommendedFanout": 3,
                    "nodes": [{
                        "nodeId":"relay-cn-local-udp",
                        "host":"127.0.0.1",
                        "port":9000,
                        "transport":"udp",
                        "priority":10
                    }]
                }]
            },
            "networkMap": {
                "selfUserId":"user-1",
                "selfDeviceId":"dev-1",
                "selfNodeId":"node-1",
                "networkId":"net-1",
                "revision":1,
                "heartbeatSeconds":15,
                "stunServers":[],
                "peers":[],
                "routes":[],
                "relayRegions":[],
                "dns":{"servers":[],"searchDomains":[]}
            }
        }"#,
    );

    let client = HttpControllerClient::new(support::DEV_CONTROL_BASE_URL, transport);
    let bootstrap = client.bootstrap("token-1", "node-1", "net-1").unwrap();
    let derp_map = bootstrap.derp_map.unwrap();
    assert_eq!(derp_map.clusters.len(), 1);
    let cluster = &derp_map.clusters[0];
    assert_eq!(bootstrap.relay.default_cluster_id, "cn-local-a");
    assert_eq!(cluster.cluster_id, "cn-local-a");
    assert_eq!(cluster.region_id, "local");
    assert_eq!(cluster.country_code.as_deref(), Some("CN"));
    assert_eq!(cluster.city_code.as_deref(), Some("local"));
    assert_eq!(cluster.nodes[0].cluster_id, "cn-local-a");
    assert_eq!(cluster.nodes[0].region_id, "local");
}

#[test]
fn bootstrap_response_projects_disabled_attachment_onto_current_member() {
    let transport = RecordingTransport::default();
    transport.respond_with(
        200,
        r#"{
            "device": {
                "device": {"deviceId":"dev-1","name":"mac","platform":"macos","status":"online","publicKey":"pk"},
                "attachments":[{
                    "attachmentId":"att-1",
                    "networkId":"net-1",
                    "subnetId":"subnet-1",
                    "deviceId":"dev-1",
                    "virtualIp":"100.64.0.10",
                    "status":"disabled"
                }]
            },
            "networks": [{
                "networkId":"net-1",
                "name":"home",
                "defaultSubnetCidr":"100.64.0.0/24",
                "members":[{
                    "memberId":"member-1",
                    "networkId":"net-1",
                    "attachmentId":"att-1",
                    "deviceId":"dev-1",
                    "role":"owner",
                    "status":"active",
                    "virtualIp":"100.64.0.10"
                }]
            }],
            "controlPlane": {"wsUrl":"mqtt://127.0.0.1:1883","heartbeatSeconds":15},
            "relay":{"defaultClusterId":"local","countries":[]},
            "networkMap": {
                "selfUserId":"user-1",
                "selfDeviceId":"dev-1",
                "selfNodeId":"node-1",
                "networkId":"net-1",
                "revision":1,
                "heartbeatSeconds":15,
                "stunServers":[],
                "peers":[],
                "routes":[],
                "relayRegions":[],
                "dns":{"servers":[],"searchDomains":[]}
            }
        }"#,
    );

    let client = HttpControllerClient::new(support::DEV_CONTROL_BASE_URL, transport);
    let bootstrap = client.bootstrap("token-1", "node-1", "net-1").unwrap();

    assert!(bootstrap.device.virtual_ip.is_none());
    assert_eq!(bootstrap.networks.len(), 1);
    assert_eq!(
        bootstrap.networks[0].members[0].status.as_deref(),
        Some("disabled")
    );
}

#[test]
fn tcp_transport_rejects_unsupported_scheme_before_network_io() {
    let transport = TcpJsonHttpTransport::new(Duration::from_secs(2));
    let error = transport
        .send(HttpRequest {
            method: controller_client::HttpMethod::Get,
            path: format!("https://{}/auth/login", support::DEV_RELAY_HOST).into(),
            bearer_token: None,
            body_json: None,
        })
        .unwrap_err();
    assert!(error.contains("unsupported url scheme"));
}
