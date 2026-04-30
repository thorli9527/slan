use std::collections::HashMap;
use std::fs::{self, OpenOptions};
use std::io::{self, BufRead, BufReader, Write};
use std::net::{Ipv4Addr, SocketAddr, TcpListener, TcpStream, UdpSocket};
use std::path::{Path, PathBuf};
#[cfg(target_os = "windows")]
use std::process::Command;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Mutex, OnceLock};
use std::thread::{self, JoinHandle};
use std::time::{Duration, SystemTime, UNIX_EPOCH};

use controller_client::{HttpControllerClient, TcpJsonHttpTransport};
use ffi_bridge::{DefaultAppCoreFacade, FileTunnelKeyProvider, JsonAppCoreFacade};
use p2p::SocketP2PConnector;
use relay_client::{InMemoryDerpPool, InMemoryPathManager, SocketRelayClient};
use serde::{Deserialize, Serialize};
use serde_json::{json, Value};
use slan_app_core::{TunnelTransport, WireGuardInterfaceConfig, WireGuardPeerConfig};
#[cfg(not(target_os = "windows"))]
use tunnel::InMemoryTunnelBackend;
#[cfg(target_os = "linux")]
use tunnel::LinuxKernelWireGuardBackend;
#[cfg(target_os = "windows")]
use tunnel::WindowsEmbeddableServiceBackend;
use tunnel::{TunnelBackend, TunnelConfig, TunnelManager};

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct PersistedHelperState {
    control_base_url: String,
    snapshot: ffi_bridge::AppCoreSnapshot,
}

fn main() {
    if let Err(err) = run() {
        write_helper_log(&format!("fatal error: {err}"));
        let _ = writeln!(io::stderr(), "{err}");
        std::process::exit(1);
    }
}

fn run() -> Result<(), String> {
    let mut command_args = std::env::args().skip(1);
    let tcp_host = match command_args.next().as_deref() {
        Some("--tcp-host") => Some(
            command_args
                .next()
                .ok_or_else(|| "missing listen address after --tcp-host".to_string())?,
        ),
        Some(other) => {
            return Err(format!(
                "unsupported app-core-helper argument '{other}'; expected --tcp-host <host:port>"
            ))
        }
        None => None,
    };
    let base_url = resolve_helper_control_base_url()?;
    write_helper_log(&format!(
        "helper start tcp_host={} control_base_url={}",
        tcp_host.as_deref().unwrap_or("<stdio>"),
        base_url
    ));
    let derp_pool = Arc::new(InMemoryDerpPool::default());
    let relay_client = Arc::new(SocketRelayClient::default());
    let p2p_connector = Arc::new(SocketP2PConnector::default());
    let tunnel_host = Arc::new(HelperTunnelHost::new());
    let key_provider: Box<dyn ffi_bridge::TunnelKeyProvider> =
        match std::env::var("SLAN_APP_CORE_TUNNEL_KEY_FILE") {
            Ok(path) => Box::new(FileTunnelKeyProvider::new(PathBuf::from(path))),
            Err(_) => Box::new(ffi_bridge::InMemoryTunnelKeyProvider),
        };
    let app_facade = DefaultAppCoreFacade::new_with_tunnel_key_provider(
        HttpControllerClient::new(base_url.clone(), TcpJsonHttpTransport::default()),
        p2p_connector.clone(),
        relay_client.clone(),
        derp_pool.clone(),
        InMemoryPathManager::new(derp_pool, relay_client, p2p_connector),
        tunnel_host.clone(),
        key_provider,
    );
    let mut restored_network_id: Option<String> = None;
    let mut restored_runtime_enabled = false;
    if let Some(snapshot) = load_persisted_helper_state()
        .filter(|state| state.control_base_url.trim() == base_url.trim())
        .map(|state| state.snapshot)
    {
        restored_network_id = snapshot.current_network_id.clone();
        restored_runtime_enabled =
            snapshot.tunnel_runtime.is_some() || snapshot.tunnel_peer_virtual_ip.is_some();
        match app_facade.restore_snapshot(snapshot) {
            Ok(()) => write_helper_log("persisted app-core snapshot restored"),
            Err(err) => write_helper_log(&format!(
                "restore persisted app-core snapshot failed: {err}"
            )),
        }
    }
    let facade = Arc::new(JsonAppCoreFacade::new(app_facade));
    if let Err(err) = refresh_persisted_session(facade.as_ref()) {
        write_helper_log(&format!("startup refreshSession skipped: {err}"));
    }
    if let Err(err) = facade.invoke("controlSync", json!({})) {
        write_helper_log(&format!("startup controlSync skipped: {err}"));
    } else {
        let _ = persist_helper_state(&base_url, facade.as_ref());
        write_helper_log("startup controlSync completed");
    }
    if restored_runtime_enabled {
        if let Some(network_id) = restored_network_id
            .as_deref()
            .filter(|value| !value.is_empty())
        {
            match facade.invoke("enableLocalNetwork", json!({ "networkId": network_id })) {
                Ok(_) => {
                    let _ = persist_helper_state(&base_url, facade.as_ref());
                    write_helper_log(&format!(
                        "startup local network restored network_id={network_id}"
                    ));
                }
                Err(err) => write_helper_log(&format!(
                    "startup local network restore skipped network_id={network_id} error={err}"
                )),
            }
        }
    }

    let control_base_url = Arc::new(base_url);
    if let Some(address) = tcp_host {
        return run_tcp_host(&address, facade, tunnel_host, control_base_url);
    }

    let stdin = io::stdin();
    let mut stdout = io::stdout().lock();
    for line in stdin.lock().lines() {
        let line = line.map_err(|err| err.to_string())?;
        if line.trim().is_empty() {
            continue;
        }
        let outcome = handle_rpc_line(
            &line,
            facade.as_ref(),
            tunnel_host.as_ref(),
            control_base_url.as_str(),
        )?;
        if outcome.response.ok && outcome.should_persist_state {
            let _ = persist_helper_state(control_base_url.as_str(), facade.as_ref());
        }
        serde_json::to_writer(&mut stdout, &outcome.response).map_err(|err| err.to_string())?;
        stdout.write_all(b"\n").map_err(|err| err.to_string())?;
        stdout.flush().map_err(|err| err.to_string())?;
    }
    Ok(())
}

fn run_tcp_host<F>(
    address: &str,
    facade: Arc<JsonAppCoreFacade<F>>,
    tunnel_host: Arc<HelperTunnelHost>,
    control_base_url: Arc<String>,
) -> Result<(), String>
where
    F: ffi_bridge::AppCoreFacade + 'static,
{
    let listener = TcpListener::bind(address)
        .map_err(|err| format!("failed to bind app-core-helper tcp host on {address}: {err}"))?;
    write_helper_log(&format!("tcp host listening on {address}"));
    for stream in listener.incoming() {
        let stream = stream.map_err(|err| format!("tcp host accept failed: {err}"))?;
        write_helper_log("accepted tcp client connection");
        let facade = facade.clone();
        let tunnel_host = tunnel_host.clone();
        let control_base_url = control_base_url.clone();
        thread::spawn(move || {
            if let Err(err) = handle_tcp_client(
                stream,
                facade.as_ref(),
                tunnel_host.as_ref(),
                control_base_url.as_str(),
            ) {
                write_helper_log(&format!("tcp client handler failed: {err}"));
            }
        });
    }
    Ok(())
}

fn handle_tcp_client<F>(
    stream: TcpStream,
    facade: &JsonAppCoreFacade<F>,
    tunnel_host: &HelperTunnelHost,
    control_base_url: &str,
) -> Result<(), String>
where
    F: ffi_bridge::AppCoreFacade,
{
    let reader_stream = stream
        .try_clone()
        .map_err(|err| format!("failed to clone tcp host stream: {err}"))?;
    let mut reader = BufReader::new(reader_stream);
    let mut writer = stream;
    loop {
        let mut line = String::new();
        let bytes_read = reader
            .read_line(&mut line)
            .map_err(|err| format!("tcp host read failed: {err}"))?;
        if bytes_read == 0 {
            return Ok(());
        }
        if line.trim().is_empty() {
            continue;
        }
        let outcome = handle_rpc_line(&line, facade, tunnel_host, control_base_url)?;
        if outcome.response.ok && outcome.should_persist_state {
            let _ = persist_helper_state(control_base_url, facade);
        }
        serde_json::to_writer(&mut writer, &outcome.response).map_err(|err| err.to_string())?;
        writer.write_all(b"\n").map_err(|err| err.to_string())?;
        writer.flush().map_err(|err| err.to_string())?;
    }
}

fn handle_rpc_line<F>(
    line: &str,
    facade: &JsonAppCoreFacade<F>,
    tunnel_host: &HelperTunnelHost,
    control_base_url: &str,
) -> Result<RpcOutcome, String>
where
    F: ffi_bridge::AppCoreFacade,
{
    let request: RpcRequest = serde_json::from_str(line).map_err(|err| err.to_string())?;
    let should_persist_state = should_persist_after_rpc(&request.method);
    Ok(RpcOutcome {
        response: handle_rpc_request(request, facade, tunnel_host, control_base_url),
        should_persist_state,
    })
}

#[derive(Debug)]
struct RpcOutcome {
    response: RpcResponse,
    should_persist_state: bool,
}

fn should_persist_after_rpc(method: &str) -> bool {
    !matches!(
        method,
        "helperStatus"
            | "platformDoctor"
            | "platformInstallPlan"
            | "tunnelRuntimeView"
            | "controlStatus"
    )
}

fn handle_rpc_request<F>(
    request: RpcRequest,
    facade: &JsonAppCoreFacade<F>,
    tunnel_host: &HelperTunnelHost,
    control_base_url: &str,
) -> RpcResponse
where
    F: ffi_bridge::AppCoreFacade,
{
    let method_name = request.method.clone();
    let result = maybe_test_override_result(&method_name).unwrap_or_else(|| {
        invoke_helper(
            &method_name,
            request.args,
            facade,
            tunnel_host,
            control_base_url,
        )
    });
    match &result {
        Ok(_) => write_helper_log(&format!("method {method_name} completed successfully")),
        Err(error) => write_helper_log(&format!("method {method_name} failed error={error}")),
    }
    build_rpc_response(&method_name, result)
}

