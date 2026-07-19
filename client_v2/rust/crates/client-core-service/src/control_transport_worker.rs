use std::{
    collections::BTreeSet,
    sync::{Arc, Mutex},
    thread,
    time::Duration,
};

use client_core::{AssignedIpPayload, AuthPayload, ClientCommand, ClientMessageNoticePayload};
use control_mqtt_client::{ThinControlMqttClient, ThinMqttCredential, ThinMqttQoS};
use serde::Deserialize;

use crate::{
    client_message_mqtt,
    control_plane::ControlPlaneClient,
    control_tasks::ControlTaskQueue,
    control_transport::{
        self, ControlTransportMessage, ControlTransportMessageKind, ControlTransportTickRequest,
        MqttQos,
    },
    current_timestamp_ms, load_session, log_service_error,
    network_event::{
        apply_device_network_membership, network_event_business_data,
        network_event_targets_session, network_event_topics_for_session,
        synthetic_snapshot_event_id, DeviceNetworkMembershipChangedPayload, NetworkEventEnvelope,
        NetworkEventType,
    },
    network_event_apply::ApplyResult,
    network_event_projection::{
        apply_network_event_projection, apply_prepared_session_projection,
        mark_network_projection_syncing,
    },
    network_runtime_state::runtime_network_state_store,
    persist_last_client_message_payload, persist_session, publish_state_business_event,
    publish_state_business_event_with_extra,
    runtime_actor::RuntimeActorHandle,
    runtime_event_hub::RuntimeEventHub,
    session_device_api_token,
    session_store::{
        current_session_runtime_epoch, prepare_session_from_control_plane, PreparedSession,
    },
    PersistedSession, BUSINESS_CONTROL_SYNC_CHANGED, BUSINESS_NETWORK_RUNTIME_CHANGED,
    BUSINESS_NETWORK_SWITCH_FAILED, BUSINESS_SESSION_CHANGED,
};

const MQTT_KEEPALIVE_PING_INTERVAL_MS: u64 = 15_000;
const ACTIVE_NETWORK_RECONCILE_INTERVAL_MS: u64 = 10_000;
const MQTT_RECONNECT_AFTER_SESSION_REFRESH_MS: u64 = 10 * 60 * 1000;

#[derive(Debug, Default)]
pub struct ControlTransportWorkerState {
    running: bool,
    reconnect_key: Option<String>,
    backoff_ms: u64,
    next_attempt_ms: u64,
}

pub fn spawn_control_transport_supervisor(
    runtime: RuntimeActorHandle,
    task_queue: Arc<Mutex<ControlTaskQueue>>,
    worker_state: Arc<Mutex<ControlTransportWorkerState>>,
    state_notifier: Arc<RuntimeEventHub>,
) {
    thread::spawn(move || loop {
        thread::sleep(Duration::from_secs(5));
        wake_control_transport_worker(&runtime, &task_queue, &worker_state, &state_notifier);
    });
}

