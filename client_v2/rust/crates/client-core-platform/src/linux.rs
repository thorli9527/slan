//! Linux platform bridge for SLAN mesh networking.
//!
//! Linux uses `client-core-service` for control/session/MQTT/path decisions and
//! keeps OS privileges behind this platform layer. The default backend manages a
//! persistent `slan0` TUN interface through `iproute2` and DNS through
//! `systemd-resolved` when available.

use std::{
    ffi::CString,
    fs::{File, OpenOptions},
    io::{ErrorKind, Read, Write},
    net::{Ipv4Addr, SocketAddr, ToSocketAddrs, UdpSocket},
    os::fd::{AsRawFd, RawFd},
    process::Command,
    sync::{
        atomic::{AtomicBool, Ordering},
        Arc, Mutex, OnceLock,
    },
    thread::{self, JoinHandle},
    time::Duration,
};

use anyhow::{anyhow, bail, Context, Result};
use client_core::{
    relay_frame::{decode_slan_relay_data_frame, encode_slan_relay_data_frame, stable_hash64},
    NetworkRuntimeState, PathCandidate, PathKind, PathState, PeerPathRuntime,
    PlatformDiagnosticCheck, PlatformNetwork, PlatformNetworkDiagnostics, RelayDataPlaneConfig,
    RelayPeerSession, RouteSpec,
};

const DEFAULT_INTERFACE_NAME: &str = "slan0";
const DEFAULT_MTU: u32 = 1280;
const MAX_PACKET_SIZE: usize = 4096;
const TUNSETIFF: libc::c_ulong = 0x400454ca;
const IFF_TUN: libc::c_short = 0x0001;
const IFF_NO_PI: libc::c_short = 0x1000;

#[derive(Debug, Clone, Default)]
pub struct LinuxPlatformNetwork;

#[derive(Debug, Default)]
struct LinuxRuntime {
    interface_name: String,
    virtual_ip: Option<String>,
    prefix_len: Option<u8>,
    dns_servers: Vec<String>,
    routes: Vec<RouteSpec>,
    relay_config: Option<RelayDataPlaneConfig>,
    tun: Option<TunRuntime>,
    mock_enabled: bool,
    network_enabled: bool,
}

impl LinuxRuntime {
    fn interface_name(&self) -> &str {
        if self.interface_name.trim().is_empty() {
            DEFAULT_INTERFACE_NAME
        } else {
            self.interface_name.as_str()
        }
    }
}

#[derive(Debug)]
struct TunRuntime {
    stop: Arc<AtomicBool>,
    handle: Option<JoinHandle<()>>,
}

impl Drop for TunRuntime {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::SeqCst);
        if let Some(handle) = self.handle.take() {
            let _ = handle.join();
        }
    }
}

#[derive(Debug)]
struct RelayPeer {
    session_id: String,
    peer_virtual_ips: Vec<String>,
    socket: UdpSocket,
}

fn runtime() -> &'static Mutex<LinuxRuntime> {
    static RUNTIME: OnceLock<Mutex<LinuxRuntime>> = OnceLock::new();
    RUNTIME.get_or_init(|| {
        Mutex::new(LinuxRuntime {
            interface_name: interface_name(),
            ..LinuxRuntime::default()
        })
    })
}

impl PlatformNetwork for LinuxPlatformNetwork {
    fn install_adapter(&self) -> Result<()> {
        let mut runtime = runtime().lock().expect("linux runtime mutex poisoned");
        runtime.mock_enabled = mock_enabled();
        if runtime.mock_enabled {
            runtime.network_enabled = true;
            return Ok(());
        }

        ensure_ip_command()?;
        let interface_name = runtime.interface_name().to_string();
        if !link_exists(&interface_name) {
            run_ip(&["tuntap", "add", "dev", &interface_name, "mode", "tun"])
                .with_context(|| format!("create Linux TUN interface {interface_name}"))?;
        }
        run_ip(&[
            "link",
            "set",
            "dev",
            &interface_name,
            "mtu",
            &DEFAULT_MTU.to_string(),
        ])
        .with_context(|| format!("set Linux TUN MTU on {interface_name}"))?;
        run_ip(&["link", "set", "dev", &interface_name, "up"])
            .with_context(|| format!("bring Linux TUN interface {interface_name} up"))?;
        runtime.network_enabled = true;
        Ok(())
    }

