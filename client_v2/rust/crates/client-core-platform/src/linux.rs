//! Linux platform bridge for SLAN mesh networking.
//!
//! Linux uses `client-core-service` for control/session/MQTT/path decisions and
//! keeps OS privileges behind this platform layer. The default backend manages a
//! persistent `slan0` TUN interface through `iproute2` and DNS through
//! `systemd-resolved` when available.

use std::{
    ffi::CString,
    fs::{self, File, OpenOptions},
    io::{BufRead, BufReader, ErrorKind, Read, Write},
    net::{Ipv4Addr, SocketAddr, TcpStream, ToSocketAddrs, UdpSocket},
    os::fd::{AsRawFd, RawFd},
    path::PathBuf,
    process::Command,
    sync::{
        atomic::{AtomicBool, Ordering},
        Arc, Mutex, OnceLock,
    },
    thread::{self, JoinHandle},
    time::{Duration, Instant, SystemTime, UNIX_EPOCH},
};

use anyhow::{anyhow, bail, Context, Result};
use client_core::{
    acl_allows_egress_packet, acl_allows_ingress_packet, icmp_echo_reply_for_request,
    ipv4_transport_checksum_valid, normalize_ipv4_transport_checksums,
    relay_frame::{
        base64_decode, base64_encode, decode_slan_relay_data_frame, encode_slan_relay_data_frame,
        stable_hash64,
    },
    NetworkRuntimeState, PathCandidate, PathKind, PathState, PeerPathRuntime, PlatformAclPeer,
    PlatformAclPolicy, PlatformDiagnosticCheck, PlatformNetwork, PlatformNetworkDiagnostics,
    RelayDataPlaneConfig, RelayPeerSession, RouteSpec,
};
use serde::Serialize;

use crate::direct_udp::{
    direct_udp_control_packet, direct_udp_probe_interval_from_ms, DirectUdpControlKind,
    DirectUdpTransport,
};

const HOST_INTERFACE_PREFIX_LEN: u8 = 32;
const DEFAULT_INTERFACE_NAME: &str = "slan0";
const DEFAULT_MTU: u32 = 1280;
const MAX_PACKET_SIZE: usize = 4096;
const TUNSETIFF: libc::c_ulong = 0x400454ca;
const IFF_TUN: libc::c_short = 0x0001;
const IFF_NO_PI: libc::c_short = 0x1000;
const RELAY_STATS_FLUSH_INTERVAL: Duration = Duration::from_secs(10);
const RELAY_KEEPALIVE_INTERVAL: Duration = Duration::from_secs(30);
const DERP_WRITE_RETRY_TIMEOUT: Duration = Duration::from_millis(750);
const DATA_PLANE_IDLE_SLEEP: Duration = Duration::from_millis(2);

/// LinuxPlatformNetwork 是 Linux 的 PlatformNetwork 实现，负责 TUN、路由、
/// DNS、relay 数据面和 direct UDP runtime 的平台适配。
#[derive(Debug, Clone, Default)]
pub struct LinuxPlatformNetwork;

/// LinuxRuntime 保存 Linux 平台层的当前网络配置和 TUN runtime。
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

/// TunRuntime 持有 Linux TUN 数据面线程生命周期。
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

/// RelayPeer 是 Linux relay 数据面中单个 peer 的 UDP relay 会话。
#[derive(Debug)]
struct RelayPeer {
    session_id: String,
    peer_node_id: String,
    local_node_id: String,
    peer_virtual_ips: Vec<String>,
    socket: UdpSocket,
}

/// DerpPeer 是 Linux TCP/DERP 保底中继的一条长连接会话。
#[derive(Debug)]
struct DerpPeer {
    session_id: String,
    peer_node_id: String,
    local_node_id: String,
    server_session_id: String,
    peer_virtual_ips: Vec<String>,
    stream: TcpStream,
    reader: TcpStream,
    read_buffer: Vec<u8>,
}

/// RelayDataPlaneStats 是 Linux relay/direct UDP 数据面写入状态文件的统计快照。
#[derive(Debug, Default, Serialize)]
#[serde(rename_all = "camelCase")]
struct RelayDataPlaneStats {
    relay_address: String,
    relay_transport: Option<String>,
    active_path: Option<String>,
    requested_relay_session_count: u32,
    relay_session_count: u32,
    attached_peer_session_count: u32,
    attached_transport_count: u32,
    ticket_expires_at: Option<String>,
    ticket_expires_in_ms: Option<i64>,
    ticket_renew_due: bool,
    relay_attach_failures: u64,
    last_relay_attach_error: Option<String>,
    peers: Vec<RelayPeerStats>,
    relay_mtu: Option<u16>,
    max_frame_payload: Option<u16>,
    tun_packets_sent: u64,
    tun_tcp_packets_sent: u64,
    tun_tcp_syn_ack_sent: u64,
    tun_tcp_rst_sent: u64,
    tun_tcp_checksum_invalid: u64,
    relay_packets_received: u64,
    relay_tcp_packets_received: u64,
    relay_tcp_syn_received: u64,
    relay_tcp_rst_received: u64,
    relay_tcp_checksum_invalid: u64,
    relay_decode_failures: u64,
    relay_config_hash_mismatches: u64,
    relay_error_responses: u64,
    direct_udp_attached_peer_count: u64,
    direct_udp_ready_peer_count: u64,
    direct_udp_probes_sent: u64,
    direct_udp_probes_received: u64,
    direct_udp_pongs_sent: u64,
    direct_udp_pongs_received: u64,
    direct_udp_frames_sent: u64,
    direct_udp_frames_received: u64,
    last_relay_error: Option<String>,
    relay_send_failures: u64,
    relay_receive_failures: u64,
    unroutable_tun_packets: u64,
    last_unroutable_destination: Option<String>,
    oversized_tun_packets: u64,
    last_oversized_tun_packet_size: Option<u32>,
    wintun_write_failures: u64,
    started_at_ms: u64,
    last_tun_destination: Option<String>,
    last_tun_protocol: Option<u8>,
    last_tun_packet_size: Option<u32>,
    last_tun_peer_node_id: Option<String>,
    last_tun_send_path: Option<String>,
    last_tun_drop_reason: Option<String>,
    last_tun_packet_at_ms: Option<u64>,
    last_relay_packet_at_ms: Option<u64>,
    last_relay_keepalive_at_ms: Option<u64>,
    updated_at_ms: u64,
}

/// RelayPeerStats 是单个 peer 的 relay/direct UDP 发送、接收和路径切换统计。
#[derive(Debug, Default, Serialize)]
#[serde(rename_all = "camelCase")]
struct RelayPeerStats {
    peer_node_id: String,
    session_id: String,
    peer_virtual_ips: Vec<String>,
    attached: bool,
    attach_error: Option<String>,
    tun_packets_sent: u64,
    relay_packets_received: u64,
    relay_errors: u64,
    last_relay_error: Option<String>,
    last_send_path: Option<String>,
    path_downgrades: u64,
    path_upgrades: u64,
    last_path_change: Option<String>,
    replayed_frames: u64,
    config_hash_mismatches: u64,
    last_rx_seq: u64,
    send_failures: u64,
    receive_failures: u64,
    wintun_write_failures: u64,
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

