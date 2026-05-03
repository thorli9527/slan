use std::{
    ffi::{c_void, OsStr},
    fs,
    io::{Read, Write},
    net::{SocketAddr, TcpStream, ToSocketAddrs, UdpSocket},
    os::windows::{ffi::OsStrExt, process::CommandExt},
    path::{Path, PathBuf},
    process::Command,
    sync::{
        atomic::{AtomicBool, Ordering},
        Arc, Mutex, OnceLock,
    },
    thread,
    thread::JoinHandle,
    time::{Duration, Instant, SystemTime, UNIX_EPOCH},
};

use anyhow::{bail, Context, Result};
use client_core::{
    ipv4_destination, ipv4_source, mark_path_ready_for_nodes, mark_peer_path_probe_success,
    normalize_virtual_ip,
    relay_frame::{decode_slan_relay_data_frame_full, encode_slan_relay_data_frame, stable_hash64},
    relay_frame_is_replayed, relay_peer_index_for_packet, selected_runtime_paths,
    update_peer_active_path, NetworkRuntimeState, PathKind, PathPolicy, PathState, PathTracker,
    PeerPathRuntime, PlatformNetwork, PlatformNetworkDiagnostics, RelayDataPlaneConfig,
    RelayPeerSession, RouteSpec,
};
use libloading::Library;
use serde::Serialize;

const DEFAULT_INTERFACE_NAME: &str = "SLAN LAN Adapter";
const WINDOWS_WINTUN_DRIVER_TYPE: &str = "Wintun";
const CREATE_NO_WINDOW: u32 = 0x08000000;
const RELAY_ATTACH_ATTEMPTS: usize = 3;
const RELAY_ATTACH_TIMEOUT: Duration = Duration::from_secs(2);
const RELAY_STATS_FLUSH_INTERVAL: Duration = Duration::from_secs(10);
const DIRECT_UDP_PROBE_INTERVAL: Duration = Duration::from_secs(15);
const RELAY_DATA_PLANE_FAILURE_THRESHOLD: u32 = 20;
const PATH_SEND_FAILURES_BEFORE_DOWNGRADE: u32 = 3;
const DIRECT_UDP_PROBE_PACKET: &[u8] = b"slan-direct-udp-probe-v1";
const DIRECT_UDP_PONG_PACKET: &[u8] = b"slan-direct-udp-pong-v1";
const RELAY_TCP_FRAME_SIZE_LIMIT: usize = 64 * 1024;

type WintunAdapterHandle = *mut c_void;
type WintunCreateAdapterFunc =
    unsafe extern "system" fn(*const u16, *const u16, *const RawGuid) -> WintunAdapterHandle;
type WintunOpenAdapterFunc = unsafe extern "system" fn(*const u16) -> WintunAdapterHandle;
type WintunCloseAdapterFunc = unsafe extern "system" fn(WintunAdapterHandle);
type WintunSessionHandle = *mut c_void;
type WintunStartSessionFunc =
    unsafe extern "system" fn(WintunAdapterHandle, u32) -> WintunSessionHandle;
type WintunEndSessionFunc = unsafe extern "system" fn(WintunSessionHandle);
type WintunReceivePacketFunc = unsafe extern "system" fn(WintunSessionHandle, *mut u32) -> *mut u8;
type WintunReleaseReceivePacketFunc = unsafe extern "system" fn(WintunSessionHandle, *const u8);
type WintunAllocateSendPacketFunc = unsafe extern "system" fn(WintunSessionHandle, u32) -> *mut u8;
type WintunSendPacketFunc = unsafe extern "system" fn(WintunSessionHandle, *const u8);

#[repr(C)]
struct RawGuid {
    data1: u32,
    data2: u16,
    data3: u16,
    data4: [u8; 8],
}

struct WintunRuntime {
    _library: Library,
    handle: WintunAdapterHandle,
    session: WintunSessionHandle,
    end_session: WintunEndSessionFunc,
    close_adapter: WintunCloseAdapterFunc,
    receive_packet: WintunReceivePacketFunc,
    release_receive_packet: WintunReleaseReceivePacketFunc,
    allocate_send_packet: WintunAllocateSendPacketFunc,
    send_packet: WintunSendPacketFunc,
    data_plane: Option<WindowsDataPlaneRuntime>,
}

struct WindowsDataPlaneRuntime {
    stop: Arc<AtomicBool>,
    handle: Option<JoinHandle<()>>,
}

impl Drop for WindowsDataPlaneRuntime {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::SeqCst);
        if let Some(handle) = self.handle.take() {
            let _ = handle.join();
        }
    }
}

static WINTUN_RUNTIME: OnceLock<Mutex<Option<WintunRuntime>>> = OnceLock::new();

unsafe impl Send for WintunRuntime {}

impl Drop for WintunRuntime {
    fn drop(&mut self) {
        let _ = self.data_plane.take();
        if !self.session.is_null() {
            unsafe {
                (self.end_session)(self.session);
            }
            self.session = std::ptr::null_mut();
        }
        if !self.handle.is_null() {
            unsafe {
                (self.close_adapter)(self.handle);
            }
            self.handle = std::ptr::null_mut();
        }
    }
}

#[derive(Debug, Clone, Default)]
pub struct WindowsPlatformNetwork;

impl PlatformNetwork for WindowsPlatformNetwork {
    fn install_adapter(&self) -> Result<()> {
        ensure_adapter_present(DEFAULT_INTERFACE_NAME)?;
        persist_state(&NetworkRuntimeState {
            adapter_present: true,
            ..load_cached_runtime_state().unwrap_or_default()
        })
    }

    fn configure_ip(&self, virtual_ip: &str, prefix_len: u8) -> Result<()> {
        ensure_installed_adapter_ready(DEFAULT_INTERFACE_NAME)
            .context("ensure installed Wintun adapter before IP")?;
        configure_adapter_ip(DEFAULT_INTERFACE_NAME, virtual_ip, prefix_len)
            .context("configure Wintun adapter IP")?;
        verify_adapter_ip(DEFAULT_INTERFACE_NAME, virtual_ip)
            .context("verify Wintun adapter IP")?;
        let mut state = load_cached_runtime_state().unwrap_or_default();
        state.adapter_present = true;
        state.network_enabled = true;
        state.virtual_ip = Some(virtual_ip.to_string());
        persist_state(&state)
    }

    fn configure_routes(&self, routes: &[RouteSpec]) -> Result<()> {
        configure_routes(DEFAULT_INTERFACE_NAME, routes).context("configure Wintun routes")?;
        persist_state(&load_cached_runtime_state().unwrap_or_default())
    }

    fn configure_dns(&self, dns_servers: &[String]) -> Result<()> {
        configure_dns(DEFAULT_INTERFACE_NAME, dns_servers).context("configure Wintun DNS")?;
        persist_state(&load_cached_runtime_state().unwrap_or_default())
    }

    fn configure_relay(&self, config: Option<&RelayDataPlaneConfig>) -> Result<()> {
        configure_wintun_data_plane(config).context("configure Wintun relay data plane")
    }

    fn disable_network(&self) -> Result<()> {
        stop_wintun_data_plane();
        disable_adapter(DEFAULT_INTERFACE_NAME)?;
        let mut state = load_cached_runtime_state().unwrap_or_default();
        state.network_enabled = false;
        state.virtual_ip = None;
        state.active_path = None;
        state.peer_paths.clear();
        persist_state(&state)
    }

    fn read_runtime_state(&self) -> Result<NetworkRuntimeState> {
        match read_windows_runtime_state(DEFAULT_INTERFACE_NAME) {
            Ok(state) => {
                persist_state(&state)?;
                Ok(state)
            }
            Err(_) => load_cached_runtime_state(),
        }
    }

    fn diagnostics(&self) -> Result<PlatformNetworkDiagnostics> {
        read_windows_network_diagnostics(DEFAULT_INTERFACE_NAME)
    }
}

fn state_file_path() -> PathBuf {
    app_data_dir()
        .join("SLAN")
        .join("client-v2-network-state.json")
}

fn app_data_dir() -> PathBuf {
    std::env::var_os("ProgramData")
        .map(PathBuf::from)
        .unwrap_or_else(|| PathBuf::from(r"C:\ProgramData"))
}

fn load_cached_runtime_state() -> Result<NetworkRuntimeState> {
    let path = state_file_path();
    if !path.exists() {
        return Ok(NetworkRuntimeState::default());
    }
    let payload = fs::read(&path).with_context(|| format!("read {}", path.display()))?;
    serde_json::from_slice(&payload).with_context(|| format!("decode {}", path.display()))
}

fn persist_state(state: &NetworkRuntimeState) -> Result<()> {
    let path = state_file_path();
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent).with_context(|| format!("create {}", parent.display()))?;
    }
    let payload = serde_json::to_vec_pretty(state).context("encode network state")?;
    fs::write(&path, payload).with_context(|| format!("write {}", path.display()))
}

fn ensure_adapter_present(interface_name: &str) -> Result<()> {
    ensure_adapter_created(interface_name)?;
    ensure_installed_adapter_ready(interface_name)
}

fn ensure_installed_adapter_ready(interface_name: &str) -> Result<()> {
    let script = format!(
        "$name = '{}'; \
         $adapter = Get-NetAdapter -IncludeHidden -Name $name -ErrorAction SilentlyContinue; \
         if (-not $adapter) {{ throw \"SLAN local network adapter '$name' was not found. Please reinstall or repair SLAN Client.\" }}; \
         Enable-NetAdapter -Name $name -Confirm:$false -ErrorAction Stop | Out-Null; \
         Write-Output $adapter.Name",
        escape_powershell_single_quoted(interface_name),
    );
    run_powershell(&script).map(|_| ())
}

fn ensure_adapter_created(interface_name: &str) -> Result<()> {
    let runtime = WINTUN_RUNTIME.get_or_init(|| Mutex::new(None));
    let mut runtime = runtime.lock().expect("wintun runtime mutex poisoned");
    if runtime.is_some() {
        rename_wintun_adapter(interface_name)?;
        return Ok(());
    }

    let library = load_wintun_library()?;
    let create_adapter = unsafe {
        let symbol: libloading::Symbol<WintunCreateAdapterFunc> = library
            .get(b"WintunCreateAdapter\0")
            .map_err(|err| anyhow::anyhow!("failed to resolve WintunCreateAdapter: {err}"))?;
        *symbol
    };
    let open_adapter = unsafe {
        let symbol: libloading::Symbol<WintunOpenAdapterFunc> = library
            .get(b"WintunOpenAdapter\0")
            .map_err(|err| anyhow::anyhow!("failed to resolve WintunOpenAdapter: {err}"))?;
        *symbol
    };
    let close_adapter = unsafe {
        let symbol: libloading::Symbol<WintunCloseAdapterFunc> = library
            .get(b"WintunCloseAdapter\0")
            .map_err(|err| anyhow::anyhow!("failed to resolve WintunCloseAdapter: {err}"))?;
        *symbol
    };
    let start_session = unsafe {
        let symbol: libloading::Symbol<WintunStartSessionFunc> = library
            .get(b"WintunStartSession\0")
            .map_err(|err| anyhow::anyhow!("failed to resolve WintunStartSession: {err}"))?;
        *symbol
    };
    let end_session = unsafe {
        let symbol: libloading::Symbol<WintunEndSessionFunc> =
            library
                .get(b"WintunEndSession\0")
                .map_err(|err| anyhow::anyhow!("failed to resolve WintunEndSession: {err}"))?;
        *symbol
    };
    let receive_packet = unsafe {
        let symbol: libloading::Symbol<WintunReceivePacketFunc> = library
            .get(b"WintunReceivePacket\0")
            .map_err(|err| anyhow::anyhow!("failed to resolve WintunReceivePacket: {err}"))?;
        *symbol
    };
    let release_receive_packet = unsafe {
        let symbol: libloading::Symbol<WintunReleaseReceivePacketFunc> = library
            .get(b"WintunReleaseReceivePacket\0")
            .map_err(|err| {
                anyhow::anyhow!("failed to resolve WintunReleaseReceivePacket: {err}")
            })?;
        *symbol
    };
    let allocate_send_packet = unsafe {
        let symbol: libloading::Symbol<WintunAllocateSendPacketFunc> = library
            .get(b"WintunAllocateSendPacket\0")
            .map_err(|err| anyhow::anyhow!("failed to resolve WintunAllocateSendPacket: {err}"))?;
        *symbol
    };
    let send_packet = unsafe {
        let symbol: libloading::Symbol<WintunSendPacketFunc> =
            library
                .get(b"WintunSendPacket\0")
                .map_err(|err| anyhow::anyhow!("failed to resolve WintunSendPacket: {err}"))?;
        *symbol
    };

    let existing_name = find_wintun_adapter();
    let adapter_name = wide_null(existing_name.as_deref().unwrap_or(interface_name));
    let tunnel_type = wide_null(WINDOWS_WINTUN_DRIVER_TYPE);
    let mut handle = unsafe { open_adapter(adapter_name.as_ptr()) };
    if handle.is_null() {
        let adapter_name = wide_null(interface_name);
        handle = unsafe {
            create_adapter(
                adapter_name.as_ptr(),
                tunnel_type.as_ptr(),
                std::ptr::null(),
            )
        };
    }
    if handle.is_null() {
        bail!(
            "failed to create or open Wintun adapter '{}': {}",
            interface_name,
            std::io::Error::last_os_error()
        );
    }
    let session = unsafe { start_session(handle, 0x400000) };
    if session.is_null() {
        unsafe {
            close_adapter(handle);
        }
        bail!(
            "failed to start Wintun session for '{}': {}",
            interface_name,
            std::io::Error::last_os_error()
        );
    }
    wait_for_wintun_adapter().ok_or_else(|| {
        anyhow::anyhow!(
            "Wintun adapter '{}' was created but not visible",
            interface_name
        )
    })?;
    rename_wintun_adapter(interface_name)?;
    *runtime = Some(WintunRuntime {
        _library: library,
        handle,
        session,
        end_session,
        close_adapter,
        receive_packet,
        release_receive_packet,
        allocate_send_packet,
        send_packet,
        data_plane: None,
    });
    Ok(())
}

fn wide_null(value: &str) -> Vec<u16> {
    OsStr::new(value).encode_wide().chain([0]).collect()
}

struct AttachedRelayPeer {
    session_id: String,
    peer_node_id: String,
    peer_virtual_ips: Vec<String>,
    path_kind: PathKind,
    socket: UdpSocket,
    stats_index: usize,
    last_rx_seq: u64,
}

struct RelayUdpTransport {
    peers: Vec<AttachedRelayPeer>,
    local_node_id: String,
}

struct DirectUdpPeer {
    peer_node_id: String,
    peer_virtual_ips: Vec<String>,
    address: String,
    socket_addr: SocketAddr,
    last_rx_seq: u64,
}

struct DirectUdpTransport {
    socket: UdpSocket,
    local_node_id: String,
    peers: Vec<DirectUdpPeer>,
}

struct DirectUdpReceive {
    peer_index: usize,
    frame_len: usize,
    remote_addr: SocketAddr,
    endpoint_changed: bool,
}

struct AttachedRelayTcpPeer {
    peer_node_id: String,
    peer_virtual_ips: Vec<String>,
    stream: Mutex<TcpStream>,
    last_rx_seq: u64,
}

struct RelayTcpTransport {
    peers: Vec<AttachedRelayTcpPeer>,
}

impl DirectUdpTransport {
    #[cfg(test)]
    fn new(socket: UdpSocket, peers: Vec<DirectUdpPeer>) -> Self {
        Self {
            socket,
            local_node_id: "node-local".to_string(),
            peers,
        }
    }

    fn attach(
        local_node_id: &str,
        configured_paths: &[client_core::PeerPathConfig],
    ) -> Option<Self> {
        let mut peers = Vec::new();
        let socket = match attach_direct_udp_socket() {
            Ok(socket) => socket,
            Err(error) => {
                eprintln!("client-core-platform direct udp attach skipped error={error:#}");
                clear_direct_udp_endpoint_report();
                return None;
            }
        };
        for path in configured_paths {
            let Some(address) = direct_udp_address_for_peer(path) else {
                continue;
            };
            match resolve_direct_udp_peer_address(address.as_str()) {
                Ok(socket_addr) => peers.push(DirectUdpPeer {
                    peer_node_id: path.peer_node_id.clone(),
                    peer_virtual_ips: path.peer_virtual_ips.clone(),
                    address,
                    socket_addr,
                    last_rx_seq: 0,
                }),
                Err(error) => {
                    eprintln!(
                        "client-core-platform direct udp attach skipped peer={} error={error:#}",
                        path.peer_node_id
                    );
                }
            }
        }
        if peers.is_empty() {
            clear_direct_udp_endpoint_report();
            return None;
        }
        persist_direct_udp_endpoint_report(&socket);
        Some(Self {
            socket,
            local_node_id: local_node_id.to_string(),
            peers,
        })
    }

