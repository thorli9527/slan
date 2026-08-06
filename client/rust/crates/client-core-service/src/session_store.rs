use std::{
    fs,
    path::PathBuf,
    sync::{
        atomic::{AtomicU64, Ordering},
        Mutex, OnceLock, RwLock, RwLockReadGuard,
    },
    time::{SystemTime, UNIX_EPOCH},
};

use anyhow::{Context, Result};
use client_core::{node_config_path_rank, ClientViewState, DeviceSessionPayload, NodeConfig};
use serde::{Deserialize, Serialize};

use crate::{
    control_plane::{
        local_stable_device_id, ControlDevice, ControlPlaneClient, DeviceNetworkConfig,
        DeviceSessionResponse, MqttCredential, RelayCandidate,
    },
    network_module::{replace_network_module_configs, sync_resolver_runtime_state},
    network_runtime_state::runtime_network_state_store,
    relay_candidates::replace_runtime_relay_candidates,
    relay_models::PersistedRelayCandidate,
    resolver_runtime_state::clear_resolver_runtime_state,
};

const SESSION_RENEW_WINDOW_MS: u64 = 5 * 60 * 1_000;
static APP_DATA_DIR_OVERRIDE: OnceLock<Mutex<Option<PathBuf>>> = OnceLock::new();
static SESSION_RUNTIME_EPOCH: AtomicU64 = AtomicU64::new(1);
static SESSION_RUNTIME_LIFECYCLE: RwLock<()> = RwLock::new(());

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct PersistedSession {
    pub(crate) access_token: String,
    #[serde(default)]
    pub(crate) session_kind: String,
    #[serde(default)]
    pub(crate) device_token_expires_at: Option<i64>,
    #[serde(default)]
    pub(crate) device_session_id: Option<String>,
    #[serde(default)]
    pub(crate) device_token: Option<String>,
    #[serde(default)]
    pub(crate) device_refresh_token: Option<String>,
    pub(crate) device_id: Option<String>,
    #[serde(default)]
    pub(crate) self_node_id: Option<String>,
    pub(crate) active_network_id: Option<String>,
    #[serde(default)]
    pub(crate) network_ids: Vec<String>,
    pub(crate) virtual_ip: Option<String>,
    #[serde(default)]
    pub(crate) relay_candidates: Vec<PersistedRelayCandidate>,
    #[serde(default)]
    pub(crate) node_configs: Vec<NodeConfig>,
    pub(crate) mqtt: Option<MqttCredential>,
    pub(crate) expires_in: Option<u64>,
    pub(crate) authenticated_at_ms: u64,
}

#[derive(Debug, Clone)]
pub(crate) struct PreparedSession {
    pub(crate) session: PersistedSession,
    pub(crate) relay_candidates: Vec<PersistedRelayCandidate>,
    pub(crate) network_configs: Vec<DeviceNetworkConfig>,
}

impl PreparedSession {
    pub(crate) fn from_session(session: PersistedSession) -> Self {
        Self {
            relay_candidates: session.relay_candidates.clone(),
            session,
            network_configs: Vec::new(),
        }
    }
}

#[derive(Debug, Clone)]
pub(crate) struct PendingDeviceActivation {
    pub(crate) base_url: Option<String>,
    pub(crate) authorization_key: String,
    pub(crate) enable_network: bool,
}

impl From<PersistedSession> for DeviceSessionPayload {
    fn from(session: PersistedSession) -> Self {
        Self {
            device_id: session.device_id,
            virtual_ip: session.virtual_ip,
        }
    }
}

fn non_empty_session_network_id(value: &str) -> Option<String> {
    let value = value.trim();
    (!value.is_empty()).then(|| value.to_string())
}

impl PersistedSession {
    #[cfg(test)]
    pub(crate) fn empty() -> Self {
        Self {
            access_token: String::new(),
            session_kind: String::new(),
            device_token_expires_at: None,
            device_session_id: None,
            device_token: None,
            device_refresh_token: None,
            device_id: None,
            self_node_id: None,
            active_network_id: None,
            network_ids: Vec::new(),
            virtual_ip: None,
            relay_candidates: Vec::new(),
            node_configs: Vec::new(),
            mqtt: None,
            expires_in: None,
            authenticated_at_ms: current_timestamp_ms(),
        }
    }

    #[cfg(test)]
    pub(crate) fn preactivation(
        device_id: impl Into<String>,
        mqtt: Option<MqttCredential>,
    ) -> Self {
        let mut session = Self::empty();
        session.session_kind = "preactivation".to_string();
        session.device_id = Some(device_id.into());
        session.mqtt = mqtt;
        session
    }
}

pub(crate) fn session_device_api_token(session: &PersistedSession) -> &str {
    session
        .device_token
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .unwrap_or(session.access_token.as_str())
}

fn normalize_mqtt_credential(mut mqtt: MqttCredential) -> MqttCredential {
    normalize_mqtt_topic_prefix(&mut mqtt);
    mqtt
}

fn normalize_mqtt_topic_prefix(mqtt: &mut MqttCredential) {
    mqtt.topic_prefix = mqtt.topic_prefix.trim().trim_matches('/').to_string();
}

fn normalize_session_mqtt_topic_prefix(session: &mut PersistedSession) {
    if let Some(mqtt) = session.mqtt.as_mut() {
        normalize_mqtt_topic_prefix(mqtt);
    }
}

