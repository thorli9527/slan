#![cfg_attr(target_os = "windows", windows_subsystem = "windows")]

mod acl_policy;
mod client_config;
mod client_message_mqtt;
mod control_plane;
mod control_tasks;
mod control_transport;
mod control_transport_worker;
mod diagnostic_upload;
mod local_api;
#[cfg(test)]
mod main_tests;
mod mobile_platform_config;
mod network_event;
mod network_event_apply;
mod network_event_projection;
mod network_module;
mod network_runtime_state;
mod platform_runtime_report;
mod platform_transition;
mod relay_candidates;
mod relay_models;
mod relay_store;
mod resolver_apply;
mod resolver_authority;
mod resolver_forwarder;
mod resolver_runtime_state;
mod resolver_server;
mod runtime_actor;
mod runtime_event_hub;
mod session_store;
mod time_utils;

use std::{
    collections::{BTreeMap, BTreeSet},
    fs::{self, OpenOptions},
    io::{BufRead, BufReader, ErrorKind, Write},
    net::{TcpListener, TcpStream},
    path::Path,
    sync::{
        atomic::{AtomicU64, AtomicUsize, Ordering},
        Arc, Mutex, OnceLock,
    },
    thread,
    time::{Duration, Instant, UNIX_EPOCH},
};

const SERVICE_LOG_MAX_BYTES: u64 = 20 * 1024 * 1024;
const SERVICE_LOG_BACKUP_COUNT: usize = 3;
static SERVICE_LOG_LOCK: OnceLock<Mutex<()>> = OnceLock::new();

#[cfg(target_os = "windows")]
use std::{
    ffi::OsString,
    process::{Command, Stdio},
};

use anyhow::{Context, Result};
use client_core::{
    assess_signal_quality, normalize_relay_transport, normalize_virtual_ip,
    relay_path_kind_for_transport, AssignedIpPayload, AuthPayload, ClientCommand, ClientRuntime,
    ClientViewState, NetworkRuntimeState, PasswordLoginPayload, PathCandidate, PathKind, PathState,
    PeerPathConfig, PlatformAclPolicy, PlatformNetwork, PlatformNetworkDiagnostics,
    PlatformResolverConfig, PlatformResolverRecord, PlatformResolverZone, RelayDataPlaneConfig,
    RelayPeerSession, RelayTicket, SLAN_DNS_SERVICE_IP,
};
use client_core_platform::direct_udp::configured_direct_udp_port;
use client_core_platform::PlatformNetworkImpl;
use serde_json::Value;
#[cfg(target_os = "windows")]
use windows_service::define_windows_service;
#[cfg(target_os = "windows")]
use windows_service::service::{
    ServiceControl, ServiceControlAccept, ServiceExitCode, ServiceState, ServiceStatus, ServiceType,
};
#[cfg(target_os = "windows")]
use windows_service::service_control_handler::{self, ServiceControlHandlerResult};
#[cfg(target_os = "windows")]
use windows_service::service_dispatcher;

use crate::acl_policy::{acl_policies_for_network, platform_acl_policies};
use crate::control_plane::{
    local_stable_device_id, reset_local_device_id, ControlPeer, ControlPlaneClient,
    PunchConnectSession, RelayCandidate,
};
use crate::control_tasks::{ControlTaskAction, ControlTaskQueue, EnqueueControlTaskRequest};
use crate::control_transport::{
    ControlTransportCadence, ControlTransportOutbox, ControlTransportOutboxRequest,
    ControlTransportPlan, ControlTransportStatus, ControlTransportTickPlan,
    ControlTransportTickRequest, PublishedControlTransportMessage,
};
use crate::control_transport_worker::ControlTransportWorkerState;
use crate::local_api::{
    request_correlation_id, response_with_correlation_id, LocalPathPlanResponse, LocalPeerView,
    LocalPeersResponse, LocalResolverResolveRequest, LocalServiceMethod, LocalSessionResponse,
    LocalStatusResponse, MarkControlAckedRequest, PlatformRuntimeStateReportRequest,
    RegisterTestUserRequest, ServiceRequest, SetRelayTransportAllowlistRequest,
    WatchBusinessEventRequest, WatchBusinessEventResponse, WatchStateRequest, WatchStateResponse,
    BUSINESS_CONTROL_SYNC_CHANGED, BUSINESS_NETWORK_RUNTIME_CHANGED,
    BUSINESS_NETWORK_SWITCH_FAILED, BUSINESS_NETWORK_SWITCH_FINISHED, BUSINESS_SESSION_CHANGED,
    BUSINESS_STATE_CHANGED,
};
use crate::mobile_platform_config::{
    MobilePlatformDeviceNetworkConfig, MobilePlatformNetworkConfig,
};
use crate::network_event_projection::apply_prepared_session_projection;
use crate::network_runtime_state::runtime_network_state_store;
use crate::platform_runtime_report::{platform_traffic_stats_payload, report_platform_runtime};
use crate::platform_transition::PlatformNetworkActivation;
use crate::relay_candidates::{
    best_relay_candidate, best_udp_relay_candidate, diagnose_direct_candidates,
    extract_persisted_relay_candidates_from_control_map, normalize_relay_candidate_address,
    relay_candidate_probe_fallback, replace_runtime_relay_candidates, runtime_relay_candidates,
    select_relay_candidates,
};
use crate::relay_models::{
    PathDiagnoseHealth, PathDiagnoseHealthReason, PathDiagnoseMtu, PathDiagnosePathCount,
    PathDiagnoseRelay, PathDiagnoseRelayPeer, PathDiagnoseResolver, PathDiagnoseResponse,
    PersistedRelayCandidate, RelayCandidateListResponse, RelayCandidateSelection,
    RelayRuntimeStats,
};
use crate::relay_store::{
    diagnostics_export_file_path, load_relay_runtime_stats, relay_only_path_policy_enabled,
    relay_path_policy, relay_payload_policy, relay_runtime_failure_total, relay_stats_file_path,
};
use crate::resolver_authority::{resolve_authoritative, resolve_authoritative_result_json};
use crate::resolver_runtime_state::resolver_runtime_state;
use crate::resolver_server::{
    desired_local_resolver_bind_addr, local_resolver_server_last_query_at_ms,
    local_resolver_server_status, set_local_resolver_server_status, LocalResolverServerStatus,
    ResolverServer,
};
use crate::runtime_actor::RuntimeActorHandle;
use crate::runtime_event_hub::RuntimeEventHub;
use crate::session_store::{
    app_data_dir, clear_pending_console_login, current_session_runtime_epoch, current_timestamp_ms,
    ensure_session_node_binding, load_pending_console_login, load_session,
    load_valid_registered_session, lock_session_runtime_epoch, persist_session,
    prepare_client_login_session, prepare_session_device_registered,
    prepare_session_from_control_plane, remove_session, remove_user_session_preserving_device,
    report_runtime_state, revoke_remote_sessions, session_auth_invalid_error,
    session_device_api_token, session_is_expired, session_not_found_error,
    sync_session_device_fields, PersistedSession, PreparedSession,
};
use crate::time_utils::{parse_rfc3339_utc_ms, ticket_timing_with_window, TicketTiming};

#[cfg(test)]
pub(crate) fn test_env_lock() -> std::sync::MutexGuard<'static, ()> {
    static LOCK: OnceLock<Mutex<()>> = OnceLock::new();
    LOCK.get_or_init(|| Mutex::new(()))
        .lock()
        .expect("test env mutex poisoned")
}

const DEFAULT_SERVICE_HOST: &str = "127.0.0.1:46392";
#[cfg(target_os = "windows")]
const WINDOWS_SERVICE_NAME: &str = "SLANClientV2Service";
const RELAY_MAINTENANCE_INTERVAL: Duration = Duration::from_secs(30);
const CONNECT_PLAN_TTL_MS: u64 = 10 * 60 * 1000;
const RELAY_TICKET_RENEW_INTERVAL_MS: u64 = 20 * 60 * 1000;
const RELAY_TICKET_RENEW_WINDOW_MS: u64 = 5 * 60 * 1000;
const RELAY_RECONFIGURE_BACKOFF_MS: u64 = 60 * 1000;
const RELAY_STATS_STALE_MS: u64 = 45 * 1000;
const RELAY_IDLE_RECONFIGURE_MS: u64 = 60 * 1000;
const RELAY_NO_RX_RECONFIGURE_INTERVALS: u32 = 2;
const RELAY_RESPONSE_GAP_DEGRADED_PACKETS: u64 = 10;
const RELAY_FAILURE_RECONFIGURE_DELTA: u64 = 5;
static RUNTIME_CONNECT_PLANS: OnceLock<Mutex<PersistedConnectPlanStore>> = OnceLock::new();
const SESSION_REFRESH_INTERVAL: Duration = Duration::from_secs(60);
const SESSION_AUTH_INVALID_GRACE: Duration = Duration::from_secs(5 * 60);
const LOCAL_REQUEST_CONCURRENCY_LIMIT: usize = 64;
const LOCAL_WATCH_CONCURRENCY_LIMIT: usize = 8;

#[cfg(target_os = "windows")]
define_windows_service!(ffi_service_main, service_main);

#[derive(Debug, Default)]
struct ControlSyncThrottle {
    last_sync_ms: u64,
}

#[derive(Clone)]
struct LocalServiceContext {
    runtime: RuntimeActorHandle,
    task_queue: Arc<Mutex<ControlTaskQueue>>,
    transport_worker_state: Arc<Mutex<ControlTransportWorkerState>>,
    sync_throttle: Arc<Mutex<ControlSyncThrottle>>,
    state_notifier: Arc<RuntimeEventHub>,
    request_metrics: Arc<LocalRequestMetrics>,
}

#[derive(Debug, Clone, PartialEq, Eq)]
struct LocalRequestDiagnostics {
    active: usize,
    active_watches: usize,
    accepted_total: u64,
    completed_total: u64,
    rejected_total: u64,
    watch_accepted_total: u64,
    watch_rejected_total: u64,
}

#[derive(Debug, Default)]
struct LocalRequestMetrics {
    active: AtomicUsize,
    active_watches: AtomicUsize,
    accepted_total: AtomicU64,
    completed_total: AtomicU64,
    rejected_total: AtomicU64,
    watch_accepted_total: AtomicU64,
    watch_rejected_total: AtomicU64,
}

impl LocalRequestMetrics {
    fn try_accept(&self) -> bool {
        if !try_increment_below(&self.active, LOCAL_REQUEST_CONCURRENCY_LIMIT) {
            self.rejected_total.fetch_add(1, Ordering::Relaxed);
            return false;
        }
        self.accepted_total.fetch_add(1, Ordering::Relaxed);
        true
    }

    fn finish(&self) {
        self.active.fetch_sub(1, Ordering::Relaxed);
        self.completed_total.fetch_add(1, Ordering::Relaxed);
    }

    fn try_begin_watch(&self) -> bool {
        if !try_increment_below(&self.active_watches, LOCAL_WATCH_CONCURRENCY_LIMIT) {
            self.watch_rejected_total.fetch_add(1, Ordering::Relaxed);
            return false;
        }
        self.watch_accepted_total.fetch_add(1, Ordering::Relaxed);
        true
    }

    fn end_watch(&self) {
        self.active_watches.fetch_sub(1, Ordering::Relaxed);
    }

    fn diagnostics(&self) -> LocalRequestDiagnostics {
        LocalRequestDiagnostics {
            active: self.active.load(Ordering::Relaxed),
            active_watches: self.active_watches.load(Ordering::Relaxed),
            accepted_total: self.accepted_total.load(Ordering::Relaxed),
            completed_total: self.completed_total.load(Ordering::Relaxed),
            rejected_total: self.rejected_total.load(Ordering::Relaxed),
            watch_accepted_total: self.watch_accepted_total.load(Ordering::Relaxed),
            watch_rejected_total: self.watch_rejected_total.load(Ordering::Relaxed),
        }
    }
}

fn try_increment_below(value: &AtomicUsize, limit: usize) -> bool {
    let mut current = value.load(Ordering::Relaxed);
    loop {
        if current >= limit {
            return false;
        }
        match value.compare_exchange_weak(current, current + 1, Ordering::AcqRel, Ordering::Relaxed)
        {
            Ok(_) => return true,
            Err(actual) => current = actual,
        }
    }
}

struct LocalRequestGuard(Arc<LocalRequestMetrics>);

impl Drop for LocalRequestGuard {
    fn drop(&mut self) {
        self.0.finish();
    }
}

struct LocalWatchGuard(Arc<LocalRequestMetrics>);

impl Drop for LocalWatchGuard {
    fn drop(&mut self) {
        self.0.end_watch();
    }
}

fn disable_platform_network_serialized() -> Result<()> {
    platform_transition::run_serialized("network.disable.background", |platform| {
        platform_transition::disable_network(platform)
    })
}

fn main() -> Result<()> {
    if std::env::args().any(|arg| arg == "--version") {
        println!("{}", env!("CARGO_PKG_VERSION"));
        return Ok(());
    }
    if std::env::args().any(|arg| arg == "--service-info") {
        println!("{}", service_info_json()?);
        return Ok(());
    }
    if std::env::args().any(|arg| arg == "--ensure-device-id") {
        println!("{}", local_stable_device_id()?);
        return Ok(());
    }
    if std::env::args().any(|arg| arg == "--reset-device-id") {
        println!("{}", reset_local_device_id()?);
        return Ok(());
    }
    if std::env::args()
        .any(|arg| arg == "--install-adapter" || arg == "--prepare-adapter" || arg == "--driver")
    {
        platform_transition::run_serialized("adapter.install.cli", |platform| {
            platform
                .install_adapter()
                .context("install platform adapter")
        })?;
        return Ok(());
    }
    #[cfg(target_os = "windows")]
    if std::env::args().any(|arg| arg == "--windows-service") {
        return service_dispatcher::start(WINDOWS_SERVICE_NAME, ffi_service_main)
            .context("start Windows service dispatcher");
    }

    run_service_server()
}

fn service_info_json() -> Result<String> {
    serde_json::to_string(&serde_json::json!({
        "service": "client-core-service",
        "version": env!("CARGO_PKG_VERSION"),
        "targetOs": std::env::consts::OS,
        "targetArch": std::env::consts::ARCH,
        "defaultHost": DEFAULT_SERVICE_HOST,
        "stateDir": app_data_dir().join("SLAN").display().to_string(),
    }))
    .context("encode service info")
}

fn run_service_server() -> Result<()> {
    log_service_error("client-core-service starting");
    ensure_elevated_runtime()?;
    let _single_instance = acquire_single_instance_guard()?;

    let bind_address = std::env::var("SLAN_CLIENT_CORE_SERVICE_HOST")
        .unwrap_or_else(|_| DEFAULT_SERVICE_HOST.to_string());
    let listener = TcpListener::bind(&bind_address)
        .with_context(|| format!("bind client-core-service on {bind_address}"))?;
    let mut initial_runtime = ClientRuntime::new(PlatformNetworkImpl);
    if let Some(session) = load_valid_registered_session()
        .filter(|session| session.session_kind == "user" && !session.access_token.trim().is_empty())
    {
        let _ = initial_runtime.dispatch(ClientCommand::ApplyDeviceUserLogin(session.into()));
    } else if let Some(state) = apply_pending_console_login(&mut initial_runtime) {
        if let Some(error) = state
            .error
            .as_ref()
            .filter(|value| !value.trim().is_empty())
        {
            log_service_error(format!(
                "client-core-service pending console login failed: {error}"
            ));
        }
    }
    #[cfg(target_os = "windows")]
    if let Err(error) = platform_transition::run_serialized("adapter.install.startup", |platform| {
        platform
            .install_adapter()
            .context("install platform adapter")
    }) {
        log_service_error(format!(
            "client-core-service failed to prepare Wintun adapter: {error:#}"
        ));
    }
    let runtime = RuntimeActorHandle::spawn(initial_runtime);
    let task_queue = Arc::new(Mutex::new(ControlTaskQueue::load_default()));
    let sync_throttle = Arc::new(Mutex::new(ControlSyncThrottle::default()));
    let transport_worker_state = Arc::new(Mutex::new(ControlTransportWorkerState::default()));
    let state_notifier = runtime.events();
    let request_metrics = Arc::new(LocalRequestMetrics::default());
    let service_context = LocalServiceContext {
        runtime: runtime.clone(),
        task_queue: Arc::clone(&task_queue),
        transport_worker_state: Arc::clone(&transport_worker_state),
        sync_throttle: Arc::clone(&sync_throttle),
        state_notifier: Arc::clone(&state_notifier),
        request_metrics: Arc::clone(&request_metrics),
    };
    spawn_session_refresh_worker(runtime.clone(), Arc::clone(&state_notifier));
    spawn_runtime_sync_worker(runtime.clone(), Arc::clone(&state_notifier));
    spawn_platform_diagnostics_worker();
    diagnostic_upload::spawn_worker();
    spawn_platform_late_completion_worker(runtime.clone(), Arc::clone(&state_notifier));
    spawn_local_resolver_supervisor(runtime.clone());
    spawn_control_task_worker(
        runtime.clone(),
        Arc::clone(&task_queue),
        Arc::clone(&state_notifier),
    );
    control_transport_worker::spawn_control_transport_supervisor(
        runtime.clone(),
        Arc::clone(&task_queue),
        Arc::clone(&transport_worker_state),
        Arc::clone(&state_notifier),
    );
    control_transport_worker::wake_control_transport_worker(
        &runtime,
        &task_queue,
        &transport_worker_state,
        &state_notifier,
    );
    spawn_relay_data_plane_maintenance_worker(runtime.clone(), Arc::clone(&state_notifier));
    spawn_startup_network_activation(runtime.clone(), Arc::clone(&state_notifier));
    println!("client-core-service listening on {bind_address}");

    loop {
        match listener.accept() {
            Ok((stream, _peer)) => {
                if !request_metrics.try_accept() {
                    log_service_error(
                        "client-core-service local request concurrency limit reached",
                    );
                    drop(stream);
                    continue;
                }
                let context = service_context.clone();
                let request_guard = LocalRequestGuard(Arc::clone(&request_metrics));
                thread::spawn(move || {
                    let _request_guard = request_guard;
                    if let Err(error) = handle_connection(stream, context) {
                        log_service_error(format!(
                            "client-core-service connection error: {error:#}"
                        ));
                    }
                });
            }
            Err(error) => {
                let delay = match error.kind() {
                    ErrorKind::Interrupted => Duration::from_millis(10),
                    ErrorKind::WouldBlock => Duration::from_millis(50),
                    _ => Duration::from_millis(250),
                };
                log_service_error(format!(
                    "client-core-service accept failed: {error}; retrying in {}ms",
                    delay.as_millis()
                ));
                thread::sleep(delay);
            }
        }
    }
}

fn apply_pending_console_login<P>(runtime: &mut ClientRuntime<P>) -> Option<ClientViewState>
where
    P: client_core::PlatformNetwork,
{
    let pending = load_pending_console_login()?;
    if let Some(base_url) = pending.base_url.as_deref() {
        crate::control_plane::set_control_base_url_override(base_url);
    }
    if let Some(device_name) = pending.device_name.as_deref() {
        log_service_error(format!(
            "client-core-service console bootstrap deviceName={device_name}"
        ));
    }
    let expected_device_id = runtime.state().device_id.clone();
    let expected_signed_in = runtime.state().signed_in;
    let prepared_login = prepare_password_login(PasswordLoginPayload {
        email: pending.email,
        password: pending.password,
    });
    let login_state = commit_prepared_login(
        runtime,
        prepared_login,
        expected_device_id,
        expected_signed_in,
        "登录失败",
    );
    if login_state.error.is_some() {
        return Some(login_state);
    }
    let final_state = if pending.enable_network {
        let prepared = prepare_latest_control_network_activation();
        // Check if plan has a virtual IP; if not, skip platform activation (no network assigned).
        let has_virtual_ip = prepared.as_ref().ok().is_some_and(|plan| {
            plan.session.virtual_ip.as_deref().map(str::trim).is_some_and(|v| !v.is_empty())
        });
        if !has_virtual_ip {
            log_service_error(
                "client-core-service startup: skipping platform activation, no virtual IP assigned",
            );
            let executed = prepared.map(|plan| plan);
            let committed = commit_control_network_activation_result(runtime, executed);
            if committed.rollback_platform {
                let _ = platform_transition::disable_network(&PlatformNetworkImpl);
            }
            committed.state
        } else {
            platform_transition::run_inline_serialized("startup.pending.enable", |platform| {
                let executed = match prepared {
                    Ok(plan) => execute_platform_network_activation(platform, &plan)
                        .map(|()| plan)
                        .inspect_err(|_| {
                            let _ = platform_transition::disable_network(platform);
                        }),
                    Err(error) => Err(error),
                };
                let committed = commit_control_network_activation_result(runtime, executed);
                if committed.rollback_platform {
                    let _ = platform_transition::disable_network(platform);
                }
                Ok(committed.state)
            })
            .unwrap_or_else(|error| state_with_error(runtime.state(), error.to_string()))
        }
    } else {
        login_state
    };
    if pending.enable_network && final_state.error.is_none() {
        report_runtime_state(&final_state);
    }
    if final_state.error.is_none() {
        if let Err(error) = clear_pending_console_login() {
            log_service_error(format!(
                "client-core-service clear pending console login failed: {error:#}"
            ));
        }
    }
    Some(final_state)
}

#[cfg(target_os = "windows")]
fn service_main(_arguments: Vec<OsString>) {
    if let Err(error) = run_windows_service() {
        log_service_error(format!(
            "client-core-service windows service fatal: {error:#}"
        ));
    }
}

#[cfg(target_os = "windows")]
fn run_windows_service() -> Result<()> {
    let status_handle =
        service_control_handler::register(WINDOWS_SERVICE_NAME, move |control_event| {
            match control_event {
                ServiceControl::Stop | ServiceControl::Shutdown => {
                    log_service_error("client-core-service windows service stopping");
                    std::process::exit(0);
                }
                _ => ServiceControlHandlerResult::NotImplemented,
            }
        })
        .context("register Windows service control handler")?;
    set_windows_service_status(&status_handle, ServiceState::StartPending)?;
    set_windows_service_status(&status_handle, ServiceState::Running)?;
    let result = run_service_server();
    let _ = set_windows_service_status(&status_handle, ServiceState::Stopped);
    result
}

#[cfg(target_os = "windows")]
fn set_windows_service_status(
    status_handle: &windows_service::service_control_handler::ServiceStatusHandle,
    state: ServiceState,
) -> Result<()> {
    let controls = match state {
        ServiceState::Running => ServiceControlAccept::STOP | ServiceControlAccept::SHUTDOWN,
        _ => ServiceControlAccept::empty(),
    };
    status_handle
        .set_service_status(ServiceStatus {
            service_type: ServiceType::OWN_PROCESS,
            current_state: state,
            controls_accepted: controls,
            exit_code: ServiceExitCode::Win32(0),
            checkpoint: 0,
            wait_hint: Duration::from_secs(10),
            process_id: None,
        })
        .context("set Windows service status")
}

fn ensure_elevated_runtime() -> Result<()> {
    if is_elevated_runtime() {
        return Ok(());
    }
    let message =
        "client-core-service requires administrator permission for Wintun and network settings";
    log_service_error(message);
    anyhow::bail!(message)
}

#[cfg(target_os = "windows")]
fn is_elevated_runtime() -> bool {
    #[link(name = "shell32")]
    extern "system" {
        fn IsUserAnAdmin() -> i32;
    }
    unsafe { IsUserAnAdmin() != 0 }
}

#[cfg(not(target_os = "windows"))]
fn is_elevated_runtime() -> bool {
    true
}

struct SingleInstanceGuard {
    path: std::path::PathBuf,
    pid: u32,
}

impl Drop for SingleInstanceGuard {
    fn drop(&mut self) {
        let current = self.pid.to_string();
        if fs::read_to_string(&self.path)
            .map(|value| value.trim() == current)
            .unwrap_or(false)
        {
            let _ = fs::remove_file(&self.path);
        }
    }
}

fn acquire_single_instance_guard() -> Result<SingleInstanceGuard> {
    let dir = app_data_dir().join("SLAN");
    fs::create_dir_all(&dir).with_context(|| format!("create {}", dir.display()))?;
    let path = dir.join("client-core-service.pid");
    if let Ok(existing) = fs::read_to_string(&path) {
        if let Ok(pid) = existing.trim().parse::<u32>() {
            if pid != std::process::id() && process_is_running(pid) {
                anyhow::bail!("client-core-service already running with pid {pid}");
            }
        }
        let _ = fs::remove_file(&path);
    }
    let pid = std::process::id();
    let mut file = OpenOptions::new()
        .write(true)
        .create_new(true)
        .open(&path)
        .with_context(|| format!("create single-instance lock {}", path.display()))?;
    writeln!(file, "{pid}").context("write single-instance pid")?;
    Ok(SingleInstanceGuard { path, pid })
}

fn process_is_running(pid: u32) -> bool {
    if pid == 0 {
        return false;
    }
    #[cfg(unix)]
    {
        std::process::Command::new("kill")
            .arg("-0")
            .arg(pid.to_string())
            .status()
            .map(|status| status.success())
            .unwrap_or(false)
    }
    #[cfg(windows)]
    {
        return std::process::Command::new("tasklist")
            .args(["/FI", &format!("PID eq {pid}")])
            .output()
            .map(|output| String::from_utf8_lossy(&output.stdout).contains(&pid.to_string()))
            .unwrap_or(false);
    }
    #[cfg(not(any(unix, windows)))]
    {
        false
    }
}

fn handle_connection(mut stream: TcpStream, context: LocalServiceContext) -> Result<()> {
    let _ = stream.set_read_timeout(Some(Duration::from_secs(95)));
    let _ = stream.set_write_timeout(Some(Duration::from_secs(10)));
    let mut line = String::new();
    {
        let mut reader = BufReader::new(&mut stream);
        if reader.read_line(&mut line)? == 0 {
            return Ok(());
        }
    }
    let _watch_guard = if request_is_watch(&line) {
        if !context.request_metrics.try_begin_watch() {
            anyhow::bail!("local watch concurrency limit reached");
        }
        Some(LocalWatchGuard(Arc::clone(&context.request_metrics)))
    } else {
        None
    };
    let correlation_id = serde_json::from_str::<ServiceRequest>(line.trim())
        .ok()
        .and_then(|request| request_correlation_id(&request.args));
    let response = route_request(line.trim(), &context).unwrap_or_else(|error| {
        let message = error.to_string();
        log_service_error(format!("client-core-service request failed: {message}"));
        error_state_json(message)
    });
    let response = response_with_correlation_id(&response, correlation_id.as_deref());
    stream.write_all(response.as_bytes())?;
    stream.write_all(b"\n")?;
    stream.flush()?;
    Ok(())
}

fn request_is_watch(line: &str) -> bool {
    serde_json::from_str::<ServiceRequest>(line)
        .ok()
        .map(|request| LocalServiceMethod::parse(&request.method))
        .is_some_and(|method| {
            matches!(
                method,
                LocalServiceMethod::LocalStateWatch | LocalServiceMethod::LocalBusinessEventWatch
            )
        })
}

fn spawn_startup_network_activation(
    runtime: RuntimeActorHandle,
    state_notifier: Arc<RuntimeEventHub>,
) {
    let should_activate = runtime.snapshot().state.signed_in;
    if !should_activate {
        return;
    }
    thread::spawn(move || {
        let transition = prepare_latest_runtime_network_activation(&runtime);
        let correlation_id = transition
            .prepared
            .as_ref()
            .ok()
            .and_then(|plan| plan.session.device_id.clone());
        let state = execute_runtime_network_activation(
            &runtime,
            "mqtt.network.reconcile.commit",
            correlation_id,
            transition,
        )
        .unwrap_or_else(|error| state_with_error(&runtime.snapshot().state, error.to_string()));
        if state.error.is_none() {
            report_runtime_state(&state);
        }
        let business_type = state
            .error
            .as_ref()
            .filter(|value| !value.trim().is_empty())
            .map(|_| BUSINESS_NETWORK_SWITCH_FAILED)
            .unwrap_or(BUSINESS_NETWORK_RUNTIME_CHANGED);
        publish_state_business_event(&state_notifier, business_type, &state);
        if let Some(error) = state.error.filter(|value| !value.trim().is_empty()) {
            log_service_error(format!(
                "client-core-service startup network activation failed: {error}"
            ));
        }
    });
}