fn invoke_helper<F>(
    method: &str,
    args: Value,
    facade: &JsonAppCoreFacade<F>,
    tunnel_host: &HelperTunnelHost,
    control_base_url: &str,
) -> Result<Value, String>
where
    F: ffi_bridge::AppCoreFacade,
{
    if method == "helperStatus" {
        helper_status_json(control_base_url, facade, tunnel_host)
    } else if HelperTunnelHost::supports_method(method) {
        write_helper_log(&format!("dispatching tunnel method {method}"));
        tunnel_host.invoke(method, args)
    } else {
        facade.invoke(method, args)
    }
}

fn helper_status_json<F>(
    control_base_url: &str,
    facade: &JsonAppCoreFacade<F>,
    tunnel_host: &HelperTunnelHost,
) -> Result<Value, String>
where
    F: ffi_bridge::AppCoreFacade,
{
    let snapshot = facade.snapshot()?;
    let session_present = snapshot.session.is_some();
    let refresh_token_present = snapshot
        .session
        .as_ref()
        .and_then(|session| session.refresh_token.as_deref())
        .map(|token| !token.trim().is_empty())
        .unwrap_or(false);
    let tunnel_state = tunnel_host
        .state
        .lock()
        .map_err(|_| "app_core_tunnel_backend_error: tunnel helper state poisoned".to_string())?
        .clone();
    let persisted_control_base_url = load_persisted_helper_state()
        .map(|state| state.control_base_url.trim().to_string())
        .filter(|value| !value.is_empty());
    Ok(json!({
        "source": "app-core-helper",
        "helperReachable": true,
        "configuredControlBaseUrl": control_base_url.trim(),
        "persistedControlBaseUrl": persisted_control_base_url,
        "stateFile": persisted_helper_state_path().display().to_string(),
        "sessionPresent": session_present,
        "refreshTokenPresent": refresh_token_present,
        "deviceId": snapshot.current_device.as_ref().map(|device| device.device_id.clone()),
        "nodeId": snapshot.current_node.as_ref().map(|node| node.node_id.clone()),
        "currentNetworkId": snapshot.current_network_id,
        "bootstrapPresent": snapshot.current_bootstrap.is_some(),
        "networkMapPresent": snapshot
            .current_bootstrap
            .as_ref()
            .and_then(|bootstrap| bootstrap.network_map.as_ref())
            .is_some(),
        "connectionState": snapshot.connection_state,
        "tunnelRuntimePresent": snapshot.tunnel_runtime.is_some(),
        "tunnelPeerVirtualIp": snapshot.tunnel_peer_virtual_ip,
        "tunnelBackendRunning": tunnel_state.is_running,
        "tunnelLastError": tunnel_state.last_error,
        "tunnelLastAppliedAtMs": tunnel_state.last_applied_at_ms,
        "tunnelLastStartedAtMs": tunnel_state.last_started_at_ms,
        "checkedAtMs": current_timestamp_ms(),
    }))
}

fn resolve_helper_control_base_url() -> Result<String, String> {
    if let Ok(base_url) = std::env::var("SLAN_CONTROL_BASE_URL") {
        let trimmed = base_url.trim();
        if !trimmed.is_empty() {
            return Ok(trimmed.to_string());
        }
    }
    if let Some(state) = load_persisted_helper_state() {
        let trimmed = state.control_base_url.trim();
        if !trimmed.is_empty() {
            return Ok(trimmed.to_string());
        }
    }
    Err("missing SLAN_CONTROL_BASE_URL for app-core-helper".to_string())
}

fn refresh_persisted_session<F>(facade: &JsonAppCoreFacade<F>) -> Result<(), String>
where
    F: ffi_bridge::AppCoreFacade,
{
    let _guard = persisted_helper_state_file_lock()
        .lock()
        .map_err(|_| "persisted app-core state lock poisoned".to_string())?;
    let snapshot = facade.snapshot()?;
    let Some(session) = snapshot.session else {
        return Err("missing persisted session".to_string());
    };
    let Some(refresh_token) = session.refresh_token.as_deref() else {
        return Err("missing refresh token".to_string());
    };
    if refresh_token.trim().is_empty() {
        return Err("missing refresh token".to_string());
    }
    facade.invoke(
        "refreshSession",
        json!({
            "refreshToken": refresh_token,
            "deviceId": session.device_id,
        }),
    )?;
    write_helper_log("persisted session refreshed on startup");
    Ok(())
}

fn persist_helper_state<F>(
    control_base_url: &str,
    facade: &JsonAppCoreFacade<F>,
) -> Result<(), String>
where
    F: ffi_bridge::AppCoreFacade,
{
    let _guard = persisted_helper_state_file_lock()
        .lock()
        .map_err(|_| "persisted app-core state lock poisoned".to_string())?;
    let state = PersistedHelperState {
        control_base_url: control_base_url.trim().to_string(),
        snapshot: facade.snapshot()?,
    };
    let path = persisted_helper_state_path();
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent)
            .map_err(|err| format!("create persisted app-core state dir failed: {err}"))?;
    }
    let payload = serde_json::to_vec_pretty(&state)
        .map_err(|err| format!("encode persisted app-core state failed: {err}"))?;
    let temp_path = path.with_extension("json.tmp");
    fs::write(&temp_path, payload)
        .map_err(|err| format!("write persisted app-core state temp file failed: {err}"))?;
    fs::rename(&temp_path, &path)
        .map_err(|err| format!("replace persisted app-core state failed: {err}"))?;
    Ok(())
}

fn persisted_helper_state_file_lock() -> &'static Mutex<()> {
    static LOCK: OnceLock<Mutex<()>> = OnceLock::new();
    LOCK.get_or_init(|| Mutex::new(()))
}

fn load_persisted_helper_state() -> Option<PersistedHelperState> {
    let path = persisted_helper_state_path();
    let payload = fs::read(&path).ok()?;
    serde_json::from_slice(&payload)
        .map_err(|err| {
            write_helper_log(&format!(
                "decode persisted app-core state failed path={} error={err}",
                path.display()
            ));
            err
        })
        .ok()
}

fn persisted_helper_state_path() -> PathBuf {
    if let Ok(configured) = std::env::var("SLAN_APP_CORE_STATE_FILE") {
        let trimmed = configured.trim();
        if !trimmed.is_empty() {
            return PathBuf::from(trimmed);
        }
    }
    #[cfg(target_os = "windows")]
    if let Ok(program_data) = std::env::var("ProgramData") {
        let trimmed = program_data.trim();
        if !trimmed.is_empty() {
            return PathBuf::from(trimmed)
                .join("SLAN")
                .join("app-core-state.json");
        }
    }
    #[cfg(target_os = "windows")]
    return PathBuf::from(r"C:\ProgramData\SLAN\app-core-state.json");
    #[cfg(not(target_os = "windows"))]
    {
        if let Ok(current_exe) = std::env::current_exe() {
            if let Some(parent) = current_exe.parent() {
                return parent.join("app-core-state.json");
            }
        }
        std::env::temp_dir().join("slan-app-core-state.json")
    }
}

fn write_helper_log(message: &str) {
    let timestamp = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|duration| duration.as_secs())
        .unwrap_or_default();
    let log_path = helper_log_path();
    if let Some(parent) = log_path.parent() {
        let _ = std::fs::create_dir_all(parent);
    }
    if let Ok(mut file) = OpenOptions::new().create(true).append(true).open(&log_path) {
        let _ = writeln!(file, "[{timestamp}] {message}");
    }
}

fn helper_log_path() -> PathBuf {
    if let Ok(configured) = std::env::var("SLAN_APP_CORE_HELPER_LOG") {
        let trimmed = configured.trim();
        if !trimmed.is_empty() {
            return PathBuf::from(trimmed);
        }
    }
    if let Ok(current_exe) = std::env::current_exe() {
        if let Some(parent) = current_exe.parent() {
            return parent.join("app-core-helper.log");
        }
    }
    std::env::temp_dir().join("app-core-helper.log")
}

