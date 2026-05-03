use std::{
    fs,
    path::PathBuf,
    sync::{Arc, Mutex},
    thread,
    time::Duration,
};

use client_core::{AuthPayload, ClientCommand, ClientRuntime};
use client_core_platform::PlatformNetworkImpl;
use control_mqtt_client::{ThinControlMqttClient, ThinMqttCredential, ThinMqttQoS};
use serde::Deserialize;

use crate::{
    control_tasks::ControlTaskQueue,
    control_transport::{
        self, ControlTransportMessage, ControlTransportMessageKind, ControlTransportTickRequest,
        MqttQos,
    },
    current_timestamp_ms, load_session, log_service_error, publish_state_business_event,
    PersistedSession, StateChangeNotifier, BUSINESS_CONTROL_SYNC_CHANGED,
    BUSINESS_NETWORK_RUNTIME_CHANGED, BUSINESS_NETWORK_SWITCH_FAILED, BUSINESS_SESSION_CHANGED,
};

#[derive(Debug, Default)]
pub struct ControlTransportWorkerState {
    running: bool,
    reconnect_key: Option<String>,
    backoff_ms: u64,
    next_attempt_ms: u64,
}

pub fn spawn_control_transport_supervisor(
    runtime: Arc<Mutex<ClientRuntime<PlatformNetworkImpl>>>,
    task_queue: Arc<Mutex<ControlTaskQueue>>,
    worker_state: Arc<Mutex<ControlTransportWorkerState>>,
    state_notifier: Arc<StateChangeNotifier>,
) {
    thread::spawn(move || loop {
        thread::sleep(Duration::from_secs(5));
        let Ok(session) = load_session() else {
            reset_worker_gate(&worker_state);
            continue;
        };
        let status = control_transport::control_transport_status(&session);
        if !status.ready {
            reset_worker_gate(&worker_state);
            continue;
        }
        let reconnect_key = reconnect_key(&session);
        if !claim_worker(&worker_state, reconnect_key.clone(), current_timestamp_ms()) {
            continue;
        }
        let runtime = Arc::clone(&runtime);
        let task_queue = Arc::clone(&task_queue);
        let worker_state = Arc::clone(&worker_state);
        let state_notifier = Arc::clone(&state_notifier);
        thread::spawn(move || {
            let result = run_control_transport_worker(session, runtime, task_queue, state_notifier);
            release_worker(&worker_state, reconnect_key, result.err());
        });
    });
}

fn run_control_transport_worker(
    session: PersistedSession,
    runtime: Arc<Mutex<ClientRuntime<PlatformNetworkImpl>>>,
    task_queue: Arc<Mutex<ControlTaskQueue>>,
    state_notifier: Arc<StateChangeNotifier>,
) -> Result<(), String> {
    let plan = control_transport::control_transport_plan(&session);
    let Some(downstream_topic) = plan.downstream_control_topic.clone() else {
        return Err("control transport downstream topic is missing".to_string());
    };
    let Some(mqtt) = session.mqtt.clone() else {
        return Err("control transport mqtt credential is missing".to_string());
    };
    let mut client = ThinControlMqttClient::connect(
        &ThinMqttCredential {
            broker_url: mqtt.broker_url,
            client_id: mqtt.client_id,
            username: mqtt.username,
            password: mqtt.password,
        },
        &downstream_topic,
    )?;

    let mut last_ack_flush_ms = None;
    let mut last_heartbeat_ms = None;
    let mut last_runtime_state_ms = None;
    let mut last_path_health_ms = None;
    loop {
        if reconnect_key(&load_session().map_err(|err| err.to_string())?) != reconnect_key(&session)
        {
            return Err("control transport session changed".to_string());
        }
        if let Some(publish) = client.read_publish(Duration::from_millis(500))? {
            ingest_downstream_publish(&publish.payload, &runtime, &task_queue, &state_notifier)?;
            client.ack_publish(&publish)?;
        }

        let now_ms = current_timestamp_ms();
        let tick = control_transport::control_transport_tick_plan(
            ControlTransportTickRequest {
                now_ms: Some(now_ms),
                last_ack_flush_ms,
                last_heartbeat_ms,
                last_runtime_state_ms,
                last_path_health_ms,
            },
            now_ms,
        );
        if !tick.outbox.include_control_acks
            && !tick.outbox.include_heartbeat
            && !tick.outbox.include_runtime_state
            && !tick.outbox.include_path_health
        {
            continue;
        }
        let messages = build_outbox_messages(
            &session,
            &runtime,
            &task_queue,
            tick.outbox.include_heartbeat,
            tick.outbox.include_runtime_state,
            tick.outbox.include_path_health,
            tick.outbox.include_control_acks,
        );
        for message in messages {
            publish_outbox_message(&mut client, &message)?;
            mark_transport_published(&message, &task_queue)?;
            match message.kind {
                ControlTransportMessageKind::ControlAck => {
                    last_ack_flush_ms = Some(now_ms);
                }
                ControlTransportMessageKind::Heartbeat => {
                    last_heartbeat_ms = Some(now_ms);
                }
                ControlTransportMessageKind::RuntimeState => {
                    last_runtime_state_ms = Some(now_ms);
                }
                ControlTransportMessageKind::PathHealth => {
                    last_path_health_ms = Some(now_ms);
                }
                ControlTransportMessageKind::EndpointReport => {
                    last_runtime_state_ms = Some(now_ms);
                }
                ControlTransportMessageKind::RelayPolicyReport => {
                    last_path_health_ms = Some(now_ms);
                }
            }
        }
    }
}

