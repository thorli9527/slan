use std::io::{Read, Write};
use std::net::{Shutdown, TcpStream};
use std::process::Command;
use std::sync::Arc;
use std::time::{Duration, SystemTime, UNIX_EPOCH};

use control_mqtt_client::{ControlMqttClient, ControlMqttConfig, ControlMqttEvent};
use controller_client::{
    ControllerClient, HttpControllerClient, RegisterDeviceRequest, RegisterNodeRequest,
    RegisterRequest, TcpJsonHttpTransport,
};
use ffi_bridge::{AppCoreFacade, AppCoreSnapshot, DefaultAppCoreFacade};
use p2p::{P2PConnector, PeerCandidate};
use relay_client::{InMemoryDerpPool, InMemoryPathManager, SocketRelayClient};
use serde_json::{json, Value};
use slan_app_core::{
    BootstrapConfig, ConnectionPath, ConnectionState, ControlPlaneConfig, Device, DnsConfig,
    Endpoint, NetworkMap, Node, Peer, RelayConfig, RelayRegion, Route, Session,
};
use tunnel::InMemoryTunnelManager;

#[test]
#[ignore = "requires local docker stack on 127.0.0.1:28080"]
fn live_control_mqtt_receives_device_ip_reassigned() {
    let base_url = std::env::var("SLAN_LIVE_BASE_URL")
        .unwrap_or_else(|_| "http://127.0.0.1:28080".to_string());
    let client = HttpControllerClient::new(base_url.clone(), TcpJsonHttpTransport::default());
    let suffix = unique_suffix();
    let owner_email = format!("owner.live.{suffix}@example.com");
    let member_email = format!("member.live.{suffix}@example.com");
    let password = "Passw0rd!".to_string();

    let owner_session = client
        .register(RegisterRequest {
            email: owner_email,
            password: password.clone(),
        })
        .expect("register owner");
    let member_session = client
        .register(RegisterRequest {
            email: member_email,
            password,
        })
        .expect("register member");

    let owner_device = client
        .register_device(
            &owner_session.access_token,
            RegisterDeviceRequest {
                name: "owner-live".into(),
                platform: "macos".into(),
                machine_id: format!("owner-machine-{suffix}"),
                public_key: format!("owner-pub-{suffix}"),
            },
        )
        .expect("register owner device");
    let member_device = client
        .register_device(
            &member_session.access_token,
            RegisterDeviceRequest {
                name: "member-live".into(),
                platform: "macos".into(),
                machine_id: format!("member-machine-{suffix}"),
                public_key: format!("member-pub-{suffix}"),
            },
        )
        .expect("register member device");

    let network = send_json(
        &base_url,
        "POST",
        "/networks",
        Some(&owner_session.access_token),
        json!({
            "name": format!("live-net-{suffix}"),
            "cidr": "10.91.0.0/24",
            "bindDeviceId": owner_device.device_id,
        }),
    )
    .expect("create network");
    let network_id = network["networkId"]
        .as_str()
        .expect("networkId string")
        .to_string();

    let join_key_resp = send_json(
        &base_url,
        "PUT",
        &format!("/networks/{network_id}/join-key"),
        Some(&owner_session.access_token),
        json!({}),
    )
    .expect("set join key");
    let join_key = join_key_resp["joinKey"]
        .as_str()
        .expect("joinKey string")
        .to_string();

    send_json(
        &base_url,
        "POST",
        "/networks/join-by-key",
        Some(&member_session.access_token),
        json!({
            "joinKey": join_key,
            "deviceId": member_device.device_id,
        }),
    )
    .expect("member join by key");

    send_json(
        &base_url,
        "POST",
        &format!("/networks/{network_id}/activate"),
        Some(&member_session.access_token),
        json!({ "deviceId": member_device.device_id }),
    )
    .expect("activate member network");
    let member_node = client
        .register_node(
            &member_session.access_token,
            RegisterNodeRequest {
                device_id: member_device.device_id.clone(),
                node_id: format!("node-live-{suffix}"),
                node_public_key: format!("node-pub-{suffix}"),
                capabilities: vec!["desktop".into()],
            },
        )
        .expect("register member node");
    let bootstrap = curl_json(
        &base_url,
        "POST",
        "/bootstrap",
        Some(&member_session.access_token),
        json!({
            "nodeId": member_node.node_id,
            "networkId": network_id,
        }),
    )
    .expect("bootstrap member");
    let control_mqtt_url = local_control_mqtt_url(
        bootstrap["controlPlane"]["wsUrl"]
            .as_str()
            .expect("controlPlane.wsUrl"),
    );
    let session_token = bootstrap["sessionToken"]
        .as_str()
        .expect("sessionToken")
        .to_string();

    let mut control_mqtt_client = ControlMqttClient::connect(&ControlMqttConfig {
        access_token: member_session.access_token.clone(),
        session_token: session_token.clone(),
        user_id: member_session.user_id.clone(),
        device_id: member_device.device_id.clone(),
        node_id: member_node.node_id.clone(),
        node_public_key: member_node.node_public_key.clone(),
        network_id: network_id.clone(),
        capabilities: member_node.capabilities.clone(),
        mqtt: member_device.mqtt.clone().expect("member mqtt credential"),
    })
    .expect("connect control mqtt");
    control_mqtt_client
        .bootstrap_session(&ControlMqttConfig {
            access_token: member_session.access_token.clone(),
            session_token,
            user_id: member_session.user_id.clone(),
            device_id: member_device.device_id.clone(),
            node_id: member_node.node_id.clone(),
            node_public_key: member_node.node_public_key.clone(),
            network_id: network_id.clone(),
            capabilities: member_node.capabilities.clone(),
            mqtt: member_device.mqtt.clone().expect("member mqtt credential"),
        })
        .expect("bootstrap control mqtt session");

    let assignments = send_json(
        &base_url,
        "GET",
        &format!("/networks/{network_id}/assignments"),
        Some(&owner_session.access_token),
        Value::Null,
    )
    .expect("list assignments");
    let member_assignment = assignments["items"]
        .as_array()
        .expect("assignment items array")
        .iter()
        .find(|item| item["deviceId"].as_str() == Some(member_device.device_id.as_str()))
        .expect("member assignment");
    let attachment_id = member_assignment["attachmentId"]
        .as_str()
        .expect("attachment id")
        .to_string();
    let next_ip = "10.91.0.77";

    let updated = send_json(
        &base_url,
        "PUT",
        &format!("/networks/{network_id}/attachments/{attachment_id}/ip"),
        Some(&owner_session.access_token),
        json!({ "virtualIp": next_ip }),
    )
    .expect("update member virtual ip");
    assert_eq!(updated["virtualIp"].as_str(), Some(next_ip));

    let events = control_mqtt_client
        .drain_pending_events(Duration::from_secs(2), 8)
        .expect("read pending events");
    let update = events.into_iter().find_map(|event| match event {
        ControlMqttEvent::DeviceIPReassigned(update) => Some(update),
        ControlMqttEvent::PeerUpdate(update)
            if update
                .peer
                .virtual_ips
                .iter()
                .any(|virtual_ip| virtual_ip == next_ip) =>
        {
            None
        }
        _ => None,
    });
    let update = update.expect("device_ip_reassigned event");
    assert_eq!(update.network_id, network_id);
    assert_eq!(update.device_id, member_device.device_id);
    assert_eq!(update.attachment_id, attachment_id);
    assert_eq!(update.virtual_ip, next_ip);
}