#[derive(Debug, Deserialize)]
struct RpcRequest {
    method: String,
    #[serde(default)]
    args: Value,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
struct RpcResponse {
    ok: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    result: Option<Value>,
    #[serde(skip_serializing_if = "Option::is_none")]
    error: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    error_code: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    error_message: Option<String>,
}

fn build_rpc_response(method: &str, result: Result<Value, String>) -> RpcResponse {
    match result {
        Ok(result) => RpcResponse {
            ok: true,
            result: Some(result),
            error: None,
            error_code: None,
            error_message: None,
        },
        Err(error) => {
            let (error_code, error_message) = classify_helper_error(method, &error);
            RpcResponse {
                ok: false,
                result: None,
                error: Some(error.clone()),
                error_code: Some(error_code),
                error_message: Some(error_message),
            }
        }
    }
}

fn maybe_test_override_result(method: &str) -> Option<Result<Value, String>> {
    let (result_key, error_key) = match method {
        "send" => (
            "SLAN_APP_CORE_HELPER_TEST_SEND_RESULT",
            "SLAN_APP_CORE_HELPER_TEST_SEND_ERROR",
        ),
        "probe" => (
            "SLAN_APP_CORE_HELPER_TEST_PROBE_RESULT",
            "SLAN_APP_CORE_HELPER_TEST_PROBE_ERROR",
        ),
        _ => return None,
    };
    if let Ok(result) = std::env::var(result_key) {
        let parsed = serde_json::from_str(&result)
            .map_err(|err| format!("invalid helper test result json in {result_key}: {err}"));
        return Some(parsed);
    }
    std::env::var(error_key).ok().map(Err)
}

fn classify_helper_error(method: &str, error: &str) -> (String, String) {
    if let Some((code, message)) = classify_structured_error(error) {
        return (code, message);
    }
    if method == "send" {
        return ("send_failed".to_string(), error.to_string());
    }
    if method == "probe" {
        return ("probe_failed".to_string(), error.to_string());
    }
    ("app_core_helper_error".to_string(), error.to_string())
}

fn classify_structured_error(error: &str) -> Option<(String, String)> {
    let (code, message) = error.split_once(": ")?;
    if code.starts_with("probe_") || code.starts_with("send_") || code.starts_with("app_core_") {
        return Some((code.to_string(), message.to_string()));
    }
    None
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct HelperTunnelConfiguration {
    transport: TunnelTransport,
    local_virtual_ip: String,
    peer_virtual_ip: String,
    #[serde(default)]
    debug_engine_mode: Option<String>,
    wireguard_interface: WireGuardInterfaceConfig,
    wireguard_peer: WireGuardPeerConfig,
}

impl HelperTunnelConfiguration {
    fn into_tunnel_config(self) -> TunnelConfig {
        TunnelConfig {
            transport: self.transport,
            local_virtual_ip: self.local_virtual_ip,
            peer_virtual_ip: self.peer_virtual_ip,
            wireguard_interface: self.wireguard_interface,
            wireguard_peer: self.wireguard_peer,
        }
    }

    fn from_tunnel_config(config: &TunnelConfig) -> Self {
        Self {
            transport: config.transport.clone(),
            local_virtual_ip: config.local_virtual_ip.clone(),
            peer_virtual_ip: config.peer_virtual_ip.clone(),
            debug_engine_mode: None,
            wireguard_interface: config.wireguard_interface.clone(),
            wireguard_peer: config.wireguard_peer.clone(),
        }
    }
}

#[derive(Debug, Default, Clone)]
struct HelperTunnelState {
    configuration: Option<HelperTunnelConfiguration>,
    last_error: Option<String>,
    last_applied_at_ms: Option<u64>,
    last_started_at_ms: Option<u64>,
    is_running: bool,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
struct BindAdapterIpArgs {
    interface_name: Option<String>,
    ip: String,
    #[serde(default = "default_bind_prefix_len")]
    prefix_len: u8,
}

fn default_bind_prefix_len() -> u8 {
    32
}

struct HelperTunnelHost {
    backend: Box<dyn TunnelBackend>,
    state: Mutex<HelperTunnelState>,
    dns_server: Mutex<Option<HelperLocalDnsServer>>,
}

impl HelperTunnelHost {
    fn new() -> Self {
        Self {
            backend: default_tunnel_backend(),
            state: Mutex::new(HelperTunnelState::default()),
            dns_server: Mutex::new(None),
        }
    }

    fn supports_method(method: &str) -> bool {
        matches!(
            method,
            "applyTunnelConfiguration"
                | "bindAdapterIp"
                | "removeTunnelPeer"
                | "bringTunnelUp"
                | "bringTunnelDown"
                | "tunnelRuntimeView"
                | "platformDoctor"
                | "platformInstallPlan"
        )
    }

    fn invoke(&self, method: &str, args: Value) -> Result<Value, String> {
        match method {
            "applyTunnelConfiguration" => self.apply_configuration(args),
            "bindAdapterIp" => self.bind_adapter_ip(args),
            "removeTunnelPeer" => self.remove_peer(args),
            "bringTunnelUp" => self.bring_up(),
            "bringTunnelDown" => self.bring_down(),
            "tunnelRuntimeView" => self.runtime_view(args),
            "platformDoctor" => Ok(platform_doctor_json(&self.backend.diagnostics())),
            "platformInstallPlan" => Ok(platform_install_plan_json()),
            _ => Err(format!("unsupported method: {method}")),
        }
    }

    fn apply_configuration(&self, args: Value) -> Result<Value, String> {
        let configuration: HelperTunnelConfiguration = serde_json::from_value(args)
            .map_err(|err| format!("app_core_invalid_tunnel_config: {err}"))?;
        write_helper_log(&format!(
            "applyTunnelConfiguration local_virtual_ip={} peer_virtual_ip={} interface_name={:?} addresses={:?}",
            configuration.local_virtual_ip,
            configuration.peer_virtual_ip,
            configuration.wireguard_interface.interface_name,
            configuration.wireguard_interface.addresses
        ));
        let tunnel_config = configuration.clone().into_tunnel_config();
        if let Err(err) = self
            .backend
            .apply_interface_config(&tunnel_config.wireguard_interface)
        {
            write_helper_log(&format!(
                "applyTunnelConfiguration apply_interface_config failed error={err}"
            ));
            return Err(format!("app_core_tunnel_backend_error: {err}"));
        }
        if let Err(err) = self.backend.apply_peer_config(
            &tunnel_config.peer_virtual_ip,
            &tunnel_config.wireguard_peer,
        ) {
            write_helper_log(&format!(
                "applyTunnelConfiguration apply_peer_config failed error={err}"
            ));
            return Err(format!("app_core_tunnel_backend_error: {err}"));
        }
        let mut state = self.state.lock().map_err(|_| {
            "app_core_tunnel_backend_error: tunnel helper state poisoned".to_string()
        })?;
        state.configuration = Some(configuration.clone());
        state.last_error = None;
        state.last_applied_at_ms = Some(current_timestamp_ms());
        write_helper_log(&format!(
            "applyTunnelConfiguration accepted peer_virtual_ip={}",
            configuration.peer_virtual_ip
        ));
        Ok(self.action_result_json(
            &state,
            "applyTunnelConfiguration",
            true,
            "configured",
            format!(
                "Rust backend accepted tunnel configuration for {}.",
                configuration.peer_virtual_ip
            ),
        ))
    }

    fn bind_adapter_ip(&self, args: Value) -> Result<Value, String> {
        let bind_args: BindAdapterIpArgs = serde_json::from_value(args)
            .map_err(|err| format!("app_core_invalid_tunnel_config: {err}"))?;
        let interface_name = bind_args
            .interface_name
            .clone()
            .filter(|value| !value.trim().is_empty())
            .unwrap_or_else(|| "SLAN LAN Adapter".to_string());
        let cidr = format!("{}/{}", bind_args.ip.trim(), bind_args.prefix_len);
        write_helper_log(&format!(
            "bindAdapterIp requested interface_name={} cidr={}",
            interface_name, cidr
        ));

        let interface = WireGuardInterfaceConfig {
            interface_name: Some(interface_name.clone()),
            key_pair: slan_app_core::WireGuardKeyPair {
                public_key: "diag-public-key".to_string(),
                private_key: "diag-private-key".to_string(),
            },
            listen_port: Some(51820),
            mtu: Some(1280),
            addresses: vec![cidr.clone()],
            dns_servers: vec![],
            peers: vec![],
        };
        self.backend
            .apply_interface_config(&interface)
            .map_err(|err| format!("app_core_tunnel_backend_error: {err}"))?;
        write_helper_log(&format!(
            "bindAdapterIp apply_interface_config accepted interface_name={} cidr={}",
            interface_name, cidr
        ));

        match self.backend.bring_up() {
            Ok(()) => {
                write_helper_log(&format!(
                    "bindAdapterIp succeeded interface_name={} cidr={}",
                    interface_name, cidr
                ));
                Ok(json!({
                    "action": "bindAdapterIp",
                    "accepted": true,
                    "phase": "started",
                    "source": "rust-helper",
                    "detail": format!("Rust backend bound {} to {}.", cidr, interface_name),
                    "interfaceName": interface_name,
                    "localVirtualIp": bind_args.ip,
                    "prefixLen": bind_args.prefix_len,
                    "connectionStatus": "connected",
                }))
            }
            Err(err) => {
                write_helper_log(&format!(
                    "bindAdapterIp failed interface_name={} cidr={} error={}",
                    interface_name, cidr, err
                ));
                Ok(json!({
                    "action": "bindAdapterIp",
                    "accepted": false,
                    "phase": "failed",
                    "source": "rust-helper",
                    "detail": format!("Rust backend failed to bind {} to {}: {}.", cidr, interface_name, err),
                    "interfaceName": interface_name,
                    "localVirtualIp": bind_args.ip,
                    "prefixLen": bind_args.prefix_len,
                    "runtimeLastError": err,
                    "connectionStatus": "disconnected",
                }))
            }
        }
    }

    fn remove_peer(&self, args: Value) -> Result<Value, String> {
        let peer_virtual_ip = args
            .get("peerVirtualIp")
            .and_then(Value::as_str)
            .filter(|value| !value.is_empty())
            .ok_or_else(|| {
                "app_core_invalid_tunnel_config: missing or invalid peerVirtualIp".to_string()
            })?;
        self.backend
            .remove_peer(peer_virtual_ip)
            .map_err(|err| format!("app_core_tunnel_backend_error: {err}"))?;
        let mut state = self.state.lock().map_err(|_| {
            "app_core_tunnel_backend_error: tunnel helper state poisoned".to_string()
        })?;
        if state
            .configuration
            .as_ref()
            .map(|config| config.peer_virtual_ip.as_str())
            == Some(peer_virtual_ip)
        {
            state.configuration = None;
            state.is_running = false;
            state.last_error = None;
        }
        Ok(self.action_result_json(
            &state,
            "removeTunnelPeer",
            true,
            if state.configuration.is_some() {
                "accepted"
            } else {
                "verified"
            },
            format!("Rust backend processed peer removal for {peer_virtual_ip}."),
        ))
    }

    fn bring_up(&self) -> Result<Value, String> {
        let mut state = self.state.lock().map_err(|_| {
            "app_core_tunnel_backend_error: tunnel helper state poisoned".to_string()
        })?;
        if state.configuration.is_none() {
            return Err("app_core_tunnel_backend_error: missing tunnel configuration".to_string());
        }
        let configuration_peer = state
            .configuration
            .as_ref()
            .map(|config| config.peer_virtual_ip.clone())
            .unwrap_or_else(|| "<missing>".to_string());
        write_helper_log(&format!(
            "bringTunnelUp requested peer_virtual_ip={configuration_peer}"
        ));
        match self.backend.bring_up() {
            Ok(()) => {
                state.is_running = true;
                state.last_error = None;
                state.last_started_at_ms = Some(current_timestamp_ms());
                write_helper_log(&format!(
                    "bringTunnelUp succeeded peer_virtual_ip={configuration_peer}"
                ));
                Ok(self.action_result_json(
                    &state,
                    "bringTunnelUp",
                    true,
                    "started",
                    "Rust backend accepted the bring-up request.".to_string(),
                ))
            }
            Err(err) => {
                state.last_error = Some(err.clone());
                write_helper_log(&format!(
                    "bringTunnelUp failed peer_virtual_ip={} error={err}",
                    configuration_peer
                ));
                Ok(self.action_result_json(
                    &state,
                    "bringTunnelUp",
                    false,
                    "failed",
                    format!("Rust backend failed to bring the tunnel up: {err}"),
                ))
            }
        }
    }

    fn bring_down(&self) -> Result<Value, String> {
        let mut state = self.state.lock().map_err(|_| {
            "app_core_tunnel_backend_error: tunnel helper state poisoned".to_string()
        })?;
        if state.configuration.is_none() {
            state.is_running = false;
            return Ok(self.action_result_json(
                &state,
                "bringTunnelDown",
                true,
                "verified",
                "Rust backend confirmed the tunnel is already down.".to_string(),
            ));
        }
        self.backend
            .bring_down()
            .map_err(|err| format!("app_core_tunnel_backend_error: {err}"))?;
        state.is_running = false;
        state.last_error = None;
        Ok(self.action_result_json(
            &state,
            "bringTunnelDown",
            true,
            "verified",
            "Rust backend accepted the bring-down request.".to_string(),
        ))
    }

    fn runtime_view(&self, args: Value) -> Result<Value, String> {
        let peer_virtual_ip = args
            .get("peerVirtualIp")
            .and_then(Value::as_str)
            .filter(|value| !value.is_empty())
            .ok_or_else(|| {
                "app_core_invalid_tunnel_config: missing or invalid peerVirtualIp".to_string()
            })?;
        let state = self.state.lock().map_err(|_| {
            "app_core_tunnel_backend_error: tunnel helper state poisoned".to_string()
        })?;
        let Some(configuration) = state.configuration.as_ref() else {
            return Ok(Value::Null);
        };
        if configuration.peer_virtual_ip != peer_virtual_ip {
            return Ok(Value::Null);
        }
        let stats = self
            .backend
            .runtime_stats(peer_virtual_ip)
            .map_err(|err| format!("app_core_tunnel_backend_error: {err}"))?;
        write_helper_log(&format!(
            "tunnelRuntimeView peer_virtual_ip={} state_running={} stats_present={}",
            peer_virtual_ip,
            state.is_running,
            stats.is_some()
        ));
        Ok(self.runtime_view_json(configuration, &state, stats))
    }

    fn action_result_json(
        &self,
        state: &HelperTunnelState,
        action: &str,
        accepted: bool,
        phase: &str,
        detail: String,
    ) -> Value {
        let configuration = state.configuration.as_ref();
        let diagnostics = self.backend.diagnostics();
        let runtime_state = configuration.map(|_| {
            if state.is_running {
                "configured"
            } else {
                "disconnected"
            }
        });
        let backend_state = if state.last_error.is_some() {
            Some("failed")
        } else if state.is_running {
            Some("started")
        } else if configuration.is_some() {
            Some("idle")
        } else {
            None
        };
        json!({
            "action": action,
            "accepted": accepted,
            "phase": phase,
            "source": "rust-helper",
            "detail": detail,
            "connectionStatus": if state.is_running { "connected" } else { "disconnected" },
            "hasConfiguration": configuration.is_some(),
            "configurationPeerVirtualIp": configuration.map(|config| config.peer_virtual_ip.clone()),
            "runtimeState": runtime_state,
            "backendName": diagnostics.name,
            "backendState": backend_state,
            "backendExecutionMode": diagnostics.execution_mode,
            "backendExecutionBackend": diagnostics.execution_backend,
            "backendInterfaceName": diagnostics.interface_name,
            "backendIsUp": diagnostics.is_up,
            "backendPlannedPeerCount": diagnostics.planned_peer_count,
            "backendRecentCommandCount": diagnostics.recent_command_count,
            "runtimeLastError": state.last_error.clone(),
        })
    }

    fn runtime_view_json(
        &self,
        configuration: &HelperTunnelConfiguration,
        state: &HelperTunnelState,
        stats: Option<slan_app_core::WireGuardRuntimeStats>,
    ) -> Value {
        let selected_endpoint = stats
            .as_ref()
            .and_then(|stats| stats.selected_endpoint.clone())
            .or_else(|| configuration.wireguard_peer.endpoint.clone());
        let backend_state = if state.last_error.is_some() {
            "failed"
        } else if state.is_running {
            "started"
        } else {
            "idle"
        };
        let diagnostics = self.backend.diagnostics();
        let bytes_received = stats
            .as_ref()
            .map(|stats| stats.bytes_received)
            .unwrap_or(0);
        let bytes_sent = stats.as_ref().map(|stats| stats.bytes_sent).unwrap_or(0);
        json!({
            "state": if state.is_running { "configured" } else { "disconnected" },
            "transport": configuration.transport.clone(),
            "debugEngineMode": configuration
                .debug_engine_mode
                .clone()
                .unwrap_or_else(|| "noop".to_string()),
            "backendName": diagnostics.name,
            "backendState": backend_state,
            "backendExecutionMode": diagnostics.execution_mode,
            "backendExecutionBackend": diagnostics.execution_backend,
            "backendInterfaceName": diagnostics.interface_name,
            "backendIsUp": diagnostics.is_up,
            "backendPlannedPeerCount": diagnostics.planned_peer_count,
            "backendRecentCommandCount": diagnostics.recent_command_count,
            "backendLastError": state.last_error.clone(),
            "backendLastStartedAtMs": state.last_started_at_ms,
            "backendPeerVirtualIp": configuration.peer_virtual_ip.clone(),
            "backendSelectedEndpoint": selected_endpoint.clone(),
            "peerVirtualIp": configuration.peer_virtual_ip.clone(),
            "peerPublicKey": configuration.wireguard_peer.public_key.clone(),
            "selectedEndpoint": selected_endpoint,
            "interfaceName": configuration.wireguard_interface.interface_name.clone(),
            "dnsServers": configuration.wireguard_interface.dns_servers.clone(),
            "allowedIps": configuration
                .wireguard_peer
                .allowed_ips
                .iter()
                .map(|allowed_ip| allowed_ip.cidr.clone())
                .collect::<Vec<_>>(),
            "localVirtualIp": configuration.local_virtual_ip.clone(),
            "remoteAddress": configuration.wireguard_peer.endpoint.clone().unwrap_or_default(),
            "mtu": configuration.wireguard_interface.mtu,
            "interfaceAddresses": configuration.wireguard_interface.addresses.clone(),
            "includedRoutes": configuration
                .wireguard_peer
                .allowed_ips
                .iter()
                .map(|allowed_ip| allowed_ip.cidr.clone())
                .collect::<Vec<_>>(),
            "packetRxCount": if state.is_running && bytes_received > 0 { 1 } else { 0 },
            "packetRxBytes": bytes_received,
            "packetTxCount": if state.is_running && bytes_sent > 0 { 1 } else { 0 },
            "packetTxBytes": bytes_sent,
            "lastPacketAtMs": state.last_started_at_ms,
            "lastAppliedAtMs": state.last_applied_at_ms,
            "lastError": state.last_error.clone(),
        })
    }
}

impl TunnelManager for HelperTunnelHost {
    fn establish(&self, config: &TunnelConfig) -> Result<(), String> {
        self.backend.establish(config)?;
        let mut state = self
            .state
            .lock()
            .map_err(|_| "tunnel helper state poisoned".to_string())?;
        state.configuration = Some(HelperTunnelConfiguration::from_tunnel_config(config));
        state.last_error = None;
        state.last_applied_at_ms = Some(current_timestamp_ms());
        state.last_started_at_ms = Some(current_timestamp_ms());
        state.is_running = true;
        Ok(())
    }

    fn close(&self, peer_virtual_ip: &str) -> Result<(), String> {
        self.backend.close(peer_virtual_ip)?;
        let mut state = self
            .state
            .lock()
            .map_err(|_| "tunnel helper state poisoned".to_string())?;
        if state
            .configuration
            .as_ref()
            .map(|config| config.peer_virtual_ip.as_str())
            == Some(peer_virtual_ip)
        {
            state.configuration = None;
            state.is_running = false;
            state.last_error = None;
        }
        Ok(())
    }

    fn start_local_dns(&self, records: Vec<(String, String)>) -> Result<(), String> {
        let records = normalize_dns_records(records);
        let mut dns_server = self
            .dns_server
            .lock()
            .map_err(|_| "local dns state poisoned".to_string())?;
        if let Some(existing) = dns_server.take() {
            existing.stop();
        }
        if records.is_empty() {
            write_helper_log("local dns skipped: no records");
            return Ok(());
        }
        let server = HelperLocalDnsServer::start(records)?;
        *dns_server = Some(server);
        Ok(())
    }

    fn stop_local_dns(&self) -> Result<(), String> {
        let mut dns_server = self
            .dns_server
            .lock()
            .map_err(|_| "local dns state poisoned".to_string())?;
        if let Some(existing) = dns_server.take() {
            existing.stop();
        }
        Ok(())
    }
}

struct HelperLocalDnsServer {
    stop_signal: Arc<AtomicBool>,
    handle: Option<JoinHandle<()>>,
    nrpt_namespaces: Vec<String>,
}

impl HelperLocalDnsServer {
    fn start(records: HashMap<String, String>) -> Result<Self, String> {
        if records.is_empty() {
            return Err("local dns requires at least one valid record".to_string());
        }
        let nrpt_namespaces = nrpt_namespaces_for_records(&records);
        let socket = UdpSocket::bind(("127.0.0.1", 53))
            .map_err(|err| format!("local dns bind 127.0.0.1:53 failed: {err}"))?;
        socket
            .set_read_timeout(Some(Duration::from_millis(250)))
            .map_err(|err| format!("local dns set read timeout failed: {err}"))?;
        configure_windows_nrpt_rules(&nrpt_namespaces)?;
        let stop_signal = Arc::new(AtomicBool::new(false));
        let thread_stop = stop_signal.clone();
        let handle = thread::spawn(move || {
            write_helper_log(&format!("local dns started records={}", records.len()));
            run_local_dns(socket, records, thread_stop);
            write_helper_log("local dns stopped");
        });
        Ok(Self {
            stop_signal,
            handle: Some(handle),
            nrpt_namespaces,
        })
    }

    fn stop(mut self) {
        self.stop_signal.store(true, Ordering::SeqCst);
        let _ = UdpSocket::bind(("127.0.0.1", 0))
            .and_then(|socket| socket.send_to(&[0], ("127.0.0.1", 53)));
        if let Some(handle) = self.handle.take() {
            let _ = handle.join();
        }
        if let Err(err) = remove_windows_nrpt_rules(&self.nrpt_namespaces) {
            write_helper_log(&format!("local dns remove NRPT skipped: {err}"));
        }
    }
}

impl Drop for HelperLocalDnsServer {
    fn drop(&mut self) {
        self.stop_signal.store(true, Ordering::SeqCst);
        if let Err(err) = remove_windows_nrpt_rules(&self.nrpt_namespaces) {
            write_helper_log(&format!("local dns drop remove NRPT skipped: {err}"));
        }
    }
}

fn run_local_dns(
    socket: UdpSocket,
    records: HashMap<String, String>,
    stop_signal: Arc<AtomicBool>,
) {
    let mut buffer = [0_u8; 512];
    while !stop_signal.load(Ordering::SeqCst) {
        match socket.recv_from(&mut buffer) {
            Ok((len, remote)) => {
                if let Some(response) = build_dns_response(&buffer[..len], &records) {
                    let _ = send_dns_response(&socket, &response, remote);
                }
            }
            Err(err)
                if err.kind() == io::ErrorKind::WouldBlock
                    || err.kind() == io::ErrorKind::TimedOut => {}
            Err(err) => {
                write_helper_log(&format!("local dns recv failed: {err}"));
                break;
            }
        }
    }
}

fn send_dns_response(socket: &UdpSocket, response: &[u8], remote: SocketAddr) -> io::Result<usize> {
    socket.send_to(response, remote)
}

fn normalize_dns_records(records: Vec<(String, String)>) -> HashMap<String, String> {
    records
        .into_iter()
        .filter_map(|(name, ip)| normalize_dns_record(&name, &ip))
        .collect()
}

fn normalize_dns_record(name: &str, ip: &str) -> Option<(String, String)> {
    let host = normalize_dns_name(name);
    if !is_allowed_wildcard_host(&host) {
        return None;
    }
    let parsed = ip.trim().parse::<Ipv4Addr>().ok()?;
    Some((host, parsed.to_string()))
}

fn normalize_dns_name(value: &str) -> String {
    value.trim().trim_end_matches('.').to_ascii_lowercase()
}

fn is_allowed_wildcard_host(host: &str) -> bool {
    let labels = host.split('.').collect::<Vec<_>>();
    let wildcard_count = labels.iter().take_while(|label| **label == "*").count();
    if wildcard_count == 0 || labels.len().saturating_sub(wildcard_count) < 2 {
        return false;
    }
    labels[wildcard_count..]
        .iter()
        .all(|label| !label.is_empty() && *label != "*")
}

fn resolve_dns_name(name: &str, records: &HashMap<String, String>) -> Option<String> {
    let normalized = normalize_dns_name(name);
    if let Some(ip) = records.get(&normalized) {
        return Some(ip.clone());
    }
    let mut matched: Option<(&String, usize)> = None;
    for pattern in records.keys() {
        if !pattern.starts_with("*.") && !pattern.starts_with("*.*.") {
            continue;
        }
        if !wildcard_matches(pattern, &normalized) {
            continue;
        }
        let labels = pattern.split('.').count();
        if matched.map(|(_, count)| labels > count).unwrap_or(true) {
            matched = Some((pattern, labels));
        }
    }
    matched.and_then(|(pattern, _)| records.get(pattern).cloned())
}

fn wildcard_matches(pattern: &str, name: &str) -> bool {
    let pattern_labels = pattern.split('.').collect::<Vec<_>>();
    let name_labels = name.split('.').collect::<Vec<_>>();
    pattern_labels.len() == name_labels.len()
        && pattern_labels
            .iter()
            .zip(name_labels.iter())
            .all(|(pattern, name)| *pattern == "*" || pattern == name)
}

fn nrpt_namespaces_for_records(records: &HashMap<String, String>) -> Vec<String> {
    let mut namespaces = records
        .keys()
        .filter_map(|host| nrpt_namespace_for_host(host))
        .collect::<Vec<_>>();
    namespaces.sort();
    namespaces.dedup();
    namespaces
}

fn nrpt_namespace_for_host(host: &str) -> Option<String> {
    let labels = host.split('.').collect::<Vec<_>>();
    let wildcard_count = labels.iter().take_while(|label| **label == "*").count();
    if wildcard_count == 0 || labels.len().saturating_sub(wildcard_count) < 2 {
        return None;
    }
    Some(format!(".{}", labels[wildcard_count..].join(".")))
}

fn configure_windows_nrpt_rules(namespaces: &[String]) -> Result<(), String> {
    if namespaces.is_empty() {
        return Ok(());
    }
    #[cfg(target_os = "windows")]
    {
        remove_windows_nrpt_rules(namespaces)?;
        let namespace_array = powershell_string_array(namespaces);
        let script = format!(
            "$ErrorActionPreference='Stop'\n\
             $namespaces=@({namespace_array})\n\
             foreach ($namespace in $namespaces) {{\n\
               Add-DnsClientNrptRule -Namespace $namespace -NameServers '127.0.0.1' -Comment 'SLAN local DNS' -ErrorAction Stop | Out-Null\n\
             }}\n\
             Clear-DnsClientCache -ErrorAction SilentlyContinue\n"
        );
        run_powershell_script(&script)
    }
    #[cfg(not(target_os = "windows"))]
    {
        let _ = namespaces;
        Ok(())
    }
}

fn remove_windows_nrpt_rules(namespaces: &[String]) -> Result<(), String> {
    if namespaces.is_empty() {
        return Ok(());
    }
    #[cfg(target_os = "windows")]
    {
        let namespace_array = powershell_string_array(namespaces);
        let script = format!(
            "$ErrorActionPreference='Stop'\n\
             $namespaces=@({namespace_array})\n\
             foreach ($namespace in $namespaces) {{\n\
               Get-DnsClientNrptRule -ErrorAction SilentlyContinue |\n\
                 Where-Object {{ $_.Namespace -contains $namespace -and ($_.NameServers -contains '127.0.0.1' -or $_.Comment -eq 'SLAN local DNS') }} |\n\
                 ForEach-Object {{ Remove-DnsClientNrptRule -Name $_.Name -Force -ErrorAction SilentlyContinue | Out-Null }}\n\
             }}\n\
             Clear-DnsClientCache -ErrorAction SilentlyContinue\n"
        );
        run_powershell_script(&script)
    }
    #[cfg(not(target_os = "windows"))]
    {
        let _ = namespaces;
        Ok(())
    }
}

#[cfg(target_os = "windows")]
fn run_powershell_script(script: &str) -> Result<(), String> {
    let output = Command::new("powershell.exe")
        .args([
            "-NoProfile",
            "-NonInteractive",
            "-ExecutionPolicy",
            "Bypass",
            "-Command",
            script,
        ])
        .output()
        .map_err(|err| format!("launch powershell failed: {err}"))?;
    if output.status.success() {
        return Ok(());
    }
    let detail = String::from_utf8_lossy(&output.stderr).trim().to_string();
    Err(if detail.is_empty() {
        format!("powershell exited with status {}", output.status)
    } else {
        detail
    })
}

#[cfg(target_os = "windows")]
fn powershell_string_array(values: &[String]) -> String {
    values
        .iter()
        .map(|value| format!("'{}'", value.replace('\'', "''")))
        .collect::<Vec<_>>()
        .join(",")
}

struct DnsQuestion {
    name: String,
    qtype: u16,
    end_offset: usize,
}

fn build_dns_response(request: &[u8], records: &HashMap<String, String>) -> Option<Vec<u8>> {
    if request.len() < 12 || read_u16(request, 4)? == 0 {
        return None;
    }
    let question = read_dns_question(request, 12)?;
    let address =
        resolve_dns_name(&question.name, records).and_then(|ip| ip.parse::<Ipv4Addr>().ok());
    let can_answer = question.qtype == 1 && address.is_some();
    let mut response = Vec::new();
    response.extend_from_slice(&request[0..2]);
    response.extend_from_slice(&u16_bytes(if can_answer { 0x8180 } else { 0x8183 }));
    response.extend_from_slice(&u16_bytes(1));
    response.extend_from_slice(&u16_bytes(if can_answer { 1 } else { 0 }));
    response.extend_from_slice(&u16_bytes(0));
    response.extend_from_slice(&u16_bytes(0));
    response.extend_from_slice(&request[12..question.end_offset]);
    if let Some(address) = address.filter(|_| can_answer) {
        response.extend_from_slice(&u16_bytes(0xc00c));
        response.extend_from_slice(&u16_bytes(1));
        response.extend_from_slice(&u16_bytes(1));
        response.extend_from_slice(&30_u32.to_be_bytes());
        response.extend_from_slice(&u16_bytes(4));
        response.extend_from_slice(&address.octets());
    }
    Some(response)
}

fn read_dns_question(data: &[u8], offset: usize) -> Option<DnsQuestion> {
    let mut labels = Vec::new();
    let mut cursor = offset;
    while cursor < data.len() {
        let len = *data.get(cursor)? as usize;
        cursor += 1;
        if len == 0 {
            break;
        }
        if (len & 0xc0) != 0 || cursor + len > data.len() {
            return None;
        }
        labels.push(String::from_utf8_lossy(&data[cursor..cursor + len]).to_string());
        cursor += len;
    }
    if labels.is_empty() || cursor + 4 > data.len() {
        return None;
    }
    Some(DnsQuestion {
        name: labels.join("."),
        qtype: read_u16(data, cursor)?,
        end_offset: cursor + 4,
    })
}

fn read_u16(data: &[u8], offset: usize) -> Option<u16> {
    Some(((*data.get(offset)? as u16) << 8) | (*data.get(offset + 1)? as u16))
}

fn u16_bytes(value: u16) -> [u8; 2] {
    value.to_be_bytes()
}

fn default_tunnel_backend() -> Box<dyn TunnelBackend> {
    #[cfg(target_os = "windows")]
    {
        Box::new(WindowsEmbeddableServiceBackend::new())
    }
    #[cfg(target_os = "linux")]
    {
        if linux_tunnel_driver_name() == "in-memory" {
            Box::new(InMemoryTunnelBackend::default())
        } else {
            Box::new(LinuxKernelWireGuardBackend::new())
        }
    }
    #[cfg(not(any(target_os = "linux", target_os = "windows")))]
    {
        Box::new(InMemoryTunnelBackend::default())
    }
}

#[cfg(test)]
fn backend_name() -> String {
    #[cfg(target_os = "windows")]
    {
        "windows-embeddable".to_string()
    }
    #[cfg(target_os = "linux")]
    {
        linux_tunnel_driver_name()
    }
    #[cfg(not(any(target_os = "linux", target_os = "windows")))]
    {
        "in-memory".to_string()
    }
}

fn platform_report_json() -> Value {
    let os = std::env::consts::OS.to_string();
    let kernel_release = fs::read_to_string("/proc/sys/kernel/osrelease")
        .ok()
        .map(|value| value.trim().to_string())
        .filter(|value| !value.is_empty());
    if os != "linux" {
        return json!({
            "os": os,
            "distroId": Value::Null,
            "versionId": Value::Null,
            "idLike": Vec::<String>::new(),
            "family": Value::Null,
            "kernelRelease": kernel_release,
            "packageManager": Value::Null,
        });
    }

    let parsed = fs::read_to_string("/etc/os-release")
        .ok()
        .map(|contents| parse_os_release(&contents))
        .unwrap_or_default();
    let distro_id = parsed.get("ID").cloned();
    let version_id = parsed.get("VERSION_ID").cloned();
    let id_like = parsed
        .get("ID_LIKE")
        .map(|value| {
            value
                .split_whitespace()
                .map(ToString::to_string)
                .collect::<Vec<_>>()
        })
        .unwrap_or_default();
    let family = linux_family(distro_id.as_deref(), &id_like);
    json!({
        "os": os,
        "distroId": distro_id,
        "versionId": version_id,
        "idLike": id_like,
        "family": family,
        "kernelRelease": kernel_release,
        "packageManager": linux_package_manager(family),
    })
}

fn platform_install_plan_json() -> Value {
    let platform = platform_report_json();
    if platform.get("os").and_then(Value::as_str) == Some("windows") {
        return json!({
            "platform": platform,
            "packages": Vec::<String>::new(),
            "supportedDriverModes": ["windows-embeddable", "wintun"],
            "warnings": [
                "SLAN Windows requires the packaged installer so app-core-service, app-core-helper, and wintun.dll are installed together",
                "the SLANAppCoreService Windows service must run as LocalSystem to apply the Wintun adapter and virtual IP",
            ],
        });
    }
    let family = platform
        .get("family")
        .and_then(Value::as_str)
        .unwrap_or("unknown");
    let mut warnings = Vec::new();
    let packages = match family {
        "debian" => vec!["wireguard-tools", "iproute2"],
        "rhel" => {
            warnings.push(
                "wireguard kernel packages may require extra repositories on some RHEL-family systems",
            );
            vec!["wireguard-tools", "iproute"]
        }
        "arch" => vec!["wireguard-tools", "iproute2"],
        _ => {
            warnings.push(
                "unsupported or unknown linux distribution; package plan may need manual adjustments",
            );
            Vec::new()
        }
    };
    let supported_driver_modes = if platform.get("os").and_then(Value::as_str) == Some("linux") {
        vec!["in-memory", "linux-kernel-shell", "linux-kernel-native"]
    } else {
        vec!["in-memory"]
    };
    json!({
        "platform": platform,
        "packages": packages,
        "supportedDriverModes": supported_driver_modes,
        "warnings": warnings,
    })
}

fn platform_doctor_json(diagnostics: &tunnel::TunnelBackendDiagnostics) -> Value {
    if std::env::consts::OS == "windows" {
        return platform_doctor_windows_json(diagnostics);
    }
    let checks = vec![
        platform_check_json(
            "ip_command",
            if command_on_path("ip") { "ok" } else { "fail" },
            if command_on_path("ip") {
                "ip command available".to_string()
            } else {
                "ip command missing from PATH".to_string()
            },
        ),
        platform_check_json(
            "wg_command",
            if command_on_path("wg") { "ok" } else { "warn" },
            if command_on_path("wg") {
                "wg command available".to_string()
            } else {
                "wg command missing from PATH".to_string()
            },
        ),
        platform_check_json(
            "tun_device",
            if Path::new("/dev/net/tun").exists() {
                "ok"
            } else {
                "warn"
            },
            if Path::new("/dev/net/tun").exists() {
                "/dev/net/tun present".to_string()
            } else {
                "/dev/net/tun missing".to_string()
            },
        ),
        platform_check_json(
            "tunnel_backend",
            "ok",
            format!(
                "backend {} mode {} executor {} peers {} commands {}",
                diagnostics.name,
                diagnostics.execution_mode.unwrap_or("unknown"),
                diagnostics.execution_backend.unwrap_or("unknown"),
                diagnostics.planned_peer_count,
                diagnostics.recent_command_count
            ),
        ),
    ];
    json!({
        "platform": platform_report_json(),
        "tunnelBackend": {
            "name": diagnostics.name,
            "executionMode": diagnostics.execution_mode,
            "executionBackend": diagnostics.execution_backend,
            "interfaceName": diagnostics.interface_name,
            "isUp": diagnostics.is_up,
            "plannedPeerCount": diagnostics.planned_peer_count,
            "recentCommandCount": diagnostics.recent_command_count,
        },
        "checks": checks,
    })
}

fn platform_doctor_windows_json(diagnostics: &tunnel::TunnelBackendDiagnostics) -> Value {
    let checks = vec![
        platform_check_json(
            "app_core_service",
            windows_service_status(),
            windows_service_detail(),
        ),
        platform_check_json(
            "helper_executable",
            if sibling_executable_exists("app-core-helper.exe") {
                "ok"
            } else {
                "fail"
            },
            if sibling_executable_exists("app-core-helper.exe") {
                "app-core-helper.exe is present next to the running binary".to_string()
            } else {
                "app-core-helper.exe is missing next to the running binary".to_string()
            },
        ),
        platform_check_json(
            "wintun_dll",
            if sibling_executable_exists("wintun.dll") {
                "ok"
            } else {
                "fail"
            },
            if sibling_executable_exists("wintun.dll") {
                "wintun.dll is present next to the running binary".to_string()
            } else {
                "wintun.dll is missing next to the running binary".to_string()
            },
        ),
        platform_check_json(
            "powershell",
            if command_on_path("powershell") || command_on_path("pwsh") {
                "ok"
            } else {
                "fail"
            },
            if command_on_path("powershell") || command_on_path("pwsh") {
                "PowerShell is available for adapter/IP management".to_string()
            } else {
                "PowerShell is missing from PATH".to_string()
            },
        ),
        platform_check_json(
            "netsh",
            if command_on_path("netsh") {
                "ok"
            } else {
                "warn"
            },
            if command_on_path("netsh") {
                "netsh is available as a Windows network fallback".to_string()
            } else {
                "netsh is missing from PATH".to_string()
            },
        ),
        platform_check_json(
            "slan_lan_adapter",
            windows_slan_adapter_status(),
            windows_slan_adapter_detail(),
        ),
        platform_check_json(
            "tunnel_backend",
            "ok",
            format!(
                "backend {} mode {} executor {} peers {} commands {}",
                diagnostics.name,
                diagnostics.execution_mode.unwrap_or("unknown"),
                diagnostics.execution_backend.unwrap_or("unknown"),
                diagnostics.planned_peer_count,
                diagnostics.recent_command_count
            ),
        ),
    ];
    json!({
        "platform": platform_report_json(),
        "tunnelBackend": {
            "name": diagnostics.name,
            "executionMode": diagnostics.execution_mode,
            "executionBackend": diagnostics.execution_backend,
            "interfaceName": diagnostics.interface_name,
            "isUp": diagnostics.is_up,
            "plannedPeerCount": diagnostics.planned_peer_count,
            "recentCommandCount": diagnostics.recent_command_count,
        },
        "checks": checks,
    })
}

fn platform_check_json(name: &str, status: &str, detail: String) -> Value {
    json!({
        "name": name,
        "status": status,
        "detail": detail,
    })
}

fn sibling_executable_exists(file_name: &str) -> bool {
    std::env::current_exe()
        .ok()
        .and_then(|path| path.parent().map(|parent| parent.join(file_name)))
        .map(|path| path.exists())
        .unwrap_or(false)
}

#[cfg(target_os = "windows")]
fn windows_service_status() -> &'static str {
    match windows_service_query_output() {
        Some(output) if output.contains("RUNNING") => "ok",
        Some(_) => "warn",
        None => "fail",
    }
}

#[cfg(not(target_os = "windows"))]
fn windows_service_status() -> &'static str {
    "warn"
}

#[cfg(target_os = "windows")]
fn windows_service_detail() -> String {
    match windows_service_query_output() {
        Some(output) if output.contains("RUNNING") => {
            "SLANAppCoreService is installed and running".to_string()
        }
        Some(output) if output.contains("STOPPED") => {
            "SLANAppCoreService is installed but stopped".to_string()
        }
        Some(_) => "SLANAppCoreService is installed but not running".to_string(),
        None => "SLANAppCoreService is not installed or sc.exe is unavailable".to_string(),
    }
}

#[cfg(not(target_os = "windows"))]
fn windows_service_detail() -> String {
    "Windows service check is only available on Windows".to_string()
}

#[cfg(target_os = "windows")]
fn windows_service_query_output() -> Option<String> {
    let output = Command::new("sc.exe")
        .args(["query", "SLANAppCoreService"])
        .output()
        .ok()?;
    if !output.status.success() {
        return None;
    }
    Some(String::from_utf8_lossy(&output.stdout).to_string())
}

#[cfg(target_os = "windows")]
fn windows_slan_adapter_status() -> &'static str {
    match windows_slan_adapter_query_output() {
        Some(output) if !output.trim().is_empty() => "ok",
        Some(_) => "warn",
        None => "warn",
    }
}