fn ingest_downstream_publish(
    payload: &[u8],
    runtime: &Arc<Mutex<ClientRuntime<PlatformNetworkImpl>>>,
    task_queue: &Arc<Mutex<ControlTaskQueue>>,
    state_notifier: &Arc<StateChangeNotifier>,
) -> Result<(), String> {
    if try_ingest_auth_callback(payload, runtime, state_notifier)? {
        return Ok(());
    }
    if try_ingest_network_map_response(payload, runtime, state_notifier)? {
        return Ok(());
    }
    if try_ingest_relay_data_plane_policy(payload)? {
        return Ok(());
    }
    let message = serde_json::from_slice(payload)
        .map_err(|err| format!("decode downstream control message: {err}"))?;
    let self_device_id = load_session()
        .ok()
        .and_then(|session| session.device_id)
        .filter(|value| !value.trim().is_empty());
    let accepted = match control_transport::normalize_downstream_control_value(
        message,
        self_device_id.as_deref(),
    )
    .map_err(|err| err.to_string())?
    {
        Some(accepted) => accepted,
        None => {
            log_service_error("client-core-service ignored downstream control message");
            return Ok(());
        }
    };
    log_service_error(format!(
        "client-core-service accepted downstream control action={} deliveryId={} requireUiRefresh={}",
        accepted.action, accepted.delivery_id, accepted.require_ui_refresh
    ));
    let mut queue = task_queue
        .lock()
        .map_err(|_| "control task queue mutex poisoned".to_string())?;
    let action = crate::control_tasks::ControlTaskAction::from_str(&accepted.action)
        .ok_or_else(|| format!("unsupported control task action: {}", accepted.action))?;
    let task = queue
        .enqueue_downstream(action, accepted.delivery_id, accepted.require_ui_refresh)
        .map_err(|err| err.to_string())?;
    if task.require_ui_refresh {
        drop(queue);
        let state = crate::drain_pending_control_tasks(runtime, task_queue);
        let business_type = if state.error.is_some() {
            BUSINESS_NETWORK_SWITCH_FAILED
        } else {
            BUSINESS_NETWORK_RUNTIME_CHANGED
        };
        publish_state_business_event(state_notifier, business_type, &state);
    } else {
        publish_state_business_event(state_notifier, BUSINESS_CONTROL_SYNC_CHANGED, &{
            let runtime = runtime
                .lock()
                .map_err(|_| "client runtime mutex poisoned".to_string())?;
            runtime.state().clone()
        });
    }
    Ok(())
}

