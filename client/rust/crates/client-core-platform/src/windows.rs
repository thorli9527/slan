use std::{
    collections::{HashMap, HashSet},
    ffi::{c_void, OsStr},
    fs::{self, OpenOptions},
    io::{BufRead, BufReader, ErrorKind, Read, Write},
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
use client_core::path::{PathProbeController, PathProbeRole};
use client_core::{
    acl_allows_egress_packet, acl_allows_ingress_packet, icmp_echo_reply_for_request,
    ipv4_destination, ipv4_source, ipv4_transport_checksum_valid, mark_peer_path_probe_success,
    normalize_ipv4_transport_checksums, normalize_virtual_ip,
    relay_frame::{
        base64_decode, base64_encode, decode_slan_relay_data_frame, encode_slan_relay_data_frame,
        stable_hash64,
    },
    relay_peer_index_for_packet, resolver_response_for_query, selected_runtime_paths,
    update_peer_active_path, NetworkRuntimeState, NodeConfig, PathKind, PathPolicy, PathState,
    PathTracker, PeerPathRuntime, PlatformAclPeer, PlatformNetwork, PlatformNetworkDiagnostics,
    PlatformResolverConfig, PlatformResolverRecord, RelayDataPlaneConfig, RelayPeerSession,
    RouteSpec,
};
use libloading::Library;
use serde::{Deserialize, Serialize};

use crate::direct_udp::configured_direct_udp_port;
use crate::effective_resolver_servers;

const DEFAULT_INTERFACE_NAME: &str = "SLAN LAN Adapter";
const WINDOWS_WINTUN_DRIVER_TYPE: &str = "Wintun";
const HOST_INTERFACE_PREFIX_LEN: u8 = 32;
const CREATE_NO_WINDOW: u32 = 0x08000000;
const RELAY_ATTACH_ATTEMPTS: usize = 3;
const RELAY_ATTACH_TIMEOUT: Duration = Duration::from_secs(2);
const RELAY_STATS_FLUSH_INTERVAL: Duration = Duration::from_secs(10);
const RELAY_KEEPALIVE_INTERVAL: Duration = Duration::from_secs(30);
const RELAY_DATA_PLANE_FAILURE_THRESHOLD: u32 = 20;
const PATH_SEND_FAILURES_BEFORE_DOWNGRADE: u32 = 3;
const DERP_WRITE_RETRY_TIMEOUT: Duration = Duration::from_millis(750);
const DATA_PLANE_IDLE_SLEEP: Duration = Duration::from_millis(2);
const RELAY_TICKET_RENEW_WINDOW_MS: u64 = 5 * 60 * 1000;

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

/// RawGuid 是调用 Wintun 动态库创建 adapter 时使用的 C ABI GUID。
#[repr(C)]
struct RawGuid {
    data1: u32,
    data2: u16,
    data3: u16,
    data4: [u8; 8],
}

/// WintunRuntime 持有 Wintun adapter、session、动态库函数指针和数据面线程。
struct WintunRuntime {
    _library: Library,
    handle: WintunAdapterHandle,
    session: WintunSessionHandle,
    start_session: WintunStartSessionFunc,
    end_session: WintunEndSessionFunc,
    close_adapter: WintunCloseAdapterFunc,
    receive_packet: WintunReceivePacketFunc,
    release_receive_packet: WintunReleaseReceivePacketFunc,
    allocate_send_packet: WintunAllocateSendPacketFunc,
    send_packet: WintunSendPacketFunc,
    data_plane: Option<WindowsDataPlaneRuntime>,
}

/// WindowsDataPlaneRuntime 持有 Windows relay/direct UDP 数据面后台线程生命周期。
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

/// WindowsPlatformNetwork 是 Windows 的 PlatformNetwork 实现，负责 Wintun、
/// 路由、DNS、relay 数据面和 direct UDP runtime。
#[derive(Debug, Clone, Default)]
pub struct WindowsPlatformNetwork;

/// WindowsRuntime 保存 Windows 平台层当前网络配置缓存。
/// 与 macOS/Linux 一致，`read_runtime_state` 从内存读取，不调用外部进程。
#[derive(Debug, Default)]
struct WindowsRuntime {
    adapter_present: bool,
    network_enabled: bool,
    virtual_ip: Option<String>,
    prefix_len: Option<u8>,
    routes: Vec<RouteSpec>,
    relay_config: Option<RelayDataPlaneConfig>,
    mtu: Option<u16>,
}

static WINDOWS_NETWORK_RUNTIME: OnceLock<Mutex<WindowsRuntime>> = OnceLock::new();

fn windows_network_runtime() -> &'static Mutex<WindowsRuntime> {
    WINDOWS_NETWORK_RUNTIME.get_or_init(|| {
        let cached = load_cached_runtime_state().unwrap_or_default();
        Mutex::new(WindowsRuntime {
            adapter_present: cached.adapter_present,
            network_enabled: cached.network_enabled,
            virtual_ip: cached.virtual_ip,
            ..WindowsRuntime::default()
        })
    })
}

#[derive(Debug, Clone, Default)]
struct WindowsResolverRuntime {
    servers: Vec<String>,
    records: Vec<PlatformResolverRecord>,
}

static WINDOWS_RESOLVER_RUNTIME: OnceLock<Mutex<WindowsResolverRuntime>> = OnceLock::new();

fn windows_resolver_runtime() -> &'static Mutex<WindowsResolverRuntime> {
    WINDOWS_RESOLVER_RUNTIME.get_or_init(|| Mutex::new(WindowsResolverRuntime::default()))
}

impl PlatformNetwork for WindowsPlatformNetwork {
    fn install_adapter(&self) -> Result<()> {
        ensure_adapter_present(DEFAULT_INTERFACE_NAME)?;
        let mut runtime = windows_network_runtime()
            .lock()
            .expect("windows network runtime lock poisoned");
        runtime.adapter_present = true;
        persist_state(&NetworkRuntimeState {
            adapter_present: true,
            ..load_cached_runtime_state().unwrap_or_default()
        })
    }

    fn configure_ip(&self, virtual_ip: &str, _prefix_len: u8) -> Result<()> {
        ensure_installed_adapter_ready(DEFAULT_INTERFACE_NAME)
            .context("ensure installed Wintun adapter before IP")?;
        let runtime_ip_matches = windows_network_runtime()
            .lock()
            .expect("windows network runtime lock poisoned")
            .virtual_ip
            .as_deref()
            == Some(virtual_ip);
        let cached_ip_matches = load_cached_runtime_state()
            .ok()
            .and_then(|state| state.virtual_ip)
            .as_deref()
            == Some(virtual_ip);
        if (runtime_ip_matches || cached_ip_matches)
            && verify_adapter_ip(DEFAULT_INTERFACE_NAME, virtual_ip).is_ok()
        {
            debug_log(&format!(
                "configure_ip: fast path — Wintun already owns {virtual_ip}, skipping address rewrite"
            ));
            return Ok(());
        }
        configure_adapter_ip(
            DEFAULT_INTERFACE_NAME,
            virtual_ip,
            HOST_INTERFACE_PREFIX_LEN,
        )
        .context("configure Wintun adapter IP")?;
        verify_adapter_ip(DEFAULT_INTERFACE_NAME, virtual_ip)
            .context("verify Wintun adapter IP")?;
        let mut runtime = windows_network_runtime()
            .lock()
            .expect("windows network runtime lock poisoned");
        runtime.adapter_present = true;
        runtime.network_enabled = true;
        runtime.virtual_ip = Some(virtual_ip.to_string());
        runtime.prefix_len = Some(HOST_INTERFACE_PREFIX_LEN);
        let mut state = load_cached_runtime_state().unwrap_or_default();
        state.adapter_present = true;
        state.network_enabled = true;
        state.virtual_ip = Some(virtual_ip.to_string());
        persist_state(&state)
    }

    fn configure_routes(&self, routes: &[RouteSpec]) -> Result<()> {
        // Fast path: skip if routes haven't changed (prevents adapter toggle from repeated netsh calls).
        let runtime = windows_network_runtime()
            .lock()
            .expect("windows network runtime lock poisoned");
        if runtime.routes == routes {
            debug_log("configure_routes: fast path — routes unchanged, skipping");
            return Ok(());
        }
        drop(runtime);
        configure_routes(DEFAULT_INTERFACE_NAME, routes).context("configure Wintun routes")?;
        let mut runtime = windows_network_runtime()
            .lock()
            .expect("windows network runtime lock poisoned");
        runtime.routes = routes.to_vec();
        persist_state(&load_cached_runtime_state().unwrap_or_default())
    }

    fn configure_resolver(&self, resolver: &PlatformResolverConfig) -> Result<()> {
        let effective = effective_resolver_servers(&resolver.servers);
        configure_dns(DEFAULT_INTERFACE_NAME, &effective).context("configure Wintun DNS")?;
        let routes = effective
            .iter()
            .filter(|server| server.parse::<std::net::Ipv4Addr>().is_ok())
            .map(|server| RouteSpec {
                destination: format!("{server}/32"),
                gateway: None,
            })
            .collect::<Vec<_>>();
        configure_routes(DEFAULT_INTERFACE_NAME, &routes)
            .context("configure Wintun DNS service route")?;
        windows_resolver_runtime()
            .lock()
            .expect("windows resolver runtime mutex poisoned")
            .servers = effective;
        persist_state(&load_cached_runtime_state().unwrap_or_default())
    }

    fn configure_resolver_map(
        &self,
        _resolver_zones: &[client_core::PlatformResolverZone],
        resolver_records: &[PlatformResolverRecord],
    ) -> Result<()> {
        let mut runtime = windows_resolver_runtime()
            .lock()
            .expect("windows resolver runtime mutex poisoned");
        let changed = runtime.records != resolver_records;
        runtime.records = resolver_records.to_vec();
        drop(runtime);
        if changed {
            let _ = run_powershell("Clear-DnsClientCache -ErrorAction SilentlyContinue");
        }
        Ok(())
    }

    fn configure_relay(&self, config: Option<&RelayDataPlaneConfig>) -> Result<()> {
        // Fast path: skip if relay config hasn't changed.
        let runtime = windows_network_runtime()
            .lock()
            .expect("windows network runtime lock poisoned");
        if runtime.relay_config.as_ref() == config {
            debug_log("configure_relay: fast path — relay config unchanged, skipping");
            return Ok(());
        }
        drop(runtime);
        configure_wintun_data_plane(config).context("configure Wintun relay data plane")?;
        let mut runtime = windows_network_runtime()
            .lock()
            .expect("windows network runtime lock poisoned");
        runtime.relay_config = config.cloned();
        Ok(())
    }

    fn disable_network(&self) -> Result<()> {
        stop_wintun_data_plane();
        let disable_result = disable_adapter(DEFAULT_INTERFACE_NAME);
        {
            let mut resolver = windows_resolver_runtime()
                .lock()
                .expect("windows resolver runtime mutex poisoned");
            resolver.servers.clear();
            resolver.records.clear();
        }
        let _ = run_powershell("Clear-DnsClientCache -ErrorAction SilentlyContinue");
        disable_result?;
        let mut runtime = windows_network_runtime()
            .lock()
            .expect("windows network runtime lock poisoned");
        runtime.network_enabled = false;
        runtime.virtual_ip = None;
        runtime.routes.clear();
        runtime.relay_config = None;
        let mut state = load_cached_runtime_state().unwrap_or_default();
        state.network_enabled = false;
        state.virtual_ip = None;
        state.active_path = None;
        state.peer_paths.clear();
        persist_state(&state)
    }

    fn read_runtime_state(&self) -> Result<NetworkRuntimeState> {
        // Read from in-memory runtime cache — same pattern as macOS/Linux.
        // This avoids shelling out to netsh/PowerShell every 10 seconds,
        // which caused frequent timeouts and unreliable adapter state detection.
        let runtime = windows_network_runtime()
            .lock()
            .expect("windows network runtime lock poisoned");
        let cached = load_cached_runtime_state().unwrap_or_default();
        // A service/runtime restart resets the process-local cache while the
        // adapter and persisted desired state remain active. Treat that state
        // as enabled until an explicit disable clears the persisted state.
        let recovered_virtual_ip = runtime
            .virtual_ip
            .clone()
            .or_else(|| cached.virtual_ip.clone());
        let network_enabled =
            runtime.network_enabled || (cached.network_enabled && recovered_virtual_ip.is_some());
        if !network_enabled {
            debug_log(&format!(
                "read_runtime_state: network_enabled=false adapter_present={} virtual_ip={:?} cached_enabled={}",
                runtime.adapter_present, runtime.virtual_ip, cached.network_enabled,
            ));
        } else if !runtime.network_enabled {
            debug_log(&format!(
                "read_runtime_state: recovered enabled state from persisted runtime virtual_ip={:?}",
                recovered_virtual_ip,
            ));
        }
        Ok(NetworkRuntimeState {
            adapter_present: runtime.adapter_present || network_enabled,
            network_enabled,
            virtual_ip: if network_enabled {
                recovered_virtual_ip
            } else {
                None
            },
            active_path: if network_enabled {
                runtime
                    .relay_config
                    .as_ref()
                    .filter(|relay| relay.enabled && !relay.sessions.is_empty())
                    .and_then(|relay| {
                        let paths = mark_ready_transports(
                            relay_runtime_paths_from_config(&relay.peer_paths, &relay.sessions),
                            None,
                        );
                        client_core::selected_runtime_paths(&relay.path_policy, paths)
                            .into_iter()
                            .find_map(|path| path.active_path)
                    })
                    .or(cached.active_path)
            } else {
                None
            },
            peer_paths: if network_enabled {
                runtime
                    .relay_config
                    .as_ref()
                    .map(|relay| {
                        let peer_paths = mark_ready_transports(
                            relay_runtime_paths_from_config(&relay.peer_paths, &relay.sessions),
                            None,
                        );
                        client_core::selected_runtime_paths(&relay.path_policy, peer_paths)
                    })
                    .unwrap_or_default()
            } else {
                Vec::new()
            },
        })
    }

    fn mark_network_enabled(&self, virtual_ip: &str) -> Result<()> {
        let mut runtime = windows_network_runtime()
            .lock()
            .expect("windows network runtime lock poisoned");
        runtime.adapter_present = true;
        runtime.network_enabled = true;
        runtime.virtual_ip = Some(virtual_ip.to_string());
        let mut state = load_cached_runtime_state().unwrap_or_default();
        state.adapter_present = true;
        state.network_enabled = true;
        state.virtual_ip = Some(virtual_ip.to_string());
        persist_state(&state)
    }

    fn verify_adapter_ip(&self, virtual_ip: &str) -> Result<bool> {
        verify_adapter_ip(DEFAULT_INTERFACE_NAME, virtual_ip)
            .map(|()| true)
            .or(Ok(false))
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
    // Fast path: check if adapter is already Up before calling Enable-NetAdapter.
    // Repeated Enable-NetAdapter calls can cause Windows to toggle the adapter state.
    let check_script = format!(
        "$adapter = Get-NetAdapter -IncludeHidden -Name '{}' -ErrorAction SilentlyContinue; \
         if (-not $adapter) {{ Write-Output 'not_found' }} else {{ Write-Output $adapter.AdminStatus.ToString() }}",
        escape_powershell_single_quoted(interface_name),
    );
    if let Ok(status) = run_powershell(&check_script) {
        let status = status.trim();
        if status == "Up" {
            debug_log(&format!(
                "ensure_installed_adapter_ready: fast path — adapter already Up, skipping"
            ));
            return Ok(());
        }
    }
    let script = format!(
        "$name = '{}'; \
         $adapter = Get-NetAdapter -IncludeHidden -Name $name -ErrorAction SilentlyContinue; \
         if (-not $adapter) {{ throw \"SLAN local network adapter '$name' was not found. Please reinstall or repair SLAN Client.\" }}; \
         $before = $adapter.AdminStatus.ToString(); \
         Enable-NetAdapter -IncludeHidden -Name $name -Confirm:$false -ErrorAction Stop | Out-Null; \
         $after = (Get-NetAdapter -IncludeHidden -Name $name -ErrorAction SilentlyContinue).AdminStatus.ToString(); \
         Write-Output \"before=$before after=$after\"",
        escape_powershell_single_quoted(interface_name),
    );
    debug_log(&format!(
        "ensure_installed_adapter_ready: enabling '{interface_name}'"
    ));
    let result = run_powershell(&script);
    debug_log(&format!(
        "ensure_installed_adapter_ready: result={result:?}"
    ));
    let output = result?;
    if adapter_enable_requires_session_restart(&output) {
        restart_wintun_session().context("restart Wintun session after adapter enable")?;
    }
    Ok(())
}

fn adapter_enable_requires_session_restart(output: &str) -> bool {
    output
        .split_whitespace()
        .find_map(|item| item.strip_prefix("before="))
        .is_some_and(|status| !status.eq_ignore_ascii_case("up"))
}

fn restart_wintun_session() -> Result<()> {
    let runtime = WINTUN_RUNTIME.get_or_init(|| Mutex::new(None));
    let mut runtime = runtime.lock().expect("wintun runtime mutex poisoned");
    let runtime = runtime
        .as_mut()
        .ok_or_else(|| anyhow::anyhow!("Wintun runtime is not ready"))?;
    let _ = runtime.data_plane.take();
    if !runtime.session.is_null() {
        unsafe {
            (runtime.end_session)(runtime.session);
        }
        runtime.session = std::ptr::null_mut();
    }
    let session = unsafe { (runtime.start_session)(runtime.handle, 0x400000) };
    if session.is_null() {
        bail!(
            "failed to restart Wintun session: {}",
            std::io::Error::last_os_error()
        );
    }
    runtime.session = session;
    debug_log("restart_wintun_session: session restarted after adapter enable");
    Ok(())
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
        start_session,
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
    local_node_id: String,
    peer_virtual_ips: Vec<String>,
    path_kind: PathKind,
    socket: UdpSocket,
    stats_index: usize,
}

struct DerpPeer {
    peer_node_id: String,
    local_node_id: String,
    server_session_id: String,
    peer_virtual_ips: Vec<String>,
    stream: TcpStream,
    reader: TcpStream,
    read_buffer: Vec<u8>,
    stats_index: usize,
}

struct RelayUdpTransport {
    peers: Vec<AttachedRelayPeer>,
    local_node_id: String,
}

struct DerpTcpTransport {
    peers: Vec<DerpPeer>,
}

struct DirectUdpPeer {
    peer_node_id: String,
    peer_virtual_ips: Vec<String>,
    path_kind: PathKind,
    address: String,
    socket_addr: SocketAddr,
}

#[derive(Debug, Clone)]
struct DirectUdpProbeTarget {
    path_kind: PathKind,
    address: String,
    socket_addr: SocketAddr,
}

struct DirectUdpTransport {
    socket: UdpSocket,
    local_node_id: String,
    peers: Vec<DirectUdpPeer>,
    probe_targets: HashMap<String, Vec<DirectUdpProbeTarget>>,
    probe_controllers: HashMap<String, PathProbeController>,
}

struct DirectUdpReceive {
    peer_index: usize,
    frame_len: usize,
    remote_addr: SocketAddr,
    endpoint_changed: bool,
}

struct DirectUdpProbeBatch {
    sent_count: usize,
    failed_paths: Vec<(String, PathKind)>,
}

impl DirectUdpTransport {
    #[cfg(test)]
    fn new(socket: UdpSocket, peers: Vec<DirectUdpPeer>) -> Self {
        let probe_controllers = direct_udp_probe_controllers(&peers);
        let probe_targets = peers
            .iter()
            .map(|peer| {
                (
                    peer.peer_node_id.clone(),
                    vec![DirectUdpProbeTarget {
                        path_kind: peer.path_kind,
                        address: peer.address.clone(),
                        socket_addr: peer.socket_addr,
                    }],
                )
            })
            .collect();
        Self {
            socket,
            local_node_id: "node-local".to_string(),
            peers,
            probe_targets,
            probe_controllers,
        }
    }