    fn configure_ip(&self, virtual_ip: &str, _prefix_len: u8) -> Result<()> {
        let mut runtime = runtime().lock().expect("linux runtime mutex poisoned");
        let virtual_ip = normalize_ipv4(virtual_ip)?;
        runtime.virtual_ip = Some(virtual_ip.to_string());
        runtime.prefix_len = Some(HOST_INTERFACE_PREFIX_LEN);
        if runtime.mock_enabled {
            return Ok(());
        }
        let interface_name = runtime.interface_name().to_string();
        run_ip(&[
            "addr",
            "replace",
            &format!("{virtual_ip}/{HOST_INTERFACE_PREFIX_LEN}"),
            "dev",
            &interface_name,
        ])
        .with_context(|| {
            format!("configure Linux TUN IP {virtual_ip}/{HOST_INTERFACE_PREFIX_LEN}")
        })?;
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
        if runtime.tun.is_some() && runtime.relay_config.as_ref() == relay_config {
            eprintln!("linux configure_relay skipped unchanged config");
            return Ok(());
        }
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
                        && (config.transport.eq_ignore_ascii_case("udp")
                            || config.transport.eq_ignore_ascii_case("relay_udp")
                            || config.transport.eq_ignore_ascii_case("derp_tcp_tls_443"))
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
            && (config.transport.eq_ignore_ascii_case("udp")
                || config.transport.eq_ignore_ascii_case("relay_udp")
                || config.transport.eq_ignore_ascii_case("derp_tcp_tls_443"))
            && !config.relay_address.trim().is_empty()
            && !config.sessions.is_empty()
    });
    let interface_name = runtime.interface_name().to_string();
    let local_virtual_ip = runtime.virtual_ip.clone().unwrap_or_default();
    runtime.tun = None;
    runtime.tun = if let Some(config) = config {
        Some(start_udp_data_plane(
            interface_name.as_str(),
            config,
            local_virtual_ip,
        )?)
    } else if !local_virtual_ip.trim().is_empty() {
        Some(start_local_data_plane(
            interface_name.as_str(),
            local_virtual_ip,
        )?)
    } else {
        None
    };
    Ok(())
}

fn start_local_data_plane(interface_name: &str, local_virtual_ip: String) -> Result<TunRuntime> {
    let file = open_tun(interface_name)
        .with_context(|| format!("open Linux TUN interface {interface_name}"))?;
    eprintln!("linux local data plane attached virtual_ip={local_virtual_ip}");
    let stop = Arc::new(AtomicBool::new(false));
    let thread_stop = Arc::clone(&stop);
    let handle = thread::spawn(move || {
        run_local_data_plane(file, local_virtual_ip, thread_stop);
    });
    Ok(TunRuntime {
        stop,
        handle: Some(handle),
    })
}