fn try_ingest_relay_data_plane_policy(payload: &[u8]) -> Result<bool, String> {
    let value: serde_json::Value =
        serde_json::from_slice(payload).map_err(|err| format!("decode downstream json: {err}"))?;
    if value.get("type").and_then(serde_json::Value::as_str) != Some("relay_data_plane_policy") {
        return Ok(false);
    }
    let policy_value = value
        .get("payload")
        .cloned()
        .ok_or_else(|| "relay_data_plane_policy payload is missing".to_string())?;
    let mut incoming: RelayDataPlanePolicy = serde_json::from_value(policy_value)
        .map_err(|err| format!("decode relay policy: {err}"))?;
    validate_relay_data_plane_policy(&incoming)?;
    if !relay_policy_targets_current_device(&incoming) {
        log_service_error(format!(
            "client-core-service ignored relay data plane policy for another device policyId={}",
            incoming.policy_id.as_deref().unwrap_or_default()
        ));
        return Ok(true);
    }
    let now_ms = current_timestamp_ms();
    incoming.updated_at_ms.get_or_insert(now_ms);
    let current = load_relay_data_plane_policy();
    if should_debounce_relay_policy(current.as_ref(), &incoming, now_ms) {
        log_service_error(format!(
            "client-core-service debounced relay data plane policy policyId={} level={:?}",
            incoming.policy_id.as_deref().unwrap_or_default(),
            incoming.recommendation_level
        ));
        return Ok(true);
    }
    let path = relay_policy_file_path();
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent).map_err(|err| format!("create relay policy dir: {err}"))?;
    }
    let payload = serde_json::to_vec_pretty(&incoming)
        .map_err(|err| format!("encode relay data plane policy: {err}"))?;
    fs::write(&path, payload).map_err(|err| format!("write relay policy: {err}"))?;
    log_service_error(format!(
        "client-core-service accepted relay data plane policy path={}",
        path.display()
    ));
    Ok(true)
}

#[derive(Debug, Clone, Deserialize, serde::Serialize)]
#[serde(rename_all = "camelCase")]
struct RelayDataPlanePolicy {
    #[serde(default)]
    policy_id: Option<String>,
    #[serde(default)]
    version: Option<u32>,
    #[serde(default)]
    scope: Option<String>,
    #[serde(default)]
    network_id: Option<String>,
    #[serde(default)]
    target_device_ids: Vec<String>,
    #[serde(default)]
    path_type: Option<String>,
    #[serde(default)]
    preferred_path_types: Vec<String>,
    #[serde(default)]
    recommendation_level: Option<u8>,
    #[serde(default)]
    execution_level: Option<u8>,
    relay_mtu: u16,
    max_frame_payload: u16,
    #[serde(default)]
    reason: Option<String>,
    #[serde(default)]
    ttl_ms: Option<u64>,
    #[serde(default)]
    effective_ms: Option<u64>,
    #[serde(default)]
    updated_at_ms: Option<u64>,
}

fn validate_relay_data_plane_policy(policy: &RelayDataPlanePolicy) -> Result<(), String> {
    if policy.version.unwrap_or(1) != 1 {
        return Err("unsupported relay data plane policy version".to_string());
    }
    if let Some(scope) = policy.scope.as_deref() {
        match scope {
            "global" | "region" | "network" | "device" | "device_override" => {}
            _ => return Err("unsupported relay data plane policy scope".to_string()),
        }
    }
    if let Some(path_type) = policy.path_type.as_deref() {
        match path_type.trim() {
            "" | "direct" | "p2p" | "relay" | "relay_udp" | "relay_tcp" | "relay_tls"
            | "relay_http3" => {}
            _ => return Err("unsupported relay data plane policy pathType".to_string()),
        }
    }
    for path_type in &policy.preferred_path_types {
        match path_type.trim() {
            "direct_udp" | "relay_udp" | "relay_tcp" | "relay_http3" | "relay_tls" => {}
            _ => return Err("unsupported relay data plane policy preferredPathTypes".to_string()),
        }
    }
    if !(576..=1500).contains(&policy.relay_mtu) {
        return Err("relayMtu out of range".to_string());
    }
    if !(512..=1400).contains(&policy.max_frame_payload) {
        return Err("maxFramePayload out of range".to_string());
    }
    if policy.max_frame_payload >= policy.relay_mtu {
        return Err("maxFramePayload must be lower than relayMtu".to_string());
    }
    if let Some(level) = policy.recommendation_level {
        if level > 11 {
            return Err("recommendationLevel must be 0..11".to_string());
        }
    }
    if let Some(level) = policy.execution_level {
        if level > 7 {
            return Err("executionLevel must be 0..7".to_string());
        }
    }
    Ok(())
}

