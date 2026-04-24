use std::fs::{self, OpenOptions};
use std::io::{self, BufRead, BufReader, Write};
use std::net::{TcpListener, TcpStream};
use std::path::{Path, PathBuf};
use std::sync::{Arc, Mutex};
use std::time::{SystemTime, UNIX_EPOCH};

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
    let base_url = std::env::var("SLAN_CONTROL_BASE_URL")
        .map_err(|_| "missing SLAN_CONTROL_BASE_URL for app-core-helper".to_string())?;
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
    let facade = JsonAppCoreFacade::new(DefaultAppCoreFacade::new_with_tunnel_key_provider(
        HttpControllerClient::new(base_url, TcpJsonHttpTransport::default()),
        p2p_connector.clone(),
        relay_client.clone(),
        derp_pool.clone(),
        InMemoryPathManager::new(derp_pool, relay_client, p2p_connector),
        tunnel_host.clone(),
        key_provider,
    ));

    if let Some(address) = tcp_host {
        return run_tcp_host(&address, &facade, tunnel_host.as_ref());
    }

    let stdin = io::stdin();
    let mut stdout = io::stdout().lock();
    for line in stdin.lock().lines() {
        let line = line.map_err(|err| err.to_string())?;
        if line.trim().is_empty() {
            continue;
        }
        let response = handle_rpc_line(&line, &facade, tunnel_host.as_ref())?;
        serde_json::to_writer(&mut stdout, &response).map_err(|err| err.to_string())?;
        stdout.write_all(b"\n").map_err(|err| err.to_string())?;
        stdout.flush().map_err(|err| err.to_string())?;
    }
    Ok(())
}

fn run_tcp_host<F>(
    address: &str,
    facade: &JsonAppCoreFacade<F>,
    tunnel_host: &HelperTunnelHost,
) -> Result<(), String>
where
    F: ffi_bridge::AppCoreFacade,
{
    let listener = TcpListener::bind(address)
        .map_err(|err| format!("failed to bind app-core-helper tcp host on {address}: {err}"))?;
    write_helper_log(&format!("tcp host listening on {address}"));
    for stream in listener.incoming() {
        let stream = stream.map_err(|err| format!("tcp host accept failed: {err}"))?;
        write_helper_log("accepted tcp client connection");
        handle_tcp_client(stream, facade, tunnel_host)?;
    }
    Ok(())
}

fn handle_tcp_client<F>(
    stream: TcpStream,
    facade: &JsonAppCoreFacade<F>,
    tunnel_host: &HelperTunnelHost,
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
        let response = handle_rpc_line(&line, facade, tunnel_host)?;
        serde_json::to_writer(&mut writer, &response).map_err(|err| err.to_string())?;
        writer.write_all(b"\n").map_err(|err| err.to_string())?;
        writer.flush().map_err(|err| err.to_string())?;
    }
}

fn handle_rpc_line<F>(
    line: &str,
    facade: &JsonAppCoreFacade<F>,
    tunnel_host: &HelperTunnelHost,
) -> Result<RpcResponse, String>
where
    F: ffi_bridge::AppCoreFacade,
{
    let request: RpcRequest = serde_json::from_str(line).map_err(|err| err.to_string())?;
    Ok(handle_rpc_request(request, facade, tunnel_host))
}

fn handle_rpc_request<F>(
    request: RpcRequest,
    facade: &JsonAppCoreFacade<F>,
    tunnel_host: &HelperTunnelHost,
) -> RpcResponse
where
    F: ffi_bridge::AppCoreFacade,
{
    let method_name = request.method.clone();
    let result = maybe_test_override_result(&method_name)
        .unwrap_or_else(|| invoke_helper(&method_name, request.args, facade, tunnel_host));
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
) -> Result<Value, String>
where
    F: ffi_bridge::AppCoreFacade,
{
    if HelperTunnelHost::supports_method(method) {
        write_helper_log(&format!("dispatching tunnel method {method}"));
        tunnel_host.invoke(method, args)
    } else {
        facade.invoke(method, args)
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
}

impl HelperTunnelHost {
    fn new() -> Self {
        Self {
            backend: default_tunnel_backend(),
            state: Mutex::new(HelperTunnelState::default()),
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

fn platform_check_json(name: &str, status: &str, detail: String) -> Value {
    json!({
        "name": name,
        "status": status,
        "detail": detail,
    })
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
        build_rpc_response, classify_helper_error, classify_structured_error, current_timestamp_ms,
        linux_family, linux_package_manager, maybe_test_override_result, parse_os_release,
        HelperTunnelConfiguration, HelperTunnelHost,
    };
    use serde_json::{json, Value};

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
        assert!(plan["supportedDriverModes"]
            .as_array()
            .expect("driver modes")
            .iter()
            .any(|mode| mode == "in-memory"));
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
