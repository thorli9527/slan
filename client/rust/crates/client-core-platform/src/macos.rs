//! macOS platform bridge for SLAN mesh networking.

use std::{
    collections::HashMap,
    env,
    ffi::CString,
    fs::{self, File},
    io::{BufRead, BufReader, ErrorKind, Read, Write},
    mem,
    net::{Ipv4Addr, SocketAddr, TcpStream, ToSocketAddrs, UdpSocket},
    os::fd::{AsRawFd, FromRawFd, RawFd},
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
    ipv4_destination_addr, ipv4_transport_checksum_valid, normalize_ipv4_transport_checksums,
    parse_virtual_ipv4 as parse_config_virtual_ipv4,
    relay_frame::{
        base64_decode, base64_encode, decode_slan_relay_data_frame, encode_slan_relay_data_frame,
        stable_hash64,
    },
    resolver_response_for_query, NetworkRuntimeState, NodeConfig, PathCandidate, PathKind,
    PathState, PeerPathRuntime, PlatformAclPeer, PlatformAclPolicy, PlatformDiagnosticCheck,
    PlatformNetwork, PlatformNetworkDiagnostics, PlatformResolverConfig, PlatformResolverRecord,
    RelayDataPlaneConfig, RelayPeerSession, RouteSpec,
};
use serde::Serialize;

use crate::direct_udp::{
    clear_direct_udp_endpoint_report, direct_udp_control_packet, DirectUdpControlKind,
    DirectUdpTransport,
};
use crate::effective_resolver_servers;

const UTUN_CONTROL_NAME: &str = "com.apple.net.utun_control";
const UTUN_OPT_IFNAME: libc::c_int = 2;
const DEFAULT_UTUN_MTU: u16 = 1280;
const MAX_PACKET_SIZE: usize = 4096;
const RELAY_PEER_NOT_ATTACHED_COOLDOWN: Duration = Duration::from_secs(30);
const UTUN_HEADER_LEN: usize = 4;
const AF_INET_HEADER: [u8; UTUN_HEADER_LEN] = [0, 0, 0, libc::AF_INET as u8];
const MOCK_INTERFACE_NAME: &str = "utun-mock";
const HOST_INTERFACE_PREFIX_LEN: u8 = 32;
const RELAY_STATS_FLUSH_INTERVAL: Duration = Duration::from_secs(10);
const RELAY_KEEPALIVE_INTERVAL: Duration = Duration::from_secs(30);
const DERP_WRITE_RETRY_TIMEOUT: Duration = Duration::from_millis(750);
const DATA_PLANE_IDLE_SLEEP: Duration = Duration::from_millis(2);
const UTUN_REOPEN_ATTEMPTS: usize = 6;
const UTUN_REOPEN_BACKOFF: Duration = Duration::from_millis(50);

fn macos_verbose_trace_enabled() -> bool {
    false
}

macro_rules! macos_trace {
    ($($arg:tt)*) => {
        if macos_verbose_trace_enabled() {
            eprintln!($($arg)*);
        }
    };
}

/// MacosPlatformNetwork 是 macOS 的 PlatformNetwork 实现，负责 utun、路由、
/// resolver、relay 数据面和 direct UDP runtime 的平台适配。
#[derive(Debug, Clone, Default)]
pub struct MacosPlatformNetwork;

/// MacosRuntime 保存 macOS 平台层当前配置缓存和后台 utun runtime。
#[derive(Debug, Default)]
struct MacosRuntime {
    interface_name: Option<String>,
    virtual_ip: Option<String>,
    prefix_len: Option<u8>,
    resolver_servers: Vec<String>,
    resolver_search_domains: Vec<String>,
    resolver_split_domains: Vec<String>,
    resolver_records: Vec<PlatformResolverRecord>,
    resolver_routes: Vec<RouteSpec>,
    routes: Vec<RouteSpec>,
    relay_config: Option<RelayDataPlaneConfig>,
    utun: Option<UtunRuntime>,
}

/// UtunRuntime 持有 utun 文件句柄和数据面线程生命周期。
#[derive(Debug)]
struct UtunRuntime {
    interface_name: String,
    file: Option<File>,
    stop: Arc<AtomicBool>,
    handle: Option<JoinHandle<()>>,
}

impl Drop for UtunRuntime {
    fn drop(&mut self) {
        if let Err(error) = self.stop_and_join() {
            eprintln!(
                "macos data plane shutdown failed interface={} error={error:#}",
                self.interface_name
            );
        }
    }
}

impl UtunRuntime {
    fn stop_and_join(&mut self) -> Result<()> {
        self.stop.store(true, Ordering::SeqCst);
        if let Some(handle) = self.handle.take() {
            handle
                .join()
                .map_err(|_| anyhow!("macos data plane thread panicked"))?;
        }
        Ok(())
    }
}

/// RelayPeer 是 macOS relay 数据面中单个 peer 的 UDP relay 会话。
#[derive(Debug)]
struct RelayPeer {
    session_id: String,
    peer_node_id: String,
    local_node_id: String,
    peer_virtual_ips: Vec<String>,
    peer_virtual_ipv4s: Vec<Ipv4Addr>,
    acl_peer: PlatformAclPeer,
    socket: UdpSocket,
}

/// DerpPeer 是 macOS TCP/DERP 保底中继的一条长连接会话。
#[derive(Debug)]
struct DerpPeer {
    session_id: String,
    peer_node_id: String,
    local_node_id: String,
    server_session_id: String,
    peer_virtual_ips: Vec<String>,
    peer_virtual_ipv4s: Vec<Ipv4Addr>,
    acl_peer: PlatformAclPeer,
    stream: TcpStream,
    reader: TcpStream,
    read_buffer: Vec<u8>,
}

