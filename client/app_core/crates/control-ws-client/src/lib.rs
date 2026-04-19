use std::fmt;
use std::io::{Read, Write};
use std::net::TcpStream;
use std::time::Duration;

use serde::{Deserialize, Serialize};
use serde_json::Value;
use slan_app_core::{
    DnsConfig, Endpoint, NetworkMap, Peer, RelayEndpoint, RelayRegion, RelayTicket, Route,
};

const STATIC_WS_KEY: &str = "dGhlIHNhbXBsZSBub25jZQ==";
const DEFAULT_IO_TIMEOUT: Duration = Duration::from_secs(5);

#[derive(Debug, Clone)]
pub struct ControlWsConfig {
    pub ws_url: String,
    pub session_token: String,
    pub user_id: String,
    pub device_id: String,
    pub node_id: String,
    pub node_public_key: String,
    pub network_id: String,
    pub capabilities: Vec<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct NodeHelloAck {
    pub control_session_id: String,
    pub heartbeat_seconds: u32,
    pub network_revision: u64,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct Pong {
    pub timestamp: i64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ControlSessionBootstrap {
    pub ack: NodeHelloAck,
    pub network_map: NetworkMap,
}

#[derive(Debug, Clone)]
pub enum ControlWsEvent {
    PeerUpdate(ControlWsPeerUpdate),
    PeerRemove(ControlWsPeerRemove),
    ConnectPlan(ControlWsConnectPlan),
}

#[derive(Debug, Clone)]
pub struct ControlWsPeerUpdate {
    pub network_id: String,
    pub revision: u64,
    pub peer: Peer,
}

#[derive(Debug, Clone)]
pub struct ControlWsPeerRemove {
    pub network_id: String,
    pub revision: u64,
    pub peer_node_id: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ControlWsConnectPlan {
    pub peer_node_id: String,
    pub prefer_direct: bool,
    #[serde(default)]
    pub paths: Vec<ControlWsPathOption>,
    #[serde(default)]
    pub derp_cluster_id: String,
    #[serde(default)]
    pub preferred_derp_node_ids: Vec<String>,
    #[serde(default)]
    pub relay_ticket: Option<RelayTicket>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ControlWsPathOption {
    pub path_type: String,
    pub endpoint: String,
    pub priority: i32,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ControlWsConnectionStateReport {
    pub network_id: String,
    pub peer_node_id: String,
    pub path: String,
    pub state: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub reason: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub observed_rtt_ms: Option<u32>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub packet_loss_ppm: Option<u32>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub path_score: Option<u32>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub derp_node_id: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ControlWsPathHealthReport {
    pub network_id: String,
    pub peer_node_id: String,
    pub path_type: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub endpoint: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub derp_node_id: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub observed_rtt_ms: Option<u32>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub packet_loss_ppm: Option<u32>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub path_score: Option<u32>,
    pub sampled_at_ms: u64,
}

#[derive(Debug)]
pub struct ControlWsClient {
    stream: TcpStream,
}

impl ControlWsClient {
    pub fn connect(config: &ControlWsConfig) -> Result<Self, String> {
        let endpoint = parse_ws_url(&config.ws_url)?;
        let mut stream = TcpStream::connect(endpoint.authority.as_str())
            .map_err(|err| format!("connect control ws {}: {err}", endpoint.authority))?;
        stream
            .set_read_timeout(Some(DEFAULT_IO_TIMEOUT))
            .map_err(|err| format!("set read timeout: {err}"))?;
        stream
            .set_write_timeout(Some(DEFAULT_IO_TIMEOUT))
            .map_err(|err| format!("set write timeout: {err}"))?;

        let request = format!(
            "GET {} HTTP/1.1\r\nHost: {}\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Version: 13\r\nSec-WebSocket-Key: {}\r\n\r\n",
            endpoint.path, endpoint.authority, STATIC_WS_KEY
        );
        stream
            .write_all(request.as_bytes())
            .map_err(|err| format!("write websocket handshake: {err}"))?;
        stream
            .flush()
            .map_err(|err| format!("flush websocket handshake: {err}"))?;

        let response = read_http_response(&mut stream)?;
        if !response.starts_with("HTTP/1.1 101") && !response.starts_with("HTTP/1.0 101") {
            return Err(format!(
                "control ws upgrade rejected: {}",
                first_response_line(&response)
            ));
        }

        Ok(Self { stream })
    }

    pub fn bootstrap_session(
        &mut self,
        config: &ControlWsConfig,
    ) -> Result<ControlSessionBootstrap, String> {
        let hello = Envelope {
            msg_type: "node_hello".to_string(),
            request_id: Some("node-hello".to_string()),
            payload: serde_json::to_value(NodeHelloPayload::from(config.clone()))
                .map_err(|err| format!("encode node_hello payload: {err}"))?,
        };
        self.send_envelope(&hello)?;

        let ack_env = self.read_envelope()?;
        if ack_env.msg_type == "error" {
            return Err(format_control_error(ack_env.payload));
        }
        if ack_env.msg_type != "node_hello_ack" {
            return Err(format!(
                "unexpected control ws response: {}",
                ack_env.msg_type
            ));
        }
        let ack: NodeHelloAck = serde_json::from_value(ack_env.payload)
            .map_err(|err| format!("decode node_hello_ack: {err}"))?;

        let network_env = self.read_envelope()?;
        if network_env.msg_type == "error" {
            return Err(format_control_error(network_env.payload));
        }
        if network_env.msg_type != "network_map_response" {
            return Err(format!(
                "unexpected control ws response after node_hello_ack: {}",
                network_env.msg_type
            ));
        }
        let network_map = decode_network_map_response(network_env.payload)?;

        Ok(ControlSessionBootstrap { ack, network_map })
    }

    pub fn request_network_map(&mut self, network_id: &str) -> Result<NetworkMap, String> {
        self.request_network_map_since(network_id, 0)
    }

    pub fn request_network_map_since(
        &mut self,
        network_id: &str,
        last_revision: u64,
    ) -> Result<NetworkMap, String> {
        let request = Envelope {
            msg_type: "network_map_request".to_string(),
            request_id: Some("network-map".to_string()),
            payload: serde_json::json!({
                "networkId": network_id,
                "lastRevision": last_revision,
            }),
        };
        self.send_envelope(&request)?;
        let response = self.read_envelope()?;
        if response.msg_type == "error" {
            return Err(format_control_error(response.payload));
        }
        if response.msg_type != "network_map_response" {
            return Err(format!(
                "unexpected network map response: {}",
                response.msg_type
            ));
        }
        decode_network_map_response(response.payload)
    }

    pub fn ping(&mut self, timestamp: i64) -> Result<Pong, String> {
        let request = Envelope {
            msg_type: "ping".to_string(),
            request_id: Some("ping".to_string()),
            payload: serde_json::json!({
                "timestamp": timestamp,
            }),
        };
        self.send_envelope(&request)?;
        let response = self.read_envelope()?;
        if response.msg_type == "error" {
            return Err(format_control_error(response.payload));
        }
        if response.msg_type != "pong" {
            return Err(format!(
                "unexpected control ws response to ping: {}",
                response.msg_type
            ));
        }
        serde_json::from_value(response.payload).map_err(|err| format!("decode pong: {err}"))
    }

    pub fn send_connection_state(
        &mut self,
        report: &ControlWsConnectionStateReport,
    ) -> Result<(), String> {
        let request = Envelope {
            msg_type: "connection_state".to_string(),
            request_id: Some("connection-state".to_string()),
            payload: serde_json::to_value(report)
                .map_err(|err| format!("encode connection_state payload: {err}"))?,
        };
        self.send_envelope(&request)
    }

    pub fn send_path_health_report(
        &mut self,
        report: &ControlWsPathHealthReport,
    ) -> Result<(), String> {
        let request = Envelope {
            msg_type: "path_health_report".to_string(),
            request_id: Some("path-health".to_string()),
            payload: serde_json::to_value(report)
                .map_err(|err| format!("encode path_health_report payload: {err}"))?,
        };
        self.send_envelope(&request)
    }

    pub fn read_event(&mut self) -> Result<ControlWsEvent, String> {
        let response = self.read_envelope()?;
        if response.msg_type == "error" {
            return Err(format_control_error(response.payload));
        }
        match response.msg_type.as_str() {
            "peer_update" => {
                let update: PeerUpdateWire = serde_json::from_value(response.payload)
                    .map_err(|err| format!("decode peer_update: {err}"))?;
                Ok(ControlWsEvent::PeerUpdate(update.into()))
            }
            "peer_remove" => {
                let remove: PeerRemoveWire = serde_json::from_value(response.payload)
                    .map_err(|err| format!("decode peer_remove: {err}"))?;
                Ok(ControlWsEvent::PeerRemove(remove.into()))
            }
            "connect_plan" => {
                let plan: ControlWsConnectPlan = serde_json::from_value(response.payload)
                    .map_err(|err| format!("decode connect_plan: {err}"))?;
                Ok(ControlWsEvent::ConnectPlan(plan))
            }
            other => Err(format!("unexpected control ws event: {other}")),
        }
    }

    pub fn drain_pending_events(
        &mut self,
        timeout: Duration,
        limit: usize,
    ) -> Result<Vec<ControlWsEvent>, String> {
        self.stream
            .set_read_timeout(Some(timeout))
            .map_err(|err| format!("set drain read timeout: {err}"))?;
        let mut events = Vec::new();
        for _ in 0..limit {
            match self.read_event() {
                Ok(event) => events.push(event),
                Err(err) if is_control_ws_timeout_error(&err) => break,
                Err(err) => {
                    self.stream
                        .set_read_timeout(Some(DEFAULT_IO_TIMEOUT))
                        .map_err(|reset_err| format!("reset read timeout after error: {reset_err}"))?;
                    return Err(err);
                }
            }
        }
        self.stream
            .set_read_timeout(Some(DEFAULT_IO_TIMEOUT))
            .map_err(|err| format!("reset read timeout: {err}"))?;
        Ok(events)
    }

    fn send_envelope(&mut self, envelope: &Envelope) -> Result<(), String> {
        let payload =
            serde_json::to_vec(envelope).map_err(|err| format!("encode ws envelope: {err}"))?;
        let frame = encode_text_frame(&payload);
        self.stream
            .write_all(&frame)
            .map_err(|err| format!("write websocket frame: {err}"))?;
        self.stream
            .flush()
            .map_err(|err| format!("flush websocket frame: {err}"))?;
        Ok(())
    }

    fn read_envelope(&mut self) -> Result<Envelope, String> {
        let payload = decode_text_frame(&mut self.stream)?;
        serde_json::from_slice::<Envelope>(&payload)
            .map_err(|err| format!("decode ws envelope: {err}"))
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct Envelope {
    #[serde(rename = "type")]
    msg_type: String,
    #[serde(default)]
    request_id: Option<String>,
    #[serde(default)]
    payload: Value,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
struct NodeHelloPayload {
    user_id: String,
    device_id: String,
    node_id: String,
    network_id: String,
    session_token: String,
    node_public_key: String,
    capabilities: Vec<String>,
}

impl From<ControlWsConfig> for NodeHelloPayload {
    fn from(value: ControlWsConfig) -> Self {
        Self {
            user_id: value.user_id,
            device_id: value.device_id,
            node_id: value.node_id,
            network_id: value.network_id,
            session_token: value.session_token,
            node_public_key: value.node_public_key,
            capabilities: value.capabilities,
        }
    }
}

#[derive(Debug, Clone)]
struct WsEndpoint {
    authority: String,
    path: String,
}

fn parse_ws_url(url: &str) -> Result<WsEndpoint, String> {
    let trimmed = url.trim();
    let stripped = trimmed
        .strip_prefix("ws://")
        .ok_or_else(|| "only ws:// control plane URLs are currently supported".to_string())?;
    let (authority, path) = match stripped.split_once('/') {
        Some((authority, rest)) => (authority.to_string(), format!("/{}", rest)),
        None => (stripped.to_string(), "/".to_string()),
    };
    if authority.trim().is_empty() {
        return Err("invalid control plane ws url".to_string());
    }
    Ok(WsEndpoint { authority, path })
}

fn read_http_response(stream: &mut TcpStream) -> Result<String, String> {
    let mut buf = Vec::with_capacity(1024);
    let mut chunk = [0_u8; 512];
    loop {
        let read = stream
            .read(&mut chunk)
            .map_err(|err| format!("read websocket handshake response: {err}"))?;
        if read == 0 {
            return Err("control ws closed during handshake".to_string());
        }
        buf.extend_from_slice(&chunk[..read]);
        if buf.windows(4).any(|window| window == b"\r\n\r\n") {
            return String::from_utf8(buf)
                .map_err(|err| format!("invalid handshake response utf8: {err}"));
        }
    }
}

fn first_response_line(response: &str) -> &str {
    response.lines().next().unwrap_or("invalid response")
}

fn encode_text_frame(payload: &[u8]) -> Vec<u8> {
    let mut frame = Vec::with_capacity(payload.len() + 16);
    frame.push(0x81);
    let mask_key = [0x12_u8, 0x34, 0x56, 0x78];
    if payload.len() < 126 {
        frame.push(0x80 | payload.len() as u8);
    } else if payload.len() <= u16::MAX as usize {
        frame.push(0x80 | 126);
        frame.extend_from_slice(&(payload.len() as u16).to_be_bytes());
    } else {
        frame.push(0x80 | 127);
        frame.extend_from_slice(&(payload.len() as u64).to_be_bytes());
    }
    frame.extend_from_slice(&mask_key);
    for (idx, byte) in payload.iter().enumerate() {
        frame.push(*byte ^ mask_key[idx % 4]);
    }
    frame
}

fn decode_text_frame(stream: &mut TcpStream) -> Result<Vec<u8>, String> {
    let mut header = [0_u8; 2];
    stream
        .read_exact(&mut header)
        .map_err(|err| format!("read websocket frame header: {err}"))?;

    let opcode = header[0] & 0x0f;
    if opcode == 0x8 {
        return Err("control ws closed by server".to_string());
    }
    if opcode != 0x1 {
        return Err(format!("unsupported websocket opcode: {opcode}"));
    }

    let masked = (header[1] & 0x80) != 0;
    let mut payload_len = (header[1] & 0x7f) as usize;
    if payload_len == 126 {
        let mut ext = [0_u8; 2];
        stream
            .read_exact(&mut ext)
            .map_err(|err| format!("read websocket extended length: {err}"))?;
        payload_len = u16::from_be_bytes(ext) as usize;
    } else if payload_len == 127 {
        let mut ext = [0_u8; 8];
        stream
            .read_exact(&mut ext)
            .map_err(|err| format!("read websocket extended length: {err}"))?;
        payload_len = u64::from_be_bytes(ext) as usize;
    }

    let mut mask = [0_u8; 4];
    if masked {
        stream
            .read_exact(&mut mask)
            .map_err(|err| format!("read websocket mask: {err}"))?;
    }

    let mut payload = vec![0_u8; payload_len];
    stream
        .read_exact(&mut payload)
        .map_err(|err| format!("read websocket payload: {err}"))?;
    if masked {
        for (idx, byte) in payload.iter_mut().enumerate() {
            *byte ^= mask[idx % 4];
        }
    }
    Ok(payload)
}

fn format_control_error(payload: Value) -> String {
    if let Ok(error) = serde_json::from_value::<ControlError>(payload.clone()) {
        if !error.message.trim().is_empty() {
            return format!("{}: {}", error.code, error.message);
        }
        return error.code;
    }
    payload.to_string()
}

fn is_control_ws_timeout_error(error: &str) -> bool {
    let lower = error.to_ascii_lowercase();
    lower.contains("timed out") || lower.contains("would block")
}

#[derive(Debug, Clone, Serialize, Deserialize)]
struct ControlError {
    #[serde(default)]
    code: String,
    #[serde(default)]
    message: String,
}

fn decode_network_map_response(payload: Value) -> Result<NetworkMap, String> {
    let response: NetworkMapResponse = serde_json::from_value(payload)
        .map_err(|err| format!("decode network_map_response: {err}"))?;
    Ok(response.map.into())
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
struct NetworkMapResponse {
    map: NetworkMapWire,
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
struct PeerUpdateWire {
    network_id: String,
    revision: u64,
    peer: PeerWire,
}

impl From<PeerUpdateWire> for ControlWsPeerUpdate {
    fn from(value: PeerUpdateWire) -> Self {
        Self {
            network_id: value.network_id,
            revision: value.revision,
            peer: value.peer.into(),
        }
    }
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
struct PeerRemoveWire {
    network_id: String,
    revision: u64,
    peer_node_id: String,
}

impl From<PeerRemoveWire> for ControlWsPeerRemove {
    fn from(value: PeerRemoveWire) -> Self {
        Self {
            network_id: value.network_id,
            revision: value.revision,
            peer_node_id: value.peer_node_id,
        }
    }
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
struct NetworkMapWire {
    self_user_id: String,
    self_device_id: String,
    self_node_id: String,
    network_id: String,
    revision: u64,
    heartbeat_seconds: u32,
    #[serde(default)]
    stun_servers: Vec<String>,
    #[serde(default)]
    peers: Vec<PeerWire>,
    #[serde(default)]
    routes: Vec<RouteWire>,
    #[serde(default)]
    relay_regions: Vec<RelayRegionWire>,
    dns: DnsConfigWire,
    #[serde(default)]
    mtu: Option<u32>,
}

impl From<NetworkMapWire> for NetworkMap {
    fn from(value: NetworkMapWire) -> Self {
        Self {
            self_user_id: value.self_user_id,
            self_device_id: value.self_device_id,
            self_node_id: value.self_node_id,
            network_id: value.network_id,
            revision: value.revision,
            heartbeat_seconds: value.heartbeat_seconds,
            stun_servers: value.stun_servers,
            peers: value.peers.into_iter().map(Into::into).collect(),
            routes: value.routes.into_iter().map(Into::into).collect(),
            relay_regions: value.relay_regions.into_iter().map(Into::into).collect(),
            dns: value.dns.into(),
            mtu: value.mtu,
        }
    }
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
struct PeerWire {
    node_id: String,
    device_id: String,
    public_key: String,
    status: String,
    relay_allowed: bool,
    #[serde(default)]
    virtual_ips: Vec<String>,
    #[serde(default)]
    endpoints: Vec<EndpointWire>,
    #[serde(default)]
    allowed_routes: Vec<String>,
}

impl From<PeerWire> for Peer {
    fn from(value: PeerWire) -> Self {
        Self {
            node_id: value.node_id,
            device_id: value.device_id,
            public_key: value.public_key,
            status: value.status,
            relay_allowed: value.relay_allowed,
            virtual_ips: value.virtual_ips,
            endpoints: value.endpoints.into_iter().map(Into::into).collect(),
            allowed_routes: value.allowed_routes,
        }
    }
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
struct EndpointWire {
    #[serde(rename = "type")]
    endpoint_type: String,
    address: String,
    updated_at: i64,
}

impl From<EndpointWire> for Endpoint {
    fn from(value: EndpointWire) -> Self {
        Self {
            endpoint_type: value.endpoint_type,
            address: value.address,
            updated_at: value.updated_at,
        }
    }
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
struct RouteWire {
    cidr: String,
    via_node_id: String,
    #[serde(default)]
    metric: Option<String>,
}

impl From<RouteWire> for Route {
    fn from(value: RouteWire) -> Self {
        Self {
            cidr: value.cidr,
            via_node_id: value.via_node_id,
            metric: value.metric,
        }
    }
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
struct DnsConfigWire {
    #[serde(default)]
    servers: Vec<String>,
    #[serde(default)]
    search_domains: Vec<String>,
}

impl From<DnsConfigWire> for DnsConfig {
    fn from(value: DnsConfigWire) -> Self {
        Self {
            servers: value.servers,
            search_domains: value.search_domains,
        }
    }
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
struct RelayRegionWire {
    region_id: String,
    region_name: String,
    #[serde(default)]
    country_code: String,
    #[serde(default)]
    country_name: String,
    #[serde(default)]
    city_code: String,
    #[serde(default)]
    city_name: String,
    #[serde(default)]
    cluster_id: String,
    #[serde(default)]
    cluster_name: String,
    #[serde(default)]
    endpoints: Vec<RelayEndpointWire>,
}

impl From<RelayRegionWire> for RelayRegion {
    fn from(value: RelayRegionWire) -> Self {
        Self {
            region_id: value.region_id,
            region_name: value.region_name,
            country_code: if value.country_code.is_empty() {
                None
            } else {
                Some(value.country_code)
            },
            country_name: if value.country_name.is_empty() {
                None
            } else {
                Some(value.country_name)
            },
            city_code: if value.city_code.is_empty() {
                None
            } else {
                Some(value.city_code)
            },
            city_name: if value.city_name.is_empty() {
                None
            } else {
                Some(value.city_name)
            },
            cluster_id: if value.cluster_id.is_empty() {
                None
            } else {
                Some(value.cluster_id)
            },
            cluster_name: if value.cluster_name.is_empty() {
                None
            } else {
                Some(value.cluster_name)
            },
            endpoints: value.endpoints.into_iter().map(Into::into).collect(),
        }
    }
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
struct RelayEndpointWire {
    endpoint_id: String,
    transport: String,
    address: String,
}

impl From<RelayEndpointWire> for RelayEndpoint {
    fn from(value: RelayEndpointWire) -> Self {
        Self {
            endpoint_id: value.endpoint_id,
            transport: value.transport,
            address: value.address,
        }
    }
}

impl fmt::Display for ControlSessionBootstrap {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(
            f,
            "control session {} heartbeat {} peers {}",
            self.ack.control_session_id,
            self.ack.heartbeat_seconds,
            self.network_map.peers.len()
        )
    }
}
