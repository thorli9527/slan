//! macOS platform bridge for SLAN mesh networking.

use std::{
    env,
    ffi::CString,
    fs::{self, File},
    io::{ErrorKind, Read, Write},
    mem,
    net::{Ipv4Addr, SocketAddr, ToSocketAddrs, UdpSocket},
    os::fd::{FromRawFd, RawFd},
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
    icmp_echo_reply_for_request, ipv4_transport_checksum_valid, normalize_ipv4_transport_checksums,
    relay_frame::{
        base64_decode, base64_encode, decode_slan_relay_data_frame, encode_slan_relay_data_frame,
        stable_hash64,
    },
    NetworkRuntimeState, PathCandidate, PathKind, PathState, PeerPathRuntime,
    PlatformDiagnosticCheck, PlatformNetwork, PlatformNetworkDiagnostics, RelayDataPlaneConfig,
    RelayPeerSession, RouteSpec,
};
use serde::Serialize;

use crate::direct_udp::{
    direct_udp_control_packet, direct_udp_probe_interval_from_ms, DirectUdpControlKind,
    DirectUdpTransport,
};

const UTUN_CONTROL_NAME: &str = "com.apple.net.utun_control";
const UTUN_OPT_IFNAME: libc::c_int = 2;
const DEFAULT_UTUN_MTU: u16 = 1280;
const MAX_PACKET_SIZE: usize = 4096;
const UTUN_HEADER_LEN: usize = 4;
const AF_INET_HEADER: [u8; UTUN_HEADER_LEN] = [0, 0, 0, libc::AF_INET as u8];
const MOCK_INTERFACE_NAME: &str = "utun-mock";
const HOST_INTERFACE_PREFIX_LEN: u8 = 32;
const RELAY_STATS_FLUSH_INTERVAL: Duration = Duration::from_secs(10);
const RELAY_KEEPALIVE_INTERVAL: Duration = Duration::from_secs(30);

#[derive(Debug, Clone, Default)]
pub struct MacosPlatformNetwork;

#[derive(Debug, Default)]
struct MacosRuntime {
    interface_name: Option<String>,
    virtual_ip: Option<String>,
    prefix_len: Option<u8>,
    dns_servers: Vec<String>,
    routes: Vec<RouteSpec>,
    relay_config: Option<RelayDataPlaneConfig>,
    utun: Option<UtunRuntime>,
}

#[derive(Debug)]
struct UtunRuntime {
    interface_name: String,
    file: Option<File>,
    stop: Arc<AtomicBool>,
    handle: Option<JoinHandle<()>>,
}

impl Drop for UtunRuntime {
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
    peer_node_id: String,
    local_node_id: String,
    peer_virtual_ips: Vec<String>,
    socket: UdpSocket,
}

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
    last_relay_error: Option<String>,
    relay_send_failures: u64,
    relay_receive_failures: u64,
    unroutable_tun_packets: u64,
    last_unroutable_destination: Option<String>,
    oversized_tun_packets: u64,
    last_oversized_tun_packet_size: Option<u32>,
    wintun_write_failures: u64,
    started_at_ms: u64,
    last_tun_packet_at_ms: Option<u64>,
    last_relay_packet_at_ms: Option<u64>,
    last_relay_keepalive_at_ms: Option<u64>,
    updated_at_ms: u64,
}

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

static MACOS_RUNTIME: OnceLock<Mutex<MacosRuntime>> = OnceLock::new();

fn runtime() -> &'static Mutex<MacosRuntime> {
    MACOS_RUNTIME.get_or_init(|| Mutex::new(MacosRuntime::default()))
}

impl PlatformNetwork for MacosPlatformNetwork {
    fn install_adapter(&self) -> Result<()> {
        let mut runtime = runtime()
            .lock()
            .map_err(|_| anyhow!("macos network runtime lock poisoned"))?;
        if macos_network_mock_enabled() {
            ensure_mock_runtime(&mut runtime);
            return Ok(());
        }
        ensure_utun_runtime(&mut runtime)
    }

