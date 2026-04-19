use std::io::ErrorKind;
use std::net::{TcpStream, ToSocketAddrs, UdpSocket};
use std::sync::{Arc, Mutex};
use std::time::{Duration, SystemTime, UNIX_EPOCH};

use base64::{engine::general_purpose::STANDARD, Engine as _};
use serde::{Deserialize, Serialize};
use slan_app_core::{
    ConnectionPath, ConnectionState, DerpHealth, DerpLinkSnapshot, DerpLinkState, DerpNodeMeta,
    DerpPoolState, DerpSwitchEvent, ProbeSample, RelayTicket, SwitchReason,
};

use p2p::P2PConnector;

use crate::{
    DerpClient, DerpPool, PathManager, PathManagerError, RelayClient, RelayClientError,
};

enum ActiveSocket {
    Tcp(TcpStream),
    Udp(UdpSocket),
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(tag = "kind", rename_all = "snake_case")]
enum RelayDaemonRequest {
    Ping,
    Attach {
        participant_id: String,
        ticket: RelayTicket,
    },
    Forward {
        session_id: String,
        from_participant_id: String,
        payload_b64: String,
    },
    Detach {
        session_id: String,
        participant_id: String,
    },
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(tag = "kind", rename_all = "snake_case")]
enum RelayDaemonResponse {
    Pong,
    Attached {
        session_id: String,
        peer_participant_id: String,
    },
    Forwarded {
        session_id: String,
        to_participant_id: String,
        bytes_forwarded: usize,
    },
    Detached {
        session_id: String,
        participant_id: String,
    },
    Packet {
        session_id: String,
        from_participant_id: String,
        payload_b64: String,
    },
    Error {
        #[serde(default)]
        code: Option<String>,
        message: String,
    },
}

#[derive(Debug, Clone, PartialEq, Eq)]
enum RelayDaemonErrorCode {
    InvalidTicket,
    InvalidSignature,
    InvalidTimestamp,
    TicketExpired,
    SessionNotFound,
    SessionAlreadyExists,
    UnauthorizedPeer,
    EmptyPayload,
    ClockSkew,
    StoreError,
    ParticipantNotInTicket,
    SessionConflict,
    SessionNotAttached,
    ParticipantNotAttached,
    ParticipantAddressMismatch,
    InvalidPayloadB64,
    PeerNotAttached,
    Unknown(String),
}

impl RelayDaemonErrorCode {
    fn parse(value: Option<&str>) -> Option<Self> {
        let value = value?.trim();
        if value.is_empty() {
            return None;
        }
        Some(match value {
            "invalid_ticket" => Self::InvalidTicket,
            "invalid_signature" => Self::InvalidSignature,
            "invalid_timestamp" => Self::InvalidTimestamp,
            "ticket_expired" => Self::TicketExpired,
            "session_not_found" => Self::SessionNotFound,
            "session_already_exists" => Self::SessionAlreadyExists,
            "unauthorized_peer" => Self::UnauthorizedPeer,
            "empty_payload" => Self::EmptyPayload,
            "clock_skew" => Self::ClockSkew,
            "store_error" => Self::StoreError,
            "participant_not_in_ticket" => Self::ParticipantNotInTicket,
            "session_conflict" => Self::SessionConflict,
            "session_not_attached" => Self::SessionNotAttached,
            "participant_not_attached" => Self::ParticipantNotAttached,
            "participant_address_mismatch" => Self::ParticipantAddressMismatch,
            "invalid_payload_b64" => Self::InvalidPayloadB64,
            "peer_not_attached" => Self::PeerNotAttached,
            other => Self::Unknown(other.to_string()),
        })
    }

    fn as_str(&self) -> &str {
        match self {
            Self::InvalidTicket => "invalid_ticket",
            Self::InvalidSignature => "invalid_signature",
            Self::InvalidTimestamp => "invalid_timestamp",
            Self::TicketExpired => "ticket_expired",
            Self::SessionNotFound => "session_not_found",
            Self::SessionAlreadyExists => "session_already_exists",
            Self::UnauthorizedPeer => "unauthorized_peer",
            Self::EmptyPayload => "empty_payload",
            Self::ClockSkew => "clock_skew",
            Self::StoreError => "store_error",
            Self::ParticipantNotInTicket => "participant_not_in_ticket",
            Self::SessionConflict => "session_conflict",
            Self::SessionNotAttached => "session_not_attached",
            Self::ParticipantNotAttached => "participant_not_attached",
            Self::ParticipantAddressMismatch => "participant_address_mismatch",
            Self::InvalidPayloadB64 => "invalid_payload_b64",
            Self::PeerNotAttached => "peer_not_attached",
            Self::Unknown(value) => value.as_str(),
        }
    }
}

#[derive(Debug, Clone, PartialEq, Eq)]
struct RelayDaemonError {
    code: Option<RelayDaemonErrorCode>,
    message: String,
}

impl RelayDaemonError {
    fn from_wire(code: Option<&str>, message: String) -> Self {
        Self {
            code: RelayDaemonErrorCode::parse(code),
            message,
        }
    }

    fn format_with_prefix(&self, prefix: &str) -> String {
        match self.code.as_ref() {
            Some(code) => format!("{prefix} [{}]: {}", code.as_str(), self.message),
            None => format!("{prefix}: {}", self.message),
        }
    }

    fn into_client_error(self, prefix: &str) -> RelayClientError {
        let message = self.format_with_prefix(prefix);
        RelayClientError::new(
            self.code.as_ref().map(|code| code.as_str().to_string()),
            message,
        )
    }
}

#[derive(Default)]
pub struct SocketDerpClient {
    state: Mutex<DerpClientState>,
}

#[derive(Default)]
struct DerpClientState {
    meta: Option<DerpNodeMeta>,
    ticket: Option<RelayTicket>,
    active_socket: Option<ActiveSocket>,
    last_probe_at_ms: u64,
    last_recv_at_ms: u64,
    closed: bool,
}

impl SocketDerpClient {
    fn now_ms() -> u64 {
        SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap_or_default()
            .as_millis() as u64
    }

    fn connect_tcp(meta: &DerpNodeMeta) -> Result<ActiveSocket, String> {
        let authority = format!("{}:{}", meta.host, meta.port);
        let addr = authority
            .to_socket_addrs()
            .map_err(|err| format!("resolve derp tcp endpoint {authority}: {err}"))?
            .next()
            .ok_or_else(|| format!("no derp tcp address resolved for {authority}"))?;
        let stream = TcpStream::connect_timeout(&addr, Duration::from_secs(1))
            .map_err(|err| format!("tcp derp connect to {authority} failed: {err}"))?;
        Ok(ActiveSocket::Tcp(stream))
    }

    fn connect_udp(meta: &DerpNodeMeta) -> Result<ActiveSocket, String> {
        let authority = format!("{}:{}", meta.host, meta.port);
        let bind_addr = if meta.host.contains(':') { "[::]:0" } else { "0.0.0.0:0" };
        let socket =
            UdpSocket::bind(bind_addr).map_err(|err| format!("bind udp socket failed: {err}"))?;
        socket
            .connect(authority.as_str())
            .map_err(|err| format!("udp derp connect to {authority} failed: {err}"))?;
        Ok(ActiveSocket::Udp(socket))
    }

    fn require_ready_state(state: &DerpClientState) -> Result<(&DerpNodeMeta, &RelayTicket), String> {
        let meta = state
            .meta
            .as_ref()
            .ok_or_else(|| "derp client has no connected node".to_string())?;
        let ticket = state
            .ticket
            .as_ref()
            .ok_or_else(|| "derp client has no attached ticket".to_string())?;
        if state.closed {
            return Err("derp client is closed".to_string());
        }
        Ok((meta, ticket))
    }
}

