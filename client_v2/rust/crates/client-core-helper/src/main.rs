use std::{
    fs,
    io::{BufRead, BufReader, Write},
    net::{TcpListener, TcpStream},
    path::PathBuf,
    process::Command,
    sync::{Arc, Mutex},
    thread,
};

use anyhow::{bail, Context, Result};
use client_core::NetworkRuntimeState;
use serde::Deserialize;
use serde_json::{json, Value};

const DEFAULT_HELPER_HOST: &str = "127.0.0.1:46393";
const DEFAULT_INTERFACE_NAME: &str = "SLAN LAN Adapter";

#[derive(Debug, Deserialize)]
struct HelperRequest {
    method: String,
    #[serde(default)]
    args: Value,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
struct ConfigureIpArgs {
    virtual_ip: String,
    #[serde(default = "default_prefix_len")]
    prefix_len: u8,
    interface_name: Option<String>,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
struct ConfigureRoutesArgs {
    #[serde(default)]
    routes: Vec<RouteArg>,
    interface_name: Option<String>,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
struct ConfigureDnsArgs {
    #[serde(default)]
    dns_servers: Vec<String>,
    interface_name: Option<String>,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
struct RouteArg {
    destination: String,
    gateway: Option<String>,
}

fn main() -> Result<()> {
    let bind_address = std::env::var("SLAN_CLIENT_CORE_HELPER_HOST")
        .unwrap_or_else(|_| DEFAULT_HELPER_HOST.to_string());
    let listener = TcpListener::bind(&bind_address)
        .with_context(|| format!("bind client-core-helper on {bind_address}"))?;
    let state = Arc::new(Mutex::new(load_state().unwrap_or_default()));
    println!("client-core-helper listening on {bind_address}");

    for stream in listener.incoming() {
        let stream = stream.context("accept client-core-helper connection")?;
        let state = Arc::clone(&state);
        thread::spawn(move || {
            if let Err(error) = handle_connection(stream, state) {
                eprintln!("client-core-helper connection error: {error:#}");
            }
        });
    }
    Ok(())
}

fn handle_connection(stream: TcpStream, state: Arc<Mutex<NetworkRuntimeState>>) -> Result<()> {
    let mut writer = stream.try_clone().context("clone helper stream")?;
    let mut reader = BufReader::new(stream);
    let mut line = String::new();
    while reader.read_line(&mut line)? > 0 {
        let response = match handle_request(line.trim(), &state) {
            Ok(data) => json!({ "ok": true, "data": data }),
            Err(error) => json!({ "ok": false, "error": error.to_string() }),
        };
        serde_json::to_writer(&mut writer, &response).context("encode helper response")?;
        writer
            .write_all(b"\n")
            .context("write helper response newline")?;
        writer.flush().context("flush helper response")?;
        line.clear();
    }
    Ok(())
}

fn handle_request(line: &str, state: &Arc<Mutex<NetworkRuntimeState>>) -> Result<Value> {
    let request: HelperRequest = serde_json::from_str(line).context("decode helper request")?;
    let mut state = state.lock().expect("helper state mutex poisoned");
    match request.method.as_str() {
        "installAdapter" => {
            ensure_adapter_present(DEFAULT_INTERFACE_NAME)?;
            state.adapter_present = true;
            persist_state(&state)?;
            Ok(json!({ "adapterPresent": state.adapter_present }))
        }
        "configureIp" => {
            let args: ConfigureIpArgs =
                serde_json::from_value(request.args).context("decode configureIp args")?;
            let interface_name = interface_name_or_default(args.interface_name);
            configure_adapter_ip(&interface_name, &args.virtual_ip, args.prefix_len)?;
            state.adapter_present = true;
            state.network_enabled = true;
            state.virtual_ip = Some(args.virtual_ip);
            persist_state(&state)?;
            Ok(json!({
                "interfaceName": interface_name,
                "virtualIp": state.virtual_ip,
                "prefixLen": args.prefix_len,
            }))
        }
        "configureRoutes" => {
            let args: ConfigureRoutesArgs =
                serde_json::from_value(request.args).context("decode configureRoutes args")?;
            let interface_name = interface_name_or_default(args.interface_name);
            configure_routes(&interface_name, &args.routes)?;
            persist_state(&state)?;
            Ok(json!({ "interfaceName": interface_name }))
        }
        "configureDns" => {
            let args: ConfigureDnsArgs =
                serde_json::from_value(request.args).context("decode configureDns args")?;
            let interface_name = interface_name_or_default(args.interface_name);
            configure_dns(&interface_name, &args.dns_servers)?;
            persist_state(&state)?;
            Ok(json!({ "interfaceName": interface_name }))
        }
        "disableNetwork" => {
            disable_adapter(DEFAULT_INTERFACE_NAME)?;
            state.network_enabled = false;
            state.virtual_ip = None;
            persist_state(&state)?;
            Ok(json!({}))
        }
        "readRuntimeState" => {
            if let Ok(runtime_state) = read_windows_runtime_state(DEFAULT_INTERFACE_NAME) {
                *state = runtime_state;
                persist_state(&state)?;
            }
            Ok(serde_json::to_value(&*state)?)
        }
        other => bail!("unsupported helper method: {other}"),
    }
}

fn state_file_path() -> PathBuf {
    let base = app_data_dir();
    base.join("SLAN").join("client-v2-network-state.json")
}

fn app_data_dir() -> PathBuf {
    if cfg!(target_os = "windows") {
        return std::env::var_os("ProgramData")
            .map(PathBuf::from)
            .unwrap_or_else(|| PathBuf::from(r"C:\ProgramData"));
    }
    if cfg!(target_os = "macos") {
        return PathBuf::from("/Library/Application Support");
    }
    std::env::var_os("SLAN_STATE_DIR")
        .map(PathBuf::from)
        .unwrap_or_else(|| PathBuf::from("/var/lib"))
}

fn load_state() -> Result<NetworkRuntimeState> {
    let path = state_file_path();
    let payload = fs::read(&path).with_context(|| format!("read {}", path.display()))?;
    serde_json::from_slice(&payload).with_context(|| format!("decode {}", path.display()))
}

fn persist_state(state: &NetworkRuntimeState) -> Result<()> {
    let path = state_file_path();
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent).with_context(|| format!("create {}", parent.display()))?;
    }
    let payload = serde_json::to_vec_pretty(state).context("encode helper state")?;
    fs::write(&path, payload).with_context(|| format!("write {}", path.display()))
}

fn default_prefix_len() -> u8 {
    32
}

fn interface_name_or_default(value: Option<String>) -> String {
    value
        .filter(|value| !value.trim().is_empty())
        .unwrap_or_else(|| DEFAULT_INTERFACE_NAME.to_string())
}

fn ensure_adapter_present(interface_name: &str) -> Result<()> {
    let script = format!(
        "$name = '{}'; \
         $adapter = Get-NetAdapter -IncludeHidden -Name $name -ErrorAction SilentlyContinue; \
         if (-not $adapter) {{ throw \"SLAN local network adapter '$name' was not found. Please reinstall or repair SLAN Client.\" }}; \
         Enable-NetAdapter -Name $name -Confirm:$false -ErrorAction SilentlyContinue | Out-Null; \
         Write-Output $adapter.Name",
        escape_powershell_single_quoted(interface_name),
    );
    run_powershell(&script).map(|_| ())
}

fn configure_adapter_ip(interface_name: &str, virtual_ip: &str, prefix_len: u8) -> Result<()> {
    let virtual_ip = virtual_ip.trim();
    if virtual_ip.is_empty() || virtual_ip.eq_ignore_ascii_case("pending") {
        bail!("device unavailable: missing assigned virtual IP");
    }
    let script = format!(
        "$name = '{}'; \
         $ip = '{}'; \
         $prefix = {}; \
         $adapter = Get-NetAdapter -IncludeHidden -Name $name -ErrorAction SilentlyContinue; \
         if (-not $adapter) {{ throw \"SLAN local network adapter '$name' was not found. Please reinstall or repair SLAN Client.\" }}; \
         Enable-NetAdapter -Name $name -Confirm:$false -ErrorAction SilentlyContinue | Out-Null; \
         Get-NetIPAddress -InterfaceAlias $name -AddressFamily IPv4 -ErrorAction SilentlyContinue | Remove-NetIPAddress -Confirm:$false -ErrorAction SilentlyContinue; \
         New-NetIPAddress -InterfaceAlias $name -IPAddress $ip -PrefixLength $prefix -ErrorAction Stop | Out-Null; \
         Write-Output ($name + '|' + $ip)",
        escape_powershell_single_quoted(interface_name),
        escape_powershell_single_quoted(virtual_ip),
        prefix_len,
    );
    run_powershell(&script).map(|_| ())
}

fn configure_routes(interface_name: &str, routes: &[RouteArg]) -> Result<()> {
    if routes.is_empty() {
        return Ok(());
    }
    let mut script = format!(
        "$name = '{}'; \
         $adapter = Get-NetAdapter -IncludeHidden -Name $name -ErrorAction SilentlyContinue; \
         if (-not $adapter) {{ throw \"SLAN local network adapter '$name' was not found. Please reinstall or repair SLAN Client.\" }}; \
         $ifIndex = $adapter.ifIndex; ",
        escape_powershell_single_quoted(interface_name),
    );
    for route in routes {
        let destination = route.destination.trim();
        if destination.is_empty() || destination.eq_ignore_ascii_case("mesh") {
            continue;
        }
        let gateway = route.gateway.as_deref().unwrap_or("0.0.0.0").trim();
        script.push_str(&format!(
            "New-NetRoute -DestinationPrefix '{}' -InterfaceIndex $ifIndex -NextHop '{}' -PolicyStore ActiveStore -ErrorAction SilentlyContinue | Out-Null; ",
            escape_powershell_single_quoted(destination),
            escape_powershell_single_quoted(gateway),
        ));
    }
    run_powershell(&script).map(|_| ())
}

fn configure_dns(interface_name: &str, dns_servers: &[String]) -> Result<()> {
    let servers = dns_servers
        .iter()
        .map(|value| value.trim())
        .filter(|value| !value.is_empty())
        .map(|value| format!("'{}'", escape_powershell_single_quoted(value)))
        .collect::<Vec<_>>();
    let script = if servers.is_empty() {
        format!(
            "Set-DnsClientServerAddress -InterfaceAlias '{}' -ResetServerAddresses -ErrorAction SilentlyContinue",
            escape_powershell_single_quoted(interface_name),
        )
    } else {
        format!(
            "Set-DnsClientServerAddress -InterfaceAlias '{}' -ServerAddresses @({}) -ErrorAction Stop",
            escape_powershell_single_quoted(interface_name),
            servers.join(","),
        )
    };
    run_powershell(&script).map(|_| ())
}

fn disable_adapter(interface_name: &str) -> Result<()> {
    let script = format!(
        "$name = '{}'; \
         $adapter = Get-NetAdapter -IncludeHidden -Name $name -ErrorAction SilentlyContinue; \
         if ($adapter) {{ \
           Get-NetIPAddress -InterfaceAlias $name -AddressFamily IPv4 -ErrorAction SilentlyContinue | Remove-NetIPAddress -Confirm:$false -ErrorAction SilentlyContinue; \
           Disable-NetAdapter -Name $name -Confirm:$false -ErrorAction SilentlyContinue | Out-Null; \
         }}",
        escape_powershell_single_quoted(interface_name),
    );
    run_powershell(&script).map(|_| ())
}

fn read_windows_runtime_state(interface_name: &str) -> Result<NetworkRuntimeState> {
    let script = format!(
        "$name = '{}'; \
         $adapter = Get-NetAdapter -IncludeHidden -Name $name -ErrorAction SilentlyContinue; \
         if (-not $adapter) {{ Write-Output 'missing||'; exit 0 }}; \
         $ip = Get-NetIPAddress -InterfaceAlias $name -AddressFamily IPv4 -ErrorAction SilentlyContinue | Select-Object -First 1 -ExpandProperty IPAddress; \
         Write-Output ($adapter.Status + '|' + $adapter.Name + '|' + $ip)",
        escape_powershell_single_quoted(interface_name),
    );
    let output = run_powershell(&script)?;
    let mut parts = output.split('|');
    let status = parts.next().unwrap_or_default().trim();
    let name = parts.next().unwrap_or_default().trim();
    let ip = parts.next().unwrap_or_default().trim();
    if status.eq_ignore_ascii_case("missing") || name.is_empty() {
        return Ok(NetworkRuntimeState::default());
    }
    Ok(NetworkRuntimeState {
        adapter_present: true,
        network_enabled: status.eq_ignore_ascii_case("up") && !ip.is_empty(),
        virtual_ip: if ip.is_empty() {
            None
        } else {
            Some(ip.to_string())
        },
    })
}

fn run_powershell(script: &str) -> Result<String> {
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
        .context("run powershell network command")?;
    if output.status.success() {
        return Ok(String::from_utf8_lossy(&output.stdout).trim().to_string());
    }
    let stderr = String::from_utf8_lossy(&output.stderr).trim().to_string();
    let stdout = String::from_utf8_lossy(&output.stdout).trim().to_string();
    let detail = if stderr.is_empty() { stdout } else { stderr };
    bail!("windows network command failed: {detail}");
}

fn escape_powershell_single_quoted(value: &str) -> String {
    value.replace('\'', "''")
}