    fn configure_ip(&self, virtual_ip: &str, prefix_len: u8) -> Result<()> {
        let mut runtime = runtime().lock().expect("linux runtime mutex poisoned");
        let virtual_ip = normalize_ipv4(virtual_ip)?;
        runtime.virtual_ip = Some(virtual_ip.to_string());
        runtime.prefix_len = Some(prefix_len);
        if runtime.mock_enabled {
            return Ok(());
        }
        let interface_name = runtime.interface_name().to_string();
        run_ip(&[
            "addr",
            "replace",
            &format!("{virtual_ip}/{prefix_len}"),
            "dev",
            &interface_name,
        ])
        .with_context(|| format!("configure Linux TUN IP {virtual_ip}/{prefix_len}"))?;
        Ok(())
    }

    fn configure_routes(&self, routes: &[RouteSpec]) -> Result<()> {
        let mut runtime = runtime().lock().expect("linux runtime mutex poisoned");
        runtime.routes = routes.to_vec();
        if runtime.mock_enabled {
            return Ok(());
        }
        let interface_name = runtime.interface_name().to_string();
        for route in routes.iter().filter(|route| route.destination != "mesh") {
            let destination = normalize_route_destination(&route.destination)?;
            if let Some(gateway) = route
                .gateway
                .as_deref()
                .filter(|value| !value.trim().is_empty())
            {
                run_ip(&[
                    "route",
                    "replace",
                    &destination,
                    "via",
                    gateway,
                    "dev",
                    &interface_name,
                ])
                .with_context(|| format!("replace Linux route {destination} via {gateway}"))?;
            } else {
                run_ip(&["route", "replace", &destination, "dev", &interface_name])
                    .with_context(|| format!("replace Linux route {destination}"))?;
            }
        }
        Ok(())
    }

    fn configure_dns(&self, dns_servers: &[String]) -> Result<()> {
        let mut runtime = runtime().lock().expect("linux runtime mutex poisoned");
        runtime.dns_servers = dns_servers
            .iter()
            .map(|value| value.trim().to_string())
            .filter(|value| !value.is_empty())
            .collect();
        if runtime.mock_enabled || runtime.dns_servers.is_empty() {
            return Ok(());
        }
        let interface_name = runtime.interface_name().to_string();
        if command_available("resolvectl") {
            let mut dns_args = vec!["dns".to_string(), interface_name.clone()];
            dns_args.extend(runtime.dns_servers.iter().cloned());
            run_command("resolvectl", &dns_args)?;
            let _ = run_command(
                "resolvectl",
                &[
                    "domain".to_string(),
                    interface_name.clone(),
                    "~slan".to_string(),
                ],
            );
            let _ = run_command(
                "resolvectl",
                &[
                    "default-route".to_string(),
                    interface_name,
                    "false".to_string(),
                ],
            );
        }
        Ok(())
    }

    fn configure_relay(&self, relay_config: Option<&RelayDataPlaneConfig>) -> Result<()> {
        let mut runtime = runtime().lock().expect("linux runtime mutex poisoned");
        runtime.relay_config = relay_config.cloned();
        if runtime.mock_enabled {
            return Ok(());
        }
        restart_data_plane(&mut runtime)?;
        Ok(())
    }

    fn diagnostics(&self) -> Result<PlatformNetworkDiagnostics> {
        let runtime = runtime().lock().expect("linux runtime mutex poisoned");
        let interface_name = runtime.interface_name().to_string();
        let adapter_present = runtime.mock_enabled || link_exists(&interface_name);
        let mut checks = vec![
            PlatformDiagnosticCheck {
                name: "iproute2".to_string(),
                ok: command_available("ip"),
                message: Some("ip command is required for Linux TUN setup".to_string()),
            },
            PlatformDiagnosticCheck {
                name: "tunDevice".to_string(),
                ok: std::path::Path::new("/dev/net/tun").exists() || runtime.mock_enabled,
                message: Some(
                    "/dev/net/tun is required unless SLAN_LINUX_NETWORK_MOCK=1".to_string(),
                ),
            },
        ];
        checks.push(PlatformDiagnosticCheck {
            name: "dns".to_string(),
            ok: command_available("resolvectl") || runtime.dns_servers.is_empty(),
            message: Some("resolvectl is used when Linux DNS servers are configured".to_string()),
        });
        checks.push(PlatformDiagnosticCheck {
            name: "virtualIp".to_string(),
            ok: runtime.virtual_ip.is_some(),
            message: runtime.virtual_ip.clone(),
        });
        checks.push(PlatformDiagnosticCheck {
            name: "routes".to_string(),
            ok: runtime.routes.iter().all(|route| {
                route.destination == "mesh"
                    || normalize_route_destination(&route.destination).is_ok()
            }),
            message: Some(format!("{} Linux routes configured", runtime.routes.len())),
        });
        checks.push(PlatformDiagnosticCheck {
            name: "relayDataPlane".to_string(),
            ok: runtime
                .relay_config
                .as_ref()
                .map(|config| {
                    config.enabled
                        && config.transport.eq_ignore_ascii_case("udp")
                        && !config.sessions.is_empty()
                })
                .unwrap_or(false),
            message: runtime.relay_config.as_ref().map(|config| {
                format!(
                    "transport={} sessions={}",
                    config.transport,
                    config.sessions.len()
                )
            }),
        });

        Ok(PlatformNetworkDiagnostics {
            platform: "linux".to_string(),
            adapter_present,
            adapter_name: Some(interface_name.clone()),
            admin_status: Some(if runtime.network_enabled {
                "up".to_string()
            } else {
                "down".to_string()
            }),
            interface_index: interface_index(&interface_name),
            virtual_ip: runtime.virtual_ip.clone(),
            mtu: Some(DEFAULT_MTU),
            mss: None,
            dns_servers: runtime.dns_servers.clone(),
            routes: runtime
                .routes
                .iter()
                .map(|route| route.destination.clone())
                .collect(),
            checks,
        })
    }