pub(crate) fn load_valid_registered_session() -> Option<PersistedSession> {
    let session = match load_session() {
        Ok(session) => session,
        Err(error) if session_not_found_error(&error) => {
            return match recover_session_with_stored_authorization_key() {
                Ok(Some(recovered)) => {
                    eprintln!(
                        "client-core-service missing session recovered with stored authorization"
                    );
                    Some(recovered)
                }
                Ok(None) => None,
                Err(recovery_error) => {
                    eprintln!(
                        "client-core-service missing session authorization recovery failed: {recovery_error:#}"
                    );
                    None
                }
            };
        }
        Err(error) => {
            eprintln!("client-core-service startup session load failed: {error:#}");
            return None;
        }
    };
    if session.access_token.trim().is_empty() {
        if preactivation_session_is_usable(&session) {
            return Some(session);
        }
        let _ = remove_session();
        return None;
    }
    match prepare_session_device_registered(session.clone()) {
        Ok(prepared) => {
            if let Err(error) = persist_session(&prepared.session) {
                eprintln!("client-core-service startup session persist skipped: {error:#}");
                return Some(session);
            }
            apply_prepared_session_runtime(&prepared);
            Some(prepared.session)
        }
        Err(error) if session_auth_invalid_error(&error) => {
            eprintln!(
                "client-core-service session invalid; attempting stored authorization recovery: {error:#}"
            );
            match recover_session_with_stored_authorization_key() {
                Ok(Some(recovered)) => {
                    eprintln!("client-core-service session recovered with stored authorization");
                    Some(recovered)
                }
                Ok(None) => {
                    eprintln!(
                        "client-core-service session invalid and no stored authorization is available"
                    );
                    let _ = remove_session();
                    None
                }
                Err(recovery_error) => {
                    eprintln!(
                        "client-core-service stored authorization recovery failed; preserving session for retry: {recovery_error:#}"
                    );
                    Some(session)
                }
            }
        }
        Err(error) => {
            eprintln!("client-core-service startup device registration skipped: {error:#}");
            Some(session)
        }
    }
}

pub(crate) fn session_is_expired(session: &PersistedSession) -> bool {
    if session.access_token.trim().is_empty() {
        return !preactivation_session_is_usable(session);
    }
    if session.session_kind == "device" {
        if let Some(expires_at) = session.device_token_expires_at {
            let now = current_timestamp_ms() / 1_000;
            return now >= expires_at as u64;
        }
    }
    let Some(expires_in) = session.expires_in else {
        return false;
    };
    let lifetime_ms = expires_in.saturating_mul(1_000);
    let expires_at_ms = session.authenticated_at_ms.saturating_add(lifetime_ms);
    current_timestamp_ms() >= expires_at_ms
}

fn preactivation_session_is_usable(session: &PersistedSession) -> bool {
    if session.session_kind != "preactivation" {
        return false;
    }
    if session
        .device_id
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .is_none()
    {
        return false;
    }
    let Some(mqtt) = session.mqtt.as_ref() else {
        return false;
    };
    let mqtt_fields_ready = [
        mqtt.broker_url.as_str(),
        mqtt.client_id.as_str(),
        mqtt.username.as_str(),
        mqtt.password.as_str(),
        mqtt.topic_prefix.as_str(),
    ]
    .into_iter()
    .all(|value| !value.trim().is_empty());
    if !mqtt_fields_ready {
        return false;
    }
    if let Some(expires_at) = mqtt.expires_at {
        let now = (current_timestamp_ms() / 1_000) as i64;
        return now < expires_at;
    }
    true
}

fn device_session_should_renew(session: &PersistedSession) -> bool {
    if mqtt_credential_should_renew(session.mqtt.as_ref()) {
        return true;
    }
    let Some(expires_at) = session.device_token_expires_at else {
        return session
            .device_token
            .as_deref()
            .map(str::trim)
            .filter(|value| !value.is_empty())
            .is_some();
    };
    let now = current_timestamp_ms() / 1_000;
    now.saturating_add(SESSION_RENEW_WINDOW_MS / 1_000) >= expires_at as u64
}

fn mqtt_credential_should_renew(mqtt: Option<&MqttCredential>) -> bool {
    let Some(mqtt) = mqtt else {
        return false;
    };
    let Some(expires_at) = mqtt.expires_at else {
        return false;
    };
    let now = (current_timestamp_ms() / 1_000) as i64;
    now.saturating_add((SESSION_RENEW_WINDOW_MS / 1_000) as i64) >= expires_at
}

fn persisted_session_from_device_session(
    response: crate::control_plane::DeviceSessionResponse,
) -> PersistedSession {
    let config = response
        .network_configs
        .as_ref()
        .and_then(|items| items.items.first());
    let active_network_id = response
        .device_session
        .active_network_ids
        .iter()
        .find_map(|value| non_empty_session_network_id(value))
        .or_else(|| config.and_then(|item| non_empty_session_network_id(&item.network_id)))
        .or_else(|| {
            response
                .device
                .active_network_id
                .as_deref()
                .and_then(non_empty_session_network_id)
        });
    let network_ids = response
        .device_session
        .active_network_ids
        .iter()
        .filter_map(|value| non_empty_session_network_id(value))
        .collect();
    let virtual_ip = config
        .and_then(|item| item.global_ip.clone())
        .or_else(|| response.device.global_ip.clone())
        .or(response.device.current_virtual_ip.clone())
        .or(response.device.virtual_ip.clone());
    let device_token = response.device_session.device_token.clone();
    let relay_candidates =
        relay_candidates_from_device_session_response(&response, active_network_id.as_deref());
    let node_configs = node_configs_from_device_session_response(&response);
    let mqtt = mqtt_from_device_session_response(&response);
    PersistedSession {
        access_token: device_token.clone(),
        session_kind: "device".to_string(),
        device_token_expires_at: Some(response.device_session.device_token_expires_at),
        device_session_id: Some(response.device_session.session_id),
        device_token: Some(device_token),
        device_refresh_token: response.device_session.device_refresh_token.clone(),
        device_id: Some(response.device_session.device_id.clone()),
        self_node_id: None,
        active_network_id,
        network_ids,
        virtual_ip,
        relay_candidates,
        node_configs,
        mqtt,
        expires_in: response
            .device_session
            .device_token_expires_at
            .checked_sub((current_timestamp_ms() / 1_000) as i64)
            .map(|value| value.max(0) as u64),
        authenticated_at_ms: current_timestamp_ms(),
    }
}