/// RelayDataPlaneStats 是 macOS relay/direct UDP 数据面写入状态文件的统计快照。
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
    relay_tcp_syn_ack_received: u64,
    relay_tcp_psh_received: u64,
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
    last_tun_destination: Option<String>,
    last_tun_protocol: Option<u8>,
    last_tun_packet_size: Option<u32>,
    last_tun_peer_node_id: Option<String>,
    last_tun_send_path: Option<String>,
    last_tun_drop_reason: Option<String>,
    oversized_tun_packets: u64,
    last_oversized_tun_packet_size: Option<u32>,
    wintun_write_failures: u64,
    started_at_ms: u64,
    last_tun_packet_at_ms: Option<u64>,
    last_relay_packet_at_ms: Option<u64>,
    last_relay_peer_node_id: Option<String>,
    last_relay_destination: Option<String>,
    last_relay_protocol: Option<u8>,
    last_relay_packet_size: Option<u32>,
    last_relay_tcp_flags: Option<String>,
    last_relay_keepalive_at_ms: Option<u64>,
    last_utun_write_error: Option<String>,
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
        macos_trace!(
            "SLAN_MACOS_CONFIGURE_IP_START interface={} virtual_ip={}/{}",
            interface_name,
            virtual_ip,
            HOST_INTERFACE_PREFIX_LEN
        );
        configure_utun_ip(&interface_name, virtual_addr, HOST_INTERFACE_PREFIX_LEN)?;
        runtime.virtual_ip = Some(virtual_ip);
        runtime.prefix_len = Some(HOST_INTERFACE_PREFIX_LEN);
        macos_trace!(
            "SLAN_MACOS_CONFIGURE_IP_OK interface={} virtual_ip={}",
            interface_name,
            runtime.virtual_ip.as_deref().unwrap_or("")
        );
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
        macos_trace!(
            "SLAN_MACOS_CONFIGURE_ROUTES_START interface={} routes={}",
            interface_name,
            routes.len()
        );
        for route in routes {
            eprintln!(
                "SLAN_MACOS_ROUTE_APPLY interface={} destination={} gateway={}",
                interface_name,
                route.destination,
                route.gateway.as_deref().unwrap_or("")
            );
            add_utun_route(&interface_name, route)?;
        }
        runtime.routes = routes.to_vec();
        macos_trace!(
            "SLAN_MACOS_CONFIGURE_ROUTES_OK interface={} routes={}",
            interface_name,
            runtime.routes.len()
        );
        Ok(())
    }

    fn configure_resolver(&self, resolver: &PlatformResolverConfig) -> Result<()> {
        let mut runtime = runtime()
            .lock()
            .map_err(|_| anyhow!("macos network runtime lock poisoned"))?;
        let effective_resolver_servers = effective_resolver_servers(&resolver.servers);
        if macos_network_mock_enabled() {
            ensure_mock_runtime(&mut runtime);
            runtime.resolver_servers = effective_resolver_servers;
            runtime.resolver_routes = resolver_route_specs(&runtime.resolver_servers);
            runtime.resolver_search_domains = resolver.search_domains.clone();
            runtime.resolver_split_domains = resolver.split_domains.clone();
            return Ok(());
        }
        ensure_utun_runtime(&mut runtime)?;
        let interface_name = runtime
            .interface_name
            .clone()
            .ok_or_else(|| anyhow!("macos utun interface is not ready"))?;
        runtime.resolver_servers = effective_resolver_servers;
        runtime.resolver_search_domains = resolver.search_domains.clone();
        runtime.resolver_split_domains = resolver.split_domains.clone();
        let resolver_routes = resolver_route_specs(&runtime.resolver_servers);
        macos_trace!(
            "SLAN_MACOS_CONFIGURE_RESOLVER_START interface={} resolver_servers={}",
            interface_name,
            runtime.resolver_servers.join(",")
        );
        sync_utun_routes(&interface_name, &runtime.resolver_routes, &resolver_routes)?;
        runtime.resolver_routes = resolver_routes;
        configure_utun_dns(
            &interface_name,
            &runtime.resolver_servers,
            &resolver.search_domains,
            &resolver.split_domains,
        )?;
        macos_trace!(
            "SLAN_MACOS_CONFIGURE_RESOLVER_OK interface={} resolver_count={}",
            interface_name,
            runtime.resolver_servers.len()
        );
        Ok(())
    }

    fn configure_resolver_map(
        &self,
        _resolver_zones: &[client_core::PlatformResolverZone],
        resolver_records: &[PlatformResolverRecord],
    ) -> Result<()> {
        let mut runtime = runtime()
            .lock()
            .map_err(|_| anyhow!("macos network runtime lock poisoned"))?;
        let changed = runtime.resolver_records != resolver_records;
        runtime.resolver_records = resolver_records.to_vec();
        let mock_enabled = macos_network_mock_enabled();
        drop(runtime);
        if changed && !mock_enabled {
            flush_macos_dns_cache();
        }
        Ok(())
    }

    fn configure_relay(&self, relay_config: Option<&RelayDataPlaneConfig>) -> Result<()> {
        let mut runtime = runtime()
            .lock()
            .map_err(|_| anyhow!("macos network runtime lock poisoned"))?;
        if runtime.utun.is_some()
            && match (runtime.relay_config.as_ref(), relay_config) {
                (Some(current), Some(next)) => current.data_plane_equivalent(next),
                (None, None) => true,
                _ => false,
            }
        {
            runtime.relay_config = relay_config.cloned();
            repair_runtime_network_state(&runtime)?;
            macos_trace!("macos configure_relay reused transport config after route verification");
            return Ok(());
        }
        let previous_relay_config = runtime.relay_config.clone();
        if let (Some(current), Some(next)) = (runtime.relay_config.as_ref(), relay_config) {
            let changed_fields = current.data_plane_change_fields(next);
            eprintln!(
                "macos configure_relay changed_fields={}",
                changed_fields.join(",")
            );
        }
        runtime.relay_config = relay_config.cloned();
        eprintln!(
            "macos configure_relay enabled={} sessions={} transport={}",
            relay_config.map(|config| config.enabled).unwrap_or(false),
            relay_config
                .map(|config| config.sessions.len())
                .unwrap_or(0),
            relay_config
                .map(|config| config.transport.as_str())
                .unwrap_or("")
        );
        if macos_network_mock_enabled() {
            ensure_mock_runtime(&mut runtime);
            return Ok(());
        }
        match restart_data_plane(&mut runtime) {
            Ok(()) => {
                macos_trace!(
                    "SLAN_MACOS_CONFIGURE_RELAY_OK interface={} runtime_attached={} virtual_ip={} relay_enabled={}",
                    runtime.interface_name.as_deref().unwrap_or(""),
                    runtime.utun.is_some(),
                    runtime.virtual_ip.as_deref().unwrap_or(""),
                    runtime
                        .relay_config
                        .as_ref()
                        .map(|config| config.enabled)
                        .unwrap_or(false)
                );
                Ok(())
            }
            Err(error) => {
                eprintln!("SLAN_MACOS_CONFIGURE_RELAY_ERROR error={error:#}");
                runtime.relay_config = previous_relay_config;
                match restart_data_plane(&mut runtime) {
                    Ok(()) => {
                        eprintln!("SLAN_MACOS_CONFIGURE_RELAY_RESTORED_PREVIOUS");
                        Err(error.context(
                            "configure new relay data plane; previous data plane restored",
                        ))
                    }
                    Err(restore_error) => {
                        eprintln!(
                            "SLAN_MACOS_CONFIGURE_RELAY_RESTORE_ERROR original_error={error:#} restore_error={restore_error:#}"
                        );
                        Err(anyhow!(
                            "configure new relay data plane failed: {error:#}; restore previous data plane failed: {restore_error:#}"
                        ))
                    }
                }
            }
        }
    }

    fn disable_network(&self) -> Result<()> {
        let mut runtime = runtime()
            .lock()
            .map_err(|_| anyhow!("macos network runtime lock poisoned"))?;
        if macos_network_mock_enabled() {
            runtime.interface_name = None;
            runtime.virtual_ip = None;
            runtime.prefix_len = None;
            runtime.resolver_servers.clear();
            runtime.resolver_search_domains.clear();
            runtime.resolver_split_domains.clear();
            runtime.resolver_records.clear();
            runtime.resolver_routes.clear();
            runtime.routes.clear();
            runtime.relay_config = None;
            runtime.utun = None;
            return Ok(());
        }
        let utun = runtime.utun.take();
        let interface_name = runtime.interface_name.take();
        let routes = mem::take(&mut runtime.routes);
        let resolver_routes = mem::take(&mut runtime.resolver_routes);
        runtime.virtual_ip = None;
        runtime.prefix_len = None;
        runtime.resolver_servers.clear();
        runtime.resolver_search_domains.clear();
        runtime.resolver_split_domains.clear();
        runtime.resolver_records.clear();
        runtime.relay_config = None;

        drop(utun);
        if let Some(interface_name) = interface_name.as_deref() {
            let _ = clear_utun_dns(interface_name);
            for route in routes.iter().rev() {
                let _ = delete_utun_route(interface_name, route);
            }
            for route in resolver_routes.iter().rev() {
                let _ = delete_utun_route(interface_name, route);
            }
            let _ = run_command("/sbin/ifconfig", &[interface_name, "down"]);
        }
        flush_macos_dns_cache();
        drop(runtime);
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
            resolver_servers: runtime.resolver_servers.clone(),
            resolver_search_domains: runtime.resolver_search_domains.clone(),
            resolver_split_domains: runtime.resolver_split_domains.clone(),
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
            && (config.transport.eq_ignore_ascii_case("udp")
                || config.transport.eq_ignore_ascii_case("relay_udp")
                || config.transport.eq_ignore_ascii_case("derp_tcp_tls_443"))
            && !config.relay_address.trim().is_empty()
            && !config.sessions.is_empty()
    });
    if let Some(mut old_utun) = runtime.utun.take() {
        old_utun
            .stop_and_join()
            .context("stop previous macos data plane")?;
        drop(old_utun);
    }
    let utun = reopen_utun_for_data_plane()?;
    let interface_name = utun.interface_name.clone();
    macos_trace!(
        "SLAN_MACOS_RESTART_DATA_PLANE_START interface={} relay_enabled={} relay_sessions={} has_virtual_ip={} routes={} resolvers={}",
        interface_name,
        config.as_ref().map(|value| value.enabled).unwrap_or(false),
        config.as_ref().map(|value| value.sessions.len()).unwrap_or(0),
        runtime.virtual_ip.is_some(),
        runtime.routes.len(),
        runtime.resolver_servers.len()
    );
    runtime.interface_name = Some(interface_name.clone());
    if let (Some(ip), Some(prefix_len)) = (&runtime.virtual_ip, runtime.prefix_len) {
        configure_utun_ip(&interface_name, ip.parse()?, prefix_len)?;
    }
    configure_utun_mtu(&interface_name, tunnel_mtu_for_relay(config.as_ref()))?;
    configure_utun_dns(&interface_name, &runtime.resolver_servers, &[], &[])?;
    for route in &runtime.routes {
        let _ = add_utun_route(&interface_name, route);
    }
    for route in &runtime.resolver_routes {
        let _ = add_utun_route(&interface_name, route);
    }
    let local_virtual_ip = runtime.virtual_ip.clone().unwrap_or_default();
    let dns_servers = runtime.resolver_servers.clone();
    let dns_records = runtime.resolver_records.clone();
    runtime.utun = Some(if let Some(config) = config {
        start_udp_data_plane(utun, config, local_virtual_ip, dns_servers, dns_records)?
    } else if !local_virtual_ip.trim().is_empty() {
        start_local_data_plane(
            utun,
            local_virtual_ip,
            dns_servers,
            dns_records,
            runtime.relay_config.clone(),
        )?
    } else {
        utun
    });
    macos_trace!(
        "SLAN_MACOS_RESTART_DATA_PLANE_OK interface={} runtime_attached={}",
        interface_name,
        runtime.utun.is_some()
    );
    Ok(())
}

fn start_local_data_plane(
    mut utun: UtunRuntime,
    local_virtual_ip: String,
    dns_servers: Vec<String>,
    dns_records: Vec<PlatformResolverRecord>,
    direct_config: Option<RelayDataPlaneConfig>,
) -> Result<UtunRuntime> {
    macos_trace!(
        "SLAN_MACOS_LOCAL_DP_START virtual_ip={} interface={}",
        local_virtual_ip,
        utun.interface_name
    );
    let file = utun
        .file
        .take()
        .ok_or_else(|| anyhow!("macos utun file descriptor is not ready"))?;
    macos_trace!(
        "SLAN_MACOS_LOCAL_DP_FILE_READY interface={}",
        utun.interface_name
    );
    macos_trace!(
        "SLAN_MACOS_LOCAL_DP_RUNTIME_CLONED interface={} resolvers={} records={}",
        utun.interface_name,
        dns_servers.len(),
        dns_records.len()
    );
    let (
        direct_udp,
        direct_udp_probe_interval,
        direct_network_id,
        node_configs,
        config_hash,
        acl_policies,
    ) = if let Some(config) = direct_config.as_ref() {
        (
            DirectUdpTransport::attach(
                config.local_node_id.as_str(),
                &config.peer_paths,
                &config.path_policy,
            ),
            Duration::from_secs(1),
            config.network_id.clone(),
            config.node_configs.clone(),
            stable_hash64(&serde_json::to_string(config)?),
            config.acl_policies.clone(),
        )
    } else {
        clear_direct_udp_endpoint_report();
        (
            None,
            Duration::from_secs(1),
            String::new(),
            Vec::new(),
            0,
            Vec::new(),
        )
    };
    eprintln!("macos local data plane attached virtual_ip={local_virtual_ip}");
    let stop = Arc::new(AtomicBool::new(false));
    let thread_stop = Arc::clone(&stop);
    macos_trace!(
        "SLAN_MACOS_LOCAL_DP_SPAWN_BEGIN interface={}",
        utun.interface_name
    );
    let handle = thread::spawn(move || {
        run_local_data_plane(
            file,
            local_virtual_ip,
            dns_servers,
            dns_records,
            direct_udp,
            direct_udp_probe_interval,
            direct_network_id,
            node_configs,
            config_hash,
            acl_policies,
            thread_stop,
        );
    });
    macos_trace!(
        "SLAN_MACOS_LOCAL_DP_SPAWN_OK interface={}",
        utun.interface_name
    );
    utun.stop = stop;
    utun.handle = Some(handle);
    macos_trace!(
        "SLAN_MACOS_LOCAL_DP_RETURN interface={}",
        utun.interface_name
    );
    Ok(utun)
}