    fn disable_network(&self) -> Result<()> {
        let mut runtime = runtime().lock().expect("linux runtime mutex poisoned");
        runtime.tun = None;
        if !runtime.mock_enabled {
            let interface_name = runtime.interface_name().to_string();
            for route in runtime
                .routes
                .iter()
                .filter(|route| route.destination != "mesh")
            {
                if let Ok(destination) = normalize_route_destination(&route.destination) {
                    let _ = run_ip(&["route", "del", &destination, "dev", &interface_name]);
                }
            }
            if command_available("resolvectl") {
                let _ = run_command(
                    "resolvectl",
                    &["revert".to_string(), interface_name.clone()],
                );
            }
            let _ = run_ip(&["addr", "flush", "dev", &interface_name]);
            let _ = run_ip(&["link", "set", "dev", &interface_name, "down"]);
        }
        runtime.network_enabled = false;
        runtime.virtual_ip = None;
        runtime.prefix_len = None;
        runtime.routes.clear();
        runtime.relay_config = None;
        Ok(())
    }

    fn read_runtime_state(&self) -> Result<NetworkRuntimeState> {
        let runtime = runtime().lock().expect("linux runtime mutex poisoned");
        let adapter_present = runtime.mock_enabled || link_exists(runtime.interface_name());
        Ok(NetworkRuntimeState {
            adapter_present,
            network_enabled: adapter_present && runtime.network_enabled,
            virtual_ip: runtime.virtual_ip.clone(),
            active_path: runtime
                .relay_config
                .as_ref()
                .filter(|relay| relay.enabled && !relay.sessions.is_empty())
                .and_then(|relay| {
                    let paths = relay_runtime_paths_from_config(relay);
                    client_core::selected_runtime_paths(&relay.path_policy, paths)
                        .into_iter()
                        .find_map(|path| path.active_path)
                }),
            peer_paths: runtime
                .relay_config
                .as_ref()
                .map(|relay| {
                    let peer_paths = relay_runtime_paths_from_config(relay);
                    client_core::selected_runtime_paths(&relay.path_policy, peer_paths)
                })
                .unwrap_or_default(),
        })
    }
}

fn restart_data_plane(runtime: &mut LinuxRuntime) -> Result<()> {
    let config = runtime.relay_config.clone().filter(|config| {
        config.enabled
            && config.transport.eq_ignore_ascii_case("udp")
            && !config.relay_address.trim().is_empty()
            && !config.sessions.is_empty()
    });
    runtime.tun = None;
    if let Some(config) = config {
        runtime.tun = Some(start_udp_data_plane(runtime.interface_name(), config)?);
    }
    Ok(())
}

fn start_udp_data_plane(interface_name: &str, config: RelayDataPlaneConfig) -> Result<TunRuntime> {
    let file = open_tun(interface_name)
        .with_context(|| format!("open Linux TUN interface {interface_name}"))?;
    let peers = attach_udp_relay_sessions(&config)?;
    let config_hash = stable_hash64(&serde_json::to_string(&config)?);
    let max_frame_payload = usize::from(config.max_frame_payload.unwrap_or(1200).clamp(512, 1400));
    let stop = Arc::new(AtomicBool::new(false));
    let thread_stop = Arc::clone(&stop);
    let local_node_id = config.local_node_id.clone();
    let handle = thread::spawn(move || {
        run_udp_data_plane(
            file,
            peers,
            local_node_id,
            max_frame_payload,
            config_hash,
            thread_stop,
        );
    });
    Ok(TunRuntime {
        stop,
        handle: Some(handle),
    })
}

