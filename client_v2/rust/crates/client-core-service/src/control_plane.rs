use std::{
    env, fs,
    io::{Read, Write},
    net::TcpStream,
    path::PathBuf,
    process::Command,
    time::Duration,
};

use anyhow::{bail, Context, Result};
use client_core::{AuthPayload, RouteSpec};
use serde::{Deserialize, Serialize};
use serde_json::Value;

const DEFAULT_CONTROL_BASE_URL: &str = "http://127.0.0.1:28080";

#[derive(Debug, Clone)]
pub struct ControlPlaneClient {
    base_url: String,
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ControlDevice {
    pub device_id: String,
    #[serde(default)]
    pub status: Option<String>,
    #[serde(default)]
    pub membership_status: Option<String>,
    #[serde(default)]
    pub current_virtual_ip: Option<String>,
    #[serde(default)]
    pub virtual_ip: Option<String>,
    #[serde(default)]
    pub mqtt: Option<MqttCredential>,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct MqttCredential {
    pub broker_url: String,
    pub client_id: String,
    pub username: String,
    pub password: String,
    pub topic_prefix: String,
    #[serde(default)]
    pub expires_at: Option<i64>,
}

#[derive(Debug, Clone, Default)]
pub struct NetworkActivationPlan {
    pub virtual_ip: String,
    pub prefix_len: u8,
    pub dns_servers: Vec<String>,
    pub routes: Vec<RouteSpec>,
    pub relay_candidates: Vec<RelayCandidate>,
    pub peer_count: usize,
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct RelayCandidate {
    pub endpoint_id: String,
    pub transport: String,
    pub address: String,
    #[serde(default)]
    pub country_code: Option<String>,
    #[serde(default)]
    pub region_id: Option<String>,
    #[serde(default)]
    pub cluster_id: Option<String>,
}

#[derive(Debug, Deserialize)]
struct ItemsResponse<T> {
    items: Vec<T>,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
struct NetworkHomeResponse {
    active_network: Option<ControlNetwork>,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
struct ControlNetwork {
    network_id: String,
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
struct ControlSubnet {
    subnet_id: String,
    cidr: String,
    #[serde(default)]
    is_default: bool,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
struct CallbackStatusResponse {
    ready: bool,
    payload: Option<CallbackPayload>,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
struct CallbackPayload {
    access_token: String,
    refresh_token: Option<String>,
    user_id: String,
    user_label: Option<String>,
    device_id: Option<String>,
    expires_in: Option<u64>,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
struct RegisterDeviceRequest {
    device_id: String,
    name: String,
    platform: String,
    device_version: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    country_code: Option<String>,
    public_key: String,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
struct DeviceNetworkStateRequest<'a> {
    device_id: &'a str,
    network_id: &'a str,
    control_reachable: bool,
    network_online: bool,
    tunnel_up: bool,
    last_probe_ok: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    virtual_ip: Option<&'a str>,
    reported_at: i64,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
struct ConsoleLoginKeyRequest<'a> {
    #[serde(skip_serializing_if = "Option::is_none")]
    device_id: Option<&'a str>,
}

impl ControlPlaneClient {
    pub fn from_env() -> Self {
        Self {
            base_url: env::var("SLAN_CONTROL_BASE_URL")
                .unwrap_or_else(|_| DEFAULT_CONTROL_BASE_URL.to_string()),
        }
    }

    pub fn ensure_device(
        &self,
        access_token: &str,
        preferred_device_id: Option<&str>,
    ) -> Result<ControlDevice> {
        let stable_device_id = stable_device_id(preferred_device_id)?;
        let devices = self.list_devices(access_token).unwrap_or_default();
        let preferred_device_id = stable_device_id.as_str();
        if let Some(device) = devices
            .iter()
            .find(|item| item.device_id == preferred_device_id)
            .cloned()
        {
            if device.mqtt.is_some() {
                return Ok(device);
            }
            return self.register_device(access_token, preferred_device_id);
        }
        self.register_device(access_token, preferred_device_id)
    }

    pub fn list_devices(&self, access_token: &str) -> Result<Vec<ControlDevice>> {
        let response = self.request_json("GET", "/devices", access_token, None)?;
        let payload: ItemsResponse<ControlDevice> =
            serde_json::from_value(response).context("decode device list")?;
        Ok(payload.items)
    }

    pub fn callback_payload(&self, callback_id: &str) -> Result<Option<AuthPayload>> {
        let path = format!("/auth/callback-status/{callback_id}");
        let response = self.request_json_without_auth("GET", &path, None)?;
        let payload: CallbackStatusResponse =
            serde_json::from_value(response).context("decode callback status")?;
        if !payload.ready {
            return Ok(None);
        }
        let Some(payload) = payload.payload else {
            return Ok(None);
        };
        Ok(Some(AuthPayload {
            access_token: payload.access_token,
            refresh_token: payload.refresh_token,
            user_id: payload.user_id,
            user_label: payload.user_label.unwrap_or_default(),
            device_id: payload.device_id,
            virtual_ip: None,
            expires_in: payload.expires_in,
        }))
    }

    pub fn active_network_id(&self, access_token: &str) -> Result<Option<String>> {
        let response = self.request_json("GET", "/networks/home", access_token, None)?;
        let payload: NetworkHomeResponse =
            serde_json::from_value(response).context("decode network home")?;
        Ok(payload.active_network.map(|network| network.network_id))
    }

    pub fn report_network_state(
        &self,
        access_token: &str,
        device_id: &str,
        network_id: &str,
        network_enabled: bool,
        virtual_ip: Option<&str>,
    ) -> Result<()> {
        let body = serde_json::to_value(DeviceNetworkStateRequest {
            device_id,
            network_id,
            control_reachable: true,
            network_online: network_enabled,
            tunnel_up: network_enabled,
            last_probe_ok: network_enabled,
            virtual_ip,
            reported_at: current_timestamp_seconds(),
        })?;
        let path = format!("/devices/{device_id}/networks/{network_id}/state");
        self.request_json("PUT", &path, access_token, Some(body))?;
        Ok(())
    }

    pub fn activate_network(
        &self,
        access_token: &str,
        device_id: &str,
        network_id: &str,
    ) -> Result<NetworkActivationPlan> {
        let body = serde_json::json!({ "deviceId": device_id });
        let path = format!("/networks/{network_id}/activate");
        let response = self.request_json("POST", &path, access_token, Some(body))?;
        let attachment = response.get("attachment");
        let virtual_ip = attachment
            .and_then(|attachment| attachment.get("virtualIp"))
            .and_then(Value::as_str)
            .filter(|value| !value.trim().is_empty())
            .map(str::to_string)
            .ok_or_else(|| {
                anyhow::anyhow!("device unavailable: current device has no assigned virtual IP")
            })?;
        let subnet_id = attachment
            .and_then(|attachment| attachment.get("subnetId"))
            .and_then(Value::as_str)
            .filter(|value| !value.trim().is_empty());
        Ok(NetworkActivationPlan {
            virtual_ip,
            prefix_len: self.network_prefix_len(access_token, network_id, subnet_id)?,
            dns_servers: extract_dns_servers(&response),
            routes: extract_routes(&response),
            relay_candidates: extract_relay_candidates(&response),
            peer_count: extract_peer_count(&response),
        })
    }

    pub fn relay_candidates(
        &self,
        access_token: &str,
        device_id: &str,
        network_id: &str,
    ) -> Result<Vec<RelayCandidate>> {
        let body = serde_json::json!({ "deviceId": device_id });
        let path = format!("/networks/{network_id}/relay-candidates");
        let response = self.request_json("POST", &path, access_token, Some(body))?;
        Ok(extract_relay_candidates(&response))
    }

    pub fn deactivate_network(
        &self,
        access_token: &str,
        device_id: &str,
        network_id: &str,
    ) -> Result<()> {
        let body = serde_json::json!({ "deviceId": device_id });
        let path = format!("/networks/{network_id}/deactivate");
        self.request_json("POST", &path, access_token, Some(body))?;
        Ok(())
    }

    pub fn network_prefix_len(
        &self,
        access_token: &str,
        network_id: &str,
        subnet_id: Option<&str>,
    ) -> Result<u8> {
        let path = format!("/networks/{network_id}/subnets");
        let response = self.request_json("GET", &path, access_token, None)?;
        let payload: ItemsResponse<ControlSubnet> =
            serde_json::from_value(response).context("decode network subnets")?;
        let cidr = payload
            .items
            .iter()
            .find(|subnet| subnet_id == Some(subnet.subnet_id.as_str()))
            .or_else(|| payload.items.iter().find(|subnet| subnet.is_default))
            .or_else(|| payload.items.first())
            .map(|subnet| subnet.cidr.as_str())
            .ok_or_else(|| anyhow::anyhow!("network has no subnet cidr"))?;
        prefix_len_from_cidr(cidr)
    }

    pub fn console_login_key(
        &self,
        access_token: &str,
        device_id: Option<&str>,
    ) -> Result<Option<String>> {
        let body = serde_json::to_value(ConsoleLoginKeyRequest { device_id })?;
        let response =
            self.request_json("POST", "/auth/console-login-key", access_token, Some(body))?;
        Ok(response
            .get("loginKey")
            .and_then(Value::as_str)
            .filter(|value| !value.trim().is_empty())
            .map(str::to_string))
    }

    fn register_device(&self, access_token: &str, device_id: &str) -> Result<ControlDevice> {
        let device_id = device_id.trim();
        let device_name = device_name();
        let body = serde_json::to_value(RegisterDeviceRequest {
            device_id: device_id.to_string(),
            name: device_name,
            platform: platform_name().to_string(),
            device_version: env!("CARGO_PKG_VERSION").to_string(),
            country_code: device_country_code(),
            public_key: format!("client-v2-{device_id}"),
        })?;
        let response = self.request_json("POST", "/devices/register", access_token, Some(body))?;
        serde_json::from_value(response).context("decode registered device")
    }

    fn request_json(
        &self,
        method: &str,
        path: &str,
        access_token: &str,
        body: Option<Value>,
    ) -> Result<Value> {
        let endpoint = HttpEndpoint::parse(&self.base_url)?;
        let body = body
            .map(|value| serde_json::to_vec(&value))
            .transpose()
            .context("encode control request")?
            .unwrap_or_default();
        let response = endpoint.request(method, path, access_token, &body)?;
        serde_json::from_slice(&response).context("decode control response")
    }

    fn request_json_without_auth(
        &self,
        method: &str,
        path: &str,
        body: Option<Value>,
    ) -> Result<Value> {
        let endpoint = HttpEndpoint::parse(&self.base_url)?;
        let body = body
            .map(|value| serde_json::to_vec(&value))
            .transpose()
            .context("encode control request")?
            .unwrap_or_default();
        let response = endpoint.request(method, path, "", &body)?;
        serde_json::from_slice(&response).context("decode control response")
    }
}

fn extract_dns_servers(response: &Value) -> Vec<String> {
    response
        .pointer("/networkMap/dns/servers")
        .or_else(|| response.pointer("/dns/servers"))
        .and_then(Value::as_array)
        .into_iter()
        .flatten()
        .filter_map(Value::as_str)
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .map(str::to_string)
        .collect()
}

fn extract_routes(response: &Value) -> Vec<RouteSpec> {
    response
        .pointer("/networkMap/routes")
        .or_else(|| response.pointer("/routes"))
        .and_then(Value::as_array)
        .into_iter()
        .flatten()
        .filter_map(|route| {
            let destination = route
                .get("cidr")
                .or_else(|| route.get("destination"))
                .and_then(Value::as_str)?
                .trim()
                .to_string();
            if destination.is_empty() {
                return None;
            }
            Some(RouteSpec {
                destination,
                gateway: route
                    .get("gateway")
                    .and_then(Value::as_str)
                    .map(str::trim)
                    .filter(|value| !value.is_empty())
                    .map(str::to_string),
            })
        })
        .collect()
}

fn extract_peer_count(response: &Value) -> usize {
    response
        .pointer("/networkMap/peers")
        .or_else(|| response.pointer("/peers"))
        .and_then(Value::as_array)
        .map(Vec::len)
        .unwrap_or_default()
}

fn extract_relay_candidates(response: &Value) -> Vec<RelayCandidate> {
    response
        .pointer("/networkMap/relayRegions")
        .or_else(|| response.pointer("/relayRegions"))
        .and_then(Value::as_array)
        .into_iter()
        .flatten()
        .flat_map(|region| {
            let country_code = region
                .get("countryCode")
                .and_then(Value::as_str)
                .map(str::trim)
                .filter(|value| !value.is_empty())
                .map(str::to_string);
            let region_id = region
                .get("regionId")
                .and_then(Value::as_str)
                .map(str::trim)
                .filter(|value| !value.is_empty())
                .map(str::to_string);
            let cluster_id = region
                .get("clusterId")
                .and_then(Value::as_str)
                .map(str::trim)
                .filter(|value| !value.is_empty())
                .map(str::to_string);
            region
                .get("endpoints")
                .and_then(Value::as_array)
                .into_iter()
                .flatten()
                .filter_map(move |endpoint| {
                    let endpoint_id = endpoint
                        .get("endpointId")
                        .and_then(Value::as_str)?
                        .trim()
                        .to_string();
                    let transport = endpoint
                        .get("transport")
                        .and_then(Value::as_str)?
                        .trim()
                        .to_string();
                    let address = endpoint
                        .get("address")
                        .and_then(Value::as_str)?
                        .trim()
                        .to_string();
                    if endpoint_id.is_empty() || transport.is_empty() || address.is_empty() {
                        return None;
                    }
                    Some(RelayCandidate {
                        endpoint_id,
                        transport,
                        address,
                        country_code: country_code.clone(),
                        region_id: region_id.clone(),
                        cluster_id: cluster_id.clone(),
                    })
                })
                .collect::<Vec<_>>()
        })
        .collect()
}

fn device_country_code() -> Option<String> {
    env::var("SLAN_DEVICE_COUNTRY_CODE")
        .ok()
        .map(|value| value.trim().to_ascii_uppercase())
        .filter(|value| !value.is_empty())
}

fn prefix_len_from_cidr(cidr: &str) -> Result<u8> {
    let (_, prefix) = cidr
        .trim()
        .split_once('/')
        .ok_or_else(|| anyhow::anyhow!("invalid subnet cidr: {cidr}"))?;
    let prefix_len = prefix
        .parse::<u8>()
        .with_context(|| format!("parse subnet prefix from {cidr}"))?;
    if prefix_len > 32 {
        bail!("invalid IPv4 subnet prefix length {prefix_len} from {cidr}");
    }
    Ok(prefix_len)
}

#[derive(Debug, Clone)]
struct HttpEndpoint {
    host: String,
    port: u16,
    base_path: String,
}

impl HttpEndpoint {
    fn parse(base_url: &str) -> Result<Self> {
        let base_url = base_url.trim().trim_end_matches('/');
        let rest = base_url.strip_prefix("http://").ok_or_else(|| {
            anyhow::anyhow!("only http control base url is supported in client_v2 bootstrap")
        })?;
        let (authority, base_path) = rest.split_once('/').unwrap_or((rest, ""));
        let (host, port) = match authority.rsplit_once(':') {
            Some((host, port)) => {
                let port = port.parse::<u16>().context("parse control port")?;
                (host.to_string(), port)
            }
            None => (authority.to_string(), 80),
        };
        if host.is_empty() {
            bail!("control base url host is empty");
        }
        Ok(Self {
            host,
            port,
            base_path: if base_path.is_empty() {
                String::new()
            } else {
                format!("/{base_path}")
            },
        })
    }

    fn request(
        &self,
        method: &str,
        path: &str,
        access_token: &str,
        body: &[u8],
    ) -> Result<Vec<u8>> {
        let mut stream = TcpStream::connect((self.host.as_str(), self.port))
            .with_context(|| format!("connect control plane {}:{}", self.host, self.port))?;
        stream.set_read_timeout(Some(Duration::from_secs(10))).ok();
        stream.set_write_timeout(Some(Duration::from_secs(10))).ok();

        let target = format!("{}{}", self.base_path, path);
        let request = format!(
            "{method} {target} HTTP/1.1\r\nHost: {host}\r\n{auth_header}Content-Type: application/json\r\nContent-Length: {length}\r\nConnection: close\r\n\r\n",
            method = method,
            target = target,
            host = self.host,
            auth_header = if access_token.trim().is_empty() {
                String::new()
            } else {
                format!("Authorization: Bearer {access_token}\r\n")
            },
            length = body.len(),
        );
        stream
            .write_all(request.as_bytes())
            .context("write control request headers")?;
        if !body.is_empty() {
            stream
                .write_all(body)
                .context("write control request body")?;
        }
        stream.flush().context("flush control request")?;

        let mut response = Vec::new();
        stream
            .read_to_end(&mut response)
            .context("read control response")?;
        decode_http_response(&response)
    }
}

fn decode_http_response(response: &[u8]) -> Result<Vec<u8>> {
    let separator = response
        .windows(4)
        .position(|window| window == b"\r\n\r\n")
        .ok_or_else(|| anyhow::anyhow!("invalid control response"))?;
    let headers = String::from_utf8_lossy(&response[..separator]);
    let status_line = headers.lines().next().unwrap_or_default();
    let status = status_line
        .split_whitespace()
        .nth(1)
        .and_then(|value| value.parse::<u16>().ok())
        .unwrap_or_default();
    let body = response[separator + 4..].to_vec();
    if (200..300).contains(&status) {
        return Ok(body);
    }
    let detail = serde_json::from_slice::<Value>(&body)
        .ok()
        .and_then(|value| {
            value
                .get("message")
                .or_else(|| value.get("error"))
                .and_then(Value::as_str)
                .map(str::to_string)
        })
        .unwrap_or_else(|| String::from_utf8_lossy(&body).to_string());
    if status == 403 || detail.to_ascii_lowercase().contains("disabled") {
        bail!("device unavailable: current device is disabled by network administrator");
    }
    bail!("control plane returned HTTP {status}: {detail}");
}

fn stable_device_id(preferred_device_id: Option<&str>) -> Result<String> {
    let path = state_dir().join("client-v2-device-id.txt");
    let deterministic = deterministic_device_id();
    if let Ok(value) = fs::read_to_string(&path) {
        let value = value.trim();
        if deterministic.as_deref().ok() == Some(value) && is_usable_device_id(value) {
            return Ok(value.to_string());
        }
    }
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent).with_context(|| format!("create {}", parent.display()))?;
    }
    let created = deterministic
        .ok()
        .filter(|value| is_usable_device_id(value))
        .or_else(|| {
            preferred_device_id
                .map(str::trim)
                .filter(|value| is_usable_device_id(value))
                .map(str::to_string)
        })
        .unwrap_or_else(|| deterministic_device_id_from_anchor(&fallback_device_anchor()));
    fs::write(&path, &created).with_context(|| format!("write {}", path.display()))?;
    Ok(created)
}

pub fn local_stable_device_id() -> Result<String> {
    stable_device_id(None)
}

fn deterministic_device_id() -> Result<String> {
    let anchor = machine_anchor().unwrap_or_else(fallback_device_anchor);
    Ok(deterministic_device_id_from_anchor(&anchor))
}

fn deterministic_device_id_from_anchor(anchor: &str) -> String {
    let normalized = anchor.trim().to_ascii_lowercase();
    format!("dev-{:016x}", fnv1a64(normalized.as_bytes()))
}

fn machine_anchor() -> Option<String> {
    if cfg!(target_os = "windows") {
        return windows_machine_guid();
    }
    if cfg!(target_os = "macos") {
        return macos_platform_uuid();
    }
    linux_machine_id()
}

fn windows_machine_guid() -> Option<String> {
    let output = Command::new("reg.exe")
        .args([
            "query",
            r"HKLM\SOFTWARE\Microsoft\Cryptography",
            "/v",
            "MachineGuid",
        ])
        .output()
        .ok()?;
    if !output.status.success() {
        return None;
    }
    let text = String::from_utf8_lossy(&output.stdout);
    text.lines()
        .find_map(|line| {
            let parts: Vec<_> = line.split_whitespace().collect();
            if parts.len() >= 3 && parts[0].eq_ignore_ascii_case("MachineGuid") {
                return Some(parts[2..].join(""));
            }
            None
        })
        .filter(|value| !value.trim().is_empty())
        .map(|value| format!("windows:{value}"))
}

fn macos_platform_uuid() -> Option<String> {
    let output = Command::new("ioreg")
        .args(["-rd1", "-c", "IOPlatformExpertDevice"])
        .output()
        .ok()?;
    if !output.status.success() {
        return None;
    }
    let text = String::from_utf8_lossy(&output.stdout);
    text.lines()
        .find_map(|line| {
            let (_, value) = line.split_once("IOPlatformUUID")?;
            value
                .split('"')
                .nth(1)
                .map(str::trim)
                .filter(|value| !value.is_empty())
                .map(str::to_string)
        })
        .map(|value| format!("macos:{value}"))
}

fn linux_machine_id() -> Option<String> {
    ["/etc/machine-id", "/var/lib/dbus/machine-id"]
        .iter()
        .find_map(|path| fs::read_to_string(path).ok())
        .map(|value| value.trim().to_string())
        .filter(|value| !value.is_empty())
        .map(|value| format!("linux:{value}"))
}

fn fallback_device_anchor() -> String {
    format!("{}:{}", platform_name(), device_name())
}

fn fnv1a64(bytes: &[u8]) -> u64 {
    let mut hash = 0xcbf29ce484222325u64;
    for byte in bytes {
        hash ^= u64::from(*byte);
        hash = hash.wrapping_mul(0x100000001b3);
    }
    hash
}

fn is_usable_device_id(device_id: &str) -> bool {
    let value = device_id.trim();
    if value.is_empty() {
        return false;
    }
    let lower = value.to_ascii_lowercase();
    !matches!(lower.as_str(), "authcallbackid" | "windows-plugin-login")
        && !lower.starts_with("cb-")
}

fn state_dir() -> PathBuf {
    if cfg!(target_os = "windows") {
        return env::var_os("ProgramData")
            .map(PathBuf::from)
            .unwrap_or_else(|| PathBuf::from(r"C:\ProgramData"))
            .join("SLAN");
    }
    if cfg!(target_os = "macos") {
        return PathBuf::from("/Library/Application Support/SLAN");
    }
    env::var_os("SLAN_STATE_DIR")
        .map(PathBuf::from)
        .unwrap_or_else(|| PathBuf::from("/var/lib"))
        .join("SLAN")
}

fn device_name() -> String {
    env::var("COMPUTERNAME")
        .or_else(|_| env::var("HOSTNAME"))
        .unwrap_or_else(|_| "slan-client-v2".to_string())
        .trim()
        .to_string()
}

fn platform_name() -> &'static str {
    if cfg!(target_os = "windows") {
        "windows"
    } else if cfg!(target_os = "macos") {
        "macos"
    } else if cfg!(target_os = "linux") {
        "linux"
    } else if cfg!(target_os = "android") {
        "android"
    } else if cfg!(target_os = "ios") {
        "ios"
    } else {
        "unknown"
    }
}

fn current_timestamp_seconds() -> i64 {
    std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .map(|duration| duration.as_secs() as i64)
        .unwrap_or_default()
}
