use std::{
    env, fs,
    io::{ErrorKind, Read, Write},
    net::TcpStream,
    path::PathBuf,
    sync::{Mutex, OnceLock},
    thread,
    time::{Duration, Instant},
};

use anyhow::{bail, Context, Result};
use client_core::{normalize_virtual_ip, AuthPayload, RelayTicket, RouteSpec};
use serde::{Deserialize, Serialize};
use serde_json::Value;

const DEFAULT_CONTROL_BASE_URL: &str = "http://47.245.40.231:28080";
const API_AUTH_DEVICE_LOGIN_DEVICES: &str = "/api/auth/device-login-devices";
const API_AUTH_LOGIN: &str = "/api/auth/login";
const API_AUTH_LOGOUT: &str = "/api/auth/logout";
const API_AUTH_RENEW: &str = "/api/auth/renew";
const API_AUTH_CONSOLE_LOGIN_KEYS: &str = "/api/auth/console-login-keys";
const API_DEVICE_SESSION_BOOTSTRAP: &str = "/api/device/session/bootstrap";
const API_DEVICE_SESSION_BIND: &str = "/api/device/session/bind";
const API_DEVICE_SESSION_RENEW: &str = "/api/device/session/renew";
const API_DEVICES: &str = "/api/devices";
const API_RELAY_TICKETS: &str = "/api/relay/tickets";
static CONTROL_BASE_URL_OVERRIDE: OnceLock<Mutex<Option<String>>> = OnceLock::new();
static CLIENT_DEVICE_ID_OVERRIDE: OnceLock<Mutex<Option<String>>> = OnceLock::new();

fn api_device_network_configs(device_id: &str) -> String {
    format!("/api/devices/{}/network-configs", device_id.trim())
}

fn api_network_config(network_id: &str, device_id: &str) -> String {
    format!(
        "/api/networks/{}/network-config?deviceId={}",
        network_id.trim(),
        device_id.trim()
    )
}

fn api_relay_candidates(network_id: &str, device_id: &str) -> String {
    format!(
        "/api/networks/{}/relay-candidates?deviceId={}",
        network_id.trim(),
        device_id.trim()
    )
}

fn api_punch_connect_sessions(network_id: &str) -> String {
    format!("/api/networks/{}/punch/connect-sessions", network_id.trim())
}

#[allow(dead_code)]
pub fn set_control_base_url_override(value: &str) {
    let value = value.trim();
    if value.is_empty() {
        return;
    }
    let mutex = CONTROL_BASE_URL_OVERRIDE.get_or_init(|| Mutex::new(None));
    let mut override_value = mutex
        .lock()
        .expect("control base url override mutex poisoned");
    *override_value = Some(value.trim_end_matches('/').to_string());
}

#[allow(dead_code)]
pub fn set_client_device_id_override(value: &str) {
    let value = value.trim();
    if !is_uuid_like(value) {
        return;
    }
    let mutex = CLIENT_DEVICE_ID_OVERRIDE.get_or_init(|| Mutex::new(None));
    let mut override_value = mutex
        .lock()
        .expect("client device id override mutex poisoned");
    *override_value = Some(value.to_string());
}

fn control_base_url_override() -> Option<String> {
    CONTROL_BASE_URL_OVERRIDE
        .get()
        .and_then(|mutex| mutex.lock().ok().and_then(|value| value.clone()))
}

/// ControlPlaneClient 封装客户端访问 service-biz 控制面的 HTTP 调用。
#[derive(Debug, Clone)]
pub struct ControlPlaneClient {
    base_url: String,
}

/// ControlDevice 是控制面返回的设备视图。
#[derive(Debug, Clone, Deserialize, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ControlDevice {
    pub device_id: String,
    #[serde(default)]
    pub active_network_id: Option<String>,
    #[serde(default)]
    pub owner_id: Option<String>,
    #[serde(default)]
    pub owner_email: Option<String>,
    #[serde(default)]
    pub status: Option<String>,
    #[serde(default)]
    pub membership_status: Option<String>,
    #[serde(default)]
    pub current_virtual_ip: Option<String>,
    #[serde(default)]
    pub virtual_ip: Option<String>,
    #[serde(default)]
    pub global_ip: Option<String>,
    #[serde(default)]
    pub global_name: Option<String>,
    #[serde(default)]
    pub mqtt: Option<MqttCredential>,
}

/// MqttCredential 是服务端下发给设备的 MQTT 连接凭据。
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

/// PunchAuthHeaders 是访问 punch connect-session 接口所需的设备签名头。
#[derive(Debug, Clone)]
struct PunchAuthHeaders {
    device_id: String,
    mqtt_username: String,
    signature: String,
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
struct ConsoleLoginKeyResponse {
    login_key: String,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct DeviceSessionResponse {
    pub device: ControlDevice,
    pub device_session: DeviceSessionPayload,
    #[serde(default)]
    pub mqtt: Option<MqttCredential>,
    #[serde(default)]
    pub network_configs: Option<ItemsResponse<DeviceNetworkConfig>>,
    #[serde(default)]
    pub runtime_endpoints: Option<RuntimeEndpointsResponse>,
}

/// RuntimeEndpointsResponse 是登录/续租响应里的运行端点总表。
#[derive(Debug, Clone, Default, Deserialize, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct RuntimeEndpointsResponse {
    #[serde(default)]
    pub mqtt: Option<MqttCredential>,
    #[serde(default)]
    pub punch_nodes: Vec<RuntimePunchNode>,
    #[serde(default)]
    pub relay_candidates: Vec<RelayCandidate>,
    #[serde(default)]
    pub networks: Vec<RuntimeNetworkEndpoint>,
    #[serde(default)]
    pub refreshed_at: i64,
}

/// RuntimePunchNode 是 UDP 打洞服务的公网访问地址。
#[derive(Debug, Clone, Default, Deserialize, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct RuntimePunchNode {
    #[serde(default)]
    pub node_id: String,
    #[serde(default)]
    pub name: Option<String>,
    #[serde(default)]
    pub region: Option<String>,
    #[serde(default)]
    pub address: String,
    #[serde(default)]
    pub public_udp_ip: String,
    #[serde(default)]
    pub public_udp_port: u16,
}

/// RuntimeNetworkEndpoint 是单个虚拟网络可用的 relay 候选。
#[derive(Debug, Clone, Default, Deserialize, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct RuntimeNetworkEndpoint {
    pub network_id: String,
    #[serde(default)]
    pub relay_candidates: Vec<RelayCandidate>,
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct DeviceSessionPayload {
    pub session_id: String,
    pub device_id: String,
    pub user_id: Option<String>,
    pub device_token: String,
    pub device_token_expires_at: i64,
    #[serde(default)]
    pub device_refresh_token: Option<String>,
    pub active_network_ids: Vec<String>,
}

/// NetworkActivationPlan 是启用虚拟网络前由控制面配置转换出的本地执行计划。
#[derive(Debug, Clone, Default)]
pub struct NetworkActivationPlan {
    pub virtual_ip: String,
    pub prefix_len: u8,
    pub dns_servers: Vec<String>,
    pub routes: Vec<RouteSpec>,
    pub relay_candidates: Vec<RelayCandidate>,
    pub self_node_id: Option<String>,
    pub peers: Vec<ControlPeer>,
    pub peer_count: usize,
}

/// RelayCandidate 是服务端下发的 relay 候选节点。
#[derive(Debug, Clone, Deserialize, Serialize)]
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

/// ControlPeer 是网络配置中可与本机通信的 peer 摘要。
#[derive(Debug, Clone, Default, Deserialize, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ControlPeer {
    pub node_id: String,
    #[serde(default)]
    pub relay_allowed: bool,
    #[serde(default)]
    pub virtual_ips: Vec<String>,
    #[serde(default)]
    pub endpoints: Vec<ControlEndpoint>,
}