pub(crate) fn activate_device_with_key(key: &str) -> Result<PersistedSession> {
    let key = key.trim();
    anyhow::ensure!(!key.is_empty(), "authorization key is empty");
    let device_id = local_stable_device_id().context("initialize local device identity")?;
    let first_use = device_is_first_use(&device_id)?;
    let client = ControlPlaneClient::from_env();
    let response = client.exchange_device_credential(key)?;
    let mut session = persisted_session_from_device_session(response);
    let _ = refresh_prepared_session_network(&client, &mut session, Vec::new());
    ensure_session_node_binding(&client, &mut session)?;
    persist_session(&session)?;
    crate::client_config::store_secret(
        &device_id,
        crate::client_config::KEY_DEVICE_AUTHORIZATION_KEY,
        &key,
    )
    .context("persist device authorization key")?;
    if first_use {
        crate::client_config::store_secret(
            &device_id,
            crate::client_config::KEY_DEVICE_PROVISIONED,
            &true,
        )
        .context("mark device provisioning complete")?;
    }
    Ok(session)
}

pub(crate) fn recover_session_with_stored_authorization_key() -> Result<Option<PersistedSession>> {
    let device_id = local_stable_device_id().context("load device identity for recovery")?;
    let key = crate::client_config::load_secret::<String>(
        &device_id,
        crate::client_config::KEY_DEVICE_AUTHORIZATION_KEY,
    )?
    .map(|value| value.trim().to_string())
    .filter(|value| !value.is_empty());
    key.map(|value| activate_device_with_key(&value))
        .transpose()
}

#[allow(dead_code)] // Used by the embedded mobile library, not the desktop service binary.
pub(crate) fn local_device_is_first_use() -> Result<bool> {
    let device_id = local_stable_device_id().context("initialize local device identity")?;
    device_is_first_use(&device_id)
}

fn device_is_first_use(device_id: &str) -> Result<bool> {
    Ok(!crate::client_config::load_secret::<bool>(
        device_id,
        crate::client_config::KEY_DEVICE_PROVISIONED,
    )?
    .unwrap_or(false))
}

pub(crate) fn session_auth_invalid_error(error: &anyhow::Error) -> bool {
    let message = error.to_string().to_ascii_lowercase();
    message.contains("http 401")
        || message.contains("unauthorized")
        || message.contains("invalid token")
        || message.contains("token expired")
}

pub(crate) fn prepare_session_device_registered(
    mut session: PersistedSession,
) -> Result<PreparedSession> {
    if session.access_token.trim().is_empty() {
        return Ok(PreparedSession::from_session(session));
    }
    let client = ControlPlaneClient::from_env();
    let mut projection = PreparedSession::from_session(session.clone());
    let (relay_candidates, network_configs) =
        prepare_bound_device_session(&client, &mut session, false)?;
    projection.relay_candidates = relay_candidates;
    projection.network_configs = network_configs;
    projection.network_configs =
        refresh_prepared_session_network(&client, &mut session, projection.network_configs);
    ensure_session_node_binding(&client, &mut session)?;
    refresh_session_runtime_endpoints(&client, &mut session)?;
    projection.session = session;
    Ok(projection)
}

pub(crate) fn force_renew_mqtt_credential() -> Result<()> {
    let mut session = load_session().context("load session for mqtt credential renewal")?;
    if session.access_token.trim().is_empty() {
        anyhow::bail!("cannot renew mqtt credential without a registered session");
    }
    let client = ControlPlaneClient::from_env();
    let (relay_candidates, network_configs) =
        prepare_bound_device_session(&client, &mut session, true)
            .context("force renew device session after mqtt authentication rejection")?;
    persist_session(&session).context("persist renewed mqtt credential")?;
    apply_prepared_session_runtime(&PreparedSession {
        session,
        relay_candidates,
        network_configs,
    });
    Ok(())
}

fn apply_prepared_session_runtime(prepared: &PreparedSession) {
    if !prepared.relay_candidates.is_empty() {
        replace_runtime_relay_candidates(prepared.relay_candidates.clone());
    }
    if !prepared.network_configs.is_empty() {
        replace_network_module_configs(prepared.network_configs.clone());
        sync_resolver_runtime_state(&prepared.session, &prepared.network_configs);
    }
}

fn prepare_bound_device_session(
    client: &ControlPlaneClient,
    session: &mut PersistedSession,
    force_renew: bool,
) -> Result<(Vec<PersistedRelayCandidate>, Vec<DeviceNetworkConfig>)> {
    let has_device_token = session
        .device_token
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .is_some();
    if !has_device_token {
        anyhow::bail!("device token is missing; activate with an authorization key");
    }
    if force_renew || device_session_needs_refresh(session) {
        return prepare_bound_device_session_response(client, session);
    }
    Ok((session.relay_candidates.clone(), Vec::new()))
}

fn device_session_needs_refresh(session: &PersistedSession) -> bool {
    let assigned_ip_missing = session
        .virtual_ip
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .is_none();
    assigned_ip_missing || device_session_should_renew(session)
}

fn prepare_bound_device_session_response(
    client: &ControlPlaneClient,
    session: &mut PersistedSession,
) -> Result<(Vec<PersistedRelayCandidate>, Vec<DeviceNetworkConfig>)> {
    let device_token = session
        .device_token
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .context("missing device token for device session renew")?
        .to_string();
    let network_enabled = session
        .virtual_ip
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .is_some();
    let response = client.renew_device_session(
        &device_token,
        session.device_refresh_token.as_deref(),
        network_enabled,
        0,
        0,
    )?;
    let relay_candidates = relay_candidates_from_device_session_response(
        &response,
        session.active_network_id.as_deref(),
    );
    let network_configs = response
        .network_configs
        .as_ref()
        .map(|configs| configs.items.clone())
        .unwrap_or_default();
    apply_device_session_fields(session, response, true);
    if !relay_candidates.is_empty() {
        session.relay_candidates = relay_candidates.clone();
    }
    Ok((relay_candidates, network_configs))
}

