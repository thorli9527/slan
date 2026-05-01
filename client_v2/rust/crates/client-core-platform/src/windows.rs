use std::{
    fs,
    io::{BufRead, BufReader, Write},
    net::TcpStream,
    path::PathBuf,
    process::{Command, Stdio},
    thread,
    time::Duration,
};

use anyhow::{bail, Context, Result};
use client_core::{NetworkRuntimeState, PlatformNetwork, RouteSpec};
use serde_json::{json, Value};

const DEFAULT_HELPER_HOST: &str = "127.0.0.1:46393";

#[derive(Debug, Clone, Default)]
pub struct WindowsPlatformNetwork;

impl PlatformNetwork for WindowsPlatformNetwork {
    fn install_adapter(&self) -> Result<()> {
        HelperClient::default().invoke_unit("installAdapter", json!({}))
    }

    fn configure_ip(&self, virtual_ip: &str) -> Result<()> {
        HelperClient::default().invoke_unit(
            "configureIp",
            json!({
                "virtualIp": virtual_ip,
            }),
        )
    }

    fn configure_routes(&self, routes: &[RouteSpec]) -> Result<()> {
        HelperClient::default().invoke_unit(
            "configureRoutes",
            json!({
                "routes": routes,
            }),
        )
    }

    fn configure_dns(&self, dns_servers: &[String]) -> Result<()> {
        HelperClient::default().invoke_unit(
            "configureDns",
            json!({
                "dnsServers": dns_servers,
            }),
        )
    }

    fn disable_network(&self) -> Result<()> {
        HelperClient::default().invoke_unit("disableNetwork", json!({}))
    }

    fn read_runtime_state(&self) -> Result<NetworkRuntimeState> {
        load_cached_runtime_state()
    }
}

#[derive(Debug, Clone)]
struct HelperClient {
    host: String,
}

impl Default for HelperClient {
    fn default() -> Self {
        Self {
            host: std::env::var("SLAN_CLIENT_CORE_HELPER_HOST")
                .unwrap_or_else(|_| DEFAULT_HELPER_HOST.to_string()),
        }
    }
}

impl HelperClient {
    fn invoke_unit(&self, method: &str, args: Value) -> Result<()> {
        self.invoke(method, args).map(|_| ())
    }

    fn invoke(&self, method: &str, args: Value) -> Result<Value> {
        match self.invoke_once(method, args.clone()) {
            Ok(value) => Ok(value),
            Err(first_error) => {
                self.start_bundled_helper();
                for _ in 0..15 {
                    thread::sleep(Duration::from_millis(100));
                    if let Ok(value) = self.invoke_once(method, args.clone()) {
                        return Ok(value);
                    }
                }
                Err(first_error)
            }
        }
    }

    fn invoke_once(&self, method: &str, args: Value) -> Result<Value> {
        let mut stream = TcpStream::connect(&self.host)
            .with_context(|| format!("connect client-core-helper at {}", self.host))?;
        let request = json!({
            "method": method,
            "args": args,
        });
        serde_json::to_writer(&mut stream, &request).context("encode helper request")?;
        stream
            .write_all(b"\n")
            .context("write helper request newline")?;
        stream.flush().context("flush helper request")?;

        let mut response_line = String::new();
        let mut reader = BufReader::new(stream);
        reader
            .read_line(&mut response_line)
            .context("read helper response")?;
        if response_line.trim().is_empty() {
            bail!("client-core-helper returned an empty response");
        }
        let response: Value =
            serde_json::from_str(response_line.trim()).context("decode helper response")?;
        if response.get("ok").and_then(Value::as_bool) == Some(true) {
            return Ok(response.get("data").cloned().unwrap_or_else(|| json!({})));
        }
        let message = response
            .get("error")
            .and_then(Value::as_str)
            .unwrap_or("client-core-helper failed");
        bail!("{message}");
    }

    fn start_bundled_helper(&self) {
        let Some(helper_path) = bundled_helper_path() else {
            return;
        };
        let _ = Command::new(helper_path)
            .stdin(Stdio::null())
            .stdout(Stdio::null())
            .stderr(Stdio::null())
            .spawn();
    }
}

fn bundled_helper_path() -> Option<PathBuf> {
    let current_exe = std::env::current_exe().ok()?;
    let dir = current_exe.parent()?;
    Some(dir.join("client-core-helper.exe"))
}

fn load_cached_runtime_state() -> Result<NetworkRuntimeState> {
    let path = state_file_path();
    if !path.exists() {
        return Ok(NetworkRuntimeState::default());
    }
    let payload = fs::read(&path).with_context(|| format!("read {}", path.display()))?;
    serde_json::from_slice(&payload).with_context(|| format!("decode {}", path.display()))
}

fn state_file_path() -> PathBuf {
    app_data_dir()
        .join("SLAN")
        .join("client-v2-network-state.json")
}

fn app_data_dir() -> PathBuf {
    std::env::var_os("ProgramData")
        .map(PathBuf::from)
        .unwrap_or_else(|| PathBuf::from(r"C:\ProgramData"))
}
