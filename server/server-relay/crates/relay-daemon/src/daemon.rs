use std::{
    collections::HashMap,
    io::{Read, Write},
    net::{SocketAddr, TcpListener, TcpStream, UdpSocket},
    sync::{
        atomic::{AtomicU64, AtomicUsize, Ordering},
        Arc, Mutex,
    },
    thread,
    time::Duration,
};

use crate::config::DaemonConfig;
use crate::mqtt::{current_timestamp_ms, publish_relay_heartbeat, RelayHeartbeatPayload};
use crate::protocol::binary;
use crate::protocol::{ClientRequest, ErrorResponse, ServerResponse};
use crate::runtime::{RelayEndpoint, RelayRuntime};

const TCP_FRAME_SIZE_LIMIT: usize = 64 * 1024;

pub struct RelayDaemon {
    socket: Arc<UdpSocket>,
    runtime: Arc<Mutex<RelayRuntime>>,
    active_sessions: Arc<AtomicUsize>,
    tcp_clients: Arc<Mutex<HashMap<u64, TcpStream>>>,
}

impl RelayDaemon {
    pub fn bind(config: DaemonConfig) -> Result<Self, String> {
        config.validate()?;
        if !config.allow_unsigned_tickets
            && config
                .ticket_signing_secret
                .as_deref()
                .map(str::trim)
                .filter(|value| !value.is_empty())
                .is_none()
        {
            return Err(
                "relay ticket signing secret is required; set SLAN_RELAY_TICKET_SIGNING_SECRET or allowUnsignedTickets=true only for local development"
                    .to_string(),
            );
        }
        let socket = UdpSocket::bind(&config.udp_bind)
            .map_err(|err| format!("bind relay udp socket {}: {err}", config.udp_bind))?;
        let socket = Arc::new(socket);
        let active_sessions = Arc::new(AtomicUsize::new(0));
        start_mqtt_heartbeat(
            config.clone(),
            active_sessions.clone(),
            socket.local_addr().ok(),
        );
        let runtime = Arc::new(Mutex::new(RelayRuntime::new(
            config.relay_url_prefix,
            config.ticket_signing_secret,
        )));
        let tcp_clients = Arc::new(Mutex::new(HashMap::new()));
        let next_tcp_client_id = Arc::new(AtomicU64::new(1));
        if let Some(tcp_bind) = config
            .tcp_bind
            .clone()
            .filter(|value| !value.trim().is_empty())
        {
            start_tcp_listener(
                tcp_bind,
                Arc::clone(&runtime),
                Arc::clone(&socket),
                Arc::clone(&tcp_clients),
                Arc::clone(&next_tcp_client_id),
                Arc::clone(&active_sessions),
            )?;
        }
        Ok(Self {
            socket,
            runtime,
            active_sessions,
            tcp_clients,
        })
    }

    pub fn local_addr(&self) -> Result<SocketAddr, String> {
        self.socket
            .local_addr()
            .map_err(|err| format!("read relay local addr: {err}"))
    }

    pub fn serve(&mut self) -> Result<(), String> {
        let mut buf = [0_u8; 64 * 1024];
        loop {
            let (len, source) = self
                .socket
                .recv_from(&mut buf)
                .map_err(|err| format!("recv relay udp packet: {err}"))?;
            if binary::is_slan_relay_frame(&buf[..len]) {
                let result = self
                    .runtime
                    .lock()
                    .map_err(|_| "relay runtime mutex poisoned".to_string())?
                    .handle_binary_data_frame(source, &buf[..len]);
                match result {
                    Ok(Some((peer_endpoint, packet))) => {
                        self.send_endpoint_raw(peer_endpoint, &packet)?;
                    }
                    Ok(None) => {}
                    Err(error) => self.send_response(
                        source,
                        &ServerResponse::Error(ErrorResponse {
                            code: error.code.to_string(),
                            message: error.message,
                        }),
                    )?,
                }
                continue;
            }
            let request = serde_json::from_slice::<ClientRequest>(&buf[..len])
                .map_err(|err| format!("decode relay request: {err}"))?;
            let result = self
                .runtime
                .lock()
                .map_err(|_| "relay runtime mutex poisoned".to_string())?
                .handle_request(source, request);
            match result {
                Ok((response, maybe_peer_packet)) => {
                    self.update_active_sessions()?;
                    if let Some((peer_addr, packet)) = maybe_peer_packet {
                        self.send_response(peer_addr, &packet)?;
                    }
                    self.send_response(source, &response)?;
                }
                Err(error) => {
                    self.send_response(
                        source,
                        &ServerResponse::Error(ErrorResponse {
                            code: error.code.to_string(),
                            message: error.message,
                        }),
                    )?;
                }
            }
        }
    }