fn apply_device_session_fields(
    session: &mut PersistedSession,
    response: DeviceSessionResponse,
    renew_device_access_token: bool,
) {
    let mqtt = mqtt_from_device_session_response(&response);
    let node_configs = node_configs_from_device_session_response(&response);
    session.device_session_id = Some(response.device_session.session_id);
    let device_token = response.device_session.device_token;
    if renew_device_access_token && session.session_kind == "device" {
        session.access_token = device_token.clone();
        session.authenticated_at_ms = current_timestamp_ms();
        session.expires_in = response
            .device_session
            .device_token_expires_at
            .checked_sub((current_timestamp_ms() / 1_000) as i64)
            .map(|value| value.max(0) as u64);
    }
    session.device_token = Some(device_token);
    session.device_refresh_token = response.device_session.device_refresh_token;
    session.device_token_expires_at = Some(response.device_session.device_token_expires_at);
    session.network_ids = response
        .device_session
        .active_network_ids
        .iter()
        .filter_map(|value| non_empty_session_network_id(value))
        .collect();
    session.mqtt = mqtt.or(session.mqtt.take());
    session.node_configs = node_configs;
    sync_session_device_fields(session, &response.device);
    if let Some(configs) = response.network_configs {
        let items = configs.items;
        if let Some(config) = items
            .iter()
            .rev()
            .find(|item| !item.network_id.trim().is_empty())
        {
            session.active_network_id = non_empty_session_network_id(&config.network_id);
            if let Some(global_ip) = config
                .global_ip
                .as_deref()
                .map(str::trim)
                .filter(|value| !value.is_empty())
            {
                session.virtual_ip = Some(global_ip.to_string());
            }
        }
    }
}

fn node_configs_from_device_session_response(response: &DeviceSessionResponse) -> Vec<NodeConfig> {
    response
        .runtime_endpoints
        .as_ref()
        .map(node_configs_from_runtime_endpoints)
        .unwrap_or_default()
}

fn node_configs_from_runtime_endpoints(
    runtime: &crate::control_plane::RuntimeEndpointsResponse,
) -> Vec<NodeConfig> {
    crate::relay_candidates::update_runtime_device_location(
        &runtime.country_code,
        &runtime.city_code,
    );
    let mut nodes: Vec<_> = runtime
        .node_configs
        .iter()
        .filter_map(|node| {
            let config = NodeConfig {
                node_id: node.node_id.trim().to_string(),
                connection_type: node.connection_type.trim().to_string(),
                transport: node.transport.trim().to_string(),
                path_kind: node.path_kind.trim().to_string(),
                address: node.address.trim().to_string(),
                country_code: node.country_code.trim().to_uppercase(),
                city_code: node.city_code.trim().to_string(),
                priority: node.priority,
            };
            config.is_valid().then_some(config)
        })
        .collect();
    nodes.sort_by(|left, right| {
        left.path_rank()
            .cmp(&right.path_rank())
            .then_with(|| left.priority.cmp(&right.priority))
            .then_with(|| left.node_id.cmp(&right.node_id))
    });
    nodes
}

pub(crate) fn refresh_session_runtime_endpoints(
    client: &ControlPlaneClient,
    session: &mut PersistedSession,
) -> Result<()> {
    let runtime = client.runtime_endpoints(session_device_api_token(session))?;
    session.node_configs = node_configs_from_runtime_endpoints(&runtime);
    Ok(())
}

fn refresh_prepared_session_network(
    client: &ControlPlaneClient,
    session: &mut PersistedSession,
    prepared_configs: Vec<DeviceNetworkConfig>,
) -> Vec<DeviceNetworkConfig> {
    let configs = if prepared_configs.is_empty() {
        session
            .device_id
            .as_deref()
            .map(str::trim)
            .filter(|value| !value.is_empty())
            .and_then(|device_id| {
                client
                    .device_network_configs(session_device_api_token(session), device_id)
                    .ok()
            })
            .unwrap_or_default()
    } else {
        prepared_configs
    };
    if let Some(config) = session
        .active_network_id
        .as_deref()
        .and_then(|network_id| {
            configs
                .iter()
                .find(|config| config.network_id == network_id)
        })
        .or_else(|| configs.last())
    {
        session.active_network_id = non_empty_session_network_id(&config.network_id);
        if let Some(global_ip) = config
            .global_ip
            .as_deref()
            .map(str::trim)
            .filter(|value| !value.is_empty())
        {
            session.virtual_ip = Some(global_ip.to_string());
        }
    } else if session.active_network_id.is_none() {
        if let Ok(Some(network_id)) = client.active_network_id(session_device_api_token(session)) {
            session.active_network_id = Some(network_id);
        }
    }
    configs
}

fn mqtt_from_device_session_response(response: &DeviceSessionResponse) -> Option<MqttCredential> {
    response
        .runtime_endpoints
        .as_ref()
        .and_then(|runtime| runtime.mqtt.clone())
        .or_else(|| response.mqtt.clone())
        .or_else(|| response.device.mqtt.clone())
        .map(normalize_mqtt_credential)
}

