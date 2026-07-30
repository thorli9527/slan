use std::{
    collections::BTreeSet,
    fs,
    path::PathBuf,
    sync::{
        atomic::{AtomicU64, Ordering},
        Mutex, OnceLock,
    },
    thread,
    time::Duration,
};

use anyhow::{Context, Result};
use client_core::{
    relay_path_kind_for_transport, AssignedIpPayload, AuthPayload, ClientCommand,
    ClientMessageNoticePayload, ClientRuntime, ClientViewState, PathCandidate, PathKind, PathState,
    PeerPathConfig, PlatformAclPolicy, PlatformResolverConfig, RelayDataPlaneConfig,
    RelayPeerSession, RouteSpec, SLAN_DNS_SERVICE_IP,
};
use client_core_platform::{
    direct_udp::configured_direct_udp_port, set_platform_log_dir, PlatformNetworkImpl,
};
use control_mqtt_client::{ThinControlMqttClient, ThinMqttCredential, ThinMqttQoS};
use serde_json::Value;

use crate::{
    acl_policy::{acl_policies_for_network, platform_acl_policies},
    client_message_mqtt,
    control_plane::{
        local_stable_device_id, set_client_device_id_override, set_control_base_url_override,
        ControlPlaneClient, RelayCandidate,
    },
    control_transport::{self, ControlTransportMessage, MqttQos},
    local_api::{
        request_correlation_id, response_with_correlation_id, LocalResolverResolveRequest,
        LocalServiceMethod, PlatformRuntimeStateReportRequest, RegisterTestUserRequest,
        SendClientMessageRequest, ServiceRequest, WatchBusinessEventRequest,
        WatchBusinessEventResponse, WatchStateRequest, WatchStateResponse,
        BUSINESS_CONTROL_SYNC_CHANGED, BUSINESS_NETWORK_RUNTIME_CHANGED, BUSINESS_SESSION_CHANGED,
        BUSINESS_STATE_CHANGED,
    },
    mobile_platform_config::{MobilePlatformDeviceNetworkConfig, MobilePlatformNetworkConfig},
    network_event::{
        apply_device_network_membership, network_event_business_data,
        network_event_targets_session, network_event_topics_for_session,
        synthetic_snapshot_event_id, NetworkEventEnvelope,
    },
    network_event_apply::ApplyResult,
    network_event_projection::{
        apply_network_event_projection, apply_prepared_session_projection,
        mark_network_projection_syncing,
    },
    network_runtime_state::runtime_network_state_store,
    platform_runtime_report::{platform_traffic_stats_payload, report_platform_runtime},
    relay_candidates::{runtime_relay_candidates, select_relay_candidates},
    relay_models::{PersistedRelayCandidate, RelayCandidateListResponse},
    relay_store::relay_only_path_policy_enabled,
    resolver_authority::{resolve_authoritative, resolve_authoritative_result_json},
    resolver_runtime_state::resolver_runtime_state,
    runtime_actor::RuntimeActorHandle,
    session_store::{
        app_data_dir, current_session_runtime_epoch, current_timestamp_ms,
        ensure_session_node_binding, load_session, load_valid_registered_session,
        lock_session_runtime_epoch, persist_session, prepare_client_login_session,
        prepare_session_device_registered, prepare_session_from_control_plane,
        refresh_session_runtime_endpoints, remove_session, report_runtime_state,
        session_device_api_token, set_app_data_dir_override, PersistedSession, PreparedSession,
    },
};

static RUNTIME: OnceLock<RuntimeActorHandle> = OnceLock::new();
static MQTT: OnceLock<Mutex<Option<EmbeddedMqttConnection>>> = OnceLock::new();
static MQTT_LAST_ERROR: OnceLock<Mutex<Option<String>>> = OnceLock::new();
static EMBEDDED_CONTROL_BASE_URL: OnceLock<Mutex<Option<String>>> = OnceLock::new();
static NEXT_EMBEDDED_MQTT_GENERATION: AtomicU64 = AtomicU64::new(1);

const EMBEDDED_MQTT_KEEPALIVE_PING_INTERVAL_MS: u64 = 15_000;
const EMBEDDED_ACTIVE_NETWORK_RECONCILE_INTERVAL_MS: u64 = 10_000;
const EMBEDDED_NETWORK_SUBSCRIBE_RETRY_INTERVAL_MS: u64 = 2_000;
const EMBEDDED_MQTT_RECONNECT_AFTER_SESSION_REFRESH_MS: u64 = 10 * 60 * 1000;

struct EmbeddedMqttConnection {
    generation: u64,
    device_id: Option<String>,
    downstream_topic: String,
    network_event_topics: Vec<String>,
    connected: bool,
    network_event_subscribed: bool,
    last_message_topic: Option<String>,
    last_message_type: Option<String>,
    last_error: Option<String>,
}

fn mqtt_connection() -> &'static Mutex<Option<EmbeddedMqttConnection>> {
    MQTT.get_or_init(|| Mutex::new(None))
}

fn mqtt_last_error_store() -> &'static Mutex<Option<String>> {
    MQTT_LAST_ERROR.get_or_init(|| Mutex::new(None))
}

fn next_embedded_mqtt_generation() -> u64 {
    NEXT_EMBEDDED_MQTT_GENERATION.fetch_add(1, Ordering::Relaxed)
}

fn embedded_mqtt_generation_active(
    generation: u64,
    device_id: &Option<String>,
    downstream_topic: &str,
) -> bool {
    let guard = mqtt_connection()
        .lock()
        .expect("embedded mqtt mutex poisoned");
    guard.as_ref().is_some_and(|connection| {
        connection.generation == generation
            && connection.device_id == *device_id
            && connection.downstream_topic == downstream_topic
    })
}

fn clear_embedded_mqtt_connection() {
    let mut guard = mqtt_connection()
        .lock()
        .expect("embedded mqtt mutex poisoned");
    *guard = None;
}

fn runtime() -> &'static RuntimeActorHandle {
    RUNTIME.get_or_init(|| {
        crate::diagnostic_upload::spawn_worker();
        let mut runtime = ClientRuntime::new(PlatformNetworkImpl);
        if let Some(session) = load_valid_registered_session() {
            let _ = runtime.dispatch(ClientCommand::ApplyDeviceUserLogin(session.into()));
        }
        RuntimeActorHandle::spawn(runtime)
    })
}

pub fn embedded_handle_request_json(request_json: &str) -> String {
    match handle_request_json(request_json) {
        Ok(response) => response,
        Err(error) => serde_json::json!({ "error": format!("{error:#}") }).to_string(),
    }
}

fn embedded_relay_candidates_response(refresh: bool) -> Result<RelayCandidateListResponse> {
    let mut session = ensure_device_session().context("ensure embedded session")?;
    let expected_snapshot = runtime().snapshot();
    let network_id = embedded_active_network_id(&mut session)?;
    let refreshed_candidates = if refresh {
        prepare_embedded_relay_candidates_for_session(&session, &network_id)?
    } else {
        Vec::new()
    };
    let refreshed = !refreshed_candidates.is_empty();
    let relay_candidates = if refreshed {
        refreshed_candidates
    } else {
        runtime_relay_candidates()
    };
    let selections = select_relay_candidates(&relay_candidates);
    let best = selections.iter().find(|item| item.selected).cloned();
    let response = RelayCandidateListResponse {
        network_id: Some(network_id),
        refreshed,
        candidates: selections,
        best,
    };
    let mut prepared = PreparedSession::from_session(session);
    prepared.relay_candidates = relay_candidates;
    runtime()
        .call_named_if_revision(
            "embedded.relay.candidates.commit",
            prepared.session.device_id.clone(),
            expected_snapshot.revision,
            move |runtime| {
                commit_embedded_prepared_login(
                    runtime,
                    &prepared,
                    expected_snapshot.state.device_id.as_deref(),
                    expected_snapshot.state.signed_in,
                )?;
                Ok(())
            },
        )?
        .ok_or_else(|| anyhow::anyhow!("stale embedded relay candidates"))?;
    Ok(response)
}

fn embedded_active_network_id(session: &mut PersistedSession) -> Result<String> {
    session.active_network_id = session
        .active_network_id
        .take()
        .and_then(|value| non_empty_embedded_network_id(&value));
    if session.active_network_id.is_none() {
        session.active_network_id = crate::network_module::network_module_snapshot()
            .configs
            .into_iter()
            .find_map(|config| non_empty_embedded_network_id(&config.network_id));
        if session.active_network_id.is_none() {
            let client = ControlPlaneClient::from_env();
            session.active_network_id =
                client.active_network_id(session_device_api_token(session))?;
        }
    }
    Ok(session
        .active_network_id
        .clone()
        .filter(|value| !value.trim().is_empty())
        .unwrap_or_default()) // Return empty string if no network (allows enabling client without peers)
}

fn non_empty_embedded_network_id(value: &str) -> Option<String> {
    let value = value.trim();
    (!value.is_empty()).then(|| value.to_string())
}

fn prepare_embedded_relay_candidates_for_session(
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
    Ok(candidates
        .iter()
        .map(embedded_persisted_relay_candidate)
        .collect())
}