fn relay_policy_targets_current_device(policy: &RelayDataPlanePolicy) -> bool {
    let targets = policy
        .target_device_ids
        .iter()
        .map(|value| value.trim())
        .filter(|value| !value.is_empty())
        .collect::<Vec<_>>();
    if targets.is_empty() {
        return true;
    }
    let Some(device_id) = load_session()
        .ok()
        .and_then(|session| session.device_id)
        .map(|value| value.trim().to_string())
        .filter(|value| !value.is_empty())
    else {
        return false;
    };
    targets.iter().any(|target| *target == device_id)
}

fn load_relay_data_plane_policy() -> Option<RelayDataPlanePolicy> {
    let payload = fs::read(relay_policy_file_path()).ok()?;
    serde_json::from_slice(&payload).ok()
}

fn should_debounce_relay_policy(
    current: Option<&RelayDataPlanePolicy>,
    incoming: &RelayDataPlanePolicy,
    now_ms: u64,
) -> bool {
    let Some(current) = current else {
        return false;
    };
    if current.policy_id.is_some() && current.policy_id == incoming.policy_id {
        return true;
    }
    let last = current.updated_at_ms.unwrap_or_default();
    if now_ms.saturating_sub(last) < 5 * 60 * 1000 {
        let current_level = current.execution_level.unwrap_or(0);
        let incoming_level = incoming.execution_level.unwrap_or(0);
        return current_level.abs_diff(incoming_level) < 2;
    }
    false
}

fn relay_policy_file_path() -> PathBuf {
    if let Some(dir) = std::env::var_os("ProgramData") {
        return PathBuf::from(dir)
            .join("SLAN")
            .join("client-v2-relay-policy.json");
    }
    if let Some(dir) = std::env::var_os("SLAN_STATE_DIR") {
        return PathBuf::from(dir).join("client-v2-relay-policy.json");
    }
    PathBuf::from("client-v2-relay-policy.json")
}

fn try_ingest_network_map_response(
    payload: &[u8],
    runtime: &Arc<Mutex<ClientRuntime<PlatformNetworkImpl>>>,
    state_notifier: &Arc<StateChangeNotifier>,
) -> Result<bool, String> {
    let value: serde_json::Value =
        serde_json::from_slice(payload).map_err(|err| format!("decode downstream json: {err}"))?;
    if value.get("type").and_then(serde_json::Value::as_str) != Some("network_map_response") {
        return Ok(false);
    }
    let Some(map) = value.pointer("/payload/map") else {
        return Err("network_map_response payload.map is missing".to_string());
    };
    let count = crate::persist_relay_candidates_from_network_map(map)
        .map_err(|err| format!("persist relay candidates from network map: {err:#}"))?;
    log_service_error(format!(
        "client-core-service refreshed relay candidates from network map: count={count}"
    ));
    let state = {
        let runtime = runtime
            .lock()
            .map_err(|_| "client runtime mutex poisoned".to_string())?;
        runtime.state().clone()
    };
    publish_state_business_event(state_notifier, BUSINESS_CONTROL_SYNC_CHANGED, &state);
    Ok(true)
}

fn try_ingest_auth_callback(
    payload: &[u8],
    runtime: &Arc<Mutex<ClientRuntime<PlatformNetworkImpl>>>,
    state_notifier: &Arc<StateChangeNotifier>,
) -> Result<bool, String> {
    let value: serde_json::Value =
        serde_json::from_slice(payload).map_err(|err| format!("decode downstream json: {err}"))?;
    if value.get("type").and_then(serde_json::Value::as_str) != Some("auth_callback") {
        return Ok(false);
    }
    let Some(auth_value) = value.get("payload").cloned() else {
        return Err("auth_callback payload is missing".to_string());
    };
    let auth: AuthPayload = serde_json::from_value(auth_value)
        .map_err(|err| format!("decode auth_callback payload: {err}"))?;
    let mut runtime = runtime
        .lock()
        .map_err(|_| "client runtime mutex poisoned".to_string())?;
    let state =
        crate::dispatch_with_side_effects(&mut runtime, ClientCommand::ApplyAuthCallback(auth));
    if let Some(error) = state.error {
        return Err(error);
    }
    publish_state_business_event(state_notifier, BUSINESS_SESSION_CHANGED, &state);
    Ok(true)
}