fn start_udp_data_plane(
    interface_name: &str,
    config: RelayDataPlaneConfig,
    local_virtual_ip: String,
) -> Result<TunRuntime> {
    let file = open_tun(interface_name)
        .with_context(|| format!("open Linux TUN interface {interface_name}"))?;
    let peers = attach_udp_relay_sessions(&config)?;
    let derp_peers = attach_derp_relay_sessions(&config)?;
    if peers.is_empty() && derp_peers.is_empty() {
        bail!("attach Linux relay sessions failed: no UDP relay or DERP sessions attached");
    }
    let direct_udp = DirectUdpTransport::attach(config.local_node_id.as_str(), &config.peer_paths);
    let config_hash = stable_hash64(&serde_json::to_string(&config)?);
    let max_frame_payload = usize::from(config.max_frame_payload.unwrap_or(1200).clamp(512, 1400));
    let direct_udp_probe_interval =
        direct_udp_probe_interval_from_ms(config.path_policy.probe_interval_ms);
    let mut stats = relay_data_plane_stats_from_config(&config, &peers, &derp_peers);
    let acl_policies = config.acl_policies.clone();
    stats.direct_udp_attached_peer_count = direct_udp
        .as_ref()
        .map(|transport| transport.peers.len() as u64)
        .unwrap_or(0);
    stats.attached_transport_count = stats
        .attached_peer_session_count
        .saturating_add(stats.direct_udp_attached_peer_count as u32);
    let stop = Arc::new(AtomicBool::new(false));
    let thread_stop = Arc::clone(&stop);
    let local_node_id = config.local_node_id.clone();
    let handle = thread::spawn(move || {
        run_udp_data_plane(
            file,
            peers,
            derp_peers,
            direct_udp,
            local_node_id,
            local_virtual_ip,
            max_frame_payload,
            config_hash,
            direct_udp_probe_interval,
            acl_policies,
            &mut stats,
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
    let mut failures = Vec::new();
    for session in &config.sessions {
        if relay_path_kind_from_session(session) != Some(PathKind::RelayUdp) {
            continue;
        }
        let relay_addr = relay_udp_address_for_session(config.relay_address.as_str(), session)
            .with_context(|| format!("parse relay address {}", config.relay_address))?;
        let socket = UdpSocket::bind("0.0.0.0:0").context("bind Linux relay UDP socket")?;
        socket
            .connect(relay_addr)
            .with_context(|| format!("connect Linux relay UDP socket {relay_addr}"))?;
        if let Err(error) =
            attach_udp_relay_session(&socket, config.local_node_id.as_str(), session)
        {
            failures.push(format!("{}: {error}", session.session_id));
            continue;
        }
        socket.set_nonblocking(true)?;
        peers.push(RelayPeer {
            session_id: session.session_id.clone(),
            peer_node_id: session.peer_node_id.clone(),
            local_node_id: config.local_node_id.clone(),
            peer_virtual_ips: session.peer_virtual_ips.clone(),
            socket,
        });
    }
    if peers.is_empty() && !failures.is_empty() {
        bail!(
            "attach Linux relay sessions failed: {}",
            failures.join("; ")
        );
    }
    Ok(peers)
}

fn attach_derp_relay_sessions(config: &RelayDataPlaneConfig) -> Result<Vec<DerpPeer>> {
    let mut peers = Vec::new();
    let mut failures = Vec::new();
    for session in &config.sessions {
        if relay_path_kind_from_session(session) != Some(PathKind::DerpTcpTls443) {
            continue;
        }
        match attach_derp_relay_session(config.local_node_id.as_str(), session) {
            Ok(peer) => peers.push(peer),
            Err(error) => failures.push(format!("{}: {error}", session.session_id)),
        }
    }
    if peers.is_empty() && !failures.is_empty() {
        bail!("attach Linux DERP sessions failed: {}", failures.join("; "));
    }
    Ok(peers)
}

fn attach_derp_relay_session(local_node_id: &str, session: &RelayPeerSession) -> Result<DerpPeer> {
    let address = normalize_derp_tcp_address(session.ticket.relay_url.as_str())
        .ok_or_else(|| anyhow!("missing Linux DERP TCP address"))?;
    let stream = TcpStream::connect(address.as_str())
        .with_context(|| format!("connect Linux DERP TCP socket {address}"))?;
    stream.set_nodelay(true)?;
    stream.set_read_timeout(Some(Duration::from_secs(3)))?;
    stream.set_write_timeout(Some(Duration::from_secs(3)))?;
    let mut writer = stream
        .try_clone()
        .context("clone Linux DERP writer stream")?;
    let mut reader = BufReader::new(
        stream
            .try_clone()
            .context("clone Linux DERP reader stream")?,
    );
    let connect = serde_json::json!({
        "kind": "connect",
        "peerId": local_node_id,
        "nodeId": derp_ticket_node_id(session),
        "regionId": derp_ticket_region_id(session),
        "ticket": derp_ticket_wire(local_node_id, session),
    });
    write_json_line(&mut writer, &connect)?;
    let line = read_derp_connect_line_with_retry(&mut reader, Duration::from_secs(3))?;
    let value: serde_json::Value =
        serde_json::from_str(line.trim()).context("decode Linux DERP connect response")?;
    if value.get("kind").and_then(serde_json::Value::as_str) != Some("connected") {
        bail!(
            "{}",
            value
                .get("error")
                .and_then(|error| error.get("message"))
                .and_then(serde_json::Value::as_str)
                .unwrap_or("Linux DERP connect failed")
        );
    }
    let server_session_id = value
        .get("sessionId")
        .and_then(serde_json::Value::as_str)
        .unwrap_or(session.session_id.as_str())
        .to_string();
    writer.set_nonblocking(true)?;
    let reader = reader.into_inner();
    reader.set_nonblocking(true)?;
    Ok(DerpPeer {
        session_id: session.session_id.clone(),
        peer_node_id: session.peer_node_id.clone(),
        local_node_id: local_node_id.to_string(),
        server_session_id,
        peer_virtual_ips: session.peer_virtual_ips.clone(),
        stream: writer,
        reader,
        read_buffer: Vec::new(),
    })
}

fn normalize_derp_tcp_address(address: &str) -> Option<String> {
    let trimmed = address.trim();
    if trimmed.is_empty() {
        return None;
    }
    let stripped = trimmed
        .strip_prefix("derp://")
        .or_else(|| trimmed.strip_prefix("derp+tcp+tls://"))
        .or_else(|| trimmed.strip_prefix("derp_tcp_tls_443://"))?;
    let normalized = stripped.trim();
    (!normalized.is_empty()).then(|| normalized.to_string())
}

fn write_json_line(stream: &mut TcpStream, value: &serde_json::Value) -> Result<()> {
    let mut payload = serde_json::to_vec(value)?;
    payload.push(b'\n');
    write_all_with_would_block_retry(stream, &payload, DERP_WRITE_RETRY_TIMEOUT)?;
    Ok(())
}

fn read_derp_connect_line_with_retry<R: BufRead>(
    reader: &mut R,
    timeout: Duration,
) -> std::io::Result<String> {
    let started = Instant::now();
    loop {
        let mut line = String::new();
        match reader.read_line(&mut line) {
            Ok(0) => {
                return Err(std::io::Error::new(
                    ErrorKind::ConnectionReset,
                    "Linux DERP TCP connection closed before connect ack",
                ));
            }
            Ok(_) => return Ok(line),
            Err(error)
                if error.kind() == ErrorKind::WouldBlock || error.kind() == ErrorKind::TimedOut =>
            {
                if started.elapsed() >= timeout {
                    return Err(error);
                }
                thread::sleep(Duration::from_millis(10));
            }
            Err(error) if error.kind() == ErrorKind::Interrupted => {}
            Err(error) => return Err(error),
        }
    }
}

fn write_all_with_would_block_retry(
    stream: &mut TcpStream,
    payload: &[u8],
    timeout: Duration,
) -> std::io::Result<()> {
    let started = Instant::now();
    let mut offset = 0;
    while offset < payload.len() {
        match stream.write(&payload[offset..]) {
            Ok(0) => {
                return Err(std::io::Error::new(
                    ErrorKind::WriteZero,
                    "Linux DERP TCP write returned zero bytes",
                ));
            }
            Ok(written) => offset += written,
            Err(error) if error.kind() == ErrorKind::WouldBlock => {
                if started.elapsed() >= timeout {
                    return Err(error);
                }
                thread::sleep(Duration::from_millis(2));
            }
            Err(error) if error.kind() == ErrorKind::Interrupted => {}
            Err(error) => return Err(error),
        }
    }
    Ok(())
}

fn derp_ticket_node_id(session: &RelayPeerSession) -> String {
    session
        .ticket
        .allowed_derp_node_ids
        .iter()
        .map(|value| value.trim())
        .find(|value| !value.is_empty())
        .unwrap_or("derp")
        .to_string()
}

fn derp_ticket_region_id(session: &RelayPeerSession) -> String {
    session
        .ticket
        .derp_cluster_id
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .unwrap_or("default")
        .to_string()
}

fn derp_ticket_wire(local_node_id: &str, session: &RelayPeerSession) -> serde_json::Value {
    let ticket = &session.ticket;
    serde_json::json!({
        "ticketId": &ticket.ticket_id,
        "peerId": local_node_id,
        "networkId": &ticket.network_id,
        "path": PathKind::DerpTcpTls443.as_str(),
        "regionId": derp_ticket_region_id(session),
        "nodeId": derp_ticket_node_id(session),
        "sessionId": &ticket.session_id,
        "srcNodeId": &ticket.src_node_id,
        "dstNodeId": &ticket.dst_node_id,
        "relayUrl": &ticket.relay_url,
        "sessionKey": &ticket.session_key,
        "allowedDerpNodeIds": &ticket.allowed_derp_node_ids,
        "expiresAt": &ticket.expires_at,
        "signature": &ticket.signature,
    })
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
    mut derp_peers: Vec<DerpPeer>,
    mut direct_udp: Option<DirectUdpTransport>,
    local_node_id: String,
    local_virtual_ip: String,
    max_frame_payload: usize,
    config_hash: u64,
    direct_udp_probe_interval: Duration,
    acl_policies: Vec<PlatformAclPolicy>,
    stats: &mut RelayDataPlaneStats,
    stop: Arc<AtomicBool>,
) {
    let mut seq = 0_u64;
    let mut tun_buffer = vec![0_u8; MAX_PACKET_SIZE];
    let mut relay_buffer = vec![0_u8; MAX_PACKET_SIZE + 512];
    let mut last_stats_flush = Instant::now();
    let mut last_keepalive = Instant::now()
        .checked_sub(RELAY_KEEPALIVE_INTERVAL)
        .unwrap_or_else(Instant::now);
    let mut last_direct_udp_probe = Instant::now()
        .checked_sub(direct_udp_probe_interval)
        .unwrap_or_else(Instant::now);
    persist_relay_stats(stats);
    while !stop.load(Ordering::SeqCst) {
        if last_keepalive.elapsed() >= RELAY_KEEPALIVE_INTERVAL {
            send_relay_keepalives(&peers);
            stats.last_relay_keepalive_at_ms = Some(current_timestamp_ms());
            last_keepalive = Instant::now();
        }
        if last_direct_udp_probe.elapsed() >= direct_udp_probe_interval {
            if let Some(direct_udp) = direct_udp.as_ref() {
                stats.direct_udp_probes_sent = stats
                    .direct_udp_probes_sent
                    .saturating_add(direct_udp.send_probe_packets() as u64);
            }
            last_direct_udp_probe = Instant::now();
        }
        match file.read(&mut tun_buffer) {
            Ok(0) => thread::sleep(DATA_PLANE_IDLE_SLEEP),
            Ok(packet_len) => {
                let packet = &tun_buffer[..packet_len];
                if packet_targets_local_virtual_ip(packet, local_virtual_ip.as_str()) {
                    if let Some(reply) = local_virtual_ip_reply(packet, local_virtual_ip.as_str()) {
                        let _ = write_tun_packet_with_retry(&mut file, &reply);
                    }
                    continue;
                }
                if packet.first().map(|byte| byte >> 4) == Some(4) {
                    stats.last_tun_packet_at_ms = Some(current_timestamp_ms());
                    stats.last_tun_destination = ipv4_destination(packet);
                    stats.last_tun_protocol = ipv4_protocol(packet);
                    stats.last_tun_packet_size = Some(packet.len() as u32);
                    stats.last_tun_peer_node_id = None;
                    stats.last_tun_send_path = None;
                    stats.last_tun_drop_reason = None;
                }
                if packet.first().map(|byte| byte >> 4) == Some(4)
                    && packet.len() <= max_frame_payload
                {
                    if let Some(peer) = relay_peer_for_packet(&peers, packet) {
                        stats.last_tun_peer_node_id = Some(peer.peer_node_id.clone());
                        if !acl_allows_egress_packet(
                            packet,
                            &acl_policies,
                            Some(&acl_peer_for_relay_peer(peer)),
                        ) {
                            stats.last_tun_drop_reason = Some("acl_egress_denied".to_string());
                            continue;
                        }
                        seq = seq.wrapping_add(1);
                        let packet = normalize_ipv4_transport_checksums(packet);
                        record_tun_tcp_packet(stats, &packet);
                        if let Some(frame) = encode_slan_relay_data_frame(seq, config_hash, &packet)
                        {
                            let mut direct_sent = false;
                            if let Some(direct_peer_index) =
                                direct_udp.as_ref().and_then(|transport| {
                                    transport.ready_peer_index_for_packet(&packet)
                                })
                            {
                                match direct_udp
                                    .as_ref()
                                    .expect("direct udp checked")
                                    .send_to_peer(direct_peer_index, &frame)
                                {
                                    Ok(_) => {
                                        direct_sent = true;
                                        stats.last_tun_send_path =
                                            Some(PathKind::DirectUdp.as_str().to_string());
                                        record_direct_tun_packet_sent(stats, peer);
                                        if should_hedge_direct_packet_to_relay(&packet) {
                                            hedge_udp_packet_to_relay(peer, &frame, stats);
                                        }
                                    }
                                    Err(error) => {
                                        stats.last_tun_drop_reason =
                                            Some("direct_udp_send_failed".to_string());
                                        record_relay_send_failure(stats, peer, error.to_string())
                                    }
                                }
                            }
                            if !direct_sent {
                                let mut relay_sent = false;
                                if let Some(payload) = encode_relay_forward(peer, &frame) {
                                    for attempt in 0..relay_send_attempt_count(&packet) {
                                        if attempt > 0 {
                                            thread::sleep(relay_send_attempt_delay(&packet));
                                        }
                                        match peer.socket.send(&payload) {
                                            Ok(_) => {
                                                relay_sent = true;
                                                stats.last_tun_send_path =
                                                    Some(PathKind::RelayUdp.as_str().to_string());
                                                record_relay_tun_packet_sent(stats, peer);
                                            }
                                            Err(error) => record_relay_send_failure(
                                                stats,
                                                peer,
                                                error.to_string(),
                                            ),
                                        }
                                    }
                                } else {
                                    stats.last_tun_drop_reason =
                                        Some("relay_forward_encode_failed".to_string());
                                }
                                if !relay_sent {
                                    if let Some(derp_peer) =
                                        derp_peer_for_packet_mut(&mut derp_peers, &packet)
                                    {
                                        for attempt in 0..relay_send_attempt_count(&packet) {
                                            if attempt > 0 {
                                                thread::sleep(relay_send_attempt_delay(&packet));
                                            }
                                            match send_derp_forward(derp_peer, &frame) {
                                                Ok(_) => {
                                                    stats.last_tun_send_path = Some(
                                                        PathKind::DerpTcpTls443
                                                            .as_str()
                                                            .to_string(),
                                                    );
                                                    stats.last_tun_drop_reason = None;
                                                    record_derp_tun_packet_sent(stats, derp_peer);
                                                }
                                                Err(error) => record_derp_send_failure(
                                                    stats,
                                                    derp_peer,
                                                    error.to_string(),
                                                ),
                                            }
                                        }
                                    }
                                }
                            }
                        } else {
                            stats.last_tun_drop_reason =
                                Some("relay_frame_encode_failed".to_string());
                        }
                    } else if let Some(derp_peer) =
                        derp_peer_for_packet_mut(&mut derp_peers, packet)
                    {
                        stats.last_tun_peer_node_id = Some(derp_peer.peer_node_id.clone());
                        if !acl_allows_egress_packet(
                            packet,
                            &acl_policies,
                            Some(&acl_peer_for_derp_peer(derp_peer)),
                        ) {
                            stats.last_tun_drop_reason = Some("acl_egress_denied".to_string());
                            continue;
                        }
                        seq = seq.wrapping_add(1);
                        let packet = normalize_ipv4_transport_checksums(packet);
                        record_tun_tcp_packet(stats, &packet);
                        if let Some(frame) = encode_slan_relay_data_frame(seq, config_hash, &packet)
                        {
                            for attempt in 0..relay_send_attempt_count(&packet) {
                                if attempt > 0 {
                                    thread::sleep(relay_send_attempt_delay(&packet));
                                }
                                match send_derp_forward(derp_peer, &frame) {
                                    Ok(_) => {
                                        stats.last_tun_send_path =
                                            Some(PathKind::DerpTcpTls443.as_str().to_string());
                                        record_derp_tun_packet_sent(stats, derp_peer);
                                    }
                                    Err(error) => record_derp_send_failure(
                                        stats,
                                        derp_peer,
                                        error.to_string(),
                                    ),
                                }
                            }
                        } else {
                            stats.last_tun_drop_reason =
                                Some("relay_frame_encode_failed".to_string());
                        }
                    } else {
                        if let Some(destination) = ipv4_destination(packet) {
                            if should_ignore_unroutable_destination(&destination) {
                                stats.last_tun_drop_reason =
                                    Some("ignored_unroutable_destination".to_string());
                                continue;
                            }
                            stats.unroutable_tun_packets =
                                stats.unroutable_tun_packets.saturating_add(1);
                            stats.last_unroutable_destination = Some(destination);
                            stats.last_tun_drop_reason = Some("unroutable".to_string());
                        }
                    }
                } else if packet.first().map(|byte| byte >> 4) == Some(4) {
                    stats.oversized_tun_packets = stats.oversized_tun_packets.saturating_add(1);
                    stats.last_oversized_tun_packet_size = Some(packet.len() as u32);
                    stats.last_tun_drop_reason = Some("oversized".to_string());
                }
            }
            Err(error) if error.kind() == ErrorKind::WouldBlock => {
                thread::sleep(DATA_PLANE_IDLE_SLEEP);
            }
            Err(error) if error.kind() == ErrorKind::Interrupted => {}
            Err(_) => break,
        }
        for peer in &peers {
            match peer.socket.recv(&mut relay_buffer) {
                Ok(frame_len) => {
                    let decoded_payload = relay_packet_payload(&relay_buffer[..frame_len]);
                    if decoded_payload.is_none()
                        && relay_control_kind(&relay_buffer[..frame_len]).is_some()
                    {
                        continue;
                    }
                    let frame = decoded_payload
                        .as_deref()
                        .unwrap_or(&relay_buffer[..frame_len]);
                    if let Some(packet) = decode_slan_relay_data_frame(frame) {
                        stats.last_relay_packet_at_ms = Some(current_timestamp_ms());
                        record_relay_tcp_packet(stats, packet);
                        if !acl_allows_ingress_packet(
                            packet,
                            &acl_policies,
                            Some(&acl_peer_for_relay_peer(peer)),
                        ) {
                            stats.last_tun_drop_reason = Some("acl_ingress_denied".to_string());
                            continue;
                        }
                        if let Some(reply) =
                            icmp_echo_reply_for_request(packet, local_virtual_ip.as_str())
                        {
                            seq = seq.wrapping_add(1);
                            if let Some(frame) =
                                encode_slan_relay_data_frame(seq, config_hash, &reply)
                            {
                                if let Some(payload) = encode_relay_forward(peer, &frame) {
                                    match peer.socket.send(&payload) {
                                        Ok(_) => record_relay_tun_packet_sent(stats, peer),
                                        Err(error) => record_relay_send_failure(
                                            stats,
                                            peer,
                                            error.to_string(),
                                        ),
                                    }
                                }
                            }
                            continue;
                        }
                        let packet = normalize_ipv4_transport_checksums(packet);
                        if !acl_allows_ingress_packet(
                            &packet,
                            &acl_policies,
                            Some(&acl_peer_for_relay_peer(peer)),
                        ) {
                            stats.last_tun_drop_reason = Some("acl_ingress_denied".to_string());
                            continue;
                        }
                        match write_tun_packet_with_retry(&mut file, &packet) {
                            Ok(_) => record_relay_packet_received(stats, peer),
                            Err(_) => record_relay_write_failure(stats, peer),
                        }
                    } else {
                        stats.relay_decode_failures = stats.relay_decode_failures.saturating_add(1);
                    }
                }
                Err(error) if error.kind() == ErrorKind::WouldBlock => {}
                Err(error) if error.kind() == ErrorKind::Interrupted => {}
                Err(error) => {
                    stats.relay_receive_failures = stats.relay_receive_failures.saturating_add(1);
                    if let Some(peer_stats) = relay_peer_stats_mut(stats, peer) {
                        peer_stats.receive_failures = peer_stats.receive_failures.saturating_add(1);
                    }
                    stats.last_relay_error = Some(error.to_string());
                }
            }
        }
        for peer in &mut derp_peers {
            match recv_derp_packet(peer) {
                Ok(Some(frame)) => {
                    if let Some(packet) = decode_slan_relay_data_frame(&frame) {
                        stats.last_relay_packet_at_ms = Some(current_timestamp_ms());
                        record_relay_tcp_packet(stats, packet);
                        if !acl_allows_ingress_packet(
                            packet,
                            &acl_policies,
                            Some(&acl_peer_for_derp_peer(peer)),
                        ) {
                            stats.last_tun_drop_reason = Some("acl_ingress_denied".to_string());
                            continue;
                        }
                        let packet = normalize_ipv4_transport_checksums(packet);
                        if !acl_allows_ingress_packet(
                            &packet,
                            &acl_policies,
                            Some(&acl_peer_for_derp_peer(peer)),
                        ) {
                            stats.last_tun_drop_reason = Some("acl_ingress_denied".to_string());
                            continue;
                        }
                        match write_tun_packet_with_retry(&mut file, &packet) {
                            Ok(_) => record_derp_packet_received(stats, peer),
                            Err(_) => record_derp_write_failure(stats, peer),
                        }
                    } else {
                        stats.relay_decode_failures = stats.relay_decode_failures.saturating_add(1);
                    }
                }
                Ok(None) => {}
                Err(error) if error.kind() == ErrorKind::WouldBlock => {}
                Err(error) if error.kind() == ErrorKind::Interrupted => {}
                Err(error) => {
                    stats.relay_receive_failures = stats.relay_receive_failures.saturating_add(1);
                    stats.last_relay_error = Some(format!("DERP receive failed: {error}"));
                }
            }
        }
        if let Some(direct_udp) = direct_udp.as_mut() {
            match direct_udp.recv_from_peer(&mut relay_buffer) {
                Ok(Some(received)) => {
                    let frame_len = received.frame_len;
                    let control_packet = direct_udp_control_packet(&relay_buffer[..frame_len]);
                    if control_packet
                        .as_ref()
                        .is_some_and(|packet| packet.kind == DirectUdpControlKind::Probe)
                    {
                        direct_udp.mark_peer_ready(received.peer_index, received.remote_addr);
                        stats.direct_udp_ready_peer_count = direct_udp.ready_peer_count() as u64;
                        stats.direct_udp_probes_received =
                            stats.direct_udp_probes_received.saturating_add(1);
                        if direct_udp.send_pong_to_peer(received.peer_index) {
                            stats.direct_udp_pongs_sent =
                                stats.direct_udp_pongs_sent.saturating_add(1);
                        }
                        mark_direct_peer_ready(stats, direct_udp, received.peer_index);
                    } else if control_packet
                        .as_ref()
                        .is_some_and(|packet| packet.kind == DirectUdpControlKind::Pong)
                    {
                        direct_udp.mark_peer_ready(received.peer_index, received.remote_addr);
                        stats.direct_udp_ready_peer_count = direct_udp.ready_peer_count() as u64;
                        stats.direct_udp_pongs_received =
                            stats.direct_udp_pongs_received.saturating_add(1);
                        mark_direct_peer_ready(stats, direct_udp, received.peer_index);
                    } else if let Some(packet) =
                        decode_slan_relay_data_frame(&relay_buffer[..frame_len])
                    {
                        direct_udp.mark_peer_ready(received.peer_index, received.remote_addr);
                        stats.direct_udp_ready_peer_count = direct_udp.ready_peer_count() as u64;
                        stats.direct_udp_frames_received =
                            stats.direct_udp_frames_received.saturating_add(1);
                        mark_direct_peer_ready(stats, direct_udp, received.peer_index);
                        stats.last_relay_packet_at_ms = Some(current_timestamp_ms());
                        record_relay_tcp_packet(stats, packet);
                        let packet = normalize_ipv4_transport_checksums(packet);
                        let acl_peer =
                            direct_udp
                                .peers
                                .get(received.peer_index)
                                .map(|peer| PlatformAclPeer {
                                    peer_node_id: Some(peer.peer_node_id.clone()),
                                    peer_virtual_ips: peer.peer_virtual_ips.clone(),
                                });
                        if !acl_allows_ingress_packet(&packet, &acl_policies, acl_peer.as_ref()) {
                            stats.last_tun_drop_reason = Some("acl_ingress_denied".to_string());
                            continue;
                        }
                        if write_tun_packet_with_retry(&mut file, &packet).is_ok() {
                            if let Some(peer) =
                                direct_udp
                                    .peers
                                    .get(received.peer_index)
                                    .and_then(|direct_peer| {
                                        peers.iter().find(|peer| {
                                            peer.peer_node_id == direct_peer.peer_node_id
                                        })
                                    })
                            {
                                record_relay_packet_received(stats, peer);
                            }
                        }
                    }
                }
                Ok(None) => {}
                Err(error) if error.kind() == ErrorKind::WouldBlock => {}
                Err(error) if error.kind() == ErrorKind::Interrupted => {}
                Err(error) => {
                    stats.relay_receive_failures = stats.relay_receive_failures.saturating_add(1);
                    stats.last_relay_error = Some(format!("direct_udp receive failed: {error}"));
                }
            }
        }
        if last_stats_flush.elapsed() >= RELAY_STATS_FLUSH_INTERVAL {
            persist_relay_stats(stats);
            last_stats_flush = Instant::now();
        }
    }
    persist_relay_stats(stats);
    detach_udp_relay_sessions(&peers, local_node_id.as_str());
    detach_derp_relay_sessions(&mut derp_peers);
}

fn encode_relay_forward(peer: &RelayPeer, frame: &[u8]) -> Option<Vec<u8>> {
    serde_json::to_vec(&serde_json::json!({
        "kind": "forward",
        "session_id": peer.session_id,
        "participant_id": peer.local_node_id,
        "payload": base64_encode(frame),
    }))
    .ok()
}

fn send_derp_forward(peer: &mut DerpPeer, frame: &[u8]) -> std::io::Result<()> {
    let payload = serde_json::json!({
        "kind": "send",
        "sessionId": peer.server_session_id,
        "targetPeerId": peer.peer_node_id,
        "payload": base64_encode(frame),
    });
    let mut line = serde_json::to_vec(&payload).map_err(json_io_error)?;
    line.push(b'\n');
    write_all_with_would_block_retry(&mut peer.stream, &line, DERP_WRITE_RETRY_TIMEOUT)
}

fn recv_derp_packet(peer: &mut DerpPeer) -> std::io::Result<Option<Vec<u8>>> {
    if let Some(line) = take_derp_line(&mut peer.read_buffer) {
        return parse_derp_packet_line(&line);
    }
    let mut chunk = [0_u8; 4096];
    loop {
        match peer.reader.read(&mut chunk) {
            Ok(0) => return Ok(None),
            Ok(len) => {
                peer.read_buffer.extend_from_slice(&chunk[..len]);
                if let Some(line) = take_derp_line(&mut peer.read_buffer) {
                    return parse_derp_packet_line(&line);
                }
            }
            Err(error) if error.kind() == ErrorKind::WouldBlock => return Ok(None),
            Err(error) if error.kind() == ErrorKind::Interrupted => {}
            Err(error) => return Err(error),
        }
    }
}

fn take_derp_line(buffer: &mut Vec<u8>) -> Option<Vec<u8>> {
    let newline = buffer.iter().position(|byte| *byte == b'\n')?;
    let mut line = buffer.drain(..=newline).collect::<Vec<_>>();
    while matches!(line.last(), Some(b'\n' | b'\r')) {
        line.pop();
    }
    Some(line)
}

fn parse_derp_packet_line(line: &[u8]) -> std::io::Result<Option<Vec<u8>>> {
    let value: serde_json::Value = serde_json::from_slice(line).map_err(json_io_error)?;
    match value.get("kind").and_then(serde_json::Value::as_str) {
        Some("recv") => Ok(value
            .get("payload")
            .and_then(serde_json::Value::as_str)
            .and_then(base64_decode)),
        Some("sent") | Some("connected") => Ok(None),
        Some("error") => Err(std::io::Error::new(
            ErrorKind::Other,
            value
                .get("error")
                .and_then(|error| error.get("message"))
                .and_then(serde_json::Value::as_str)
                .unwrap_or("Linux DERP error")
                .to_string(),
        )),
        _ => Ok(None),
    }
}

fn json_io_error(error: serde_json::Error) -> std::io::Error {
    std::io::Error::new(ErrorKind::InvalidData, error)
}

fn write_tun_packet_with_retry(file: &mut File, packet: &[u8]) -> std::io::Result<()> {
    let deadline = Instant::now() + Duration::from_secs(1);
    let mut offset = 0_usize;
    while offset < packet.len() {
        match file.write(&packet[offset..]) {
            Ok(0) => {
                if Instant::now() >= deadline {
                    return Err(std::io::Error::new(
                        ErrorKind::WriteZero,
                        "linux tun write made no progress",
                    ));
                }
                thread::sleep(Duration::from_millis(2));
            }
            Ok(written) => offset += written,
            Err(error) if error.kind() == ErrorKind::WouldBlock => {
                if Instant::now() >= deadline {
                    return Err(error);
                }
                thread::sleep(Duration::from_millis(1));
            }
            Err(error) if error.kind() == ErrorKind::Interrupted => {}
            Err(error) => return Err(error),
        }
    }
    Ok(())
}

fn send_relay_keepalives(peers: &[RelayPeer]) {
    for peer in peers {
        let Ok(payload) = serde_json::to_vec(&serde_json::json!({
            "kind": "ping",
            "session_id": peer.session_id,
            "participant_id": peer.local_node_id,
        })) else {
            continue;
        };
        let _ = peer.socket.send(&payload);
    }
}

fn relay_packet_payload(frame: &[u8]) -> Option<Vec<u8>> {
    let value = serde_json::from_slice::<serde_json::Value>(frame).ok()?;
    if value.get("kind").and_then(serde_json::Value::as_str) != Some("packet") {
        return None;
    }
    base64_decode(value.get("payload")?.as_str()?)
}

fn relay_control_kind(frame: &[u8]) -> Option<String> {
    let value = serde_json::from_slice::<serde_json::Value>(frame).ok()?;
    match value.get("kind").and_then(serde_json::Value::as_str)? {
        "pong" | "attached" | "forwarded" | "detached" | "error" => value
            .get("kind")
            .and_then(serde_json::Value::as_str)
            .map(str::to_string),
        _ => None,
    }
}

fn relay_data_plane_stats_from_config(
    config: &RelayDataPlaneConfig,
    peers: &[RelayPeer],
    derp_peers: &[DerpPeer],
) -> RelayDataPlaneStats {
    let attached_count = peers.len() + derp_peers.len();
    RelayDataPlaneStats {
        relay_address: config.relay_address.clone(),
        relay_transport: Some(config.transport.clone()),
        active_path: Some(config.transport.clone()),
        requested_relay_session_count: config.sessions.len() as u32,
        relay_session_count: attached_count as u32,
        attached_peer_session_count: attached_count as u32,
        attached_transport_count: attached_count as u32,
        ticket_expires_at: earliest_relay_ticket_expires_at(&config.sessions),
        peers: peers
            .iter()
            .map(|peer| RelayPeerStats {
                peer_node_id: peer.peer_node_id.clone(),
                session_id: peer.session_id.clone(),
                peer_virtual_ips: peer.peer_virtual_ips.clone(),
                attached: true,
                ..RelayPeerStats::default()
            })
            .chain(derp_peers.iter().map(|peer| RelayPeerStats {
                peer_node_id: peer.peer_node_id.clone(),
                session_id: peer.session_id.clone(),
                peer_virtual_ips: peer.peer_virtual_ips.clone(),
                attached: true,
                last_send_path: Some(PathKind::DerpTcpTls443.as_str().to_string()),
                ..RelayPeerStats::default()
            }))
            .collect(),
        relay_mtu: config.relay_mtu,
        max_frame_payload: config.max_frame_payload,
        started_at_ms: current_timestamp_ms(),
        ..RelayDataPlaneStats::default()
    }
}

fn record_relay_tun_packet_sent(stats: &mut RelayDataPlaneStats, peer: &RelayPeer) {
    stats.tun_packets_sent = stats.tun_packets_sent.saturating_add(1);
    if let Some(peer_stats) = relay_peer_stats_mut(stats, peer) {
        peer_stats.tun_packets_sent = peer_stats.tun_packets_sent.saturating_add(1);
        peer_stats.last_send_path = Some(PathKind::RelayUdp.as_str().to_string());
    }
}

fn record_direct_tun_packet_sent(stats: &mut RelayDataPlaneStats, peer: &RelayPeer) {
    stats.tun_packets_sent = stats.tun_packets_sent.saturating_add(1);
    stats.direct_udp_frames_sent = stats.direct_udp_frames_sent.saturating_add(1);
    stats.active_path = Some(PathKind::DirectUdp.as_str().to_string());
    if let Some(peer_stats) = relay_peer_stats_mut(stats, peer) {
        peer_stats.tun_packets_sent = peer_stats.tun_packets_sent.saturating_add(1);
        if peer_stats.last_send_path.as_deref() != Some(PathKind::DirectUdp.as_str()) {
            peer_stats.path_upgrades = peer_stats.path_upgrades.saturating_add(1);
            peer_stats.last_path_change = Some(format!(
                "{} -> {} after direct udp ready",
                peer_stats
                    .last_send_path
                    .as_deref()
                    .unwrap_or(PathKind::RelayUdp.as_str()),
                PathKind::DirectUdp.as_str()
            ));
        }
        peer_stats.last_send_path = Some(PathKind::DirectUdp.as_str().to_string());
    }
}

fn mark_direct_peer_ready(
    stats: &mut RelayDataPlaneStats,
    direct_udp: &DirectUdpTransport,
    peer_index: usize,
) {
    let Some(peer) = direct_udp.peers.get(peer_index) else {
        return;
    };
    stats.active_path = Some(PathKind::DirectUdp.as_str().to_string());
    if let Some(peer_stats) = relay_peer_stats_mut_by_node_id(stats, peer.peer_node_id.as_str()) {
        if peer_stats.last_send_path.as_deref() != Some(PathKind::DirectUdp.as_str()) {
            peer_stats.path_upgrades = peer_stats.path_upgrades.saturating_add(1);
            peer_stats.last_path_change = Some(format!(
                "{} -> {} after probe success",
                peer_stats
                    .last_send_path
                    .as_deref()
                    .unwrap_or(PathKind::RelayUdp.as_str()),
                PathKind::DirectUdp.as_str()
            ));
        }
        peer_stats.last_send_path = Some(PathKind::DirectUdp.as_str().to_string());
    }
}

fn record_relay_packet_received(stats: &mut RelayDataPlaneStats, peer: &RelayPeer) {
    stats.relay_packets_received = stats.relay_packets_received.saturating_add(1);
    if let Some(peer_stats) = relay_peer_stats_mut(stats, peer) {
        peer_stats.relay_packets_received = peer_stats.relay_packets_received.saturating_add(1);
    }
}

fn record_derp_tun_packet_sent(stats: &mut RelayDataPlaneStats, peer: &DerpPeer) {
    stats.tun_packets_sent = stats.tun_packets_sent.saturating_add(1);
    stats.active_path = Some(PathKind::DerpTcpTls443.as_str().to_string());
    if let Some(peer_stats) = relay_peer_stats_mut_by_session_id(stats, peer.session_id.as_str()) {
        peer_stats.tun_packets_sent = peer_stats.tun_packets_sent.saturating_add(1);
        peer_stats.last_send_path = Some(PathKind::DerpTcpTls443.as_str().to_string());
    }
}

fn record_derp_packet_received(stats: &mut RelayDataPlaneStats, peer: &DerpPeer) {
    stats.relay_packets_received = stats.relay_packets_received.saturating_add(1);
    if let Some(peer_stats) = relay_peer_stats_mut_by_session_id(stats, peer.session_id.as_str()) {
        peer_stats.relay_packets_received = peer_stats.relay_packets_received.saturating_add(1);
    }
}

fn record_tun_tcp_packet(stats: &mut RelayDataPlaneStats, packet: &[u8]) {
    let Some(flags) = ipv4_tcp_flags(packet) else {
        return;
    };
    stats.tun_tcp_packets_sent = stats.tun_tcp_packets_sent.saturating_add(1);
    if flags & 0x12 == 0x12 {
        stats.tun_tcp_syn_ack_sent = stats.tun_tcp_syn_ack_sent.saturating_add(1);
    }
    if flags & 0x04 != 0 {
        stats.tun_tcp_rst_sent = stats.tun_tcp_rst_sent.saturating_add(1);
    }
    if ipv4_transport_checksum_valid(packet) == Some(false) {
        stats.tun_tcp_checksum_invalid = stats.tun_tcp_checksum_invalid.saturating_add(1);
    }
}

fn relay_send_attempt_count(packet: &[u8]) -> usize {
    if is_ipv4_udp_packet(packet) {
        return 3;
    }
    ipv4_tcp_flags(packet)
        .map(|flags| {
            if flags & 0x12 == 0x02 {
                4
            } else if flags & 0x0b != 0 || ipv4_tcp_payload_len(packet).unwrap_or(0) > 0 {
                2
            } else {
                1
            }
        })
        .unwrap_or(1)
}

fn should_hedge_direct_packet_to_relay(packet: &[u8]) -> bool {
    if is_ipv4_udp_packet(packet) {
        return true;
    }
    let Some(flags) = ipv4_tcp_flags(packet) else {
        return false;
    };
    flags & 0x12 == 0x02 || flags & 0x0b != 0 || ipv4_tcp_payload_len(packet).unwrap_or(0) > 0
}

fn relay_send_attempt_delay(packet: &[u8]) -> Duration {
    ipv4_tcp_flags(packet)
        .map(|flags| {
            if flags & 0x12 == 0x02 {
                Duration::from_millis(30)
            } else {
                DATA_PLANE_IDLE_SLEEP
            }
        })
        .unwrap_or(DATA_PLANE_IDLE_SLEEP)
}

fn record_relay_tcp_packet(stats: &mut RelayDataPlaneStats, packet: &[u8]) {
    let Some(flags) = ipv4_tcp_flags(packet) else {
        return;
    };
    stats.relay_tcp_packets_received = stats.relay_tcp_packets_received.saturating_add(1);
    if flags & 0x02 != 0 {
        stats.relay_tcp_syn_received = stats.relay_tcp_syn_received.saturating_add(1);
    }
    if flags & 0x04 != 0 {
        stats.relay_tcp_rst_received = stats.relay_tcp_rst_received.saturating_add(1);
    }
    if ipv4_transport_checksum_valid(packet) == Some(false) {
        stats.relay_tcp_checksum_invalid = stats.relay_tcp_checksum_invalid.saturating_add(1);
    }
}

fn ipv4_tcp_flags(packet: &[u8]) -> Option<u8> {
    if packet.len() < 20 || packet[0] >> 4 != 4 || packet.get(9).copied() != Some(6) {
        return None;
    }
    let ihl = usize::from(packet[0] & 0x0f) * 4;
    if ihl < 20 || packet.len() < ihl + 14 {
        return None;
    }
    Some(packet[ihl + 13])
}

fn ipv4_tcp_payload_len(packet: &[u8]) -> Option<usize> {
    if packet.len() < 20 || packet[0] >> 4 != 4 || packet.get(9).copied() != Some(6) {
        return None;
    }
    let ihl = usize::from(packet[0] & 0x0f) * 4;
    if ihl < 20 || packet.len() < ihl + 20 {
        return None;
    }
    let total_len = usize::from(u16::from_be_bytes([packet[2], packet[3]]));
    if total_len < ihl + 20 || total_len > packet.len() {
        return None;
    }
    let data_offset = usize::from(packet[ihl + 12] >> 4) * 4;
    if data_offset < 20 || total_len < ihl + data_offset {
        return None;
    }
    Some(total_len - ihl - data_offset)
}

fn record_relay_send_failure(stats: &mut RelayDataPlaneStats, peer: &RelayPeer, error: String) {
    stats.relay_send_failures = stats.relay_send_failures.saturating_add(1);
    stats.last_relay_error = Some(error.clone());
    if let Some(peer_stats) = relay_peer_stats_mut(stats, peer) {
        peer_stats.send_failures = peer_stats.send_failures.saturating_add(1);
        peer_stats.last_relay_error = Some(error);
        peer_stats.last_send_path = Some(PathKind::RelayUdp.as_str().to_string());
    }
}

fn record_derp_send_failure(stats: &mut RelayDataPlaneStats, peer: &DerpPeer, error: String) {
    stats.relay_send_failures = stats.relay_send_failures.saturating_add(1);
    stats.last_relay_error = Some(error.clone());
    if let Some(peer_stats) = relay_peer_stats_mut_by_session_id(stats, peer.session_id.as_str()) {
        peer_stats.send_failures = peer_stats.send_failures.saturating_add(1);
        peer_stats.last_relay_error = Some(error);
        peer_stats.last_send_path = Some(PathKind::DerpTcpTls443.as_str().to_string());
    }
}

fn record_relay_write_failure(stats: &mut RelayDataPlaneStats, peer: &RelayPeer) {
    stats.wintun_write_failures = stats.wintun_write_failures.saturating_add(1);
    if let Some(peer_stats) = relay_peer_stats_mut(stats, peer) {
        peer_stats.wintun_write_failures = peer_stats.wintun_write_failures.saturating_add(1);
    }
}

fn record_derp_write_failure(stats: &mut RelayDataPlaneStats, peer: &DerpPeer) {
    stats.wintun_write_failures = stats.wintun_write_failures.saturating_add(1);
    if let Some(peer_stats) = relay_peer_stats_mut_by_session_id(stats, peer.session_id.as_str()) {
        peer_stats.wintun_write_failures = peer_stats.wintun_write_failures.saturating_add(1);
    }
}

fn relay_peer_stats_mut<'a>(
    stats: &'a mut RelayDataPlaneStats,
    peer: &RelayPeer,
) -> Option<&'a mut RelayPeerStats> {
    relay_peer_stats_mut_by_session_id(stats, peer.session_id.as_str())
}

fn relay_peer_stats_mut_by_session_id<'a>(
    stats: &'a mut RelayDataPlaneStats,
    session_id: &str,
) -> Option<&'a mut RelayPeerStats> {
    stats
        .peers
        .iter_mut()
        .find(|stats| stats.session_id == session_id)
}