    fn send_response(&self, target: SocketAddr, response: &ServerResponse) -> Result<(), String> {
        let payload =
            serde_json::to_vec(response).map_err(|err| format!("encode relay response: {err}"))?;
        self.send_raw(target, &payload)
    }

    fn send_raw(&self, target: SocketAddr, payload: &[u8]) -> Result<(), String> {
        self.socket
            .send_to(payload, target)
            .map_err(|err| format!("send relay udp response: {err}"))?;
        Ok(())
    }

    fn send_endpoint_raw(&self, target: RelayEndpoint, payload: &[u8]) -> Result<(), String> {
        match target {
            RelayEndpoint::Udp(addr) => self.send_raw(addr, payload),
            RelayEndpoint::Tcp(client_id) => {
                write_tcp_client_frame(&self.tcp_clients, client_id, payload)
            }
        }
    }

    fn update_active_sessions(&self) -> Result<(), String> {
        let active = self
            .runtime
            .lock()
            .map_err(|_| "relay runtime mutex poisoned".to_string())?
            .active_session_count();
        self.active_sessions.store(active, Ordering::Relaxed);
        Ok(())
    }
}

fn start_tcp_listener(
    tcp_bind: String,
    runtime: Arc<Mutex<RelayRuntime>>,
    udp_socket: Arc<UdpSocket>,
    tcp_clients: Arc<Mutex<HashMap<u64, TcpStream>>>,
    next_tcp_client_id: Arc<AtomicU64>,
    active_sessions: Arc<AtomicUsize>,
) -> Result<(), String> {
    let listener = TcpListener::bind(&tcp_bind)
        .map_err(|err| format!("bind relay tcp listener {tcp_bind}: {err}"))?;
    thread::spawn(move || {
        for incoming in listener.incoming() {
            let Ok(stream) = incoming else {
                continue;
            };
            let client_id = next_tcp_client_id.fetch_add(1, Ordering::Relaxed);
            let Ok(write_stream) = stream.try_clone() else {
                continue;
            };
            if tcp_clients
                .lock()
                .map(|mut clients| clients.insert(client_id, write_stream))
                .is_err()
            {
                continue;
            }
            let runtime = Arc::clone(&runtime);
            let udp_socket = Arc::clone(&udp_socket);
            let tcp_clients = Arc::clone(&tcp_clients);
            let active_sessions = Arc::clone(&active_sessions);
            thread::spawn(move || {
                handle_tcp_client(
                    client_id,
                    stream,
                    runtime,
                    udp_socket,
                    tcp_clients,
                    active_sessions,
                );
            });
        }
    });
    Ok(())
}