fn route_request(line: &str, context: &LocalServiceContext) -> Result<String> {
    let request: ServiceRequest = serde_json::from_str(line).context("decode service request")?;
    let method = LocalServiceMethod::parse(&request.method);
    let correlation_id = request_correlation_id(&request.args);
    match method {
        LocalServiceMethod::LocalStateWatch => {
            return handle_watch_state(request, &context.runtime, &context.state_notifier)
        }
        LocalServiceMethod::LocalBusinessEventWatch => {
            return handle_watch_business_event(request, &context.runtime, &context.state_notifier)
        }
        LocalServiceMethod::LocalState => return handle_state_snapshot(&context.runtime),
        LocalServiceMethod::LocalStatus => {
            return handle_local_status(&context.runtime, &context.request_metrics)
        }
        LocalServiceMethod::LocalSession => return handle_local_session(),
        LocalServiceMethod::LocalPeers => return handle_local_peers(),
        LocalServiceMethod::LocalNetworkModule => {
            return serde_json::to_string(&crate::network_module::network_module_snapshot())
                .context("encode local network module");
        }
        LocalServiceMethod::LocalResolverState => return handle_local_resolver_state(),
        LocalServiceMethod::LocalResolverResolve => return handle_local_resolver_resolve(request),
        LocalServiceMethod::LocalPathPlan => return handle_local_path_plan(),
        LocalServiceMethod::LocalPathDiagnose => return handle_path_diagnose(&context.runtime),
        LocalServiceMethod::LocalRelayCandidates => {
            return handle_relay_candidates(&context.runtime, false)
        }
        LocalServiceMethod::LocalRefreshRelayCandidates => {
            return handle_relay_candidates(&context.runtime, true)
        }
        LocalServiceMethod::LocalSetRelayTransportAllowlist => {
            return handle_set_relay_transport_allowlist(request)
        }
        LocalServiceMethod::LocalRelayPrepare => {
            return handle_prepare_relay_data_plane(&context.runtime)
        }
        LocalServiceMethod::LocalControlStatus => {
            return serde_json::to_string(&control_transport_status()?)
                .context("encode local control status")
        }
        LocalServiceMethod::LocalEnsureDevice => {
            return serde_json::to_string(&handle_local_ensure_device(&context.runtime)?)
                .context("encode local ensure device")
        }
        LocalServiceMethod::LocalConnectControlMqtt => {
            return serde_json::to_string(&handle_local_connect_control_mqtt(context)?)
                .context("encode local mqtt connect status")
        }
        LocalServiceMethod::LocalControlPlan => {
            return serde_json::to_string(&control_transport_plan()?)
                .context("encode local control plan")
        }
        LocalServiceMethod::LocalControlCadence => {
            return serde_json::to_string(&control_transport_cadence())
                .context("encode local control cadence")
        }
        LocalServiceMethod::LocalControlTickPlan => {
            return serde_json::to_string(&control_transport_tick_plan(request.args)?)
                .context("encode local control tick plan")
        }
        LocalServiceMethod::LocalPendingControlAcks => {
            return handle_pending_control_acks(&context.task_queue)
        }
        LocalServiceMethod::LocalMarkControlAcked => {
            return handle_mark_control_acked(request, &context.task_queue)
        }
        LocalServiceMethod::LocalControlOutbox => {
            return handle_control_transport_outbox(request, &context.runtime, &context.task_queue)
        }
        LocalServiceMethod::LocalMarkTransportPublished => {
            return handle_mark_transport_published(request, &context.runtime, &context.task_queue)
        }
        LocalServiceMethod::LocalPlatformNetworkConfig => {
            return handle_local_platform_network_config(&context.runtime)
        }
        LocalServiceMethod::LocalNetworkActivate => {
            let response = handle_local_network_activate(&context.runtime, correlation_id.clone())?;
            publish_method_business_event(&context.state_notifier, line, &response);
            return Ok(response);
        }
        LocalServiceMethod::LocalNetworkDeactivate | LocalServiceMethod::LocalNetworkShutdown => {
            let state = handle_network_deactivate(
                &context.runtime,
                format!("local.{}", request.method),
                correlation_id.clone(),
            )?;
            let response = serde_json::to_string(&state).context("encode disabled client state")?;
            publish_method_business_event(&context.state_notifier, line, &response);
            return Ok(response);
        }
        LocalServiceMethod::LocalLogout => {
            let state =
                handle_local_logout(&context.runtime, "local.logout", correlation_id.clone())?;
            let response =
                serde_json::to_string(&state).context("encode logged out client state")?;
            publish_method_business_event(&context.state_notifier, line, &response);
            return Ok(response);
        }
        LocalServiceMethod::Dispatch => {
            let command_type = request.args.get("type").and_then(Value::as_str);
            let should_wake_control_mqtt = matches!(
                command_type,
                Some("openClientLogin" | "loginWithPassword" | "applyDeviceUserLogin")
            );
            let should_wait_for_control_mqtt = command_type == Some("openClientLogin");
            let response = handle_dispatch_request(request, &context.runtime)?;
            if should_wake_control_mqtt {
                control_transport_worker::wake_control_transport_worker(
                    &context.runtime,
                    &context.task_queue,
                    &context.transport_worker_state,
                    &context.state_notifier,
                );
            }
            if should_wait_for_control_mqtt
                && !control_transport_worker::wait_until_connected(
                    &context.transport_worker_state,
                    Duration::from_secs(5),
                )
            {
                anyhow::bail!("device MQTT subscription was not ready before browser login");
            }
            publish_method_business_event(&context.state_notifier, line, &response);
            return Ok(response);
        }
        LocalServiceMethod::IngestPlatformRuntimeState => {
            return handle_ingest_platform_runtime_state(
                request,
                &context.runtime,
                &context.state_notifier,
            )
        }
        LocalServiceMethod::LocalRegisterTestUser => return handle_register_test_user(request),
        LocalServiceMethod::LocalAcceptNetworkInvite => {
            return handle_accept_network_invite(request, &context.runtime)
        }
        LocalServiceMethod::LocalSendClientMessage => return handle_send_client_message(request),
        LocalServiceMethod::LocalDiagnosticsExport => {
            return handle_export_diagnostics(&context.runtime)
        }
        LocalServiceMethod::Start => {
            sync_control_assignment(&context.runtime);
            mark_control_sync(&context.sync_throttle);
            let response = handle_start(&context.runtime, correlation_id.clone())?;
            publish_method_business_event(&context.state_notifier, line, &response);
            return Ok(response);
        }
        LocalServiceMethod::Refresh => {
            if should_sync_control(&context.sync_throttle) {
                sync_control_assignment(&context.runtime);
            }
            let response = handle_runtime_refresh(
                &context.runtime,
                "local.refresh.commit",
                correlation_id.clone(),
            )?;
            publish_method_business_event(&context.state_notifier, line, &response);
            return Ok(response);
        }
        LocalServiceMethod::EnqueueControlTask => {
            let response = handle_task_request(request, &context.runtime, &context.task_queue)?;
            publish_control_sync_event(&context.state_notifier, method);
            return Ok(response);
        }
        LocalServiceMethod::EnqueueDownstreamControlTask => {
            let response =
                handle_downstream_task_request(request, &context.runtime, &context.task_queue)?;
            publish_control_sync_event(&context.state_notifier, method);
            return Ok(response);
        }
        LocalServiceMethod::IngestDownstreamControlMessage => {
            let response =
                handle_downstream_control_message(request, &context.runtime, &context.task_queue)?;
            publish_control_sync_event(&context.state_notifier, method);
            return Ok(response);
        }
        _ => {}
    }
    let should_notify = method.should_notify();
    let response = handle_request(request, &context.runtime)?;
    if should_notify {
        publish_method_business_event(&context.state_notifier, line, &response);
    }
    Ok(response)
}

fn publish_control_sync_event(state_notifier: &Arc<RuntimeEventHub>, method: LocalServiceMethod) {
    if let Some(method) = method.control_sync_event_method() {
        publish_business_event(
            state_notifier,
            BUSINESS_CONTROL_SYNC_CHANGED,
            serde_json::json!({ "method": method }),
        );
    }
}

fn handle_state_snapshot(runtime: &RuntimeActorHandle) -> Result<String> {
    let mut state = runtime.snapshot().state;
    merge_persisted_client_message_into_state(&mut state);
    enrich_signal_quality(&mut state);
    serde_json::to_string(&state).context("encode client state")
}

fn enrich_signal_quality(state: &mut ClientViewState) {
    if !state.network_enabled {
        state.signal_score = Some(0);
        state.signal_quality = Some("offline".to_string());
        state.signal_path = None;
        return;
    }
    let platform_path = platform_transition::snapshot().runtime_state.active_path;
    let stats = load_relay_runtime_stats();
    let path = local_status_active_path(
        platform_path.as_ref(),
        stats.as_ref().and_then(|value| value.active_path.clone()),
    )
    .and_then(|value| value.as_str().map(str::to_string));
    let loss = stats
        .as_ref()
        .and_then(crate::relay_store::runtime_packet_loss_ppm);
    let rtt = stats.as_ref().and_then(|stats| {
        runtime_relay_candidates()
            .iter()
            .find(|candidate| candidate.address == stats.relay_address)
            .and_then(|candidate| candidate.observed_rtt_ms_hint)
    });
    let assessment = assess_signal_quality(true, path.as_deref(), rtt, loss);
    state.signal_score = Some(assessment.score);
    state.signal_quality = Some(assessment.quality);
    state.signal_path = path;
}

fn handle_accept_network_invite(
    request: ServiceRequest,
    runtime: &RuntimeActorHandle,
) -> Result<String> {
    let invite_code = request
        .args
        .get("inviteCode")
        .and_then(Value::as_str)
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .context("接入码不能为空")?;
    let session = load_session().context("当前未登录")?;
    let device_id = session.device_id.as_deref().context("当前设备未注册")?;
    let value = ControlPlaneClient::from_env().accept_network_invite(
        &session.access_token,
        invite_code,
        device_id,
    )?;
    sync_control_assignment(runtime);
    Ok(serde_json::to_string(&value).context("encode accepted network invite")?)
}

fn handle_local_status(
    runtime: &RuntimeActorHandle,
    request_metrics: &LocalRequestMetrics,
) -> Result<String> {
    let mut state = runtime.snapshot().state;
    merge_persisted_client_message_into_state(&mut state);
    let session = load_session().ok();
    let actor = runtime.diagnostics();
    let snapshots = runtime.snapshots().diagnostics();
    let events = runtime.events().diagnostics();
    let platform = platform_transition::diagnostics();
    let platform_snapshot = platform_transition::snapshot();
    let local_requests = request_metrics.diagnostics();
    let network_snapshot = crate::network_module::network_module_snapshot();
    let active_path = local_status_active_path(
        platform_snapshot.runtime_state.active_path.as_ref(),
        load_relay_runtime_stats().and_then(|stats| stats.active_path),
    );
    let runtime_error = platform.stalled.then(|| {
        format!(
            "platform transition stalled: {}",
            platform.active_kind.as_deref().unwrap_or("unknown")
        )
    });
    let response = LocalStatusResponse {
        service: "client-core-service".to_string(),
        version: env!("CARGO_PKG_VERSION").to_string(),
        signed_in: state.signed_in,
        device_id: state.device_id,
        self_node_id: session
            .as_ref()
            .and_then(|session| session.self_node_id.clone()),
        active_network_id: session
            .as_ref()
            .and_then(|session| session.active_network_id.clone()),
        virtual_ip: state.virtual_ip,
        network_enabled: state.network_enabled,
        switch_enabled: state.switch_enabled,
        syncing: state.syncing,
        sync_reason: state.sync_reason,
        active_path,
        peer_count: network_snapshot.peer_count,
        relay_candidate_count: runtime_relay_candidates().len(),
        connect_plan_count: load_recent_connect_plans(current_timestamp_ms()).len(),
        local_request_concurrency_limit: LOCAL_REQUEST_CONCURRENCY_LIMIT,
        local_watch_concurrency_limit: LOCAL_WATCH_CONCURRENCY_LIMIT,
        local_request_active: local_requests.active,
        local_watch_active: local_requests.active_watches,
        local_request_accepted_total: local_requests.accepted_total,
        local_request_completed_total: local_requests.completed_total,
        local_request_rejected_total: local_requests.rejected_total,
        local_watch_accepted_total: local_requests.watch_accepted_total,
        local_watch_rejected_total: local_requests.watch_rejected_total,
        runtime_command_queue_capacity: actor.queue_capacity,
        runtime_command_queue_depth: actor.queue_depth,
        runtime_command_running: actor.running,
        runtime_command_accepted_total: actor.accepted_total,
        runtime_command_completed_total: actor.completed_total,
        runtime_command_failed_total: actor.failed_total,
        runtime_command_rejected_total: actor.rejected_total,
        runtime_command_timed_out_total: actor.timed_out_total,
        runtime_command_cancelled_before_start_total: actor.cancelled_before_start_total,
        runtime_active_command_id: actor.active_command.as_ref().map(|value| value.command_id),
        runtime_active_command_kind: actor
            .active_command
            .as_ref()
            .map(|value| value.kind.clone()),
        runtime_active_command_correlation_id: actor
            .active_command
            .as_ref()
            .and_then(|value| value.correlation_id.clone()),
        runtime_active_command_queued_at_ms: actor
            .active_command
            .as_ref()
            .map(|value| value.queued_at_ms),
        runtime_active_command_started_at_ms: actor
            .active_command
            .as_ref()
            .map(|value| value.started_at_ms),
        runtime_active_command_caller_timed_out: actor
            .active_command
            .as_ref()
            .map(|value| value.caller_timed_out),
        runtime_last_command_id: actor.last_command.as_ref().map(|value| value.command_id),
        runtime_last_command_kind: actor.last_command.as_ref().map(|value| value.kind.clone()),
        runtime_last_command_correlation_id: actor
            .last_command
            .as_ref()
            .and_then(|value| value.correlation_id.clone()),
        runtime_last_command_queued_at_ms: actor
            .last_command
            .as_ref()
            .map(|value| value.queued_at_ms),
        runtime_last_command_started_at_ms: actor
            .last_command
            .as_ref()
            .and_then(|value| value.started_at_ms),
        runtime_last_command_finished_at_ms: actor
            .last_command
            .as_ref()
            .map(|value| value.finished_at_ms),
        runtime_last_command_duration_ms: actor
            .last_command
            .as_ref()
            .and_then(|value| value.duration_ms),
        runtime_last_command_outcome: actor
            .last_command
            .as_ref()
            .map(|value| value.outcome.clone()),
        runtime_last_command_caller_timed_out: actor
            .last_command
            .as_ref()
            .map(|value| value.caller_timed_out),
        runtime_snapshot_revision: snapshots.revision,
        runtime_snapshot_publish_attempt_total: snapshots.publish_attempt_total,
        runtime_snapshot_changed_total: snapshots.changed_total,
        runtime_snapshot_read_total: snapshots.read_total,
        runtime_snapshot_wait_total: snapshots.wait_total,
        runtime_snapshot_wait_timeout_total: snapshots.wait_timeout_total,
        event_stream_id: events.stream_id,
        event_queue_capacity: events.queue_capacity,
        event_queue_depth: events.queue_depth,
        event_dedup_capacity: events.dedup_capacity,
        event_dedup_entries: events.dedup_entries,
        event_oldest_available_revision: events.oldest_available_revision,
        event_latest_revision: events.latest_revision,
        event_published_total: events.published_total,
        event_duplicate_suppressed_total: events.duplicate_suppressed_total,
        event_evicted_total: events.evicted_total,
        event_read_total: events.read_total,
        event_delivered_total: events.delivered_total,
        event_wait_total: events.wait_total,
        event_wait_timeout_total: events.wait_timeout_total,
        event_replay_gap_total: events.replay_gap_total,
        platform_transition_running: platform.running,
        platform_transition_queue_depth: platform.queue_depth,
        platform_transition_accepted_total: platform.accepted_total,
        platform_transition_completed_total: platform.completed_total,
        platform_transition_failed_total: platform.failed_total,
        platform_transition_rejected_total: platform.rejected_total,
        platform_transition_timed_out_total: platform.timed_out_total,
        platform_transition_cancelled_before_start_total: platform.cancelled_before_start_total,
        platform_transition_active_operation_id: platform.active_operation_id,
        platform_transition_active_kind: platform.active_kind,
        platform_transition_active_correlation_id: platform.active_correlation_id,
        platform_transition_active_caller_timed_out: platform.active_caller_timed_out,
        platform_transition_active_started_at_ms: platform.active_started_at_ms,
        platform_transition_active_duration_ms: platform.active_duration_ms,
        platform_transition_stalled: platform.stalled,
        platform_transition_last_operation_id: platform.last_operation_id,
        platform_transition_last_kind: platform.last_kind,
        platform_transition_last_correlation_id: platform.last_correlation_id,
        platform_transition_last_caller_timed_out: platform.last_caller_timed_out,
        platform_transition_last_started_at_ms: platform.last_started_at_ms,
        platform_transition_last_finished_at_ms: platform.last_finished_at_ms,
        platform_transition_last_duration_ms: platform.last_duration_ms,
        platform_transition_last_error: platform.last_error,
        platform_transition_last_timed_out_operation_id: platform.last_timed_out_operation_id,
        platform_transition_last_timed_out_kind: platform.last_timed_out_kind,
        platform_transition_last_timed_out_correlation_id: platform.last_timed_out_correlation_id,
        platform_transition_late_completion_total: platform.late_completion_total,
        platform_transition_late_completion_buffered: platform.late_completion_buffered,
        platform_transition_late_completion_evicted_total: platform.late_completion_evicted_total,
        platform_transition_last_late_completion_operation_id: platform
            .last_late_completion_operation_id,
        platform_transition_last_late_completion_kind: platform.last_late_completion_kind,
        platform_transition_last_late_completion_correlation_id: platform
            .last_late_completion_correlation_id,
        platform_transition_last_late_completion_succeeded: platform.last_late_completion_succeeded,
        platform_runtime_state_available: platform_snapshot.runtime_state_available,
        platform_runtime_state_updated_at_ms: platform_snapshot.runtime_state_updated_at_ms,
        platform_runtime_state_age_ms: platform_snapshot.runtime_state_age_ms,
        platform_runtime_state_last_error: platform_snapshot.runtime_state_last_error,
        platform_diagnostics_updated_at_ms: platform_snapshot.platform_diagnostics_updated_at_ms,
        platform_diagnostics_age_ms: platform_snapshot.platform_diagnostics_age_ms,
        platform_diagnostics_last_error: platform_snapshot.platform_diagnostics_last_error,
        error: state.error,
        runtime_error,
    };
    serde_json::to_string(&response).context("encode local status")
}

fn local_status_active_path(
    platform_path: Option<&client_core::PathKind>,
    relay_path: Option<String>,
) -> Option<Value> {
    platform_path
        .and_then(|path| serde_json::to_value(path).ok())
        .or_else(|| relay_path.map(Value::String))
}

fn handle_local_resolver_state() -> Result<String> {
    let network_snapshot = runtime_network_state_store().snapshot();
    let network = Some(network_snapshot.state);
    let resolver = resolver_runtime_state()
        .lock()
        .ok()
        .map(|guard| guard.clone());
    let server = local_resolver_server_status()
        .try_lock()
        .ok()
        .map(|guard| guard.clone())
        .unwrap_or_default();
    serde_json::to_string(&serde_json::json!({
        "networkStateRevision": network_snapshot.revision,
        "activeNetworkId": resolver.as_ref().and_then(|value| value.active_network_id().map(str::to_string)),
        "selfDeviceId": network.as_ref().and_then(|value| value.self_device_id.clone()),
        "selfVirtualIp": network.as_ref().and_then(|value| value.self_virtual_ip.clone()),
        "zoneCount": resolver.as_ref().map(|value| value.zone_count()).unwrap_or(0),
        "recordCount": resolver.as_ref().map(|value| value.record_count()).unwrap_or(0),
        "cacheCount": resolver.as_ref().map(|value| value.cache_count()).unwrap_or(0),
        "memberCount": network.as_ref().map(|value| value.members_by_device_id.len()).unwrap_or(0),
        "syncStatus": network
            .as_ref()
            .map(|value| format!("{:?}", value.sync_status))
            .unwrap_or_else(|| "Busy".to_string()),
        "lastReloadAtMs": resolver.as_ref().and_then(|value| value.last_reload_at_ms),
        "upstreamServers": resolver
            .as_ref()
            .map(|value| value.effective_upstream_resolvers())
            .unwrap_or_default(),
        "searchDomains": resolver
            .as_ref()
            .map(|value| value.config.search_domains.clone())
            .unwrap_or_default(),
        "splitDomains": resolver
            .as_ref()
            .map(|value| value.config.split_domains.clone())
            .unwrap_or_default(),
        "fallbackToSystemResolvers": resolver
            .as_ref()
            .map(|value| value.config.fallback_to_system_resolvers)
            .unwrap_or(false),
        "serverEnabled": server.enabled,
        "serverListening": server.listening,
        "serverBindAddr": server.bind_addr,
        "serverRequesterDeviceId": server.requester_device_id,
        "serverLastError": server.last_error,
        "serverLastQueryAtMs": server.last_query_at_ms,
        "desiredEnabled": server.desired_enabled,
        "desiredSignedIn": server.desired_signed_in,
        "desiredNetworkEnabled": server.desired_network_enabled,
        "desiredHasRequesterDeviceId": server.desired_has_requester_device_id,
        "desiredHasResolverData": server.desired_has_resolver_data,
        "networkLockBusy": false,
        "resolverLockBusy": false,
    }))
    .context("encode local resolver state")
}

fn handle_local_resolver_resolve(request: ServiceRequest) -> Result<String> {
    let input: LocalResolverResolveRequest =
        serde_json::from_value(request.args).context("decode local resolver resolve request")?;
    let session = load_session().ok();
    let network = runtime_network_state_store()
        .snapshot_for_session(session.as_ref())
        .state;
    let mut resolver = resolver_runtime_state().lock().map_err(|_| {
        anyhow::anyhow!("network resolver runtime busy: resolver state lock unavailable")
    })?;
    let payload = resolve_authoritative_result_json(resolve_authoritative(
        &network,
        &mut resolver,
        &input.requester_device_id,
        &input.qname,
        &input.qtype,
    ));
    serde_json::to_string(&payload).context("encode local resolver resolve response")
}

fn last_client_message_file_path() -> std::path::PathBuf {
    app_data_dir()
        .join("SLAN")
        .join("client-v2-last-client-message.json")
}

pub(crate) fn persist_last_client_message_payload(value: &Value) -> Result<()> {
    let path = last_client_message_file_path();
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent).with_context(|| format!("create {}", parent.display()))?;
    }
    let payload = serde_json::to_vec_pretty(value).context("encode last client message")?;
    fs::write(&path, payload).with_context(|| format!("write {}", path.display()))?;
    Ok(())
}

pub(crate) fn merge_persisted_client_message_into_state(state: &mut ClientViewState) {
    if state.last_client_message_id.is_some()
        && state.last_client_message_from_device_id.is_some()
        && state.last_client_message_body.is_some()
    {
        return;
    }
    let Ok(payload) = fs::read_to_string(last_client_message_file_path()) else {
        return;
    };
    let Ok(value) = serde_json::from_str::<Value>(&payload) else {
        return;
    };
    let payload = value.get("payload").unwrap_or(&value);
    if state.last_client_message_id.is_none() {
        state.last_client_message_id = payload
            .get("messageId")
            .or_else(|| payload.get("message_id"))
            .and_then(Value::as_str)
            .map(str::to_string);
    }
    if state.last_client_message_from_device_id.is_none() {
        state.last_client_message_from_device_id = payload
            .get("fromDeviceId")
            .or_else(|| payload.get("from_device_id"))
            .and_then(Value::as_str)
            .map(str::to_string);
    }
    if state.last_client_message_body.is_none() {
        state.last_client_message_body = payload
            .get("body")
            .and_then(Value::as_str)
            .map(str::to_string);
    }
}

fn handle_local_session() -> Result<String> {
    let session = load_session().ok();
    let response = match session {
        Some(session) => {
            let expires_at_ms = session.expires_in.map(|seconds| {
                session
                    .authenticated_at_ms
                    .saturating_add(seconds.saturating_mul(1000))
            });
            LocalSessionResponse {
                signed_in: session.session_kind == "user"
                    && !session.access_token.trim().is_empty(),
                expired: session_is_expired(&session),
                user_id: Some(session.user_id),
                user_label: Some(session.user_label),
                device_id: session.device_id,
                self_node_id: session.self_node_id,
                active_network_id: session.active_network_id,
                virtual_ip: session.virtual_ip,
                relay_candidate_count: runtime_relay_candidates().len(),
                mqtt_configured: session.mqtt.is_some(),
                expires_in: session.expires_in,
                authenticated_at_ms: Some(session.authenticated_at_ms),
                expires_at_ms,
            }
        }
        None => LocalSessionResponse {
            signed_in: false,
            expired: false,
            user_id: None,
            user_label: None,
            device_id: None,
            self_node_id: None,
            active_network_id: None,
            virtual_ip: None,
            relay_candidate_count: 0,
            mqtt_configured: false,
            expires_in: None,
            authenticated_at_ms: None,
            expires_at_ms: None,
        },
    };
    serde_json::to_string(&response).context("encode local session")
}

fn handle_local_peers() -> Result<String> {
    let runtime_state = platform_transition::snapshot().runtime_state;
    let items = runtime_state
        .peer_paths
        .into_iter()
        .map(|peer| LocalPeerView {
            peer_node_id: peer.peer_node_id,
            peer_virtual_ips: peer.peer_virtual_ips,
            active_path: peer
                .active_path
                .and_then(|path| serde_json::to_value(path).ok()),
            candidates: peer
                .candidates
                .into_iter()
                .filter_map(|candidate| serde_json::to_value(candidate).ok())
                .collect(),
        })
        .collect();
    serde_json::to_string(&LocalPeersResponse { items }).context("encode local peers")
}

fn handle_local_path_plan() -> Result<String> {
    let items = load_recent_connect_plans(current_timestamp_ms())
        .into_iter()
        .filter_map(|plan| serde_json::to_value(plan).ok())
        .collect();
    serde_json::to_string(&LocalPathPlanResponse { items }).context("encode local path plan")
}

fn handle_request(request: ServiceRequest, _runtime: &RuntimeActorHandle) -> Result<String> {
    let method = LocalServiceMethod::parse(&request.method);
    if method == LocalServiceMethod::ConsoleLoginKey {
        return serde_json::to_string(&console_login_key()?).context("encode console login key");
    }
    let request_method = request.method.clone();
    match method {
        LocalServiceMethod::Other => anyhow::bail!("unsupported service method {request_method}"),
        _ => anyhow::bail!(
            "service method {} is handled outside the runtime command interface",
            request_method
        ),
    }
}

fn prepare_runtime_refresh(signed_in: bool) -> Result<Option<NetworkRuntimeState>> {
    if !signed_in {
        return Ok(None);
    }
    let snapshot = platform_transition::snapshot();
    Ok(snapshot
        .runtime_state_available
        .then_some(snapshot.runtime_state))
}

fn apply_prepared_runtime_refresh<P>(
    runtime: &mut ClientRuntime<P>,
    prepared: Result<Option<NetworkRuntimeState>>,
) -> ClientViewState
where
    P: client_core::PlatformNetwork,
{
    let current = runtime.state();
    let preserve_enabled_network = current.signed_in
        && current.network_enabled
        && prepared
            .as_ref()
            .ok()
            .and_then(Option::as_ref)
            .is_none_or(|state| !state.network_enabled);
    if preserve_enabled_network {
        log_service_error(format!(
            "client-core-service ignored transient disabled platform runtime snapshot: current_ip={:?}",
            current.virtual_ip
        ));
        return current.clone();
    }
    match prepared {
        Ok(Some(state)) => runtime
            .dispatch(ClientCommand::ApplyPlatformRuntimeState(state))
            .unwrap_or_else(|error| state_with_error(runtime.state(), error.to_string())),
        Ok(None) => runtime
            .dispatch(ClientCommand::ApplyPlatformRuntimeState(
                NetworkRuntimeState::default(),
            ))
            .unwrap_or_else(|error| state_with_error(runtime.state(), error.to_string())),
        Err(error) => state_with_error(runtime.state(), error.to_string()),
    }
}

