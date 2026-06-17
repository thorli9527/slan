use std::{
    fs,
    path::PathBuf,
    sync::{Mutex, OnceLock},
    thread,
    time::Duration,
};

use anyhow::{Context, Result};
use client_core::{
    relay_path_kind_for_transport, AssignedIpPayload, AuthPayload, ClientCommand,
    ClientMessageNoticePayload, ClientRuntime, ClientViewState, PathCandidate, PathKind, PathState,
    PeerPathConfig, PlatformAclPolicy, PlatformDeviceNetworkConfig, PlatformNetworkConfig,
    RelayDataPlaneConfig, RelayPeerSession, RouteSpec,
};
use client_core_platform::PlatformNetworkImpl;
use control_mqtt_client::{ThinControlMqttClient, ThinMqttCredential, ThinMqttQoS};
use serde_json::Value;

use crate::{
    acl_policy::{acl_policies_for_network, platform_acl_policies},
    client_message_mqtt,
    control_plane::{
        local_stable_device_id, set_client_device_id_override, set_control_base_url_override,
        ControlPlaneClient,
    },
    control_transport::{self, ControlTransportMessage, MqttQos},
    local_api::{
        LocalServiceMethod, SendClientMessageRequest, ServiceRequest, WatchBusinessEventRequest,
        WatchBusinessEventResponse, WatchStateRequest, WatchStateResponse,
        BUSINESS_CONTROL_SYNC_CHANGED, BUSINESS_SESSION_CHANGED, BUSINESS_STATE_CHANGED,
    },
    session_store::{
        app_data_dir, current_timestamp_ms, ensure_session_device_registered,
        ensure_session_node_and_control_session, hydrate_session_from_control_plane, load_session,
        persist_session, prepare_client_login_session, remove_session, report_runtime_state,
        PersistedSession,
    },
};

static RUNTIME: OnceLock<Mutex<ClientRuntime<PlatformNetworkImpl>>> = OnceLock::new();
static MQTT: OnceLock<Mutex<Option<EmbeddedMqttConnection>>> = OnceLock::new();
static MQTT_LAST_ERROR: OnceLock<Mutex<Option<String>>> = OnceLock::new();
static BUSINESS_EVENT: OnceLock<Mutex<EmbeddedBusinessEvent>> = OnceLock::new();
static EMBEDDED_CONTROL_BASE_URL: OnceLock<Mutex<Option<String>>> = OnceLock::new();

const EMBEDDED_MQTT_KEEPALIVE_PING_INTERVAL_MS: u64 = 15_000;
const EMBEDDED_MQTT_RECONNECT_AFTER_SESSION_REFRESH_MS: u64 = 10 * 60 * 1000;

struct EmbeddedMqttConnection {
    device_id: Option<String>,
    downstream_topic: String,
    network_broadcast_topic: Option<String>,
    connected: bool,
    last_message_type: Option<String>,
    last_error: Option<String>,
}

struct EmbeddedBusinessEvent {
    revision: u64,
    business_type: String,
    business_data: Value,
}

fn mqtt_connection() -> &'static Mutex<Option<EmbeddedMqttConnection>> {
    MQTT.get_or_init(|| Mutex::new(None))
}

fn mqtt_last_error_store() -> &'static Mutex<Option<String>> {
    MQTT_LAST_ERROR.get_or_init(|| Mutex::new(None))
}

fn business_event() -> &'static Mutex<EmbeddedBusinessEvent> {
    BUSINESS_EVENT.get_or_init(|| {
        Mutex::new(EmbeddedBusinessEvent {
            revision: 0,
            business_type: BUSINESS_STATE_CHANGED.to_string(),
            business_data: serde_json::json!({}),
        })
    })
}

fn runtime() -> &'static Mutex<ClientRuntime<PlatformNetworkImpl>> {
    RUNTIME.get_or_init(|| {
        let mut runtime = ClientRuntime::new(PlatformNetworkImpl);
        if let Ok(session) = load_session() {
            let _ = runtime.dispatch(ClientCommand::ApplyDeviceUserLogin(session.into()));
        }
        Mutex::new(runtime)
    })
}