impl DerpClient for SocketDerpClient {
    fn connect_node(&self, meta: &DerpNodeMeta) -> Result<(), String> {
        let active_socket = match meta.transport {
            slan_app_core::DerpTransport::Tcp => Self::connect_tcp(meta)?,
            slan_app_core::DerpTransport::Udp => Self::connect_udp(meta)?,
            slan_app_core::DerpTransport::Quic => {
                return Err(format!(
                    "quic derp transport not implemented for {}",
                    meta.node_id
                ));
            }
        };
        let mut state = self
            .state
            .lock()
            .map_err(|_| "derp client state poisoned".to_string())?;
        state.meta = Some(meta.clone());
        state.active_socket = Some(active_socket);
        state.closed = false;
        state.last_recv_at_ms = Self::now_ms();
        Ok(())
    }

    fn attach_ticket(&self, ticket: &RelayTicket) -> Result<(), String> {
        let mut state = self
            .state
            .lock()
            .map_err(|_| "derp client state poisoned".to_string())?;
        let meta = state
            .meta
            .as_ref()
            .ok_or_else(|| "derp client has no connected node".to_string())?;
        if let Some(cluster_id) = &ticket.derp_cluster_id {
            if cluster_id != &meta.cluster_id {
                return Err(format!(
                    "relay ticket cluster {} does not match derp node cluster {}",
                    cluster_id, meta.cluster_id
                ));
            }
        }
        if !ticket.allowed_derp_node_ids.is_empty()
            && !ticket.allowed_derp_node_ids.iter().any(|id| id == &meta.node_id)
        {
            return Err(format!(
                "relay ticket does not allow derp node {}",
                meta.node_id
            ));
        }
        state.ticket = Some(ticket.clone());
        state.closed = false;
        Ok(())
    }

    fn send_transport_packet(&self, packet: &[u8]) -> Result<(), String> {
        if packet.is_empty() {
            return Err("cannot send empty packet to derp node".to_string());
        }
        let mut state = self
            .state
            .lock()
            .map_err(|_| "derp client state poisoned".to_string())?;
        let _ = Self::require_ready_state(&state)?;
        match state
            .active_socket
            .as_mut()
            .ok_or_else(|| "derp client has no active socket".to_string())?
        {
            ActiveSocket::Tcp(stream) => {
                use std::io::Write;
                stream
                    .write_all(packet)
                    .map_err(|err| format!("send packet to derp tcp socket failed: {err}"))?;
            }
            ActiveSocket::Udp(socket) => {
                socket
                    .send(packet)
                    .map_err(|err| format!("send packet to derp udp socket failed: {err}"))?;
            }
        }
        Ok(())
    }

    fn poll_transport_packet(&self) -> Result<Option<Vec<u8>>, String> {
        let mut state = self
            .state
            .lock()
            .map_err(|_| "derp client state poisoned".to_string())?;
        let _ = Self::require_ready_state(&state)?;
        let result = match state
            .active_socket
            .as_mut()
            .ok_or_else(|| "derp client has no active socket".to_string())?
        {
            ActiveSocket::Tcp(stream) => {
                use std::io::Read;
                stream
                    .set_nonblocking(true)
                    .map_err(|err| format!("set derp tcp socket nonblocking failed: {err}"))?;
                let mut buf = vec![0_u8; 2048];
                match stream.read(&mut buf) {
                    Ok(0) => Ok(None),
                    Ok(n) => {
                        buf.truncate(n);
                        Ok(Some(buf))
                    }
                    Err(err) if err.kind() == ErrorKind::WouldBlock => Ok(None),
                    Err(err) => Err(format!("read derp tcp socket failed: {err}")),
                }
            }
            ActiveSocket::Udp(socket) => {
                socket
                    .set_nonblocking(true)
                    .map_err(|err| format!("set derp udp socket nonblocking failed: {err}"))?;
                let mut buf = vec![0_u8; 2048];
                match socket.recv(&mut buf) {
                    Ok(n) => {
                        buf.truncate(n);
                        Ok(Some(buf))
                    }
                    Err(err) if err.kind() == ErrorKind::WouldBlock => Ok(None),
                    Err(err) => Err(format!("read derp udp socket failed: {err}")),
                }
            }
        };
        if matches!(result, Ok(Some(_))) {
            state.last_recv_at_ms = Self::now_ms();
        }
        match state
            .active_socket
            .as_mut()
            .ok_or_else(|| "derp client has no active socket".to_string())?
        {
            ActiveSocket::Tcp(stream) => {
                let _ = stream.set_nonblocking(false);
            }
            ActiveSocket::Udp(socket) => {
                socket
                    .set_nonblocking(false)
                    .map_err(|err| format!("reset derp udp socket nonblocking failed: {err}"))?;
            }
        }
        result
    }

    fn probe(&self) -> Result<ProbeSample, String> {
        let mut state = self
            .state
            .lock()
            .map_err(|_| "derp client state poisoned".to_string())?;
        let _ = Self::require_ready_state(&state)?;
        let sampled_at_ms = Self::now_ms();
        state.last_probe_at_ms = sampled_at_ms;
        Ok(ProbeSample {
            rtt_ms: 0,
            timed_out: false,
            packet_loss_ppm: 0,
            sampled_at_ms,
        })
    }

    fn snapshot(&self) -> DerpLinkSnapshot {
        let now_ms = Self::now_ms();
        let Ok(state) = self.state.lock() else {
            return DerpLinkSnapshot {
                meta: DerpNodeMeta {
                    cluster_id: String::new(),
                    region_id: String::new(),
                    country_code: None,
                    country_name: None,
                    city_code: None,
                    city_name: None,
                    node_id: String::new(),
                    host: String::new(),
                    port: 0,
                    transport: slan_app_core::DerpTransport::Udp,
                    priority: 0,
                    tags: vec![],
                },
                state: DerpLinkState::Failed,
                health: DerpHealth {
                    rtt_ms_ewma: 0,
                    loss_ppm: 0,
                    timeout_count: 0,
                    consecutive_failures: 0,
                    last_probe_at_ms: now_ms,
                    last_recv_at_ms: now_ms,
                    score: u32::MAX,
                },
                is_active: false,
            };
        };
        let meta = state.meta.clone().unwrap_or(DerpNodeMeta {
            cluster_id: String::new(),
            region_id: String::new(),
            country_code: None,
            country_name: None,
            city_code: None,
            city_name: None,
            node_id: String::new(),
            host: String::new(),
            port: 0,
            transport: slan_app_core::DerpTransport::Udp,
            priority: 0,
            tags: vec![],
        });
        let link_state = if state.closed {
            DerpLinkState::Closed
        } else if state.active_socket.is_none() {
            DerpLinkState::Connecting
        } else if state.ticket.is_none() {
            DerpLinkState::Connecting
        } else {
            DerpLinkState::Ready
        };
        DerpLinkSnapshot {
            health: DerpHealth {
                rtt_ms_ewma: 0,
                loss_ppm: 0,
                timeout_count: 0,
                consecutive_failures: 0,
                last_probe_at_ms: state.last_probe_at_ms,
                last_recv_at_ms: state.last_recv_at_ms,
                score: meta.priority,
            },
            meta,
            state: link_state,
            is_active: false,
        }
    }