#[test]
#[ignore = "requires local docker stack on 127.0.0.1:28080"]
fn live_facade_control_sync_reconciles_device_and_tunnel_ip() {
    let base_url = std::env::var("SLAN_LIVE_BASE_URL")
        .unwrap_or_else(|_| "http://127.0.0.1:28080".to_string());
    let client = HttpControllerClient::new(base_url.clone(), TcpJsonHttpTransport::default());
    let suffix = unique_suffix();
    let owner_email = format!("owner.facade.{suffix}@example.com");
    let member_email = format!("member.facade.{suffix}@example.com");
    let password = "Passw0rd!".to_string();

    let owner_session = client
        .register(RegisterRequest {
            email: owner_email,
            password: password.clone(),
        })
        .expect("register owner");
    let member_session = client
        .register(RegisterRequest {
            email: member_email,
            password,
        })
        .expect("register member");

    let owner_device = client
        .register_device(
            &owner_session.access_token,
            RegisterDeviceRequest {
                name: "owner-facade".into(),
                platform: "macos".into(),
                machine_id: format!("owner-facade-machine-{suffix}"),
                public_key: format!("owner-facade-pub-{suffix}"),
            },
        )
        .expect("register owner device");
    let member_device = client
        .register_device(
            &member_session.access_token,
            RegisterDeviceRequest {
                name: "member-facade".into(),
                platform: "macos".into(),
                machine_id: format!("member-facade-machine-{suffix}"),
                public_key: format!("member-facade-pub-{suffix}"),
            },
        )
        .expect("register member device");

    let network = send_json(
        &base_url,
        "POST",
        "/networks",
        Some(&owner_session.access_token),
        json!({
            "name": format!("facade-net-{suffix}"),
            "cidr": "10.92.0.0/24",
            "bindDeviceId": owner_device.device_id,
        }),
    )
    .expect("create network");
    let network_id = network["networkId"]
        .as_str()
        .expect("networkId")
        .to_string();

    let join_key = send_json(
        &base_url,
        "PUT",
        &format!("/networks/{network_id}/join-key"),
        Some(&owner_session.access_token),
        json!({}),
    )
    .expect("set join key")["joinKey"]
        .as_str()
        .expect("joinKey")
        .to_string();

    send_json(
        &base_url,
        "POST",
        "/networks/join-by-key",
        Some(&member_session.access_token),
        json!({
            "joinKey": join_key,
            "deviceId": member_device.device_id,
        }),
    )
    .expect("member join by key");

    send_json(
        &base_url,
        "POST",
        &format!("/networks/{network_id}/activate"),
        Some(&owner_session.access_token),
        json!({ "deviceId": owner_device.device_id }),
    )
    .expect("activate owner");
    send_json(
        &base_url,
        "POST",
        &format!("/networks/{network_id}/activate"),
        Some(&member_session.access_token),
        json!({ "deviceId": member_device.device_id }),
    )
    .expect("activate member");

    let owner_node = client
        .register_node(
            &owner_session.access_token,
            RegisterNodeRequest {
                device_id: owner_device.device_id.clone(),
                node_id: format!("owner-node-{suffix}"),
                node_public_key: format!("owner-node-pub-{suffix}"),
                capabilities: vec!["desktop".into()],
            },
        )
        .expect("register owner node");
    let member_node = client
        .register_node(
            &member_session.access_token,
            RegisterNodeRequest {
                device_id: member_device.device_id.clone(),
                node_id: format!("member-node-{suffix}"),
                node_public_key: format!("member-node-pub-{suffix}"),
                capabilities: vec!["desktop".into()],
            },
        )
        .expect("register member node");

    let owner_bootstrap = bootstrap_config_from_json(
        &curl_json(
            &base_url,
            "POST",
            "/bootstrap",
            Some(&owner_session.access_token),
            json!({
                "nodeId": owner_node.node_id,
                "networkId": network_id,
            }),
        )
        .expect("owner bootstrap"),
    );
    let member_bootstrap = bootstrap_config_from_json(
        &curl_json(
            &base_url,
            "POST",
            "/bootstrap",
            Some(&member_session.access_token),
            json!({
                "nodeId": member_node.node_id,
                "networkId": network_id,
            }),
        )
        .expect("member bootstrap"),
    );
    let old_member_ip = member_bootstrap
        .device
        .virtual_ip
        .clone()
        .expect("member bootstrap virtual ip");

    let owner_tunnel = Arc::new(InMemoryTunnelManager::default());
    let owner_facade = build_live_facade(
        &base_url,
        owner_session.clone(),
        owner_device.clone(),
        owner_node.clone(),
        owner_bootstrap.clone(),
        owner_tunnel.clone(),
    );
    let member_facade = build_live_facade(
        &base_url,
        member_session.clone(),
        member_device.clone(),
        member_node.clone(),
        member_bootstrap.clone(),
        Arc::new(InMemoryTunnelManager::default()),
    );

    member_facade
        .control_sync()
        .expect("member control sync warmup");
    owner_facade
        .control_sync()
        .expect("owner control sync warmup");

    let connect_state = owner_facade
        .connect(network_id.clone(), member_node.node_id.clone())
        .expect("owner connect to member");
    assert!(matches!(
        connect_state,
        ConnectionState::Connected(ConnectionPath::P2P)
    ));
    let owner_before = owner_facade.snapshot().expect("owner snapshot before");
    assert_eq!(
        owner_before
            .tunnel_runtime
            .as_ref()
            .expect("owner tunnel runtime before")
            .peer_virtual_ip,
        old_member_ip
    );

    let assignments = send_json(
        &base_url,
        "GET",
        &format!("/networks/{network_id}/assignments"),
        Some(&owner_session.access_token),
        Value::Null,
    )
    .expect("list assignments");
    let member_assignment = assignments["items"]
        .as_array()
        .expect("assignment items")
        .iter()
        .find(|item| item["deviceId"].as_str() == Some(member_device.device_id.as_str()))
        .expect("member assignment");
    let member_attachment_id = member_assignment["attachmentId"]
        .as_str()
        .expect("member attachment");
    let next_ip = "10.92.0.77";

    send_json(
        &base_url,
        "PUT",
        &format!("/networks/{network_id}/attachments/{member_attachment_id}/ip"),
        Some(&owner_session.access_token),
        json!({ "virtualIp": next_ip }),
    )
    .expect("update member ip");

    let mut member_snapshot = None;
    for _ in 0..6 {
        member_facade
            .control_sync()
            .expect("member control sync after update");
        let snapshot = member_facade.snapshot().expect("member snapshot after");
        if snapshot
            .current_bootstrap
            .as_ref()
            .and_then(|bootstrap| bootstrap.device.virtual_ip.as_deref())
            == Some(next_ip)
        {
            member_snapshot = Some(snapshot);
            break;
        }
        std::thread::sleep(Duration::from_millis(150));
    }
    let member_snapshot = member_snapshot.expect("member snapshot converged");
    assert_eq!(
        member_snapshot
            .current_device
            .as_ref()
            .and_then(|device| device.virtual_ip.as_deref()),
        Some(next_ip)
    );
    assert_eq!(
        member_snapshot
            .current_bootstrap
            .as_ref()
            .and_then(|bootstrap| bootstrap.device.virtual_ip.as_deref()),
        Some(next_ip)
    );

    let owner_synced = owner_facade
        .control_sync()
        .expect("owner control sync after update");
    assert_eq!(
        owner_synced
            .network_map
            .as_ref()
            .expect("owner network map")
            .network_id,
        network_id
    );
}

