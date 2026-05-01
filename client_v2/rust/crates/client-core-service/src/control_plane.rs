use std::{
    env,
    io::{Read, Write},
    net::TcpStream,
    time::Duration,
};

use anyhow::{bail, Context, Result};
use client_core::AuthPayload;
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
    name: String,
    platform: String,
    device_version: String,
    machine_id: String,
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
        let devices = self.list_devices(access_token).unwrap_or_default();
        if let Some(device_id) = preferred_device_id.filter(|value| !value.trim().is_empty()) {
            if let Some(device) = devices
                .iter()
                .find(|item| item.device_id == device_id)
                .cloned()
            {
                return Ok(device);
            }
        }
        if let Some(device) = devices.into_iter().next() {
            return Ok(device);
        }
        self.register_device(access_token)
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
    ) -> Result<Option<String>> {
        let body = serde_json::json!({ "deviceId": device_id });
        let path = format!("/networks/{network_id}/activate");
        let response = self.request_json("POST", &path, access_token, Some(body))?;
        Ok(response
            .get("attachment")
            .and_then(|attachment| attachment.get("virtualIp"))
            .and_then(Value::as_str)
            .map(str::to_string))
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

    fn register_device(&self, access_token: &str) -> Result<ControlDevice> {
        let machine_id = machine_id();
        let body = serde_json::to_value(RegisterDeviceRequest {
            name: machine_id.clone(),
            platform: platform_name().to_string(),
            device_version: env!("CARGO_PKG_VERSION").to_string(),
            machine_id: machine_id.clone(),
            public_key: format!("client-v2-{machine_id}"),
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

fn machine_id() -> String {
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