    fn peer_index_for_packet(&self, payload: &[u8]) -> Option<usize> {
        relay_peer_index_for_packet(
            &self
                .peers
                .iter()
                .map(|peer| peer.peer_virtual_ips.as_slice())
                .collect::<Vec<_>>(),
            payload,
        )
        .or_else(|| (self.peers.len() == 1).then_some(0))
    }

    fn send_to_peer(&self, peer_index: usize, frame: &[u8]) -> PathSendResult {
        let Some(peer) = self.peers.get(peer_index) else {
            return PathSendResult::NoRoute;
        };
        if self.socket.send_to(frame, peer.socket_addr).is_ok() {
            PathSendResult::Sent {
                peer_index,
                peer_node_id: peer.peer_node_id.clone(),
                path_kind: PathKind::DirectUdp,
            }
        } else {
            PathSendResult::SendFailed {
                peer_index,
                peer_node_id: peer.peer_node_id.clone(),
                path_kind: PathKind::DirectUdp,
            }
        }
    }

    fn recv_from_peer(&mut self, buffer: &mut [u8]) -> std::io::Result<Option<DirectUdpReceive>> {
        let (frame_len, remote_addr) = self.socket.recv_from(buffer)?;
        if let Some(peer_index) = self
            .peers
            .iter()
            .position(|peer| peer.socket_addr == remote_addr)
        {
            return Ok(Some(DirectUdpReceive {
                peer_index,
                frame_len,
                remote_addr,
                endpoint_changed: false,
            }));
        }
        Ok(self
            .peer_index_for_unknown_inbound_packet(&buffer[..frame_len])
            .map(|peer_index| DirectUdpReceive {
                peer_index,
                frame_len,
                remote_addr,
                endpoint_changed: true,
            }))
    }

    fn peer_index_for_unknown_inbound_packet(&self, frame: &[u8]) -> Option<usize> {
        if let Some(control_packet) = direct_udp_control_packet(frame) {
            if let Some(peer_node_id) = control_packet.peer_node_id() {
                if let Some(peer_index) = self
                    .peers
                    .iter()
                    .position(|peer| peer.peer_node_id == peer_node_id)
                {
                    return Some(peer_index);
                }
            }
            return (self.peers.len() == 1).then_some(0);
        }
        let decoded = decode_slan_relay_data_frame_full(frame)?;
        let source = ipv4_source(decoded.payload)?;
        self.peers.iter().position(|peer| {
            peer.peer_virtual_ips
                .iter()
                .any(|ip| normalize_virtual_ip(ip) == source)
        })
    }

    fn send_probe_packets(&self) {
        let payload = direct_udp_control_payload(DirectUdpControlKind::Probe, &self.local_node_id);
        for peer in &self.peers {
            let _ = self.socket.send_to(payload.as_bytes(), peer.socket_addr);
        }
    }

    fn send_pong_to_peer(&self, peer_index: usize) {
        if let Some(peer) = self.peers.get(peer_index) {
            let payload =
                direct_udp_control_payload(DirectUdpControlKind::Pong, &self.local_node_id);
            let _ = self.socket.send_to(payload.as_bytes(), peer.socket_addr);
        }
    }

    fn update_peer_endpoint(&mut self, peer_index: usize, remote_addr: SocketAddr) -> bool {
        let Some(peer) = self.peers.get_mut(peer_index) else {
            return false;
        };
        if peer.socket_addr == remote_addr {
            return false;
        }
        peer.socket_addr = remote_addr;
        peer.address = remote_addr.to_string();
        true
    }
}

impl RelayTcpTransport {
    #[cfg(test)]
    fn new(peers: Vec<AttachedRelayTcpPeer>) -> Self {
        Self { peers }
    }

    fn attach(
        configured_paths: &[client_core::PeerPathConfig],
        local_node_id: &str,
        sessions: &[RelayPeerSession],
    ) -> Option<Self> {
        let mut peers = Vec::new();
        for session in sessions {
            let Some(address) = relay_tcp_address_for_peer(configured_paths, &session.peer_node_id)
            else {
                continue;
            };
            match attach_tcp_relay_session(address.as_str(), local_node_id, session) {
                Ok(stream) => peers.push(AttachedRelayTcpPeer {
                    peer_node_id: session.peer_node_id.clone(),
                    peer_virtual_ips: session.peer_virtual_ips.clone(),
                    stream: Mutex::new(stream),
                    last_rx_seq: 0,
                }),
                Err(error) => {
                    eprintln!(
                        "client-core-platform relay tcp attach skipped peer={} error={error:#}",
                        session.peer_node_id
                    );
                }
            }
        }
        (!peers.is_empty()).then_some(Self { peers })
    }

    fn peer_index_for_packet(&self, payload: &[u8]) -> Option<usize> {
        relay_peer_index_for_packet(
            &self
                .peers
                .iter()
                .map(|peer| peer.peer_virtual_ips.as_slice())
                .collect::<Vec<_>>(),
            payload,
        )
        .or_else(|| (self.peers.len() == 1).then_some(0))
    }

    fn send_to_peer(&self, peer_index: usize, frame: &[u8]) -> PathSendResult {
        let Some(peer) = self.peers.get(peer_index) else {
            return PathSendResult::NoRoute;
        };
        if send_relay_tcp_frame(peer, frame) {
            PathSendResult::Sent {
                peer_index,
                peer_node_id: peer.peer_node_id.clone(),
                path_kind: PathKind::RelayTcp,
            }
        } else {
            PathSendResult::SendFailed {
                peer_index,
                peer_node_id: peer.peer_node_id.clone(),
                path_kind: PathKind::RelayTcp,
            }
        }
    }

    fn peers_mut(&mut self) -> &mut [AttachedRelayTcpPeer] {
        &mut self.peers
    }
}

struct RelayUdpAttachResult {
    transport: RelayUdpTransport,
    peer_stats: Vec<WindowsRelayPeerStats>,
    attach_failures: u64,
    last_attach_error: Option<String>,
}

impl RelayUdpTransport {
    fn new(peers: Vec<AttachedRelayPeer>, local_node_id: String) -> Self {
        Self {
            peers,
            local_node_id,
        }
    }

    fn attach(
        relay_address: &str,
        local_node_id: &str,
        sessions: &[RelayPeerSession],
    ) -> RelayUdpAttachResult {
        let mut peers = Vec::new();
        let mut peer_stats = Vec::new();
        let mut attach_failures = 0_u64;
        let mut last_attach_error = None;
        for session in sessions {
            let stats_index = peer_stats.len();
            match attach_udp_relay_session(relay_address, local_node_id, session) {
                Ok(socket) => {
                    peer_stats.push(WindowsRelayPeerStats {
                        peer_node_id: session.peer_node_id.clone(),
                        session_id: session.session_id.clone(),
                        peer_virtual_ips: session.peer_virtual_ips.clone(),
                        attached: true,
                        ..WindowsRelayPeerStats::default()
                    });
                    peers.push(AttachedRelayPeer {
                        session_id: session.session_id.clone(),
                        peer_node_id: session.peer_node_id.clone(),
                        peer_virtual_ips: session.peer_virtual_ips.clone(),
                        path_kind: PathKind::RelayUdp,
                        socket,
                        stats_index,
                        last_rx_seq: 0,
                    });
                }
                Err(error) => {
                    let message = format!(
                        "peer {} session {}: {error:#}",
                        session.peer_node_id, session.session_id
                    );
                    attach_failures = attach_failures.saturating_add(1);
                    last_attach_error = Some(message.clone());
                    peer_stats.push(WindowsRelayPeerStats {
                        peer_node_id: session.peer_node_id.clone(),
                        session_id: session.session_id.clone(),
                        peer_virtual_ips: session.peer_virtual_ips.clone(),
                        attached: false,
                        attach_error: Some(message),
                        ..WindowsRelayPeerStats::default()
                    });
                }
            }
        }
        RelayUdpAttachResult {
            transport: Self::new(peers, local_node_id.to_string()),
            peer_stats,
            attach_failures,
            last_attach_error,
        }
    }

    fn peer_count(&self) -> usize {
        self.peers.len()
    }

    fn peers(&self) -> &[AttachedRelayPeer] {
        &self.peers
    }

    fn peers_mut(&mut self) -> &mut [AttachedRelayPeer] {
        &mut self.peers
    }

    fn peer_index_for_packet(&self, payload: &[u8]) -> Option<usize> {
        relay_peer_index_for_packet(
            &self
                .peers
                .iter()
                .map(|peer| peer.peer_virtual_ips.as_slice())
                .collect::<Vec<_>>(),
            payload,
        )
        .or_else(|| (self.peers.len() == 1).then_some(0))
    }

    fn send_to_peer(&self, peer_index: usize, frame: &[u8]) -> PathSendResult {
        let Some(peer) = self.peers.get(peer_index) else {
            return PathSendResult::NoRoute;
        };
        if send_relay_udp_frame(peer, frame) {
            PathSendResult::Sent {
                peer_index,
                peer_node_id: peer.peer_node_id.clone(),
                path_kind: PathKind::RelayUdp,
            }
        } else {
            PathSendResult::SendFailed {
                peer_index,
                peer_node_id: peer.peer_node_id.clone(),
                path_kind: PathKind::RelayUdp,
            }
        }
    }
}

impl Drop for RelayUdpTransport {
    fn drop(&mut self) {
        detach_udp_relay_sessions(&self.peers, self.local_node_id.as_str());
    }
}

impl Drop for RelayTcpTransport {
    fn drop(&mut self) {
        for peer in &self.peers {
            if let Ok(stream) = peer.stream.lock() {
                let _ = stream.shutdown(std::net::Shutdown::Both);
            }
        }
    }
}

struct WindowsPathManager {
    tracker: PathTracker,
    direct_udp: Option<DirectUdpTransport>,
    relay_udp: RelayUdpTransport,
    relay_tcp: Option<RelayTcpTransport>,
}

impl WindowsPathManager {
    fn new(
        policy: PathPolicy,
        direct_udp: Option<DirectUdpTransport>,
        relay_udp: RelayUdpTransport,
        relay_tcp: Option<RelayTcpTransport>,
    ) -> Self {
        let active_paths = relay_udp
            .peers()
            .iter()
            .map(|peer| (peer.peer_node_id.clone(), peer.path_kind))
            .collect::<Vec<_>>();
        Self {
            tracker: PathTracker::new(policy, active_paths),
            direct_udp,
            relay_udp,
            relay_tcp,
        }
    }

    fn apply_runtime_paths(&mut self, peer_paths: &[PeerPathRuntime]) {
        self.tracker.apply_runtime_paths(peer_paths);
    }

    fn relay_udp_peers_mut(&mut self) -> &mut [AttachedRelayPeer] {
        self.relay_udp.peers_mut()
    }

    fn direct_udp_transport_mut(&mut self) -> Option<&mut DirectUdpTransport> {
        self.direct_udp.as_mut()
    }

    fn relay_tcp_peers_mut(&mut self) -> Option<&mut [AttachedRelayTcpPeer]> {
        self.relay_tcp
            .as_mut()
            .map(|transport| transport.peers_mut())
    }

    fn send(&self, payload: &[u8], frame: &[u8]) -> PathSendResult {
        let Some(peer_index) = self.relay_udp.peer_index_for_packet(payload) else {
            return PathSendResult::NoRoute;
        };
        let Some(peer) = self.relay_udp.peers().get(peer_index) else {
            return PathSendResult::NoRoute;
        };
        match self.active_path_for_peer(peer) {
            PathKind::DirectUdp => self
                .direct_udp
                .as_ref()
                .and_then(|transport| {
                    transport
                        .peer_index_for_packet(payload)
                        .map(|index| transport.send_to_peer(index, frame))
                })
                .unwrap_or_else(|| {
                    self.fallback_or_missing(peer_index, PathKind::DirectUdp, frame)
                }),
            PathKind::RelayUdp => self.relay_udp.send_to_peer(peer_index, frame),
            PathKind::RelayTcp => self
                .relay_tcp
                .as_ref()
                .and_then(|transport| {
                    transport
                        .peer_index_for_packet(payload)
                        .map(|index| transport.send_to_peer(index, frame))
                })
                .unwrap_or_else(|| self.fallback_or_missing(peer_index, PathKind::RelayTcp, frame)),
            PathKind::RelayTls | PathKind::RelayHttp3 => {
                if self.tracker.fallback_enabled() {
                    self.relay_udp.send_to_peer(peer_index, frame)
                } else {
                    PathSendResult::NoTransport {
                        peer_index,
                        peer_node_id: peer.peer_node_id.clone(),
                        path_kind: self.active_path_for_peer(peer),
                    }
                }
            }
        }
    }

    fn active_path_for_peer(&self, peer: &AttachedRelayPeer) -> PathKind {
        self.tracker
            .active_path_for_node(peer.peer_node_id.as_str(), peer.path_kind)
    }

    fn active_path_for_peer_node(&self, peer_node_id: &str) -> PathKind {
        self.tracker
            .active_path_for_node(peer_node_id, PathKind::RelayUdp)
    }

    fn record_send_success(&mut self, peer_node_id: &str) {
        self.tracker.record_send_success(peer_node_id);
    }

    fn record_send_failure(&mut self, peer_node_id: &str, path_kind: PathKind) -> bool {
        self.tracker.record_send_failure(
            peer_node_id,
            path_kind,
            PATH_SEND_FAILURES_BEFORE_DOWNGRADE,
        )
    }

    fn record_probe_success(&mut self, peer_node_id: &str, path_kind: PathKind) -> bool {
        self.tracker.record_probe_success(peer_node_id, path_kind)
    }

    fn active_path_summary(&self) -> String {
        self.tracker.active_path_summary()
    }

    fn fallback_or_missing(
        &self,
        peer_index: usize,
        path_kind: PathKind,
        frame: &[u8],
    ) -> PathSendResult {
        if self.tracker.fallback_enabled() {
            self.relay_udp.send_to_peer(peer_index, frame)
        } else {
            let peer_node_id = self
                .relay_udp
                .peers()
                .get(peer_index)
                .map(|peer| peer.peer_node_id.clone())
                .unwrap_or_default();
            PathSendResult::NoTransport {
                peer_index,
                peer_node_id,
                path_kind,
            }
        }
    }
}

#[derive(Debug, Default, Serialize)]
#[serde(rename_all = "camelCase")]
struct WindowsRelayDataPlaneStats {
    relay_address: String,
    network_id: String,
    local_node_id: String,
    active_path: String,
    path_policy: PathPolicy,
    path_count: u32,
    requested_relay_session_count: u32,
    relay_session_count: u32,
    ticket_expires_at: Option<String>,
    relay_attach_failures: u64,
    last_relay_attach_error: Option<String>,
    peers: Vec<WindowsRelayPeerStats>,
    relay_mtu: Option<u32>,
    max_frame_payload: Option<u32>,
    tun_packets_sent: u64,
    relay_packets_received: u64,
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
    updated_at_ms: u64,
}