fn unique_suffix() -> u128 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .expect("unix epoch")
        .as_millis()
}

fn local_control_mqtt_url(raw: &str) -> String {
    if raw.starts_with("mqtt://") {
        return "mqtt://127.0.0.1:1883".to_string();
    }
    "mqtt://127.0.0.1:1883".to_string()
}

fn build_live_facade(
    base_url: &str,
    session: Session,
    device: Device,
    node: Node,
    bootstrap: BootstrapConfig,
    tunnel_manager: Arc<InMemoryTunnelManager>,
) -> DefaultAppCoreFacade<
    HttpControllerClient<TcpJsonHttpTransport>,
    Arc<TestP2PConnector>,
    Arc<SocketRelayClient>,
    Arc<InMemoryDerpPool>,
    Arc<InMemoryPathManager<Arc<InMemoryDerpPool>, Arc<SocketRelayClient>, Arc<TestP2PConnector>>>,
    Arc<InMemoryTunnelManager>,
> {
    let controller =
        HttpControllerClient::new(base_url.to_string(), TcpJsonHttpTransport::default());
    let p2p = Arc::new(TestP2PConnector);
    let relay = Arc::new(SocketRelayClient::default());
    let derp_pool = Arc::new(InMemoryDerpPool::default());
    let path_manager = Arc::new(InMemoryPathManager::new(
        derp_pool.clone(),
        relay.clone(),
        p2p.clone(),
    ));
    let facade = DefaultAppCoreFacade::new(
        controller,
        p2p,
        relay,
        derp_pool,
        path_manager,
        tunnel_manager,
    );
    facade
        .restore_snapshot(AppCoreSnapshot {
            session: Some(session),
            current_device: Some(device),
            current_node: Some(node),
            current_bootstrap: Some(bootstrap.clone()),
            current_network_id: bootstrap
                .network_map
                .as_ref()
                .map(|network_map| network_map.network_id.clone()),
            ..Default::default()
        })
        .expect("restore live snapshot");
    facade
}