fn relay_candidates_from_device_session_response(
    response: &DeviceSessionResponse,
    active_network_id: Option<&str>,
) -> Vec<PersistedRelayCandidate> {
    let mut candidates = Vec::new();
    if let Some(runtime) = response.runtime_endpoints.as_ref() {
        let mut relay_nodes: Vec<_> = runtime
            .node_configs
            .iter()
            .filter(|node| node.connection_type == "relay")
            .collect();
        relay_nodes.sort_by(|left, right| {
            node_config_path_rank(&left.path_kind)
                .cmp(&node_config_path_rank(&right.path_kind))
                .then_with(|| left.priority.cmp(&right.priority))
                .then_with(|| left.node_id.cmp(&right.node_id))
        });
        candidates.extend(relay_nodes.into_iter().filter_map(|node| {
            if node.connection_type != "relay"
                || active_network_id.is_some_and(|network_id| {
                    !node.network_ids.is_empty()
                        && !node.network_ids.iter().any(|value| value == network_id)
                })
            {
                return None;
            }
            Some(RelayCandidate {
                endpoint_id: node.node_id.clone(),
                transport: match node.transport.as_str() {
                    "tcp" => "derp_tcp_tls_443".to_string(),
                    value => value.to_string(),
                },
                address: node.address.clone(),
                country_code: (!node.country_code.trim().is_empty())
                    .then(|| node.country_code.trim().to_uppercase()),
                city_code: (!node.city_code.trim().is_empty())
                    .then(|| node.city_code.trim().to_string()),
                region_id: None,
                cluster_id: None,
                priority: node.priority,
            })
        }));
    }
    dedupe_relay_candidates(candidates)
}

fn dedupe_relay_candidates(candidates: Vec<RelayCandidate>) -> Vec<PersistedRelayCandidate> {
    let mut seen = std::collections::BTreeSet::new();
    candidates
        .into_iter()
        .filter_map(|candidate| {
            let endpoint_id = candidate.endpoint_id.trim().to_string();
            let transport = candidate.transport.trim().to_string();
            let address = candidate.address.trim().to_string();
            if endpoint_id.is_empty() || transport.is_empty() || address.is_empty() {
                return None;
            }
            let key = format!("{transport}|{address}");
            if !seen.insert(key) {
                return None;
            }
            Some(PersistedRelayCandidate {
                endpoint_id,
                transport,
                address,
                country_code: candidate.country_code,
                city_code: candidate.city_code,
                region_id: candidate.region_id,
                cluster_id: candidate.cluster_id,
                reachable_hint: false,
                observed_rtt_ms_hint: None,
                path_score_hint: Some(candidate.priority.into()),
            })
        })
        .collect()
}

// The current app control plane does not expose an independent "create control
// session" endpoint. The only server-side source of truth we need here is the
// resolved self node id from network-config.
pub(crate) fn ensure_session_node_binding(
    client: &ControlPlaneClient,
    session: &mut PersistedSession,
) -> Result<()> {
    let Some(device_id) = session
        .device_id
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .map(str::to_string)
    else {
        return Ok(());
    };
    if session
        .active_network_id
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .is_none()
    {
        return Ok(());
    }
    let node_id = session
        .self_node_id
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .map(str::to_string)
        .unwrap_or_else(|| format!("node-{device_id}"));
    let Ok(node) = client.register_node(session_device_api_token(session), &device_id, &node_id)
    else {
        session.self_node_id = Some(node_id);
        return Ok(());
    };
    let node_id = node.node_id.trim();
    if node_id.is_empty() {
        return Ok(());
    }
    session.self_node_id = Some(node_id.to_string());
    Ok(())
}