/// ControlEndpoint 是 peer 的直连候选端点。
#[derive(Debug, Clone, Default, Deserialize, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct ControlEndpoint {
    #[serde(default)]
    #[serde(rename = "type")]
    pub endpoint_type: String,
    pub address: String,
    #[serde(default)]
    pub updated_at: i64,
}

#[derive(Debug, Clone, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct ControlNode {
    pub node_id: String,
}

#[derive(Debug, Deserialize)]
pub(crate) struct ItemsResponse<T> {
    pub(crate) items: Vec<T>,
}

/// DeviceNetworkConfig 是 service-biz 下发给设备的数据面配置。
#[derive(Debug, Clone, Default, Deserialize, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct DeviceNetworkConfig {
    pub network_id: String,
    #[serde(default)]
    pub network_name: Option<String>,
    #[serde(default)]
    pub network_code: Option<String>,
    #[serde(default)]
    pub config_version: Option<i64>,
    pub device_id: String,
    #[serde(default)]
    pub global_ip: Option<String>,
    #[serde(default)]
    pub global_name: Option<String>,
    #[serde(default)]
    pub peers: Vec<DeviceNetworkPeer>,
    #[serde(default)]
    pub security_groups: Vec<DeviceSecurityGroup>,
    #[serde(default)]
    pub rules: Vec<DeviceSecurityRule>,
    #[serde(default)]
    pub dns_zones: Vec<DeviceDnsZone>,
    #[serde(default)]
    pub dns_records: Vec<DeviceDnsRecord>,
    #[serde(default)]
    pub relay_candidates: Vec<RelayCandidate>,
}

/// DeviceNetworkPeer 是同一虚拟网络内的对端设备摘要。
#[derive(Debug, Clone, Default, Deserialize, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct DeviceNetworkPeer {
    pub device_id: String,
    #[serde(default)]
    pub owner_id: Option<String>,
    #[serde(default)]
    pub owner_email: Option<String>,
    #[serde(default)]
    pub alias: Option<String>,
    #[serde(default)]
    pub global_ip: Option<String>,
    #[serde(default)]
    pub global_name: Option<String>,
    #[serde(default)]
    pub status: Option<String>,
}

/// DeviceSecurityGroup 是下发给设备的安全组摘要。
#[derive(Debug, Clone, Default, Deserialize, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct DeviceSecurityGroup {
    pub security_group_id: String,
    pub network_id: String,
    pub name: String,
}

/// DeviceSecurityRule 是下发给设备的数据面访问控制规则。
#[derive(Debug, Clone, Default, Deserialize, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct DeviceSecurityRule {
    pub rule_id: String,
    pub security_group_id: String,
    pub direction: String,
    pub priority: i64,
    pub action: String,
    pub protocol: String,
    pub port_from: i64,
    pub port_to: i64,
    pub peer_type: String,
    pub peer_value: String,
    #[serde(default)]
    pub enabled: bool,
}

/// DeviceDnsZone 是下发给设备的 DNS Zone。
#[derive(Debug, Clone, Default, Deserialize, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct DeviceDnsZone {
    pub zone_id: String,
    pub network_id: String,
    pub zone_name: String,
}

/// DeviceDnsRecord 是下发给设备的 DNS 解析记录。
#[derive(Debug, Clone, Default, Deserialize, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct DeviceDnsRecord {
    pub record_id: String,
    pub zone_id: String,
    pub network_id: String,
    pub name: String,
    #[serde(default)]
    pub fqdn: Option<String>,
    pub record_type: String,
    #[serde(default)]
    pub target_device_id: Option<String>,
    #[serde(default)]
    pub target_ip: Option<String>,
    #[serde(default)]
    pub cname: Option<String>,
    #[serde(default)]
    pub port: Option<String>,
    #[serde(default)]
    pub ttl: Option<i64>,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct DeviceLoginPrepareResponse {
    pub device_id: String,
    #[serde(default)]
    pub mqtt: Option<MqttCredential>,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
struct RegisterDeviceRequest {
    device_id: String,
    name: String,
    platform: String,
    os_name: String,
    os_version: String,
    alias: String,
    device_version: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    country_code: Option<String>,
    public_key: String,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
struct PasswordLoginRequest<'a> {
    email: &'a str,
    password: &'a str,
    #[serde(skip_serializing_if = "Option::is_none")]
    device_id: Option<&'a str>,
}

/// RelayTicketRequest 是客户端向 biz 申请 relay/DERP 票据的请求体。
#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
struct RelayTicketRequest<'a> {
    network_id: &'a str,
    src_node_id: &'a str,
    dst_node_id: &'a str,
    #[serde(skip_serializing_if = "Option::is_none")]
    derp_cluster_id: Option<&'a str>,
    #[serde(skip_serializing_if = "Vec::is_empty")]
    preferred_derp_node_ids: Vec<&'a str>,
    #[serde(skip_serializing_if = "Vec::is_empty")]
    preferred_relay_endpoint_ids: Vec<&'a str>,
    reason: &'a str,
    #[serde(skip_serializing_if = "Option::is_none")]
    relay_region_id: Option<&'a str>,
}

/// PunchConnectSessionRequest 是客户端向 biz 申请 punch 协商会话的请求体。
#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
struct PunchConnectSessionRequest<'a> {
    requester_node_id: &'a str,
    peer_node_id: &'a str,
    #[serde(skip_serializing_if = "Option::is_none")]
    ttl_seconds: Option<u32>,
}

/// PunchEndpoint 是 punch-service 返回的单端端点快照。
#[derive(Debug, Clone, Default, Deserialize, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct PunchEndpoint {
    #[serde(default)]
    pub network_id: String,
    #[serde(default)]
    pub node_id: String,
    #[serde(default, rename = "type")]
    pub endpoint_type: String,
    #[serde(default)]
    pub address: String,
    #[serde(default)]
    pub reflexive: String,
    #[serde(default)]
    pub nat_type: String,
}

/// PunchConnectSession 是 punch-service 返回的一次 P2P 直连协商会话。
#[derive(Debug, Clone, Default, Deserialize, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct PunchConnectSession {
    pub session_id: String,
    pub network_id: String,
    pub requester_node_id: String,
    pub peer_node_id: String,
    #[serde(default)]
    pub requester: Option<PunchEndpoint>,
    #[serde(default)]
    pub peer: Option<PunchEndpoint>,
}

impl ControlPlaneClient {
    pub fn from_env() -> Self {
        Self {
            base_url: control_base_url_override()
                .or_else(|| env::var("SLAN_CONTROL_BASE_URL").ok())
                .unwrap_or_else(|| DEFAULT_CONTROL_BASE_URL.to_string()),
        }
    }

    pub fn list_devices(&self, access_token: &str) -> Result<Vec<ControlDevice>> {
        let response = self.request_json("GET", API_DEVICES, access_token, None)?;
        let payload: ItemsResponse<ControlDevice> =
            serde_json::from_value(response).context("decode device list")?;
        Ok(payload
            .items
            .into_iter()
            .map(|mut device| {
                normalize_control_device(&mut device);
                device
            })
            .collect())
    }

    pub fn prepare_device_login(
        &self,
        device_id: &str,
        platform: &str,
    ) -> Result<DeviceLoginPrepareResponse> {
        let mut body = register_device_body(device_id)?;
        if let Some(object) = body.as_object_mut() {
            object.insert(
                "platform".to_string(),
                Value::String(platform.trim().to_string()),
            );
        }
        let response =
            self.request_json_without_auth("POST", API_AUTH_DEVICE_LOGIN_DEVICES, Some(body))?;
        serde_json::from_value(response).context("decode device login prepare")
    }