pub fn embedded_handle_request_json(request_json: &str) -> String {
    match handle_request_json(request_json) {
        Ok(response) => response,
        Err(error) => serde_json::json!({ "error": format!("{error:#}") }).to_string(),
    }
}

fn handle_request_json(request_json: &str) -> Result<String> {
    let request: ServiceRequest = serde_json::from_str(request_json).context("decode request")?;
    apply_embedded_request_overrides(&request.args);
    match LocalServiceMethod::parse(&request.method) {
        LocalServiceMethod::Start
        | LocalServiceMethod::LocalState
        | LocalServiceMethod::Refresh => {
            serde_json::to_string(&refresh_state()).context("encode state")
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
            serde_json::to_string(&watch_embedded_business_event(input.last_revision))
                .context("encode watch business event response")
        }
        LocalServiceMethod::LocalSession => local_session_json(),
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
            serde_json::to_string(&crate::network_module::network_module_snapshot())
                .context("encode local network module")
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
        LocalServiceMethod::Dispatch => {
            let command: ClientCommand =
                serde_json::from_value(request.args).context("decode client command")?;
            serde_json::to_string(&dispatch_embedded(command)?).context("encode state")
        }
        LocalServiceMethod::LocalLogout => {
            serde_json::to_string(&dispatch_embedded(ClientCommand::Logout)?)
                .context("encode state")
        }
        LocalServiceMethod::LocalNetworkShutdown => {
            serde_json::to_string(&dispatch_embedded(ClientCommand::DisableNetwork)?)
                .context("encode network shutdown state")
        }
        LocalServiceMethod::IngestPlatformRuntimeState => {
            let runtime_state = request
                .args
                .get("runtimeState")
                .cloned()
                .unwrap_or(Value::Null);
            let state = dispatch_embedded(ClientCommand::ApplyPlatformRuntimeState(
                serde_json::from_value(runtime_state).context("decode runtime state")?,
            ))?;
            report_runtime_state(&state);
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
    }
}

fn apply_embedded_request_overrides(args: &Value) {
    if let Some(state_dir) = args
        .get("stateDir")
        .and_then(|value| value.as_str())
        .map(str::trim)
        .filter(|value| !value.is_empty())
    {
        std::env::set_var("SLAN_STATE_DIR", state_dir);
    }
    if let Some(device_id) = args
        .get("deviceId")
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

fn platform_network_config() -> Result<Value> {
    let mut session = ensure_device_session().context("ensure device")?;
    let client = ControlPlaneClient::from_env();
    let network_configs =
        crate::network_module::refresh_network_module_from_session(&client, &session)
            .unwrap_or_default();
    let network_id = match session
        .active_network_id
        .clone()
        .filter(|value| !value.trim().is_empty())
    {
        Some(value) => value,
        None => client
            .active_network_id(&session.access_token)?
            .ok_or_else(|| anyhow::anyhow!("active network is not available"))?,
    };
    session.active_network_id = Some(network_id.clone());
    let device_id = session
        .device_id
        .clone()
        .filter(|value| !value.trim().is_empty())
        .ok_or_else(|| anyhow::anyhow!("device id is not available"))?;
    let activation = client.activate_network(&session.access_token, &device_id, &network_id)?;
    session.self_node_id = activation.self_node_id.clone();
    session.virtual_ip = Some(activation.virtual_ip.clone());
    ensure_session_node_and_control_session(&client, &mut session)
        .context("ensure node control session after network activation")?;
    persist_session(&session)?;
    {
        let mut runtime = runtime().lock().expect("embedded runtime mutex poisoned");
        let _ = runtime.dispatch(ClientCommand::ApplyDeviceUserLogin(session.clone().into()));
        let _ = runtime.dispatch(ClientCommand::SyncAssignedIp(AssignedIpPayload {
            virtual_ip: activation.virtual_ip.clone(),
            prefix_len: Some(activation.prefix_len),
        }));
    }
    let best_relay = activation
        .relay_candidates
        .iter()
        .find(|candidate| candidate.transport == "udp")
        .or_else(|| activation.relay_candidates.first());
    let all_acl_policies = platform_acl_policies(&network_configs);
    let eligible_relay_peer_count =
        embedded_eligible_relay_peer_count(activation.self_node_id.as_deref(), &activation.peers);
    let (relay_data_plane, relay_build_error) = match build_embedded_relay_data_plane_config(
        &client,
        &session,
        network_id.as_str(),
        activation.self_node_id.as_deref(),
        &activation.peers,
        best_relay,
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
        .or_else(|| best_relay.map(|relay| relay.address.clone()));
    let config = PlatformNetworkConfig {
        session_name: "SLAN".to_string(),
        virtual_ip: activation.virtual_ip.clone(),
        prefix_len: activation.prefix_len,
        network_configs: platform_network_configs(&network_configs),
        dns_servers: activation.dns_servers,
        routes: embedded_routes_with_peer_virtual_ips(
            activation.routes,
            &activation.peers,
            activation.virtual_ip.as_str(),
        ),
        mtu: Some(1280),
        relay_endpoint_id: best_relay.map(|relay| relay.endpoint_id.clone()),
        relay_transport: best_relay.map(|relay| relay.transport.clone()),
        relay_address: platform_relay_address,
        acl_policies: all_acl_policies.clone(),
        relay_data_plane,
    };
    let mut value = serde_json::to_value(config).context("encode platform config value")?;
    if let Value::Object(map) = &mut value {
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
    Ok(value)
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
            intra_group_policy: config.intra_group_policy.clone(),
            network_created_at: config.network_created_at,
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
    let access_token = session.access_token.trim();
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
            Ok(ticket) => sessions.push(RelayPeerSession {
                session_id: ticket.session_id.clone(),
                peer_node_id: peer.node_id.clone(),
                peer_virtual_ips: peer.virtual_ips.clone(),
                ticket,
            }),
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
    Ok(RelayDataPlaneConfig {
        enabled: !sessions.is_empty(),
        transport: relay.transport.clone(),
        relay_address,
        local_node_id: local_node_id.to_string(),
        network_id: network_id.to_string(),
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
    peers
        .iter()
        .filter(|peer| peer.node_id != local_node_id)
        .map(|peer| {
            let relay_session = relay_sessions
                .iter()
                .find(|session| session.peer_node_id == peer.node_id);
            let mut candidates = Vec::new();
            let mut direct_addresses = Vec::new();
            for endpoint in &peer.endpoints {
                let address = endpoint.address.trim();
                if address.is_empty() || direct_addresses.iter().any(|value| value == address) {
                    continue;
                }
                direct_addresses.push(address.to_string());
                candidates.push(PathCandidate {
                    kind: PathKind::DirectUdp,
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

fn ensure_device_session() -> Result<PersistedSession> {
    let session = load_session().context("load session")?;
    let session = align_embedded_session_device_id(session)?;
    let session = ensure_session_device_registered(session).context("register device")?;
    {
        let mut runtime = runtime().lock().expect("embedded runtime mutex poisoned");
        let _ = runtime.dispatch(ClientCommand::ApplyDeviceUserLogin(session.clone().into()));
    }
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
    let network_broadcast_topic = embedded_network_broadcast_topic(session);
    if let Some(topic) = network_broadcast_topic.as_deref() {
        if let Err(error) = client.subscribe(topic) {
            eprintln!("SLAN_EMBEDDED_MQTT_NETWORK_SUBSCRIBE_FAILED topic={topic} error={error}");
        } else {
            eprintln!("SLAN_EMBEDDED_MQTT_NETWORK_SUBSCRIBED topic={topic}");
        }
    }
    {
        let mut guard = mqtt_connection()
            .lock()
            .expect("embedded mqtt mutex poisoned");
        *guard = Some(EmbeddedMqttConnection {
            device_id: device_id.clone(),
            downstream_topic: downstream_topic.clone(),
            network_broadcast_topic: network_broadcast_topic.clone(),
            connected: true,
            last_message_type: None,
            last_error: None,
        });
    }
    set_embedded_mqtt_last_error(None);
    spawn_embedded_mqtt_consumer(client, device_id.clone(), downstream_topic.clone());
    Ok(serde_json::json!({
        "connected": true,
        "deviceId": device_id,
        "downstreamTopic": downstream_topic,
        "networkBroadcastTopic": network_broadcast_topic,
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

fn embedded_network_broadcast_topic(session: &PersistedSession) -> Option<String> {
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
    device_id: Option<String>,
    downstream_topic: String,
) {
    thread::spawn(move || {
        let mut last_heartbeat_ms = None;
        let mut last_runtime_state_ms = None;
        let mut last_path_health_ms = None;
        let mut last_session_key = String::new();
        let connected_at_ms = current_timestamp_ms();
        let mut last_keepalive_ping_ms = Some(connected_at_ms);
        loop {
            match client.read_publish(Duration::from_millis(500)) {
                Ok(Some(publish)) => {
                    let message_type = embedded_downstream_message_type(&publish.payload);
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
                    update_embedded_mqtt_status(
                        &device_id,
                        &downstream_topic,
                        ack_result.is_ok(),
                        message_type,
                        last_error.clone(),
                    );
                    if ack_result.is_err() {
                        break;
                    }
                    if business_ack_failed {
                        break;
                    }
                }
                Ok(None) => {}
                Err(error) => {
                    update_embedded_mqtt_status(
                        &device_id,
                        &downstream_topic,
                        false,
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
            if now_ms.saturating_sub(connected_at_ms)
                >= EMBEDDED_MQTT_RECONNECT_AFTER_SESSION_REFRESH_MS
            {
                update_embedded_mqtt_status(
                    &device_id,
                    &downstream_topic,
                    false,
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
                    update_embedded_mqtt_status(
                        &device_id,
                        &downstream_topic,
                        false,
                        None,
                        Some(error),
                    );
                    return;
                }
                last_keepalive_ping_ms = Some(now_ms);
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
            let state = {
                let runtime = runtime().lock().expect("embedded runtime mutex poisoned");
                runtime.state().clone()
            };
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
                    update_embedded_mqtt_status(
                        &device_id,
                        &downstream_topic,
                        false,
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
    device_id: &Option<String>,
    downstream_topic: &str,
    connected: bool,
    last_message_type: Option<String>,
    last_error: Option<String>,
) {
    let mut guard = mqtt_connection()
        .lock()
        .expect("embedded mqtt mutex poisoned");
    if let Some(connection) = guard.as_mut() {
        if connection.device_id == *device_id && connection.downstream_topic == downstream_topic {
            connection.connected = connected;
            if last_message_type.is_some() {
                connection.last_message_type = last_message_type;
            }
            connection.last_error = last_error;
        }
    }
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
        .or_else(|| value.get("deliveryId"))
        .and_then(Value::as_str)
        .map(str::trim)
        .filter(|value| !value.is_empty())?
        .to_string();
    let action = match message_type {
        "device_user_login_succeeded" => "deviceUserLoginSucceeded",
        "network_config_changed" => "refreshNetworkConfig",
        "device_ip_reassigned" => "refreshNetworkConfig",
        "device_network_enabled" => "refreshNetworkConfig",
        "device_network_disabled" => "refreshNetworkConfig",
        "client_message" => "clientMessage",
        _ => return None,
    };
    Some((delivery_id, action))
}

fn ingest_embedded_downstream_publish(payload: &[u8]) -> Result<()> {
    let value: Value = serde_json::from_slice(payload).context("decode downstream json")?;
    match value
        .get("type")
        .and_then(Value::as_str)
        .unwrap_or_default()
    {
        "network_config_changed" => ingest_embedded_network_config_changed(&value),
        "device_user_login_succeeded" => ingest_embedded_device_user_login_succeeded(&value),
        "device_ip_reassigned" => ingest_embedded_device_ip_reassigned(&value),
        "device_network_enabled" | "device_network_disabled" => {
            ingest_embedded_device_network_presence(&value)
        }
        "client_message" => persist_embedded_client_message(&value),
        _ => Ok(()),
    }
}

fn ingest_embedded_device_user_login_succeeded(value: &Value) -> Result<()> {
    let auth_value = value
        .get("payload")
        .cloned()
        .ok_or_else(|| anyhow::anyhow!("device_user_login_succeeded payload is missing"))?;
    let auth: AuthPayload =
        serde_json::from_value(auth_value).context("decode device_user_login_succeeded payload")?;
    {
        let runtime = runtime().lock().expect("embedded runtime mutex poisoned");
        validate_embedded_device_user_login_succeeded(&runtime, &auth)?;
    }
    let session =
        hydrate_session_from_control_plane(auth).context("hydrate embedded device user login")?;
    persist_session(&session)?;
    connect_embedded_control_mqtt_with_session(&session)
        .context("connect mqtt after embedded device user login")?;
    let state = {
        let mut runtime = runtime().lock().expect("embedded runtime mutex poisoned");
        runtime
            .dispatch(ClientCommand::ApplyDeviceUserLogin(session.into()))
            .context("apply embedded device user login")?
    };
    publish_embedded_business_event(
        BUSINESS_SESSION_CHANGED,
        serde_json::json!({ "messageType": "device_user_login_succeeded" }),
        &state,
    );
    Ok(())
}

fn validate_embedded_device_user_login_succeeded(
    runtime: &ClientRuntime<PlatformNetworkImpl>,
    auth: &AuthPayload,
) -> Result<()> {
    let expected_device_id = local_stable_device_id()
        .ok()
        .filter(|value| !value.trim().is_empty())
        .or_else(|| {
            runtime
                .state()
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

fn ingest_embedded_network_config_changed(value: &Value) -> Result<()> {
    let session = load_session().context("load session for network config refresh")?;
    let client = ControlPlaneClient::from_env();
    let configs = crate::network_module::refresh_network_module_from_session(&client, &session)
        .context("refresh device network configs")?;
    let payload = value.get("payload").unwrap_or(value);
    let network_id = payload
        .get("networkId")
        .and_then(Value::as_str)
        .unwrap_or_default();
    let config_version = payload.get("configVersion").and_then(Value::as_i64);
    let state = {
        let runtime = runtime().lock().expect("embedded runtime mutex poisoned");
        runtime.state().clone()
    };
    publish_embedded_business_event(
        BUSINESS_CONTROL_SYNC_CHANGED,
        serde_json::json!({
            "messageType": "network_config_changed",
            "networkId": network_id,
            "configVersion": config_version,
            "networkConfigCount": configs.len(),
            "reconfigureRequired": true,
        }),
        &state,
    );
    Ok(())
}

fn ingest_embedded_device_network_presence(value: &Value) -> Result<()> {
    let payload = value.get("payload").unwrap_or(value);
    let message_type = value
        .get("type")
        .and_then(Value::as_str)
        .unwrap_or("device_network_presence");
    let device_id = payload
        .get("deviceId")
        .and_then(Value::as_str)
        .unwrap_or_default();
    let network_id = payload
        .get("networkId")
        .and_then(Value::as_str)
        .unwrap_or_default();
    let online = payload
        .get("online")
        .and_then(Value::as_bool)
        .unwrap_or(message_type == "device_network_enabled");
    let online_count = payload
        .get("onlineDevices")
        .and_then(Value::as_array)
        .map(Vec::len)
        .unwrap_or_default();
    eprintln!(
        "SLAN_EMBEDDED_NETWORK_PRESENCE type={message_type} networkId={network_id} deviceId={device_id} online={online} onlineDevices={online_count}"
    );
    let state = {
        let runtime = runtime().lock().expect("embedded runtime mutex poisoned");
        runtime.state().clone()
    };
    publish_embedded_business_event(
        BUSINESS_CONTROL_SYNC_CHANGED,
        serde_json::json!({
            "messageType": message_type,
            "networkId": network_id,
            "deviceId": device_id,
            "online": online,
            "reconfigureRequired": true,
            "onlineDevices": payload.get("onlineDevices").cloned().unwrap_or_else(|| serde_json::json!([])),
        }),
        &state,
    );
    Ok(())
}

fn ingest_embedded_device_ip_reassigned(value: &Value) -> Result<()> {
    let payload = value
        .get("payload")
        .ok_or_else(|| anyhow::anyhow!("device_ip_reassigned payload is missing"))?;
    let target_device_id = payload
        .get("deviceId")
        .or_else(|| payload.get("device_id"))
        .and_then(Value::as_str)
        .unwrap_or_default()
        .trim();
    let virtual_ip = payload
        .get("virtualIp")
        .or_else(|| payload.get("virtual_ip"))
        .and_then(Value::as_str)
        .unwrap_or_default()
        .trim();
    if target_device_id.is_empty() || virtual_ip.is_empty() {
        return Ok(());
    }
    let prefix_len = payload
        .get("prefixLen")
        .or_else(|| payload.get("prefixLength"))
        .or_else(|| payload.get("prefix_len"))
        .and_then(Value::as_u64)
        .and_then(|value| u8::try_from(value).ok());
    let mut session = load_session().context("load session for device ip reassignment")?;
    if session.device_id.as_deref().map(str::trim) != Some(target_device_id) {
        return Ok(());
    }
    session.virtual_ip = Some(virtual_ip.to_string());
    persist_session(&session).context("persist reassigned ip")?;
    let state = {
        let mut runtime = runtime().lock().expect("embedded runtime mutex poisoned");
        runtime.dispatch(ClientCommand::SyncAssignedIp(AssignedIpPayload {
            virtual_ip: virtual_ip.to_string(),
            prefix_len,
        }))?
    };
    publish_embedded_business_event(
        BUSINESS_CONTROL_SYNC_CHANGED,
        serde_json::json!({
            "messageType": "device_ip_reassigned",
            "deviceId": target_device_id,
            "virtualIp": virtual_ip,
        }),
        &state,
    );
    Ok(())
}

fn persist_embedded_client_message(value: &Value) -> Result<()> {
    write_embedded_downstream_payload("client-v2-last-client-message.json", value)?;
    let payload = value.get("payload").unwrap_or(value);
    let message_id = payload
        .get("messageId")
        .or_else(|| payload.get("message_id"))
        .and_then(Value::as_str)
        .map(str::to_string);
    let from_device_id = payload
        .get("fromDeviceId")
        .or_else(|| payload.get("from_device_id"))
        .and_then(Value::as_str)
        .map(str::to_string);
    let body = payload
        .get("body")
        .and_then(Value::as_str)
        .map(str::to_string);
    if maybe_reply_embedded_client_ping(from_device_id.as_deref(), body.as_deref())? {
        return Ok(());
    }
    let state = {
        let mut runtime = runtime().lock().expect("embedded runtime mutex poisoned");
        runtime.dispatch(ClientCommand::ApplyClientMessage(
            ClientMessageNoticePayload {
                message_id: message_id.clone(),
                from_device_id: from_device_id.clone(),
                body: body.clone(),
            },
        ))?
    };
    publish_embedded_business_event(
        BUSINESS_CONTROL_SYNC_CHANGED,
        serde_json::json!({
            "messageType": "client_message",
            "messageId": message_id,
            "fromDeviceId": from_device_id,
            "body": body,
        }),
        &state,
    );
    Ok(())
}

fn maybe_reply_embedded_client_ping(
    from_device_id: Option<&str>,
    body: Option<&str>,
) -> Result<bool> {
    let Some(from_device_id) = from_device_id
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

fn embedded_state_file(file_name: &str) -> PathBuf {
    app_data_dir().join("SLAN").join(file_name)
}

fn publish_embedded_business_event(
    business_type: &str,
    business_data: Value,
    state: &ClientViewState,
) {
    let mut event = business_event()
        .lock()
        .expect("embedded business event mutex poisoned");
    event.revision = event.revision.saturating_add(1);
    event.business_type = business_type.to_string();
    event.business_data = match serde_json::to_value(state) {
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
}

fn watch_embedded_business_event(last_revision: u64) -> WatchBusinessEventResponse {
    let event = business_event()
        .lock()
        .expect("embedded business event mutex poisoned");
    if event.revision > last_revision {
        return WatchBusinessEventResponse {
            revision: event.revision,
            business_type: event.business_type.clone(),
            business_data: event.business_data.clone(),
            snapshot: refresh_state(),
        };
    }
    WatchBusinessEventResponse {
        revision: last_revision,
        business_type: BUSINESS_STATE_CHANGED.to_string(),
        business_data: serde_json::json!({}),
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
    let mqtt = session
        .mqtt
        .as_ref()
        .ok_or_else(|| anyhow::anyhow!("mqtt credential is not available"))?;
    crate::client_message_mqtt::publish_client_message(
        mqtt,
        network_id,
        from_device_id,
        &input.target_device_id,
        &input.body,
        input.metadata.as_ref(),
    )
}

fn embedded_control_status() -> Value {
    match load_session() {
        Ok(session) => {
            let (mqtt_connected, mqtt_last_error, mqtt_last_message_type) = {
                let guard = mqtt_connection()
                    .lock()
                    .expect("embedded mqtt mutex poisoned");
                guard
                    .as_ref()
                    .map(|connection| {
                        (
                            connection.connected,
                            connection.last_error.clone(),
                            connection.last_message_type.clone(),
                        )
                    })
                    .unwrap_or((false, None, None))
            };
            let mqtt_last_error = mqtt_last_error.or_else(|| {
                mqtt_last_error_store()
                    .lock()
                    .expect("embedded mqtt last error mutex poisoned")
                    .clone()
            });
            let mqtt_credential_ready = session.mqtt.is_some();
            let device_ready = session
                .device_id
                .as_deref()
                .is_some_and(|value| !value.trim().is_empty());
            let network_ready = session
                .active_network_id
                .as_deref()
                .is_some_and(|value| !value.trim().is_empty());
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
                "mqttLastMessageType": mqtt_last_message_type,
                "activeNetworkId": session.active_network_id,
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

fn dispatch_embedded(command: ClientCommand) -> Result<ClientViewState> {
    if matches!(command, ClientCommand::EnableNetwork) {
        let _ = platform_network_config().context("activate embedded network")?;
        let mut runtime = runtime().lock().expect("embedded runtime mutex poisoned");
        let state = runtime
            .dispatch(ClientCommand::EnableNetwork)
            .context("enable embedded network")?;
        report_runtime_state(&state);
        return Ok(state);
    }
    let mut runtime = runtime().lock().expect("embedded runtime mutex poisoned");
    match command {
        ClientCommand::OpenClientLogin => {
            let session = prepare_client_login_session(std::env::consts::OS)
                .context("prepare embedded client login")?;
            connect_embedded_control_mqtt_with_session(&session)
                .context("connect mqtt after embedded client login prepare")?;
            Ok(runtime.request_browser_login(session.device_id))
        }
        ClientCommand::LoginWithPassword(payload) => {
            let auth = ControlPlaneClient::from_env()
                .login_with_password(&payload.email, &payload.password)
                .context("password login")?;
            let session =
                hydrate_session_from_control_plane(auth).context("hydrate password session")?;
            persist_session(&session)?;
            connect_embedded_control_mqtt_with_session(&session)
                .context("connect mqtt after login")?;
            runtime
                .dispatch(ClientCommand::ApplyDeviceUserLogin(session.into()))
                .context("apply login session")
        }
        ClientCommand::ApplyDeviceUserLogin(payload) => {
            validate_embedded_device_user_login_succeeded(&runtime, &payload)?;
            let session =
                hydrate_session_from_control_plane(payload).context("hydrate device user login")?;
            persist_session(&session)?;
            connect_embedded_control_mqtt_with_session(&session)
                .context("connect mqtt after device user login")?;
            runtime
                .dispatch(ClientCommand::ApplyDeviceUserLogin(session.into()))
                .context("apply device user login")
        }
        ClientCommand::Logout => {
            crate::network_module::clear_network_module();
            remove_session()?;
            runtime.dispatch(ClientCommand::Logout).context("logout")
        }
        other => runtime.dispatch(other).context("dispatch command"),
    }
}

fn refresh_state() -> ClientViewState {
    let mut runtime = runtime().lock().expect("embedded runtime mutex poisoned");
    match runtime.refresh() {
        Ok(()) => runtime.state().clone(),
        Err(error) => runtime.set_error(error.to_string()),
    }
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
    use serde_json::Value;

    use super::{embedded_handle_request_json, rewrite_local_mqtt_broker_host, url_host};

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
        super::ingest_embedded_downstream_publish(
            serde_json::json!({
                "type": "client_message",
                "payload": {
                    "messageId": "msg-1",
                    "fromDeviceId": "ios-peer",
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
}