fn build_outbox_messages(
    session: &PersistedSession,
    runtime: &Arc<Mutex<ClientRuntime<PlatformNetworkImpl>>>,
    task_queue: &Arc<Mutex<ControlTaskQueue>>,
    include_heartbeat: bool,
    include_runtime_state: bool,
    include_path_health: bool,
    include_control_acks: bool,
) -> Vec<ControlTransportMessage> {
    let state = {
        let runtime = runtime.lock().expect("client runtime mutex poisoned");
        runtime.state().clone()
    };
    let acks = if include_control_acks {
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
    control_transport::control_transport_outbox(
        session,
        &state,
        acks,
        current_timestamp_ms(),
        include_heartbeat,
        include_runtime_state,
        include_path_health,
    )
    .messages
}

fn publish_outbox_message(
    client: &mut ThinControlMqttClient,
    message: &ControlTransportMessage,
) -> Result<(), String> {
    let payload = serde_json::to_vec(&message.payload)
        .map_err(|err| format!("encode outbox payload {}: {err}", message.id))?;
    client.publish(&message.topic, &payload, thin_qos(message.qos))
}

fn mark_transport_published(
    message: &ControlTransportMessage,
    task_queue: &Arc<Mutex<ControlTaskQueue>>,
) -> Result<(), String> {
    if let Some(task_id) = message.ack_task_id.as_deref() {
        let mut queue = task_queue
            .lock()
            .map_err(|_| "control task queue mutex poisoned".to_string())?;
        queue
            .mark_acknowledged(task_id)
            .map_err(|err| err.to_string())?;
    }
    Ok(())
}

fn thin_qos(qos: MqttQos) -> ThinMqttQoS {
    match qos {
        MqttQos::QoS0 => ThinMqttQoS::AtMostOnce,
        MqttQos::QoS2 => ThinMqttQoS::ExactlyOnce,
    }
}

fn reconnect_key(session: &PersistedSession) -> String {
    let mqtt = session.mqtt.as_ref();
    format!(
        "{}|{}|{}|{}",
        session.device_id.as_deref().unwrap_or_default(),
        session.active_network_id.as_deref().unwrap_or_default(),
        mqtt.map(|value| value.client_id.as_str())
            .unwrap_or_default(),
        mqtt.map(|value| value.topic_prefix.as_str())
            .unwrap_or_default(),
    )
}

fn claim_worker(
    worker_state: &Arc<Mutex<ControlTransportWorkerState>>,
    reconnect_key: String,
    now_ms: u64,
) -> bool {
    let mut state = worker_state
        .lock()
        .expect("control transport worker state poisoned");
    if now_ms < state.next_attempt_ms {
        return false;
    }
    if state.running && state.reconnect_key.as_deref() == Some(reconnect_key.as_str()) {
        return false;
    }
    state.running = true;
    state.reconnect_key = Some(reconnect_key);
    true
}

fn release_worker(
    worker_state: &Arc<Mutex<ControlTransportWorkerState>>,
    reconnect_key: String,
    error: Option<String>,
) {
    let mut state = worker_state
        .lock()
        .expect("control transport worker state poisoned");
    if state.reconnect_key.as_deref() != Some(reconnect_key.as_str()) {
        return;
    }
    state.running = false;
    state.backoff_ms = if error.is_some() {
        (state.backoff_ms.max(1_000) * 2).min(30_000)
    } else {
        1_000
    };
    state.next_attempt_ms = current_timestamp_ms().saturating_add(state.backoff_ms);
}

fn reset_worker_gate(worker_state: &Arc<Mutex<ControlTransportWorkerState>>) {
    let mut state = worker_state
        .lock()
        .expect("control transport worker state poisoned");
    if state.running {
        return;
    }
    state.reconnect_key = None;
    state.backoff_ms = 1_000;
    state.next_attempt_ms = 0;
}