    pub fn login_with_password(&self, email: &str, password: &str) -> Result<AuthPayload> {
        let email = email.trim();
        if email.is_empty() || password.is_empty() {
            bail!("账号和密码不能为空");
        }
        let device_id = local_stable_device_id()?;
        let body = serde_json::to_value(PasswordLoginRequest {
            email,
            password,
            device_id: Some(device_id.as_str()),
        })?;
        let response = self.request_json_without_auth("POST", API_AUTH_LOGIN, Some(body))?;
        parse_login_response(&response, email, &device_id)
    }

    pub fn bootstrap_device_session(&self, session_key: &str) -> Result<DeviceSessionResponse> {
        let session_key = session_key.trim();
        if session_key.is_empty() {
            bail!("SLAN_SESSION_KEY is empty");
        }
        let device_id = local_stable_device_id()?;
        let mut body = register_device_body(&device_id)?;
        if let Some(object) = body.as_object_mut() {
            object.insert(
                "sessionKey".to_string(),
                Value::String(session_key.to_string()),
            );
        }
        let response =
            self.request_json_without_auth("POST", API_DEVICE_SESSION_BOOTSTRAP, Some(body))?;
        let mut payload: DeviceSessionResponse =
            serde_json::from_value(response).context("decode device session bootstrap")?;
        normalize_control_device(&mut payload.device);
        Ok(payload)
    }

    pub fn renew_device_session(
        &self,
        device_token: &str,
        network_enabled: bool,
        rx_bytes_total: u64,
        tx_bytes_total: u64,
    ) -> Result<DeviceSessionResponse> {
        let body = serde_json::json!({
            "networkEnabled": network_enabled,
            "rxBytesTotal": rx_bytes_total,
            "txBytesTotal": tx_bytes_total,
        });
        let response =
            self.request_json("POST", API_DEVICE_SESSION_RENEW, device_token, Some(body))?;
        let mut payload: DeviceSessionResponse =
            serde_json::from_value(response).context("decode device session renew")?;
        normalize_control_device(&mut payload.device);
        Ok(payload)
    }

    pub fn bind_device_session(
        &self,
        access_token: &str,
        device_id: &str,
    ) -> Result<DeviceSessionResponse> {
        let mut body = register_device_body(device_id)?;
        if let Some(object) = body.as_object_mut() {
            object.insert(
                "deviceId".to_string(),
                Value::String(device_id.trim().to_string()),
            );
        }
        let response =
            self.request_json("POST", API_DEVICE_SESSION_BIND, access_token, Some(body))?;
        let mut payload: DeviceSessionResponse =
            serde_json::from_value(response).context("decode device session bind")?;
        normalize_control_device(&mut payload.device);
        Ok(payload)
    }

    pub fn logout_sessions(&self, access_token: &str, device_token: Option<&str>) -> Result<()> {
        let body = serde_json::json!({
            "deviceToken": device_token.unwrap_or_default(),
        });
        let _ = self.request_json("POST", API_AUTH_LOGOUT, access_token, Some(body))?;
        Ok(())
    }

    pub fn renew_user_session(
        &self,
        access_token: &str,
        device_id: Option<&str>,
        user_label: &str,
    ) -> Result<AuthPayload> {
        let response = self.request_json(
            "POST",
            API_AUTH_RENEW,
            access_token,
            Some(serde_json::json!({})),
        )?;
        let fallback_device_id = device_id.unwrap_or_default();
        parse_login_response(&response, user_label, fallback_device_id)
    }

    pub fn active_network_id(&self, access_token: &str) -> Result<Option<String>> {
        let device_id = local_stable_device_id()?;
        Ok(self
            .device_network_configs(access_token, &device_id)?
            .into_iter()
            .next()
            .map(|config| config.network_id))
    }

    pub fn device_network_configs(
        &self,
        access_token: &str,
        device_id: &str,
    ) -> Result<Vec<DeviceNetworkConfig>> {
        let path = api_device_network_configs(device_id);
        let response = self.request_json("GET", &path, access_token, None)?;
        let payload: ItemsResponse<DeviceNetworkConfig> =
            serde_json::from_value(response).context("decode device network configs")?;
        Ok(payload.items)
    }

    pub fn register_node(
        &self,
        access_token: &str,
        device_id: &str,
        node_id: &str,
    ) -> Result<ControlNode> {
        let _ = (access_token, device_id);
        Ok(ControlNode {
            node_id: node_id.to_string(),
        })
    }

    pub fn create_control_session(
        &self,
        access_token: &str,
        node_id: &str,
        network_id: &str,
    ) -> Result<()> {
        let _ = (access_token, node_id, network_id);
        Ok(())
    }

    pub fn activate_network(
        &self,
        access_token: &str,
        device_id: &str,
        network_id: &str,
    ) -> Result<NetworkActivationPlan> {
        let config_path = api_network_config(network_id, device_id);
        let response = self.request_json("GET", &config_path, access_token, None)?;
        activation_plan_from_network_config(&response)
    }

    #[allow(clippy::too_many_arguments)]
    pub fn issue_relay_ticket(
        &self,
        access_token: &str,
        network_id: &str,
        src_node_id: &str,
        dst_node_id: &str,
        derp_cluster_id: Option<&str>,
        preferred_derp_node_id: Option<&str>,
        preferred_relay_endpoint_id: Option<&str>,
        relay_region_id: Option<&str>,
    ) -> Result<RelayTicket> {
        let body = serde_json::to_value(RelayTicketRequest {
            network_id,
            src_node_id,
            dst_node_id,
            derp_cluster_id,
            preferred_derp_node_ids: preferred_derp_node_id.into_iter().collect(),
            preferred_relay_endpoint_ids: preferred_relay_endpoint_id.into_iter().collect(),
            reason: "udp_relay_fallback",
            relay_region_id,
        })?;
        let response = self.request_json("POST", API_RELAY_TICKETS, access_token, Some(body))?;
        serde_json::from_value(response).context("decode relay ticket")
    }

    pub fn relay_candidates(
        &self,
        access_token: &str,
        device_id: &str,
        network_id: &str,
    ) -> Result<Vec<RelayCandidate>> {
        let path = api_relay_candidates(network_id, device_id);
        let response = self.request_json("GET", &path, access_token, None)?;
        let payload: ItemsResponse<RelayCandidate> =
            serde_json::from_value(response).context("decode relay candidates")?;
        Ok(payload.items)
    }

    /// 创建 P2P punch 协商会话。请求会携带设备 ID、MQTT username 和
    /// deviceId+mqttPassword 的 MD5 签名，biz 校验后再代理到 punch-service。
    pub fn create_punch_connect_session(
        &self,
        access_token: &str,
        device_id: &str,
        mqtt: Option<&MqttCredential>,
        network_id: &str,
        requester_node_id: &str,
        peer_node_id: &str,
    ) -> Result<PunchConnectSession> {
        let punch_auth = punch_auth_headers(device_id, mqtt)?;
        let body = serde_json::to_value(PunchConnectSessionRequest {
            requester_node_id,
            peer_node_id,
            ttl_seconds: Some(60),
        })?;
        let path = api_punch_connect_sessions(network_id);
        let response = self.request_json_with_headers(
            "POST",
            &path,
            access_token,
            &[
                ("X-Slan-Device-ID", punch_auth.device_id.as_str()),
                ("X-Slan-MQTT-Username", punch_auth.mqtt_username.as_str()),
                ("X-Slan-Punch-Signature", punch_auth.signature.as_str()),
            ],
            Some(body),
        )?;
        serde_json::from_value(response).context("decode punch connect session")
    }

    pub fn deactivate_network(
        &self,
        access_token: &str,
        device_id: &str,
        network_id: &str,
    ) -> Result<()> {
        let _ = (access_token, device_id, network_id);
        Ok(())
    }

    pub fn network_prefix_len(
        &self,
        access_token: &str,
        network_id: &str,
        subnet_id: Option<&str>,
    ) -> Result<u8> {
        let _ = (access_token, network_id, subnet_id);
        Ok(8)
    }