fn bootstrap_config_from_json(raw: &Value) -> BootstrapConfig {
    let device_value = &raw["device"]["device"];
    let device = Device {
        device_id: device_value["deviceId"]
            .as_str()
            .expect("deviceId")
            .to_string(),
        name: device_value["name"]
            .as_str()
            .unwrap_or_default()
            .to_string(),
        platform: device_value["platform"]
            .as_str()
            .unwrap_or_default()
            .to_string(),
        status: device_value["status"]
            .as_str()
            .unwrap_or_default()
            .to_string(),
        virtual_ip: raw["device"]["attachments"]
            .as_array()
            .and_then(|items| items.first())
            .and_then(|item| item["virtualIp"].as_str())
            .map(ToString::to_string),
        public_key: device_value["publicKey"].as_str().map(ToString::to_string),
        mqtt: None,
    };
    let mut control_plane: ControlPlaneConfig =
        serde_json::from_value(raw["controlPlane"].clone()).expect("controlPlane");
    control_plane.session_token = raw["sessionToken"].as_str().map(ToString::to_string);
    control_plane.ws_url = local_control_mqtt_url(&control_plane.ws_url);
    BootstrapConfig {
        device,
        networks: Vec::new(),
        control_plane,
        stun_servers: serde_json::from_value(raw["stunServers"].clone()).unwrap_or_default(),
        relay: serde_json::from_value(raw["relay"].clone()).unwrap_or_else(|_| RelayConfig {
            default_cluster_id: String::new(),
            countries: Vec::new(),
        }),
        derp_map: serde_json::from_value(raw["derpMap"].clone()).ok(),
        network_map: Some(network_map_from_json(&raw["networkMap"])),
    }
}