fn open_tun(interface_name: &str) -> Result<File> {
    let file = OpenOptions::new()
        .read(true)
        .write(true)
        .open("/dev/net/tun")
        .context("open /dev/net/tun")?;
    let mut request = TunIfReq::new(interface_name)?;
    let result = unsafe { libc::ioctl(file.as_raw_fd(), TUNSETIFF, &mut request) };
    if result < 0 {
        return Err(std::io::Error::last_os_error())
            .with_context(|| format!("ioctl TUNSETIFF {interface_name}"));
    }
    set_nonblocking(file.as_raw_fd())?;
    Ok(file)
}

#[repr(C)]
struct TunIfReq {
    name: [libc::c_char; libc::IFNAMSIZ],
    flags: libc::c_short,
    padding: [u8; 64],
}

impl TunIfReq {
    fn new(interface_name: &str) -> Result<Self> {
        let name = CString::new(interface_name.trim())?;
        let bytes = name.as_bytes_with_nul();
        if bytes.len() > libc::IFNAMSIZ {
            bail!("Linux TUN interface name is too long: {interface_name}");
        }
        let mut request = Self {
            name: [0; libc::IFNAMSIZ],
            flags: IFF_TUN | IFF_NO_PI,
            padding: [0; 64],
        };
        for (index, byte) in bytes.iter().enumerate() {
            request.name[index] = *byte as libc::c_char;
        }
        Ok(request)
    }
}

fn attach_udp_relay_sessions(config: &RelayDataPlaneConfig) -> Result<Vec<RelayPeer>> {
    let mut peers = Vec::new();
    for session in &config.sessions {
        let relay_addr = relay_udp_address_for_session(config.relay_address.as_str(), session)
            .with_context(|| format!("parse relay address {}", config.relay_address))?;
        let socket = UdpSocket::bind("0.0.0.0:0").context("bind Linux relay UDP socket")?;
        socket
            .connect(relay_addr)
            .with_context(|| format!("connect Linux relay UDP socket {relay_addr}"))?;
        attach_udp_relay_session(&socket, config.local_node_id.as_str(), session)
            .with_context(|| format!("attach Linux relay session {}", session.session_id))?;
        socket.set_nonblocking(true)?;
        peers.push(RelayPeer {
            session_id: session.session_id.clone(),
            peer_virtual_ips: session.peer_virtual_ips.clone(),
            socket,
        });
    }
    Ok(peers)
}

fn attach_udp_relay_session(
    socket: &UdpSocket,
    local_node_id: &str,
    session: &RelayPeerSession,
) -> Result<()> {
    socket.set_read_timeout(Some(Duration::from_secs(2)))?;
    socket.set_write_timeout(Some(Duration::from_secs(2)))?;
    let attach = serde_json::json!({
        "kind": "attach",
        "participant_id": local_node_id,
        "ticket": relay_ticket_wire(session),
        "transport": PathKind::RelayUdp.as_str(),
    });
    let payload = serde_json::to_vec(&attach)?;
    let mut response = vec![0_u8; 4096];
    let mut last_error = None;
    for _ in 0..3 {
        if let Err(error) = socket.send(&payload) {
            last_error = Some(anyhow!(error));
            thread::sleep(Duration::from_millis(250));
            continue;
        }
        match socket.recv(&mut response) {
            Ok(len) => {
                verify_relay_attach_ack(&response[..len], &session.session_id)?;
                socket.set_read_timeout(None)?;
                socket.set_write_timeout(None)?;
                return Ok(());
            }
            Err(error)
                if error.kind() == ErrorKind::WouldBlock || error.kind() == ErrorKind::TimedOut =>
            {
                last_error = Some(anyhow!(error));
                thread::sleep(Duration::from_millis(250));
            }
            Err(error) => {
                last_error = Some(anyhow!(error));
                thread::sleep(Duration::from_millis(250));
            }
        }
    }
    Err(last_error.unwrap_or_else(|| anyhow!("Linux relay attach timed out")))
}

fn relay_udp_address_for_session(
    default_address: &str,
    session: &RelayPeerSession,
) -> Result<SocketAddr> {
    let address = normalize_relay_udp_address(session.ticket.relay_url.as_str())
        .or_else(|| normalize_relay_udp_address(default_address))
        .or_else(|| {
            let trimmed = default_address.trim();
            (!trimmed.is_empty() && !trimmed.contains("://")).then(|| trimmed.to_string())
        })
        .ok_or_else(|| anyhow!("missing Linux relay UDP address"))?;
    address
        .to_socket_addrs()
        .with_context(|| format!("resolve Linux relay UDP socket address {address}"))?
        .next()
        .ok_or_else(|| anyhow!("Linux relay UDP address resolved no endpoints: {address}"))
}

