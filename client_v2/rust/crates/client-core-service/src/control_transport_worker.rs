use std::{
    fs,
    path::PathBuf,
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
            let error = result.err();
            release_worker(&worker_state, reconnect_key, error);
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
            publish_outbox_message(&mut client, &message)?;
            last_keepalive_ping_ms = Some(now_ms);
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
        let suffix = format!("/{device_id}");
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
    if try_ingest_auth_callback(payload, runtime, state_notifier)? {
        log_service_error(
            "client-core-service consumed downstream control message as auth_callback",
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
            if let Some(delivery_id) = connect_plan.rebuild_delivery_id {
                log_service_error(format!(
                    "client-core-service scheduling network rebuild for connect_plan deliveryId={delivery_id}"
                ));
                {
                    let mut queue = task_queue
                        .lock()
                        .map_err(|_| "control task queue mutex poisoned".to_string())?;
                    queue
                        .enqueue_downstream(
                            crate::control_tasks::ControlTaskAction::EnableNetwork,
                            delivery_id,
                            false,
                        )
                        .map_err(|err| err.to_string())?;
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
    if try_ingest_relay_data_plane_policy(payload, runtime, state_notifier)? {
        log_service_error(
            "client-core-service consumed downstream control message as relay_data_plane_policy",
        );
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
    task_queue: &Arc<Mutex<ControlTaskQueue>>,
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
    let changed_at = payload_value
        .get("changedAt")
        .and_then(serde_json::Value::as_i64)
        .unwrap_or_default();
    log_service_error(format!(
        "client-core-service network presence message type={message_type} networkId={network_id} deviceId={device_id} online={online} onlineDevices={online_count}"
    ));
    if let Ok(session) = load_session() {
        let is_self = session
            .device_id
            .as_deref()
            .map(str::trim)
            .filter(|value| !value.is_empty())
            == Some(device_id.trim());
        let state = {
            let runtime = runtime
                .lock()
                .map_err(|_| "client runtime mutex poisoned".to_string())?;
            runtime.state().clone()
        };
        if state.signed_in && state.network_enabled && !is_self {
            let delivery_id = network_presence_delivery_id(
                &value,
                message_type,
                network_id,
                device_id,
                changed_at,
            );
            log_service_error(format!(
                "client-core-service scheduling peer rebuild for network presence deliveryId={delivery_id}"
            ));
            {
                let mut queue = task_queue
                    .lock()
                    .map_err(|_| "control task queue mutex poisoned".to_string())?;
                queue
                    .enqueue_downstream(
                        crate::control_tasks::ControlTaskAction::EnableNetwork,
                        delivery_id,
                        false,
                    )
                    .map_err(|err| err.to_string())?;
            }
            let state = crate::drain_pending_control_tasks(runtime, task_queue);
            let business_type = if state.error.is_some() {
                BUSINESS_NETWORK_SWITCH_FAILED
            } else {
                BUSINESS_NETWORK_RUNTIME_CHANGED
            };
            publish_state_business_event(state_notifier, business_type, &state);
        } else {
            publish_state_business_event(state_notifier, BUSINESS_CONTROL_SYNC_CHANGED, &state);
        }
    }
    Ok(true)
}

fn is_device_network_presence_event(message_type: &str) -> bool {
    matches!(
        message_type,
        DEVICE_NETWORK_ENABLED_EVENT | DEVICE_NETWORK_DISABLED_EVENT
    )
}

fn network_presence_delivery_id(
    envelope: &serde_json::Value,
    message_type: &str,
    network_id: &str,
    device_id: &str,
    changed_at: i64,
) -> String {
    envelope
        .get("messageId")
        .or_else(|| envelope.get("message_id"))
        .and_then(serde_json::Value::as_str)
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .map(str::to_string)
        .unwrap_or_else(|| {
            format!("network-presence-{message_type}-{network_id}-{device_id}-{changed_at}")
        })
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

fn try_ingest_relay_data_plane_policy(
    payload: &[u8],
    runtime: &Arc<Mutex<ClientRuntime<PlatformNetworkImpl>>>,
    state_notifier: &Arc<StateChangeNotifier>,
) -> Result<bool, String> {
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
    log_service_error(format!(
        "client-core-service decoded relay data plane policy policyId={} networkId={} pathType={} ttlMs={} targetDevices={}",
        incoming.policy_id.as_deref().unwrap_or_default(),
        incoming.network_id.as_deref().unwrap_or_default(),
        incoming.path_type.as_deref().unwrap_or_default(),
        incoming.ttl_ms
            .map(|value| value.to_string())
            .unwrap_or_default(),
        incoming.target_device_ids.len()
    ));
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
        "client-core-service accepted relay data plane policy policyId={} networkId={} pathType={} relayMtu={} maxFramePayload={} path={}",
        incoming.policy_id.as_deref().unwrap_or_default(),
        incoming.network_id.as_deref().unwrap_or_default(),
        incoming.path_type.as_deref().unwrap_or_default(),
        incoming.relay_mtu,
        incoming.max_frame_payload,
        path.display()
    ));
    apply_relay_data_plane_policy(runtime, state_notifier, incoming.policy_id.as_deref());
    Ok(true)
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

fn apply_relay_data_plane_policy(
    runtime: &Arc<Mutex<ClientRuntime<PlatformNetworkImpl>>>,
    state_notifier: &Arc<StateChangeNotifier>,
    policy_id: Option<&str>,
) {
    let current_state = {
        let runtime = runtime.lock().expect("client runtime mutex poisoned");
        runtime.state().clone()
    };
    if !current_state.signed_in || !current_state.network_enabled {
        log_service_error(format!(
            "client-core-service relay data plane policy stored without active apply policyId={} signedIn={} networkEnabled={}",
            policy_id.unwrap_or_default(),
            current_state.signed_in,
            current_state.network_enabled
        ));
        publish_state_business_event(
            state_notifier,
            BUSINESS_CONTROL_SYNC_CHANGED,
            &current_state,
        );
        return;
    }
    log_service_error(format!(
        "client-core-service applying relay data plane policy policyId={}",
        policy_id.unwrap_or_default()
    ));
    let state = crate::reconfigure_active_control_network(runtime);
    log_service_error(format!(
        "client-core-service applied relay data plane policy policyId={} networkEnabled={} ip={} error={}",
        policy_id.unwrap_or_default(),
        state.network_enabled,
        state.virtual_ip.as_deref().unwrap_or_default(),
        state.error.as_deref().unwrap_or_default()
    ));
    let business_type = if state.error.is_some() {
        BUSINESS_NETWORK_SWITCH_FAILED
    } else {
        BUSINESS_NETWORK_RUNTIME_CHANGED
    };
    publish_state_business_event(state_notifier, business_type, &state);
}

#[derive(Debug)]
struct ConnectPlanIngest {
    rebuild_delivery_id: Option<String>,
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
    let delivery_id = connect_plan_delivery_id(&value, plan);
    if crate::persist_connect_plan_from_value(plan)
        .map_err(|err| format!("persist connect plan: {err:#}"))?
    {
        log_service_error("client-core-service accepted connect_plan for relay data plane");
        return Ok(Some(ConnectPlanIngest {
            rebuild_delivery_id: Some(delivery_id),
        }));
    }
    log_service_error("client-core-service ignored empty connect_plan for relay data plane");
    Ok(Some(ConnectPlanIngest {
        rebuild_delivery_id: None,
    }))
}

fn connect_plan_delivery_id(envelope: &serde_json::Value, plan: &serde_json::Value) -> String {
    envelope
        .get("messageId")
        .or_else(|| envelope.get("message_id"))
        .and_then(serde_json::Value::as_str)
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .map(|value| format!("connect-plan-{value}"))
        .unwrap_or_else(|| {
            let peer = plan
                .get("peerNodeId")
                .or_else(|| plan.get("peer_node_id"))
                .and_then(serde_json::Value::as_str)
                .map(str::trim)
                .filter(|value| !value.is_empty())
                .unwrap_or("peer");
            format!("connect-plan-{peer}-{}", current_timestamp_ms())
        })
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
    source_device_id: Option<String>,
    #[serde(default)]
    peer_device_id: Option<String>,
    #[serde(default)]
    path_type: Option<String>,
    #[serde(default)]
    preferred_path_types: Vec<String>,
    #[serde(default)]
    probe_interval_ms: Option<u64>,
    #[serde(default)]
    failover_after_ms: Option<u64>,
    #[serde(default)]
    upgrade_successes: Option<u32>,
    #[serde(default)]
    failed_path_cooldown_probes: Option<u32>,
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
            "global" | "region" | "network" | "device" | "device_override" | "peer_pair" => {}
            _ => return Err("unsupported relay data plane policy scope".to_string()),
        }
        if scope == "peer_pair" {
            let source_device_id = policy
                .source_device_id
                .as_deref()
                .map(str::trim)
                .unwrap_or_default();
            let peer_device_id = policy
                .peer_device_id
                .as_deref()
                .map(str::trim)
                .unwrap_or_default();
            if source_device_id.is_empty() || peer_device_id.is_empty() {
                return Err(
                    "peer_pair relay policy requires sourceDeviceId and peerDeviceId".to_string(),
                );
            }
        }
    }
    if let Some(path_type) = policy.path_type.as_deref() {
        match path_type.trim() {
            "" | "direct" | "p2p" | "relay" | "lan_udp" | "ipv6_udp" | "direct_udp"
            | "relay_udp" | "derp_tcp_tls_443" => {}
            _ => return Err("unsupported relay data plane policy pathType".to_string()),
        }
    }
    for path_type in &policy.preferred_path_types {
        match path_type.trim() {
            "lan_udp" | "ipv6_udp" | "direct_udp" | "relay_udp" | "derp_tcp_tls_443" => {}
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
    if let Some(probe_interval_ms) = policy.probe_interval_ms {
        if !(1_000..=300_000).contains(&probe_interval_ms) {
            return Err("probeIntervalMs out of range".to_string());
        }
    }
    if let Some(failover_after_ms) = policy.failover_after_ms {
        if !(1_000..=600_000).contains(&failover_after_ms) {
            return Err("failoverAfterMs out of range".to_string());
        }
    }
    if let Some(upgrade_successes) = policy.upgrade_successes {
        if !(1..=10).contains(&upgrade_successes) {
            return Err("upgradeSuccesses out of range".to_string());
        }
    }
    if let Some(failed_path_cooldown_probes) = policy.failed_path_cooldown_probes {
        if !(1..=20).contains(&failed_path_cooldown_probes) {
            return Err("failedPathCooldownProbes out of range".to_string());
        }
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
    let session =
        load_session().map_err(|err| format!("load session for network module: {err}"))?;
    let client = crate::control_plane::ControlPlaneClient::from_env();
    crate::network_module::refresh_network_module_from_session(&client, &session)
        .map_err(|err| format!("refresh client network module: {err:#}"))?;
    let current_state = {
        let runtime = runtime
            .lock()
            .map_err(|_| "client runtime mutex poisoned".to_string())?;
        runtime.state().clone()
    };
    if current_state.signed_in && current_state.network_enabled {
        {
            let mut queue = task_queue
                .lock()
                .map_err(|_| "control task queue mutex poisoned".to_string())?;
            queue
                .enqueue_downstream(
                    crate::control_tasks::ControlTaskAction::EnableNetwork,
                    format!("network-module-refresh-{}", crate::current_timestamp_ms()),
                    false,
                )
                .map_err(|err| err.to_string())?;
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

    use client_core::ClientRuntime;
    use client_core_platform::PlatformNetworkImpl;

    use super::ingest_downstream_publish;
    use crate::{
        control_tasks::ControlTaskQueue, StateChangeNotifier, BUSINESS_CONTROL_SYNC_CHANGED,
    };

    #[test]
    fn client_message_downstream_updates_runtime_state_and_notifies() {
        let runtime = Arc::new(Mutex::new(ClientRuntime::new(
            PlatformNetworkImpl::default(),
        )));
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
}