fn network_map_from_json(raw: &Value) -> NetworkMap {
    let peers = raw["peers"]
        .as_array()
        .map(|items| {
            items
                .iter()
                .map(|item| Peer {
                    node_id: item["nodeId"].as_str().unwrap_or_default().to_string(),
                    device_id: item["deviceId"].as_str().unwrap_or_default().to_string(),
                    public_key: item["publicKey"].as_str().unwrap_or_default().to_string(),
                    status: item["status"].as_str().unwrap_or_default().to_string(),
                    relay_allowed: item["relayAllowed"].as_bool().unwrap_or(false),
                    virtual_ips: item["virtualIps"]
                        .as_array()
                        .map(|values| {
                            values
                                .iter()
                                .filter_map(|value| value.as_str().map(ToString::to_string))
                                .collect()
                        })
                        .unwrap_or_default(),
                    endpoints: item["endpoints"]
                        .as_array()
                        .map(|values| {
                            values
                                .iter()
                                .map(|endpoint| Endpoint {
                                    endpoint_type: endpoint["type"]
                                        .as_str()
                                        .unwrap_or_default()
                                        .to_string(),
                                    address: endpoint["address"]
                                        .as_str()
                                        .unwrap_or_default()
                                        .to_string(),
                                    updated_at: endpoint["updatedAt"].as_i64().unwrap_or_default(),
                                })
                                .collect()
                        })
                        .unwrap_or_default(),
                    allowed_routes: item["allowedRoutes"]
                        .as_array()
                        .map(|values| {
                            values
                                .iter()
                                .filter_map(|value| value.as_str().map(ToString::to_string))
                                .collect()
                        })
                        .unwrap_or_default(),
                })
                .collect()
        })
        .unwrap_or_default();
    NetworkMap {
        self_user_id: raw["selfUserId"].as_str().unwrap_or_default().to_string(),
        self_device_id: raw["selfDeviceId"].as_str().unwrap_or_default().to_string(),
        self_node_id: raw["selfNodeId"].as_str().unwrap_or_default().to_string(),
        network_id: raw["networkId"].as_str().unwrap_or_default().to_string(),
        revision: raw["revision"].as_u64().unwrap_or_default(),
        heartbeat_seconds: raw["heartbeatSeconds"].as_u64().unwrap_or_default() as u32,
        stun_servers: raw["stunServers"]
            .as_array()
            .map(|values| {
                values
                    .iter()
                    .filter_map(|value| value.as_str().map(ToString::to_string))
                    .collect()
            })
            .unwrap_or_default(),
        peers,
        routes: serde_json::from_value::<Vec<Route>>(raw["routes"].clone()).unwrap_or_default(),
        relay_regions: serde_json::from_value::<Vec<RelayRegion>>(raw["relayRegions"].clone())
            .unwrap_or_default(),
        dns: serde_json::from_value::<DnsConfig>(raw["dns"].clone()).unwrap_or(DnsConfig {
            servers: Vec::new(),
            search_domains: Vec::new(),
        }),
        mtu: raw["mtu"].as_u64().map(|value| value as u32),
    }
}

struct TestP2PConnector;