#[cfg(not(target_os = "windows"))]
fn windows_slan_adapter_status() -> &'static str {
    "warn"
}

#[cfg(target_os = "windows")]
fn windows_slan_adapter_detail() -> String {
    match windows_slan_adapter_query_output() {
        Some(output) if !output.trim().is_empty() => {
            format!("SLAN LAN Adapter detected: {}", output.trim())
        }
        Some(_) => {
            "SLAN LAN Adapter was not found; the installer or service can recreate it".to_string()
        }
        None => "unable to query SLAN LAN Adapter with PowerShell".to_string(),
    }
}

#[cfg(not(target_os = "windows"))]
fn windows_slan_adapter_detail() -> String {
    "Windows adapter check is only available on Windows".to_string()
}

#[cfg(target_os = "windows")]
fn windows_slan_adapter_query_output() -> Option<String> {
    let script = "$adapter = Get-NetAdapter -IncludeHidden -ErrorAction SilentlyContinue | Where-Object { $_.Name -eq 'SLAN LAN Adapter' -or $_.InterfaceDescription -like '*Wintun*' -or $_.InterfaceDescription -like '*WireGuard*Tunnel*' -or $_.InterfaceDescription -like '*WireGuardNT*' } | Select-Object -First 1 Name, InterfaceDescription, Status; if ($adapter) { Write-Output ($adapter.Name + '|' + $adapter.InterfaceDescription + '|' + $adapter.Status) }";
    let output = Command::new("powershell.exe")
        .args([
            "-NoProfile",
            "-NonInteractive",
            "-ExecutionPolicy",
            "Bypass",
            "-Command",
            script,
        ])
        .output()
        .ok()?;
    if !output.status.success() {
        return None;
    }
    Some(String::from_utf8_lossy(&output.stdout).to_string())
}