fn commit_runtime_refresh(
    runtime: &RuntimeActorHandle,
    command_kind: impl Into<String>,
    correlation_id: Option<String>,
) -> Result<ClientViewState> {
    let snapshot = runtime.snapshot().state;
    let expected_device_id = snapshot.device_id.clone();
    let expected_signed_in = snapshot.signed_in;
    let prepared = prepare_runtime_refresh(snapshot.signed_in);
    runtime.call_named(command_kind, correlation_id, move |runtime| {
        if !runtime_login_context_matches(
            runtime,
            expected_device_id.as_deref(),
            expected_signed_in,
        ) {
            log_service_error("client-core-service ignored stale platform runtime refresh");
            return Ok(runtime.state().clone());
        }
        Ok(apply_prepared_runtime_refresh(runtime, prepared))
    })
}

fn handle_runtime_refresh(
    runtime: &RuntimeActorHandle,
    command_kind: impl Into<String>,
    correlation_id: Option<String>,
) -> Result<String> {
    let state = commit_runtime_refresh(runtime, command_kind, correlation_id)?;
    serde_json::to_string(&state).context("encode refreshed client state")
}

fn handle_start(
    runtime: &RuntimeActorHandle,
    request_correlation_id: Option<String>,
) -> Result<String> {
    let snapshot = runtime.snapshot().state;
    let expected_device_id = snapshot.device_id.clone();
    let expected_signed_in = snapshot.signed_in;
    let prepared = load_valid_registered_session();
    let user_session_ready = prepared.as_ref().is_some_and(|session| {
        session.session_kind == "user" && !session.access_token.trim().is_empty()
    });
    let prepared_runtime = prepare_runtime_refresh(user_session_ready || snapshot.signed_in);
    if !user_session_ready {
        if let Err(error) = disable_platform_network_serialized() {
            log_service_error(format!(
                "client-core-service startup platform disable failed: {error:#}"
            ));
        }
    }
    let correlation_id = request_correlation_id.or_else(|| {
        prepared
            .as_ref()
            .and_then(|session| session.device_id.clone())
    });
    let state = runtime.call_named("local.start.commit", correlation_id, move |runtime| {
        if runtime_login_context_matches(runtime, expected_device_id.as_deref(), expected_signed_in)
        {
            match prepared {
                Some(session)
                    if session.session_kind == "user"
                        && !session.access_token.trim().is_empty() =>
                {
                    runtime.dispatch(ClientCommand::ApplyDeviceUserLogin(session.into()))?;
                }
                Some(session) if session.session_kind == "device" => {
                    runtime.apply_logout_state();
                }
                None => {
                    runtime.apply_logout_state();
                }
                Some(_) => {}
            }
        } else {
            log_service_error("client-core-service ignored stale startup session result");
            return Ok(runtime.state().clone());
        }
        Ok(apply_prepared_runtime_refresh(runtime, prepared_runtime))
    })?;
    serde_json::to_string(&state).context("encode started client state")
}

fn handle_local_platform_network_config(runtime: &RuntimeActorHandle) -> Result<String> {
    let expected_revision = runtime.snapshot().revision;
    let prepared = prepare_platform_network_config_from_latest_control()?;
    let correlation_id = prepared.prepared_session.session.device_id.clone();
    let PreparedMobilePlatformNetworkConfig {
        config,
        prepared_session,
        assigned_ip,
    } = prepared;
    let config = runtime
        .call_named_if_revision(
            "local.network.platform_config.commit",
            correlation_id,
            expected_revision,
            move |runtime| {
                ensure_runtime_session_matches(runtime, &prepared_session.session)?;
                commit_registered_session(runtime, &prepared_session)?;
                runtime.apply_assigned_ip_state(assigned_ip);
                Ok(config)
            },
        )?
        .ok_or_else(|| anyhow::anyhow!("stale mobile platform network config"))?;
    serde_json::to_string(&config).context("encode local platform network config")
}

fn handle_local_network_activate(
    runtime: &RuntimeActorHandle,
    request_correlation_id: Option<String>,
) -> Result<String> {
    let transition = prepare_latest_runtime_network_activation(runtime);
    let correlation_id = request_correlation_id.or_else(|| {
        transition
            .prepared
            .as_ref()
            .ok()
            .and_then(|plan| plan.session.device_id.clone())
    });
    let state = execute_runtime_network_activation(
        runtime,
        "local.network.activate.commit",
        correlation_id,
        transition,
    )?;
    if state.error.is_none() {
        report_runtime_state(&state);
    }
    serde_json::to_string(&state).context("encode activated client state")
}

fn normalized_device_id(value: Option<&str>) -> Option<&str> {
    value.map(str::trim).filter(|value| !value.is_empty())
}

fn runtime_login_context_matches<P>(
    runtime: &ClientRuntime<P>,
    expected_device_id: Option<&str>,
    expected_signed_in: bool,
) -> bool
where
    P: client_core::PlatformNetwork,
{
    runtime.state().signed_in == expected_signed_in
        && normalized_device_id(runtime.state().device_id.as_deref())
            == normalized_device_id(expected_device_id)
}

fn prepare_password_login(payload: PasswordLoginPayload) -> Result<PreparedSession> {
    let auth_payload = ControlPlaneClient::from_env()
        .login_with_password(&payload.email, &payload.password)
        .context("password login")?;
    match prepare_session_from_control_plane(auth_payload.clone()) {
        Ok(session) => Ok(session),
        Err(hydrate_error) => {
            log_service_error(format!(
                "client-core-service password login hydrate failed; retry register device: {hydrate_error:#}"
            ));
            let recovered = PersistedSession::from(auth_payload);
            match prepare_session_device_registered(recovered.clone()) {
                Ok(prepared) => Ok(prepared),
                Err(register_error) => {
                    log_service_error(format!(
                        "client-core-service password login device registration recovery failed: {register_error:#}"
                    ));
                    Ok(PreparedSession::from_session(recovered))
                }
            }
        }
    }
}

fn prepare_device_user_login(payload: AuthPayload) -> Result<PreparedSession> {
    match prepare_session_from_control_plane(payload.clone()) {
        Ok(session) => Ok(session),
        Err(hydrate_error) => {
            log_service_error(format!(
                "client-core-service apply device user login hydrate failed; retry register device: {hydrate_error:#}"
            ));
            let recovered = PersistedSession::from(payload);
            match prepare_session_device_registered(recovered.clone()) {
                Ok(prepared) => Ok(prepared),
                Err(register_error) => {
                    log_service_error(format!(
                        "client-core-service apply device user login recovery failed: {register_error:#}"
                    ));
                    Ok(PreparedSession::from_session(recovered))
                }
            }
        }
    }
}

fn commit_prepared_login<P>(
    runtime: &mut ClientRuntime<P>,
    prepared: Result<PreparedSession>,
    expected_device_id: Option<String>,
    expected_signed_in: bool,
    error_prefix: &str,
) -> ClientViewState
where
    P: client_core::PlatformNetwork,
{
    if !runtime_login_context_matches(runtime, expected_device_id.as_deref(), expected_signed_in) {
        log_service_error("client-core-service ignored stale prepared login result");
        return runtime.state().clone();
    }
    let prepared = match prepared {
        Ok(prepared) => prepared,
        Err(error) => {
            log_service_error(format!(
                "client-core-service login preparation failed: {error:#}"
            ));
            return state_with_error(runtime.state(), format!("{error_prefix}: {error:#}"));
        }
    };
    commit_registered_session(runtime, &prepared)
        .unwrap_or_else(|error| state_with_error(runtime.state(), error.to_string()))
}

fn commit_registered_session<P>(
    runtime: &mut ClientRuntime<P>,
    prepared: &PreparedSession,
) -> Result<ClientViewState>
where
    P: client_core::PlatformNetwork,
{
    persist_session(&prepared.session)?;
    apply_prepared_session_projection(current_session_runtime_epoch(), prepared)?;
    if prepared.session.session_kind == "device" {
        return Ok(runtime.apply_logout_state());
    }
    runtime
        .dispatch(ClientCommand::ApplyDeviceUserLogin(
            prepared.session.clone().into(),
        ))
        .map_err(anyhow::Error::from)
}

fn handle_open_client_login(
    runtime: &RuntimeActorHandle,
    correlation_id: Option<String>,
) -> Result<String> {
    let snapshot = runtime.snapshot().state;
    if !browser_login_requires_preparation(&snapshot) {
        log_service_error(
            "client-core-service kept existing user session for browser login request",
        );
        return serde_json::to_string(&snapshot).context("encode current client login state");
    }
    let expected_device_id = snapshot.device_id.clone();
    let expected_signed_in = snapshot.signed_in;
    let prepared = prepare_client_login_session(std::env::consts::OS);
    let state = runtime.call_named(
        "local.login.browser.commit",
        correlation_id,
        move |runtime| {
            if !runtime_login_context_matches(
                runtime,
                expected_device_id.as_deref(),
                expected_signed_in,
            ) {
                log_service_error("client-core-service ignored stale browser login preparation");
                return Ok(runtime.state().clone());
            }
            let session = match prepared {
                Ok(session) => session,
                Err(error) => {
                    return Ok(state_with_error(
                        runtime.state(),
                        format!("准备客户端登录失败: {error:#}"),
                    ));
                }
            };
            if let Err(error) = persist_session(&session) {
                return Ok(state_with_error(runtime.state(), error.to_string()));
            }
            log_service_error(format!(
                "client-core-service prepared browser login device={} mqttReady=true",
                session.device_id.as_deref().unwrap_or_default()
            ));
            Ok(runtime.request_browser_login(session.device_id))
        },
    )?;
    serde_json::to_string(&state).context("encode browser login client state")
}

fn browser_login_requires_preparation(state: &ClientViewState) -> bool {
    !state.signed_in
}

fn handle_password_login(
    runtime: &RuntimeActorHandle,
    payload: PasswordLoginPayload,
    correlation_id: Option<String>,
) -> Result<String> {
    let snapshot = runtime.snapshot().state;
    let expected_device_id = snapshot.device_id.clone();
    let expected_signed_in = snapshot.signed_in;
    let prepared = prepare_password_login(payload);
    let state = runtime.call_named(
        "local.login.password.commit",
        correlation_id,
        move |runtime| {
            Ok(commit_prepared_login(
                runtime,
                prepared,
                expected_device_id,
                expected_signed_in,
                "登录失败",
            ))
        },
    )?;
    serde_json::to_string(&state).context("encode password login client state")
}

fn handle_device_user_login(
    runtime: &RuntimeActorHandle,
    payload: AuthPayload,
    correlation_id: Option<String>,
) -> Result<String> {
    let snapshot = runtime.snapshot().state;
    let expected_device_id = snapshot.device_id.clone();
    let expected_signed_in = snapshot.signed_in;
    let prepared = if normalized_device_id(expected_device_id.as_deref()).is_some()
        && normalized_device_id(payload.device_id.as_deref())
            != normalized_device_id(expected_device_id.as_deref())
    {
        Err(anyhow::anyhow!("登录设备不匹配"))
    } else {
        prepare_device_user_login(payload)
    };
    let state = runtime.call_named(
        "local.login.device.commit",
        correlation_id,
        move |runtime| {
            Ok(commit_prepared_login(
                runtime,
                prepared,
                expected_device_id,
                expected_signed_in,
                "登录失败",
            ))
        },
    )?;
    serde_json::to_string(&state).context("encode device login client state")
}

fn prepare_platform_network_deactivation(platform: &PlatformNetworkImpl) -> Result<()> {
    platform_transition::disable_network(platform)
}

fn commit_network_deactivation<P>(
    runtime: &mut ClientRuntime<P>,
    prepared: Result<()>,
) -> ClientViewState
where
    P: client_core::PlatformNetwork,
{
    match prepared {
        Ok(()) => {
            let mut state = runtime.apply_network_disabled_state();
            let _ = clear_session_virtual_ip();
            state.virtual_ip = None;
            state
        }
        Err(error) => state_with_error(runtime.state(), error.to_string()),
    }
}

fn handle_network_deactivate(
    runtime: &RuntimeActorHandle,
    command_kind: impl Into<String>,
    correlation_id: Option<String>,
) -> Result<ClientViewState> {
    let expected_revision = runtime.snapshot().revision;
    let snapshots = runtime.snapshots();
    let command_kind = command_kind.into();
    let prepared = platform_transition::run_serialized_correlated(
        command_kind.clone(),
        correlation_id.clone(),
        move |platform| {
            if snapshots.latest().revision != expected_revision {
                anyhow::bail!("stale platform network deactivation");
            }
            prepare_platform_network_deactivation(platform)
        },
    );
    if runtime.snapshot().revision != expected_revision {
        return Ok(runtime.snapshot().state);
    }
    let state = runtime.call_named(command_kind, correlation_id, move |runtime| {
        Ok(commit_network_deactivation(runtime, prepared))
    })?;
    if state.error.is_none() {
        report_runtime_state(&state);
    }
    Ok(state)
}

fn prepare_logout_remote() {
    if let Ok(session) = load_session() {
        revoke_remote_sessions(&session);
    }
    deactivate_control_network();
}

fn commit_logout<P>(
    runtime: &mut ClientRuntime<P>,
    expected_device_id: Option<String>,
    expected_signed_in: bool,
) -> ClientViewState
where
    P: client_core::PlatformNetwork,
{
    if !runtime_login_context_matches(runtime, expected_device_id.as_deref(), expected_signed_in) {
        log_service_error("client-core-service ignored stale logout result");
        return runtime.state().clone();
    }
    if let Err(error) = remove_user_session_preserving_device() {
        return state_with_error(runtime.state(), error.to_string());
    }
    runtime.apply_logout_state()
}

fn handle_local_logout(
    runtime: &RuntimeActorHandle,
    command_kind: impl Into<String>,
    correlation_id: Option<String>,
) -> Result<ClientViewState> {
    let snapshot = runtime.snapshot();
    let expected_revision = snapshot.revision;
    let expected_device_id = snapshot.state.device_id;
    let expected_signed_in = snapshot.state.signed_in;
    let snapshots = runtime.snapshots();
    prepare_logout_remote();
    let command_kind = command_kind.into();
    if let Err(error) = platform_transition::run_serialized_correlated(
        command_kind.clone(),
        correlation_id.clone(),
        move |platform| {
            if snapshots.latest().revision != expected_revision {
                anyhow::bail!("stale platform logout");
            }
            platform_transition::disable_network(platform)
        },
    ) {
        log_service_error(format!(
            "client-core-service logout platform disable failed: {error:#}"
        ));
    }
    if runtime.snapshot().revision != expected_revision {
        return Ok(runtime.snapshot().state);
    }
    let committed = runtime.call_named_if_revision(
        command_kind.clone(),
        correlation_id.clone(),
        expected_revision,
        move |runtime| {
            Ok(commit_logout(
                runtime,
                expected_device_id,
                expected_signed_in,
            ))
        },
    )?;
    match committed {
        Some(state) => Ok(state),
        None => runtime.call_named(format!("{command_kind}.stale"), correlation_id, |runtime| {
            Ok(runtime.apply_network_disabled_state())
        }),
    }
}

fn prepare_assigned_ip_session(payload: &AssignedIpPayload) -> Result<Option<PersistedSession>> {
    let Ok(mut session) = load_session() else {
        return Ok(None);
    };
    if session.access_token.trim().is_empty() && session.device_token.is_none() {
        return Ok(None);
    }
    session.virtual_ip = Some(payload.virtual_ip.clone());
    Ok(Some(session))
}

fn execute_runtime_assigned_ip(
    runtime: &RuntimeActorHandle,
    command_kind: impl Into<String>,
    correlation_id: Option<String>,
    payload: AssignedIpPayload,
) -> Result<ClientViewState> {
    let prepared_session = prepare_assigned_ip_session(&payload)?;
    let snapshot = runtime.snapshot();
    let configure_platform = snapshot.state.network_enabled;
    let command_kind = command_kind.into();
    if !configure_platform {
        return Ok(runtime
            .call_named_if_revision(
                command_kind,
                correlation_id,
                snapshot.revision,
                move |runtime| {
                    if let Some(session) = prepared_session.as_ref() {
                        persist_session(session)?;
                    }
                    Ok(runtime.apply_assigned_ip_state(payload))
                },
            )?
            .unwrap_or_else(|| runtime.snapshot().state));
    }
    let virtual_ip = normalize_virtual_ip(&payload.virtual_ip);
    let prefix_len = payload.prefix_len.unwrap_or(32);
    let expected_revision = snapshot.revision;
    let snapshots = runtime.snapshots();
    let platform_result = if virtual_ip.is_empty() {
        Err(anyhow::anyhow!("assigned virtual IP is empty"))
    } else {
        platform_transition::run_serialized_correlated(
            command_kind.clone(),
            correlation_id.clone(),
            move |platform| {
                if snapshots.latest().revision != expected_revision {
                    anyhow::bail!("stale assigned IP platform transition");
                }
                platform_transition::configure_ip(platform, &virtual_ip, prefix_len).inspect_err(
                    |_| {
                        let _ = platform_transition::disable_network(platform);
                    },
                )
            },
        )
    };
    let committed = runtime.call_named_if_revision(
        command_kind.clone(),
        correlation_id.clone(),
        expected_revision,
        move |runtime| {
            let committed = match platform_result {
                Ok(()) => {
                    if let Some(session) = prepared_session.as_ref() {
                        if let Err(error) = persist_session(session) {
                            (state_with_error(runtime.state(), error.to_string()), true)
                        } else {
                            (runtime.apply_assigned_ip_state(payload), false)
                        }
                    } else {
                        (runtime.apply_assigned_ip_state(payload), false)
                    }
                }
                Err(error) => (state_with_error(runtime.state(), error.to_string()), false),
            };
            Ok(committed)
        },
    )?;
    if let Some((state, rollback_platform)) = committed {
        if rollback_platform {
            let _ = disable_platform_network_serialized();
        }
        return Ok(state);
    }
    let _ = disable_platform_network_serialized();
    runtime.call_named(
        format!("{command_kind}.rollback"),
        correlation_id,
        |runtime| Ok(runtime.apply_network_disabled_state()),
    )
}

fn handle_sync_assigned_ip(
    runtime: &RuntimeActorHandle,
    payload: AssignedIpPayload,
    correlation_id: Option<String>,
) -> Result<String> {
    let state = execute_runtime_assigned_ip(
        runtime,
        "local.dispatch.assigned_ip.commit",
        correlation_id,
        payload,
    )?;
    serde_json::to_string(&state).context("encode assigned IP client state")
}

fn handle_dispatch_request(
    request: ServiceRequest,
    runtime: &RuntimeActorHandle,
) -> Result<String> {
    let correlation_id = ["requestId", "messageId", "eventId", "deliveryId"]
        .into_iter()
        .find_map(|key| {
            request
                .args
                .get(key)
                .and_then(Value::as_str)
                .map(str::trim)
                .filter(|value| !value.is_empty())
                .map(str::to_string)
        });
    let command: ClientCommand =
        serde_json::from_value(request.args).context("decode client command")?;
    match command {
        ClientCommand::EnableNetwork => {
            return handle_local_network_activate(runtime, correlation_id)
        }
        ClientCommand::DisableNetwork => {
            let state = handle_network_deactivate(
                runtime,
                "local.dispatch.network.deactivate",
                correlation_id,
            )?;
            return serde_json::to_string(&state).context("encode disabled client state");
        }
        ClientCommand::Logout => {
            let state = handle_local_logout(runtime, "local.dispatch.logout", correlation_id)?;
            return serde_json::to_string(&state).context("encode logged out client state");
        }
        ClientCommand::OpenClientLogin => return handle_open_client_login(runtime, correlation_id),
        ClientCommand::LoginWithPassword(payload) => {
            return handle_password_login(runtime, payload, correlation_id)
        }
        ClientCommand::ApplyDeviceUserLogin(payload) => {
            return handle_device_user_login(runtime, payload, correlation_id)
        }
        ClientCommand::Refresh => {
            return handle_runtime_refresh(runtime, "local.dispatch.refresh.commit", correlation_id)
        }
        ClientCommand::SyncAssignedIp(payload) => {
            return handle_sync_assigned_ip(runtime, payload, correlation_id)
        }
        command => return handle_regular_dispatch(runtime, command, correlation_id),
    }
}

fn handle_regular_dispatch(
    runtime: &RuntimeActorHandle,
    command: ClientCommand,
    correlation_id: Option<String>,
) -> Result<String> {
    let state = runtime.call_named("local.dispatch", correlation_id, move |runtime| {
        Ok(dispatch_runtime_command(runtime, command))
    })?;
    serde_json::to_string(&state).context("encode dispatched client state")
}

fn handle_ingest_platform_runtime_state(
    request: ServiceRequest,
    runtime: &RuntimeActorHandle,
    state_notifier: &Arc<RuntimeEventHub>,
) -> Result<String> {
    let report: PlatformRuntimeStateReportRequest =
        serde_json::from_value(request.args).context("decode platform runtime state report")?;
    let runtime_details = report.runtime_state.clone();
    let reported_runtime_state: NetworkRuntimeState =
        serde_json::from_value(runtime_details.clone()).context("decode platform runtime state")?;
    let reported_traffic = report.traffic.clone();
    let reported_at_ms = report.reported_at_ms;
    let session_epoch = current_session_runtime_epoch();
    let Ok(_session_guard) = lock_session_runtime_epoch(session_epoch) else {
        return serde_json::to_string(&serde_json::json!({
            "accepted": false,
            "ignored": true,
            "reason": "stale_session",
            "state": runtime.snapshot().state,
        }))
        .context("encode stale platform runtime state ingest response");
    };
    let reported_error = report
        .error
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .map(str::to_string);
    let actor_error = reported_error.clone();
    let state = runtime.call_named("platform.runtime.report", None, move |runtime| {
        let mut state = runtime
            .dispatch(ClientCommand::ApplyPlatformRuntimeState(
                reported_runtime_state,
            ))
            .unwrap_or_else(|error| state_with_error(runtime.state(), error.to_string()));
        if let Some(traffic) =
            platform_traffic_stats_payload(session_epoch, reported_traffic.as_ref(), reported_at_ms)
        {
            state = runtime
                .dispatch(ClientCommand::ApplyTrafficStats(traffic))
                .unwrap_or_else(|error| state_with_error(runtime.state(), error.to_string()));
        }
        if let Some(error) = actor_error {
            state = runtime.set_error(error);
        }
        Ok(state)
    })?;
    publish_business_event(
        state_notifier,
        BUSINESS_NETWORK_RUNTIME_CHANGED,
        serde_json::json!({
            "platform": report.platform,
            "runtimeState": runtime_details,
            "traffic": report.traffic,
            "error": reported_error,
            "reportedAtMs": report.reported_at_ms,
            "state": state,
        }),
    );
    let report_state = state.clone();
    thread::spawn(move || {
        report_platform_runtime(
            session_epoch,
            report_state,
            report.platform,
            runtime_details,
            report.traffic,
            report.reported_at_ms,
        );
    });
    serde_json::to_string(&serde_json::json!({
        "accepted": true,
        "state": state,
    }))
    .context("encode platform runtime state ingest response")
}

fn handle_register_test_user(request: ServiceRequest) -> Result<String> {
    let input: RegisterTestUserRequest =
        serde_json::from_value(request.args).context("decode register test user request")?;
    let auth = ControlPlaneClient::from_env()
        .register_user_with_password(&input.email, &input.password)
        .context("register test user")?;
    serde_json::to_string(&auth).context("encode register test user response")
}

fn control_transport_status() -> Result<ControlTransportStatus> {
    let session = load_session()?;
    Ok(control_transport::control_transport_status(&session))
}

fn control_transport_plan() -> Result<ControlTransportPlan> {
    let session = load_session()?;
    Ok(control_transport::control_transport_plan(&session))
}

fn control_transport_cadence() -> ControlTransportCadence {
    control_transport::control_transport_cadence()
}

fn handle_local_ensure_device(runtime: &RuntimeActorHandle) -> Result<Value> {
    let expected_state = runtime.snapshot().state;
    let prepared = prepare_session_device_registered(load_session()?)?;
    let session = prepared.session.clone();
    let correlation_id = session.device_id.clone();
    runtime.call_named("session.device.ensure", correlation_id, move |runtime| {
        if !runtime_login_context_matches(
            runtime,
            expected_state.device_id.as_deref(),
            expected_state.signed_in,
        ) {
            anyhow::bail!("stale device ensure session");
        }
        commit_registered_session(runtime, &prepared)?;
        Ok(())
    })?;
    Ok(serde_json::json!({
        "registered": session.device_id.as_deref().is_some_and(|value| !value.trim().is_empty()),
        "deviceId": session.device_id,
        "activeNetworkId": session.active_network_id,
        "virtualIp": session.virtual_ip,
        "mqttCredentialReady": session.mqtt.is_some(),
        "mqttExpiresAt": session.mqtt.as_ref().and_then(|credential| credential.expires_at),
        "controlStatus": control_transport::control_transport_status(&session),
    }))
}

fn handle_local_connect_control_mqtt(context: &LocalServiceContext) -> Result<Value> {
    let session = load_valid_registered_session()
        .ok_or_else(|| anyhow::anyhow!("registered device session is unavailable"))?;
    let mqtt = session
        .mqtt
        .as_ref()
        .ok_or_else(|| anyhow::anyhow!("mqtt credential is missing after device registration"))?;
    control_transport_worker::wake_control_transport_worker(
        &context.runtime,
        &context.task_queue,
        &context.transport_worker_state,
        &context.state_notifier,
    );
    let downstream_topic = format!("{}/control/down", mqtt.topic_prefix.trim_end_matches('/'));
    let topic_root = mqtt
        .topic_prefix
        .split("/devices/")
        .next()
        .unwrap_or("slan")
        .trim_end_matches('/');
    let network_event_topics = session
        .network_ids
        .iter()
        .filter(|network_id| !network_id.trim().is_empty())
        .map(|network_id| format!("{topic_root}/networks/{network_id}/broadcast"))
        .collect::<Vec<_>>();
    Ok(serde_json::json!({
        "connected": true,
        "deviceId": session.device_id,
        "downstreamTopic": downstream_topic,
        "networkEventTopics": network_event_topics,
        "networkEventSubscribed": true,
        "networkSubscribeError": null,
        "brokerUrl": mqtt.broker_url,
        "controlStatus": control_transport::control_transport_status(&session),
    }))
}

fn console_login_key() -> Result<Value> {
    let session = load_session()?;
    if session.session_kind == "device" || session.user_id.trim().is_empty() {
        anyhow::bail!("user login is required before opening Web Console");
    }
    let client = ControlPlaneClient::from_env();
    let login_key = client.console_login_key(
        &session.access_token,
        &session.user_id,
        session
            .device_id
            .as_deref()
            .filter(|value| !value.is_empty()),
    )?;
    Ok(serde_json::json!({
        "loginKey": login_key,
        "deviceId": session.device_id,
    }))
}

fn handle_send_client_message(request: ServiceRequest) -> Result<String> {
    let input: crate::local_api::SendClientMessageRequest =
        serde_json::from_value(request.args).context("decode send client message request")?;
    let mut session = load_session().context("load session")?;
    let network_id = ensure_active_network_id(&mut session)?;
    let from_device_id = session
        .device_id
        .as_deref()
        .filter(|value| !value.trim().is_empty())
        .ok_or_else(|| anyhow::anyhow!("device id is not available"))?;
    let client = ControlPlaneClient::from_env();
    let response = client.send_client_message(
        session_device_api_token(&session),
        &network_id,
        from_device_id,
        &input.target_device_id,
        &input.body,
        input.metadata.clone(),
    )?;
    serde_json::to_string(&serde_json::json!({
        "messageId": response.message_id,
        "topic": response.topic,
        "transport": response.transport,
        "qos": response.qos,
    }))
    .context("encode send client message response")
}

fn control_transport_tick_plan(args: Value) -> Result<ControlTransportTickPlan> {
    let input: ControlTransportTickRequest =
        serde_json::from_value(args).context("decode control transport tick request")?;
    Ok(control_transport::control_transport_tick_plan(
        input,
        current_timestamp_ms(),
    ))
}

