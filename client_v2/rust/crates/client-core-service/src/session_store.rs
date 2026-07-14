use std::{
    fs,
    path::PathBuf,
    time::{SystemTime, UNIX_EPOCH},
};

use anyhow::{Context, Result};
use client_core::{AuthPayload, ClientCommand, ClientRuntime, ClientViewState};
use serde::{Deserialize, Serialize};

use crate::{
    control_plane::{
        local_stable_device_id, set_control_base_url_override, ControlDevice, ControlPlaneClient,
        DeviceSessionResponse, MqttCredential, RelayCandidate,
    },
    network_module::{network_module_configs_for_session, replace_network_module_configs},
    relay_candidates::replace_runtime_relay_candidates,
    relay_models::PersistedRelayCandidate,
};

const SESSION_RENEW_WINDOW_MS: u64 = 5 * 60 * 1_000;

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct PersistedSession {
    pub(crate) access_token: String,
    pub(crate) refresh_token: Option<String>,
    pub(crate) user_id: String,
    pub(crate) user_label: String,
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
    pub(crate) mqtt: Option<MqttCredential>,
    pub(crate) expires_in: Option<u64>,
    pub(crate) authenticated_at_ms: u64,
}

#[derive(Debug, Clone, Default)]
pub(crate) struct PendingConsoleLogin {
    pub(crate) base_url: Option<String>,
    pub(crate) email: String,
    pub(crate) password: String,
    pub(crate) device_name: Option<String>,
    pub(crate) enable_network: bool,
}

impl From<PersistedSession> for AuthPayload {
    fn from(session: PersistedSession) -> Self {
        Self {
            access_token: session.access_token,
            refresh_token: session.refresh_token,
            user_id: session.user_id,
            user_label: session.user_label,
            device_id: session.device_id,
            active_network_id: session.active_network_id,
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
            session_kind: "user".to_string(),
            device_token_expires_at: None,
            device_session_id: None,
            device_token: None,
            device_refresh_token: None,
            device_id: payload.device_id,
            self_node_id: None,
            active_network_id: payload.active_network_id.clone(),
            network_ids: payload.active_network_id.into_iter().collect(),
            virtual_ip: payload.virtual_ip,
            relay_candidates: Vec::new(),
            mqtt: None,
            expires_in: payload.expires_in,
            authenticated_at_ms: current_timestamp_ms(),
        }
    }
}

impl PersistedSession {
    pub(crate) fn empty() -> Self {
        Self {
            access_token: String::new(),
            refresh_token: None,
            user_id: String::new(),
            user_label: String::new(),
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
            mqtt: None,
            expires_in: None,
            authenticated_at_ms: current_timestamp_ms(),
        }
    }