fn relay_peer_stats_mut_by_node_id<'a>(
    stats: &'a mut RelayDataPlaneStats,
    peer_node_id: &str,
) -> Option<&'a mut RelayPeerStats> {
    stats
        .peers
        .iter_mut()
        .find(|stats| stats.peer_node_id == peer_node_id)
}

fn persist_relay_stats(stats: &mut RelayDataPlaneStats) {
    stats.updated_at_ms = current_timestamp_ms();
    let path = relay_stats_file_path();
    let Some(parent) = path.parent() else {
        return;
    };
    if fs::create_dir_all(parent).is_err() {
        return;
    }
    let Ok(payload) = serde_json::to_vec_pretty(stats) else {
        return;
    };
    let _ = fs::write(path, payload);
}

fn relay_stats_file_path() -> PathBuf {
    app_data_dir()
        .join("SLAN")
        .join("client-v2-relay-stats.json")
}

fn app_data_dir() -> PathBuf {
    std::env::var_os("SLAN_STATE_DIR")
        .map(PathBuf::from)
        .unwrap_or_else(|| PathBuf::from("/var/lib"))
}

fn current_timestamp_ms() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|duration| duration.as_millis() as u64)
        .unwrap_or_default()
}

fn earliest_relay_ticket_expires_at(sessions: &[RelayPeerSession]) -> Option<String> {
    sessions
        .iter()
        .map(|session| session.ticket.expires_at.trim())
        .filter(|value| !value.is_empty())
        .min()
        .map(str::to_string)
}