fn embedded_persisted_relay_candidate(candidate: &RelayCandidate) -> PersistedRelayCandidate {
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

fn embedded_relay_candidate_from_selection(
    selection: &crate::relay_models::RelayCandidateSelection,
) -> RelayCandidate {
    let transport =
        client_core::normalize_relay_transport(selection.transport.as_str()).unwrap_or("udp");
    RelayCandidate {
        endpoint_id: selection.endpoint_id.clone(),
        transport: selection.transport.clone(),
        address: relay_address_for_transport(transport, selection.address.as_str()),
        country_code: selection.country_code.clone(),
        region_id: selection.region_id.clone(),
        cluster_id: selection.cluster_id.clone(),
        reachable: selection.reachable,
        observed_rtt_ms: selection.rtt_ms,
        path_score: Some(selection.path_score),
        selected: selection.selected,
    }
}

fn normalize_embedded_relay_ticket_for_candidate(
    ticket: &mut client_core::RelayTicket,
    candidate: &RelayCandidate,
) {
    let transport =
        client_core::normalize_relay_transport(candidate.transport.as_str()).unwrap_or("udp");
    if transport == "derp_tcp_tls_443" {
        if ticket.allowed_derp_node_ids.is_empty() {
            ticket
                .allowed_derp_node_ids
                .push(candidate.endpoint_id.clone());
        }
        ticket.relay_url = relay_address_for_transport(transport, candidate.address.as_str());
    }
}

fn relay_address_for_transport(transport: &str, address: &str) -> String {
    let trimmed = address.trim();
    if transport == "derp_tcp_tls_443"
        && !trimmed.starts_with("derp://")
        && !trimmed.starts_with("derp+tcp+tls://")
        && !trimmed.starts_with("derp_tcp_tls_443://")
    {
        return format!("derp://{trimmed}");
    }
    trimmed.to_string()
}

fn handle_request_json(request_json: &str) -> Result<String> {
    let request: ServiceRequest = serde_json::from_str(request_json).context("decode request")?;
    apply_embedded_request_overrides(&request.args);
    let correlation_id = request_correlation_id(&request.args);
    let response = match LocalServiceMethod::parse(&request.method) {
        LocalServiceMethod::Start
        | LocalServiceMethod::LocalState
        | LocalServiceMethod::Refresh => {
            serde_json::to_string(&refresh_state_json()).context("encode state")
        }
        LocalServiceMethod::LocalStateWatch => {
            let input: WatchStateRequest =
                serde_json::from_value(request.args).context("decode watch state request")?;
            serde_json::to_string(&WatchStateResponse {
                revision: input.last_revision,
                state: refresh_state(),
            })
            .context("encode watch state response")
        }
        LocalServiceMethod::LocalBusinessEventWatch => {
            let input: WatchBusinessEventRequest = serde_json::from_value(request.args)
                .context("decode watch business event request")?;
            serde_json::to_string(&watch_embedded_business_event(
                input.last_revision,
                input.follow_latest,
                input.stream_id.as_deref(),
                Duration::from_millis(input.timeout_ms.clamp(1_000, 60_000)),
            ))
            .context("encode watch business event response")
        }
        LocalServiceMethod::LocalSession => local_session_json(),
        LocalServiceMethod::LocalNetworkModule => {
            serde_json::to_string(&crate::network_module::network_module_snapshot())
                .context("encode local network module")
        }
        LocalServiceMethod::LocalResolverState => {
            serde_json::to_string(&local_resolver_state_json())
                .context("encode local resolver state")
        }
        LocalServiceMethod::LocalResolverResolve => {
            let input: LocalResolverResolveRequest = serde_json::from_value(request.args)
                .context("decode local resolver resolve request")?;
            serde_json::to_string(&local_resolver_resolve_json(input))
                .context("encode local resolver resolve response")
        }
        LocalServiceMethod::LocalRelayCandidates => {
            serde_json::to_string(&embedded_relay_candidates_response(false)?)
                .context("encode relay candidates")
        }
        LocalServiceMethod::LocalRefreshRelayCandidates => {
            serde_json::to_string(&embedded_relay_candidates_response(true)?)
                .context("encode relay candidates")
        }
        LocalServiceMethod::LocalSetRelayTransportAllowlist => {
            let input: crate::local_api::SetRelayTransportAllowlistRequest =
                serde_json::from_value(request.args)
                    .context("decode relay transport allowlist request")?;
            let allowlist = crate::relay_candidates::set_test_relay_transport_allowlist(
                (!input.transports.is_empty()).then_some(input.transports),
            );
            serde_json::to_string(&serde_json::json!({
                "transports": allowlist.unwrap_or_default(),
            }))
            .context("encode relay transport allowlist response")
        }
        LocalServiceMethod::LocalControlStatus => {
            serde_json::to_string(&embedded_control_status()).context("encode control status")
        }
        LocalServiceMethod::LocalEnsureDevice => {
            serde_json::to_string(&ensure_embedded_device()?).context("encode ensure device")
        }
        LocalServiceMethod::LocalConnectControlMqtt => {
            serde_json::to_string(&connect_embedded_control_mqtt()?)
                .context("encode mqtt connect status")
        }
        LocalServiceMethod::LocalSendClientMessage => {
            let input: SendClientMessageRequest = serde_json::from_value(request.args)
                .context("decode send client message request")?;
            serde_json::to_string(&send_embedded_client_message(input)?)
                .context("encode send client message response")
        }
        LocalServiceMethod::LocalRegisterTestUser => {
            let input: RegisterTestUserRequest =
                serde_json::from_value(request.args).context("decode register test user")?;
            serde_json::to_string(&register_embedded_test_user(input)?)
                .context("encode register test user response")
        }
        LocalServiceMethod::Dispatch => {
            let command: ClientCommand =
                serde_json::from_value(request.args).context("decode client command")?;
            serde_json::to_string(&dispatch_embedded(command, correlation_id.clone())?)
                .context("encode state")
        }
        LocalServiceMethod::LocalLogout => serde_json::to_string(&dispatch_embedded(
            ClientCommand::Logout,
            correlation_id.clone(),
        )?)
        .context("encode state"),
        LocalServiceMethod::LocalNetworkShutdown => serde_json::to_string(&dispatch_embedded(
            ClientCommand::DisableNetwork,
            correlation_id.clone(),
        )?)
        .context("encode network shutdown state"),
        LocalServiceMethod::IngestPlatformRuntimeState => {
            let report: PlatformRuntimeStateReportRequest = serde_json::from_value(request.args)
                .context("decode embedded platform runtime report")?;
            let runtime_details = report.runtime_state.clone();
            let runtime_state = serde_json::from_value(runtime_details.clone())
                .context("decode embedded platform runtime state")?;
            let reported_error = report
                .error
                .as_deref()
                .map(str::trim)
                .filter(|value| !value.is_empty())
                .map(str::to_string);
            let actor_error = reported_error.clone();
            let traffic = report.traffic.clone();
            let reported_at_ms = report.reported_at_ms;
            let session_epoch = current_session_runtime_epoch();
            let Ok(_session_guard) = lock_session_runtime_epoch(session_epoch) else {
                return serde_json::to_string(&serde_json::json!({
                    "accepted": false,
                    "ignored": true,
                    "reason": "stale_session",
                    "state": runtime().snapshot().state,
                }))
                .context("encode stale embedded runtime state ingest");
            };
            let state = runtime().call_named(
                "embedded.platform.runtime.report",
                correlation_id.clone(),
                move |runtime| {
                    let mut state = runtime
                        .dispatch(ClientCommand::ApplyPlatformRuntimeState(runtime_state))
                        .context("apply embedded platform runtime state")?;
                    if let Some(traffic) = platform_traffic_stats_payload(
                        session_epoch,
                        traffic.as_ref(),
                        reported_at_ms,
                    ) {
                        state = runtime
                            .dispatch(ClientCommand::ApplyTrafficStats(traffic))
                            .context("apply embedded platform traffic stats")?;
                    }
                    if let Some(error) = actor_error {
                        state = runtime.set_error(error);
                    }
                    Ok(state)
                },
            )?;
            report_runtime_state(&state);
            publish_embedded_business_event(
                BUSINESS_NETWORK_RUNTIME_CHANGED,
                serde_json::json!({
                    "messageType": "platform_runtime_state",
                    "platform": report.platform,
                    "error": reported_error,
                }),
                &state,
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
            .context("encode runtime state ingest")
        }
        LocalServiceMethod::LocalPlatformNetworkConfig => {
            serde_json::to_string(&platform_network_config()?).context("encode platform config")
        }
        _ => anyhow::bail!("unsupported embedded service method {}", request.method),
    }?;
    Ok(response_with_correlation_id(
        &response,
        correlation_id.as_deref(),
    ))
}

fn refresh_state_json() -> Value {
    let mut value = serde_json::to_value(refresh_state()).unwrap_or_else(|_| serde_json::json!({}));
    if let Value::Object(ref mut object) = value {
        if let Some(summary) = load_embedded_downstream_summary() {
            object.insert("lastDownstreamSummary".to_string(), summary);
        }
        if let Some(summary) = load_embedded_mqtt_publish_summary() {
            object.insert("lastMqttPublishSummary".to_string(), summary);
        }
    }
    value
}

fn local_resolver_state_json() -> Value {
    let network_snapshot = runtime_network_state_store().snapshot();
    let network = Some(network_snapshot.state);
    let resolver = resolver_runtime_state()
        .lock()
        .ok()
        .map(|guard| guard.clone());
    let server = crate::resolver_server::local_resolver_server_status()
        .try_lock()
        .ok()
        .map(|guard| guard.clone())
        .unwrap_or_default();
    serde_json::json!({
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
    })
}

fn local_resolver_resolve_json(input: LocalResolverResolveRequest) -> Value {
    let session = load_session().ok();
    let network = runtime_network_state_store()
        .snapshot_for_session(session.as_ref())
        .state;
    let Ok(mut resolver) = resolver_runtime_state().lock() else {
        return serde_json::json!({ "error": "network resolver runtime busy: resolver state lock unavailable" });
    };
    resolve_authoritative_result_json(resolve_authoritative(
        &network,
        &mut resolver,
        &input.requester_device_id,
        &input.qname,
        &input.qtype,
    ))
}

fn apply_embedded_request_overrides(args: &Value) {
    if let Some(state_dir) = args
        .get("stateDir")
        .and_then(|value| value.as_str())
        .map(str::trim)
        .filter(|value| !value.is_empty())
    {
        set_app_data_dir_override(state_dir);
        set_platform_log_dir(state_dir);
    }
    if let Some(device_id) = args
        .get("deviceIdOverride")
        .and_then(|value| value.as_str())
        .map(str::trim)
        .filter(|value| !value.is_empty())
    {
        set_client_device_id_override(device_id);
    }
    if let Some(control_base_url) = args
        .get("controlBaseUrl")
        .and_then(|value| value.as_str())
        .map(str::trim)
        .filter(|value| !value.is_empty())
    {
        set_control_base_url_override(control_base_url);
        let mutex = EMBEDDED_CONTROL_BASE_URL.get_or_init(|| Mutex::new(None));
        let mut value = mutex
            .lock()
            .expect("embedded control base url mutex poisoned");
        *value = Some(control_base_url.trim_end_matches('/').to_string());
    }
}

fn recover_embedded_session_from_auth_payload(
    payload: AuthPayload,
    label: &str,
) -> Result<PreparedSession> {
    let mut prepared = match prepare_session_from_control_plane(payload.clone()) {
        Ok(prepared) => prepared,
        Err(hydrate_error) => {
            eprintln!(
                "client-core-service embedded {label} hydrate failed; retry register device: {hydrate_error:#}"
            );
            let recovered = PersistedSession::from(payload);
            match prepare_session_device_registered(recovered.clone()) {
                Ok(prepared) => prepared,
                Err(register_error) => {
                    eprintln!(
                        "client-core-service embedded {label} device registration recovery failed: {register_error:#}"
                    );
                    PreparedSession::from_session(recovered)
                }
            }
        }
    };
    if prepared.session.mqtt.is_none() {
        if let Some(device_id) = prepared
            .session
            .device_id
            .as_deref()
            .map(str::trim)
            .filter(|value| !value.is_empty())
        {
            match ControlPlaneClient::from_env()
                .prepare_device_login(device_id, std::env::consts::OS)
            {
                Ok(login) => {
                    if let Some(mqtt) = login.mqtt {
                        prepared.session.mqtt = Some(mqtt);
                    }
                }
                Err(error) => {
                    eprintln!(
                        "client-core-service embedded {label} mqtt backfill failed device={device_id}: {error:#}"
                    );
                }
            }
        }
    }
    Ok(prepared)
}

fn commit_embedded_prepared_login(
    runtime: &mut ClientRuntime<PlatformNetworkImpl>,
    prepared: &PreparedSession,
    expected_device_id: Option<&str>,
    expected_signed_in: bool,
) -> Result<ClientViewState> {
    if runtime.state().signed_in != expected_signed_in
        || runtime.state().device_id.as_deref().map(str::trim) != expected_device_id.map(str::trim)
    {
        return Err(anyhow::anyhow!("stale embedded login session"));
    }
    persist_session(&prepared.session)?;
    apply_prepared_session_projection(current_session_runtime_epoch(), prepared)?;
    runtime
        .dispatch(ClientCommand::ApplyDeviceUserLogin(
            prepared.session.clone().into(),
        ))
        .context("apply embedded prepared login")
}

fn platform_network_config() -> Result<Value> {
    let mut session = ensure_device_session().context("ensure device")?;
    let expected_snapshot = runtime().snapshot();
    let client = ControlPlaneClient::from_env();
    refresh_session_runtime_endpoints(&client, &mut session)
        .context("refresh embedded server-managed runtime endpoints")?;
    let network_id = match session
        .active_network_id
        .clone()
        .filter(|value| !value.trim().is_empty())
    {
        Some(value) => value,
        None => client
            .active_network_id(session_device_api_token(&session))?
            .ok_or_else(|| anyhow::anyhow!("active network is not available"))?,
    };
    session.active_network_id = Some(network_id.clone());
    let device_id = session
        .device_id
        .clone()
        .filter(|value| !value.trim().is_empty())
        .ok_or_else(|| anyhow::anyhow!("device id is not available"))?;
    let activation =
        client.activate_device_networks(session_device_api_token(&session), &device_id)?;
    session.self_node_id = activation.self_node_id.clone();
    session.virtual_ip = Some(activation.virtual_ip.clone());
    ensure_session_node_binding(&client, &mut session)
        .context("ensure node binding after network activation")?;
    let network_configs = client
        .device_network_configs(session_device_api_token(&session), &device_id)
        .unwrap_or_else(|_| crate::network_module::network_module_snapshot().configs);
    let persisted_relay_candidates = if activation.relay_candidates.is_empty() {
        runtime_relay_candidates()
    } else {
        activation
            .relay_candidates
            .iter()
            .map(embedded_persisted_relay_candidate)
            .collect()
    };
    let relay_candidates = select_relay_candidates(&persisted_relay_candidates);
    let best_relay = relay_candidates
        .iter()
        .find(|candidate| candidate.selected)
        .map(embedded_relay_candidate_from_selection)
        .or_else(|| activation.relay_candidates.first().cloned());
    let all_acl_policies = platform_acl_policies(&network_configs);
    let eligible_relay_peer_count =
        embedded_eligible_relay_peer_count(activation.self_node_id.as_deref(), &activation.peers);
    let (relay_data_plane, relay_build_error) = match build_embedded_relay_data_plane_config(
        &client,
        &session,
        network_id.as_str(),
        activation.self_node_id.as_deref(),
        &activation.peers,
        best_relay.as_ref(),
        &all_acl_policies,
    ) {
        Ok(config) => (Some(config), None),
        Err(error) => (None, Some(format!("{error:#}"))),
    };
    let relay_session_count = relay_data_plane
        .as_ref()
        .map(|config| config.sessions.len())
        .unwrap_or_default();
    let platform_relay_address = relay_data_plane
        .as_ref()
        .map(|config| config.relay_address.clone())
        .or_else(|| best_relay.as_ref().map(|relay| relay.address.clone()));
    let resolver = platform_resolver_config(&network_id, &activation.resolver, &network_configs);
    let resolver_records =
        crate::resolver_apply::platform_resolver_records_from_configs(&network_configs);
    let config = MobilePlatformNetworkConfig {
        session_name: "SLAN".to_string(),
        virtual_ip: activation.virtual_ip.clone(),
        prefix_len: activation.prefix_len,
        network_configs: platform_network_configs(&network_configs),
        resolver,
        routes: embedded_routes_with_peer_virtual_ips(
            activation.routes,
            &activation.peers,
            activation.virtual_ip.as_str(),
        ),
        mtu: Some(1280),
        relay_endpoint_id: best_relay.as_ref().map(|relay| relay.endpoint_id.clone()),
        relay_transport: best_relay.as_ref().map(|relay| relay.transport.clone()),
        relay_address: platform_relay_address,
        acl_policies: all_acl_policies.clone(),
        relay_data_plane,
    };
    let mut value = serde_json::to_value(config).context("encode platform config value")?;
    if let Value::Object(map) = &mut value {
        if let Some(Value::Object(resolver)) = map.get_mut("resolver") {
            resolver.insert(
                "records".to_string(),
                serde_json::to_value(resolver_records).context("encode native resolver records")?,
            );
        }
        map.insert(
            "relayDebug".to_string(),
            serde_json::json!({
                "activationPeerCount": activation.peers.len(),
                "reportedPeerCount": activation.peer_count,
                "eligibleRelayPeerCount": eligible_relay_peer_count,
                "relaySessionCount": relay_session_count,
                "relayBuildError": relay_build_error,
            }),
        );
    }
    let assigned_virtual_ip = session.virtual_ip.clone().unwrap_or_default();
    let mut prepared_session = PreparedSession::from_session(session);
    prepared_session.relay_candidates = persisted_relay_candidates;
    prepared_session.network_configs = network_configs;
    runtime()
        .call_named_if_revision(
            "embedded.network.activation.apply",
            Some(device_id),
            expected_snapshot.revision,
            move |runtime| {
                commit_embedded_prepared_login(
                    runtime,
                    &prepared_session,
                    expected_snapshot.state.device_id.as_deref(),
                    expected_snapshot.state.signed_in,
                )?;
                runtime.dispatch(ClientCommand::SyncAssignedIp(AssignedIpPayload {
                    virtual_ip: assigned_virtual_ip,
                    prefix_len: Some(activation.prefix_len),
                }))?;
                Ok(())
            },
        )?
        .ok_or_else(|| anyhow::anyhow!("stale embedded platform network config"))?;
    Ok(value)
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

fn embedded_eligible_relay_peer_count(
    self_node_id: Option<&str>,
    peers: &[crate::control_plane::ControlPeer],
) -> usize {
    let Some(local_node_id) = self_node_id
        .map(str::trim)
        .filter(|value| !value.is_empty())
    else {
        return 0;
    };
    peers
        .iter()
        .filter(|peer| peer.relay_allowed)
        .filter(|peer| peer.node_id != local_node_id)
        .count()
}

fn embedded_routes_with_peer_virtual_ips(
    mut routes: Vec<RouteSpec>,
    peers: &[crate::control_plane::ControlPeer],
    self_ip: &str,
) -> Vec<RouteSpec> {
    let self_ip = client_core::normalize_virtual_ip(self_ip);
    for peer in peers {
        for ip in &peer.virtual_ips {
            let peer_ip = client_core::normalize_virtual_ip(ip);
            if peer_ip.is_empty() || peer_ip == self_ip {
                continue;
            }
            let destination = format!("{peer_ip}/32");
            if routes.iter().any(|route| route.destination == destination) {
                continue;
            }
            routes.push(RouteSpec {
                destination,
                gateway: None,
            });
        }
    }
    routes
}

fn build_embedded_relay_data_plane_config(
    client: &ControlPlaneClient,
    session: &PersistedSession,
    network_id: &str,
    self_node_id: Option<&str>,
    peers: &[crate::control_plane::ControlPeer],
    relay: Option<&crate::control_plane::RelayCandidate>,
    acl_policies: &[PlatformAclPolicy],
) -> Result<RelayDataPlaneConfig> {
    let relay = relay.ok_or_else(|| anyhow::anyhow!("no relay candidate is available"))?;
    let relay_transport = relay.transport.trim().to_ascii_lowercase();
    let relay_path_kind =
        relay_path_kind_for_transport(relay_transport.as_str()).ok_or_else(|| {
            anyhow::anyhow!("unsupported embedded relay transport: {relay_transport}")
        })?;
    let local_node_id = self_node_id
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .ok_or_else(|| anyhow::anyhow!("network map missing self node id"))?;
    let access_token = session_device_api_token(session).trim();
    let eligible_peers = peers
        .iter()
        .filter(|peer| peer.relay_allowed)
        .filter(|peer| peer.node_id != local_node_id)
        .collect::<Vec<_>>();
    let mut sessions = Vec::with_capacity(eligible_peers.len());
    let mut ticket_errors = Vec::new();
    let relay_transport =
        client_core::normalize_relay_transport(relay.transport.as_str()).unwrap_or("udp");
    let preferred_derp_node_id =
        (relay_transport == "derp_tcp_tls_443").then_some(relay.endpoint_id.as_str());
    let preferred_relay_endpoint_id =
        (relay_transport != "derp_tcp_tls_443").then_some(relay.endpoint_id.as_str());
    for peer in eligible_peers {
        match client.issue_relay_ticket(
            access_token,
            network_id,
            local_node_id,
            peer.node_id.as_str(),
            relay.cluster_id.as_deref(),
            preferred_derp_node_id,
            preferred_relay_endpoint_id,
            relay.region_id.as_deref(),
        ) {
            Ok(mut ticket) => {
                normalize_embedded_relay_ticket_for_candidate(&mut ticket, relay);
                sessions.push(RelayPeerSession {
                    session_id: ticket.session_id.clone(),
                    peer_node_id: peer.node_id.clone(),
                    peer_virtual_ips: peer.virtual_ips.clone(),
                    ticket,
                })
            }
            Err(error) => ticket_errors.push(format!("{}:{error:#}", peer.node_id)),
        }
    }
    if sessions.is_empty() && !ticket_errors.is_empty() {
        anyhow::bail!(
            "embedded relay ticket issue failed: {}",
            ticket_errors.join("; ")
        );
    }
    let relay_address = sessions
        .first()
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
        path_policy: Default::default(),
        peer_paths: embedded_peer_path_configs(
            peers,
            local_node_id,
            relay,
            relay_path_kind,
            sessions.as_slice(),
        ),
        relay_mtu: Some(1280),
        max_frame_payload: Some(1200),
        acl_policies: acl_policies_for_network(acl_policies, network_id),
        sessions,
    })
}

fn embedded_peer_path_configs(
    peers: &[crate::control_plane::ControlPeer],
    local_node_id: &str,
    relay: &crate::control_plane::RelayCandidate,
    relay_path_kind: PathKind,
    relay_sessions: &[RelayPeerSession],
) -> Vec<PeerPathConfig> {
    let relay_only = relay_only_path_policy_enabled();
    peers
        .iter()
        .filter(|peer| peer.node_id != local_node_id)
        .map(|peer| {
            let relay_session = relay_sessions
                .iter()
                .find(|session| session.peer_node_id == peer.node_id);
            let mut candidates = Vec::new();
            let mut direct_addresses = Vec::new();
            if !relay_only {
                for endpoint in &peer.endpoints {
                    let address = endpoint.address.trim();
                    let Some(path_kind) = embedded_direct_path_kind(&endpoint.endpoint_type) else {
                        continue;
                    };
                    if address.is_empty()
                        || !valid_embedded_direct_candidate_address(address)
                        || direct_addresses.iter().any(|value| value == address)
                    {
                        continue;
                    }
                    direct_addresses.push(address.to_string());
                    candidates.push(PathCandidate {
                        kind: path_kind,
                        state: PathState::Probing,
                        endpoint_id: None,
                        address: Some(address.to_string()),
                        session_id: None,
                        transport: Some("udp".to_string()),
                        rtt_ms: None,
                        path_score: Some(900),
                        last_ok_at_ms: None,
                        last_error: None,
                    });
                }
                candidates.sort_by_key(|candidate| candidate.kind.priority());
            }
            if let Some(session) = relay_session {
                let relay_transport = relay.transport.trim().to_ascii_lowercase();
                candidates.push(PathCandidate {
                    kind: relay_path_kind,
                    state: PathState::Standby,
                    endpoint_id: Some(relay.endpoint_id.clone()),
                    address: Some(relay.address.clone()),
                    session_id: Some(session.session_id.clone()),
                    transport: Some(relay_transport),
                    rtt_ms: None,
                    path_score: Some(700),
                    last_ok_at_ms: None,
                    last_error: None,
                });
            }
            PeerPathConfig {
                peer_node_id: peer.node_id.clone(),
                peer_virtual_ips: peer.virtual_ips.clone(),
                candidates,
            }
        })
        .collect()
}

fn embedded_direct_path_kind(endpoint_type: &str) -> Option<PathKind> {
    match endpoint_type.trim() {
        "lan_udp" => Some(PathKind::LanUdp),
        "ipv6_udp" => Some(PathKind::Ipv6Udp),
        "direct_udp" => Some(PathKind::DirectUdp),
        _ => None,
    }
}

fn valid_embedded_direct_candidate_address(address: &str) -> bool {
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

fn ensure_device_session() -> Result<PersistedSession> {
    let session = load_session().context("load session")?;
    let session = align_embedded_session_device_id(session)?;
    let expected_state = runtime().snapshot().state;
    let prepared = prepare_session_device_registered(session).context("prepare device")?;
    let session = prepared.session.clone();
    let correlation_id = session.device_id.clone();
    runtime().call_named("embedded.device.ensure", correlation_id, move |runtime| {
        commit_embedded_prepared_login(
            runtime,
            &prepared,
            expected_state.device_id.as_deref(),
            expected_state.signed_in,
        )?;
        Ok(())
    })?;
    Ok(session)
}

fn align_embedded_session_device_id(mut session: PersistedSession) -> Result<PersistedSession> {
    let expected_device_id = local_stable_device_id().context("resolve embedded device id")?;
    let current_device_id = session
        .device_id
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty());
    if current_device_id == Some(expected_device_id.as_str()) {
        return Ok(session);
    }
    session.device_id = Some(expected_device_id);
    session.device_session_id = None;
    session.device_token = None;
    session.device_refresh_token = None;
    session.device_token_expires_at = None;
    session.self_node_id = None;
    session.virtual_ip = None;
    session.mqtt = None;
    Ok(session)
}

fn ensure_embedded_device() -> Result<Value> {
    let session = ensure_device_session()?;
    Ok(serde_json::json!({
        "registered": session.device_id.as_deref().is_some_and(|value| !value.trim().is_empty()),
        "deviceId": session.device_id,
        "activeNetworkId": session.active_network_id,
        "virtualIp": session.virtual_ip,
        "mqttCredentialReady": session.mqtt.is_some(),
        "mqttExpiresAt": session.mqtt.as_ref().and_then(|credential| credential.expires_at),
        "controlStatus": embedded_control_status(),
    }))
}

fn connect_embedded_control_mqtt() -> Result<Value> {
    let session = ensure_device_session().context("ensure device before mqtt")?;
    connect_embedded_control_mqtt_with_session(&session)
}

fn connect_embedded_control_mqtt_with_session(session: &PersistedSession) -> Result<Value> {
    let mqtt = session
        .mqtt
        .clone()
        .ok_or_else(|| anyhow::anyhow!("mqtt credential is missing after device registration"))?;
    let downstream_topic = format!("{}/control/down", mqtt.topic_prefix.trim_end_matches('/'));
    let device_id = session.device_id.clone();
    let credential = ThinMqttCredential {
        broker_url: embedded_mqtt_broker_url(&mqtt.broker_url),
        client_id: mqtt.client_id,
        username: mqtt.username,
        password: mqtt.password,
    };
    let mut client = connect_embedded_control_mqtt_with_retry(&credential, &downstream_topic)
        .map_err(|error| {
            let message = format!("connect control mqtt: {error}");
            set_embedded_mqtt_last_error(Some(message.clone()));
            anyhow::anyhow!(message)
        })?;
    let generation = next_embedded_mqtt_generation();
    eprintln!(
        "SLAN_EMBEDDED_MQTT_CONNECTED generation={} deviceId={} downstreamTopic={} brokerUrl={}",
        generation,
        device_id.as_deref().unwrap_or_default(),
        downstream_topic,
        credential.broker_url,
    );
    let desired_network_event_topics = network_event_topics_for_session(session);
    let mut subscribed_network_event_topics = Vec::new();
    let mut network_subscribe_errors = Vec::new();
    for topic in &desired_network_event_topics {
        if let Err(error) = client.subscribe(topic) {
            eprintln!("SLAN_EMBEDDED_MQTT_NETWORK_SUBSCRIBE_FAILED topic={topic} error={error}");
            network_subscribe_errors
                .push(format!("subscribe network event topic {topic}: {error}"));
        } else {
            subscribed_network_event_topics.push(topic.clone());
            eprintln!("SLAN_EMBEDDED_MQTT_NETWORK_SUBSCRIBED topic={topic}");
        }
    }
    let network_event_subscribed =
        subscribed_network_event_topics.len() == desired_network_event_topics.len();
    let network_subscribe_error =
        (!network_subscribe_errors.is_empty()).then(|| network_subscribe_errors.join("; "));
    {
        let mut guard = mqtt_connection()
            .lock()
            .expect("embedded mqtt mutex poisoned");
        *guard = Some(EmbeddedMqttConnection {
            generation,
            device_id: device_id.clone(),
            downstream_topic: downstream_topic.clone(),
            network_event_topics: subscribed_network_event_topics.clone(),
            connected: true,
            network_event_subscribed,
            last_message_topic: None,
            last_message_type: None,
            last_error: network_subscribe_error.clone(),
        });
    }
    set_embedded_mqtt_last_error(network_subscribe_error.clone());
    spawn_embedded_mqtt_consumer(
        client,
        generation,
        device_id.clone(),
        downstream_topic.clone(),
        subscribed_network_event_topics.clone(),
    );
    Ok(serde_json::json!({
        "connected": true,
        "deviceId": device_id,
        "downstreamTopic": downstream_topic,
        "networkEventTopics": subscribed_network_event_topics,
        "networkEventSubscribed": network_event_subscribed,
        "networkSubscribeError": network_subscribe_error,
        "brokerUrl": credential.broker_url,
        "controlStatus": embedded_control_status(),
    }))
}

fn connect_embedded_control_mqtt_with_retry(
    credential: &ThinMqttCredential,
    downstream_topic: &str,
) -> std::result::Result<ThinControlMqttClient, String> {
    let mut errors = Vec::new();
    for attempt in 1..=3 {
        let suffix = format!("v2-embedded-{}-{attempt}", current_timestamp_ms());
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

fn set_embedded_mqtt_last_error(error: Option<String>) {
    let mut guard = mqtt_last_error_store()
        .lock()
        .expect("embedded mqtt last error mutex poisoned");
    *guard = error;
}

fn embedded_mqtt_broker_url(broker_url: &str) -> String {
    let Some(override_host) = embedded_control_host_override() else {
        return broker_url.to_string();
    };
    rewrite_local_mqtt_broker_host(broker_url, &override_host)
}

fn embedded_control_host_override() -> Option<String> {
    let control_base_url = EMBEDDED_CONTROL_BASE_URL
        .get()
        .and_then(|mutex| mutex.lock().ok().and_then(|value| value.clone()))?;
    let host = url_host(&control_base_url)?;
    (!is_loopback_host(&host)).then_some(host)
}

fn rewrite_local_mqtt_broker_host(broker_url: &str, override_host: &str) -> String {
    let Some(rest) = broker_url.strip_prefix("mqtt://") else {
        return broker_url.to_string();
    };
    let (authority, path) = rest.split_once('/').unwrap_or((rest, ""));
    let host = authority
        .rsplit_once('@')
        .map(|(_, value)| value)
        .unwrap_or(authority);
    let (host_part, port_part) = split_host_port(host);
    if !is_loopback_host(host_part) {
        return broker_url.to_string();
    }
    let path = if path.is_empty() {
        String::new()
    } else {
        format!("/{path}")
    };
    format!("mqtt://{override_host}{port_part}{path}")
}

fn url_host(url: &str) -> Option<String> {
    let rest = url.split_once("://").map(|(_, value)| value).unwrap_or(url);
    let authority = rest
        .split('/')
        .next()?
        .split('?')
        .next()?
        .split('#')
        .next()?;
    let host = authority
        .rsplit_once('@')
        .map(|(_, value)| value)
        .unwrap_or(authority);
    Some(split_host_port(host).0.trim_matches(['[', ']']).to_string())
        .filter(|value| !value.is_empty())
}

fn split_host_port(authority: &str) -> (&str, &str) {
    if authority.starts_with('[') {
        if let Some(end) = authority.find(']') {
            let host = &authority[..=end];
            let port = &authority[end + 1..];
            return (host, port);
        }
    }
    authority
        .rsplit_once(':')
        .map(|(host, port)| (host, &authority[host.len()..host.len() + port.len() + 1]))
        .unwrap_or((authority, ""))
}

fn is_loopback_host(host: &str) -> bool {
    matches!(
        host.trim_matches(['[', ']']).to_ascii_lowercase().as_str(),
        "localhost" | "127.0.0.1" | "::1" | "0.0.0.0"
    )
}

fn spawn_embedded_mqtt_consumer(
    mut client: ThinControlMqttClient,
    generation: u64,
    device_id: Option<String>,
    downstream_topic: String,
    subscribed_network_event_topics: Vec<String>,
) {
    thread::spawn(move || {
        eprintln!(
            "SLAN_EMBEDDED_MQTT_CONSUMER_START generation={} deviceId={} downstreamTopic={}",
            generation,
            device_id.as_deref().unwrap_or_default(),
            downstream_topic,
        );
        let mut last_heartbeat_ms = None;
        let mut last_runtime_state_ms = None;
        let mut last_path_health_ms = None;
        let mut last_session_key = String::new();
        let mut subscribed_network_event_topics = subscribed_network_event_topics
            .into_iter()
            .collect::<BTreeSet<_>>();
        let connected_at_ms = current_timestamp_ms();
        let mut last_keepalive_ping_ms = Some(connected_at_ms);
        let mut last_active_network_reconcile_ms = Some(connected_at_ms);
        let mut last_network_subscribe_attempt_ms = Some(connected_at_ms);
        loop {
            if !embedded_mqtt_generation_active(generation, &device_id, &downstream_topic) {
                eprintln!(
                    "SLAN_EMBEDDED_MQTT_CONSUMER_EXIT reason=superseded generation={} deviceId={} downstreamTopic={}",
                    generation,
                    device_id.as_deref().unwrap_or_default(),
                    downstream_topic,
                );
                return;
            }
            match client.read_publish(Duration::from_millis(500)) {
                Ok(Some(publish)) => {
                    let message_type = embedded_downstream_message_type(&publish.payload);
                    if let Err(error) = persist_embedded_mqtt_publish_summary(
                        &publish.topic,
                        message_type.as_deref(),
                        &publish.payload,
                    ) {
                        eprintln!(
                            "SLAN_EMBEDDED_MQTT_PUBLISH_SUMMARY_FAILED topic={} messageType={} error={:#}",
                            publish.topic,
                            message_type.as_deref().unwrap_or_default(),
                            error,
                        );
                    }
                    eprintln!(
                        "SLAN_EMBEDDED_MQTT_PUBLISH_RECV topic={} qos={} payloadBytes={} messageType={}",
                        publish.topic,
                        publish.qos,
                        publish.payload.len(),
                        message_type.as_deref().unwrap_or_default(),
                    );
                    let consume_result = ingest_embedded_downstream_publish(&publish.payload);
                    let ack_result = client.ack_publish(&publish);
                    let business_ack_result = if ack_result.is_ok() {
                        publish_embedded_downstream_business_ack(
                            &mut client,
                            &publish.payload,
                            consume_result.as_ref().err().map(|error| error.to_string()),
                        )
                    } else {
                        Ok(())
                    };
                    let business_ack_failed = business_ack_result.is_err();
                    let last_error = consume_result
                        .err()
                        .map(|error| error.to_string())
                        .or_else(|| business_ack_result.err())
                        .or_else(|| ack_result.clone().err());
                    if let Some(error) = last_error.as_deref() {
                        eprintln!(
                            "SLAN_EMBEDDED_MQTT_PUBLISH_HANDLE_FAILED topic={} messageType={} error={}",
                            publish.topic,
                            message_type.as_deref().unwrap_or_default(),
                            error,
                        );
                    } else {
                        eprintln!(
                            "SLAN_EMBEDDED_MQTT_PUBLISH_HANDLED topic={} messageType={} acked=true",
                            publish.topic,
                            message_type.as_deref().unwrap_or_default(),
                        );
                    }
                    update_embedded_mqtt_status(
                        generation,
                        &device_id,
                        &downstream_topic,
                        ack_result.is_ok(),
                        Some(publish.topic.clone()),
                        message_type,
                        last_error.clone(),
                    );
                    if ack_result.is_err() {
                        eprintln!(
                            "SLAN_EMBEDDED_MQTT_CONSUMER_EXIT reason=ack_publish_failed deviceId={} downstreamTopic={} error={}",
                            device_id.as_deref().unwrap_or_default(),
                            downstream_topic,
                            ack_result.err().unwrap_or_default(),
                        );
                        break;
                    }
                    if business_ack_failed {
                        eprintln!(
                            "SLAN_EMBEDDED_MQTT_CONSUMER_EXIT reason=business_ack_failed deviceId={} downstreamTopic={} error={}",
                            device_id.as_deref().unwrap_or_default(),
                            downstream_topic,
                            last_error.unwrap_or_default(),
                        );
                        break;
                    }
                }
                Ok(None) => {}
                Err(error) => {
                    eprintln!(
                        "SLAN_EMBEDDED_MQTT_READ_FAILED deviceId={} downstreamTopic={} error={}",
                        device_id.as_deref().unwrap_or_default(),
                        downstream_topic,
                        error,
                    );
                    update_embedded_mqtt_status(
                        generation,
                        &device_id,
                        &downstream_topic,
                        false,
                        None,
                        None,
                        Some(error),
                    );
                    break;
                }
            }

            let Ok(session) = load_session() else {
                continue;
            };
            let session_key = embedded_transport_session_key(&session);
            if session_key != last_session_key {
                last_session_key = session_key;
                last_heartbeat_ms = None;
                last_runtime_state_ms = None;
                last_path_health_ms = None;
            }
            let now_ms = current_timestamp_ms();
            if now_ms.saturating_sub(last_network_subscribe_attempt_ms.unwrap_or(0))
                >= EMBEDDED_NETWORK_SUBSCRIBE_RETRY_INTERVAL_MS
            {
                let desired_topics = network_event_topics_for_session(&session);
                let mut errors = Vec::new();
                for topic in &desired_topics {
                    if subscribed_network_event_topics.contains(topic) {
                        continue;
                    }
                    match client.subscribe(topic) {
                        Ok(()) => {
                            subscribed_network_event_topics.insert(topic.clone());
                            eprintln!(
                                "SLAN_EMBEDDED_MQTT_NETWORK_DYNAMIC_SUBSCRIBED topic={topic}"
                            );
                        }
                        Err(error) => {
                            eprintln!(
                                "SLAN_EMBEDDED_MQTT_NETWORK_DYNAMIC_SUBSCRIBE_FAILED topic={topic} error={error}"
                            );
                            errors.push(format!("subscribe network event topic {topic}: {error}"));
                        }
                    }
                }
                update_embedded_mqtt_subscriptions(
                    generation,
                    &device_id,
                    &downstream_topic,
                    &desired_topics,
                    &subscribed_network_event_topics,
                    errors,
                );
                last_network_subscribe_attempt_ms = Some(now_ms);
            }
            if now_ms.saturating_sub(connected_at_ms)
                >= EMBEDDED_MQTT_RECONNECT_AFTER_SESSION_REFRESH_MS
            {
                eprintln!(
                    "SLAN_EMBEDDED_MQTT_CONSUMER_EXIT reason=proactive_reconnect_after_session_refresh deviceId={} downstreamTopic={} connectedAtMs={} nowMs={}",
                    device_id.as_deref().unwrap_or_default(),
                    downstream_topic,
                    connected_at_ms,
                    now_ms,
                );
                update_embedded_mqtt_status(
                    generation,
                    &device_id,
                    &downstream_topic,
                    false,
                    None,
                    None,
                    Some(
                        "embedded mqtt proactive reconnect after session refresh window"
                            .to_string(),
                    ),
                );
                return;
            }
            if now_ms.saturating_sub(last_keepalive_ping_ms.unwrap_or(0))
                >= EMBEDDED_MQTT_KEEPALIVE_PING_INTERVAL_MS
            {
                if let Err(error) = client.ping() {
                    eprintln!(
                        "SLAN_EMBEDDED_MQTT_CONSUMER_EXIT reason=ping_failed deviceId={} downstreamTopic={} error={}",
                        device_id.as_deref().unwrap_or_default(),
                        downstream_topic,
                        error,
                    );
                    update_embedded_mqtt_status(
                        generation,
                        &device_id,
                        &downstream_topic,
                        false,
                        None,
                        None,
                        Some(error),
                    );
                    return;
                }
                last_keepalive_ping_ms = Some(now_ms);
            }
            if now_ms.saturating_sub(last_active_network_reconcile_ms.unwrap_or(0))
                >= EMBEDDED_ACTIVE_NETWORK_RECONCILE_INTERVAL_MS
            {
                reconcile_embedded_active_network_state();
                last_active_network_reconcile_ms = Some(now_ms);
            }
            let tick = control_transport::control_transport_tick_plan(
                control_transport::ControlTransportTickRequest {
                    now_ms: Some(now_ms),
                    last_ack_flush_ms: None,
                    last_heartbeat_ms,
                    last_runtime_state_ms,
                    last_path_health_ms,
                },
                now_ms,
            );
            if !tick.outbox.include_heartbeat
                && !tick.outbox.include_runtime_state
                && !tick.outbox.include_path_health
            {
                continue;
            }
            let state = runtime().snapshot().state;
            let outbox = control_transport::control_transport_outbox(
                &session,
                &state,
                Vec::new(),
                now_ms,
                tick.outbox.include_heartbeat,
                tick.outbox.include_runtime_state,
                tick.outbox.include_path_health,
            );
            for message in outbox.messages {
                if let Err(error) = publish_embedded_outbox_message(&mut client, &message) {
                    eprintln!(
                        "SLAN_EMBEDDED_MQTT_CONSUMER_EXIT reason=publish_outbox_failed deviceId={} downstreamTopic={} topic={} messageKind={:?} error={}",
                        device_id.as_deref().unwrap_or_default(),
                        downstream_topic,
                        message.topic,
                        message.kind,
                        error,
                    );
                    update_embedded_mqtt_status(
                        generation,
                        &device_id,
                        &downstream_topic,
                        false,
                        None,
                        None,
                        Some(error),
                    );
                    return;
                }
                last_keepalive_ping_ms = Some(now_ms);
                match message.kind {
                    control_transport::ControlTransportMessageKind::Heartbeat => {
                        last_heartbeat_ms = Some(now_ms);
                    }
                    control_transport::ControlTransportMessageKind::RuntimeState
                    | control_transport::ControlTransportMessageKind::EndpointReport => {
                        last_runtime_state_ms = Some(now_ms);
                    }
                    control_transport::ControlTransportMessageKind::PathHealth => {
                        last_path_health_ms = Some(now_ms);
                    }
                    control_transport::ControlTransportMessageKind::ControlAck => {}
                }
            }
        }
    });
}

fn reconcile_embedded_active_network_state() {
    let before = runtime().snapshot().state;
    if !before.signed_in || !before.network_enabled {
        return;
    }
    eprintln!("SLAN_EMBEDDED_ACTIVE_NETWORK_RECONCILE start");
    match platform_network_config() {
        Ok(_) => {
            let state = runtime().snapshot().state;
            publish_embedded_business_event(
                BUSINESS_CONTROL_SYNC_CHANGED,
                serde_json::json!({
                    "messageType": "active_network_reconcile",
                    "reconfigureRequired": true,
                }),
                &state,
            );
        }
        Err(error) => {
            let error_message = error.to_string();
            let correlation_id = before.device_id.clone();
            let state = runtime()
                .call_named(
                    "embedded.network.reconcile.error",
                    correlation_id,
                    move |runtime| Ok(runtime.set_error(error_message)),
                )
                .unwrap_or_else(|actor_error| {
                    let mut state = runtime().snapshot().state;
                    state.error = Some(actor_error.to_string());
                    state
                });
            publish_embedded_business_event(
                BUSINESS_CONTROL_SYNC_CHANGED,
                serde_json::json!({
                    "messageType": "active_network_reconcile_failed",
                    "error": error.to_string(),
                }),
                &state,
            );
        }
    }
}

fn embedded_transport_session_key(session: &PersistedSession) -> String {
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

fn publish_embedded_outbox_message(
    client: &mut ThinControlMqttClient,
    message: &ControlTransportMessage,
) -> std::result::Result<(), String> {
    let payload = serde_json::to_vec(&message.payload)
        .map_err(|err| format!("encode embedded outbox payload {}: {err}", message.id))?;
    client.publish(&message.topic, &payload, embedded_thin_qos(message.qos))
}

fn embedded_thin_qos(qos: MqttQos) -> ThinMqttQoS {
    match qos {
        MqttQos::QoS0 => ThinMqttQoS::AtMostOnce,
        MqttQos::QoS2 => ThinMqttQoS::ExactlyOnce,
    }
}

fn update_embedded_mqtt_status(
    generation: u64,
    device_id: &Option<String>,
    downstream_topic: &str,
    connected: bool,
    last_message_topic: Option<String>,
    last_message_type: Option<String>,
    last_error: Option<String>,
) {
    let mut guard = mqtt_connection()
        .lock()
        .expect("embedded mqtt mutex poisoned");
    if let Some(connection) = guard.as_mut() {
        if connection.generation == generation
            && connection.device_id == *device_id
            && connection.downstream_topic == downstream_topic
        {
            connection.connected = connected;
            if last_message_topic.is_some() {
                connection.last_message_topic = last_message_topic;
            }
            if last_message_type.is_some() {
                connection.last_message_type = last_message_type;
            }
            connection.last_error = last_error;
        }
    }
}

fn update_embedded_mqtt_subscriptions(
    generation: u64,
    device_id: &Option<String>,
    downstream_topic: &str,
    desired_topics: &[String],
    subscribed_topics: &BTreeSet<String>,
    errors: Vec<String>,
) {
    let mut guard = mqtt_connection()
        .lock()
        .expect("embedded mqtt mutex poisoned");
    let Some(connection) = guard.as_mut() else {
        return;
    };
    if connection.generation != generation
        || connection.device_id != *device_id
        || connection.downstream_topic != downstream_topic
    {
        return;
    }
    connection.network_event_topics = subscribed_topics.iter().cloned().collect();
    connection.network_event_subscribed = desired_topics
        .iter()
        .all(|topic| subscribed_topics.contains(topic));
    connection.last_error = (!errors.is_empty()).then(|| errors.join("; "));
}

fn embedded_downstream_message_type(payload: &[u8]) -> Option<String> {
    serde_json::from_slice::<Value>(payload)
        .ok()
        .and_then(|value| {
            value
                .get("type")
                .and_then(Value::as_str)
                .map(str::to_string)
        })
}

fn publish_embedded_downstream_business_ack(
    client: &mut ThinControlMqttClient,
    payload: &[u8],
    error: Option<String>,
) -> std::result::Result<(), String> {
    let value: Value = serde_json::from_slice(payload).map_err(|err| format!("{err}"))?;
    let Some((delivery_id, action)) = embedded_downstream_ack_identity(&value) else {
        return Ok(());
    };
    let session = load_session().map_err(|err| err.to_string())?;
    let mqtt = session
        .mqtt
        .as_ref()
        .ok_or_else(|| "embedded mqtt ack missing mqtt credential".to_string())?;
    let topic = format!("{}/control/ack", mqtt.topic_prefix.trim_end_matches('/'));
    let status = if error.is_some() {
        "failed"
    } else {
        "succeeded"
    };
    let ack = serde_json::json!({
        "taskId": format!("embedded-{action}-{delivery_id}"),
        "deliveryId": delivery_id,
        "action": action,
        "status": status,
        "error": error,
        "processedAtMs": current_timestamp_ms(),
    });
    let payload = serde_json::to_vec(&ack).map_err(|err| format!("encode embedded ack: {err}"))?;
    client.publish(&topic, &payload, ThinMqttQoS::ExactlyOnce)
}

fn embedded_downstream_ack_identity(value: &Value) -> Option<(String, &'static str)> {
    let message_type = value.get("type").and_then(Value::as_str)?;
    let delivery_id = value
        .get("messageId")
        .or_else(|| value.get("eventId"))
        .and_then(Value::as_str)
        .map(str::trim)
        .filter(|value| !value.is_empty())?
        .to_string();
    let action = match message_type {
        "device_user_login_succeeded" => "deviceUserLoginSucceeded",
        "network_event" => "reconcileNetworkState",
        "device_ip_reassigned" => "reconcileNetworkState",
        "client_message" => "clientMessage",
        _ => return None,
    };
    Some((delivery_id, action))
}

fn ingest_embedded_downstream_publish(payload: &[u8]) -> Result<()> {
    let value: Value = serde_json::from_slice(payload).context("decode downstream json")?;
    let message_type = value
        .get("type")
        .and_then(Value::as_str)
        .unwrap_or_default();
    let downstream_payload = value.get("payload");
    let network_id = downstream_payload
        .and_then(|payload| payload.get("networkId"))
        .and_then(Value::as_str)
        .unwrap_or_default();
    let from_device_id = downstream_payload
        .and_then(|payload| payload.get("fromDeviceId"))
        .and_then(Value::as_str)
        .unwrap_or_default();
    let target_device_id = downstream_payload
        .and_then(|payload| payload.get("targetDeviceId"))
        .and_then(Value::as_str)
        .unwrap_or_default();
    let body_bytes = downstream_payload
        .and_then(|payload| payload.get("body"))
        .and_then(Value::as_str)
        .map(str::len)
        .unwrap_or(0);
    persist_embedded_downstream_summary(&value)?;
    eprintln!(
        "SLAN_EMBEDDED_DOWNSTREAM_PUBLISH type={} messageId={} networkId={} fromDeviceId={} targetDeviceId={} bodyBytes={} payloadBytes={}",
        message_type,
        value
            .get("messageId")
            .and_then(Value::as_str)
            .unwrap_or_default(),
        network_id,
        from_device_id,
        target_device_id,
        body_bytes,
        payload.len(),
    );
    match message_type {
        "network_event" => ingest_embedded_network_event(&value),
        "device_user_login_succeeded" => ingest_embedded_device_user_login_succeeded(&value),
        "device_network_membership_changed" => {
            ingest_embedded_device_network_membership_changed(&value)
        }
        "device_ip_reassigned" => ingest_embedded_device_ip_reassigned(&value),
        "client_message" => persist_embedded_client_message(&value),
        _ => Ok(()),
    }
}

fn ingest_embedded_device_network_membership_changed(value: &Value) -> Result<()> {
    let payload = crate::network_event::decode_device_network_membership_payload(
        value.get("payload").cloned().ok_or_else(|| {
            anyhow::anyhow!("device_network_membership_changed payload is missing")
        })?,
    )
    .context("decode embedded device network membership")?;
    let changed_network_id = payload
        .changed_network_id
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .map(str::to_string);
    let operation = payload.operation.clone().unwrap_or_default();
    let membership_version = payload.membership_version;
    let snapshot = runtime().snapshot();
    let mut session = load_session().context("load embedded membership session")?;
    apply_device_network_membership(&mut session, payload)?;
    let mut prepared = PreparedSession::from_session(session.clone());
    if !session.access_token.trim().is_empty() {
        let client = ControlPlaneClient::from_env();
        if let Some(device_id) = session
            .device_id
            .as_deref()
            .map(str::trim)
            .filter(|value| !value.is_empty())
        {
            prepared.network_configs = client
                .device_network_configs(session_device_api_token(&session), device_id)
                .context("refresh full embedded network configs after membership change")?;
            if operation == "joined" {
                if let Some(network_id) = changed_network_id.as_deref() {
                    let network_snapshot = client
                        .network_snapshot(session_device_api_token(&session), network_id, device_id)
                        .with_context(|| {
                            format!("load embedded joined network snapshot networkId={network_id}")
                        })?;
                    let snapshot_envelope = NetworkEventEnvelope {
                        r#type: "network_event".to_string(),
                        network_id: network_snapshot.network_id.clone(),
                        version: network_snapshot.version,
                        event_id: synthetic_snapshot_event_id(
                            "membership",
                            &network_snapshot.network_id,
                            network_snapshot.version,
                        ),
                        event_type: crate::network_event::NetworkEventType::NetworkSnapshot,
                        occurred_at: current_timestamp_ms(),
                        payload: serde_json::to_value(network_snapshot.snapshot)
                            .context("encode embedded joined network snapshot")?,
                    };
                    crate::network_module::apply_network_module_event(
                        network_id,
                        device_id,
                        &snapshot_envelope,
                    )
                    .context("apply embedded joined network snapshot")?;
                }
            }
        }
    }
    let network_ids = session.network_ids.clone();
    let active_network_id = session.active_network_id.clone();
    let state = runtime()
        .call_named_if_revision(
            "embedded.network.membership.commit",
            session.device_id.clone(),
            snapshot.revision,
            move |runtime| {
                commit_embedded_prepared_login(
                    runtime,
                    &prepared,
                    snapshot.state.device_id.as_deref(),
                    snapshot.state.signed_in,
                )
            },
        )?
        .unwrap_or_else(|| runtime().snapshot().state);
    publish_embedded_business_event(
        BUSINESS_CONTROL_SYNC_CHANGED,
        serde_json::json!({
            "messageType": "device_network_membership_changed",
            "messageId": value.get("messageId").and_then(Value::as_str),
            "networkIds": network_ids,
            "activeNetworkId": active_network_id,
            "changedNetworkId": changed_network_id,
            "operation": operation,
            "membershipVersion": membership_version,
            "fullNetworkRefresh": true,
            "reconfigureRequired": true,
        }),
        &state,
    );
    Ok(())
}

fn ingest_embedded_network_event(value: &Value) -> Result<()> {
    let envelope: NetworkEventEnvelope =
        serde_json::from_value(value.clone()).context("decode network_event envelope")?;
    let session_epoch = current_session_runtime_epoch();
    let session = load_session().context("load session for embedded network event")?;
    let runtime_active_network_id =
        runtime_network_state_store().read(|state| state.active_network_id.clone());
    if !network_event_targets_session(&envelope, &session, runtime_active_network_id.as_deref()) {
        eprintln!(
            "SLAN_EMBEDDED_NETWORK_EVENT_IGNORED reason=network_mismatch networkId={} activeNetworkId={}",
            envelope.network_id,
            session.active_network_id.as_deref().unwrap_or_default(),
        );
        return Ok(());
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
    let apply_result = runtime().call_named(
        "embedded.network.event.projection",
        Some(envelope.event_id.clone()),
        move |_runtime| {
            apply_network_event_projection(
                session_epoch,
                &projection_session,
                &projection_device_id,
                &projection_envelope,
            )
        },
    )?;
    eprintln!(
        "SLAN_EMBEDDED_NETWORK_EVENT_APPLIED networkId={} eventType={:?} version={} result={:?}",
        envelope.network_id, envelope.event_type, envelope.version, apply_result,
    );
    if matches!(apply_result, ApplyResult::NeedsSnapshot) {
        runtime().call_named(
            "embedded.network.snapshot.mark_syncing",
            Some(envelope.event_id.clone()),
            move |_runtime| mark_network_projection_syncing(session_epoch),
        )?;
        let client = ControlPlaneClient::from_env();
        let snapshot = client
            .network_snapshot(
                session_device_api_token(&session),
                &envelope.network_id,
                &local_device_id,
            )
            .context("load embedded network snapshot")?;
        let snapshot_event_id =
            synthetic_snapshot_event_id("recovery", &snapshot.network_id, snapshot.version);
        let snapshot_envelope = NetworkEventEnvelope {
            r#type: "network_event".to_string(),
            network_id: snapshot.network_id,
            version: snapshot.version,
            event_id: snapshot_event_id,
            event_type: crate::network_event::NetworkEventType::NetworkSnapshot,
            occurred_at: current_timestamp_ms(),
            payload: serde_json::to_value(snapshot.snapshot)
                .context("encode embedded network snapshot payload")?,
        };
        let projection_session = session.clone();
        let projection_device_id = local_device_id.clone();
        let projection_envelope = snapshot_envelope.clone();
        let snapshot_result = runtime().call_named(
            "embedded.network.snapshot.projection",
            Some(snapshot_envelope.event_id.clone()),
            move |_runtime| {
                apply_network_event_projection(
                    session_epoch,
                    &projection_session,
                    &projection_device_id,
                    &projection_envelope,
                )
            },
        )?;
        eprintln!(
            "SLAN_EMBEDDED_NETWORK_SNAPSHOT_APPLIED networkId={} version={} result={:?}",
            envelope.network_id, snapshot.version, snapshot_result,
        );
        let state = runtime().snapshot().state;
        publish_embedded_business_event(
            BUSINESS_CONTROL_SYNC_CHANGED,
            network_event_business_data(
                &snapshot_envelope.event_id,
                &envelope.network_id,
                &envelope.event_type,
                snapshot.version,
                false,
                "snapshot",
            ),
            &state,
        );
        return Ok(());
    }
    let state = runtime().snapshot().state;
    publish_embedded_business_event(
        BUSINESS_CONTROL_SYNC_CHANGED,
        network_event_business_data(
            &envelope.event_id,
            &envelope.network_id,
            &envelope.event_type,
            envelope.version,
            matches!(apply_result, ApplyResult::Applied),
            "event",
        ),
        &state,
    );
    Ok(())
}

fn ingest_embedded_device_user_login_succeeded(value: &Value) -> Result<()> {
    let auth_value = value
        .get("payload")
        .cloned()
        .ok_or_else(|| anyhow::anyhow!("device_user_login_succeeded payload is missing"))?;
    let auth: AuthPayload =
        serde_json::from_value(auth_value).context("decode device_user_login_succeeded payload")?;
    let expected_state = runtime().snapshot().state;
    validate_embedded_device_user_login_succeeded(&expected_state, &auth)?;
    let prepared =
        prepare_session_from_control_plane(auth).context("prepare embedded device user login")?;
    let session = prepared.session.clone();
    let correlation_id = value
        .get("messageId")
        .and_then(Value::as_str)
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .map(str::to_string);
    let delivery_id = correlation_id.clone();
    let state = runtime()
        .call_named(
            "embedded.mqtt.login.apply",
            correlation_id,
            move |runtime| {
                commit_embedded_prepared_login(
                    runtime,
                    &prepared,
                    expected_state.device_id.as_deref(),
                    expected_state.signed_in,
                )
            },
        )
        .context("dispatch embedded device user login")?;
    clear_embedded_mqtt_connection();
    connect_embedded_control_mqtt_with_session(&session)
        .context("connect mqtt after embedded device user login")?;
    publish_embedded_business_event(
        BUSINESS_SESSION_CHANGED,
        serde_json::json!({
            "messageType": "device_user_login_succeeded",
            "messageId": delivery_id,
        }),
        &state,
    );
    Ok(())
}

fn validate_embedded_device_user_login_succeeded(
    state: &ClientViewState,
    auth: &AuthPayload,
) -> Result<()> {
    let expected_device_id = local_stable_device_id()
        .ok()
        .filter(|value| !value.trim().is_empty())
        .or_else(|| {
            state
                .device_id
                .as_deref()
                .map(str::trim)
                .filter(|value| !value.is_empty())
                .map(str::to_string)
        });
    if let Some(expected_device_id) = expected_device_id.as_deref() {
        if auth
            .device_id
            .as_deref()
            .map(str::trim)
            .filter(|value| !value.is_empty())
            != Some(expected_device_id)
        {
            anyhow::bail!("登录设备不匹配");
        }
    }
    Ok(())
}

fn ingest_embedded_device_ip_reassigned(value: &Value) -> Result<()> {
    let payload = value
        .get("payload")
        .ok_or_else(|| anyhow::anyhow!("device_ip_reassigned payload is missing"))?;
    let target_device_id = payload
        .get("deviceId")
        .and_then(Value::as_str)
        .unwrap_or_default()
        .trim();
    let virtual_ip = payload
        .get("virtualIp")
        .and_then(Value::as_str)
        .unwrap_or_default()
        .trim();
    if target_device_id.is_empty() || virtual_ip.is_empty() {
        return Ok(());
    }
    let prefix_len = payload
        .get("prefixLen")
        .and_then(Value::as_u64)
        .and_then(|value| u8::try_from(value).ok());
    let snapshot = runtime().snapshot();
    let mut session = load_session().context("load session for device ip reassignment")?;
    if session.device_id.as_deref().map(str::trim) != Some(target_device_id) {
        return Ok(());
    }
    session.virtual_ip = Some(virtual_ip.to_string());
    let assigned_virtual_ip = virtual_ip.to_string();
    let correlation_id = value
        .get("messageId")
        .and_then(Value::as_str)
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .map(str::to_string);
    let business_message_id = correlation_id.clone();
    let state = runtime()
        .call_named_if_revision(
            "embedded.mqtt.ip.sync",
            correlation_id,
            snapshot.revision,
            move |runtime| {
                persist_session(&session).context("persist reassigned ip")?;
                runtime.dispatch(ClientCommand::SyncAssignedIp(AssignedIpPayload {
                    virtual_ip: assigned_virtual_ip,
                    prefix_len,
                }))
            },
        )?
        .unwrap_or_else(|| runtime().snapshot().state);
    publish_embedded_business_event(
        BUSINESS_CONTROL_SYNC_CHANGED,
        serde_json::json!({
            "messageType": "device_ip_reassigned",
            "messageId": business_message_id,
            "deviceId": target_device_id,
            "virtualIp": virtual_ip,
        }),
        &state,
    );
    Ok(())
}

fn persist_embedded_client_message(value: &Value) -> Result<()> {
    let message = EmbeddedClientMessage::from_value(value);
    let session = load_session().context("load session for embedded client message")?;
    let local_device_id = session
        .device_id
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty());
    eprintln!(
        "SLAN_EMBEDDED_CLIENT_MESSAGE_ROUTE messageId={} fromDeviceId={} targetDeviceId={} localDeviceId={} activeNetworkId={}",
        message.message_id_str(),
        message.from_device_id_str(),
        message.target_device_id_str(),
        local_device_id.unwrap_or_default(),
        session.active_network_id.as_deref().unwrap_or_default(),
    );
    if let Some(target_device_id) = message.trimmed_target_device_id() {
        if Some(target_device_id) != local_device_id {
            eprintln!(
                "SLAN_EMBEDDED_CLIENT_MESSAGE_IGNORE messageId={} fromDeviceId={} targetDeviceId={} localDeviceId={}",
                message.message_id_str(),
                message.from_device_id_str(),
                target_device_id,
                local_device_id.unwrap_or_default(),
            );
            return Ok(());
        }
    }
    write_embedded_downstream_payload("client-v2-last-client-message.json", value)?;
    eprintln!(
        "SLAN_EMBEDDED_CLIENT_MESSAGE_RECV messageId={} fromDeviceId={} targetDeviceId={} bodyBytes={}",
        message.message_id_str(),
        message.from_device_id_str(),
        message.target_device_id_str(),
        message.body_len(),
    );
    eprintln!(
        "SLAN_EMBEDDED_CLIENT_MESSAGE_PERSIST path={} hasMessageId={} hasFromDeviceId={} hasBody={}",
        embedded_state_file("client-v2-last-client-message.json").display(),
        message.message_id.is_some(),
        message.from_device_id.is_some(),
        message.body.as_deref().is_some_and(|value| !value.is_empty()),
    );
    if maybe_reply_embedded_client_ping(
        message.trimmed_from_device_id(),
        message.trimmed_target_device_id(),
        message.body.as_deref(),
    )? {
        return Ok(());
    }
    let runtime_message = message.clone();
    let correlation_id = message.message_id.clone();
    let state = runtime().call_named(
        "embedded.mqtt.client_message.apply",
        correlation_id,
        move |runtime| {
            runtime.dispatch(ClientCommand::ApplyClientMessage(
                ClientMessageNoticePayload {
                    message_id: runtime_message.message_id,
                    from_device_id: runtime_message.from_device_id,
                    body: runtime_message.body,
                },
            ))
        },
    )?;
    eprintln!(
        "SLAN_EMBEDDED_CLIENT_MESSAGE_APPLIED messageId={} stateLastClientMessageId={} stateLastClientMessageFromDeviceId={} stateLastClientMessageBodyBytes={}",
        message.message_id_str(),
        state.last_client_message_id.as_deref().unwrap_or_default(),
        state.last_client_message_from_device_id.as_deref().unwrap_or_default(),
        state.last_client_message_body.as_deref().map(str::len).unwrap_or(0),
    );
    publish_embedded_business_event(
        BUSINESS_CONTROL_SYNC_CHANGED,
        serde_json::json!({
            "messageType": "client_message",
            "messageId": message.message_id,
            "fromDeviceId": message.from_device_id,
            "body": message.body,
        }),
        &state,
    );
    Ok(())
}

fn maybe_reply_embedded_client_ping(
    from_device_id: Option<&str>,
    target_device_id: Option<&str>,
    body: Option<&str>,
) -> Result<bool> {
    let Some(from_device_id) = from_device_id
        .map(str::trim)
        .filter(|value| !value.is_empty())
    else {
        return Ok(false);
    };
    let Some(target_device_id) = target_device_id
        .map(str::trim)
        .filter(|value| !value.is_empty())
    else {
        return Ok(false);
    };
    let Some((ping_id, sent_at_ms)) = body.and_then(client_message_mqtt::parse_client_ping_body)
    else {
        return Ok(false);
    };
    let session = load_session().context("load session for client ping reply")?;
    let Some(local_device_id) = session
        .device_id
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
    else {
        return Ok(true);
    };
    if local_device_id != target_device_id {
        return Ok(true);
    }
    if local_device_id == from_device_id {
        return Ok(true);
    }
    let Some(network_id) = session
        .active_network_id
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
    else {
        return Ok(true);
    };
    let Some(mqtt) = session.mqtt.as_ref() else {
        return Ok(true);
    };
    let replied_at_ms = current_timestamp_ms();
    let pong_body =
        client_message_mqtt::build_client_pong_body(&ping_id, sent_at_ms, replied_at_ms);
    client_message_mqtt::publish_client_message(
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
    )
    .context("publish embedded client ping reply")?;
    Ok(true)
}

fn write_embedded_downstream_payload(file_name: &str, value: &Value) -> Result<()> {
    let path = embedded_state_file(file_name);
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent).with_context(|| format!("create {}", parent.display()))?;
    }
    let payload = serde_json::to_vec_pretty(value).context("encode downstream payload")?;
    fs::write(&path, payload).with_context(|| format!("write {}", path.display()))
}

fn persist_embedded_mqtt_publish_summary(
    topic: &str,
    message_type: Option<&str>,
    payload: &[u8],
) -> Result<()> {
    let decoded = serde_json::from_slice::<Value>(payload).ok();
    let payload_value = decoded
        .as_ref()
        .and_then(|value| value.get("payload"))
        .unwrap_or(&Value::Null);
    let summary = serde_json::json!({
        "topic": topic,
        "messageType": message_type,
        "payloadBytes": payload.len(),
        "messageId": decoded
            .as_ref()
            .and_then(|value| value.get("messageId"))
            .and_then(Value::as_str),
        "fromDeviceId": payload_value
            .get("fromDeviceId")
            .and_then(Value::as_str),
        "targetDeviceId": payload_value
            .get("targetDeviceId")
            .and_then(Value::as_str),
    });
    write_embedded_downstream_payload("client-v2-last-mqtt-publish.json", &summary)
}

#[derive(Debug, Clone, Default)]
struct EmbeddedClientMessage {
    message_id: Option<String>,
    from_device_id: Option<String>,
    target_device_id: Option<String>,
    body: Option<String>,
}

impl EmbeddedClientMessage {
    fn from_value(value: &Value) -> Self {
        let payload = value.get("payload").unwrap_or(value);
        Self {
            message_id: payload
                .get("messageId")
                .and_then(Value::as_str)
                .map(str::to_string),
            from_device_id: payload
                .get("fromDeviceId")
                .and_then(Value::as_str)
                .map(str::to_string),
            target_device_id: payload
                .get("targetDeviceId")
                .and_then(Value::as_str)
                .map(str::to_string),
            body: payload
                .get("body")
                .and_then(Value::as_str)
                .map(str::to_string),
        }
    }

    fn message_id_str(&self) -> &str {
        self.message_id.as_deref().unwrap_or_default()
    }

    fn from_device_id_str(&self) -> &str {
        self.from_device_id.as_deref().unwrap_or_default()
    }

    fn target_device_id_str(&self) -> &str {
        self.target_device_id.as_deref().unwrap_or_default()
    }

    fn body_len(&self) -> usize {
        self.body.as_deref().map(str::len).unwrap_or(0)
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

fn persist_embedded_downstream_summary(value: &Value) -> Result<()> {
    let payload = value.get("payload").unwrap_or(value);
    let message = EmbeddedClientMessage::from_value(value);
    let message_id = value
        .get("messageId")
        .and_then(Value::as_str)
        .map(str::to_string)
        .or(message.message_id.clone());
    let summary = serde_json::json!({
        "type": value.get("type").and_then(Value::as_str),
        "messageId": message_id,
        "fromDeviceId": message.from_device_id,
        "targetDeviceId": message.target_device_id,
        "networkId": payload
            .get("networkId")
            .and_then(Value::as_str),
        "bodyLength": Some(message.body_len()),
    });
    write_embedded_downstream_payload("client-v2-last-downstream-summary.json", &summary)
}

fn embedded_state_file(file_name: &str) -> PathBuf {
    app_data_dir().join("SLAN").join(file_name)
}

fn merge_persisted_client_message_into_state(state: &mut ClientViewState) {
    if state.last_client_message_id.is_some()
        && state.last_client_message_from_device_id.is_some()
        && state.last_client_message_body.is_some()
    {
        return;
    }
    let Ok(payload) = fs::read_to_string(embedded_state_file("client-v2-last-client-message.json"))
    else {
        return;
    };
    let Ok(value) = serde_json::from_str::<Value>(&payload) else {
        return;
    };
    let message = EmbeddedClientMessage::from_value(&value);
    if state.last_client_message_id.is_none() {
        state.last_client_message_id = message.message_id;
    }
    if state.last_client_message_from_device_id.is_none() {
        state.last_client_message_from_device_id = message.from_device_id;
    }
    if state.last_client_message_body.is_none() {
        state.last_client_message_body = message.body;
    }
}

fn load_embedded_downstream_summary() -> Option<Value> {
    let payload = fs::read_to_string(embedded_state_file(
        "client-v2-last-downstream-summary.json",
    ))
    .ok()?;
    serde_json::from_str::<Value>(&payload).ok()
}

fn load_embedded_mqtt_publish_summary() -> Option<Value> {
    let payload =
        fs::read_to_string(embedded_state_file("client-v2-last-mqtt-publish.json")).ok()?;
    serde_json::from_str::<Value>(&payload).ok()
}

fn publish_embedded_business_event(
    business_type: &str,
    business_data: Value,
    state: &ClientViewState,
) {
    let mut event_state = state.clone();
    merge_persisted_client_message_into_state(&mut event_state);
    let business_data = match serde_json::to_value(event_state) {
        Ok(Value::Object(mut object)) => {
            if let Value::Object(extra) = business_data {
                for (key, value) in extra {
                    object.insert(key, value);
                }
            }
            Value::Object(object)
        }
        _ => business_data,
    };
    let event = runtime().publish_event(business_type, business_data);
    eprintln!(
        "SLAN_EMBEDDED_BUSINESS_EVENT revision={} businessType={} messageType={} lastClientMessageId={} lastClientMessageFromDeviceId={} lastClientMessageBodyBytes={}",
        event.revision,
        event.business_type,
        event.business_data
            .get("messageType")
            .and_then(Value::as_str)
            .unwrap_or_default(),
        event.business_data
            .get("lastClientMessageId")
            .and_then(Value::as_str)
            .unwrap_or_default(),
        event.business_data
            .get("lastClientMessageFromDeviceId")
            .and_then(Value::as_str)
            .unwrap_or_default(),
        event.business_data
            .get("lastClientMessageBody")
            .and_then(Value::as_str)
            .map(str::len)
            .unwrap_or(0),
    );
}

fn watch_embedded_business_event(
    last_revision: u64,
    follow_latest: bool,
    requested_stream_id: Option<&str>,
    timeout: Duration,
) -> WatchBusinessEventResponse {
    let event_hub = runtime().events();
    let stream_reset =
        requested_stream_id.is_some_and(|stream_id| stream_id != event_hub.stream_id());
    let effective_last_revision = if follow_latest && !stream_reset {
        event_hub.latest_revision()
    } else {
        last_revision
    };
    let read = if stream_reset {
        event_hub.read_after(0)
    } else {
        event_hub.wait_read_after(effective_last_revision, timeout)
    };
    if let Some(event) = read.event {
        return WatchBusinessEventResponse {
            revision: event.revision,
            stream_id: event_hub.stream_id().to_string(),
            stream_reset,
            oldest_available_revision: read.oldest_available_revision,
            latest_revision: read.latest_revision,
            replay_gap: read.replay_gap,
            event_id: Some(event.event_id),
            business_type: event.business_type,
            business_data: event.business_data,
            published_at_ms: Some(event.published_at_ms),
            replayed: event.revision < read.latest_revision,
            snapshot: refresh_state(),
        };
    }
    WatchBusinessEventResponse {
        revision: if stream_reset {
            read.latest_revision
        } else {
            effective_last_revision
        },
        stream_id: event_hub.stream_id().to_string(),
        stream_reset,
        oldest_available_revision: read.oldest_available_revision,
        latest_revision: read.latest_revision,
        replay_gap: read.replay_gap,
        event_id: None,
        business_type: BUSINESS_STATE_CHANGED.to_string(),
        business_data: serde_json::json!({}),
        published_at_ms: None,
        replayed: false,
        snapshot: refresh_state(),
    }
}

fn send_embedded_client_message(input: SendClientMessageRequest) -> Result<Value> {
    let session = ensure_device_session().context("ensure device before send message")?;
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
    eprintln!(
        "SLAN_EMBEDDED_CLIENT_MESSAGE_SEND networkId={} fromDeviceId={} targetDeviceId={} bodyBytes={}",
        network_id,
        from_device_id,
        input.target_device_id,
        input.body.len(),
    );
    let client = ControlPlaneClient::from_env();
    let response = client.send_client_message(
        session_device_api_token(&session),
        network_id,
        from_device_id,
        &input.target_device_id,
        &input.body,
        input.metadata.clone(),
    )?;
    eprintln!(
        "SLAN_EMBEDDED_CLIENT_MESSAGE_SEND_OK messageId={} topic={} transport={} qos={}",
        response.message_id, response.topic, response.transport, response.qos,
    );
    Ok(serde_json::json!({
        "messageId": response.message_id,
        "topic": response.topic,
        "transport": response.transport,
        "qos": response.qos,
    }))
}

fn register_embedded_test_user(input: RegisterTestUserRequest) -> Result<Value> {
    let auth = ControlPlaneClient::from_env()
        .register_user_with_password(&input.email, &input.password)
        .context("register embedded test user")?;
    Ok(serde_json::to_value(auth).context("encode registered auth payload")?)
}

fn embedded_control_status() -> Value {
    match load_session() {
        Ok(session) => {
            let active_network_id = session
                .active_network_id
                .as_deref()
                .filter(|value| !value.trim().is_empty())
                .map(str::to_string)
                .or_else(|| {
                    runtime_network_state_store()
                        .read(|state| state.active_network_id.clone())
                        .filter(|value| !value.trim().is_empty())
                })
                .or_else(|| {
                    crate::network_module::network_module_snapshot()
                        .configs
                        .into_iter()
                        .next()
                        .map(|config| config.network_id)
                        .filter(|value| !value.trim().is_empty())
                });
            let (
                mqtt_connected,
                mqtt_last_error,
                mqtt_last_message_topic,
                mqtt_last_message_type,
                mqtt_network_event_topics,
                mqtt_network_event_subscribed,
            ) = {
                let guard = mqtt_connection()
                    .lock()
                    .expect("embedded mqtt mutex poisoned");
                guard
                    .as_ref()
                    .and_then(|connection| {
                        let session_device_id = session
                            .device_id
                            .as_deref()
                            .map(str::trim)
                            .unwrap_or_default();
                        let connection_device_id = connection
                            .device_id
                            .as_deref()
                            .map(str::trim)
                            .unwrap_or_default();
                        if !session_device_id.is_empty()
                            && session_device_id != connection_device_id
                        {
                            return None;
                        }
                        Some((
                            connection.connected && connection.network_event_subscribed,
                            connection.last_error.clone(),
                            connection.last_message_topic.clone(),
                            connection.last_message_type.clone(),
                            connection.network_event_topics.clone(),
                            connection.network_event_subscribed,
                        ))
                    })
                    .unwrap_or((false, None, None, None, Vec::new(), false))
            };
            let mqtt_last_error = mqtt_last_error.or_else(|| {
                mqtt_last_error_store()
                    .lock()
                    .expect("embedded mqtt last error mutex poisoned")
                    .clone()
            });
            let last_mqtt_publish_summary = load_embedded_mqtt_publish_summary();
            let mqtt_credential_ready = session.mqtt.is_some();
            let device_ready = session
                .device_id
                .as_deref()
                .is_some_and(|value| !value.trim().is_empty());
            let network_ready = active_network_id.is_some();
            let mut missing = Vec::new();
            if !mqtt_credential_ready {
                missing.push("mqttCredential");
            }
            if !device_ready {
                missing.push("deviceId");
            }
            if !network_ready && session.session_kind != "prelogin" {
                missing.push("activeNetworkId");
            }
            serde_json::json!({
                "mqttCredentialReady": mqtt_credential_ready,
                "controlSessionReady": device_ready,
                "ready": mqtt_credential_ready && device_ready,
                "missing": missing,
                "mqttExpiresAt": session.mqtt.as_ref().and_then(|credential| credential.expires_at),
                "mqttConnected": mqtt_connected,
                "mqttLastError": mqtt_last_error,
                "mqttLastMessageTopic": mqtt_last_message_topic,
                "mqttLastMessageType": mqtt_last_message_type,
                "lastMqttPublishSummary": last_mqtt_publish_summary,
                "mqttNetworkEventTopics": mqtt_network_event_topics,
                "mqttNetworkEventSubscribed": mqtt_network_event_subscribed,
                "activeNetworkId": active_network_id,
                "deviceId": session.device_id,
            })
        }
        Err(_) => serde_json::json!({
            "mqttCredentialReady": false,
            "controlSessionReady": false,
            "ready": false,
            "missing": ["session"],
        }),
    }
}

fn dispatch_embedded(
    command: ClientCommand,
    correlation_id: Option<String>,
) -> Result<ClientViewState> {
    if matches!(command, ClientCommand::EnableNetwork) {
        let _ = platform_network_config().context("activate embedded network")?;
        let state = runtime().call_named("embedded.network.enable", correlation_id, |runtime| {
            runtime
                .dispatch(ClientCommand::EnableNetwork)
                .context("enable embedded network")
        })?;
        report_runtime_state(&state);
        return Ok(state);
    }
    match command {
        ClientCommand::OpenClientLogin => {
            let session = prepare_client_login_session(std::env::consts::OS)
                .context("prepare embedded client login")?;
            connect_embedded_control_mqtt_with_session(&session)
                .context("connect mqtt after embedded client login prepare")?;
            runtime().call_named("embedded.login.open", correlation_id, move |runtime| {
                Ok(runtime.request_browser_login(session.device_id))
            })
        }
        ClientCommand::LoginWithPassword(payload) => {
            let expected_state = runtime().snapshot().state;
            let auth = ControlPlaneClient::from_env()
                .login_with_password(&payload.email, &payload.password)
                .context("password login")?;
            let prepared = recover_embedded_session_from_auth_payload(auth, "password login")
                .context("hydrate password session")?;
            let session = prepared.session.clone();
            let state = runtime().call_named(
                "embedded.login.password",
                correlation_id,
                move |runtime| {
                    commit_embedded_prepared_login(
                        runtime,
                        &prepared,
                        expected_state.device_id.as_deref(),
                        expected_state.signed_in,
                    )
                },
            )?;
            clear_embedded_mqtt_connection();
            connect_embedded_control_mqtt_with_session(&session)
                .context("connect mqtt after login")?;
            Ok(state)
        }
        ClientCommand::ApplyDeviceUserLogin(payload) => {
            let expected_state = runtime().snapshot().state;
            validate_embedded_device_user_login_succeeded(&expected_state, &payload)?;
            let prepared = recover_embedded_session_from_auth_payload(payload, "device user login")
                .context("hydrate device user login")?;
            let session = prepared.session.clone();
            let state =
                runtime().call_named("embedded.login.apply", correlation_id, move |runtime| {
                    commit_embedded_prepared_login(
                        runtime,
                        &prepared,
                        expected_state.device_id.as_deref(),
                        expected_state.signed_in,
                    )
                })?;
            clear_embedded_mqtt_connection();
            connect_embedded_control_mqtt_with_session(&session)
                .context("connect mqtt after device user login")?;
            Ok(state)
        }
        ClientCommand::Logout => {
            let expected_revision = runtime().snapshot().revision;
            let state = runtime()
                .call_named_if_revision(
                    "embedded.logout",
                    correlation_id,
                    expected_revision,
                    |runtime| {
                        remove_session()?;
                        runtime.dispatch(ClientCommand::Logout).context("logout")
                    },
                )?
                .ok_or_else(|| anyhow::anyhow!("stale embedded logout"))?;
            clear_embedded_mqtt_connection();
            set_embedded_mqtt_last_error(None);
            let _ = fs::remove_file(embedded_state_file("client-v2-last-client-message.json"));
            let _ = fs::remove_file(embedded_state_file(
                "client-v2-last-downstream-summary.json",
            ));
            let _ = fs::remove_file(embedded_state_file("client-v2-last-mqtt-publish.json"));
            Ok(state)
        }
        ClientCommand::DisableNetwork => {
            runtime().call_named("embedded.network.disable", correlation_id, move |runtime| {
                runtime
                    .dispatch(ClientCommand::DisableNetwork)
                    .context("disable embedded network")
            })
        }
        other => runtime().call_named("embedded.dispatch", correlation_id, move |runtime| {
            runtime.dispatch(other).context("dispatch command")
        }),
    }
}

fn refresh_state() -> ClientViewState {
    let mut state = runtime().snapshot().state;
    merge_persisted_client_message_into_state(&mut state);
    eprintln!(
        "SLAN_EMBEDDED_REFRESH_STATE lastClientMessageId={} lastClientMessageFromDeviceId={} lastClientMessageBodyBytes={}",
        state.last_client_message_id.as_deref().unwrap_or_default(),
        state.last_client_message_from_device_id.as_deref().unwrap_or_default(),
        state.last_client_message_body.as_deref().map(str::len).unwrap_or(0),
    );
    state
}

fn local_session_json() -> Result<String> {
    let session = load_session().ok();
    serde_json::to_string(&serde_json::json!({
        "signedIn": session.is_some(),
        "userId": session.as_ref().map(|value| value.user_id.clone()),
        "userLabel": session.as_ref().map(|value| value.user_label.clone()),
        "deviceId": session.as_ref().and_then(|value| value.device_id.clone()),
        "activeNetworkId": session.as_ref().and_then(|value| value.active_network_id.clone()),
        "virtualIp": session.as_ref().and_then(|value| value.virtual_ip.clone()),
        "mqttConfigured": session.as_ref().and_then(|value| value.mqtt.as_ref()).is_some(),
    }))
    .context("encode local session")
}

#[cfg(test)]
mod tests {
    use client_core::{AuthPayload, ClientCommand, NetworkRuntimeState, PathKind};
    use serde_json::Value;
    use std::time::Duration;

    use crate::session_store::{persist_session, PersistedSession};

    use super::{
        embedded_handle_request_json, reconcile_embedded_active_network_state,
        rewrite_local_mqtt_broker_host, runtime, url_host, watch_embedded_business_event,
        BUSINESS_NETWORK_RUNTIME_CHANGED,
    };

    fn request(method: &str, args: Value) -> Value {
        let response = embedded_handle_request_json(
            &serde_json::json!({
                "method": method,
                "args": args,
            })
            .to_string(),
        );
        serde_json::from_str(&response).expect("embedded response json")
    }

    #[test]
    fn embedded_peer_paths_prefer_lan_then_public_udp() {
        let peer = crate::control_plane::ControlPeer {
            node_id: "node-peer".to_string(),
            relay_allowed: true,
            virtual_ips: vec!["10.0.0.2".to_string()],
            endpoints: vec![
                crate::control_plane::ControlEndpoint {
                    endpoint_type: "direct_udp".to_string(),
                    address: "203.0.113.20:49152".to_string(),
                    updated_at: 1,
                },
                crate::control_plane::ControlEndpoint {
                    endpoint_type: "lan_udp".to_string(),
                    address: "192.168.1.20:49152".to_string(),
                    updated_at: 1,
                },
            ],
        };
        let relay = crate::control_plane::RelayCandidate {
            endpoint_id: "relay-1".to_string(),
            transport: "udp".to_string(),
            address: "relay.example:3478".to_string(),
            country_code: None,
            region_id: None,
            cluster_id: None,
            reachable: true,
            observed_rtt_ms: None,
            path_score: None,
            selected: true,
        };

        let paths = super::embedded_peer_path_configs(
            &[peer],
            "node-local",
            &relay,
            PathKind::RelayUdp,
            &[],
        );

        assert_eq!(paths.len(), 1);
        assert_eq!(paths[0].candidates.len(), 2);
        assert_eq!(paths[0].candidates[0].kind, PathKind::LanUdp);
        assert_eq!(paths[0].candidates[1].kind, PathKind::DirectUdp);
    }

    #[test]
    fn mqtt_generation_is_not_reused_after_connection_clear() {
        let _lock = crate::test_env_lock();
        let device_id = Some("device-generation-test".to_string());
        let topic = "slan/devices/device-generation-test/control/down";
        let first = super::next_embedded_mqtt_generation();
        {
            let mut connection = super::mqtt_connection()
                .lock()
                .expect("embedded mqtt mutex poisoned");
            *connection = Some(super::EmbeddedMqttConnection {
                generation: first,
                device_id: device_id.clone(),
                downstream_topic: topic.to_string(),
                network_event_topics: Vec::new(),
                connected: true,
                network_event_subscribed: false,
                last_message_topic: None,
                last_message_type: None,
                last_error: None,
            });
        }
        super::clear_embedded_mqtt_connection();
        let second = super::next_embedded_mqtt_generation();
        assert!(second > first);
        {
            let mut connection = super::mqtt_connection()
                .lock()
                .expect("embedded mqtt mutex poisoned");
            *connection = Some(super::EmbeddedMqttConnection {
                generation: second,
                device_id: device_id.clone(),
                downstream_topic: topic.to_string(),
                network_event_topics: Vec::new(),
                connected: true,
                network_event_subscribed: false,
                last_message_topic: None,
                last_message_type: None,
                last_error: None,
            });
        }

        assert!(!super::embedded_mqtt_generation_active(
            first, &device_id, topic
        ));
        assert!(super::embedded_mqtt_generation_active(
            second, &device_id, topic
        ));
        super::clear_embedded_mqtt_connection();
    }

    #[test]
    fn local_state_returns_client_state() {
        let response = request("localState", serde_json::json!({}));

        assert!(response.get("signedIn").and_then(Value::as_bool).is_some());
        assert!(response
            .get("networkEnabled")
            .and_then(Value::as_bool)
            .is_some());
        assert!(response
            .get("switchEnabled")
            .and_then(Value::as_bool)
            .is_some());
    }

    #[test]
    fn embedded_dispatch_correlation_reaches_response_and_runtime_actor() {
        let response = request(
            "dispatch",
            serde_json::json!({
                "type": "disableNetwork",
                "requestId": "mobile-request-1",
            }),
        );

        assert_eq!(
            response.get("requestId").and_then(Value::as_str),
            Some("mobile-request-1")
        );
        assert_eq!(
            runtime()
                .diagnostics()
                .last_command
                .and_then(|command| command.correlation_id)
                .as_deref(),
            Some("mobile-request-1")
        );
    }

    #[test]
    fn rewrites_loopback_mqtt_broker_for_mobile_override_host() {
        assert_eq!(
            rewrite_local_mqtt_broker_host("mqtt://127.0.0.1:1883", "10.0.2.2"),
            "mqtt://10.0.2.2:1883"
        );
        assert_eq!(
            rewrite_local_mqtt_broker_host("mqtt://localhost:1883/down", "10.0.2.2"),
            "mqtt://10.0.2.2:1883/down"
        );
        assert_eq!(
            rewrite_local_mqtt_broker_host("mqtt://broker.example:1883", "10.0.2.2"),
            "mqtt://broker.example:1883"
        );
        assert_eq!(
            url_host("http://10.0.2.2:28080"),
            Some("10.0.2.2".to_string())
        );
    }

    #[test]
    fn ingest_platform_runtime_state_updates_state() {
        let _lock = crate::test_env_lock();
        let state_dir = std::env::temp_dir().join(format!(
            "slan-embedded-runtime-state-test-{}",
            std::process::id()
        ));
        let _ = std::fs::remove_dir_all(&state_dir);
        std::fs::create_dir_all(&state_dir).expect("create temp state dir");
        let previous_state_dir = std::env::var_os("SLAN_STATE_DIR");
        std::env::set_var("SLAN_STATE_DIR", &state_dir);
        runtime().events().reset();

        let response = request(
            "ingestPlatformRuntimeState",
            serde_json::json!({
                "runtimeState": {
                    "adapterPresent": true,
                    "networkEnabled": true,
                    "virtualIp": "10.0.0.99"
                },
                "platform": "test"
            }),
        );

        assert_eq!(
            response.get("accepted").and_then(Value::as_bool),
            Some(true)
        );
        let state = response.get("state").expect("state");
        assert_eq!(
            state.get("networkEnabled").and_then(Value::as_bool),
            Some(true)
        );
        assert_eq!(
            state.get("virtualIp").and_then(Value::as_str),
            Some("10.0.0.99")
        );
        let event = watch_embedded_business_event(0, false, None, Duration::ZERO);
        assert_eq!(event.business_type, BUSINESS_NETWORK_RUNTIME_CHANGED);
        assert_eq!(
            event
                .business_data
                .get("messageType")
                .and_then(Value::as_str),
            Some("platform_runtime_state")
        );
        assert_eq!(
            event.business_data.get("platform").and_then(Value::as_str),
            Some("test")
        );

        let response = request(
            "ingestPlatformRuntimeState",
            serde_json::json!({
                "runtimeState": {
                    "adapterPresent": false,
                    "networkEnabled": false,
                    "virtualIp": null
                },
                "platform": "test",
                "error": "expected platform failure"
            }),
        );
        assert_eq!(
            response.pointer("/state/error").and_then(Value::as_str),
            Some("expected platform failure")
        );
        let error_event =
            watch_embedded_business_event(event.revision, false, None, Duration::ZERO);
        assert_eq!(error_event.business_type, BUSINESS_NETWORK_RUNTIME_CHANGED);
        assert_eq!(
            error_event
                .business_data
                .get("error")
                .and_then(Value::as_str),
            Some("expected platform failure")
        );

        if let Some(value) = previous_state_dir {
            std::env::set_var("SLAN_STATE_DIR", value);
        } else {
            std::env::remove_var("SLAN_STATE_DIR");
        }
        let _ = std::fs::remove_dir_all(&state_dir);
    }

    #[test]
    fn unsupported_method_returns_error_json() {
        let response = request("missingMethod", serde_json::json!({}));

        assert!(response
            .get("error")
            .and_then(Value::as_str)
            .unwrap_or_default()
            .contains("unsupported embedded service method"));
    }

    #[test]
    fn local_business_event_watch_returns_snapshot_without_native_queue() {
        let _lock = crate::test_env_lock();
        let response = request(
            "localBusinessEventWatch",
            serde_json::json!({
                "lastRevision": 7,
                "timeoutMs": 1000
            }),
        );

        assert_eq!(response.get("revision").and_then(Value::as_u64), Some(7));
        assert_eq!(
            response.get("businessType").and_then(Value::as_str),
            Some("state.changed")
        );
        assert!(response
            .get("snapshot")
            .and_then(Value::as_object)
            .is_some());
    }

    #[test]
    fn local_business_event_watch_wakes_when_rust_event_arrives() {
        let _lock = crate::test_env_lock();
        let last_revision = runtime().events().latest_revision();
        let watcher = std::thread::spawn(move || {
            request(
                "localBusinessEventWatch",
                serde_json::json!({
                    "lastRevision": last_revision,
                    "timeoutMs": 1000
                }),
            )
        });
        std::thread::sleep(Duration::from_millis(20));
        runtime().publish_event(
            "test.embedded.watch",
            serde_json::json!({"eventId": "test-embedded-watch-wakeup"}),
        );

        let response = watcher.join().expect("join embedded event watcher");
        assert_eq!(
            response.get("businessType").and_then(Value::as_str),
            Some("test.embedded.watch")
        );
        assert!(response
            .get("revision")
            .and_then(Value::as_u64)
            .is_some_and(|revision| revision > last_revision));
    }

    #[test]
    fn local_control_status_returns_mobile_embedded_shape() {
        let response = request("localControlStatus", serde_json::json!({}));

        assert!(response
            .get("mqttCredentialReady")
            .and_then(Value::as_bool)
            .is_some());
        assert!(response
            .get("controlSessionReady")
            .and_then(Value::as_bool)
            .is_some());
        assert!(response.get("ready").and_then(Value::as_bool).is_some());
        assert!(response.get("missing").and_then(Value::as_array).is_some());
    }

    #[test]
    fn embedded_login_downstream_ack_identity_uses_message_id() {
        let value = serde_json::json!({
            "type": "device_user_login_succeeded",
            "messageId": "login-msg-1",
            "payload": {
                "deviceId": "dev-1"
            }
        });

        let identity = super::embedded_downstream_ack_identity(&value).expect("ack identity");

        assert_eq!(identity.0, "login-msg-1");
        assert_eq!(identity.1, "deviceUserLoginSucceeded");
    }

    #[test]
    fn embedded_unknown_downstream_has_no_business_ack() {
        let value = serde_json::json!({
            "type": "unknown_message",
            "messageId": "msg-1"
        });

        assert!(super::embedded_downstream_ack_identity(&value).is_none());
    }

    #[test]
    fn client_message_ingest_advances_embedded_business_event() {
        let _lock = crate::test_env_lock();
        let state_dir =
            std::env::temp_dir().join(format!("slan-embedded-event-test-{}", std::process::id()));
        let _ = std::fs::remove_dir_all(&state_dir);
        std::fs::create_dir_all(&state_dir).expect("create temp state dir");
        let previous_state_dir = std::env::var_os("SLAN_STATE_DIR");
        std::env::set_var("SLAN_STATE_DIR", &state_dir);
        super::runtime().events().reset();
        super::runtime()
            .call(|runtime| {
                runtime.dispatch(ClientCommand::Logout)?;
                Ok(())
            })
            .expect("reset embedded runtime");
        let mut session = PersistedSession::empty();
        session.device_id = Some("device-1".to_string());
        persist_session(&session).expect("persist embedded test session");
        super::ingest_embedded_downstream_publish(
            serde_json::json!({
                "type": "client_message",
                "payload": {
                    "messageId": "msg-1",
                    "fromDeviceId": "ios-peer",
                    "targetDeviceId": "device-1",
                    "body": "hello"
                }
            })
            .to_string()
            .as_bytes(),
        )
        .expect("ingest client message");

        let response = request(
            "localBusinessEventWatch",
            serde_json::json!({
                "lastRevision": 0,
                "timeoutMs": 1000
            }),
        );

        assert_eq!(
            response.get("businessType").and_then(Value::as_str),
            Some("control.sync.changed")
        );
        assert_eq!(
            response
                .pointer("/businessData/messageType")
                .and_then(Value::as_str),
            Some("client_message")
        );
        assert_eq!(
            response
                .pointer("/businessData/lastClientMessageBody")
                .and_then(Value::as_str),
            Some("hello")
        );
        if let Some(value) = previous_state_dir {
            std::env::set_var("SLAN_STATE_DIR", value);
        } else {
            std::env::remove_var("SLAN_STATE_DIR");
        }
        let _ = std::fs::remove_dir_all(&state_dir);
    }

    #[test]
    fn client_message_ingest_ignores_other_embedded_targets_without_persisting() {
        let _lock = crate::test_env_lock();
        let state_dir = std::env::temp_dir().join(format!(
            "slan-embedded-ignore-event-test-{}",
            std::process::id()
        ));
        let _ = std::fs::remove_dir_all(&state_dir);
        std::fs::create_dir_all(&state_dir).expect("create temp state dir");
        let previous_state_dir = std::env::var_os("SLAN_STATE_DIR");
        std::env::set_var("SLAN_STATE_DIR", &state_dir);
        super::runtime().events().reset();
        super::runtime()
            .call(|runtime| {
                runtime.dispatch(ClientCommand::Logout)?;
                Ok(())
            })
            .expect("reset embedded runtime");
        let mut session = PersistedSession::empty();
        session.device_id = Some("device-1".to_string());
        persist_session(&session).expect("persist embedded test session");
        super::ingest_embedded_downstream_publish(
            serde_json::json!({
                "type": "client_message",
                "payload": {
                    "messageId": "msg-2",
                    "fromDeviceId": "ios-peer",
                    "targetDeviceId": "device-2",
                    "body": "ignore"
                }
            })
            .to_string()
            .as_bytes(),
        )
        .expect("ingest ignored client message");

        let state = request("localState", serde_json::json!({}));
        assert_eq!(
            state.get("lastClientMessageId").and_then(Value::as_str),
            None
        );
        assert_eq!(
            state
                .get("lastClientMessageFromDeviceId")
                .and_then(Value::as_str),
            None
        );
        assert_eq!(
            state.get("lastClientMessageBody").and_then(Value::as_str),
            None
        );
        assert!(
            !super::embedded_state_file("client-v2-last-client-message.json").exists(),
            "ignored embedded client_message should not persist last message state"
        );

        if let Some(value) = previous_state_dir {
            std::env::set_var("SLAN_STATE_DIR", value);
        } else {
            std::env::remove_var("SLAN_STATE_DIR");
        }
        let _ = std::fs::remove_dir_all(&state_dir);
    }

    #[test]
    fn later_embedded_business_event_keeps_last_client_message_fields() {
        let _lock = crate::test_env_lock();
        let state_dir = std::env::temp_dir().join(format!(
            "slan-embedded-client-message-carry-test-{}",
            std::process::id()
        ));
        let _ = std::fs::remove_dir_all(&state_dir);
        std::fs::create_dir_all(&state_dir).expect("create temp state dir");
        let previous_state_dir = std::env::var_os("SLAN_STATE_DIR");
        std::env::set_var("SLAN_STATE_DIR", &state_dir);
        super::runtime().events().reset();
        super::runtime()
            .call(|runtime| {
                runtime.dispatch(ClientCommand::Logout)?;
                Ok(())
            })
            .expect("reset embedded runtime");
        let mut session = PersistedSession::empty();
        session.device_id = Some("device-1".to_string());
        persist_session(&session).expect("persist embedded test session");

        super::ingest_embedded_downstream_publish(
            serde_json::json!({
                "type": "client_message",
                "payload": {
                    "messageId": "msg-1",
                    "fromDeviceId": "ios-peer",
                    "targetDeviceId": "device-1",
                    "body": "hello"
                }
            })
            .to_string()
            .as_bytes(),
        )
        .expect("ingest client message");
        let client_message_revision = super::runtime().events().latest_revision();

        super::ingest_embedded_downstream_publish(
            serde_json::json!({
                "type": "network_event",
                "networkId": "net-1",
                "version": 2,
                "eventId": "evt-2",
                "eventType": "network_config_changed",
                "occurredAt": 2,
                "payload": {
                    "network": {
                        "networkId": "net-1",
                        "name": "",
                        "tags": [],
                        "defaultAclPolicy": "",
                        "updatedAt": 2
                    }
                }
            })
            .to_string()
            .as_bytes(),
        )
        .expect("ingest network event");

        let response = request(
            "localBusinessEventWatch",
            serde_json::json!({
                "lastRevision": client_message_revision,
                "timeoutMs": 1000
            }),
        );

        assert_eq!(
            response.get("businessType").and_then(Value::as_str),
            Some("control.sync.changed")
        );
        assert_eq!(
            response
                .pointer("/businessData/messageType")
                .and_then(Value::as_str),
            Some("network_event")
        );
        assert_eq!(
            response
                .pointer("/businessData/lastClientMessageBody")
                .and_then(Value::as_str),
            Some("hello")
        );
        assert_eq!(
            response
                .pointer("/snapshot/lastClientMessageBody")
                .and_then(Value::as_str),
            Some("hello")
        );

        if let Some(value) = previous_state_dir {
            std::env::set_var("SLAN_STATE_DIR", value);
        } else {
            std::env::remove_var("SLAN_STATE_DIR");
        }
        let _ = std::fs::remove_dir_all(&state_dir);
    }

    #[test]
    fn embedded_active_network_reconcile_skips_when_network_is_disabled() {
        let _lock = crate::test_env_lock();
        let state_dir = std::env::temp_dir().join(format!(
            "slan-embedded-reconcile-disabled-test-{}",
            std::process::id()
        ));
        let _ = std::fs::remove_dir_all(&state_dir);
        std::fs::create_dir_all(&state_dir).expect("create temp state dir");
        let previous_state_dir = std::env::var_os("SLAN_STATE_DIR");
        std::env::set_var("SLAN_STATE_DIR", &state_dir);
        let _ = std::fs::remove_file(super::embedded_state_file(
            "client-v2-last-client-message.json",
        ));

        super::runtime().events().reset();
        super::runtime()
            .call(|runtime| {
                runtime.dispatch(ClientCommand::Logout)?;
                Ok(())
            })
            .expect("reset embedded runtime");
        reconcile_embedded_active_network_state();
        let after = watch_embedded_business_event(0, false, None, Duration::ZERO).revision;
        assert_eq!(after, 0);

        if let Some(value) = previous_state_dir {
            std::env::set_var("SLAN_STATE_DIR", value);
        } else {
            std::env::remove_var("SLAN_STATE_DIR");
        }
        let _ = std::fs::remove_dir_all(&state_dir);
    }

    #[test]
    fn embedded_downstream_ack_identity_maps_network_event_to_reconcile_network_state() {
        let value = serde_json::json!({
            "type": "network_event",
            "eventId": "event-msg-1",
            "networkId": "net-1",
            "version": 7,
            "eventType": "snapshot",
            "occurredAt": 1,
            "payload": {}
        });

        assert_eq!(
            super::embedded_downstream_ack_identity(&value),
            Some(("event-msg-1".to_string(), "reconcileNetworkState"))
        );
    }

    #[test]
    fn embedded_active_network_reconcile_emits_business_event_when_network_is_enabled() {
        let _lock = crate::test_env_lock();
        let state_dir = std::env::temp_dir().join(format!(
            "slan-embedded-reconcile-test-{}",
            std::process::id()
        ));
        let _ = std::fs::remove_dir_all(&state_dir);
        std::fs::create_dir_all(&state_dir).expect("create temp state dir");
        let previous_state_dir = std::env::var_os("SLAN_STATE_DIR");
        std::env::set_var("SLAN_STATE_DIR", &state_dir);
        let _ = std::fs::remove_file(super::embedded_state_file(
            "client-v2-last-client-message.json",
        ));

        super::runtime().events().reset();

        super::runtime()
            .call(|runtime| {
                runtime.dispatch(ClientCommand::Logout)?;
                runtime
                    .dispatch(ClientCommand::ApplyDeviceUserLogin(AuthPayload {
                        access_token: "token-1".to_string(),
                        refresh_token: None,
                        user_id: "user-1".to_string(),
                        user_label: "user@example.com".to_string(),
                        device_id: Some("device-1".to_string()),
                        active_network_id: Some("net-1".to_string()),
                        virtual_ip: Some("10.0.0.99".to_string()),
                        expires_in: None,
                    }))
                    .expect("seed embedded signed-in runtime");
                runtime
                    .dispatch(ClientCommand::ApplyPlatformRuntimeState(
                        NetworkRuntimeState {
                            adapter_present: true,
                            network_enabled: true,
                            virtual_ip: Some("10.0.0.99".to_string()),
                            active_path: None,
                            peer_paths: Vec::new(),
                        },
                    ))
                    .expect("seed embedded enabled runtime");
                Ok(())
            })
            .expect("seed embedded runtime");
        let mut session = PersistedSession::empty();
        session.access_token = "token-1".to_string();
        session.user_id = "user-1".to_string();
        session.user_label = "user@example.com".to_string();
        session.device_id = Some("device-1".to_string());
        session.active_network_id = Some("net-1".to_string());
        session.virtual_ip = Some("10.0.0.99".to_string());
        persist_session(&session).expect("persist embedded reconcile session");

        reconcile_embedded_active_network_state();
        let event = watch_embedded_business_event(0, false, None, Duration::ZERO);
        assert!(event.revision > 0);
        assert_eq!(event.business_type, "control.sync.changed");
        assert_eq!(
            event
                .business_data
                .get("messageType")
                .and_then(Value::as_str),
            Some("active_network_reconcile_failed")
        );

        if let Some(value) = previous_state_dir {
            std::env::set_var("SLAN_STATE_DIR", value);
        } else {
            std::env::remove_var("SLAN_STATE_DIR");
        }
        let _ = std::fs::remove_dir_all(&state_dir);
    }
}