    pub fn console_login_key(
        &self,
        access_token: &str,
        device_id: Option<&str>,
    ) -> Result<Option<String>> {
        let response = self.request_json(
            "POST",
            API_AUTH_CONSOLE_LOGIN_KEYS,
            access_token,
            Some(serde_json::json!({
                "deviceId": device_id.unwrap_or_default(),
            })),
        )?;
        let payload: ConsoleLoginKeyResponse =
            serde_json::from_value(response).context("decode console login key")?;
        Ok(Some(payload.login_key))
    }

    fn request_json(
        &self,
        method: &str,
        path: &str,
        access_token: &str,
        body: Option<Value>,
    ) -> Result<Value> {
        self.request_json_with_headers(method, path, access_token, &[], body)
    }

    fn request_json_with_headers(
        &self,
        method: &str,
        path: &str,
        access_token: &str,
        headers: &[(&str, &str)],
        body: Option<Value>,
    ) -> Result<Value> {
        let endpoint = HttpEndpoint::parse(&self.base_url)?;
        let body = body
            .map(|value| serde_json::to_vec(&value))
            .transpose()
            .context("encode control request")?
            .unwrap_or_default();
        let response = endpoint.request(method, path, access_token, headers, &body)?;
        decode_control_json(&response)
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
        let response = endpoint.request(method, path, "", &[], &body)?;
        decode_control_json(&response)
    }
}

/// 根据设备 ID 和 MQTT 凭据生成 punch 鉴权头。
fn punch_auth_headers(device_id: &str, mqtt: Option<&MqttCredential>) -> Result<PunchAuthHeaders> {
    let device_id = device_id.trim();
    if device_id.is_empty() {
        bail!("device id is required for punch auth");
    }
    let mqtt = mqtt.ok_or_else(|| anyhow::anyhow!("mqtt credential is required for punch auth"))?;
    if mqtt.username.trim().is_empty() || mqtt.password.trim().is_empty() {
        bail!("mqtt username and password are required for punch auth");
    }
    Ok(PunchAuthHeaders {
        device_id: device_id.to_string(),
        mqtt_username: mqtt.username.trim().to_string(),
        signature: punch_mqtt_signature(device_id, &mqtt.password),
    })
}

/// punch 签名算法：md5(trim(deviceId) + trim(mqttPassword))。
fn punch_mqtt_signature(device_id: &str, mqtt_password: &str) -> String {
    md5_hex(format!("{}{}", device_id.trim(), mqtt_password.trim()).as_bytes())
}

fn decode_control_json(response: &[u8]) -> Result<Value> {
    match serde_json::from_slice(response) {
        Ok(value) => Ok(value),
        Err(strict_error) => {
            let mut stream = serde_json::Deserializer::from_slice(response).into_iter::<Value>();
            match stream.next() {
                Some(Ok(value)) => Ok(value),
                _ => Err(strict_error).context("decode control response"),
            }
        }
    }
}

fn register_device_body(device_id: &str) -> Result<Value> {
    let device_id = device_id.trim();
    let device_name = device_name();
    let public_key = local_device_public_key(device_id)?;
    serde_json::to_value(RegisterDeviceRequest {
        device_id: device_id.to_string(),
        name: device_name.clone(),
        platform: platform_name().to_string(),
        os_name: platform_name().to_string(),
        os_version: env::var("SLAN_OS_VERSION").unwrap_or_default(),
        alias: device_name,
        device_version: env!("CARGO_PKG_VERSION").to_string(),
        country_code: device_country_code(),
        public_key,
    })
    .context("encode register device request")
}

fn parse_login_response(response: &Value, email: &str, device_id: &str) -> Result<AuthPayload> {
    if let Some(auth) = response.get("auth") {
        let user = auth
            .get("user")
            .ok_or_else(|| anyhow::anyhow!("login response missing auth.user"))?;
        let session = auth
            .get("session")
            .ok_or_else(|| anyhow::anyhow!("login response missing auth.session"))?;
        return Ok(AuthPayload {
            access_token: required_string(session, "token")?,
            refresh_token: optional_string(session, "token"),
            user_id: required_string(user, "userId")?,
            user_label: optional_string(user, "email").unwrap_or_else(|| email.to_string()),
            device_id: non_empty_string(device_id),
            active_network_id: login_active_network_id(response),
            virtual_ip: None,
            expires_in: login_expires_in(session),
        });
    }
    Ok(AuthPayload {
        access_token: required_string(response, "accessToken")?,
        refresh_token: optional_string(response, "refreshToken"),
        user_id: required_string(response, "userId")?,
        user_label: optional_string(response, "email").unwrap_or_else(|| email.to_string()),
        device_id: optional_string(response, "deviceId").or_else(|| non_empty_string(device_id)),
        active_network_id: login_active_network_id(response),
        virtual_ip: optional_string(response, "virtualIp"),
        expires_in: response.get("expiresIn").and_then(Value::as_u64),
    })
}

fn login_active_network_id(response: &Value) -> Option<String> {
    response
        .get("defaultNetwork")
        .and_then(|value| optional_string(value, "networkId"))
        .or_else(|| {
            response
                .get("defaultNetworkDevice")
                .and_then(|value| optional_string(value, "networkId"))
        })
        .or_else(|| {
            response
                .get("networkDevice")
                .and_then(|value| optional_string(value, "networkId"))
        })
        .or_else(|| optional_string(response, "activeNetworkId"))
        .or_else(|| optional_string(response, "networkId"))
}

fn login_expires_in(session: &Value) -> Option<u64> {
    let expires_at = session.get("expiresAt").and_then(Value::as_i64)?;
    let now = current_timestamp_seconds();
    Some(expires_at.saturating_sub(now).max(0) as u64)
}

fn normalize_control_device(device: &mut ControlDevice) {
    if device.current_virtual_ip.is_none() {
        device.current_virtual_ip = device
            .global_ip
            .clone()
            .or_else(|| device.virtual_ip.clone());
    }
    if device.virtual_ip.is_none() {
        device.virtual_ip = device.global_ip.clone();
    }
}

fn activation_plan_from_network_config(response: &Value) -> Result<NetworkActivationPlan> {
    let virtual_ip = response
        .get("globalIp")
        .and_then(Value::as_str)
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .map(normalize_virtual_ip)
        .ok_or_else(|| {
            anyhow::anyhow!("device unavailable: current device has no assigned global IP")
        })?;
    let peers = network_config_control_peers(response);
    let peer_count = peers.len();
    let self_node_id = optional_string(response, "selfNodeId")
        .or_else(|| optional_string(response, "nodeId"))
        .or_else(|| optional_string(response, "self_node_id"))
        .or_else(|| {
            optional_string(response, "deviceId").map(|device_id| format!("node-{device_id}"))
        });
    Ok(NetworkActivationPlan {
        virtual_ip,
        prefix_len: network_config_prefix_len(response).unwrap_or(32),
        dns_servers: extract_dns_servers(response),
        routes: network_config_routes(response),
        relay_candidates: extract_relay_candidates(response),
        self_node_id,
        peers,
        peer_count,
    })
}

fn network_config_prefix_len(response: &Value) -> Option<u8> {
    response
        .get("prefixLen")
        .or_else(|| response.get("prefixLength"))
        .and_then(Value::as_u64)
        .and_then(|value| u8::try_from(value).ok())
        .filter(|value| *value <= 32)
}