fn normalize_relay_udp_address(address: &str) -> Option<String> {
    let trimmed = address.trim();
    if trimmed.is_empty() {
        return None;
    }
    let stripped = trimmed
        .strip_prefix("udp://")
        .or_else(|| trimmed.strip_prefix("relay+udp://"));
    if stripped.is_none() && trimmed.contains("://") {
        return None;
    }
    let normalized = stripped.unwrap_or(trimmed).trim();
    (!normalized.is_empty()).then(|| normalized.to_string())
}

fn relay_ticket_wire(session: &RelayPeerSession) -> serde_json::Value {
    let ticket = &session.ticket;
    serde_json::json!({
        "ticket_id": &ticket.ticket_id,
        "network_id": &ticket.network_id,
        "session_id": &ticket.session_id,
        "src_node_id": &ticket.src_node_id,
        "dst_node_id": &ticket.dst_node_id,
        "derp_cluster_id": &ticket.derp_cluster_id,
        "country_code": &ticket.country_code,
        "city_code": &ticket.city_code,
        "allowed_derp_node_ids": &ticket.allowed_derp_node_ids,
        "relay_url": &ticket.relay_url,
        "expires_at": &ticket.expires_at,
        "session_key": &ticket.session_key,
        "signature": &ticket.signature,
    })
}

fn verify_relay_attach_ack(response: &[u8], session_id: &str) -> Result<()> {
    let value: serde_json::Value = serde_json::from_slice(response)?;
    match value.get("kind").and_then(serde_json::Value::as_str) {
        Some("attached") => {
            let ack_session = value
                .get("session_id")
                .or_else(|| value.get("sessionId"))
                .and_then(serde_json::Value::as_str)
                .unwrap_or_default();
            if ack_session == session_id {
                Ok(())
            } else {
                bail!("Linux relay attach session mismatch")
            }
        }
        Some("error") => bail!(
            "{}",
            value
                .get("message")
                .and_then(serde_json::Value::as_str)
                .unwrap_or("Linux relay attach failed")
        ),
        _ => bail!("unexpected Linux relay attach response"),
    }
}

fn run_udp_data_plane(
    mut file: File,
    peers: Vec<RelayPeer>,
    local_node_id: String,
    max_frame_payload: usize,
    config_hash: u64,
    stop: Arc<AtomicBool>,
) {
    let mut seq = 0_u64;
    let mut tun_buffer = vec![0_u8; MAX_PACKET_SIZE];
    let mut relay_buffer = vec![0_u8; MAX_PACKET_SIZE + 512];
    while !stop.load(Ordering::SeqCst) {
        match file.read(&mut tun_buffer) {
            Ok(0) => thread::sleep(Duration::from_millis(10)),
            Ok(packet_len) => {
                let packet = &tun_buffer[..packet_len];
                if packet.first().map(|byte| byte >> 4) == Some(4)
                    && packet.len() <= max_frame_payload
                {
                    if let Some(peer) = relay_peer_for_packet(&peers, packet) {
                        seq = seq.wrapping_add(1);
                        if let Some(frame) = encode_slan_relay_data_frame(seq, config_hash, packet)
                        {
                            let _ = peer.socket.send(&frame);
                        }
                    }
                }
            }
            Err(error) if error.kind() == ErrorKind::WouldBlock => {
                thread::sleep(Duration::from_millis(10));
            }
            Err(error) if error.kind() == ErrorKind::Interrupted => {}
            Err(_) => break,
        }
        for peer in &peers {
            match peer.socket.recv(&mut relay_buffer) {
                Ok(frame_len) => {
                    if let Some(packet) = decode_slan_relay_data_frame(&relay_buffer[..frame_len]) {
                        let _ = file.write_all(packet);
                    }
                }
                Err(error) if error.kind() == ErrorKind::WouldBlock => {}
                Err(error) if error.kind() == ErrorKind::Interrupted => {}
                Err(_) => {}
            }
        }
    }
    detach_udp_relay_sessions(&peers, local_node_id.as_str());
}

fn relay_peer_for_packet<'a>(peers: &'a [RelayPeer], packet: &[u8]) -> Option<&'a RelayPeer> {
    if peers.is_empty() {
        return None;
    }
    if peers.len() == 1 {
        return Some(&peers[0]);
    }
    let destination = ipv4_destination(packet)?;
    peers.iter().find(|peer| {
        peer.peer_virtual_ips
            .iter()
            .any(|ip| normalize_virtual_ip(ip) == destination)
    })
}

fn ipv4_destination(packet: &[u8]) -> Option<String> {
    if packet.len() < 20 || packet[0] >> 4 != 4 {
        return None;
    }
    Some(format!(
        "{}.{}.{}.{}",
        packet[16], packet[17], packet[18], packet[19]
    ))
}