fn handle_relay_candidates(runtime: &RuntimeActorHandle, refresh: bool) -> Result<String> {
    let expected_revision = runtime.snapshot().revision;
    let prepared = prepare_relay_candidates_response(refresh)?;
    let response = commit_prepared_runtime_value(
        runtime,
        "local.relay.candidates.commit",
        expected_revision,
        prepared,
    )?;
    serde_json::to_string(&response).context("encode relay candidates")
}

fn handle_set_relay_transport_allowlist(request: ServiceRequest) -> Result<String> {
    let input: SetRelayTransportAllowlistRequest =
        serde_json::from_value(request.args).context("decode relay transport allowlist request")?;
    let allowlist = crate::relay_candidates::set_test_relay_transport_allowlist(
        (!input.transports.is_empty()).then_some(input.transports),
    );
    serde_json::to_string(&serde_json::json!({
        "transports": allowlist.unwrap_or_default(),
    }))
    .context("encode relay transport allowlist response")
}

struct PreparedRuntimeValue<T> {
    value: T,
    prepared_session: PreparedSession,
}

fn commit_prepared_runtime_value<T: Send + 'static>(
    runtime: &RuntimeActorHandle,
    command_kind: &'static str,
    expected_revision: u64,
    prepared: PreparedRuntimeValue<T>,
) -> Result<T> {
    let correlation_id = prepared.prepared_session.session.device_id.clone();
    runtime
        .call_named_if_revision(
            command_kind,
            correlation_id,
            expected_revision,
            move |runtime| {
                ensure_runtime_session_matches(runtime, &prepared.prepared_session.session)?;
                commit_registered_session(runtime, &prepared.prepared_session)?;
                Ok(prepared.value)
            },
        )?
        .ok_or_else(|| anyhow::anyhow!("stale prepared runtime value: {command_kind}"))
}

fn handle_prepare_relay_data_plane(runtime: &RuntimeActorHandle) -> Result<String> {
    let expected_revision = runtime.snapshot().revision;
    let prepared = prepare_relay_data_plane_from_latest_control()?;
    let config = commit_prepared_runtime_value(
        runtime,
        "local.relay.prepare.commit",
        expected_revision,
        prepared,
    )?;
    serde_json::to_string(&config).context("encode relay data plane config")
}

fn handle_path_diagnose(runtime: &RuntimeActorHandle) -> Result<String> {
    let expected_revision = runtime.snapshot().revision;
    let prepared = prepare_path_diagnose_response()?;
    let response = commit_prepared_runtime_value(
        runtime,
        "local.path.diagnose.commit",
        expected_revision,
        prepared,
    )?;
    serde_json::to_string(&response).context("encode path diagnose")
}

fn handle_export_diagnostics(runtime: &RuntimeActorHandle) -> Result<String> {
    let state = runtime.snapshot().state;
    let expected_revision = runtime.snapshot().revision;
    let diagnose = prepare_path_diagnose_response()
        .and_then(|prepared| {
            commit_prepared_runtime_value(
                runtime,
                "local.diagnostics.path.commit",
                expected_revision,
                prepared,
            )
        })
        .ok();
    let platform_snapshot = platform_transition::snapshot();
    let platform = platform_snapshot.platform_diagnostics;
    let payload = serde_json::json!({
        "exportedAtMs": current_timestamp_ms(),
        "state": state,
        "session": diagnostic_session_summary(),
        "path": diagnose,
        "platform": platform,
        "relayStats": load_relay_runtime_stats(),
        "localData": diagnostic_local_data_snapshot(),
        "localLogs": diagnostic_log_snapshot(),
        "systemSnapshot": diagnostic_system_snapshot(),
    });
    let path = diagnostics_export_file_path();
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent).context("create diagnostics directory")?;
    }
    fs::write(&path, serde_json::to_vec_pretty(&payload)?).context("write diagnostics export")?;
    Ok(serde_json::json!({
        "path": path.display().to_string(),
        "exportedAtMs": current_timestamp_ms(),
    })
    .to_string())
}

fn diagnostic_local_data_snapshot() -> serde_json::Value {
    serde_json::json!({
        "relayStatsFile": diagnostic_json_file_snapshot(&relay_stats_file_path()),
        "connectPlanFile": diagnostic_connect_plan_file_snapshot(),
    })
}

fn diagnostic_json_file_snapshot(path: &std::path::Path) -> serde_json::Value {
    let metadata = fs::metadata(path).ok();
    let exists = metadata.is_some();
    let size_bytes = metadata.as_ref().map(std::fs::Metadata::len);
    let modified_at_ms = metadata
        .and_then(|metadata| metadata.modified().ok())
        .and_then(|modified| modified.duration_since(UNIX_EPOCH).ok())
        .map(|duration| duration.as_millis() as u64);
    let json = fs::read(path)
        .ok()
        .and_then(|payload| serde_json::from_slice::<Value>(&payload).ok());
    serde_json::json!({
        "path": path.display().to_string(),
        "exists": exists,
        "sizeBytes": size_bytes,
        "modifiedAtMs": modified_at_ms,
        "json": json,
    })
}

fn diagnostic_connect_plan_file_snapshot() -> serde_json::Value {
    let store = load_connect_plan_store();
    let summaries = diagnostic_connect_plan_summaries(&store, current_timestamp_ms());
    serde_json::json!({
        "storage": "memory",
        "exists": false,
        "sizeBytes": null,
        "modifiedAtMs": null,
        "planCount": store.plans.len(),
        "plans": summaries,
    })
}

fn diagnostic_connect_plan_summaries(
    store: &PersistedConnectPlanStore,
    now_ms: u64,
) -> Vec<serde_json::Value> {
    let cutoff = now_ms.saturating_sub(CONNECT_PLAN_TTL_MS);
    store
        .plans
        .iter()
        .map(|plan| {
            serde_json::json!({
                "peerNodeId": plan.peer_node_id,
                "preferDirect": plan.prefer_direct,
                "pathCount": plan.paths.len(),
                "pathTypes": plan.paths.iter().map(|path| path.path_type.as_str()).collect::<Vec<_>>(),
                "hasRelayTicket": plan.relay_ticket.is_some(),
                "relayTicketExpiresAt": plan.relay_ticket.as_ref().map(|ticket| ticket.expires_at.as_str()),
                "updatedAtMs": plan.updated_at_ms,
                "expired": plan.updated_at_ms < cutoff,
            })
        })
        .collect()
}

fn diagnostic_log_snapshot() -> serde_json::Value {
    let service_log = app_data_dir().join("SLAN").join("client-core-service.log");
    let ui_log = std::env::temp_dir().join("slan").join("client-v2-ui.log");
    serde_json::json!({
        "serviceLogPath": service_log.display().to_string(),
        "serviceLogTail": tail_text_file(&service_log, 160),
        "uiLogPath": ui_log.display().to_string(),
        "uiLogTail": tail_text_file(&ui_log, 160),
    })
}

fn tail_text_file(path: &std::path::Path, max_lines: usize) -> Option<String> {
    let payload = fs::read_to_string(path).ok()?;
    let mut lines = payload.lines().rev().take(max_lines).collect::<Vec<_>>();
    lines.reverse();
    Some(lines.join("\n"))
}

#[cfg(target_os = "windows")]
fn diagnostic_system_snapshot() -> serde_json::Value {
    serde_json::json!({
        "os": std::env::consts::OS,
        "commands": {
            "ipconfig": diagnostic_command("ipconfig", &["/all"]),
            "routePrint": diagnostic_command("route", &["print"]),
            "slanAdapters": diagnostic_powershell("Get-NetAdapter | Where-Object { $_.Name -like '*SLAN*' -or $_.InterfaceDescription -like '*SLAN*' -or $_.InterfaceDescription -like '*Wintun*' } | Format-List Name,InterfaceDescription,Status,ifIndex,MacAddress,LinkSpeed"),
            "slanDns": diagnostic_powershell("Get-DnsClientServerAddress -AddressFamily IPv4 | Where-Object { $_.InterfaceAlias -like '*SLAN*' } | Format-List InterfaceAlias,InterfaceIndex,ServerAddresses"),
            "slanRoutes": diagnostic_powershell("Get-NetRoute -AddressFamily IPv4 | Where-Object { $_.InterfaceAlias -like '*SLAN*' -or $_.DestinationPrefix -like '10.*' } | Sort-Object DestinationPrefix | Select-Object -First 80 | Format-Table -AutoSize DestinationPrefix,NextHop,InterfaceAlias,InterfaceIndex,RouteMetric"),
            "service": diagnostic_powershell("Get-Service -Name 'SLANClientV2Service' -ErrorAction SilentlyContinue | Format-List Name,Status,StartType,ServiceType,CanStop"),
        }
    })
}

#[cfg(not(target_os = "windows"))]
fn diagnostic_system_snapshot() -> serde_json::Value {
    serde_json::json!({
        "os": std::env::consts::OS,
        "commands": {}
    })
}

#[cfg(target_os = "windows")]
fn diagnostic_powershell(script: &str) -> serde_json::Value {
    diagnostic_command(
        "powershell.exe",
        &[
            "-NoProfile",
            "-ExecutionPolicy",
            "Bypass",
            "-Command",
            script,
        ],
    )
}

#[cfg(target_os = "windows")]
fn diagnostic_command(program: &str, args: &[&str]) -> serde_json::Value {
    let timeout = Duration::from_secs(5);
    let started = Instant::now();
    let mut child = match Command::new(program)
        .args(args)
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()
    {
        Ok(child) => child,
        Err(error) => {
            return serde_json::json!({
                "ok": false,
                "error": error.to_string(),
            });
        }
    };
    loop {
        match child.try_wait() {
            Ok(Some(_status)) => match child.wait_with_output() {
                Ok(output) => {
                    return serde_json::json!({
                        "ok": output.status.success(),
                        "code": output.status.code(),
                        "elapsedMs": started.elapsed().as_millis() as u64,
                        "stdout": diagnostic_truncate(&String::from_utf8_lossy(&output.stdout), 24_000),
                        "stderr": diagnostic_truncate(&String::from_utf8_lossy(&output.stderr), 8_000),
                    });
                }
                Err(error) => {
                    return serde_json::json!({
                        "ok": false,
                        "elapsedMs": started.elapsed().as_millis() as u64,
                        "error": error.to_string(),
                    });
                }
            },
            Ok(None) if started.elapsed() >= timeout => {
                let _ = child.kill();
                let output = child.wait_with_output().ok();
                return serde_json::json!({
                    "ok": false,
                    "timedOut": true,
                    "elapsedMs": started.elapsed().as_millis() as u64,
                    "stdout": output
                        .as_ref()
                        .map(|value| diagnostic_truncate(&String::from_utf8_lossy(&value.stdout), 24_000)),
                    "stderr": output
                        .as_ref()
                        .map(|value| diagnostic_truncate(&String::from_utf8_lossy(&value.stderr), 8_000)),
                });
            }
            Ok(None) => thread::sleep(Duration::from_millis(50)),
            Err(error) => {
                let _ = child.kill();
                return serde_json::json!({
                    "ok": false,
                    "elapsedMs": started.elapsed().as_millis() as u64,
                    "error": error.to_string(),
                });
            }
        }
    }
}

#[cfg(target_os = "windows")]
fn diagnostic_truncate(value: &str, max_chars: usize) -> String {
    let mut output = value.chars().take(max_chars).collect::<String>();
    if value.chars().count() > max_chars {
        output.push_str("\n...[truncated]");
    }
    output
}

fn prepare_relay_candidates_response(
    refresh: bool,
) -> Result<PreparedRuntimeValue<RelayCandidateListResponse>> {
    let mut prepared_session = prepare_session_device_registered(load_network_session()?)?;
    let network_id = ensure_active_network_id(&mut prepared_session.session)?;
    let refreshed_candidates = if refresh {
        prepare_relay_candidates_for_session(&prepared_session.session, &network_id)?
    } else {
        Vec::new()
    };
    let refreshed = !refreshed_candidates.is_empty();
    if refreshed {
        prepared_session.relay_candidates = refreshed_candidates;
    }
    let selections = select_relay_candidates(&prepared_session.relay_candidates);
    let best = selections.iter().find(|item| item.selected).cloned();
    Ok(PreparedRuntimeValue {
        value: RelayCandidateListResponse {
            network_id: Some(network_id),
            refreshed,
            candidates: selections,
            best,
        },
        prepared_session,
    })
}

fn prepare_path_diagnose_response() -> Result<PreparedRuntimeValue<PathDiagnoseResponse>> {
    let mut prepared_session = prepare_session_device_registered(load_network_session()?)?;
    let prepared_relay_candidates = prepared_session.relay_candidates.clone();
    let session = &mut prepared_session.session;
    let network_id = ensure_active_network_id(session)?;
    let client = ControlPlaneClient::from_env();
    let device_id = session
        .device_id
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .ok_or_else(|| anyhow::anyhow!("device unavailable: current device is not registered"))?
        .to_string();
    let activation =
        client.activate_device_networks(session_device_api_token(session), &device_id)?;
    let persisted_relay_candidates = if activation.relay_candidates.is_empty() {
        if prepared_relay_candidates.is_empty() {
            runtime_relay_candidates()
        } else {
            prepared_relay_candidates
        }
    } else {
        activation
            .relay_candidates
            .iter()
            .map(persisted_relay_candidate)
            .collect()
    };
    session.self_node_id = activation.self_node_id.clone();
    session.virtual_ip = Some(activation.virtual_ip);

    let stats = load_relay_runtime_stats();
    let platform_snapshot = platform_transition::snapshot();
    let platform =
        platform_snapshot
            .platform_diagnostics
            .unwrap_or_else(|| PlatformNetworkDiagnostics {
                platform: std::env::consts::OS.to_string(),
                checks: vec![client_core::PlatformDiagnosticCheck {
                    name: "platformDiagnostics".to_string(),
                    ok: false,
                    message: Some(
                        platform_snapshot
                            .platform_diagnostics_last_error
                            .unwrap_or_else(|| {
                                "platform diagnostics snapshot is not ready".to_string()
                            }),
                    ),
                }],
                ..PlatformNetworkDiagnostics::default()
            });
    let runtime_state = platform_snapshot.runtime_state;
    let peer_paths = runtime_state.peer_paths;
    let active_path_counts = path_diagnose_active_path_counts(&peer_paths);
    let active_path_type = stats
        .as_ref()
        .filter(|stats| {
            current_timestamp_ms().saturating_sub(stats.updated_at_ms) <= RELAY_STATS_STALE_MS
        })
        .and_then(|stats| stats.active_path.clone())
        .or_else(|| {
            runtime_state
                .active_path
                .map(|path| path.as_str().to_string())
        })
        .unwrap_or_else(|| "unknown".to_string());
    let relay = stats.as_ref().map(|stats| {
        let ticket_timing =
            relay_ticket_timing(current_timestamp_ms(), stats.ticket_expires_at.as_deref());
        PathDiagnoseRelay {
            address: stats.relay_address.clone(),
            transport: stats.relay_transport.clone(),
            active_path: stats.active_path.clone(),
            requested_relay_session_count: stats.requested_relay_session_count,
            relay_session_count: stats.relay_session_count,
            attached_peer_session_count: relay_attached_peer_session_count(stats),
            attached_transport_count: relay_attached_transport_count(stats),
            ticket_expires_at: stats.ticket_expires_at.clone(),
            ticket_expires_in_ms: ticket_timing.expires_in_ms,
            ticket_renew_due: ticket_timing.renew_due,
            relay_attach_failures: stats.relay_attach_failures,
            last_relay_attach_error: stats.last_relay_attach_error.clone(),
            peers: stats
                .peers
                .iter()
                .map(|peer| PathDiagnoseRelayPeer {
                    peer_node_id: peer.peer_node_id.clone(),
                    session_id: peer.session_id.clone(),
                    peer_virtual_ips: peer.peer_virtual_ips.clone(),
                    attached: peer.attached,
                    attach_error: peer.attach_error.clone(),
                    tun_packets_sent: peer.tun_packets_sent,
                    relay_packets_received: peer.relay_packets_received,
                    relay_errors: peer.relay_errors,
                    last_relay_error: peer.last_relay_error.clone(),
                    last_send_path: peer.last_send_path.clone(),
                    path_downgrades: peer.path_downgrades,
                    path_upgrades: peer.path_upgrades,
                    last_path_change: peer.last_path_change.clone(),
                    replayed_frames: peer.replayed_frames,
                    config_hash_mismatches: peer.config_hash_mismatches,
                    last_rx_seq: peer.last_rx_seq,
                    send_failures: peer.send_failures,
                    receive_failures: peer.receive_failures,
                    wintun_write_failures: peer.wintun_write_failures,
                })
                .collect(),
            relay_mtu: stats.relay_mtu,
            max_frame_payload: stats.max_frame_payload,
            tun_packets_sent: stats.tun_packets_sent,
            relay_packets_received: stats.relay_packets_received,
            relay_error_responses: stats.relay_error_responses,
            relay_config_hash_mismatches: stats.relay_config_hash_mismatches,
            last_relay_error: stats.last_relay_error.clone(),
            failures: relay_runtime_failure_total(stats),
            unroutable_tun_packets: stats.unroutable_tun_packets,
            last_unroutable_destination: stats.last_unroutable_destination.clone(),
            oversized_tun_packets: stats.oversized_tun_packets,
            last_oversized_tun_packet_size: stats.last_oversized_tun_packet_size,
            started_at_ms: stats.started_at_ms,
            last_tun_packet_at_ms: stats.last_tun_packet_at_ms,
            last_relay_packet_at_ms: stats.last_relay_packet_at_ms,
            last_relay_keepalive_at_ms: stats.last_relay_keepalive_at_ms,
            updated_at_ms: stats.updated_at_ms,
            stale: current_timestamp_ms().saturating_sub(stats.updated_at_ms)
                > RELAY_STATS_STALE_MS,
        }
    });
    let direct_candidates = diagnose_direct_candidates(&activation.peers);
    let relay_candidates = select_relay_candidates(&persisted_relay_candidates);
    let expected_mtu = stats.as_ref().and_then(|stats| stats.relay_mtu);
    let expected_payload = stats.as_ref().and_then(|stats| stats.max_frame_payload);
    let mtu = PathDiagnoseMtu {
        relay_mtu: expected_mtu,
        max_frame_payload: expected_payload,
        policy_scope: None,
        policy_path_type: None,
        actual_mtu_checked: expected_mtu.is_some() && platform.mtu.is_some(),
        actual_mtu_ok: expected_mtu.and_then(|expected| {
            platform
                .mtu
                .map(|actual| actual == u32::from(expected))
        }),
        actual_mss_checked: expected_payload.is_some() && platform.mss.is_some(),
        actual_mss_ok: expected_payload.and_then(|expected| {
            platform
                .mss
                .map(|actual| actual >= u32::from(expected))
        }),
        note: Some("Windows applies adapter MTU directly. MSS is reported as an effective IPv4 payload guard when platform readback is available; otherwise the relay maxFramePayload guard is enforced in the local data thread.".to_string()),
    };
    let resolver = path_diagnose_resolver(&activation.resolver.servers, &platform.resolver_servers);
    let health = path_diagnose_health(
        relay.as_ref(),
        &mtu,
        &resolver,
        &platform,
        &relay_candidates,
        &peer_paths,
    );
    let response = PathDiagnoseResponse {
        network_id: Some(network_id),
        health,
        active_path_type,
        active_path_counts,
        peer_paths,
        relay,
        direct_candidates,
        relay_candidates,
        mtu,
        resolver,
        platform,
        export_path: None,
    };
    prepared_session.relay_candidates = persisted_relay_candidates;
    Ok(PreparedRuntimeValue {
        value: response,
        prepared_session,
    })
}

fn path_diagnose_health(
    relay: Option<&PathDiagnoseRelay>,
    mtu: &PathDiagnoseMtu,
    resolver: &PathDiagnoseResolver,
    platform: &PlatformNetworkDiagnostics,
    relay_candidates: &[RelayCandidateSelection],
    peer_paths: &[client_core::PeerPathRuntime],
) -> PathDiagnoseHealth {
    let mut reasons = Vec::new();
    let mut failed = false;
    let mut degraded = false;

    if relay_candidates.is_empty() {
        degraded = true;
        push_path_health_reason(
            &mut reasons,
            "no_relay_candidates",
            "degraded",
            "no relay candidates are available from the control plane",
        );
    } else if !relay_candidates.iter().any(|candidate| candidate.reachable) {
        failed = true;
        push_path_health_reason(
            &mut reasons,
            "no_reachable_relay_candidates",
            "failed",
            "relay candidates exist but none are reachable",
        );
    }

    match relay {
        Some(relay) => {
            if relay.stale {
                degraded = true;
                push_path_health_reason(
                    &mut reasons,
                    "stale_relay_stats",
                    "degraded",
                    "relay runtime stats are stale",
                );
            }
            if relay.requested_relay_session_count > 0
                && relay.attached_peer_session_count < relay.requested_relay_session_count
            {
                failed = true;
                push_path_health_reason(
                    &mut reasons,
                    "relay_peer_sessions_not_attached",
                    "failed",
                    "one or more relay peer sessions are not attached",
                );
            }
            if relay.requested_relay_session_count > 0 && relay.attached_transport_count == 0 {
                failed = true;
                push_path_health_reason(
                    &mut reasons,
                    "no_attached_transports",
                    "failed",
                    "no relay data-plane transport is attached",
                );
            }
            if relay.ticket_expires_in_ms.is_some_and(|value| value <= 0) {
                failed = true;
                push_path_health_reason(
                    &mut reasons,
                    "relay_ticket_expired",
                    "failed",
                    "relay ticket is expired",
                );
            } else if relay.ticket_renew_due {
                degraded = true;
                push_path_health_reason(
                    &mut reasons,
                    "relay_ticket_renew_due",
                    "degraded",
                    "relay ticket is near expiration and should renew",
                );
            }
            if relay.unroutable_tun_packets > 0 {
                failed = true;
                push_path_health_reason(
                    &mut reasons,
                    "unroutable_tun_packets",
                    "failed",
                    "TUN packets could not be routed to a peer path",
                );
            }
            if relay.oversized_tun_packets > 0 {
                failed = true;
                push_path_health_reason(
                    &mut reasons,
                    "oversized_tun_packets",
                    "failed",
                    "TUN packets exceeded the relay frame payload limit",
                );
            }
            if relay.relay_config_hash_mismatches > 0 {
                failed = true;
                push_path_health_reason(
                    &mut reasons,
                    "relay_config_hash_mismatch",
                    "failed",
                    "relay frames from an old or different data-plane config were received",
                );
            }
            if relay.peers.iter().any(|peer| peer.replayed_frames > 0) {
                failed = true;
                push_path_health_reason(
                    &mut reasons,
                    "relay_replayed_frames",
                    "failed",
                    "old or replayed relay frames were received",
                );
            }
            if relay_response_gap_is_active(relay) {
                degraded = true;
                push_path_health_reason(
                    &mut reasons,
                    "relay_response_gap",
                    "degraded",
                    "relay sent packets are not being matched by relay receive packets",
                );
            }
            if relay.failures > 0 {
                degraded = true;
                push_path_health_reason(
                    &mut reasons,
                    "relay_runtime_failures",
                    "degraded",
                    "relay runtime failure counters are non-zero",
                );
            }
        }
        None => {
            if peer_paths.is_empty() {
                failed = true;
                push_path_health_reason(
                    &mut reasons,
                    "missing_relay_stats",
                    "failed",
                    "relay runtime stats are missing",
                );
            }
        }
    }

    if peer_paths.is_empty() {
        degraded = true;
        push_path_health_reason(
            &mut reasons,
            "missing_peer_paths",
            "degraded",
            "no runtime peer paths are recorded",
        );
    }
    if resolver.checked && resolver.ok == Some(false) {
        failed = true;
        push_path_health_reason(
            &mut reasons,
            "resolver_mismatch",
            "failed",
            "Windows resolver configuration does not match the expected network resolver servers",
        );
    }
    if mtu.actual_mtu_checked && mtu.actual_mtu_ok == Some(false) {
        failed = true;
        push_path_health_reason(
            &mut reasons,
            "mtu_mismatch",
            "failed",
            "Windows adapter MTU does not match relay policy",
        );
    }
    if mtu.actual_mss_checked && mtu.actual_mss_ok == Some(false) {
        failed = true;
        push_path_health_reason(
            &mut reasons,
            "mss_too_small",
            "failed",
            "effective IPv4 payload size is below relay policy",
        );
    }
    for check in &platform.checks {
        if !check.ok {
            degraded = true;
            push_path_health_reason(
                &mut reasons,
                format!("platform_check_{}", check.name),
                "degraded",
                check
                    .message
                    .clone()
                    .unwrap_or_else(|| format!("platform check {} failed", check.name)),
            );
        }
    }

    let status = if failed {
        "failed"
    } else if degraded {
        "degraded"
    } else {
        "ok"
    };
    PathDiagnoseHealth {
        status: status.to_string(),
        reasons,
    }
}

fn push_path_health_reason(
    reasons: &mut Vec<PathDiagnoseHealthReason>,
    code: impl Into<String>,
    severity: impl Into<String>,
    message: impl Into<String>,
) {
    reasons.push(PathDiagnoseHealthReason {
        code: code.into(),
        severity: severity.into(),
        message: message.into(),
    });
}

fn path_diagnose_active_path_counts(
    peer_paths: &[client_core::PeerPathRuntime],
) -> Vec<PathDiagnosePathCount> {
    let mut counts = BTreeMap::<String, usize>::new();
    for peer in peer_paths {
        let path_type = peer
            .active_path
            .map(|path| path.as_str().to_string())
            .unwrap_or_else(|| "unknown".to_string());
        *counts.entry(path_type).or_default() += 1;
    }
    let mut values = counts
        .into_iter()
        .map(|(path_type, count)| PathDiagnosePathCount { path_type, count })
        .collect::<Vec<_>>();
    values.sort_by(|left, right| {
        right
            .count
            .cmp(&left.count)
            .then_with(|| left.path_type.cmp(&right.path_type))
    });
    values
}

fn relay_attached_peer_session_count(stats: &RelayRuntimeStats) -> u32 {
    if stats.attached_peer_session_count > 0 {
        return stats.attached_peer_session_count;
    }
    if !stats.peers.is_empty() {
        return stats.peers.iter().filter(|peer| peer.attached).count() as u32;
    }
    stats.relay_session_count
}

fn relay_attached_transport_count(stats: &RelayRuntimeStats) -> u32 {
    if stats.attached_transport_count > 0 {
        return stats.attached_transport_count;
    }
    stats.relay_session_count
}

fn path_diagnose_resolver(
    expected_servers: &[String],
    actual_servers: &[String],
) -> PathDiagnoseResolver {
    let expected = normalized_resolver_servers(expected_servers);
    let actual = normalized_resolver_servers(actual_servers);
    let missing = expected
        .iter()
        .filter(|server| {
            !actual
                .iter()
                .any(|value| value.eq_ignore_ascii_case(server))
        })
        .cloned()
        .collect::<Vec<_>>();
    let extra = actual
        .iter()
        .filter(|server| {
            !expected
                .iter()
                .any(|value| value.eq_ignore_ascii_case(server))
        })
        .cloned()
        .collect::<Vec<_>>();
    let checked = !expected.is_empty();
    PathDiagnoseResolver {
        expected_servers: expected,
        actual_servers: actual,
        checked,
        ok: checked.then_some(missing.is_empty()),
        missing_servers: missing,
        extra_servers: extra,
    }
}

fn normalized_resolver_servers(servers: &[String]) -> Vec<String> {
    let mut normalized = servers
        .iter()
        .map(|value| value.trim())
        .filter(|value| !value.is_empty())
        .map(str::to_string)
        .collect::<Vec<_>>();
    normalized.sort_by_key(|value| value.to_ascii_lowercase());
    normalized.dedup_by(|left, right| left.eq_ignore_ascii_case(right));
    normalized
}

fn handle_watch_state(
    request: ServiceRequest,
    runtime: &RuntimeActorHandle,
    _state_notifier: &Arc<RuntimeEventHub>,
) -> Result<String> {
    let input: WatchStateRequest =
        serde_json::from_value(request.args).context("decode watch state request")?;
    let timeout = Duration::from_millis(input.timeout_ms.clamp(1_000, 60_000));
    let snapshot = runtime.snapshots().wait_after(input.last_revision, timeout);
    let mut state = snapshot.state;
    enrich_signal_quality(&mut state);
    serde_json::to_string(&WatchStateResponse {
        revision: snapshot.revision,
        state,
    })
    .context("encode watch state response")
}