fn network_config_control_peers(response: &Value) -> Vec<ControlPeer> {
    response
        .get("peers")
        .and_then(Value::as_array)
        .into_iter()
        .flatten()
        .filter_map(|peer| {
            let device_id = peer
                .get("deviceId")
                .and_then(Value::as_str)
                .map(str::trim)
                .filter(|value| !value.is_empty())?
                .to_string();
            let global_ip = peer
                .get("globalIp")
                .and_then(Value::as_str)
                .map(str::trim)
                .filter(|value| !value.is_empty())
                .map(str::to_string);
            Some(ControlPeer {
                node_id: format!("node-{device_id}"),
                relay_allowed: true,
                virtual_ips: global_ip.into_iter().collect(),
                endpoints: network_config_control_endpoints(peer),
            })
        })
        .collect()
}

fn network_config_control_endpoints(peer: &Value) -> Vec<ControlEndpoint> {
    peer.get("endpoints")
        .and_then(Value::as_array)
        .into_iter()
        .flatten()
        .filter_map(|endpoint| {
            let address = endpoint
                .get("address")
                .and_then(Value::as_str)
                .map(str::trim)
                .filter(|value| !value.is_empty())?
                .to_string();
            Some(ControlEndpoint {
                endpoint_type: endpoint
                    .get("type")
                    .or_else(|| endpoint.get("kind"))
                    .and_then(Value::as_str)
                    .unwrap_or("direct_udp")
                    .to_string(),
                address,
                updated_at: endpoint
                    .get("updatedAt")
                    .or_else(|| endpoint.get("observedAt"))
                    .and_then(Value::as_i64)
                    .unwrap_or_default(),
            })
        })
        .collect()
}

fn network_config_routes(response: &Value) -> Vec<RouteSpec> {
    response
        .get("peers")
        .and_then(Value::as_array)
        .into_iter()
        .flatten()
        .filter_map(|peer| {
            let destination = peer
                .get("globalIp")
                .and_then(Value::as_str)
                .map(str::trim)
                .filter(|value| !value.is_empty())?;
            Some(RouteSpec {
                destination: format!("{destination}/32"),
                gateway: None,
            })
        })
        .collect()
}

fn required_string(response: &Value, field: &str) -> Result<String> {
    optional_string(response, field)
        .ok_or_else(|| anyhow::anyhow!("login response missing {field}"))
}

fn optional_string(response: &Value, field: &str) -> Option<String> {
    response
        .get(field)
        .and_then(Value::as_str)
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .map(str::to_string)
}

