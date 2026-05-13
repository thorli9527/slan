use std::{
    fs,
    path::PathBuf,
    time::{SystemTime, UNIX_EPOCH},
};

use anyhow::{Context, Result};
use client_core::{AuthPayload, ClientCommand, ClientRuntime, ClientViewState};
use serde::{Deserialize, Serialize};

use crate::{
    control_plane::{ControlDevice, ControlPlaneClient, MqttCredential},
    network_module::refresh_network_module_from_session,
    relay_models::PersistedRelayCandidate,
};

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct PersistedSession {
    pub(crate) access_token: String,
    pub(crate) refresh_token: Option<String>,
    pub(crate) user_id: String,
    pub(crate) user_label: String,
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
}

pub(crate) fn load_valid_registered_session() -> Option<PersistedSession> {
    let Ok(session) = load_session() else {
        return None;
    };
    if session.access_token.trim().is_empty() {
        let _ = remove_session();
        return None;
    }
    if session_is_expired(&session) {
        let _ = remove_session();
        return None;
    }
    match ensure_session_device_registered(session.clone()) {
        Ok(session) => Some(session),
        Err(error) if session_auth_invalid_error(&error) => {
            eprintln!("client-core-service session invalid; clearing local session: {error:#}");
            let _ = remove_session();
            None
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
    let _ = runtime.dispatch(ClientCommand::ApplyAuthCallback(session.into()));
}

pub(crate) fn session_is_expired(session: &PersistedSession) -> bool {
    let Some(expires_in) = session.expires_in else {
        return false;
    };
    let lifetime_ms = expires_in.saturating_mul(1_000);
    let expires_at_ms = session.authenticated_at_ms.saturating_add(lifetime_ms);
    current_timestamp_ms().saturating_add(30_000) >= expires_at_ms
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
    let device = client.ensure_device_for_user(
        &session.access_token,
        &session.user_id,
        session.device_id.as_deref(),
    )?;
    sync_session_device_fields(&mut session, &device);
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
    let device = client.ensure_device_for_user(
        &session.access_token,
        &session.user_id,
        session.device_id.as_deref(),
    )?;
    sync_session_device_fields(&mut session, &device);
    refresh_session_network_from_device_configs(&client, &mut session);
    ensure_session_node_and_control_session(&client, &mut session)?;
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
    serde_json::from_slice(&payload).with_context(|| format!("decode {}", path.display()))
}

pub(crate) fn persist_session(session: &PersistedSession) -> Result<()> {
    let path = session_file_path();
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent).with_context(|| format!("create {}", parent.display()))?;
    }
    let payload = serde_json::to_vec_pretty(session).context("encode client session")?;
    fs::write(&path, payload).with_context(|| format!("write {}", path.display()))
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