pub(crate) fn report_runtime_state(state: &ClientViewState) {
    let Ok(mut session) = load_session() else {
        return;
    };
    if session.active_network_id.is_none() {
        let client = ControlPlaneClient::from_env();
        if let Ok(Some(network_id)) = client.active_network_id(session_device_api_token(&session)) {
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
    let reported_at_ms = current_timestamp_ms();
    let Some(device_token) = session
        .device_token
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
    else {
        return;
    };
    if state.network_enabled {
        let body = serde_json::json!({
            "deviceId": device_id,
            "networkId": network_id,
            "reportedAtMs": reported_at_ms,
            "lastSeenAt": reported_at_ms / 1000,
            "status": "active",
            "rxBytesTotal": state.traffic_rx_bytes.unwrap_or_default(),
            "txBytesTotal": state.traffic_tx_bytes.unwrap_or_default(),
        });
        let _ = client.report_device_runtime(device_token, device_id, body);
    } else {
        let _ = client.deactivate_network(device_token, device_id, network_id);
    }
}

pub(crate) fn sync_session_device_fields(session: &mut PersistedSession, device: &ControlDevice) {
    let device_id = device.device_id.trim();
    if !device_id.is_empty() {
        if session.device_id.as_deref().map(str::trim) != Some(device_id)
            && session
                .device_id
                .as_deref()
                .map(str::trim)
                .filter(|value| !value.is_empty())
                .is_some()
        {
            eprintln!(
                "client-core-service updating session device identity local={} remote={}",
                session.device_id.as_deref().unwrap_or_default(),
                device_id,
            );
        }
        session.device_id = Some(device_id.to_string());
    }
    if session.active_network_id.is_none() {
        if let Some(network_id) = device
            .active_network_id
            .as_deref()
            .map(str::trim)
            .filter(|value| !value.is_empty())
        {
            session.active_network_id = Some(network_id.to_string());
        }
    }
    if let Some(virtual_ip) = device
        .global_ip
        .as_deref()
        .or(device.current_virtual_ip.as_deref())
        .or(device.virtual_ip.as_deref())
        .map(str::trim)
        .filter(|value| !value.is_empty())
    {
        session.virtual_ip = Some(virtual_ip.to_string());
    }
    if let Some(mqtt) = device.mqtt.clone() {
        session.mqtt = Some(normalize_mqtt_credential(mqtt));
    }
}

fn console_env_paths() -> Vec<PathBuf> {
    let mut paths = Vec::new();
    if cfg!(any(target_os = "macos", target_os = "linux")) {
        paths.push(PathBuf::from("/etc/slan/client-v2-console.env"));
    }
    paths.push(app_data_dir().join("SLAN").join("client-v2-console.env"));
    paths
}

fn read_env_value_from_paths(paths: &[PathBuf], key: &str) -> Option<String> {
    for path in paths {
        let Ok(payload) = fs::read_to_string(path) else {
            continue;
        };
        if let Some(value) = payload.lines().find_map(|line| {
            let (name, value) = line.split_once('=')?;
            if name.trim() == key {
                let value = value.trim().trim_matches('"').trim_matches('\'');
                return Some(value.to_string());
            }
            None
        }) {
            return Some(value);
        }
    }
    None
}

fn read_console_env_value(key: &str) -> Option<String> {
    read_env_value_from_paths(&console_env_paths(), key)
        .or_else(|| std::env::var(key).ok())
        .map(|value| value.trim().to_string())
        .filter(|value| !value.is_empty())
}

pub(crate) fn load_pending_device_activation() -> Option<PendingDeviceActivation> {
    let authorization_key = read_console_env_value("SLAN_DEVICE_AUTHORIZATION_KEY")?;
    let base_url = read_console_env_value("SLAN_CONTROL_BASE_URL")
        .map(|value| value.trim_end_matches('/').to_string());
    let enable_network =
        read_console_env_value("SLAN_PENDING_ENABLE_NETWORK").is_some_and(|value| {
            matches!(
                value.to_ascii_lowercase().as_str(),
                "1" | "true" | "yes" | "on"
            )
        });
    Some(PendingDeviceActivation {
        base_url,
        authorization_key,
        enable_network,
    })
}

pub(crate) fn clear_pending_device_activation() -> Result<()> {
    for path in console_env_paths() {
        if path.exists() {
            fs::remove_file(&path).with_context(|| format!("remove {}", path.display()))?;
        }
    }
    Ok(())
}

pub(crate) fn app_data_dir() -> PathBuf {
    if let Some(dir) = APP_DATA_DIR_OVERRIDE.get().and_then(|value| {
        value
            .lock()
            .ok()
            .and_then(|current| current.as_ref().cloned())
    }) {
        return dir;
    }
    if let Some(dir) = std::env::var_os("SLAN_STATE_DIR") {
        return PathBuf::from(dir);
    }
    if cfg!(target_os = "windows") {
        return std::env::var_os("ProgramData")
            .map(PathBuf::from)
            .unwrap_or_else(|| PathBuf::from(r"C:\ProgramData"));
    }
    if cfg!(target_os = "macos") {
        return PathBuf::from("/Library/Application Support");
    }
    if cfg!(target_os = "ios") {
        if let Some(home) = std::env::var_os("HOME") {
            return PathBuf::from(home)
                .join("Library")
                .join("Application Support");
        }
        return std::env::temp_dir();
    }
    if cfg!(target_os = "android") {
        return std::env::temp_dir();
    }
    PathBuf::from("/var/lib")
}

#[allow(dead_code)] // Used by the mobile embedded library, not the desktop service binary.
pub(crate) fn set_app_data_dir_override(value: &str) {
    let value = value.trim();
    if value.is_empty() {
        return;
    }
    let override_dir = APP_DATA_DIR_OVERRIDE.get_or_init(|| Mutex::new(None));
    let mut current = override_dir
        .lock()
        .expect("app data dir override mutex poisoned");
    let next = PathBuf::from(value);
    match current.as_ref() {
        None => *current = Some(next),
        Some(existing) if existing != &next => {
            eprintln!(
                "client-core-service ignored app data dir change after initialization: {}",
                next.display()
            );
        }
        Some(_) => {}
    }
}

pub(crate) fn load_session() -> Result<PersistedSession> {
    let device_id = local_stable_device_id().context("load device id for client config")?;
    let mut session = crate::client_config::load_secret::<PersistedSession>(
        &device_id,
        crate::client_config::KEY_SESSION,
    )?
    .ok_or_else(|| std::io::Error::new(std::io::ErrorKind::NotFound, "session not found"))?;
    session.relay_candidates.clear();
    normalize_session_mqtt_topic_prefix(&mut session);
    Ok(session)
}

pub(crate) fn persist_session(session: &PersistedSession) -> Result<()> {
    let device_id = local_stable_device_id().context("load device id for client config")?;
    let mut session = session.clone();
    session.relay_candidates.clear();
    normalize_session_mqtt_topic_prefix(&mut session);
    let previous = crate::client_config::load_secret::<PersistedSession>(
        &device_id,
        crate::client_config::KEY_SESSION,
    )?;
    if previous
        .as_ref()
        .is_some_and(|previous| session_scope_changed(previous, &session))
    {
        clear_session_runtime_state();
    }
    crate::client_config::store_secret(&device_id, crate::client_config::KEY_SESSION, &session)
}

fn session_scope_changed(previous: &PersistedSession, next: &PersistedSession) -> bool {
    normalized_session_scope_value(previous.device_id.as_deref())
        != normalized_session_scope_value(next.device_id.as_deref())
        || normalized_session_scope_value(previous.active_network_id.as_deref())
            != normalized_session_scope_value(next.active_network_id.as_deref())
}

fn normalized_session_scope_value(value: Option<&str>) -> Option<&str> {
    value.map(str::trim).filter(|value| !value.is_empty())
}

#[cfg(test)]
mod session_scope_tests {
    use super::{
        clear_session_runtime_state, current_session_runtime_epoch, lock_session_runtime_epoch,
        session_scope_changed, PersistedSession,
    };
    use std::{sync::mpsc, time::Duration};

    fn session(device_id: Option<&str>, network_id: Option<&str>) -> PersistedSession {
        let mut session = PersistedSession::empty();
        session.device_id = device_id.map(str::to_string);
        session.active_network_id = network_id.map(str::to_string);
        session
    }

    #[test]
    fn token_refresh_keeps_scope_but_device_or_network_changes_reset_it() {
        let previous = session(Some("device-1"), Some("network-1"));
        let mut refreshed = previous.clone();
        refreshed.access_token = "new-access-token".to_string();
        refreshed.device_token = Some("new-device-token".to_string());
        assert!(!session_scope_changed(&previous, &refreshed));

        assert!(session_scope_changed(
            &previous,
            &session(Some("device-2"), Some("network-1"))
        ));
        assert!(session_scope_changed(
            &previous,
            &session(Some("device-1"), Some("network-2"))
        ));
        assert!(!session_scope_changed(
            &session(None, None),
            &session(Some("  "), Some(""))
        ));
    }

    #[test]
    fn session_clear_waits_for_active_event_commit_and_invalidates_old_epoch() {
        let _test_lock = crate::test_env_lock();
        let epoch = current_session_runtime_epoch();
        let event_guard = lock_session_runtime_epoch(epoch).expect("lock event epoch");
        let (cleared_tx, cleared_rx) = mpsc::sync_channel(1);
        let clear_task = std::thread::spawn(move || {
            clear_session_runtime_state();
            cleared_tx.send(()).expect("announce session clear");
        });

        assert!(cleared_rx.recv_timeout(Duration::from_millis(20)).is_err());
        drop(event_guard);
        cleared_rx
            .recv_timeout(Duration::from_secs(1))
            .expect("session clear completes after event commit");
        clear_task.join().expect("join session clear task");

        assert!(current_session_runtime_epoch() > epoch);
        assert!(lock_session_runtime_epoch(epoch).is_err());
    }
}

fn clear_session_runtime_state() {
    let _lifecycle = SESSION_RUNTIME_LIFECYCLE
        .write()
        .unwrap_or_else(|error| error.into_inner());
    SESSION_RUNTIME_EPOCH.fetch_add(1, Ordering::AcqRel);
    crate::network_module::clear_network_module();
    runtime_network_state_store().clear();
    clear_resolver_runtime_state();
    replace_runtime_relay_candidates(Vec::new());
}

pub(crate) fn current_session_runtime_epoch() -> u64 {
    SESSION_RUNTIME_EPOCH.load(Ordering::Acquire)
}

pub(crate) fn lock_session_runtime_epoch(
    expected_epoch: u64,
) -> Result<RwLockReadGuard<'static, ()>> {
    let guard = SESSION_RUNTIME_LIFECYCLE
        .read()
        .unwrap_or_else(|error| error.into_inner());
    anyhow::ensure!(
        current_session_runtime_epoch() == expected_epoch,
        "stale session runtime epoch"
    );
    Ok(guard)
}

pub(crate) fn remove_session() -> Result<()> {
    clear_session_runtime_state();
    let device_id = local_stable_device_id().context("load device id for client config")?;
    crate::client_config::remove_secret(&device_id, crate::client_config::KEY_SESSION)?;
    Ok(())
}

pub(crate) fn remove_device_authorization() -> Result<()> {
    remove_session()?;
    let device_id = local_stable_device_id().context("load device id for client config")?;
    crate::client_config::remove_secret(
        &device_id,
        crate::client_config::KEY_DEVICE_AUTHORIZATION_KEY,
    )?;
    Ok(())
}

pub(crate) fn session_not_found_error(error: &anyhow::Error) -> bool {
    error
        .root_cause()
        .downcast_ref::<std::io::Error>()
        .is_some_and(|error| error.kind() == std::io::ErrorKind::NotFound)
}

pub(crate) fn current_timestamp_ms() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|duration| duration.as_millis() as u64)
        .unwrap_or_default()
}