fn handle_tcp_client(
    client_id: u64,
    mut stream: TcpStream,
    runtime: Arc<Mutex<RelayRuntime>>,
    udp_socket: Arc<UdpSocket>,
    tcp_clients: Arc<Mutex<HashMap<u64, TcpStream>>>,
    active_sessions: Arc<AtomicUsize>,
) {
    loop {
        let Ok(frame) = read_tcp_frame(&mut stream) else {
            break;
        };
        if binary::is_slan_relay_frame(&frame) {
            let result = runtime.lock().map_err(|_| ()).and_then(|mut runtime| {
                runtime
                    .handle_binary_data_frame(RelayEndpoint::Tcp(client_id), &frame)
                    .map_err(|_| ())
            });
            if let Ok(Some((peer_endpoint, packet))) = result {
                let _ = send_endpoint_raw(&udp_socket, &tcp_clients, peer_endpoint, &packet);
            }
            continue;
        }
        let Ok(request) = serde_json::from_slice::<ClientRequest>(&frame) else {
            let _ = write_tcp_frame(
                &mut stream,
                &error_response_payload("invalid_request", "decode relay tcp request failed"),
            );
            continue;
        };
        let result = runtime.lock().map_err(|_| ()).and_then(|mut runtime| {
            runtime
                .handle_request_from(RelayEndpoint::Tcp(client_id), request)
                .map_err(|_| ())
        });
        match result {
            Ok((response, maybe_peer_packet)) => {
                if let Ok(runtime) = runtime.lock() {
                    active_sessions.store(runtime.active_session_count(), Ordering::Relaxed);
                }
                if let Some((peer_endpoint, packet)) = maybe_peer_packet {
                    if let Ok(payload) = serde_json::to_vec(&packet) {
                        let _ =
                            send_endpoint_raw(&udp_socket, &tcp_clients, peer_endpoint, &payload);
                    }
                }
                if let Ok(payload) = serde_json::to_vec(&response) {
                    let _ = write_tcp_frame(&mut stream, &payload);
                }
            }
            Err(()) => {
                let _ = write_tcp_frame(
                    &mut stream,
                    &error_response_payload("relay_error", "relay tcp request failed"),
                );
            }
        }
    }
    if let Ok(mut clients) = tcp_clients.lock() {
        clients.remove(&client_id);
    }
    if let Ok(mut runtime) = runtime.lock() {
        runtime.detach_endpoint(RelayEndpoint::Tcp(client_id));
        active_sessions.store(runtime.active_session_count(), Ordering::Relaxed);
    }
}

fn send_endpoint_raw(
    udp_socket: &UdpSocket,
    tcp_clients: &Mutex<HashMap<u64, TcpStream>>,
    target: RelayEndpoint,
    payload: &[u8],
) -> Result<(), String> {
    match target {
        RelayEndpoint::Udp(addr) => {
            udp_socket
                .send_to(payload, addr)
                .map_err(|err| format!("send relay udp response: {err}"))?;
            Ok(())
        }
        RelayEndpoint::Tcp(client_id) => write_tcp_client_frame(tcp_clients, client_id, payload),
    }
}

fn write_tcp_client_frame(
    tcp_clients: &Mutex<HashMap<u64, TcpStream>>,
    client_id: u64,
    payload: &[u8],
) -> Result<(), String> {
    let mut clients = tcp_clients
        .lock()
        .map_err(|_| "relay tcp clients mutex poisoned".to_string())?;
    let stream = clients
        .get_mut(&client_id)
        .ok_or_else(|| format!("relay tcp client {client_id} is not connected"))?;
    write_tcp_frame(stream, payload)
}

fn read_tcp_frame(stream: &mut TcpStream) -> Result<Vec<u8>, String> {
    let mut len_buffer = [0_u8; 4];
    stream
        .read_exact(&mut len_buffer)
        .map_err(|err| format!("read relay tcp frame length: {err}"))?;
    let len = u32::from_be_bytes(len_buffer) as usize;
    if len == 0 || len > TCP_FRAME_SIZE_LIMIT {
        return Err(format!("invalid relay tcp frame length: {len}"));
    }
    let mut frame = vec![0_u8; len];
    stream
        .read_exact(&mut frame)
        .map_err(|err| format!("read relay tcp frame payload: {err}"))?;
    Ok(frame)
}