impl P2PConnector for TestP2PConnector {
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

fn send_json(
    base_url: &str,
    method: &str,
    path: &str,
    bearer_token: Option<&str>,
    body: Value,
) -> Result<Value, String> {
    let (host, port) = parse_base_url(base_url)?;
    let mut stream = TcpStream::connect((host.as_str(), port))
        .map_err(|err| format!("connect {host}:{port}: {err}"))?;
    stream
        .set_read_timeout(Some(Duration::from_secs(5)))
        .map_err(|err| err.to_string())?;
    stream
        .set_write_timeout(Some(Duration::from_secs(5)))
        .map_err(|err| err.to_string())?;

    let body_bytes = if body.is_null() {
        Vec::new()
    } else {
        serde_json::to_vec(&body).map_err(|err| err.to_string())?
    };
    let mut wire = format!(
        "{method} {path} HTTP/1.1\r\nHost: {host}:{port}\r\nAccept: application/json\r\nConnection: close\r\n"
    );
    if let Some(token) = bearer_token {
        wire.push_str(&format!("Authorization: Bearer {token}\r\n"));
    }
    if !body_bytes.is_empty() {
        wire.push_str("Content-Type: application/json\r\n");
        wire.push_str(&format!("Content-Length: {}\r\n", body_bytes.len()));
    }
    wire.push_str("\r\n");
    stream
        .write_all(wire.as_bytes())
        .and_then(|_| {
            if body_bytes.is_empty() {
                Ok(())
            } else {
                stream.write_all(&body_bytes)
            }
        })
        .map_err(|err| err.to_string())?;
    let _ = stream.shutdown(Shutdown::Write);

    let mut raw = Vec::new();
    stream
        .read_to_end(&mut raw)
        .map_err(|err| err.to_string())?;
    let response = parse_http_response(&raw)?;
    if !(200..300).contains(&response.status) {
        return Err(format!(
            "unexpected status {} body {}",
            response.status, response.body
        ));
    }
    serde_json::from_slice::<Value>(response.body.as_bytes()).map_err(|err| err.to_string())
}

fn curl_json(
    base_url: &str,
    method: &str,
    path: &str,
    bearer_token: Option<&str>,
    body: Value,
) -> Result<Value, String> {
    let url = format!("{base_url}{path}");
    let mut command = Command::new("curl");
    command.arg("--silent").arg("--show-error");
    command.arg("-X").arg(method);
    command.arg("-H").arg("Content-Type: application/json");
    if let Some(token) = bearer_token {
        command
            .arg("-H")
            .arg(format!("Authorization: Bearer {token}"));
    }
    if !body.is_null() {
        command.arg("-d").arg(body.to_string());
    }
    command.arg(url);
    let output = command.output().map_err(|err| err.to_string())?;
    if !output.status.success() {
        return Err(String::from_utf8_lossy(&output.stderr).trim().to_string());
    }
    serde_json::from_slice::<Value>(&output.stdout).map_err(|err| {
        format!(
            "decode curl json: {err}; body={}",
            String::from_utf8_lossy(&output.stdout)
        )
    })
}

fn parse_base_url(base_url: &str) -> Result<(String, u16), String> {
    let rest = base_url
        .trim()
        .strip_prefix("http://")
        .ok_or_else(|| format!("unsupported base url: {base_url}"))?;
    let authority = rest.split('/').next().unwrap_or(rest);
    let (host, port) = match authority.split_once(':') {
        Some((host, port)) => (
            host.to_string(),
            port.parse::<u16>()
                .map_err(|err| format!("invalid port in {base_url}: {err}"))?,
        ),
        None => (authority.to_string(), 80),
    };
    Ok((host, port))
}

struct ParsedResponse {
    status: u16,
    body: String,
}

fn parse_http_response(raw: &[u8]) -> Result<ParsedResponse, String> {
    let separator = b"\r\n\r\n";
    let header_end = raw
        .windows(separator.len())
        .position(|window| window == separator)
        .ok_or_else(|| "invalid http response".to_string())?;
    let header_text =
        String::from_utf8(raw[..header_end].to_vec()).map_err(|err| err.to_string())?;
    let body = String::from_utf8(raw[header_end + separator.len()..].to_vec())
        .map_err(|err| err.to_string())?;
    let status = header_text
        .lines()
        .next()
        .and_then(|line| line.split_whitespace().nth(1))
        .ok_or_else(|| "missing status line".to_string())?
        .parse::<u16>()
        .map_err(|err| err.to_string())?;
    Ok(ParsedResponse { status, body })
}