fn handle_watch_business_event(
    request: ServiceRequest,
    runtime: &RuntimeActorHandle,
    state_notifier: &Arc<RuntimeEventHub>,
) -> Result<String> {
    let input: WatchBusinessEventRequest =
        serde_json::from_value(request.args).context("decode watch business event request")?;
    let timeout = Duration::from_millis(input.timeout_ms.clamp(1_000, 60_000));
    let stream_reset = input
        .stream_id
        .as_deref()
        .is_some_and(|stream_id| stream_id != state_notifier.stream_id());
    let effective_last_revision = if input.follow_latest && !stream_reset {
        state_notifier.latest_revision()
    } else {
        input.last_revision
    };
    let read = if stream_reset {
        state_notifier.read_after(0)
    } else {
        state_notifier.wait_read_after(effective_last_revision, timeout)
    };
    let event = read.event;
    let mut snapshot = runtime.snapshot().state;
    enrich_signal_quality(&mut snapshot);
    serde_json::to_string(&WatchBusinessEventResponse {
        revision: event
            .as_ref()
            .map(|event| event.revision)
            .unwrap_or(if stream_reset {
                read.latest_revision
            } else {
                effective_last_revision
            }),
        stream_id: state_notifier.stream_id().to_string(),
        stream_reset,
        oldest_available_revision: read.oldest_available_revision,
        latest_revision: read.latest_revision,
        replay_gap: read.replay_gap,
        event_id: event.as_ref().map(|event| event.event_id.clone()),
        business_type: event
            .as_ref()
            .map(|event| event.business_type.clone())
            .unwrap_or_else(|| BUSINESS_STATE_CHANGED.to_string()),
        business_data: event
            .as_ref()
            .map(|event| event.business_data.clone())
            .unwrap_or_else(|| serde_json::json!({})),
        published_at_ms: event.as_ref().map(|event| event.published_at_ms),
        replayed: event
            .as_ref()
            .is_some_and(|event| event.revision < read.latest_revision),
        snapshot,
    })
    .context("encode watch business event response")
}

fn handle_task_request(
    request: ServiceRequest,
    runtime: &RuntimeActorHandle,
    task_queue: &Arc<Mutex<ControlTaskQueue>>,
) -> Result<String> {
    let input: EnqueueControlTaskRequest =
        serde_json::from_value(request.args).context("decode control task request")?;
    let mut queue = task_queue
        .lock()
        .expect("control task queue mutex poisoned");
    let task = queue.enqueue(input)?;
    if task.require_ui_refresh {
        drop(queue);
        let state = drain_pending_control_tasks(runtime, task_queue);
        return serde_json::to_string(&state).context("encode client state after control task");
    }
    Ok(serde_json::json!({
        "accepted": true,
        "taskId": task.id,
        "requireUiRefresh": task.require_ui_refresh,
    })
    .to_string())
}

fn handle_downstream_task_request(
    request: ServiceRequest,
    runtime: &RuntimeActorHandle,
    task_queue: &Arc<Mutex<ControlTaskQueue>>,
) -> Result<String> {
    let input: EnqueueControlTaskRequest =
        serde_json::from_value(request.args).context("decode downstream control task request")?;
    let action = ControlTaskAction::from_str(input.action.trim())
        .ok_or_else(|| anyhow::anyhow!("unsupported control task action: {}", input.action))?;
    let mut queue = task_queue
        .lock()
        .expect("control task queue mutex poisoned");
    let task = queue.enqueue_downstream(
        action,
        input
            .delivery_id
            .map(|value| value.trim().to_string())
            .filter(|value| !value.is_empty())
            .unwrap_or_else(|| format!("downstream-{}", current_timestamp_ms())),
        input.require_ui_refresh,
    )?;
    if task.require_ui_refresh {
        drop(queue);
        let state = drain_pending_control_tasks(runtime, task_queue);
        return serde_json::to_string(&state).context("encode client state after downstream task");
    }
    Ok(serde_json::json!({
        "accepted": true,
        "taskId": task.id,
        "direction": "downstream",
        "requireUiRefresh": task.require_ui_refresh,
    })
    .to_string())
}

fn handle_downstream_control_message(
    request: ServiceRequest,
    runtime: &RuntimeActorHandle,
    task_queue: &Arc<Mutex<ControlTaskQueue>>,
) -> Result<String> {
    let self_device_id = load_session()
        .ok()
        .and_then(|session| session.device_id)
        .filter(|value| !value.trim().is_empty());
    let Some(accepted) = control_transport::normalize_downstream_control_value(
        request.args,
        self_device_id.as_deref(),
    )?
    else {
        return Ok(serde_json::json!({
            "accepted": false,
            "ignored": true,
        })
        .to_string());
    };
    let task_request = ServiceRequest {
        method: "enqueueDownstreamControlTask".to_string(),
        args: serde_json::json!({
            "action": accepted.action,
            "deliveryId": accepted.delivery_id,
            "requireUiRefresh": accepted.require_ui_refresh,
        }),
    };
    handle_downstream_task_request(task_request, runtime, task_queue)
}

fn handle_pending_control_acks(task_queue: &Arc<Mutex<ControlTaskQueue>>) -> Result<String> {
    let queue = task_queue
        .lock()
        .expect("control task queue mutex poisoned");
    let acks: Vec<_> = queue
        .pending_downstream_acks()
        .iter()
        .filter_map(control_transport::downstream_task_ack)
        .collect();
    serde_json::to_string(&acks).context("encode pending control acks")
}

fn handle_mark_control_acked(
    request: ServiceRequest,
    task_queue: &Arc<Mutex<ControlTaskQueue>>,
) -> Result<String> {
    let input: MarkControlAckedRequest =
        serde_json::from_value(request.args).context("decode control ack request")?;
    let task_id = input.task_id.trim();
    if task_id.is_empty() {
        anyhow::bail!("control ack taskId is required");
    }
    let mut queue = task_queue
        .lock()
        .expect("control task queue mutex poisoned");
    queue.mark_acknowledged(task_id)?;
    Ok(serde_json::json!({
        "acknowledged": true,
        "taskId": task_id,
    })
    .to_string())
}

fn handle_control_transport_outbox(
    request: ServiceRequest,
    runtime: &RuntimeActorHandle,
    task_queue: &Arc<Mutex<ControlTaskQueue>>,
) -> Result<String> {
    let input: ControlTransportOutboxRequest =
        serde_json::from_value(request.args).context("decode control transport outbox request")?;
    let session = load_session()?;
    let state = runtime.snapshot().state;
    let acks = if input.include_control_acks {
        let queue = task_queue
            .lock()
            .expect("control task queue mutex poisoned");
        queue
            .pending_downstream_acks()
            .iter()
            .filter_map(control_transport::downstream_task_ack)
            .collect()
    } else {
        Vec::new()
    };
    let outbox: ControlTransportOutbox = control_transport::control_transport_outbox(
        &session,
        &state,
        acks,
        current_timestamp_ms(),
        input.include_heartbeat,
        input.include_runtime_state,
        input.include_path_health,
    );
    serde_json::to_string(&outbox).context("encode control transport outbox")
}

fn handle_mark_transport_published(
    request: ServiceRequest,
    _runtime: &RuntimeActorHandle,
    task_queue: &Arc<Mutex<ControlTaskQueue>>,
) -> Result<String> {
    let input: PublishedControlTransportMessage =
        serde_json::from_value(request.args).context("decode published transport message")?;
    let message_id = input.id.trim();
    if message_id.is_empty() {
        anyhow::bail!("published transport message id is required");
    }
    let mut acknowledged_task_id = None;
    if let Some(task_id) = control_transport::ack_task_id_from_transport_message_id(message_id) {
        let mut queue = task_queue
            .lock()
            .expect("control task queue mutex poisoned");
        queue.mark_acknowledged(&task_id)?;
        acknowledged_task_id = Some(task_id);
    }
    Ok(serde_json::json!({
        "published": true,
        "id": message_id,
        "acknowledgedTaskId": acknowledged_task_id,
    })
    .to_string())
}

pub(crate) fn drain_pending_control_tasks(
    runtime: &RuntimeActorHandle,
    task_queue: &Arc<Mutex<ControlTaskQueue>>,
) -> ClientViewState {
    let mut last_state = runtime.snapshot().state;
    loop {
        let task = {
            let mut task_queue = task_queue
                .lock()
                .expect("control task queue mutex poisoned");
            match task_queue.take_next_pending() {
                Ok(task) => task,
                Err(error) => {
                    log_service_error(format!("CONTROL_TASK_DEQUEUE_ERROR error={error:#}"));
                    return state_with_error(&last_state, error.to_string());
                }
            }
        };
        let Some(task) = task else {
            return last_state;
        };
        last_state = execute_control_task(runtime, task_queue, task);
        if last_state.error.is_some() {
            return last_state;
        }
    }
}

fn execute_control_task(
    runtime: &RuntimeActorHandle,
    task_queue: &Arc<Mutex<ControlTaskQueue>>,
    task: control_tasks::ControlTask,
) -> ClientViewState {
    if task.action == ControlTaskAction::ReconcileNetworkState {
        let snapshot = runtime.snapshot();
        let prepared = prepare_downstream_network_assignment(snapshot.state.network_enabled);
        let correlation_id = Some(task.id.clone());
        let state = execute_downstream_network_assignment(
            runtime,
            snapshot.revision,
            correlation_id,
            prepared,
        )
        .unwrap_or_else(|error| state_with_error(&runtime.snapshot().state, error.to_string()));
        let mut task_queue = task_queue
            .lock()
            .expect("control task queue mutex poisoned");
        if let Some(error) = state.error.clone() {
            log_service_error(format!(
                "CONTROL_TASK_EXECUTION_ERROR taskId={} action={} error={error}",
                task.id,
                task.action.as_str()
            ));
            let actor_error = error.clone();
            let correlation_id = Some(task.id.clone());
            let _ = runtime.call_named(
                "control_task.error.record",
                correlation_id,
                move |runtime| Ok(runtime.set_error(actor_error)),
            );
            if let Err(persist_error) = task_queue.mark_failed(&task.id, error) {
                log_service_error(format!(
                    "CONTROL_TASK_STATUS_PERSIST_ERROR taskId={} status=failed error={persist_error:#}",
                    task.id
                ));
            }
        } else {
            if let Err(error) = task_queue.mark_succeeded(&task.id) {
                log_service_error(format!(
                    "CONTROL_TASK_STATUS_PERSIST_ERROR taskId={} status=succeeded error={error:#}",
                    task.id
                ));
            }
        }
        return state;
    }
    if task.action == ControlTaskAction::DeviceUserLoginSucceeded {
        let state = runtime.snapshot().state;
        let mut task_queue = task_queue
            .lock()
            .expect("control task queue mutex poisoned");
        let _ = task_queue.mark_succeeded(&task.id);
        return state;
    }
    let correlation_id = Some(task.id.clone());
    let state = if task.action == ControlTaskAction::EnableNetwork {
        let transition = prepare_latest_runtime_network_activation(runtime);
        execute_runtime_network_activation(
            runtime,
            "control_task.network.activate.commit",
            correlation_id,
            transition,
        )
        .unwrap_or_else(|error| state_with_error(&runtime.snapshot().state, error.to_string()))
    } else if task.action == ControlTaskAction::DisableNetwork {
        handle_network_deactivate(runtime, "control_task.network.deactivate", correlation_id)
            .unwrap_or_else(|error| state_with_error(&runtime.snapshot().state, error.to_string()))
    } else {
        unreachable!("control task action handled above")
    };
    if task.action == ControlTaskAction::EnableNetwork && state.error.is_none() {
        report_runtime_state(&state);
    }
    let mut task_queue = task_queue
        .lock()
        .expect("control task queue mutex poisoned");
    if let Some(error) = state.error.clone() {
        log_service_error(format!(
            "CONTROL_TASK_EXECUTION_ERROR taskId={} action={} error={error}",
            task.id,
            task.action.as_str()
        ));
        let actor_error = error.clone();
        let correlation_id = Some(task.id.clone());
        let _ = runtime.call_named(
            "control_task.error.record",
            correlation_id,
            move |runtime| Ok(runtime.set_error(actor_error)),
        );
        if let Err(persist_error) = task_queue.mark_failed(&task.id, error) {
            log_service_error(format!(
                "CONTROL_TASK_STATUS_PERSIST_ERROR taskId={} status=failed error={persist_error:#}",
                task.id
            ));
        }
    } else {
        if let Err(error) = task_queue.mark_succeeded(&task.id) {
            log_service_error(format!(
                "CONTROL_TASK_STATUS_PERSIST_ERROR taskId={} status=succeeded error={error:#}",
                task.id
            ));
        }
    }
    state
}

fn dispatch_runtime_command<P>(
    runtime: &mut ClientRuntime<P>,
    command: ClientCommand,
) -> ClientViewState
where
    P: client_core::PlatformNetwork,
{
    let command = match command {
        ClientCommand::OpenClientLogin
        | ClientCommand::LoginWithPassword(_)
        | ClientCommand::ApplyDeviceUserLogin(_)
        | ClientCommand::EnableNetwork
        | ClientCommand::DisableNetwork
        | ClientCommand::SyncAssignedIp(_)
        | ClientCommand::Logout => {
            return state_with_error(
                runtime.state(),
                "command requires a prepared runtime transition".to_string(),
            );
        }
        other => other,
    };

    match runtime.dispatch(command) {
        Ok(state) => state,
        Err(error) => {
            log_service_error(format!("client-core-service dispatch failed: {error:#}"));
            state_with_error(runtime.state(), error.to_string())
        }
    }
}

pub(crate) fn log_service_error(message: impl AsRef<str>) {
    let _guard = SERVICE_LOG_LOCK.get_or_init(|| Mutex::new(())).lock().ok();
    let dir = app_data_dir().join("SLAN");
    let _ = fs::create_dir_all(&dir);
    let path = dir.join("client-core-service.log");
    let _ = rotate_log_file(&path, SERVICE_LOG_MAX_BYTES, SERVICE_LOG_BACKUP_COUNT);
    let timestamp = current_timestamp_ms();
    if let Ok(mut file) = OpenOptions::new().create(true).append(true).open(path) {
        let _ = writeln!(file, "{timestamp} {}", message.as_ref());
    }
}

fn rotate_log_file(path: &Path, max_bytes: u64, backup_count: usize) -> Result<()> {
    if backup_count == 0
        || fs::metadata(path)
            .map(|metadata| metadata.len() < max_bytes)
            .unwrap_or(true)
    {
        return Ok(());
    }
    for index in (1..backup_count).rev() {
        let source = path.with_extension(format!("log.{index}"));
        let destination = path.with_extension(format!("log.{}", index + 1));
        if source.exists() {
            if destination.exists() {
                fs::remove_file(&destination)?;
            }
            fs::rename(source, destination)?;
        }
    }
    let first_backup = path.with_extension("log.1");
    if first_backup.exists() {
        fs::remove_file(&first_backup)?;
    }
    fs::rename(path, first_backup).context("rotate client service log")
}

struct PreparedMobilePlatformNetworkConfig {
    config: MobilePlatformNetworkConfig,
    prepared_session: PreparedSession,
    assigned_ip: AssignedIpPayload,
}

fn prepare_platform_network_config_from_latest_control(
) -> Result<PreparedMobilePlatformNetworkConfig> {
    let mut prepared_session = prepare_session_device_registered(load_network_session()?)?;
    let prepared_relay_candidates = prepared_session.relay_candidates.clone();
    let prepared_network_configs = prepared_session.network_configs.clone();
    let session = &mut prepared_session.session;
    let Some(device_id) = session.device_id.clone().filter(|value| !value.is_empty()) else {
        return Err(anyhow::anyhow!(
            "device unavailable: current device is not registered"
        ));
    };
    let client = ControlPlaneClient::from_env();
    let network_id = ensure_active_network_id(session)?;
    let mut activation =
        client.activate_device_networks(session_device_api_token(&session), &device_id)?;
    activation.virtual_ip = normalize_virtual_ip(&activation.virtual_ip);
    session.self_node_id = activation.self_node_id.clone();
    session.virtual_ip = Some(activation.virtual_ip.clone());
    ensure_session_node_binding(&client, session)
        .context("ensure node binding after network activation")?;
    let relay_candidates = if activation.relay_candidates.is_empty() {
        if prepared_relay_candidates.is_empty() {
            runtime_relay_candidates()
        } else {
            prepared_relay_candidates
        }
    } else {
        activation
            .relay_candidates
            .iter()
            .map(persisted_relay_candidate)
            .collect()
    };
    let network_configs = client
        .device_network_configs(session_device_api_token(session), &device_id)
        .unwrap_or_else(|_| {
            if prepared_network_configs.is_empty() {
                crate::network_module::network_module_snapshot().configs
            } else {
                prepared_network_configs
            }
        });
    let all_acl_policies = platform_acl_policies(&network_configs);
    let best_relay = android_data_plane_relay_candidate(&relay_candidates);
    let assigned_ip = AssignedIpPayload {
        virtual_ip: activation.virtual_ip.clone(),
        prefix_len: Some(activation.prefix_len),
    };
    let routes = routes_with_peer_virtual_ips(
        activation.routes.clone(),
        &activation.peers,
        &network_configs,
        activation.virtual_ip.as_str(),
    );
    let resolver = platform_resolver_config(&network_id, &activation.resolver, &network_configs);
    let config = MobilePlatformNetworkConfig {
        session_name: "SLAN".to_string(),
        virtual_ip: activation.virtual_ip,
        prefix_len: activation.prefix_len,
        network_configs: platform_network_configs(&network_configs),
        resolver,
        routes,
        mtu: Some(1280),
        relay_endpoint_id: best_relay.as_ref().map(|relay| relay.endpoint_id.clone()),
        relay_transport: best_relay.as_ref().map(|relay| relay.transport.clone()),
        relay_address: best_relay.as_ref().map(|relay| relay.address.clone()),
        acl_policies: all_acl_policies.clone(),
        relay_data_plane: build_relay_data_plane_config(
            &client,
            session,
            &network_id,
            activation.self_node_id.as_deref(),
            &activation.peers,
            &network_configs,
            best_relay.as_ref(),
            &all_acl_policies,
            true,
        )
        .ok(),
    };
    prepared_session.relay_candidates = relay_candidates;
    prepared_session.network_configs = network_configs;
    Ok(PreparedMobilePlatformNetworkConfig {
        config,
        prepared_session,
        assigned_ip,
    })
}

fn platform_network_configs(
    configs: &[crate::control_plane::DeviceNetworkConfig],
) -> Vec<MobilePlatformDeviceNetworkConfig> {
    configs
        .iter()
        .map(|config| MobilePlatformDeviceNetworkConfig {
            network_id: config.network_id.clone(),
            device_id: config.device_id.clone(),
            network_name: config.network_name.clone(),
            intra_group_policy: config.intra_group_policy.clone(),
            network_created_at: config.network_created_at,
            config_version: config.config_version,
            global_ip: config.global_ip.clone(),
            global_name: config.global_name.clone(),
            peer_count: config.peers.len(),
            security_rule_count: config.rules.len(),
            relay_candidate_count: config.relay_candidates.len(),
        })
        .collect()
}

fn platform_resolver_zones(
    configs: &[crate::control_plane::DeviceNetworkConfig],
) -> Vec<PlatformResolverZone> {
    configs
        .iter()
        .flat_map(|config| {
            config
                .resolver_zones
                .iter()
                .map(|zone| PlatformResolverZone {
                    zone_id: zone.zone_id.clone(),
                    network_id: zone.network_id.clone(),
                    zone_name: zone.zone_name.clone(),
                })
        })
        .collect()
}

fn platform_resolver_records(
    configs: &[crate::control_plane::DeviceNetworkConfig],
) -> Vec<PlatformResolverRecord> {
    resolver_apply::platform_resolver_records_from_configs(configs)
}

fn platform_resolver_config(
    _active_network_id: &str,
    activation_dns: &crate::control_plane::DeviceResolverConfig,
    configs: &[crate::control_plane::DeviceNetworkConfig],
) -> PlatformResolverConfig {
    let search_domains = configs
        .iter()
        .flat_map(|config| config.resolver.search_domains.iter())
        .chain(activation_dns.search_domains.iter())
        .map(|value| value.trim())
        .filter(|value| !value.is_empty())
        .map(str::to_string)
        .collect::<BTreeSet<_>>()
        .into_iter()
        .collect::<Vec<_>>();
    let mut split_domains = configs
        .iter()
        .flat_map(|config| config.resolver.split_domains.iter())
        .chain(activation_dns.split_domains.iter())
        .map(|value| value.trim())
        .filter(|value| !value.is_empty())
        .map(str::to_string)
        .collect::<BTreeSet<_>>()
        .into_iter()
        .collect::<Vec<_>>();
    if split_domains.is_empty() {
        split_domains = search_domains.clone();
    }
    let has_managed_dns = !split_domains.is_empty()
        || configs
            .iter()
            .any(|config| !config.resolver_zones.is_empty() || !config.resolver_records.is_empty());
    PlatformResolverConfig {
        servers: has_managed_dns
            .then(|| SLAN_DNS_SERVICE_IP.to_string())
            .into_iter()
            .collect(),
        search_domains,
        split_domains,
        fallback_to_system_resolvers: false,
    }
}

fn prepare_relay_data_plane_from_latest_control(
) -> Result<PreparedRuntimeValue<RelayDataPlaneConfig>> {
    let mut prepared_session = prepare_session_device_registered(load_network_session()?)?;
    let prepared_relay_candidates = prepared_session.relay_candidates.clone();
    let prepared_network_configs = prepared_session.network_configs.clone();
    let session = &mut prepared_session.session;
    let Some(device_id) = session.device_id.clone().filter(|value| !value.is_empty()) else {
        return Err(anyhow::anyhow!(
            "device unavailable: current device is not registered"
        ));
    };
    let client = ControlPlaneClient::from_env();
    let network_id = ensure_active_network_id(session)?;
    let activation =
        client.activate_device_networks(session_device_api_token(session), &device_id)?;
    session.self_node_id = activation.self_node_id.clone();
    let relay_candidates = if activation.relay_candidates.is_empty() {
        if prepared_relay_candidates.is_empty() {
            runtime_relay_candidates()
        } else {
            prepared_relay_candidates
        }
    } else {
        activation
            .relay_candidates
            .iter()
            .map(persisted_relay_candidate)
            .collect()
    };
    session.virtual_ip = Some(activation.virtual_ip);
    let best_relay = data_plane_relay_candidate(&relay_candidates);
    let network_configs = client
        .device_network_configs(session_device_api_token(session), &device_id)
        .unwrap_or_else(|_| {
            if prepared_network_configs.is_empty() {
                crate::network_module::network_module_snapshot().configs
            } else {
                prepared_network_configs
            }
        });
    let config = build_relay_data_plane_config(
        &client,
        session,
        &network_id,
        activation.self_node_id.as_deref(),
        &activation.peers,
        &network_configs,
        best_relay.as_ref(),
        &platform_acl_policies(&network_configs),
        false,
    )?;
    prepared_session.relay_candidates = relay_candidates;
    prepared_session.network_configs = network_configs;
    Ok(PreparedRuntimeValue {
        value: config,
        prepared_session,
    })
}

fn android_data_plane_relay_candidate(
    candidates: &[PersistedRelayCandidate],
) -> Option<RelayCandidateSelection> {
    // Android now protects both UDP relay sockets and DERP TCP sockets through
    // VpnService before handing fds to Rust. Prefer UDP when available, and
    // fall back to DERP/TCP for TCP-only relay tests.
    best_udp_relay_candidate(candidates).or_else(|| best_relay_candidate(candidates))
}

fn data_plane_relay_candidate(
    candidates: &[PersistedRelayCandidate],
) -> Option<RelayCandidateSelection> {
    best_relay_candidate_for_connect_plans(candidates)
        .or_else(|| best_udp_relay_candidate(candidates))
        .or_else(|| best_relay_candidate(candidates))
        .or_else(|| relay_candidate_probe_fallback(candidates))
}

fn best_relay_candidate_for_connect_plans(
    candidates: &[PersistedRelayCandidate],
) -> Option<RelayCandidateSelection> {
    let selections = select_relay_candidates(candidates);
    if selections.is_empty() {
        return None;
    }
    load_recent_connect_plans(current_timestamp_ms())
        .iter()
        .flat_map(|plan| plan.paths.iter())
        .find_map(|path| relay_candidate_matching_connect_plan_path(path, &selections))
}

fn relay_candidate_matching_connect_plan_path(
    path: &PersistedConnectPlanPath,
    candidates: &[RelayCandidateSelection],
) -> Option<RelayCandidateSelection> {
    let transport = relay_transport_for_path_type(path.path_type.as_str())?;
    let address = normalize_relay_candidate_address(path.endpoint.as_str(), transport)?;
    candidates
        .iter()
        .find(|candidate| {
            candidate.reachable
                && normalize_relay_transport(candidate.transport.as_str()) == Some(transport)
                && candidate.address.trim() == address
        })
        .cloned()
}

fn load_network_session() -> Result<PersistedSession> {
    let session = load_session().map_err(|error| {
        if session_not_found_error(&error) {
            anyhow::anyhow!("login required")
        } else {
            error
        }
    })?;
    if session.access_token.trim().is_empty() {
        return Err(anyhow::anyhow!("login required"));
    }
    Ok(session)
}

#[derive(Clone)]
struct PreparedControlNetworkActivation {
    session: PersistedSession,
    relay_candidates: Vec<PersistedRelayCandidate>,
    network_configs: Vec<crate::control_plane::DeviceNetworkConfig>,
    prefix_len: u8,
    resolver: PlatformResolverConfig,
    resolver_zones: Vec<PlatformResolverZone>,
    resolver_records: Vec<PlatformResolverRecord>,
    routes: Vec<client_core::RouteSpec>,
    relay_config: Option<RelayDataPlaneConfig>,
}

struct PreparedRuntimeNetworkActivation {
    prepared: Result<PreparedControlNetworkActivation>,
}

fn prepare_latest_runtime_network_activation(
    _runtime: &RuntimeActorHandle,
) -> PreparedRuntimeNetworkActivation {
    let prepared = prepare_latest_control_network_activation();
    PreparedRuntimeNetworkActivation { prepared }
}

fn prepare_runtime_network_activation(
    _runtime: &RuntimeActorHandle,
    session: PersistedSession,
) -> PreparedRuntimeNetworkActivation {
    let prepared =
        prepare_session_device_registered(session).and_then(prepare_control_network_activation);
    PreparedRuntimeNetworkActivation { prepared }
}

fn prepare_latest_control_network_activation() -> Result<PreparedControlNetworkActivation> {
    let prepared_session = prepare_session_device_registered(load_network_session()?)?;
    prepare_control_network_activation(prepared_session)
}

