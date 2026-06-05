use std::{
    collections::HashMap,
    sync::OnceLock,
    sync::{Arc, Mutex},
    thread,
    time::Duration,
};

use client_core::{
    AssignedIpPayload, AuthPayload, ClientCommand, ClientMessageNoticePayload, ClientRuntime,
};
use client_core_platform::PlatformNetworkImpl;
use control_mqtt_client::{ThinControlMqttClient, ThinMqttCredential, ThinMqttQoS};
use serde::Deserialize;

use crate::{
    client_message_mqtt,
    control_tasks::ControlTaskQueue,
    control_transport::{
        self, ControlTransportMessage, ControlTransportMessageKind, ControlTransportTickRequest,
        MqttQos,
    },
    current_timestamp_ms, load_session, log_service_error, publish_state_business_event,
    PersistedSession, StateChangeNotifier, BUSINESS_CONTROL_SYNC_CHANGED,
    BUSINESS_NETWORK_RUNTIME_CHANGED, BUSINESS_NETWORK_SWITCH_FAILED, BUSINESS_SESSION_CHANGED,
};

const DEVICE_NETWORK_ENABLED_EVENT: &str = "device_network_enabled";
const DEVICE_NETWORK_DISABLED_EVENT: &str = "device_network_disabled";
const MQTT_KEEPALIVE_PING_INTERVAL_MS: u64 = 15_000;
const MQTT_RECONNECT_AFTER_SESSION_REFRESH_MS: u64 = 10 * 60 * 1000;
const REMOTE_NETWORK_CONFIG_REBUILD_COOLDOWN_MS: u64 = 120_000;

static REMOTE_NETWORK_CONFIG_REBUILDS: OnceLock<Mutex<HashMap<String, u64>>> = OnceLock::new();

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
        wake_control_transport_worker(&runtime, &task_queue, &worker_state, &state_notifier);
    });
}

pub fn wake_control_transport_worker(
    runtime: &Arc<Mutex<ClientRuntime<PlatformNetworkImpl>>>,
    task_queue: &Arc<Mutex<ControlTaskQueue>>,
    worker_state: &Arc<Mutex<ControlTransportWorkerState>>,
    state_notifier: &Arc<StateChangeNotifier>,
) {
    let Ok(session) = load_session() else {
        reset_worker_gate(worker_state);
        return;
    };
    let status = control_transport::control_transport_status(&session);
    if !status.ready {
        log_service_error(format!(
            "client-core-service control mqtt not ready missing={:?} deviceId={}",
            status.missing,
            status.device_id.as_deref().unwrap_or_default()
        ));
        reset_worker_gate(worker_state);
        return;
    }
    let reconnect_key = reconnect_key(&session);
    if !claim_worker(worker_state, reconnect_key.clone(), current_timestamp_ms()) {
        return;
    }
    let runtime = Arc::clone(runtime);
    let task_queue = Arc::clone(task_queue);
    let worker_state = Arc::clone(worker_state);
    let state_notifier = Arc::clone(state_notifier);
    thread::spawn(move || {
        let result = run_control_transport_worker(session, runtime, task_queue, state_notifier);
        let error = result.err();
        release_worker(&worker_state, reconnect_key, error);
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
    let credential = ThinMqttCredential {
        broker_url: mqtt.broker_url,
        client_id: mqtt.client_id,
        username: mqtt.username,
        password: mqtt.password,
    };
    let mut client = connect_control_mqtt_with_retry(&credential, &downstream_topic)?;
    if let Some(topic) = network_broadcast_topic(&session) {
        match client.subscribe(&topic) {
            Ok(()) => log_service_error(format!(
                "client-core-service subscribed network broadcast topic={topic}"
            )),
            Err(error) => log_service_error(format!(
                "client-core-service failed to subscribe network broadcast topic={topic}: {error}"
            )),
        }
    }
    log_service_error(format!(
        "client-core-service control mqtt connected topic={} deviceId={}",
        downstream_topic,
        session.device_id.as_deref().unwrap_or_default()
    ));
    sync_after_control_mqtt_connected(&runtime, &task_queue, &state_notifier)?;

    let mut last_ack_flush_ms = None;
    let mut last_heartbeat_ms = None;
    let mut last_runtime_state_ms = None;
    let mut last_path_health_ms = None;
    let mut last_keepalive_ping_ms = Some(current_timestamp_ms());
    let connected_at_ms = current_timestamp_ms();
    loop {
        if reconnect_key(&load_session().map_err(|err| err.to_string())?) != reconnect_key(&session)
        {
            return Err("control transport session changed".to_string());
        }
        if let Some(publish) = client.read_publish(Duration::from_millis(500))? {
            log_service_error(format!(
                "client-core-service received control mqtt down topic={} qos={} packetId={} bytes={}",
                publish.topic,
                publish.qos,
                publish
                    .packet_id
                    .map(|value| value.to_string())
                    .unwrap_or_default(),
                publish.payload.len()
            ));
            ingest_downstream_publish(&publish.payload, &runtime, &task_queue, &state_notifier)?;
            client.ack_publish(&publish)?;
            log_service_error(format!(
                "client-core-service acked control mqtt down topic={} qos={} packetId={}",
                publish.topic,
                publish.qos,
                publish
                    .packet_id
                    .map(|value| value.to_string())
                    .unwrap_or_default()
            ));
        }

        let now_ms = current_timestamp_ms();
        if now_ms.saturating_sub(connected_at_ms) >= MQTT_RECONNECT_AFTER_SESSION_REFRESH_MS {
            return Err(
                "control mqtt proactive reconnect after session refresh window".to_string(),
            );
        }
        if now_ms.saturating_sub(last_keepalive_ping_ms.unwrap_or(0))
            >= MQTT_KEEPALIVE_PING_INTERVAL_MS
        {
            client.ping()?;
            last_keepalive_ping_ms = Some(now_ms);
            log_service_error("client-core-service sent mqtt keepalive ping");
        }
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
            publish_outbox_message_async(session.clone(), message.clone(), Arc::clone(&task_queue));
            last_keepalive_ping_ms = Some(now_ms);
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
            }
        }
    }
}