fn write_tcp_frame(stream: &mut TcpStream, payload: &[u8]) -> Result<(), String> {
    if payload.is_empty() || payload.len() > TCP_FRAME_SIZE_LIMIT {
        return Err(format!(
            "invalid relay tcp response length: {}",
            payload.len()
        ));
    }
    let len = (payload.len() as u32).to_be_bytes();
    stream
        .write_all(&len)
        .and_then(|_| stream.write_all(payload))
        .map_err(|err| format!("write relay tcp frame: {err}"))
}

fn error_response_payload(code: &str, message: &str) -> Vec<u8> {
    serde_json::to_vec(&ServerResponse::Error(ErrorResponse {
        code: code.to_string(),
        message: message.to_string(),
    }))
    .unwrap_or_else(|_| b"{\"kind\":\"error\"}".to_vec())
}

#[cfg(test)]
mod tests {
    use super::{read_tcp_frame, write_tcp_frame, TCP_FRAME_SIZE_LIMIT};
    use std::net::{TcpListener, TcpStream};
    use std::thread;
    use std::time::Duration;

    #[test]
    fn tcp_frame_round_trips_length_prefixed_payload() {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let address = listener.local_addr().unwrap();
        let writer = thread::spawn(move || {
            let mut stream = TcpStream::connect(address).unwrap();
            write_tcp_frame(&mut stream, b"relay-frame").unwrap();
        });
        let (mut stream, _) = listener.accept().unwrap();
        stream
            .set_read_timeout(Some(Duration::from_millis(100)))
            .unwrap();

        assert_eq!(read_tcp_frame(&mut stream).unwrap(), b"relay-frame");
        writer.join().unwrap();
    }

    #[test]
    fn tcp_frame_rejects_oversized_payload() {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let address = listener.local_addr().unwrap();
        let writer = thread::spawn(move || {
            let mut stream = TcpStream::connect(address).unwrap();
            let payload = vec![0_u8; TCP_FRAME_SIZE_LIMIT + 1];
            write_tcp_frame(&mut stream, &payload).unwrap_err();
        });
        let (_stream, _) = listener.accept().unwrap();

        writer.join().unwrap();
    }
}

fn start_mqtt_heartbeat(
    config: DaemonConfig,
    active_sessions: Arc<AtomicUsize>,
    local_addr: Option<SocketAddr>,
) {
    let Some(mqtt) = config.mqtt.clone().filter(|value| value.enabled) else {
        return;
    };
    let nodes = mqtt.effective_nodes();
    if nodes.is_empty() {
        return;
    }
    let tcp_bind = config.tcp_bind.clone();
    thread::spawn(move || {
        let interval = Duration::from_secs(mqtt.interval_seconds.unwrap_or(30).max(5));
        loop {
            for node in &nodes {
                let address = heartbeat_node_address(
                    node.transport.as_str(),
                    &node.address,
                    local_addr,
                    tcp_bind.as_deref(),
                );
                let payload = RelayHeartbeatPayload {
                    node_id: node.node_id.clone(),
                    cluster_id: node.cluster_id.clone(),
                    country_code: node.country_code.clone(),
                    city_code: node.city_code.clone(),
                    transport: node.transport.clone(),
                    address,
                    healthy: true,
                    active_sessions: active_sessions.load(Ordering::Relaxed),
                    reported_at_ms: current_timestamp_ms(),
                };
                if let Err(err) = publish_relay_heartbeat(&mqtt, &payload) {
                    eprintln!(
                        "server-relay mqtt heartbeat failed for {}: {err}",
                        node.node_id
                    );
                }
            }
            thread::sleep(interval);
        }
    });
}

fn heartbeat_node_address(
    transport: &str,
    configured: &str,
    local_udp_addr: Option<SocketAddr>,
    tcp_bind: Option<&str>,
) -> String {
    let configured = configured.trim();
    if !configured.is_empty() {
        return configured.to_string();
    }
    match transport {
        "udp" => local_udp_addr
            .map(|value| value.to_string())
            .unwrap_or_default(),
        "tcp" => tcp_bind.unwrap_or_default().to_string(),
        _ => String::new(),
    }
}