#[cfg(test)]
mod tests {
    use super::*;

    fn test_mqtt(expires_at: Option<i64>) -> MqttCredential {
        MqttCredential {
            broker_url: "mqtt://127.0.0.1:1883".to_string(),
            client_id: "client-1".to_string(),
            username: "user".to_string(),
            password: "pass".to_string(),
            topic_prefix: "slan/devices/device-1".to_string(),
            expires_at,
        }
    }

    #[test]
    fn device_session_renews_when_mqtt_credential_is_expiring() {
        let mut session = PersistedSession::empty();
        session.session_kind = "device".to_string();
        session.device_token = Some("dt-1".to_string());
        session.device_token_expires_at = Some(((current_timestamp_ms() / 1_000) + 86_400) as i64);
        session.mqtt = Some(test_mqtt(Some(
            ((current_timestamp_ms() + SESSION_RENEW_WINDOW_MS - 1_000) / 1_000) as i64,
        )));

        assert!(device_session_should_renew(&session));
    }

    #[test]
    fn device_session_does_not_renew_for_far_future_mqtt_credential() {
        let mut session = PersistedSession::empty();
        session.session_kind = "device".to_string();
        session.device_token = Some("dt-1".to_string());
        session.device_token_expires_at = Some(((current_timestamp_ms() / 1_000) + 86_400) as i64);
        session.mqtt = Some(test_mqtt(Some(
            ((current_timestamp_ms() + SESSION_RENEW_WINDOW_MS + 86_400_000) / 1_000) as i64,
        )));

        assert!(!device_session_should_renew(&session));
    }

    #[test]
    fn expired_standalone_device_session_is_renewed() {
        let mut session = PersistedSession::empty();
        session.session_kind = "device".to_string();
        session.access_token = "device-token-old".to_string();
        session.device_token = Some("device-token-old".to_string());
        session.device_refresh_token = Some("device-refresh-token".to_string());
        session.device_token_expires_at = Some((current_timestamp_ms() / 1_000) as i64 - 1);

        assert!(session_is_expired(&session));
        assert!(device_session_should_renew(&session));
    }

    #[test]
    fn standalone_device_session_without_ip_is_renewable_before_expiry() {
        let mut session = PersistedSession::empty();
        session.session_kind = "device".to_string();
        session.access_token = "device-token".to_string();
        session.device_token = Some("device-token".to_string());
        session.device_refresh_token = Some("device-refresh-token".to_string());
        session.device_token_expires_at = Some(((current_timestamp_ms() / 1_000) + 86_400) as i64);

        assert!(session.virtual_ip.is_none());
        assert!(!device_session_should_renew(&session));
        assert!(device_session_needs_refresh(&session));
    }