fn command_on_path(command: &str) -> bool {
    std::env::var_os("PATH")
        .map(|paths| {
            std::env::split_paths(&paths).any(|dir| {
                let candidate = dir.join(command);
                if candidate.exists() {
                    return true;
                }
                #[cfg(target_os = "windows")]
                {
                    return dir.join(format!("{command}.exe")).exists();
                }
                #[cfg(not(target_os = "windows"))]
                {
                    false
                }
            })
        })
        .unwrap_or(false)
}

fn parse_os_release(contents: &str) -> std::collections::BTreeMap<String, String> {
    contents
        .lines()
        .filter_map(|line| {
            let line = line.trim();
            if line.is_empty() || line.starts_with('#') {
                return None;
            }
            let (key, value) = line.split_once('=')?;
            Some((
                key.trim().to_string(),
                value.trim_matches('"').to_ascii_lowercase(),
            ))
        })
        .collect()
}

fn linux_family(id: Option<&str>, id_like: &[String]) -> &'static str {
    let mut values = id_like.iter().map(String::as_str).collect::<Vec<_>>();
    if let Some(id) = id {
        values.push(id);
    }
    if values
        .iter()
        .any(|value| matches!(*value, "ubuntu" | "debian"))
    {
        "debian"
    } else if values
        .iter()
        .any(|value| matches!(*value, "rhel" | "centos" | "fedora" | "rocky" | "almalinux"))
    {
        "rhel"
    } else if values.iter().any(|value| *value == "arch") {
        "arch"
    } else {
        "unknown"
    }
}