    fn close(&self) -> Result<(), String> {
        let mut state = self
            .state
            .lock()
            .map_err(|_| "derp client state poisoned".to_string())?;
        state.active_socket = None;
        state.ticket = None;
        state.closed = true;
        Ok(())
    }
}

pub struct SocketRelayClient {
    connect_timeout: Duration,
    state: Mutex<RelayClientState>,
}

#[derive(Default)]
struct RelayClientState {
    relay_url: Option<String>,
    session_id: Option<String>,
    local_participant_id: Option<String>,
    peer_participant_id: Option<String>,
    active_socket: Option<ActiveSocket>,
}

impl SocketRelayClient {
    pub fn new(connect_timeout: Duration) -> Self {
        Self {
            connect_timeout,
            state: Mutex::new(RelayClientState::default()),
        }
    }
}

impl Default for SocketRelayClient {
    fn default() -> Self {
        Self::new(Duration::from_secs(1))
    }
}

impl SocketRelayClient {
    fn read_udp_response(
        socket: &UdpSocket,
        timeout: Option<Duration>,
    ) -> Result<Option<RelayDaemonResponse>, String> {
        let use_nonblocking = matches!(timeout, Some(duration) if duration.is_zero());
        if use_nonblocking {
            socket
                .set_nonblocking(true)
                .map_err(|err| format!("set relay udp socket nonblocking failed: {err}"))?;
        } else {
            socket
                .set_read_timeout(timeout)
                .map_err(|err| format!("set relay udp read timeout failed: {err}"))?;
        }
        let mut buf = vec![0_u8; 64 * 1024];
        let result = match socket.recv(&mut buf) {
            Ok(n) => {
                buf.truncate(n);
                let response = serde_json::from_slice::<RelayDaemonResponse>(&buf)
                    .map_err(|err| format!("decode relay daemon response failed: {err}"))?;
                Ok(Some(response))
            }
            Err(err)
                if matches!(
                    err.kind(),
                    ErrorKind::WouldBlock | ErrorKind::TimedOut | ErrorKind::ConnectionRefused
                ) =>
            {
                Ok(None)
            }
            Err(err) => Err(format!("read relay udp socket failed: {err}")),
        };
        if use_nonblocking {
            socket
                .set_nonblocking(false)
                .map_err(|err| format!("reset relay udp socket nonblocking failed: {err}"))?;
        } else {
            socket
                .set_read_timeout(None)
                .map_err(|err| format!("reset relay udp read timeout failed: {err}"))?;
        }
        result
    }

    fn send_udp_request(
        socket: &UdpSocket,
        request: &RelayDaemonRequest,
    ) -> Result<(), String> {
        let payload = serde_json::to_vec(request)
            .map_err(|err| format!("encode relay daemon request failed: {err}"))?;
        socket
            .send(&payload)
            .map_err(|err| format!("send packet to relay udp socket failed: {err}"))?;
        Ok(())
    }
}

impl RelayClient for SocketRelayClient {
    fn connect(&self, ticket: &RelayTicket) -> Result<ConnectionState, RelayClientError> {
        let (scheme, authority) = ticket
            .relay_url
            .split_once("://")
            .ok_or_else(|| RelayClientError::message(format!("invalid relay url: {}", ticket.relay_url)))?;
        match scheme {
            "tcp" => Err(RelayClientError::message(
                "tcp relay daemon protocol not implemented; use udp relay url",
            )),
            "udp" => {
                let bind_addr = if authority.starts_with('[') { "[::]:0" } else { "0.0.0.0:0" };
                let socket = UdpSocket::bind(bind_addr)
                    .map_err(|err| RelayClientError::message(format!("bind udp socket failed: {err}")))?;
                socket
                    .connect(authority)
                    .map_err(|err| RelayClientError::message(format!("udp relay connect to {authority} failed: {err}")))?;
                let attach_request = RelayDaemonRequest::Attach {
                    participant_id: ticket.src_node_id.clone(),
                    ticket: ticket.clone(),
                };
                Self::send_udp_request(&socket, &attach_request).map_err(RelayClientError::from)?;
                let response = Self::read_udp_response(&socket, Some(self.connect_timeout))?
                    .ok_or_else(|| RelayClientError::message("timed out waiting for relay attach response"))?;
                match response {
                    RelayDaemonResponse::Attached {
                        session_id,
                        peer_participant_id,
                    } => {
                        let mut state = self
                            .state
                            .lock()
                            .map_err(|_| RelayClientError::message("relay client state poisoned"))?;
                        state.relay_url = Some(ticket.relay_url.clone());
                        state.session_id = Some(session_id);
                        state.local_participant_id = Some(ticket.src_node_id.clone());
                        state.peer_participant_id = Some(peer_participant_id);
                        state.active_socket = Some(ActiveSocket::Udp(socket));
                        Ok(ConnectionState::Connected(ConnectionPath::Relay))
                    }
                    RelayDaemonResponse::Error { code, message } => {
                        Err(RelayDaemonError::from_wire(code.as_deref(), message)
                            .into_client_error("relay attach failed"))
                    }
                    other => Err(RelayClientError::message(format!(
                        "unexpected relay attach response: {other:?}"
                    ))),
                }
            }
            other => Err(RelayClientError::message(format!(
                "unsupported relay url scheme: {other}"
            ))),
        }
    }

    fn send_transport_packet(&self, packet: &[u8]) -> Result<(), RelayClientError> {
        if packet.is_empty() {
            return Err(RelayClientError::message("cannot send empty packet via relay"));
        }
        let mut state = self
            .state
            .lock()
            .map_err(|_| RelayClientError::message("relay client state poisoned"))?;
        let session_id = state.session_id.clone();
        let local_participant_id = state.local_participant_id.clone();
        let socket = state
            .active_socket
            .as_mut()
            .ok_or_else(|| RelayClientError::message("relay client has no active connection"))?;
        match socket {
            ActiveSocket::Tcp(stream) => {
                use std::io::Write;
                stream
                    .write_all(packet)
                    .map_err(|err| RelayClientError::message(format!("send packet to relay tcp socket failed: {err}")))?;
            }
            ActiveSocket::Udp(socket) => {
                let session_id = session_id
                    .clone()
                    .ok_or_else(|| RelayClientError::message("relay client has no attached session"))?;
                let from_participant_id = local_participant_id
                    .clone()
                    .ok_or_else(|| RelayClientError::message("relay client has no local participant id"))?;
                Self::send_udp_request(
                    socket,
                    &RelayDaemonRequest::Forward {
                        session_id: session_id.clone(),
                        from_participant_id,
                        payload_b64: STANDARD.encode(packet),
                    },
                )
                .map_err(RelayClientError::from)?;
                let response = Self::read_udp_response(socket, Some(self.connect_timeout))?
                    .ok_or_else(|| RelayClientError::message("timed out waiting for relay forward ack"))?;
                match response {
                    RelayDaemonResponse::Forwarded {
                        session_id: ack_session_id,
                        bytes_forwarded,
                        ..
                    } => {
                        if ack_session_id != session_id {
                            return Err(RelayClientError::message(format!(
                                "relay forward ack session mismatch: expected {session_id}, got {ack_session_id}"
                            )));
                        }
                        if bytes_forwarded != packet.len() {
                            return Err(RelayClientError::message(format!(
                                "relay forward ack bytes mismatch: expected {}, got {}",
                                packet.len(),
                                bytes_forwarded
                            )));
                        }
                    }
                    RelayDaemonResponse::Error { code, message } => {
                        return Err(RelayDaemonError::from_wire(code.as_deref(), message)
                            .into_client_error("relay forward failed"));
                    }
                    other => {
                        return Err(RelayClientError::message(format!(
                            "unexpected relay forward response: {other:?}"
                        )));
                    }
                }
            }
        }
        Ok(())
    }

