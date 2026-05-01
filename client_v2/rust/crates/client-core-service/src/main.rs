mod control_plane;
mod control_tasks;
mod control_transport;
mod control_transport_worker;

use std::{
    fs,
    io::{BufRead, BufReader, Write},
    net::{TcpListener, TcpStream},
    path::PathBuf,
    sync::{Arc, Condvar, Mutex},
    thread,
    time::{Duration, SystemTime, UNIX_EPOCH},
};

use anyhow::{Context, Result};
use client_core::{AssignedIpPayload, AuthPayload, ClientCommand, ClientRuntime, ClientViewState};
use client_core_platform::PlatformNetworkImpl;
use serde::{Deserialize, Serialize};
use serde_json::Value;

use crate::control_plane::{ControlDevice, ControlPlaneClient, MqttCredential};
use crate::control_tasks::{
    ControlTaskAction, ControlTaskDirection, ControlTaskQueue, EnqueueControlTaskRequest,
};
use crate::control_transport::{
    ControlTransportCadence, ControlTransportOutbox, ControlTransportOutboxRequest,
    ControlTransportPlan, ControlTransportStatus, ControlTransportTickPlan,
    ControlTransportTickRequest, DownstreamControlMessage, PublishedControlTransportMessage,
};
use crate::control_transport_worker::ControlTransportWorkerState;

const DEFAULT_SERVICE_HOST: &str = "127.0.0.1:46392";

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
    pub(crate) mqtt: Option<MqttCredential>,
    pub(crate) expires_in: Option<u64>,
    pub(crate) authenticated_at_ms: u64,
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
struct StateChangeNotifier {
    revision: Mutex<u64>,
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

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum CommandOrigin {
    Upstream,
    Downstream,
}

fn main() -> Result<()> {
    let bind_address = std::env::var("SLAN_CLIENT_CORE_SERVICE_HOST")
        .unwrap_or_else(|_| DEFAULT_SERVICE_HOST.to_string());
    let listener = TcpListener::bind(&bind_address)
        .with_context(|| format!("bind client-core-service on {bind_address}"))?;
    let mut initial_runtime = ClientRuntime::new(PlatformNetworkImpl::default());
    if let Ok(session) = load_session() {
        let _ = initial_runtime.dispatch(ClientCommand::ApplyAuthCallback(session.into()));
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
                eprintln!("client-core-service connection error: {error:#}");
            }
        });
    }
    Ok(())
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
        .unwrap_or_else(|error| error_state_json(error.to_string()));
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
        notify_state_changed(state_notifier);
        return Ok(response);
    }
    if request.method == "enqueueDownstreamControlTask" {
        let response = handle_downstream_task_request(request, runtime, task_queue)?;
        notify_state_changed(state_notifier);
        return Ok(response);
    }
    if request.method == "ingestDownstreamControlMessage" {
        let response = handle_downstream_control_message(request, runtime, task_queue)?;
        notify_state_changed(state_notifier);
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
    let should_notify = matches!(
        request.method.as_str(),
        "start" | "dispatch" | "refresh" | "shutdownNetwork"
    );
    let response = handle_request(request, runtime)?;
    if should_notify {
        notify_state_changed(state_notifier);
    }
    Ok(response)
}

fn handle_state_snapshot(
    runtime: &Arc<Mutex<ClientRuntime<PlatformNetworkImpl>>>,
) -> Result<String> {
    let state = match runtime.try_lock() {
        Ok(mut runtime) => match runtime.refresh() {
            Ok(()) => runtime.state().clone(),
            Err(error) => state_with_error(runtime.state(), error.to_string()),
        },
        Err(_) => busy_state_from_session(),
    };
    serde_json::to_string(&state).context("encode client state")
}

fn handle_request(
    request: ServiceRequest,
    runtime: &Arc<Mutex<ClientRuntime<PlatformNetworkImpl>>>,
) -> Result<String> {
    let mut runtime = runtime.lock().expect("client runtime mutex poisoned");
    let state = match request.method.as_str() {
        "start" | "state" => match runtime.refresh() {
            Ok(()) => runtime.state().clone(),
            Err(error) => state_with_error(runtime.state(), error.to_string()),
        },
        "dispatch" => {
            let command: ClientCommand =
                serde_json::from_value(request.args).context("decode client command")?;
            dispatch_with_side_effects(&mut runtime, command)
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

fn busy_state_from_session() -> ClientViewState {
    let mut state = ClientViewState::default();
    if let Ok(session) = load_session() {
        state.signed_in = !session.access_token.trim().is_empty();
        state.user_label = Some(session.user_label);
        state.device_id = session.device_id;
        state.virtual_ip = session.virtual_ip;
    }
    state.syncing = true;
    state.sync_reason = Some("networkTask".to_string());
    state.switch_enabled = false;
    state
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

fn control_transport_tick_plan(args: Value) -> Result<ControlTransportTickPlan> {
    let input: ControlTransportTickRequest =
        serde_json::from_value(args).context("decode control transport tick request")?;
    Ok(control_transport::control_transport_tick_plan(
        input,
        current_timestamp_ms(),
    ))
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
            .wait_timeout_while(revision, timeout, |current| {
                *current <= input.last_revision
            })
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
    let input: DownstreamControlMessage =
        serde_json::from_value(request.args).context("decode downstream control message")?;
    let accepted = control_transport::normalize_downstream_control_message(input)?;
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

fn dispatch_with_side_effects<P>(
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
        ClientCommand::Logout => (ClientCommand::Logout, remove_session()),
        other => (other, Ok(())),
    };
    if let Err(error) = side_effect {
        return state_with_error(runtime.state(), error.to_string());
    }

    let command_was_enable = matches!(&command, ClientCommand::EnableNetwork);
    let command_was_disable = matches!(&command, ClientCommand::DisableNetwork);

    if command_was_enable && origin == CommandOrigin::Upstream {
        if let Err(error) = activate_control_network(runtime) {
            return state_with_error(runtime.state(), error.to_string());
        }
    }
    if command_was_enable && origin == CommandOrigin::Downstream {
        if let Err(error) = sync_downstream_network_assignment(runtime) {
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
                }));
            }
        }
    }

    match runtime.dispatch(command) {
        Ok(mut state) => {
            if command_was_disable && state.error.is_none() {
                let _ = clear_session_virtual_ip();
                state.virtual_ip = None;
            }
            if state.virtual_ip.is_none() {
                if let Ok(session) = load_session() {
                    state.virtual_ip = session.virtual_ip;
                }
            }
            if command_was_enable || command_was_disable {
                report_runtime_state(&state);
            }
            state
        }
        Err(error) => state_with_error(runtime.state(), error.to_string()),
    }
}