fn normalize_virtual_ip(value: &str) -> String {
    value
        .trim()
        .split_once('/')
        .map(|(ip, _)| ip)
        .unwrap_or_else(|| value.trim())
        .to_string()
}

fn detach_udp_relay_sessions(peers: &[RelayPeer], local_node_id: &str) {
    for peer in peers {
        if peer.session_id.is_empty() {
            continue;
        }
        let detach = serde_json::json!({
            "kind": "detach",
            "session_id": &peer.session_id,
            "participant_id": local_node_id,
        });
        if let Ok(payload) = serde_json::to_vec(&detach) {
            let _ = peer.socket.send(&payload);
        }
    }
}

fn relay_runtime_paths_from_config(config: &RelayDataPlaneConfig) -> Vec<PeerPathRuntime> {
    let mut paths = config
        .peer_paths
        .iter()
        .map(|path| PeerPathRuntime {
            peer_node_id: path.peer_node_id.clone(),
            peer_virtual_ips: path.peer_virtual_ips.clone(),
            active_path: None,
            candidates: path.candidates.clone(),
        })
        .collect::<Vec<_>>();
    for session in &config.sessions {
        let Some(path_kind) = relay_path_kind_from_session(session) else {
            continue;
        };
        if let Some(path) = paths
            .iter_mut()
            .find(|path| path.peer_node_id == session.peer_node_id)
        {
            mark_relay_path_ready(path, session, path_kind);
        } else {
            let mut path = PeerPathRuntime {
                peer_node_id: session.peer_node_id.clone(),
                peer_virtual_ips: session.peer_virtual_ips.clone(),
                active_path: None,
                candidates: Vec::new(),
            };
            mark_relay_path_ready(&mut path, session, path_kind);
            paths.push(path);
        }
    }
    paths
}

fn mark_relay_path_ready(
    path: &mut PeerPathRuntime,
    session: &RelayPeerSession,
    path_kind: PathKind,
) {
    if path.peer_virtual_ips.is_empty() {
        path.peer_virtual_ips = session.peer_virtual_ips.clone();
    }
    if let Some(candidate) = path
        .candidates
        .iter_mut()
        .find(|candidate| candidate.kind == path_kind)
    {
        candidate.state = PathState::Ready;
        candidate.session_id = Some(session.session_id.clone());
        candidate.address = Some(session.ticket.relay_url.clone());
        candidate.transport = relay_transport_for_path(path_kind).map(str::to_string);
        return;
    }
    path.candidates.push(PathCandidate {
        kind: path_kind,
        state: PathState::Ready,
        endpoint_id: None,
        address: Some(session.ticket.relay_url.clone()),
        session_id: Some(session.session_id.clone()),
        transport: relay_transport_for_path(path_kind).map(str::to_string),
        rtt_ms: None,
        path_score: None,
        last_ok_at_ms: None,
        last_error: None,
    });
}

fn relay_path_kind_from_session(session: &RelayPeerSession) -> Option<PathKind> {
    let relay_url = session.ticket.relay_url.trim();
    let (scheme, _) = relay_url.split_once("://")?;
    match scheme.to_ascii_lowercase().as_str() {
        "udp" | "relay+udp" => Some(PathKind::RelayUdp),
        "derp" | "derp+tcp+tls" | "derp_tcp_tls_443" => Some(PathKind::DerpTcpTls443),
        _ => None,
    }
}

fn relay_transport_for_path(path_kind: PathKind) -> Option<&'static str> {
    match path_kind {
        PathKind::RelayUdp => Some("udp"),
        PathKind::DerpTcpTls443 => Some("derp_tcp_tls_443"),
        PathKind::LanUdp | PathKind::Ipv6Udp | PathKind::DirectUdp => None,
    }
}

fn set_nonblocking(fd: RawFd) -> Result<()> {
    let flags = unsafe { libc::fcntl(fd, libc::F_GETFL, 0) };
    if flags < 0 {
        return Err(std::io::Error::last_os_error()).context("fcntl F_GETFL");
    }
    let result = unsafe { libc::fcntl(fd, libc::F_SETFL, flags | libc::O_NONBLOCK) };
    if result < 0 {
        return Err(std::io::Error::last_os_error()).context("fcntl F_SETFL O_NONBLOCK");
    }
    Ok(())
}

fn interface_index(interface_name: &str) -> Option<u32> {
    let name = CString::new(interface_name).ok()?;
    let index = unsafe { libc::if_nametoindex(name.as_ptr()) };
    (index != 0).then_some(index)
}