    fn poll_transport_packet(&self) -> Result<Option<Vec<u8>>, RelayClientError> {
        let mut state = self
            .state
            .lock()
            .map_err(|_| RelayClientError::message("relay client state poisoned"))?;
        let socket = state
            .active_socket
            .as_mut()
            .ok_or_else(|| RelayClientError::message("relay client has no active connection"))?;
        match socket {
            ActiveSocket::Tcp(stream) => {
                use std::io::Read;
                stream
                    .set_nonblocking(true)
                    .map_err(|err| RelayClientError::message(format!("set relay tcp socket nonblocking failed: {err}")))?;
                let mut buf = vec![0_u8; 2048];
                let result = match stream.read(&mut buf) {
                    Ok(0) => Ok(None),
                    Ok(n) => {
                        buf.truncate(n);
                        Ok(Some(buf))
                    }
                    Err(err) if err.kind() == ErrorKind::WouldBlock => Ok(None),
                    Err(err) => Err(RelayClientError::message(format!("read relay tcp socket failed: {err}"))),
                };
                let _ = stream.set_nonblocking(false);
                result
            }
            ActiveSocket::Udp(socket) => {
                match Self::read_udp_response(socket, Some(Duration::from_millis(0)))
                    .map_err(RelayClientError::from)?
                {
                    Some(RelayDaemonResponse::Packet { payload_b64, .. }) => STANDARD
                        .decode(payload_b64.as_bytes())
                        .map(Some)
                        .map_err(|err| RelayClientError::message(format!("decode relay packet payload failed: {err}"))),
                    Some(RelayDaemonResponse::Error { code, message }) => Err(
                        RelayDaemonError::from_wire(code.as_deref(), message)
                            .into_client_error("relay receive failed"),
                    ),
                    Some(_) => Ok(None),
                    None => Ok(None),
                }
            }
        }
    }
}

pub struct InMemoryPathManager<D, R, P>
where
    D: DerpPool,
    R: RelayClient,
    P: P2PConnector,
{
    derp_pool: D,
    relay_client: R,
    p2p_connector: P,
    state: Mutex<PathState>,
}

struct PathState {
    active_path: slan_app_core::ActivePath,
    last_peer_node_id: Option<String>,
    last_failure_reason: Option<String>,
}

impl Default for PathState {
    fn default() -> Self {
        Self {
            active_path: slan_app_core::ActivePath::None,
            last_peer_node_id: None,
            last_failure_reason: None,
        }
    }
}

impl<D, R, P> InMemoryPathManager<D, R, P>
where
    D: DerpPool,
    R: RelayClient,
    P: P2PConnector,
{
    pub fn new(derp_pool: D, relay_client: R, p2p_connector: P) -> Self {
        Self {
            derp_pool,
            relay_client,
            p2p_connector,
            state: Mutex::new(PathState::default()),
        }
    }
}

impl<D, R, P> PathManager for InMemoryPathManager<D, R, P>
where
    D: DerpPool,
    R: RelayClient,
    P: P2PConnector,
{
    fn on_p2p_failed(&self, peer_node_id: &str, reason: &str) -> Result<(), PathManagerError> {
        let fallback_path = match self.derp_pool.active_link() {
            Some(link) => slan_app_core::ActivePath::Derp {
                cluster_id: link.meta.cluster_id,
                node_id: link.meta.node_id,
            },
            None => slan_app_core::ActivePath::Relay {
                peer_node_id: peer_node_id.to_string(),
            },
        };
        let mut state = self
            .state
            .lock()
            .map_err(|_| PathManagerError::message("path manager state poisoned"))?;
        state.active_path = fallback_path;
        state.last_peer_node_id = Some(peer_node_id.to_string());
        state.last_failure_reason = Some(reason.to_string());
        Ok(())
    }

    fn on_p2p_recovered(&self, peer_node_id: &str) -> Result<(), PathManagerError> {
        let mut state = self
            .state
            .lock()
            .map_err(|_| PathManagerError::message("path manager state poisoned"))?;
        state.active_path = slan_app_core::ActivePath::P2P {
            peer_node_id: peer_node_id.to_string(),
        };
        state.last_peer_node_id = Some(peer_node_id.to_string());
        state.last_failure_reason = None;
        Ok(())
    }

    fn current_path(&self) -> slan_app_core::ActivePath {
        self.state
            .lock()
            .map(|state| state.active_path.clone())
            .unwrap_or(slan_app_core::ActivePath::None)
    }

    fn send_transport_packet(&self, packet: &[u8]) -> Result<(), PathManagerError> {
        if packet.is_empty() {
            return Err(PathManagerError::message(
                "cannot send empty packet through path manager",
            ));
        }
        let path = self
            .state
            .lock()
            .map_err(|_| PathManagerError::message("path manager state poisoned"))?
            .active_path
            .clone();
        match path {
            slan_app_core::ActivePath::Derp { .. } => {
                self.derp_pool
                    .send_transport_packet_via_active(packet)
                    .map_err(PathManagerError::from)
            }
            slan_app_core::ActivePath::P2P { .. } => {
                self.p2p_connector
                    .send_transport_packet(packet)
                    .map_err(PathManagerError::from)
            }
            slan_app_core::ActivePath::Relay { .. } => {
                self.relay_client.send_transport_packet(packet).map_err(|err| {
                    PathManagerError::new(err.code.clone(), err.to_string())
                })
            }
            slan_app_core::ActivePath::None => {
                Err(PathManagerError::message("no active path selected"))
            }
        }
    }

    fn poll_transport_packet(&self) -> Result<Option<Vec<u8>>, PathManagerError> {
        let path = self
            .state
            .lock()
            .map_err(|_| PathManagerError::message("path manager state poisoned"))?
            .active_path
            .clone();
        match path {
            slan_app_core::ActivePath::Derp { .. } => {
                self.derp_pool
                    .poll_transport_packet_via_active()
                    .map_err(PathManagerError::from)
            }
            slan_app_core::ActivePath::P2P { .. } => {
                self.p2p_connector
                    .poll_transport_packet()
                    .map_err(PathManagerError::from)
            }
            slan_app_core::ActivePath::Relay { .. } => {
                self.relay_client.poll_transport_packet().map_err(|err| {
                    PathManagerError::new(err.code.clone(), err.to_string())
                })
            }
            slan_app_core::ActivePath::None => {
                Err(PathManagerError::message("no active path selected"))
            }
        }
    }
}

pub struct InMemoryDerpPool {
    client_factory: DerpClientFactory,
    state: Mutex<PoolState>,
}

type DerpClientFactory = Arc<dyn Fn() -> Box<dyn DerpClient> + Send + Sync>;

#[derive(Default)]
struct PoolState {
    cluster_id: String,
    session_id: String,
    ticket: Option<RelayTicket>,
    active_node_id: Option<String>,
    links: Vec<ManagedLink>,
    switch_epoch: u64,
}

struct ManagedLink {
    meta: DerpNodeMeta,
    client: Box<dyn DerpClient>,
    last_probe: Option<ProbeSample>,
    timeout_count: u32,
    consecutive_failures: u32,
    last_error: Option<String>,
}

impl InMemoryDerpPool {
    fn new_with_factory(factory: DerpClientFactory) -> Self {
        Self {
            client_factory: factory,
            state: Mutex::new(PoolState::default()),
        }
    }

    fn now_ms() -> u64 {
        SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap_or_default()
            .as_millis() as u64
    }