fn activate_control_network<P>(runtime: &mut ClientRuntime<P>) -> Result<()>
where
    P: client_core::PlatformNetwork,
{
    let mut session = load_session()?;
    let Some(device_id) = session.device_id.clone().filter(|value| !value.is_empty()) else {
        return Ok(());
    };
    let client = ControlPlaneClient::from_env();
    if session.active_network_id.is_none() {
        session.active_network_id = client.active_network_id(&session.access_token)?;
    }
    let Some(network_id) = session.active_network_id.clone() else {
        persist_session(&session)?;
        return Ok(());
    };
    let Some(virtual_ip) =
        client.activate_network(&session.access_token, &device_id, &network_id)?
    else {
        session.virtual_ip = None;
        persist_session(&session)?;
        return Err(anyhow::anyhow!(
            "device unavailable: current device has no assigned virtual IP"
        ));
    };
    session.virtual_ip = Some(virtual_ip.clone());
    let _ = runtime.dispatch(ClientCommand::SyncAssignedIp(AssignedIpPayload {
        virtual_ip,
    }));
    persist_session(&session)
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
    let _ = runtime.dispatch(ClientCommand::SyncAssignedIp(AssignedIpPayload {
        virtual_ip,
    }));
    persist_session(&session)
}

fn deactivate_control_network() {
    let Ok(session) = load_session() else {
        return;
    };
    if session.virtual_ip.is_none() {
        return;
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
    let _ = client.deactivate_network(&session.access_token, device_id, network_id);
}

fn clear_session_virtual_ip() -> Result<()> {
    let mut session = load_session().unwrap_or_else(|_| PersistedSession::empty());
    session.virtual_ip = None;
    persist_session(&session)
}

fn state_with_error(state: &ClientViewState, error: String) -> ClientViewState {
    let mut state = state.clone();
    state.syncing = false;
    state.switch_enabled = true;
    state.error = Some(error);
    state
}

fn default_watch_timeout_ms() -> u64 {
    30_000
}

fn notify_state_changed(state_notifier: &StateChangeNotifier) {
    let mut revision = state_notifier
        .revision
        .lock()
        .expect("state revision mutex poisoned");
    *revision = revision.saturating_add(1);
    state_notifier.changed.notify_all();
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
        notify_state_changed(&state_notifier);
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
        if state != before {
            notify_state_changed(&state_notifier);
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
        let _ = persist_session(&session);
        return;
    };
    let Ok(devices) = client.list_devices(&session.access_token) else {
        let _ = persist_session(&session);
        return;
    };
    let Some(device) = devices.into_iter().find(|item| item.device_id == device_id) else {
        let _ = persist_session(&session);
        return;
    };
    sync_session_device_fields(&mut session, &device);
    if !control_device_network_available(&device) {
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
    let _ = runtime.dispatch(ClientCommand::SyncAssignedIp(AssignedIpPayload {
        virtual_ip: assigned_ip,
    }));
}

fn control_device_network_available(device: &control_plane::ControlDevice) -> bool {
    let device_status = device.status.as_deref().unwrap_or("active");
    let member_status = device.membership_status.as_deref().unwrap_or("active");
    status_is_active(device_status) && status_is_active(member_status)
}

fn sync_session_device_fields(session: &mut PersistedSession, device: &ControlDevice) {
    session.device_id = Some(device.device_id.clone());
    if device.mqtt.is_some() {
        session.mqtt = device.mqtt.clone();
    }
}

fn status_is_active(status: &str) -> bool {
    let status = status.trim();
    status.is_empty() || status.eq_ignore_ascii_case("active")
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
            mqtt: None,
            expires_in: None,
            authenticated_at_ms: current_timestamp_ms(),
        }
    }
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

fn persist_session(session: &PersistedSession) -> Result<()> {
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