fn prepare_control_network_activation(
    prepared_session: PreparedSession,
) -> Result<PreparedControlNetworkActivation> {
    let PreparedSession {
        mut session,
        relay_candidates: prepared_relay_candidates,
        network_configs: prepared_network_configs,
    } = prepared_session;
    let Some(device_id) = session.device_id.clone().filter(|value| !value.is_empty()) else {
        return Err(anyhow::anyhow!(
            "device unavailable: current device is not registered"
        ));
    };
    let client = ControlPlaneClient::from_env();
    let network_id = ensure_active_network_id(&mut session)?;
    let mut activation =
        client.activate_device_networks(session_device_api_token(&session), &device_id)?;
    activation.virtual_ip = normalize_virtual_ip(&activation.virtual_ip);
    session.self_node_id = activation.self_node_id.clone();
    log_service_error(format!(
        "client-core-service enable preflight ok: ip={}/{} resolvers={} routes={} peers={} relays={}",
        activation.virtual_ip,
        activation.prefix_len,
        activation.resolver.servers.len(),
        activation.routes.len(),
        activation.peer_count,
        activation.relay_candidates.len()
    ));
    session.virtual_ip = Some(activation.virtual_ip.clone());
    ensure_session_node_binding(&client, &mut session)
        .context("ensure node binding after network activation")?;
    let relay_candidates = if activation.relay_candidates.is_empty() {
        if prepared_relay_candidates.is_empty() {
            runtime_relay_candidates()
        } else {
            prepared_relay_candidates
        }
    } else {
        activation
            .relay_candidates
            .iter()
            .map(persisted_relay_candidate)
            .collect()
    };
    let best_relay = data_plane_relay_candidate(&relay_candidates);
    let network_configs = client
        .device_network_configs(session_device_api_token(&session), &device_id)
        .unwrap_or_else(|_| {
            if prepared_network_configs.is_empty() {
                crate::network_module::network_module_snapshot().configs
            } else {
                prepared_network_configs
            }
        });
    let acl_policies = platform_acl_policies(&network_configs);
    let relay_config = build_relay_data_plane_config(
        &client,
        &session,
        &network_id,
        activation.self_node_id.as_deref(),
        &activation.peers,
        &network_configs,
        best_relay.as_ref(),
        &acl_policies,
        false,
    )
    .map_err(|error| {
        log_service_error(format!(
            "client-core-service relay data plane config skipped: {error:#}"
        ));
        error
    })
    .ok();
    let routes = routes_with_peer_virtual_ips(
        activation.routes,
        &activation.peers,
        &network_configs,
        activation.virtual_ip.as_str(),
    );
    log_service_error(format!(
        "client-core-service enable apply start: prefix={} resolvers={} routes={} relay={}",
        activation.prefix_len,
        activation.resolver.servers.len(),
        routes.len(),
        relay_config.is_some()
    ));
    let resolver = platform_resolver_config(&network_id, &activation.resolver, &network_configs);
    let resolver_zones = platform_resolver_zones(&network_configs);
    let resolver_records = platform_resolver_records(&network_configs);
    Ok(PreparedControlNetworkActivation {
        session,
        relay_candidates,
        network_configs,
        prefix_len: activation.prefix_len,
        resolver,
        resolver_zones,
        resolver_records,
        routes,
        relay_config,
    })
}

fn commit_control_network_activation<P>(
    runtime: &mut ClientRuntime<P>,
    plan: PreparedControlNetworkActivation,
) -> Result<()>
where
    P: client_core::PlatformNetwork,
{
    ensure_runtime_session_matches(runtime, &plan.session)?;
    let virtual_ip = plan.session.virtual_ip.clone().unwrap_or_default();
    let prepared_session = PreparedSession {
        session: plan.session,
        relay_candidates: plan.relay_candidates,
        network_configs: plan.network_configs,
    };
    commit_registered_session(runtime, &prepared_session)?;
    log_service_error(format!(
        "client-core-service applying latest assigned IP locally: ip={virtual_ip}"
    ));
    runtime.apply_network_enabled_state(virtual_ip);
    log_service_error("client-core-service enable apply ok".to_string());
    Ok(())
}

fn execute_platform_network_activation(
    platform: &PlatformNetworkImpl,
    plan: &PreparedControlNetworkActivation,
) -> Result<()> {
    let virtual_ip = plan
        .session
        .virtual_ip
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .ok_or_else(|| anyhow::anyhow!("device unavailable: missing assigned virtual IP"))?;
    platform_transition::activate_network(
        platform,
        PlatformNetworkActivation {
            virtual_ip,
            prefix_len: plan.prefix_len,
            resolver: &plan.resolver,
            resolver_zones: &plan.resolver_zones,
            resolver_records: &plan.resolver_records,
            routes: &plan.routes,
            relay_config: plan.relay_config.as_ref(),
        },
    )
}

fn execute_runtime_network_activation(
    runtime: &RuntimeActorHandle,
    command_kind: impl Into<String>,
    correlation_id: Option<String>,
    transition: PreparedRuntimeNetworkActivation,
) -> Result<ClientViewState> {
    let command_kind = command_kind.into();
    // Check staleness and whether a virtual IP is assigned, without borrowing transition.prepared
    // for the entire function lifetime (which would conflict with the later move).
    let is_stale = transition.prepared.as_ref().ok().is_some_and(|plan| {
        !activation_context_matches(&runtime.snapshot().state, &plan.session)
            || !persisted_activation_context_matches(&plan.session)
    });
    if is_stale {
        log_service_error(
            "client-core-service skipped stale network activation before platform apply",
        );
        return Ok(runtime.snapshot().state);
    }
    let has_virtual_ip = transition.prepared.as_ref().ok().is_some_and(|plan| {
        plan.session
            .virtual_ip
            .as_deref()
            .map(str::trim)
            .is_some_and(|v| !v.is_empty())
    });
    if !has_virtual_ip {
        // No virtual IP assigned (device not in any network) — skip platform activation entirely.
        // The network can still be "enabled" in the UI, just without peers or an adapter IP.
        log_service_error(
            "client-core-service skipping platform activation: no virtual IP assigned (device not in any network)",
        );
        let plan = transition.prepared.ok();
        let committed = runtime.call_named(
            command_kind.clone(),
            correlation_id.clone(),
            move |runtime| {
                let executed: Result<PreparedControlNetworkActivation, _> =
                    plan.ok_or_else(|| anyhow::anyhow!("no activation plan"));
                Ok(commit_control_network_activation_result(runtime, executed))
            },
        )?;
        return Ok(committed.state);
    }
    let network_was_enabled = runtime.snapshot().state.network_enabled;
    let snapshots = runtime.snapshots();
    let executed = match transition.prepared {
        Ok(plan) => platform_transition::run_serialized_correlated(
            command_kind.clone(),
            correlation_id.clone(),
            move |platform| {
                if !activation_context_matches(&snapshots.latest().state, &plan.session)
                    || !persisted_activation_context_matches(&plan.session)
                {
                    anyhow::bail!("stale platform network activation");
                }
                let network_was_enabled = platform
                    .read_runtime_state()
                    .ok()
                    .is_some_and(|state| state.network_enabled);
                execute_platform_network_activation(platform, &plan)
                    .map(|()| plan)
                    .inspect_err(|_| {
                        if network_was_enabled {
                            log_service_error(
                                "client-core-service preserved existing network after reconfiguration failure",
                            );
                        } else {
                            let _ = platform_transition::disable_network(platform);
                        }
                    })
            },
        ),
        Err(error) => Err(error),
    };
    let committed = runtime.call_named(
        command_kind.clone(),
        correlation_id.clone(),
        move |runtime| {
            let executed = executed.and_then(|plan| {
                if !persisted_activation_context_matches(&plan.session) {
                    anyhow::bail!(
                        "stale network activation plan: persisted session changed during platform apply"
                    );
                }
                Ok(plan)
            });
            Ok(commit_control_network_activation_result(runtime, executed))
        },
    )?;
    if committed.rollback_platform {
        log_service_error("client-core-service rolled back failed network activation commit");
        if network_was_enabled {
            log_service_error(
                "client-core-service preserved existing network after activation commit rollback",
            );
        } else {
            let _ = disable_platform_network_serialized();
        }
    }
    Ok(committed.state)
}

fn activation_context_matches(state: &ClientViewState, session: &PersistedSession) -> bool {
    state.signed_in
        && normalized_device_id(state.device_id.as_deref())
            == normalized_device_id(session.device_id.as_deref())
}

fn persisted_activation_context_matches(session: &PersistedSession) -> bool {
    load_session().ok().is_some_and(|current| {
        normalized_device_id(current.device_id.as_deref())
            == normalized_device_id(session.device_id.as_deref())
            && current.device_session_id == session.device_session_id
            && current.access_token == session.access_token
            && current.active_network_id.as_deref().map(str::trim)
                == session.active_network_id.as_deref().map(str::trim)
    })
}

fn ensure_runtime_session_matches<P>(
    runtime: &ClientRuntime<P>,
    session: &PersistedSession,
) -> Result<()>
where
    P: client_core::PlatformNetwork,
{
    let planned_device_id = session
        .device_id
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .ok_or_else(|| anyhow::anyhow!("stale network activation plan: missing device id"))?;
    let current_device_id = runtime
        .state()
        .device_id
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty());
    if !runtime.state().signed_in || current_device_id != Some(planned_device_id) {
        anyhow::bail!("stale network activation plan: session changed during preflight");
    }
    Ok(())
}

struct ControlNetworkActivationCommit {
    state: ClientViewState,
    rollback_platform: bool,
}

fn commit_control_network_activation_result<P>(
    runtime: &mut ClientRuntime<P>,
    prepared: Result<PreparedControlNetworkActivation>,
) -> ControlNetworkActivationCommit
where
    P: client_core::PlatformNetwork,
{
    match prepared {
        Ok(plan) => match commit_control_network_activation(runtime, plan) {
            Ok(()) => {
                let mut state = runtime.state().clone();
                if !state.network_enabled {
                    state.virtual_ip = None;
                }
                ControlNetworkActivationCommit {
                    state,
                    rollback_platform: false,
                }
            }
            Err(error) => {
                if error
                    .to_string()
                    .starts_with("stale network activation plan:")
                {
                    log_service_error(format!(
                        "client-core-service ignored stale network activation plan (platform already configured): {error:#}"
                    ));
                    // Do NOT rollback the platform — the platform activation succeeded,
                    // so the adapter is already configured. A newer network event changed
                    // the persisted session during the platform apply, but that newer event
                    // will trigger its own activation. Rolling back here would tear down
                    // a working network, causing the adapter to go Disabled.
                    return ControlNetworkActivationCommit {
                        state: runtime.state().clone(),
                        rollback_platform: false,
                    };
                }
                log_service_error(format!(
                    "client-core-service activate network failed: {error:#}"
                ));
                let error_message = error.to_string();
                if session_auth_invalid_error(&error) || error_message.contains("session expired") {
                    runtime.apply_logout_state();
                }
                let _ = clear_session_virtual_ip();
                ControlNetworkActivationCommit {
                    state: state_with_error(runtime.state(), error_message),
                    rollback_platform: true,
                }
            }
        },
        Err(error) => {
            log_service_error(format!(
                "client-core-service activate network failed: {error:#}"
            ));
            let error_message = error.to_string();
            if session_auth_invalid_error(&error) || error_message.contains("session expired") {
                runtime.apply_logout_state();
            }
            let _ = clear_session_virtual_ip();
            ControlNetworkActivationCommit {
                state: state_with_error(runtime.state(), error_message),
                rollback_platform: false,
            }
        }
    }
}

fn ensure_active_network_id(session: &mut PersistedSession) -> Result<String> {
    session.active_network_id = session
        .active_network_id
        .take()
        .and_then(|value| non_empty_network_id(&value));
    if session.active_network_id.is_none() {
        let client = ControlPlaneClient::from_env();
        session.active_network_id = crate::network_module::network_module_snapshot()
            .configs
            .into_iter()
            .find_map(|config| non_empty_network_id(&config.network_id));
        if session.active_network_id.is_none() {
            session.active_network_id =
                client.active_network_id(session_device_api_token(session))?;
        }
    }
    // Return empty string if no network (allows enabling client without peers)
    Ok(session.active_network_id.clone().unwrap_or_default())
}

fn non_empty_network_id(value: &str) -> Option<String> {
    let value = value.trim();
    (!value.is_empty()).then(|| value.to_string())
}

fn prepare_relay_candidates_for_session(
    session: &PersistedSession,
    network_id: &str,
) -> Result<Vec<PersistedRelayCandidate>> {
    // No network — no relay candidates needed
    if network_id.trim().is_empty() {
        return Ok(Vec::new());
    }
    let device_id = session
        .device_id
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .ok_or_else(|| anyhow::anyhow!("device unavailable: current device is not registered"))?;
    let client = ControlPlaneClient::from_env();
    let candidates =
        client.relay_candidates(session_device_api_token(session), device_id, network_id)?;
    Ok(candidates.iter().map(persisted_relay_candidate).collect())
}

fn build_relay_data_plane_config(
    client: &ControlPlaneClient,
    session: &PersistedSession,
    network_id: &str,
    self_node_id: Option<&str>,
    peers: &[ControlPeer],
    network_configs: &[crate::control_plane::DeviceNetworkConfig],
    best_relay: Option<&RelayCandidateSelection>,
    acl_policies: &[PlatformAclPolicy],
    single_target_only: bool,
) -> Result<RelayDataPlaneConfig> {
    let relay = best_relay.ok_or_else(|| anyhow::anyhow!("no reachable relay candidate"))?;
    let relay_transport = normalize_relay_transport(&relay.transport).unwrap_or("udp");
    let relay_path_kind =
        relay_path_kind_for_transport(relay_transport).unwrap_or(PathKind::RelayUdp);
    let local_node_id = self_node_id
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .ok_or_else(|| anyhow::anyhow!("network map missing self node id"))?;
    let connect_plans = load_recent_connect_plans(current_timestamp_ms())
        .into_iter()
        .map(|plan| (plan.peer_node_id.clone(), plan))
        .collect::<BTreeMap<_, _>>();
    let punch_sessions = create_punch_connect_sessions(
        client,
        session,
        network_id,
        local_node_id,
        peers,
        network_configs,
    );

    let relay_candidates = select_relay_candidates(&runtime_relay_candidates());
    let relay_targets = relay_session_targets(best_relay, &relay_candidates, single_target_only);
    let sessions = peers
        .iter()
        .filter(|peer| peer.relay_allowed)
        .filter(|peer| peer.node_id != local_node_id)
        .flat_map(|peer| {
            let mut sessions = Vec::new();
            let peer_network_id = peer_network_id(network_id, peer, network_configs);
            if let Some(session) = connect_plans.get(&peer.node_id).and_then(|plan| {
                relay_session_from_connect_plan_ticket(
                    plan,
                    peer_network_id,
                    local_node_id,
                    peer,
                )
            }) {
                if relay_targets
                    .iter()
                    .any(|target| relay_session_matches_candidate(&session, target))
                {
                    sessions.push(session);
                }
            }
            for target in &relay_targets {
                if sessions
                    .iter()
                    .any(|session| relay_session_matches_candidate(session, target))
                {
                    continue;
                }
                let transport = normalize_relay_transport(target.transport.as_str()).unwrap_or("udp");
                let preferred_derp_node_id =
                    (transport == "derp_tcp_tls_443").then_some(target.endpoint_id.as_str());
                let preferred_relay_endpoint_id =
                    (transport != "derp_tcp_tls_443").then_some(target.endpoint_id.as_str());
                match client.issue_relay_ticket(
                    session_device_api_token(session),
                    peer_network_id,
                    local_node_id,
                    peer.node_id.as_str(),
                    target.cluster_id.as_deref(),
                    preferred_derp_node_id,
                    preferred_relay_endpoint_id,
                    target.region_id.as_deref(),
                ) {
                    Ok(ticket) => sessions.push(RelayPeerSession {
                        session_id: ticket.session_id.clone(),
                        peer_node_id: peer.node_id.clone(),
                        peer_virtual_ips: peer.virtual_ips.clone(),
                        ticket,
                    }),
                    Err(error) => {
                        log_service_error(format!(
                            "client-core-service issue relay ticket skipped: peerNodeId={} endpointId={} transport={} error={error:#}",
                            peer.node_id, target.endpoint_id, target.transport
                        ));
                    }
                }
            }
            sessions
        })
        .collect::<Vec<_>>();
    let sessions = select_relay_sessions_for_candidate(&sessions, relay_transport, relay);
    log_service_error(format!(
        "client-core-service relay config transport={} target={} sessions={} filtered_sessions=[{}]",
        relay_transport,
        relay.address,
        sessions.len(),
        sessions
            .iter()
            .map(relay_session_debug_summary)
            .collect::<Vec<_>>()
            .join(", ")
    ));
    let policy = relay_payload_policy(
        relay.address.as_str(),
        network_id,
        session.device_id.as_deref(),
        Some(relay_path_kind.as_str()),
    );
    let path_policy = relay_path_policy(
        network_id,
        session.device_id.as_deref(),
        Some(relay_path_kind.as_str()),
    );

    let relay_address = sessions
        .iter()
        .find(|session| relay_session_matches_candidate(session, relay))
        .or_else(|| sessions.first())
        .map(|session| session.ticket.relay_url.trim())
        .filter(|value| !value.is_empty())
        .map(str::to_string)
        .unwrap_or_else(|| relay.address.clone());
    let direct_udp_port = configured_direct_udp_port();
    Ok(RelayDataPlaneConfig {
        enabled: !sessions.is_empty(),
        transport: relay.transport.clone(),
        relay_address,
        local_node_id: local_node_id.to_string(),
        network_id: network_id.to_string(),
        direct_udp_port,
        randomize_direct_udp_port: direct_udp_port == 0,
        node_configs: session.node_configs.clone(),
        path_policy,
        peer_paths: peer_path_configs(
            peers,
            local_node_id,
            relay,
            &relay_candidates,
            &sessions,
            Some(connect_plans),
            Some(punch_sessions),
        ),
        relay_mtu: Some(policy.relay_mtu),
        max_frame_payload: Some(policy.max_frame_payload),
        acl_policies: acl_policies_for_network(acl_policies, network_id),
        sessions,
    })
}

#[cfg(test)]
fn filter_relay_sessions_for_transport(
    sessions: &[RelayPeerSession],
    relay_transport: &str,
) -> Vec<RelayPeerSession> {
    sessions
        .iter()
        .filter(|session| relay_session_transport_matches(session, relay_transport))
        .cloned()
        .collect()
}

fn select_relay_sessions_for_candidate(
    sessions: &[RelayPeerSession],
    relay_transport: &str,
    selected_relay: &RelayCandidateSelection,
) -> Vec<RelayPeerSession> {
    let mut selected = BTreeMap::<String, RelayPeerSession>::new();
    for session in sessions
        .iter()
        .filter(|session| relay_session_transport_matches(session, relay_transport))
    {
        let replace = selected.get(&session.peer_node_id).is_none_or(|current| {
            !relay_session_matches_candidate(current, selected_relay)
                && relay_session_matches_candidate(session, selected_relay)
        });
        if replace {
            selected.insert(session.peer_node_id.clone(), session.clone());
        }
    }
    selected.into_values().collect()
}

fn relay_session_transport_matches(session: &RelayPeerSession, relay_transport: &str) -> bool {
    let Some(expected_kind) = relay_path_kind_for_transport(relay_transport) else {
        return false;
    };
    relay_path_kind_from_relay_url(session.ticket.relay_url.as_str()) == Some(expected_kind)
}

fn relay_session_debug_summary(session: &RelayPeerSession) -> String {
    format!(
        "{}:{}:{}",
        session.peer_node_id, session.session_id, session.ticket.relay_url
    )
}

fn relay_path_kind_from_relay_url(relay_url: &str) -> Option<PathKind> {
    let (scheme, _) = relay_url.trim().split_once("://")?;
    match scheme.to_ascii_lowercase().as_str() {
        "udp" | "relay+udp" => Some(PathKind::RelayUdp),
        "derp" | "derp+tcp+tls" | "derp_tcp_tls_443" => Some(PathKind::DerpTcpTls443),
        _ => None,
    }
}

fn peer_path_configs(
    peers: &[ControlPeer],
    local_node_id: &str,
    relay: &RelayCandidateSelection,
    relay_candidates: &[RelayCandidateSelection],
    relay_sessions: &[RelayPeerSession],
    connect_plans: Option<BTreeMap<String, PersistedConnectPlan>>,
    punch_sessions: Option<BTreeMap<String, PunchConnectSession>>,
) -> Vec<PeerPathConfig> {
    let relay_only = relay_only_path_policy_enabled();
    let connect_plans = connect_plans.unwrap_or_else(|| {
        load_recent_connect_plans(current_timestamp_ms())
            .into_iter()
            .map(|plan| (plan.peer_node_id.clone(), plan))
            .collect::<BTreeMap<_, _>>()
    });
    peers
        .iter()
        .filter(|peer| peer.node_id != local_node_id)
        .map(|peer| {
            let punch_session = punch_sessions
                .as_ref()
                .and_then(|sessions| sessions.get(&peer.node_id));
            let mut candidates = Vec::new();
            let mut direct_addresses = Vec::new();
            if !relay_only {
                if let Some(address) = punch_session.and_then(punch_peer_direct_udp_address) {
                    direct_addresses.push(address.clone());
                    candidates.push(direct_path_candidate(PathKind::DirectUdp, &address));
                }
            }
            if !relay_only {
                if let Some(plan) = connect_plans.get(&peer.node_id) {
                    for path in &plan.paths {
                        let address = path.endpoint.trim();
                        if address.is_empty() {
                            continue;
                        }
                        if let Some(kind) = direct_path_kind_for_path_type(&path.path_type) {
                            if !valid_direct_candidate_address(address) {
                                continue;
                            }
                            if direct_addresses
                                .iter()
                                .any(|value: &String| value == address)
                            {
                                continue;
                            }
                            direct_addresses.push(address.to_string());
                            let mut candidate = direct_path_candidate(kind, address);
                            candidate.path_score = u32::try_from(path.priority).ok();
                            candidates.push(candidate);
                        } else if let Some(candidate) = relay_path_candidate_from_connect_plan(
                            path,
                            relay_sessions,
                            relay,
                            &peer.node_id,
                        ) {
                            push_unique_relay_path_candidate(&mut candidates, candidate);
                        }
                    }
                }
            } else if let Some(plan) = connect_plans.get(&peer.node_id) {
                for path in &plan.paths {
                    if let Some(candidate) = relay_path_candidate_from_connect_plan(
                        path,
                        relay_sessions,
                        relay,
                        &peer.node_id,
                    ) {
                        push_unique_relay_path_candidate(&mut candidates, candidate);
                    }
                }
            }
            if !relay_only {
                let mut endpoint_candidates = Vec::new();
                for endpoint in &peer.endpoints {
                    let address = endpoint.address.trim();
                    let Some(kind) = direct_path_kind_for_path_type(&endpoint.endpoint_type) else {
                        continue;
                    };
                    if address.is_empty() || !valid_direct_candidate_address(address) {
                        continue;
                    }
                    if let Some(existing) = candidates.iter_mut().find(|candidate| {
                        candidate.address.as_deref().map(str::trim) == Some(address)
                            && matches!(
                                candidate.kind,
                                PathKind::LanUdp | PathKind::Ipv6Udp | PathKind::DirectUdp
                            )
                    }) {
                        existing.kind = kind;
                        continue;
                    }
                    direct_addresses.push(address.to_string());
                    let mut candidate = direct_path_candidate(kind, address);
                    candidate.last_ok_at_ms = u64::try_from(endpoint.updated_at)
                        .ok()
                        .filter(|value| *value > 0)
                        .map(|value| value.saturating_mul(1_000));
                    endpoint_candidates.push(candidate);
                }
                endpoint_candidates.sort_by_key(|candidate| match candidate.kind {
                    PathKind::LanUdp => 0,
                    PathKind::Ipv6Udp => 1,
                    PathKind::DirectUdp => 2,
                    _ => 3,
                });
                candidates.extend(endpoint_candidates);
            }
            if let Some(selected_session) =
                relay_session_for_candidate(relay_sessions, &peer.node_id, relay)
            {
                let relay_transport = normalize_relay_transport(&relay.transport).unwrap_or("udp");
                let relay_path_kind =
                    relay_path_kind_for_transport(relay_transport).unwrap_or(PathKind::RelayUdp);
                push_unique_relay_path_candidate(
                    &mut candidates,
                    PathCandidate {
                        kind: relay_path_kind,
                        state: PathState::Standby,
                        endpoint_id: Some(relay.endpoint_id.clone()),
                        address: Some(relay.address.clone()),
                        session_id: Some(selected_session.session_id.clone()),
                        transport: Some(relay_transport.to_string()),
                        rtt_ms: relay.rtt_ms,
                        path_score: Some(relay.path_score),
                        last_ok_at_ms: None,
                        last_error: None,
                    },
                );
            }
            for candidate in relay_candidates {
                let Some(transport) = normalize_relay_transport(&candidate.transport) else {
                    continue;
                };
                let Some(path_kind) = relay_path_kind_for_transport(transport) else {
                    continue;
                };
                push_unique_relay_path_candidate(
                    &mut candidates,
                    PathCandidate {
                        kind: path_kind,
                        state: PathState::Standby,
                        endpoint_id: Some(candidate.endpoint_id.clone()),
                        address: Some(candidate.address.clone()),
                        session_id: None,
                        transport: Some(transport.to_string()),
                        rtt_ms: candidate.rtt_ms,
                        path_score: Some(candidate.path_score),
                        last_ok_at_ms: None,
                        last_error: None,
                    },
                );
            }
            client_core::sort_path_candidates(&mut candidates);
            PeerPathConfig {
                peer_node_id: peer.node_id.clone(),
                peer_virtual_ips: peer.virtual_ips.clone(),
                candidates,
            }
        })
        .collect()
}

fn relay_session_from_connect_plan_ticket(
    plan: &PersistedConnectPlan,
    network_id: &str,
    local_node_id: &str,
    peer: &ControlPeer,
) -> Option<RelayPeerSession> {
    let ticket = plan.relay_ticket.as_ref()?;
    if !relay_ticket_matches_peer(ticket, network_id, local_node_id, peer.node_id.as_str()) {
        return None;
    }
    Some(RelayPeerSession {
        session_id: ticket.session_id.clone(),
        peer_node_id: peer.node_id.clone(),
        peer_virtual_ips: peer.virtual_ips.clone(),
        ticket: ticket.clone(),
    })
}

fn relay_session_targets(
    best_relay: Option<&RelayCandidateSelection>,
    relay_candidates: &[RelayCandidateSelection],
    single_target_only: bool,
) -> Vec<RelayCandidateSelection> {
    let mut targets = Vec::new();
    if let Some(relay) = best_relay {
        push_unique_relay_target(&mut targets, relay.clone());
    }
    if single_target_only {
        return targets;
    }
    for transport in ["udp", "derp_tcp_tls_443"] {
        if let Some(relay) = relay_candidates.iter().find(|candidate| {
            candidate.reachable
                && normalize_relay_transport(candidate.transport.as_str()) == Some(transport)
        }) {
            push_unique_relay_target(&mut targets, relay.clone());
        }
    }
    targets
}

fn push_unique_relay_target(
    targets: &mut Vec<RelayCandidateSelection>,
    candidate: RelayCandidateSelection,
) {
    if targets.iter().any(|existing| {
        normalize_relay_transport(existing.transport.as_str())
            == normalize_relay_transport(candidate.transport.as_str())
    }) {
        return;
    }
    targets.push(candidate);
}

fn relay_session_for_candidate<'a>(
    sessions: &'a [RelayPeerSession],
    peer_node_id: &str,
    candidate: &RelayCandidateSelection,
) -> Option<&'a RelayPeerSession> {
    sessions.iter().find(|session| {
        session.peer_node_id == peer_node_id && relay_session_matches_candidate(session, candidate)
    })
}

fn relay_session_matches_candidate(
    session: &RelayPeerSession,
    candidate: &RelayCandidateSelection,
) -> bool {
    let Some(transport) = normalize_relay_transport(candidate.transport.as_str()) else {
        return false;
    };
    let Some(session_address) =
        normalize_relay_candidate_address(session.ticket.relay_url.as_str(), transport)
    else {
        return false;
    };
    session_address == candidate.address
}

fn relay_ticket_matches_peer(
    ticket: &RelayTicket,
    network_id: &str,
    local_node_id: &str,
    peer_node_id: &str,
) -> bool {
    if ticket.network_id.trim() != network_id
        || ticket.src_node_id.trim() != local_node_id
        || ticket.dst_node_id.trim() != peer_node_id
        || ticket.session_id.trim().is_empty()
        || ticket.session_key.trim().is_empty()
        || ticket.relay_url.trim().is_empty()
    {
        return false;
    }
    parse_rfc3339_utc_ms(ticket.expires_at.as_str())
        .map(|expires_at| expires_at > current_timestamp_ms().saturating_add(30_000))
        .unwrap_or(false)
}