#[derive(Debug, Default, Serialize)]
#[serde(rename_all = "camelCase")]
struct WindowsRelayPeerStats {
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

fn configure_wintun_data_plane(config: Option<&RelayDataPlaneConfig>) -> Result<()> {
    let Some(config) = config.filter(|value| {
        value.enabled
            && value.transport.eq_ignore_ascii_case("udp")
            && !value.relay_address.trim().is_empty()
            && !value.sessions.is_empty()
    }) else {
        stop_wintun_data_plane();
        return Ok(());
    };
    let relay_address = config.relay_address.trim();
    let relay_mtu = config.relay_mtu.unwrap_or(1280).clamp(576, 1500);
    let max_frame_payload = usize::from(config.max_frame_payload.unwrap_or(1200).clamp(512, 1400));
    let runtime = WINTUN_RUNTIME.get_or_init(|| Mutex::new(None));
    let mut runtime = runtime.lock().expect("wintun runtime mutex poisoned");
    let Some(runtime) = runtime.as_mut() else {
        bail!("Wintun runtime is not ready");
    };
    let _ = runtime.data_plane.take();
    configure_adapter_mtu(DEFAULT_INTERFACE_NAME, relay_mtu)
        .context("configure Wintun relay MTU")?;
    let requested_relay_session_count = config.sessions.len() as u32;
    let ticket_expires_at = earliest_relay_ticket_expires_at(&config.sessions);
    let relay_udp_attach = RelayUdpTransport::attach(
        relay_address,
        config.local_node_id.as_str(),
        &config.sessions,
    );
    let relay_session_count = relay_udp_attach.transport.peer_count() as u32;
    if relay_session_count == 0 {
        let mut stats = WindowsRelayDataPlaneStats {
            relay_address: relay_address.to_string(),
            network_id: config.network_id.clone(),
            local_node_id: config.local_node_id.clone(),
            active_path: PathKind::RelayUdp.as_str().to_string(),
            path_policy: config.path_policy.clone(),
            path_count: config.peer_paths.len() as u32,
            requested_relay_session_count,
            relay_session_count,
            ticket_expires_at,
            relay_attach_failures: relay_udp_attach.attach_failures,
            last_relay_attach_error: relay_udp_attach.last_attach_error.clone(),
            peers: relay_udp_attach.peer_stats,
            relay_mtu: Some(u32::from(relay_mtu)),
            max_frame_payload: Some(max_frame_payload as u32),
            ..WindowsRelayDataPlaneStats::default()
        };
        persist_relay_stats(&mut stats);
        bail!(
            "{}",
            relay_udp_attach
                .last_attach_error
                .unwrap_or_else(|| "relay attach failed for all peers".to_string())
        );
    }

    let stop = Arc::new(AtomicBool::new(false));
    let thread_stop = Arc::clone(&stop);
    let session = runtime.session as usize;
    let receive_packet = runtime.receive_packet;
    let release_receive_packet = runtime.release_receive_packet;
    let allocate_send_packet = runtime.allocate_send_packet;
    let send_packet = runtime.send_packet;
    let relay_address_owned = relay_address.to_string();
    let network_id = config.network_id.clone();
    let local_node_id = config.local_node_id.clone();
    let config_path_policy = config.path_policy.clone();
    let config_path_count = config.peer_paths.len() as u32;
    let config_peer_paths = config.peer_paths.clone();
    let config_sessions = config.sessions.clone();
    let attached_peer_paths = relay_runtime_paths_from_config(&config.peer_paths, &config.sessions);
    let config_hash = stable_hash64(&format!(
        "{}:{}:{}",
        config.network_id, config.local_node_id, relay_address
    ));
    let handle = thread::spawn(move || {
        let session = session as WintunSessionHandle;
        let mut seq = 0_u64;
        let mut relay_buffer = vec![0_u8; 4096];
        let direct_udp_transport =
            DirectUdpTransport::attach(local_node_id.as_str(), &config_peer_paths);
        let relay_tcp_transport =
            RelayTcpTransport::attach(&config_peer_paths, local_node_id.as_str(), &config_sessions);
        let mut selected_peer_paths = selected_runtime_paths(
            &config_path_policy,
            mark_ready_transports(
                attached_peer_paths.clone(),
                direct_udp_transport.as_ref(),
                relay_tcp_transport.as_ref(),
            ),
        );
        let active_path = selected_peer_paths
            .iter()
            .find_map(|path| path.active_path)
            .unwrap_or(PathKind::RelayUdp);
        persist_active_path_state(active_path, selected_peer_paths.clone());
        let mut path_manager = WindowsPathManager::new(
            config_path_policy.clone(),
            direct_udp_transport,
            relay_udp_attach.transport,
            relay_tcp_transport,
        );
        path_manager.apply_runtime_paths(&selected_peer_paths);
        let mut stats = WindowsRelayDataPlaneStats {
            relay_address: relay_address_owned,
            network_id,
            local_node_id,
            active_path: active_path.as_str().to_string(),
            path_policy: config_path_policy,
            path_count: config_path_count,
            requested_relay_session_count,
            relay_session_count,
            ticket_expires_at,
            relay_attach_failures: relay_udp_attach.attach_failures,
            last_relay_attach_error: relay_udp_attach.last_attach_error,
            peers: relay_udp_attach.peer_stats,
            relay_mtu: Some(u32::from(relay_mtu)),
            max_frame_payload: Some(max_frame_payload as u32),
            ..WindowsRelayDataPlaneStats::default()
        };
        let mut last_stats_flush = Instant::now();
        let mut last_direct_udp_probe = Instant::now()
            .checked_sub(DIRECT_UDP_PROBE_INTERVAL)
            .unwrap_or_else(Instant::now);
        let mut consecutive_data_plane_failures = 0_u32;
        persist_relay_stats(&mut stats);
        while !thread_stop.load(Ordering::SeqCst) {
            if last_direct_udp_probe.elapsed() >= DIRECT_UDP_PROBE_INTERVAL {
                if let Some(direct_udp) = path_manager.direct_udp_transport_mut() {
                    direct_udp.send_probe_packets();
                }
                last_direct_udp_probe = Instant::now();
            }
            let mut packet_size = 0_u32;
            let packet = unsafe { receive_packet(session, &mut packet_size as *mut u32) };
            if !packet.is_null() && packet_size > 0 {
                let payload = unsafe { std::slice::from_raw_parts(packet, packet_size as usize) };
                if payload.len() > max_frame_payload {
                    stats.oversized_tun_packets = stats.oversized_tun_packets.saturating_add(1);
                    stats.last_oversized_tun_packet_size = Some(payload.len() as u32);
                    unsafe {
                        release_receive_packet(session, packet);
                    }
                    thread::sleep(Duration::from_millis(5));
                    continue;
                }
                if let Some(frame) =
                    encode_slan_relay_data_frame(seq.wrapping_add(1), config_hash, payload)
                {
                    seq = seq.wrapping_add(1);
                    match path_manager.send(payload, &frame) {
                        PathSendResult::Sent {
                            peer_node_id,
                            path_kind,
                            ..
                        } => {
                            path_manager.record_send_success(&peer_node_id);
                            stats.tun_packets_sent = stats.tun_packets_sent.saturating_add(1);
                            if let Some(peer_stats) =
                                peer_stats_mut_by_node_id(&mut stats.peers, &peer_node_id)
                            {
                                peer_stats.tun_packets_sent =
                                    peer_stats.tun_packets_sent.saturating_add(1);
                                peer_stats.last_send_path = Some(path_kind.as_str().to_string());
                            }
                            consecutive_data_plane_failures = 0;
                        }
                        PathSendResult::NoRoute => {
                            stats.unroutable_tun_packets =
                                stats.unroutable_tun_packets.saturating_add(1);
                            stats.last_unroutable_destination = ipv4_destination(payload);
                        }
                        PathSendResult::SendFailed {
                            peer_node_id,
                            path_kind,
                            ..
                        } => {
                            if path_manager.record_send_failure(&peer_node_id, path_kind) {
                                update_peer_active_path(
                                    &mut selected_peer_paths,
                                    &peer_node_id,
                                    PathKind::RelayUdp,
                                );
                                stats.active_path = path_manager.active_path_summary();
                                if let Some(peer_stats) =
                                    peer_stats_mut_by_node_id(&mut stats.peers, &peer_node_id)
                                {
                                    peer_stats.path_downgrades =
                                        peer_stats.path_downgrades.saturating_add(1);
                                    peer_stats.last_path_change = Some(format!(
                                        "{} -> relay_udp after consecutive send failures",
                                        path_kind.as_str()
                                    ));
                                }
                                persist_active_path_state(
                                    PathKind::RelayUdp,
                                    selected_peer_paths.clone(),
                                );
                            }
                            stats.relay_send_failures = stats.relay_send_failures.saturating_add(1);
                            if let Some(peer_stats) =
                                peer_stats_mut_by_node_id(&mut stats.peers, &peer_node_id)
                            {
                                peer_stats.send_failures =
                                    peer_stats.send_failures.saturating_add(1);
                                peer_stats.last_send_path = Some(path_kind.as_str().to_string());
                            }
                            consecutive_data_plane_failures =
                                consecutive_data_plane_failures.saturating_add(1);
                        }
                        PathSendResult::NoTransport {
                            peer_node_id,
                            path_kind,
                            ..
                        } => {
                            if path_manager.record_send_failure(&peer_node_id, path_kind) {
                                update_peer_active_path(
                                    &mut selected_peer_paths,
                                    &peer_node_id,
                                    PathKind::RelayUdp,
                                );
                                stats.active_path = path_manager.active_path_summary();
                                if let Some(peer_stats) =
                                    peer_stats_mut_by_node_id(&mut stats.peers, &peer_node_id)
                                {
                                    peer_stats.path_downgrades =
                                        peer_stats.path_downgrades.saturating_add(1);
                                    peer_stats.last_path_change = Some(format!(
                                        "{} -> relay_udp after missing transport",
                                        path_kind.as_str()
                                    ));
                                }
                                persist_active_path_state(
                                    PathKind::RelayUdp,
                                    selected_peer_paths.clone(),
                                );
                            }
                            stats.relay_send_failures = stats.relay_send_failures.saturating_add(1);
                            if let Some(peer_stats) =
                                peer_stats_mut_by_node_id(&mut stats.peers, &peer_node_id)
                            {
                                peer_stats.send_failures =
                                    peer_stats.send_failures.saturating_add(1);
                                peer_stats.last_send_path = Some(path_kind.as_str().to_string());
                                peer_stats.last_relay_error = Some(format!(
                                    "path transport unavailable: {}",
                                    path_kind.as_str()
                                ));
                            }
                            consecutive_data_plane_failures =
                                consecutive_data_plane_failures.saturating_add(1);
                        }
                    }
                }
                unsafe {
                    release_receive_packet(session, packet);
                }
            }

            for peer in path_manager.relay_udp_peers_mut() {
                match peer.socket.recv(&mut relay_buffer) {
                    Ok(frame_len) => {
                        if let Some(decoded) =
                            decode_slan_relay_data_frame_full(&relay_buffer[..frame_len])
                        {
                            if decoded.config_hash != config_hash {
                                stats.relay_config_hash_mismatches =
                                    stats.relay_config_hash_mismatches.saturating_add(1);
                                if let Some(peer_stats) = stats.peers.get_mut(peer.stats_index) {
                                    peer_stats.config_hash_mismatches =
                                        peer_stats.config_hash_mismatches.saturating_add(1);
                                }
                                continue;
                            }
                            if relay_frame_is_replayed(peer.last_rx_seq, decoded.seq) {
                                if let Some(peer_stats) = stats.peers.get_mut(peer.stats_index) {
                                    peer_stats.replayed_frames =
                                        peer_stats.replayed_frames.saturating_add(1);
                                }
                                continue;
                            }
                            peer.last_rx_seq = decoded.seq;
                            if let Some(peer_stats) = stats.peers.get_mut(peer.stats_index) {
                                peer_stats.last_rx_seq = decoded.seq;
                            }
                            if write_wintun_packet(
                                session,
                                allocate_send_packet,
                                send_packet,
                                decoded.payload,
                            ) {
                                stats.relay_packets_received =
                                    stats.relay_packets_received.saturating_add(1);
                                if let Some(peer_stats) = stats.peers.get_mut(peer.stats_index) {
                                    peer_stats.relay_packets_received =
                                        peer_stats.relay_packets_received.saturating_add(1);
                                }
                                consecutive_data_plane_failures = 0;
                            } else {
                                stats.wintun_write_failures =
                                    stats.wintun_write_failures.saturating_add(1);
                                if let Some(peer_stats) = stats.peers.get_mut(peer.stats_index) {
                                    peer_stats.wintun_write_failures =
                                        peer_stats.wintun_write_failures.saturating_add(1);
                                }
                                consecutive_data_plane_failures =
                                    consecutive_data_plane_failures.saturating_add(1);
                            }
                        } else if let Some(error) = relay_error_message(&relay_buffer[..frame_len])
                        {
                            stats.relay_error_responses =
                                stats.relay_error_responses.saturating_add(1);
                            stats.last_relay_error = Some(format!(
                                "peer {} session {}: {error}",
                                peer.peer_node_id, peer.session_id
                            ));
                            if let Some(peer_stats) = stats.peers.get_mut(peer.stats_index) {
                                peer_stats.relay_errors = peer_stats.relay_errors.saturating_add(1);
                                peer_stats.last_relay_error = Some(error);
                            }
                        } else {
                            stats.relay_decode_failures =
                                stats.relay_decode_failures.saturating_add(1);
                        }
                    }
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {}
                    Err(error) if error.kind() == std::io::ErrorKind::Interrupted => {}
                    Err(_) => {
                        stats.relay_receive_failures =
                            stats.relay_receive_failures.saturating_add(1);
                        if let Some(peer_stats) = stats.peers.get_mut(peer.stats_index) {
                            peer_stats.receive_failures =
                                peer_stats.receive_failures.saturating_add(1);
                        }
                        consecutive_data_plane_failures =
                            consecutive_data_plane_failures.saturating_add(1);
                    }
                }
            }
            let mut direct_udp_probe_success_peer = None;
            if let Some(direct_udp) = path_manager.direct_udp_transport_mut() {
                match direct_udp.recv_from_peer(&mut relay_buffer) {
                    Ok(Some(received)) => {
                        let peer_index = received.peer_index;
                        let frame_len = received.frame_len;
                        let control_packet = direct_udp_control_packet(&relay_buffer[..frame_len]);
                        if control_packet
                            .as_ref()
                            .is_some_and(|packet| packet.kind == DirectUdpControlKind::Probe)
                        {
                            if received.endpoint_changed {
                                direct_udp.update_peer_endpoint(peer_index, received.remote_addr);
                            }
                            direct_udp.send_pong_to_peer(peer_index);
                            direct_udp_probe_success_peer =
                                Some(direct_udp.peers[peer_index].peer_node_id.clone());
                        } else if control_packet
                            .as_ref()
                            .is_some_and(|packet| packet.kind == DirectUdpControlKind::Pong)
                        {
                            if received.endpoint_changed {
                                direct_udp.update_peer_endpoint(peer_index, received.remote_addr);
                            }
                            direct_udp_probe_success_peer =
                                Some(direct_udp.peers[peer_index].peer_node_id.clone());
                        } else if let Some(decoded) =
                            decode_slan_relay_data_frame_full(&relay_buffer[..frame_len])
                        {
                            let peer_node_id = direct_udp.peers[peer_index].peer_node_id.clone();
                            let last_rx_seq = direct_udp.peers[peer_index].last_rx_seq;
                            if decoded.config_hash != config_hash {
                                stats.relay_config_hash_mismatches =
                                    stats.relay_config_hash_mismatches.saturating_add(1);
                                if let Some(peer_stats) =
                                    peer_stats_mut_by_node_id(&mut stats.peers, &peer_node_id)
                                {
                                    peer_stats.config_hash_mismatches =
                                        peer_stats.config_hash_mismatches.saturating_add(1);
                                    peer_stats.last_relay_error =
                                        Some("direct udp config hash mismatch".to_string());
                                }
                            } else if relay_frame_is_replayed(last_rx_seq, decoded.seq) {
                                if let Some(peer_stats) =
                                    peer_stats_mut_by_node_id(&mut stats.peers, &peer_node_id)
                                {
                                    peer_stats.replayed_frames =
                                        peer_stats.replayed_frames.saturating_add(1);
                                }
                            } else {
                                if received.endpoint_changed {
                                    direct_udp
                                        .update_peer_endpoint(peer_index, received.remote_addr);
                                }
                                let peer = &mut direct_udp.peers[peer_index];
                                direct_udp_probe_success_peer = Some(peer.peer_node_id.clone());
                                peer.last_rx_seq = decoded.seq;
                                if let Some(peer_stats) =
                                    peer_stats_mut_by_node_id(&mut stats.peers, &peer.peer_node_id)
                                {
                                    peer_stats.last_rx_seq = decoded.seq;
                                }
                                if write_wintun_packet(
                                    session,
                                    allocate_send_packet,
                                    send_packet,
                                    decoded.payload,
                                ) {
                                    stats.relay_packets_received =
                                        stats.relay_packets_received.saturating_add(1);
                                    if let Some(peer_stats) = peer_stats_mut_by_node_id(
                                        &mut stats.peers,
                                        &peer.peer_node_id,
                                    ) {
                                        peer_stats.relay_packets_received =
                                            peer_stats.relay_packets_received.saturating_add(1);
                                    }
                                    consecutive_data_plane_failures = 0;
                                } else {
                                    stats.wintun_write_failures =
                                        stats.wintun_write_failures.saturating_add(1);
                                    if let Some(peer_stats) = peer_stats_mut_by_node_id(
                                        &mut stats.peers,
                                        &peer.peer_node_id,
                                    ) {
                                        peer_stats.wintun_write_failures =
                                            peer_stats.wintun_write_failures.saturating_add(1);
                                    }
                                    consecutive_data_plane_failures =
                                        consecutive_data_plane_failures.saturating_add(1);
                                }
                            }
                        } else {
                            let peer = &direct_udp.peers[peer_index];
                            stats.relay_decode_failures =
                                stats.relay_decode_failures.saturating_add(1);
                            if let Some(peer_stats) =
                                peer_stats_mut_by_node_id(&mut stats.peers, &peer.peer_node_id)
                            {
                                peer_stats.receive_failures =
                                    peer_stats.receive_failures.saturating_add(1);
                            }
                            stats.last_relay_error = Some(format!(
                                "direct udp decode failed peer={} address={}",
                                peer.peer_node_id, peer.address
                            ));
                        }
                    }
                    Ok(None) => {}
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {}
                    Err(error) if error.kind() == std::io::ErrorKind::Interrupted => {}
                    Err(_) => {
                        stats.relay_receive_failures =
                            stats.relay_receive_failures.saturating_add(1);
                        consecutive_data_plane_failures =
                            consecutive_data_plane_failures.saturating_add(1);
                    }
                }
            }
            if let Some(peer_node_id) = direct_udp_probe_success_peer {
                let previous_path = path_manager.active_path_for_peer_node(&peer_node_id);
                if path_manager.record_probe_success(&peer_node_id, PathKind::DirectUdp) {
                    mark_peer_path_probe_success(
                        &mut selected_peer_paths,
                        &peer_node_id,
                        PathKind::DirectUdp,
                        current_timestamp_ms(),
                    );
                    stats.active_path = path_manager.active_path_summary();
                    if let Some(peer_stats) =
                        peer_stats_mut_by_node_id(&mut stats.peers, &peer_node_id)
                    {
                        peer_stats.path_upgrades = peer_stats.path_upgrades.saturating_add(1);
                        peer_stats.last_path_change = Some(format!(
                            "{} -> direct_udp after probe success",
                            previous_path.as_str()
                        ));
                    }
                    persist_active_path_state(PathKind::DirectUdp, selected_peer_paths.clone());
                }
            }
            if let Some(peers) = path_manager.relay_tcp_peers_mut() {
                for peer in peers {
                    let frame = {
                        let Ok(mut stream) = peer.stream.lock() else {
                            stats.relay_receive_failures =
                                stats.relay_receive_failures.saturating_add(1);
                            consecutive_data_plane_failures =
                                consecutive_data_plane_failures.saturating_add(1);
                            continue;
                        };
                        match read_relay_tcp_frame(&mut stream) {
                            Ok(frame) => frame,
                            Err(error)
                                if error.downcast_ref::<std::io::Error>().is_some_and(
                                    |io_error| {
                                        io_error.kind() == std::io::ErrorKind::WouldBlock
                                            || io_error.kind() == std::io::ErrorKind::TimedOut
                                    },
                                ) =>
                            {
                                continue;
                            }
                            Err(_) => {
                                stats.relay_receive_failures =
                                    stats.relay_receive_failures.saturating_add(1);
                                if let Some(peer_stats) =
                                    peer_stats_mut_by_node_id(&mut stats.peers, &peer.peer_node_id)
                                {
                                    peer_stats.receive_failures =
                                        peer_stats.receive_failures.saturating_add(1);
                                }
                                consecutive_data_plane_failures =
                                    consecutive_data_plane_failures.saturating_add(1);
                                continue;
                            }
                        }
                    };
                    if let Some(decoded) = decode_slan_relay_data_frame_full(&frame) {
                        if decoded.config_hash != config_hash {
                            stats.relay_config_hash_mismatches =
                                stats.relay_config_hash_mismatches.saturating_add(1);
                            if let Some(peer_stats) =
                                peer_stats_mut_by_node_id(&mut stats.peers, &peer.peer_node_id)
                            {
                                peer_stats.config_hash_mismatches =
                                    peer_stats.config_hash_mismatches.saturating_add(1);
                                peer_stats.last_relay_error =
                                    Some("relay tcp config hash mismatch".to_string());
                            }
                            continue;
                        }
                        if relay_frame_is_replayed(peer.last_rx_seq, decoded.seq) {
                            if let Some(peer_stats) =
                                peer_stats_mut_by_node_id(&mut stats.peers, &peer.peer_node_id)
                            {
                                peer_stats.replayed_frames =
                                    peer_stats.replayed_frames.saturating_add(1);
                            }
                            continue;
                        }
                        peer.last_rx_seq = decoded.seq;
                        if let Some(peer_stats) =
                            peer_stats_mut_by_node_id(&mut stats.peers, &peer.peer_node_id)
                        {
                            peer_stats.last_rx_seq = decoded.seq;
                        }
                        if write_wintun_packet(
                            session,
                            allocate_send_packet,
                            send_packet,
                            decoded.payload,
                        ) {
                            stats.relay_packets_received =
                                stats.relay_packets_received.saturating_add(1);
                            if let Some(peer_stats) =
                                peer_stats_mut_by_node_id(&mut stats.peers, &peer.peer_node_id)
                            {
                                peer_stats.relay_packets_received =
                                    peer_stats.relay_packets_received.saturating_add(1);
                            }
                            consecutive_data_plane_failures = 0;
                        } else {
                            stats.wintun_write_failures =
                                stats.wintun_write_failures.saturating_add(1);
                            if let Some(peer_stats) =
                                peer_stats_mut_by_node_id(&mut stats.peers, &peer.peer_node_id)
                            {
                                peer_stats.wintun_write_failures =
                                    peer_stats.wintun_write_failures.saturating_add(1);
                            }
                            consecutive_data_plane_failures =
                                consecutive_data_plane_failures.saturating_add(1);
                        }
                    } else if let Some(error) = relay_error_message(&frame) {
                        stats.relay_error_responses = stats.relay_error_responses.saturating_add(1);
                        stats.last_relay_error =
                            Some(format!("peer {} tcp: {error}", peer.peer_node_id));
                        if let Some(peer_stats) =
                            peer_stats_mut_by_node_id(&mut stats.peers, &peer.peer_node_id)
                        {
                            peer_stats.relay_errors = peer_stats.relay_errors.saturating_add(1);
                            peer_stats.last_relay_error = Some(error);
                        }
                    } else {
                        stats.relay_decode_failures = stats.relay_decode_failures.saturating_add(1);
                        if let Some(peer_stats) =
                            peer_stats_mut_by_node_id(&mut stats.peers, &peer.peer_node_id)
                        {
                            peer_stats.receive_failures =
                                peer_stats.receive_failures.saturating_add(1);
                        }
                    }
                }
            }
            if consecutive_data_plane_failures >= RELAY_DATA_PLANE_FAILURE_THRESHOLD {
                persist_relay_stats(&mut stats);
                break;
            }
            if last_stats_flush.elapsed() >= RELAY_STATS_FLUSH_INTERVAL {
                persist_relay_stats(&mut stats);
                last_stats_flush = Instant::now();
            }
            thread::sleep(Duration::from_millis(5));
        }
        persist_relay_stats(&mut stats);
    });
    runtime.data_plane = Some(WindowsDataPlaneRuntime {
        stop,
        handle: Some(handle),
    });
    Ok(())
}

fn persist_active_path_state(active_path: PathKind, peer_paths: Vec<PeerPathRuntime>) {
    let mut state = load_cached_runtime_state().unwrap_or_default();
    state.active_path = Some(active_path);
    state.peer_paths = peer_paths;
    let _ = persist_state(&state);
}

fn relay_runtime_paths_from_config(
    configured_paths: &[client_core::PeerPathConfig],
    sessions: &[RelayPeerSession],
) -> Vec<PeerPathRuntime> {
    let mut paths = configured_paths
        .iter()
        .map(|path| PeerPathRuntime {
            peer_node_id: path.peer_node_id.clone(),
            peer_virtual_ips: path.peer_virtual_ips.clone(),
            active_path: None,
            candidates: path.candidates.clone(),
        })
        .collect::<Vec<_>>();
    for session in sessions {
        let Some(path) = paths
            .iter_mut()
            .find(|path| path.peer_node_id == session.peer_node_id)
        else {
            paths.push(relay_runtime_path_from_session(session));
            continue;
        };
        mark_relay_udp_ready(path, session);
    }
    paths
}

fn relay_runtime_path_from_session(session: &RelayPeerSession) -> PeerPathRuntime {
    let mut path = PeerPathRuntime {
        peer_node_id: session.peer_node_id.clone(),
        peer_virtual_ips: session.peer_virtual_ips.clone(),
        active_path: None,
        candidates: Vec::new(),
    };
    mark_relay_udp_ready(&mut path, session);
    path
}

fn mark_relay_udp_ready(path: &mut PeerPathRuntime, session: &RelayPeerSession) {
    if path.peer_virtual_ips.is_empty() {
        path.peer_virtual_ips = session.peer_virtual_ips.clone();
    }
    let Some(candidate) = path
        .candidates
        .iter_mut()
        .find(|candidate| candidate.kind == PathKind::RelayUdp)
    else {
        path.candidates.push(relay_udp_ready_candidate(session));
        return;
    };
    candidate.state = PathState::Ready;
    candidate.session_id = Some(session.session_id.clone());
    if candidate
        .address
        .as_deref()
        .unwrap_or_default()
        .trim()
        .is_empty()
    {
        candidate.address = Some(session.ticket.relay_url.clone());
    }
    candidate.transport = Some("udp".to_string());
}

fn relay_udp_ready_candidate(session: &RelayPeerSession) -> client_core::PathCandidate {
    client_core::PathCandidate {
        kind: PathKind::RelayUdp,
        state: PathState::Ready,
        endpoint_id: None,
        address: Some(session.ticket.relay_url.clone()),
        session_id: Some(session.session_id.clone()),
        transport: Some("udp".to_string()),
        rtt_ms: None,
        path_score: None,
        last_ok_at_ms: None,
        last_error: None,
    }
}

fn mark_ready_transports(
    mut peer_paths: Vec<PeerPathRuntime>,
    direct_udp: Option<&DirectUdpTransport>,
    relay_tcp: Option<&RelayTcpTransport>,
) -> Vec<PeerPathRuntime> {
    if let Some(direct_udp) = direct_udp {
        peer_paths = mark_path_ready_for_nodes(
            peer_paths,
            direct_udp
                .peers
                .iter()
                .map(|peer| peer.peer_node_id.as_str()),
            PathKind::DirectUdp,
        );
    }
    let Some(relay_tcp) = relay_tcp else {
        return peer_paths;
    };
    mark_path_ready_for_nodes(
        peer_paths,
        relay_tcp
            .peers
            .iter()
            .map(|peer| peer.peer_node_id.as_str()),
        PathKind::RelayTcp,
    )
}

fn attach_udp_relay_session(
    relay_address: &str,
    local_node_id: &str,
    session: &RelayPeerSession,
) -> Result<UdpSocket> {
    validate_relay_peer_session(local_node_id, session)?;
    let socket = UdpSocket::bind("0.0.0.0:0").context("bind Wintun relay UDP socket")?;
    socket
        .connect(relay_address)
        .with_context(|| format!("connect Wintun relay UDP socket to {relay_address}"))?;
    socket
        .set_read_timeout(Some(RELAY_ATTACH_TIMEOUT))
        .context("set Wintun relay attach read timeout")?;
    socket
        .set_write_timeout(Some(RELAY_ATTACH_TIMEOUT))
        .context("set Wintun relay attach write timeout")?;

    let attach = serde_json::json!({
        "kind": "attach",
        "participant_id": local_node_id,
        "ticket": relay_ticket_wire(session),
    });
    let payload = serde_json::to_vec(&attach).context("encode Wintun relay attach")?;
    let mut response = vec![0_u8; 4096];
    let mut last_error = None;
    for attempt in 1..=RELAY_ATTACH_ATTEMPTS {
        if let Err(error) = socket.send(&payload) {
            last_error = Some(anyhow::anyhow!(
                "send Wintun relay attach to {relay_address} attempt {attempt}/{RELAY_ATTACH_ATTEMPTS}: {error}"
            ));
            thread::sleep(Duration::from_millis((attempt as u64) * 250));
            continue;
        }
        match socket.recv(&mut response) {
            Ok(len) => {
                verify_relay_attach_ack(&response[..len], &session.session_id)?;
                last_error = None;
                break;
            }
            Err(error)
                if error.kind() == std::io::ErrorKind::WouldBlock
                    || error.kind() == std::io::ErrorKind::TimedOut =>
            {
                last_error = Some(anyhow::anyhow!(
                    "receive Wintun relay attach ack from {relay_address} attempt {attempt}/{RELAY_ATTACH_ATTEMPTS}: timeout"
                ));
                thread::sleep(Duration::from_millis((attempt as u64) * 250));
            }
            Err(error) => {
                last_error = Some(anyhow::anyhow!(
                    "receive Wintun relay attach ack from {relay_address} attempt {attempt}/{RELAY_ATTACH_ATTEMPTS}: {error}"
                ));
                thread::sleep(Duration::from_millis((attempt as u64) * 250));
            }
        }
    }
    if let Some(error) = last_error {
        return Err(error).context(format!(
            "relay attach failed after {RELAY_ATTACH_ATTEMPTS} attempts for peer {}",
            session.peer_node_id
        ));
    }
    socket
        .set_read_timeout(None)
        .context("clear Wintun relay attach read timeout")?;
    socket
        .set_write_timeout(None)
        .context("clear Wintun relay attach write timeout")?;
    socket
        .set_nonblocking(true)
        .context("set Wintun relay UDP socket nonblocking")?;
    Ok(socket)
}

fn direct_udp_address_for_peer(path: &client_core::PeerPathConfig) -> Option<String> {
    path.candidates.iter().find_map(|candidate| {
        (candidate.kind == PathKind::DirectUdp)
            .then(|| candidate.address.as_deref())
            .flatten()
            .map(str::trim)
            .filter(|address| !address.is_empty())
            .map(str::to_string)
    })
}

fn attach_direct_udp_socket() -> Result<UdpSocket> {
    let bind_address = std::env::var("SLAN_DIRECT_UDP_BIND")
        .ok()
        .map(|value| value.trim().to_string())
        .filter(|value| !value.is_empty())
        .unwrap_or_else(|| "0.0.0.0:0".to_string());
    let socket = UdpSocket::bind(&bind_address)
        .with_context(|| format!("bind direct UDP socket to {bind_address}"))?;
    socket
        .set_nonblocking(true)
        .context("set direct UDP socket nonblocking")?;
    Ok(socket)
}

fn resolve_direct_udp_peer_address(address: &str) -> Result<SocketAddr> {
    address
        .to_socket_addrs()
        .with_context(|| format!("resolve direct UDP peer address {address}"))?
        .next()
        .ok_or_else(|| anyhow::anyhow!("direct UDP peer address has no socket address: {address}"))
}

fn relay_tcp_address_for_peer(
    configured_paths: &[client_core::PeerPathConfig],
    peer_node_id: &str,
) -> Option<String> {
    configured_paths
        .iter()
        .find(|peer| peer.peer_node_id == peer_node_id)
        .and_then(|peer| {
            peer.candidates.iter().find_map(|candidate| {
                (candidate.kind == PathKind::RelayTcp)
                    .then(|| candidate.address.as_deref())
                    .flatten()
                    .map(normalize_relay_tcp_address)
            })
        })
        .filter(|address| !address.trim().is_empty())
}

fn normalize_relay_tcp_address(address: &str) -> String {
    address
        .trim()
        .strip_prefix("tcp://")
        .or_else(|| address.trim().strip_prefix("relay+tcp://"))
        .unwrap_or_else(|| address.trim())
        .to_string()
}

fn attach_tcp_relay_session(
    relay_address: &str,
    local_node_id: &str,
    session: &RelayPeerSession,
) -> Result<TcpStream> {
    validate_relay_peer_session(local_node_id, session)?;
    let mut stream = TcpStream::connect(relay_address)
        .with_context(|| format!("connect Wintun relay TCP stream to {relay_address}"))?;
    stream
        .set_read_timeout(Some(RELAY_ATTACH_TIMEOUT))
        .context("set Wintun relay TCP attach read timeout")?;
    stream
        .set_write_timeout(Some(RELAY_ATTACH_TIMEOUT))
        .context("set Wintun relay TCP attach write timeout")?;

    let attach = serde_json::json!({
        "kind": "attach",
        "participant_id": local_node_id,
        "ticket": relay_ticket_wire(session),
    });
    let payload = serde_json::to_vec(&attach).context("encode Wintun relay TCP attach")?;
    write_relay_tcp_frame(&mut stream, &payload).context("send Wintun relay TCP attach")?;
    let response =
        read_relay_tcp_frame(&mut stream).context("receive Wintun relay TCP attach ack")?;
    verify_relay_attach_ack(&response, &session.session_id)?;
    stream
        .set_read_timeout(Some(Duration::from_millis(1)))
        .context("set Wintun relay TCP nonblocking read timeout")?;
    stream
        .set_write_timeout(None)
        .context("clear Wintun relay TCP write timeout")?;
    Ok(stream)
}

fn read_relay_tcp_frame(stream: &mut TcpStream) -> Result<Vec<u8>> {
    let mut len_buffer = [0_u8; 4];
    stream
        .read_exact(&mut len_buffer)
        .context("read relay TCP frame length")?;
    let len = u32::from_be_bytes(len_buffer) as usize;
    if len == 0 || len > RELAY_TCP_FRAME_SIZE_LIMIT {
        bail!("invalid relay TCP frame length: {len}");
    }
    let mut frame = vec![0_u8; len];
    stream
        .read_exact(&mut frame)
        .context("read relay TCP frame payload")?;
    Ok(frame)
}

fn write_relay_tcp_frame(stream: &mut TcpStream, frame: &[u8]) -> Result<()> {
    if frame.is_empty() || frame.len() > RELAY_TCP_FRAME_SIZE_LIMIT {
        bail!("invalid relay TCP frame length: {}", frame.len());
    }
    stream
        .write_all(&(frame.len() as u32).to_be_bytes())
        .and_then(|_| stream.write_all(frame))
        .context("write relay TCP frame")
}

fn validate_relay_peer_session(local_node_id: &str, session: &RelayPeerSession) -> Result<()> {
    let local_node_id = local_node_id.trim();
    let ticket = &session.ticket;
    if session.session_id.trim().is_empty()
        || session.peer_node_id.trim().is_empty()
        || ticket.ticket_id.trim().is_empty()
        || ticket.network_id.trim().is_empty()
        || ticket.session_id.trim().is_empty()
        || ticket.src_node_id.trim().is_empty()
        || ticket.dst_node_id.trim().is_empty()
        || ticket.relay_url.trim().is_empty()
        || ticket.expires_at.trim().is_empty()
        || ticket.signature.trim().is_empty()
    {
        bail!("relay ticket is incomplete");
    }
    if session.session_id != ticket.session_id {
        bail!("relay ticket session mismatch");
    }
    if ticket.src_node_id != local_node_id && ticket.dst_node_id != local_node_id {
        bail!("relay ticket does not include local node");
    }
    if ticket.src_node_id != session.peer_node_id && ticket.dst_node_id != session.peer_node_id {
        bail!("relay ticket does not include peer node");
    }
    if !ticket.relay_url.contains("://") {
        bail!("relay ticket relayUrl is invalid");
    }
    Ok(())
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
    let value: serde_json::Value =
        serde_json::from_slice(response).context("decode Wintun relay attach ack")?;
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
                bail!("relay attach session mismatch: expected {session_id}, got {ack_session}")
            }
        }
        Some("error") => {
            let message = value
                .get("message")
                .and_then(serde_json::Value::as_str)
                .unwrap_or("relay attach failed");
            bail!("{message}")
        }
        kind => bail!("unexpected relay attach response: {:?}", kind),
    }
}

