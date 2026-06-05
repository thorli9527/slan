#![cfg_attr(target_os = "windows", windows_subsystem = "windows")]

mod acl_policy;
mod client_message_mqtt;
mod control_plane;
mod control_tasks;
mod control_transport;
mod control_transport_worker;
mod local_api;
#[cfg(test)]
mod main_tests;
mod network_module;
mod relay_candidates;
mod relay_models;
mod relay_store;
mod session_store;
mod time_utils;

use std::{
    collections::BTreeMap,
    fs::{self, OpenOptions},
    io::{BufRead, BufReader, ErrorKind, Write},
    net::{TcpListener, TcpStream},
    sync::{Arc, Condvar, Mutex, OnceLock},
    thread,
    time::{Duration, UNIX_EPOCH},
};

#[cfg(target_os = "windows")]
use std::{
    ffi::OsString,
    process::{Command, Stdio},
    time::Instant,
};

use anyhow::{Context, Result};
use client_core::{
    normalize_relay_transport, normalize_virtual_ip, relay_path_kind_for_transport,
    AssignedIpPayload, ClientCommand, ClientRuntime, ClientViewState, PathCandidate, PathKind,
    PathState, PeerPathConfig, PlatformAclPolicy, PlatformDeviceNetworkConfig, PlatformNetwork,
    PlatformNetworkConfig, PlatformNetworkDiagnostics, RelayDataPlaneConfig, RelayPeerSession,
    RelayTicket, TrafficStatsPayload,
};
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
use crate::control_tasks::{
    ControlTaskAction, ControlTaskDirection, ControlTaskQueue, EnqueueControlTaskRequest,
};
use crate::control_transport::{
    ControlTransportCadence, ControlTransportOutbox, ControlTransportOutboxRequest,
    ControlTransportPlan, ControlTransportStatus, ControlTransportTickPlan,
    ControlTransportTickRequest, PublishedControlTransportMessage,
};
use crate::control_transport_worker::ControlTransportWorkerState;
use crate::local_api::{
    LocalPathPlanResponse, LocalPeerView, LocalPeersResponse, LocalServiceMethod,
    LocalSessionResponse, LocalStatusResponse, MarkControlAckedRequest,
    PlatformRuntimeStateReportRequest, ServiceRequest, StoredBusinessEvent,
    WatchBusinessEventRequest, WatchBusinessEventResponse, WatchStateRequest, WatchStateResponse,
    BUSINESS_CONTROL_SYNC_CHANGED, BUSINESS_NETWORK_RUNTIME_CHANGED,
    BUSINESS_NETWORK_SWITCH_FAILED, BUSINESS_NETWORK_SWITCH_FINISHED, BUSINESS_SESSION_CHANGED,
    BUSINESS_STATE_CHANGED,
};
use crate::relay_candidates::{
    best_relay_candidate, best_udp_relay_candidate, diagnose_direct_candidates,
    extract_persisted_relay_candidates_from_network_map, normalize_relay_candidate_address,
    relay_candidate_probe_fallback, replace_runtime_relay_candidates, runtime_relay_candidates,
    select_relay_candidates,
};
use crate::relay_models::{
    PathDiagnoseDns, PathDiagnoseHealth, PathDiagnoseHealthReason, PathDiagnoseMtu,
    PathDiagnosePathCount, PathDiagnoseRelay, PathDiagnoseRelayPeer, PathDiagnoseResponse,
    PersistedRelayCandidate, RelayCandidateListResponse, RelayCandidateSelection,
    RelayRuntimeStats,
};
use crate::relay_store::{
    diagnostics_export_file_path, load_relay_runtime_stats, relay_path_policy,
    relay_payload_policy, relay_runtime_failure_total, relay_stats_file_path,
};
use crate::session_store::{
    app_data_dir, current_timestamp_ms, ensure_session_device_registered,
    ensure_session_node_and_control_session, hydrate_session_from_control_plane, load_session,
    load_valid_registered_session, persist_session, prepare_client_login_session,
    refresh_startup_session, remove_session, report_runtime_state, revoke_remote_sessions,
    session_auth_invalid_error, session_is_expired, sync_session_device_fields, PersistedSession,
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

#[cfg(target_os = "windows")]
define_windows_service!(ffi_service_main, service_main);

#[derive(Debug, Default)]
struct ControlSyncThrottle {
    last_sync_ms: u64,
}

#[derive(Debug, Default)]
pub(crate) struct StateChangeNotifier {
    revision: Mutex<u64>,
    event: Mutex<Option<StoredBusinessEvent>>,
    changed: Condvar,
}

#[derive(Clone)]
struct LocalServiceContext {
    runtime: Arc<Mutex<ClientRuntime<PlatformNetworkImpl>>>,
    task_queue: Arc<Mutex<ControlTaskQueue>>,
    transport_worker_state: Arc<Mutex<ControlTransportWorkerState>>,
    sync_throttle: Arc<Mutex<ControlSyncThrottle>>,
    state_notifier: Arc<StateChangeNotifier>,
}

#[derive(Debug, Clone, Copy)]
struct TrafficSample {
    tx_bytes: u64,
    rx_bytes: u64,
    updated_at_ms: u64,
}

static LAST_TRAFFIC_SAMPLE: OnceLock<Mutex<Option<TrafficSample>>> = OnceLock::new();

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum CommandOrigin {
    Upstream,
    Downstream,
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
        PlatformNetworkImpl.install_adapter()?;
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
    if let Some(session) =
        load_valid_registered_session().filter(|session| !session.access_token.trim().is_empty())
    {
        let _ = initial_runtime.dispatch(ClientCommand::ApplyDeviceUserLogin(session.into()));
    }
    if let Err(error) = PlatformNetworkImpl.install_adapter() {
        log_service_error(format!(
            "client-core-service failed to prepare Wintun adapter: {error:#}"
        ));
    }
    let runtime = Arc::new(Mutex::new(initial_runtime));
    let task_queue = Arc::new(Mutex::new(ControlTaskQueue::load_default()));
    let sync_throttle = Arc::new(Mutex::new(ControlSyncThrottle::default()));
    let transport_worker_state = Arc::new(Mutex::new(ControlTransportWorkerState::default()));
    let state_notifier = Arc::new(StateChangeNotifier::default());
    let service_context = LocalServiceContext {
        runtime: Arc::clone(&runtime),
        task_queue: Arc::clone(&task_queue),
        transport_worker_state: Arc::clone(&transport_worker_state),
        sync_throttle: Arc::clone(&sync_throttle),
        state_notifier: Arc::clone(&state_notifier),
    };
    spawn_session_refresh_worker(Arc::clone(&runtime), Arc::clone(&state_notifier));
    spawn_runtime_sync_worker(Arc::clone(&runtime), Arc::clone(&state_notifier));
    spawn_control_task_worker(
        Arc::clone(&runtime),
        Arc::clone(&task_queue),
        Arc::clone(&state_notifier),
    );
    control_transport_worker::spawn_control_transport_supervisor(
        Arc::clone(&runtime),
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
    spawn_relay_data_plane_maintenance_worker(Arc::clone(&runtime), Arc::clone(&state_notifier));
    println!("client-core-service listening on {bind_address}");

    loop {
        match listener.accept() {
            Ok((stream, _peer)) => {
                let context = service_context.clone();
                thread::spawn(move || {
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
    let response = route_request(line.trim(), &context).unwrap_or_else(|error| {
        let message = error.to_string();
        log_service_error(format!("client-core-service request failed: {message}"));
        error_state_json(message)
    });
    stream.write_all(response.as_bytes())?;
    stream.write_all(b"\n")?;
    stream.flush()?;
    Ok(())
}

fn route_request(line: &str, context: &LocalServiceContext) -> Result<String> {
    let request: ServiceRequest = serde_json::from_str(line).context("decode service request")?;
    let method = LocalServiceMethod::parse(&request.method);
    match method {
        LocalServiceMethod::LocalStateWatch => {
            return handle_watch_state(request, &context.runtime, &context.state_notifier)
        }
        LocalServiceMethod::LocalBusinessEventWatch => {
            return handle_watch_business_event(request, &context.runtime, &context.state_notifier)
        }
        LocalServiceMethod::LocalState => return handle_state_snapshot(&context.runtime),
        LocalServiceMethod::LocalStatus => return handle_local_status(&context.runtime),
        LocalServiceMethod::LocalSession => return handle_local_session(),
        LocalServiceMethod::LocalPeers => return handle_local_peers(),
        LocalServiceMethod::LocalNetworkModule => {
            match load_session() {
                Ok(session) => {
                    let client = ControlPlaneClient::from_env();
                    if let Err(err) = crate::network_module::refresh_network_module_from_session(
                        &client, &session,
                    ) {
                        eprintln!("client-core-service localNetworkModule refresh failed: {err:#}");
                    }
                }
                Err(_) => crate::network_module::clear_network_module(),
            }
            return serde_json::to_string(&crate::network_module::network_module_snapshot())
                .context("encode local network module");
        }
        LocalServiceMethod::LocalPathPlan => return handle_local_path_plan(),
        LocalServiceMethod::LocalPathDiagnose => return handle_path_diagnose(),
        LocalServiceMethod::LocalRelayCandidates => return handle_relay_candidates(false),
        LocalServiceMethod::LocalRefreshRelayCandidates => return handle_relay_candidates(true),
        LocalServiceMethod::LocalRelayPrepare => return handle_prepare_relay_data_plane(),
        LocalServiceMethod::LocalControlStatus => {
            return serde_json::to_string(&control_transport_status()?)
                .context("encode local control status")
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
        LocalServiceMethod::IngestPlatformRuntimeState => {
            return handle_ingest_platform_runtime_state(
                request,
                &context.runtime,
                &context.state_notifier,
            )
        }
        LocalServiceMethod::LocalSendClientMessage => return handle_send_client_message(request),
        LocalServiceMethod::LocalDiagnosticsExport => {
            return handle_export_diagnostics(&context.runtime)
        }
        LocalServiceMethod::Start => {
            sync_control_assignment(&context.runtime);
            mark_control_sync(&context.sync_throttle);
        }
        LocalServiceMethod::Refresh if should_sync_control(&context.sync_throttle) => {
            sync_control_assignment(&context.runtime);
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
    let should_wake_control_mqtt = method == LocalServiceMethod::Dispatch
        && matches!(
            request.args.get("type").and_then(Value::as_str),
            Some("openClientLogin" | "loginWithPassword" | "applyDeviceUserLogin")
        );
    let response = handle_request(request, &context.runtime)?;
    if should_wake_control_mqtt {
        control_transport_worker::wake_control_transport_worker(
            &context.runtime,
            &context.task_queue,
            &context.transport_worker_state,
            &context.state_notifier,
        );
    }
    if should_notify {
        publish_method_business_event(&context.state_notifier, line, &response);
    }
    Ok(response)
}

fn publish_control_sync_event(
    state_notifier: &Arc<StateChangeNotifier>,
    method: LocalServiceMethod,
) {
    if let Some(method) = method.control_sync_event_method() {
        publish_business_event(
            state_notifier,
            BUSINESS_CONTROL_SYNC_CHANGED,
            serde_json::json!({ "method": method }),
        );
    }
}

fn handle_state_snapshot(
    runtime: &Arc<Mutex<ClientRuntime<PlatformNetworkImpl>>>,
) -> Result<String> {
    let state = {
        let mut runtime = runtime.lock().expect("client runtime mutex poisoned");
        match runtime.refresh() {
            Ok(()) => runtime.state().clone(),
            Err(error) => state_with_error(runtime.state(), error.to_string()),
        }
    };
    serde_json::to_string(&state).context("encode client state")
}

fn handle_local_status(runtime: &Arc<Mutex<ClientRuntime<PlatformNetworkImpl>>>) -> Result<String> {
    let state = {
        let mut runtime = runtime.lock().expect("client runtime mutex poisoned");
        match runtime.refresh() {
            Ok(()) => runtime.state().clone(),
            Err(error) => state_with_error(runtime.state(), error.to_string()),
        }
    };
    let session = load_session().ok();
    let runtime_state = PlatformNetworkImpl.read_runtime_state();
    let (active_path, peer_count, runtime_error) = match runtime_state {
        Ok(runtime_state) => (
            runtime_state
                .active_path
                .and_then(|path| serde_json::to_value(path).ok()),
            runtime_state.peer_paths.len(),
            None,
        ),
        Err(error) => (None, 0, Some(error.to_string())),
    };
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
        peer_count,
        relay_candidate_count: runtime_relay_candidates().len(),
        connect_plan_count: load_recent_connect_plans(current_timestamp_ms()).len(),
        error: state.error,
        runtime_error,
    };
    serde_json::to_string(&response).context("encode local status")
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
                signed_in: !session.access_token.trim().is_empty(),
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
    let runtime_state = PlatformNetworkImpl
        .read_runtime_state()
        .context("read local peer runtime state")?;
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

fn handle_request(
    request: ServiceRequest,
    runtime: &Arc<Mutex<ClientRuntime<PlatformNetworkImpl>>>,
) -> Result<String> {
    let mut runtime = runtime.lock().expect("client runtime mutex poisoned");
    let method = LocalServiceMethod::parse(&request.method);
    let state = match method {
        LocalServiceMethod::Start => {
            refresh_startup_session(&mut runtime);
            match runtime.refresh() {
                Ok(()) => runtime.state().clone(),
                Err(error) => state_with_error(runtime.state(), error.to_string()),
            }
        }
        LocalServiceMethod::Dispatch => {
            let command: ClientCommand =
                serde_json::from_value(request.args).context("decode client command")?;
            let is_open_client_login = matches!(command, ClientCommand::OpenClientLogin);
            let mut state = dispatch_with_side_effects(&mut runtime, command);
            if is_open_client_login
                && state
                    .device_id
                    .as_deref()
                    .map(str::trim)
                    .filter(|value| !value.is_empty())
                    .is_none()
            {
                state.device_id = local_stable_device_id().ok();
            }
            state
        }
        LocalServiceMethod::LocalNetworkActivate => {
            activate_network_from_latest_control(&mut runtime)
        }
        LocalServiceMethod::LocalNetworkDeactivate => {
            dispatch_with_side_effects(&mut runtime, ClientCommand::DisableNetwork)
        }
        LocalServiceMethod::Refresh => match runtime.refresh() {
            Ok(()) => runtime.state().clone(),
            Err(error) => state_with_error(runtime.state(), error.to_string()),
        },
        LocalServiceMethod::LocalNetworkShutdown => {
            dispatch_with_side_effects(&mut runtime, ClientCommand::DisableNetwork)
        }
        LocalServiceMethod::ConsoleLoginKey => {
            return serde_json::to_string(&console_login_key()?).context("encode console login key")
        }
        LocalServiceMethod::LocalLogout => {
            dispatch_with_side_effects(&mut runtime, ClientCommand::Logout)
        }
        LocalServiceMethod::Other => {
            anyhow::bail!("unsupported service method {}", request.method)
        }
        _ => {
            anyhow::bail!(
                "service method {} is handled outside the runtime command interface",
                request.method
            )
        }
    };
    serde_json::to_string(&state).context("encode client state")
}

fn handle_local_platform_network_config(
    runtime: &Arc<Mutex<ClientRuntime<PlatformNetworkImpl>>>,
) -> Result<String> {
    let mut runtime = runtime.lock().expect("client runtime mutex poisoned");
    serde_json::to_string(&platform_network_config_from_latest_control(&mut runtime)?)
        .context("encode local platform network config")
}

fn handle_ingest_platform_runtime_state(
    request: ServiceRequest,
    runtime: &Arc<Mutex<ClientRuntime<PlatformNetworkImpl>>>,
    state_notifier: &Arc<StateChangeNotifier>,
) -> Result<String> {
    let report: PlatformRuntimeStateReportRequest =
        serde_json::from_value(request.args).context("decode platform runtime state report")?;
    let state = {
        let mut runtime = runtime.lock().expect("client runtime mutex poisoned");
        let mut state = runtime
            .dispatch(ClientCommand::ApplyPlatformRuntimeState(
                report.runtime_state.clone(),
            ))
            .unwrap_or_else(|error| state_with_error(runtime.state(), error.to_string()));
        if let Some(traffic) = traffic_stats_payload(report.traffic.as_ref(), report.reported_at_ms)
        {
            state = runtime
                .dispatch(ClientCommand::ApplyTrafficStats(traffic))
                .unwrap_or_else(|error| state_with_error(runtime.state(), error.to_string()));
        }
        state
    };
    publish_business_event(
        state_notifier,
        BUSINESS_NETWORK_RUNTIME_CHANGED,
        serde_json::json!({
            "platform": report.platform,
            "runtimeState": report.runtime_state,
            "traffic": report.traffic,
            "error": report.error,
            "reportedAtMs": report.reported_at_ms,
            "state": state,
        }),
    );
    serde_json::to_string(&serde_json::json!({
        "accepted": true,
        "state": state,
    }))
    .context("encode platform runtime state ingest response")
}

fn traffic_stats_payload(
    traffic: Option<&Value>,
    reported_at_ms: Option<u64>,
) -> Option<TrafficStatsPayload> {
    let traffic = traffic?;
    let updated_at_ms = reported_at_ms
        .or_else(|| value_u64(traffic, &["updatedAtMs", "reportedAtMs"]))
        .unwrap_or_else(current_timestamp_ms);
    let tx_bytes = value_u64(
        traffic,
        &["txBytes", "outboundBytes", "sentBytes", "bytesRead"],
    )
    .unwrap_or_default();
    let rx_bytes = value_u64(
        traffic,
        &[
            "rxBytes",
            "inboundBytes",
            "receivedBytes",
            "bytesWritten",
            "tunBytesWritten",
        ],
    )
    .unwrap_or_default();
    let previous = {
        let mut guard = LAST_TRAFFIC_SAMPLE
            .get_or_init(|| Mutex::new(None))
            .lock()
            .expect("traffic sample mutex poisoned");
        let previous = *guard;
        *guard = Some(TrafficSample {
            tx_bytes,
            rx_bytes,
            updated_at_ms,
        });
        previous
    };
    let (tx_bytes_per_minute, rx_bytes_per_minute) = previous
        .filter(|sample| updated_at_ms > sample.updated_at_ms)
        .map(|sample| {
            let elapsed_ms = updated_at_ms.saturating_sub(sample.updated_at_ms).max(1);
            (
                bytes_per_minute(tx_bytes.saturating_sub(sample.tx_bytes), elapsed_ms),
                bytes_per_minute(rx_bytes.saturating_sub(sample.rx_bytes), elapsed_ms),
            )
        })
        .unwrap_or((0, 0));
    Some(TrafficStatsPayload {
        tx_bytes,
        rx_bytes,
        tx_bytes_per_minute,
        rx_bytes_per_minute,
        updated_at_ms,
    })
}

fn bytes_per_minute(delta_bytes: u64, elapsed_ms: u64) -> u64 {
    ((delta_bytes as u128).saturating_mul(60_000) / elapsed_ms as u128) as u64
}

fn value_u64(value: &Value, keys: &[&str]) -> Option<u64> {
    keys.iter().find_map(|key| {
        value.get(*key).and_then(|value| {
            value
                .as_u64()
                .or_else(|| value.as_i64().and_then(|value| u64::try_from(value).ok()))
                .or_else(|| {
                    value
                        .as_f64()
                        .filter(|value| *value >= 0.0)
                        .map(|value| value as u64)
                })
        })
    })
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

fn console_login_key() -> Result<Value> {
    let session = ensure_session_device_registered(load_session()?)?;
    if session.session_kind == "device" || session.user_id.trim().is_empty() {
        anyhow::bail!("user login is required before opening Web Console");
    }
    let client = ControlPlaneClient::from_env();
    let login_key = client.console_login_key(
        &session.access_token,
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
    let session = load_session().context("load session")?;
    let network_id = session
        .active_network_id
        .as_deref()
        .filter(|value| !value.trim().is_empty())
        .ok_or_else(|| anyhow::anyhow!("active network is not available"))?;
    let from_device_id = session
        .device_id
        .as_deref()
        .filter(|value| !value.trim().is_empty())
        .ok_or_else(|| anyhow::anyhow!("device id is not available"))?;
    let mqtt = session
        .mqtt
        .as_ref()
        .ok_or_else(|| anyhow::anyhow!("mqtt credential is not available"))?;
    let response = client_message_mqtt::publish_client_message(
        mqtt,
        network_id,
        from_device_id,
        &input.target_device_id,
        &input.body,
        input.metadata.as_ref(),
    )?;
    serde_json::to_string(&response).context("encode send client message response")
}

fn control_transport_tick_plan(args: Value) -> Result<ControlTransportTickPlan> {
    let input: ControlTransportTickRequest =
        serde_json::from_value(args).context("decode control transport tick request")?;
    Ok(control_transport::control_transport_tick_plan(
        input,
        current_timestamp_ms(),
    ))
}

fn handle_relay_candidates(refresh: bool) -> Result<String> {
    let response = relay_candidates_response(refresh)?;
    serde_json::to_string(&response).context("encode relay candidates")
}

fn handle_prepare_relay_data_plane() -> Result<String> {
    let config = prepare_relay_data_plane_from_latest_control()?;
    serde_json::to_string(&config).context("encode relay data plane config")
}

fn handle_path_diagnose() -> Result<String> {
    let response = path_diagnose_response()?;
    serde_json::to_string(&response).context("encode path diagnose")
}

fn handle_export_diagnostics(
    runtime: &Arc<Mutex<ClientRuntime<PlatformNetworkImpl>>>,
) -> Result<String> {
    let state = {
        let runtime = runtime.lock().expect("client runtime mutex poisoned");
        runtime.state().clone()
    };
    let diagnose = path_diagnose_response().ok();
    let platform = PlatformNetworkImpl.diagnostics().ok();
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

fn relay_candidates_response(refresh: bool) -> Result<RelayCandidateListResponse> {
    let mut session = load_network_session()?;
    let network_id = ensure_active_network_id(&mut session)?;
    let refreshed = if refresh {
        refresh_relay_candidates_for_session(&mut session, &network_id)?
    } else {
        false
    };
    let selections = select_relay_candidates(&runtime_relay_candidates());
    let best = selections.iter().find(|item| item.selected).cloned();
    Ok(RelayCandidateListResponse {
        network_id: Some(network_id),
        refreshed,
        candidates: selections,
        best,
    })
}

fn path_diagnose_response() -> Result<PathDiagnoseResponse> {
    let mut session = load_network_session()?;
    let network_id = ensure_active_network_id(&mut session)?;
    let client = ControlPlaneClient::from_env();
    let device_id = session
        .device_id
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .ok_or_else(|| anyhow::anyhow!("device unavailable: current device is not registered"))?;
    let activation = client.activate_network(&session.access_token, device_id, &network_id)?;
    if !activation.relay_candidates.is_empty() {
        replace_runtime_relay_candidates(
            activation
                .relay_candidates
                .iter()
                .map(persisted_relay_candidate)
                .collect(),
        );
    }
    session.self_node_id = activation.self_node_id.clone();
    session.virtual_ip = Some(activation.virtual_ip);
    persist_session(&session)?;

    let stats = load_relay_runtime_stats();
    let platform =
        PlatformNetworkImpl
            .diagnostics()
            .unwrap_or_else(|error| PlatformNetworkDiagnostics {
                platform: std::env::consts::OS.to_string(),
                checks: vec![client_core::PlatformDiagnosticCheck {
                    name: "platformDiagnostics".to_string(),
                    ok: false,
                    message: Some(error.to_string()),
                }],
                ..PlatformNetworkDiagnostics::default()
            });
    let runtime_state = PlatformNetworkImpl.read_runtime_state().unwrap_or_default();
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
    let relay_candidates = select_relay_candidates(&runtime_relay_candidates());
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
    let dns = path_diagnose_dns(&activation.dns_servers, &platform.dns_servers);
    let health = path_diagnose_health(
        relay.as_ref(),
        &mtu,
        &dns,
        &platform,
        &relay_candidates,
        &peer_paths,
    );
    Ok(PathDiagnoseResponse {
        network_id: Some(network_id),
        health,
        active_path_type,
        active_path_counts,
        peer_paths,
        relay,
        direct_candidates,
        relay_candidates,
        mtu,
        dns,
        platform,
        export_path: None,
    })
}

fn path_diagnose_health(
    relay: Option<&PathDiagnoseRelay>,
    mtu: &PathDiagnoseMtu,
    dns: &PathDiagnoseDns,
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
    if dns.checked && dns.ok == Some(false) {
        failed = true;
        push_path_health_reason(
            &mut reasons,
            "dns_mismatch",
            "failed",
            "Windows DNS configuration does not match the expected network DNS servers",
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

fn path_diagnose_dns(expected_servers: &[String], actual_servers: &[String]) -> PathDiagnoseDns {
    let expected = normalized_dns_servers(expected_servers);
    let actual = normalized_dns_servers(actual_servers);
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
    PathDiagnoseDns {
        expected_servers: expected,
        actual_servers: actual,
        checked,
        ok: checked.then_some(missing.is_empty()),
        missing_servers: missing,
        extra_servers: extra,
    }
}

fn normalized_dns_servers(servers: &[String]) -> Vec<String> {
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
    runtime: &Arc<Mutex<ClientRuntime<PlatformNetworkImpl>>>,
    state_notifier: &Arc<StateChangeNotifier>,
) -> Result<String> {
    let input: WatchStateRequest =
        serde_json::from_value(request.args).context("decode watch state request")?;
    let timeout = Duration::from_millis(input.timeout_ms.clamp(1_000, 60_000));
    let mut revision = state_notifier
        .revision
        .lock()
        .expect("state revision mutex poisoned");
    if *revision <= input.last_revision {
        let wait_result = state_notifier
            .changed
            .wait_timeout_while(revision, timeout, |current| *current <= input.last_revision)
            .expect("state revision condvar poisoned");
        revision = wait_result.0;
    }
    let state = {
        let mut runtime = runtime.lock().expect("client runtime mutex poisoned");
        match runtime.refresh() {
            Ok(()) => runtime.state().clone(),
            Err(error) => state_with_error(runtime.state(), error.to_string()),
        }
    };
    serde_json::to_string(&WatchStateResponse {
        revision: *revision,
        state,
    })
    .context("encode watch state response")
}

fn handle_watch_business_event(
    request: ServiceRequest,
    runtime: &Arc<Mutex<ClientRuntime<PlatformNetworkImpl>>>,
    state_notifier: &Arc<StateChangeNotifier>,
) -> Result<String> {
    let input: WatchBusinessEventRequest =
        serde_json::from_value(request.args).context("decode watch business event request")?;
    let timeout = Duration::from_millis(input.timeout_ms.clamp(1_000, 60_000));
    let mut revision = state_notifier
        .revision
        .lock()
        .expect("state revision mutex poisoned");
    if *revision <= input.last_revision {
        let wait_result = state_notifier
            .changed
            .wait_timeout_while(revision, timeout, |current| *current <= input.last_revision)
            .expect("state revision condvar poisoned");
        revision = wait_result.0;
    }
    let current_revision = *revision;
    drop(revision);

    let event = state_notifier
        .event
        .lock()
        .expect("business event mutex poisoned")
        .clone()
        .unwrap_or_else(|| StoredBusinessEvent {
            business_type: BUSINESS_STATE_CHANGED.to_string(),
            business_data: serde_json::json!({}),
        });
    let snapshot = {
        let mut runtime = runtime.lock().expect("client runtime mutex poisoned");
        match runtime.refresh() {
            Ok(()) => runtime.state().clone(),
            Err(error) => state_with_error(runtime.state(), error.to_string()),
        }
    };
    serde_json::to_string(&WatchBusinessEventResponse {
        revision: current_revision,
        business_type: event.business_type,
        business_data: event.business_data,
        snapshot,
    })
    .context("encode watch business event response")
}

fn handle_task_request(
    request: ServiceRequest,
    runtime: &Arc<Mutex<ClientRuntime<PlatformNetworkImpl>>>,
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
    runtime: &Arc<Mutex<ClientRuntime<PlatformNetworkImpl>>>,
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
    runtime: &Arc<Mutex<ClientRuntime<PlatformNetworkImpl>>>,
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
    runtime: &Arc<Mutex<ClientRuntime<PlatformNetworkImpl>>>,
    task_queue: &Arc<Mutex<ControlTaskQueue>>,
) -> Result<String> {
    let input: ControlTransportOutboxRequest =
        serde_json::from_value(request.args).context("decode control transport outbox request")?;
    let session = load_session()?;
    let state = {
        let runtime = runtime.lock().expect("client runtime mutex poisoned");
        runtime.state().clone()
    };
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
    _runtime: &Arc<Mutex<ClientRuntime<PlatformNetworkImpl>>>,
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
    runtime: &Arc<Mutex<ClientRuntime<PlatformNetworkImpl>>>,
    task_queue: &Arc<Mutex<ControlTaskQueue>>,
) -> ClientViewState {
    let mut last_state = {
        let runtime = runtime.lock().expect("client runtime mutex poisoned");
        runtime.state().clone()
    };
    loop {
        let task = {
            let mut task_queue = task_queue
                .lock()
                .expect("control task queue mutex poisoned");
            match task_queue.take_next_pending() {
                Ok(task) => task,
                Err(error) => return state_with_error(&last_state, error.to_string()),
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
    runtime: &Arc<Mutex<ClientRuntime<PlatformNetworkImpl>>>,
    task_queue: &Arc<Mutex<ControlTaskQueue>>,
    task: control_tasks::ControlTask,
) -> ClientViewState {
    if task.action == ControlTaskAction::RefreshNetworkConfig {
        let state = {
            let mut runtime = runtime.lock().expect("client runtime mutex poisoned");
            match sync_downstream_network_assignment(&mut runtime) {
                Ok(()) => runtime.state().clone(),
                Err(error) => state_with_error(runtime.state(), error.to_string()),
            }
        };
        let mut task_queue = task_queue
            .lock()
            .expect("control task queue mutex poisoned");
        if let Some(error) = state.error.clone() {
            let mut runtime = runtime.lock().expect("client runtime mutex poisoned");
            let _ = runtime.set_error(error.clone());
            let _ = task_queue.mark_failed(&task.id, error);
        } else {
            let _ = task_queue.mark_succeeded(&task.id);
        }
        return state;
    }
    if task.action == ControlTaskAction::DeviceUserLoginSucceeded {
        let state = {
            let runtime = runtime.lock().expect("client runtime mutex poisoned");
            runtime.state().clone()
        };
        let mut task_queue = task_queue
            .lock()
            .expect("control task queue mutex poisoned");
        let _ = task_queue.mark_succeeded(&task.id);
        return state;
    }
    let command = match task.action {
        ControlTaskAction::EnableNetwork => ClientCommand::EnableNetwork,
        ControlTaskAction::DisableNetwork => ClientCommand::DisableNetwork,
        ControlTaskAction::RefreshNetworkConfig => unreachable!("handled above"),
        ControlTaskAction::DeviceUserLoginSucceeded => unreachable!("handled above"),
    };
    let state = {
        let mut runtime = runtime.lock().expect("client runtime mutex poisoned");
        let origin = match task.direction {
            ControlTaskDirection::Upstream => CommandOrigin::Upstream,
            ControlTaskDirection::Downstream => CommandOrigin::Downstream,
        };
        dispatch_with_origin(&mut runtime, command, origin)
    };
    let mut task_queue = task_queue
        .lock()
        .expect("control task queue mutex poisoned");
    if let Some(error) = state.error.clone() {
        let mut runtime = runtime.lock().expect("client runtime mutex poisoned");
        let _ = runtime.set_error(error.clone());
        let _ = task_queue.mark_failed(&task.id, error);
    } else {
        let _ = task_queue.mark_succeeded(&task.id);
    }
    state
}

pub(crate) fn dispatch_with_side_effects<P>(
    runtime: &mut ClientRuntime<P>,
    command: ClientCommand,
) -> ClientViewState
where
    P: client_core::PlatformNetwork,
{
    dispatch_with_origin(runtime, command, CommandOrigin::Upstream)
}

fn dispatch_with_origin<P>(
    runtime: &mut ClientRuntime<P>,
    command: ClientCommand,
    origin: CommandOrigin,
) -> ClientViewState
where
    P: client_core::PlatformNetwork,
{
    let (command, side_effect) = match command {
        ClientCommand::OpenClientLogin => {
            let session = match prepare_client_login_session(std::env::consts::OS) {
                Ok(session) => session,
                Err(error) => {
                    return state_with_error(
                        runtime.state(),
                        format!("准备客户端登录失败: {error:#}"),
                    );
                }
            };
            log_service_error(format!(
                "client-core-service prepared browser login device={} mqttReady=true",
                session.device_id.as_deref().unwrap_or_default()
            ));
            return runtime.request_browser_login(session.device_id);
        }
        ClientCommand::LoginWithPassword(payload) => {
            let auth_payload = match ControlPlaneClient::from_env()
                .login_with_password(&payload.email, &payload.password)
            {
                Ok(payload) => payload,
                Err(error) => {
                    log_service_error(format!(
                        "client-core-service password login failed: {error:#}"
                    ));
                    return state_with_error(runtime.state(), format!("登录失败: {error:#}"));
                }
            };
            let session = hydrate_session_from_control_plane(auth_payload.clone())
                .unwrap_or_else(|_| PersistedSession::from(auth_payload));
            let side_effect = persist_session(&session);
            (
                ClientCommand::ApplyDeviceUserLogin(session.into()),
                side_effect,
            )
        }
        ClientCommand::ApplyDeviceUserLogin(payload) => {
            if let Some(expected_device_id) = runtime
                .state()
                .device_id
                .as_deref()
                .map(str::trim)
                .filter(|value| !value.is_empty())
            {
                if payload
                    .device_id
                    .as_deref()
                    .map(str::trim)
                    .filter(|value| !value.is_empty())
                    != Some(expected_device_id)
                {
                    return state_with_error(runtime.state(), "登录设备不匹配".to_string());
                }
            }
            let session = hydrate_session_from_control_plane(payload.clone())
                .unwrap_or_else(|_| PersistedSession::from(payload.clone()));
            let side_effect = persist_session(&session);
            (
                ClientCommand::ApplyDeviceUserLogin(session.into()),
                side_effect,
            )
        }
        ClientCommand::SyncAssignedIp(payload) => {
            let side_effect = match load_session() {
                Ok(mut session) => {
                    if session.access_token.trim().is_empty() && session.device_token.is_none() {
                        Ok(())
                    } else {
                        session.virtual_ip = Some(payload.virtual_ip.clone());
                        persist_session(&session)
                    }
                }
                Err(error) => {
                    log_service_error(format!(
                        "client-core-service skipped assigned IP persistence without session: {error:#}"
                    ));
                    Ok(())
                }
            };
            (ClientCommand::SyncAssignedIp(payload), side_effect)
        }
        ClientCommand::Logout => {
            if let Ok(session) = load_session() {
                revoke_remote_sessions(&session);
            }
            deactivate_control_network();
            crate::network_module::clear_network_module();
            (ClientCommand::Logout, remove_session())
        }
        other => (other, Ok(())),
    };
    if let Err(error) = side_effect {
        log_service_error(format!(
            "client-core-service command side effect failed: {error:#}"
        ));
        return state_with_error(runtime.state(), error.to_string());
    }

    let command_was_enable = matches!(&command, ClientCommand::EnableNetwork);
    let command_was_disable = matches!(&command, ClientCommand::DisableNetwork);

    if command_was_enable && origin == CommandOrigin::Upstream {
        if let Err(error) = activate_control_network(runtime) {
            log_service_error(format!(
                "client-core-service activate network failed: {error:#}"
            ));
            let _ = clear_session_virtual_ip();
            return state_with_error(runtime.state(), error.to_string());
        }
    }
    if command_was_enable && origin == CommandOrigin::Downstream {
        if let Err(error) = sync_downstream_network_assignment(runtime) {
            log_service_error(format!(
                "client-core-service sync downstream network assignment failed: {error:#}"
            ));
            return state_with_error(runtime.state(), error.to_string());
        }
    }
    if command_was_disable && origin == CommandOrigin::Upstream {
        deactivate_control_network();
    }

    if matches!(&command, ClientCommand::EnableNetwork) && runtime.state().virtual_ip.is_none() {
        if let Ok(session) = load_session() {
            if let Some(virtual_ip) = session.virtual_ip {
                let _ = runtime.dispatch(ClientCommand::SyncAssignedIp(AssignedIpPayload {
                    virtual_ip,
                    prefix_len: session.active_network_id.as_deref().and_then(|network_id| {
                        ControlPlaneClient::from_env()
                            .network_prefix_len(&session.access_token, network_id, None)
                            .ok()
                    }),
                }));
            }
        }
    }

    if command_was_enable {
        let mut state = runtime.state().clone();
        if !state.network_enabled {
            state.virtual_ip = None;
        }
        report_runtime_state(&state);
        return state;
    }

    match runtime.dispatch(command) {
        Ok(mut state) => {
            if command_was_disable && state.error.is_none() {
                let _ = clear_session_virtual_ip();
                state.virtual_ip = None;
            }
            if state.network_enabled && state.virtual_ip.is_none() {
                if let Ok(session) = load_session() {
                    state.virtual_ip = session.virtual_ip;
                }
            }
            if command_was_enable || command_was_disable {
                report_runtime_state(&state);
            }
            state
        }
        Err(error) => {
            log_service_error(format!("client-core-service dispatch failed: {error:#}"));
            state_with_error(runtime.state(), error.to_string())
        }
    }
}

pub(crate) fn log_service_error(message: impl AsRef<str>) {
    let dir = app_data_dir().join("SLAN");
    let _ = fs::create_dir_all(&dir);
    let path = dir.join("client-core-service.log");
    let timestamp = current_timestamp_ms();
    if let Ok(mut file) = OpenOptions::new().create(true).append(true).open(path) {
        let _ = writeln!(file, "{timestamp} {}", message.as_ref());
    }
}

fn activate_control_network<P>(runtime: &mut ClientRuntime<P>) -> Result<()>
where
    P: client_core::PlatformNetwork,
{
    let mut session = load_network_session()?;
    let _ = runtime.dispatch(ClientCommand::ApplyDeviceUserLogin(session.clone().into()));
    activate_control_network_for_session(runtime, &mut session)
}

fn activate_network_from_latest_control<P>(runtime: &mut ClientRuntime<P>) -> ClientViewState
where
    P: client_core::PlatformNetwork,
{
    match load_network_session()
        .and_then(|mut session| {
            let _ = runtime.dispatch(ClientCommand::ApplyDeviceUserLogin(session.clone().into()));
            activate_control_network_for_session(runtime, &mut session)
        })
        .map(|_| runtime.state().clone())
    {
        Ok(mut state) => {
            if !state.network_enabled {
                state.virtual_ip = None;
            }
            report_runtime_state(&state);
            state
        }
        Err(error) => {
            log_service_error(format!(
                "client-core-service activate network failed: {error:#}"
            ));
            let error_message = error.to_string();
            if session_auth_invalid_error(&error) || error_message.contains("session expired") {
                let _ = runtime.dispatch(ClientCommand::Logout);
            }
            let _ = clear_session_virtual_ip();
            state_with_error(runtime.state(), error_message)
        }
    }
}

fn platform_network_config_from_latest_control<P>(
    runtime: &mut ClientRuntime<P>,
) -> Result<PlatformNetworkConfig>
where
    P: client_core::PlatformNetwork,
{
    let mut session = load_network_session()?;
    let _ = runtime.dispatch(ClientCommand::ApplyDeviceUserLogin(session.clone().into()));
    let Some(device_id) = session.device_id.clone().filter(|value| !value.is_empty()) else {
        return Err(anyhow::anyhow!(
            "device unavailable: current device is not registered"
        ));
    };
    let client = ControlPlaneClient::from_env();
    let network_id = ensure_active_network_id(&mut session)?;
    let network_configs =
        crate::network_module::refresh_network_module_from_session(&client, &session)
            .unwrap_or_default();
    let mut activation = client.activate_network(&session.access_token, &device_id, &network_id)?;
    activation.virtual_ip = normalize_virtual_ip(&activation.virtual_ip);
    session.self_node_id = activation.self_node_id.clone();
    session.virtual_ip = Some(activation.virtual_ip.clone());
    ensure_session_node_and_control_session(&client, &mut session)
        .context("ensure node control session after network activation")?;
    if !activation.relay_candidates.is_empty() {
        replace_runtime_relay_candidates(
            activation
                .relay_candidates
                .iter()
                .map(persisted_relay_candidate)
                .collect(),
        );
    }
    let all_acl_policies = platform_acl_policies(&network_configs);
    let active_acl_policies = acl_policies_for_network(&all_acl_policies, &network_id);
    let relay_candidates = runtime_relay_candidates();
    let best_relay = android_data_plane_relay_candidate(&relay_candidates);
    persist_session(&session)?;
    let _ = runtime.dispatch(ClientCommand::SyncAssignedIp(AssignedIpPayload {
        virtual_ip: activation.virtual_ip.clone(),
        prefix_len: Some(activation.prefix_len),
    }));
    let routes = routes_with_peer_virtual_ips(
        activation.routes.clone(),
        &activation.peers,
        activation.virtual_ip.as_str(),
    );
    Ok(PlatformNetworkConfig {
        session_name: "SLAN".to_string(),
        virtual_ip: activation.virtual_ip,
        prefix_len: activation.prefix_len,
        network_configs: platform_network_configs(&network_configs),
        dns_servers: activation.dns_servers,
        routes,
        mtu: Some(1280),
        relay_endpoint_id: best_relay.as_ref().map(|relay| relay.endpoint_id.clone()),
        relay_transport: best_relay.as_ref().map(|relay| relay.transport.clone()),
        relay_address: best_relay.as_ref().map(|relay| relay.address.clone()),
        acl_policies: active_acl_policies,
        relay_data_plane: build_relay_data_plane_config(
            &client,
            &session,
            &network_id,
            activation.self_node_id.as_deref(),
            &activation.peers,
            best_relay.as_ref(),
            &all_acl_policies,
        )
        .ok(),
    })
}

fn platform_network_configs(
    configs: &[crate::control_plane::DeviceNetworkConfig],
) -> Vec<PlatformDeviceNetworkConfig> {
    configs
        .iter()
        .map(|config| PlatformDeviceNetworkConfig {
            network_id: config.network_id.clone(),
            device_id: config.device_id.clone(),
            network_name: config.network_name.clone(),
            network_code: config.network_code.clone(),
            config_version: config.config_version,
            global_ip: config.global_ip.clone(),
            global_name: config.global_name.clone(),
            peer_count: config.peers.len(),
            dns_record_count: config.dns_records.len(),
            security_rule_count: config.rules.len(),
            relay_candidate_count: config.relay_candidates.len(),
        })
        .collect()
}

fn prepare_relay_data_plane_from_latest_control() -> Result<RelayDataPlaneConfig> {
    let mut session = load_network_session()?;
    let Some(device_id) = session.device_id.clone().filter(|value| !value.is_empty()) else {
        return Err(anyhow::anyhow!(
            "device unavailable: current device is not registered"
        ));
    };
    let client = ControlPlaneClient::from_env();
    let network_id = ensure_active_network_id(&mut session)?;
    let activation = client.activate_network(&session.access_token, &device_id, &network_id)?;
    session.self_node_id = activation.self_node_id.clone();
    if !activation.relay_candidates.is_empty() {
        replace_runtime_relay_candidates(
            activation
                .relay_candidates
                .iter()
                .map(persisted_relay_candidate)
                .collect(),
        );
    }
    session.virtual_ip = Some(activation.virtual_ip);
    persist_session(&session)?;
    let relay_candidates = runtime_relay_candidates();
    let best_relay = data_plane_relay_candidate(&relay_candidates);
    build_relay_data_plane_config(
        &client,
        &session,
        &network_id,
        activation.self_node_id.as_deref(),
        &activation.peers,
        best_relay.as_ref(),
        &platform_acl_policies(
            &crate::network_module::refresh_network_module_from_session(&client, &session)
                .unwrap_or_default(),
        ),
    )
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
    let session = match load_session() {
        Ok(session) => session,
        Err(error) => {
            if error.to_string().contains("No such file or directory") {
                let _ = remove_session();
                return Err(anyhow::anyhow!("login required"));
            }
            return Err(error);
        }
    };
    if session.access_token.trim().is_empty() {
        let _ = remove_session();
        return Err(anyhow::anyhow!("login required"));
    }
    if session_is_expired(&session) {
        let _ = remove_session();
        return Err(anyhow::anyhow!("session expired; please login again"));
    }
    ensure_session_device_registered(session).inspect_err(|error| {
        if session_auth_invalid_error(error) {
            let _ = remove_session();
        }
    })
}

fn activate_control_network_for_session<P>(
    runtime: &mut ClientRuntime<P>,
    session: &mut PersistedSession,
) -> Result<()>
where
    P: client_core::PlatformNetwork,
{
    let Some(device_id) = session.device_id.clone().filter(|value| !value.is_empty()) else {
        return Err(anyhow::anyhow!(
            "device unavailable: current device is not registered"
        ));
    };
    let client = ControlPlaneClient::from_env();
    let network_id = ensure_active_network_id(session)?;
    let mut activation =
        match client.activate_network(&session.access_token, &device_id, &network_id) {
            Ok(activation) => activation,
            Err(error) if error.to_string().contains("HTTP 404") => {
                session.active_network_id = None;
                let refreshed_network_id = ensure_active_network_id(session)?;
                client
                    .activate_network(&session.access_token, &device_id, &refreshed_network_id)
                    .with_context(|| {
                        format!(
                            "activate refreshed network after stale network config {network_id}"
                        )
                    })?
            }
            Err(error) => return Err(error),
        };
    activation.virtual_ip = normalize_virtual_ip(&activation.virtual_ip);
    session.self_node_id = activation.self_node_id.clone();
    log_service_error(format!(
        "client-core-service enable preflight ok: ip={}/{} dns={} routes={} peers={} relays={}",
        activation.virtual_ip,
        activation.prefix_len,
        activation.dns_servers.len(),
        activation.routes.len(),
        activation.peer_count,
        activation.relay_candidates.len()
    ));
    session.virtual_ip = Some(activation.virtual_ip.clone());
    ensure_session_node_and_control_session(&client, session)
        .context("ensure node control session after network activation")?;
    if !activation.relay_candidates.is_empty() {
        replace_runtime_relay_candidates(
            activation
                .relay_candidates
                .iter()
                .map(persisted_relay_candidate)
                .collect(),
        );
    }
    let relay_candidates = runtime_relay_candidates();
    let best_relay = data_plane_relay_candidate(&relay_candidates);
    let network_configs =
        crate::network_module::refresh_network_module_from_session(&client, session)
            .unwrap_or_default();
    let acl_policies = platform_acl_policies(&network_configs);
    let relay_config = build_relay_data_plane_config(
        &client,
        session,
        &network_id,
        activation.self_node_id.as_deref(),
        &activation.peers,
        best_relay.as_ref(),
        &acl_policies,
    )
    .map_err(|error| {
        log_service_error(format!(
            "client-core-service relay data plane config skipped: {error:#}"
        ));
        error
    })
    .ok();
    persist_session(session)?;
    let _ = runtime.dispatch(ClientCommand::SyncAssignedIp(AssignedIpPayload {
        virtual_ip: activation.virtual_ip.clone(),
        prefix_len: Some(activation.prefix_len),
    }));
    log_service_error(format!(
        "client-core-service applying latest assigned IP locally: ip={}",
        activation.virtual_ip
    ));
    runtime.enable_network_with_config(
        activation.prefix_len,
        &activation.dns_servers,
        &routes_with_peer_virtual_ips(
            activation.routes.clone(),
            &activation.peers,
            activation.virtual_ip.as_str(),
        ),
        relay_config.as_ref(),
    )?;
    Ok(())
}

fn ensure_active_network_id(session: &mut PersistedSession) -> Result<String> {
    if session.active_network_id.is_none() {
        let client = ControlPlaneClient::from_env();
        if let Ok(configs) =
            crate::network_module::refresh_network_module_from_session(&client, session)
        {
            session.active_network_id = configs.into_iter().next().map(|config| config.network_id);
        }
        if session.active_network_id.is_none() {
            session.active_network_id = client.active_network_id(&session.access_token)?;
        }
    }
    let Some(network_id) = session.active_network_id.clone() else {
        persist_session(session)?;
        return Err(anyhow::anyhow!(
            "device unavailable: current user has no active network"
        ));
    };
    Ok(network_id)
}

fn refresh_relay_candidates_for_session(
    session: &mut PersistedSession,
    network_id: &str,
) -> Result<bool> {
    let device_id = session
        .device_id
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .ok_or_else(|| anyhow::anyhow!("device unavailable: current device is not registered"))?;
    let client = ControlPlaneClient::from_env();
    let candidates = client.relay_candidates(&session.access_token, device_id, network_id)?;
    if candidates.is_empty() {
        return Ok(false);
    }
    replace_runtime_relay_candidates(candidates.iter().map(persisted_relay_candidate).collect());
    Ok(true)
}

fn build_relay_data_plane_config(
    client: &ControlPlaneClient,
    session: &PersistedSession,
    network_id: &str,
    self_node_id: Option<&str>,
    peers: &[ControlPeer],
    best_relay: Option<&RelayCandidateSelection>,
    acl_policies: &[PlatformAclPolicy],
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
    let punch_sessions =
        create_punch_connect_sessions(client, session, network_id, local_node_id, peers);

    let relay_candidates = select_relay_candidates(&runtime_relay_candidates());
    let relay_targets = relay_session_targets(best_relay, &relay_candidates);
    let sessions = peers
        .iter()
        .filter(|peer| peer.relay_allowed)
        .filter(|peer| peer.node_id != local_node_id)
        .flat_map(|peer| {
            let mut sessions = Vec::new();
            if let Some(session) = connect_plans.get(&peer.node_id).and_then(|plan| {
                relay_session_from_connect_plan_ticket(plan, network_id, local_node_id, peer)
            }) {
                sessions.push(session);
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
                    &session.access_token,
                    network_id,
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
        .first()
        .map(|session| session.ticket.relay_url.trim())
        .filter(|value| !value.is_empty())
        .map(str::to_string)
        .unwrap_or_else(|| relay.address.clone());
    Ok(RelayDataPlaneConfig {
        enabled: !sessions.is_empty(),
        transport: relay.transport.clone(),
        relay_address,
        local_node_id: local_node_id.to_string(),
        network_id: network_id.to_string(),
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

fn peer_path_configs(
    peers: &[ControlPeer],
    local_node_id: &str,
    relay: &RelayCandidateSelection,
    relay_candidates: &[RelayCandidateSelection],
    relay_sessions: &[RelayPeerSession],
    connect_plans: Option<BTreeMap<String, PersistedConnectPlan>>,
    punch_sessions: Option<BTreeMap<String, PunchConnectSession>>,
) -> Vec<PeerPathConfig> {
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
            if let Some(address) = punch_session.and_then(punch_peer_direct_udp_address) {
                direct_addresses.push(address.clone());
                candidates.push(direct_path_candidate(PathKind::DirectUdp, &address));
            }
            if let Some(plan) = connect_plans.get(&peer.node_id) {
                for path in &plan.paths {
                    let address = path.endpoint.trim();
                    if address.is_empty() {
                        continue;
                    }
                    if let Some(kind) = direct_path_kind_for_path_type(&path.path_type) {
                        if direct_addresses
                            .iter()
                            .any(|value: &String| value == address)
                        {
                            continue;
                        }
                        direct_addresses.push(address.to_string());
                        candidates.push(direct_path_candidate(kind, address));
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
            candidates.extend(peer.endpoints.iter().filter_map(|endpoint| {
                let address = endpoint.address.trim();
                if address.is_empty() {
                    return None;
                }
                if direct_addresses.iter().any(|value| value == address) {
                    return None;
                }
                direct_addresses.push(address.to_string());
                Some(direct_path_candidate(PathKind::DirectUdp, address))
            }));
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
) -> Vec<RelayCandidateSelection> {
    let mut targets = Vec::new();
    if let Some(relay) = best_relay {
        push_unique_relay_target(&mut targets, relay.clone());
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
            && existing.address == candidate.address
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
) -> BTreeMap<String, PunchConnectSession> {
    if !punch_connect_enabled() {
        return BTreeMap::new();
    }
    let device_id = session.device_id.as_deref().unwrap_or_default();
    peers
        .iter()
        .filter(|peer| peer.node_id != local_node_id)
        .filter_map(|peer| {
            match client.create_punch_connect_session(
                &session.access_token,
                device_id,
                session.mqtt.as_ref(),
                network_id,
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

/// 判断是否启用 punch 协商。默认启用，可通过 SLAN_PUNCH_CONNECT_ENABLED=0/false 关闭。
fn punch_connect_enabled() -> bool {
    match std::env::var("SLAN_PUNCH_CONNECT_ENABLED") {
        Ok(value) => !matches!(
            value.trim().to_ascii_lowercase().as_str(),
            "0" | "false" | "no" | "off" | "disabled"
        ),
        Err(_) => true,
    }
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

fn routes_with_peer_virtual_ips(
    mut routes: Vec<client_core::RouteSpec>,
    peers: &[ControlPeer],
    self_virtual_ip: &str,
) -> Vec<client_core::RouteSpec> {
    let mut existing = routes
        .iter()
        .map(|route| normalize_route_destination(route.destination.as_str()))
        .collect::<std::collections::HashSet<_>>();
    let self_ip = normalize_virtual_ip_for_route(self_virtual_ip);
    for peer in peers {
        for ip in &peer.virtual_ips {
            let Some(peer_ip) = usable_peer_virtual_ip(ip) else {
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
    if ip.is_empty()
        || ip == "0.0.0.0"
        || ip.starts_with("169.254.")
        || ip.contains(':')
        || ip.eq_ignore_ascii_case("pending")
    {
        return None;
    }
    Some(ip)
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

fn sync_downstream_network_assignment<P>(runtime: &mut ClientRuntime<P>) -> Result<()>
where
    P: client_core::PlatformNetwork,
{
    let mut session = load_session()?;
    let Some(device_id) = session.device_id.clone().filter(|value| !value.is_empty()) else {
        return Err(anyhow::anyhow!(
            "device unavailable: current device is not registered"
        ));
    };
    let client = ControlPlaneClient::from_env();
    session = ensure_session_device_registered(session)?;
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
            .network_prefix_len(&session.access_token, network_id, None)
            .ok()
    });
    let _ = runtime.dispatch(ClientCommand::SyncAssignedIp(AssignedIpPayload {
        virtual_ip,
        prefix_len,
    }));
    if runtime.state().network_enabled {
        activate_control_network_for_session(runtime, &mut session)?;
    }
    persist_session(&session)
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
    match client.deactivate_network(&session.access_token, device_id, network_id) {
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
    }
}

pub(crate) fn persist_relay_candidates_from_network_map(map: &Value) -> Result<usize> {
    let candidates = extract_persisted_relay_candidates_from_network_map(map);
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
    plan.updated_at_ms = current_timestamp_ms();

    let mut store = load_connect_plan_store();
    let cutoff = plan.updated_at_ms.saturating_sub(CONNECT_PLAN_TTL_MS);
    store
        .plans
        .retain(|item| item.peer_node_id != plan.peer_node_id && item.updated_at_ms >= cutoff);
    store.plans.push(plan);
    persist_connect_plan_store(&store)?;
    Ok(true)
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

fn notify_state_changed(state_notifier: &StateChangeNotifier) {
    publish_business_event(
        state_notifier,
        BUSINESS_STATE_CHANGED,
        serde_json::json!({}),
    );
}

fn publish_business_event(
    state_notifier: &StateChangeNotifier,
    business_type: impl Into<String>,
    business_data: Value,
) {
    let mut revision = state_notifier
        .revision
        .lock()
        .expect("state revision mutex poisoned");
    *revision = revision.saturating_add(1);
    let mut event = state_notifier
        .event
        .lock()
        .expect("business event mutex poisoned");
    *event = Some(StoredBusinessEvent {
        business_type: business_type.into(),
        business_data,
    });
    state_notifier.changed.notify_all();
}

pub(crate) fn publish_state_business_event(
    state_notifier: &Arc<StateChangeNotifier>,
    business_type: impl Into<String>,
    state: &ClientViewState,
) {
    let business_data = serde_json::to_value(state).unwrap_or_else(|_| serde_json::json!({}));
    publish_business_event(state_notifier, business_type, business_data);
}

fn publish_method_business_event(
    state_notifier: &StateChangeNotifier,
    request_line: &str,
    response: &str,
) {
    let method = serde_json::from_str::<ServiceRequest>(request_line)
        .map(|request| request.method)
        .unwrap_or_default();
    let state = serde_json::from_str::<ClientViewState>(response).ok();
    let business_type = match LocalServiceMethod::parse(&method) {
        LocalServiceMethod::LocalNetworkActivate => state
            .as_ref()
            .and_then(|state| state.error.as_ref())
            .map(|_| BUSINESS_NETWORK_SWITCH_FAILED)
            .unwrap_or(BUSINESS_NETWORK_SWITCH_FINISHED),
        LocalServiceMethod::LocalNetworkDeactivate | LocalServiceMethod::LocalNetworkShutdown => {
            state
                .as_ref()
                .and_then(|state| state.error.as_ref())
                .map(|_| BUSINESS_NETWORK_SWITCH_FAILED)
                .unwrap_or(BUSINESS_NETWORK_SWITCH_FINISHED)
        }
        LocalServiceMethod::LocalLogout | LocalServiceMethod::Dispatch => BUSINESS_SESSION_CHANGED,
        LocalServiceMethod::Start | LocalServiceMethod::Refresh => BUSINESS_STATE_CHANGED,
        _ => BUSINESS_STATE_CHANGED,
    };
    let business_data = state
        .and_then(|state| serde_json::to_value(state).ok())
        .unwrap_or_else(|| serde_json::json!({}));
    publish_business_event(state_notifier, business_type, business_data);
}

fn error_state_json(error: String) -> String {
    let state = state_with_error(&ClientViewState::default(), error);
    serde_json::to_string(&state).unwrap_or_else(|_| "{}".to_string())
}

fn spawn_session_refresh_worker(
    runtime: Arc<Mutex<ClientRuntime<PlatformNetworkImpl>>>,
    state_notifier: Arc<StateChangeNotifier>,
) {
    thread::spawn(move || loop {
        thread::sleep(SESSION_REFRESH_INTERVAL);
        let signed_in = {
            let runtime = runtime.lock().expect("client runtime mutex poisoned");
            runtime.state().signed_in
        };
        if !signed_in {
            continue;
        }
        match refresh_logged_in_session() {
            Ok(Some(session)) => {
                let mut runtime = runtime.lock().expect("client runtime mutex poisoned");
                let before = runtime.state().clone();
                let _ = runtime.dispatch(ClientCommand::ApplyDeviceUserLogin(session.into()));
                let state = runtime.state().clone();
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
            Ok(None) => {}
            Err(error) => {
                log_service_error(format!(
                    "client-core-service session refresh skipped: {error:#}"
                ));
                if session_auth_invalid_error(&error)
                    || error.to_string().contains("session expired")
                {
                    let _ = remove_session();
                    let mut runtime = runtime.lock().expect("client runtime mutex poisoned");
                    let _ = runtime.dispatch(ClientCommand::Logout);
                    let business_data = serde_json::to_value(runtime.state())
                        .unwrap_or_else(|_| serde_json::json!({}));
                    publish_business_event(
                        &state_notifier,
                        BUSINESS_SESSION_CHANGED,
                        business_data,
                    );
                }
            }
        }
    });
}

fn refresh_logged_in_session() -> Result<Option<PersistedSession>> {
    let session = match load_session() {
        Ok(session) => session,
        Err(error) => {
            if error.to_string().contains("No such file or directory") {
                return Err(anyhow::anyhow!("login required"));
            }
            return Err(error);
        }
    };
    if session.access_token.trim().is_empty() {
        return Ok(None);
    }
    if session_is_expired(&session) {
        let _ = remove_session();
        return Err(anyhow::anyhow!("session expired; please login again"));
    }
    ensure_session_device_registered(session).map(Some)
}

fn spawn_runtime_sync_worker(
    runtime: Arc<Mutex<ClientRuntime<PlatformNetworkImpl>>>,
    state_notifier: Arc<StateChangeNotifier>,
) {
    thread::spawn(move || loop {
        thread::sleep(Duration::from_secs(10));
        let before = {
            let runtime = runtime.lock().expect("client runtime mutex poisoned");
            runtime.state().clone()
        };
        sync_control_assignment(&runtime);
        let state = {
            let mut runtime = runtime.lock().expect("client runtime mutex poisoned");
            match runtime.refresh() {
                Ok(()) => runtime.state().clone(),
                Err(error) => state_with_error(runtime.state(), error.to_string()),
            }
        };
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
        report_runtime_state(&state);
    });
}

fn spawn_control_task_worker(
    runtime: Arc<Mutex<ClientRuntime<PlatformNetworkImpl>>>,
    task_queue: Arc<Mutex<ControlTaskQueue>>,
    state_notifier: Arc<StateChangeNotifier>,
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

        let _ = execute_control_task(&runtime, &task_queue, task);
        notify_state_changed(&state_notifier);
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
    runtime: Arc<Mutex<ClientRuntime<PlatformNetworkImpl>>>,
    state_notifier: Arc<StateChangeNotifier>,
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
    runtime: &Arc<Mutex<ClientRuntime<PlatformNetworkImpl>>>,
    state_notifier: &StateChangeNotifier,
    maintenance: &mut RelayMaintenanceState,
) -> Result<()> {
    let now = current_timestamp_ms();
    let before = {
        let runtime = runtime.lock().expect("client runtime mutex poisoned");
        runtime.state().clone()
    };
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
    let mut session = session;
    let state = {
        let mut runtime = runtime.lock().expect("client runtime mutex poisoned");
        match activate_control_network_for_session(&mut runtime, &mut session) {
            Ok(()) => runtime.state().clone(),
            Err(error) => state_with_error(runtime.state(), error.to_string()),
        }
    };
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
    if relay_response_stalled(stats, maintenance) {
        return Some("relay_response_stalled");
    }
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

pub(crate) fn sync_control_assignment(runtime: &Arc<Mutex<ClientRuntime<PlatformNetworkImpl>>>) {
    let Ok(mut session) = load_session() else {
        return;
    };
    if session_is_expired(&session) {
        let _ = remove_session();
        let mut runtime = runtime.lock().expect("client runtime mutex poisoned");
        let _ = runtime.dispatch(ClientCommand::Logout);
        return;
    }
    if session.access_token.trim().is_empty() {
        return;
    }
    let client = ControlPlaneClient::from_env();
    if session.active_network_id.is_none() {
        if let Ok(network_id) = client.active_network_id(&session.access_token) {
            session.active_network_id = network_id;
        }
    }

    let Some(device_id) = session.device_id.clone().filter(|value| !value.is_empty()) else {
        if let Ok(session) = ensure_session_device_registered(session) {
            let mut runtime = runtime.lock().expect("client runtime mutex poisoned");
            let _ = runtime.dispatch(ClientCommand::ApplyDeviceUserLogin(session.into()));
        }
        return;
    };
    let Ok(devices) = client.list_devices(&session.access_token) else {
        let _ = persist_session(&session);
        return;
    };
    let Some(device) = devices.into_iter().find(|item| item.device_id == device_id) else {
        if let Ok(session) = ensure_session_device_registered(session) {
            let mut runtime = runtime.lock().expect("client runtime mutex poisoned");
            let _ = runtime.dispatch(ClientCommand::ApplyDeviceUserLogin(session.into()));
        }
        return;
    };
    sync_session_device_fields(&mut session, &device);
    if !control_device_network_available(&device) {
        log_service_error(format!(
            "client-core-service downstream disabled local network: deviceStatus={:?} memberStatus={:?}",
            device.status, device.membership_status
        ));
        session.virtual_ip = None;
        let _ = persist_session(&session);
        let mut runtime = runtime.lock().expect("client runtime mutex poisoned");
        let _ = dispatch_with_origin(
            &mut runtime,
            ClientCommand::DisableNetwork,
            CommandOrigin::Downstream,
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
        let _ = persist_session(&session);
        let mut runtime = runtime.lock().expect("client runtime mutex poisoned");
        let _ = dispatch_with_origin(
            &mut runtime,
            ClientCommand::DisableNetwork,
            CommandOrigin::Downstream,
        );
        return;
    };
    if session.virtual_ip.as_deref() == Some(assigned_ip.as_str()) {
        let _ = persist_session(&session);
        return;
    }
    session.virtual_ip = Some(assigned_ip.clone());
    let _ = persist_session(&session);
    let mut runtime = runtime.lock().expect("client runtime mutex poisoned");
    log_service_error(format!(
        "client-core-service syncing latest assigned IP locally: ip={assigned_ip}"
    ));
    let prefix_len = session.active_network_id.as_deref().and_then(|network_id| {
        client
            .network_prefix_len(&session.access_token, network_id, None)
            .ok()
    });
    let _ = runtime.dispatch(ClientCommand::SyncAssignedIp(AssignedIpPayload {
        virtual_ip: assigned_ip,
        prefix_len,
    }));
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
