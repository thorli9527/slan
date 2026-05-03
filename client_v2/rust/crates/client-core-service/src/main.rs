#![cfg_attr(target_os = "windows", windows_subsystem = "windows")]

mod control_plane;
mod control_tasks;
mod control_transport;
mod control_transport_worker;
mod local_api;
#[cfg(test)]
mod main_tests;
mod relay_candidates;
mod relay_models;
mod relay_store;
mod session_store;

use std::{
    collections::BTreeMap,
    ffi::OsString,
    fs::{self, OpenOptions},
    io::{BufRead, BufReader, Write},
    net::{TcpListener, TcpStream},
    sync::{Arc, Condvar, Mutex},
    thread,
    time::Duration,
};

use anyhow::{Context, Result};
use client_core::{
    normalize_relay_transport, relay_path_kind_for_transport, AndroidVpnSessionConfig,
    AssignedIpPayload, ClientCommand, ClientRuntime, ClientViewState, PathCandidate, PathKind,
    PathState, PeerPathConfig, PlatformNetwork, PlatformNetworkDiagnostics, RelayDataPlaneConfig,
    RelayPeerSession,
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

use crate::control_plane::{
    local_stable_device_id, ControlPeer, ControlPlaneClient, RelayCandidate,
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
    LocalServiceMethod, MarkControlAckedRequest, ServiceRequest, StoredBusinessEvent,
    WatchBusinessEventRequest, WatchBusinessEventResponse, WatchStateRequest, WatchStateResponse,
    BUSINESS_CONTROL_SYNC_CHANGED, BUSINESS_NETWORK_RUNTIME_CHANGED,
    BUSINESS_NETWORK_SWITCH_FAILED, BUSINESS_NETWORK_SWITCH_FINISHED, BUSINESS_SESSION_CHANGED,
    BUSINESS_STATE_CHANGED,
};
use crate::relay_candidates::{
    best_relay_candidate, best_udp_relay_candidate, diagnose_direct_candidates,
    extract_persisted_relay_candidates_from_network_map, select_relay_candidates,
    sorted_persisted_relay_candidates,
};
use crate::relay_models::{
    PathDiagnoseDns, PathDiagnoseMtu, PathDiagnosePathCount, PathDiagnoseRelay,
    PathDiagnoseRelayPeer, PathDiagnoseResponse, PersistedRelayCandidate,
    RelayCandidateListResponse, RelayCandidateSelection, RelayRuntimeStats,
};
use crate::relay_store::{
    diagnostics_export_file_path, load_recent_relay_data_plane_policy_for_path,
    load_relay_runtime_stats, relay_path_policy, relay_payload_policy, relay_runtime_failure_total,
};
use crate::session_store::{
    app_data_dir, current_timestamp_ms, ensure_session_device_registered,
    hydrate_session_from_control_plane, load_session, load_valid_registered_session,
    persist_session, refresh_startup_session, remove_session, report_runtime_state,
    session_auth_invalid_error, session_is_expired, sync_session_device_fields, PersistedSession,
};

const DEFAULT_SERVICE_HOST: &str = "127.0.0.1:46392";
#[cfg(target_os = "windows")]
const WINDOWS_SERVICE_NAME: &str = "SLANClientV2Service";
const RELAY_MAINTENANCE_INTERVAL: Duration = Duration::from_secs(30);
const RELAY_TICKET_RENEW_INTERVAL_MS: u64 = 20 * 60 * 1000;
const RELAY_TICKET_RENEW_WINDOW_MS: u64 = 2 * 60 * 1000;
const RELAY_RECONFIGURE_BACKOFF_MS: u64 = 60 * 1000;
const RELAY_STATS_STALE_MS: u64 = 45 * 1000;
const RELAY_FAILURE_RECONFIGURE_DELTA: u64 = 5;

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
    sync_throttle: Arc<Mutex<ControlSyncThrottle>>,
    state_notifier: Arc<StateChangeNotifier>,
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
    let service_context = LocalServiceContext {
        runtime: Arc::clone(&runtime),
        task_queue: Arc::clone(&task_queue),
        sync_throttle: Arc::clone(&sync_throttle),
        state_notifier: Arc::clone(&state_notifier),
    };
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
    spawn_relay_data_plane_maintenance_worker(Arc::clone(&runtime), Arc::clone(&state_notifier));
    println!("client-core-service listening on {bind_address}");

    for stream in listener.incoming() {
        let stream = stream.context("accept client-core-service connection")?;
        let context = service_context.clone();
        thread::spawn(move || {
            if let Err(error) = handle_connection(stream, context) {
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

fn handle_connection(stream: TcpStream, context: LocalServiceContext) -> Result<()> {
    let mut writer = stream.try_clone().context("clone service stream")?;
    let mut reader = BufReader::new(stream);
    let mut line = String::new();
    while reader.read_line(&mut line)? > 0 {
        let response = route_request(line.trim(), &context).unwrap_or_else(|error| {
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

fn route_request(line: &str, context: &LocalServiceContext) -> Result<String> {
    let request: ServiceRequest = serde_json::from_str(line).context("decode service request")?;
    let method = LocalServiceMethod::parse(&request.method);
    match method {
        LocalServiceMethod::WatchState => {
            return handle_watch_state(request, &context.runtime, &context.state_notifier)
        }
        LocalServiceMethod::WatchBusinessEvent => {
            return handle_watch_business_event(request, &context.runtime, &context.state_notifier)
        }
        LocalServiceMethod::State => return handle_state_snapshot(&context.runtime),
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
        LocalServiceMethod::PendingControlAcks => {
            return handle_pending_control_acks(&context.task_queue)
        }
        LocalServiceMethod::MarkControlAcked => {
            return handle_mark_control_acked(request, &context.task_queue)
        }
        LocalServiceMethod::ControlTransportOutbox => {
            return handle_control_transport_outbox(request, &context.runtime, &context.task_queue)
        }
        LocalServiceMethod::MarkTransportPublished => {
            return handle_mark_transport_published(request, &context.runtime, &context.task_queue)
        }
        LocalServiceMethod::RelayCandidates => return handle_relay_candidates(false),
        LocalServiceMethod::RefreshRelayCandidates => return handle_relay_candidates(true),
        LocalServiceMethod::PrepareRelayDataPlane => return handle_prepare_relay_data_plane(),
        LocalServiceMethod::PathDiagnose => return handle_path_diagnose(),
        LocalServiceMethod::ExportDiagnostics => {
            return handle_export_diagnostics(&context.runtime)
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
        LocalServiceMethod::State => match runtime.refresh() {
            Ok(()) => runtime.state().clone(),
            Err(error) => state_with_error(runtime.state(), error.to_string()),
        },
        LocalServiceMethod::Dispatch => {
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
        LocalServiceMethod::ActivateNetwork => activate_network_from_latest_control(&mut runtime),
        LocalServiceMethod::DeactivateNetwork => {
            dispatch_with_side_effects(&mut runtime, ClientCommand::DisableNetwork)
        }
        LocalServiceMethod::Refresh => match runtime.refresh() {
            Ok(()) => runtime.state().clone(),
            Err(error) => state_with_error(runtime.state(), error.to_string()),
        },
        LocalServiceMethod::ShutdownNetwork => {
            dispatch_with_side_effects(&mut runtime, ClientCommand::DisableNetwork)
        }
        LocalServiceMethod::ControlTransportStatus => {
            return serde_json::to_string(&control_transport_status()?)
                .context("encode control transport status")
        }
        LocalServiceMethod::ControlTransportPlan => {
            return serde_json::to_string(&control_transport_plan()?)
                .context("encode control transport plan")
        }
        LocalServiceMethod::ControlTransportCadence => {
            return serde_json::to_string(&control_transport_cadence())
                .context("encode control transport cadence")
        }
        LocalServiceMethod::ConsoleLoginKey => {
            return serde_json::to_string(&console_login_key()?).context("encode console login key")
        }
        LocalServiceMethod::AndroidNetworkConfig => {
            return serde_json::to_string(&android_network_config_from_latest_control(
                &mut runtime,
            )?)
            .context("encode android network config")
        }
        LocalServiceMethod::ControlTransportTickPlan => {
            return serde_json::to_string(&control_transport_tick_plan(request.args)?)
                .context("encode control transport tick plan")
        }
        LocalServiceMethod::Other => {
            let command_json = serde_json::json!({ "type": request.method });
            let command: ClientCommand = serde_json::from_value(command_json)
                .with_context(|| format!("unsupported service method {}", request.method))?;
            dispatch_with_side_effects(&mut runtime, command)
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
    let platform = PlatformNetworkImpl::default().diagnostics().ok();
    let payload = serde_json::json!({
        "exportedAtMs": current_timestamp_ms(),
        "state": state,
        "session": diagnostic_session_summary(),
        "path": diagnose,
        "platform": platform,
        "relayStats": load_relay_runtime_stats(),
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
        session.relay_candidates = sorted_persisted_relay_candidates(
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
    let platform = PlatformNetworkImpl::default()
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
    let runtime_state = PlatformNetworkImpl::default()
        .read_runtime_state()
        .unwrap_or_default();
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
    let relay = stats.as_ref().map(|stats| PathDiagnoseRelay {
        address: stats.relay_address.clone(),
        active_path: stats.active_path.clone(),
        requested_relay_session_count: stats.requested_relay_session_count,
        relay_session_count: stats.relay_session_count,
        ticket_expires_at: stats.ticket_expires_at.clone(),
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
        updated_at_ms: stats.updated_at_ms,
        stale: current_timestamp_ms().saturating_sub(stats.updated_at_ms) > RELAY_STATS_STALE_MS,
    });
    let direct_candidates = diagnose_direct_candidates(&activation.peers);
    let relay_candidates = select_relay_candidates(&session.relay_candidates);
    let policy = load_recent_relay_data_plane_policy_for_path(
        &network_id,
        session.device_id.as_deref(),
        Some("relay_udp"),
    );
    let expected_mtu = policy
        .as_ref()
        .and_then(|policy| policy.relay_mtu)
        .or_else(|| stats.as_ref().and_then(|stats| stats.relay_mtu));
    let expected_payload = policy
        .as_ref()
        .and_then(|policy| policy.max_frame_payload)
        .or_else(|| stats.as_ref().and_then(|stats| stats.max_frame_payload));
    let mtu = PathDiagnoseMtu {
        relay_mtu: expected_mtu,
        max_frame_payload: expected_payload,
        policy_scope: policy.as_ref().and_then(|policy| policy.scope.clone()),
        policy_path_type: policy.as_ref().and_then(|policy| policy.path_type.clone()),
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
    Ok(PathDiagnoseResponse {
        network_id: Some(network_id),
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

fn android_network_config_from_latest_control<P>(
    runtime: &mut ClientRuntime<P>,
) -> Result<AndroidVpnSessionConfig>
where
    P: client_core::PlatformNetwork,
{
    let mut session = load_network_session()?;
    let _ = runtime.dispatch(ClientCommand::ApplyAuthCallback(session.clone().into()));
    let Some(device_id) = session.device_id.clone().filter(|value| !value.is_empty()) else {
        return Err(anyhow::anyhow!(
            "device unavailable: current device is not registered"
        ));
    };
    let client = ControlPlaneClient::from_env();
    let network_id = ensure_active_network_id(&mut session)?;
    let _ = refresh_relay_candidates_for_session(&mut session, &network_id);
    let activation = client.activate_network(&session.access_token, &device_id, &network_id)?;
    session.self_node_id = activation.self_node_id.clone();
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
    let best_relay = best_relay_candidate(&session.relay_candidates);
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
    Ok(AndroidVpnSessionConfig {
        session_name: "SLAN".to_string(),
        virtual_ip: activation.virtual_ip,
        prefix_len: activation.prefix_len,
        dns_servers: activation.dns_servers,
        routes,
        mtu: Some(1280),
        relay_endpoint_id: best_relay.as_ref().map(|relay| relay.endpoint_id.clone()),
        relay_transport: best_relay.as_ref().map(|relay| relay.transport.clone()),
        relay_address: best_relay.as_ref().map(|relay| relay.address.clone()),
        relay_data_plane: build_relay_data_plane_config(
            &client,
            &session,
            &network_id,
            activation.self_node_id.as_deref(),
            &activation.peers,
            best_relay.as_ref(),
        )
        .ok(),
    })
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
    let _ = refresh_relay_candidates_for_session(&mut session, &network_id);
    let activation = client.activate_network(&session.access_token, &device_id, &network_id)?;
    session.self_node_id = activation.self_node_id.clone();
    if !activation.relay_candidates.is_empty() {
        session.relay_candidates = sorted_persisted_relay_candidates(
            activation
                .relay_candidates
                .iter()
                .map(persisted_relay_candidate)
                .collect(),
        );
    }
    session.virtual_ip = Some(activation.virtual_ip);
    persist_session(&session)?;
    let best_relay = best_udp_relay_candidate(&session.relay_candidates)
        .or_else(|| best_relay_candidate(&session.relay_candidates));
    build_relay_data_plane_config(
        &client,
        &session,
        &network_id,
        activation.self_node_id.as_deref(),
        &activation.peers,
        best_relay.as_ref(),
    )
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
    session.self_node_id = activation.self_node_id.clone();
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
    let best_relay = best_udp_relay_candidate(&session.relay_candidates)
        .or_else(|| best_relay_candidate(&session.relay_candidates));
    let relay_config = build_relay_data_plane_config(
        &client,
        session,
        &network_id,
        activation.self_node_id.as_deref(),
        &activation.peers,
        best_relay.as_ref(),
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

fn build_relay_data_plane_config(
    client: &ControlPlaneClient,
    session: &PersistedSession,
    network_id: &str,
    self_node_id: Option<&str>,
    peers: &[ControlPeer],
    best_relay: Option<&RelayCandidateSelection>,
) -> Result<RelayDataPlaneConfig> {
    let relay = best_relay
        .filter(|relay| relay.transport.eq_ignore_ascii_case("udp"))
        .ok_or_else(|| anyhow::anyhow!("no reachable udp relay candidate"))?;
    let local_node_id = self_node_id
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .ok_or_else(|| anyhow::anyhow!("network map missing self node id"))?;

    let sessions = peers
        .iter()
        .filter(|peer| peer.relay_allowed)
        .filter(|peer| peer.node_id != local_node_id)
        .filter_map(|peer| {
            match client.issue_relay_ticket(
                &session.access_token,
                network_id,
                local_node_id,
                peer.node_id.as_str(),
                relay.cluster_id.as_deref(),
                Some(relay.endpoint_id.as_str()),
                relay.region_id.as_deref(),
            ) {
                Ok(ticket) => Some(RelayPeerSession {
                    session_id: ticket.session_id.clone(),
                    peer_node_id: peer.node_id.clone(),
                    peer_virtual_ips: peer.virtual_ips.clone(),
                    ticket,
                }),
                Err(error) => {
                    log_service_error(format!(
                        "client-core-service issue relay ticket skipped: peerNodeId={} error={error:#}",
                        peer.node_id
                    ));
                    None
                }
            }
        })
        .collect::<Vec<_>>();
    let policy = relay_payload_policy(
        relay.address.as_str(),
        network_id,
        session.device_id.as_deref(),
        Some("relay_udp"),
    );
    let path_policy =
        relay_path_policy(network_id, session.device_id.as_deref(), Some("relay_udp"));

    Ok(RelayDataPlaneConfig {
        enabled: !sessions.is_empty(),
        transport: relay.transport.clone(),
        relay_address: relay.address.clone(),
        local_node_id: local_node_id.to_string(),
        network_id: network_id.to_string(),
        path_policy,
        peer_paths: peer_path_configs(
            peers,
            local_node_id,
            relay,
            &select_relay_candidates(&session.relay_candidates),
            &sessions,
        ),
        relay_mtu: Some(policy.relay_mtu),
        max_frame_payload: Some(policy.max_frame_payload),
        sessions,
    })
}

fn peer_path_configs(
    peers: &[ControlPeer],
    local_node_id: &str,
    relay: &RelayCandidateSelection,
    relay_candidates: &[RelayCandidateSelection],
    relay_sessions: &[RelayPeerSession],
) -> Vec<PeerPathConfig> {
    peers
        .iter()
        .filter(|peer| peer.node_id != local_node_id)
        .map(|peer| {
            let relay_session = relay_sessions
                .iter()
                .find(|session| session.peer_node_id == peer.node_id);
            let mut candidates = Vec::new();
            candidates.extend(peer.endpoints.iter().filter_map(|endpoint| {
                let address = endpoint.address.trim();
                if address.is_empty() {
                    return None;
                }
                Some(PathCandidate {
                    kind: PathKind::DirectUdp,
                    state: PathState::Probing,
                    endpoint_id: None,
                    address: Some(address.to_string()),
                    session_id: None,
                    transport: Some("udp".to_string()),
                    rtt_ms: None,
                    path_score: None,
                    last_ok_at_ms: None,
                    last_error: None,
                })
            }));
            if let Some(session) = relay_session {
                let relay_transport = normalize_relay_transport(&relay.transport).unwrap_or("udp");
                candidates.push(PathCandidate {
                    kind: PathKind::RelayUdp,
                    state: PathState::Standby,
                    endpoint_id: Some(relay.endpoint_id.clone()),
                    address: Some(relay.address.clone()),
                    session_id: Some(session.session_id.clone()),
                    transport: Some(relay_transport.to_string()),
                    rtt_ms: relay.rtt_ms,
                    path_score: Some(relay.path_score),
                    last_ok_at_ms: None,
                    last_error: None,
                });
            }
            candidates.extend(relay_candidates.iter().filter_map(|candidate| {
                let transport = normalize_relay_transport(&candidate.transport)?;
                let path_kind = relay_path_kind_for_transport(transport)?;
                Some(PathCandidate {
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
                })
            }));
            PeerPathConfig {
                peer_node_id: peer.node_id.clone(),
                peer_virtual_ips: peer.virtual_ips.clone(),
                candidates,
            }
        })
        .collect()
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
            "relayCandidateCount": session.relay_candidates.len(),
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
        LocalServiceMethod::ActivateNetwork => state
            .as_ref()
            .and_then(|state| state.error.as_ref())
            .map(|_| BUSINESS_NETWORK_SWITCH_FAILED)
            .unwrap_or(BUSINESS_NETWORK_SWITCH_FINISHED),
        LocalServiceMethod::DeactivateNetwork | LocalServiceMethod::ShutdownNetwork => state
            .as_ref()
            .and_then(|state| state.error.as_ref())
            .map(|_| BUSINESS_NETWORK_SWITCH_FAILED)
            .unwrap_or(BUSINESS_NETWORK_SWITCH_FINISHED),
        LocalServiceMethod::Logout | LocalServiceMethod::Dispatch => BUSINESS_SESSION_CHANGED,
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

#[derive(Default)]
struct RelayMaintenanceState {
    last_reconfigure_ms: u64,
    last_failure_total: u64,
    last_attach_failures: u64,
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
    if session.relay_candidates.is_empty() {
        return Ok(());
    }
    let stats = load_relay_runtime_stats();
    let reconfigure_reason = relay_maintenance_reconfigure_reason(now, stats.as_ref(), maintenance);
    let Some(reason) = reconfigure_reason else {
        return Ok(());
    };
    if maintenance.last_reconfigure_ms > 0
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
    } else {
        maintenance.last_failure_total = 0;
        maintenance.last_attach_failures = 0;
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
) -> Option<&'static str> {
    if maintenance.last_reconfigure_ms == 0 {
        if let Some(stats) = stats {
            maintenance.last_failure_total = relay_runtime_failure_total(stats);
            maintenance.last_attach_failures = stats.relay_attach_failures;
            if relay_ticket_should_renew(now, stats.ticket_expires_at.as_deref()) {
                return Some("ticket_expiring");
            }
        }
        maintenance.last_reconfigure_ms = now;
        return None;
    }
    let Some(stats) = stats else {
        return Some("missing_relay_stats");
    };
    if relay_ticket_should_renew(now, stats.ticket_expires_at.as_deref()) {
        return Some("ticket_expiring");
    }
    if now.saturating_sub(maintenance.last_reconfigure_ms) >= RELAY_TICKET_RENEW_INTERVAL_MS {
        return Some("ticket_renew");
    }
    if now.saturating_sub(stats.updated_at_ms) > RELAY_STATS_STALE_MS {
        return Some("stale_relay_stats");
    }
    if relay_sessions_missing(stats) {
        return Some("relay_session_missing");
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

fn relay_sessions_missing(stats: &RelayRuntimeStats) -> bool {
    stats.requested_relay_session_count > 0
        && stats.relay_session_count < stats.requested_relay_session_count
}

fn relay_ticket_should_renew(now_ms: u64, ticket_expires_at: Option<&str>) -> bool {
    let Some(expires_at_ms) = ticket_expires_at.and_then(parse_rfc3339_utc_ms) else {
        return false;
    };
    now_ms.saturating_add(RELAY_TICKET_RENEW_WINDOW_MS) >= expires_at_ms
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