fn non_empty_string(value: &str) -> Option<String> {
    let normalized = value
        .trim()
        .trim_matches(char::from(0))
        .to_string()
        .split_whitespace()
        .collect::<Vec<_>>()
        .join(" ")
        .trim()
        .to_string();
    (!normalized.is_empty()).then_some(normalized)
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

fn extract_relay_candidates(response: &Value) -> Vec<RelayCandidate> {
    if let Some(items) = response
        .get("relayCandidates")
        .and_then(Value::as_array)
        .map(|items| {
            items
                .iter()
                .filter_map(|item| serde_json::from_value(item.clone()).ok())
                .collect::<Vec<RelayCandidate>>()
        })
        .filter(|items| !items.is_empty())
    {
        return items;
    }
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
        headers: &[(&str, &str)],
        body: &[u8],
    ) -> Result<Vec<u8>> {
        let mut last_error = None;
        for attempt in 1..=5 {
            match self.request_once(method, path, access_token, headers, body) {
                Ok(response) => return Ok(response),
                Err(error) if transient_control_request_error(&error) && attempt < 5 => {
                    last_error = Some(error);
                    thread::sleep(Duration::from_millis(120 * attempt));
                }
                Err(error) => return Err(error),
            }
        }
        Err(last_error.unwrap_or_else(|| anyhow::anyhow!("control request failed")))
    }

    fn request_once(
        &self,
        method: &str,
        path: &str,
        access_token: &str,
        headers: &[(&str, &str)],
        body: &[u8],
    ) -> Result<Vec<u8>> {
        let mut stream = TcpStream::connect((self.host.as_str(), self.port))
            .with_context(|| format!("connect control plane {}:{}", self.host, self.port))?;
        stream.set_read_timeout(Some(Duration::from_secs(10))).ok();
        stream.set_write_timeout(Some(Duration::from_secs(10))).ok();

        let target = format!("{}{}", self.base_path, path);
        let extra_headers = encode_extra_headers(headers)?;
        let request = format!(
            "{method} {target} HTTP/1.1\r\nHost: {host}\r\n{auth_header}{extra_headers}Content-Type: application/json\r\nContent-Length: {length}\r\nConnection: close\r\n\r\n",
            method = method,
            target = target,
            host = self.host,
            auth_header = if access_token.trim().is_empty() {
                String::new()
            } else {
                format!("Authorization: Bearer {access_token}\r\n")
            },
            extra_headers = extra_headers,
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

        let response = read_control_response(&mut stream).context("read control response")?;
        decode_http_response(&response)
    }
}

fn transient_control_request_error(error: &anyhow::Error) -> bool {
    let message = format!("{error:#}").to_ascii_lowercase();
    message.contains("connect control plane")
        || message.contains("read control response")
        || message.contains("try again")
        || message.contains("would block")
        || message.contains("timed out")
        || message.contains("connection reset")
        || message.contains("connection refused")
        || message.contains("software caused connection abort")
        || message.contains("http 502")
        || message.contains("http 503")
        || message.contains("http 504")
        || (message.contains("http 404")
            && (message.contains("document error") || message.contains("site or page not found")))
}

fn encode_extra_headers(headers: &[(&str, &str)]) -> Result<String> {
    let mut encoded = String::new();
    for (name, value) in headers {
        let name = name.trim();
        let value = value.trim();
        if name.is_empty() || value.is_empty() {
            continue;
        }
        if !name
            .bytes()
            .all(|byte| byte.is_ascii_alphanumeric() || matches!(byte, b'-'))
            || value.bytes().any(|byte| matches!(byte, b'\r' | b'\n'))
        {
            bail!("invalid control request header");
        }
        encoded.push_str(name);
        encoded.push_str(": ");
        encoded.push_str(value);
        encoded.push_str("\r\n");
    }
    Ok(encoded)
}

fn read_control_response(stream: &mut TcpStream) -> Result<Vec<u8>> {
    let mut response = Vec::new();
    let mut buffer = [0_u8; 8192];
    let deadline = Instant::now() + Duration::from_secs(10);
    loop {
        match stream.read(&mut buffer) {
            Ok(0) => return Ok(response),
            Ok(size) => response.extend_from_slice(&buffer[..size]),
            Err(error) if error.kind() == ErrorKind::Interrupted => continue,
            Err(error)
                if (error.kind() == ErrorKind::WouldBlock
                    || error.kind() == ErrorKind::TimedOut)
                    && looks_like_complete_http_response(&response) =>
            {
                return Ok(response);
            }
            Err(error)
                if error.kind() == ErrorKind::WouldBlock || error.kind() == ErrorKind::TimedOut =>
            {
                if Instant::now() >= deadline {
                    return Err(error).context("read control socket timed out");
                }
                thread::sleep(Duration::from_millis(25));
            }
            Err(error) => return Err(error).context("read control socket"),
        }
    }
}

fn looks_like_complete_http_response(response: &[u8]) -> bool {
    let Some(separator) = response.windows(4).position(|window| window == b"\r\n\r\n") else {
        return false;
    };
    let headers = String::from_utf8_lossy(&response[..separator]);
    if headers.lines().any(|line| {
        line.to_ascii_lowercase().starts_with("transfer-encoding:")
            && line.to_ascii_lowercase().contains("chunked")
    }) {
        return response.ends_with(b"\r\n0\r\n\r\n") || response.ends_with(b"0\r\n\r\n");
    }
    let Some(content_length) = headers.lines().find_map(|line| {
        let (name, value) = line.split_once(':')?;
        if name.trim().eq_ignore_ascii_case("content-length") {
            value.trim().parse::<usize>().ok()
        } else {
            None
        }
    }) else {
        return !response[separator + 4..].is_empty();
    };
    response.len().saturating_sub(separator + 4) >= content_length
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
    let mut body = response[separator + 4..].to_vec();
    if headers.lines().any(|line| {
        line.to_ascii_lowercase().starts_with("transfer-encoding:")
            && line.to_ascii_lowercase().contains("chunked")
    }) {
        body = decode_chunked_body(&body)?;
    }
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

fn decode_chunked_body(body: &[u8]) -> Result<Vec<u8>> {
    let mut offset = 0usize;
    let mut out = Vec::new();
    loop {
        let line_end = body[offset..]
            .windows(2)
            .position(|window| window == b"\r\n")
            .map(|pos| offset + pos)
            .ok_or_else(|| anyhow::anyhow!("invalid chunked control response"))?;
        let size_line = String::from_utf8_lossy(&body[offset..line_end]);
        let size_hex = size_line.split(';').next().unwrap_or_default().trim();
        let size = usize::from_str_radix(size_hex, 16).context("parse chunk size")?;
        offset = line_end + 2;
        if size == 0 {
            return Ok(out);
        }
        if offset + size + 2 > body.len() || &body[offset + size..offset + size + 2] != b"\r\n" {
            bail!("invalid chunked control response body");
        }
        out.extend_from_slice(&body[offset..offset + size]);
        offset += size + 2;
    }
}

fn md5_hex(input: &[u8]) -> String {
    const SHIFT: [u32; 64] = [
        7, 12, 17, 22, 7, 12, 17, 22, 7, 12, 17, 22, 7, 12, 17, 22, 5, 9, 14, 20, 5, 9, 14, 20, 5,
        9, 14, 20, 5, 9, 14, 20, 4, 11, 16, 23, 4, 11, 16, 23, 4, 11, 16, 23, 4, 11, 16, 23, 6, 10,
        15, 21, 6, 10, 15, 21, 6, 10, 15, 21, 6, 10, 15, 21,
    ];
    const K: [u32; 64] = [
        0xd76aa478, 0xe8c7b756, 0x242070db, 0xc1bdceee, 0xf57c0faf, 0x4787c62a, 0xa8304613,
        0xfd469501, 0x698098d8, 0x8b44f7af, 0xffff5bb1, 0x895cd7be, 0x6b901122, 0xfd987193,
        0xa679438e, 0x49b40821, 0xf61e2562, 0xc040b340, 0x265e5a51, 0xe9b6c7aa, 0xd62f105d,
        0x02441453, 0xd8a1e681, 0xe7d3fbc8, 0x21e1cde6, 0xc33707d6, 0xf4d50d87, 0x455a14ed,
        0xa9e3e905, 0xfcefa3f8, 0x676f02d9, 0x8d2a4c8a, 0xfffa3942, 0x8771f681, 0x6d9d6122,
        0xfde5380c, 0xa4beea44, 0x4bdecfa9, 0xf6bb4b60, 0xbebfbc70, 0x289b7ec6, 0xeaa127fa,
        0xd4ef3085, 0x04881d05, 0xd9d4d039, 0xe6db99e5, 0x1fa27cf8, 0xc4ac5665, 0xf4292244,
        0x432aff97, 0xab9423a7, 0xfc93a039, 0x655b59c3, 0x8f0ccc92, 0xffeff47d, 0x85845dd1,
        0x6fa87e4f, 0xfe2ce6e0, 0xa3014314, 0x4e0811a1, 0xf7537e82, 0xbd3af235, 0x2ad7d2bb,
        0xeb86d391,
    ];

    let bit_len = (input.len() as u64).wrapping_mul(8);
    let mut message = input.to_vec();
    message.push(0x80);
    while message.len() % 64 != 56 {
        message.push(0);
    }
    message.extend_from_slice(&bit_len.to_le_bytes());

    let mut a0 = 0x67452301_u32;
    let mut b0 = 0xefcdab89_u32;
    let mut c0 = 0x98badcfe_u32;
    let mut d0 = 0x10325476_u32;

    for chunk in message.chunks_exact(64) {
        let mut words = [0_u32; 16];
        for (index, word) in words.iter_mut().enumerate() {
            let start = index * 4;
            *word = u32::from_le_bytes([
                chunk[start],
                chunk[start + 1],
                chunk[start + 2],
                chunk[start + 3],
            ]);
        }

        let mut a = a0;
        let mut b = b0;
        let mut c = c0;
        let mut d = d0;
        for i in 0..64 {
            let (f, g) = if i < 16 {
                ((b & c) | ((!b) & d), i)
            } else if i < 32 {
                ((d & b) | ((!d) & c), (5 * i + 1) % 16)
            } else if i < 48 {
                (b ^ c ^ d, (3 * i + 5) % 16)
            } else {
                (c ^ (b | (!d)), (7 * i) % 16)
            };
            let next = b.wrapping_add(
                a.wrapping_add(f)
                    .wrapping_add(K[i])
                    .wrapping_add(words[g])
                    .rotate_left(SHIFT[i]),
            );
            a = d;
            d = c;
            c = b;
            b = next;
        }

        a0 = a0.wrapping_add(a);
        b0 = b0.wrapping_add(b);
        c0 = c0.wrapping_add(c);
        d0 = d0.wrapping_add(d);
    }

    let mut digest = Vec::with_capacity(16);
    digest.extend_from_slice(&a0.to_le_bytes());
    digest.extend_from_slice(&b0.to_le_bytes());
    digest.extend_from_slice(&c0.to_le_bytes());
    digest.extend_from_slice(&d0.to_le_bytes());
    let mut output = String::with_capacity(32);
    for byte in digest {
        output.push_str(&format!("{byte:02x}"));
    }
    output
}

fn stable_device_id(preferred_device_id: Option<&str>) -> Result<String> {
    let path = state_dir().join("client-v2-device-id.txt");
    stable_device_id_at_path(&path, preferred_device_id)
}

fn stable_device_id_at_path(
    path: &std::path::Path,
    preferred_device_id: Option<&str>,
) -> Result<String> {
    if let Some(value) = env_device_id_override() {
        if let Some(parent) = path.parent() {
            fs::create_dir_all(parent).with_context(|| format!("create {}", parent.display()))?;
        }
        fs::write(path, &value).with_context(|| format!("write {}", path.display()))?;
        return Ok(value);
    }
    if let Ok(value) = fs::read_to_string(path) {
        let value = value.trim();
        if is_uuid_like(value) {
            return Ok(value.to_string());
        }
    }
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent).with_context(|| format!("create {}", parent.display()))?;
    }
    let created = preferred_device_id
        .map(str::trim)
        .filter(|value| is_uuid_like(value))
        .map(str::to_string)
        .unwrap_or_else(uuid_v4_device_id);
    fs::write(path, &created).with_context(|| format!("write {}", path.display()))?;
    Ok(created)
}

pub fn local_stable_device_id() -> Result<String> {
    stable_device_id(None)
}

fn local_device_public_key(device_id: &str) -> Result<String> {
    let path = state_dir().join("client-v2-device-public-key.txt");
    device_public_key_at_path(&path, device_id)
}

fn device_public_key_at_path(path: &std::path::Path, device_id: &str) -> Result<String> {
    let legacy = format!("client-v2-{}", device_id.trim());
    if let Ok(value) = fs::read_to_string(path) {
        let value = value.trim();
        if is_strong_device_public_key(value) && value != legacy {
            return Ok(value.to_string());
        }
    }
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent).with_context(|| format!("create {}", parent.display()))?;
    }
    let created = random_device_public_key();
    fs::write(path, &created).with_context(|| format!("write {}", path.display()))?;
    Ok(created)
}

pub fn reset_local_device_id() -> Result<String> {
    let path = state_dir().join("client-v2-device-id.txt");
    reset_device_id_at_path(&path)
}

