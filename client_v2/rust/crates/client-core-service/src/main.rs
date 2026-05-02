#![cfg_attr(target_os = "windows", windows_subsystem = "windows")]

mod control_plane;
mod control_tasks;
mod control_transport;
mod control_transport_worker;

use std::{
    ffi::OsString,
    fs::{self, OpenOptions},
    io::{BufRead, BufReader, Write},
    net::{TcpListener, TcpStream, ToSocketAddrs},
    path::PathBuf,
    sync::{Arc, Condvar, Mutex},
    thread,
    time::{Duration, Instant, SystemTime, UNIX_EPOCH},
};

use anyhow::{Context, Result};
use client_core::{
    AssignedIpPayload, AuthPayload, ClientCommand, ClientRuntime, ClientViewState, PlatformNetwork,
};
use client_core_platform::PlatformNetworkImpl;
use serde::{Deserialize, Serialize};
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

use crate::control_plane::{
    local_stable_device_id, ControlDevice, ControlPlaneClient, MqttCredential, RelayCandidate,
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

const DEFAULT_SERVICE_HOST: &str = "127.0.0.1:46392";
#[cfg(target_os = "windows")]
const WINDOWS_SERVICE_NAME: &str = "SLANClientV2Service";
pub(crate) const BUSINESS_SESSION_CHANGED: &str = "session.changed";
pub(crate) const BUSINESS_NETWORK_SWITCH_FINISHED: &str = "network.switch.finished";
pub(crate) const BUSINESS_NETWORK_SWITCH_FAILED: &str = "network.switch.failed";
pub(crate) const BUSINESS_NETWORK_RUNTIME_CHANGED: &str = "network.runtime.changed";
pub(crate) const BUSINESS_CONTROL_SYNC_CHANGED: &str = "control.sync.changed";
pub(crate) const BUSINESS_STATE_CHANGED: &str = "state.changed";

#[cfg(target_os = "windows")]
define_windows_service!(ffi_service_main, service_main);

#[derive(Debug, Deserialize)]
struct ServiceRequest {
    method: String,
    #[serde(default)]
    args: Value,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct PersistedSession {
    pub(crate) access_token: String,
    pub(crate) refresh_token: Option<String>,
    pub(crate) user_id: String,
    pub(crate) user_label: String,
    pub(crate) device_id: Option<String>,
    pub(crate) active_network_id: Option<String>,
    pub(crate) virtual_ip: Option<String>,
    #[serde(default)]
    pub(crate) relay_candidates: Vec<PersistedRelayCandidate>,
    pub(crate) mqtt: Option<MqttCredential>,
    pub(crate) expires_in: Option<u64>,
    pub(crate) authenticated_at_ms: u64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct PersistedRelayCandidate {
    pub(crate) endpoint_id: String,
    pub(crate) transport: String,
    pub(crate) address: String,
    #[serde(default)]
    pub(crate) country_code: Option<String>,
    #[serde(default)]
    pub(crate) region_id: Option<String>,
    #[serde(default)]
    pub(crate) cluster_id: Option<String>,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
struct RelayCandidateSelection {
    endpoint_id: String,
    transport: String,
    address: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    country_code: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    region_id: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    cluster_id: Option<String>,
    reachable: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    rtt_ms: Option<u32>,
    path_score: u32,
    selected: bool,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
struct RelayCandidateListResponse {
    network_id: Option<String>,
    refreshed: bool,
    candidates: Vec<RelayCandidateSelection>,
    best: Option<RelayCandidateSelection>,
}

#[derive(Debug, Default)]
struct ControlSyncThrottle {
    last_sync_ms: u64,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
struct MarkControlAckedRequest {
    task_id: String,
}

#[derive(Debug, Default)]
pub(crate) struct StateChangeNotifier {
    revision: Mutex<u64>,
    event: Mutex<Option<StoredBusinessEvent>>,
    changed: Condvar,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
struct WatchStateRequest {
    #[serde(default)]
    last_revision: u64,
    #[serde(default = "default_watch_timeout_ms")]
    timeout_ms: u64,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
struct WatchStateResponse {
    revision: u64,
    state: ClientViewState,
}

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
struct StoredBusinessEvent {
    business_type: String,
    business_data: Value,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
struct WatchBusinessEventRequest {
    #[serde(default)]
    last_revision: u64,
    #[serde(default = "default_watch_timeout_ms")]
    timeout_ms: u64,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
struct WatchBusinessEventResponse {
    revision: u64,
    business_type: String,
    business_data: Value,
    snapshot: ClientViewState,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum CommandOrigin {
    Upstream,
    Downstream,
}

fn main() -> Result<()> {
    if std::env::args().any(|arg| arg == "--ensure-device-id") {
        println!("{}", local_stable_device_id()?);
        return Ok(());
    }
    if std::env::args()
        .any(|arg| arg == "--install-adapter" || arg == "--prepare-adapter" || arg == "--driver")
    {
        PlatformNetworkImpl::default().install_adapter()?;
        return Ok(());
    }
    #[cfg(target_os = "windows")]
    if std::env::args().any(|arg| arg == "--windows-service") {
        return service_dispatcher::start(WINDOWS_SERVICE_NAME, ffi_service_main)
            .context("start Windows service dispatcher");
    }

    run_service_server()
}

fn run_service_server() -> Result<()> {
    log_service_error("client-core-service starting");
    ensure_elevated_runtime()?;

    let bind_address = std::env::var("SLAN_CLIENT_CORE_SERVICE_HOST")
        .unwrap_or_else(|_| DEFAULT_SERVICE_HOST.to_string());
    let listener = TcpListener::bind(&bind_address)
        .with_context(|| format!("bind client-core-service on {bind_address}"))?;
    let mut initial_runtime = ClientRuntime::new(PlatformNetworkImpl::default());
    if let Some(session) = load_valid_registered_session() {
        let _ = initial_runtime.dispatch(ClientCommand::ApplyAuthCallback(session.into()));
    }
    if let Err(error) = PlatformNetworkImpl::default().install_adapter() {
        log_service_error(format!(
            "client-core-service failed to prepare Wintun adapter: {error:#}"
        ));
    }
    let runtime = Arc::new(Mutex::new(initial_runtime));
    let task_queue = Arc::new(Mutex::new(ControlTaskQueue::load_default()));
    let sync_throttle = Arc::new(Mutex::new(ControlSyncThrottle::default()));
    let transport_worker_state = Arc::new(Mutex::new(ControlTransportWorkerState::default()));
    let state_notifier = Arc::new(StateChangeNotifier::default());
    spawn_auth_callback_poller(Arc::clone(&runtime), Arc::clone(&state_notifier));
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
    println!("client-core-service listening on {bind_address}");

    for stream in listener.incoming() {
        let stream = stream.context("accept client-core-service connection")?;
        let runtime = Arc::clone(&runtime);
        let task_queue = Arc::clone(&task_queue);
        let sync_throttle = Arc::clone(&sync_throttle);
        let state_notifier = Arc::clone(&state_notifier);
        thread::spawn(move || {
            if let Err(error) =
                handle_connection(stream, runtime, task_queue, sync_throttle, state_notifier)
            {
                log_service_error(format!("client-core-service connection error: {error:#}"));
            }
        });
    }
    Ok(())
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

fn handle_connection(
    stream: TcpStream,
    runtime: Arc<Mutex<ClientRuntime<PlatformNetworkImpl>>>,
    task_queue: Arc<Mutex<ControlTaskQueue>>,
    sync_throttle: Arc<Mutex<ControlSyncThrottle>>,
    state_notifier: Arc<StateChangeNotifier>,
) -> Result<()> {
    let mut writer = stream.try_clone().context("clone service stream")?;
    let mut reader = BufReader::new(stream);
    let mut line = String::new();
    while reader.read_line(&mut line)? > 0 {
        let response = route_request(
            line.trim(),
            &runtime,
            &task_queue,
            &sync_throttle,
            &state_notifier,
        )
        .unwrap_or_else(|error| {
            let message = error.to_string();
            log_service_error(format!("client-core-service request failed: {message}"));
            error_state_json(message)
        });
        writer.write_all(response.as_bytes())?;
        writer.write_all(b"\n")?;
        writer.flush()?;
        line.clear();
    }
    Ok(())
}

fn route_request(
    line: &str,
    runtime: &Arc<Mutex<ClientRuntime<PlatformNetworkImpl>>>,
    task_queue: &Arc<Mutex<ControlTaskQueue>>,
    sync_throttle: &Arc<Mutex<ControlSyncThrottle>>,
    state_notifier: &Arc<StateChangeNotifier>,
) -> Result<String> {
    let request: ServiceRequest = serde_json::from_str(line).context("decode service request")?;
    if request.method == "watchState" {
        return handle_watch_state(request, runtime, state_notifier);
    }
    if request.method == "watchBusinessEvent" {
        return handle_watch_business_event(request, runtime, state_notifier);
    }
    if request.method == "state" {
        return handle_state_snapshot(runtime);
    }
    if request.method == "start" {
        sync_control_assignment(runtime);
        mark_control_sync(sync_throttle);
    } else if request.method == "refresh" && should_sync_control(sync_throttle) {
        sync_control_assignment(runtime);
    }
    if request.method == "enqueueControlTask" {
        let response = handle_task_request(request, runtime, task_queue)?;
        publish_business_event(
            state_notifier,
            BUSINESS_CONTROL_SYNC_CHANGED,
            serde_json::json!({"method": "enqueueControlTask"}),
        );
        return Ok(response);
    }
    if request.method == "enqueueDownstreamControlTask" {
        let response = handle_downstream_task_request(request, runtime, task_queue)?;
        publish_business_event(
            state_notifier,
            BUSINESS_CONTROL_SYNC_CHANGED,
            serde_json::json!({"method": "enqueueDownstreamControlTask"}),
        );
        return Ok(response);
    }
    if request.method == "ingestDownstreamControlMessage" {
        let response = handle_downstream_control_message(request, runtime, task_queue)?;
        publish_business_event(
            state_notifier,
            BUSINESS_CONTROL_SYNC_CHANGED,
            serde_json::json!({"method": "ingestDownstreamControlMessage"}),
        );
        return Ok(response);
    }
    if request.method == "pendingControlAcks" {
        return handle_pending_control_acks(task_queue);
    }
    if request.method == "markControlAcked" {
        return handle_mark_control_acked(request, task_queue);
    }
    if request.method == "controlTransportOutbox" {
        return handle_control_transport_outbox(request, runtime, task_queue);
    }
    if request.method == "markTransportPublished" {
        return handle_mark_transport_published(request, runtime, task_queue);
    }
    if request.method == "relayCandidates" || request.method == "refreshRelayCandidates" {
        return handle_relay_candidates(request.method == "refreshRelayCandidates");
    }
    let should_notify = matches!(
        request.method.as_str(),
        "start"
            | "dispatch"
            | "refresh"
            | "shutdownNetwork"
            | "activateNetwork"
            | "deactivateNetwork"
            | "logout"
    );
    let response = handle_request(request, runtime)?;
    if should_notify {
        publish_method_business_event(state_notifier, line, &response);
    }
    Ok(response)
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

fn handle_request(
    request: ServiceRequest,
    runtime: &Arc<Mutex<ClientRuntime<PlatformNetworkImpl>>>,
) -> Result<String> {
    let mut runtime = runtime.lock().expect("client runtime mutex poisoned");
    let state = match request.method.as_str() {
        "start" => {
            refresh_startup_session(&mut runtime);
            match runtime.refresh() {
                Ok(()) => runtime.state().clone(),
                Err(error) => state_with_error(runtime.state(), error.to_string()),
            }
        }
        "state" => match runtime.refresh() {
            Ok(()) => runtime.state().clone(),
            Err(error) => state_with_error(runtime.state(), error.to_string()),
        },
        "dispatch" => {
            let command: ClientCommand =
                serde_json::from_value(request.args).context("decode client command")?;
            let is_login_with_browser = matches!(command, ClientCommand::LoginWithBrowser);
            let mut state = dispatch_with_side_effects(&mut runtime, command);
            if is_login_with_browser
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
        "activateNetwork" => activate_network_from_latest_control(&mut runtime),
        "deactivateNetwork" => {
            dispatch_with_side_effects(&mut runtime, ClientCommand::DisableNetwork)
        }
        "refresh" => match runtime.refresh() {
            Ok(()) => runtime.state().clone(),
            Err(error) => state_with_error(runtime.state(), error.to_string()),
        },
        "shutdownNetwork" => {
            dispatch_with_side_effects(&mut runtime, ClientCommand::DisableNetwork)
        }
        "controlTransportStatus" => {
            return serde_json::to_string(&control_transport_status()?)
                .context("encode control transport status")
        }
        "controlTransportPlan" => {
            return serde_json::to_string(&control_transport_plan()?)
                .context("encode control transport plan")
        }
        "controlTransportCadence" => {
            return serde_json::to_string(&control_transport_cadence())
                .context("encode control transport cadence")
        }
        "consoleLoginKey" => {
            return serde_json::to_string(&console_login_key()?).context("encode console login key")
        }
        "controlTransportTickPlan" => {
            return serde_json::to_string(&control_transport_tick_plan(request.args)?)
                .context("encode control transport tick plan")
        }
        other => {
            let command_json = serde_json::json!({ "type": other });
            let command: ClientCommand = serde_json::from_value(command_json)
                .with_context(|| format!("unsupported service method {other}"))?;
            dispatch_with_side_effects(&mut runtime, command)
        }
    };
    serde_json::to_string(&state).context("encode client state")
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
    let session = load_session()?;
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

fn relay_candidates_response(refresh: bool) -> Result<RelayCandidateListResponse> {
    let mut session = load_network_session()?;
    let network_id = ensure_active_network_id(&mut session)?;
    let refreshed = if refresh {
        refresh_relay_candidates_for_session(&mut session, &network_id)?
    } else {
        false
    };
    let selections = select_relay_candidates(&session.relay_candidates);
    let best = selections.iter().find(|item| item.selected).cloned();
    Ok(RelayCandidateListResponse {
        network_id: Some(network_id),
        refreshed,
        candidates: selections,
        best,
    })
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
    let command = match task.action {
        ControlTaskAction::EnableNetwork => ClientCommand::EnableNetwork,
        ControlTaskAction::DisableNetwork => ClientCommand::DisableNetwork,
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
        ClientCommand::ApplyAuthCallback(payload) => {
            let session = hydrate_session_from_control_plane(payload.clone())
                .unwrap_or_else(|_| PersistedSession::from(payload.clone()));
            let side_effect = persist_session(&session);
            (
                ClientCommand::ApplyAuthCallback(session.into()),
                side_effect,
            )
        }
        ClientCommand::SyncAssignedIp(payload) => {
            let mut session = load_session().unwrap_or_else(|_| PersistedSession::empty());
            session.virtual_ip = Some(payload.virtual_ip.clone());
            let side_effect = persist_session(&session);
            (ClientCommand::SyncAssignedIp(payload), side_effect)
        }
        ClientCommand::Logout => {
            deactivate_control_network();
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
    let _ = runtime.dispatch(ClientCommand::ApplyAuthCallback(session.clone().into()));
    activate_control_network_for_session(runtime, &mut session)
}

fn activate_network_from_latest_control<P>(runtime: &mut ClientRuntime<P>) -> ClientViewState
where
    P: client_core::PlatformNetwork,
{
    match load_network_session()
        .and_then(|mut session| {
            let _ = runtime.dispatch(ClientCommand::ApplyAuthCallback(session.clone().into()));
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

fn load_network_session() -> Result<PersistedSession> {
    let session = load_session()?;
    if session.access_token.trim().is_empty() {
        let _ = remove_session();
        return Err(anyhow::anyhow!("login required"));
    }
    if session_is_expired(&session) {
        let _ = remove_session();
        return Err(anyhow::anyhow!("session expired; please login again"));
    }
    ensure_session_device_registered(session).map_err(|error| {
        if session_auth_invalid_error(&error) {
            let _ = remove_session();
        }
        error
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
    let refreshed_relay_count = refresh_relay_candidates_for_session(session, &network_id)
        .map(|_| session.relay_candidates.len())
        .unwrap_or_else(|error| {
            log_service_error(format!(
                "client-core-service relay candidate refresh skipped before enable: {error:#}"
            ));
            session.relay_candidates.len()
        });
    let best_relay = best_relay_candidate(&session.relay_candidates);
    let activation = client.activate_network(&session.access_token, &device_id, &network_id)?;
    log_service_error(format!(
        "client-core-service enable preflight ok: ip={}/{} dns={} routes={} peers={} relays={} refreshedRelays={} bestRelay={}",
        activation.virtual_ip,
        activation.prefix_len,
        activation.dns_servers.len(),
        activation.routes.len(),
        activation.peer_count,
        activation.relay_candidates.len(),
        refreshed_relay_count,
        best_relay
            .as_ref()
            .map(|relay| format!("{}:{} score={}", relay.transport, relay.address, relay.path_score))
            .unwrap_or_else(|| "none".to_string())
    ));
    session.virtual_ip = Some(activation.virtual_ip.clone());
    if !activation.relay_candidates.is_empty() {
        session.relay_candidates = sorted_persisted_relay_candidates(
            activation
                .relay_candidates
                .iter()
                .map(persisted_relay_candidate)
                .collect(),
        );
    }
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
        &activation.routes,
    )?;
    Ok(())
}

fn ensure_active_network_id(session: &mut PersistedSession) -> Result<String> {
    if session.active_network_id.is_none() {
        let client = ControlPlaneClient::from_env();
        session.active_network_id = client.active_network_id(&session.access_token)?;
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
    session.relay_candidates = sorted_persisted_relay_candidates(
        candidates.iter().map(persisted_relay_candidate).collect(),
    );
    persist_session(session)?;
    Ok(true)
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
    let devices = client.list_devices(&session.access_token)?;
    let Some(device) = devices.into_iter().find(|item| item.device_id == device_id) else {
        return Err(anyhow::anyhow!(
            "device unavailable: current device is not registered"
        ));
    };
    sync_session_device_fields(&mut session, &device);
    if !control_device_network_available(&device) {
        session.virtual_ip = None;
        persist_session(&session)?;
        return Err(anyhow::anyhow!(
            "device unavailable: current device has been disabled by network admin"
        ));
    }
    let Some(virtual_ip) = device
        .current_virtual_ip
        .or(device.virtual_ip)
        .filter(|value| !value.trim().is_empty())
    else {
        session.virtual_ip = None;
        persist_session(&session)?;
        return Err(anyhow::anyhow!(
            "device unavailable: current device has no assigned virtual IP"
        ));
    };
    session.virtual_ip = Some(virtual_ip.clone());
    let prefix_len = session.active_network_id.as_deref().and_then(|network_id| {
        client
            .network_prefix_len(&session.access_token, network_id, None)
            .ok()
    });
    let _ = runtime.dispatch(ClientCommand::SyncAssignedIp(AssignedIpPayload {
        virtual_ip,
        prefix_len,
    }));
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

fn sorted_persisted_relay_candidates(
    mut candidates: Vec<PersistedRelayCandidate>,
) -> Vec<PersistedRelayCandidate> {
    let selections = select_relay_candidates(&candidates);
    let rank_by_endpoint = selections
        .iter()
        .enumerate()
        .map(|(index, item)| (item.endpoint_id.clone(), index))
        .collect::<std::collections::HashMap<_, _>>();
    candidates.sort_by_key(|candidate| {
        rank_by_endpoint
            .get(candidate.endpoint_id.as_str())
            .copied()
            .unwrap_or(usize::MAX)
    });
    candidates
}

fn best_relay_candidate(candidates: &[PersistedRelayCandidate]) -> Option<RelayCandidateSelection> {
    select_relay_candidates(candidates)
        .into_iter()
        .find(|candidate| candidate.selected)
}

fn select_relay_candidates(candidates: &[PersistedRelayCandidate]) -> Vec<RelayCandidateSelection> {
    let mut selections = candidates
        .iter()
        .filter(|candidate| {
            !candidate.endpoint_id.trim().is_empty()
                && !candidate.transport.trim().is_empty()
                && !candidate.address.trim().is_empty()
        })
        .map(score_relay_candidate)
        .collect::<Vec<_>>();
    selections.sort_by(|left, right| {
        right
            .reachable
            .cmp(&left.reachable)
            .then_with(|| left.path_score.cmp(&right.path_score))
            .then_with(|| left.endpoint_id.cmp(&right.endpoint_id))
    });
    for (index, selection) in selections.iter_mut().enumerate() {
        selection.selected = index == 0 && selection.reachable;
    }
    selections
}

fn score_relay_candidate(candidate: &PersistedRelayCandidate) -> RelayCandidateSelection {
    let transport = candidate.transport.trim().to_ascii_lowercase();
    let mut reachable = true;
    let mut rtt_ms = None;
    let path_score = match transport.as_str() {
        "tcp" | "tls" => match probe_relay_tcp_rtt_ms(&candidate.address) {
            Some(rtt) => {
                rtt_ms = Some(rtt);
                rtt.saturating_add(if transport == "tls" { 150 } else { 100 })
            }
            None => {
                reachable = false;
                10_000
            }
        },
        "quic" => 700,
        "udp" => 800,
        _ => 9_000,
    };
    RelayCandidateSelection {
        endpoint_id: candidate.endpoint_id.clone(),
        transport: candidate.transport.clone(),
        address: candidate.address.clone(),
        country_code: candidate.country_code.clone(),
        region_id: candidate.region_id.clone(),
        cluster_id: candidate.cluster_id.clone(),
        reachable,
        rtt_ms,
        path_score,
        selected: false,
    }
}

fn probe_relay_tcp_rtt_ms(address: &str) -> Option<u32> {
    let socket = address
        .to_socket_addrs()
        .ok()
        .and_then(|mut values| values.next())?;
    let started = Instant::now();
    TcpStream::connect_timeout(&socket, Duration::from_millis(750)).ok()?;
    Some(started.elapsed().as_millis().min(u32::MAX as u128) as u32)
}

pub(crate) fn persist_relay_candidates_from_network_map(map: &Value) -> Result<usize> {
    let candidates = extract_persisted_relay_candidates_from_network_map(map);
    if candidates.is_empty() {
        return Ok(0);
    }
    let mut session = load_session()?;
    session.relay_candidates = sorted_persisted_relay_candidates(candidates);
    persist_session(&session)?;
    Ok(session.relay_candidates.len())
}

fn extract_persisted_relay_candidates_from_network_map(
    map: &Value,
) -> Vec<PersistedRelayCandidate> {
    map.get("relayRegions")
        .and_then(Value::as_array)
        .into_iter()
        .flatten()
        .flat_map(|region| {
            let country_code = optional_trimmed_string(region.get("countryCode"));
            let region_id = optional_trimmed_string(region.get("regionId"));
            let cluster_id = optional_trimmed_string(region.get("clusterId"));
            region
                .get("endpoints")
                .and_then(Value::as_array)
                .into_iter()
                .flatten()
                .filter_map(move |endpoint| {
                    let endpoint_id = optional_trimmed_string(endpoint.get("endpointId"))?;
                    let transport = optional_trimmed_string(endpoint.get("transport"))?;
                    let address = optional_trimmed_string(endpoint.get("address"))?;
                    Some(PersistedRelayCandidate {
                        endpoint_id,
                        transport,
                        address,
                        country_code: country_code.clone(),
                        region_id: region_id.clone(),
                        cluster_id: cluster_id.clone(),
                    })
                })
                .collect::<Vec<_>>()
        })
        .collect()
}

fn optional_trimmed_string(value: Option<&Value>) -> Option<String> {
    value
        .and_then(Value::as_str)
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .map(str::to_string)
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

fn default_watch_timeout_ms() -> u64 {
    30_000
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
    let business_type = match method.as_str() {
        "activateNetwork" => state
            .as_ref()
            .and_then(|state| state.error.as_ref())
            .map(|_| BUSINESS_NETWORK_SWITCH_FAILED)
            .unwrap_or(BUSINESS_NETWORK_SWITCH_FINISHED),
        "deactivateNetwork" | "shutdownNetwork" => state
            .as_ref()
            .and_then(|state| state.error.as_ref())
            .map(|_| BUSINESS_NETWORK_SWITCH_FAILED)
            .unwrap_or(BUSINESS_NETWORK_SWITCH_FINISHED),
        "logout" => BUSINESS_SESSION_CHANGED,
        "dispatch" => BUSINESS_SESSION_CHANGED,
        "start" | "refresh" => BUSINESS_STATE_CHANGED,
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

fn spawn_auth_callback_poller(
    runtime: Arc<Mutex<ClientRuntime<PlatformNetworkImpl>>>,
    state_notifier: Arc<StateChangeNotifier>,
) {
    thread::spawn(move || loop {
        thread::sleep(Duration::from_secs(1));
        let callback_id = {
            let runtime = runtime.lock().expect("client runtime mutex poisoned");
            runtime.state().auth_callback_id.clone()
        };
        let Some(callback_id) = callback_id.filter(|value| !value.trim().is_empty()) else {
            continue;
        };
        let client = ControlPlaneClient::from_env();
        let Ok(Some(payload)) = client.callback_payload(&callback_id) else {
            continue;
        };
        let mut runtime = runtime.lock().expect("client runtime mutex poisoned");
        let _ = dispatch_with_side_effects(&mut runtime, ClientCommand::ApplyAuthCallback(payload));
        let business_data =
            serde_json::to_value(runtime.state()).unwrap_or_else(|_| serde_json::json!({}));
        publish_business_event(&state_notifier, BUSINESS_SESSION_CHANGED, business_data);
    });
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
            match task_queue.take_next_pending() {
                Ok(task) => task,
                Err(_) => None,
            }
        };
        let Some(task) = task else {
            continue;
        };

        let _ = execute_control_task(&runtime, &task_queue, task);
        notify_state_changed(&state_notifier);
    });
}

fn sync_control_assignment(runtime: &Arc<Mutex<ClientRuntime<PlatformNetworkImpl>>>) {
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
            let _ = runtime.dispatch(ClientCommand::ApplyAuthCallback(session.into()));
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
            let _ = runtime.dispatch(ClientCommand::ApplyAuthCallback(session.into()));
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

fn sync_session_device_fields(session: &mut PersistedSession, device: &ControlDevice) {
    session.device_id = Some(device.device_id.clone());
    if device.mqtt.is_some() {
        session.mqtt = device.mqtt.clone();
    }
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

#[cfg(test)]
mod tests {
    use super::{select_relay_candidates, status_is_managed_disabled, PersistedRelayCandidate};

    #[test]
    fn online_presence_statuses_do_not_disable_local_network() {
        for status in [
            None,
            Some(""),
            Some("active"),
            Some("online"),
            Some("offline"),
        ] {
            assert!(!status_is_managed_disabled(status));
        }
    }

    #[test]
    fn managed_disable_statuses_disable_local_network() {
        for status in [
            Some("disabled"),
            Some("suspended"),
            Some("blocked"),
            Some("revoked"),
            Some("deleted"),
            Some("removed"),
        ] {
            assert!(status_is_managed_disabled(status));
        }
    }

    #[test]
    fn relay_selection_prefers_reachable_low_score_candidate() {
        let selections = select_relay_candidates(&[
            PersistedRelayCandidate {
                endpoint_id: "relay-bad".to_string(),
                transport: "unknown".to_string(),
                address: "127.0.0.1:1".to_string(),
                country_code: None,
                region_id: None,
                cluster_id: None,
            },
            PersistedRelayCandidate {
                endpoint_id: "relay-quic".to_string(),
                transport: "quic".to_string(),
                address: "127.0.0.1:9000".to_string(),
                country_code: Some("CN".to_string()),
                region_id: None,
                cluster_id: None,
            },
            PersistedRelayCandidate {
                endpoint_id: "relay-udp".to_string(),
                transport: "udp".to_string(),
                address: "127.0.0.1:9001".to_string(),
                country_code: Some("CN".to_string()),
                region_id: None,
                cluster_id: None,
            },
        ]);
        assert_eq!(selections[0].endpoint_id, "relay-quic");
        assert!(selections[0].selected);
        assert_eq!(selections[1].endpoint_id, "relay-udp");
        assert!(!selections[1].selected);
    }
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

impl From<PersistedSession> for AuthPayload {
    fn from(session: PersistedSession) -> Self {
        Self {
            access_token: session.access_token,
            refresh_token: session.refresh_token,
            user_id: session.user_id,
            user_label: session.user_label,
            device_id: session.device_id,
            virtual_ip: session.virtual_ip,
            expires_in: session.expires_in,
        }
    }
}

impl From<AuthPayload> for PersistedSession {
    fn from(payload: AuthPayload) -> Self {
        Self {
            access_token: payload.access_token,
            refresh_token: payload.refresh_token,
            user_id: payload.user_id,
            user_label: payload.user_label,
            device_id: payload.device_id,
            active_network_id: None,
            virtual_ip: payload.virtual_ip,
            relay_candidates: Vec::new(),
            mqtt: None,
            expires_in: payload.expires_in,
            authenticated_at_ms: current_timestamp_ms(),
        }
    }
}

impl PersistedSession {
    fn empty() -> Self {
        Self {
            access_token: String::new(),
            refresh_token: None,
            user_id: String::new(),
            user_label: String::new(),
            device_id: None,
            active_network_id: None,
            virtual_ip: None,
            relay_candidates: Vec::new(),
            mqtt: None,
            expires_in: None,
            authenticated_at_ms: current_timestamp_ms(),
        }
    }
}

fn load_valid_registered_session() -> Option<PersistedSession> {
    let Ok(session) = load_session() else {
        return None;
    };
    if session.access_token.trim().is_empty() {
        let _ = remove_session();
        return None;
    }
    if session_is_expired(&session) {
        let _ = remove_session();
        return None;
    }
    match ensure_session_device_registered(session.clone()) {
        Ok(session) => Some(session),
        Err(error) if session_auth_invalid_error(&error) => {
            eprintln!("client-core-service session invalid; clearing local session: {error:#}");
            let _ = remove_session();
            None
        }
        Err(error) => {
            eprintln!("client-core-service startup device registration skipped: {error:#}");
            Some(session)
        }
    }
}

fn refresh_startup_session<P>(runtime: &mut ClientRuntime<P>)
where
    P: client_core::PlatformNetwork,
{
    let Some(session) = load_valid_registered_session() else {
        let _ = runtime.dispatch(ClientCommand::Logout);
        return;
    };
    let _ = runtime.dispatch(ClientCommand::ApplyAuthCallback(session.into()));
}

fn session_is_expired(session: &PersistedSession) -> bool {
    let Some(expires_in) = session.expires_in else {
        return false;
    };
    let lifetime_ms = expires_in.saturating_mul(1_000);
    let expires_at_ms = session.authenticated_at_ms.saturating_add(lifetime_ms);
    current_timestamp_ms().saturating_add(30_000) >= expires_at_ms
}

fn session_auth_invalid_error(error: &anyhow::Error) -> bool {
    let message = error.to_string().to_ascii_lowercase();
    message.contains("http 401")
        || message.contains("unauthorized")
        || message.contains("invalid token")
        || message.contains("token expired")
}

fn ensure_session_device_registered(mut session: PersistedSession) -> Result<PersistedSession> {
    if session.access_token.trim().is_empty() {
        return Ok(session);
    }
    let client = ControlPlaneClient::from_env();
    let device = client.ensure_device(&session.access_token, session.device_id.as_deref())?;
    sync_session_device_fields(&mut session, &device);
    if session.active_network_id.is_none() {
        if let Ok(Some(network_id)) = client.active_network_id(&session.access_token) {
            session.active_network_id = Some(network_id);
        }
    }
    if session.virtual_ip.is_none() {
        if let Some(virtual_ip) = device
            .current_virtual_ip
            .or(device.virtual_ip)
            .filter(|value| !value.trim().is_empty())
        {
            session.virtual_ip = Some(virtual_ip);
        }
    }
    persist_session(&session)?;
    Ok(session)
}

fn hydrate_session_from_control_plane(payload: AuthPayload) -> Result<PersistedSession> {
    let client = ControlPlaneClient::from_env();
    let mut session = PersistedSession::from(payload);
    let device = client.ensure_device(&session.access_token, session.device_id.as_deref())?;
    sync_session_device_fields(&mut session, &device);
    if let Ok(Some(network_id)) = client.active_network_id(&session.access_token) {
        session.active_network_id = Some(network_id);
    }
    if let Some(virtual_ip) = device
        .current_virtual_ip
        .filter(|value| !value.trim().is_empty())
    {
        session.virtual_ip = Some(virtual_ip);
        return Ok(session);
    }

    let devices = client.list_devices(&session.access_token)?;
    if let Some(current) = devices
        .into_iter()
        .find(|item| Some(item.device_id.as_str()) == session.device_id.as_deref())
    {
        if let Some(virtual_ip) = current
            .current_virtual_ip
            .or(current.virtual_ip)
            .filter(|value| !value.trim().is_empty())
        {
            session.virtual_ip = Some(virtual_ip);
        }
    }
    Ok(session)
}

fn report_runtime_state(state: &ClientViewState) {
    let Ok(mut session) = load_session() else {
        return;
    };
    if session.active_network_id.is_none() {
        let client = ControlPlaneClient::from_env();
        if let Ok(Some(network_id)) = client.active_network_id(&session.access_token) {
            session.active_network_id = Some(network_id);
            let _ = persist_session(&session);
        }
    }
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
    let _ = client.report_network_state(
        &session.access_token,
        device_id,
        network_id,
        state.network_enabled,
        state.virtual_ip.as_deref(),
    );
}

fn session_file_path() -> PathBuf {
    let base = app_data_dir();
    base.join("SLAN").join("client-v2-session.json")
}

fn app_data_dir() -> PathBuf {
    if cfg!(target_os = "windows") {
        return std::env::var_os("ProgramData")
            .map(PathBuf::from)
            .unwrap_or_else(|| PathBuf::from(r"C:\ProgramData"));
    }
    if cfg!(target_os = "macos") {
        return PathBuf::from("/Library/Application Support");
    }
    std::env::var_os("SLAN_STATE_DIR")
        .map(PathBuf::from)
        .unwrap_or_else(|| PathBuf::from("/var/lib"))
}

pub(crate) fn load_session() -> Result<PersistedSession> {
    let path = session_file_path();
    let payload = fs::read(&path).with_context(|| format!("read {}", path.display()))?;
    serde_json::from_slice(&payload).with_context(|| format!("decode {}", path.display()))
}

pub(crate) fn persist_session(session: &PersistedSession) -> Result<()> {
    let path = session_file_path();
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent).with_context(|| format!("create {}", parent.display()))?;
    }
    let payload = serde_json::to_vec_pretty(session).context("encode client session")?;
    fs::write(&path, payload).with_context(|| format!("write {}", path.display()))
}

fn remove_session() -> Result<()> {
    let path = session_file_path();
    if path.exists() {
        fs::remove_file(&path).with_context(|| format!("remove {}", path.display()))?;
    }
    Ok(())
}

pub(crate) fn current_timestamp_ms() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|duration| duration.as_millis() as u64)
        .unwrap_or_default()
}