fn relay_peer_for_packet<'a>(peers: &'a [RelayPeer], packet: &[u8]) -> Option<&'a RelayPeer> {
    let destination = ipv4_destination(packet)?;
    peers.iter().find(|peer| {
        peer.peer_virtual_ips
            .iter()
            .any(|ip| normalize_virtual_ip(ip) == destination)
    })
}

fn acl_peer_for_relay_peer(peer: &RelayPeer) -> PlatformAclPeer {
    PlatformAclPeer {
        peer_node_id: Some(peer.peer_node_id.clone()),
        peer_virtual_ips: peer.peer_virtual_ips.clone(),
    }
}

fn acl_peer_for_derp_peer(peer: &DerpPeer) -> PlatformAclPeer {
    PlatformAclPeer {
        peer_node_id: Some(peer.peer_node_id.clone()),
        peer_virtual_ips: peer.peer_virtual_ips.clone(),
    }
}

fn run_local_data_plane(mut file: File, local_virtual_ip: String, stop: Arc<AtomicBool>) {
    let mut tun_buffer = vec![0_u8; MAX_PACKET_SIZE];
    while !stop.load(Ordering::SeqCst) {
        match file.read(&mut tun_buffer) {
            Ok(0) => thread::sleep(DATA_PLANE_IDLE_SLEEP),
            Ok(packet_len) => {
                let packet = &tun_buffer[..packet_len];
                if let Some(reply) = local_virtual_ip_reply(packet, local_virtual_ip.as_str()) {
                    let _ = write_tun_packet_with_retry(&mut file, &reply);
                }
            }
            Err(error) if error.kind() == ErrorKind::WouldBlock => {
                thread::sleep(DATA_PLANE_IDLE_SLEEP);
            }
            Err(error) if error.kind() == ErrorKind::Interrupted => {}
            Err(_) => break,
        }
    }
}