fn interface_name() -> String {
    std::env::var("SLAN_LINUX_TUN_NAME")
        .ok()
        .map(|value| value.trim().to_string())
        .filter(|value| !value.is_empty())
        .unwrap_or_else(|| DEFAULT_INTERFACE_NAME.to_string())
}

fn mock_enabled() -> bool {
    matches!(
        std::env::var("SLAN_LINUX_NETWORK_MOCK")
            .unwrap_or_default()
            .to_ascii_lowercase()
            .as_str(),
        "1" | "true" | "yes" | "on"
    )
}

fn normalize_ipv4(value: &str) -> Result<Ipv4Addr> {
    value
        .trim()
        .parse::<Ipv4Addr>()
        .with_context(|| format!("invalid Linux virtual IPv4 address: {value}"))
}

fn normalize_route_destination(destination: &str) -> Result<String> {
    let value = destination.trim();
    if value.is_empty() {
        bail!("Linux route destination is empty");
    }
    if value == "mesh" {
        return Ok(value.to_string());
    }
    if value.contains('/') {
        let (ip, prefix) = value
            .split_once('/')
            .ok_or_else(|| anyhow!("invalid Linux route destination: {value}"))?;
        normalize_ipv4(ip)?;
        let prefix_len = prefix
            .parse::<u8>()
            .with_context(|| format!("invalid Linux route prefix: {value}"))?;
        if prefix_len > 32 {
            bail!("invalid Linux route prefix: {value}");
        }
        return Ok(format!("{ip}/{prefix_len}"));
    }
    normalize_ipv4(value)?;
    Ok(format!("{value}/32"))
}

fn ensure_ip_command() -> Result<()> {
    if command_available("ip") {
        Ok(())
    } else {
        bail!("Linux iproute2 command 'ip' is required")
    }
}

fn link_exists(interface_name: &str) -> bool {
    Command::new("ip")
        .args(["link", "show", "dev", interface_name])
        .output()
        .map(|output| output.status.success())
        .unwrap_or(false)
}

fn run_ip(args: &[&str]) -> Result<()> {
    let owned: Vec<String> = args.iter().map(|value| value.to_string()).collect();
    run_command("ip", &owned)
}

fn run_command(program: &str, args: &[String]) -> Result<()> {
    let output = Command::new(program)
        .args(args)
        .output()
        .with_context(|| format!("run {program} {}", args.join(" ")))?;
    if output.status.success() {
        return Ok(());
    }
    let stderr = String::from_utf8_lossy(&output.stderr).trim().to_string();
    let stdout = String::from_utf8_lossy(&output.stdout).trim().to_string();
    let message = if !stderr.is_empty() { stderr } else { stdout };
    bail!(
        "{} {} failed: {}",
        program,
        args.join(" "),
        if message.is_empty() {
            output.status.to_string()
        } else {
            message
        }
    )
}

fn command_available(program: &str) -> bool {
    Command::new(program)
        .arg("-V")
        .output()
        .map(|output| output.status.success())
        .unwrap_or(false)
        || Command::new("which")
            .arg(program)
            .output()
            .map(|output| output.status.success())
            .unwrap_or(false)
}

#[cfg(test)]
mod tests {
    use super::*;
    use client_core::{PathPolicy, PeerPathConfig, RelayTicket};
    use std::sync::{Mutex, OnceLock};