pub fn wake_control_transport_worker(
    runtime: &RuntimeActorHandle,
    task_queue: &Arc<Mutex<ControlTaskQueue>>,
    worker_state: &Arc<Mutex<ControlTransportWorkerState>>,
    state_notifier: &Arc<RuntimeEventHub>,
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
    let runtime = runtime.clone();
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
    mut session: PersistedSession,
    runtime: RuntimeActorHandle,
    task_queue: Arc<Mutex<ControlTaskQueue>>,
    state_notifier: Arc<RuntimeEventHub>,
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
    let mut subscribed_network_topics = BTreeSet::new();
    for topic in network_event_topics_for_session(&session) {
        match client.subscribe(&topic) {
            Ok(()) => {
                subscribed_network_topics.insert(topic.clone());
                log_service_error(format!(
                    "client-core-service subscribed network event topic={topic}"
                ));
            }
            Err(error) => log_service_error(format!(
                "client-core-service failed to subscribe network event topic={topic}: {error}"
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
    let mut last_active_network_reconcile_ms = Some(current_timestamp_ms());
    let mut last_network_subscribe_attempt_ms = Some(current_timestamp_ms());
    let mut initial_endpoint_report_queued = false;
    let connected_at_ms = current_timestamp_ms();
    loop {
        let latest_session = load_session().map_err(|err| err.to_string())?;
        if !mqtt_connection_matches(&latest_session, &session) {
            return Err("control transport session changed".to_string());
        }
        session = latest_session;
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
        if !initial_endpoint_report_queued && last_runtime_state_ms.is_some() {
            if let Some(message) =
                control_transport::pending_endpoint_report_message(&session, now_ms)
            {
                publish_outbox_message_async(session.clone(), message, Arc::clone(&task_queue));
                initial_endpoint_report_queued = true;
            }
        }
        if now_ms.saturating_sub(last_network_subscribe_attempt_ms.unwrap_or(0)) >= 2_000 {
            for topic in network_event_topics_for_session(&session) {
                if subscribed_network_topics.contains(&topic) {
                    continue;
                }
                match client.subscribe(&topic) {
                    Ok(()) => {
                        subscribed_network_topics.insert(topic.clone());
                        log_service_error(format!(
                            "client-core-service dynamically subscribed network event topic={topic}"
                        ));
                    }
                    Err(error) => log_service_error(format!(
                        "client-core-service failed dynamic network event subscription topic={topic}: {error}"
                    )),
                }
            }
            last_network_subscribe_attempt_ms = Some(now_ms);
        }
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
        if now_ms.saturating_sub(last_active_network_reconcile_ms.unwrap_or(0))
            >= ACTIVE_NETWORK_RECONCILE_INTERVAL_MS
        {
            reconcile_active_network_state(&runtime, &task_queue, &state_notifier)?;
            last_active_network_reconcile_ms = Some(now_ms);
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
                    initial_endpoint_report_queued = true;
                }
            }
        }
    }
}

fn reconcile_active_network_state(
    runtime: &RuntimeActorHandle,
    task_queue: &Arc<Mutex<ControlTaskQueue>>,
    state_notifier: &Arc<RuntimeEventHub>,
) -> Result<(), String> {
    let current_state = runtime.snapshot().state;
    if !current_state.signed_in || !current_state.network_enabled {
        return Ok(());
    }
    log_service_error("client-core-service reconciling active network state");
    {
        let mut queue = task_queue
            .lock()
            .map_err(|_| "control task queue mutex poisoned".to_string())?;
        queue
            .enqueue_downstream_unacked(
                crate::control_tasks::ControlTaskAction::ReconcileNetworkState,
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
    Ok(())
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
    runtime: &RuntimeActorHandle,
    _task_queue: &Arc<Mutex<ControlTaskQueue>>,
    state_notifier: &Arc<RuntimeEventHub>,
) -> Result<(), String> {
    crate::sync_control_assignment(runtime);
    let session = load_session().map_err(|err| err.to_string())?;
    let state = runtime.snapshot().state;
    if !state.signed_in || !state.network_enabled {
        publish_state_business_event(state_notifier, BUSINESS_CONTROL_SYNC_CHANGED, &state);
        return Ok(());
    }
    if let Err(error) = refresh_network_snapshot_cache(runtime, &session) {
        log_service_error(format!(
            "client-core-service startup network snapshot refresh skipped: {error}"
        ));
    }
    log_service_error("client-core-service skipped reconnect config rebuild for active data plane");
    let business_type = BUSINESS_CONTROL_SYNC_CHANGED;
    publish_state_business_event(state_notifier, business_type, &state);
    Ok(())
}

fn refresh_network_snapshot_cache(
    runtime: &RuntimeActorHandle,
    session: &PersistedSession,
) -> Result<(), String> {
    let session_epoch = current_session_runtime_epoch();
    let network_id = session
        .active_network_id
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .ok_or_else(|| "active network id missing".to_string())?;
    let local_device_id = session
        .device_id
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .ok_or_else(|| "local device id missing".to_string())?;
    let client = ControlPlaneClient::from_env();
    let snapshot = client
        .network_snapshot(
            session_device_api_token(session),
            network_id,
            local_device_id,
        )
        .map_err(|err| format!("load startup network snapshot: {err:#}"))?;
    let snapshot_envelope = NetworkEventEnvelope {
        r#type: "network_event".to_string(),
        network_id: snapshot.network_id.clone(),
        version: snapshot.version,
        event_id: synthetic_snapshot_event_id("startup", &snapshot.network_id, snapshot.version),
        event_type: NetworkEventType::NetworkSnapshot,
        occurred_at: current_timestamp_ms(),
        payload: serde_json::to_value(&snapshot.snapshot)
            .map_err(|err| format!("encode startup network snapshot payload: {err}"))?,
    };
    let projection_session = session.clone();
    let projection_device_id = local_device_id.to_string();
    let projection_envelope = snapshot_envelope.clone();
    let snapshot_result = runtime
        .call_named(
            "network.startup_snapshot.projection",
            Some(snapshot_envelope.event_id.clone()),
            move |_runtime| {
                apply_network_event_projection(
                    session_epoch,
                    &projection_session,
                    &projection_device_id,
                    &projection_envelope,
                )
            },
        )
        .map_err(|err| format!("apply startup network snapshot: {err:#}"))?;
    log_service_error(format!(
        "client-core-service refreshed startup network snapshot networkId={} version={} result={:?}",
        snapshot.network_id, snapshot.version, snapshot_result
    ));
    Ok(())
}

fn ingest_downstream_publish(
    payload: &[u8],
    runtime: &RuntimeActorHandle,
    task_queue: &Arc<Mutex<ControlTaskQueue>>,
    state_notifier: &Arc<RuntimeEventHub>,
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
    if try_ingest_device_network_membership_changed(payload, runtime, state_notifier)? {
        log_service_error(
            "client-core-service consumed downstream control message as device_network_membership_changed",
        );
        return Ok(());
    }
    if try_ingest_device_ip_reassigned(payload, runtime, state_notifier)? {
        log_service_error(
            "client-core-service consumed downstream control message as device_ip_reassigned",
        );
        return Ok(());
    }
    // `network_map_response` is retained only for relay candidate refreshes.
    // Network membership / resolver / ACL state now flows through `network_event`.
    if try_ingest_relay_candidates_response(payload, runtime, state_notifier)? {
        log_service_error(
            "client-core-service consumed downstream control message as relay_candidates_response",
        );
        return Ok(());
    }
    if try_ingest_network_event(payload, runtime, task_queue, state_notifier)? {
        log_service_error(
            "client-core-service consumed downstream control message as network_event",
        );
        return Ok(());
    }
    if let Some(connect_plan) = try_ingest_connect_plan(payload, runtime)? {
        let current_state = runtime.snapshot().state;
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
                                crate::control_tasks::ControlTaskAction::ReconcileNetworkState,
                                delivery_id,
                                false,
                            )
                            .map_err(|err| err.to_string())?;
                    } else {
                        queue
                            .enqueue_downstream_unacked(
                                crate::control_tasks::ControlTaskAction::ReconcileNetworkState,
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
    let message: serde_json::Value = serde_json::from_slice(payload)
        .map_err(|err| format!("decode downstream control message: {err}"))?;
    let message_type = message
        .get("type")
        .and_then(serde_json::Value::as_str)
        .unwrap_or_default()
        .to_string();
    let extra_business_data = downstream_control_business_data(&message);
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
        if let Some(extra) = extra_business_data.clone() {
            publish_state_business_event_with_extra(state_notifier, business_type, &state, extra);
        } else {
            publish_state_business_event(state_notifier, business_type, &state);
        }
    } else {
        let state = runtime.snapshot().state;
        if let Some(extra) = extra_business_data {
            publish_state_business_event_with_extra(
                state_notifier,
                BUSINESS_CONTROL_SYNC_CHANGED,
                &state,
                extra,
            );
        } else {
            publish_state_business_event(state_notifier, BUSINESS_CONTROL_SYNC_CHANGED, &state);
        }
    }
    Ok(())
}

fn try_ingest_device_network_membership_changed(
    payload: &[u8],
    runtime: &RuntimeActorHandle,
    state_notifier: &Arc<RuntimeEventHub>,
) -> Result<bool, String> {
    let value: serde_json::Value =
        serde_json::from_slice(payload).map_err(|err| format!("decode downstream json: {err}"))?;
    if value.get("type").and_then(serde_json::Value::as_str)
        != Some("device_network_membership_changed")
    {
        return Ok(false);
    }
    let delivery_id = downstream_message_id(&value);
    let event: DeviceNetworkMembershipChangedPayload = serde_json::from_value(
        value
            .get("payload")
            .cloned()
            .ok_or_else(|| "device_network_membership_changed payload is missing".to_string())?,
    )
    .map_err(|err| format!("decode device network membership payload: {err}"))?;
    let expected_revision = runtime.snapshot().revision;
    let mut session = load_session().map_err(|err| err.to_string())?;
    let expected_device_id = session
        .device_id
        .as_deref()
        .unwrap_or_default()
        .trim()
        .to_string();
    apply_device_network_membership(&mut session, event).map_err(|error| error.to_string())?;
    let mut prepared = PreparedSession::from_session(session.clone());
    if !session.access_token.trim().is_empty() {
        let client = ControlPlaneClient::from_env();
        if let Ok(configs) =
            client.device_network_configs(session_device_api_token(&session), &expected_device_id)
        {
            prepared.network_configs = configs;
        }
    }
    let state = runtime
        .call_named_if_revision(
            "network.membership.commit",
            delivery_id
                .clone()
                .or_else(|| Some(expected_device_id.clone())),
            expected_revision,
            move |runtime| {
                crate::ensure_runtime_session_matches(runtime, &prepared.session)?;
                crate::commit_registered_session(runtime, &prepared)?;
                Ok(runtime.state().clone())
            },
        )
        .map_err(|err| format!("commit network membership: {err:#}"))?
        .unwrap_or_else(|| runtime.snapshot().state);
    publish_state_business_event_with_extra(
        state_notifier,
        BUSINESS_NETWORK_RUNTIME_CHANGED,
        &state,
        serde_json::json!({
            "messageType": "device_network_membership_changed",
            "messageId": delivery_id,
        }),
    );
    Ok(true)
}

fn downstream_control_business_data(message: &serde_json::Value) -> Option<serde_json::Value> {
    let message_type = message.get("type").and_then(serde_json::Value::as_str)?;
    let payload = message.get("payload")?;
    match message_type {
        "device_network_disabled" => Some(serde_json::json!({
            "messageType": "device_network_disabled",
            "networkId": payload.get("networkId").and_then(serde_json::Value::as_str).unwrap_or_default(),
            "deviceId": payload.get("deviceId").and_then(serde_json::Value::as_str).unwrap_or_default(),
            "attachmentId": payload.get("attachmentId").and_then(serde_json::Value::as_str).unwrap_or_default(),
            "reconfigureRequired": true,
        })),
        _ => None,
    }
}

fn try_ingest_network_event(
    payload: &[u8],
    runtime: &RuntimeActorHandle,
    task_queue: &Arc<Mutex<ControlTaskQueue>>,
    state_notifier: &Arc<RuntimeEventHub>,
) -> Result<bool, String> {
    let value: serde_json::Value =
        serde_json::from_slice(payload).map_err(|err| format!("decode downstream json: {err}"))?;
    if value.get("type").and_then(serde_json::Value::as_str) != Some("network_event") {
        return Ok(false);
    }
    let envelope: NetworkEventEnvelope =
        serde_json::from_value(value).map_err(|err| format!("decode network_event: {err}"))?;
    let session_epoch = current_session_runtime_epoch();
    let session = load_session().map_err(|err| format!("load session for network event: {err}"))?;
    let runtime_active_network_id =
        runtime_network_state_store().read(|state| state.active_network_id.clone());
    if !network_event_targets_session(&envelope, &session, runtime_active_network_id.as_deref()) {
        return Ok(true);
    }
    let local_device_id = session
        .device_id
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .unwrap_or_default()
        .to_string();
    let projection_session = session.clone();
    let projection_device_id = local_device_id.clone();
    let projection_envelope = envelope.clone();
    let apply_result = runtime
        .call_named(
            "network.event.projection",
            Some(envelope.event_id.clone()),
            move |_runtime| {
                apply_network_event_projection(
                    session_epoch,
                    &projection_session,
                    &projection_device_id,
                    &projection_envelope,
                )
            },
        )
        .map_err(|err| format!("apply network event: {err:#}"))?;
    let mut config_version = envelope.version;
    let mut sync_mode = match apply_result {
        ApplyResult::Applied => "event",
        ApplyResult::IgnoredDuplicate | ApplyResult::IgnoredStale => "ignored",
        ApplyResult::NeedsSnapshot => "snapshot",
    };
    log_service_error(format!(
        "client-core-service applied network_event networkId={} eventType={:?} version={} result={:?}",
        envelope.network_id, envelope.event_type, envelope.version, apply_result
    ));
    let client = ControlPlaneClient::from_env();
    if apply_result == ApplyResult::NeedsSnapshot {
        runtime
            .call_named(
                "network.snapshot.mark_syncing",
                Some(envelope.event_id.clone()),
                move |_runtime| mark_network_projection_syncing(session_epoch),
            )
            .map_err(|err| format!("mark network snapshot syncing: {err:#}"))?;
        let snapshot = client
            .network_snapshot(
                session_device_api_token(&session),
                &envelope.network_id,
                &local_device_id,
            )
            .map_err(|err| format!("load network snapshot: {err:#}"))?;
        config_version = snapshot.version;
        sync_mode = "snapshot";
        let snapshot_event_id =
            synthetic_snapshot_event_id("recovery", &snapshot.network_id, snapshot.version);
        let snapshot_envelope = NetworkEventEnvelope {
            r#type: "network_event".to_string(),
            network_id: snapshot.network_id,
            version: snapshot.version,
            event_id: snapshot_event_id,
            event_type: NetworkEventType::NetworkSnapshot,
            occurred_at: current_timestamp_ms(),
            payload: serde_json::to_value(&snapshot.snapshot)
                .map_err(|err| format!("encode network snapshot payload: {err}"))?,
        };
        let projection_session = session.clone();
        let projection_device_id = local_device_id.clone();
        let projection_envelope = snapshot_envelope.clone();
        let snapshot_result = runtime
            .call_named(
                "network.snapshot.projection",
                Some(snapshot_envelope.event_id.clone()),
                move |_runtime| {
                    apply_network_event_projection(
                        session_epoch,
                        &projection_session,
                        &projection_device_id,
                        &projection_envelope,
                    )
                },
            )
            .map_err(|err| format!("apply network snapshot: {err:#}"))?;
        log_service_error(format!(
            "client-core-service applied network snapshot networkId={} version={} result={:?}",
            envelope.network_id, snapshot.version, snapshot_result
        ));
    }
    let current_state = runtime.snapshot().state;
    let reconfigure_required = current_state.signed_in
        && current_state.network_enabled
        && matches!(
            apply_result,
            ApplyResult::Applied | ApplyResult::NeedsSnapshot
        );
    let business_data = network_event_business_data(
        &envelope.event_id,
        &envelope.network_id,
        &envelope.event_type,
        config_version,
        reconfigure_required,
        sync_mode,
    );
    if reconfigure_required {
        {
            let mut queue = task_queue
                .lock()
                .map_err(|_| "control task queue mutex poisoned".to_string())?;
            queue
                .enqueue_downstream_unacked(
                    crate::control_tasks::ControlTaskAction::ReconcileNetworkState,
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
        publish_state_business_event_with_extra(
            state_notifier,
            business_type,
            &state,
            business_data,
        );
    } else {
        publish_state_business_event_with_extra(
            state_notifier,
            BUSINESS_CONTROL_SYNC_CHANGED,
            &current_state,
            business_data,
        );
    }
    Ok(true)
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
        .and_then(serde_json::Value::as_str)
        .unwrap_or_default();
    let device_id = payload
        .and_then(|payload| payload.get("deviceId"))
        .and_then(serde_json::Value::as_str)
        .unwrap_or_default();
    let virtual_ip = payload
        .and_then(|payload| payload.get("virtualIp"))
        .and_then(serde_json::Value::as_str)
        .unwrap_or_default();
    let policy_id = payload
        .and_then(|payload| payload.get("policyId"))
        .and_then(serde_json::Value::as_str)
        .unwrap_or_default();
    let network_id = payload
        .and_then(|payload| payload.get("networkId"))
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
        .and_then(serde_json::Value::as_str)
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .map(str::to_string)
}

fn try_ingest_client_message(
    payload: &[u8],
    runtime: &RuntimeActorHandle,
    state_notifier: &Arc<RuntimeEventHub>,
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
    let session =
        load_session().map_err(|err| format!("load session for client_message: {err}"))?;
    let self_device_id = session
        .device_id
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty());
    if let Some(target_device_id) = message.trimmed_target_device_id() {
        if Some(target_device_id) != self_device_id {
            log_service_error(format!(
                "client-core-service ignored client_message messageId={} fromDeviceId={} targetDeviceId={} selfDeviceId={}",
                message.message_id_str(),
                message.from_device_id_str(),
                target_device_id,
                self_device_id.unwrap_or_default(),
            ));
            return Ok(true);
        }
    }
    log_service_error(format!(
        "client-core-service accepted client_message messageId={} networkId={} fromDeviceId={} targetDeviceId={} bodyBytes={}",
        message.message_id_str(),
        message.network_id_str(),
        message.from_device_id_str(),
        message.target_device_id_str(),
        message.body_len()
    ));
    if maybe_reply_client_ping(&message) {
        return Ok(true);
    }
    let _ = persist_last_client_message_payload(&value);
    let correlation_id = message
        .message_id
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .map(str::to_string);
    let business_message_id = correlation_id.clone();
    let state = runtime
        .call_named(
            "mqtt.client_message.apply",
            correlation_id,
            move |runtime| {
                runtime
                    .dispatch(ClientCommand::ApplyClientMessage(
                        ClientMessageNoticePayload {
                            message_id: message.message_id,
                            from_device_id: message.from_device_id,
                            body: message.body,
                        },
                    ))
                    .map_err(anyhow::Error::from)
            },
        )
        .map_err(|err| format!("apply client_message: {err}"))?;
    publish_state_business_event_with_extra(
        state_notifier,
        BUSINESS_CONTROL_SYNC_CHANGED,
        &state,
        serde_json::json!({
            "messageType": "client_message",
            "messageId": business_message_id,
        }),
    );
    Ok(true)
}

fn maybe_reply_client_ping(message: &ClientMessagePayload) -> bool {
    let Some(from_device_id) = message.trimmed_from_device_id() else {
        return false;
    };
    let Some(target_device_id) = message.trimmed_target_device_id() else {
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
    if local_device_id != target_device_id {
        return true;
    }
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

fn try_ingest_connect_plan(
    payload: &[u8],
    runtime: &RuntimeActorHandle,
) -> Result<Option<ConnectPlanIngest>, String> {
    let value: serde_json::Value =
        serde_json::from_slice(payload).map_err(|err| format!("decode downstream json: {err}"))?;
    if value.get("type").and_then(serde_json::Value::as_str) != Some("connect_plan") {
        return Ok(None);
    }
    let plan = value
        .get("payload")
        .ok_or_else(|| "connect_plan payload is missing".to_string())?;
    let expected_revision = runtime.snapshot().revision;
    let plan = plan.clone();
    let peer_node_id = plan
        .get("peerNodeId")
        .and_then(serde_json::Value::as_str)
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .map(str::to_string);
    let persisted = runtime
        .call_named_if_revision(
            "mqtt.connect_plan.commit",
            peer_node_id,
            expected_revision,
            move |_runtime| crate::persist_connect_plan_from_value(&plan),
        )
        .map_err(|err| format!("persist connect plan: {err:#}"))?
        .unwrap_or(false);
    if persisted {
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

impl ClientMessagePayload {
    fn message_id_str(&self) -> &str {
        self.message_id.as_deref().unwrap_or_default()
    }

    fn network_id_str(&self) -> &str {
        self.network_id.as_deref().unwrap_or_default()
    }

    fn from_device_id_str(&self) -> &str {
        self.from_device_id.as_deref().unwrap_or_default()
    }

    fn target_device_id_str(&self) -> &str {
        self.target_device_id.as_deref().unwrap_or_default()
    }

    fn body_len(&self) -> usize {
        self.body.as_deref().unwrap_or_default().len()
    }

    fn trimmed_from_device_id(&self) -> Option<&str> {
        self.from_device_id
            .as_deref()
            .map(str::trim)
            .filter(|value| !value.is_empty())
    }

    fn trimmed_target_device_id(&self) -> Option<&str> {
        self.target_device_id
            .as_deref()
            .map(str::trim)
            .filter(|value| !value.is_empty())
    }
}

fn try_ingest_relay_candidates_response(
    payload: &[u8],
    runtime: &RuntimeActorHandle,
    state_notifier: &Arc<RuntimeEventHub>,
) -> Result<bool, String> {
    let value: serde_json::Value =
        serde_json::from_slice(payload).map_err(|err| format!("decode downstream json: {err}"))?;
    if value.get("type").and_then(serde_json::Value::as_str) != Some("network_map_response") {
        return Ok(false);
    }
    // The wire message type stays `network_map_response` for now, but the only
    // supported behavior here is refreshing persisted relay candidates.
    let Some(map) = value.pointer("/payload/map") else {
        return Err("network_map_response payload.map is missing".to_string());
    };
    let expected_revision = runtime.snapshot().revision;
    let map = map.clone();
    let count = runtime
        .call_named_if_revision(
            "mqtt.relay_candidates.commit",
            downstream_message_id(&value),
            expected_revision,
            move |_runtime| crate::persist_relay_candidates_from_control_map(&map),
        )
        .map_err(|err| format!("persist relay candidates from control map: {err:#}"))?
        .unwrap_or_default();
    log_service_error(format!(
        "client-core-service refreshed relay candidates from control map: count={count}"
    ));
    let state = runtime.snapshot().state;
    publish_state_business_event(state_notifier, BUSINESS_CONTROL_SYNC_CHANGED, &state);
    Ok(true)
}

fn try_ingest_device_user_login_succeeded(
    payload: &[u8],
    runtime: &RuntimeActorHandle,
    task_queue: &Arc<Mutex<ControlTaskQueue>>,
    state_notifier: &Arc<RuntimeEventHub>,
) -> Result<bool, String> {
    let value: serde_json::Value =
        serde_json::from_slice(payload).map_err(|err| format!("decode downstream json: {err}"))?;
    if value.get("type").and_then(serde_json::Value::as_str) != Some("device_user_login_succeeded")
    {
        return Ok(false);
    }
    let delivery_id = value
        .get("messageId")
        .and_then(serde_json::Value::as_str)
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .ok_or_else(|| "device_user_login_succeeded messageId is missing".to_string())?
        .to_string();
    let Some(auth_value) = value.get("payload").cloned() else {
        return Err("device_user_login_succeeded payload is missing".to_string());
    };
    let auth: AuthPayload = serde_json::from_value(auth_value)
        .map_err(|err| format!("decode device_user_login_succeeded payload: {err}"))?;
    let current_state = runtime.snapshot().state;
    if let Some(expected_device_id) = current_state
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
    let expected_runtime_device_id = current_state.device_id.clone();
    let expected_runtime_signed_in = current_state.signed_in;
    let prepared = prepare_session_from_control_plane(auth)
        .map_err(|error| format!("prepare device user login session: {error:#}"))?;
    let session = prepared.session.clone();
    let command_correlation_id = Some(delivery_id.clone());
    let state = runtime
        .call_named(
            "mqtt.device_user_login.apply",
            command_correlation_id,
            move |runtime| {
                if runtime.state().signed_in != expected_runtime_signed_in
                    || runtime.state().device_id.as_deref().map(str::trim)
                        != expected_runtime_device_id.as_deref().map(str::trim)
                {
                    return Err(anyhow::anyhow!("stale device user login session"));
                }
                persist_session(&session)?;
                apply_prepared_session_projection(current_session_runtime_epoch(), &prepared)?;
                runtime
                    .dispatch(ClientCommand::ApplyDeviceUserLogin(session.into()))
                    .map_err(anyhow::Error::from)
            },
        )
        .map_err(|err| err.to_string())?;
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
                delivery_id.clone(),
                false,
            )
            .map_err(|err| err.to_string())?;
        queue
            .mark_succeeded(&task.id)
            .map_err(|err| err.to_string())?;
    }
    publish_state_business_event_with_extra(
        state_notifier,
        BUSINESS_SESSION_CHANGED,
        &state,
        serde_json::json!({
            "messageType": "device_user_login_succeeded",
            "messageId": delivery_id,
        }),
    );
    Ok(true)
}

fn try_ingest_device_ip_reassigned(
    payload: &[u8],
    runtime: &RuntimeActorHandle,
    state_notifier: &Arc<RuntimeEventHub>,
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
        .and_then(serde_json::Value::as_u64)
        .and_then(|value| u8::try_from(value).ok());
    let assigned_virtual_ip = virtual_ip.to_string();
    let correlation_id = downstream_message_id(&value);
    let business_message_id = correlation_id.clone();
    let assigned_ip = AssignedIpPayload {
        virtual_ip: assigned_virtual_ip,
        prefix_len,
    };
    let state =
        crate::execute_runtime_assigned_ip(runtime, "mqtt.ip.sync", correlation_id, assigned_ip)
            .map_err(|err| err.to_string())?;
    if let Some(error) = state.error {
        return Err(error);
    }
    publish_state_business_event_with_extra(
        state_notifier,
        BUSINESS_CONTROL_SYNC_CHANGED,
        &state,
        serde_json::json!({
            "messageType": "device_ip_reassigned",
            "messageId": business_message_id,
            "deviceId": target_device_id,
            "virtualIp": virtual_ip,
            "prefixLen": prefix_len,
        }),
    );
    Ok(true)
}

fn build_outbox_messages(
    session: &PersistedSession,
    runtime: &RuntimeActorHandle,
    task_queue: &Arc<Mutex<ControlTaskQueue>>,
    include_heartbeat: bool,
    include_runtime_state: bool,
    include_path_health: bool,
    include_control_acks: bool,
) -> Vec<ControlTransportMessage> {
    let state = runtime.snapshot().state;
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
        log_service_error(format!(
            "client-core-service outbox publish start id={} kind={:?} topic={} summary={}",
            message.id,
            message.kind,
            message.topic,
            outbox_message_summary(&message.payload),
        ));
        if let Err(error) = publish_outbox_message(&session, &message)
            .and_then(|_| mark_transport_published(&message, &task_queue))
        {
            log_service_error(format!(
                "client-core-service outbox publish failed id={} topic={} error={}",
                message.id, message.topic, error
            ));
        } else {
            log_service_error(format!(
                "client-core-service outbox publish ok id={} kind={:?} topic={}",
                message.id, message.kind, message.topic
            ));
        }
    });
}

fn outbox_message_summary(value: &serde_json::Value) -> String {
    let message_type = value
        .get("type")
        .and_then(serde_json::Value::as_str)
        .unwrap_or_default();
    let network_id = value
        .get("networkId")
        .and_then(serde_json::Value::as_str)
        .or_else(|| {
            value
                .get("payload")
                .and_then(|payload| payload.get("networkId"))
                .and_then(serde_json::Value::as_str)
        })
        .unwrap_or_default();
    let node_id = value
        .get("payload")
        .and_then(|payload| payload.get("nodeId"))
        .and_then(serde_json::Value::as_str)
        .unwrap_or_default();
    let peer_node_id = value
        .get("payload")
        .and_then(|payload| payload.get("peerNodeId"))
        .and_then(serde_json::Value::as_str)
        .unwrap_or_default();
    let endpoint = value
        .get("payload")
        .and_then(|payload| payload.get("endpoint"))
        .and_then(serde_json::Value::as_str)
        .unwrap_or_default();
    let relay_transport = value
        .get("payload")
        .and_then(|payload| payload.get("relayTransport"))
        .and_then(serde_json::Value::as_str)
        .unwrap_or_default();
    format!(
        "type={} networkId={} nodeId={} peerNodeId={} endpoint={} relayTransport={}",
        message_type, network_id, node_id, peer_node_id, endpoint, relay_transport
    )
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
        mqtt.map(|value| value.broker_url.as_str())
            .unwrap_or_default(),
        mqtt.map(|value| value.client_id.as_str())
            .unwrap_or_default(),
        mqtt.map(|value| value.topic_prefix.as_str())
            .unwrap_or_default(),
    )
}

fn mqtt_connection_matches(left: &PersistedSession, right: &PersistedSession) -> bool {
    if left.session_kind != right.session_kind
        || left.user_id != right.user_id
        || left.device_id != right.device_id
    {
        return false;
    }
    match (left.mqtt.as_ref(), right.mqtt.as_ref()) {
        (Some(left), Some(right)) => {
            left.broker_url == right.broker_url
                && left.client_id == right.client_id
                && left.username == right.username
                && left.password == right.password
                && left.topic_prefix == right.topic_prefix
        }
        (None, None) => true,
        _ => false,
    }
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
    use std::fs;
    use std::sync::{Arc, Mutex};

    use client_core::{AuthPayload, ClientCommand, ClientRuntime, NetworkRuntimeState};
    use client_core_platform::PlatformNetworkImpl;

    use super::{
        ingest_downstream_publish, mqtt_connection_matches, reconcile_active_network_state,
        reconnect_key, try_ingest_device_ip_reassigned,
    };
    use crate::control_plane::MqttCredential;
    use crate::network_event::{
        network_event_targets_session, network_event_topics_for_session, NetworkEventEnvelope,
        NetworkEventType,
    };
    use crate::runtime_actor::RuntimeActorHandle;
    use crate::runtime_event_hub::RuntimeEventHub;
    use crate::session_store::PersistedSession;
    use crate::{
        control_tasks::ControlTaskQueue, BUSINESS_CONTROL_SYNC_CHANGED,
        BUSINESS_NETWORK_RUNTIME_CHANGED, BUSINESS_NETWORK_SWITCH_FAILED,
    };

    fn test_runtime() -> RuntimeActorHandle {
        RuntimeActorHandle::spawn(ClientRuntime::new(PlatformNetworkImpl))
    }

    #[test]
    fn subscribes_all_session_network_topics() {
        let mut session = PersistedSession::prelogin(
            "device-1",
            Some(MqttCredential {
                broker_url: "mqtt://127.0.0.1:1883".to_string(),
                client_id: "device-1".to_string(),
                username: "device-1".to_string(),
                password: "secret".to_string(),
                topic_prefix: "slan/devices/device-1".to_string(),
                expires_at: None,
            }),
        );
        session.network_ids = vec!["network-b".to_string(), "network-a".to_string()];
        session.active_network_id = Some("network-a".to_string());

        assert_eq!(
            network_event_topics_for_session(&session),
            vec![
                "slan/networks/network-a/broadcast".to_string(),
                "slan/networks/network-b/broadcast".to_string(),
            ]
        );
    }

    #[test]
    fn network_membership_change_keeps_the_same_worker_connection() {
        let mut before = PersistedSession::prelogin(
            "device-1",
            Some(MqttCredential {
                broker_url: "mqtt://127.0.0.1:1883".to_string(),
                client_id: "device-1".to_string(),
                username: "device-1".to_string(),
                password: "secret".to_string(),
                topic_prefix: "slan/devices/device-1".to_string(),
                expires_at: None,
            }),
        );
        before.network_ids = vec!["network-a".to_string()];
        before.active_network_id = Some("network-a".to_string());
        let mut after = before.clone();
        after.network_ids.push("network-b".to_string());
        after.active_network_id = Some("network-b".to_string());

        assert_eq!(reconnect_key(&before), reconnect_key(&after));
        assert!(mqtt_connection_matches(&before, &after));
    }

    #[test]
    fn mqtt_credential_change_requires_a_new_connection() {
        let before = PersistedSession::prelogin(
            "device-1",
            Some(MqttCredential {
                broker_url: "mqtt://127.0.0.1:1883".to_string(),
                client_id: "device-1".to_string(),
                username: "device-1".to_string(),
                password: "secret-1".to_string(),
                topic_prefix: "slan/devices/device-1".to_string(),
                expires_at: None,
            }),
        );
        let mut after = before.clone();
        after.mqtt.as_mut().expect("mqtt credential").password = "secret-2".to_string();

        assert!(!mqtt_connection_matches(&before, &after));
    }

    #[test]
    fn accepts_events_for_any_session_network() {
        let mut session = PersistedSession::empty();
        session.active_network_id = Some("network-a".to_string());
        session.network_ids = vec!["network-a".to_string(), "network-b".to_string()];
        let envelope = NetworkEventEnvelope {
            r#type: "network_event".to_string(),
            network_id: "network-b".to_string(),
            version: 1,
            event_id: "event-1".to_string(),
            event_type: NetworkEventType::NetworkConfigChanged,
            occurred_at: 1,
            payload: serde_json::json!({}),
        };

        assert!(network_event_targets_session(
            &envelope,
            &session,
            Some("network-a")
        ));
    }

    #[test]
    fn client_message_downstream_updates_runtime_state_and_notifies() {
        let _lock = crate::test_env_lock();
        let state_dir = std::env::temp_dir().join(format!(
            "slan-client-message-target-test-{}",
            std::process::id()
        ));
        let _ = std::fs::remove_dir_all(&state_dir);
        std::fs::create_dir_all(&state_dir).expect("create temp state dir");
        let previous_state_dir = std::env::var_os("SLAN_STATE_DIR");
        std::env::set_var("SLAN_STATE_DIR", &state_dir);
        let mut session = PersistedSession::empty();
        session.device_id = Some("ios-device".to_string());
        crate::session_store::persist_session(&session).expect("persist test session");
        let runtime = test_runtime();
        let task_queue = Arc::new(Mutex::new(ControlTaskQueue::load_default()));
        let state_notifier = Arc::new(RuntimeEventHub::default());
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
        ingest_downstream_publish(&payload, &runtime, &task_queue, &state_notifier)
            .expect("ingest duplicate client message");

        let state = runtime.snapshot().state;
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

        assert_eq!(state_notifier.latest_revision(), 1);
        assert_eq!(state_notifier.diagnostics().duplicate_suppressed_total, 1);
        let event = state_notifier.next_after(0).expect("business event");
        assert_eq!(event.business_type, BUSINESS_CONTROL_SYNC_CHANGED);
        assert_eq!(event.event_id, "client-msg-1");
        assert_eq!(
            event
                .business_data
                .get("lastClientMessageBody")
                .and_then(serde_json::Value::as_str),
            Some("hello ios")
        );
        if let Some(value) = previous_state_dir {
            std::env::set_var("SLAN_STATE_DIR", value);
        } else {
            std::env::remove_var("SLAN_STATE_DIR");
        }
        let _ = std::fs::remove_dir_all(&state_dir);
    }

    #[test]
    fn client_message_downstream_ignores_other_targets() {
        let _lock = crate::test_env_lock();
        let state_dir = std::env::temp_dir().join(format!(
            "slan-client-message-ignore-test-{}",
            std::process::id()
        ));
        let _ = std::fs::remove_dir_all(&state_dir);
        std::fs::create_dir_all(&state_dir).expect("create temp state dir");
        let previous_state_dir = std::env::var_os("SLAN_STATE_DIR");
        std::env::set_var("SLAN_STATE_DIR", &state_dir);
        let mut session = PersistedSession::empty();
        session.device_id = Some("ios-device".to_string());
        crate::session_store::persist_session(&session).expect("persist test session");

        let runtime = test_runtime();
        let task_queue = Arc::new(Mutex::new(ControlTaskQueue::load_default()));
        let state_notifier = Arc::new(RuntimeEventHub::default());
        let payload = serde_json::json!({
            "type": "client_message",
            "messageId": "envelope-msg-2",
            "payload": {
                "messageId": "client-msg-2",
                "networkId": "net-1",
                "fromDeviceId": "mac-device",
                "targetDeviceId": "android-device",
                "body": "hello android"
            }
        });
        let payload = serde_json::to_vec(&payload).expect("encode payload");

        ingest_downstream_publish(&payload, &runtime, &task_queue, &state_notifier)
            .expect("ingest client message");

        let state = runtime.snapshot().state;
        assert_eq!(state.last_client_message_id, None);
        assert_eq!(state.last_client_message_from_device_id, None);
        assert_eq!(state.last_client_message_body, None);

        if let Some(value) = previous_state_dir {
            std::env::set_var("SLAN_STATE_DIR", value);
        } else {
            std::env::remove_var("SLAN_STATE_DIR");
        }
        let _ = std::fs::remove_dir_all(&state_dir);
    }

    #[test]
    fn client_message_publish_uses_target_device_downstream_topic() {
        let mqtt = MqttCredential {
            broker_url: "mqtt://47.245.40.231:1883".to_string(),
            client_id: "slan-device-ios-device".to_string(),
            username: "device:ios-device:4102444800".to_string(),
            password: "secret".to_string(),
            topic_prefix: "slan/devices/ios-device".to_string(),
            expires_at: Some(4_102_444_800),
        };

        let response = crate::client_message_mqtt::publish_client_message_topic_for_test(
            &mqtt,
            "android-device",
        );

        assert_eq!(response, "slan/devices/android-device/control/down");
    }

    #[test]
    fn device_user_login_downstream_rejects_mismatched_device_id() {
        let runtime = test_runtime();
        runtime
            .call(|runtime| {
                runtime
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
                Ok(())
            })
            .expect("seed runtime");
        let task_queue = Arc::new(Mutex::new(ControlTaskQueue::load_default()));
        let state_notifier = Arc::new(RuntimeEventHub::default());
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
        let state = runtime.snapshot().state;
        assert_eq!(state.device_id.as_deref(), Some("dev-1"));
        assert_eq!(state.user_label.as_deref(), Some("user@example.com"));
    }

    #[test]
    fn network_event_rejects_stale_network() {
        let mut session = PersistedSession::empty();
        session.device_id = Some("dev-current".to_string());
        session.active_network_id = Some("net-current".to_string());

        let current = NetworkEventEnvelope {
            r#type: "network_event".to_string(),
            network_id: "net-current".to_string(),
            version: 1,
            event_id: "evt-1".to_string(),
            event_type: NetworkEventType::ResolverChanged,
            occurred_at: 1,
            payload: serde_json::json!({}),
        };
        assert!(network_event_targets_session(&current, &session, None));

        let stale_network = NetworkEventEnvelope {
            network_id: "net-old".to_string(),
            event_id: "evt-2".to_string(),
            ..current.clone()
        };
        assert!(!network_event_targets_session(
            &stale_network,
            &session,
            None
        ));

        let peer_device = NetworkEventEnvelope {
            network_id: "net-current".to_string(),
            event_id: "evt-3".to_string(),
            ..current
        };
        assert!(network_event_targets_session(&peer_device, &session, None));
    }

    #[test]
    fn network_event_targets_runtime_network_when_session_is_empty() {
        let mut session = PersistedSession::empty();
        session.device_id = Some("dev-current".to_string());

        let current = NetworkEventEnvelope {
            r#type: "network_event".to_string(),
            network_id: "net-current".to_string(),
            version: 1,
            event_id: "evt-1".to_string(),
            event_type: NetworkEventType::ResolverChanged,
            occurred_at: 1,
            payload: serde_json::json!({}),
        };

        assert!(network_event_targets_session(
            &current,
            &session,
            Some("net-current"),
        ));
        assert!(!network_event_targets_session(
            &current,
            &session,
            Some("net-other"),
        ));
    }

    #[test]
    fn network_event_publish_includes_sync_metadata_without_reconfigure() {
        let _lock = crate::test_env_lock();
        let state_dir = std::env::temp_dir().join(format!(
            "slan-network-event-metadata-test-{}",
            std::process::id()
        ));
        let _ = std::fs::remove_dir_all(&state_dir);
        std::fs::create_dir_all(&state_dir).expect("create temp state dir");
        let previous_state_dir = std::env::var_os("SLAN_STATE_DIR");
        std::env::set_var("SLAN_STATE_DIR", &state_dir);

        let mut session = PersistedSession::empty();
        session.device_id = Some("dev-1".to_string());
        session.active_network_id = Some("net-1".to_string());
        crate::session_store::persist_session(&session).expect("persist session");

        let runtime = test_runtime();
        let task_queue = Arc::new(Mutex::new(ControlTaskQueue::load_default()));
        let state_notifier = Arc::new(RuntimeEventHub::default());
        let payload = serde_json::json!({
            "type": "network_event",
            "networkId": "net-1",
            "version": 1,
            "eventId": "evt-1",
            "eventType": "resolver_changed",
            "occurredAt": 1,
            "payload": {
                "records": []
            }
        });
        let payload = serde_json::to_vec(&payload).expect("encode payload");

        ingest_downstream_publish(&payload, &runtime, &task_queue, &state_notifier)
            .expect("ingest network event");

        let event = state_notifier.next_after(0).expect("business event");
        assert_eq!(event.business_type, BUSINESS_CONTROL_SYNC_CHANGED);
        assert_eq!(
            event
                .business_data
                .get("messageType")
                .and_then(serde_json::Value::as_str),
            Some("network_event")
        );
        assert_eq!(
            event
                .business_data
                .get("networkId")
                .and_then(serde_json::Value::as_str),
            Some("net-1")
        );
        assert_eq!(
            event
                .business_data
                .get("reconfigureRequired")
                .and_then(serde_json::Value::as_bool),
            Some(false)
        );
        assert_eq!(
            event
                .business_data
                .get("syncMode")
                .and_then(serde_json::Value::as_str),
            Some("event")
        );

        if let Some(value) = previous_state_dir {
            std::env::set_var("SLAN_STATE_DIR", value);
        } else {
            std::env::remove_var("SLAN_STATE_DIR");
        }
        let _ = std::fs::remove_dir_all(&state_dir);
    }

    #[test]
    fn device_ip_reassigned_publish_includes_device_metadata() {
        let _lock = crate::test_env_lock();
        let state_dir = std::env::temp_dir().join(format!(
            "slan-device-ip-reassigned-metadata-test-{}",
            std::process::id()
        ));
        let _ = std::fs::remove_dir_all(&state_dir);
        std::fs::create_dir_all(&state_dir).expect("create temp state dir");
        let previous_state_dir = std::env::var_os("SLAN_STATE_DIR");
        std::env::set_var("SLAN_STATE_DIR", &state_dir);

        let mut session = PersistedSession::empty();
        session.device_id = Some("dev-1".to_string());
        session.active_network_id = Some("net-1".to_string());
        crate::session_store::persist_session(&session).expect("persist session");

        let runtime = test_runtime();
        let state_notifier = Arc::new(RuntimeEventHub::default());
        let payload = serde_json::json!({
            "type": "device_ip_reassigned",
            "messageId": "ip-msg-1",
            "payload": {
                "deviceId": "dev-1",
                "networkId": "net-1",
                "virtualIp": "10.0.0.9",
                "prefixLen": 32
            }
        });
        let payload = serde_json::to_vec(&payload).expect("encode payload");

        let accepted = try_ingest_device_ip_reassigned(&payload, &runtime, &state_notifier)
            .expect("ingest device ip reassigned");
        assert!(accepted);

        let event = state_notifier.next_after(0).expect("business event");
        assert_eq!(event.business_type, BUSINESS_CONTROL_SYNC_CHANGED);
        assert_eq!(
            event
                .business_data
                .get("messageType")
                .and_then(serde_json::Value::as_str),
            Some("device_ip_reassigned")
        );
        assert_eq!(
            event
                .business_data
                .get("deviceId")
                .and_then(serde_json::Value::as_str),
            Some("dev-1")
        );
        assert_eq!(
            event
                .business_data
                .get("virtualIp")
                .and_then(serde_json::Value::as_str),
            Some("10.0.0.9")
        );

        if let Some(value) = previous_state_dir {
            std::env::set_var("SLAN_STATE_DIR", value);
        } else {
            std::env::remove_var("SLAN_STATE_DIR");
        }
        let _ = std::fs::remove_dir_all(&state_dir);
    }

    #[test]
    fn device_network_disabled_publish_includes_device_metadata() {
        let _lock = crate::test_env_lock();
        let state_dir = std::env::temp_dir().join(format!(
            "slan-device-disabled-metadata-test-{}",
            std::process::id()
        ));
        let _ = std::fs::remove_dir_all(&state_dir);
        std::fs::create_dir_all(&state_dir).expect("create temp state dir");
        let previous_state_dir = std::env::var_os("SLAN_STATE_DIR");
        std::env::set_var("SLAN_STATE_DIR", &state_dir);

        let mut session = PersistedSession::empty();
        session.device_id = Some("dev-1".to_string());
        session.active_network_id = Some("net-1".to_string());
        crate::session_store::persist_session(&session).expect("persist session");

        let runtime = test_runtime();
        let task_queue = Arc::new(Mutex::new(ControlTaskQueue::load_default()));
        let state_notifier = Arc::new(RuntimeEventHub::default());
        let payload = serde_json::json!({
            "type": "device_network_disabled",
            "messageId": "disable-msg-1",
            "payload": {
                "deviceId": "dev-1",
                "networkId": "net-1",
                "attachmentId": "att-1"
            }
        });
        let payload = serde_json::to_vec(&payload).expect("encode payload");

        let _ = ingest_downstream_publish(&payload, &runtime, &task_queue, &state_notifier);

        let event = state_notifier.next_after(0).expect("business event");
        assert_eq!(event.business_type, BUSINESS_NETWORK_RUNTIME_CHANGED);
        assert_eq!(
            event
                .business_data
                .get("messageType")
                .and_then(serde_json::Value::as_str),
            Some("device_network_disabled")
        );
        assert_eq!(
            event
                .business_data
                .get("deviceId")
                .and_then(serde_json::Value::as_str),
            Some("dev-1")
        );
        assert_eq!(
            event
                .business_data
                .get("networkId")
                .and_then(serde_json::Value::as_str),
            Some("net-1")
        );
        assert_eq!(
            event
                .business_data
                .get("reconfigureRequired")
                .and_then(serde_json::Value::as_bool),
            Some(true)
        );

        if let Some(value) = previous_state_dir {
            std::env::set_var("SLAN_STATE_DIR", value);
        } else {
            std::env::remove_var("SLAN_STATE_DIR");
        }
        let _ = std::fs::remove_dir_all(&state_dir);
    }

    #[test]
    fn active_network_reconcile_enqueues_reconcile_task_when_network_is_enabled() {
        let _lock = crate::test_env_lock();
        let state_dir = std::env::temp_dir().join(format!(
            "slan-active-network-reconcile-test-{}",
            std::process::id()
        ));
        let _ = fs::remove_dir_all(&state_dir);
        fs::create_dir_all(&state_dir).expect("create temp state dir");
        let previous_state_dir = std::env::var_os("SLAN_STATE_DIR");
        std::env::set_var("SLAN_STATE_DIR", &state_dir);

        let runtime = test_runtime();
        runtime
            .call(|runtime| {
                runtime
                    .dispatch(ClientCommand::ApplyDeviceUserLogin(AuthPayload {
                        access_token: "token-1".to_string(),
                        refresh_token: None,
                        user_id: "user-1".to_string(),
                        user_label: "user@example.com".to_string(),
                        device_id: Some("device-1".to_string()),
                        active_network_id: Some("net-1".to_string()),
                        virtual_ip: Some("10.0.0.2".to_string()),
                        expires_in: None,
                    }))
                    .expect("seed signed-in runtime");
                runtime
                    .dispatch(ClientCommand::ApplyPlatformRuntimeState(
                        NetworkRuntimeState {
                            adapter_present: true,
                            network_enabled: true,
                            virtual_ip: Some("10.0.0.2".to_string()),
                            active_path: None,
                            peer_paths: Vec::new(),
                        },
                    ))
                    .expect("seed enabled network state");
                Ok(())
            })
            .expect("seed runtime");
        let task_queue = Arc::new(Mutex::new(ControlTaskQueue::load_default()));
        let state_notifier = Arc::new(RuntimeEventHub::default());

        let _ = reconcile_active_network_state(&runtime, &task_queue, &state_notifier);

        assert_eq!(state_notifier.latest_revision(), 1);
        let event = state_notifier.next_after(0).expect("business event");
        assert_eq!(event.business_type, BUSINESS_NETWORK_SWITCH_FAILED);

        if let Some(value) = previous_state_dir {
            std::env::set_var("SLAN_STATE_DIR", value);
        } else {
            std::env::remove_var("SLAN_STATE_DIR");
        }
        let _ = fs::remove_dir_all(&state_dir);
    }

    #[test]
    fn active_network_reconcile_skips_when_network_is_disabled() {
        let runtime = test_runtime();
        let task_queue = Arc::new(Mutex::new(ControlTaskQueue::load_default()));
        let state_notifier = Arc::new(RuntimeEventHub::default());

        reconcile_active_network_state(&runtime, &task_queue, &state_notifier)
            .expect("disabled network should not reconcile");

        let mut queue = task_queue.lock().expect("task queue mutex");
        let task = queue.take_next_pending().expect("read pending task");
        assert!(task.is_none(), "disabled network must not enqueue refresh");
    }
}