    fn build_snapshot(link: &ManagedLink, is_active: bool) -> DerpLinkSnapshot {
        let mut snapshot = link.client.snapshot();
        snapshot.meta = link.meta.clone();
        snapshot.is_active = is_active;
        snapshot.health.timeout_count = link.timeout_count;
        snapshot.health.consecutive_failures = link.consecutive_failures;
        snapshot.health.score = link.meta.priority
            + link
                .last_probe
                .as_ref()
                .map(|probe| probe.rtt_ms.saturating_add(probe.packet_loss_ppm / 1_000))
                .unwrap_or(0)
            + link.timeout_count.saturating_mul(1_000)
            + link.consecutive_failures.saturating_mul(100);
        if let Some(probe) = &link.last_probe {
            snapshot.health.rtt_ms_ewma = probe.rtt_ms;
            snapshot.health.loss_ppm = probe.packet_loss_ppm;
            snapshot.health.last_probe_at_ms = probe.sampled_at_ms;
        }
        if link.last_error.is_some() && matches!(snapshot.state, DerpLinkState::Connecting) {
            snapshot.state = DerpLinkState::Failed;
        }
        if matches!(snapshot.state, DerpLinkState::Ready)
            && (link.timeout_count > 0
                || link
                    .last_probe
                    .as_ref()
                    .map(|probe| probe.timed_out)
                    .unwrap_or(false))
        {
            snapshot.state = DerpLinkState::Suspect;
        }
        snapshot
    }

    fn collect_snapshots(state: &PoolState) -> Vec<DerpLinkSnapshot> {
        state
            .links
            .iter()
            .map(|link| {
                Self::build_snapshot(
                    link,
                    state.active_node_id.as_deref() == Some(link.meta.node_id.as_str()),
                )
            })
            .collect()
    }

    fn choose_active_index(links: &[DerpLinkSnapshot]) -> Option<usize> {
        links.iter()
            .enumerate()
            .filter(|(_, link)| {
                matches!(link.state, DerpLinkState::Ready | DerpLinkState::Suspect)
            })
            .min_by_key(|(_, link)| (link.health.score, link.meta.priority))
            .map(|(idx, _)| idx)
    }

    fn ensure_link_ready(link: &mut ManagedLink, ticket: &RelayTicket) -> Result<(), String> {
        let snapshot = link.client.snapshot();
        if matches!(snapshot.state, DerpLinkState::Ready | DerpLinkState::Suspect) {
            link.last_error = None;
            return Ok(());
        }
        link.client.connect_node(&link.meta)?;
        link.client.attach_ticket(ticket)?;
        link.last_error = None;
        Ok(())
    }
}

impl Default for InMemoryDerpPool {
    fn default() -> Self {
        Self::new_with_factory(Arc::new(|| Box::new(SocketDerpClient::default())))
    }
}

impl DerpPool for InMemoryDerpPool {
    fn install_cluster(
        &self,
        cluster_id: &str,
        nodes: Vec<DerpNodeMeta>,
        ticket: RelayTicket,
    ) -> Result<(), String> {
        let now_ms = Self::now_ms();
        let mut state = self
            .state
            .lock()
            .map_err(|_| "derp pool state poisoned".to_string())?;
        for link in &mut state.links {
            let _ = link.client.close();
        }
        state.cluster_id = cluster_id.to_string();
        state.session_id = ticket.session_id.clone();
        state.ticket = Some(ticket);
        state.active_node_id = None;
        state.links = nodes
            .into_iter()
            .map(|meta| ManagedLink {
                meta,
                client: (self.client_factory)(),
                last_probe: Some(ProbeSample {
                    rtt_ms: 0,
                    timed_out: false,
                    packet_loss_ppm: 0,
                    sampled_at_ms: now_ms,
                }),
                timeout_count: 0,
                consecutive_failures: 0,
                last_error: None,
            })
            .collect();
        state.switch_epoch = 0;
        Ok(())
    }

    fn warm_up(&self, fanout: usize) -> Result<(), String> {
        let mut state = self
            .state
            .lock()
            .map_err(|_| "derp pool state poisoned".to_string())?;
        if state.links.is_empty() {
            return Err("derp pool has no installed cluster".to_string());
        }
        let ticket = state
            .ticket
            .clone()
            .ok_or_else(|| "derp pool has no installed ticket".to_string())?;
        let warm_count = fanout.max(1).min(state.links.len());
        for (idx, link) in state.links.iter_mut().enumerate() {
            if idx < warm_count {
                if let Err(err) = Self::ensure_link_ready(link, &ticket) {
                    link.last_error = Some(err.clone());
                    return Err(err);
                }
            } else {
                let _ = link.client.close();
                link.last_error = None;
            }
        }
        let snapshots = Self::collect_snapshots(&state);
        state.active_node_id = Self::choose_active_index(&snapshots)
            .map(|idx| state.links[idx].meta.node_id.clone());
        Ok(())
    }

    fn active_link(&self) -> Option<DerpLinkSnapshot> {
        let state = self.state.lock().ok()?;
        Self::collect_snapshots(&state)
            .into_iter()
            .find(|link| link.is_active)
    }

    fn send_transport_packet_via_active(&self, packet: &[u8]) -> Result<(), String> {
        if packet.is_empty() {
            return Err("cannot send empty packet via derp active link".to_string());
        }
        let mut state = self
            .state
            .lock()
            .map_err(|_| "derp pool state poisoned".to_string())?;
        let active_node_id = state
            .active_node_id
            .clone()
            .ok_or_else(|| "derp pool has no active link".to_string())?;
        let link = state
            .links
            .iter_mut()
            .find(|link| link.meta.node_id == active_node_id)
            .ok_or_else(|| "derp pool active node missing".to_string())?;
        link.client.send_transport_packet(packet)
    }

    fn poll_transport_packet_via_active(&self) -> Result<Option<Vec<u8>>, String> {
        let mut state = self
            .state
            .lock()
            .map_err(|_| "derp pool state poisoned".to_string())?;
        let active_node_id = state
            .active_node_id
            .clone()
            .ok_or_else(|| "derp pool has no active link".to_string())?;
        let link = state
            .links
            .iter_mut()
            .find(|link| link.meta.node_id == active_node_id)
            .ok_or_else(|| "derp pool active node missing".to_string())?;
        link.client.poll_transport_packet()
    }

    fn tick_health_check(&self) -> Result<(), String> {
        let mut state = self
            .state
            .lock()
            .map_err(|_| "derp pool state poisoned".to_string())?;
        for link in &mut state.links {
            let snapshot = link.client.snapshot();
            if !matches!(snapshot.state, DerpLinkState::Ready | DerpLinkState::Suspect) {
                continue;
            }
            match link.client.probe() {
                Ok(probe) => {
                    if probe.timed_out {
                        link.timeout_count = link.timeout_count.saturating_add(1);
                        link.consecutive_failures = link.consecutive_failures.saturating_add(1);
                    } else {
                        link.consecutive_failures = 0;
                    }
                    link.last_probe = Some(probe);
                    link.last_error = None;
                }
                Err(err) => {
                    link.timeout_count = link.timeout_count.saturating_add(1);
                    link.consecutive_failures = link.consecutive_failures.saturating_add(1);
                    link.last_error = Some(err);
                }
            }
        }
        Ok(())
    }