    fn test_lock() -> std::sync::MutexGuard<'static, ()> {
        static LOCK: OnceLock<Mutex<()>> = OnceLock::new();
        LOCK.get_or_init(|| Mutex::new(()))
            .lock()
            .expect("linux platform test mutex poisoned")
    }

    fn reset_runtime() {
        let mut runtime = runtime().lock().expect("linux runtime mutex poisoned");
        *runtime = LinuxRuntime {
            interface_name: interface_name(),
            ..LinuxRuntime::default()
        };
    }

    fn test_ticket(relay_url: &str) -> RelayTicket {
        RelayTicket {
            ticket_id: "ticket-1".to_string(),
            network_id: "network-1".to_string(),
            session_id: "session-1".to_string(),
            src_node_id: "node-local".to_string(),
            dst_node_id: "node-peer".to_string(),
            derp_cluster_id: None,
            country_code: None,
            city_code: None,
            allowed_derp_node_ids: Vec::new(),
            relay_url: relay_url.to_string(),
            expires_at: "2026-05-13T00:00:00Z".to_string(),
            session_key: "session-key".to_string(),
            signature: "signature".to_string(),
        }
    }

    fn test_session(relay_url: &str) -> RelayPeerSession {
        RelayPeerSession {
            session_id: "session-1".to_string(),
            peer_node_id: "node-peer".to_string(),
            peer_virtual_ips: vec!["100.64.0.2".to_string()],
            ticket: test_ticket(relay_url),
        }
    }

    fn test_relay_config() -> RelayDataPlaneConfig {
        RelayDataPlaneConfig {
            enabled: true,
            transport: "udp".to_string(),
            relay_address: "127.0.0.1:29110".to_string(),
            local_node_id: "node-local".to_string(),
            network_id: "network-1".to_string(),
            path_policy: PathPolicy {
                preferred: vec![PathKind::RelayUdp],
                ..PathPolicy::default()
            },
            peer_paths: vec![PeerPathConfig {
                peer_node_id: "node-peer".to_string(),
                peer_virtual_ips: vec!["100.64.0.2".to_string()],
                candidates: vec![PathCandidate {
                    kind: PathKind::RelayUdp,
                    state: PathState::Probing,
                    endpoint_id: Some("relay-1".to_string()),
                    address: None,
                    session_id: None,
                    transport: None,
                    rtt_ms: None,
                    path_score: None,
                    last_ok_at_ms: None,
                    last_error: None,
                }],
            }],
            relay_mtu: Some(1280),
            max_frame_payload: Some(1200),
            sessions: vec![test_session("udp://127.0.0.1:29110")],
        }
    }

    #[test]
    fn linux_route_destinations_are_normalized_for_acl_routes() {
        assert_eq!(
            normalize_route_destination("100.64.0.9").unwrap(),
            "100.64.0.9/32"
        );
        assert_eq!(
            normalize_route_destination("100.64.0.0/24").unwrap(),
            "100.64.0.0/24"
        );
        assert_eq!(normalize_route_destination("mesh").unwrap(), "mesh");
        assert!(normalize_route_destination("not-an-ip").is_err());
    }

    #[test]
    fn linux_relay_udp_address_prefers_session_ticket_url() {
        let session = test_session("relay+udp://127.0.0.1:29111");
        assert_eq!(
            relay_udp_address_for_session("127.0.0.1:29110", &session)
                .unwrap()
                .to_string(),
            "127.0.0.1:29111"
        );
    }

    #[test]
    fn linux_relay_udp_address_resolves_hostnames() {
        let session = test_session("udp://localhost:29111");
        assert_eq!(
            relay_udp_address_for_session("127.0.0.1:29110", &session)
                .unwrap()
                .port(),
            29111
        );
    }

    #[test]
    fn linux_relay_paths_mark_sessions_ready() {
        let config = test_relay_config();
        let paths = client_core::selected_runtime_paths(
            &config.path_policy,
            relay_runtime_paths_from_config(&config),
        );

        let peer = paths
            .iter()
            .find(|path| path.peer_node_id == "node-peer")
            .expect("peer path should exist");
        assert_eq!(peer.active_path, Some(PathKind::RelayUdp));
        assert!(peer.candidates.iter().any(|candidate| {
            candidate.kind == PathKind::RelayUdp
                && candidate.state == PathState::Ready
                && candidate.session_id.as_deref() == Some("session-1")
        }));
    }

    #[test]
    fn linux_mock_runtime_records_dns_acl_and_relay_config() {
        let _guard = test_lock();
        std::env::set_var("SLAN_LINUX_NETWORK_MOCK", "1");
        reset_runtime();

        let platform = LinuxPlatformNetwork;
        platform.install_adapter().unwrap();
        platform.configure_ip("100.64.0.1", 32).unwrap();
        platform
            .configure_dns(&["100.64.0.53".to_string(), " ".to_string()])
            .unwrap();
        platform
            .configure_routes(&[
                RouteSpec {
                    destination: "mesh".to_string(),
                    gateway: None,
                },
                RouteSpec {
                    destination: "100.64.0.2".to_string(),
                    gateway: None,
                },
            ])
            .unwrap();
        let relay = test_relay_config();
        platform.configure_relay(Some(&relay)).unwrap();

        let state = platform.read_runtime_state().unwrap();
        assert!(state.adapter_present);
        assert!(state.network_enabled);
        assert_eq!(state.virtual_ip.as_deref(), Some("100.64.0.1"));
        assert_eq!(state.active_path, Some(PathKind::RelayUdp));

        let diagnostics = platform.diagnostics().unwrap();
        assert_eq!(diagnostics.dns_servers, vec!["100.64.0.53"]);
        assert_eq!(diagnostics.routes, vec!["mesh", "100.64.0.2"]);
        assert!(diagnostics
            .checks
            .iter()
            .any(|check| check.name == "relayDataPlane" && check.ok));

        platform.disable_network().unwrap();
        std::env::remove_var("SLAN_LINUX_NETWORK_MOCK");
        reset_runtime();
    }
}