#[allow(dead_code)]
enum PathSendResult {
    Sent {
        peer_index: usize,
        peer_node_id: String,
        path_kind: PathKind,
    },
    NoRoute,
    SendFailed {
        peer_index: usize,
        peer_node_id: String,
        path_kind: PathKind,
    },
    NoTransport {
        peer_index: usize,
        peer_node_id: String,
        path_kind: PathKind,
    },
}

#[cfg(test)]
fn send_frame_to_peer(peers: &[AttachedRelayPeer], payload: &[u8], frame: &[u8]) -> PathSendResult {
    if let Some(peer_index) = relay_peer_index_for_packet(
        &peers
            .iter()
            .map(|peer| peer.peer_virtual_ips.as_slice())
            .collect::<Vec<_>>(),
        payload,
    ) {
        return if send_relay_udp_frame(&peers[peer_index], frame) {
            PathSendResult::Sent {
                peer_index,
                peer_node_id: peers[peer_index].peer_node_id.clone(),
                path_kind: PathKind::RelayUdp,
            }
        } else {
            PathSendResult::SendFailed {
                peer_index,
                peer_node_id: peers[peer_index].peer_node_id.clone(),
                path_kind: PathKind::RelayUdp,
            }
        };
    }
    if peers.len() == 1 {
        return if send_relay_udp_frame(&peers[0], frame) {
            PathSendResult::Sent {
                peer_index: 0,
                peer_node_id: peers[0].peer_node_id.clone(),
                path_kind: PathKind::RelayUdp,
            }
        } else {
            PathSendResult::SendFailed {
                peer_index: 0,
                peer_node_id: peers[0].peer_node_id.clone(),
                path_kind: PathKind::RelayUdp,
            }
        };
    }
    PathSendResult::NoRoute
}