    fn attach(
        local_node_id: &str,
        configured_paths: &[client_core::PeerPathConfig],
    ) -> Option<Self> {
        let mut peers = Vec::new();
        let mut probe_targets = HashMap::new();
        let socket = match attach_direct_udp_socket() {
            Ok(socket) => socket,
            Err(error) => {
                eprintln!("client-core-platform direct udp attach skipped error={error:#}");
                clear_direct_udp_endpoint_report();
                return None;
            }
        };
        for path in configured_paths {
            let targets = udp_probe_targets_for_peer(path);
            let Some(primary) = targets.first() else {
                continue;
            };
            peers.push(DirectUdpPeer {
                peer_node_id: path.peer_node_id.clone(),
                peer_virtual_ips: path.peer_virtual_ips.clone(),
                path_kind: primary.path_kind,
                address: primary.address.clone(),
                socket_addr: primary.socket_addr,
            });
            probe_targets.insert(path.peer_node_id.clone(), targets);
        }
        if peers.is_empty() {
            clear_direct_udp_endpoint_report();
            return None;
        }
        persist_direct_udp_endpoint_report(&socket);
        let probe_controllers = direct_udp_probe_controllers(&peers);
        Some(Self {
            socket,
            local_node_id: local_node_id.to_string(),
            peers,
            probe_targets,
            probe_controllers,
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
        if !self
            .probe_controllers
            .get(&peer.peer_node_id)
            .is_some_and(PathProbeController::usable)
        {
            return PathSendResult::NoRoute;
        }
        if self.socket.send_to(frame, peer.socket_addr).is_ok() {
            PathSendResult::Sent {
                peer_index,
                peer_node_id: peer.peer_node_id.clone(),
                path_kind: peer.path_kind,
            }
        } else {
            PathSendResult::SendFailed {
                peer_index,
                peer_node_id: peer.peer_node_id.clone(),
                path_kind: peer.path_kind,
            }
        }
    }

    fn recv_from_peer(&mut self, buffer: &mut [u8]) -> std::io::Result<Option<DirectUdpReceive>> {
        let (frame_len, remote_addr) = self.socket.recv_from(buffer)?;
        if self.handle_punch_response(&buffer[..frame_len]) {
            return Ok(None);
        }
        if let Some(peer_index) = self.peers.iter().position(|peer| {
            peer.socket_addr == remote_addr
                || self
                    .probe_targets
                    .get(&peer.peer_node_id)
                    .is_some_and(|targets| {
                        targets
                            .iter()
                            .any(|target| target.socket_addr == remote_addr)
                    })
        }) {
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
            return self
                .peers
                .iter()
                .position(|peer| peer.peer_node_id == control_packet.peer_node_id());
        }
        let payload = decode_slan_relay_data_frame(frame)?;
        let source = ipv4_source(payload)?;
        self.peers.iter().position(|peer| {
            peer.peer_virtual_ips
                .iter()
                .any(|ip| normalize_virtual_ip(ip) == source)
        })
    }

    fn peer_count(&self) -> usize {
        self.peers.len()
    }

    fn send_probe_packets(
        &mut self,
        active_peer_node_ids: &HashSet<String>,
    ) -> DirectUdpProbeBatch {
        let now_ms = current_timestamp_ms();
        let payload = direct_udp_control_payload(DirectUdpControlKind::Probe, &self.local_node_id);
        let mut sent_count = 0;
        let mut failed_paths = Vec::new();
        for peer in &self.peers {
            let Some(controller) = self.probe_controllers.get_mut(&peer.peer_node_id) else {
                continue;
            };
            let was_usable = controller.usable();
            let role = if active_peer_node_ids.contains(&peer.peer_node_id) {
                PathProbeRole::Active
            } else {
                PathProbeRole::Standby
            };
            if !controller.should_probe(now_ms, role) {
                if was_usable && !controller.usable() {
                    failed_paths.push((peer.peer_node_id.clone(), peer.path_kind));
                }
                continue;
            }
            let sent = self
                .probe_targets
                .get(&peer.peer_node_id)
                .map(|targets| {
                    targets
                        .iter()
                        .filter(|target| {
                            self.socket
                                .send_to(payload.as_bytes(), target.socket_addr)
                                .is_ok()
                        })
                        .count()
                })
                .unwrap_or_else(|| {
                    usize::from(
                        self.socket
                            .send_to(payload.as_bytes(), peer.socket_addr)
                            .is_ok(),
                    )
                });
            if sent > 0 {
                controller.on_probe_sent(now_ms);
                sent_count += 1;
            }
        }
        DirectUdpProbeBatch {
            sent_count,
            failed_paths,
        }
    }

    fn mark_peer_inbound(&mut self, peer_index: usize) {
        let Some(peer) = self.peers.get(peer_index) else {
            return;
        };
        if let Some(controller) = self.probe_controllers.get_mut(&peer.peer_node_id) {
            controller.on_inbound(current_timestamp_ms());
        }
    }

    fn send_punch_endpoint_probes(&self, network_id: &str, node_configs: &[NodeConfig]) -> usize {
        let payload = serde_json::json!({
            "kind": "endpoint_probe",
            "networkId": network_id,
            "nodeId": self.local_node_id,
            "type": "direct_udp",
            "natType": "unknown",
        })
        .to_string();
        node_configs
            .iter()
            .filter(|node| node.is_direct_udp_discovery())
            .filter_map(|node| resolve_direct_udp_peer_address(&node.address).ok())
            .filter(|address| self.socket.send_to(payload.as_bytes(), address).is_ok())
            .count()
    }

    fn handle_punch_response(&self, frame: &[u8]) -> bool {
        let Ok(value) = serde_json::from_slice::<serde_json::Value>(frame) else {
            return false;
        };
        if value.get("kind").and_then(serde_json::Value::as_str) != Some("endpoint_reflexive") {
            return false;
        }
        if let Some(endpoint) = value
            .get("endpoint")
            .and_then(|value| value.get("reflexive"))
            .and_then(serde_json::Value::as_str)
            .map(str::trim)
            .filter(|value| !value.is_empty())
        {
            persist_direct_udp_reflexive_endpoint(&self.socket, endpoint);
        }
        true
    }

    fn send_pong_to_peer(&self, peer_index: usize) -> bool {
        if let Some(peer) = self.peers.get(peer_index) {
            let payload =
                direct_udp_control_payload(DirectUdpControlKind::Pong, &self.local_node_id);
            return self
                .socket
                .send_to(payload.as_bytes(), peer.socket_addr)
                .is_ok();
        }
        false
    }

    fn update_peer_endpoint(&mut self, peer_index: usize, remote_addr: SocketAddr) -> bool {
        let Some(peer) = self.peers.get_mut(peer_index) else {
            return false;
        };
        if let Some(target) = self
            .probe_targets
            .get(&peer.peer_node_id)
            .and_then(|targets| {
                targets
                    .iter()
                    .find(|target| target.socket_addr == remote_addr)
            })
        {
            peer.path_kind = target.path_kind;
        } else if peer.socket_addr != remote_addr {
            peer.path_kind = PathKind::DirectUdp;
        }
        if peer.socket_addr == remote_addr {
            return false;
        }
        peer.socket_addr = remote_addr;
        peer.address = remote_addr.to_string();
        true
    }
}

fn direct_udp_probe_controllers(peers: &[DirectUdpPeer]) -> HashMap<String, PathProbeController> {
    peers
        .iter()
        .map(|peer| (peer.peer_node_id.clone(), PathProbeController::new()))
        .collect()
}

struct RelayUdpAttachResult {
    transport: RelayUdpTransport,
    peer_stats: Vec<WindowsRelayPeerStats>,
    attach_failures: u64,
    last_attach_error: Option<String>,
}

struct DerpTcpAttachResult {
    transport: DerpTcpTransport,
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
            if relay_path_kind_from_ticket(session) != Some(PathKind::RelayUdp) {
                continue;
            }
            let stats_index = peer_stats.len();
            let session_relay_address = relay_udp_address_for_session(relay_address, session)
                .unwrap_or_else(|| relay_address.trim().to_string());
            match attach_udp_relay_session(session_relay_address.as_str(), local_node_id, session) {
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
                        local_node_id: local_node_id.to_string(),
                        peer_virtual_ips: session.peer_virtual_ips.clone(),
                        path_kind: PathKind::RelayUdp,
                        socket,
                        stats_index,
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

    fn send_to_peer(&self, peer_index: usize, frame: &[u8], payload: &[u8]) -> PathSendResult {
        let Some(peer) = self.peers.get(peer_index) else {
            return PathSendResult::NoRoute;
        };
        let mut sent = false;
        for attempt in 0..relay_send_attempt_count(payload) {
            if attempt > 0 {
                thread::sleep(relay_send_attempt_delay(payload));
            }
            if peer.path_kind != PathKind::RelayUdp {
                continue;
            }
            let Some(payload_bytes) = encode_relay_forward(peer, frame) else {
                continue;
            };
            sent |= peer.socket.send(&payload_bytes).is_ok();
        }
        if !sent {
            PathSendResult::SendFailed {
                peer_index,
                peer_node_id: peer.peer_node_id.clone(),
                path_kind: PathKind::RelayUdp,
            }
        } else {
            PathSendResult::Sent {
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

impl DerpTcpTransport {
    fn new(peers: Vec<DerpPeer>) -> Self {
        Self { peers }
    }

    fn attach(local_node_id: &str, sessions: &[RelayPeerSession]) -> DerpTcpAttachResult {
        let mut peers = Vec::new();
        let mut peer_stats = Vec::new();
        let mut attach_failures = 0_u64;
        let mut last_attach_error = None;
        for session in sessions {
            if relay_path_kind_from_ticket(session) != Some(PathKind::DerpTcpTls443) {
                continue;
            }
            let stats_index = peer_stats.len();
            match attach_derp_relay_session(local_node_id, session, stats_index) {
                Ok(peer) => {
                    peer_stats.push(WindowsRelayPeerStats {
                        peer_node_id: session.peer_node_id.clone(),
                        session_id: session.session_id.clone(),
                        peer_virtual_ips: session.peer_virtual_ips.clone(),
                        attached: true,
                        last_send_path: Some(PathKind::DerpTcpTls443.as_str().to_string()),
                        ..WindowsRelayPeerStats::default()
                    });
                    peers.push(peer);
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
                        last_send_path: Some(PathKind::DerpTcpTls443.as_str().to_string()),
                        ..WindowsRelayPeerStats::default()
                    });
                }
            }
        }
        DerpTcpAttachResult {
            transport: Self::new(peers),
            peer_stats,
            attach_failures,
            last_attach_error,
        }
    }

    fn peer_count(&self) -> usize {
        self.peers.len()
    }

    fn peers(&self) -> &[DerpPeer] {
        &self.peers
    }

    fn peers_mut(&mut self) -> &mut [DerpPeer] {
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

    fn peer_index_by_node(&self, peer_node_id: &str) -> Option<usize> {
        self.peers
            .iter()
            .position(|peer| peer.peer_node_id == peer_node_id)
    }

    fn send_to_peer(&self, peer_index: usize, frame: &[u8], payload: &[u8]) -> PathSendResult {
        let Some(peer) = self.peers.get(peer_index) else {
            return PathSendResult::NoRoute;
        };
        let mut sent = false;
        for attempt in 0..relay_send_attempt_count(payload) {
            if attempt > 0 {
                thread::sleep(relay_send_attempt_delay(payload));
            }
            sent |= send_derp_forward(peer, frame).is_ok();
        }
        if sent {
            PathSendResult::Sent {
                peer_index,
                peer_node_id: peer.peer_node_id.clone(),
                path_kind: PathKind::DerpTcpTls443,
            }
        } else {
            PathSendResult::SendFailed {
                peer_index,
                peer_node_id: peer.peer_node_id.clone(),
                path_kind: PathKind::DerpTcpTls443,
            }
        }
    }
}

impl Drop for DerpTcpTransport {
    fn drop(&mut self) {
        detach_derp_relay_sessions(&mut self.peers);
    }
}

struct WindowsPathManager {
    tracker: PathTracker,
    direct_udp: Option<DirectUdpTransport>,
    relay_udp: RelayUdpTransport,
    derp_tcp: DerpTcpTransport,
    peer_paths: Vec<PeerPathRuntime>,
}

struct PacketRoute {
    peer_node_id: String,
    peer_virtual_ips: Vec<String>,
    default_path: PathKind,
    relay_udp_index: Option<usize>,
    derp_tcp_index: Option<usize>,
}

impl WindowsPathManager {
    fn new(
        policy: PathPolicy,
        direct_udp: Option<DirectUdpTransport>,
        relay_udp: RelayUdpTransport,
        derp_tcp: DerpTcpTransport,
    ) -> Self {
        let mut active_paths = relay_udp
            .peers()
            .iter()
            .map(|peer| (peer.peer_node_id.clone(), peer.path_kind))
            .collect::<Vec<_>>();
        active_paths.extend(
            derp_tcp
                .peers()
                .iter()
                .map(|peer| (peer.peer_node_id.clone(), PathKind::DerpTcpTls443)),
        );
        Self {
            tracker: PathTracker::new(policy, active_paths),
            direct_udp,
            relay_udp,
            derp_tcp,
            peer_paths: Vec::new(),
        }
    }

    fn apply_runtime_paths(&mut self, peer_paths: &[PeerPathRuntime]) {
        self.tracker.apply_runtime_paths(peer_paths);
        self.peer_paths = peer_paths.to_vec();
    }

    fn relay_udp_peers_mut(&mut self) -> &mut [AttachedRelayPeer] {
        self.relay_udp.peers_mut()
    }

    fn relay_udp_peers(&self) -> &[AttachedRelayPeer] {
        self.relay_udp.peers()
    }

    fn derp_peers_mut(&mut self) -> &mut [DerpPeer] {
        self.derp_tcp.peers_mut()
    }

    fn direct_udp_transport_mut(&mut self) -> Option<&mut DirectUdpTransport> {
        self.direct_udp.as_mut()
    }

    fn send(&self, payload: &[u8], frame: &[u8]) -> PathSendResult {
        let Some(route) = self.route_for_packet(payload) else {
            return PathSendResult::NoRoute;
        };
        let active_path = self.active_path_for_route(&route);
        match active_path {
            PathKind::LanUdp | PathKind::Ipv6Udp | PathKind::DirectUdp => {
                let result = self.direct_udp.as_ref().and_then(|transport| {
                    transport
                        .peer_index_for_packet(payload)
                        .map(|index| transport.send_to_peer(index, frame))
                });
                match result {
                    Some(PathSendResult::Sent {
                        peer_index,
                        peer_node_id,
                        path_kind,
                    }) => {
                        self.hedge_direct_packet_to_relay(&route, payload, frame);
                        PathSendResult::Sent {
                            peer_index,
                            peer_node_id,
                            path_kind,
                        }
                    }
                    Some(result) => self.fallback_after_send_failure(
                        &route,
                        active_path,
                        payload,
                        frame,
                        result,
                    ),
                    None => self.fallback_or_missing(&route, active_path, payload, frame),
                }
            }
            PathKind::RelayUdp => {
                if route.relay_udp_index.is_some() {
                    route
                        .relay_udp_index
                        .map(|peer_index| self.relay_udp.send_to_peer(peer_index, frame, payload))
                        .map(|result| {
                            self.fallback_after_send_failure(
                                &route,
                                PathKind::RelayUdp,
                                payload,
                                frame,
                                result,
                            )
                        })
                        .unwrap_or_else(|| {
                            self.fallback_or_missing(&route, PathKind::RelayUdp, payload, frame)
                        })
                } else if route.derp_tcp_index.is_some() {
                    // Relay UDP transport has no peers but DERP TCP does.
                    // This happens when relay sessions use DERP URLs.
                    route
                        .derp_tcp_index
                        .map(|peer_index| self.derp_tcp.send_to_peer(peer_index, frame, payload))
                        .map(|result| {
                            self.fallback_after_send_failure(
                                &route,
                                PathKind::DerpTcpTls443,
                                payload,
                                frame,
                                result,
                            )
                        })
                        .unwrap_or_else(|| {
                            self.fallback_or_missing(
                                &route,
                                PathKind::DerpTcpTls443,
                                payload,
                                frame,
                            )
                        })
                } else {
                    self.fallback_or_missing(&route, PathKind::RelayUdp, payload, frame)
                }
            }
            PathKind::DerpTcpTls443 => {
                if route.derp_tcp_index.is_some() {
                    route
                        .derp_tcp_index
                        .map(|peer_index| self.derp_tcp.send_to_peer(peer_index, frame, payload))
                        .map(|result| {
                            self.fallback_after_send_failure(
                                &route,
                                active_path,
                                payload,
                                frame,
                                result,
                            )
                        })
                        .unwrap_or_else(|| {
                            self.fallback_or_missing(&route, active_path, payload, frame)
                        })
                } else if route.relay_udp_index.is_some() {
                    // DERP TCP transport has no peers but relay UDP does.
                    route
                        .relay_udp_index
                        .map(|peer_index| self.relay_udp.send_to_peer(peer_index, frame, payload))
                        .map(|result| {
                            self.fallback_after_send_failure(
                                &route,
                                PathKind::RelayUdp,
                                payload,
                                frame,
                                result,
                            )
                        })
                        .unwrap_or_else(|| {
                            self.fallback_or_missing(&route, PathKind::RelayUdp, payload, frame)
                        })
                } else {
                    self.fallback_or_missing(&route, active_path, payload, frame)
                }
            }
        }
    }

    fn route_for_packet(&self, payload: &[u8]) -> Option<PacketRoute> {
        if let Some(peer_index) = self.relay_udp.peer_index_for_packet(payload) {
            if let Some(peer) = self.relay_udp.peers().get(peer_index) {
                return Some(PacketRoute {
                    peer_node_id: peer.peer_node_id.clone(),
                    peer_virtual_ips: peer.peer_virtual_ips.clone(),
                    default_path: peer.path_kind,
                    relay_udp_index: Some(peer_index),
                    derp_tcp_index: self.derp_tcp.peer_index_by_node(&peer.peer_node_id),
                });
            }
        }
        if let Some(direct_udp) = &self.direct_udp {
            if let Some(peer_index) = direct_udp.peer_index_for_packet(payload) {
                if let Some(peer) = direct_udp.peers.get(peer_index) {
                    return Some(PacketRoute {
                        peer_node_id: peer.peer_node_id.clone(),
                        peer_virtual_ips: peer.peer_virtual_ips.clone(),
                        default_path: peer.path_kind,
                        relay_udp_index: self.relay_udp_index_by_node(&peer.peer_node_id),
                        derp_tcp_index: self.derp_tcp.peer_index_by_node(&peer.peer_node_id),
                    });
                }
            }
        }
        if let Some(peer_index) = self.derp_tcp.peer_index_for_packet(payload) {
            if let Some(peer) = self.derp_tcp.peers().get(peer_index) {
                return Some(PacketRoute {
                    peer_node_id: peer.peer_node_id.clone(),
                    peer_virtual_ips: peer.peer_virtual_ips.clone(),
                    default_path: PathKind::DerpTcpTls443,
                    relay_udp_index: self.relay_udp_index_by_node(&peer.peer_node_id),
                    derp_tcp_index: Some(peer_index),
                });
            }
        }
        None
    }

    fn relay_udp_index_by_node(&self, peer_node_id: &str) -> Option<usize> {
        self.relay_udp
            .peers()
            .iter()
            .position(|peer| peer.peer_node_id == peer_node_id)
    }

    fn active_path_for_route(&self, route: &PacketRoute) -> PathKind {
        self.tracker
            .active_path_for_node(route.peer_node_id.as_str(), route.default_path)
    }

    fn active_path_for_peer_node(&self, peer_node_id: &str) -> PathKind {
        self.tracker
            .active_path_for_node(peer_node_id, PathKind::RelayUdp)
    }

    fn fallback_path_for_node(
        &self,
        peer_node_id: &str,
        failed_path: PathKind,
    ) -> Option<PathKind> {
        self.tracker.preferred_paths().into_iter().find(|path| {
            *path != failed_path
                && self.path_available_for_node(peer_node_id, *path)
                && path_candidate_is_ready(&self.peer_paths, peer_node_id, *path)
        })
    }

    fn update_peer_paths(&mut self, peer_paths: &[PeerPathRuntime]) {
        self.peer_paths = peer_paths.to_vec();
    }

    fn path_available_for_node(&self, peer_node_id: &str, path_kind: PathKind) -> bool {
        match path_kind {
            PathKind::LanUdp | PathKind::Ipv6Udp | PathKind::DirectUdp => {
                self.direct_udp.as_ref().is_some_and(|transport| {
                    transport.peers.iter().any(|peer| {
                        peer.peer_node_id == peer_node_id && peer.path_kind == path_kind
                    })
                })
            }
            PathKind::RelayUdp => self.relay_udp_index_by_node(peer_node_id).is_some(),
            PathKind::DerpTcpTls443 => self.derp_tcp.peer_index_by_node(peer_node_id).is_some(),
        }
    }

    fn record_send_success(&mut self, peer_node_id: &str) {
        self.tracker.record_send_success(peer_node_id);
    }

    fn record_sent_path(&mut self, peer_node_id: &str, path_kind: PathKind) -> Option<PathKind> {
        let previous_path = self.active_path_for_peer_node(peer_node_id);
        self.record_send_success(peer_node_id);
        if previous_path == path_kind {
            return None;
        }
        self.tracker
            .set_active_path(peer_node_id.to_string(), path_kind);
        Some(previous_path)
    }

    fn record_send_failure(&mut self, peer_node_id: &str, path_kind: PathKind) -> bool {
        self.tracker.record_send_failure_at(
            peer_node_id,
            path_kind,
            PATH_SEND_FAILURES_BEFORE_DOWNGRADE,
            Some(current_timestamp_ms()),
        )
    }

    fn record_probe_timeout(&mut self, peer_node_id: &str, path_kind: PathKind) -> bool {
        self.tracker
            .record_send_failure_at(peer_node_id, path_kind, 1, None)
    }

    fn record_probe_success(&mut self, peer_node_id: &str, path_kind: PathKind) -> bool {
        self.tracker.record_probe_success(peer_node_id, path_kind)
    }

    fn active_path_summary(&self) -> String {
        self.tracker.active_path_summary()
    }

    fn fallback_or_missing(
        &self,
        route: &PacketRoute,
        path_kind: PathKind,
        payload: &[u8],
        frame: &[u8],
    ) -> PathSendResult {
        if self.tracker.fallback_enabled() {
            if let Some(fallback_path) = self.fallback_path_for_node(&route.peer_node_id, path_kind)
            {
                if let Some(result) =
                    self.send_available_path(&route.peer_node_id, fallback_path, payload, frame)
                {
                    return result;
                }
            }
        }
        PathSendResult::NoTransport {
            peer_index: route.relay_udp_index.unwrap_or(0),
            peer_node_id: route.peer_node_id.clone(),
            path_kind,
        }
    }

    fn fallback_after_send_failure(
        &self,
        route: &PacketRoute,
        path_kind: PathKind,
        payload: &[u8],
        frame: &[u8],
        original: PathSendResult,
    ) -> PathSendResult {
        match original {
            PathSendResult::SendFailed { .. } | PathSendResult::NoTransport { .. } => {
                if self.tracker.fallback_enabled() {
                    if let Some(fallback_path) =
                        self.fallback_path_for_node(&route.peer_node_id, path_kind)
                    {
                        if let Some(result) = self.send_available_path(
                            &route.peer_node_id,
                            fallback_path,
                            payload,
                            frame,
                        ) {
                            return result;
                        }
                    }
                }
                original
            }
            _ => original,
        }
    }

    fn send_available_path(
        &self,
        peer_node_id: &str,
        path_kind: PathKind,
        payload: &[u8],
        frame: &[u8],
    ) -> Option<PathSendResult> {
        match path_kind {
            PathKind::RelayUdp => self
                .relay_udp_index_by_node(peer_node_id)
                .map(|peer_index| self.relay_udp.send_to_peer(peer_index, frame, payload)),
            PathKind::DerpTcpTls443 => self
                .derp_tcp
                .peer_index_by_node(peer_node_id)
                .map(|peer_index| self.derp_tcp.send_to_peer(peer_index, frame, payload)),
            PathKind::LanUdp | PathKind::Ipv6Udp | PathKind::DirectUdp => {
                self.direct_udp.as_ref().and_then(|transport| {
                    transport
                        .peers
                        .iter()
                        .position(|peer| {
                            peer.peer_node_id == peer_node_id && peer.path_kind == path_kind
                        })
                        .map(|peer_index| transport.send_to_peer(peer_index, frame))
                })
            }
        }
    }

    fn hedge_direct_packet_to_relay(&self, route: &PacketRoute, payload: &[u8], frame: &[u8]) {
        if !should_hedge_direct_packet_to_relay(payload) {
            return;
        }
        if let Some(peer_index) = route.relay_udp_index {
            let _ = self.relay_udp.send_to_peer(peer_index, frame, payload);
        }
    }
}

#[derive(Debug, Default, Serialize)]
#[serde(rename_all = "camelCase")]
struct WindowsRelayDataPlaneStats {
    relay_address: String,
    relay_transport: String,
    network_id: String,
    local_node_id: String,
    active_path: String,
    path_policy: PathPolicy,
    path_count: u32,
    requested_relay_session_count: u32,
    relay_session_count: u32,
    attached_peer_session_count: u32,
    attached_transport_count: u32,
    ticket_expires_at: Option<String>,
    ticket_expires_in_ms: Option<i64>,
    ticket_renew_due: bool,
    relay_attach_failures: u64,
    last_relay_attach_error: Option<String>,
    peers: Vec<WindowsRelayPeerStats>,
    relay_mtu: Option<u32>,
    max_frame_payload: Option<u32>,
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
            && !value.sessions.is_empty()
            && (!value.transport.eq_ignore_ascii_case("udp")
                || !value.relay_address.trim().is_empty())
    }) else {
        return configure_wintun_local_data_plane();
    };
    let relay_address = config.relay_address.trim();
    let relay_mtu = config.relay_mtu.unwrap_or(1280).clamp(576, 1500);
    let max_frame_payload = usize::from(config.max_frame_payload.unwrap_or(1200).clamp(512, 1400));
    let runtime = WINTUN_RUNTIME.get_or_init(|| Mutex::new(None));
    let mut runtime = runtime.lock().expect("wintun runtime mutex poisoned");
    let Some(runtime) = runtime.as_mut() else {
        bail!("Wintun runtime is not ready");
    };
    // Release the previous direct UDP socket before binding the replacement.
    // Otherwise every config refresh falls back from 41642 to a random port and
    // peers keep probing stale LAN endpoints.
    if runtime.data_plane.take().is_some() {
        debug_log("configure_wintun_data_plane: stopped previous data plane before rebind");
    }
    if let Err(error) = configure_adapter_mtu(DEFAULT_INTERFACE_NAME, relay_mtu) {
        eprintln!(
            "SLAN warning: Wintun MTU configuration failed (mtu={relay_mtu}): {error:#}. \
             Continuing with current adapter MTU."
        );
    }
    let requested_relay_session_count = config.sessions.len() as u32;
    let ticket_expires_at = earliest_relay_ticket_expires_at(&config.sessions);
    let relay_udp_attach = RelayUdpTransport::attach(
        relay_address,
        config.local_node_id.as_str(),
        &config.sessions,
    );
    let derp_tcp_attach = DerpTcpTransport::attach(config.local_node_id.as_str(), &config.sessions);
    let relay_udp_attachment_count = relay_udp_attach.transport.peer_count() as u32;
    let derp_tcp_attachment_count = derp_tcp_attach.transport.peer_count() as u32;
    let stop = Arc::new(AtomicBool::new(false));
    let thread_stop = Arc::clone(&stop);
    let session = runtime.session as usize;
    let receive_packet = runtime.receive_packet;
    let release_receive_packet = runtime.release_receive_packet;
    let allocate_send_packet = runtime.allocate_send_packet;
    let send_packet = runtime.send_packet;
    let resolver = windows_resolver_runtime()
        .lock()
        .expect("windows resolver runtime mutex poisoned")
        .clone();
    let dns_servers = resolver.servers;
    let dns_records = resolver.records;
    let relay_address_owned = relay_address.to_string();
    let config_transport = config.transport.clone();
    let network_id = config.network_id.clone();
    let direct_network_id = network_id.clone();
    let node_configs = config.node_configs.clone();
    let local_node_id = config.local_node_id.clone();
    let local_virtual_ip = load_cached_runtime_state()
        .ok()
        .and_then(|state| state.virtual_ip)
        .unwrap_or_default();
    let config_path_policy = config.path_policy.clone();
    let config_path_count = config.peer_paths.len() as u32;
    let config_peer_paths = config.peer_paths.clone();
    let acl_policies = config.acl_policies.clone();
    let attached_peer_paths = relay_runtime_paths_from_config(&config.peer_paths, &config.sessions);
    let direct_udp_transport =
        DirectUdpTransport::attach(config.local_node_id.as_str(), &config_peer_paths);
    let direct_udp_attached_peer_count = direct_udp_transport
        .as_ref()
        .map(DirectUdpTransport::peer_count)
        .unwrap_or(0) as u64;
    let mut selected_peer_paths = selected_runtime_paths(
        &config_path_policy,
        mark_ready_transports(attached_peer_paths.clone(), direct_udp_transport.as_ref()),
    );
    let active_path = selected_peer_paths
        .iter()
        .find_map(|path| path.active_path)
        .unwrap_or(PathKind::RelayUdp);
    let attached_transport_count = relay_udp_attachment_count
        + derp_tcp_attachment_count
        + direct_udp_transport
            .as_ref()
            .map(DirectUdpTransport::peer_count)
            .unwrap_or(0) as u32;
    let RelayUdpAttachResult {
        transport: relay_udp_transport,
        mut peer_stats,
        attach_failures: _relay_udp_attach_failures,
        last_attach_error: relay_udp_last_attach_error,
    } = relay_udp_attach;
    let DerpTcpAttachResult {
        transport: mut derp_tcp_transport,
        peer_stats: derp_peer_stats,
        attach_failures: _derp_tcp_attach_failures,
        last_attach_error: derp_tcp_last_attach_error,
    } = derp_tcp_attach;
    let derp_stats_offset = peer_stats.len();
    for peer in &mut derp_tcp_transport.peers {
        peer.stats_index += derp_stats_offset;
    }
    peer_stats.extend(derp_peer_stats);
    mark_peer_stats_attached_from_transports(&mut peer_stats, direct_udp_transport.as_ref());
    let relay_attach_failures = peer_stats.iter().filter(|peer| !peer.attached).count() as u64;
    let relay_session_count = peer_stats.len() as u32;
    let attached_peer_session_count = peer_stats.iter().filter(|peer| peer.attached).count() as u32;
    let last_relay_attach_error = if relay_attach_failures == 0 {
        None
    } else {
        relay_udp_last_attach_error.or(derp_tcp_last_attach_error)
    };
    if attached_transport_count == 0 {
        let mut stats = WindowsRelayDataPlaneStats {
            relay_address: relay_address.to_string(),
            relay_transport: config.transport.clone(),
            network_id: config.network_id.clone(),
            local_node_id: config.local_node_id.clone(),
            active_path: active_path.as_str().to_string(),
            path_policy: config.path_policy.clone(),
            path_count: config.peer_paths.len() as u32,
            requested_relay_session_count,
            relay_session_count,
            attached_peer_session_count,
            attached_transport_count: 0,
            ticket_expires_at,
            relay_attach_failures,
            last_relay_attach_error: last_relay_attach_error.clone(),
            peers: peer_stats,
            relay_mtu: Some(u32::from(relay_mtu)),
            max_frame_payload: Some(max_frame_payload as u32),
            direct_udp_attached_peer_count,
            direct_udp_ready_peer_count: direct_udp_attached_peer_count,
            started_at_ms: current_timestamp_ms(),
            ..WindowsRelayDataPlaneStats::default()
        };
        persist_relay_stats(&mut stats);
        bail!(
            "{}",
            last_relay_attach_error
                .unwrap_or_else(|| "no Windows data plane transport attached".to_string())
        );
    }
    let config_hash = stable_hash64(&format!(
        "{}:{}:{}",
        config.network_id, config.local_node_id, relay_address
    ));
    // Keep the old data plane alive until the replacement has attached at
    // least one usable transport. This makes relay/ticket reconfigure closer
    // to a no-gap rebuild: attach failures are observable, but they do not
    // tear down an otherwise working path.
    let _ = runtime.data_plane.take();
    let handle = thread::spawn(move || {
        let session = session as WintunSessionHandle;
        let mut seq = 0_u64;
        let mut relay_buffer = vec![0_u8; 4096];
        persist_active_path_state(active_path, selected_peer_paths.clone());
        let mut path_manager = WindowsPathManager::new(
            config_path_policy.clone(),
            direct_udp_transport,
            relay_udp_transport,
            derp_tcp_transport,
        );
        path_manager.apply_runtime_paths(&selected_peer_paths);
        let mut stats = WindowsRelayDataPlaneStats {
            relay_address: relay_address_owned,
            relay_transport: config_transport,
            network_id,
            local_node_id,
            active_path: active_path.as_str().to_string(),
            path_policy: config_path_policy,
            path_count: config_path_count,
            requested_relay_session_count,
            relay_session_count,
            attached_peer_session_count,
            attached_transport_count,
            ticket_expires_at,
            relay_attach_failures,
            last_relay_attach_error,
            peers: peer_stats,
            relay_mtu: Some(u32::from(relay_mtu)),
            max_frame_payload: Some(max_frame_payload as u32),
            direct_udp_attached_peer_count,
            direct_udp_ready_peer_count: direct_udp_attached_peer_count,
            started_at_ms: current_timestamp_ms(),
            ..WindowsRelayDataPlaneStats::default()
        };
        let mut last_stats_flush = Instant::now();
        let mut last_keepalive = Instant::now()
            .checked_sub(RELAY_KEEPALIVE_INTERVAL)
            .unwrap_or_else(Instant::now);
        let direct_udp_probe_interval = Duration::from_secs(1);
        let mut last_direct_udp_probe = Instant::now()
            .checked_sub(direct_udp_probe_interval)
            .unwrap_or_else(Instant::now);
        let mut consecutive_data_plane_failures = 0_u32;
        persist_relay_stats(&mut stats);
        while !thread_stop.load(Ordering::SeqCst) {
            path_manager.update_peer_paths(&selected_peer_paths);
            if last_keepalive.elapsed() >= RELAY_KEEPALIVE_INTERVAL {
                send_relay_keepalives(path_manager.relay_udp_peers());
                stats.last_relay_keepalive_at_ms = Some(current_timestamp_ms());
                last_keepalive = Instant::now();
            }
            if last_direct_udp_probe.elapsed() >= direct_udp_probe_interval {
                let active_direct_peer_ids = selected_peer_paths
                    .iter()
                    .filter(|path| path.active_path.is_some_and(PathKind::is_direct_udp))
                    .map(|path| path.peer_node_id.clone())
                    .collect::<HashSet<_>>();
                // 发送新一轮探测
                let probe_batch = if let Some(direct_udp) = path_manager.direct_udp_transport_mut()
                {
                    let _ =
                        direct_udp.send_punch_endpoint_probes(&direct_network_id, &node_configs);
                    Some(direct_udp.send_probe_packets(&active_direct_peer_ids))
                } else {
                    None
                };
                if let Some(probe_batch) = probe_batch {
                    stats.direct_udp_probes_sent = stats
                        .direct_udp_probes_sent
                        .saturating_add(probe_batch.sent_count as u64);
                    for (peer_node_id, path_kind) in probe_batch.failed_paths {
                        let should_failover =
                            path_manager.record_probe_timeout(&peer_node_id, path_kind);
                        for peer_path in selected_peer_paths.iter_mut() {
                            if peer_path.peer_node_id == peer_node_id {
                                for candidate in peer_path.candidates.iter_mut() {
                                    if candidate.kind == path_kind {
                                        candidate.state = PathState::Degraded;
                                        candidate.last_error =
                                            Some("direct udp adaptive probe failed".to_string());
                                    }
                                }
                            }
                        }
                        if should_failover {
                            path_manager
                                .tracker
                                .set_active_path(peer_node_id.clone(), PathKind::RelayUdp);
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
                                    "{} -> relay_udp after adaptive probe failure",
                                    path_kind.as_str()
                                ));
                            }
                            persist_active_path_state(
                                PathKind::RelayUdp,
                                selected_peer_paths.clone(),
                            );
                        }
                    }
                }
                last_direct_udp_probe = Instant::now();
            }
            let mut packet_size = 0_u32;
            let packet = unsafe { receive_packet(session, &mut packet_size as *mut u32) };
            if !packet.is_null() && packet_size > 0 {
                let payload = unsafe { std::slice::from_raw_parts(packet, packet_size as usize) };
                if payload.first().map(|byte| byte >> 4) == Some(4) {
                    if let Some(reply) = local_dns_reply(payload, &dns_servers, &dns_records) {
                        let _ =
                            write_wintun_packet(session, allocate_send_packet, send_packet, &reply);
                        unsafe {
                            release_receive_packet(session, packet);
                        }
                        continue;
                    }
                    if packet_targets_local_virtual_ip(payload, local_virtual_ip.as_str()) {
                        if let Some(reply) =
                            local_virtual_ip_reply(payload, local_virtual_ip.as_str())
                        {
                            let _ = write_wintun_packet(
                                session,
                                allocate_send_packet,
                                send_packet,
                                &reply,
                            );
                        } else {
                            let packet = normalize_ipv4_transport_checksums(payload);
                            let _ = write_wintun_packet(
                                session,
                                allocate_send_packet,
                                send_packet,
                                &packet,
                            );
                        }
                        unsafe {
                            release_receive_packet(session, packet);
                        }
                        thread::sleep(DATA_PLANE_IDLE_SLEEP);
                        continue;
                    }
                    stats.last_tun_packet_at_ms = Some(current_timestamp_ms());
                    stats.last_tun_destination = ipv4_destination(payload);
                    stats.last_tun_protocol = ipv4_protocol(payload);
                    stats.last_tun_packet_size = Some(payload.len() as u32);
                    stats.last_tun_peer_node_id = None;
                    stats.last_tun_send_path = None;
                    stats.last_tun_drop_reason = None;
                }
                if payload.len() > max_frame_payload {
                    stats.oversized_tun_packets = stats.oversized_tun_packets.saturating_add(1);
                    stats.last_oversized_tun_packet_size = Some(payload.len() as u32);
                    stats.last_tun_drop_reason = Some("oversized".to_string());
                    unsafe {
                        release_receive_packet(session, packet);
                    }
                    thread::sleep(DATA_PLANE_IDLE_SLEEP);
                    continue;
                }
                let payload = normalize_ipv4_transport_checksums(payload);
                record_tun_tcp_packet(&mut stats, &payload);
                if let Some(route) = path_manager.route_for_packet(&payload) {
                    let acl_peer = PlatformAclPeer {
                        peer_node_id: Some(route.peer_node_id.clone()),
                        peer_virtual_ips: route.peer_virtual_ips.clone(),
                    };
                    if !acl_allows_egress_packet(&payload, &acl_policies, Some(&acl_peer)) {
                        stats.last_tun_peer_node_id = Some(route.peer_node_id);
                        stats.last_tun_drop_reason = Some("acl_egress_denied".to_string());
                        unsafe {
                            release_receive_packet(session, packet);
                        }
                        thread::sleep(DATA_PLANE_IDLE_SLEEP);
                        continue;
                    }
                }
                if let Some(frame) =
                    encode_slan_relay_data_frame(seq.wrapping_add(1), config_hash, &payload)
                {
                    seq = seq.wrapping_add(1);
                    match path_manager.send(&payload, &frame) {
                        PathSendResult::Sent {
                            peer_node_id,
                            path_kind,
                            ..
                        } => {
                            stats.last_tun_peer_node_id = Some(peer_node_id.clone());
                            stats.last_tun_send_path = Some(path_kind.as_str().to_string());
                            if let Some(previous_path) =
                                path_manager.record_sent_path(&peer_node_id, path_kind)
                            {
                                update_peer_active_path(
                                    &mut selected_peer_paths,
                                    &peer_node_id,
                                    path_kind,
                                );
                                stats.active_path = path_manager.active_path_summary();
                                if let Some(peer_stats) =
                                    peer_stats_mut_by_node_id(&mut stats.peers, &peer_node_id)
                                {
                                    peer_stats.path_downgrades =
                                        peer_stats.path_downgrades.saturating_add(1);
                                    peer_stats.last_path_change = Some(format!(
                                        "{} -> {} after fallback send success",
                                        previous_path.as_str(),
                                        path_kind.as_str()
                                    ));
                                }
                                persist_active_path_state(path_kind, selected_peer_paths.clone());
                            }
                            stats.tun_packets_sent = stats.tun_packets_sent.saturating_add(1);
                            if is_direct_udp_path(path_kind) {
                                stats.direct_udp_frames_sent =
                                    stats.direct_udp_frames_sent.saturating_add(1);
                            }
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
                            if let Some(destination) = ipv4_destination(&payload) {
                                if should_ignore_unroutable_destination(&destination) {
                                    stats.last_tun_drop_reason =
                                        Some("ignored_unroutable_destination".to_string());
                                } else {
                                    stats.unroutable_tun_packets =
                                        stats.unroutable_tun_packets.saturating_add(1);
                                    stats.last_unroutable_destination = Some(destination);
                                    stats.last_tun_drop_reason = Some("unroutable".to_string());
                                }
                            } else {
                                stats.last_tun_drop_reason = Some("unroutable".to_string());
                            }
                        }
                        PathSendResult::SendFailed {
                            peer_node_id,
                            path_kind,
                            ..
                        } => {
                            stats.last_tun_peer_node_id = Some(peer_node_id.clone());
                            stats.last_tun_send_path = Some(path_kind.as_str().to_string());
                            stats.last_tun_drop_reason =
                                Some(format!("{}_send_failed", path_kind.as_str()));
                            if path_manager.record_send_failure(&peer_node_id, path_kind) {
                                if let Some(fallback_path) =
                                    path_manager.fallback_path_for_node(&peer_node_id, path_kind)
                                {
                                    path_manager
                                        .tracker
                                        .set_active_path(peer_node_id.clone(), fallback_path);
                                    update_peer_active_path(
                                        &mut selected_peer_paths,
                                        &peer_node_id,
                                        fallback_path,
                                    );
                                    stats.active_path = path_manager.active_path_summary();
                                    if let Some(peer_stats) =
                                        peer_stats_mut_by_node_id(&mut stats.peers, &peer_node_id)
                                    {
                                        peer_stats.path_downgrades =
                                            peer_stats.path_downgrades.saturating_add(1);
                                        peer_stats.last_path_change = Some(format!(
                                            "{} -> {} after consecutive send failures",
                                            path_kind.as_str(),
                                            fallback_path.as_str()
                                        ));
                                    }
                                    persist_active_path_state(
                                        fallback_path,
                                        selected_peer_paths.clone(),
                                    );
                                }
                            }
                            stats.relay_send_failures = stats.relay_send_failures.saturating_add(1);
                            if let Some(peer_stats) =
                                peer_stats_mut_by_node_id(&mut stats.peers, &peer_node_id)
                            {
                                peer_stats.send_failures =
                                    peer_stats.send_failures.saturating_add(1);
                                peer_stats.last_send_path = Some(path_kind.as_str().to_string());
                                peer_stats.last_relay_error =
                                    Some(format!("{} send failed", path_kind.as_str()));
                            }
                            stats.last_relay_error = Some(format!(
                                "peer {} {} send failed",
                                peer_node_id,
                                path_kind.as_str()
                            ));
                            consecutive_data_plane_failures =
                                consecutive_data_plane_failures.saturating_add(1);
                        }
                        PathSendResult::NoTransport {
                            peer_node_id,
                            path_kind,
                            ..
                        } => {
                            stats.last_tun_peer_node_id = Some(peer_node_id.clone());
                            stats.last_tun_send_path = Some(path_kind.as_str().to_string());
                            stats.last_tun_drop_reason =
                                Some(format!("{}_transport_unavailable", path_kind.as_str()));
                            if path_manager.record_send_failure(&peer_node_id, path_kind) {
                                if let Some(fallback_path) =
                                    path_manager.fallback_path_for_node(&peer_node_id, path_kind)
                                {
                                    path_manager
                                        .tracker
                                        .set_active_path(peer_node_id.clone(), fallback_path);
                                    update_peer_active_path(
                                        &mut selected_peer_paths,
                                        &peer_node_id,
                                        fallback_path,
                                    );
                                    stats.active_path = path_manager.active_path_summary();
                                    if let Some(peer_stats) =
                                        peer_stats_mut_by_node_id(&mut stats.peers, &peer_node_id)
                                    {
                                        peer_stats.path_downgrades =
                                            peer_stats.path_downgrades.saturating_add(1);
                                        peer_stats.last_path_change = Some(format!(
                                            "{} -> {} after missing transport",
                                            path_kind.as_str(),
                                            fallback_path.as_str()
                                        ));
                                    }
                                    persist_active_path_state(
                                        fallback_path,
                                        selected_peer_paths.clone(),
                                    );
                                }
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
                } else {
                    stats.last_tun_drop_reason = Some("relay_frame_encode_failed".to_string());
                }
                unsafe {
                    release_receive_packet(session, packet);
                }
            }

            for peer in path_manager.relay_udp_peers_mut() {
                match peer.socket.recv(&mut relay_buffer) {
                    Ok(frame_len) => {
                        if let Some(error) = relay_error_message(&relay_buffer[..frame_len]) {
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
                            stats.last_relay_packet_at_ms = Some(current_timestamp_ms());
                            record_relay_tcp_packet(&mut stats, packet);
                            let acl_peer = PlatformAclPeer {
                                peer_node_id: Some(peer.peer_node_id.clone()),
                                peer_virtual_ips: peer.peer_virtual_ips.clone(),
                            };
                            if !acl_allows_ingress_packet(packet, &acl_policies, Some(&acl_peer)) {
                                stats.last_tun_drop_reason = Some("acl_ingress_denied".to_string());
                                continue;
                            }
                            if let Some(reply) =
                                icmp_echo_reply_for_request(packet, &local_virtual_ip)
                            {
                                seq = seq.wrapping_add(1);
                                if let Some(frame) =
                                    encode_slan_relay_data_frame(seq, config_hash, &reply)
                                {
                                    if send_relay_udp_frame(peer, &frame) {
                                        stats.tun_packets_sent =
                                            stats.tun_packets_sent.saturating_add(1);
                                        if let Some(peer_stats) =
                                            stats.peers.get_mut(peer.stats_index)
                                        {
                                            peer_stats.tun_packets_sent =
                                                peer_stats.tun_packets_sent.saturating_add(1);
                                            peer_stats.last_send_path =
                                                Some(PathKind::RelayUdp.as_str().to_string());
                                        }
                                    }
                                }
                                continue;
                            }
                            let packet = normalize_ipv4_transport_checksums(packet);
                            if !acl_allows_ingress_packet(&packet, &acl_policies, Some(&acl_peer)) {
                                stats.last_tun_drop_reason = Some("acl_ingress_denied".to_string());
                                continue;
                            }
                            if write_wintun_packet(
                                session,
                                allocate_send_packet,
                                send_packet,
                                &packet,
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
                        } else {
                            stats.relay_decode_failures =
                                stats.relay_decode_failures.saturating_add(1);
                        }
                    }
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {}
                    Err(error) if error.kind() == std::io::ErrorKind::Interrupted => {}
                    Err(error) => {
                        stats.relay_receive_failures =
                            stats.relay_receive_failures.saturating_add(1);
                        if let Some(peer_stats) = stats.peers.get_mut(peer.stats_index) {
                            peer_stats.receive_failures =
                                peer_stats.receive_failures.saturating_add(1);
                            peer_stats.last_relay_error =
                                Some(format!("relay_udp receive failed: {error}"));
                        }
                        stats.last_relay_error = Some(format!(
                            "peer {} relay_udp receive failed: {error}",
                            peer.peer_node_id
                        ));
                        consecutive_data_plane_failures =
                            consecutive_data_plane_failures.saturating_add(1);
                    }
                }
            }
            for peer in path_manager.derp_peers_mut() {
                match recv_derp_packet(peer) {
                    Ok(Some(frame)) => {
                        if let Some(packet) = decode_slan_relay_data_frame(&frame) {
                            stats.last_relay_packet_at_ms = Some(current_timestamp_ms());
                            record_relay_tcp_packet(&mut stats, packet);
                            let acl_peer = PlatformAclPeer {
                                peer_node_id: Some(peer.peer_node_id.clone()),
                                peer_virtual_ips: peer.peer_virtual_ips.clone(),
                            };
                            if !acl_allows_ingress_packet(packet, &acl_policies, Some(&acl_peer)) {
                                stats.last_tun_drop_reason = Some("acl_ingress_denied".to_string());
                                continue;
                            }
                            if let Some(reply) =
                                icmp_echo_reply_for_request(packet, &local_virtual_ip)
                            {
                                seq = seq.wrapping_add(1);
                                if let Some(frame) =
                                    encode_slan_relay_data_frame(seq, config_hash, &reply)
                                {
                                    if send_derp_forward(peer, &frame).is_ok() {
                                        stats.tun_packets_sent =
                                            stats.tun_packets_sent.saturating_add(1);
                                        if let Some(peer_stats) =
                                            stats.peers.get_mut(peer.stats_index)
                                        {
                                            peer_stats.tun_packets_sent =
                                                peer_stats.tun_packets_sent.saturating_add(1);
                                            peer_stats.last_send_path =
                                                Some(PathKind::DerpTcpTls443.as_str().to_string());
                                        }
                                    }
                                }
                                continue;
                            }
                            let packet = normalize_ipv4_transport_checksums(packet);
                            if !acl_allows_ingress_packet(&packet, &acl_policies, Some(&acl_peer)) {
                                stats.last_tun_drop_reason = Some("acl_ingress_denied".to_string());
                                continue;
                            }
                            if write_wintun_packet(
                                session,
                                allocate_send_packet,
                                send_packet,
                                &packet,
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
                        } else {
                            stats.relay_decode_failures =
                                stats.relay_decode_failures.saturating_add(1);
                        }
                    }
                    Ok(None) => {}
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {}
                    Err(error) if error.kind() == std::io::ErrorKind::Interrupted => {}
                    Err(error) => {
                        stats.relay_receive_failures =
                            stats.relay_receive_failures.saturating_add(1);
                        if let Some(peer_stats) = stats.peers.get_mut(peer.stats_index) {
                            peer_stats.receive_failures =
                                peer_stats.receive_failures.saturating_add(1);
                            peer_stats.last_relay_error =
                                Some(format!("derp_tcp receive failed: {error}"));
                        }
                        stats.last_relay_error = Some(format!(
                            "peer {} derp_tcp receive failed: {error}",
                            peer.peer_node_id
                        ));
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
                            direct_udp.mark_peer_inbound(peer_index);
                            stats.direct_udp_probes_received =
                                stats.direct_udp_probes_received.saturating_add(1);
                            if received.endpoint_changed {
                                direct_udp.update_peer_endpoint(peer_index, received.remote_addr);
                            }
                            if direct_udp.send_pong_to_peer(peer_index) {
                                stats.direct_udp_pongs_sent =
                                    stats.direct_udp_pongs_sent.saturating_add(1);
                            }
                            stats.direct_udp_ready_peer_count = direct_udp.peer_count() as u64;
                            direct_udp_probe_success_peer = Some((
                                direct_udp.peers[peer_index].peer_node_id.clone(),
                                direct_udp.peers[peer_index].path_kind,
                            ));
                        } else if control_packet
                            .as_ref()
                            .is_some_and(|packet| packet.kind == DirectUdpControlKind::Pong)
                        {
                            direct_udp.mark_peer_inbound(peer_index);
                            stats.direct_udp_pongs_received =
                                stats.direct_udp_pongs_received.saturating_add(1);
                            if received.endpoint_changed {
                                direct_udp.update_peer_endpoint(peer_index, received.remote_addr);
                            }
                            stats.direct_udp_ready_peer_count = direct_udp.peer_count() as u64;
                            direct_udp_probe_success_peer = Some((
                                direct_udp.peers[peer_index].peer_node_id.clone(),
                                direct_udp.peers[peer_index].path_kind,
                            ));
                        } else if let Some(packet) =
                            decode_slan_relay_data_frame(&relay_buffer[..frame_len])
                        {
                            direct_udp.mark_peer_inbound(peer_index);
                            let peer_node_id = direct_udp.peers[peer_index].peer_node_id.clone();
                            if received.endpoint_changed {
                                direct_udp.update_peer_endpoint(peer_index, received.remote_addr);
                            }
                            direct_udp_probe_success_peer = Some((
                                peer_node_id.clone(),
                                direct_udp.peers[peer_index].path_kind,
                            ));
                            stats.direct_udp_ready_peer_count = direct_udp.peer_count() as u64;
                            stats.direct_udp_frames_received =
                                stats.direct_udp_frames_received.saturating_add(1);
                            record_relay_tcp_packet(&mut stats, packet);
                            let peer = &direct_udp.peers[peer_index];
                            let acl_peer = PlatformAclPeer {
                                peer_node_id: Some(peer.peer_node_id.clone()),
                                peer_virtual_ips: peer.peer_virtual_ips.clone(),
                            };
                            if !acl_allows_ingress_packet(packet, &acl_policies, Some(&acl_peer)) {
                                stats.last_tun_drop_reason = Some("acl_ingress_denied".to_string());
                                continue;
                            }
                            if let Some(reply) =
                                icmp_echo_reply_for_request(packet, &local_virtual_ip)
                            {
                                seq = seq.wrapping_add(1);
                                if let Some(frame) =
                                    encode_slan_relay_data_frame(seq, config_hash, &reply)
                                {
                                    let _ = direct_udp.send_to_peer(peer_index, &frame);
                                    stats.tun_packets_sent =
                                        stats.tun_packets_sent.saturating_add(1);
                                    if let Some(peer_stats) =
                                        peer_stats_mut_by_node_id(&mut stats.peers, &peer_node_id)
                                    {
                                        peer_stats.tun_packets_sent =
                                            peer_stats.tun_packets_sent.saturating_add(1);
                                        peer_stats.last_send_path =
                                            Some(peer.path_kind.as_str().to_string());
                                    }
                                }
                                continue;
                            }
                            let packet = normalize_ipv4_transport_checksums(packet);
                            if !acl_allows_ingress_packet(&packet, &acl_policies, Some(&acl_peer)) {
                                stats.last_tun_drop_reason = Some("acl_ingress_denied".to_string());
                                continue;
                            }
                            if write_wintun_packet(
                                session,
                                allocate_send_packet,
                                send_packet,
                                &packet,
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
                    Err(error) => {
                        stats.relay_receive_failures =
                            stats.relay_receive_failures.saturating_add(1);
                        stats.last_relay_error =
                            Some(format!("direct_udp receive failed: {error}"));
                        consecutive_data_plane_failures =
                            consecutive_data_plane_failures.saturating_add(1);
                    }
                }
            }
            if let Some((peer_node_id, path_kind)) = direct_udp_probe_success_peer {
                let previous_path = path_manager.active_path_for_peer_node(&peer_node_id);
                if path_manager.record_probe_success(&peer_node_id, path_kind) {
                    mark_peer_path_probe_success(
                        &mut selected_peer_paths,
                        &peer_node_id,
                        path_kind,
                        current_timestamp_ms(),
                    );
                    stats.active_path = path_manager.active_path_summary();
                    if let Some(peer_stats) =
                        peer_stats_mut_by_node_id(&mut stats.peers, &peer_node_id)
                    {
                        peer_stats.path_upgrades = peer_stats.path_upgrades.saturating_add(1);
                        peer_stats.last_path_change = Some(format!(
                            "{} -> {} after probe success",
                            previous_path.as_str(),
                            path_kind.as_str()
                        ));
                    }
                    persist_active_path_state(path_kind, selected_peer_paths.clone());
                }
            }
            if consecutive_data_plane_failures >= RELAY_DATA_PLANE_FAILURE_THRESHOLD {
                persist_relay_stats(&mut stats);
                break;
            }
            if last_stats_flush.elapsed() >= RELAY_STATS_FLUSH_INTERVAL {
                persist_relay_stats_async(&mut stats);
                last_stats_flush = Instant::now();
            }
            thread::sleep(DATA_PLANE_IDLE_SLEEP);
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
        mark_relay_session_path_ready(path, session);
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
    mark_relay_session_path_ready(&mut path, session);
    path
}

fn mark_relay_session_path_ready(path: &mut PeerPathRuntime, session: &RelayPeerSession) {
    let Some(path_kind) = relay_path_kind_from_ticket(session) else {
        return;
    };
    if path.peer_virtual_ips.is_empty() {
        path.peer_virtual_ips = session.peer_virtual_ips.clone();
    }
    let Some(candidate) = path
        .candidates
        .iter_mut()
        .find(|candidate| candidate.kind == path_kind)
    else {
        path.candidates
            .push(relay_ready_candidate(path_kind, session));
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
    candidate.transport = relay_url_scheme_for_path(path_kind).map(str::to_string);
}

fn relay_ready_candidate(
    path_kind: PathKind,
    session: &RelayPeerSession,
) -> client_core::PathCandidate {
    client_core::PathCandidate {
        kind: path_kind,
        state: PathState::Ready,
        endpoint_id: None,
        address: Some(session.ticket.relay_url.clone()),
        session_id: Some(session.session_id.clone()),
        transport: relay_url_scheme_for_path(path_kind).map(str::to_string),
        rtt_ms: None,
        path_score: None,
        last_ok_at_ms: None,
        last_error: None,
    }
}

fn relay_path_kind_from_ticket(session: &RelayPeerSession) -> Option<PathKind> {
    let relay_url = session.ticket.relay_url.trim();
    let (scheme, _) = relay_url.split_once("://")?;
    match scheme.to_ascii_lowercase().as_str() {
        "udp" | "relay+udp" => Some(PathKind::RelayUdp),
        "derp" | "derp+tcp+tls" | "derp_tcp_tls_443" => Some(PathKind::DerpTcpTls443),
        _ => None,
    }
}

fn mark_ready_transports(
    mut peer_paths: Vec<PeerPathRuntime>,
    direct_udp: Option<&DirectUdpTransport>,
) -> Vec<PeerPathRuntime> {
    // 与 macOS/Linux 保持一致：直连 UDP 路径初始状态为 Probing，
    // 只有在收到探测回复（probe/pong）后才升级为 Ready。
    // 这样 select_active_path 会优先选择已 Ready 的 relay 路径，
    // 避免在直连地址不可达时数据包全部走 lan_udp 导致不通。
    reset_direct_candidates_to_probing(&mut peer_paths, direct_udp.map(|t| &t.peers));
    peer_paths
}

/// 将直连 UDP 候选路径状态重置为 Probing。
/// 当 `direct_peers` 为 None 时，重置所有直连候选；否则只重置有对应 peer 的候选。
fn reset_direct_candidates_to_probing(
    peer_paths: &mut [PeerPathRuntime],
    direct_peers: Option<&Vec<DirectUdpPeer>>,
) {
    for path in peer_paths.iter_mut() {
        for candidate in path.candidates.iter_mut() {
            if candidate.kind.is_direct_udp() {
                let should_reset = match direct_peers {
                    Some(peers) => peers.iter().any(|peer| {
                        peer.peer_node_id == path.peer_node_id && peer.path_kind == candidate.kind
                    }),
                    None => true,
                };
                if should_reset {
                    candidate.state = PathState::Probing;
                }
            }
        }
    }
}

/// 检查某个 peer 的指定路径候选是否处于 Ready 状态。
fn path_candidate_is_ready(
    peer_paths: &[PeerPathRuntime],
    peer_node_id: &str,
    path_kind: PathKind,
) -> bool {
    peer_paths
        .iter()
        .find(|path| path.peer_node_id == peer_node_id)
        .is_some_and(|path| {
            path.candidates
                .iter()
                .any(|c| c.kind == path_kind && c.state == PathState::Ready)
        })
}

fn attach_udp_relay_session(
    relay_address: &str,
    local_node_id: &str,
    session: &RelayPeerSession,
) -> Result<UdpSocket> {
    validate_relay_peer_session_for_path(local_node_id, session, PathKind::RelayUdp)?;
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
        "transport": PathKind::RelayUdp.as_str(),
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
            Ok(len) => match verify_relay_attach_ack(&response[..len], &session.session_id) {
                Ok(_) => {
                    last_error = None;
                    break;
                }
                Err(verify_error) => {
                    last_error = Some(verify_error.context(format!(
                            "verify relay attach ack from {relay_address} attempt {attempt}/{RELAY_ATTACH_ATTEMPTS}"
                        )));
                }
            },
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

fn attach_derp_relay_session(
    local_node_id: &str,
    session: &RelayPeerSession,
    stats_index: usize,
) -> Result<DerpPeer> {
    validate_relay_peer_session_for_path(local_node_id, session, PathKind::DerpTcpTls443)?;
    let address = normalize_derp_tcp_address(session.ticket.relay_url.as_str())
        .ok_or_else(|| anyhow::anyhow!("missing Wintun DERP TCP address"))?;
    let stream = TcpStream::connect(address.as_str())
        .with_context(|| format!("connect Wintun DERP TCP socket {address}"))?;
    stream.set_nodelay(true)?;
    stream.set_read_timeout(Some(Duration::from_secs(3)))?;
    stream.set_write_timeout(Some(Duration::from_secs(3)))?;
    let mut writer = stream
        .try_clone()
        .context("clone Wintun DERP writer stream")?;
    let mut reader = BufReader::new(
        stream
            .try_clone()
            .context("clone Wintun DERP reader stream")?,
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
        serde_json::from_str(line.trim()).context("decode Wintun DERP connect response")?;
    if value.get("kind").and_then(serde_json::Value::as_str) != Some("connected") {
        bail!(
            "{}",
            value
                .get("error")
                .and_then(|error| error.get("message"))
                .and_then(serde_json::Value::as_str)
                .unwrap_or("Wintun DERP connect failed")
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
        peer_node_id: session.peer_node_id.clone(),
        local_node_id: local_node_id.to_string(),
        server_session_id,
        peer_virtual_ips: session.peer_virtual_ips.clone(),
        stream: writer,
        reader,
        read_buffer: Vec::new(),
        stats_index,
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
                    "Wintun DERP TCP connection closed before connect ack",
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
                    "Wintun DERP TCP write returned zero bytes",
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

fn udp_probe_targets_for_peer(path: &client_core::PeerPathConfig) -> Vec<DirectUdpProbeTarget> {
    let mut targets = path
        .candidates
        .iter()
        .filter(|candidate| candidate.kind.is_direct_udp())
        .filter_map(|candidate| {
            let address = candidate
                .address
                .as_deref()
                .map(str::trim)
                .filter(|address| !address.is_empty())?;
            match resolve_direct_udp_peer_address(address) {
                Ok(socket_addr) => Some(DirectUdpProbeTarget {
                    path_kind: candidate.kind,
                    address: address.to_string(),
                    socket_addr,
                }),
                Err(error) => {
                    eprintln!(
                        "client-core-platform direct udp candidate skipped peer={} address={} error={error:#}",
                        path.peer_node_id, address
                    );
                    None
                }
            }
        })
        .collect::<Vec<_>>();
    targets.sort_by_key(|target| target.path_kind.priority());
    targets.dedup_by_key(|target| target.socket_addr);
    targets
}

fn attach_direct_udp_socket() -> Result<UdpSocket> {
    let preferred_port = configured_direct_udp_port();
    let bind_address = format!("0.0.0.0:{preferred_port}");
    let socket = UdpSocket::bind(&bind_address).or_else(|error| {
        if preferred_port == 0 {
            return Err(error);
        }
        eprintln!(
            "windows direct UDP preferred port {preferred_port} unavailable; using random port: {error}"
        );
        UdpSocket::bind("0.0.0.0:0")
    })
    .with_context(|| format!("bind direct UDP socket to {bind_address}"))?;
    socket
        .set_nonblocking(true)
        .context("set direct UDP socket nonblocking")?;
    Ok(socket)
}

fn resolve_direct_udp_peer_address(address: &str) -> Result<SocketAddr> {
    let address = normalize_direct_udp_address(address)
        .ok_or_else(|| anyhow::anyhow!("missing direct UDP peer address"))?;
    address
        .to_socket_addrs()
        .with_context(|| format!("resolve direct UDP peer address {address}"))?
        .next()
        .ok_or_else(|| anyhow::anyhow!("direct UDP peer address has no socket address: {address}"))
}

fn normalize_direct_udp_address(address: &str) -> Option<String> {
    let trimmed = address.trim();
    if trimmed.is_empty() {
        return None;
    }
    if trimmed.starts_with("relay+udp://") {
        return None;
    }
    let normalized = trimmed
        .strip_prefix("udp://")
        .or_else(|| trimmed.strip_prefix("direct+udp://"))
        .or_else(|| (!trimmed.contains("://")).then_some(trimmed))?
        .trim();
    (!normalized.is_empty()).then(|| normalized.to_string())
}

fn relay_udp_address_for_session(
    default_address: &str,
    session: &RelayPeerSession,
) -> Option<String> {
    normalize_relay_udp_address(session.ticket.relay_url.as_str()).or_else(|| {
        normalize_relay_udp_address(default_address).or_else(|| {
            let trimmed = default_address.trim();
            (!trimmed.is_empty() && !trimmed.contains("://")).then(|| trimmed.to_string())
        })
    })
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

fn validate_relay_peer_session(local_node_id: &str, session: &RelayPeerSession) -> Result<()> {
    let local_node_id = local_node_id.trim();
    let ticket = &session.ticket;
    if local_node_id.is_empty()
        || session.session_id.trim().is_empty()
        || session.peer_node_id.trim().is_empty()
        || ticket.ticket_id.trim().is_empty()
        || ticket.network_id.trim().is_empty()
        || ticket.session_id.trim().is_empty()
        || ticket.src_node_id.trim().is_empty()
        || ticket.dst_node_id.trim().is_empty()
        || ticket.relay_url.trim().is_empty()
        || ticket.expires_at.trim().is_empty()
        || ticket.session_key.trim().is_empty()
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

fn validate_relay_peer_session_for_path(
    local_node_id: &str,
    session: &RelayPeerSession,
    path_kind: PathKind,
) -> Result<()> {
    validate_relay_peer_session(local_node_id, session)?;
    let Some(expected_scheme) = relay_url_scheme_for_path(path_kind) else {
        return Ok(());
    };
    let relay_url = session.ticket.relay_url.trim();
    let Some((scheme, _)) = relay_url.split_once("://") else {
        return Ok(());
    };
    if path_kind == PathKind::DerpTcpTls443 && derp_relay_scheme_matches(scheme) {
        return Ok(());
    }
    if scheme.eq_ignore_ascii_case(expected_scheme) {
        return Ok(());
    }
    bail!(
        "relay ticket relayUrl scheme mismatch for {}: expected {}, got {}",
        path_kind.as_str(),
        expected_scheme,
        scheme
    )
}

fn relay_url_scheme_for_path(path_kind: PathKind) -> Option<&'static str> {
    match path_kind {
        PathKind::RelayUdp => Some("udp"),
        PathKind::DerpTcpTls443 => Some("derp"),
        PathKind::LanUdp | PathKind::Ipv6Udp | PathKind::DirectUdp => None,
    }
}

fn derp_relay_scheme_matches(scheme: &str) -> bool {
    matches!(
        scheme.to_ascii_lowercase().as_str(),
        "derp" | "derp+tcp+tls" | "derp_tcp_tls_443"
    )
}

fn relay_ticket_wire(session: &RelayPeerSession) -> serde_json::Value {
    let ticket = &session.ticket;
    // Use camelCase to match Go server's json tags directly.
    // This is compatible with both old servers (no UnmarshalJSON)
    // and new servers (with UnmarshalJSON that checks camelCase first).
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
    if peer.path_kind != PathKind::RelayUdp {
        return false;
    }
    encode_relay_forward(peer, frame).is_some_and(|payload| peer.socket.send(&payload).is_ok())
}

fn encode_relay_forward(peer: &AttachedRelayPeer, frame: &[u8]) -> Option<Vec<u8>> {
    serde_json::to_vec(&serde_json::json!({
        "kind": "forward",
        "session_id": peer.session_id,
        "participant_id": peer.local_node_id,
        "payload": base64_encode(frame),
    }))
    .ok()
}

fn send_relay_keepalives(peers: &[AttachedRelayPeer]) {
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
        "pong" | "attached" | "forwarded" | "detached" => value
            .get("kind")
            .and_then(serde_json::Value::as_str)
            .map(str::to_string),
        _ => None,
    }
}

fn send_derp_forward(peer: &DerpPeer, frame: &[u8]) -> std::io::Result<()> {
    let payload = serde_json::json!({
        "kind": "send",
        "sessionId": peer.server_session_id,
        "targetPeerId": peer.peer_node_id,
        "payload": base64_encode(frame),
    });
    let mut line = serde_json::to_vec(&payload).map_err(json_io_error)?;
    line.push(b'\n');
    let mut stream = peer.stream.try_clone()?;
    write_all_with_would_block_retry(&mut stream, &line, DERP_WRITE_RETRY_TIMEOUT)
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
                .unwrap_or("Wintun DERP error")
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
        if let Ok(mut stream) = peer.stream.try_clone() {
            let _ = write_json_line(&mut stream, &payload);
        }
    }
}

fn json_io_error(error: serde_json::Error) -> std::io::Error {
    std::io::Error::new(ErrorKind::InvalidData, error)
}

fn relay_send_attempt_count(packet: &[u8]) -> usize {
    if ipv4_protocol(packet) == Some(17) {
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

fn should_hedge_direct_packet_to_relay(packet: &[u8]) -> bool {
    if ipv4_protocol(packet) == Some(17) {
        return true;
    }
    let Some(flags) = ipv4_tcp_flags(packet) else {
        return false;
    };
    flags & 0x12 == 0x02 || flags & 0x0b != 0 || ipv4_tcp_payload_len(packet).unwrap_or(0) > 0
}

fn is_direct_udp_path(path_kind: PathKind) -> bool {
    path_kind.is_direct_udp()
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

fn local_dns_reply(
    packet: &[u8],
    dns_servers: &[String],
    dns_records: &[PlatformResolverRecord],
) -> Option<Vec<u8>> {
    dns_servers
        .iter()
        .find_map(|server| resolver_response_for_query(packet, server, dns_records))
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

fn record_tun_tcp_packet(stats: &mut WindowsRelayDataPlaneStats, packet: &[u8]) {
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

fn record_relay_tcp_packet(stats: &mut WindowsRelayDataPlaneStats, packet: &[u8]) {
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

fn ipv4_protocol(packet: &[u8]) -> Option<u8> {
    if packet.len() < 20 || packet[0] >> 4 != 4 {
        return None;
    }
    packet.get(9).copied()
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

fn peer_stats_mut_by_node_id<'a>(
    peers: &'a mut [WindowsRelayPeerStats],
    peer_node_id: &str,
) -> Option<&'a mut WindowsRelayPeerStats> {
    peers
        .iter_mut()
        .find(|peer| peer.peer_node_id == peer_node_id)
}

fn mark_peer_stats_attached_from_transports(
    peers: &mut [WindowsRelayPeerStats],
    direct_udp: Option<&DirectUdpTransport>,
) {
    if let Some(direct_udp) = direct_udp {
        for peer in &direct_udp.peers {
            if let Some(stats) = peer_stats_mut_by_node_id(peers, &peer.peer_node_id) {
                stats.attached = true;
                stats.attach_error = None;
            }
        }
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum DirectUdpControlKind {
    Probe,
    Pong,
}

#[derive(Debug, Clone, PartialEq, Eq)]
struct DirectUdpControlPacket {
    kind: DirectUdpControlKind,
    peer_node_id: String,
}

impl DirectUdpControlPacket {
    fn peer_node_id(&self) -> &str {
        &self.peer_node_id
    }
}

fn direct_udp_control_packet(payload: &[u8]) -> Option<DirectUdpControlPacket> {
    let value = serde_json::from_slice::<serde_json::Value>(payload).ok()?;
    if value.get("kind").and_then(serde_json::Value::as_str) != Some("direct_udp") {
        return None;
    }
    let kind = match value.get("type").and_then(serde_json::Value::as_str)? {
        "probe" => DirectUdpControlKind::Probe,
        "pong" => DirectUdpControlKind::Pong,
        _ => return None,
    };
    let peer_node_id = value
        .get("nodeId")
        .and_then(serde_json::Value::as_str)
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .map(str::to_string)?;
    Some(DirectUdpControlPacket { kind, peer_node_id })
}

fn direct_udp_control_payload(kind: DirectUdpControlKind, local_node_id: &str) -> String {
    let kind = match kind {
        DirectUdpControlKind::Probe => "probe",
        DirectUdpControlKind::Pong => "pong",
    };
    serde_json::json!({
        "kind": "direct_udp",
        "type": kind,
        "nodeId": local_node_id.trim(),
    })
    .to_string()
}

fn configure_wintun_local_data_plane() -> Result<()> {
    let local_virtual_ip = load_cached_runtime_state()
        .ok()
        .and_then(|state| state.virtual_ip)
        .unwrap_or_default();
    if local_virtual_ip.trim().is_empty() {
        stop_wintun_data_plane();
        return Ok(());
    }
    let runtime = WINTUN_RUNTIME.get_or_init(|| Mutex::new(None));
    let mut runtime = runtime.lock().expect("wintun runtime mutex poisoned");
    let Some(runtime) = runtime.as_mut() else {
        bail!("Wintun runtime is not ready");
    };
    let _ = runtime.data_plane.take();
    let resolver = windows_resolver_runtime()
        .lock()
        .expect("windows resolver runtime mutex poisoned")
        .clone();
    let dns_servers = resolver.servers;
    let dns_records = resolver.records;
    let stop = Arc::new(AtomicBool::new(false));
    let thread_stop = Arc::clone(&stop);
    let session = runtime.session as usize;
    let receive_packet = runtime.receive_packet;
    let release_receive_packet = runtime.release_receive_packet;
    let allocate_send_packet = runtime.allocate_send_packet;
    let send_packet = runtime.send_packet;
    let handle = thread::spawn(move || {
        let session = session as WintunSessionHandle;
        while !thread_stop.load(Ordering::SeqCst) {
            let mut packet_size = 0_u32;
            let packet = unsafe { receive_packet(session, &mut packet_size as *mut u32) };
            if packet.is_null() || packet_size == 0 {
                thread::sleep(DATA_PLANE_IDLE_SLEEP);
                continue;
            }
            let payload = unsafe { std::slice::from_raw_parts(packet, packet_size as usize) };
            if let Some(reply) = local_dns_reply(payload, &dns_servers, &dns_records) {
                let _ = write_wintun_packet(session, allocate_send_packet, send_packet, &reply);
            } else if let Some(reply) = local_virtual_ip_reply(payload, local_virtual_ip.as_str()) {
                let _ = write_wintun_packet(session, allocate_send_packet, send_packet, &reply);
            } else if packet_targets_local_virtual_ip(payload, local_virtual_ip.as_str()) {
                let packet = normalize_ipv4_transport_checksums(payload);
                let _ = write_wintun_packet(session, allocate_send_packet, send_packet, &packet);
            }
            unsafe {
                release_receive_packet(session, packet);
            }
        }
    });
    runtime.data_plane = Some(WindowsDataPlaneRuntime {
        stop,
        handle: Some(handle),
    });
    Ok(())
}

fn write_wintun_packet(
    session: WintunSessionHandle,
    allocate_send_packet: WintunAllocateSendPacketFunc,
    send_packet: WintunSendPacketFunc,
    payload: &[u8],
) -> bool {
    let Ok(size) = u32::try_from(payload.len()) else {
        return false;
    };
    let deadline = Instant::now() + Duration::from_secs(1);
    loop {
        let send_packet_ptr = unsafe { allocate_send_packet(session, size) };
        if !send_packet_ptr.is_null() {
            unsafe {
                std::ptr::copy_nonoverlapping(payload.as_ptr(), send_packet_ptr, payload.len());
                send_packet(session, send_packet_ptr);
            }
            return true;
        }
        if Instant::now() >= deadline {
            return false;
        }
        thread::sleep(Duration::from_millis(1));
    }
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

#[derive(Debug, Clone, Deserialize, Serialize)]
#[serde(rename_all = "camelCase")]
struct DirectUdpEndpointReport {
    endpoint: String,
    endpoint_type: String,
    #[serde(default)]
    lan_endpoint: String,
    nat_type: String,
    bind_address: String,
    updated_at_ms: u64,
}

static WINDOWS_DIRECT_UDP_ENDPOINT_REPORT: OnceLock<Mutex<Option<DirectUdpEndpointReport>>> =
    OnceLock::new();

fn direct_udp_endpoint_file_path() -> PathBuf {
    app_data_dir()
        .join("SLAN")
        .join("client-v2-direct-udp-endpoint.json")
}

fn persist_direct_udp_endpoint_report(socket: &UdpSocket) {
    let Ok(local_addr) = socket.local_addr() else {
        return;
    };
    let report_host = direct_udp_lan_host().unwrap_or_else(|| local_addr.ip().to_string());
    let report = DirectUdpEndpointReport {
        endpoint: format!("{report_host}:{}", local_addr.port()),
        endpoint_type: "lan_udp".to_string(),
        lan_endpoint: format!("{report_host}:{}", local_addr.port()),
        nat_type: "unknown".to_string(),
        bind_address: local_addr.to_string(),
        updated_at_ms: current_timestamp_ms(),
    };
    persist_direct_udp_endpoint_report_value(report);
}

fn persist_direct_udp_reflexive_endpoint(socket: &UdpSocket, endpoint: &str) {
    let Ok(local_addr) = socket.local_addr() else {
        return;
    };
    let lan_endpoint = WINDOWS_DIRECT_UDP_ENDPOINT_REPORT
        .get_or_init(|| Mutex::new(None))
        .lock()
        .ok()
        .and_then(|report| report.clone())
        .map(|report| {
            if report.lan_endpoint.trim().is_empty() && report.endpoint_type == "lan_udp" {
                report.endpoint
            } else {
                report.lan_endpoint
            }
        })
        .unwrap_or_default();
    let report = DirectUdpEndpointReport {
        endpoint: endpoint.trim().to_string(),
        endpoint_type: "direct_udp".to_string(),
        lan_endpoint,
        nat_type: "unknown".to_string(),
        bind_address: local_addr.to_string(),
        updated_at_ms: current_timestamp_ms(),
    };
    persist_direct_udp_endpoint_report_value(report);
}

fn persist_direct_udp_endpoint_report_value(report: DirectUdpEndpointReport) {
    let mut changed = true;
    if let Ok(mut current) = WINDOWS_DIRECT_UDP_ENDPOINT_REPORT
        .get_or_init(|| Mutex::new(None))
        .lock()
    {
        changed = current
            .as_ref()
            .is_none_or(|value| !same_direct_udp_endpoint(value, &report));
        *current = Some(report.clone());
    }
    if !changed {
        return;
    }
    let path = direct_udp_endpoint_file_path();
    let Some(parent) = path.parent() else {
        return;
    };
    if fs::create_dir_all(parent).is_err() {
        return;
    }
    if let Ok(payload) = serde_json::to_vec_pretty(&report) {
        let _ = fs::write(path, payload);
    }
}

fn same_direct_udp_endpoint(
    left: &DirectUdpEndpointReport,
    right: &DirectUdpEndpointReport,
) -> bool {
    left.endpoint == right.endpoint
        && left.endpoint_type == right.endpoint_type
        && left.lan_endpoint == right.lan_endpoint
        && left.nat_type == right.nat_type
        && left.bind_address == right.bind_address
}

fn clear_direct_udp_endpoint_report() {
    if let Ok(mut current) = WINDOWS_DIRECT_UDP_ENDPOINT_REPORT
        .get_or_init(|| Mutex::new(None))
        .lock()
    {
        *current = None;
    }
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
    let now_ms = current_timestamp_ms();
    stats.updated_at_ms = now_ms;
    refresh_relay_ticket_timing(stats, now_ms);
    let Ok(payload) = serde_json::to_vec_pretty(stats) else {
        return;
    };
    write_relay_stats(payload);
}

fn persist_relay_stats_async(stats: &mut WindowsRelayDataPlaneStats) {
    let now_ms = current_timestamp_ms();
    stats.updated_at_ms = now_ms;
    refresh_relay_ticket_timing(stats, now_ms);
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

fn refresh_relay_ticket_timing(stats: &mut WindowsRelayDataPlaneStats, now_ms: u64) {
    let Some(expires_at_ms) = stats
        .ticket_expires_at
        .as_deref()
        .and_then(parse_rfc3339_utc_ms)
    else {
        stats.ticket_expires_in_ms = None;
        stats.ticket_renew_due = false;
        return;
    };
    let expires_in_ms = expires_at_ms as i128 - now_ms as i128;
    stats.ticket_expires_in_ms =
        Some(expires_in_ms.clamp(i64::MIN as i128, i64::MAX as i128) as i64);
    stats.ticket_renew_due = now_ms.saturating_add(RELAY_TICKET_RENEW_WINDOW_MS) >= expires_at_ms;
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

fn parse_rfc3339_utc_ms(value: &str) -> Option<u64> {
    let value = value.trim();
    let (date, time) = value.split_once('T')?;
    let time = time.strip_suffix('Z')?;
    let mut date_parts = date.split('-');
    let year = date_parts.next()?.parse::<i32>().ok()?;
    let month = date_parts.next()?.parse::<u32>().ok()?;
    let day = date_parts.next()?.parse::<u32>().ok()?;
    if date_parts.next().is_some() {
        return None;
    }
    let time = time.split_once('.').map(|(whole, _)| whole).unwrap_or(time);
    let mut time_parts = time.split(':');
    let hour = time_parts.next()?.parse::<u32>().ok()?;
    let minute = time_parts.next()?.parse::<u32>().ok()?;
    let second = time_parts.next()?.parse::<u32>().ok()?;
    if time_parts.next().is_some()
        || !(1..=12).contains(&month)
        || !(1..=31).contains(&day)
        || hour > 23
        || minute > 59
        || second > 59
    {
        return None;
    }
    let days = days_from_civil(year, month, day)?;
    let seconds = days
        .checked_mul(86_400)?
        .checked_add(i64::from(hour) * 3_600)?
        .checked_add(i64::from(minute) * 60)?
        .checked_add(i64::from(second))?;
    u64::try_from(seconds).ok()?.checked_mul(1_000)
}

fn days_from_civil(year: i32, month: u32, day: u32) -> Option<i64> {
    let mut year = i64::from(year);
    let month = i64::from(month);
    let day = i64::from(day);
    year -= if month <= 2 { 1 } else { 0 };
    let era = if year >= 0 { year } else { year - 399 } / 400;
    let yoe = year - era * 400;
    let month_prime = month + if month > 2 { -3 } else { 9 };
    let doy = (153 * month_prime + 2) / 5 + day - 1;
    if !(0..=365).contains(&doy) {
        return None;
    }
    let doe = yoe * 365 + yoe / 4 - yoe / 100 + doy;
    Some(era * 146_097 + doe - 719_468)
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

/// Get the network interface index for a named adapter.
/// Using the numeric index with netsh avoids all command-line parsing issues
/// with interface names containing spaces (e.g. "SLAN LAN Adapter").
fn get_interface_index(interface_name: &str) -> Result<String> {
    let script = format!(
        "(Get-NetAdapter -IncludeHidden -Name '{}' -ErrorAction SilentlyContinue).ifIndex",
        escape_powershell_single_quoted(interface_name),
    );
    let output = run_powershell(&script)?;
    let idx = output.trim().to_string();
    if idx.is_empty() {
        bail!("SLAN local network adapter '{interface_name}' has no interface index");
    }
    Ok(idx)
}

fn configure_adapter_ip(interface_name: &str, virtual_ip: &str, prefix_len: u8) -> Result<()> {
    let virtual_ip = virtual_ip.trim();
    if virtual_ip.is_empty() || virtual_ip.eq_ignore_ascii_case("pending") {
        bail!("device unavailable: missing assigned virtual IP");
    }
    // Windows 10 can keep DHCP/APIPA active for several seconds after a Wintun
    // adapter is enabled. Disable DHCP explicitly and verify the exact address
    // in the same PowerShell operation before continuing with routes and DNS.
    let script = format!(
        "$ErrorActionPreference = 'Stop'; \
         $expected = '{}'; \
         $idx = (Get-NetAdapter -IncludeHidden -Name '{}' -ErrorAction SilentlyContinue).ifIndex; \
         if (-not $idx) {{ throw 'SLAN adapter not found' }}; \
         Set-NetIPInterface -InterfaceIndex $idx -AddressFamily IPv4 -Dhcp Disabled -ErrorAction Stop | Out-Null; \
         Remove-NetIPAddress -InterfaceIndex $idx -AddressFamily IPv4 -Confirm:$false -ErrorAction SilentlyContinue; \
         New-NetIPAddress -InterfaceIndex $idx -IPAddress $expected -PrefixLength {} -AddressFamily IPv4 -ErrorAction Stop | Out-Null; \
         $applied = $false; \
         for ($attempt = 0; $attempt -lt 40; $attempt++) {{ \
           $applied = $null -ne (Get-NetIPAddress -InterfaceIndex $idx -AddressFamily IPv4 -ErrorAction SilentlyContinue | Where-Object {{ $_.IPAddress -eq $expected -and $_.AddressState -eq 'Preferred' }} | Select-Object -First 1); \
           if ($applied) {{ break }}; \
           Start-Sleep -Milliseconds 250; \
         }}; \
         if (-not $applied) {{ \
           $state = (Get-NetIPAddress -InterfaceIndex $idx -AddressFamily IPv4 -ErrorAction SilentlyContinue | Where-Object {{ $_.IPAddress -eq $expected }} | Select-Object -First 1).AddressState; \
           throw \"SLAN adapter IP '$expected' did not become usable (state=$state)\" \
         }}; \
         Get-NetIPAddress -InterfaceIndex $idx -AddressFamily IPv4 -ErrorAction SilentlyContinue | Where-Object {{ $_.IPAddress -like '169.254.*' }} | Remove-NetIPAddress -Confirm:$false -ErrorAction SilentlyContinue; \
         Write-Output $expected",
        virtual_ip,
        escape_powershell_single_quoted(interface_name),
        prefix_len,
    );
    debug_log(&format!(
        "configure_adapter_ip: setting {virtual_ip}/{prefix_len} on '{interface_name}'"
    ));
    run_powershell(&script).context("set Wintun adapter IP via PowerShell")?;
    debug_log("configure_adapter_ip: PowerShell static IP configuration verified");
    // Verify via ipconfig.
    let ipconfig_output = run_ipconfig().unwrap_or_default();
    if output_contains_exact_ipv4(&ipconfig_output, virtual_ip) {
        debug_log("configure_adapter_ip: OK (ipconfig verified)");
        return Ok(());
    }
    debug_log(&format!(
        "configure_adapter_ip: ipconfig does not contain {virtual_ip}, trying Get-NetIPAddress"
    ));
    // Fallback: verify via Get-NetIPAddress.
    let verify_script = format!(
        "(Get-NetIPAddress -InterfaceAlias '{}' -AddressFamily IPv4 -ErrorAction SilentlyContinue).IPAddress -join ','",
        escape_powershell_single_quoted(interface_name),
    );
    let ips = run_powershell(&verify_script).unwrap_or_default();
    if output_contains_exact_ipv4(&ips, virtual_ip) {
        debug_log("configure_adapter_ip: OK (Get-NetIPAddress verified)");
        return Ok(());
    }
    debug_log(&format!("configure_adapter_ip: FAILED - ipconfig contains exact {virtual_ip}: {}, Get-NetIPAddress: [{ips}]", output_contains_exact_ipv4(&ipconfig_output, virtual_ip)));
    bail!(
        "SLAN local network adapter '{interface_name}' did not apply IP '{virtual_ip}': \
         ipconfig contains: {}, Get-NetIPAddress returned: [{ips}]",
        output_contains_exact_ipv4(&ipconfig_output, virtual_ip)
    );
}

fn verify_adapter_ip(interface_name: &str, virtual_ip: &str) -> Result<()> {
    debug_log(&format!(
        "verify_adapter_ip: checking '{interface_name}' for {virtual_ip}"
    ));
    // Use PowerShell for all verification — netsh is unreliable on Wintun adapters.
    // The address must be Preferred. A Tentative address exists in the store but
    // cannot be selected as a source address or carry overlay traffic.
    let script = format!(
        "$a = Get-NetAdapter -IncludeHidden -Name '{}' -ErrorAction SilentlyContinue; \
         if (-not $a) {{ throw 'adapter not found' }}; \
         $status = $a.Status.ToString(); \
         $admin = $a.AdminStatus.ToString(); \
         $addresses = Get-NetIPAddress -InterfaceAlias '{}' -AddressFamily IPv4 -ErrorAction SilentlyContinue; \
         $ips = ($addresses.IPAddress) -join ','; \
         $expectedState = ($addresses | Where-Object {{ $_.IPAddress -eq '{}' }} | Select-Object -First 1).AddressState; \
         Write-Output \"$admin|$status|$ips|$expectedState\"",
        escape_powershell_single_quoted(interface_name),
        escape_powershell_single_quoted(interface_name),
        escape_powershell_single_quoted(virtual_ip),
    );
    let output = run_powershell(&script).context("verify Wintun adapter IP via PowerShell")?;
    let parts: Vec<&str> = output.trim().splitn(4, '|').collect();
    let admin = parts.first().copied().unwrap_or("");
    let status = parts.get(1).copied().unwrap_or("");
    let ips = parts.get(2).copied().unwrap_or("");
    let address_state = parts.get(3).copied().unwrap_or("");
    debug_log(&format!(
        "verify_adapter_ip: admin={admin} status={status} ips=[{ips}] address_state={address_state}"
    ));
    if !output_contains_exact_ipv4(ips, virtual_ip) {
        bail!(
            "SLAN local network adapter '{interface_name}' expected IP '{virtual_ip}' but it was not applied. \
             admin={admin} status={status} Get-NetIPAddress returned: [{ips}]"
        );
    }
    if !address_state.eq_ignore_ascii_case("preferred") {
        bail!(
            "SLAN local network adapter '{interface_name}' IP '{virtual_ip}' is not usable. \
             admin={admin} status={status} addressState={address_state}"
        );
    }
    debug_log("verify_adapter_ip: OK");
    Ok(())
}

fn output_contains_exact_ipv4(output: &str, expected: &str) -> bool {
    output
        .split(|character: char| !(character.is_ascii_digit() || character == '.'))
        .any(|candidate| candidate == expected)
}

fn configure_routes(interface_name: &str, routes: &[RouteSpec]) -> Result<()> {
    if routes.is_empty() {
        return Ok(());
    }
    debug_log(&format!(
        "configure_routes: {} routes for '{interface_name}'",
        routes.len()
    ));
    // Use netsh with interface index for reliable Wintun route configuration.
    let idx =
        get_interface_index(interface_name).context("get adapter interface index for routes")?;
    debug_log(&format!("configure_routes: ifIndex={idx}"));
    for route in routes {
        let destination = route.destination.trim();
        if destination.is_empty() || destination.eq_ignore_ascii_case("mesh") {
            continue;
        }
        let gateway = route.gateway.as_deref().unwrap_or("0.0.0.0").trim();
        // netsh route destination must be in prefix/length format (e.g. "10.0.0.53/32").
        // The RouteSpec destination is already in CIDR format, pass it through directly.
        let netsh_dest = if destination.contains('/') {
            destination.to_string()
        } else {
            format!("{destination}/32")
        };
        // Remove existing route first (ignore errors if it doesn't exist).
        let _ = run_netsh(&[
            "interface",
            "ipv4",
            "delete",
            "route",
            &netsh_dest,
            &idx,
            gateway,
        ]);
        // Add the new route.
        run_netsh(&[
            "interface",
            "ipv4",
            "add",
            "route",
            &netsh_dest,
            &idx,
            gateway,
        ])
        .with_context(|| format!("add route {destination} via {gateway}"))?;
    }
    Ok(())
}

fn configure_adapter_mtu(interface_name: &str, mtu: u16) -> Result<()> {
    // Fast path: skip if MTU hasn't changed (prevents adapter toggle from repeated netsh calls).
    let runtime = windows_network_runtime()
        .lock()
        .expect("windows network runtime lock poisoned");
    if runtime.mtu == Some(mtu) {
        debug_log(&format!(
            "configure_adapter_mtu: fast path — mtu={mtu} unchanged, skipping"
        ));
        return Ok(());
    }
    drop(runtime);
    // Use netsh with interface index for reliable Wintun MTU configuration.
    let idx = get_interface_index(interface_name).context("get adapter interface index for MTU")?;
    run_netsh(&[
        "interface",
        "ipv4",
        "set",
        "subinterface",
        &idx,
        &format!("mtu={mtu}"),
        "store=active",
    ])
    .map(|_| ())
    .with_context(|| {
        format!("SLAN local network adapter '{interface_name}' MTU set failed (target={mtu})")
    })?;
    let mut runtime = windows_network_runtime()
        .lock()
        .expect("windows network runtime lock poisoned");
    runtime.mtu = Some(mtu);
    Ok(())
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
           Disable-NetAdapter -Name $name -Confirm:$false -ErrorAction SilentlyContinue | Out-Null; \
         }}",
        escape_powershell_single_quoted(interface_name),
    );
    run_powershell(&script).map(|_| ())
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
           dnsSearchDomains=@(); \
           dnsSplitDomains=@(); \
           routes=$routes; \
           checks=$checks \
         }} | ConvertTo-Json -Compress -Depth 5"
    );
    let output = run_powershell(&script)?;
    serde_json::from_str::<PlatformNetworkDiagnostics>(&output)
        .context("decode Windows network diagnostics")
}

#[cfg(test)]
fn is_usable_virtual_ip(ip: &str) -> bool {
    let ip = ip.trim();
    !ip.is_empty() && ip != "0.0.0.0" && !ip.starts_with("169.254.")
}

/// Write a debug message to `%ProgramData%\SLAN\slan-debug.log`.
/// This is necessary because `eprintln!` output is lost when running as a Windows service.
fn debug_log(message: &str) {
    let Ok(program_data) = std::env::var("ProgramData") else {
        return;
    };
    let path = PathBuf::from(program_data)
        .join("SLAN")
        .join("slan-debug.log");
    let _ = fs::create_dir_all(path.parent().unwrap());
    let timestamp = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_millis();
    if let Ok(mut file) = OpenOptions::new().create(true).append(true).open(&path) {
        let _ = writeln!(file, "{timestamp} {message}");
    }
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

/// Execute a `netsh` command and return stdout on success.
/// `netsh` is significantly faster and more reliable than PowerShell cmdlets for
/// virtual network adapter (Wintun/TAP) configuration — same approach used by
/// WireGuard and OpenVPN on Windows.
fn run_netsh(args: &[&str]) -> Result<String> {
    debug_log(&format!("netsh exec: netsh {}", args.join(" ")));
    let output = Command::new("netsh")
        .args(args)
        .creation_flags(CREATE_NO_WINDOW)
        .output()
        .context("run netsh command")?;
    let stdout = String::from_utf8_lossy(&output.stdout).trim().to_string();
    let stderr = String::from_utf8_lossy(&output.stderr).trim().to_string();
    debug_log(&format!(
        "netsh result: status={} stdout=[{}] stderr=[{}]",
        output.status, stdout, stderr
    ));
    if output.status.success() {
        return Ok(stdout);
    }
    let detail = if !stderr.is_empty() {
        stderr
    } else if !stdout.is_empty() {
        stdout
    } else {
        format!("exit status {}", output.status)
    };
    bail!("netsh command failed: {detail}");
}

/// Execute `ipconfig` and return its stdout.
/// Used for IP address verification because its output is more reliably parsed
/// than `netsh show addresses` on non-English Windows installations.
fn run_ipconfig() -> Result<String> {
    let output = Command::new("ipconfig")
        .creation_flags(CREATE_NO_WINDOW)
        .output()
        .context("run ipconfig")?;
    Ok(String::from_utf8_lossy(&output.stdout).to_string())
}

/// Prepare a value for use as a netsh argument.
/// Rust's `Command::args()` on Windows automatically handles quoting for arguments
/// containing spaces via the standard MSVCRT command-line escaping conventions.
/// We must NOT add manual quotes — that would cause double-quoting and netsh
/// would receive literal quote characters as part of the value.
#[cfg(test)]
fn escape_netsh_arg(value: &str) -> String {
    value.to_string()
}

/// Convert an IPv4 prefix length (0–32) to a dotted-decimal subnet mask.
#[cfg(test)]
fn prefix_len_to_subnet_mask(prefix_len: u8) -> String {
    if prefix_len == 0 {
        return "0.0.0.0".to_string();
    }
    let mask: u32 = 0xFFFFFFFFu32 << (32 - prefix_len.min(32));
    format!(
        "{}.{}.{}.{}",
        (mask >> 24) & 0xFF,
        (mask >> 16) & 0xFF,
        (mask >> 8) & 0xFF,
        mask & 0xFF,
    )
}

/// Parse a CIDR string like "10.0.0.0/8" into (prefix, mask) for netsh commands.
/// netsh expects routes in the form "prefix/mask" (e.g. "10.0.0.0/255.0.0.0").
#[cfg(test)]
fn parse_cidr_for_netsh(cidr: &str) -> (String, String) {
    if let Some((prefix, len_str)) = cidr.split_once('/') {
        if let Ok(len) = len_str.parse::<u8>() {
            return (prefix.to_string(), prefix_len_to_subnet_mask(len));
        }
    }
    // If already in prefix/mask form or unparseable, return as-is.
    let parts: Vec<&str> = cidr.splitn(2, '/').collect();
    (
        parts.first().copied().unwrap_or(cidr).to_string(),
        parts
            .get(1)
            .copied()
            .unwrap_or("255.255.255.255")
            .to_string(),
    )
}

#[cfg(test)]
mod tests {
    use super::{
        adapter_enable_requires_session_restart, detach_udp_relay_sessions,
        direct_udp_control_packet, direct_udp_control_payload, earliest_relay_ticket_expires_at,
        escape_netsh_arg, is_usable_dns_server, is_usable_virtual_ip, local_virtual_ip_reply,
        mark_ready_transports, normalize_direct_udp_address, output_contains_exact_ipv4,
        parse_cidr_for_netsh, prefix_len_to_subnet_mask, refresh_relay_ticket_timing,
        relay_error_message, relay_runtime_paths_from_config, relay_udp_address_for_session,
        send_frame_to_peer, validate_relay_peer_session, validate_relay_peer_session_for_path,
        AttachedRelayPeer, DerpTcpTransport, DirectUdpControlKind, DirectUdpPeer,
        DirectUdpTransport, PathSendResult, RelayUdpTransport, WindowsPathManager,
        WindowsRelayDataPlaneStats,
    };
    use crate::windows::parse_rfc3339_utc_ms;

    #[test]
    fn restarts_wintun_session_only_after_adapter_state_transition() {
        assert!(adapter_enable_requires_session_restart(
            "before=Down after=Up"
        ));
        assert!(adapter_enable_requires_session_restart(
            "before=Disabled after=Up"
        ));
        assert!(!adapter_enable_requires_session_restart(
            "before=Up after=Up"
        ));
        assert!(!adapter_enable_requires_session_restart(
            "unexpected output"
        ));
    }
    use client_core::{
        ipv4_destination,
        relay_frame::{base64_decode, encode_slan_relay_data_frame},
        relay_frame_is_replayed, relay_peer_index_for_packet, select_active_path,
        selected_runtime_paths, update_peer_active_path, PathCandidate, PathKind, PathPolicy,
        PathState, PeerPathConfig, PeerPathRuntime, RelayPeerSession, RelayTicket,
    };
    use std::net::UdpSocket;
    use std::time::Duration;

    #[test]
    fn rejects_windows_link_local_autoconfig_ip() {
        assert!(!is_usable_virtual_ip(""));
        assert!(!is_usable_virtual_ip("0.0.0.0"));
        assert!(!is_usable_virtual_ip("169.254.92.148"));
        assert!(is_usable_virtual_ip("10.0.0.10"));
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
    fn windows_local_virtual_ip_replies_to_icmp_echo() {
        let request = icmp_echo_request("10.0.0.9", "10.0.0.2");
        let reply = local_virtual_ip_reply(&request, "10.0.0.2/32").unwrap();

        assert_eq!(&reply[12..16], &[10, 0, 0, 2]);
        assert_eq!(&reply[16..20], &[10, 0, 0, 9]);
        assert_eq!(reply[20], 0);
        assert!(local_virtual_ip_reply(&request, "10.0.0.3/32").is_none());
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
            path_candidate(PathKind::DerpTcpTls443, PathState::Ready),
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
                path_candidate(PathKind::DerpTcpTls443, PathState::Standby),
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
            .any(|candidate| candidate.kind == PathKind::DerpTcpTls443));
    }

    #[test]
    fn runtime_paths_mark_ticket_scheme_path_ready_only() {
        let mut session = test_relay_peer_session("session-derp", "2026-05-03T10:00:00Z");
        session.ticket.relay_url = "derp://127.0.0.1:443".to_string();
        let configured = vec![PeerPathConfig {
            peer_node_id: "node-peer".to_string(),
            peer_virtual_ips: vec!["10.0.0.9".to_string()],
            candidates: vec![
                path_candidate(PathKind::RelayUdp, PathState::Standby),
                path_candidate(PathKind::DerpTcpTls443, PathState::Standby),
            ],
        }];

        let paths = relay_runtime_paths_from_config(&configured, &[session]);

        assert!(paths[0].candidates.iter().any(|candidate| {
            candidate.kind == PathKind::DerpTcpTls443 && candidate.state == PathState::Ready
        }));
        assert!(paths[0].candidates.iter().any(|candidate| {
            candidate.kind == PathKind::RelayUdp && candidate.state == PathState::Standby
        }));
    }

    #[test]
    fn runtime_paths_ignore_unknown_ticket_scheme() {
        let mut session = test_relay_peer_session("session-h3", "2026-05-03T10:00:00Z");
        session.ticket.relay_url = "h3://127.0.0.1:9443".to_string();
        let configured = vec![PeerPathConfig {
            peer_node_id: "node-peer".to_string(),
            peer_virtual_ips: vec!["10.0.0.9".to_string()],
            candidates: vec![path_candidate(PathKind::DerpTcpTls443, PathState::Standby)],
        }];

        let paths = relay_runtime_paths_from_config(&configured, &[session]);

        assert!(paths[0]
            .candidates
            .iter()
            .all(|candidate| candidate.state != PathState::Ready));
    }

    #[test]
    fn runtime_paths_keep_direct_udp_probing_until_probe_success() {
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
                path_kind: PathKind::DirectUdp,
                address: peer_socket.local_addr().unwrap().to_string(),
                socket_addr: peer_socket.local_addr().unwrap(),
            }],
        );

        let paths = selected_runtime_paths(
            &PathPolicy::default(),
            mark_ready_transports(configured_runtime_paths(configured), Some(&direct_udp)),
        );

        // 直连 UDP 路径在探测成功前保持 Probing，优先选择已 Ready 的 relay
        assert_eq!(paths[0].active_path, Some(PathKind::RelayUdp));
        assert!(paths[0].candidates.iter().any(|candidate| {
            candidate.kind == PathKind::DirectUdp && candidate.state == PathState::Probing
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
                    path_kind: PathKind::DirectUdp,
                    address: peer_a_addr.to_string(),
                    socket_addr: peer_a_addr,
                },
                DirectUdpPeer {
                    peer_node_id: "node-b".to_string(),
                    peer_virtual_ips: vec!["10.0.0.9/32".to_string()],
                    path_kind: PathKind::DirectUdp,
                    address: peer_b_addr.to_string(),
                    socket_addr: peer_b_addr,
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
                path_kind: PathKind::DirectUdp,
                address: old_peer_addr.to_string(),
                socket_addr: old_peer_addr,
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
                path_kind: PathKind::DirectUdp,
                address: old_peer_addr.to_string(),
                socket_addr: old_peer_addr,
            }],
        );

        assert!(!direct_udp.update_peer_endpoint(0, old_peer_addr));
        assert!(direct_udp.update_peer_endpoint(0, roamed_peer_addr));
        assert_eq!(direct_udp.peers[0].socket_addr, roamed_peer_addr);
        assert_eq!(direct_udp.peers[0].address, roamed_peer_addr.to_string());
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
                    path_kind: PathKind::DirectUdp,
                    address: peer_a_socket.local_addr().unwrap().to_string(),
                    socket_addr: peer_a_socket.local_addr().unwrap(),
                },
                DirectUdpPeer {
                    peer_node_id: "node-b".to_string(),
                    peer_virtual_ips: vec!["10.0.0.3/32".to_string()],
                    path_kind: PathKind::DirectUdp,
                    address: peer_b_socket.local_addr().unwrap().to_string(),
                    socket_addr: peer_b_socket.local_addr().unwrap(),
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
        assert_eq!(packet.peer_node_id(), "node-b");
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
                local_node_id: "node-local".to_string(),
                peer_virtual_ips: vec!["10.0.0.9/32".to_string()],
                path_kind: PathKind::RelayUdp,
                socket: sender,
                stats_index: 0,
            }],
            "node-local".to_string(),
        );
        let mut manager = WindowsPathManager::new(
            PathPolicy::default(),
            None,
            relay_udp,
            DerpTcpTransport::new(Vec::new()),
        );
        manager.update_peer_paths(&[PeerPathRuntime {
            peer_node_id: "node-a".to_string(),
            peer_virtual_ips: vec!["10.0.0.9/32".to_string()],
            active_path: Some(PathKind::DerpTcpTls443),
            candidates: vec![
                path_candidate(PathKind::RelayUdp, PathState::Ready),
                path_candidate(PathKind::DerpTcpTls443, PathState::Ready),
            ],
        }]);
        manager
            .tracker
            .set_active_path("node-a".to_string(), PathKind::DerpTcpTls443);
        let packet = ipv4_packet("10.0.0.1", "10.0.0.9");

        assert!(matches!(
            manager.send(&packet, b"relay-frame"),
            PathSendResult::Sent { peer_index: 0, .. }
        ));
        assert_forward_packet(&receiver, "session-a", "node-local", b"relay-frame");
    }

    #[test]
    fn path_manager_records_actual_sent_path_after_fallback_success() {
        let relay_udp = RelayUdpTransport::new(Vec::new(), "node-local".to_string());
        let mut manager = WindowsPathManager::new(
            PathPolicy::default(),
            None,
            relay_udp,
            DerpTcpTransport::new(Vec::new()),
        );
        manager
            .tracker
            .set_active_path("node-a".to_string(), PathKind::DirectUdp);

        let previous = manager.record_sent_path("node-a", PathKind::RelayUdp);

        assert_eq!(previous, Some(PathKind::DirectUdp));
        assert_eq!(
            manager.active_path_for_peer_node("node-a"),
            PathKind::RelayUdp
        );
    }

    #[test]
    fn path_manager_reports_missing_transport_when_fallback_is_disabled() {
        let relay_udp = RelayUdpTransport::new(Vec::new(), "node-local".to_string());
        let mut manager = WindowsPathManager::new(
            PathPolicy {
                fallback_enabled: false,
                ..PathPolicy::default()
            },
            None,
            relay_udp,
            DerpTcpTransport::new(Vec::new()),
        );
        manager
            .tracker
            .set_active_path("node-a".to_string(), PathKind::DerpTcpTls443);
        let packet = ipv4_packet("10.0.0.1", "10.0.0.9");

        assert!(matches!(
            manager.send(&packet, b"relay-frame"),
            PathSendResult::NoRoute
        ));
    }

    #[test]
    fn path_manager_records_failed_preferred_path_without_inventing_fallback() {
        let relay_udp = RelayUdpTransport::new(Vec::new(), "node-local".to_string());
        let mut manager = WindowsPathManager::new(
            PathPolicy::default(),
            None,
            relay_udp,
            DerpTcpTransport::new(Vec::new()),
        );
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
            PathKind::DirectUdp
        );
    }

    #[test]
    fn path_manager_upgrades_after_stable_direct_udp_probe_success() {
        let relay_udp = RelayUdpTransport::new(Vec::new(), "node-local".to_string());
        let mut manager = WindowsPathManager::new(
            PathPolicy::default(),
            None,
            relay_udp,
            DerpTcpTransport::new(Vec::new()),
        );
        manager
            .tracker
            .set_active_path("node-a".to_string(), PathKind::RelayUdp);

        assert!(!manager.record_probe_success("node-a", PathKind::DirectUdp));
        assert_eq!(
            manager
                .tracker
                .active_path_for_node("node-a", PathKind::RelayUdp),
            PathKind::RelayUdp
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
    fn path_manager_preserves_lan_udp_probe_success_kind() {
        let relay_udp = RelayUdpTransport::new(Vec::new(), "node-local".to_string());
        let mut manager = WindowsPathManager::new(
            PathPolicy::default(),
            None,
            relay_udp,
            DerpTcpTransport::new(Vec::new()),
        );
        manager
            .tracker
            .set_active_path("node-a".to_string(), PathKind::RelayUdp);

        assert!(!manager.record_probe_success("node-a", PathKind::LanUdp));
        assert!(manager.record_probe_success("node-a", PathKind::LanUdp));
        assert_eq!(
            manager
                .tracker
                .active_path_for_node("node-a", PathKind::RelayUdp),
            PathKind::LanUdp
        );
    }

    #[test]
    fn path_manager_treats_inbound_direct_probe_as_success() {
        let relay_udp = RelayUdpTransport::new(Vec::new(), "node-local".to_string());
        let mut manager = WindowsPathManager::new(
            PathPolicy::default(),
            None,
            relay_udp,
            DerpTcpTransport::new(Vec::new()),
        );
        manager
            .tracker
            .set_active_path("node-a".to_string(), PathKind::RelayUdp);

        let probe = direct_udp_control_payload(DirectUdpControlKind::Probe, "node-a");
        assert_eq!(
            direct_udp_control_packet(probe.as_bytes()).map(|packet| packet.kind),
            Some(DirectUdpControlKind::Probe)
        );
        assert!(!manager.record_probe_success("node-a", PathKind::DirectUdp));
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
        let mut manager = WindowsPathManager::new(
            PathPolicy::default(),
            None,
            relay_udp,
            DerpTcpTransport::new(Vec::new()),
        );

        assert_eq!(
            manager.active_path_for_peer_node("node-a"),
            PathKind::RelayUdp
        );
        manager
            .tracker
            .set_active_path("node-a".to_string(), PathKind::DerpTcpTls443);
        assert_eq!(
            manager.active_path_for_peer_node("node-a"),
            PathKind::DerpTcpTls443
        );
    }

    #[test]
    fn direct_udp_control_packets_are_recognized() {
        let probe = direct_udp_control_payload(DirectUdpControlKind::Probe, "node-local");
        let parsed = direct_udp_control_packet(probe.as_bytes()).unwrap();
        assert_eq!(parsed.kind, DirectUdpControlKind::Probe);
        assert_eq!(parsed.peer_node_id(), "node-local");
        assert!(probe.contains("\"kind\":\"direct_udp\""));
        let pong = br#"{"kind":"direct_udp","type":"pong","nodeId":"node-peer"}"#;
        let parsed = direct_udp_control_packet(pong).unwrap();
        assert_eq!(parsed.kind, DirectUdpControlKind::Pong);
        assert_eq!(parsed.peer_node_id(), "node-peer");
        assert_eq!(
            direct_udp_control_packet(
                br#"{"kind":"direct_udp","type":"probe","node_id":"node-peer"}"#
            ),
            None
        );
        assert_eq!(direct_udp_control_packet(b"not-control"), None);
    }

    #[test]
    fn direct_udp_address_normalization_accepts_udp_scheme() {
        assert_eq!(
            normalize_direct_udp_address("udp://127.0.0.1:3478").as_deref(),
            Some("127.0.0.1:3478")
        );
        assert_eq!(
            normalize_direct_udp_address("relay+udp://127.0.0.1:3478"),
            None
        );
        assert_eq!(
            normalize_direct_udp_address("direct+udp://127.0.0.1:3478").as_deref(),
            Some("127.0.0.1:3478")
        );
        assert_eq!(normalize_direct_udp_address("https://127.0.0.1:3478"), None);
        assert_eq!(normalize_direct_udp_address("   "), None);
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
    fn path_manager_reports_no_fallback_when_no_path_is_available() {
        let relay_udp = RelayUdpTransport::new(Vec::new(), "node-local".to_string());
        let manager = WindowsPathManager::new(
            PathPolicy::default(),
            None,
            relay_udp,
            DerpTcpTransport::new(Vec::new()),
        );

        assert_eq!(
            manager.fallback_path_for_node("node-a", PathKind::DirectUdp),
            None
        );
    }

    #[test]
    fn relay_udp_address_prefers_matching_ticket_url() {
        let mut session = test_relay_peer_session("session-udp", "2026-05-03T10:00:00Z");
        session.ticket.relay_url = "udp://127.0.0.1:3479".to_string();

        assert_eq!(
            relay_udp_address_for_session("127.0.0.1:3478", &session).as_deref(),
            Some("127.0.0.1:3479")
        );
    }

    #[test]
    fn relay_udp_address_falls_back_when_ticket_is_not_udp() {
        let mut session = test_relay_peer_session("session-derp", "2026-05-03T10:00:00Z");
        session.ticket.relay_url = "derp://127.0.0.1:443".to_string();

        assert_eq!(
            relay_udp_address_for_session("udp://127.0.0.1:3478", &session).as_deref(),
            Some("127.0.0.1:3478")
        );
    }

    #[test]
    fn relay_peer_session_requires_session_key() {
        let mut session = test_relay_peer_session("session-a", "2026-05-03T10:00:00Z");

        session.ticket.session_key.clear();

        assert!(validate_relay_peer_session("node-local", &session).is_err());
    }

    #[test]
    fn relay_peer_session_path_validation_requires_matching_url_scheme() {
        let mut session = test_relay_peer_session("session-a", "2026-05-03T10:00:00Z");

        assert!(
            validate_relay_peer_session_for_path("node-local", &session, PathKind::RelayUdp)
                .is_ok()
        );
        assert!(validate_relay_peer_session_for_path(
            "node-local",
            &session,
            PathKind::DerpTcpTls443
        )
        .is_err());

        session.ticket.relay_url = "relay+udp://127.0.0.1:3478".to_string();
        assert!(
            validate_relay_peer_session_for_path("node-local", &session, PathKind::RelayUdp)
                .is_ok()
        );

        session.ticket.relay_url = "derp://127.0.0.1:443".to_string();
        assert!(validate_relay_peer_session_for_path(
            "node-local",
            &session,
            PathKind::DerpTcpTls443
        )
        .is_ok());
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
                local_node_id: "node-local".to_string(),
                peer_virtual_ips: vec!["10.0.0.2/32".to_string()],
                path_kind: PathKind::RelayUdp,
                socket: sender_a,
                stats_index: 0,
            },
            AttachedRelayPeer {
                session_id: "session-b".to_string(),
                peer_node_id: "node-b".to_string(),
                local_node_id: "node-local".to_string(),
                peer_virtual_ips: vec!["10.0.0.9/32".to_string()],
                path_kind: PathKind::RelayUdp,
                socket: sender_b,
                stats_index: 1,
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
        assert_forward_packet(&receiver_b, "session-b", "node-local", frame);
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
                local_node_id: "node-local".to_string(),
                peer_virtual_ips: vec!["10.0.0.2/32".to_string()],
                path_kind: PathKind::RelayUdp,
                socket: sender_a,
                stats_index: 0,
            },
            AttachedRelayPeer {
                session_id: "session-b".to_string(),
                peer_node_id: "node-b".to_string(),
                local_node_id: "node-local".to_string(),
                peer_virtual_ips: vec!["10.0.0.9/32".to_string()],
                path_kind: PathKind::RelayUdp,
                socket: sender_b,
                stats_index: 1,
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
                local_node_id: "node-local".to_string(),
                peer_virtual_ips: vec!["10.0.0.2/32".to_string()],
                path_kind: PathKind::RelayUdp,
                socket: sender_a,
                stats_index: 0,
            },
            AttachedRelayPeer {
                session_id: "session-b".to_string(),
                peer_node_id: "node-b".to_string(),
                local_node_id: "node-local".to_string(),
                peer_virtual_ips: vec!["10.0.0.9/32".to_string()],
                path_kind: PathKind::RelayUdp,
                socket: sender_b,
                stats_index: 1,
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

    #[test]
    fn relay_ticket_timing_fields_are_refreshed_before_persisting() {
        let now = parse_rfc3339_utc_ms("2026-05-03T10:00:00Z").unwrap();
        let mut stats = WindowsRelayDataPlaneStats {
            ticket_expires_at: Some("2026-05-03T10:01:30Z".to_string()),
            ..WindowsRelayDataPlaneStats::default()
        };

        refresh_relay_ticket_timing(&mut stats, now);

        assert_eq!(stats.ticket_expires_in_ms, Some(90_000));
        assert!(stats.ticket_renew_due);
    }

    #[test]
    fn relay_ticket_timing_fields_clear_invalid_expiration() {
        let mut stats = WindowsRelayDataPlaneStats {
            ticket_expires_at: Some("not-a-date".to_string()),
            ticket_expires_in_ms: Some(1),
            ticket_renew_due: true,
            ..WindowsRelayDataPlaneStats::default()
        };

        refresh_relay_ticket_timing(&mut stats, 0);

        assert_eq!(stats.ticket_expires_in_ms, None);
        assert!(!stats.ticket_renew_due);
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

    fn icmp_echo_request(src: &str, dst: &str) -> Vec<u8> {
        let mut packet = ipv4_packet(src, dst);
        packet.resize(32, 0);
        packet[2..4].copy_from_slice(&(32_u16).to_be_bytes());
        packet[9] = 1;
        packet[20] = 8;
        packet[24..26].copy_from_slice(&7_u16.to_be_bytes());
        packet[26..28].copy_from_slice(&9_u16.to_be_bytes());
        packet[28..32].copy_from_slice(b"slan");
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

    fn assert_forward_packet(
        receiver: &UdpSocket,
        session_id: &str,
        participant_id: &str,
        frame: &[u8],
    ) {
        let mut buffer = [0_u8; 512];
        let size = receiver.recv(&mut buffer).unwrap();
        let value: serde_json::Value = serde_json::from_slice(&buffer[..size]).unwrap();
        assert_eq!(
            value.get("kind").and_then(serde_json::Value::as_str),
            Some("forward")
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
        let payload = value
            .get("payload")
            .and_then(serde_json::Value::as_str)
            .and_then(base64_decode)
            .unwrap();
        assert_eq!(payload, frame);
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

    #[test]
    fn prefix_len_to_subnet_mask_returns_correct_values() {
        assert_eq!(prefix_len_to_subnet_mask(0), "0.0.0.0");
        assert_eq!(prefix_len_to_subnet_mask(8), "255.0.0.0");
        assert_eq!(prefix_len_to_subnet_mask(16), "255.255.0.0");
        assert_eq!(prefix_len_to_subnet_mask(24), "255.255.255.0");
        assert_eq!(prefix_len_to_subnet_mask(32), "255.255.255.255");
        assert_eq!(prefix_len_to_subnet_mask(30), "255.255.255.252");
    }

    #[test]
    fn parse_cidr_for_netsh_handles_various_formats() {
        assert_eq!(
            parse_cidr_for_netsh("10.0.0.0/8"),
            ("10.0.0.0".to_string(), "255.0.0.0".to_string())
        );
        assert_eq!(
            parse_cidr_for_netsh("192.168.1.0/24"),
            ("192.168.1.0".to_string(), "255.255.255.0".to_string())
        );
        assert_eq!(
            parse_cidr_for_netsh("10.0.1.114/32"),
            ("10.0.1.114".to_string(), "255.255.255.255".to_string())
        );
    }

    #[test]
    fn escape_netsh_arg_wraps_in_double_quotes() {
        assert_eq!(escape_netsh_arg("SLAN LAN Adapter"), "SLAN LAN Adapter");
        assert_eq!(escape_netsh_arg("simple"), "simple");
    }

    #[test]
    fn exact_ipv4_match_does_not_accept_address_prefixes() {
        assert!(output_contains_exact_ipv4(
            "169.254.20.1,10.0.1.114",
            "10.0.1.114"
        ));
        assert!(!output_contains_exact_ipv4("10.0.1.114", "10.0.1.11"));
        assert!(!output_contains_exact_ipv4("10.0.1.1140", "10.0.1.114"));
    }
}
