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
        set_control_base_url_override, ControlDevice, ControlPlaneClient, DeviceSessionResponse,
        MqttCredential, RelayCandidate,
    },
    network_module::refresh_network_module_from_session,
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
    pub(crate) virtual_ip: Option<String>,
    #[serde(default)]
    pub(crate) relay_candidates: Vec<PersistedRelayCandidate>,
    pub(crate) mqtt: Option<MqttCredential>,
    pub(crate) expires_in: Option<u64>,
    pub(crate) authenticated_at_ms: u64,
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
            active_network_id: payload.active_network_id,
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
    if session_is_expired(&session) {
        if session.session_kind == "device" {
            if let Ok(renewed) = renew_device_session(session.clone()) {
                return Some(renewed);
            }
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
    let session_key = std::env::var("SLAN_SESSION_KEY")
        .ok()
        .map(|value| value.trim().to_string())
        .filter(|value| !value.is_empty())
        .or_else(|| read_bootstrap_env_value("SLAN_SESSION_KEY"))
        .context("missing SLAN_SESSION_KEY")?;
    let client = ControlPlaneClient::from_env();
    let response = client.bootstrap_device_session(&session_key)?;
    let mut session = persisted_session_from_device_session(response);
    refresh_session_network_from_device_configs(&client, &mut session);
    ensure_session_node_and_control_session(&client, &mut session)?;
    persist_session(&session)?;
    Ok(session)
}

fn renew_device_session(session: PersistedSession) -> Result<PersistedSession> {
    let client = ControlPlaneClient::from_env();
    let network_enabled = session
        .virtual_ip
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .is_some();
    let response = client.renew_device_session(&session.access_token, network_enabled, 0, 0)?;
    let mut renewed = persisted_session_from_device_session(response);
    renewed.user_label = default_string(&session.user_label, &renewed.user_label);
    refresh_session_network_from_device_configs(&client, &mut renewed);
    ensure_session_node_and_control_session(&client, &mut renewed)?;
    persist_session(&renewed)?;
    Ok(renewed)
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
    renew_user_session_if_needed(&client, &mut session)?;
    let device = client.ensure_device_for_user(
        &session.access_token,
        &session.user_id,
        session.device_id.as_deref(),
    )?;
    sync_session_device_fields(&mut session, &device);
    ensure_bound_device_session(&client, &mut session)?;
    refresh_session_network_from_device_configs(&client, &mut session);
    ensure_session_node_and_control_session(&client, &mut session)?;
    if session.virtual_ip.is_none() {
        if let Some(virtual_ip) = device
            .current_virtual_ip
            .or(device.virtual_ip)
            .or(device.global_ip)
            .filter(|value| !value.trim().is_empty())
        {
            session.virtual_ip = Some(virtual_ip);
        }
    }
    persist_session(&session)?;
    Ok(session)
}

pub(crate) fn hydrate_session_from_control_plane(payload: AuthPayload) -> Result<PersistedSession> {
    let client = ControlPlaneClient::from_env();
    let mut session = PersistedSession::from(payload);
    bind_session_device_session(&client, &mut session)?;
    refresh_session_network_from_device_configs(&client, &mut session);
    ensure_session_node_and_control_session(&client, &mut session)?;
    let device = client.ensure_device_for_user(
        &session.access_token,
        &session.user_id,
        session.device_id.as_deref(),
    )?;
    sync_session_device_fields(&mut session, &device);
    if let Some(virtual_ip) = device
        .current_virtual_ip
        .or(device.global_ip)
        .filter(|value| !value.trim().is_empty())
    {
        session.virtual_ip = Some(virtual_ip);
        return Ok(session);
    }

    let devices = client.list_devices_for_user(&session.access_token, &session.user_id)?;
    if let Some(current) = devices
        .into_iter()
        .find(|item| Some(item.device_id.as_str()) == session.device_id.as_deref())
    {
        if let Some(virtual_ip) = current
            .current_virtual_ip
            .or(current.virtual_ip)
            .or(current.global_ip)
            .filter(|value| !value.trim().is_empty())
        {
            session.virtual_ip = Some(virtual_ip);
        }
    }
    Ok(session)
}

fn renew_user_session_if_needed(
    client: &ControlPlaneClient,
    session: &mut PersistedSession,
) -> Result<()> {
    if !user_session_should_renew(session) {
        return Ok(());
    }
    let payload = client.renew_user_session(
        &session.access_token,
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
    Ok(())
}

fn ensure_bound_device_session(
    client: &ControlPlaneClient,
    session: &mut PersistedSession,
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
    if device_session_should_renew(session) {
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
    let response = client.renew_device_session(&device_token, network_enabled, 0, 0)?;
    let relay_candidates = relay_candidates_from_device_session_response(
        &response,
        session.active_network_id.as_deref(),
    );
    let mqtt = mqtt_from_device_session_response(&response);
    session.device_session_id = Some(response.device_session.session_id);
    session.device_token = Some(response.device_session.device_token);
    session.device_refresh_token = response.device_session.device_refresh_token;
    session.device_token_expires_at = Some(response.device_session.device_token_expires_at);
    session.mqtt = mqtt.or(session.mqtt.take());
    if !relay_candidates.is_empty() {
        session.relay_candidates = relay_candidates.clone();
        replace_runtime_relay_candidates(relay_candidates);
    }
    sync_session_device_fields(session, &response.device);
    if let Some(configs) = response.network_configs {
        if let Some(config) = configs.items.last() {
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
    session.mqtt = mqtt.or(session.mqtt.take());
    if !relay_candidates.is_empty() {
        session.relay_candidates = relay_candidates.clone();
        replace_runtime_relay_candidates(relay_candidates);
    }
    sync_session_device_fields(session, &response.device);
    if let Some(configs) = response.network_configs {
        if let Some(config) = configs.items.last() {
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
            })
        })
        .collect()
}

fn refresh_session_network_from_device_configs(
    client: &ControlPlaneClient,
    session: &mut PersistedSession,
) {
    if let Ok(configs) = refresh_network_module_from_session(client, session) {
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
        if let Ok(Some(network_id)) = client.active_network_id(&session.access_token) {
            session.active_network_id = Some(network_id);
        }
    }
}

pub(crate) fn ensure_session_node_and_control_session(
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
    let Some(network_id) = session
        .active_network_id
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .map(str::to_string)
    else {
        return Ok(());
    };
    let node_id = session
        .self_node_id
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .map(str::to_string)
        .unwrap_or_else(|| format!("node-{device_id}"));
    let Ok(node) = client.register_node(&session.access_token, &device_id, &node_id) else {
        session.self_node_id = Some(node_id);
        return Ok(());
    };
    let node_id = node.node_id.trim();
    if node_id.is_empty() {
        return Ok(());
    }
    session.self_node_id = Some(node_id.to_string());
    let _ = client.create_control_session(&session.access_token, node_id, &network_id);
    Ok(())
}

pub(crate) fn report_runtime_state(state: &ClientViewState) {
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
        &session.user_id,
        device_id,
        network_id,
        state.network_enabled,
        state.virtual_ip.as_deref(),
    );
}

pub(crate) fn sync_session_device_fields(session: &mut PersistedSession, device: &ControlDevice) {
    session.device_id = Some(device.device_id.clone());
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
    if device.mqtt.is_some() {
        session.mqtt = device.mqtt.clone();
    }
}

fn session_file_path() -> PathBuf {
    let base = app_data_dir();
    base.join("SLAN").join("client-v2-session.json")
}

fn bootstrap_env_path() -> PathBuf {
    let base = app_data_dir();
    base.join("SLAN").join("bootstrap.env")
}

fn read_bootstrap_env_value(key: &str) -> Option<String> {
    let payload = fs::read_to_string(bootstrap_env_path()).ok()?;
    payload.lines().find_map(|line| {
        let (name, value) = line.split_once('=')?;
        if name.trim() == key {
            let value = value.trim().trim_matches('"').trim_matches('\'');
            if !value.is_empty() {
                return Some(value.to_string());
            }
        }
        None
    })
}

fn default_string(value: &str, fallback: &str) -> String {
    let value = value.trim();
    if value.is_empty() {
        fallback.to_string()
    } else {
        value.to_string()
    }
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
    let path = session_file_path();
    let payload = fs::read(&path).with_context(|| format!("read {}", path.display()))?;
    let mut session: PersistedSession =
        serde_json::from_slice(&payload).with_context(|| format!("decode {}", path.display()))?;
    session.relay_candidates.clear();
    Ok(session)
}

pub(crate) fn persist_session(session: &PersistedSession) -> Result<()> {
    let path = session_file_path();
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent).with_context(|| format!("create {}", parent.display()))?;
    }
    let mut session = session.clone();
    session.relay_candidates.clear();
    let payload = serde_json::to_vec_pretty(&session).context("encode client session")?;
    fs::write(&path, payload).with_context(|| format!("write {}", path.display()))
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

#[cfg(test)]
mod tests {
    use super::*;

    fn test_mqtt(expires_at: Option<i64>) -> MqttCredential {
        MqttCredential {
            broker_url: "mqtt://127.0.0.1:1883".to_string(),
            client_id: "client-1".to_string(),
            username: "user".to_string(),
            password: "pass".to_string(),
            topic_prefix: "slan/v1/devices/device-1".to_string(),
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
                "topicPrefix": "slan/v1/devices/device-1",
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
        assert_eq!(session.active_network_id.as_deref(), Some("net-1"));
        assert_eq!(session.virtual_ip.as_deref(), Some("10.0.0.2"));
        assert_eq!(session.relay_candidates.len(), 1);
        assert_eq!(session.relay_candidates[0].endpoint_id, "derp-1");
        assert_eq!(session.relay_candidates[0].address, "47.245.40.231:29120");
    }
}