fn linux_package_manager(family: &str) -> Option<&'static str> {
    match family {
        "debian" => Some("apt-get"),
        "rhel" => Some("dnf"),
        "arch" => Some("pacman"),
        _ => None,
    }
}

#[cfg(target_os = "linux")]
fn linux_tunnel_driver_name() -> String {
    match std::env::var("SLAN_TUNNEL_DRIVER")
        .ok()
        .map(|value| value.trim().to_ascii_lowercase())
        .as_deref()
    {
        Some("in-memory") | Some("memory") | Some("inmemory") => "in-memory".to_string(),
        Some("auto") | Some("linux") | Some("linux-kernel") | Some("wg-kernel") | None
        | Some("") => "linux-kernel".to_string(),
        Some(other) => {
            write_helper_log(&format!(
                "unsupported SLAN_TUNNEL_DRIVER '{other}', falling back to linux-kernel"
            ));
            "linux-kernel".to_string()
        }
    }
}

fn current_timestamp_ms() -> u64 {
    std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .map(|duration| duration.as_millis() as u64)
        .unwrap_or_default()
}

#[cfg(test)]
mod tests {
    use super::{
        build_dns_response, build_rpc_response, classify_helper_error, classify_structured_error,
        current_timestamp_ms, linux_family, linux_package_manager, maybe_test_override_result,
        normalize_dns_records, nrpt_namespaces_for_records, parse_os_release,
        persisted_helper_state_path, resolve_dns_name, resolve_helper_control_base_url,
        should_persist_after_rpc, HelperTunnelConfiguration, HelperTunnelHost,
        PersistedHelperState,
    };
    use serde_json::{json, Value};
    use std::fs;
    use std::path::PathBuf;
    use std::sync::{Mutex, OnceLock};
    use tunnel::TunnelManager;