    fn configure_ip(&self, virtual_ip: &str, _prefix_len: u8) -> Result<()> {
        let virtual_ip = virtual_ip.trim();
        if virtual_ip.is_empty() {
            bail!("macos virtual IP is empty");
        }
        let virtual_addr = parse_virtual_ipv4(virtual_ip)
            .with_context(|| format!("parse macos virtual IP {virtual_ip}"))?;
        let virtual_ip = virtual_addr.to_string();
        let mut runtime = runtime()
            .lock()
            .map_err(|_| anyhow!("macos network runtime lock poisoned"))?;
        if macos_network_mock_enabled() {
            ensure_mock_runtime(&mut runtime);
            runtime.virtual_ip = Some(virtual_ip);
            runtime.prefix_len = Some(HOST_INTERFACE_PREFIX_LEN);
            return Ok(());
        }
        ensure_utun_runtime(&mut runtime)?;
        let interface_name = runtime
            .interface_name
            .clone()
            .ok_or_else(|| anyhow!("macos utun interface is not ready"))?;
        configure_utun_ip(&interface_name, virtual_addr, HOST_INTERFACE_PREFIX_LEN)?;
        runtime.virtual_ip = Some(virtual_ip);
        runtime.prefix_len = Some(HOST_INTERFACE_PREFIX_LEN);
        Ok(())
    }

    fn configure_routes(&self, routes: &[RouteSpec]) -> Result<()> {
        let mut runtime = runtime()
            .lock()
            .map_err(|_| anyhow!("macos network runtime lock poisoned"))?;
        if macos_network_mock_enabled() {
            ensure_mock_runtime(&mut runtime);
            runtime.routes = routes.to_vec();
            return Ok(());
        }
        ensure_utun_runtime(&mut runtime)?;
        let interface_name = runtime
            .interface_name
            .clone()
            .ok_or_else(|| anyhow!("macos utun interface is not ready"))?;
        for route in routes {
            add_utun_route(&interface_name, route)?;
        }
        runtime.routes = routes.to_vec();
        Ok(())
    }

    fn configure_dns(&self, dns_servers: &[String]) -> Result<()> {
        let mut runtime = runtime()
            .lock()
            .map_err(|_| anyhow!("macos network runtime lock poisoned"))?;
        if macos_network_mock_enabled() {
            ensure_mock_runtime(&mut runtime);
            runtime.dns_servers = dns_servers
                .iter()
                .map(|value| value.trim())
                .filter(|value| !value.is_empty())
                .map(str::to_string)
                .collect();
            return Ok(());
        }
        ensure_utun_runtime(&mut runtime)?;
        let interface_name = runtime
            .interface_name
            .clone()
            .ok_or_else(|| anyhow!("macos utun interface is not ready"))?;
        runtime.dns_servers = dns_servers
            .iter()
            .map(|value| value.trim())
            .filter(|value| !value.is_empty())
            .map(str::to_string)
            .collect();
        configure_utun_dns(&interface_name, &runtime.dns_servers)?;
        Ok(())
    }

    fn configure_relay(&self, relay_config: Option<&RelayDataPlaneConfig>) -> Result<()> {
        let mut runtime = runtime()
            .lock()
            .map_err(|_| anyhow!("macos network runtime lock poisoned"))?;
        runtime.relay_config = relay_config.cloned();
        if macos_network_mock_enabled() {
            ensure_mock_runtime(&mut runtime);
            return Ok(());
        }
        restart_data_plane(&mut runtime)
    }

    fn disable_network(&self) -> Result<()> {
        let mut runtime = runtime()
            .lock()
            .map_err(|_| anyhow!("macos network runtime lock poisoned"))?;
        if macos_network_mock_enabled() {
            runtime.interface_name = None;
            runtime.virtual_ip = None;
            runtime.prefix_len = None;
            runtime.dns_servers.clear();
            runtime.routes.clear();
            runtime.relay_config = None;
            runtime.utun = None;
            return Ok(());
        }
        runtime.utun = None;
        let interface_name = runtime.interface_name.take();
        let routes = mem::take(&mut runtime.routes);
        if let Some(interface_name) = interface_name.as_deref() {
            let _ = clear_utun_dns(interface_name);
            for route in routes.iter().rev() {
                let _ = delete_utun_route(interface_name, route);
            }
            let _ = run_command("/sbin/ifconfig", &[interface_name, "down"]);
        }
        runtime.virtual_ip = None;
        runtime.prefix_len = None;
        runtime.dns_servers.clear();
        runtime.relay_config = None;
        Ok(())
    }