    fn maybe_switch(&self) -> Result<Option<DerpSwitchEvent>, String> {
        let mut state = self
            .state
            .lock()
            .map_err(|_| "derp pool state poisoned".to_string())?;
        let snapshots = Self::collect_snapshots(&state);
        let Some(target_idx) = Self::choose_active_index(&snapshots) else {
            return Ok(None);
        };
        let target_node_id = state.links[target_idx].meta.node_id.clone();
        if state.active_node_id.as_deref() == Some(target_node_id.as_str()) {
            return Ok(None);
        }
        let event = DerpSwitchEvent {
            cluster_id: state.cluster_id.clone(),
            from_node_id: state.active_node_id.clone(),
            to_node_id: target_node_id.clone(),
            reason: SwitchReason::BootstrapPreferred,
            happened_at_ms: Self::now_ms(),
        };
        state.active_node_id = Some(target_node_id);
        state.switch_epoch += 1;
        Ok(Some(event))
    }

    fn state(&self) -> DerpPoolState {
        if let Ok(state) = self.state.lock() {
            DerpPoolState {
                cluster_id: state.cluster_id.clone(),
                session_id: state.session_id.clone(),
                active_node_id: state.active_node_id.clone(),
                links: Self::collect_snapshots(&state),
                switch_epoch: state.switch_epoch,
            }
        } else {
            DerpPoolState {
                cluster_id: String::new(),
                session_id: String::new(),
                active_node_id: None,
                links: vec![],
                switch_epoch: 0,
            }
        }
    }

    fn force_switch(
        &self,
        target_node_id: &str,
        reason: SwitchReason,
    ) -> Result<DerpSwitchEvent, String> {
        let mut state = self
            .state
            .lock()
            .map_err(|_| "derp pool state poisoned".to_string())?;
        let ticket = state
            .ticket
            .clone()
            .ok_or_else(|| "derp pool has no installed ticket".to_string())?;
        let target_idx = state
            .links
            .iter()
            .position(|link| link.meta.node_id == target_node_id)
            .ok_or_else(|| format!("derp node not found: {target_node_id}"))?;
        Self::ensure_link_ready(&mut state.links[target_idx], &ticket)?;
        let event = DerpSwitchEvent {
            cluster_id: state.cluster_id.clone(),
            from_node_id: state.active_node_id.clone(),
            to_node_id: target_node_id.to_string(),
            reason,
            happened_at_ms: Self::now_ms(),
        };
        state.active_node_id = Some(target_node_id.to_string());
        state.switch_epoch += 1;
        Ok(event)
    }

    fn close(&self) -> Result<(), String> {
        let mut state = self
            .state
            .lock()
            .map_err(|_| "derp pool state poisoned".to_string())?;
        for link in &mut state.links {
            link.client.close()?;
            link.last_error = None;
        }
        state.active_node_id = None;
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use p2p::PeerCandidate;
    use slan_app_core::DerpTransport;
    use std::sync::atomic::{AtomicUsize, Ordering};

    #[derive(Default)]
    struct FakeDerpClient {
        state: Mutex<FakeDerpClientState>,
    }

    #[derive(Default)]
    struct FakeDerpClientState {
        snapshot: Option<DerpLinkSnapshot>,
        connect_calls: usize,
        attach_calls: usize,
        send_calls: usize,
        close_calls: usize,
        next_probe: Option<ProbeSample>,
    }

    impl FakeDerpClient {
        fn new() -> Self {
            Self::default()
        }
    }

    #[derive(Clone, Default)]
    struct FakeCounters {
        connect_calls: Arc<AtomicUsize>,
        attach_calls: Arc<AtomicUsize>,
        send_calls: Arc<AtomicUsize>,
        close_calls: Arc<AtomicUsize>,
    }

    impl DerpClient for FakeDerpClient {
        fn connect_node(&self, meta: &DerpNodeMeta) -> Result<(), String> {
            let mut state = self.state.lock().unwrap();
            state.connect_calls += 1;
            state.snapshot = Some(DerpLinkSnapshot {
                meta: meta.clone(),
                state: DerpLinkState::Connecting,
                health: DerpHealth {
                    rtt_ms_ewma: 0,
                    loss_ppm: 0,
                    timeout_count: 0,
                    consecutive_failures: 0,
                    last_probe_at_ms: 0,
                    last_recv_at_ms: 0,
                    score: meta.priority,
                },
                is_active: false,
            });
            Ok(())
        }

        fn attach_ticket(&self, _ticket: &RelayTicket) -> Result<(), String> {
            let mut state = self.state.lock().unwrap();
            state.attach_calls += 1;
            if let Some(snapshot) = &mut state.snapshot {
                snapshot.state = DerpLinkState::Ready;
            }
            Ok(())
        }

        fn send_transport_packet(&self, _packet: &[u8]) -> Result<(), String> {
            let mut state = self.state.lock().unwrap();
            state.send_calls += 1;
            Ok(())
        }

        fn poll_transport_packet(&self) -> Result<Option<Vec<u8>>, String> {
            Ok(None)
        }

        fn probe(&self) -> Result<ProbeSample, String> {
            let mut state = self.state.lock().unwrap();
            Ok(state.next_probe.take().unwrap_or(ProbeSample {
                rtt_ms: 0,
                timed_out: false,
                packet_loss_ppm: 0,
                sampled_at_ms: 1,
            }))
        }

        fn snapshot(&self) -> DerpLinkSnapshot {
            self.state.lock().unwrap().snapshot.clone().unwrap_or(DerpLinkSnapshot {
                meta: sample_derp_node(),
                state: DerpLinkState::Connecting,
                health: DerpHealth {
                    rtt_ms_ewma: 0,
                    loss_ppm: 0,
                    timeout_count: 0,
                    consecutive_failures: 0,
                    last_probe_at_ms: 0,
                    last_recv_at_ms: 0,
                    score: 0,
                },
                is_active: false,
            })
        }

        fn close(&self) -> Result<(), String> {
            let mut state = self.state.lock().unwrap();
            state.close_calls += 1;
            if let Some(snapshot) = &mut state.snapshot {
                snapshot.state = DerpLinkState::Closed;
            }
            Ok(())
        }
    }

    fn sample_derp_node() -> DerpNodeMeta {
        DerpNodeMeta {
            cluster_id: "cluster-1".into(),
            region_id: "region-1".into(),
            country_code: None,
            country_name: None,
            city_code: None,
            city_name: None,
            node_id: "node-a".into(),
            host: "127.0.0.1".into(),
            port: 9000,
            transport: DerpTransport::Udp,
            priority: 10,
            tags: vec![],
        }
    }

    fn sample_ticket() -> RelayTicket {
        RelayTicket {
            ticket_id: "ticket-1".into(),
            network_id: "net-1".into(),
            session_id: "session-1".into(),
            src_node_id: "src".into(),
            dst_node_id: "dst".into(),
            derp_cluster_id: Some("cluster-1".into()),
            country_code: None,
            city_code: None,
            allowed_derp_node_ids: vec!["node-a".into(), "node-b".into()],
            relay_url: "udp://127.0.0.1:9000".into(),
            expires_at: "2099-01-01T00:00:00Z".into(),
            session_key: None,
            signature: "sig".into(),
        }
    }

    fn fake_pool() -> InMemoryDerpPool {
        InMemoryDerpPool::new_with_factory(Arc::new(|| Box::new(FakeDerpClient::new())))
    }

    fn fake_pool_with_counters(counters: Vec<FakeCounters>) -> InMemoryDerpPool {
        let counters = Arc::new(counters);
        let next = Arc::new(AtomicUsize::new(0));
        InMemoryDerpPool::new_with_factory({
            let counters = counters.clone();
            let next = next.clone();
            Arc::new(move || {
                let idx = next.fetch_add(1, Ordering::SeqCst);
                let client = FakeDerpClient::new();
                let observed = counters[idx].clone();
                {
                    let mut state = client.state.lock().unwrap();
                    state.connect_calls = observed.connect_calls.load(Ordering::SeqCst);
                    state.attach_calls = observed.attach_calls.load(Ordering::SeqCst);
                    state.send_calls = observed.send_calls.load(Ordering::SeqCst);
                    state.close_calls = observed.close_calls.load(Ordering::SeqCst);
                }
                Box::new(ObservedFakeDerpClient { inner: client, counters: observed })
            })
        })
    }

    struct ObservedFakeDerpClient {
        inner: FakeDerpClient,
        counters: FakeCounters,
    }

    impl DerpClient for ObservedFakeDerpClient {
        fn connect_node(&self, meta: &DerpNodeMeta) -> Result<(), String> {
            self.counters.connect_calls.fetch_add(1, Ordering::SeqCst);
            self.inner.connect_node(meta)
        }

        fn attach_ticket(&self, ticket: &RelayTicket) -> Result<(), String> {
            self.counters.attach_calls.fetch_add(1, Ordering::SeqCst);
            self.inner.attach_ticket(ticket)
        }

        fn send_transport_packet(&self, packet: &[u8]) -> Result<(), String> {
            self.counters.send_calls.fetch_add(1, Ordering::SeqCst);
            self.inner.send_transport_packet(packet)
        }

        fn poll_transport_packet(&self) -> Result<Option<Vec<u8>>, String> {
            self.inner.poll_transport_packet()
        }

        fn probe(&self) -> Result<ProbeSample, String> {
            self.inner.probe()
        }

        fn snapshot(&self) -> DerpLinkSnapshot {
            self.inner.snapshot()
        }

        fn close(&self) -> Result<(), String> {
            self.counters.close_calls.fetch_add(1, Ordering::SeqCst);
            self.inner.close()
        }
    }

    #[derive(Default)]
    struct FakeDerpPool {
        active_link: Mutex<Option<DerpLinkSnapshot>>,
        send_calls: Arc<AtomicUsize>,
    }

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
            self.active_link.lock().unwrap().clone()
        }