    #[test]
    fn persisted_session_json_contains_device_credentials() {
        let mut session = PersistedSession::empty();
        session.session_kind = "device".to_string();
        session.access_token = "device-token".to_string();
        session.device_session_id = Some("device-session-1".to_string());
        session.device_token = Some("device-token".to_string());
        session.device_refresh_token = Some("device-refresh-token".to_string());
        session.device_token_expires_at = Some(4_102_444_800);

        let payload = serde_json::to_value(&session).expect("encode persisted session");

        assert_eq!(payload["deviceSessionId"], "device-session-1");
        assert_eq!(payload["deviceToken"], "device-token");
        assert_eq!(payload["deviceRefreshToken"], "device-refresh-token");
        assert_eq!(payload["deviceTokenExpiresAt"], 4_102_444_800i64);
    }

    #[test]
    fn device_session_response_uses_runtime_endpoints() {
        let response: DeviceSessionResponse = serde_json::from_value(serde_json::json!({
            "device": {
                "deviceId": "device-1",
                "activeNetworkId": "net-1"
            },
            "deviceSession": {
                "sessionId": "session-1",
                "deviceId": "device-1",
                "deviceToken": "token-1",
                "deviceTokenExpiresAt": 4_102_444_800i64,
                "activeNetworkIds": ["net-1"]
            },
            "mqtt": {
                "brokerUrl": "mqtt://old.example.com:1883",
                "clientId": "old-client",
                "username": "old-user",
                "password": "old-pass",
                "topicPrefix": "slan/devices/device-1",
                "expiresAt": 4_102_444_800i64
            },
            "networkConfigs": {
                "items": [{
                    "networkId": "net-1",
                    "deviceId": "device-1",
                    "globalIp": "10.0.0.2",
                    "relayCandidates": [{
                        "endpointId": "relay-old",
                        "transport": "udp",
                        "address": "relay.example.com:29110"
                    }]
                }]
            },
            "runtimeEndpoints": {
                "mqtt": {
                    "brokerUrl": "mqtt://47.245.40.231:1883",
                    "clientId": "client-1",
                    "username": "user",
                    "password": "pass",
                    "topicPrefix": "slan/devices/device-1",
                    "expiresAt": 4_102_444_800i64
                },
                "nodeConfigs": [{
                    "nodeId": "punch-1",
                    "connectionType": "direct",
                    "transport": "udp",
                    "pathKind": "direct_udp",
                    "address": "47.245.40.231:29130",
                    "priority": 100,
                    "networkIds": []
                }, {
                    "nodeId": "derp-1",
                    "connectionType": "relay",
                    "transport": "tcp",
                    "pathKind": "relay_tcp",
                    "address": "47.245.40.231:29120",
                    "priority": 300,
                    "networkIds": ["net-1"]
                }],
                "refreshedAt": 1000
            }
        }))
        .expect("decode device session response");

        let session = persisted_session_from_device_session(response);

        assert_eq!(
            session.mqtt.as_ref().map(|item| item.broker_url.as_str()),
            Some("mqtt://47.245.40.231:1883")
        );
        assert_eq!(
            session.mqtt.as_ref().map(|item| item.topic_prefix.as_str()),
            Some("slan/devices/device-1")
        );
        assert_eq!(session.active_network_id.as_deref(), Some("net-1"));
        assert_eq!(session.virtual_ip.as_deref(), Some("10.0.0.2"));
        assert_eq!(session.relay_candidates.len(), 1);
        assert_eq!(session.relay_candidates[0].endpoint_id, "derp-1");
        assert_eq!(session.relay_candidates[0].address, "47.245.40.231:29120");
        assert_eq!(session.node_configs.len(), 2);
        assert_eq!(session.node_configs[0].node_id, "punch-1");
        assert_eq!(session.node_configs[0].path_kind, "direct_udp");
    }

    #[test]
    fn empty_runtime_endpoint_list_clears_stale_node_configs() {
        let mut session = PersistedSession::empty();
        session.node_configs.push(NodeConfig {
            node_id: "stale-punch".to_string(),
            connection_type: "direct".to_string(),
            transport: "udp".to_string(),
            path_kind: "direct_udp".to_string(),
            address: "192.0.2.1:29130".to_string(),
            country_code: "US".to_string(),
            city_code: "".to_string(),
            priority: 100,
        });
        let response: DeviceSessionResponse = serde_json::from_value(serde_json::json!({
            "device": {"deviceId": "device-1"},
            "deviceSession": {
                "sessionId": "session-1",
                "deviceId": "device-1",
                "deviceToken": "token-1",
                "deviceTokenExpiresAt": 4_102_444_800i64,
                "activeNetworkIds": []
            },
            "runtimeEndpoints": {"nodeConfigs": []}
        }))
        .expect("decode empty runtime endpoints");

        apply_device_session_fields(&mut session, response, false);

        assert!(session.node_configs.is_empty());
    }

    #[test]
    fn sync_session_device_fields_updates_existing_local_device_identity() {
        let mut session = PersistedSession::empty();
        session.device_id = Some("local-device".to_string());

        sync_session_device_fields(
            &mut session,
            &ControlDevice {
                device_id: "remote-device".to_string(),
                active_network_id: Some("net-1".to_string()),
                status: None,
                membership_status: None,
                current_virtual_ip: Some("10.0.1.63".to_string()),
                virtual_ip: None,
                global_ip: None,
                global_name: None,
                mqtt: None,
            },
        );

        assert_eq!(session.device_id.as_deref(), Some("remote-device"));
        assert_eq!(session.active_network_id.as_deref(), Some("net-1"));
        assert_eq!(session.virtual_ip.as_deref(), Some("10.0.1.63"));
    }
}