fn connect_control_mqtt_with_retry(
    credential: &ThinMqttCredential,
    downstream_topic: &str,
) -> Result<ThinControlMqttClient, String> {
    let mut errors = Vec::new();
    for attempt in 1..=3 {
        let suffix = format!("v2-worker-{}-{attempt}", current_timestamp_ms());
        match ThinControlMqttClient::connect_with_subscription_suffix(
            credential,
            downstream_topic,
            &suffix,
        ) {
            Ok(client) => return Ok(client),
            Err(error) => {
                errors.push(format!("attempt {attempt}: {error}"));
                thread::sleep(Duration::from_millis(250 * attempt as u64));
            }
        }
    }
    Err(errors.join("; "))
}

fn sync_after_control_mqtt_connected(
    runtime: &Arc<Mutex<ClientRuntime<PlatformNetworkImpl>>>,
    _task_queue: &Arc<Mutex<ControlTaskQueue>>,
    state_notifier: &Arc<StateChangeNotifier>,
) -> Result<(), String> {
    crate::sync_control_assignment(runtime);
    let state = {
        let runtime = runtime
            .lock()
            .map_err(|_| "client runtime mutex poisoned".to_string())?;
        runtime.state().clone()
    };
    if !state.signed_in || !state.network_enabled {
        publish_state_business_event(state_notifier, BUSINESS_CONTROL_SYNC_CHANGED, &state);
        return Ok(());
    }
    log_service_error("client-core-service skipped reconnect config rebuild for active data plane");
    let business_type = BUSINESS_CONTROL_SYNC_CHANGED;
    publish_state_business_event(state_notifier, business_type, &state);
    Ok(())
}

fn network_broadcast_topic(session: &PersistedSession) -> Option<String> {
    let network_id = session.active_network_id.as_deref()?.trim();
    if network_id.is_empty() {
        return None;
    }
    let mqtt = session.mqtt.as_ref()?;
    let mut prefix = mqtt.topic_prefix.trim_end_matches('/').to_string();
    if let Some(device_id) = session
        .device_id
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
    {
        let suffix = format!("/devices/{device_id}");
        if prefix.ends_with(&suffix) {
            prefix.truncate(prefix.len() - suffix.len());
        }
    }
    Some(format!("{prefix}/networks/{network_id}/broadcast"))
}