    fn env_lock() -> std::sync::MutexGuard<'static, ()> {
        static LOCK: OnceLock<Mutex<()>> = OnceLock::new();
        LOCK.get_or_init(|| Mutex::new(()))
            .lock()
            .expect("env lock")
    }

    fn unique_state_path(name: &str) -> PathBuf {
        std::env::temp_dir().join(format!(
            "slan-helper-{name}-{}-{}.json",
            std::process::id(),
            current_timestamp_ms()
        ))
    }

    fn write_persisted_state(path: &PathBuf, control_base_url: &str) {
        let payload = serde_json::to_vec(&PersistedHelperState {
            control_base_url: control_base_url.to_string(),
            snapshot: ffi_bridge::AppCoreSnapshot::default(),
        })
        .expect("encode persisted helper state");
        fs::write(path, payload).expect("write persisted helper state");
    }

    fn dns_query(name: &str) -> Vec<u8> {
        let mut request = vec![0x12, 0x34, 0x01, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0];
        for label in name.split('.') {
            request.push(label.len() as u8);
            request.extend_from_slice(label.as_bytes());
        }
        request.push(0);
        request.extend_from_slice(&1_u16.to_be_bytes());
        request.extend_from_slice(&1_u16.to_be_bytes());
        request
    }

    #[test]
    fn local_dns_resolves_specific_wildcard_records() {
        let records = normalize_dns_records(vec![
            ("*.xx.com".to_string(), "10.0.0.2".to_string()),
            ("*.*.xx.net".to_string(), "10.0.0.3".to_string()),
        ]);
        assert_eq!(
            resolve_dns_name("api.xx.com", &records).as_deref(),
            Some("10.0.0.2")
        );
        assert_eq!(
            resolve_dns_name("a.b.xx.net", &records).as_deref(),
            Some("10.0.0.3")
        );
        assert_eq!(resolve_dns_name("xx.com", &records), None);
        assert_eq!(resolve_dns_name("api.foo.xx.com", &records), None);
    }

    #[test]
    fn local_dns_builds_a_record_response() {
        let records = normalize_dns_records(vec![("*.xx.com".into(), "10.0.0.2".into())]);
        let response = build_dns_response(&dns_query("api.xx.com"), &records).expect("response");
        assert_eq!(&response[0..2], &[0x12, 0x34]);
        assert_eq!(&response[2..4], &[0x81, 0x80]);
        assert_eq!(&response[6..8], &[0x00, 0x01]);
        assert!(response.ends_with(&[10, 0, 0, 2]));
    }

    #[test]
    fn local_dns_derives_nrpt_namespaces_from_wildcards() {
        let records = normalize_dns_records(vec![
            ("*.xx.com".into(), "10.0.0.2".into()),
            ("*.*.xx.net".into(), "10.0.0.3".into()),
        ]);

        assert_eq!(
            nrpt_namespaces_for_records(&records),
            vec![".xx.com".to_string(), ".xx.net".to_string()]
        );
    }

    #[test]
    fn local_dns_start_skips_empty_effective_records() {
        let host = HelperTunnelHost::new();
        host.start_local_dns(vec![("".into(), "".into())])
            .expect("empty effective records should be skipped");
        assert!(host.dns_server.lock().expect("dns lock").is_none());
    }

    #[test]
    fn helper_state_file_env_overrides_default_persisted_path() {
        let _guard = env_lock();
        let path = unique_state_path("path-env");
        unsafe {
            std::env::set_var("SLAN_APP_CORE_STATE_FILE", &path);
        }

        assert_eq!(persisted_helper_state_path(), path);

        unsafe {
            std::env::remove_var("SLAN_APP_CORE_STATE_FILE");
        }
    }

    #[test]
    fn helper_read_only_rpc_methods_do_not_request_state_persistence() {
        for method in [
            "helperStatus",
            "platformDoctor",
            "platformInstallPlan",
            "tunnelRuntimeView",
            "controlStatus",
        ] {
            assert!(
                !should_persist_after_rpc(method),
                "{method} should not persist helper state"
            );
        }
        assert!(should_persist_after_rpc("controlSync"));
        assert!(should_persist_after_rpc("enableLocalNetwork"));
    }

    #[test]
    fn helper_control_base_url_uses_env_before_persisted_state() {
        let _guard = env_lock();
        let path = unique_state_path("env-first");
        write_persisted_state(&path, "http://persisted.example");
        unsafe {
            std::env::set_var("SLAN_APP_CORE_STATE_FILE", &path);
            std::env::set_var("SLAN_CONTROL_BASE_URL", " http://env.example ");
        }

        assert_eq!(
            resolve_helper_control_base_url().expect("resolve helper control base url"),
            "http://env.example"
        );

        unsafe {
            std::env::remove_var("SLAN_CONTROL_BASE_URL");
            std::env::remove_var("SLAN_APP_CORE_STATE_FILE");
        }
        let _ = fs::remove_file(path);
    }

    #[test]
    fn helper_control_base_url_falls_back_to_persisted_state() {
        let _guard = env_lock();
        let path = unique_state_path("persisted");
        write_persisted_state(&path, " http://persisted.example ");
        unsafe {
            std::env::set_var("SLAN_APP_CORE_STATE_FILE", &path);
            std::env::remove_var("SLAN_CONTROL_BASE_URL");
        }

        assert_eq!(
            resolve_helper_control_base_url().expect("resolve helper control base url"),
            "http://persisted.example"
        );

        unsafe {
            std::env::remove_var("SLAN_APP_CORE_STATE_FILE");
        }
        let _ = fs::remove_file(path);
    }

    #[test]
    fn classifies_probe_timeout_errors() {
        let (code, message) =
            classify_helper_error("probe", "probe_timeout: timed out waiting for probe reply");
        assert_eq!(code, "probe_timeout");
        assert_eq!(message, "timed out waiting for probe reply");
    }

    #[test]
    fn classifies_probe_transport_errors() {
        let (code, _) = classify_helper_error(
            "probe",
            "probe_transport_error: read relay udp socket failed: connection reset",
        );
        assert_eq!(code, "probe_transport_error");
    }

    #[test]
    fn classifies_probe_unsupported_errors() {
        let (code, message) = classify_helper_error(
            "probe",
            "probe_unsupported_path: active path does not support probe",
        );
        assert_eq!(code, "probe_unsupported_path");
        assert_eq!(message, "active path does not support probe");
    }

    #[test]
    fn classifies_app_core_structured_errors() {
        let (code, message) = classify_helper_error(
            "applyTunnelConfiguration",
            "app_core_invalid_tunnel_config: missing peerVirtualIp",
        );
        assert_eq!(code, "app_core_invalid_tunnel_config");
        assert_eq!(message, "missing peerVirtualIp");
    }

    #[test]
    fn leaves_non_probe_errors_generic() {
        let (code, _) = classify_helper_error("connect", "missing bootstrap config");
        assert_eq!(code, "app_core_helper_error");
    }

    #[test]
    fn classifies_send_errors_from_probe_prefix() {
        let (code, message) =
            classify_helper_error("send", "send_timeout: timed out waiting for send reply");
        assert_eq!(code, "send_timeout");
        assert_eq!(message, "timed out waiting for send reply");
    }

    #[test]
    fn classifies_send_unsupported_errors() {
        let (code, message) = classify_helper_error(
            "send",
            "send_unsupported_path: active path does not support send",
        );
        assert_eq!(code, "send_unsupported_path");
        assert_eq!(message, "active path does not support send");
    }

    #[test]
    fn falls_back_to_send_failed_for_untyped_send_errors() {
        let (code, message) = classify_helper_error("send", "relay client exploded");
        assert_eq!(code, "send_failed");
        assert_eq!(message, "relay client exploded");
    }

    #[test]
    fn classifies_structured_error_prefixes_directly() {
        let (code, message) =
            classify_structured_error("send_unsupported_path: active path does not support send")
                .expect("typed data-plane error");
        assert_eq!(code, "send_unsupported_path");
        assert_eq!(message, "active path does not support send");
    }

    #[test]
    fn rpc_response_serializes_send_unsupported_error_fields() {
        let response = build_rpc_response(
            "send",
            Err("send_unsupported_path: active path does not support send".to_string()),
        );
        let json = serde_json::to_value(response).expect("serialize rpc response");
        assert_eq!(json["ok"], false);
        assert_eq!(
            json["error"],
            "send_unsupported_path: active path does not support send"
        );
        assert_eq!(json["errorCode"], "send_unsupported_path");
        assert_eq!(json["errorMessage"], "active path does not support send");
    }

    #[test]
    fn rpc_response_serializes_probe_unsupported_error_fields() {
        let response = build_rpc_response(
            "probe",
            Err("probe_unsupported_path: active path does not support probe".to_string()),
        );
        let json = serde_json::to_value(response).expect("serialize rpc response");
        assert_eq!(json["ok"], false);
        assert_eq!(
            json["error"],
            "probe_unsupported_path: active path does not support probe"
        );
        assert_eq!(json["errorCode"], "probe_unsupported_path");
        assert_eq!(json["errorMessage"], "active path does not support probe");
    }

    #[test]
    fn rpc_response_serializes_success_without_error_fields() {
        let response = build_rpc_response("send", Ok(json!({ "bytesSent": 5 })));
        let json = serde_json::to_value(response).expect("serialize rpc response");
        assert_eq!(json["ok"], true);
        assert_eq!(json["result"]["bytesSent"], 5);
        assert_eq!(json.get("error"), None);
        assert_eq!(json.get("errorCode"), None);
        assert_eq!(json.get("errorMessage"), None);
    }

    #[test]
    fn test_override_returns_send_error_from_env() {
        unsafe {
            std::env::set_var(
                "SLAN_APP_CORE_HELPER_TEST_SEND_ERROR",
                "send_unsupported_path: active path does not support send",
            );
        }
        let result = maybe_test_override_result("send");
        unsafe {
            std::env::remove_var("SLAN_APP_CORE_HELPER_TEST_SEND_ERROR");
        }
        assert_eq!(
            result,
            Some(Err(
                "send_unsupported_path: active path does not support send".to_string()
            ))
        );
    }

    #[test]
    fn test_override_returns_probe_result_from_env() {
        unsafe {
            std::env::set_var(
                "SLAN_APP_CORE_HELPER_TEST_PROBE_RESULT",
                r#"{"probeId":"probe-1","bytesSent":5}"#,
            );
        }
        let result = maybe_test_override_result("probe");
        unsafe {
            std::env::remove_var("SLAN_APP_CORE_HELPER_TEST_PROBE_RESULT");
        }
        assert_eq!(
            result,
            Some(Ok(json!({
                "probeId": "probe-1",
                "bytesSent": 5,
            })))
        );
    }

    #[test]
    fn test_override_returns_invalid_json_error_for_probe_result_env() {
        unsafe {
            std::env::set_var("SLAN_APP_CORE_HELPER_TEST_PROBE_RESULT", "{");
        }
        let result = maybe_test_override_result("probe");
        unsafe {
            std::env::remove_var("SLAN_APP_CORE_HELPER_TEST_PROBE_RESULT");
        }
        assert!(result
            .expect("override result")
            .err()
            .expect("override should return parse error")
            .starts_with(
                "invalid helper test result json in SLAN_APP_CORE_HELPER_TEST_PROBE_RESULT:"
            ));
    }

    #[test]
    fn tunnel_runtime_view_tracks_apply_and_bring_up() {
        let host = HelperTunnelHost::new();
        let result = host
            .invoke(
                "applyTunnelConfiguration",
                json!({
                    "transport": "relay",
                    "localVirtualIp": "100.64.0.10",
                    "peerVirtualIp": "100.64.0.2",
                    "debugEngineMode": "loopback",
                    "wireguardInterface": {
                        "interfaceName": "slan0",
                        "keyPair": {
                            "publicKey": "self-pk",
                            "privateKey": "self-sk"
                        },
                        "listenPort": 51820,
                        "mtu": 1280,
                        "addresses": ["100.64.0.10/32"],
                        "dnsServers": ["1.1.1.1"],
                        "peers": [],
                    },
                    "wireguardPeer": {
                        "peerNodeId": "peer-1",
                        "publicKey": "peer-pk",
                        "endpoint": "203.0.113.10:51820",
                        "allowedIps": [{"cidr": "100.64.0.2/32"}],
                        "persistentKeepaliveSeconds": 15,
                    }
                }),
            )
            .expect("apply result");
        assert_eq!(result["accepted"], true);
        assert_eq!(result["phase"], "configured");

        let started = host
            .invoke("bringTunnelUp", json!({}))
            .expect("bring up result");
        assert_eq!(started["action"], "bringTunnelUp");

        let runtime = host
            .invoke(
                "tunnelRuntimeView",
                json!({
                    "peerVirtualIp": "100.64.0.2"
                }),
            )
            .expect("runtime");
        assert_eq!(runtime["peerVirtualIp"], "100.64.0.2");
        assert_eq!(runtime["backendName"], super::backend_name());
        assert_eq!(runtime["backendExecutionMode"].as_str().is_some(), true);
        assert_eq!(runtime["backendExecutionBackend"].as_str().is_some(), true);
        assert_eq!(runtime["backendPlannedPeerCount"].as_u64(), Some(1));
        assert_eq!(runtime["lastAppliedAtMs"].as_u64().is_some(), true);
        assert_eq!(runtime["state"].as_str().is_some(), true);
    }

    #[test]
    fn tunnel_runtime_view_clears_after_remove() {
        let host = HelperTunnelHost::new();
        let now = current_timestamp_ms();
        assert!(now > 0);
        host.invoke(
            "applyTunnelConfiguration",
            json!({
                "transport": "relay",
                "localVirtualIp": "100.64.0.10",
                "peerVirtualIp": "100.64.0.2",
                "wireguardInterface": {
                    "interfaceName": "slan0",
                    "keyPair": {
                        "publicKey": "self-pk",
                        "privateKey": "self-sk"
                    },
                    "addresses": ["100.64.0.10/32"],
                    "dnsServers": [],
                    "peers": [],
                },
                "wireguardPeer": {
                    "publicKey": "peer-pk",
                    "allowedIps": [{"cidr": "100.64.0.2/32"}],
                }
            }),
        )
        .expect("apply");
        host.invoke(
            "removeTunnelPeer",
            json!({
                "peerVirtualIp": "100.64.0.2"
            }),
        )
        .expect("remove");

        let runtime = host
            .invoke(
                "tunnelRuntimeView",
                json!({
                    "peerVirtualIp": "100.64.0.2"
                }),
            )
            .expect("runtime");
        assert_eq!(runtime, Value::Null);
    }

    #[test]
    fn helper_tunnel_configuration_deserializes_camel_case_payload() {
        let configuration: HelperTunnelConfiguration = serde_json::from_value(json!({
            "transport": "relay",
            "localVirtualIp": "100.64.0.10",
            "peerVirtualIp": "100.64.0.2",
            "wireguardInterface": {
                "interfaceName": "slan0",
                "keyPair": {
                    "publicKey": "self-pk",
                    "privateKey": "self-sk"
                },
                "addresses": ["100.64.0.10/32"],
                "dnsServers": [],
                "peers": [],
            },
            "wireguardPeer": {
                "publicKey": "peer-pk",
                "allowedIps": [{"cidr": "100.64.0.2/32"}],
            }
        }))
        .expect("configuration");
        assert_eq!(configuration.local_virtual_ip, "100.64.0.10");
        assert_eq!(configuration.peer_virtual_ip, "100.64.0.2");
    }

    #[test]
    fn platform_doctor_reports_backend_diagnostics() {
        let host = HelperTunnelHost::new();
        let report = host
            .invoke("platformDoctor", json!({}))
            .expect("platform doctor");

        assert_eq!(report["platform"]["os"].as_str().is_some(), true);
        assert_eq!(report["tunnelBackend"]["name"].as_str().is_some(), true);
        assert!(report["checks"]
            .as_array()
            .expect("checks")
            .iter()
            .any(|check| check["name"] == "tunnel_backend"
                && check["detail"]
                    .as_str()
                    .unwrap_or_default()
                    .contains("backend")));
    }

    #[test]
    fn platform_install_plan_reports_supported_driver_modes() {
        let host = HelperTunnelHost::new();
        let plan = host
            .invoke("platformInstallPlan", json!({}))
            .expect("platform install plan");

        assert_eq!(plan["platform"]["os"].as_str().is_some(), true);
        let modes = plan["supportedDriverModes"]
            .as_array()
            .expect("driver modes");
        assert!(!modes.is_empty());
        if std::env::consts::OS == "windows" {
            assert!(modes.iter().any(|mode| mode == "wintun"));
        } else {
            assert!(modes.iter().any(|mode| mode == "in-memory"));
        }
    }

    #[test]
    fn parses_linux_os_release_family() {
        let parsed = parse_os_release(
            r#"
ID=ubuntu
VERSION_ID="24.04"
ID_LIKE="debian"
"#,
        );
        let id_like = parsed
            .get("ID_LIKE")
            .map(|value| {
                value
                    .split_whitespace()
                    .map(ToString::to_string)
                    .collect::<Vec<_>>()
            })
            .unwrap_or_default();

        assert_eq!(parsed.get("ID").map(String::as_str), Some("ubuntu"));
        assert_eq!(
            linux_family(parsed.get("ID").map(String::as_str), &id_like),
            "debian"
        );
        assert_eq!(linux_package_manager("debian"), Some("apt-get"));
    }
}