fn derp_peer_for_packet_mut<'a>(
    peers: &'a mut [DerpPeer],
    packet: &[u8],
) -> Option<&'a mut DerpPeer> {
    let destination = ipv4_destination(packet)?;
    peers.iter_mut().find(|peer| {
        peer.peer_virtual_ips
            .iter()
            .any(|ip| normalize_virtual_ip(ip) == destination)
    })
}

fn packet_targets_local_virtual_ip(packet: &[u8], local_virtual_ip: &str) -> bool {
    ipv4_destination(packet)
        .map(|destination| destination == normalize_virtual_ip(local_virtual_ip))
        .unwrap_or(false)
}

fn local_virtual_ip_reply(packet: &[u8], local_virtual_ip: &str) -> Option<Vec<u8>> {
    if packet_targets_local_virtual_ip(packet, local_virtual_ip) {
        icmp_echo_reply_for_request(packet, local_virtual_ip)
    } else {
        None
    }
}

fn should_ignore_unroutable_destination(destination: &str) -> bool {
    let mut parts = destination
        .split('.')
        .filter_map(|part| part.parse::<u8>().ok());
    let Some(first) = parts.next() else {
        return false;
    };
    first >= 224 || destination == "255.255.255.255"
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

fn ipv4_protocol(packet: &[u8]) -> Option<u8> {
    if packet.len() < 20 || packet[0] >> 4 != 4 {
        return None;
    }
    packet.get(9).copied()
}

fn is_ipv4_udp_packet(packet: &[u8]) -> bool {
    ipv4_protocol(packet) == Some(17)
}

fn hedge_udp_packet_to_relay(peer: &RelayPeer, frame: &[u8], stats: &mut RelayDataPlaneStats) {
    let Some(payload) = encode_relay_forward(peer, frame) else {
        return;
    };
    match peer.socket.send(&payload) {
        Ok(_) => {
            stats.last_tun_send_path = Some("direct_udp+relay_udp".to_string());
            record_relay_tun_packet_sent(stats, peer);
        }
        Err(error) => record_relay_send_failure(stats, peer, error.to_string()),
    }
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

fn detach_derp_relay_sessions(peers: &mut [DerpPeer]) {
    for peer in peers {
        let payload = serde_json::json!({
            "kind": "disconnect",
            "sessionId": peer.server_session_id,
            "peerId": peer.local_node_id,
        });
        let _ = write_json_line(&mut peer.stream, &payload);
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
            acl_policies: vec![PlatformAclPolicy {
                network_id: "network-1".to_string(),
                default_policy: "allow".to_string(),
                rules: Vec::new(),
            }],
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
    fn linux_local_virtual_ip_replies_to_icmp_echo() {
        let request = icmp_echo_request("10.0.0.9", "10.0.0.2");
        let reply = local_virtual_ip_reply(&request, "10.0.0.2/32").unwrap();

        assert_eq!(&reply[12..16], &[10, 0, 0, 2]);
        assert_eq!(&reply[16..20], &[10, 0, 0, 9]);
        assert_eq!(reply[20], 0);
        assert!(local_virtual_ip_reply(&request, "10.0.0.3/32").is_none());
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
        let runtime = runtime().lock().expect("linux runtime mutex poisoned");
        let relay = runtime
            .relay_config
            .as_ref()
            .expect("mock runtime should keep relay config");
        assert_eq!(relay.acl_policies.len(), 1);
        assert_eq!(relay.acl_policies[0].network_id, "network-1");
        drop(runtime);

        platform.disable_network().unwrap();
        std::env::remove_var("SLAN_LINUX_NETWORK_MOCK");
        reset_runtime();
    }

    fn ipv4_packet(src: &str, dst: &str) -> Vec<u8> {
        let mut packet = vec![0_u8; 20];
        packet[0] = 0x45;
        for (index, part) in src.split('.').enumerate().take(4) {
            packet[12 + index] = part.parse::<u8>().unwrap();
        }
        for (index, part) in dst.split('.').enumerate().take(4) {
            packet[16 + index] = part.parse::<u8>().unwrap();
        }
        packet
    }

    fn icmp_echo_request(src: &str, dst: &str) -> Vec<u8> {
        let mut packet = ipv4_packet(src, dst);
        packet.resize(32, 0);
        packet[2..4].copy_from_slice(&(32_u16).to_be_bytes());
        packet[8] = 64;
        packet[9] = 1;
        packet[20] = 8;
        packet[24..26].copy_from_slice(&7_u16.to_be_bytes());
        packet[26..28].copy_from_slice(&9_u16.to_be_bytes());
        packet[28..32].copy_from_slice(b"slan");
        packet
    }
}