fn create_punch_connect_sessions(
    client: &ControlPlaneClient,
    session: &PersistedSession,
    network_id: &str,
    local_node_id: &str,
    peers: &[ControlPeer],
    network_configs: &[crate::control_plane::DeviceNetworkConfig],
) -> BTreeMap<String, PunchConnectSession> {
    let device_id = session.device_id.as_deref().unwrap_or_default();
    peers
        .iter()
        .filter(|peer| peer.node_id != local_node_id)
        .filter_map(|peer| {
            let peer_network_id = peer_network_id(network_id, peer, network_configs);
            match client.create_punch_connect_session(
                session_device_api_token(session),
                device_id,
                session.mqtt.as_ref(),
                peer_network_id,
                local_node_id,
                peer.node_id.as_str(),
            ) {
                Ok(session) => Some((peer.node_id.clone(), session)),
                Err(error) => {
                    log_service_error(format!(
                        "client-core-service punch connect session skipped: peerNodeId={} error={error:#}",
                        peer.node_id
                    ));
                    None
                }
            }
        })
        .collect()
}

fn peer_network_id<'a>(
    fallback_network_id: &'a str,
    peer: &ControlPeer,
    network_configs: &'a [crate::control_plane::DeviceNetworkConfig],
) -> &'a str {
    let peer_device_id = peer
        .node_id
        .strip_prefix("node-")
        .unwrap_or(peer.node_id.as_str());
    network_configs
        .iter()
        .find(|config| {
            config
                .peers
                .iter()
                .any(|candidate| candidate.device_id.trim() == peer_device_id.trim())
        })
        .map(|config| config.network_id.as_str())
        .filter(|network_id| !network_id.trim().is_empty())
        .unwrap_or(fallback_network_id)
}

/// 从 punch 会话中选择对端 direct UDP 地址，优先使用服务端观测到的 reflexive 地址。
fn punch_peer_direct_udp_address(session: &PunchConnectSession) -> Option<String> {
    let endpoint = session.peer.as_ref()?;
    [endpoint.reflexive.as_str(), endpoint.address.as_str()]
        .into_iter()
        .map(str::trim)
        .find(|value| !value.is_empty())
        .map(str::to_string)
}

fn relay_path_candidate_from_connect_plan(
    path: &PersistedConnectPlanPath,
    relay_sessions: &[RelayPeerSession],
    selected_relay: &RelayCandidateSelection,
    peer_node_id: &str,
) -> Option<PathCandidate> {
    let transport = relay_transport_for_path_type(path.path_type.as_str())?;
    let address = normalize_relay_candidate_address(path.endpoint.as_str(), transport)?;
    let kind = relay_path_kind_for_transport(transport)?;
    let selected_transport = normalize_relay_transport(selected_relay.transport.as_str());
    let uses_selected_relay =
        selected_transport == Some(transport) && selected_relay.address == address;
    let relay_session = relay_sessions.iter().find(|session| {
        session.peer_node_id == peer_node_id
            && normalize_relay_candidate_address(session.ticket.relay_url.as_str(), transport)
                .as_deref()
                == Some(address.as_str())
    });
    Some(PathCandidate {
        kind,
        state: PathState::Standby,
        endpoint_id: uses_selected_relay.then(|| selected_relay.endpoint_id.clone()),
        address: Some(address),
        session_id: relay_session.map(|session| session.session_id.clone()),
        transport: Some(transport.to_string()),
        rtt_ms: None,
        path_score: u32::try_from(path.priority).ok(),
        last_ok_at_ms: None,
        last_error: None,
    })
}

fn relay_transport_for_path_type(path_type: &str) -> Option<&'static str> {
    match path_type.trim() {
        "relay_udp" => Some("udp"),
        "derp_tcp_tls_443" => Some("derp_tcp_tls_443"),
        _ => None,
    }
}

fn direct_path_kind_for_path_type(path_type: &str) -> Option<PathKind> {
    match path_type.trim() {
        "lan_udp" => Some(PathKind::LanUdp),
        "ipv6_udp" => Some(PathKind::Ipv6Udp),
        "direct_udp" => Some(PathKind::DirectUdp),
        _ => None,
    }
}

fn push_unique_relay_path_candidate(candidates: &mut Vec<PathCandidate>, candidate: PathCandidate) {
    if !relay_path_candidate_exists(candidates, &candidate) {
        candidates.push(candidate);
    }
}

fn relay_path_candidate_exists(candidates: &[PathCandidate], candidate: &PathCandidate) -> bool {
    candidates.iter().any(|existing| {
        existing.kind == candidate.kind
            && existing.address.as_deref().map(str::trim)
                == candidate.address.as_deref().map(str::trim)
    })
}

fn direct_path_candidate(kind: PathKind, address: &str) -> PathCandidate {
    PathCandidate {
        kind,
        state: PathState::Probing,
        endpoint_id: None,
        address: Some(address.to_string()),
        session_id: None,
        transport: Some("udp".to_string()),
        rtt_ms: None,
        path_score: None,
        last_ok_at_ms: None,
        last_error: None,
    }
}

fn valid_direct_candidate_address(address: &str) -> bool {
    let trimmed = address.trim();
    if trimmed.starts_with("relay+udp://") {
        return false;
    }
    let normalized = trimmed
        .strip_prefix("udp://")
        .or_else(|| trimmed.strip_prefix("direct+udp://"))
        .unwrap_or(trimmed);
    normalized
        .parse::<std::net::SocketAddr>()
        .map(|socket_addr| socket_addr.port() > 0)
        .unwrap_or(false)
}

fn routes_with_peer_virtual_ips(
    mut routes: Vec<client_core::RouteSpec>,
    peers: &[ControlPeer],
    network_configs: &[crate::control_plane::DeviceNetworkConfig],
    self_virtual_ip: &str,
) -> Vec<client_core::RouteSpec> {
    routes.retain(|route| managed_virtual_route_destination(&route.destination));
    let mut existing = routes
        .iter()
        .map(|route| normalize_route_destination(route.destination.as_str()))
        .collect::<std::collections::HashSet<_>>();
    let self_ip = normalize_virtual_ip_for_route(self_virtual_ip);
    for peer in peers {
        for ip in peer_route_ips(peer, network_configs) {
            let Some(peer_ip) = usable_peer_virtual_ip(ip.as_str()) else {
                continue;
            };
            if peer_ip == self_ip {
                continue;
            }
            let destination = format!("{peer_ip}/32");
            if existing.insert(destination.clone()) {
                routes.push(client_core::RouteSpec {
                    destination,
                    gateway: None,
                });
            }
        }
    }
    routes
}

fn peer_route_ips(
    peer: &ControlPeer,
    network_configs: &[crate::control_plane::DeviceNetworkConfig],
) -> Vec<String> {
    let device_id = peer
        .node_id
        .strip_prefix("node-")
        .unwrap_or(peer.node_id.as_str());
    network_configs
        .iter()
        .flat_map(|config| config.peers.iter())
        .filter(|candidate| candidate.device_id.trim() == device_id.trim())
        .filter_map(|candidate| candidate.global_ip.clone())
        .collect()
}

fn normalize_route_destination(value: &str) -> String {
    let trimmed = value.trim();
    if trimmed.contains('/') {
        trimmed.to_string()
    } else {
        format!("{trimmed}/32")
    }
}

fn usable_peer_virtual_ip(value: &str) -> Option<String> {
    let ip = normalize_virtual_ip_for_route(value);
    if !managed_virtual_ipv4(&ip) {
        return None;
    }
    Some(ip)
}

fn managed_virtual_route_destination(value: &str) -> bool {
    managed_virtual_ipv4(&normalize_virtual_ip_for_route(value))
}

fn managed_virtual_ipv4(value: &str) -> bool {
    value
        .parse::<std::net::Ipv4Addr>()
        .map(|address| address.octets()[0] == 10)
        .unwrap_or(false)
}

fn normalize_virtual_ip_for_route(value: &str) -> String {
    value
        .trim()
        .split_once('/')
        .map(|(ip, _)| ip)
        .unwrap_or_else(|| value.trim())
        .to_string()
}

fn diagnostic_session_summary() -> serde_json::Value {
    match load_session() {
        Ok(session) => serde_json::json!({
            "signedIn": !session.access_token.trim().is_empty(),
            "userId": session.user_id,
            "userLabel": session.user_label,
            "deviceId": session.device_id,
            "activeNetworkId": session.active_network_id,
            "virtualIp": session.virtual_ip,
            "relayCandidateCount": runtime_relay_candidates().len(),
            "mqttConfigured": session.mqtt.is_some(),
            "authenticatedAtMs": session.authenticated_at_ms,
        }),
        Err(error) => serde_json::json!({
            "signedIn": false,
            "error": error.to_string(),
        }),
    }
}

struct PreparedDownstreamNetworkAssignment {
    prepared_session: PreparedSession,
    assigned_ip: AssignedIpPayload,
    activation: Option<PreparedControlNetworkActivation>,
}

fn prepare_downstream_network_assignment(
    reactivate_network: bool,
) -> Result<PreparedDownstreamNetworkAssignment> {
    let session = load_session()?;
    let Some(device_id) = session.device_id.clone().filter(|value| !value.is_empty()) else {
        return Err(anyhow::anyhow!(
            "device unavailable: current device is not registered"
        ));
    };
    let client = ControlPlaneClient::from_env();
    let prepared_session = prepare_session_device_registered(session)?;
    let session = &prepared_session.session;
    let refreshed_device_id = session
        .device_id
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.trim().is_empty())
        .ok_or_else(|| anyhow::anyhow!("device unavailable: current device is not registered"))?;
    if refreshed_device_id != device_id {
        return Err(anyhow::anyhow!(
            "device unavailable: current device is not registered"
        ));
    }
    let virtual_ip = session
        .virtual_ip
        .clone()
        .filter(|value| !value.trim().is_empty())
        .ok_or_else(|| {
            anyhow::anyhow!("device unavailable: current device has no assigned virtual IP")
        })?;
    let prefix_len = session.active_network_id.as_deref().and_then(|network_id| {
        client
            .network_prefix_len(session_device_api_token(&session), network_id, None)
            .ok()
    });
    let assigned_ip = AssignedIpPayload {
        virtual_ip,
        prefix_len,
    };
    if reactivate_network {
        let activation = prepare_control_network_activation(prepared_session)?;
        return Ok(PreparedDownstreamNetworkAssignment {
            prepared_session: PreparedSession::from_session(activation.session.clone()),
            assigned_ip,
            activation: Some(activation),
        });
    }
    Ok(PreparedDownstreamNetworkAssignment {
        prepared_session,
        assigned_ip,
        activation: None,
    })
}

fn commit_downstream_network_assignment<P>(
    runtime: &mut ClientRuntime<P>,
    prepared: Result<PreparedDownstreamNetworkAssignment>,
) -> Result<()>
where
    P: client_core::PlatformNetwork,
{
    let prepared = prepared?;
    if let Some(activation) = prepared.activation {
        return commit_control_network_activation(runtime, activation);
    }
    ensure_runtime_session_matches(runtime, &prepared.prepared_session.session)?;
    commit_registered_session(runtime, &prepared.prepared_session)?;
    runtime.apply_assigned_ip_state(prepared.assigned_ip);
    Ok(())
}

fn execute_downstream_network_assignment(
    runtime: &RuntimeActorHandle,
    expected_revision: u64,
    correlation_id: Option<String>,
    prepared: Result<PreparedDownstreamNetworkAssignment>,
) -> Result<ClientViewState> {
    let needs_platform = prepared
        .as_ref()
        .ok()
        .and_then(|assignment| assignment.activation.as_ref())
        .is_some();
    if !needs_platform {
        let committed = runtime.call_named_if_revision(
            "control_task.network.reconcile.commit",
            correlation_id,
            expected_revision,
            move |runtime| {
                Ok(
                    match commit_downstream_network_assignment(runtime, prepared) {
                        Ok(()) => runtime.state().clone(),
                        Err(error) => state_with_error(runtime.state(), error.to_string()),
                    },
                )
            },
        )?;
        return Ok(committed.unwrap_or_else(|| runtime.snapshot().state));
    }
    if runtime.snapshot().revision != expected_revision {
        return Ok(runtime.snapshot().state);
    }
    let snapshots = runtime.snapshots();
    let network_was_enabled = runtime.snapshot().state.network_enabled;
    let prepared = platform_transition::run_serialized_correlated(
        "control_task.network.reconcile",
        correlation_id.clone(),
        move |platform| {
            if snapshots.latest().revision != expected_revision {
                anyhow::bail!("stale downstream platform network assignment");
            }
            prepared.and_then(|assignment| {
                if let Some(activation) = assignment.activation.as_ref() {
                    execute_platform_network_activation(platform, activation)?;
                }
                Ok(assignment)
            })
        },
    );
    if let Err(ref error) = prepared {
        let error_msg = format!("{error:#}");
        // A stale error means a newer network event already superseded this
        // reconcile. The newer event will trigger its own reconcile, so we
        // must NOT tear down the working network here.
        if error_msg.contains("stale") {
            log_service_error(format!(
                "client-core-service ignored stale reconcile error (network_was_enabled={network_was_enabled}): {error_msg}"
            ));
        } else {
            log_service_error(format!(
                "client-core-service downstream reconcile failed (network_was_enabled={network_was_enabled}): {error_msg}"
            ));
            if network_was_enabled {
                log_service_error(
                    "client-core-service preserved existing network after downstream reconcile failure",
                );
            } else {
                let _ = disable_platform_network_serialized();
            }
        }
    }
    let committed = runtime.call_named_if_revision(
        "control_task.network.reconcile.commit",
        correlation_id.clone(),
        expected_revision,
        move |runtime| {
            let platform_applied = prepared
                .as_ref()
                .ok()
                .and_then(|assignment| assignment.activation.as_ref())
                .is_some();
            let committed = match commit_downstream_network_assignment(runtime, prepared) {
                Ok(()) => (runtime.state().clone(), false),
                Err(error) => {
                    log_service_error(format!(
                        "client-core-service downstream network commit failed (platform_applied={platform_applied}): {error:#}"
                    ));
                    // If the platform activation succeeded, do NOT rollback — the adapter
                    // is already configured. A newer event may have changed the session,
                    // but that event will trigger its own activation.
                    (
                        state_with_error(runtime.state(), error.to_string()),
                        false,
                    )
                }
            };
            Ok(committed)
        },
    )?;
    if committed.is_none() {
        // Revision changed during commit — a newer event is handling the
        // network state. Do NOT disable the network here.
        log_service_error(
            "client-core-service skipped downstream reconcile rollback (revision changed during commit)",
        );
        return Ok(runtime.snapshot().state);
    }
    let (state, rollback_platform) = committed.unwrap();
    if rollback_platform {
        log_service_error("client-core-service rolled back failed downstream network commit");
        if network_was_enabled {
            log_service_error(
                "client-core-service preserved existing network after downstream commit rollback",
            );
        } else {
            let _ = disable_platform_network_serialized();
        }
    }
    Ok(state)
}

fn deactivate_control_network() {
    let Ok(session) = load_session() else {
        return;
    };
    let Some(device_id) = session
        .device_id
        .as_deref()
        .filter(|value| !value.is_empty())
    else {
        return;
    };
    let Some(network_id) = session
        .active_network_id
        .as_deref()
        .filter(|value| !value.is_empty())
    else {
        return;
    };
    let client = ControlPlaneClient::from_env();
    match client.deactivate_network(session_device_api_token(&session), device_id, network_id) {
        Ok(()) => log_service_error(format!(
            "client-core-service deactivated control network on logout/disable: deviceId={device_id} networkId={network_id}"
        )),
        Err(error) => log_service_error(format!(
            "client-core-service failed to deactivate control network on logout/disable: {error:#}"
        )),
    }
}

fn persisted_relay_candidate(candidate: &RelayCandidate) -> PersistedRelayCandidate {
    PersistedRelayCandidate {
        endpoint_id: candidate.endpoint_id.clone(),
        transport: candidate.transport.clone(),
        address: candidate.address.clone(),
        country_code: candidate.country_code.clone(),
        region_id: candidate.region_id.clone(),
        cluster_id: candidate.cluster_id.clone(),
        reachable_hint: candidate.reachable,
        observed_rtt_ms_hint: candidate.observed_rtt_ms,
        path_score_hint: candidate.path_score,
        selected_hint: candidate.selected,
    }
}

pub(crate) fn persist_relay_candidates_from_control_map(map: &Value) -> Result<usize> {
    let candidates = extract_persisted_relay_candidates_from_control_map(map);
    if candidates.is_empty() {
        return Ok(0);
    }
    Ok(replace_runtime_relay_candidates(candidates).len())
}

#[derive(Debug, Clone, serde::Deserialize, serde::Serialize)]
#[serde(rename_all = "camelCase")]
struct PersistedConnectPlan {
    peer_node_id: String,
    #[serde(default)]
    prefer_direct: bool,
    #[serde(default)]
    paths: Vec<PersistedConnectPlanPath>,
    #[serde(default)]
    relay_ticket: Option<RelayTicket>,
    updated_at_ms: u64,
}

#[derive(Debug, Clone, serde::Deserialize, serde::Serialize)]
#[serde(rename_all = "camelCase")]
struct PersistedConnectPlanPath {
    path_type: String,
    endpoint: String,
    #[serde(default)]
    priority: i32,
}

#[derive(Debug, Clone, Default, serde::Deserialize, serde::Serialize)]
#[serde(rename_all = "camelCase")]
struct PersistedConnectPlanStore {
    #[serde(default)]
    plans: Vec<PersistedConnectPlan>,
}

pub(crate) fn persist_connect_plan_from_value(value: &Value) -> Result<bool> {
    let mut plan: PersistedConnectPlan =
        serde_json::from_value(value.clone()).context("decode connect_plan payload")?;
    plan.peer_node_id = plan.peer_node_id.trim().to_string();
    if plan.peer_node_id.is_empty() {
        return Ok(false);
    }
    plan.paths = plan
        .paths
        .into_iter()
        .filter_map(|mut path| {
            path.path_type = path.path_type.trim().to_string();
            path.endpoint = path.endpoint.trim().to_string();
            (!path.path_type.is_empty() && !path.endpoint.is_empty()).then_some(path)
        })
        .collect();
    plan.paths.sort_by_key(|path| path.priority);
    let mut store = load_connect_plan_store();
    let changed = store
        .plans
        .iter()
        .find(|item| item.peer_node_id == plan.peer_node_id)
        .is_none_or(|item| !connect_plan_content_matches(item, &plan));
    plan.updated_at_ms = current_timestamp_ms();
    let cutoff = plan.updated_at_ms.saturating_sub(CONNECT_PLAN_TTL_MS);
    store
        .plans
        .retain(|item| item.peer_node_id != plan.peer_node_id && item.updated_at_ms >= cutoff);
    store.plans.push(plan);
    persist_connect_plan_store(&store)?;
    Ok(changed)
}

fn connect_plan_content_matches(left: &PersistedConnectPlan, right: &PersistedConnectPlan) -> bool {
    left.peer_node_id == right.peer_node_id
        && left.prefer_direct == right.prefer_direct
        && left.paths.len() == right.paths.len()
        && left.paths.iter().zip(&right.paths).all(|(left, right)| {
            left.path_type == right.path_type
                && left.endpoint == right.endpoint
                && left.priority == right.priority
        })
        && left.relay_ticket == right.relay_ticket
}

fn load_recent_connect_plans(now_ms: u64) -> Vec<PersistedConnectPlan> {
    let cutoff = now_ms.saturating_sub(CONNECT_PLAN_TTL_MS);
    load_connect_plan_store()
        .plans
        .into_iter()
        .filter(|plan| plan.updated_at_ms >= cutoff)
        .collect()
}

fn latest_connect_plan_updated_at_ms() -> u64 {
    load_recent_connect_plans(current_timestamp_ms())
        .into_iter()
        .map(|plan| plan.updated_at_ms)
        .max()
        .unwrap_or_default()
}

fn load_connect_plan_store() -> PersistedConnectPlanStore {
    RUNTIME_CONNECT_PLANS
        .get_or_init(|| Mutex::new(PersistedConnectPlanStore::default()))
        .lock()
        .expect("runtime connect plan store mutex poisoned")
        .clone()
}

fn persist_connect_plan_store(store: &PersistedConnectPlanStore) -> Result<()> {
    *RUNTIME_CONNECT_PLANS
        .get_or_init(|| Mutex::new(PersistedConnectPlanStore::default()))
        .lock()
        .expect("runtime connect plan store mutex poisoned") = store.clone();
    Ok(())
}

fn clear_session_virtual_ip() -> Result<()> {
    let Ok(mut session) = load_session() else {
        return Ok(());
    };
    if session.access_token.trim().is_empty() {
        let _ = remove_session();
        return Ok(());
    }
    session.virtual_ip = None;
    persist_session(&session)
}

fn state_with_error(state: &ClientViewState, error: String) -> ClientViewState {
    let mut state = state.clone();
    state.syncing = false;
    state.switch_enabled = true;
    if !state.network_enabled {
        state.virtual_ip = None;
    }
    state.error = Some(error);
    state
}

fn publish_business_event(
    state_notifier: &RuntimeEventHub,
    business_type: impl Into<String>,
    business_data: Value,
) {
    state_notifier.publish(business_type, business_data);
}

pub(crate) fn publish_state_business_event(
    state_notifier: &Arc<RuntimeEventHub>,
    business_type: impl Into<String>,
    state: &ClientViewState,
) {
    let business_data = serde_json::to_value(state).unwrap_or_else(|_| serde_json::json!({}));
    publish_business_event(state_notifier, business_type, business_data);
}

pub(crate) fn publish_state_business_event_with_extra(
    state_notifier: &Arc<RuntimeEventHub>,
    business_type: impl Into<String>,
    state: &ClientViewState,
    extra_business_data: Value,
) {
    let mut business_data = serde_json::to_value(state).unwrap_or_else(|_| serde_json::json!({}));
    if let (Value::Object(current), Value::Object(extra)) =
        (&mut business_data, extra_business_data)
    {
        for (key, value) in extra {
            current.insert(key, value);
        }
    }
    publish_business_event(state_notifier, business_type, business_data);
}

fn publish_method_business_event(
    state_notifier: &RuntimeEventHub,
    request_line: &str,
    response: &str,
) {
    let request = serde_json::from_str::<ServiceRequest>(request_line).ok();
    let method = request
        .as_ref()
        .map(|request| LocalServiceMethod::parse(&request.method))
        .unwrap_or(LocalServiceMethod::Other);
    let command_type = request
        .as_ref()
        .filter(|_| method == LocalServiceMethod::Dispatch)
        .and_then(|request| request.args.get("type"))
        .and_then(Value::as_str);
    let state = serde_json::from_str::<ClientViewState>(response).ok();
    let business_type = method_business_event_type(method, command_type, state.as_ref());
    let mut business_data = state
        .and_then(|state| serde_json::to_value(state).ok())
        .unwrap_or_else(|| serde_json::json!({}));
    if let (Some(event_id), Value::Object(data)) = (
        request
            .as_ref()
            .and_then(|request| request_correlation_id(&request.args)),
        &mut business_data,
    ) {
        data.insert("eventId".to_string(), Value::String(event_id));
    }
    publish_business_event(state_notifier, business_type, business_data);
}

fn method_business_event_type(
    method: LocalServiceMethod,
    command_type: Option<&str>,
    state: Option<&ClientViewState>,
) -> &'static str {
    let switch_result = || {
        state
            .and_then(|state| state.error.as_ref())
            .map(|_| BUSINESS_NETWORK_SWITCH_FAILED)
            .unwrap_or(BUSINESS_NETWORK_SWITCH_FINISHED)
    };
    match method {
        LocalServiceMethod::LocalNetworkActivate
        | LocalServiceMethod::LocalNetworkDeactivate
        | LocalServiceMethod::LocalNetworkShutdown => switch_result(),
        LocalServiceMethod::LocalLogout => BUSINESS_SESSION_CHANGED,
        LocalServiceMethod::Dispatch => match command_type.unwrap_or_default() {
            "enableNetwork" | "disableNetwork" => switch_result(),
            "loginWithPassword" | "applyDeviceUserLogin" | "logout" => BUSINESS_SESSION_CHANGED,
            "syncAssignedIp" | "applyPlatformRuntimeState" => BUSINESS_NETWORK_RUNTIME_CHANGED,
            _ => BUSINESS_STATE_CHANGED,
        },
        _ => BUSINESS_STATE_CHANGED,
    }
}

fn error_state_json(error: String) -> String {
    let state = state_with_error(&ClientViewState::default(), error);
    serde_json::to_string(&state).unwrap_or_else(|_| "{}".to_string())
}

fn spawn_session_refresh_worker(runtime: RuntimeActorHandle, state_notifier: Arc<RuntimeEventHub>) {
    thread::spawn(move || {
        let mut auth_invalid_since = None;
        loop {
            thread::sleep(SESSION_REFRESH_INTERVAL);
            let expected_snapshot = runtime.snapshot();
            let expected_state = expected_snapshot.state.clone();
            match refresh_logged_in_session() {
                Ok(Some(prepared)) => {
                    auth_invalid_since = None;
                    let before = expected_state.clone();
                    let correlation_id = prepared.session.device_id.clone();
                    let state = runtime
                        .call_named_if_revision(
                            "session.refresh.apply",
                            correlation_id,
                            expected_snapshot.revision,
                            move |runtime| {
                                if !runtime_login_context_matches(
                                    runtime,
                                    expected_state.device_id.as_deref(),
                                    expected_state.signed_in,
                                ) {
                                    anyhow::bail!("stale session refresh");
                                }
                                commit_registered_session(runtime, &prepared)
                            },
                        )
                        .map(|state| state.unwrap_or_else(|| runtime.snapshot().state))
                        .unwrap_or_else(|error| state_with_error(&before, error.to_string()));
                    if state != before {
                        let business_data =
                            serde_json::to_value(&state).unwrap_or_else(|_| serde_json::json!({}));
                        publish_business_event(
                            &state_notifier,
                            BUSINESS_SESSION_CHANGED,
                            business_data,
                        );
                    }
                }
                Ok(None) => {
                    auth_invalid_since = None;
                }
                Err(error) => {
                    log_service_error(format!(
                        "client-core-service session refresh skipped: {error:#}"
                    ));
                    let auth_invalid = session_auth_invalid_error(&error)
                        || error.to_string().contains("session expired");
                    if !auth_invalid {
                        auth_invalid_since = None;
                        continue;
                    }
                    let first_failure = auth_invalid_since.get_or_insert_with(Instant::now);
                    if first_failure.elapsed() < SESSION_AUTH_INVALID_GRACE {
                        continue;
                    }
                    {
                        let state = invalidate_runtime_session(
                            &runtime,
                            "session.refresh.logout",
                            expected_snapshot.revision,
                            expected_snapshot.state.device_id,
                            expected_snapshot.state.signed_in,
                        )
                        .ok()
                        .flatten()
                        .unwrap_or_else(|| runtime.snapshot().state);
                        let business_data =
                            serde_json::to_value(state).unwrap_or_else(|_| serde_json::json!({}));
                        publish_business_event(
                            &state_notifier,
                            BUSINESS_SESSION_CHANGED,
                            business_data,
                        );
                    }
                }
            }
        }
    });
}

fn invalidate_runtime_session(
    runtime: &RuntimeActorHandle,
    command_kind: &'static str,
    expected_revision: u64,
    expected_device_id: Option<String>,
    expected_signed_in: bool,
) -> Result<Option<ClientViewState>> {
    if runtime.snapshot().revision != expected_revision {
        return Ok(None);
    }
    let snapshots = runtime.snapshots();
    if let Err(error) = platform_transition::run_serialized_correlated(
        format!("{command_kind}.platform"),
        expected_device_id.clone(),
        move |platform| {
            if snapshots.latest().revision != expected_revision {
                anyhow::bail!("stale invalid session platform transition");
            }
            platform_transition::disable_network(platform)
        },
    ) {
        log_service_error(format!(
            "client-core-service invalid session platform disable failed: {error:#}"
        ));
    }
    runtime.call_named_if_revision(
        command_kind,
        expected_device_id.clone(),
        expected_revision,
        move |runtime| {
            Ok(commit_logout(
                runtime,
                expected_device_id,
                expected_signed_in,
            ))
        },
    )
}

fn refresh_logged_in_session() -> Result<Option<PreparedSession>> {
    let session = match load_session() {
        Ok(session) => session,
        Err(error) => {
            if session_not_found_error(&error) {
                return Ok(None);
            }
            return Err(error);
        }
    };
    if session.access_token.trim().is_empty() {
        return Ok(None);
    }
    prepare_session_device_registered(session).map(Some)
}