fn reset_device_id_at_path(path: &std::path::Path) -> Result<String> {
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent).with_context(|| format!("create {}", parent.display()))?;
    }
    let created = uuid_v4_device_id();
    fs::write(path, &created).with_context(|| format!("write {}", path.display()))?;
    Ok(created)
}

fn env_device_id_override() -> Option<String> {
    if let Some(value) = CLIENT_DEVICE_ID_OVERRIDE
        .get()
        .and_then(|mutex| mutex.lock().ok().and_then(|value| value.clone()))
        .filter(|value| is_uuid_like(value))
    {
        return Some(value);
    }
    env::var("SLAN_CLIENT_DEVICE_ID")
        .ok()
        .map(|value| value.trim().to_string())
        .filter(|value| is_uuid_like(value))
}

fn uuid_v4_device_id() -> String {
    #[cfg(windows)]
    if let Some(value) = windows_uuid_v4_string() {
        return value;
    }
    if let Some(bytes) = os_random_bytes() {
        return format_uuid_v4(bytes);
    }
    fallback_uuid_v4_device_id()
}

fn os_random_bytes() -> Option<[u8; 16]> {
    let mut bytes = [0_u8; 16];
    let mut file = fs::File::open("/dev/urandom").ok()?;
    file.read_exact(&mut bytes).ok()?;
    Some(bytes)
}

fn os_random_32_bytes() -> Option<[u8; 32]> {
    let mut bytes = [0_u8; 32];
    let mut file = fs::File::open("/dev/urandom").ok()?;
    file.read_exact(&mut bytes).ok()?;
    Some(bytes)
}

fn random_device_public_key() -> String {
    if let Some(bytes) = os_random_32_bytes() {
        return "pk_".to_string() + &hex_bytes(&bytes);
    }
    let seed = format!(
        "{}:{}:{}:{}:{:?}",
        platform_name(),
        device_name(),
        std::process::id(),
        current_timestamp_seconds(),
        std::time::SystemTime::now()
    );
    let mut bytes = [0_u8; 32];
    for (index, byte) in seed.as_bytes().iter().enumerate() {
        bytes[index % 32] ^= byte.wrapping_add(index as u8);
        bytes[(index * 11) % 32] = bytes[(index * 11) % 32]
            .wrapping_mul(31)
            .wrapping_add(*byte);
    }
    "pk_".to_string() + &hex_bytes(&bytes)
}

fn is_strong_device_public_key(value: &str) -> bool {
    let Some(hex) = value.strip_prefix("pk_") else {
        return false;
    };
    hex.len() >= 64 && hex.chars().all(|ch| ch.is_ascii_hexdigit())
}

fn hex_bytes(bytes: &[u8]) -> String {
    bytes
        .iter()
        .map(|byte| format!("{byte:02x}"))
        .collect::<Vec<_>>()
        .join("")
}

#[cfg(windows)]
fn windows_uuid_v4_string() -> Option<String> {
    let output = std::process::Command::new("powershell.exe")
        .args([
            "-NoProfile",
            "-Command",
            "[guid]::NewGuid().ToString().ToLowerInvariant()",
        ])
        .output()
        .ok()?;
    if !output.status.success() {
        return None;
    }
    let value = String::from_utf8_lossy(&output.stdout).trim().to_string();
    is_uuid_like(&value).then_some(value)
}

fn is_uuid_like(value: &str) -> bool {
    let parts = value.split('-').collect::<Vec<_>>();
    parts.iter().map(|part| part.len()).collect::<Vec<_>>() == vec![8, 4, 4, 4, 12]
        && parts
            .iter()
            .all(|part| part.chars().all(|ch| ch.is_ascii_hexdigit()))
        && parts[2].starts_with('4')
        && matches!(
            parts[3].chars().next(),
            Some('8' | '9' | 'a' | 'b' | 'A' | 'B')
        )
}

fn fallback_uuid_v4_device_id() -> String {
    let seed = format!(
        "{}:{}:{}:{}:{:?}",
        platform_name(),
        device_name(),
        std::process::id(),
        current_timestamp_seconds(),
        std::time::SystemTime::now()
    );
    let mut bytes = [0_u8; 16];
    for (index, byte) in seed.as_bytes().iter().enumerate() {
        bytes[index % 16] ^= byte.wrapping_add(index as u8);
        bytes[(index * 7) % 16] = bytes[(index * 7) % 16].wrapping_mul(31).wrapping_add(*byte);
    }
    format_uuid_v4(bytes)
}

fn format_uuid_v4(mut bytes: [u8; 16]) -> String {
    bytes[6] = (bytes[6] & 0x0f) | 0x40;
    bytes[8] = (bytes[8] & 0x3f) | 0x80;
    format!(
        "{:02x}{:02x}{:02x}{:02x}-{:02x}{:02x}-{:02x}{:02x}-{:02x}{:02x}-{:02x}{:02x}{:02x}{:02x}{:02x}{:02x}",
        bytes[0],
        bytes[1],
        bytes[2],
        bytes[3],
        bytes[4],
        bytes[5],
        bytes[6],
        bytes[7],
        bytes[8],
        bytes[9],
        bytes[10],
        bytes[11],
        bytes[12],
        bytes[13],
        bytes[14],
        bytes[15]
    )
}

