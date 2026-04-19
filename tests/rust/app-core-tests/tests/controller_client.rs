use std::sync::Mutex;
use std::time::Duration;

use controller_client::{
    ControllerClient, HttpControllerClient, HttpRequest, HttpResponse, JsonHttpTransport,
    RelayTicketRequest, TcpJsonHttpTransport,
};
use serde_json::Value;

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

    let client = HttpControllerClient::new("http://127.0.0.1:8080", transport);
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
            },
        )
        .unwrap();

    let request = client.transport.take_request();
    assert_eq!(request.path, "http://127.0.0.1:8080/relay/tickets");
    let body: Value = serde_json::from_slice(&request.body_json.unwrap()).unwrap();
    assert_eq!(body["derpClusterId"], "cn-local-a");
    assert_eq!(body["preferredDerpNodeIds"][0], "relay-cn-local-udp");
    assert_eq!(body["preferredDerpNodeIds"][1], "relay-cn-local-tcp");
}

#[test]
fn bootstrap_response_parses_derp_map() {
    let transport = RecordingTransport::default();
    transport.respond_with(
        200,
        r#"{
            "device": {"device":{"deviceId":"dev-1","name":"mac","platform":"macos","status":"online","publicKey":"pk"}},
            "networks": [],
            "controlPlane": {"wsUrl":"ws://127.0.0.1:8080/control/ws","heartbeatSeconds":15},
            "stunServers": ["stun:stun.l.google.com:19302"],
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

    let client = HttpControllerClient::new("http://127.0.0.1:8080", transport);
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
fn tcp_transport_rejects_unsupported_scheme_before_network_io() {
    let transport = TcpJsonHttpTransport::new(Duration::from_secs(2));
    let error = transport
        .send(HttpRequest {
            method: controller_client::HttpMethod::Get,
            path: "https://127.0.0.1/auth/login".into(),
            bearer_token: None,
            body_json: None,
        })
        .unwrap_err();
    assert!(error.contains("unsupported url scheme"));
}