        fn send_transport_packet_via_active(&self, _packet: &[u8]) -> Result<(), String> {
            self.send_calls.fetch_add(1, Ordering::SeqCst);
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
                cluster_id: "cluster-1".into(),
                session_id: "session-1".into(),
                active_node_id: self
                    .active_link
                    .lock()
                    .unwrap()
                    .as_ref()
                    .map(|link| link.meta.node_id.clone()),
                links: self.active_link.lock().unwrap().clone().into_iter().collect(),
                switch_epoch: 0,
            }
        }

        fn force_switch(
            &self,
            _target_node_id: &str,
            _reason: SwitchReason,
        ) -> Result<DerpSwitchEvent, String> {
            Err("not needed in fake pool".to_string())
        }

        fn close(&self) -> Result<(), String> {
            Ok(())
        }
    }

    #[derive(Default)]
    struct FakeRelayClient {
        send_calls: Arc<AtomicUsize>,
    }

    impl RelayClient for FakeRelayClient {
        fn connect(&self, _ticket: &RelayTicket) -> Result<ConnectionState, RelayClientError> {
            Ok(ConnectionState::Connected(ConnectionPath::Relay))
        }

        fn send_transport_packet(&self, _packet: &[u8]) -> Result<(), RelayClientError> {
            self.send_calls.fetch_add(1, Ordering::SeqCst);
            Ok(())
        }

        fn poll_transport_packet(&self) -> Result<Option<Vec<u8>>, RelayClientError> {
            Ok(None)
        }
    }

    #[derive(Default)]
    struct FakeP2PConnector {
        send_calls: Arc<AtomicUsize>,
    }

    impl P2PConnector for FakeP2PConnector {
        fn connect(&self, _peer: &PeerCandidate) -> Result<ConnectionState, String> {
            Ok(ConnectionState::Connected(ConnectionPath::P2P))
        }

        fn send_transport_packet(&self, _packet: &[u8]) -> Result<(), String> {
            self.send_calls.fetch_add(1, Ordering::SeqCst);
            Ok(())
        }

        fn poll_transport_packet(&self) -> Result<Option<Vec<u8>>, String> {
            Ok(None)
        }
    }

    #[test]
    fn unsupported_relay_url_scheme_fails() {
        let client = SocketRelayClient::default();
        let error = client
            .connect(&RelayTicket {
                ticket_id: "ticket-1".into(),
                network_id: "net-1".into(),
                session_id: "session-1".into(),
                src_node_id: "src".into(),
                dst_node_id: "dst".into(),
                derp_cluster_id: None,
                country_code: None,
                city_code: None,
                allowed_derp_node_ids: vec![],
                relay_url: "quic://127.0.0.1:9000".into(),
                expires_at: "2099-01-01T00:00:00Z".into(),
                session_key: None,
                signature: "sig".into(),
            })
            .unwrap_err();

        assert!(error.to_string().contains("unsupported relay url scheme"));
    }

    #[test]
    fn relay_daemon_attach_request_serializes_ticket_fields() {
        let request = RelayDaemonRequest::Attach {
            participant_id: "src".into(),
            ticket: sample_ticket(),
        };

        let json = serde_json::to_value(&request).unwrap();

        assert_eq!(json["kind"], "attach");
        assert_eq!(json["participant_id"], "src");
        assert_eq!(json["ticket"]["sessionId"], "session-1");
        assert_eq!(json["ticket"]["srcNodeId"], "src");
        assert_eq!(json["ticket"]["dstNodeId"], "dst");
    }

    #[test]
    fn relay_daemon_packet_response_decodes_payload() {
        let response = RelayDaemonResponse::Packet {
            session_id: "session-1".into(),
            from_participant_id: "dst".into(),
            payload_b64: STANDARD.encode(b"world"),
        };

        let payload = match response {
            RelayDaemonResponse::Packet { payload_b64, .. } => {
                STANDARD.decode(payload_b64.as_bytes()).unwrap()
            }
            other => panic!("unexpected response: {other:?}"),
        };

        assert_eq!(payload, b"world");
    }

    #[test]
    fn relay_daemon_error_formats_code_when_present() {
        let formatted = RelayDaemonError::from_wire(
            Some("ticket_expired"),
            "relay ticket expired".into(),
        )
        .format_with_prefix("relay attach failed");

        assert_eq!(
            formatted,
            "relay attach failed [ticket_expired]: relay ticket expired"
        );
    }

    #[test]
    fn relay_daemon_error_code_parses_known_values() {
        let error = RelayDaemonError::from_wire(
            Some("participant_address_mismatch"),
            "participant is bound to a different udp address".into(),
        );

        assert_eq!(
            error.code,
            Some(RelayDaemonErrorCode::ParticipantAddressMismatch)
        );
        assert_eq!(
            error.format_with_prefix("relay forward failed"),
            "relay forward failed [participant_address_mismatch]: participant is bound to a different udp address"
        );
    }

    #[test]
    fn derp_client_rejects_attach_before_connect() {
        let client = SocketDerpClient::default();
        let error = client.attach_ticket(&sample_ticket()).unwrap_err();

        assert!(error.contains("no connected node"));
    }

    #[test]
    fn derp_client_rejects_cluster_mismatch() {
        let client = SocketDerpClient::default();
        let mut state = client.state.lock().unwrap();
        state.meta = Some(sample_derp_node());
        state.closed = false;
        drop(state);

        let mut ticket = sample_ticket();
        ticket.derp_cluster_id = Some("cluster-2".into());
        let error = client.attach_ticket(&ticket).unwrap_err();

        assert!(error.contains("does not match derp node cluster"));
    }

    #[test]
    fn derp_client_close_marks_snapshot_closed() {
        let client = SocketDerpClient::default();
        let mut state = client.state.lock().unwrap();
        state.meta = Some(sample_derp_node());
        state.ticket = Some(sample_ticket());
        state.closed = false;
        drop(state);

        client.close().unwrap();
        let snapshot = client.snapshot();

        assert!(matches!(snapshot.state, DerpLinkState::Closed));
    }

    #[test]
    fn derp_pool_marks_active_link_after_warm_up() {
        let pool = fake_pool();
        pool.install_cluster(
            "cluster-1",
            vec![
                sample_derp_node(),
                DerpNodeMeta {
                    cluster_id: "cluster-1".into(),
                    region_id: "region-1".into(),
                    country_code: None,
                    country_name: None,
                    city_code: None,
                    city_name: None,
                    node_id: "node-b".into(),
                    host: "127.0.0.1".into(),
                    port: 9001,
                    transport: DerpTransport::Udp,
                    priority: 20,
                    tags: vec![],
                },
            ],
            sample_ticket(),
        )
        .unwrap();

        pool.warm_up(2).unwrap();

        let active = pool.active_link().unwrap();
        assert_eq!(active.meta.node_id, "node-a");
        assert!(active.is_active);
    }

    #[test]
    fn derp_pool_send_transport_packet_via_active_uses_selected_client() {
        let counters = FakeCounters::default();
        let pool = fake_pool_with_counters(vec![counters.clone()]);
        pool.install_cluster("cluster-1", vec![sample_derp_node()], sample_ticket())
            .unwrap();
        pool.warm_up(1).unwrap();

        pool.send_transport_packet_via_active(b"hello").unwrap();

        assert_eq!(counters.connect_calls.load(Ordering::SeqCst), 1);
        assert_eq!(counters.attach_calls.load(Ordering::SeqCst), 1);
        assert_eq!(counters.send_calls.load(Ordering::SeqCst), 1);
    }

    #[test]
    fn derp_pool_switches_to_healthier_link() {
        let counter = Arc::new(AtomicUsize::new(0));
        let pool = InMemoryDerpPool::new_with_factory({
            let counter = counter.clone();
            Arc::new(move || {
                let idx = counter.fetch_add(1, Ordering::SeqCst);
                let client = FakeDerpClient::new();
                client.state.lock().unwrap().next_probe = Some(ProbeSample {
                    rtt_ms: if idx == 0 { 100 } else { 5 },
                    timed_out: false,
                    packet_loss_ppm: 0,
                    sampled_at_ms: 42,
                });
                Box::new(client)
            })
        });
        pool.install_cluster(
            "cluster-1",
            vec![
                sample_derp_node(),
                DerpNodeMeta {
                    node_id: "node-b".into(),
                    priority: 20,
                    port: 9001,
                    ..sample_derp_node()
                },
            ],
            sample_ticket(),
        )
        .unwrap();
        pool.warm_up(2).unwrap();
        assert_eq!(pool.active_link().unwrap().meta.node_id, "node-a");

        pool.tick_health_check().unwrap();
        let event = pool.maybe_switch().unwrap().unwrap();

        assert_eq!(event.to_node_id, "node-b");
        assert_eq!(pool.active_link().unwrap().meta.node_id, "node-b");
    }

    #[test]
    fn path_manager_prefers_derp_after_p2p_failure() {
        let pool = FakeDerpPool {
            active_link: Mutex::new(Some(DerpLinkSnapshot {
                meta: sample_derp_node(),
                state: DerpLinkState::Ready,
                health: DerpHealth {
                    rtt_ms_ewma: 0,
                    loss_ppm: 0,
                    timeout_count: 0,
                    consecutive_failures: 0,
                    last_probe_at_ms: 0,
                    last_recv_at_ms: 0,
                    score: 0,
                },
                is_active: true,
            })),
            send_calls: Arc::new(AtomicUsize::new(0)),
        };
        let manager =
            InMemoryPathManager::new(pool, FakeRelayClient::default(), FakeP2PConnector::default());

        manager.on_p2p_failed("peer-1", "timeout").unwrap();

        assert!(matches!(
            manager.current_path(),
            slan_app_core::ActivePath::Derp { ref node_id, .. } if node_id == "node-a"
        ));
    }

    #[test]
    fn path_manager_falls_back_to_relay_without_derp_link() {
        let relay = FakeRelayClient::default();
        let manager =
            InMemoryPathManager::new(FakeDerpPool::default(), relay, FakeP2PConnector::default());

        manager.on_p2p_failed("peer-1", "timeout").unwrap();

        assert!(matches!(
            manager.current_path(),
            slan_app_core::ActivePath::Relay { ref peer_node_id } if peer_node_id == "peer-1"
        ));
    }

    #[test]
    fn path_manager_recovers_to_p2p_and_sends_via_derp() {
        let send_calls = Arc::new(AtomicUsize::new(0));
        let pool = FakeDerpPool {
            active_link: Mutex::new(Some(DerpLinkSnapshot {
                meta: sample_derp_node(),
                state: DerpLinkState::Ready,
                health: DerpHealth {
                    rtt_ms_ewma: 0,
                    loss_ppm: 0,
                    timeout_count: 0,
                    consecutive_failures: 0,
                    last_probe_at_ms: 0,
                    last_recv_at_ms: 0,
                    score: 0,
                },
                is_active: true,
            })),
            send_calls: send_calls.clone(),
        };
        let manager =
            InMemoryPathManager::new(pool, FakeRelayClient::default(), FakeP2PConnector::default());

        manager.on_p2p_failed("peer-1", "timeout").unwrap();
        manager.send_transport_packet(b"hello").unwrap();
        manager.on_p2p_recovered("peer-1").unwrap();

        assert_eq!(send_calls.load(Ordering::SeqCst), 1);
        assert!(matches!(
            manager.current_path(),
            slan_app_core::ActivePath::P2P { ref peer_node_id } if peer_node_id == "peer-1"
        ));
    }

    #[test]
    fn path_manager_sends_via_relay_after_relay_fallback() {
        let relay = FakeRelayClient::default();
        let send_calls = relay.send_calls.clone();
        let manager =
            InMemoryPathManager::new(FakeDerpPool::default(), relay, FakeP2PConnector::default());

        manager.on_p2p_failed("peer-1", "timeout").unwrap();
        manager.send_transport_packet(b"hello").unwrap();

        assert_eq!(send_calls.load(Ordering::SeqCst), 1);
        assert!(matches!(
            manager.current_path(),
            slan_app_core::ActivePath::Relay { ref peer_node_id } if peer_node_id == "peer-1"
        ));
    }

    #[test]
    fn path_manager_sends_via_p2p_after_recovery() {
        let p2p = FakeP2PConnector::default();
        let send_calls = p2p.send_calls.clone();
        let manager =
            InMemoryPathManager::new(FakeDerpPool::default(), FakeRelayClient::default(), p2p);

        manager.on_p2p_recovered("peer-1").unwrap();
        manager.send_transport_packet(b"hello").unwrap();

        assert_eq!(send_calls.load(Ordering::SeqCst), 1);
        assert!(matches!(
            manager.current_path(),
            slan_app_core::ActivePath::P2P { ref peer_node_id } if peer_node_id == "peer-1"
        ));
    }
}