fn send_relay_udp_frame(peer: &AttachedRelayPeer, frame: &[u8]) -> bool {
    peer.path_kind == PathKind::RelayUdp && peer.socket.send(frame).is_ok()
}

fn send_relay_tcp_frame(peer: &AttachedRelayTcpPeer, frame: &[u8]) -> bool {
    if peer.peer_node_id.trim().is_empty()
        || frame.is_empty()
        || frame.len() > RELAY_TCP_FRAME_SIZE_LIMIT
    {
        return false;
    }
    let Ok(mut stream) = peer.stream.lock() else {
        return false;
    };
    write_relay_tcp_frame(&mut stream, frame).is_ok()
}

fn peer_stats_mut_by_node_id<'a>(
    peers: &'a mut [WindowsRelayPeerStats],
    peer_node_id: &str,
) -> Option<&'a mut WindowsRelayPeerStats> {
    peers
        .iter_mut()
        .find(|peer| peer.peer_node_id == peer_node_id)
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum DirectUdpControlKind {
    Probe,
    Pong,
}

#[derive(Debug, Clone, PartialEq, Eq)]
struct DirectUdpControlPacket {
    kind: DirectUdpControlKind,
    peer_node_id: Option<String>,
}

impl DirectUdpControlPacket {
    fn peer_node_id(&self) -> Option<&str> {
        self.peer_node_id.as_deref()
    }
}

fn direct_udp_control_packet(payload: &[u8]) -> Option<DirectUdpControlPacket> {
    if payload == DIRECT_UDP_PROBE_PACKET {
        return Some(DirectUdpControlPacket {
            kind: DirectUdpControlKind::Probe,
            peer_node_id: None,
        });
    }
    if payload == DIRECT_UDP_PONG_PACKET {
        return Some(DirectUdpControlPacket {
            kind: DirectUdpControlKind::Pong,
            peer_node_id: None,
        });
    }
    let value = std::str::from_utf8(payload).ok()?;
    let (kind, peer_node_id) = value.split_once(':')?;
    if peer_node_id.trim().is_empty() {
        return None;
    }
    let kind = if kind == std::str::from_utf8(DIRECT_UDP_PROBE_PACKET).ok()? {
        DirectUdpControlKind::Probe
    } else if kind == std::str::from_utf8(DIRECT_UDP_PONG_PACKET).ok()? {
        DirectUdpControlKind::Pong
    } else {
        return None;
    };
    Some(DirectUdpControlPacket {
        kind,
        peer_node_id: Some(peer_node_id.trim().to_string()),
    })
}

fn direct_udp_control_payload(kind: DirectUdpControlKind, local_node_id: &str) -> String {
    let prefix = match kind {
        DirectUdpControlKind::Probe => std::str::from_utf8(DIRECT_UDP_PROBE_PACKET).unwrap(),
        DirectUdpControlKind::Pong => std::str::from_utf8(DIRECT_UDP_PONG_PACKET).unwrap(),
    };
    format!("{prefix}:{}", local_node_id.trim())
}

fn write_wintun_packet(
    session: WintunSessionHandle,
    allocate_send_packet: WintunAllocateSendPacketFunc,
    send_packet: WintunSendPacketFunc,
    payload: &[u8],
) -> bool {
    if let Ok(size) = u32::try_from(payload.len()) {
        let send_packet_ptr = unsafe { allocate_send_packet(session, size) };
        if !send_packet_ptr.is_null() {
            unsafe {
                std::ptr::copy_nonoverlapping(payload.as_ptr(), send_packet_ptr, payload.len());
                send_packet(session, send_packet_ptr);
            }
            return true;
        }
    }
    false
}

fn stop_wintun_data_plane() {
    let runtime = WINTUN_RUNTIME.get_or_init(|| Mutex::new(None));
    let mut runtime = runtime.lock().expect("wintun runtime mutex poisoned");
    if let Some(runtime) = runtime.as_mut() {
        let _ = runtime.data_plane.take();
    }
    clear_direct_udp_endpoint_report();
}