    pub(crate) fn prelogin(device_id: impl Into<String>, mqtt: Option<MqttCredential>) -> Self {
        let mut session = Self::empty();
        session.session_kind = "prelogin".to_string();
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
    let prefix = mqtt.topic_prefix.trim().trim_matches('/');
    mqtt.topic_prefix = match prefix.strip_prefix("slan/v1/") {
        Some(suffix) => format!("slan/{suffix}"),
        None if prefix == "slan/v1" => "slan".to_string(),
        None => prefix.to_string(),
    };
}

fn normalize_session_mqtt_topic_prefix(session: &mut PersistedSession) {
    if let Some(mqtt) = session.mqtt.as_mut() {
        normalize_mqtt_topic_prefix(mqtt);
    }
}

pub(crate) fn load_valid_registered_session() -> Option<PersistedSession> {
    let Ok(session) = load_session() else {
        return bootstrap_session_from_env().ok();
    };
    if session.access_token.trim().is_empty() {
        if prelogin_session_is_usable(&session) {
            return Some(session);
        }
        let _ = remove_session();
        return bootstrap_session_from_env().ok();
    }
    match ensure_session_device_registered(session.clone()) {
        Ok(session) => Some(session),
        Err(error) if session_auth_invalid_error(&error) => {
            eprintln!("client-core-service session invalid; clearing local session: {error:#}");
            let _ = remove_session();
            bootstrap_session_from_env().ok()
        }
        Err(error) => {
            eprintln!("client-core-service startup device registration skipped: {error:#}");
            Some(session)
        }
    }
}

pub(crate) fn refresh_startup_session<P>(runtime: &mut ClientRuntime<P>)
where
    P: client_core::PlatformNetwork,
{
    let Some(session) = load_valid_registered_session() else {
        let _ = runtime.dispatch(ClientCommand::Logout);
        return;
    };
    if session.access_token.trim().is_empty() {
        return;
    }
    let _ = runtime.dispatch(ClientCommand::ApplyDeviceUserLogin(session.into()));
}

pub(crate) fn session_is_expired(session: &PersistedSession) -> bool {
    if session.access_token.trim().is_empty() {
        return !prelogin_session_is_usable(session);
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

fn prelogin_session_is_usable(session: &PersistedSession) -> bool {
    if session.session_kind != "prelogin" {
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

fn user_session_should_renew(session: &PersistedSession) -> bool {
    if session.access_token.trim().is_empty() || session.session_kind == "device" {
        return false;
    }
    let Some(expires_in) = session.expires_in else {
        return false;
    };
    let expires_at_ms = session
        .authenticated_at_ms
        .saturating_add(expires_in.saturating_mul(1_000));
    current_timestamp_ms().saturating_add(SESSION_RENEW_WINDOW_MS) >= expires_at_ms
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

fn bootstrap_session_from_env() -> Result<PersistedSession> {
    if let Some(base_url) = read_bootstrap_env_value("SLAN_CONTROL_BASE_URL") {
        set_control_base_url_override(&base_url);
    }
    let installation_key = read_bootstrap_env_value("SLAN_INSTALLATION_KEY")
        .context("missing SLAN_INSTALLATION_KEY")?;
    let client = ControlPlaneClient::from_env();
    let response = client.bootstrap_device_session(&installation_key)?;
    let mut session = persisted_session_from_device_session(response);
    backfill_desktop_session_mqtt(&client, &mut session);
    refresh_session_network_from_device_configs(&client, &mut session);
    ensure_session_node_binding(&client, &mut session)?;
    persist_session(&session)?;
    Ok(session)
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
        .first()
        .cloned()
        .or_else(|| config.map(|item| item.network_id.clone()))
        .or_else(|| response.device.active_network_id.clone());
    let network_ids = response.device_session.active_network_ids.clone();
    let virtual_ip = config
        .and_then(|item| item.global_ip.clone())
        .or_else(|| response.device.global_ip.clone())
        .or(response.device.current_virtual_ip.clone())
        .or(response.device.virtual_ip.clone());
    let device_token = response.device_session.device_token.clone();
    let relay_candidates =
        relay_candidates_from_device_session_response(&response, active_network_id.as_deref());
    let mqtt = mqtt_from_device_session_response(&response);
    PersistedSession {
        access_token: device_token.clone(),
        refresh_token: None,
        user_id: response.device_session.user_id.unwrap_or_default(),
        user_label: response
            .device
            .owner_email
            .clone()
            .unwrap_or_else(|| "device-session".to_string()),
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
        mqtt,
        expires_in: response
            .device_session
            .device_token_expires_at
            .checked_sub((current_timestamp_ms() / 1_000) as i64)
            .map(|value| value.max(0) as u64),
        authenticated_at_ms: current_timestamp_ms(),
    }
}

pub(crate) fn session_auth_invalid_error(error: &anyhow::Error) -> bool {
    let message = error.to_string().to_ascii_lowercase();
    message.contains("http 401")
        || message.contains("unauthorized")
        || message.contains("invalid token")
        || message.contains("token expired")
}

pub(crate) fn ensure_session_device_registered(
    mut session: PersistedSession,
) -> Result<PersistedSession> {
    if session.access_token.trim().is_empty() {
        return Ok(session);
    }
    let client = ControlPlaneClient::from_env();
    if session.session_kind == "device" {
        ensure_bound_device_session(&client, &mut session, false)?;
        backfill_desktop_session_mqtt(&client, &mut session);
        refresh_session_network_from_device_configs(&client, &mut session);
        ensure_session_node_binding(&client, &mut session)?;
        persist_session(&session)?;
        return Ok(session);
    }
    let user_token_renewed = renew_user_session_if_needed(&client, &mut session)?;
    ensure_bound_device_session(&client, &mut session, user_token_renewed)?;
    backfill_desktop_session_mqtt(&client, &mut session);
    refresh_session_network_from_device_configs(&client, &mut session);
    ensure_session_node_binding(&client, &mut session)?;
    persist_session(&session)?;
    Ok(session)
}

pub(crate) fn hydrate_session_from_control_plane(payload: AuthPayload) -> Result<PersistedSession> {
    let client = ControlPlaneClient::from_env();
    let mut session = PersistedSession::from(payload);
    bind_session_device_session(&client, &mut session)?;
    backfill_desktop_session_mqtt(&client, &mut session);
    refresh_session_network_from_device_configs(&client, &mut session);
    ensure_session_node_binding(&client, &mut session)?;
    Ok(session)
}

pub(crate) fn prepare_client_login_session(platform: &str) -> Result<PersistedSession> {
    let device_id = local_stable_device_id().context("init client device id")?;
    let login = ControlPlaneClient::from_env()
        .prepare_device_login(&device_id, platform)
        .context("prepare client device login")?;
    let mqtt = login
        .mqtt
        .ok_or_else(|| anyhow::anyhow!("server did not return mqtt credential"))?;
    let session = PersistedSession::prelogin(login.device_id, Some(mqtt));
    persist_session(&session)?;
    Ok(session)
}

fn renew_user_session_if_needed(
    client: &ControlPlaneClient,
    session: &mut PersistedSession,
) -> Result<bool> {
    if !user_session_should_renew(session) {
        return Ok(false);
    }
    let payload = client.renew_user_session(
        &session.access_token,
        session.refresh_token.as_deref(),
        session.device_id.as_deref(),
        &session.user_label,
    )?;
    session.access_token = payload.access_token;
    session.refresh_token = payload.refresh_token;
    session.user_id = payload.user_id;
    session.user_label = payload.user_label;
    session.expires_in = payload.expires_in;
    session.authenticated_at_ms = current_timestamp_ms();
    if let Some(device_id) = payload.device_id {
        session.device_id = Some(device_id);
    }
    if let Some(active_network_id) = payload.active_network_id {
        session.active_network_id = Some(active_network_id);
    }
    if let Some(virtual_ip) = payload.virtual_ip {
        session.virtual_ip = Some(virtual_ip);
    }
    Ok(true)
}

fn ensure_bound_device_session(
    client: &ControlPlaneClient,
    session: &mut PersistedSession,
    force_renew: bool,
) -> Result<()> {
    let has_device_token = session
        .device_token
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .is_some();
    if !has_device_token {
        return bind_session_device_session(client, session);
    }
    if force_renew || device_session_should_renew(session) {
        return renew_bound_device_session(client, session);
    }
    Ok(())
}

fn renew_bound_device_session(
    client: &ControlPlaneClient,
    session: &mut PersistedSession,
) -> Result<()> {
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
    let mqtt = mqtt_from_device_session_response(&response);
    session.device_session_id = Some(response.device_session.session_id);
    let renewed_device_token = response.device_session.device_token;
    if session.session_kind == "device" {
        session.access_token = renewed_device_token.clone();
        session.authenticated_at_ms = current_timestamp_ms();
        session.expires_in = response
            .device_session
            .device_token_expires_at
            .checked_sub((current_timestamp_ms() / 1_000) as i64)
            .map(|value| value.max(0) as u64);
    }
    session.device_token = Some(renewed_device_token);
    session.device_refresh_token = response.device_session.device_refresh_token;
    session.device_token_expires_at = Some(response.device_session.device_token_expires_at);
    session.network_ids = response.device_session.active_network_ids.clone();
    session.mqtt = mqtt.or(session.mqtt.take());
    if !relay_candidates.is_empty() {
        session.relay_candidates = relay_candidates.clone();
        replace_runtime_relay_candidates(relay_candidates);
    }
    sync_session_device_fields(session, &response.device);
    if let Some(configs) = response.network_configs {
        let items = configs.items;
        replace_network_module_configs(items.clone());
        if let Some(config) = items.last() {
            session.active_network_id = Some(config.network_id.clone());
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
    Ok(())
}

fn bind_session_device_session(
    client: &ControlPlaneClient,
    session: &mut PersistedSession,
) -> Result<()> {
    let device_id = session
        .device_id
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .map(str::to_string)
        .context("missing device id for device session bind")?;
    let response = client.bind_device_session(&session.access_token, &device_id)?;
    let relay_candidates = relay_candidates_from_device_session_response(
        &response,
        session.active_network_id.as_deref(),
    );
    let mqtt = mqtt_from_device_session_response(&response);
    session.device_session_id = Some(response.device_session.session_id);
    session.device_token = Some(response.device_session.device_token);
    session.device_refresh_token = response.device_session.device_refresh_token;
    session.device_token_expires_at = Some(response.device_session.device_token_expires_at);
    session.network_ids = response.device_session.active_network_ids.clone();
    session.mqtt = mqtt.or(session.mqtt.take());
    if !relay_candidates.is_empty() {
        session.relay_candidates = relay_candidates.clone();
        replace_runtime_relay_candidates(relay_candidates);
    }
    sync_session_device_fields(session, &response.device);
    if let Some(configs) = response.network_configs {
        let items = configs.items;
        replace_network_module_configs(items.clone());
        if let Some(config) = items.last() {
            session.active_network_id = Some(config.network_id.clone());
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
    Ok(())
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

fn should_backfill_desktop_session_mqtt(session: &PersistedSession) -> bool {
    cfg!(any(
        target_os = "macos",
        target_os = "windows",
        target_os = "linux"
    )) && session.mqtt.is_none()
        && session
            .device_id
            .as_deref()
            .map(str::trim)
            .filter(|value| !value.is_empty())
            .is_some()
}

fn backfill_desktop_session_mqtt(client: &ControlPlaneClient, session: &mut PersistedSession) {
    if !should_backfill_desktop_session_mqtt(session) {
        return;
    }
    let Some(device_id) = session
        .device_id
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
    else {
        return;
    };
    match client.prepare_device_login(device_id, std::env::consts::OS) {
        Ok(prepared) => {
            if let Some(mqtt) = prepared.mqtt.map(normalize_mqtt_credential) {
                session.mqtt = Some(mqtt);
            }
        }
        Err(error) => {
            eprintln!(
                "client-core-service desktop mqtt backfill skipped device={} error={error:#}",
                device_id
            );
        }
    }
}

fn relay_candidates_from_device_session_response(
    response: &DeviceSessionResponse,
    active_network_id: Option<&str>,
) -> Vec<PersistedRelayCandidate> {
    let mut candidates = Vec::new();
    if let Some(runtime) = response.runtime_endpoints.as_ref() {
        if let Some(network_id) = active_network_id {
            if let Some(network) = runtime
                .networks
                .iter()
                .find(|network| network.network_id == network_id)
            {
                candidates.extend(network.relay_candidates.iter().cloned());
            }
        }
        if candidates.is_empty() {
            candidates.extend(runtime.relay_candidates.iter().cloned());
        }
    }
    if candidates.is_empty() {
        if let Some(configs) = response.network_configs.as_ref() {
            let selected = active_network_id
                .and_then(|network_id| {
                    configs
                        .items
                        .iter()
                        .find(|config| config.network_id == network_id)
                })
                .or_else(|| configs.items.last());
            if let Some(config) = selected {
                candidates.extend(config.relay_candidates.iter().cloned());
            }
        }
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
                region_id: candidate.region_id,
                cluster_id: candidate.cluster_id,
                reachable_hint: candidate.reachable,
                observed_rtt_ms_hint: candidate.observed_rtt_ms,
                path_score_hint: candidate.path_score,
                selected_hint: candidate.selected,
            })
        })
        .collect()
}

fn refresh_session_network_from_device_configs(
    client: &ControlPlaneClient,
    session: &mut PersistedSession,
) {
    let configs = network_module_configs_for_session(client, session);
    if !configs.is_empty() {
        let selected = session
            .active_network_id
            .as_deref()
            .and_then(|network_id| {
                configs
                    .iter()
                    .find(|config| config.network_id == network_id)
            })
            .or_else(|| configs.last());
        if let Some(config) = selected {
            session.active_network_id = Some(config.network_id.clone());
            if let Some(global_ip) = config
                .global_ip
                .as_deref()
                .map(str::trim)
                .filter(|value| !value.is_empty())
            {
                session.virtual_ip = Some(global_ip.to_string());
            }
            return;
        }
    }
    if session.active_network_id.is_none() {
        if let Ok(Some(network_id)) = client.active_network_id(session_device_api_token(session)) {
            session.active_network_id = Some(network_id);
        }
    }
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
    if let Some(mqtt) = device.mqtt.clone() {
        session.mqtt = Some(normalize_mqtt_credential(mqtt));
    }
}

fn legacy_session_file_path() -> PathBuf {
    let base = app_data_dir();
    base.join("SLAN").join("client-v2-session.json")
}

fn bootstrap_env_paths() -> Vec<PathBuf> {
    let mut paths = Vec::new();
    if cfg!(any(target_os = "macos", target_os = "linux")) {
        paths.push(PathBuf::from("/etc/slan/bootstrap.env"));
    }
    let base = app_data_dir();
    paths.push(base.join("SLAN").join("bootstrap.env"));
    paths
}

fn console_env_paths() -> Vec<PathBuf> {
    let mut paths = Vec::new();
    if cfg!(any(target_os = "macos", target_os = "linux")) {
        paths.push(PathBuf::from("/etc/slan/client-v2-console.env"));
    }
    let base = app_data_dir();
    if cfg!(target_os = "windows") {
        paths.push(base.join("SLAN").join("client-v2-console.env"));
    } else {
        paths.push(base.join("SLAN").join("client-v2-console.env"));
    }
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

fn read_bootstrap_env_value(key: &str) -> Option<String> {
    read_env_value_from_paths(&bootstrap_env_paths(), key).and_then(|value| {
        let trimmed = value.trim();
        if trimmed.is_empty() {
            None
        } else {
            Some(trimmed.to_string())
        }
    })
}

fn read_console_env_value(key: &str) -> Option<String> {
    read_env_value_from_paths(&console_env_paths(), key)
}

fn parse_env_bool(value: &str) -> bool {
    matches!(
        value.trim().to_ascii_lowercase().as_str(),
        "1" | "true" | "yes" | "on"
    )
}

pub(crate) fn load_pending_console_login() -> Option<PendingConsoleLogin> {
    let email = read_console_env_value("SLAN_PENDING_EMAIL")?
        .trim()
        .to_string();
    let password = read_console_env_value("SLAN_PENDING_PASSWORD")?
        .trim()
        .to_string();
    if email.is_empty() || password.is_empty() {
        return None;
    }
    let base_url = read_console_env_value("SLAN_CONTROL_BASE_URL")
        .map(|value| value.trim().trim_end_matches('/').to_string())
        .filter(|value| !value.is_empty());
    let device_name = read_console_env_value("SLAN_PENDING_DEVICE_NAME")
        .map(|value| value.trim().to_string())
        .filter(|value| !value.is_empty());
    let enable_network = read_console_env_value("SLAN_PENDING_ENABLE_NETWORK")
        .map(|value| parse_env_bool(&value))
        .unwrap_or(false);
    Some(PendingConsoleLogin {
        base_url,
        email,
        password,
        device_name,
        enable_network,
    })
}

pub(crate) fn clear_pending_console_login() -> Result<()> {
    for path in console_env_paths() {
        if path.exists() {
            fs::remove_file(&path).with_context(|| format!("remove {}", path.display()))?;
        }
    }
    Ok(())
}

pub(crate) fn app_data_dir() -> PathBuf {
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

pub(crate) fn load_session() -> Result<PersistedSession> {
    let device_id = local_stable_device_id().context("load device id for client config")?;
    let mut session = match crate::client_config::load_secret::<PersistedSession>(
        &device_id,
        crate::client_config::KEY_SESSION,
    )? {
        Some(session) => session,
        None => migrate_legacy_session(&device_id)?,
    };
    session.relay_candidates.clear();
    normalize_session_mqtt_topic_prefix(&mut session);
    Ok(session)
}

pub(crate) fn persist_session(session: &PersistedSession) -> Result<()> {
    let device_id = local_stable_device_id().context("load device id for client config")?;
    let mut session = session.clone();
    session.relay_candidates.clear();
    normalize_session_mqtt_topic_prefix(&mut session);
    crate::client_config::store_secret(&device_id, crate::client_config::KEY_SESSION, &session)
}

fn migrate_legacy_session(device_id: &str) -> Result<PersistedSession> {
    let path = legacy_session_file_path();
    let payload = fs::read(&path).with_context(|| format!("read {}", path.display()))?;
    let session: PersistedSession =
        serde_json::from_slice(&payload).with_context(|| format!("decode {}", path.display()))?;
    crate::client_config::store_secret(device_id, crate::client_config::KEY_SESSION, &session)?;
    fs::remove_file(&path).with_context(|| format!("remove {}", path.display()))?;
    Ok(session)
}

pub(crate) fn revoke_remote_sessions(session: &PersistedSession) {
    if session.access_token.trim().is_empty() {
        return;
    }
    let client = ControlPlaneClient::from_env();
    if let Err(error) =
        client.logout_sessions(&session.access_token, session.device_token.as_deref())
    {
        eprintln!("client-core-service remote logout skipped: {error:#}");
    }
}

pub(crate) fn remove_session() -> Result<()> {
    let device_id = local_stable_device_id().context("load device id for client config")?;
    crate::client_config::remove_secret(&device_id, crate::client_config::KEY_SESSION)?;
    let legacy_path = legacy_session_file_path();
    if legacy_path.exists() {
        fs::remove_file(&legacy_path)
            .with_context(|| format!("remove {}", legacy_path.display()))?;
    }
    Ok(())
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
        session.session_kind = "user".to_string();
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
        session.session_kind = "user".to_string();
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
    fn persisted_session_json_contains_device_credentials() {
        let mut session = PersistedSession::empty();
        session.session_kind = "user".to_string();
        session.access_token = "user-token".to_string();
        session.refresh_token = Some("user-refresh-token".to_string());
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
    fn legacy_mqtt_topic_prefix_is_normalized() {
        let mut mqtt = test_mqtt(None);
        mqtt.topic_prefix = "slan/v1/devices/device-1".to_string();
        normalize_mqtt_topic_prefix(&mut mqtt);
        assert_eq!(mqtt.topic_prefix, "slan/devices/device-1");

        mqtt.topic_prefix = " /slan/v1/ ".to_string();
        normalize_mqtt_topic_prefix(&mut mqtt);
        assert_eq!(mqtt.topic_prefix, "slan");
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
                "userId": "user-1",
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
                    "topicPrefix": "slan/v1/devices/device-1",
                    "expiresAt": 4_102_444_800i64
                },
                "punchNodes": [{
                    "nodeId": "punch-1",
                    "name": "Punch",
                    "region": "ap-east",
                    "address": "47.245.40.231:29130",
                    "publicUdpIp": "47.245.40.231",
                    "publicUdpPort": 29130
                }],
                "relayCandidates": [{
                    "endpointId": "relay-1",
                    "transport": "udp",
                    "address": "47.245.40.231:29110"
                }],
                "networks": [{
                    "networkId": "net-1",
                    "relayCandidates": [{
                        "endpointId": "derp-1",
                        "transport": "derp_tcp_tls_443",
                        "address": "47.245.40.231:29120"
                    }]
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
                owner_id: None,
                owner_email: None,
                status: None,
                membership_status: None,
                current_virtual_ip: None,
                virtual_ip: None,
                global_ip: None,
                global_name: None,
                mqtt: None,
            },
        );

        assert_eq!(session.device_id.as_deref(), Some("remote-device"));
        assert_eq!(session.active_network_id.as_deref(), Some("net-1"));
    }
}