fn ingest_downstream_publish(
    payload: &[u8],
    runtime: &Arc<Mutex<ClientRuntime<PlatformNetworkImpl>>>,
    task_queue: &Arc<Mutex<ControlTaskQueue>>,
    state_notifier: &Arc<StateChangeNotifier>,
) -> Result<(), String> {
    let message_type = serde_json::from_slice::<serde_json::Value>(payload)
        .ok()
        .and_then(|value| log_downstream_message_summary(&value))
        .unwrap_or_else(|| "unknown".to_string());
    log_service_error(format!(
        "client-core-service ingesting downstream control message type={} bytes={}",
        message_type,
        payload.len()
    ));
    if try_ingest_device_user_login_succeeded(payload, runtime, task_queue, state_notifier)? {
        log_service_error(
            "client-core-service consumed downstream control message as device_user_login_succeeded",
        );
        return Ok(());
    }
    if try_ingest_device_ip_reassigned(payload, runtime, state_notifier)? {
        log_service_error(
            "client-core-service consumed downstream control message as device_ip_reassigned",
        );
        return Ok(());
    }
    if try_ingest_network_map_response(payload, runtime, state_notifier)? {
        log_service_error(
            "client-core-service consumed downstream control message as network_map_response",
        );
        return Ok(());
    }
    if try_ingest_network_config_changed(payload, runtime, task_queue, state_notifier)? {
        log_service_error(
            "client-core-service consumed downstream control message as network_config_changed",
        );
        return Ok(());
    }
    if let Some(connect_plan) = try_ingest_connect_plan(payload)? {
        let current_state = {
            let runtime = runtime
                .lock()
                .map_err(|_| "client runtime mutex poisoned".to_string())?;
            runtime.state().clone()
        };
        if current_state.signed_in && current_state.network_enabled {
            if connect_plan.should_rebuild {
                log_service_error(format!(
                    "client-core-service scheduling network rebuild for connect_plan messageId={}",
                    connect_plan.ack_delivery_id.as_deref().unwrap_or_default()
                ));
                {
                    let mut queue = task_queue
                        .lock()
                        .map_err(|_| "control task queue mutex poisoned".to_string())?;
                    if let Some(delivery_id) = connect_plan.ack_delivery_id {
                        queue
                            .enqueue_downstream(
                                crate::control_tasks::ControlTaskAction::RefreshNetworkConfig,
                                delivery_id,
                                false,
                            )
                            .map_err(|err| err.to_string())?;
                    } else {
                        queue
                            .enqueue_downstream_unacked(
                                crate::control_tasks::ControlTaskAction::RefreshNetworkConfig,
                                false,
                            )
                            .map_err(|err| err.to_string())?;
                    }
                }
                let state = crate::drain_pending_control_tasks(runtime, task_queue);
                let business_type = if state.error.is_some() {
                    BUSINESS_NETWORK_SWITCH_FAILED
                } else {
                    BUSINESS_NETWORK_RUNTIME_CHANGED
                };
                publish_state_business_event(state_notifier, business_type, &state);
            } else {
                publish_state_business_event(
                    state_notifier,
                    BUSINESS_CONTROL_SYNC_CHANGED,
                    &current_state,
                );
            }
        } else {
            publish_state_business_event(
                state_notifier,
                BUSINESS_CONTROL_SYNC_CHANGED,
                &current_state,
            );
        }
        return Ok(());
    }
    if try_ingest_client_message(payload, runtime, state_notifier)? {
        log_service_error(
            "client-core-service consumed downstream control message as client_message",
        );
        return Ok(());
    }
    if try_ingest_device_network_presence(payload, runtime, task_queue, state_notifier)? {
        log_service_error(
            "client-core-service consumed downstream control message as device_network_presence",
        );
        return Ok(());
    }
    let message: serde_json::Value = serde_json::from_slice(payload)
        .map_err(|err| format!("decode downstream control message: {err}"))?;
    let message_type = message
        .get("type")
        .and_then(serde_json::Value::as_str)
        .unwrap_or_default()
        .to_string();
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
            log_service_error(format!(
                "client-core-service ignored downstream control message type={message_type}"
            ));
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

fn try_ingest_device_network_presence(
    payload: &[u8],
    runtime: &Arc<Mutex<ClientRuntime<PlatformNetworkImpl>>>,
    _task_queue: &Arc<Mutex<ControlTaskQueue>>,
    state_notifier: &Arc<StateChangeNotifier>,
) -> Result<bool, String> {
    let value: serde_json::Value =
        serde_json::from_slice(payload).map_err(|err| format!("decode downstream json: {err}"))?;
    let message_type = value
        .get("type")
        .and_then(serde_json::Value::as_str)
        .unwrap_or_default();
    if !is_device_network_presence_event(message_type) {
        return Ok(false);
    }
    let payload_value = value.get("payload").unwrap_or(&value);
    let device_id = payload_value
        .get("deviceId")
        .and_then(serde_json::Value::as_str)
        .unwrap_or_default();
    let network_id = payload_value
        .get("networkId")
        .and_then(serde_json::Value::as_str)
        .unwrap_or_default();
    let online = payload_value
        .get("online")
        .and_then(serde_json::Value::as_bool)
        .unwrap_or(message_type == DEVICE_NETWORK_ENABLED_EVENT);
    let online_count = payload_value
        .get("onlineDevices")
        .and_then(serde_json::Value::as_array)
        .map(Vec::len)
        .unwrap_or_default();
    log_service_error(format!(
        "client-core-service network presence message type={message_type} networkId={network_id} deviceId={device_id} online={online} onlineDevices={online_count}"
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

fn is_device_network_presence_event(message_type: &str) -> bool {
    matches!(
        message_type,
        DEVICE_NETWORK_ENABLED_EVENT | DEVICE_NETWORK_DISABLED_EVENT
    )
}

fn log_downstream_message_summary(value: &serde_json::Value) -> Option<String> {
    let message_type = value
        .get("type")
        .and_then(serde_json::Value::as_str)
        .unwrap_or_default();
    if message_type.is_empty() {
        return None;
    }
    let payload = value.get("payload");
    let message_id = value
        .get("messageId")
        .or_else(|| value.get("message_id"))
        .and_then(serde_json::Value::as_str)
        .unwrap_or_default();
    let device_id = payload
        .and_then(|payload| payload.get("deviceId"))
        .or_else(|| payload.and_then(|payload| payload.get("device_id")))
        .and_then(serde_json::Value::as_str)
        .unwrap_or_default();
    let virtual_ip = payload
        .and_then(|payload| payload.get("virtualIp"))
        .or_else(|| payload.and_then(|payload| payload.get("virtual_ip")))
        .and_then(serde_json::Value::as_str)
        .unwrap_or_default();
    let policy_id = payload
        .and_then(|payload| payload.get("policyId"))
        .or_else(|| payload.and_then(|payload| payload.get("policy_id")))
        .and_then(serde_json::Value::as_str)
        .unwrap_or_default();
    let network_id = payload
        .and_then(|payload| payload.get("networkId"))
        .or_else(|| payload.and_then(|payload| payload.get("network_id")))
        .and_then(serde_json::Value::as_str)
        .unwrap_or_default();
    log_service_error(format!(
        "client-core-service downstream envelope type={} messageId={} networkId={} deviceId={} virtualIp={} policyId={}",
        message_type, message_id, network_id, device_id, virtual_ip, policy_id
    ));
    Some(message_type.to_string())
}

fn downstream_message_id(value: &serde_json::Value) -> Option<String> {
    value
        .get("messageId")
        .or_else(|| value.get("message_id"))
        .and_then(serde_json::Value::as_str)
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .map(str::to_string)
}

fn try_ingest_client_message(
    payload: &[u8],
    runtime: &Arc<Mutex<ClientRuntime<PlatformNetworkImpl>>>,
    state_notifier: &Arc<StateChangeNotifier>,
) -> Result<bool, String> {
    let value: serde_json::Value =
        serde_json::from_slice(payload).map_err(|err| format!("decode downstream json: {err}"))?;
    if value.get("type").and_then(serde_json::Value::as_str) != Some("client_message") {
        return Ok(false);
    }
    let message_value = value
        .get("payload")
        .cloned()
        .ok_or_else(|| "client_message payload is missing".to_string())?;
    let message: ClientMessagePayload = serde_json::from_value(message_value)
        .map_err(|err| format!("decode client_message: {err}"))?;
    log_service_error(format!(
        "client-core-service accepted client_message messageId={} networkId={} fromDeviceId={} targetDeviceId={} bodyBytes={}",
        message.message_id.as_deref().unwrap_or_default(),
        message.network_id.as_deref().unwrap_or_default(),
        message.from_device_id.as_deref().unwrap_or_default(),
        message.target_device_id.as_deref().unwrap_or_default(),
        message.body.as_deref().unwrap_or_default().len()
    ));
    if maybe_reply_client_ping(&message) {
        return Ok(true);
    }
    let state = {
        let mut runtime = runtime.lock().expect("client runtime mutex poisoned");
        runtime
            .dispatch(ClientCommand::ApplyClientMessage(
                ClientMessageNoticePayload {
                    message_id: message.message_id,
                    from_device_id: message.from_device_id,
                    body: message.body,
                },
            ))
            .map_err(|err| format!("apply client_message: {err}"))?
    };
    publish_state_business_event(state_notifier, BUSINESS_CONTROL_SYNC_CHANGED, &state);
    Ok(true)
}

fn maybe_reply_client_ping(message: &ClientMessagePayload) -> bool {
    let Some(from_device_id) = message
        .from_device_id
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
    else {
        return false;
    };
    let Some((ping_id, sent_at_ms)) = message
        .body
        .as_deref()
        .and_then(client_message_mqtt::parse_client_ping_body)
    else {
        return false;
    };
    let Ok(session) = load_session() else {
        log_service_error("client-core-service ignored client ping because session is missing");
        return true;
    };
    let Some(local_device_id) = session
        .device_id
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
    else {
        log_service_error(
            "client-core-service ignored client ping because local device is missing",
        );
        return true;
    };
    if local_device_id == from_device_id {
        return true;
    }
    let Some(network_id) = session
        .active_network_id
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
    else {
        log_service_error(
            "client-core-service ignored client ping because active network is missing",
        );
        return true;
    };
    let Some(mqtt) = session.mqtt.as_ref() else {
        log_service_error(
            "client-core-service ignored client ping because mqtt credential is missing",
        );
        return true;
    };
    let replied_at_ms = current_timestamp_ms();
    let pong_body =
        client_message_mqtt::build_client_pong_body(&ping_id, sent_at_ms, replied_at_ms);
    match client_message_mqtt::publish_client_message(
        mqtt,
        network_id,
        local_device_id,
        from_device_id,
        &pong_body,
        Some(&serde_json::json!({
            "kind": "client_ping_pong",
            "pingId": ping_id,
            "sentAtMs": sent_at_ms,
            "repliedAtMs": replied_at_ms,
        })),
    ) {
        Ok(_) => log_service_error(format!(
            "client-core-service replied client ping pingId={} fromDeviceId={} toDeviceId={}",
            ping_id, from_device_id, local_device_id
        )),
        Err(error) => log_service_error(format!(
            "client-core-service failed to reply client ping pingId={}: {error:#}",
            ping_id
        )),
    }
    true
}

#[derive(Debug)]
struct ConnectPlanIngest {
    should_rebuild: bool,
    ack_delivery_id: Option<String>,
}

fn try_ingest_connect_plan(payload: &[u8]) -> Result<Option<ConnectPlanIngest>, String> {
    let value: serde_json::Value =
        serde_json::from_slice(payload).map_err(|err| format!("decode downstream json: {err}"))?;
    if value.get("type").and_then(serde_json::Value::as_str) != Some("connect_plan") {
        return Ok(None);
    }
    let plan = value
        .get("payload")
        .ok_or_else(|| "connect_plan payload is missing".to_string())?;
    if crate::persist_connect_plan_from_value(plan)
        .map_err(|err| format!("persist connect plan: {err:#}"))?
    {
        log_service_error("client-core-service accepted connect_plan for relay data plane");
        return Ok(Some(ConnectPlanIngest {
            should_rebuild: true,
            ack_delivery_id: downstream_message_id(&value),
        }));
    }
    log_service_error("client-core-service ignored empty connect_plan for relay data plane");
    Ok(Some(ConnectPlanIngest {
        should_rebuild: false,
        ack_delivery_id: None,
    }))
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
struct ClientMessagePayload {
    #[serde(default)]
    message_id: Option<String>,
    #[serde(default)]
    network_id: Option<String>,
    #[serde(default)]
    from_device_id: Option<String>,
    #[serde(default)]
    target_device_id: Option<String>,
    #[serde(default)]
    body: Option<String>,
}

fn try_ingest_network_config_changed(
    payload: &[u8],
    runtime: &Arc<Mutex<ClientRuntime<PlatformNetworkImpl>>>,
    task_queue: &Arc<Mutex<ControlTaskQueue>>,
    state_notifier: &Arc<StateChangeNotifier>,
) -> Result<bool, String> {
    let value: serde_json::Value =
        serde_json::from_slice(payload).map_err(|err| format!("decode downstream json: {err}"))?;
    let message_type = value
        .get("type")
        .and_then(serde_json::Value::as_str)
        .unwrap_or_default();
    if message_type != "network_config_changed" && message_type != "network_member_state_changed" {
        return Ok(false);
    }
    let ack_delivery_id = if message_type == "network_config_changed" {
        downstream_message_id(&value)
    } else {
        None
    };
    let session =
        load_session().map_err(|err| format!("load session for network module: {err}"))?;
    if !network_config_changed_targets_session(&value, &session) {
        log_service_error(
            "client-core-service ignored downstream network config change for stale session",
        );
        return Ok(true);
    }
    let remote_member_state_change = message_type == "network_member_state_changed"
        && !network_message_targets_self_device(&value, &session);
    let remote_network_config_change = message_type == "network_config_changed"
        && !network_message_targets_self_device(&value, &session);
    let suppress_remote_network_rebuild = remote_network_config_change
        && !claim_remote_network_config_rebuild(&value, current_timestamp_ms());
    let client = crate::control_plane::ControlPlaneClient::from_env();
    crate::network_module::refresh_network_module_from_session(&client, &session)
        .map_err(|err| format!("refresh client network module: {err:#}"))?;
    if let Some((virtual_ip, prefix_len)) = network_config_changed_assignment(&value, &session) {
        let mut runtime = runtime
            .lock()
            .map_err(|_| "client runtime mutex poisoned".to_string())?;
        crate::dispatch_with_side_effects(
            &mut runtime,
            ClientCommand::SyncAssignedIp(AssignedIpPayload {
                virtual_ip,
                prefix_len,
            }),
        );
    }
    let current_state = {
        let runtime = runtime
            .lock()
            .map_err(|_| "client runtime mutex poisoned".to_string())?;
        runtime.state().clone()
    };
    if remote_member_state_change || suppress_remote_network_rebuild {
        log_service_error(
            "client-core-service ignored remote network change for data plane refresh",
        );
        publish_state_business_event(
            state_notifier,
            BUSINESS_CONTROL_SYNC_CHANGED,
            &current_state,
        );
    } else if current_state.signed_in && current_state.network_enabled {
        {
            let mut queue = task_queue
                .lock()
                .map_err(|_| "control task queue mutex poisoned".to_string())?;
            if let Some(delivery_id) = ack_delivery_id {
                queue
                    .enqueue_downstream(
                        crate::control_tasks::ControlTaskAction::RefreshNetworkConfig,
                        delivery_id,
                        false,
                    )
                    .map_err(|err| err.to_string())?;
            } else {
                queue
                    .enqueue_downstream_unacked(
                        crate::control_tasks::ControlTaskAction::RefreshNetworkConfig,
                        false,
                    )
                    .map_err(|err| err.to_string())?;
            }
        }
        let state = crate::drain_pending_control_tasks(runtime, task_queue);
        let business_type = if state.error.is_some() {
            BUSINESS_NETWORK_SWITCH_FAILED
        } else {
            BUSINESS_NETWORK_RUNTIME_CHANGED
        };
        publish_state_business_event(state_notifier, business_type, &state);
    } else {
        publish_state_business_event(
            state_notifier,
            BUSINESS_CONTROL_SYNC_CHANGED,
            &current_state,
        );
    }
    Ok(true)
}

fn network_config_changed_targets_session(
    value: &serde_json::Value,
    session: &PersistedSession,
) -> bool {
    let payload = value.get("payload").unwrap_or(value);
    let expected_network_id = session
        .active_network_id
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty());
    if let Some(network_id) = payload
        .get("networkId")
        .or_else(|| value.get("networkId"))
        .and_then(serde_json::Value::as_str)
        .map(str::trim)
        .filter(|value| !value.is_empty())
    {
        if expected_network_id != Some(network_id) {
            return false;
        }
    }
    true
}

fn network_message_targets_self_device(
    value: &serde_json::Value,
    session: &PersistedSession,
) -> bool {
    let payload = value.get("payload").unwrap_or(value);
    let Some(self_device_id) = session
        .device_id
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
    else {
        return false;
    };
    payload
        .get("deviceId")
        .or_else(|| value.get("deviceId"))
        .and_then(serde_json::Value::as_str)
        .map(str::trim)
        .is_some_and(|device_id| device_id == self_device_id)
}

fn claim_remote_network_config_rebuild(value: &serde_json::Value, now_ms: u64) -> bool {
    let payload = value.get("payload").unwrap_or(value);
    let network_id = payload
        .get("networkId")
        .or_else(|| payload.get("network_id"))
        .or_else(|| value.get("networkId"))
        .or_else(|| value.get("network_id"))
        .and_then(serde_json::Value::as_str)
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .unwrap_or("-");
    let device_id = payload
        .get("deviceId")
        .or_else(|| payload.get("device_id"))
        .or_else(|| value.get("deviceId"))
        .or_else(|| value.get("device_id"))
        .and_then(serde_json::Value::as_str)
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .unwrap_or("-");
    let key = format!("{network_id}/{device_id}");
    let mut guard = REMOTE_NETWORK_CONFIG_REBUILDS
        .get_or_init(|| Mutex::new(HashMap::new()))
        .lock()
        .expect("remote network config rebuild mutex poisoned");
    if let Some(previous_ms) = guard.get(&key).copied() {
        if now_ms.saturating_sub(previous_ms) < REMOTE_NETWORK_CONFIG_REBUILD_COOLDOWN_MS {
            return false;
        }
    }
    guard.insert(key, now_ms);
    true
}

fn network_config_changed_assignment(
    value: &serde_json::Value,
    session: &PersistedSession,
) -> Option<(String, Option<u8>)> {
    let payload = value.get("payload")?;
    let target_device_id = payload
        .get("deviceId")
        .or_else(|| payload.get("device_id"))
        .or_else(|| value.get("deviceId"))
        .or_else(|| value.get("device_id"))
        .and_then(serde_json::Value::as_str)
        .map(str::trim)
        .filter(|value| !value.is_empty())?;
    let self_device_id = session
        .device_id
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())?;
    if target_device_id != self_device_id {
        return None;
    }
    let virtual_ip = payload
        .get("virtualIp")
        .and_then(serde_json::Value::as_str)
        .map(str::trim)
        .filter(|value| !value.is_empty())?
        .to_string();
    let prefix_len = payload
        .get("prefixLen")
        .or_else(|| payload.get("prefixLength"))
        .and_then(serde_json::Value::as_u64)
        .and_then(|value| u8::try_from(value).ok())
        .filter(|value| *value <= 32);
    Some((virtual_ip, prefix_len))
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

fn try_ingest_device_user_login_succeeded(
    payload: &[u8],
    runtime: &Arc<Mutex<ClientRuntime<PlatformNetworkImpl>>>,
    task_queue: &Arc<Mutex<ControlTaskQueue>>,
    state_notifier: &Arc<StateChangeNotifier>,
) -> Result<bool, String> {
    let value: serde_json::Value =
        serde_json::from_slice(payload).map_err(|err| format!("decode downstream json: {err}"))?;
    if value.get("type").and_then(serde_json::Value::as_str) != Some("device_user_login_succeeded")
    {
        return Ok(false);
    }
    let delivery_id = value
        .get("messageId")
        .or_else(|| value.get("deliveryId"))
        .and_then(serde_json::Value::as_str)
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .ok_or_else(|| "device_user_login_succeeded deliveryId/messageId is missing".to_string())?
        .to_string();
    let Some(auth_value) = value.get("payload").cloned() else {
        return Err("device_user_login_succeeded payload is missing".to_string());
    };
    let auth: AuthPayload = serde_json::from_value(auth_value)
        .map_err(|err| format!("decode device_user_login_succeeded payload: {err}"))?;
    let mut runtime = runtime
        .lock()
        .map_err(|_| "client runtime mutex poisoned".to_string())?;
    if let Some(expected_device_id) = runtime
        .state()
        .device_id
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
    {
        if auth
            .device_id
            .as_deref()
            .map(str::trim)
            .filter(|value| !value.is_empty())
            != Some(expected_device_id)
        {
            return Err("device_user_login_succeeded target device mismatch".to_string());
        }
    }
    let state =
        crate::dispatch_with_side_effects(&mut runtime, ClientCommand::ApplyDeviceUserLogin(auth));
    if let Some(error) = state.error {
        return Err(error);
    }
    {
        let mut queue = task_queue
            .lock()
            .map_err(|_| "control task queue mutex poisoned".to_string())?;
        let task = queue
            .enqueue_downstream(
                crate::control_tasks::ControlTaskAction::DeviceUserLoginSucceeded,
                delivery_id,
                false,
            )
            .map_err(|err| err.to_string())?;
        queue
            .mark_succeeded(&task.id)
            .map_err(|err| err.to_string())?;
    }
    publish_state_business_event(state_notifier, BUSINESS_SESSION_CHANGED, &state);
    Ok(true)
}

fn try_ingest_device_ip_reassigned(
    payload: &[u8],
    runtime: &Arc<Mutex<ClientRuntime<PlatformNetworkImpl>>>,
    state_notifier: &Arc<StateChangeNotifier>,
) -> Result<bool, String> {
    let value: serde_json::Value =
        serde_json::from_slice(payload).map_err(|err| format!("decode downstream json: {err}"))?;
    if value.get("type").and_then(serde_json::Value::as_str) != Some("device_ip_reassigned") {
        return Ok(false);
    }
    let Some(ip_payload) = value.get("payload") else {
        return Err("device_ip_reassigned payload is missing".to_string());
    };
    let target_device_id = ip_payload
        .get("deviceId")
        .and_then(serde_json::Value::as_str)
        .unwrap_or_default()
        .trim();
    let self_device_id = load_session()
        .ok()
        .and_then(|session| session.device_id)
        .unwrap_or_default();
    if target_device_id.is_empty() || target_device_id != self_device_id.trim() {
        return Ok(false);
    }
    let virtual_ip = ip_payload
        .get("virtualIp")
        .and_then(serde_json::Value::as_str)
        .unwrap_or_default()
        .trim();
    if virtual_ip.is_empty() {
        return Ok(false);
    }
    let prefix_len = ip_payload
        .get("prefixLen")
        .or_else(|| ip_payload.get("prefixLength"))
        .and_then(serde_json::Value::as_u64)
        .and_then(|value| u8::try_from(value).ok());
    let mut runtime = runtime
        .lock()
        .map_err(|_| "client runtime mutex poisoned".to_string())?;
    let state = crate::dispatch_with_side_effects(
        &mut runtime,
        ClientCommand::SyncAssignedIp(AssignedIpPayload {
            virtual_ip: virtual_ip.to_string(),
            prefix_len,
        }),
    );
    if let Some(error) = state.error {
        return Err(error);
    }
    publish_state_business_event(state_notifier, BUSINESS_CONTROL_SYNC_CHANGED, &state);
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
    session: &PersistedSession,
    message: &ControlTransportMessage,
) -> Result<(), String> {
    let payload = serde_json::to_vec(&message.payload)
        .map_err(|err| format!("encode outbox payload {}: {err}", message.id))?;
    let Some(mqtt) = session.mqtt.as_ref() else {
        return Err("mqtt credential is missing for outbox publish".to_string());
    };
    let credential = ThinMqttCredential {
        broker_url: mqtt.broker_url.clone(),
        client_id: mqtt.client_id.clone(),
        username: mqtt.username.clone(),
        password: mqtt.password.clone(),
    };
    let mut errors = Vec::new();
    for attempt in 1..=3 {
        let suffix = outbox_client_suffix(message, attempt);
        match publish_outbox_message_once(&credential, &suffix, message, &payload) {
            Ok(()) => return Ok(()),
            Err(error) => {
                errors.push(format!("attempt {attempt}: {error}"));
                thread::sleep(Duration::from_millis(250 * attempt as u64));
            }
        }
    }
    Err(errors.join("; "))
}

fn publish_outbox_message_once(
    credential: &ThinMqttCredential,
    suffix: &str,
    message: &ControlTransportMessage,
    payload: &[u8],
) -> Result<(), String> {
    let mut client = ThinControlMqttClient::connect_without_subscription(credential, suffix)?;
    client.publish(&message.topic, payload, thin_qos(message.qos))
}

fn outbox_client_suffix(message: &ControlTransportMessage, attempt: usize) -> String {
    let mut suffix = String::with_capacity(64);
    suffix.push_str("v2-outbox-");
    suffix.push_str(&current_timestamp_ms().to_string());
    suffix.push('-');
    suffix.push_str(&attempt.to_string());
    for ch in message.id.chars() {
        if suffix.len() >= 72 {
            break;
        }
        if ch.is_ascii_alphanumeric() || ch == '-' {
            suffix.push(ch);
        } else {
            suffix.push('-');
        }
    }
    suffix
}

fn publish_outbox_message_async(
    session: PersistedSession,
    message: ControlTransportMessage,
    task_queue: Arc<Mutex<ControlTaskQueue>>,
) {
    std::thread::spawn(move || {
        if let Err(error) = publish_outbox_message(&session, &message)
            .and_then(|_| mark_transport_published(&message, &task_queue))
        {
            log_service_error(format!(
                "client-core-service outbox publish failed id={} topic={} error={}",
                message.id, message.topic, error
            ));
        }
    });
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
        "{}|{}|{}|{}|{}|{}",
        session.session_kind.as_str(),
        session.user_id.as_str(),
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
    if let Some(error) = error.as_deref() {
        log_service_error(format!(
            "client-core-service control mqtt worker stopped reconnectKey={} error={}",
            reconnect_key, error
        ));
    } else {
        log_service_error(format!(
            "client-core-service control mqtt worker stopped reconnectKey={} error=",
            reconnect_key
        ));
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

#[cfg(test)]
mod tests {
    use std::sync::{Arc, Mutex};

    use client_core::{AuthPayload, ClientCommand, ClientRuntime};
    use client_core_platform::PlatformNetworkImpl;

    use super::{
        ingest_downstream_publish, network_config_changed_assignment,
        network_config_changed_targets_session,
    };
    use crate::session_store::PersistedSession;
    use crate::{
        control_tasks::ControlTaskQueue, StateChangeNotifier, BUSINESS_CONTROL_SYNC_CHANGED,
    };

    #[test]
    fn client_message_downstream_updates_runtime_state_and_notifies() {
        let runtime = Arc::new(Mutex::new(ClientRuntime::new(PlatformNetworkImpl)));
        let task_queue = Arc::new(Mutex::new(ControlTaskQueue::load_default()));
        let state_notifier = Arc::new(StateChangeNotifier::default());
        let payload = serde_json::json!({
            "type": "client_message",
            "messageId": "envelope-msg-1",
            "payload": {
                "messageId": "client-msg-1",
                "networkId": "net-1",
                "fromDeviceId": "mac-device",
                "targetDeviceId": "ios-device",
                "body": "hello ios"
            }
        });
        let payload = serde_json::to_vec(&payload).expect("encode payload");

        ingest_downstream_publish(&payload, &runtime, &task_queue, &state_notifier)
            .expect("ingest client message");

        let state = runtime.lock().expect("runtime mutex").state().clone();
        assert_eq!(
            state.last_client_message_id.as_deref(),
            Some("client-msg-1")
        );
        assert_eq!(
            state.last_client_message_from_device_id.as_deref(),
            Some("mac-device")
        );
        assert_eq!(state.last_client_message_body.as_deref(), Some("hello ios"));
        assert_eq!(state.notice.as_deref(), Some("clientMessageReceived"));

        let revision = *state_notifier.revision.lock().expect("revision mutex");
        assert_eq!(revision, 1);
        let event = state_notifier
            .event
            .lock()
            .expect("event mutex")
            .clone()
            .expect("business event");
        assert_eq!(event.business_type, BUSINESS_CONTROL_SYNC_CHANGED);
        assert_eq!(
            event
                .business_data
                .get("lastClientMessageBody")
                .and_then(serde_json::Value::as_str),
            Some("hello ios")
        );
    }

    #[test]
    fn device_user_login_downstream_rejects_mismatched_device_id() {
        let runtime = Arc::new(Mutex::new(ClientRuntime::new(PlatformNetworkImpl)));
        {
            let mut guard = runtime.lock().expect("runtime mutex");
            guard
                .dispatch(ClientCommand::ApplyDeviceUserLogin(AuthPayload {
                    access_token: "existing-token".to_string(),
                    refresh_token: None,
                    user_id: "user-1".to_string(),
                    user_label: "user@example.com".to_string(),
                    device_id: Some("dev-1".to_string()),
                    active_network_id: None,
                    virtual_ip: None,
                    expires_in: None,
                }))
                .expect("seed runtime device id");
        }
        let task_queue = Arc::new(Mutex::new(ControlTaskQueue::load_default()));
        let state_notifier = Arc::new(StateChangeNotifier::default());
        let payload = serde_json::json!({
            "type": "device_user_login_succeeded",
            "messageId": "login-msg-1",
            "payload": {
                "accessToken": "new-token",
                "userId": "user-2",
                "userLabel": "other@example.com",
                "deviceId": "dev-other"
            }
        });
        let payload = serde_json::to_vec(&payload).expect("encode payload");

        let err = ingest_downstream_publish(&payload, &runtime, &task_queue, &state_notifier)
            .expect_err("mismatched login must be rejected");

        assert!(err.contains("target device mismatch"), "{err}");
        let state = runtime.lock().expect("runtime mutex").state().clone();
        assert_eq!(state.device_id.as_deref(), Some("dev-1"));
        assert_eq!(state.user_label.as_deref(), Some("user@example.com"));
    }

    #[test]
    fn network_config_changed_rejects_stale_network() {
        let mut session = PersistedSession::empty();
        session.device_id = Some("dev-current".to_string());
        session.active_network_id = Some("net-current".to_string());

        let current = serde_json::json!({
            "type": "network_config_changed",
            "payload": {
                "networkId": "net-current",
                "deviceId": "dev-current",
                "virtualIp": "10.0.0.8",
                "prefixLen": 20
            }
        });
        assert!(network_config_changed_targets_session(&current, &session));

        let stale_network = serde_json::json!({
            "type": "network_config_changed",
            "payload": {
                "networkId": "net-old",
                "deviceId": "dev-current",
                "virtualIp": "10.0.0.9",
                "prefixLen": 20
            }
        });
        assert!(!network_config_changed_targets_session(
            &stale_network,
            &session
        ));

        let peer_device = serde_json::json!({
            "type": "network_config_changed",
            "payload": {
                "networkId": "net-current",
                "deviceId": "dev-old",
                "virtualIp": "10.0.0.10",
                "prefixLen": 20
            }
        });
        assert!(network_config_changed_targets_session(
            &peer_device,
            &session
        ));
    }

    #[test]
    fn network_config_changed_assignment_requires_current_device() {
        let mut session = PersistedSession::empty();
        session.device_id = Some("dev-current".to_string());
        session.active_network_id = Some("net-current".to_string());

        let current = serde_json::json!({
            "type": "network_config_changed",
            "payload": {
                "networkId": "net-current",
                "deviceId": "dev-current",
                "virtualIp": "10.0.0.8",
                "prefixLen": 20
            }
        });
        assert_eq!(
            network_config_changed_assignment(&current, &session),
            Some(("10.0.0.8".to_string(), Some(20)))
        );

        let peer_device = serde_json::json!({
            "type": "network_config_changed",
            "payload": {
                "networkId": "net-current",
                "deviceId": "dev-peer",
                "virtualIp": "10.0.0.1",
                "prefixLen": 20
            }
        });
        assert_eq!(
            network_config_changed_assignment(&peer_device, &session),
            None
        );

        let missing_device = serde_json::json!({
            "type": "network_config_changed",
            "payload": {
                "networkId": "net-current",
                "virtualIp": "10.0.0.2",
                "prefixLen": 20
            }
        });
        assert_eq!(
            network_config_changed_assignment(&missing_device, &session),
            None
        );
    }
}