fn detach_udp_relay_sessions(peers: &[AttachedRelayPeer], local_node_id: &str) {
    for peer in peers {
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

fn relay_error_message(payload: &[u8]) -> Option<String> {
    let value = serde_json::from_slice::<serde_json::Value>(payload).ok()?;
    if value.get("kind").and_then(serde_json::Value::as_str) != Some("error") {
        return None;
    }
    let code = value
        .get("code")
        .and_then(serde_json::Value::as_str)
        .unwrap_or("relay_error");
    let message = value
        .get("message")
        .and_then(serde_json::Value::as_str)
        .unwrap_or_default();
    if message.is_empty() {
        Some(code.to_string())
    } else {
        Some(format!("{code}: {message}"))
    }
}

fn relay_stats_file_path() -> PathBuf {
    app_data_dir()
        .join("SLAN")
        .join("client-v2-relay-stats.json")
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
struct DirectUdpEndpointReport {
    endpoint: String,
    endpoint_type: String,
    nat_type: String,
    bind_address: String,
    updated_at_ms: u64,
}

fn direct_udp_endpoint_file_path() -> PathBuf {
    app_data_dir()
        .join("SLAN")
        .join("client-v2-direct-udp-endpoint.json")
}

fn persist_direct_udp_endpoint_report(socket: &UdpSocket) {
    let Ok(local_addr) = socket.local_addr() else {
        return;
    };
    let report_host = std::env::var("SLAN_DIRECT_UDP_PUBLIC_HOST")
        .ok()
        .map(|value| value.trim().to_string())
        .filter(|value| !value.is_empty())
        .or_else(|| direct_udp_lan_host())
        .unwrap_or_else(|| local_addr.ip().to_string());
    let report = DirectUdpEndpointReport {
        endpoint: format!("{report_host}:{}", local_addr.port()),
        endpoint_type: std::env::var("SLAN_DIRECT_UDP_ENDPOINT_TYPE")
            .ok()
            .map(|value| value.trim().to_string())
            .filter(|value| !value.is_empty())
            .unwrap_or_else(|| "lan".to_string()),
        nat_type: std::env::var("SLAN_DIRECT_UDP_NAT_TYPE")
            .ok()
            .map(|value| value.trim().to_string())
            .filter(|value| !value.is_empty())
            .unwrap_or_else(|| "unknown".to_string()),
        bind_address: local_addr.to_string(),
        updated_at_ms: current_timestamp_ms(),
    };
    let path = direct_udp_endpoint_file_path();
    let Some(parent) = path.parent() else {
        return;
    };
    if fs::create_dir_all(parent).is_err() {
        return;
    }
    let Ok(payload) = serde_json::to_vec_pretty(&report) else {
        return;
    };
    let _ = fs::write(path, payload);
}

fn clear_direct_udp_endpoint_report() {
    let path = direct_udp_endpoint_file_path();
    if path.exists() {
        let _ = fs::remove_file(path);
    }
}

fn direct_udp_lan_host() -> Option<String> {
    let socket = UdpSocket::bind("0.0.0.0:0").ok()?;
    socket.connect("8.8.8.8:80").ok()?;
    let local_addr = socket.local_addr().ok()?;
    let ip = local_addr.ip();
    (!ip.is_unspecified() && !ip.is_loopback()).then(|| ip.to_string())
}

fn persist_relay_stats(stats: &mut WindowsRelayDataPlaneStats) {
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

fn find_wintun_adapter() -> Option<String> {
    let escaped_alias = escape_powershell_single_quoted(DEFAULT_INTERFACE_NAME);
    let script = format!(
        "$adapter = Get-NetAdapter -IncludeHidden -ErrorAction SilentlyContinue | Where-Object {{ \
         $_.Name -eq '{escaped_alias}' -or \
         $_.InterfaceDescription -like '*Wintun*' -or \
         $_.InterfaceDescription -like '*WireGuard*Tunnel*' -or \
         $_.InterfaceDescription -like '*WireGuardNT*' \
         }} | Select-Object -First 1 Name; \
         if ($adapter) {{ Write-Output $adapter.Name }}"
    );
    run_powershell(&script).ok().and_then(|output| {
        output
            .lines()
            .map(str::trim)
            .find(|line| !line.is_empty())
            .map(str::to_string)
    })
}

fn wait_for_wintun_adapter() -> Option<String> {
    for _ in 0..20 {
        if let Some(adapter) = find_wintun_adapter() {
            return Some(adapter);
        }
        thread::sleep(Duration::from_millis(250));
    }
    None
}

fn rename_wintun_adapter(interface_name: &str) -> Result<()> {
    let escaped_alias = escape_powershell_single_quoted(interface_name);
    let script = format!(
        "$ErrorActionPreference='Stop'; \
         $adapter = Get-NetAdapter -IncludeHidden -ErrorAction SilentlyContinue | Where-Object {{ \
           $_.Name -eq '{escaped_alias}' -or \
           $_.InterfaceDescription -like '*Wintun*' -or \
           $_.InterfaceDescription -like '*WireGuard*Tunnel*' -or \
           $_.InterfaceDescription -like '*WireGuardNT*' \
         }} | Select-Object -First 1; \
         if (-not $adapter) {{ throw \"SLAN Wintun adapter was not found.\" }}; \
         if ($adapter.Name -ne '{escaped_alias}') {{ Rename-NetAdapter -Name $adapter.Name -NewName '{escaped_alias}' -Confirm:$false -ErrorAction Stop | Out-Null }}; \
         Write-Output '{escaped_alias}'"
    );
    run_powershell(&script).map(|_| ())
}

fn wintun_dll_candidates() -> Vec<PathBuf> {
    let mut candidates = Vec::new();
    if let Ok(current_exe) = std::env::current_exe() {
        if let Some(parent) = current_exe.parent() {
            candidates.push(parent.join("wintun.dll"));
        }
    }
    candidates.push(
        Path::new(env!("CARGO_MANIFEST_DIR")).join("../../../drivers/wintun/bin/amd64/wintun.dll"),
    );
    candidates
}

fn load_wintun_library() -> Result<Library> {
    let mut attempted = Vec::new();
    for candidate in wintun_dll_candidates() {
        attempted.push(candidate.display().to_string());
        if candidate.exists() {
            return unsafe { Library::new(&candidate) }
                .with_context(|| format!("load wintun.dll from {}", candidate.display()));
        }
    }
    bail!("wintun.dll not found; searched: {}", attempted.join(", "))
}

fn configure_adapter_ip(interface_name: &str, virtual_ip: &str, prefix_len: u8) -> Result<()> {
    let virtual_ip = virtual_ip.trim();
    if virtual_ip.is_empty() || virtual_ip.eq_ignore_ascii_case("pending") {
        bail!("device unavailable: missing assigned virtual IP");
    }
    let script = format!(
        "$name = '{}'; \
         $ip = '{}'; \
         $prefix = {}; \
         $adapter = Get-NetAdapter -IncludeHidden -Name $name -ErrorAction SilentlyContinue; \
         if (-not $adapter) {{ throw \"SLAN local network adapter '$name' was not found. Please reinstall or repair SLAN Client.\" }}; \
         Enable-NetAdapter -Name $name -Confirm:$false -ErrorAction Stop | Out-Null; \
         Start-Sleep -Milliseconds 800; \
         Set-NetIPInterface -InterfaceAlias $name -AddressFamily IPv4 -Dhcp Disabled -ErrorAction SilentlyContinue | Out-Null; \
         Get-NetIPAddress -InterfaceAlias $name -AddressFamily IPv4 -ErrorAction SilentlyContinue | Remove-NetIPAddress -Confirm:$false -ErrorAction SilentlyContinue; \
         New-NetIPAddress -InterfaceAlias $name -IPAddress $ip -PrefixLength $prefix -ErrorAction Stop | Out-Null; \
         $actual = Get-NetIPAddress -InterfaceAlias $name -AddressFamily IPv4 -ErrorAction Stop | Where-Object {{ $_.IPAddress -eq $ip }} | Select-Object -First 1 -ExpandProperty IPAddress; \
         if ($actual -ne $ip) {{ throw \"SLAN local network adapter '$name' did not apply IP '$ip'.\" }}; \
         Write-Output ($name + '|' + $ip)",
        escape_powershell_single_quoted(interface_name),
        escape_powershell_single_quoted(virtual_ip),
        prefix_len,
    );
    run_powershell(&script).map(|_| ())
}

fn verify_adapter_ip(interface_name: &str, virtual_ip: &str) -> Result<()> {
    let script = format!(
        "$name = '{}'; \
         $ip = '{}'; \
         $adapter = Get-NetAdapter -IncludeHidden -Name $name -ErrorAction Stop; \
         $actual = Get-NetIPAddress -InterfaceAlias $name -AddressFamily IPv4 -ErrorAction SilentlyContinue | Where-Object {{ $_.IPAddress -eq $ip }} | Select-Object -First 1 -ExpandProperty IPAddress; \
         if ($adapter.AdminStatus -ne 'Up') {{ throw \"SLAN local network adapter '$name' is disabled; adminStatus=\" + $adapter.AdminStatus }}; \
         if ($actual -ne $ip) {{ throw \"SLAN local network adapter '$name' expected IP '$ip' but it was not applied.\" }}; \
         Write-Output ([string]$adapter.AdminStatus + '|' + $actual)",
        escape_powershell_single_quoted(interface_name),
        escape_powershell_single_quoted(virtual_ip),
    );
    run_powershell(&script).map(|_| ())
}

fn configure_routes(interface_name: &str, routes: &[RouteSpec]) -> Result<()> {
    if routes.is_empty() {
        return Ok(());
    }
    let mut script = format!(
        "$name = '{}'; \
         $adapter = Get-NetAdapter -IncludeHidden -Name $name -ErrorAction SilentlyContinue; \
         if (-not $adapter) {{ throw \"SLAN local network adapter '$name' was not found. Please reinstall or repair SLAN Client.\" }}; \
         $ifIndex = $adapter.ifIndex; ",
        escape_powershell_single_quoted(interface_name),
    );
    for route in routes {
        let destination = route.destination.trim();
        if destination.is_empty() || destination.eq_ignore_ascii_case("mesh") {
            continue;
        }
        let gateway = route.gateway.as_deref().unwrap_or("0.0.0.0").trim();
        script.push_str(&format!(
            "New-NetRoute -DestinationPrefix '{}' -InterfaceIndex $ifIndex -NextHop '{}' -PolicyStore ActiveStore -ErrorAction SilentlyContinue | Out-Null; ",
            escape_powershell_single_quoted(destination),
            escape_powershell_single_quoted(gateway),
        ));
    }
    run_powershell(&script).map(|_| ())
}

fn configure_adapter_mtu(interface_name: &str, mtu: u16) -> Result<()> {
    let interface_name = escape_powershell_single_quoted(interface_name);
    let script = format!(
        "$name = '{interface_name}'; \
         $adapter = Get-NetAdapter -IncludeHidden -Name $name -ErrorAction SilentlyContinue; \
         if (-not $adapter) {{ throw \"SLAN local network adapter '$name' was not found. Please reinstall or repair SLAN Client.\" }}; \
         Set-NetIPInterface -InterfaceAlias $name -AddressFamily IPv4 -NlMtuBytes {mtu} -ErrorAction Stop | Out-Null; \
         $actual = (Get-NetIPInterface -InterfaceAlias $name -AddressFamily IPv4 -ErrorAction Stop | Select-Object -First 1).NlMtuBytes; \
         if ([int]$actual -ne {mtu}) {{ throw \"SLAN local network adapter '$name' MTU did not apply: \" + $actual }}; \
         Write-Output ($name + '|mtu=' + $actual)"
    );
    run_powershell(&script).map(|_| ())
}

fn configure_dns(interface_name: &str, dns_servers: &[String]) -> Result<()> {
    let servers = dns_servers
        .iter()
        .map(|value| value.trim())
        .filter(|value| is_usable_dns_server(value))
        .map(|value| format!("'{}'", escape_powershell_single_quoted(value)))
        .collect::<Vec<_>>();
    let interface_name = escape_powershell_single_quoted(interface_name);
    let script = if servers.is_empty() {
        format!(
            "$name = '{interface_name}'; \
             $adapter = Get-NetAdapter -IncludeHidden -Name $name -ErrorAction SilentlyContinue; \
             if (-not $adapter) {{ throw \"SLAN local network adapter '$name' was not found. Please reinstall or repair SLAN Client.\" }}; \
             Set-DnsClientServerAddress -InterfaceAlias $name -ResetServerAddresses -ErrorAction Stop | Out-Null; \
             $remaining = (Get-DnsClientServerAddress -InterfaceAlias $name -AddressFamily IPv4 -ErrorAction Stop).ServerAddresses; \
             if ($remaining -and $remaining.Count -gt 0) {{ throw \"SLAN local network adapter '$name' DNS reset did not apply.\" }}; \
             Write-Output ($name + '|dns-reset')",
        )
    } else {
        format!(
            "$name = '{interface_name}'; \
             $expected = @({}); \
             $adapter = Get-NetAdapter -IncludeHidden -Name $name -ErrorAction SilentlyContinue; \
             if (-not $adapter) {{ throw \"SLAN local network adapter '$name' was not found. Please reinstall or repair SLAN Client.\" }}; \
             Set-DnsClientServerAddress -InterfaceAlias $name -ServerAddresses $expected -ErrorAction Stop | Out-Null; \
             $actual = @((Get-DnsClientServerAddress -InterfaceAlias $name -AddressFamily IPv4 -ErrorAction Stop).ServerAddresses); \
             $missing = @($expected | Where-Object {{ $actual -notcontains $_ }}); \
             if ($missing.Count -gt 0) {{ throw \"SLAN local network adapter '$name' DNS did not apply: \" + ($missing -join ',') }}; \
             Write-Output ($name + '|dns=' + ($actual -join ','))",
            servers.join(","),
        )
    };
    run_powershell(&script).map(|_| ())
}

fn is_usable_dns_server(value: &str) -> bool {
    let value = value.trim();
    !value.is_empty()
        && !value.eq_ignore_ascii_case("pending")
        && !value.eq_ignore_ascii_case("null")
        && value != "0.0.0.0"
        && !value.starts_with("169.254.")
}

fn disable_adapter(interface_name: &str) -> Result<()> {
    let script = format!(
        "$name = '{}'; \
         $adapter = Get-NetAdapter -IncludeHidden -Name $name -ErrorAction SilentlyContinue; \
         if ($adapter) {{ \
           Set-NetIPInterface -InterfaceAlias $name -AddressFamily IPv4 -Dhcp Disabled -ErrorAction SilentlyContinue | Out-Null; \
           Get-NetIPAddress -InterfaceAlias $name -AddressFamily IPv4 -ErrorAction SilentlyContinue | Remove-NetIPAddress -Confirm:$false -ErrorAction SilentlyContinue; \
           Set-DnsClientServerAddress -InterfaceAlias $name -ResetServerAddresses -ErrorAction SilentlyContinue | Out-Null; \
           Disable-NetAdapter -Name $name -Confirm:$false -ErrorAction SilentlyContinue | Out-Null; \
         }}",
        escape_powershell_single_quoted(interface_name),
    );
    run_powershell(&script).map(|_| ())
}

fn read_windows_runtime_state(interface_name: &str) -> Result<NetworkRuntimeState> {
    let script = format!(
        "$name = '{}'; \
         $adapter = Get-NetAdapter -IncludeHidden -Name $name -ErrorAction SilentlyContinue; \
         if (-not $adapter) {{ Write-Output 'missing||'; exit 0 }}; \
         $ip = Get-NetIPAddress -InterfaceAlias $name -AddressFamily IPv4 -ErrorAction SilentlyContinue | \
           Where-Object {{ $_.IPAddress -and $_.IPAddress -ne '0.0.0.0' -and $_.IPAddress -notlike '169.254.*' }} | \
           Select-Object -First 1 -ExpandProperty IPAddress; \
         Write-Output ([string]$adapter.AdminStatus + '|' + $adapter.Name + '|' + $ip)",
        escape_powershell_single_quoted(interface_name),
    );
    let output = run_powershell(&script)?;
    let mut parts = output.split('|');
    let status = parts.next().unwrap_or_default().trim();
    let name = parts.next().unwrap_or_default().trim();
    let ip = parts.next().unwrap_or_default().trim();
    if status.eq_ignore_ascii_case("missing") || name.is_empty() {
        return Ok(NetworkRuntimeState::default());
    }
    let network_enabled = status.eq_ignore_ascii_case("up") && is_usable_virtual_ip(ip);
    let cached = load_cached_runtime_state().unwrap_or_default();
    Ok(NetworkRuntimeState {
        adapter_present: true,
        network_enabled,
        virtual_ip: if network_enabled {
            Some(ip.to_string())
        } else {
            None
        },
        active_path: if network_enabled {
            cached.active_path
        } else {
            None
        },
        peer_paths: if network_enabled {
            cached.peer_paths
        } else {
            Vec::new()
        },
    })
}

fn read_windows_network_diagnostics(interface_name: &str) -> Result<PlatformNetworkDiagnostics> {
    let interface_name = escape_powershell_single_quoted(interface_name);
    let script = format!(
        "$name = '{interface_name}'; \
         $adapter = Get-NetAdapter -IncludeHidden -Name $name -ErrorAction SilentlyContinue; \
         if (-not $adapter) {{ Write-Output '{{\"platform\":\"windows\",\"adapterPresent\":false,\"checks\":[{{\"name\":\"adapter\",\"ok\":false,\"message\":\"adapter missing\"}}]}}'; exit 0 }}; \
         $ip = Get-NetIPAddress -InterfaceAlias $name -AddressFamily IPv4 -ErrorAction SilentlyContinue | \
           Where-Object {{ $_.IPAddress -and $_.IPAddress -ne '0.0.0.0' -and $_.IPAddress -notlike '169.254.*' }} | \
           Select-Object -First 1; \
         $dns = @((Get-DnsClientServerAddress -InterfaceAlias $name -AddressFamily IPv4 -ErrorAction SilentlyContinue).ServerAddresses); \
         $iface = Get-NetIPInterface -InterfaceAlias $name -AddressFamily IPv4 -ErrorAction SilentlyContinue | Select-Object -First 1; \
         $routes = @(Get-NetRoute -InterfaceIndex $adapter.ifIndex -AddressFamily IPv4 -ErrorAction SilentlyContinue | Select-Object -First 20 -ExpandProperty DestinationPrefix); \
         $checks = @(); \
         $checks += [pscustomobject]@{{name='adapter';ok=$true;message=[string]$adapter.AdminStatus}}; \
         $checks += [pscustomobject]@{{name='ip';ok=($null -ne $ip);message=if ($ip) {{ $ip.IPAddress }} else {{ 'missing usable IPv4' }}}}; \
         $checks += [pscustomobject]@{{name='dns';ok=($dns.Count -gt 0);message=($dns -join ',')}}; \
         $mss = if ($iface -and [int]$iface.NlMtuBytes -ge 576) {{ [int]$iface.NlMtuBytes - 40 }} else {{ $null }}; \
         $checks += [pscustomobject]@{{name='mtu';ok=($null -ne $iface -and [int]$iface.NlMtuBytes -ge 576);message=if ($iface) {{ [string]$iface.NlMtuBytes }} else {{ 'missing' }}}}; \
         $checks += [pscustomobject]@{{name='mss';ok=($null -ne $mss -and [int]$mss -ge 536);message=if ($mss) {{ [string]$mss }} else {{ 'derived MSS unavailable' }}}}; \
         [pscustomobject]@{{ \
           platform='windows'; \
           adapterPresent=$true; \
           adapterName=[string]$adapter.Name; \
           adminStatus=[string]$adapter.AdminStatus; \
           interfaceIndex=[int]$adapter.ifIndex; \
           virtualIp=if ($ip) {{ [string]$ip.IPAddress }} else {{ $null }}; \
           mtu=if ($iface) {{ [int]$iface.NlMtuBytes }} else {{ $null }}; \
           mss=$mss; \
           dnsServers=$dns; \
           routes=$routes; \
           checks=$checks \
         }} | ConvertTo-Json -Compress -Depth 5"
    );
    let output = run_powershell(&script)?;
    serde_json::from_str::<PlatformNetworkDiagnostics>(&output)
        .context("decode Windows network diagnostics")
}

fn is_usable_virtual_ip(ip: &str) -> bool {
    let ip = ip.trim();
    !ip.is_empty() && ip != "0.0.0.0" && !ip.starts_with("169.254.")
}

fn run_powershell(script: &str) -> Result<String> {
    let output = Command::new("powershell.exe")
        .args([
            "-NoProfile",
            "-NonInteractive",
            "-ExecutionPolicy",
            "Bypass",
            "-Command",
            script,
        ])
        .creation_flags(CREATE_NO_WINDOW)
        .output()
        .context("run powershell network command")?;
    if output.status.success() {
        return Ok(String::from_utf8_lossy(&output.stdout).trim().to_string());
    }
    let stderr = String::from_utf8_lossy(&output.stderr).trim().to_string();
    let stdout = String::from_utf8_lossy(&output.stdout).trim().to_string();
    let detail = if !stderr.is_empty() {
        stderr
    } else if !stdout.is_empty() {
        stdout
    } else {
        format!("exit status {}", output.status)
    };
    bail!("windows network command failed: {detail}");
}

fn escape_powershell_single_quoted(value: &str) -> String {
    value.replace('\'', "''")
}

#[cfg(test)]
mod tests {
    use super::{
        detach_udp_relay_sessions, direct_udp_control_packet, direct_udp_control_payload,
        earliest_relay_ticket_expires_at, is_usable_dns_server, is_usable_virtual_ip,
        mark_ready_transports, relay_error_message, relay_runtime_paths_from_config,
        send_frame_to_peer, AttachedRelayPeer, AttachedRelayTcpPeer, DirectUdpControlKind,
        DirectUdpPeer, DirectUdpTransport, PathSendResult, RelayTcpTransport, RelayUdpTransport,
        WindowsPathManager, DIRECT_UDP_PONG_PACKET, DIRECT_UDP_PROBE_PACKET,
    };
    use client_core::{
        ipv4_destination, relay_frame::encode_slan_relay_data_frame, relay_frame_is_replayed,
        relay_peer_index_for_packet, select_active_path, selected_runtime_paths,
        update_peer_active_path, PathCandidate, PathKind, PathPolicy, PathState, PeerPathConfig,
        PeerPathRuntime, RelayPeerSession, RelayTicket,
    };
    use std::io::Read;
    use std::net::{TcpListener, TcpStream, UdpSocket};
    use std::sync::Mutex;
    use std::time::Duration;

    #[test]
    fn rejects_windows_link_local_autoconfig_ip() {
        assert!(!is_usable_virtual_ip(""));
        assert!(!is_usable_virtual_ip("0.0.0.0"));
        assert!(!is_usable_virtual_ip("169.254.92.148"));
        assert!(is_usable_virtual_ip("100.64.0.10"));
    }

    #[test]
    fn rejects_placeholder_dns_servers() {
        assert!(!is_usable_dns_server(""));
        assert!(!is_usable_dns_server("pending"));
        assert!(!is_usable_dns_server("null"));
        assert!(!is_usable_dns_server("0.0.0.0"));
        assert!(!is_usable_dns_server("169.254.1.1"));
        assert!(is_usable_dns_server("10.0.0.1"));
        assert!(is_usable_dns_server("8.8.8.8"));
    }

    #[test]
    fn extracts_ipv4_destination_for_routing() {
        let packet = ipv4_packet("10.0.0.2", "10.0.0.9");

        assert_eq!(ipv4_destination(&packet).as_deref(), Some("10.0.0.9"));
    }

    #[test]
    fn routes_multi_peer_packet_by_destination_virtual_ip() {
        let peer_a = vec!["10.0.0.2/32".to_string()];
        let peer_b = vec!["10.0.0.9".to_string()];
        let packet = ipv4_packet("10.0.0.1", "10.0.0.9");

        assert_eq!(
            relay_peer_index_for_packet(&[peer_a.as_slice(), peer_b.as_slice()], &packet),
            Some(1)
        );
    }

    #[test]
    fn leaves_unknown_destination_unrouted_when_multiple_peers_exist() {
        let peer_a = vec!["10.0.0.2/32".to_string()];
        let peer_b = vec!["10.0.0.9".to_string()];
        let packet = ipv4_packet("10.0.0.1", "10.0.0.99");

        assert_eq!(
            relay_peer_index_for_packet(&[peer_a.as_slice(), peer_b.as_slice()], &packet),
            None
        );
    }

    #[test]
    fn path_manager_upgrades_to_higher_priority_ready_path() {
        let policy = PathPolicy::default();
        let candidates = vec![
            path_candidate(PathKind::RelayUdp, PathState::Ready),
            path_candidate(PathKind::DirectUdp, PathState::Ready),
            path_candidate(PathKind::RelayTcp, PathState::Ready),
        ];

        assert_eq!(
            select_active_path(&policy, Some(PathKind::RelayUdp), &candidates),
            Some(PathKind::DirectUdp)
        );
    }

    #[test]
    fn path_manager_keeps_current_when_fallback_is_disabled() {
        let policy = PathPolicy {
            fallback_enabled: false,
            ..PathPolicy::default()
        };
        let candidates = vec![
            path_candidate(PathKind::RelayUdp, PathState::Ready),
            path_candidate(PathKind::DirectUdp, PathState::Ready),
        ];

        assert_eq!(
            select_active_path(&policy, Some(PathKind::RelayUdp), &candidates),
            Some(PathKind::RelayUdp)
        );
    }

    #[test]
    fn runtime_paths_preserve_candidates_and_mark_attached_relay_ready() {
        let session = test_relay_peer_session("session-b", "2026-05-03T10:00:00Z");
        let configured = vec![PeerPathConfig {
            peer_node_id: "node-peer".to_string(),
            peer_virtual_ips: vec!["10.0.0.9".to_string()],
            candidates: vec![
                path_candidate(PathKind::DirectUdp, PathState::Probing),
                path_candidate(PathKind::RelayUdp, PathState::Standby),
                path_candidate(PathKind::RelayTcp, PathState::Standby),
            ],
        }];

        let paths = selected_runtime_paths(
            &PathPolicy::default(),
            relay_runtime_paths_from_config(&configured, &[session]),
        );

        assert_eq!(paths.len(), 1);
        assert_eq!(paths[0].active_path, Some(PathKind::RelayUdp));
        assert!(paths[0]
            .candidates
            .iter()
            .any(|candidate| candidate.kind == PathKind::DirectUdp));
        assert!(paths[0].candidates.iter().any(|candidate| {
            candidate.kind == PathKind::RelayUdp && candidate.state == PathState::Ready
        }));
        assert!(paths[0]
            .candidates
            .iter()
            .any(|candidate| candidate.kind == PathKind::RelayTcp));
    }

    #[test]
    fn runtime_paths_mark_attached_relay_tcp_ready_and_select_it_when_preferred() {
        let configured = vec![PeerPathConfig {
            peer_node_id: "node-peer".to_string(),
            peer_virtual_ips: vec!["10.0.0.9".to_string()],
            candidates: vec![
                path_candidate(PathKind::RelayUdp, PathState::Ready),
                path_candidate(PathKind::RelayTcp, PathState::Standby),
            ],
        }];
        let tcp_transport = RelayTcpTransport::new(vec![AttachedRelayTcpPeer {
            peer_node_id: "node-peer".to_string(),
            peer_virtual_ips: vec!["10.0.0.9/32".to_string()],
            stream: Mutex::new(tcp_pair().0),
            last_rx_seq: 0,
        }]);
        let policy = PathPolicy {
            preferred: vec![PathKind::RelayTcp, PathKind::RelayUdp],
            ..PathPolicy::default()
        };

        let paths = selected_runtime_paths(
            &policy,
            mark_ready_transports(
                configured_runtime_paths(configured),
                None,
                Some(&tcp_transport),
            ),
        );

        assert_eq!(paths[0].active_path, Some(PathKind::RelayTcp));
        assert!(paths[0].candidates.iter().any(|candidate| {
            candidate.kind == PathKind::RelayTcp && candidate.state == PathState::Ready
        }));
    }

    #[test]
    fn runtime_paths_mark_attached_direct_udp_ready_and_select_it() {
        let configured = vec![PeerPathConfig {
            peer_node_id: "node-peer".to_string(),
            peer_virtual_ips: vec!["10.0.0.9".to_string()],
            candidates: vec![
                path_candidate(PathKind::DirectUdp, PathState::Probing),
                path_candidate(PathKind::RelayUdp, PathState::Ready),
            ],
        }];
        let direct_socket = UdpSocket::bind("127.0.0.1:0").unwrap();
        let peer_socket = UdpSocket::bind("127.0.0.1:0").unwrap();
        let direct_udp = DirectUdpTransport::new(
            direct_socket,
            vec![DirectUdpPeer {
                peer_node_id: "node-peer".to_string(),
                peer_virtual_ips: vec!["10.0.0.9/32".to_string()],
                address: peer_socket.local_addr().unwrap().to_string(),
                socket_addr: peer_socket.local_addr().unwrap(),
                last_rx_seq: 0,
            }],
        );

        let paths = selected_runtime_paths(
            &PathPolicy::default(),
            mark_ready_transports(
                configured_runtime_paths(configured),
                Some(&direct_udp),
                None,
            ),
        );

        assert_eq!(paths[0].active_path, Some(PathKind::DirectUdp));
        assert!(paths[0].candidates.iter().any(|candidate| {
            candidate.kind == PathKind::DirectUdp && candidate.state == PathState::Ready
        }));
    }

    #[test]
    fn direct_udp_transport_uses_one_node_socket_for_all_peers() {
        let direct_socket = UdpSocket::bind("127.0.0.1:0").unwrap();
        direct_socket
            .set_read_timeout(Some(Duration::from_millis(100)))
            .unwrap();
        let peer_a = UdpSocket::bind("127.0.0.1:0").unwrap();
        let peer_b = UdpSocket::bind("127.0.0.1:0").unwrap();
        peer_b
            .set_read_timeout(Some(Duration::from_millis(100)))
            .unwrap();
        let direct_addr = direct_socket.local_addr().unwrap();
        let peer_a_addr = peer_a.local_addr().unwrap();
        let peer_b_addr = peer_b.local_addr().unwrap();
        let mut direct_udp = DirectUdpTransport::new(
            direct_socket,
            vec![
                DirectUdpPeer {
                    peer_node_id: "node-a".to_string(),
                    peer_virtual_ips: vec!["10.0.0.2/32".to_string()],
                    address: peer_a_addr.to_string(),
                    socket_addr: peer_a_addr,
                    last_rx_seq: 0,
                },
                DirectUdpPeer {
                    peer_node_id: "node-b".to_string(),
                    peer_virtual_ips: vec!["10.0.0.9/32".to_string()],
                    address: peer_b_addr.to_string(),
                    socket_addr: peer_b_addr,
                    last_rx_seq: 0,
                },
            ],
        );

        let packet = ipv4_packet("10.0.0.1", "10.0.0.9");
        let peer_index = direct_udp.peer_index_for_packet(&packet).unwrap();
        assert_eq!(peer_index, 1);
        assert!(matches!(
            direct_udp.send_to_peer(peer_index, b"direct-frame"),
            PathSendResult::Sent { peer_index: 1, .. }
        ));
        let mut buffer = [0_u8; 64];
        let size = peer_b.recv(&mut buffer).unwrap();
        assert_eq!(&buffer[..size], b"direct-frame");

        peer_a.send_to(b"peer-a-frame", direct_addr).unwrap();
        let received = direct_udp.recv_from_peer(&mut buffer).unwrap().unwrap();
        assert_eq!(received.peer_index, 0);
        assert_eq!(&buffer[..received.frame_len], b"peer-a-frame");
    }

    #[test]
    fn direct_udp_receive_matches_roamed_endpoint_by_inner_source_ip() {
        let direct_socket = UdpSocket::bind("127.0.0.1:0").unwrap();
        direct_socket
            .set_read_timeout(Some(Duration::from_millis(100)))
            .unwrap();
        let direct_addr = direct_socket.local_addr().unwrap();
        let old_peer_socket = UdpSocket::bind("127.0.0.1:0").unwrap();
        let roamed_peer_socket = UdpSocket::bind("127.0.0.1:0").unwrap();
        let old_peer_addr = old_peer_socket.local_addr().unwrap();
        let roamed_peer_addr = roamed_peer_socket.local_addr().unwrap();
        let mut direct_udp = DirectUdpTransport::new(
            direct_socket,
            vec![DirectUdpPeer {
                peer_node_id: "node-a".to_string(),
                peer_virtual_ips: vec!["10.0.0.2/32".to_string()],
                address: old_peer_addr.to_string(),
                socket_addr: old_peer_addr,
                last_rx_seq: 0,
            }],
        );
        let payload = ipv4_packet("10.0.0.2", "10.0.0.1");
        let frame = encode_slan_relay_data_frame(1, 42, &payload).unwrap();

        roamed_peer_socket.send_to(&frame, direct_addr).unwrap();
        let mut buffer = [0_u8; 128];
        let received = direct_udp.recv_from_peer(&mut buffer).unwrap().unwrap();

        assert_eq!(received.peer_index, 0);
        assert_eq!(received.remote_addr, roamed_peer_addr);
        assert!(received.endpoint_changed);
        assert_eq!(&buffer[..received.frame_len], frame.as_slice());
    }

    #[test]
    fn direct_udp_updates_peer_endpoint_only_when_changed() {
        let direct_socket = UdpSocket::bind("127.0.0.1:0").unwrap();
        let old_peer_socket = UdpSocket::bind("127.0.0.1:0").unwrap();
        let roamed_peer_socket = UdpSocket::bind("127.0.0.1:0").unwrap();
        let old_peer_addr = old_peer_socket.local_addr().unwrap();
        let roamed_peer_addr = roamed_peer_socket.local_addr().unwrap();
        let mut direct_udp = DirectUdpTransport::new(
            direct_socket,
            vec![DirectUdpPeer {
                peer_node_id: "node-a".to_string(),
                peer_virtual_ips: vec!["10.0.0.2/32".to_string()],
                address: old_peer_addr.to_string(),
                socket_addr: old_peer_addr,
                last_rx_seq: 0,
            }],
        );

        assert!(!direct_udp.update_peer_endpoint(0, old_peer_addr));
        assert!(direct_udp.update_peer_endpoint(0, roamed_peer_addr));
        assert_eq!(direct_udp.peers[0].socket_addr, roamed_peer_addr);
        assert_eq!(direct_udp.peers[0].address, roamed_peer_addr.to_string());
    }

    #[test]
    fn direct_udp_unknown_control_packet_matches_single_peer() {
        let direct_socket = UdpSocket::bind("127.0.0.1:0").unwrap();
        direct_socket
            .set_read_timeout(Some(Duration::from_millis(100)))
            .unwrap();
        let direct_addr = direct_socket.local_addr().unwrap();
        let old_peer_socket = UdpSocket::bind("127.0.0.1:0").unwrap();
        let roamed_peer_socket = UdpSocket::bind("127.0.0.1:0").unwrap();
        let old_peer_addr = old_peer_socket.local_addr().unwrap();
        let roamed_peer_addr = roamed_peer_socket.local_addr().unwrap();
        let mut direct_udp = DirectUdpTransport::new(
            direct_socket,
            vec![DirectUdpPeer {
                peer_node_id: "node-a".to_string(),
                peer_virtual_ips: vec!["10.0.0.2/32".to_string()],
                address: old_peer_addr.to_string(),
                socket_addr: old_peer_addr,
                last_rx_seq: 0,
            }],
        );

        roamed_peer_socket
            .send_to(DIRECT_UDP_PROBE_PACKET, direct_addr)
            .unwrap();
        let mut buffer = [0_u8; 128];
        let received = direct_udp.recv_from_peer(&mut buffer).unwrap().unwrap();

        assert_eq!(received.peer_index, 0);
        assert_eq!(received.remote_addr, roamed_peer_addr);
        assert!(received.endpoint_changed);
        let packet = direct_udp_control_packet(&buffer[..received.frame_len]).unwrap();
        assert_eq!(packet.kind, DirectUdpControlKind::Probe);
        assert_eq!(packet.peer_node_id(), None);
    }

    #[test]
    fn direct_udp_unknown_control_packet_does_not_guess_among_multiple_peers() {
        let direct_socket = UdpSocket::bind("127.0.0.1:0").unwrap();
        direct_socket
            .set_read_timeout(Some(Duration::from_millis(100)))
            .unwrap();
        let direct_addr = direct_socket.local_addr().unwrap();
        let peer_a_socket = UdpSocket::bind("127.0.0.1:0").unwrap();
        let peer_b_socket = UdpSocket::bind("127.0.0.1:0").unwrap();
        let unknown_socket = UdpSocket::bind("127.0.0.1:0").unwrap();
        let mut direct_udp = DirectUdpTransport::new(
            direct_socket,
            vec![
                DirectUdpPeer {
                    peer_node_id: "node-a".to_string(),
                    peer_virtual_ips: vec!["10.0.0.2/32".to_string()],
                    address: peer_a_socket.local_addr().unwrap().to_string(),
                    socket_addr: peer_a_socket.local_addr().unwrap(),
                    last_rx_seq: 0,
                },
                DirectUdpPeer {
                    peer_node_id: "node-b".to_string(),
                    peer_virtual_ips: vec!["10.0.0.3/32".to_string()],
                    address: peer_b_socket.local_addr().unwrap().to_string(),
                    socket_addr: peer_b_socket.local_addr().unwrap(),
                    last_rx_seq: 0,
                },
            ],
        );

        unknown_socket
            .send_to(DIRECT_UDP_PROBE_PACKET, direct_addr)
            .unwrap();
        let mut buffer = [0_u8; 128];

        assert!(direct_udp.recv_from_peer(&mut buffer).unwrap().is_none());
    }

    #[test]
    fn direct_udp_unknown_control_packet_with_node_id_matches_multi_peer() {
        let direct_socket = UdpSocket::bind("127.0.0.1:0").unwrap();
        direct_socket
            .set_read_timeout(Some(Duration::from_millis(100)))
            .unwrap();
        let direct_addr = direct_socket.local_addr().unwrap();
        let peer_a_socket = UdpSocket::bind("127.0.0.1:0").unwrap();
        let peer_b_socket = UdpSocket::bind("127.0.0.1:0").unwrap();
        let unknown_socket = UdpSocket::bind("127.0.0.1:0").unwrap();
        let mut direct_udp = DirectUdpTransport::new(
            direct_socket,
            vec![
                DirectUdpPeer {
                    peer_node_id: "node-a".to_string(),
                    peer_virtual_ips: vec!["10.0.0.2/32".to_string()],
                    address: peer_a_socket.local_addr().unwrap().to_string(),
                    socket_addr: peer_a_socket.local_addr().unwrap(),
                    last_rx_seq: 0,
                },
                DirectUdpPeer {
                    peer_node_id: "node-b".to_string(),
                    peer_virtual_ips: vec!["10.0.0.3/32".to_string()],
                    address: peer_b_socket.local_addr().unwrap().to_string(),
                    socket_addr: peer_b_socket.local_addr().unwrap(),
                    last_rx_seq: 0,
                },
            ],
        );
        let payload = direct_udp_control_payload(DirectUdpControlKind::Probe, "node-b");

        unknown_socket
            .send_to(payload.as_bytes(), direct_addr)
            .unwrap();
        let mut buffer = [0_u8; 128];
        let received = direct_udp.recv_from_peer(&mut buffer).unwrap().unwrap();

        assert_eq!(received.peer_index, 1);
        assert!(received.endpoint_changed);
        let packet = direct_udp_control_packet(&buffer[..received.frame_len]).unwrap();
        assert_eq!(packet.peer_node_id(), Some("node-b"));
    }

    #[test]
    fn path_manager_falls_back_to_relay_udp_when_active_transport_is_missing() {
        let receiver = UdpSocket::bind("127.0.0.1:0").unwrap();
        receiver
            .set_read_timeout(Some(Duration::from_millis(100)))
            .unwrap();
        let sender = UdpSocket::bind("127.0.0.1:0").unwrap();
        sender.connect(receiver.local_addr().unwrap()).unwrap();
        let relay_udp = RelayUdpTransport::new(
            vec![AttachedRelayPeer {
                session_id: "session-a".to_string(),
                peer_node_id: "node-a".to_string(),
                peer_virtual_ips: vec!["10.0.0.9/32".to_string()],
                path_kind: PathKind::RelayUdp,
                socket: sender,
                stats_index: 0,
                last_rx_seq: 0,
            }],
            "node-local".to_string(),
        );
        let mut manager = WindowsPathManager::new(PathPolicy::default(), None, relay_udp, None);
        manager
            .tracker
            .set_active_path("node-a".to_string(), PathKind::RelayTcp);
        let packet = ipv4_packet("10.0.0.1", "10.0.0.9");

        assert!(matches!(
            manager.send(&packet, b"relay-frame"),
            PathSendResult::Sent { peer_index: 0, .. }
        ));
        let mut buffer = [0_u8; 64];
        let size = receiver.recv(&mut buffer).unwrap();
        assert_eq!(&buffer[..size], b"relay-frame");
    }

    #[test]
    fn path_manager_reports_missing_transport_when_fallback_is_disabled() {
        let receiver = UdpSocket::bind("127.0.0.1:0").unwrap();
        let sender = UdpSocket::bind("127.0.0.1:0").unwrap();
        sender.connect(receiver.local_addr().unwrap()).unwrap();
        let relay_udp = RelayUdpTransport::new(
            vec![AttachedRelayPeer {
                session_id: "session-a".to_string(),
                peer_node_id: "node-a".to_string(),
                peer_virtual_ips: vec!["10.0.0.9/32".to_string()],
                path_kind: PathKind::RelayUdp,
                socket: sender,
                stats_index: 0,
                last_rx_seq: 0,
            }],
            "node-local".to_string(),
        );
        let mut manager = WindowsPathManager::new(
            PathPolicy {
                fallback_enabled: false,
                ..PathPolicy::default()
            },
            None,
            relay_udp,
            None,
        );
        manager
            .tracker
            .set_active_path("node-a".to_string(), PathKind::RelayTcp);
        let packet = ipv4_packet("10.0.0.1", "10.0.0.9");

        assert!(matches!(
            manager.send(&packet, b"relay-frame"),
            PathSendResult::NoTransport {
                peer_index: 0,
                path_kind: PathKind::RelayTcp,
                ..
            }
        ));
    }

    #[test]
    fn path_manager_downgrades_failed_preferred_path_to_relay_udp() {
        let relay_udp = RelayUdpTransport::new(Vec::new(), "node-local".to_string());
        let mut manager = WindowsPathManager::new(PathPolicy::default(), None, relay_udp, None);
        manager
            .tracker
            .set_active_path("node-a".to_string(), PathKind::DirectUdp);

        assert!(!manager.record_send_failure("node-a", PathKind::DirectUdp));
        assert!(!manager.record_send_failure("node-a", PathKind::DirectUdp));
        assert!(manager.record_send_failure("node-a", PathKind::DirectUdp));
        assert_eq!(
            manager
                .tracker
                .active_path_for_node("node-a", PathKind::RelayUdp),
            PathKind::RelayUdp
        );
    }

    #[test]
    fn path_manager_upgrades_after_direct_udp_probe_success() {
        let relay_udp = RelayUdpTransport::new(Vec::new(), "node-local".to_string());
        let mut manager = WindowsPathManager::new(PathPolicy::default(), None, relay_udp, None);
        manager
            .tracker
            .set_active_path("node-a".to_string(), PathKind::RelayUdp);

        assert!(manager.record_probe_success("node-a", PathKind::DirectUdp));
        assert_eq!(
            manager
                .tracker
                .active_path_for_node("node-a", PathKind::RelayUdp),
            PathKind::DirectUdp
        );
    }

    #[test]
    fn path_manager_treats_inbound_direct_probe_as_success() {
        let relay_udp = RelayUdpTransport::new(Vec::new(), "node-local".to_string());
        let mut manager = WindowsPathManager::new(PathPolicy::default(), None, relay_udp, None);
        manager
            .tracker
            .set_active_path("node-a".to_string(), PathKind::RelayUdp);

        assert_eq!(
            direct_udp_control_packet(DIRECT_UDP_PROBE_PACKET).map(|packet| packet.kind),
            Some(DirectUdpControlKind::Probe)
        );
        assert!(manager.record_probe_success("node-a", PathKind::DirectUdp));
        assert_eq!(
            manager
                .tracker
                .active_path_for_node("node-a", PathKind::RelayUdp),
            PathKind::DirectUdp
        );
    }

    #[test]
    fn path_manager_reports_previous_active_path_for_peer_node() {
        let relay_udp = RelayUdpTransport::new(Vec::new(), "node-local".to_string());
        let mut manager = WindowsPathManager::new(PathPolicy::default(), None, relay_udp, None);

        assert_eq!(
            manager.active_path_for_peer_node("node-a"),
            PathKind::RelayUdp
        );
        manager
            .tracker
            .set_active_path("node-a".to_string(), PathKind::RelayTcp);
        assert_eq!(
            manager.active_path_for_peer_node("node-a"),
            PathKind::RelayTcp
        );
    }

    #[test]
    fn direct_udp_control_packets_are_recognized() {
        assert_eq!(
            direct_udp_control_packet(DIRECT_UDP_PROBE_PACKET).map(|packet| packet.kind),
            Some(DirectUdpControlKind::Probe)
        );
        assert_eq!(
            direct_udp_control_packet(DIRECT_UDP_PONG_PACKET).map(|packet| packet.kind),
            Some(DirectUdpControlKind::Pong)
        );
        let probe = direct_udp_control_payload(DirectUdpControlKind::Probe, "node-local");
        let parsed = direct_udp_control_packet(probe.as_bytes()).unwrap();
        assert_eq!(parsed.kind, DirectUdpControlKind::Probe);
        assert_eq!(parsed.peer_node_id(), Some("node-local"));
        assert_eq!(direct_udp_control_packet(b"not-control"), None);
    }

    #[test]
    fn update_peer_active_path_marks_downgraded_candidates() {
        let mut paths = vec![PeerPathRuntime {
            peer_node_id: "node-a".to_string(),
            peer_virtual_ips: vec!["10.0.0.9/32".to_string()],
            active_path: Some(PathKind::DirectUdp),
            candidates: vec![
                path_candidate(PathKind::DirectUdp, PathState::Ready),
                path_candidate(PathKind::RelayUdp, PathState::Ready),
            ],
        }];

        update_peer_active_path(&mut paths, "node-a", PathKind::RelayUdp);

        assert_eq!(paths[0].active_path, Some(PathKind::RelayUdp));
        assert!(paths[0].candidates.iter().any(|candidate| {
            candidate.kind == PathKind::DirectUdp
                && candidate.state == PathState::Degraded
                && candidate.last_error.is_some()
        }));
    }

    #[test]
    fn path_manager_uses_relay_tcp_when_transport_exists() {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let client_stream = TcpStream::connect(listener.local_addr().unwrap()).unwrap();
        let (mut server_stream, _) = listener.accept().unwrap();
        server_stream
            .set_read_timeout(Some(Duration::from_millis(100)))
            .unwrap();

        let relay_udp = RelayUdpTransport::new(
            vec![AttachedRelayPeer {
                session_id: "session-a".to_string(),
                peer_node_id: "node-a".to_string(),
                peer_virtual_ips: vec!["10.0.0.9/32".to_string()],
                path_kind: PathKind::RelayUdp,
                socket: UdpSocket::bind("127.0.0.1:0").unwrap(),
                stats_index: 0,
                last_rx_seq: 0,
            }],
            "node-local".to_string(),
        );
        let relay_tcp = RelayTcpTransport::new(vec![AttachedRelayTcpPeer {
            peer_node_id: "node-a".to_string(),
            peer_virtual_ips: vec!["10.0.0.9/32".to_string()],
            stream: Mutex::new(client_stream),
            last_rx_seq: 0,
        }]);
        let mut manager =
            WindowsPathManager::new(PathPolicy::default(), None, relay_udp, Some(relay_tcp));
        manager
            .tracker
            .set_active_path("node-a".to_string(), PathKind::RelayTcp);
        let packet = ipv4_packet("10.0.0.1", "10.0.0.9");

        assert!(matches!(
            manager.send(&packet, b"tcp-frame"),
            PathSendResult::Sent { peer_index: 0, .. }
        ));

        let mut len_buffer = [0_u8; 4];
        server_stream.read_exact(&mut len_buffer).unwrap();
        let frame_len = u32::from_be_bytes(len_buffer) as usize;
        let mut frame_buffer = vec![0_u8; frame_len];
        server_stream.read_exact(&mut frame_buffer).unwrap();
        assert_eq!(frame_buffer, b"tcp-frame");
    }

    fn tcp_pair() -> (TcpStream, TcpStream) {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let client_stream = TcpStream::connect(listener.local_addr().unwrap()).unwrap();
        let (server_stream, _) = listener.accept().unwrap();
        (client_stream, server_stream)
    }

    fn configured_runtime_paths(configured: Vec<PeerPathConfig>) -> Vec<PeerPathRuntime> {
        configured
            .into_iter()
            .map(|path| PeerPathRuntime {
                peer_node_id: path.peer_node_id,
                peer_virtual_ips: path.peer_virtual_ips,
                active_path: None,
                candidates: path.candidates,
            })
            .collect()
    }

    #[test]
    fn multi_peer_relay_socket_send_targets_matching_virtual_ip_only() {
        let receiver_a = UdpSocket::bind("127.0.0.1:0").unwrap();
        let receiver_b = UdpSocket::bind("127.0.0.1:0").unwrap();
        receiver_a
            .set_read_timeout(Some(Duration::from_millis(100)))
            .unwrap();
        receiver_b
            .set_read_timeout(Some(Duration::from_millis(100)))
            .unwrap();

        let sender_a = UdpSocket::bind("127.0.0.1:0").unwrap();
        sender_a.connect(receiver_a.local_addr().unwrap()).unwrap();
        let sender_b = UdpSocket::bind("127.0.0.1:0").unwrap();
        sender_b.connect(receiver_b.local_addr().unwrap()).unwrap();

        let peers = vec![
            AttachedRelayPeer {
                session_id: "session-a".to_string(),
                peer_node_id: "node-a".to_string(),
                peer_virtual_ips: vec!["10.0.0.2/32".to_string()],
                path_kind: PathKind::RelayUdp,
                socket: sender_a,
                stats_index: 0,
                last_rx_seq: 0,
            },
            AttachedRelayPeer {
                session_id: "session-b".to_string(),
                peer_node_id: "node-b".to_string(),
                peer_virtual_ips: vec!["10.0.0.9/32".to_string()],
                path_kind: PathKind::RelayUdp,
                socket: sender_b,
                stats_index: 1,
                last_rx_seq: 0,
            },
        ];
        let packet = ipv4_packet("10.0.0.1", "10.0.0.9");
        let frame = b"relay-frame";

        assert!(matches!(
            send_frame_to_peer(&peers, &packet, frame),
            PathSendResult::Sent { peer_index: 1, .. }
        ));

        let mut buffer = [0_u8; 64];
        assert!(receiver_a.recv(&mut buffer).is_err());
        let size = receiver_b.recv(&mut buffer).unwrap();
        assert_eq!(&buffer[..size], frame);
    }

    #[test]
    fn multi_peer_relay_socket_send_rejects_unknown_virtual_ip() {
        let receiver_a = UdpSocket::bind("127.0.0.1:0").unwrap();
        let receiver_b = UdpSocket::bind("127.0.0.1:0").unwrap();
        receiver_a
            .set_read_timeout(Some(Duration::from_millis(100)))
            .unwrap();
        receiver_b
            .set_read_timeout(Some(Duration::from_millis(100)))
            .unwrap();

        let sender_a = UdpSocket::bind("127.0.0.1:0").unwrap();
        sender_a.connect(receiver_a.local_addr().unwrap()).unwrap();
        let sender_b = UdpSocket::bind("127.0.0.1:0").unwrap();
        sender_b.connect(receiver_b.local_addr().unwrap()).unwrap();

        let peers = vec![
            AttachedRelayPeer {
                session_id: "session-a".to_string(),
                peer_node_id: "node-a".to_string(),
                peer_virtual_ips: vec!["10.0.0.2/32".to_string()],
                path_kind: PathKind::RelayUdp,
                socket: sender_a,
                stats_index: 0,
                last_rx_seq: 0,
            },
            AttachedRelayPeer {
                session_id: "session-b".to_string(),
                peer_node_id: "node-b".to_string(),
                peer_virtual_ips: vec!["10.0.0.9/32".to_string()],
                path_kind: PathKind::RelayUdp,
                socket: sender_b,
                stats_index: 1,
                last_rx_seq: 0,
            },
        ];
        let packet = ipv4_packet("10.0.0.1", "10.0.0.99");

        assert!(matches!(
            send_frame_to_peer(&peers, &packet, b"relay-frame"),
            PathSendResult::NoRoute
        ));

        let mut buffer = [0_u8; 64];
        assert!(receiver_a.recv(&mut buffer).is_err());
        assert!(receiver_b.recv(&mut buffer).is_err());
    }

    #[test]
    fn detach_sends_json_detach_for_each_relay_peer_session() {
        let receiver_a = UdpSocket::bind("127.0.0.1:0").unwrap();
        let receiver_b = UdpSocket::bind("127.0.0.1:0").unwrap();
        receiver_a
            .set_read_timeout(Some(Duration::from_millis(100)))
            .unwrap();
        receiver_b
            .set_read_timeout(Some(Duration::from_millis(100)))
            .unwrap();

        let sender_a = UdpSocket::bind("127.0.0.1:0").unwrap();
        sender_a.connect(receiver_a.local_addr().unwrap()).unwrap();
        let sender_b = UdpSocket::bind("127.0.0.1:0").unwrap();
        sender_b.connect(receiver_b.local_addr().unwrap()).unwrap();
        let peers = vec![
            AttachedRelayPeer {
                session_id: "session-a".to_string(),
                peer_node_id: "node-a".to_string(),
                peer_virtual_ips: vec!["10.0.0.2/32".to_string()],
                path_kind: PathKind::RelayUdp,
                socket: sender_a,
                stats_index: 0,
                last_rx_seq: 0,
            },
            AttachedRelayPeer {
                session_id: "session-b".to_string(),
                peer_node_id: "node-b".to_string(),
                peer_virtual_ips: vec!["10.0.0.9/32".to_string()],
                path_kind: PathKind::RelayUdp,
                socket: sender_b,
                stats_index: 1,
                last_rx_seq: 0,
            },
        ];

        detach_udp_relay_sessions(&peers, "node-local");

        assert_detach_packet(&receiver_a, "session-a", "node-local");
        assert_detach_packet(&receiver_b, "session-b", "node-local");
    }

    #[test]
    fn relay_json_error_response_is_observable() {
        let payload = br#"{"kind":"error","code":"peer_not_attached","message":"target participant has not attached"}"#;

        assert_eq!(
            relay_error_message(payload).as_deref(),
            Some("peer_not_attached: target participant has not attached")
        );
    }

    #[test]
    fn non_error_json_is_not_counted_as_relay_error() {
        let payload = br#"{"kind":"attached","session_id":"session-a"}"#;

        assert_eq!(relay_error_message(payload), None);
    }

    #[test]
    fn relay_replay_guard_rejects_duplicate_or_older_sequence() {
        assert!(!relay_frame_is_replayed(0, 1));
        assert!(!relay_frame_is_replayed(10, 11));
        assert!(relay_frame_is_replayed(10, 10));
        assert!(relay_frame_is_replayed(10, 9));
    }

    #[test]
    fn relay_ticket_diagnose_uses_earliest_expiration() {
        let sessions = vec![
            test_relay_peer_session("session-a", "2026-05-03T11:00:00Z"),
            test_relay_peer_session("session-b", "2026-05-03T10:00:00Z"),
        ];

        assert_eq!(
            earliest_relay_ticket_expires_at(&sessions).as_deref(),
            Some("2026-05-03T10:00:00Z")
        );
    }

    #[test]
    fn relay_ticket_diagnose_ignores_blank_expiration() {
        let sessions = vec![
            test_relay_peer_session("session-a", ""),
            test_relay_peer_session("session-b", "2026-05-03T10:00:00Z"),
        ];

        assert_eq!(
            earliest_relay_ticket_expires_at(&sessions).as_deref(),
            Some("2026-05-03T10:00:00Z")
        );
    }

    fn ipv4_packet(src: &str, dst: &str) -> Vec<u8> {
        let mut packet = vec![0_u8; 20];
        packet[0] = 0x45;
        packet[2] = 0;
        packet[3] = 20;
        packet[8] = 64;
        packet[9] = 17;
        write_ipv4(src, &mut packet[12..16]);
        write_ipv4(dst, &mut packet[16..20]);
        packet
    }

    fn write_ipv4(value: &str, target: &mut [u8]) {
        for (index, part) in value.split('.').enumerate().take(4) {
            target[index] = part.parse::<u8>().unwrap();
        }
    }

    fn path_candidate(kind: PathKind, state: PathState) -> PathCandidate {
        PathCandidate {
            kind,
            state,
            endpoint_id: None,
            address: None,
            session_id: None,
            transport: None,
            rtt_ms: None,
            path_score: None,
            last_ok_at_ms: None,
            last_error: None,
        }
    }

    fn assert_detach_packet(receiver: &UdpSocket, session_id: &str, participant_id: &str) {
        let mut buffer = [0_u8; 512];
        let size = receiver.recv(&mut buffer).unwrap();
        let value: serde_json::Value = serde_json::from_slice(&buffer[..size]).unwrap();
        assert_eq!(
            value.get("kind").and_then(serde_json::Value::as_str),
            Some("detach")
        );
        assert_eq!(
            value.get("session_id").and_then(serde_json::Value::as_str),
            Some(session_id)
        );
        assert_eq!(
            value
                .get("participant_id")
                .and_then(serde_json::Value::as_str),
            Some(participant_id)
        );
    }

    fn test_relay_peer_session(session_id: &str, expires_at: &str) -> RelayPeerSession {
        RelayPeerSession {
            session_id: session_id.to_string(),
            peer_node_id: "node-peer".to_string(),
            peer_virtual_ips: vec!["10.0.0.2".to_string()],
            ticket: RelayTicket {
                ticket_id: format!("ticket-{session_id}"),
                network_id: "net-1".to_string(),
                session_id: session_id.to_string(),
                src_node_id: "node-local".to_string(),
                dst_node_id: "node-peer".to_string(),
                derp_cluster_id: None,
                country_code: None,
                city_code: None,
                allowed_derp_node_ids: Vec::new(),
                relay_url: "udp://127.0.0.1:3478".to_string(),
                expires_at: expires_at.to_string(),
                session_key: "session-key".to_string(),
                signature: "signature".to_string(),
            },
        }
    }
}