fn start_udp_data_plane(
    mut utun: UtunRuntime,
    config: RelayDataPlaneConfig,
    local_virtual_ip: String,
    dns_servers: Vec<String>,
    dns_records: Vec<PlatformResolverRecord>,
) -> Result<UtunRuntime> {
    macos_trace!(
        "SLAN_MACOS_UDP_DP_START interface={} virtual_ip={} relay={} sessions={} transport={}",
        utun.interface_name,
        local_virtual_ip,
        config.relay_address,
        config.sessions.len(),
        config.transport
    );
    let file = utun
        .file
        .take()
        .ok_or_else(|| anyhow!("macos utun file descriptor is not ready"))?;
    macos_trace!(
        "SLAN_MACOS_UDP_DP_FILE_READY interface={}",
        utun.interface_name
    );
    let peers = attach_udp_relay_sessions(&config)?;
    let derp_peers = attach_derp_relay_sessions(&config)?;
    eprintln!(
        "macos relay data plane attached udp={} derp={} sessions={} transport={}",
        peers.len(),
        derp_peers.len(),
        config.sessions.len(),
        config.transport
    );
    if peers.is_empty() && derp_peers.is_empty() {
        bail!("attach relay sessions failed: no UDP relay or DERP sessions attached");
    }
    let direct_udp = DirectUdpTransport::attach(
        config.local_node_id.as_str(),
        &config.peer_paths,
        &config.path_policy,
    );
    let config_hash = stable_hash64(&serde_json::to_string(&config)?);
    let max_frame_payload = usize::from(config.max_frame_payload.unwrap_or(1200).clamp(512, 1400));
    let direct_udp_probe_interval = Duration::from_secs(1);
    let direct_network_id = config.network_id.clone();
    let node_configs = config.node_configs.clone();
    let mut stats = relay_data_plane_stats_from_config(&config, &peers, &derp_peers);
    let acl_policies = config.acl_policies.clone();
    stats.direct_udp_attached_peer_count = direct_udp
        .as_ref()
        .map(|transport| transport.peers.len() as u64)
        .unwrap_or(0);
    stats.attached_transport_count = stats
        .attached_peer_session_count
        .saturating_add(stats.direct_udp_attached_peer_count as u32);
    let udp_peer_count = peers.len();
    let derp_peer_count = derp_peers.len();
    let direct_udp_attached_peer_count = stats.direct_udp_attached_peer_count;
    let stop = Arc::new(AtomicBool::new(false));
    let thread_stop = Arc::clone(&stop);
    let handle = thread::spawn(move || {
        run_udp_data_plane(
            file,
            peers,
            derp_peers,
            direct_udp,
            config.local_node_id,
            local_virtual_ip,
            dns_servers,
            max_frame_payload,
            config_hash,
            direct_udp_probe_interval,
            direct_network_id,
            node_configs,
            acl_policies,
            dns_records,
            &mut stats,
            thread_stop,
        );
    });
    macos_trace!(
        "SLAN_MACOS_UDP_DP_SPAWN_OK interface={} udp_peers={} derp_peers={} direct_udp_peers={}",
        utun.interface_name,
        udp_peer_count,
        derp_peer_count,
        direct_udp_attached_peer_count
    );
    utun.stop = stop;
    utun.handle = Some(handle);
    macos_trace!("SLAN_MACOS_UDP_DP_RETURN interface={}", utun.interface_name);
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

fn reopen_utun_for_data_plane() -> Result<UtunRuntime> {
    let mut last_error = None;
    for attempt in 1..=UTUN_REOPEN_ATTEMPTS {
        match open_utun() {
            Ok(utun) => return Ok(utun),
            Err(error) => {
                eprintln!(
                    "SLAN_MACOS_UTUN_REOPEN_RETRY attempt={attempt} max_attempts={UTUN_REOPEN_ATTEMPTS} error={error:#}"
                );
                last_error = Some(error);
                if attempt < UTUN_REOPEN_ATTEMPTS {
                    thread::sleep(UTUN_REOPEN_BACKOFF);
                }
            }
        }
    }
    Err(last_error.unwrap_or_else(|| anyhow!("macos utun reopen failed")))
        .context("reopen macos utun interface for data plane")
}

fn create_utun_socket() -> Result<RawFd> {
    let fd = unsafe { libc::socket(libc::PF_SYSTEM, libc::SOCK_DGRAM, libc::SYSPROTO_CONTROL) };
    if fd < 0 {
        let error = std::io::Error::last_os_error();
        eprintln!(
            "SLAN_MACOS_UTUN_SOCKET_ERROR control={} error={error}",
            UTUN_CONTROL_NAME
        );
        return Err(error).context(format!(
            "socket PF_SYSTEM/SOCK_DGRAM/SYSPROTO_CONTROL failed \
             (control={UTUN_CONTROL_NAME})"
        ));
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
        let error = std::io::Error::last_os_error();
        eprintln!(
            "SLAN_MACOS_UTUN_IOCTL_ERROR fd={} control={} error={error}",
            fd, UTUN_CONTROL_NAME
        );
        return Err(error).context(format!(
            "ioctl CTLIOCGINFO failed for macos utun \
             (fd={fd} control={UTUN_CONTROL_NAME})"
        ));
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
        let error = std::io::Error::last_os_error();
        eprintln!(
            "SLAN_MACOS_UTUN_CONNECT_ERROR fd={} ctl_id={} sc_unit={} control={} error={error}",
            fd, info.ctl_id, addr.sc_unit, UTUN_CONTROL_NAME
        );
        return Err(error).context(format!(
            "connect utun control socket failed \
             (fd={fd} ctl_id={} sc_unit={} control={UTUN_CONTROL_NAME})",
            info.ctl_id, addr.sc_unit
        ));
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

fn configure_utun_mtu(interface_name: &str, mtu: u16) -> Result<()> {
    run_command("/sbin/ifconfig", &[interface_name, "mtu", &mtu.to_string()])
        .with_context(|| format!("configure {interface_name} MTU {mtu}"))
}

fn tunnel_mtu_for_relay(config: Option<&RelayDataPlaneConfig>) -> u16 {
    config
        .and_then(|value| value.max_frame_payload)
        .map(|value| value.clamp(512, 1400))
        .unwrap_or(DEFAULT_UTUN_MTU)
}

fn clear_utun_ipv4_addresses(interface_name: &str) -> Result<()> {
    for address in inspect_interface_ipv4_addresses(interface_name)? {
        let address = address.to_string();
        let _ = run_command(
            "/sbin/ifconfig",
            &[interface_name, "inet", &address, "delete"],
        );
    }
    Ok(())
}

fn inspect_interface_ipv4_addresses(interface_name: &str) -> Result<Vec<Ipv4Addr>> {
    let output = Command::new("/sbin/ifconfig")
        .arg(interface_name)
        .output()
        .with_context(|| format!("inspect {interface_name} IPv4 addresses"))?;
    if !output.status.success() {
        return Ok(Vec::new());
    }
    Ok(parse_ifconfig_ipv4_addresses(&String::from_utf8_lossy(
        &output.stdout,
    )))
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

fn resolver_route_specs(servers: &[String]) -> Vec<RouteSpec> {
    let mut routes = Vec::new();
    for server in servers {
        let Ok(address) = server.trim().parse::<Ipv4Addr>() else {
            continue;
        };
        if address.is_loopback() || address.is_unspecified() {
            continue;
        }
        let route = RouteSpec {
            destination: format!("{address}/32"),
            gateway: None,
        };
        if !routes.contains(&route) {
            routes.push(route);
        }
    }
    routes
}

fn sync_utun_routes(
    interface_name: &str,
    previous: &[RouteSpec],
    next: &[RouteSpec],
) -> Result<()> {
    for route in previous.iter().rev().filter(|route| !next.contains(route)) {
        delete_utun_route(interface_name, route)?;
    }
    for route in next.iter().filter(|route| !previous.contains(route)) {
        add_utun_route(interface_name, route)?;
    }
    Ok(())
}

fn repair_runtime_network_state(runtime: &MacosRuntime) -> Result<()> {
    let interface_name = runtime
        .interface_name
        .as_deref()
        .ok_or_else(|| anyhow!("macos utun interface is not ready"))?;
    if let (Some(ip), Some(prefix_len)) = (&runtime.virtual_ip, runtime.prefix_len) {
        let configured = inspect_interface_ipv4_addresses(interface_name)?;
        let address = ip.parse::<Ipv4Addr>()?;
        if !configured.contains(&address) {
            eprintln!(
                "SLAN_MACOS_REPAIR_IP interface={} virtual_ip={}/{}",
                interface_name, ip, prefix_len
            );
            configure_utun_ip(interface_name, address, prefix_len)?;
        }
    }
    for route in runtime.routes.iter().chain(&runtime.resolver_routes) {
        if !utun_route_is_owned_by(interface_name, route)? {
            eprintln!(
                "SLAN_MACOS_REPAIR_ROUTE interface={} destination={}",
                interface_name, route.destination
            );
            add_utun_route(interface_name, route)?;
        }
    }
    Ok(())
}

fn add_utun_route(interface_name: &str, route: &RouteSpec) -> Result<()> {
    let destination = route.destination.trim();
    if destination.is_empty() {
        return Ok(());
    }
    let (target, prefix_len) = parse_route_destination(destination)?;
    let route_kind = if prefix_len == 32 { "-host" } else { "-net" };
    eprintln!(
        "SLAN_MACOS_ROUTE_ADD interface={} destination={} target={} prefix_len={}",
        interface_name, destination, target, prefix_len
    );
    match run_command(
        "/sbin/route",
        &[
            "-n",
            "add",
            route_kind,
            &target.to_string(),
            "-interface",
            interface_name,
        ],
    ) {
        Ok(()) => Ok(()),
        Err(error) if error.to_string().contains("File exists") => {
            eprintln!(
                "SLAN_MACOS_ROUTE_EXISTS interface={} destination={} target={}",
                interface_name, destination, target
            );
            replace_utun_route(interface_name, route_kind, target)
                .with_context(|| format!("replace route {destination} via {interface_name}"))
        }
        Err(error) => {
            Err(error).with_context(|| format!("add route {destination} via {interface_name}"))
        }
    }
}

fn delete_utun_route(interface_name: &str, route: &RouteSpec) -> Result<()> {
    let destination = route.destination.trim();
    if destination.is_empty() {
        return Ok(());
    }
    let (target, prefix_len) = parse_route_destination(destination)?;
    let route_kind = if prefix_len == 32 { "-host" } else { "-net" };
    if !utun_route_is_owned_by(interface_name, route)? {
        eprintln!(
            "SLAN_MACOS_ROUTE_DELETE_SKIPPED interface={} destination={} reason=route_not_owned",
            interface_name, destination
        );
        return Ok(());
    }
    delete_route_target(route_kind, target).or_else(|error| {
        let message = error.to_string();
        if message.contains("not in table") {
            Ok(())
        } else {
            Err(error).with_context(|| format!("delete route {destination} via {interface_name}"))
        }
    })
}

fn utun_route_is_owned_by(interface_name: &str, route: &RouteSpec) -> Result<bool> {
    let destination = route.destination.trim();
    if destination.is_empty() {
        return Ok(false);
    }
    let (target, _) = parse_route_destination(destination)?;
    Ok(route_interface_for_target(target)?.as_deref() == Some(interface_name))
}

fn route_interface_for_target(target: Ipv4Addr) -> Result<Option<String>> {
    let output = Command::new("/sbin/route")
        .args(["-n", "get", &target.to_string()])
        .output()
        .with_context(|| format!("inspect route for {target}"))?;
    if !output.status.success() {
        return Ok(None);
    }
    Ok(parse_route_interface(&String::from_utf8_lossy(
        &output.stdout,
    )))
}

fn parse_route_interface(output: &str) -> Option<String> {
    output.lines().find_map(|line| {
        let (key, value) = line.trim().split_once(':')?;
        (key.trim() == "interface")
            .then(|| value.trim().to_string())
            .filter(|value| !value.is_empty())
    })
}

fn replace_utun_route(interface_name: &str, route_kind: &str, target: Ipv4Addr) -> Result<()> {
    eprintln!(
        "SLAN_MACOS_ROUTE_REPLACE interface={} target={} kind={}",
        interface_name, target, route_kind
    );
    let _ = delete_route_target(route_kind, target);
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
}

fn delete_route_target(route_kind: &str, target: Ipv4Addr) -> Result<()> {
    let target = target.to_string();
    eprintln!(
        "SLAN_MACOS_ROUTE_DELETE target={} kind={}",
        target, route_kind
    );
    run_command(
        "/sbin/route",
        &["-n", "delete", route_kind, target.as_str()],
    )
}

fn configure_utun_dns(
    interface_name: &str,
    dns_servers: &[String],
    search_domains: &[String],
    split_domains: &[String],
) -> Result<()> {
    let script = build_scutil_dns_script(dns_servers, search_domains, split_domains);
    if script.is_empty() {
        return clear_utun_dns(interface_name);
    }
    let mut script = script;
    script.push_str(&format!(
        "set State:/Network/Service/{}/DNS\n",
        scutil_key_component(interface_name)
    ));
    run_command_with_input("/usr/sbin/scutil", &[], script.as_bytes())
        .with_context(|| format!("configure macos DNS for {interface_name}"))
}

fn build_scutil_dns_script(
    dns_servers: &[String],
    search_domains: &[String],
    split_domains: &[String],
) -> String {
    let servers = dns_servers
        .iter()
        .map(|value| value.trim())
        .filter(|value| !value.is_empty())
        .collect::<Vec<_>>();
    if servers.is_empty() {
        return String::new();
    }
    let mut script = String::new();
    script.push_str("d.init\n");
    script.push_str("d.add ServerAddresses *");
    for server in servers {
        script.push(' ');
        script.push_str(server);
    }
    script.push('\n');
    let search_domains = search_domains
        .iter()
        .map(|value| value.trim())
        .filter(|value| !value.is_empty())
        .collect::<Vec<_>>();
    if !search_domains.is_empty() {
        script.push_str("d.add SearchDomains *");
        for domain in &search_domains {
            script.push(' ');
            script.push_str(domain);
        }
        script.push('\n');
    }
    let split_domains = split_domains
        .iter()
        .map(|value| value.trim())
        .filter(|value| !value.is_empty())
        .collect::<Vec<_>>();
    if !split_domains.is_empty() {
        script.push_str("d.add SupplementalMatchDomains *");
        for domain in &split_domains {
            script.push(' ');
            script.push_str(domain);
        }
        script.push('\n');
    }
    script
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
        if relay_path_kind_from_session(session) != Some(PathKind::RelayUdp) {
            continue;
        }
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
            peer_virtual_ipv4s: session
                .peer_virtual_ips
                .iter()
                .filter_map(|value| parse_config_virtual_ipv4(value))
                .collect(),
            acl_peer: PlatformAclPeer {
                peer_node_id: Some(session.peer_node_id.clone()),
                peer_virtual_ips: session.peer_virtual_ips.clone(),
            },
            socket,
        });
    }
    if peers.is_empty() && !failures.is_empty() {
        bail!("attach relay sessions failed: {}", failures.join("; "));
    }
    Ok(peers)
}

fn attach_derp_relay_sessions(config: &RelayDataPlaneConfig) -> Result<Vec<DerpPeer>> {
    let mut peers = Vec::new();
    let mut failures = Vec::new();
    let has_udp_sessions = config
        .sessions
        .iter()
        .any(|session| relay_path_kind_from_session(session) == Some(PathKind::RelayUdp));
    for session in &config.sessions {
        if relay_path_kind_from_session(session) != Some(PathKind::DerpTcpTls443) {
            continue;
        }
        match attach_derp_relay_session(config.local_node_id.as_str(), session) {
            Ok(peer) => peers.push(peer),
            Err(error) => failures.push(format!("{}: {error}", session.session_id)),
        }
    }
    if !failures.is_empty() {
        eprintln!(
            "SLAN_MACOS_DERP_ATTACH_WARN failures={} attached={} has_udp_sessions={}",
            failures.join("; "),
            peers.len(),
            has_udp_sessions
        );
    }
    if peers.is_empty() && !failures.is_empty() {
        if has_udp_sessions {
            eprintln!("SLAN_MACOS_DERP_ATTACH_FALLBACK udp_only=true");
            return Ok(peers);
        }
        bail!("attach DERP sessions failed: {}", failures.join("; "));
    }
    Ok(peers)
}

fn attach_derp_relay_session(local_node_id: &str, session: &RelayPeerSession) -> Result<DerpPeer> {
    let address = normalize_derp_tcp_address(session.ticket.relay_url.as_str())
        .ok_or_else(|| anyhow!("missing DERP TCP address"))?;
    let stream = TcpStream::connect(address.as_str())
        .with_context(|| format!("connect DERP TCP socket {address}"))?;
    stream.set_nodelay(true)?;
    stream.set_read_timeout(Some(Duration::from_secs(3)))?;
    stream.set_write_timeout(Some(Duration::from_secs(3)))?;
    let mut writer = stream.try_clone().context("clone DERP writer stream")?;
    let mut reader = BufReader::new(stream.try_clone().context("clone DERP reader stream")?);
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
        serde_json::from_str(line.trim()).context("decode DERP connect response")?;
    if value.get("kind").and_then(serde_json::Value::as_str) != Some("connected") {
        bail!(
            "{}",
            value
                .get("error")
                .and_then(|error| error.get("message"))
                .and_then(serde_json::Value::as_str)
                .unwrap_or("DERP connect failed")
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
        peer_virtual_ipv4s: session
            .peer_virtual_ips
            .iter()
            .filter_map(|value| parse_config_virtual_ipv4(value))
            .collect(),
        acl_peer: PlatformAclPeer {
            peer_node_id: Some(session.peer_node_id.clone()),
            peer_virtual_ips: session.peer_virtual_ips.clone(),
        },
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
                    "DERP TCP connection closed before connect ack",
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
                    "DERP TCP write returned zero bytes",
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
        "ticketId": &ticket.ticket_id,
        "networkId": &ticket.network_id,
        "sessionId": &ticket.session_id,
        "srcNodeId": &ticket.src_node_id,
        "dstNodeId": &ticket.dst_node_id,
        "derpClusterId": &ticket.derp_cluster_id,
        "countryCode": &ticket.country_code,
        "cityCode": &ticket.city_code,
        "allowedDerpNodeIds": &ticket.allowed_derp_node_ids,
        "relayUrl": &ticket.relay_url,
        "expiresAt": &ticket.expires_at,
        "sessionKey": &ticket.session_key,
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

#[allow(clippy::too_many_arguments)]
fn run_udp_data_plane(
    mut file: File,
    peers: Vec<RelayPeer>,
    mut derp_peers: Vec<DerpPeer>,
    mut direct_udp: Option<DirectUdpTransport>,
    local_node_id: String,
    local_virtual_ip: String,
    dns_servers: Vec<String>,
    max_frame_payload: usize,
    config_hash: u64,
    direct_udp_probe_interval: Duration,
    direct_network_id: String,
    node_configs: Vec<NodeConfig>,
    acl_policies: Vec<PlatformAclPolicy>,
    dns_records: Vec<PlatformResolverRecord>,
    stats: &mut RelayDataPlaneStats,
    stop: Arc<AtomicBool>,
) {
    let mut seq = 0_u64;
    let mut logged_first_tun_send = false;
    let mut logged_first_relay_recv = false;
    let mut logged_first_utun_write = false;
    let mut tun_buffer = vec![0_u8; MAX_PACKET_SIZE + UTUN_HEADER_LEN];
    let mut relay_buffer = vec![0_u8; MAX_PACKET_SIZE + 512];
    let mut last_stats_flush = Instant::now();
    let mut last_keepalive = Instant::now()
        .checked_sub(RELAY_KEEPALIVE_INTERVAL)
        .unwrap_or_else(Instant::now);
    let mut last_direct_udp_probe = Instant::now()
        .checked_sub(direct_udp_probe_interval)
        .unwrap_or_else(Instant::now);
    let mut relay_send_blocked_until: HashMap<String, Instant> = HashMap::new();
    let mut poll_fds = data_plane_poll_fds(&file, &peers, &derp_peers, direct_udp.as_ref());
    persist_relay_stats(stats);
    while !stop.load(Ordering::SeqCst) {
        let mut did_work = false;
        if last_keepalive.elapsed() >= RELAY_KEEPALIVE_INTERVAL {
            send_relay_keepalives(&peers);
            stats.last_relay_keepalive_at_ms = Some(current_timestamp_ms());
            last_keepalive = Instant::now();
        }
        if last_direct_udp_probe.elapsed() >= direct_udp_probe_interval {
            if let Some(direct_udp) = direct_udp.as_mut() {
                for peer_node_id in direct_udp.poll_probe_health(current_timestamp_ms()) {
                    macos_trace!("SLAN_MACOS_DIRECT_UDP_PEER_FAILED peer={}", peer_node_id);
                }
                stats.direct_udp_ready_peer_count = direct_udp.ready_peer_count() as u64;
                let _ = direct_udp.send_punch_endpoint_probes(&direct_network_id, &node_configs);
                stats.direct_udp_probes_sent = stats
                    .direct_udp_probes_sent
                    .saturating_add(direct_udp.send_probe_packets() as u64);
            }
            last_direct_udp_probe = Instant::now();
        }
        match file.read(&mut tun_buffer) {
            Ok(0) => {}
            Ok(packet_len) => {
                did_work = true;
                if let Some(packet) = strip_utun_header(&tun_buffer[..packet_len]) {
                    if let Some(reply) = local_dns_reply(packet, &dns_servers, &dns_records) {
                        let _ = write_utun_ipv4_packet(&mut file, &reply);
                        continue;
                    }
                    if packet_targets_local_virtual_ip(packet, local_virtual_ip.as_str()) {
                        if let Some(reply) =
                            local_virtual_ip_reply(packet, local_virtual_ip.as_str())
                        {
                            let _ = write_utun_ipv4_packet(&mut file, &reply);
                        } else {
                            let packet = normalize_ipv4_transport_checksums(packet);
                            let _ = write_utun_ipv4_packet(&mut file, &packet);
                        }
                        continue;
                    }
                    stats.last_tun_packet_at_ms = Some(current_timestamp_ms());
                    stats.last_tun_destination = ipv4_destination(packet);
                    stats.last_tun_protocol = ipv4_protocol(packet);
                    stats.last_tun_packet_size = Some(packet.len() as u32);
                    stats.last_tun_peer_node_id = None;
                    stats.last_tun_send_path = None;
                    stats.last_tun_drop_reason = None;
                    if packet.len() <= max_frame_payload {
                        if let Some(peer) = relay_peer_for_packet(&peers, packet) {
                            stats.last_tun_peer_node_id = Some(peer.peer_node_id.clone());
                            macos_trace!(
                                "SLAN_MACOS_TUN_PACKET peer={} dst={:?} proto={:?} size={} direct_ready={}",
                                peer.peer_node_id,
                                ipv4_destination(packet),
                                ipv4_protocol(packet),
                                packet.len(),
                                direct_udp
                                    .as_ref()
                                    .and_then(|transport| {
                                        transport.ready_peer_index_for_packet(packet)
                                    })
                                    .is_some()
                            );
                            if !acl_allows_egress_packet(
                                packet,
                                &acl_policies,
                                Some(acl_peer_for_relay_peer(peer)),
                            ) {
                                eprintln!(
                                    "SLAN_MACOS_TUN_DROP reason=acl_egress_denied peer={} dst={:?}",
                                    peer.peer_node_id,
                                    ipv4_destination(packet)
                                );
                                stats.last_tun_drop_reason = Some("acl_egress_denied".to_string());
                                continue;
                            }
                            seq = seq.wrapping_add(1);
                            let packet = normalize_ipv4_transport_checksums(packet);
                            record_tun_tcp_packet(stats, &packet);
                            if let Some(frame) =
                                encode_slan_relay_data_frame(seq, config_hash, &packet)
                            {
                                let mut direct_sent = false;
                                if let Some(direct_peer_index) =
                                    direct_udp.as_ref().and_then(|transport| {
                                        transport.ready_peer_index_for_packet(&packet)
                                    })
                                {
                                    let direct_path_kind = direct_udp
                                        .as_ref()
                                        .and_then(|transport| {
                                            transport.peers.get(direct_peer_index)
                                        })
                                        .map(|peer| peer.path_kind)
                                        .unwrap_or(PathKind::DirectUdp);
                                    match direct_udp
                                        .as_ref()
                                        .expect("direct udp checked")
                                        .send_to_peer(direct_peer_index, &frame)
                                    {
                                        Ok(_) => {
                                            direct_sent = true;
                                            macos_trace!(
                                                "SLAN_MACOS_DIRECT_SEND_OK peer={} dst={:?} size={}",
                                                peer.peer_node_id,
                                                ipv4_destination(&packet),
                                                packet.len()
                                            );
                                            stats.last_tun_send_path =
                                                Some(direct_path_kind.as_str().to_string());
                                            record_direct_tun_packet_sent(
                                                stats,
                                                peer,
                                                direct_path_kind,
                                            );
                                            if should_hedge_direct_packet_to_relay(&packet) {
                                                hedge_udp_packet_to_relay(peer, &frame, stats);
                                            }
                                        }
                                        Err(error) => {
                                            eprintln!(
                                                "SLAN_MACOS_DIRECT_SEND_ERROR peer={} dst={:?} error={}",
                                                peer.peer_node_id,
                                                ipv4_destination(&packet),
                                                error
                                            );
                                            stats.last_tun_drop_reason =
                                                Some("direct_udp_send_failed".to_string());
                                            record_relay_send_failure(
                                                stats,
                                                peer,
                                                error.to_string(),
                                            );
                                        }
                                    }
                                }
                                if !direct_sent {
                                    if relay_send_blocked_until
                                        .get(&peer.peer_node_id)
                                        .is_some_and(|until| Instant::now() < *until)
                                    {
                                        stats.last_tun_drop_reason =
                                            Some("relay_peer_not_attached_backoff".to_string());
                                        continue;
                                    }
                                    let mut relay_sent = false;
                                    if let Some(payload) = encode_relay_forward(peer, &frame) {
                                        for attempt in 0..relay_send_attempt_count(&packet) {
                                            if attempt > 0 {
                                                thread::sleep(relay_send_attempt_delay(&packet));
                                            }
                                            match peer.socket.send(&payload) {
                                                Ok(_) => {
                                                    relay_sent = true;
                                                    macos_trace!(
                                                        "SLAN_MACOS_RELAY_SEND_OK peer={} dst={:?} size={} attempt={}",
                                                        peer.peer_node_id,
                                                        ipv4_destination(&packet),
                                                        packet.len(),
                                                        attempt + 1
                                                    );
                                                    if !logged_first_tun_send {
                                                        macos_trace!(
                                                            "SLAN_MACOS_UDP_DP_TUN_TO_RELAY_OK peer={} dst={:?} size={}",
                                                            peer.peer_node_id,
                                                            ipv4_destination(&packet),
                                                            packet.len()
                                                        );
                                                        logged_first_tun_send = true;
                                                    }
                                                    stats.last_tun_send_path = Some(
                                                        PathKind::RelayUdp.as_str().to_string(),
                                                    );
                                                    record_relay_tun_packet_sent(stats, peer);
                                                }
                                                Err(error) => {
                                                    record_relay_send_failure(stats, peer, {
                                                        eprintln!(
                                                            "SLAN_MACOS_RELAY_SEND_ERROR peer={} dst={:?} attempt={} error={}",
                                                            peer.peer_node_id,
                                                            ipv4_destination(&packet),
                                                            attempt + 1,
                                                            error
                                                        );
                                                        error.to_string()
                                                    })
                                                }
                                            }
                                        }
                                    } else {
                                        eprintln!(
                                            "SLAN_MACOS_TUN_DROP reason=relay_forward_encode_failed peer={} dst={:?}",
                                            peer.peer_node_id,
                                            ipv4_destination(&packet)
                                        );
                                        stats.last_tun_drop_reason =
                                            Some("relay_forward_encode_failed".to_string());
                                    }
                                    if !relay_sent {
                                        if let Some(derp_peer) =
                                            derp_peer_for_packet_mut(&mut derp_peers, &packet)
                                        {
                                            for attempt in 0..relay_send_attempt_count(&packet) {
                                                if attempt > 0 {
                                                    thread::sleep(relay_send_attempt_delay(
                                                        &packet,
                                                    ));
                                                }
                                                match send_derp_forward(derp_peer, &frame) {
                                                    Ok(_) => {
                                                        macos_trace!(
                                                            "SLAN_MACOS_DERP_SEND_OK peer={} dst={:?} size={} attempt={}",
                                                            derp_peer.peer_node_id,
                                                            ipv4_destination(&packet),
                                                            packet.len(),
                                                            attempt + 1
                                                        );
                                                        stats.last_tun_send_path = Some(
                                                            PathKind::DerpTcpTls443
                                                                .as_str()
                                                                .to_string(),
                                                        );
                                                        stats.last_tun_drop_reason = None;
                                                        record_derp_tun_packet_sent(
                                                            stats, derp_peer,
                                                        );
                                                    }
                                                    Err(error) => record_derp_send_failure(
                                                        stats,
                                                        derp_peer,
                                                        {
                                                            eprintln!(
                                                                "SLAN_MACOS_DERP_SEND_ERROR peer={} dst={:?} attempt={} error={}",
                                                                derp_peer.peer_node_id,
                                                                ipv4_destination(&packet),
                                                                attempt + 1,
                                                                error
                                                            );
                                                            error.to_string()
                                                        },
                                                    ),
                                                }
                                            }
                                        }
                                    }
                                }
                            } else {
                                eprintln!(
                                    "SLAN_MACOS_TUN_DROP reason=relay_frame_encode_failed peer={} dst={:?}",
                                    peer.peer_node_id,
                                    ipv4_destination(&packet)
                                );
                                stats.last_tun_drop_reason =
                                    Some("relay_frame_encode_failed".to_string());
                            }
                        } else if let Some(derp_peer) =
                            derp_peer_for_packet_mut(&mut derp_peers, packet)
                        {
                            stats.last_tun_peer_node_id = Some(derp_peer.peer_node_id.clone());
                            macos_trace!(
                                "SLAN_MACOS_TUN_PACKET_DERP peer={} dst={:?} proto={:?} size={}",
                                derp_peer.peer_node_id,
                                ipv4_destination(packet),
                                ipv4_protocol(packet),
                                packet.len()
                            );
                            if !acl_allows_egress_packet(
                                packet,
                                &acl_policies,
                                Some(acl_peer_for_derp_peer(derp_peer)),
                            ) {
                                eprintln!(
                                    "SLAN_MACOS_TUN_DROP reason=acl_egress_denied_derp peer={} dst={:?}",
                                    derp_peer.peer_node_id,
                                    ipv4_destination(packet)
                                );
                                stats.last_tun_drop_reason = Some("acl_egress_denied".to_string());
                                continue;
                            }
                            seq = seq.wrapping_add(1);
                            let packet = normalize_ipv4_transport_checksums(packet);
                            record_tun_tcp_packet(stats, &packet);
                            if let Some(frame) =
                                encode_slan_relay_data_frame(seq, config_hash, &packet)
                            {
                                for attempt in 0..relay_send_attempt_count(&packet) {
                                    if attempt > 0 {
                                        thread::sleep(relay_send_attempt_delay(&packet));
                                    }
                                    match send_derp_forward(derp_peer, &frame) {
                                        Ok(_) => {
                                            macos_trace!(
                                                "SLAN_MACOS_DERP_SEND_OK peer={} dst={:?} size={} attempt={}",
                                                derp_peer.peer_node_id,
                                                ipv4_destination(&packet),
                                                packet.len(),
                                                attempt + 1
                                            );
                                            stats.last_tun_send_path =
                                                Some(PathKind::DerpTcpTls443.as_str().to_string());
                                            record_derp_tun_packet_sent(stats, derp_peer);
                                        }
                                        Err(error) => record_derp_send_failure(stats, derp_peer, {
                                            eprintln!(
                                                    "SLAN_MACOS_DERP_SEND_ERROR peer={} dst={:?} attempt={} error={}",
                                                    derp_peer.peer_node_id,
                                                    ipv4_destination(&packet),
                                                    attempt + 1,
                                                    error
                                                );
                                            error.to_string()
                                        }),
                                    }
                                }
                            } else {
                                eprintln!(
                                    "SLAN_MACOS_TUN_DROP reason=relay_frame_encode_failed_derp peer={} dst={:?}",
                                    derp_peer.peer_node_id,
                                    ipv4_destination(&packet)
                                );
                                stats.last_tun_drop_reason =
                                    Some("relay_frame_encode_failed".to_string());
                            }
                        } else if let Some(destination) = ipv4_destination(packet) {
                            if should_ignore_unroutable_destination(&destination) {
                                macos_trace!(
                                    "SLAN_MACOS_TUN_DROP reason=ignored_unroutable dst={}",
                                    destination
                                );
                                stats.last_tun_drop_reason =
                                    Some("ignored_unroutable_destination".to_string());
                                continue;
                            }
                            eprintln!(
                                "SLAN_MACOS_TUN_DROP reason=unroutable dst={} proto={:?} size={}",
                                destination,
                                ipv4_protocol(packet),
                                packet.len()
                            );
                            stats.unroutable_tun_packets =
                                stats.unroutable_tun_packets.saturating_add(1);
                            stats.last_unroutable_destination = Some(destination);
                            stats.last_tun_drop_reason = Some("unroutable".to_string());
                        }
                    } else {
                        eprintln!(
                            "SLAN_MACOS_TUN_DROP reason=oversized dst={:?} size={} max={}",
                            ipv4_destination(packet),
                            packet.len(),
                            max_frame_payload
                        );
                        stats.oversized_tun_packets = stats.oversized_tun_packets.saturating_add(1);
                        stats.last_oversized_tun_packet_size = Some(packet.len() as u32);
                        stats.last_tun_drop_reason = Some("oversized".to_string());
                    }
                }
            }
            Err(error) if error.kind() == ErrorKind::WouldBlock => {}
            Err(error) if error.kind() == ErrorKind::Interrupted => {}
            Err(_) => break,
        }
        for peer in &peers {
            match peer.socket.recv(&mut relay_buffer) {
                Ok(frame_len) => {
                    did_work = true;
                    if let Some(error) = relay_error_message(&relay_buffer[..frame_len]) {
                        stats.relay_error_responses = stats.relay_error_responses.saturating_add(1);
                        stats.last_relay_error = Some(format!(
                            "peer {} session {}: {error}",
                            peer.peer_node_id, peer.session_id
                        ));
                        if let Some(peer_stats) = relay_peer_stats_mut(stats, peer) {
                            peer_stats.relay_errors = peer_stats.relay_errors.saturating_add(1);
                            peer_stats.last_relay_error = Some(error.clone());
                        }
                        if error.contains("peer not attached")
                            || error.contains("participant not attached")
                        {
                            relay_send_blocked_until.insert(
                                peer.peer_node_id.clone(),
                                Instant::now() + RELAY_PEER_NOT_ATTACHED_COOLDOWN,
                            );
                        }
                        continue;
                    }
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
                        if !logged_first_relay_recv {
                            macos_trace!(
                                "SLAN_MACOS_UDP_DP_RELAY_TO_TUN_RX peer={} dst={:?} size={}",
                                peer.peer_node_id,
                                ipv4_destination(packet),
                                packet.len()
                            );
                            logged_first_relay_recv = true;
                        }
                        stats.last_relay_packet_at_ms = Some(current_timestamp_ms());
                        stats.last_relay_peer_node_id = Some(peer.peer_node_id.clone());
                        stats.last_relay_destination = ipv4_destination(packet);
                        stats.last_relay_protocol = ipv4_protocol(packet);
                        stats.last_relay_packet_size = Some(packet.len() as u32);
                        stats.last_relay_tcp_flags = ipv4_tcp_flags(packet).map(tcp_flags_summary);
                        record_relay_tcp_packet(stats, packet);
                        if stats.last_relay_protocol == Some(6) {
                            macos_trace!(
                                "SLAN_MACOS_RELAY_TCP_RX peer={} dst={:?} flags={} bytes={} checksum_valid={:?}",
                                peer.peer_node_id,
                                stats.last_relay_destination,
                                stats.last_relay_tcp_flags.as_deref().unwrap_or("NONE"),
                                packet.len(),
                                ipv4_transport_checksum_valid(packet)
                            );
                        }
                        if !acl_allows_ingress_packet(
                            packet,
                            &acl_policies,
                            Some(acl_peer_for_relay_peer(peer)),
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
                            Some(acl_peer_for_relay_peer(peer)),
                        ) {
                            stats.last_tun_drop_reason = Some("acl_ingress_denied".to_string());
                            continue;
                        }
                        match write_utun_ipv4_packet(&mut file, &packet) {
                            Ok(_) => {
                                macos_trace!(
                                    "SLAN_MACOS_RELAY_UDP_TUN_WRITE_OK peer={} dst={:?} flags={} bytes={} checksum_valid={:?}",
                                    peer.peer_node_id,
                                    ipv4_destination(&packet),
                                    ipv4_tcp_flags(&packet)
                                        .map(tcp_flags_summary)
                                        .unwrap_or_else(|| "NONE".to_string()),
                                    packet.len(),
                                    ipv4_transport_checksum_valid(&packet)
                                );
                                if !logged_first_utun_write {
                                    macos_trace!(
                                        "SLAN_MACOS_UDP_DP_TUN_WRITE_OK peer={} dst={:?} size={}",
                                        peer.peer_node_id,
                                        ipv4_destination(&packet),
                                        packet.len()
                                    );
                                    logged_first_utun_write = true;
                                }
                                stats.last_utun_write_error = None;
                                record_relay_packet_received(stats, peer)
                            }
                            Err(error) => {
                                macos_trace!(
                                    "SLAN_MACOS_RELAY_UDP_TUN_WRITE_ERROR peer={} dst={:?} flags={} bytes={} error={}",
                                    peer.peer_node_id,
                                    ipv4_destination(&packet),
                                    ipv4_tcp_flags(&packet)
                                        .map(tcp_flags_summary)
                                        .unwrap_or_else(|| "NONE".to_string()),
                                    packet.len(),
                                    error
                                );
                                eprintln!(
                                    "SLAN_MACOS_UDP_DP_TUN_WRITE_ERROR peer={} error={}",
                                    peer.peer_node_id, error
                                );
                                stats.last_utun_write_error = Some(error.to_string());
                                record_relay_write_failure(stats, peer)
                            }
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
                    did_work = true;
                    if let Some(packet) = decode_slan_relay_data_frame(&frame) {
                        stats.last_relay_packet_at_ms = Some(current_timestamp_ms());
                        stats.last_relay_peer_node_id = Some(peer.peer_node_id.clone());
                        stats.last_relay_destination = ipv4_destination(packet);
                        stats.last_relay_protocol = ipv4_protocol(packet);
                        stats.last_relay_packet_size = Some(packet.len() as u32);
                        stats.last_relay_tcp_flags = ipv4_tcp_flags(packet).map(tcp_flags_summary);
                        record_relay_tcp_packet(stats, packet);
                        if stats.last_relay_protocol == Some(6) {
                            macos_trace!(
                                "SLAN_MACOS_DERP_TCP_RX peer={} dst={:?} flags={} bytes={} checksum_valid={:?}",
                                peer.peer_node_id,
                                stats.last_relay_destination,
                                stats.last_relay_tcp_flags.as_deref().unwrap_or("NONE"),
                                packet.len(),
                                ipv4_transport_checksum_valid(packet)
                            );
                        }
                        if !acl_allows_ingress_packet(
                            packet,
                            &acl_policies,
                            Some(acl_peer_for_derp_peer(peer)),
                        ) {
                            stats.last_tun_drop_reason = Some("acl_ingress_denied".to_string());
                            continue;
                        }
                        let packet = normalize_ipv4_transport_checksums(packet);
                        if !acl_allows_ingress_packet(
                            &packet,
                            &acl_policies,
                            Some(acl_peer_for_derp_peer(peer)),
                        ) {
                            stats.last_tun_drop_reason = Some("acl_ingress_denied".to_string());
                            continue;
                        }
                        match write_utun_ipv4_packet(&mut file, &packet) {
                            Ok(_) => {
                                macos_trace!(
                                    "SLAN_MACOS_DERP_TUN_WRITE_OK peer={} dst={:?} flags={} bytes={} checksum_valid={:?}",
                                    peer.peer_node_id,
                                    ipv4_destination(&packet),
                                    ipv4_tcp_flags(&packet)
                                        .map(tcp_flags_summary)
                                        .unwrap_or_else(|| "NONE".to_string()),
                                    packet.len(),
                                    ipv4_transport_checksum_valid(&packet)
                                );
                                stats.last_utun_write_error = None;
                                record_derp_packet_received(stats, peer)
                            }
                            Err(error) => {
                                macos_trace!(
                                    "SLAN_MACOS_DERP_TUN_WRITE_ERROR peer={} dst={:?} flags={} bytes={} error={}",
                                    peer.peer_node_id,
                                    ipv4_destination(&packet),
                                    ipv4_tcp_flags(&packet)
                                        .map(tcp_flags_summary)
                                        .unwrap_or_else(|| "NONE".to_string()),
                                    packet.len(),
                                    error
                                );
                                stats.last_utun_write_error = Some(error.to_string());
                                record_derp_write_failure(stats, peer)
                            }
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
                    did_work = true;
                    let frame_len = received.frame_len;
                    let control_packet = direct_udp_control_packet(&relay_buffer[..frame_len]);
                    if let Some(packet) = control_packet
                        .as_ref()
                        .filter(|packet| packet.kind == DirectUdpControlKind::Probe)
                    {
                        direct_udp.mark_peer_ready(received.peer_index, received.remote_addr, None);
                        stats.direct_udp_ready_peer_count = direct_udp.ready_peer_count() as u64;
                        stats.direct_udp_probes_received =
                            stats.direct_udp_probes_received.saturating_add(1);
                        if direct_udp.send_pong_to_remote(received.remote_addr, packet.probe_id) {
                            stats.direct_udp_pongs_sent =
                                stats.direct_udp_pongs_sent.saturating_add(1);
                        }
                        mark_direct_peer_ready(stats, direct_udp, received.peer_index);
                    } else if let Some(packet) = control_packet
                        .as_ref()
                        .filter(|packet| packet.kind == DirectUdpControlKind::Pong)
                    {
                        direct_udp.mark_peer_ready(
                            received.peer_index,
                            received.remote_addr,
                            Some(packet.probe_id),
                        );
                        stats.direct_udp_ready_peer_count = direct_udp.ready_peer_count() as u64;
                        stats.direct_udp_pongs_received =
                            stats.direct_udp_pongs_received.saturating_add(1);
                        mark_direct_peer_ready(stats, direct_udp, received.peer_index);
                    } else if let Some(packet) =
                        decode_slan_relay_data_frame(&relay_buffer[..frame_len])
                    {
                        direct_udp.mark_peer_ready(received.peer_index, received.remote_addr, None);
                        stats.direct_udp_ready_peer_count = direct_udp.ready_peer_count() as u64;
                        stats.direct_udp_frames_received =
                            stats.direct_udp_frames_received.saturating_add(1);
                        mark_direct_peer_ready(stats, direct_udp, received.peer_index);
                        stats.last_relay_packet_at_ms = Some(current_timestamp_ms());
                        stats.last_relay_peer_node_id = direct_udp
                            .peers
                            .get(received.peer_index)
                            .map(|peer| peer.peer_node_id.clone());
                        stats.last_relay_destination = ipv4_destination(packet);
                        stats.last_relay_protocol = ipv4_protocol(packet);
                        stats.last_relay_packet_size = Some(packet.len() as u32);
                        stats.last_relay_tcp_flags = ipv4_tcp_flags(packet).map(tcp_flags_summary);
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
                        if write_utun_ipv4_packet(&mut file, &packet).is_ok() {
                            let peer_node_id = direct_udp
                                .peers
                                .get(received.peer_index)
                                .map(|peer| peer.peer_node_id.as_str())
                                .unwrap_or("unknown");
                            macos_trace!(
                                "SLAN_MACOS_DIRECT_UDP_TUN_WRITE_OK peer={} dst={:?} flags={} bytes={} checksum_valid={:?}",
                                peer_node_id,
                                ipv4_destination(&packet),
                                ipv4_tcp_flags(&packet)
                                    .map(tcp_flags_summary)
                                    .unwrap_or_else(|| "NONE".to_string()),
                                packet.len(),
                                ipv4_transport_checksum_valid(&packet)
                            );
                            stats.last_utun_write_error = None;
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
                        } else {
                            let peer_node_id = direct_udp
                                .peers
                                .get(received.peer_index)
                                .map(|peer| peer.peer_node_id.as_str())
                                .unwrap_or("unknown");
                            macos_trace!(
                                "SLAN_MACOS_DIRECT_UDP_TUN_WRITE_ERROR peer={} dst={:?} flags={} bytes={}",
                                peer_node_id,
                                ipv4_destination(&packet),
                                ipv4_tcp_flags(&packet)
                                    .map(tcp_flags_summary)
                                    .unwrap_or_else(|| "NONE".to_string()),
                                packet.len()
                            );
                            stats.last_utun_write_error =
                                Some("direct_udp utun write failed".to_string());
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
        if !did_work {
            wait_for_data_plane_activity(&mut poll_fds);
        }
        if last_stats_flush.elapsed() >= RELAY_STATS_FLUSH_INTERVAL {
            persist_relay_stats_async(stats);
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
        Some("error") => Err(std::io::Error::other(
            value
                .get("error")
                .and_then(|error| error.get("message"))
                .and_then(serde_json::Value::as_str)
                .unwrap_or("DERP error")
                .to_string(),
        )),
        _ => Ok(None),
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

fn json_io_error(error: serde_json::Error) -> std::io::Error {
    std::io::Error::new(ErrorKind::InvalidData, error)
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

fn relay_error_message(payload: &[u8]) -> Option<String> {
    let value: serde_json::Value = serde_json::from_slice(payload).ok()?;
    if value.get("kind").and_then(serde_json::Value::as_str) != Some("error") {
        return None;
    }
    Some(
        value
            .get("error")
            .and_then(|error| error.get("message"))
            .or_else(|| value.get("message"))
            .and_then(serde_json::Value::as_str)
            .unwrap_or("relay_error")
            .to_string(),
    )
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

fn record_direct_tun_packet_sent(
    stats: &mut RelayDataPlaneStats,
    peer: &RelayPeer,
    path_kind: PathKind,
) {
    stats.tun_packets_sent = stats.tun_packets_sent.saturating_add(1);
    stats.direct_udp_frames_sent = stats.direct_udp_frames_sent.saturating_add(1);
    stats.active_path = Some(path_kind.as_str().to_string());
    if let Some(peer_stats) = relay_peer_stats_mut(stats, peer) {
        peer_stats.tun_packets_sent = peer_stats.tun_packets_sent.saturating_add(1);
        if peer_stats.last_send_path.as_deref() != Some(path_kind.as_str()) {
            peer_stats.path_upgrades = peer_stats.path_upgrades.saturating_add(1);
            peer_stats.last_path_change = Some(format!(
                "{} -> {} after direct udp ready",
                peer_stats
                    .last_send_path
                    .as_deref()
                    .unwrap_or(PathKind::RelayUdp.as_str()),
                path_kind.as_str()
            ));
        }
        peer_stats.last_send_path = Some(path_kind.as_str().to_string());
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
    stats.active_path = Some(peer.path_kind.as_str().to_string());
    if let Some(peer_stats) = relay_peer_stats_mut_by_node_id(stats, peer.peer_node_id.as_str()) {
        if peer_stats.last_send_path.as_deref() != Some(peer.path_kind.as_str()) {
            peer_stats.path_upgrades = peer_stats.path_upgrades.saturating_add(1);
            peer_stats.last_path_change = Some(format!(
                "{} -> {} after probe success",
                peer_stats
                    .last_send_path
                    .as_deref()
                    .unwrap_or(PathKind::RelayUdp.as_str()),
                peer.path_kind.as_str()
            ));
        }
        peer_stats.last_send_path = Some(peer.path_kind.as_str().to_string());
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
    if flags & 0x12 == 0x12 {
        stats.relay_tcp_syn_ack_received = stats.relay_tcp_syn_ack_received.saturating_add(1);
    }
    if flags & 0x08 != 0 {
        stats.relay_tcp_psh_received = stats.relay_tcp_psh_received.saturating_add(1);
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

fn tcp_flags_summary(flags: u8) -> String {
    let mut parts = Vec::new();
    if flags & 0x01 != 0 {
        parts.push("FIN");
    }
    if flags & 0x02 != 0 {
        parts.push("SYN");
    }
    if flags & 0x04 != 0 {
        parts.push("RST");
    }
    if flags & 0x08 != 0 {
        parts.push("PSH");
    }
    if flags & 0x10 != 0 {
        parts.push("ACK");
    }
    if flags & 0x20 != 0 {
        parts.push("URG");
    }
    if parts.is_empty() {
        "NONE".to_string()
    } else {
        parts.join("|")
    }
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
    let Ok(payload) = serde_json::to_vec_pretty(stats) else {
        return;
    };
    write_relay_stats(payload);
}

fn persist_relay_stats_async(stats: &mut RelayDataPlaneStats) {
    stats.updated_at_ms = current_timestamp_ms();
    let Ok(payload) = serde_json::to_vec_pretty(stats) else {
        return;
    };
    let _ = thread::Builder::new()
        .name("slan-relay-stats".to_string())
        .spawn(move || write_relay_stats(payload));
}

fn write_relay_stats(payload: Vec<u8>) {
    let path = relay_stats_file_path();
    let Some(parent) = path.parent() else {
        return;
    };
    if fs::create_dir_all(parent).is_err() {
        return;
    }
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
    let destination = ipv4_destination_addr(packet)?;
    peers
        .iter()
        .find(|peer| peer.peer_virtual_ipv4s.contains(&destination))
}

fn acl_peer_for_relay_peer(peer: &RelayPeer) -> &PlatformAclPeer {
    &peer.acl_peer
}

fn acl_peer_for_derp_peer(peer: &DerpPeer) -> &PlatformAclPeer {
    &peer.acl_peer
}

fn run_local_data_plane(
    mut file: File,
    local_virtual_ip: String,
    dns_servers: Vec<String>,
    dns_records: Vec<PlatformResolverRecord>,
    mut direct_udp: Option<DirectUdpTransport>,
    direct_udp_probe_interval: Duration,
    direct_network_id: String,
    node_configs: Vec<NodeConfig>,
    config_hash: u64,
    acl_policies: Vec<PlatformAclPolicy>,
    stop: Arc<AtomicBool>,
) {
    let mut seq = 0_u64;
    let mut tun_buffer = vec![0_u8; MAX_PACKET_SIZE + UTUN_HEADER_LEN];
    let mut relay_buffer = vec![0_u8; MAX_PACKET_SIZE + 512];
    let mut last_direct_udp_probe = Instant::now()
        .checked_sub(direct_udp_probe_interval)
        .unwrap_or_else(Instant::now);
    let mut poll_fds = data_plane_poll_fds(&file, &[], &[], direct_udp.as_ref());
    while !stop.load(Ordering::SeqCst) {
        let mut did_work = false;
        if last_direct_udp_probe.elapsed() >= direct_udp_probe_interval {
            if let Some(direct_udp) = direct_udp.as_mut() {
                for peer_node_id in direct_udp.poll_probe_health(current_timestamp_ms()) {
                    macos_trace!(
                        "SLAN_MACOS_LOCAL_DIRECT_UDP_PEER_FAILED peer={}",
                        peer_node_id
                    );
                }
                let _ = direct_udp.send_punch_endpoint_probes(&direct_network_id, &node_configs);
                let _ = direct_udp.send_probe_packets();
            }
            last_direct_udp_probe = Instant::now();
        }
        match file.read(&mut tun_buffer) {
            Ok(0) => {}
            Ok(packet_len) => {
                did_work = true;
                if let Some(packet) = strip_utun_header(&tun_buffer[..packet_len]) {
                    if let Some(reply) = local_dns_reply(packet, &dns_servers, &dns_records) {
                        let _ = write_utun_ipv4_packet(&mut file, &reply);
                    } else if let Some(reply) =
                        local_virtual_ip_reply(packet, local_virtual_ip.as_str())
                    {
                        let _ = write_utun_ipv4_packet(&mut file, &reply);
                    } else if packet_targets_local_virtual_ip(packet, local_virtual_ip.as_str()) {
                        let packet = normalize_ipv4_transport_checksums(packet);
                        let _ = write_utun_ipv4_packet(&mut file, &packet);
                    } else if let Some(direct_peer_index) = direct_udp
                        .as_ref()
                        .and_then(|transport| transport.ready_peer_index_for_packet(packet))
                    {
                        let packet = normalize_ipv4_transport_checksums(packet);
                        let acl_peer = direct_udp
                            .as_ref()
                            .and_then(|transport| transport.peers.get(direct_peer_index))
                            .map(|peer| PlatformAclPeer {
                                peer_node_id: Some(peer.peer_node_id.clone()),
                                peer_virtual_ips: peer.peer_virtual_ips.clone(),
                            });
                        if !acl_allows_egress_packet(&packet, &acl_policies, acl_peer.as_ref()) {
                            continue;
                        }
                        seq = seq.wrapping_add(1);
                        if let Some(frame) = encode_slan_relay_data_frame(seq, config_hash, &packet)
                        {
                            if let Some(transport) = direct_udp.as_ref() {
                                let _ = transport.send_to_peer(direct_peer_index, &frame);
                            }
                        }
                    }
                }
            }
            Err(error) if error.kind() == ErrorKind::WouldBlock => {}
            Err(error) if error.kind() == ErrorKind::Interrupted => {}
            Err(_) => break,
        }
        if let Some(direct_udp) = direct_udp.as_mut() {
            match direct_udp.recv_from_peer(&mut relay_buffer) {
                Ok(Some(received)) => {
                    did_work = true;
                    let frame_len = received.frame_len;
                    let control_packet = direct_udp_control_packet(&relay_buffer[..frame_len]);
                    if let Some(packet) = control_packet
                        .as_ref()
                        .filter(|packet| packet.kind == DirectUdpControlKind::Probe)
                    {
                        direct_udp.mark_peer_ready(received.peer_index, received.remote_addr, None);
                        let _ =
                            direct_udp.send_pong_to_remote(received.remote_addr, packet.probe_id);
                    } else if let Some(packet) = control_packet
                        .as_ref()
                        .filter(|packet| packet.kind == DirectUdpControlKind::Pong)
                    {
                        direct_udp.mark_peer_ready(
                            received.peer_index,
                            received.remote_addr,
                            Some(packet.probe_id),
                        );
                    } else if let Some(packet) =
                        decode_slan_relay_data_frame(&relay_buffer[..frame_len])
                    {
                        direct_udp.mark_peer_ready(received.peer_index, received.remote_addr, None);
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
                            continue;
                        }
                        let _ = write_utun_ipv4_packet(&mut file, &packet);
                    }
                }
                Ok(None) => {}
                Err(error) if error.kind() == ErrorKind::WouldBlock => {}
                Err(error) if error.kind() == ErrorKind::Interrupted => {}
                Err(_) => {}
            }
        }
        if !did_work {
            wait_for_data_plane_activity(&mut poll_fds);
        }
    }
}

fn data_plane_poll_fds(
    file: &File,
    relay_peers: &[RelayPeer],
    derp_peers: &[DerpPeer],
    direct_udp: Option<&DirectUdpTransport>,
) -> Vec<libc::pollfd> {
    let mut fds = Vec::with_capacity(
        1 + relay_peers.len() + derp_peers.len() + usize::from(direct_udp.is_some()),
    );
    fds.push(read_poll_fd(file.as_raw_fd()));
    fds.extend(
        relay_peers
            .iter()
            .map(|peer| read_poll_fd(peer.socket.as_raw_fd())),
    );
    fds.extend(
        derp_peers
            .iter()
            .map(|peer| read_poll_fd(peer.reader.as_raw_fd())),
    );
    if let Some(direct_udp) = direct_udp {
        fds.push(read_poll_fd(direct_udp.socket.as_raw_fd()));
    }
    fds
}

fn read_poll_fd(fd: RawFd) -> libc::pollfd {
    libc::pollfd {
        fd,
        events: libc::POLLIN,
        revents: 0,
    }
}

fn wait_for_data_plane_activity(fds: &mut [libc::pollfd]) {
    let timeout_ms = i32::try_from(DATA_PLANE_IDLE_SLEEP.as_millis()).unwrap_or(1);
    // The descriptors remain owned by the data-plane runtime for this thread's lifetime.
    unsafe {
        libc::poll(fds.as_mut_ptr(), fds.len() as libc::nfds_t, timeout_ms);
    }
}

fn derp_peer_for_packet_mut<'a>(
    peers: &'a mut [DerpPeer],
    packet: &[u8],
) -> Option<&'a mut DerpPeer> {
    let destination = ipv4_destination_addr(packet)?;
    peers
        .iter_mut()
        .find(|peer| peer.peer_virtual_ipv4s.contains(&destination))
}

fn packet_targets_local_virtual_ip(packet: &[u8], local_virtual_ip: &str) -> bool {
    ipv4_destination_addr(packet)
        .map(|destination| parse_config_virtual_ipv4(local_virtual_ip) == Some(destination))
        .unwrap_or(false)
}

fn local_dns_reply(
    packet: &[u8],
    dns_servers: &[String],
    dns_records: &[PlatformResolverRecord],
) -> Option<Vec<u8>> {
    for dns_server in dns_servers {
        if let Some(reply) = resolver_response_for_query(packet, dns_server, dns_records) {
            return Some(reply);
        }
    }
    None
}

fn local_virtual_ip_reply(packet: &[u8], local_virtual_ip: &str) -> Option<Vec<u8>> {
    icmp_echo_reply_for_request(packet, local_virtual_ip)
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

fn flush_macos_dns_cache() {
    let _ = run_command("/usr/bin/dscacheutil", &["-flushcache"]);
    let _ = run_command("/usr/bin/killall", &["-HUP", "mDNSResponder"]);
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
    use std::{
        net::Ipv4Addr,
        sync::{Mutex, OnceLock},
    };

    use client_core::RelayTicket;

    use super::*;

    fn test_lock() -> std::sync::MutexGuard<'static, ()> {
        static LOCK: OnceLock<Mutex<()>> = OnceLock::new();
        LOCK.get_or_init(|| Mutex::new(()))
            .lock()
            .expect("macos platform test mutex poisoned")
    }

    fn reset_runtime() {
        let mut runtime = runtime()
            .lock()
            .expect("macos network runtime mutex poisoned");
        *runtime = MacosRuntime::default();
    }

    #[test]
    fn utun_runtime_drop_waits_for_cooperative_data_plane_shutdown() {
        let stop = Arc::new(AtomicBool::new(false));
        let thread_stop = Arc::clone(&stop);
        let handle = thread::spawn(move || {
            while !thread_stop.load(Ordering::SeqCst) {
                thread::sleep(DATA_PLANE_IDLE_SLEEP);
            }
        });
        let runtime = UtunRuntime {
            interface_name: "utun-test".to_string(),
            file: None,
            stop: Arc::clone(&stop),
            handle: Some(handle),
        };

        let started = Instant::now();
        drop(runtime);

        assert!(stop.load(Ordering::SeqCst));
        assert!(started.elapsed() < Duration::from_secs(1));
    }

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
    fn parses_route_interface() {
        let output = r#"   route to: 10.0.1.137
destination: 10.0.1.137
  interface: utun6
      flags: <UP,HOST,DONE,STATIC>
"#;

        assert_eq!(parse_route_interface(output).as_deref(), Some("utun6"));
        assert_eq!(parse_route_interface("route unavailable"), None);
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

    #[test]
    fn macos_local_virtual_ip_replies_to_icmp_echo() {
        let request = icmp_echo_request("10.0.0.9", "10.0.0.2");
        let reply = local_virtual_ip_reply(&request, "10.0.0.2/32").unwrap();

        assert_eq!(&reply[12..16], &[10, 0, 0, 2]);
        assert_eq!(&reply[16..20], &[10, 0, 0, 9]);
        assert_eq!(reply[20], 0);
        assert!(local_virtual_ip_reply(&request, "10.0.0.3/32").is_none());
    }

    #[test]
    fn macos_mock_runtime_keeps_configured_dns_service() {
        let _guard = test_lock();
        std::env::set_var("SLAN_MACOS_NETWORK_MOCK", "1");
        reset_runtime();

        let platform = MacosPlatformNetwork;
        platform.install_adapter().unwrap();
        platform.configure_ip("10.0.0.1", 32).unwrap();
        platform
            .configure_resolver(&client_core::PlatformResolverConfig {
                servers: vec!["10.0.0.53".to_string()],
                search_domains: vec!["slan.test".to_string()],
                split_domains: vec!["slan.test".to_string()],
                ..client_core::PlatformResolverConfig::default()
            })
            .unwrap();
        platform
            .configure_resolver_map(
                &[],
                &[PlatformResolverRecord {
                    fqdn: Some("api.slan.test".to_string()),
                    target_ip: Some("10.0.1.2".to_string()),
                    ..PlatformResolverRecord::default()
                }],
            )
            .unwrap();

        let diagnostics = platform.diagnostics().unwrap();
        assert_eq!(diagnostics.resolver_servers, vec!["10.0.0.53"]);

        let runtime_guard = runtime()
            .lock()
            .expect("macos network runtime mutex poisoned");
        assert_eq!(
            runtime_guard.resolver_servers,
            vec!["10.0.0.53".to_string()]
        );
        drop(runtime_guard);

        platform.disable_network().unwrap();
        let runtime_guard = runtime()
            .lock()
            .expect("macos network runtime mutex poisoned");
        assert!(runtime_guard.resolver_servers.is_empty());
        assert!(runtime_guard.resolver_search_domains.is_empty());
        assert!(runtime_guard.resolver_split_domains.is_empty());
        assert!(runtime_guard.resolver_records.is_empty());
        drop(runtime_guard);
        std::env::remove_var("SLAN_MACOS_NETWORK_MOCK");
        reset_runtime();
    }

    #[test]
    fn scutil_dns_script_includes_search_and_split_domains() {
        let script = build_scutil_dns_script(
            &["10.0.0.53".to_string()],
            &["corp.lan".to_string()],
            &["mesh.local".to_string()],
        );

        assert!(script.contains("d.add ServerAddresses * 10.0.0.53"));
        assert!(script.contains("d.add SearchDomains * corp.lan"));
        assert!(script.contains("d.add SupplementalMatchDomains * mesh.local"));
    }

    #[test]
    fn resolver_routes_include_unique_non_loopback_ipv4_servers() {
        let routes = resolver_route_specs(&[
            "10.0.0.53".to_string(),
            " 10.0.0.53 ".to_string(),
            "127.0.0.1".to_string(),
            "0.0.0.0".to_string(),
            "::1".to_string(),
            "invalid".to_string(),
        ]);

        assert_eq!(
            routes,
            vec![RouteSpec {
                destination: "10.0.0.53/32".to_string(),
                gateway: None,
            }]
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