fn spawn_runtime_sync_worker(runtime: RuntimeActorHandle, state_notifier: Arc<RuntimeEventHub>) {
    thread::spawn(move || loop {
        if let Err(error) = platform_transition::refresh_runtime_state() {
            log_service_error(format!(
                "client-core-service platform runtime refresh failed: {error:#}"
            ));
        }
        sync_control_assignment(&runtime);
        let before = runtime.snapshot().state;
        // Guard: a periodic platform read is observational and must never
        // disable an already-enabled network. If the platform cache
        // spuriously reports network_enabled=false (e.g. during a Windows
        // adapter state transition), applying it would tear down the working
        // network. Explicit disable flows (logout, deactivation) handle
        // network teardown through their own dedicated paths.
        let platform_snapshot = platform_transition::snapshot();
        if before.network_enabled && !platform_snapshot.runtime_state.network_enabled {
            log_service_error(format!(
                "client-core-service periodic refresh guard: skipping network disable — before_ip={:?} platform_ip={:?} platform_enabled={}",
                before.virtual_ip,
                platform_snapshot.runtime_state.virtual_ip,
                platform_snapshot.runtime_state.network_enabled,
            ));
            // Still report the current (enabled) state, but do NOT apply the
            // platform read that would disable the network.
            if before.network_enabled {
                report_runtime_state(&before);
            }
            thread::sleep(Duration::from_secs(10));
            continue;
        }
        let state = commit_runtime_refresh(&runtime, "runtime.periodic.refresh", None)
            .unwrap_or_else(|error| state_with_error(&before, error.to_string()));
        if before.network_enabled && !state.network_enabled {
            log_service_error(format!(
                "client-core-service runtime refresh disabled network: before_ip={:?} after_ip={:?} notice={:?} error={:?}",
                before.virtual_ip, state.virtual_ip, state.notice, state.error
            ));
        }
        if state != before {
            let business_type = if state.signed_in != before.signed_in {
                BUSINESS_SESSION_CHANGED
            } else if state.network_enabled != before.network_enabled
                || state.virtual_ip != before.virtual_ip
            {
                BUSINESS_NETWORK_RUNTIME_CHANGED
            } else {
                BUSINESS_STATE_CHANGED
            };
            let business_data =
                serde_json::to_value(&state).unwrap_or_else(|_| serde_json::json!({}));
            publish_business_event(&state_notifier, business_type, business_data);
        }
        // A periodic platform read is observational. In particular, Windows may
        // briefly have no initialized runtime state while the service restores
        // the adapter after restart. Do not turn that transient state into a
        // control-plane network deactivation; explicit disable flows report it.
        if state.network_enabled {
            report_runtime_state(&state);
        }
        thread::sleep(Duration::from_secs(10));
    });
}

fn spawn_platform_diagnostics_worker() {
    thread::spawn(move || {
        thread::sleep(Duration::from_secs(2));
        loop {
            if let Err(error) = platform_transition::refresh_platform_diagnostics() {
                log_service_error(format!(
                    "client-core-service platform diagnostics refresh failed: {error:#}"
                ));
            }
            thread::sleep(Duration::from_secs(60));
        }
    });
}

fn spawn_platform_late_completion_worker(
    runtime: RuntimeActorHandle,
    state_notifier: Arc<RuntimeEventHub>,
) {
    thread::spawn(move || {
        let mut revision = 0;
        loop {
            let Some(completion) =
                platform_transition::wait_late_completion_after(revision, Duration::from_secs(120))
            else {
                continue;
            };
            revision = completion.revision;
            let _ = runtime.wait_for_command_completion(
                &completion.kind,
                completion.correlation_id.as_deref(),
                completion.caller_timed_out_at_ms,
                Duration::from_secs(2),
            );
            let before = runtime.snapshot().state;
            let state = commit_runtime_refresh(
                &runtime,
                "platform.late_completion.commit",
                completion.correlation_id.clone(),
            )
            .unwrap_or_else(|error| state_with_error(&before, error.to_string()));
            let business_type = if completion.succeeded {
                BUSINESS_NETWORK_RUNTIME_CHANGED
            } else {
                BUSINESS_NETWORK_SWITCH_FAILED
            };
            publish_state_business_event_with_extra(
                &state_notifier,
                business_type,
                &state,
                serde_json::json!({
                    "eventId": format!("platform-late-{}", completion.operation_id),
                    "requestId": completion.correlation_id,
                    "platformOperationId": completion.operation_id,
                    "platformOperationKind": completion.kind,
                    "platformLateCompletion": true,
                    "platformOperationSucceeded": completion.succeeded,
                    "platformOperationError": completion.error,
                    "platformOperationFinishedAtMs": completion.finished_at_ms,
                }),
            );
            report_runtime_state(&state);
        }
    });
}

#[derive(Debug, Clone, PartialEq, Eq)]
struct LocalResolverDesiredState {
    enabled: bool,
    bind_addr: String,
    requester_device_id: Option<String>,
    signed_in: bool,
    network_enabled: bool,
    has_requester_device_id: bool,
    has_resolver_data: bool,
}

fn spawn_local_resolver_supervisor(runtime: RuntimeActorHandle) {
    thread::spawn(move || {
        log_service_error("client-core-service local resolver supervisor started");
        let mut server: Option<ResolverServer> = None;
        let mut bound_addr: Option<String> = None;
        let mut last_desired: Option<LocalResolverDesiredState> = None;
        loop {
            let desired = desired_local_resolver_state(&runtime);
            if last_desired.as_ref() != Some(&desired) {
                log_service_error(format!(
                    "client-core-service local resolver supervisor desired changed enabled={} signedIn={} networkEnabled={} hasRequester={} hasResolverData={} bind={}",
                    desired.enabled,
                    desired.signed_in,
                    desired.network_enabled,
                    desired.has_requester_device_id,
                    desired.has_resolver_data,
                    desired.bind_addr,
                ));
                last_desired = Some(desired.clone());
            }
            if !desired.enabled {
                server = None;
                bound_addr = None;
                set_local_resolver_server_status(LocalResolverServerStatus {
                    enabled: false,
                    listening: false,
                    bind_addr: Some(desired.bind_addr),
                    requester_device_id: desired.requester_device_id,
                    last_error: None,
                    last_query_at_ms: local_resolver_server_last_query_at_ms(),
                    desired_enabled: Some(desired.enabled),
                    desired_signed_in: Some(desired.signed_in),
                    desired_network_enabled: Some(desired.network_enabled),
                    desired_has_requester_device_id: Some(desired.has_requester_device_id),
                    desired_has_resolver_data: Some(desired.has_resolver_data),
                });
                thread::sleep(Duration::from_millis(500));
                continue;
            }

            let should_rebind =
                server.is_none() || bound_addr.as_deref() != Some(desired.bind_addr.as_str());
            if should_rebind {
                log_service_error(format!(
                    "client-core-service local resolver supervisor binding {}",
                    desired.bind_addr
                ));
                match ResolverServer::bind(&desired.bind_addr) {
                    Ok(next) => {
                        let listening_addr = next
                            .local_addr()
                            .map(|value| value.to_string())
                            .unwrap_or_else(|_| desired.bind_addr.clone());
                        bound_addr = Some(listening_addr.clone());
                        server = Some(next);
                        set_local_resolver_server_status(LocalResolverServerStatus {
                            enabled: true,
                            listening: true,
                            bind_addr: Some(listening_addr),
                            requester_device_id: desired.requester_device_id.clone(),
                            last_error: None,
                            last_query_at_ms: local_resolver_server_last_query_at_ms(),
                            desired_enabled: Some(desired.enabled),
                            desired_signed_in: Some(desired.signed_in),
                            desired_network_enabled: Some(desired.network_enabled),
                            desired_has_requester_device_id: Some(desired.has_requester_device_id),
                            desired_has_resolver_data: Some(desired.has_resolver_data),
                        });
                        log_service_error(format!(
                            "client-core-service local resolver supervisor bound {}",
                            bound_addr.as_deref().unwrap_or("")
                        ));
                    }
                    Err(error) => {
                        server = None;
                        bound_addr = None;
                        set_local_resolver_server_status(LocalResolverServerStatus {
                            enabled: true,
                            listening: false,
                            bind_addr: Some(desired.bind_addr.clone()),
                            requester_device_id: desired.requester_device_id.clone(),
                            last_error: Some(error.to_string()),
                            last_query_at_ms: local_resolver_server_last_query_at_ms(),
                            desired_enabled: Some(desired.enabled),
                            desired_signed_in: Some(desired.signed_in),
                            desired_network_enabled: Some(desired.network_enabled),
                            desired_has_requester_device_id: Some(desired.has_requester_device_id),
                            desired_has_resolver_data: Some(desired.has_resolver_data),
                        });
                        log_service_error(format!(
                            "client-core-service local resolver supervisor bind failed: {error}"
                        ));
                        thread::sleep(Duration::from_secs(1));
                        continue;
                    }
                }
            }

            {
                let mut status = local_resolver_server_status()
                    .lock()
                    .expect("local resolver server status mutex poisoned");
                status.enabled = true;
                status.listening = server.is_some();
                status.bind_addr = bound_addr
                    .clone()
                    .or_else(|| Some(desired.bind_addr.clone()));
                status.requester_device_id = desired.requester_device_id.clone();
                status.desired_enabled = Some(desired.enabled);
                status.desired_signed_in = Some(desired.signed_in);
                status.desired_network_enabled = Some(desired.network_enabled);
                status.desired_has_requester_device_id = Some(desired.has_requester_device_id);
                status.desired_has_resolver_data = Some(desired.has_resolver_data);
            }
            let session = load_session().ok();
            let network = runtime_network_state_store()
                .snapshot_for_session(session.as_ref())
                .state;
            let requester_device_id = desired.requester_device_id.clone().unwrap_or_default();
            let Some(active_server) = server.as_ref() else {
                thread::sleep(Duration::from_millis(250));
                continue;
            };
            let mut resolver = resolver_runtime_state()
                .lock()
                .expect("resolver runtime mutex poisoned");
            if let Err(error) =
                active_server.serve_once(&network, &mut resolver, &requester_device_id)
            {
                log_service_error(format!(
                    "client-core-service local resolver supervisor serve_once failed: {error:#}"
                ));
                {
                    let mut status = local_resolver_server_status()
                        .lock()
                        .expect("local resolver server status mutex poisoned");
                    status.last_error = Some(error.to_string());
                    status.listening = false;
                }
                server = None;
                bound_addr = None;
                thread::sleep(Duration::from_secs(1));
            }
        }
    });
}

fn desired_local_resolver_state(runtime: &RuntimeActorHandle) -> LocalResolverDesiredState {
    let state = runtime.snapshot().state;
    let resolver = resolver_runtime_state()
        .lock()
        .expect("resolver runtime mutex poisoned");
    let has_resolver_data = resolver.has_resolver_data();
    let requester_device_id = state
        .device_id
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .map(str::to_string);
    let has_requester_device_id = requester_device_id.is_some();
    LocalResolverDesiredState {
        enabled: state.signed_in
            && state.network_enabled
            && has_requester_device_id
            && has_resolver_data,
        bind_addr: desired_local_resolver_bind_addr(),
        requester_device_id,
        signed_in: state.signed_in,
        network_enabled: state.network_enabled,
        has_requester_device_id,
        has_resolver_data,
    }
}

fn spawn_control_task_worker(
    runtime: RuntimeActorHandle,
    task_queue: Arc<Mutex<ControlTaskQueue>>,
    state_notifier: Arc<RuntimeEventHub>,
) {
    thread::spawn(move || loop {
        thread::sleep(Duration::from_secs(1));
        let task = {
            let mut task_queue = task_queue
                .lock()
                .expect("control task queue mutex poisoned");
            task_queue.take_next_pending().unwrap_or_default()
        };
        let Some(task) = task else {
            continue;
        };

        let before = runtime.snapshot();
        let require_ui_refresh = task.require_ui_refresh;
        let state = execute_control_task(&runtime, &task_queue, task);
        let after = runtime.snapshot();
        if require_ui_refresh || after.revision != before.revision {
            let business_type = if state.error.is_some() {
                BUSINESS_NETWORK_SWITCH_FAILED
            } else if state.signed_in != before.state.signed_in {
                BUSINESS_SESSION_CHANGED
            } else if state.network_enabled != before.state.network_enabled
                || state.virtual_ip != before.state.virtual_ip
            {
                BUSINESS_NETWORK_RUNTIME_CHANGED
            } else {
                BUSINESS_STATE_CHANGED
            };
            publish_state_business_event(&state_notifier, business_type, &state);
        }
    });
}

#[derive(Default)]
struct RelayMaintenanceState {
    last_reconfigure_ms: u64,
    last_failure_total: u64,
    last_attach_failures: u64,
    last_connect_plan_ms: u64,
    last_tun_packets_sent: u64,
    last_relay_packets_received: u64,
    no_rx_intervals: u32,
}

fn spawn_relay_data_plane_maintenance_worker(
    runtime: RuntimeActorHandle,
    state_notifier: Arc<RuntimeEventHub>,
) {
    thread::spawn(move || {
        let mut maintenance = RelayMaintenanceState::default();
        loop {
            thread::sleep(RELAY_MAINTENANCE_INTERVAL);
            if let Err(error) =
                maintain_relay_data_plane(&runtime, &state_notifier, &mut maintenance)
            {
                log_service_error(format!(
                    "client-core-service relay data plane maintenance skipped: {error:#}"
                ));
            }
        }
    });
}

fn maintain_relay_data_plane(
    runtime: &RuntimeActorHandle,
    state_notifier: &RuntimeEventHub,
    maintenance: &mut RelayMaintenanceState,
) -> Result<()> {
    let now = current_timestamp_ms();
    let before = runtime.snapshot().state;
    if !before.signed_in || !before.network_enabled {
        return Ok(());
    }
    let session = load_network_session()?;
    if session
        .active_network_id
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .is_none()
    {
        return Ok(());
    }
    if runtime_relay_candidates().is_empty() {
        return Ok(());
    }
    let stats = load_relay_runtime_stats();
    let connect_plan_updated_at_ms = latest_connect_plan_updated_at_ms();
    let reconfigure_reason = relay_maintenance_reconfigure_reason(
        now,
        stats.as_ref(),
        maintenance,
        connect_plan_updated_at_ms,
    );
    let Some(reason) = reconfigure_reason else {
        return Ok(());
    };
    if relay_reconfigure_backoff_applies(reason)
        && maintenance.last_reconfigure_ms > 0
        && now.saturating_sub(maintenance.last_reconfigure_ms) < RELAY_RECONFIGURE_BACKOFF_MS
    {
        return Ok(());
    }
    maintenance.last_reconfigure_ms = now;
    log_service_error(format!(
        "client-core-service relay data plane reconfigure started: reason={reason}"
    ));
    let transition = prepare_runtime_network_activation(runtime, session);
    let correlation_id = transition
        .prepared
        .as_ref()
        .ok()
        .and_then(|plan| plan.session.device_id.clone())
        .or_else(|| Some(reason.to_string()));
    let state = execute_runtime_network_activation(
        runtime,
        "relay.data_plane.reconfigure.commit",
        correlation_id,
        transition,
    )?;
    if state.error.is_none() {
        report_runtime_state(&state);
    }
    let next_stats = load_relay_runtime_stats();
    if let Some(stats) = next_stats.as_ref() {
        maintenance.last_failure_total = relay_runtime_failure_total(stats);
        maintenance.last_attach_failures = stats.relay_attach_failures;
        maintenance.last_tun_packets_sent = stats.tun_packets_sent;
        maintenance.last_relay_packets_received = stats.relay_packets_received;
        maintenance.no_rx_intervals = 0;
    } else {
        maintenance.last_failure_total = 0;
        maintenance.last_attach_failures = 0;
        maintenance.last_tun_packets_sent = 0;
        maintenance.last_relay_packets_received = 0;
        maintenance.no_rx_intervals = 0;
    }
    let business_type = if state.error.is_some() {
        BUSINESS_NETWORK_SWITCH_FAILED
    } else {
        BUSINESS_NETWORK_RUNTIME_CHANGED
    };
    let business_data = serde_json::to_value(&state).unwrap_or_else(|_| serde_json::json!({}));
    publish_business_event(state_notifier, business_type, business_data);
    Ok(())
}

fn relay_maintenance_reconfigure_reason(
    now: u64,
    stats: Option<&RelayRuntimeStats>,
    maintenance: &mut RelayMaintenanceState,
    connect_plan_updated_at_ms: u64,
) -> Option<&'static str> {
    if maintenance.last_reconfigure_ms == 0 {
        maintenance.last_connect_plan_ms = connect_plan_updated_at_ms;
        if let Some(stats) = stats {
            maintenance.last_failure_total = relay_runtime_failure_total(stats);
            maintenance.last_attach_failures = stats.relay_attach_failures;
            maintenance.last_tun_packets_sent = stats.tun_packets_sent;
            maintenance.last_relay_packets_received = stats.relay_packets_received;
            maintenance.no_rx_intervals = 0;
            if relay_ticket_expired(now, stats.ticket_expires_at.as_deref()) {
                return Some("ticket_expired");
            }
            if relay_ticket_should_renew(now, stats.ticket_expires_at.as_deref()) {
                return Some("ticket_expiring");
            }
        }
        maintenance.last_reconfigure_ms = now;
        return None;
    }
    if connect_plan_updated_at_ms > maintenance.last_connect_plan_ms {
        maintenance.last_connect_plan_ms = connect_plan_updated_at_ms;
        return Some("connect_plan_updated");
    }
    let Some(stats) = stats else {
        return Some("missing_relay_stats");
    };
    if relay_ticket_expired(now, stats.ticket_expires_at.as_deref()) {
        return Some("ticket_expired");
    }
    if relay_ticket_should_renew(now, stats.ticket_expires_at.as_deref()) {
        return Some("ticket_expiring");
    }
    if now.saturating_sub(maintenance.last_reconfigure_ms) >= RELAY_TICKET_RENEW_INTERVAL_MS {
        return Some("ticket_renew");
    }
    if now.saturating_sub(stats.updated_at_ms) > RELAY_STATS_STALE_MS {
        return Some("stale_relay_stats");
    }
    if relay_runtime_idle(now, stats) {
        return Some("relay_runtime_idle");
    }
    if relay_sessions_missing(stats) {
        return Some("relay_session_missing");
    }
    // Missing replies may simply mean that every peer is offline. Rebuilding the
    // local adapter cannot repair that condition and previously caused a restart
    // loop roughly once per minute. Path diagnostics still report the response gap.
    let _ = relay_response_stalled(stats, maintenance);
    if stats.relay_attach_failures > maintenance.last_attach_failures {
        maintenance.last_attach_failures = stats.relay_attach_failures;
        return Some("relay_attach_failure");
    }
    let failure_total = relay_runtime_failure_total(stats);
    if failure_total.saturating_sub(maintenance.last_failure_total)
        >= RELAY_FAILURE_RECONFIGURE_DELTA
    {
        maintenance.last_failure_total = failure_total;
        return Some("relay_failure_delta");
    }
    maintenance.last_failure_total = failure_total;
    maintenance.last_attach_failures = stats.relay_attach_failures;
    None
}

fn relay_reconfigure_backoff_applies(reason: &str) -> bool {
    reason != "ticket_expired"
}

fn relay_runtime_idle(now_ms: u64, stats: &RelayRuntimeStats) -> bool {
    if stats.requested_relay_session_count == 0 || relay_attached_peer_session_count(stats) == 0 {
        return false;
    }
    let last_activity = [
        stats.last_tun_packet_at_ms,
        stats.last_relay_packet_at_ms,
        stats.last_relay_keepalive_at_ms,
        Some(stats.updated_at_ms),
    ]
    .into_iter()
    .flatten()
    .max()
    .unwrap_or(0);
    last_activity > 0 && now_ms.saturating_sub(last_activity) > RELAY_IDLE_RECONFIGURE_MS
}

fn relay_sessions_missing(stats: &RelayRuntimeStats) -> bool {
    stats.requested_relay_session_count > 0
        && relay_attached_peer_session_count(stats) < stats.requested_relay_session_count
}

fn relay_response_stalled(
    stats: &RelayRuntimeStats,
    maintenance: &mut RelayMaintenanceState,
) -> bool {
    let tun_delta = stats
        .tun_packets_sent
        .saturating_sub(maintenance.last_tun_packets_sent);
    let relay_rx_delta = stats
        .relay_packets_received
        .saturating_sub(maintenance.last_relay_packets_received);
    maintenance.last_tun_packets_sent = stats.tun_packets_sent;
    maintenance.last_relay_packets_received = stats.relay_packets_received;

    if stats.requested_relay_session_count == 0
        || relay_attached_peer_session_count(stats) == 0
        || tun_delta == 0
    {
        maintenance.no_rx_intervals = 0;
        return false;
    }
    if relay_rx_delta > 0 {
        maintenance.no_rx_intervals = 0;
        return false;
    }
    maintenance.no_rx_intervals = maintenance.no_rx_intervals.saturating_add(1);
    maintenance.no_rx_intervals >= RELAY_NO_RX_RECONFIGURE_INTERVALS
}

fn relay_response_gap(tun_packets_sent: u64, relay_packets_received: u64) -> u64 {
    tun_packets_sent.saturating_sub(relay_packets_received)
}

fn relay_response_gap_is_active(relay: &PathDiagnoseRelay) -> bool {
    if relay_response_gap(relay.tun_packets_sent, relay.relay_packets_received)
        < RELAY_RESPONSE_GAP_DEGRADED_PACKETS
    {
        return false;
    }
    let Some(last_tun_packet_at_ms) = relay.last_tun_packet_at_ms else {
        return false;
    };
    match relay.last_relay_packet_at_ms {
        Some(last_relay_packet_at_ms) => last_tun_packet_at_ms > last_relay_packet_at_ms,
        None => true,
    }
}

fn relay_ticket_should_renew(now_ms: u64, ticket_expires_at: Option<&str>) -> bool {
    relay_ticket_timing(now_ms, ticket_expires_at).renew_due
}

fn relay_ticket_expired(now_ms: u64, ticket_expires_at: Option<&str>) -> bool {
    relay_ticket_timing(now_ms, ticket_expires_at)
        .expires_in_ms
        .is_some_and(|expires_in_ms| expires_in_ms <= 0)
}

fn relay_ticket_timing(now_ms: u64, ticket_expires_at: Option<&str>) -> TicketTiming {
    ticket_timing_with_window(now_ms, ticket_expires_at, RELAY_TICKET_RENEW_WINDOW_MS)
}

pub(crate) fn sync_control_assignment(runtime: &RuntimeActorHandle) {
    let expected_snapshot = runtime.snapshot();
    let Ok(session) = load_session() else {
        return;
    };
    if session.access_token.trim().is_empty() {
        return;
    }
    let mut session =
        match prepare_and_commit_assignment_session(runtime, "assignment.session.refresh", session)
        {
            Ok(session) => session,
            Err(error) => {
                if session_auth_invalid_error(&error) {
                    let _ = invalidate_runtime_session(
                        runtime,
                        "assignment.invalid_session.logout",
                        expected_snapshot.revision,
                        expected_snapshot.state.device_id,
                        expected_snapshot.state.signed_in,
                    );
                }
                return;
            }
        };
    let client = ControlPlaneClient::from_env();
    if session.active_network_id.is_none() {
        if let Ok(network_id) = client.active_network_id(session_device_api_token(&session)) {
            session.active_network_id = network_id;
        }
    }

    let Some(device_id) = session.device_id.clone().filter(|value| !value.is_empty()) else {
        let _ =
            prepare_and_commit_assignment_session(runtime, "assignment.device.register", session);
        return;
    };
    let Ok(devices) = client.list_devices(&session.access_token) else {
        persist_assignment_session(runtime, session);
        return;
    };
    let Some(device) = devices.into_iter().find(|item| item.device_id == device_id) else {
        let _ =
            prepare_and_commit_assignment_session(runtime, "assignment.device.restore", session);
        return;
    };
    sync_session_device_fields(&mut session, &device);
    if !control_device_network_available(&device) {
        log_service_error(format!(
            "client-core-service downstream disabled local network: deviceStatus={:?} memberStatus={:?}",
            device.status, device.membership_status
        ));
        session.virtual_ip = None;
        persist_assignment_session(runtime, session);
        let _ = handle_network_deactivate(
            runtime,
            "assignment.network.disable",
            Some(device_id.clone()),
        );
        return;
    }
    let assigned_ip = device
        .current_virtual_ip
        .or(device.virtual_ip)
        .filter(|value| !value.trim().is_empty());
    let Some(assigned_ip) = assigned_ip else {
        log_service_error(
            "client-core-service downstream disabled local network: device list has no assigned virtual IP",
        );
        session.virtual_ip = None;
        persist_assignment_session(runtime, session);
        let _ = handle_network_deactivate(
            runtime,
            "assignment.network.disable",
            Some(device_id.clone()),
        );
        return;
    };
    if session.virtual_ip.as_deref() == Some(assigned_ip.as_str()) {
        persist_assignment_session(runtime, session);
        return;
    }
    session.virtual_ip = Some(assigned_ip.clone());
    let prefix_len_access_token = session.access_token.clone();
    let prefix_len_network_id = session.active_network_id.clone();
    persist_assignment_session(runtime, session);
    log_service_error(format!(
        "client-core-service syncing latest assigned IP locally: ip={assigned_ip}"
    ));
    let prefix_len = prefix_len_network_id.as_deref().and_then(|network_id| {
        client
            .network_prefix_len(&prefix_len_access_token, network_id, None)
            .ok()
    });
    let _ = execute_runtime_assigned_ip(
        runtime,
        "assignment.ip.sync",
        Some(device_id),
        AssignedIpPayload {
            virtual_ip: assigned_ip,
            prefix_len,
        },
    );
}

fn persist_assignment_session(runtime: &RuntimeActorHandle, session: PersistedSession) {
    let expected_revision = runtime.snapshot().revision;
    let fallback = PreparedSession::from_session(session.clone());
    let prepared = prepare_session_device_registered(session).unwrap_or(fallback);
    if let Err(error) = commit_assignment_session(
        runtime,
        "assignment.session.persist",
        expected_revision,
        prepared,
    ) {
        log_service_error(format!(
            "client-core-service assignment session commit skipped: {error:#}"
        ));
    }
}

fn prepare_and_commit_assignment_session(
    runtime: &RuntimeActorHandle,
    command_kind: &'static str,
    session: PersistedSession,
) -> Result<PersistedSession> {
    let expected_revision = runtime.snapshot().revision;
    let prepared = prepare_session_device_registered(session)?;
    commit_assignment_session(runtime, command_kind, expected_revision, prepared)
}

fn commit_assignment_session(
    runtime: &RuntimeActorHandle,
    command_kind: &'static str,
    expected_revision: u64,
    prepared: PreparedSession,
) -> Result<PersistedSession> {
    let session = prepared.session.clone();
    let committed_session = session.clone();
    let committed = runtime.call_named_if_revision(
        command_kind,
        session.device_id.clone(),
        expected_revision,
        move |runtime| {
            ensure_runtime_session_matches(runtime, &prepared.session)?;
            commit_registered_session(runtime, &prepared)?;
            Ok(())
        },
    )?;
    if committed.is_none() {
        anyhow::bail!("stale assignment session preparation");
    }
    Ok(committed_session)
}

fn control_device_network_available(device: &control_plane::ControlDevice) -> bool {
    !status_is_managed_disabled(device.status.as_deref())
        && !status_is_managed_disabled(device.membership_status.as_deref())
}

fn status_is_managed_disabled(status: Option<&str>) -> bool {
    let Some(status) = status else {
        return false;
    };
    matches!(
        status.trim().to_ascii_lowercase().as_str(),
        "disabled" | "suspended" | "blocked" | "revoked" | "deleted" | "removed"
    )
}

fn should_sync_control(throttle: &Arc<Mutex<ControlSyncThrottle>>) -> bool {
    let now = current_timestamp_ms();
    let mut throttle = throttle.lock().expect("control sync throttle poisoned");
    if now.saturating_sub(throttle.last_sync_ms) < 5_000 {
        return false;
    }
    throttle.last_sync_ms = now;
    true
}

fn mark_control_sync(throttle: &Arc<Mutex<ControlSyncThrottle>>) {
    let mut throttle = throttle.lock().expect("control sync throttle poisoned");
    throttle.last_sync_ms = current_timestamp_ms();
}