fn state_dir() -> PathBuf {
    if let Some(dir) = env::var_os("SLAN_STATE_DIR") {
        return PathBuf::from(dir).join("SLAN");
    }
    if cfg!(target_os = "windows") {
        return env::var_os("ProgramData")
            .map(PathBuf::from)
            .unwrap_or_else(|| PathBuf::from(r"C:\ProgramData"))
            .join("SLAN");
    }
    if cfg!(target_os = "macos") {
        return PathBuf::from("/Library/Application Support/SLAN");
    }
    if cfg!(target_os = "ios") {
        if let Some(home) = env::var_os("HOME") {
            return PathBuf::from(home)
                .join("Library")
                .join("Application Support")
                .join("SLAN");
        }
        return env::temp_dir().join("SLAN");
    }
    if cfg!(target_os = "android") {
        return env::temp_dir().join("SLAN");
    }
    PathBuf::from("/var/lib").join("SLAN")
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

#[cfg(test)]
mod tests {
    use std::{
        env, fs,
        io::{Read, Write},
        net::TcpListener,
        path::PathBuf,
        thread,
    };

    use serde_json::Value;

    use super::{
        decode_control_json, device_public_key_at_path, is_strong_device_public_key, md5_hex,
        punch_auth_headers, punch_mqtt_signature, stable_device_id_at_path, ControlPlaneClient,
        MqttCredential, DEFAULT_CONTROL_BASE_URL,
    };

    #[test]
    fn default_control_base_url_points_to_remote_ip_endpoint() {
        assert_eq!(DEFAULT_CONTROL_BASE_URL, "http://47.245.40.231:28080");
    }

    #[test]
    fn punch_auth_uses_device_id_and_mqtt_password_md5() {
        let mqtt = MqttCredential {
            broker_url: "mqtt://127.0.0.1:1883".to_string(),
            client_id: "slan-device-1".to_string(),
            username: "slan-device-1-4102444800".to_string(),
            password: "mqtt-secret".to_string(),
            topic_prefix: "slan/device-1".to_string(),
            expires_at: Some(4_102_444_800),
        };

        assert_eq!(md5_hex(b"abc"), "900150983cd24fb0d6963f7d28e17f72");
        assert_eq!(
            punch_mqtt_signature(" device-1 ", " mqtt-secret "),
            md5_hex(b"device-1mqtt-secret")
        );
        let headers = punch_auth_headers("device-1", Some(&mqtt)).expect("punch auth headers");
        assert_eq!(headers.device_id, "device-1");
        assert_eq!(headers.mqtt_username, "slan-device-1-4102444800");
        assert_eq!(headers.signature, md5_hex(b"device-1mqtt-secret"));
    }

    #[test]
    fn password_login_posts_stable_device_id() {
        let _guard = crate::test_env_lock();
        let device_id = "11111111-1111-4111-8111-111111111111";
        let state_dir = unique_test_state_dir("password-login-device-id");
        fs::create_dir_all(&state_dir).expect("create state dir");
        env::set_var("SLAN_CLIENT_DEVICE_ID", device_id);
        env::set_var("SLAN_STATE_DIR", &state_dir);

        let listener = TcpListener::bind("127.0.0.1:0").expect("bind listener");
        let address = listener.local_addr().expect("local address");
        let request_handle = thread::spawn(move || {
            let (mut stream, _) = listener.accept().expect("accept request");
            let request = read_http_request(&mut stream);
            let response_body = br#"{"auth":{"user":{"userId":"user-1","email":"android@example.test","status":"active","createdAt":1,"updatedAt":1},"session":{"sessionId":"session-1","userId":"user-1","token":"access-1","createdAt":1,"expiresAt":4102444800}}}"#;
            write!(
                stream,
                "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",
                response_body.len()
            )
            .expect("write response headers");
            stream
                .write_all(response_body)
                .expect("write response body");
            request
        });

        let client = ControlPlaneClient {
            base_url: format!("http://{address}"),
        };
        let auth = client
            .login_with_password("android@example.test", "password-1")
            .expect("login with password");

        assert_eq!(auth.device_id.as_deref(), Some(device_id));
        let request = request_handle.join().expect("request handle");
        assert!(request.starts_with("POST /api/auth/login HTTP/1.1"));
        let (_, body) = request.split_once("\r\n\r\n").expect("login body");
        let body: Value = serde_json::from_str(body).expect("decode login body");
        assert_eq!(
            body.get("deviceId").and_then(Value::as_str),
            Some(device_id)
        );

        env::remove_var("SLAN_CLIENT_DEVICE_ID");
        env::remove_var("SLAN_STATE_DIR");
        let _ = fs::remove_dir_all(state_dir);
    }

    #[test]
    fn prepare_device_login_reads_prelogin_mqtt_credential() {
        let listener = TcpListener::bind("127.0.0.1:0").expect("bind listener");
        let address = listener.local_addr().expect("local address");
        let request_handle = thread::spawn(move || {
            let (mut stream, _) = listener.accept().expect("accept request");
            let request = read_http_request(&mut stream);
            let response_body = br#"{"deviceId":"device-1","loginUrl":"http://127.0.0.1/login","mqtt":{"brokerUrl":"mqtt://47.245.40.231:1883","clientId":"slan-device-1","username":"slan-device-1-4102444800","password":"mqtt-secret","topicPrefix":"slan/device-1","expiresAt":4102444800}}"#;
            write!(
                stream,
                "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",
                response_body.len()
            )
            .expect("write response headers");
            stream
                .write_all(response_body)
                .expect("write response body");
            request
        });

        let client = ControlPlaneClient {
            base_url: format!("http://{address}"),
        };
        let prepared = client
            .prepare_device_login("device-1", "macos")
            .expect("prepare device login");
        let mqtt = prepared.mqtt.expect("mqtt credential");

        let request = request_handle.join().expect("request handle");
        assert!(request.starts_with("POST /api/auth/device-login-devices HTTP/1.1"));
        assert_eq!(mqtt.broker_url, "mqtt://47.245.40.231:1883");
        assert_eq!(mqtt.client_id, "slan-device-1");
        assert_eq!(mqtt.password, "mqtt-secret");
        assert_eq!(mqtt.topic_prefix, "slan/device-1");
    }

    #[test]
    fn decode_control_json_accepts_trailing_response_bytes() {
        let value = decode_control_json(
            br#"{"ok":true}
0
"#,
        )
        .expect("decode first json value");
        assert_eq!(value.get("ok").and_then(Value::as_bool), Some(true));
    }

    #[test]
    fn device_id_is_uuid_v4_and_persisted() {
        let _guard = crate::test_env_lock();
        env::remove_var("SLAN_CLIENT_DEVICE_ID");

        let first_state_dir = unique_test_state_dir("uuid-device-id-first");
        let first_path = first_state_dir.join("client-v2-device-id.txt");
        let first = stable_device_id_at_path(&first_path, None).expect("create first device id");
        let first_again =
            stable_device_id_at_path(&first_path, None).expect("reuse first device id");
        assert_eq!(first, first_again);
        assert_uuid_v4(&first);
        assert_eq!(
            fs::read_to_string(&first_path)
                .expect("read persisted first device id")
                .trim(),
            first
        );

        let second_state_dir = unique_test_state_dir("uuid-device-id-second");
        let second_path = second_state_dir.join("client-v2-device-id.txt");
        let second = stable_device_id_at_path(&second_path, None).expect("create second device id");
        assert_ne!(first, second);
        assert_uuid_v4(&second);

        let _ = fs::remove_dir_all(first_state_dir);
        let _ = fs::remove_dir_all(second_state_dir);
    }

    #[test]
    fn device_public_key_is_random_persisted_and_replaces_legacy_value() {
        let state_dir = unique_test_state_dir("device-public-key");
        let path = state_dir.join("client-v2-device-public-key.txt");
        let device_id = "11111111-1111-4111-8111-111111111111";
        let first = device_public_key_at_path(&path, device_id).expect("create public key");
        let second = device_public_key_at_path(&path, device_id).expect("reuse public key");
        assert_eq!(first, second);
        assert!(is_strong_device_public_key(&first));
        assert_ne!(first, format!("client-v2-{device_id}"));

        fs::write(&path, format!("client-v2-{device_id}")).expect("write legacy public key");
        let replaced = device_public_key_at_path(&path, device_id).expect("replace legacy key");
        assert!(is_strong_device_public_key(&replaced));
        assert_ne!(replaced, format!("client-v2-{device_id}"));

        let _ = fs::remove_dir_all(state_dir);
    }

    fn assert_uuid_v4(value: &str) {
        let parts = value.split('-').collect::<Vec<_>>();
        assert_eq!(
            parts.iter().map(|part| part.len()).collect::<Vec<_>>(),
            vec![8, 4, 4, 4, 12]
        );
        assert!(parts
            .iter()
            .all(|part| part.chars().all(|ch| ch.is_ascii_hexdigit())));
        assert!(parts[2].starts_with('4'));
        assert!(matches!(
            parts[3].chars().next(),
            Some('8' | '9' | 'a' | 'b' | 'A' | 'B')
        ));
    }

    fn read_http_request(stream: &mut std::net::TcpStream) -> String {
        let mut buffer = Vec::new();
        let mut chunk = [0_u8; 512];
        loop {
            let read = stream.read(&mut chunk).expect("read request");
            assert!(read > 0, "request closed before headers");
            buffer.extend_from_slice(&chunk[..read]);
            let Some(separator) = buffer.windows(4).position(|window| window == b"\r\n\r\n") else {
                continue;
            };
            let headers = String::from_utf8_lossy(&buffer[..separator]);
            let content_length = headers
                .lines()
                .find_map(|line| {
                    let (name, value) = line.split_once(':')?;
                    name.eq_ignore_ascii_case("content-length")
                        .then(|| value.trim().parse::<usize>().ok())
                        .flatten()
                })
                .unwrap_or_default();
            let body_start = separator + 4;
            if buffer.len() >= body_start + content_length {
                return String::from_utf8(buffer).expect("utf8 request");
            }
        }
    }

    fn unique_test_state_dir(name: &str) -> PathBuf {
        env::temp_dir().join(format!(
            "slan-client-core-service-{name}-{}",
            std::time::SystemTime::now()
                .duration_since(std::time::UNIX_EPOCH)
                .expect("system time")
                .as_nanos()
        ))
    }
}