    fn read_runtime_state(&self) -> Result<NetworkRuntimeState> {
        let runtime = runtime()
            .lock()
            .map_err(|_| anyhow!("macos network runtime lock poisoned"))?;
        let network_enabled = if macos_network_mock_enabled() {
            runtime.virtual_ip.is_some()
        } else {
            runtime.utun.is_some() && runtime.virtual_ip.is_some()
        };
        Ok(NetworkRuntimeState {
            adapter_present: runtime.interface_name.is_some(),
            network_enabled,
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

    fn diagnostics(&self) -> Result<PlatformNetworkDiagnostics> {
        let runtime = runtime()
            .lock()
            .map_err(|_| anyhow!("macos network runtime lock poisoned"))?;
        let relay = runtime.relay_config.as_ref();
        Ok(PlatformNetworkDiagnostics {
            platform: "macos".to_string(),
            adapter_present: runtime.interface_name.is_some(),
            adapter_name: runtime.interface_name.clone(),
            admin_status: Some(
                if runtime.utun.is_some() {
                    "enabled"
                } else {
                    "disabled"
                }
                .to_string(),
            ),
            interface_index: runtime.interface_name.as_deref().and_then(interface_index),
            virtual_ip: runtime.virtual_ip.clone(),
            mtu: relay
                .and_then(|config| config.relay_mtu)
                .or(Some(DEFAULT_UTUN_MTU))
                .map(u32::from),
            mss: None,
            dns_servers: runtime.dns_servers.clone(),
            routes: runtime
                .routes
                .iter()
                .map(|route| route.destination.clone())
                .collect(),
            checks: macos_diagnostic_checks(&runtime),
        })
    }
}

fn macos_network_mock_enabled() -> bool {
    matches!(
        env::var("SLAN_MACOS_NETWORK_MOCK").ok().as_deref(),
        Some("1" | "true" | "TRUE" | "yes" | "YES")
    )
}

fn ensure_mock_runtime(runtime: &mut MacosRuntime) {
    if runtime.interface_name.is_none() {
        runtime.interface_name = Some(MOCK_INTERFACE_NAME.to_string());
    }
}

fn ensure_utun_runtime(runtime: &mut MacosRuntime) -> Result<()> {
    if runtime.interface_name.is_some() && runtime.utun.is_some() {
        return Ok(());
    }
    let utun = open_utun().context("open macos utun interface")?;
    runtime.interface_name = Some(utun.interface_name.clone());
    runtime.utun = Some(utun);
    Ok(())
}

fn restart_data_plane(runtime: &mut MacosRuntime) -> Result<()> {
    let config = runtime.relay_config.clone().filter(|config| {
        config.enabled
            && config.transport.eq_ignore_ascii_case("udp")
            && !config.relay_address.trim().is_empty()
            && !config.sessions.is_empty()
    });
    let old_utun = runtime.utun.take();
    drop(old_utun);
    let utun = open_utun().context("reopen macos utun interface for data plane")?;
    let interface_name = utun.interface_name.clone();
    runtime.interface_name = Some(interface_name.clone());
    if let (Some(ip), Some(prefix_len)) = (&runtime.virtual_ip, runtime.prefix_len) {
        configure_utun_ip(&interface_name, ip.parse()?, prefix_len)?;
    }
    configure_utun_dns(&interface_name, &runtime.dns_servers)?;
    for route in &runtime.routes {
        let _ = add_utun_route(&interface_name, route);
    }
    runtime.utun = Some(if let Some(config) = config {
        start_udp_data_plane(utun, config, runtime.virtual_ip.clone().unwrap_or_default())?
    } else {
        utun
    });
    Ok(())
}

fn start_udp_data_plane(
    mut utun: UtunRuntime,
    config: RelayDataPlaneConfig,
    local_virtual_ip: String,
) -> Result<UtunRuntime> {
    let file = utun
        .file
        .take()
        .ok_or_else(|| anyhow!("macos utun file descriptor is not ready"))?;
    let peers = attach_udp_relay_sessions(&config)?;
    let direct_udp = DirectUdpTransport::attach(config.local_node_id.as_str(), &config.peer_paths);
    let config_hash = stable_hash64(&serde_json::to_string(&config)?);
    let max_frame_payload = usize::from(config.max_frame_payload.unwrap_or(1200).clamp(512, 1400));
    let direct_udp_probe_interval =
        direct_udp_probe_interval_from_ms(config.path_policy.probe_interval_ms);
    let mut stats = relay_data_plane_stats_from_config(&config, &peers);
    let stop = Arc::new(AtomicBool::new(false));
    let thread_stop = Arc::clone(&stop);
    let handle = thread::spawn(move || {
        run_udp_data_plane(
            file,
            peers,
            direct_udp,
            config.local_node_id,
            local_virtual_ip,
            max_frame_payload,
            config_hash,
            direct_udp_probe_interval,
            &mut stats,
            thread_stop,
        );
    });
    utun.stop = stop;
    utun.handle = Some(handle);
    Ok(utun)
}

fn open_utun() -> Result<UtunRuntime> {
    let fd = create_utun_socket()?;
    let interface_name = utun_interface_name(fd)?;
    set_nonblocking(fd)?;
    let file = unsafe { File::from_raw_fd(fd) };
    Ok(UtunRuntime {
        interface_name,
        file: Some(file),
        stop: Arc::new(AtomicBool::new(false)),
        handle: None,
    })
}

fn create_utun_socket() -> Result<RawFd> {
    let fd = unsafe { libc::socket(libc::PF_SYSTEM, libc::SOCK_DGRAM, libc::SYSPROTO_CONTROL) };
    if fd < 0 {
        return Err(std::io::Error::last_os_error()).context("socket PF_SYSTEM/SYSPROTO_CONTROL");
    }
    match connect_utun_socket(fd) {
        Ok(()) => Ok(fd),
        Err(error) => {
            unsafe {
                libc::close(fd);
            }
            Err(error)
        }
    }
}

fn connect_utun_socket(fd: RawFd) -> Result<()> {
    let mut info = ctl_info_for_utun()?;
    let name = CString::new(UTUN_CONTROL_NAME)?;
    let bytes = name.as_bytes_with_nul();
    for (index, byte) in bytes.iter().enumerate().take(info.ctl_name.len()) {
        info.ctl_name[index] = *byte as libc::c_char;
    }
    let ioctl_result = unsafe { libc::ioctl(fd, libc::CTLIOCGINFO, &mut info) };
    if ioctl_result < 0 {
        return Err(std::io::Error::last_os_error()).context("ioctl CTLIOCGINFO utun");
    }
    let addr = libc::sockaddr_ctl {
        sc_len: mem::size_of::<libc::sockaddr_ctl>() as u8,
        sc_family: libc::AF_SYSTEM as u8,
        ss_sysaddr: libc::AF_SYS_CONTROL as u16,
        sc_id: info.ctl_id,
        sc_unit: 0,
        sc_reserved: [0; 5],
    };
    let result = unsafe {
        libc::connect(
            fd,
            &addr as *const libc::sockaddr_ctl as *const libc::sockaddr,
            mem::size_of::<libc::sockaddr_ctl>() as libc::socklen_t,
        )
    };
    if result < 0 {
        return Err(std::io::Error::last_os_error()).context("connect utun control socket");
    }
    Ok(())
}

fn ctl_info_for_utun() -> Result<libc::ctl_info> {
    Ok(unsafe { mem::zeroed() })
}

fn utun_interface_name(fd: RawFd) -> Result<String> {
    let mut buffer = [0_i8; 64];
    let mut len = buffer.len() as libc::socklen_t;
    let result = unsafe {
        libc::getsockopt(
            fd,
            libc::SYSPROTO_CONTROL,
            UTUN_OPT_IFNAME,
            buffer.as_mut_ptr() as *mut libc::c_void,
            &mut len,
        )
    };
    if result < 0 {
        return Err(std::io::Error::last_os_error()).context("getsockopt UTUN_OPT_IFNAME");
    }
    let bytes = buffer
        .iter()
        .take_while(|value| **value != 0)
        .map(|value| *value as u8)
        .collect::<Vec<_>>();
    String::from_utf8(bytes).context("decode utun interface name")
}

fn configure_utun_ip(interface_name: &str, virtual_ip: Ipv4Addr, prefix_len: u8) -> Result<()> {
    let netmask = prefix_to_netmask(prefix_len)?;
    clear_utun_ipv4_addresses(interface_name)?;
    run_command(
        "/sbin/ifconfig",
        &[
            interface_name,
            "inet",
            &virtual_ip.to_string(),
            &virtual_ip.to_string(),
            "netmask",
            &netmask.to_string(),
            "up",
            "mtu",
            &DEFAULT_UTUN_MTU.to_string(),
        ],
    )
    .with_context(|| format!("configure {interface_name} address {virtual_ip}/{prefix_len}"))
}

fn clear_utun_ipv4_addresses(interface_name: &str) -> Result<()> {
    let output = Command::new("/sbin/ifconfig")
        .arg(interface_name)
        .output()
        .with_context(|| format!("inspect {interface_name} IPv4 addresses"))?;
    if !output.status.success() {
        return Ok(());
    }
    let text = String::from_utf8_lossy(&output.stdout);
    for address in parse_ifconfig_ipv4_addresses(&text) {
        let address = address.to_string();
        let _ = run_command(
            "/sbin/ifconfig",
            &[interface_name, "inet", &address, "delete"],
        );
    }
    Ok(())
}

fn parse_ifconfig_ipv4_addresses(text: &str) -> Vec<Ipv4Addr> {
    text.lines()
        .filter_map(|line| {
            let line = line.trim_start();
            let rest = line.strip_prefix("inet ")?;
            let value = rest.split_whitespace().next()?;
            value.parse::<Ipv4Addr>().ok()
        })
        .collect()
}

fn add_utun_route(interface_name: &str, route: &RouteSpec) -> Result<()> {
    let destination = route.destination.trim();
    if destination.is_empty() {
        return Ok(());
    }
    let (target, prefix_len) = parse_route_destination(destination)?;
    let route_kind = if prefix_len == 32 { "-host" } else { "-net" };
    run_command(
        "/sbin/route",
        &[
            "-n",
            "add",
            route_kind,
            &target.to_string(),
            "-interface",
            interface_name,
        ],
    )
    .or_else(|error| {
        if error.to_string().contains("File exists") {
            Ok(())
        } else {
            Err(error)
        }
    })
    .with_context(|| format!("add route {destination} via {interface_name}"))
}

fn delete_utun_route(interface_name: &str, route: &RouteSpec) -> Result<()> {
    let destination = route.destination.trim();
    if destination.is_empty() {
        return Ok(());
    }
    let (target, prefix_len) = parse_route_destination(destination)?;
    let route_kind = if prefix_len == 32 { "-host" } else { "-net" };
    run_command(
        "/sbin/route",
        &[
            "-n",
            "delete",
            route_kind,
            &target.to_string(),
            "-interface",
            interface_name,
        ],
    )
}

fn configure_utun_dns(interface_name: &str, dns_servers: &[String]) -> Result<()> {
    let servers = dns_servers
        .iter()
        .map(|value| value.trim())
        .filter(|value| !value.is_empty())
        .collect::<Vec<_>>();
    if servers.is_empty() {
        return clear_utun_dns(interface_name);
    }
    let mut script = String::new();
    script.push_str("d.init\n");
    script.push_str("d.add ServerAddresses *");
    for server in servers {
        script.push(' ');
        script.push_str(server);
    }
    script.push('\n');
    script.push_str(&format!(
        "set State:/Network/Service/{}/DNS\n",
        scutil_key_component(interface_name)
    ));
    run_command_with_input("/usr/sbin/scutil", &[], script.as_bytes())
        .with_context(|| format!("configure macos DNS for {interface_name}"))
}

fn clear_utun_dns(interface_name: &str) -> Result<()> {
    let script = format!(
        "remove State:/Network/Service/{}/DNS\n",
        scutil_key_component(interface_name)
    );
    run_command_with_input("/usr/sbin/scutil", &[], script.as_bytes()).or_else(|error| {
        let message = error.to_string();
        if message.contains("No such key") || message.contains("not found") {
            Ok(())
        } else {
            Err(error)
        }
    })
}

fn scutil_key_component(value: &str) -> String {
    value
        .chars()
        .filter(|ch| ch.is_ascii_alphanumeric() || *ch == '-' || *ch == '_' || *ch == '.')
        .collect()
}

fn parse_route_destination(destination: &str) -> Result<(Ipv4Addr, u8)> {
    let (ip, prefix) = destination
        .split_once('/')
        .map(|(ip, prefix)| (ip, prefix.parse::<u8>()))
        .unwrap_or((destination, Ok(32)));
    let prefix = prefix.with_context(|| format!("parse route prefix {destination}"))?;
    if prefix > 32 {
        bail!("invalid IPv4 route prefix {prefix} for {destination}");
    }
    let ip = ip
        .parse::<Ipv4Addr>()
        .with_context(|| format!("parse route IP {destination}"))?;
    Ok((network_addr(ip, prefix), prefix))
}

fn parse_virtual_ipv4(value: &str) -> Result<Ipv4Addr> {
    let ip = value
        .trim()
        .split_once('/')
        .map_or(value.trim(), |(ip, _)| ip.trim());
    ip.parse::<Ipv4Addr>()
        .with_context(|| format!("parse IPv4 address {ip}"))
}

fn prefix_to_netmask(prefix_len: u8) -> Result<Ipv4Addr> {
    if prefix_len > 32 {
        bail!("invalid IPv4 prefix length {prefix_len}");
    }
    let mask = if prefix_len == 0 {
        0
    } else {
        u32::MAX << (32 - prefix_len)
    };
    Ok(Ipv4Addr::from(mask))
}

fn network_addr(ip: Ipv4Addr, prefix_len: u8) -> Ipv4Addr {
    if prefix_len == 0 {
        return Ipv4Addr::UNSPECIFIED;
    }
    let mask = u32::MAX << (32 - prefix_len);
    Ipv4Addr::from(u32::from(ip) & mask)
}

fn attach_udp_relay_sessions(config: &RelayDataPlaneConfig) -> Result<Vec<RelayPeer>> {
    let mut peers = Vec::new();
    let mut failures = Vec::new();
    for session in &config.sessions {
        let relay_addr = relay_udp_address_for_session(config.relay_address.as_str(), session)
            .with_context(|| format!("parse relay address {}", config.relay_address))?;
        let socket = UdpSocket::bind("0.0.0.0:0").context("bind relay UDP socket")?;
        socket
            .connect(relay_addr)
            .with_context(|| format!("connect relay UDP socket {relay_addr}"))?;
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
    if peers.is_empty() {
        bail!("attach relay sessions failed: {}", failures.join("; "));
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
    Err(last_error.unwrap_or_else(|| anyhow!("relay attach timed out")))
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
        .ok_or_else(|| anyhow!("missing relay UDP address"))?;
    address
        .to_socket_addrs()
        .with_context(|| format!("resolve macos relay UDP socket address {address}"))?
        .next()
        .ok_or_else(|| anyhow!("macos relay UDP address resolved no endpoints: {address}"))
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
                bail!("relay attach session mismatch")
            }
        }
        Some("error") => bail!(
            "{}",
            value
                .get("message")
                .and_then(serde_json::Value::as_str)
                .unwrap_or("relay attach failed")
        ),
        _ => bail!("unexpected relay attach response"),
    }
}

fn run_udp_data_plane(
    mut file: File,
    peers: Vec<RelayPeer>,
    mut direct_udp: Option<DirectUdpTransport>,
    local_node_id: String,
    local_virtual_ip: String,
    max_frame_payload: usize,
    config_hash: u64,
    direct_udp_probe_interval: Duration,
    stats: &mut RelayDataPlaneStats,
    stop: Arc<AtomicBool>,
) {
    let mut seq = 0_u64;
    let mut tun_buffer = vec![0_u8; MAX_PACKET_SIZE + UTUN_HEADER_LEN];
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
                direct_udp.send_probe_packets();
            }
            last_direct_udp_probe = Instant::now();
        }
        match file.read(&mut tun_buffer) {
            Ok(0) => thread::sleep(Duration::from_millis(10)),
            Ok(packet_len) => {
                if let Some(packet) = strip_utun_header(&tun_buffer[..packet_len]) {
                    if packet_targets_local_virtual_ip(packet, local_virtual_ip.as_str()) {
                        continue;
                    }
                    stats.last_tun_packet_at_ms = Some(current_timestamp_ms());
                    if packet.len() <= max_frame_payload {
                        if let Some(peer) = relay_peer_for_packet(&peers, packet) {
                            seq = seq.wrapping_add(1);
                            let packet = normalize_ipv4_transport_checksums(packet);
                            record_tun_tcp_packet(stats, &packet);
                            if let Some(frame) =
                                encode_slan_relay_data_frame(seq, config_hash, &packet)
                            {
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
                                        Ok(_) => record_relay_tun_packet_sent(stats, peer),
                                        Err(error) => record_relay_send_failure(
                                            stats,
                                            peer,
                                            error.to_string(),
                                        ),
                                    }
                                } else if let Some(payload) = encode_relay_forward(peer, &frame) {
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
                        } else {
                            if let Some(destination) = ipv4_destination(packet) {
                                if should_ignore_unroutable_destination(&destination) {
                                    continue;
                                }
                                stats.unroutable_tun_packets =
                                    stats.unroutable_tun_packets.saturating_add(1);
                                stats.last_unroutable_destination = Some(destination);
                            }
                        }
                    } else {
                        stats.oversized_tun_packets = stats.oversized_tun_packets.saturating_add(1);
                        stats.last_oversized_tun_packet_size = Some(packet.len() as u32);
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
                        match write_utun_ipv4_packet(&mut file, &packet) {
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
                        direct_udp.send_pong_to_peer(received.peer_index);
                    } else if control_packet
                        .as_ref()
                        .is_some_and(|packet| packet.kind == DirectUdpControlKind::Pong)
                    {
                        direct_udp.mark_peer_ready(received.peer_index, received.remote_addr);
                    } else if let Some(packet) =
                        decode_slan_relay_data_frame(&relay_buffer[..frame_len])
                    {
                        direct_udp.mark_peer_ready(received.peer_index, received.remote_addr);
                        stats.last_relay_packet_at_ms = Some(current_timestamp_ms());
                        record_relay_tcp_packet(stats, packet);
                        let packet = normalize_ipv4_transport_checksums(packet);
                        if write_utun_ipv4_packet(&mut file, &packet).is_ok() {
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
) -> RelayDataPlaneStats {
    RelayDataPlaneStats {
        relay_address: config.relay_address.clone(),
        relay_transport: Some(config.transport.clone()),
        active_path: Some(PathKind::RelayUdp.as_str().to_string()),
        requested_relay_session_count: peers.len() as u32,
        relay_session_count: peers.len() as u32,
        attached_peer_session_count: peers.len() as u32,
        attached_transport_count: peers.len() as u32,
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

fn record_relay_packet_received(stats: &mut RelayDataPlaneStats, peer: &RelayPeer) {
    stats.relay_packets_received = stats.relay_packets_received.saturating_add(1);
    if let Some(peer_stats) = relay_peer_stats_mut(stats, peer) {
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

fn record_relay_send_failure(stats: &mut RelayDataPlaneStats, peer: &RelayPeer, error: String) {
    stats.relay_send_failures = stats.relay_send_failures.saturating_add(1);
    stats.last_relay_error = Some(error.clone());
    if let Some(peer_stats) = relay_peer_stats_mut(stats, peer) {
        peer_stats.send_failures = peer_stats.send_failures.saturating_add(1);
        peer_stats.last_relay_error = Some(error);
        peer_stats.last_send_path = Some(PathKind::RelayUdp.as_str().to_string());
    }
}

fn record_relay_write_failure(stats: &mut RelayDataPlaneStats, peer: &RelayPeer) {
    stats.wintun_write_failures = stats.wintun_write_failures.saturating_add(1);
    if let Some(peer_stats) = relay_peer_stats_mut(stats, peer) {
        peer_stats.wintun_write_failures = peer_stats.wintun_write_failures.saturating_add(1);
    }
}

fn relay_peer_stats_mut<'a>(
    stats: &'a mut RelayDataPlaneStats,
    peer: &RelayPeer,
) -> Option<&'a mut RelayPeerStats> {
    stats
        .peers
        .iter_mut()
        .find(|stats| stats.session_id == peer.session_id)
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
        .unwrap_or_else(|| PathBuf::from("/Library/Application Support"))
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

fn strip_utun_header(frame: &[u8]) -> Option<&[u8]> {
    if frame.len() <= UTUN_HEADER_LEN {
        return None;
    }
    let packet = &frame[UTUN_HEADER_LEN..];
    if packet.first().map(|byte| byte >> 4) == Some(4) {
        Some(packet)
    } else {
        None
    }
}

fn write_utun_ipv4_packet(file: &mut File, packet: &[u8]) -> std::io::Result<()> {
    let mut frame = Vec::with_capacity(UTUN_HEADER_LEN + packet.len());
    frame.extend_from_slice(&AF_INET_HEADER);
    frame.extend_from_slice(packet);
    write_packet_with_retry(file, &frame)
}

fn write_packet_with_retry(file: &mut File, packet: &[u8]) -> std::io::Result<()> {
    let deadline = Instant::now() + Duration::from_secs(1);
    let mut offset = 0_usize;
    while offset < packet.len() {
        match file.write(&packet[offset..]) {
            Ok(0) => {
                if Instant::now() >= deadline {
                    return Err(std::io::Error::new(
                        ErrorKind::WriteZero,
                        "macos utun write made no progress",
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

fn relay_peer_for_packet<'a>(peers: &'a [RelayPeer], packet: &[u8]) -> Option<&'a RelayPeer> {
    let destination = ipv4_destination(packet)?;
    peers.iter().find(|peer| {
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
    if index == 0 {
        None
    } else {
        Some(index)
    }
}

fn run_command(program: &str, args: &[&str]) -> Result<()> {
    let output = Command::new(program)
        .args(args)
        .output()
        .with_context(|| format!("run {program} {}", args.join(" ")))?;
    if output.status.success() {
        return Ok(());
    }
    let stderr = String::from_utf8_lossy(&output.stderr).trim().to_string();
    let stdout = String::from_utf8_lossy(&output.stdout).trim().to_string();
    bail!(
        "{} {} failed: {}{}{}",
        program,
        args.join(" "),
        output.status,
        if stderr.is_empty() { "" } else { ": " },
        if stderr.is_empty() { stdout } else { stderr }
    )
}

fn run_command_with_input(program: &str, args: &[&str], input: &[u8]) -> Result<()> {
    let mut child = Command::new(program)
        .args(args)
        .stdin(std::process::Stdio::piped())
        .stdout(std::process::Stdio::piped())
        .stderr(std::process::Stdio::piped())
        .spawn()
        .with_context(|| format!("run {program} {}", args.join(" ")))?;
    if let Some(mut stdin) = child.stdin.take() {
        stdin
            .write_all(input)
            .with_context(|| format!("write stdin to {program} {}", args.join(" ")))?;
    }
    let output = child
        .wait_with_output()
        .with_context(|| format!("wait {program} {}", args.join(" ")))?;
    if output.status.success() {
        return Ok(());
    }
    let stderr = String::from_utf8_lossy(&output.stderr).trim().to_string();
    let stdout = String::from_utf8_lossy(&output.stdout).trim().to_string();
    bail!(
        "{} {} failed: {}{}{}",
        program,
        args.join(" "),
        output.status,
        if stderr.is_empty() { "" } else { ": " },
        if stderr.is_empty() { stdout } else { stderr }
    )
}

fn macos_diagnostic_checks(runtime: &MacosRuntime) -> Vec<PlatformDiagnosticCheck> {
    vec![
        PlatformDiagnosticCheck {
            name: "utun".to_string(),
            ok: runtime.interface_name.is_some(),
            message: runtime.interface_name.clone(),
        },
        PlatformDiagnosticCheck {
            name: "virtualIp".to_string(),
            ok: runtime.virtual_ip.is_some(),
            message: runtime.virtual_ip.clone(),
        },
        PlatformDiagnosticCheck {
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
        },
    ]
}

#[cfg(test)]
mod tests {
    use std::net::Ipv4Addr;

    use client_core::RelayTicket;

    use super::{parse_ifconfig_ipv4_addresses, parse_virtual_ipv4, relay_udp_address_for_session};

    #[test]
    fn parses_ifconfig_ipv4_addresses() {
        let output = r#"utun5: flags=8051<UP,POINTOPOINT,RUNNING,MULTICAST> mtu 1280
	inet 10.0.0.1 --> 10.0.0.1 netmask 0xff000000
	inet6 fe80::1%utun5 prefixlen 64 scopeid 0x17
	inet 10.0.0.2 --> 10.0.0.2 netmask 0xff000000
"#;

        assert_eq!(
            parse_ifconfig_ipv4_addresses(output),
            vec![Ipv4Addr::new(10, 0, 0, 1), Ipv4Addr::new(10, 0, 0, 2),]
        );
    }

    #[test]
    fn parses_cidr_virtual_ipv4() {
        assert_eq!(
            parse_virtual_ipv4(" 10.0.0.1/32 ").unwrap(),
            Ipv4Addr::new(10, 0, 0, 1)
        );
        assert_eq!(
            parse_virtual_ipv4("10.0.0.2").unwrap(),
            Ipv4Addr::new(10, 0, 0, 2)
        );
    }

    #[test]
    fn macos_relay_udp_address_resolves_hostnames() {
        let session = test_session("udp://localhost:29111");
        assert_eq!(
            relay_udp_address_for_session("127.0.0.1:29110", &session)
                .unwrap()
                .port(),
            29111
        );
    }

    fn test_session(relay_url: &str) -> client_core::RelayPeerSession {
        client_core::RelayPeerSession {
            peer_node_id: "peer-node".to_string(),
            session_id: "relay-session".to_string(),
            peer_virtual_ips: vec!["10.0.0.2/32".to_string()],
            ticket: RelayTicket {
                ticket_id: "ticket-1".to_string(),
                network_id: "network-1".to_string(),
                session_id: "relay-session".to_string(),
                src_node_id: "node-local".to_string(),
                dst_node_id: "peer-node".to_string(),
                derp_cluster_id: None,
                country_code: None,
                city_code: None,
                allowed_derp_node_ids: Vec::new(),
                relay_url: relay_url.to_string(),
                expires_at: "2099-01-01T00:00:00Z".to_string(),
                session_key: "session-key".to_string(),
                signature: "signature".to_string(),
            },
        }
    }
}
