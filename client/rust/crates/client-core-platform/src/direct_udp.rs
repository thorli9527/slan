use std::{
    fs,
    net::{SocketAddr, ToSocketAddrs, UdpSocket},
    path::PathBuf,
    sync::{Mutex, OnceLock},
    time::{Duration, SystemTime, UNIX_EPOCH},
};

use anyhow::{Context, Result};
use client_core::{
    compare_path_candidates, ipv4_source, normalize_virtual_ip,
    relay_frame::decode_slan_relay_data_frame_full, relay_peer_index_for_packet, NodeConfig,
    PathKind, PeerPathConfig,
};
use serde::{Deserialize, Serialize};

use crate::log_platform_error;

/// 兼容旧版直连 UDP 探测包。
pub const DIRECT_UDP_PROBE_PACKET: &[u8] = b"slan-direct-udp-probe-v1";
/// 兼容旧版直连 UDP 探测响应包。
pub const DIRECT_UDP_PONG_PACKET: &[u8] = b"slan-direct-udp-pong-v1";

/// SLAN 默认直连 UDP 端口，刻意避开 Tailscale 默认使用的 41641。
pub const DEFAULT_DIRECT_UDP_PORT: u16 = 41642;
const TAILSCALE_DEFAULT_UDP_PORT: u16 = 41641;

/// DirectUdpControlKind 表示 direct UDP 控制包类型。
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum DirectUdpControlKind {
    /// 主动探测对端是否可达。
    Probe,
    /// 对 probe 的响应，表示本端可收可发。
    Pong,
}

/// DirectUdpControlPacket 是 direct UDP probe/pong 的解析结果。
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct DirectUdpControlPacket {
    /// 控制包类型。
    pub kind: DirectUdpControlKind,
    /// 发送方节点 ID；旧版固定字节包可能没有该字段。
    pub node_id: Option<String>,
}

impl DirectUdpControlPacket {
    pub fn peer_node_id(&self) -> Option<&str> {
        self.node_id.as_deref().filter(|value| !value.is_empty())
    }
}

/// DirectUdpPeer 是 direct UDP runtime 中单个对端的直连状态。
#[derive(Debug)]
pub struct DirectUdpPeer {
    /// 对端节点 ID。
    pub peer_node_id: String,
    /// 对端虚拟 IP 列表，用于从 TUN 包匹配 peer。
    pub peer_virtual_ips: Vec<String>,
    /// 当前使用的直连路径类型：lan_udp/ipv6_udp/direct_udp。
    pub path_kind: PathKind,
    /// 当前记录的对端 UDP 地址。
    pub address: String,
    /// 已解析的对端 socket 地址。
    pub socket_addr: SocketAddr,
    /// 最近接收的序列号，预留给重放或乱序检测。
    pub last_rx_seq: u64,
    /// 是否已经通过 probe/pong 或有效数据包确认可用。
    pub ready: bool,
}

/// DirectUdpTransport 管理本机 direct UDP socket 以及所有 peer 的直连状态。
#[derive(Debug)]
pub struct DirectUdpTransport {
    /// 本机 UDP socket。
    pub socket: UdpSocket,
    /// 本机节点 ID，用于 probe/pong 标识发送方。
    pub local_node_id: String,
    /// 当前配置的 peer 直连表。
    pub peers: Vec<DirectUdpPeer>,
}

/// DirectUdpReceive 是 direct UDP socket 收到一帧后的匹配结果。
#[derive(Debug, Clone, Copy)]
pub struct DirectUdpReceive {
    /// 命中的 peer 下标。
    pub peer_index: usize,
    /// 收到的帧长度。
    pub frame_len: usize,
    /// 实际远端地址。
    pub remote_addr: SocketAddr,
    /// 是否发现对端地址漂移，需要更新 peer endpoint。
    pub endpoint_changed: bool,
}

/// DirectUdpEndpointReport 是本机 direct UDP 端点上报文件内容。
#[derive(Debug, Clone, Deserialize, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct DirectUdpEndpointReport {
    pub endpoint: String,
    pub endpoint_type: String,
    #[serde(default)]
    pub lan_endpoint: String,
    pub nat_type: String,
    pub bind_address: String,
    pub updated_at_ms: u64,
}

static DIRECT_UDP_ENDPOINT_REPORT: OnceLock<Mutex<Option<DirectUdpEndpointReport>>> =
    OnceLock::new();

impl DirectUdpTransport {
    /// 使用系统 UDP socket 附加 direct UDP runtime。
    pub fn attach(local_node_id: &str, configured_paths: &[PeerPathConfig]) -> Option<Self> {
        Self::attach_with_socket(local_node_id, configured_paths, || {
            attach_direct_udp_socket()
        })
    }

    /// 使用调用方提供的 socket factory 附加 direct UDP runtime，主要用于测试。
    pub fn attach_with_socket(
        local_node_id: &str,
        configured_paths: &[PeerPathConfig],
        socket_factory: impl FnOnce() -> std::io::Result<UdpSocket>,
    ) -> Option<Self> {
        let socket = match socket_factory() {
            Ok(socket) => socket,
            Err(error) => {
                let message =
                    format!("client-core-platform direct udp attach skipped error={error:#}");
                eprintln!("{message}");
                log_platform_error(message);
                clear_direct_udp_endpoint_report();
                return None;
            }
        };
        let mut peers = Vec::new();
        for path in configured_paths {
            let Some((path_kind, address)) = udp_address_for_peer(path) else {
                peers.push(DirectUdpPeer {
                    peer_node_id: path.peer_node_id.clone(),
                    peer_virtual_ips: path.peer_virtual_ips.clone(),
                    path_kind: PathKind::DirectUdp,
                    address: String::new(),
                    socket_addr: SocketAddr::from(([0, 0, 0, 0], 0)),
                    last_rx_seq: 0,
                    ready: false,
                });
                continue;
            };
            match resolve_direct_udp_peer_address(address.as_str()) {
                Ok(socket_addr) => peers.push(DirectUdpPeer {
                    peer_node_id: path.peer_node_id.clone(),
                    peer_virtual_ips: path.peer_virtual_ips.clone(),
                    path_kind,
                    address,
                    socket_addr,
                    last_rx_seq: 0,
                    ready: false,
                }),
                Err(error) => {
                    let message = format!(
                        "client-core-platform direct udp attach skipped peer={} error={error:#}",
                        path.peer_node_id
                    );
                    eprintln!("{message}");
                    log_platform_error(message);
                    peers.push(DirectUdpPeer {
                        peer_node_id: path.peer_node_id.clone(),
                        peer_virtual_ips: path.peer_virtual_ips.clone(),
                        path_kind: PathKind::DirectUdp,
                        address: String::new(),
                        socket_addr: SocketAddr::from(([0, 0, 0, 0], 0)),
                        last_rx_seq: 0,
                        ready: false,
                    });
                }
            }
        }
        if let Err(error) = socket.set_nonblocking(true) {
            let message =
                format!("client-core-platform direct udp nonblocking setup failed error={error:#}");
            eprintln!("{message}");
            log_platform_error(message);
            clear_direct_udp_endpoint_report();
            return None;
        }
        persist_direct_udp_endpoint_report(&socket);
        Some(Self {
            socket,
            local_node_id: local_node_id.to_string(),
            peers,
        })
    }

    /// 根据 TUN 包目标虚拟 IP 推断应该发送给哪个 peer。
    pub fn peer_index_for_packet(&self, payload: &[u8]) -> Option<usize> {
        relay_peer_index_for_packet(
            &self
                .peers
                .iter()
                .map(|peer| peer.peer_virtual_ips.as_slice())
                .collect::<Vec<_>>(),
            payload,
        )
        .or_else(|| (self.peers.len() == 1).then_some(0))
    }

    /// 只在 peer 已经探测 ready 时返回发送目标。
    pub fn ready_peer_index_for_packet(&self, payload: &[u8]) -> Option<usize> {
        self.peer_index_for_packet(payload)
            .filter(|index| self.peers.get(*index).is_some_and(|peer| peer.ready))
    }

    /// 通过 direct UDP socket 发送一帧数据到指定 peer。
    pub fn send_to_peer(&self, peer_index: usize, frame: &[u8]) -> std::io::Result<usize> {
        let Some(peer) = self.peers.get(peer_index) else {
            return Err(std::io::Error::new(
                std::io::ErrorKind::NotFound,
                "direct udp peer not found",
            ));
        };
        if peer.socket_addr.port() == 0 {
            return Err(std::io::Error::new(
                std::io::ErrorKind::NotConnected,
                "direct udp peer endpoint is not learned",
            ));
        }
        self.socket.send_to(frame, peer.socket_addr)
    }

    /// 从 direct UDP socket 接收一帧，并把未知来源按控制包或内层源 IP 匹配到 peer。
    pub fn recv_from_peer(
        &mut self,
        buffer: &mut [u8],
    ) -> std::io::Result<Option<DirectUdpReceive>> {
        let (frame_len, remote_addr) = self.socket.recv_from(buffer)?;
        if self.handle_punch_response(&buffer[..frame_len]) {
            return Ok(None);
        }
        if let Some(peer_index) = self
            .peers
            .iter()
            .position(|peer| peer.socket_addr == remote_addr)
        {
            return Ok(Some(DirectUdpReceive {
                peer_index,
                frame_len,
                remote_addr,
                endpoint_changed: false,
            }));
        }
        Ok(self
            .peer_index_for_unknown_inbound_packet(&buffer[..frame_len])
            .map(|peer_index| DirectUdpReceive {
                peer_index,
                frame_len,
                remote_addr,
                endpoint_changed: true,
            }))
    }

    /// 向所有已有 UDP 地址的 peer 发送 probe。
    pub fn send_probe_packets(&self) -> usize {
        let payload = direct_udp_control_payload(DirectUdpControlKind::Probe, &self.local_node_id);
        let mut sent = 0_usize;
        for peer in &self.peers {
            if peer.socket_addr.port() == 0 {
                continue;
            }
            if self
                .socket
                .send_to(payload.as_bytes(), peer.socket_addr)
                .is_ok()
            {
                sent = sent.saturating_add(1);
            }
        }
        sent
    }

    /// Register this exact data-plane socket with every configured punch node.
    pub fn send_punch_endpoint_probes(
        &self,
        network_id: &str,
        node_configs: &[NodeConfig],
    ) -> usize {
        let payload = serde_json::json!({
            "kind": "endpoint_probe",
            "networkId": network_id,
            "nodeId": self.local_node_id,
            "type": "direct_udp",
            "natType": "unknown",
        })
        .to_string();
        let mut sent = 0;
        for node in node_configs
            .iter()
            .filter(|node| node.is_direct_udp_discovery())
        {
            let address = match resolve_direct_udp_peer_address(&node.address) {
                Ok(address) => address,
                Err(error) => {
                    let message = format!(
                        "direct udp punch address rejected node={} address={} error={error:#}",
                        node.node_id, node.address
                    );
                    eprintln!("{message}");
                    log_platform_error(message);
                    continue;
                }
            };
            match self.socket.send_to(payload.as_bytes(), address) {
                Ok(_) => sent += 1,
                Err(error) => {
                    let message = format!(
                        "direct udp punch probe failed network={} node={} address={} error={error}",
                        network_id, node.node_id, address
                    );
                    eprintln!("{message}");
                    log_platform_error(message);
                }
            }
        }
        sent
    }

    /// Consume a punch response and retain the server-observed reflexive endpoint.
    pub fn handle_punch_response(&self, frame: &[u8]) -> bool {
        let Ok(value) = serde_json::from_slice::<serde_json::Value>(frame) else {
            return false;
        };
        if value.get("kind").and_then(serde_json::Value::as_str) != Some("endpoint_reflexive") {
            return false;
        }
        let endpoint = value
            .get("endpoint")
            .and_then(|value| value.get("reflexive"))
            .and_then(serde_json::Value::as_str)
            .map(str::trim)
            .filter(|value| !value.is_empty());
        let Some(endpoint) = endpoint else {
            return true;
        };
        eprintln!(
            "direct udp punch reflexive endpoint node={} endpoint={}",
            self.local_node_id, endpoint
        );
        persist_direct_udp_reflexive_endpoint(&self.socket, endpoint);
        true
    }

    /// 回复指定 peer 的 probe。
    pub fn send_pong_to_peer(&self, peer_index: usize) -> bool {
        if let Some(peer) = self.peers.get(peer_index) {
            if peer.socket_addr.port() == 0 {
                return false;
            }
            let payload =
                direct_udp_control_payload(DirectUdpControlKind::Pong, &self.local_node_id);
            return self
                .socket
                .send_to(payload.as_bytes(), peer.socket_addr)
                .is_ok();
        }
        false
    }

    /// 返回已确认 direct UDP 可用的 peer 数量。
    pub fn ready_peer_count(&self) -> usize {
        self.peers.iter().filter(|peer| peer.ready).count()
    }

    /// 标记 peer 已就绪，并在观察到地址变化时更新远端地址。
    pub fn mark_peer_ready(
        &mut self,
        peer_index: usize,
        remote_addr: SocketAddr,
    ) -> Option<String> {
        let peer = self.peers.get_mut(peer_index)?;
        if peer.socket_addr != remote_addr {
            peer.socket_addr = remote_addr;
            peer.address = remote_addr.to_string();
        }
        peer.ready = true;
        Some(peer.peer_node_id.clone())
    }

    fn peer_index_for_unknown_inbound_packet(&self, frame: &[u8]) -> Option<usize> {
        if let Some(control_packet) = direct_udp_control_packet(frame) {
            if let Some(peer_node_id) = control_packet.peer_node_id() {
                if let Some(peer_index) = self
                    .peers
                    .iter()
                    .position(|peer| peer.peer_node_id == peer_node_id)
                {
                    return Some(peer_index);
                }
            }
            return (self.peers.len() == 1).then_some(0);
        }
        let decoded = decode_slan_relay_data_frame_full(frame)?;
        let source = ipv4_source(decoded.payload)?;
        self.peers.iter().position(|peer| {
            peer.peer_virtual_ips
                .iter()
                .any(|ip| normalize_virtual_ip(ip) == source)
        })
    }
}

fn attach_direct_udp_socket() -> std::io::Result<UdpSocket> {
    let port = configured_direct_udp_port();
    if port > 0 {
        if let Ok(socket) = UdpSocket::bind(("0.0.0.0", port)) {
            return Ok(socket);
        }
    }
    UdpSocket::bind("0.0.0.0:0")
}

/// 返回直连 UDP 监听端口；0 表示由操作系统随机分配。
pub fn configured_direct_udp_port() -> u16 {
    configured_direct_udp_port_from(
        std::env::var("SLAN_DIRECT_UDP_RANDOMIZE").ok().as_deref(),
        std::env::var("SLAN_DIRECT_UDP_PORT").ok().as_deref(),
    )
}

fn configured_direct_udp_port_from(randomize: Option<&str>, port: Option<&str>) -> u16 {
    if randomize.is_some_and(env_flag_enabled) {
        return 0;
    }
    port.and_then(|value| value.trim().parse::<u16>().ok())
        .filter(|port| *port > 0 && *port != TAILSCALE_DEFAULT_UDP_PORT)
        .unwrap_or(DEFAULT_DIRECT_UDP_PORT)
}

fn env_flag_enabled(value: &str) -> bool {
    matches!(
        value.trim().to_ascii_lowercase().as_str(),
        "1" | "true" | "yes" | "on"
    )
}

pub fn direct_udp_control_payload(kind: DirectUdpControlKind, node_id: &str) -> String {
    let kind = match kind {
        DirectUdpControlKind::Probe => "probe",
        DirectUdpControlKind::Pong => "pong",
    };
    serde_json::json!({
        "kind": "direct_udp",
        "type": kind,
        "nodeId": node_id,
    })
    .to_string()
}

pub fn direct_udp_control_packet(frame: &[u8]) -> Option<DirectUdpControlPacket> {
    if frame == DIRECT_UDP_PROBE_PACKET {
        return Some(DirectUdpControlPacket {
            kind: DirectUdpControlKind::Probe,
            node_id: None,
        });
    }
    if frame == DIRECT_UDP_PONG_PACKET {
        return Some(DirectUdpControlPacket {
            kind: DirectUdpControlKind::Pong,
            node_id: None,
        });
    }
    let value = serde_json::from_slice::<serde_json::Value>(frame).ok()?;
    if value.get("kind").and_then(serde_json::Value::as_str) != Some("direct_udp") {
        return None;
    }
    let kind = match value.get("type").and_then(serde_json::Value::as_str)? {
        "probe" => DirectUdpControlKind::Probe,
        "pong" => DirectUdpControlKind::Pong,
        _ => return None,
    };
    let node_id = value
        .get("nodeId")
        .or_else(|| value.get("node_id"))
        .and_then(serde_json::Value::as_str)
        .map(str::to_string);
    Some(DirectUdpControlPacket { kind, node_id })
}

pub fn direct_udp_probe_interval_from_ms(value: u64) -> Duration {
    if value == 0 {
        return Duration::from_secs(15);
    }
    Duration::from_millis(value).clamp(Duration::from_secs(1), Duration::from_secs(300))
}

fn udp_address_for_peer(path: &PeerPathConfig) -> Option<(PathKind, String)> {
    path.candidates
        .iter()
        .filter(|candidate| candidate.kind.is_direct_udp())
        .filter(|candidate| {
            candidate
                .address
                .as_deref()
                .is_some_and(|value| !value.trim().is_empty())
        })
        .min_by(|left, right| compare_path_candidates(left, right))
        .map(|candidate| {
            (
                candidate.kind,
                candidate
                    .address
                    .as_deref()
                    .unwrap_or_default()
                    .trim()
                    .to_string(),
            )
        })
}

fn resolve_direct_udp_peer_address(address: &str) -> Result<SocketAddr> {
    let address = normalize_direct_udp_address(address)
        .ok_or_else(|| anyhow::anyhow!("missing direct UDP address"))?;
    address
        .to_socket_addrs()
        .with_context(|| format!("resolve direct UDP peer address {address}"))?
        .next()
        .ok_or_else(|| anyhow::anyhow!("direct UDP address resolved no endpoints: {address}"))
}

fn normalize_direct_udp_address(address: &str) -> Option<String> {
    let trimmed = address.trim();
    if trimmed.is_empty() {
        return None;
    }
    if trimmed.starts_with("relay+udp://") {
        return None;
    }
    let stripped = trimmed
        .strip_prefix("udp://")
        .or_else(|| trimmed.strip_prefix("direct+udp://"));
    if stripped.is_none() && trimmed.contains("://") {
        return None;
    }
    let normalized = stripped.unwrap_or(trimmed).trim();
    (!normalized.is_empty()).then(|| normalized.to_string())
}

fn direct_udp_endpoint_file_path() -> PathBuf {
    if let Some(dir) = std::env::var_os("ProgramData") {
        return PathBuf::from(dir)
            .join("SLAN")
            .join("client-v2-direct-udp-endpoint.json");
    }
    if let Some(dir) = std::env::var_os("SLAN_STATE_DIR") {
        return PathBuf::from(dir).join("client-v2-direct-udp-endpoint.json");
    }
    app_data_dir()
        .join("SLAN")
        .join("client-v2-direct-udp-endpoint.json")
}

pub fn persist_direct_udp_endpoint_report(socket: &UdpSocket) {
    let Ok(local_addr) = socket.local_addr() else {
        return;
    };
    let report_host = direct_udp_lan_host().unwrap_or_else(|| local_addr.ip().to_string());
    let report = DirectUdpEndpointReport {
        endpoint: format!("{report_host}:{}", local_addr.port()),
        endpoint_type: "lan_udp".to_string(),
        lan_endpoint: format!("{report_host}:{}", local_addr.port()),
        nat_type: "unknown".to_string(),
        bind_address: local_addr.to_string(),
        updated_at_ms: current_timestamp_ms(),
    };
    persist_direct_udp_endpoint_report_value(report);
}

pub fn persist_direct_udp_reflexive_endpoint(socket: &UdpSocket, endpoint: &str) {
    let Ok(local_addr) = socket.local_addr() else {
        return;
    };
    let lan_endpoint = current_direct_udp_endpoint_report()
        .map(|report| {
            if report.lan_endpoint.trim().is_empty() && report.endpoint_type == "lan_udp" {
                report.endpoint
            } else {
                report.lan_endpoint
            }
        })
        .unwrap_or_default();
    persist_direct_udp_endpoint_report_value(DirectUdpEndpointReport {
        endpoint: endpoint.trim().to_string(),
        endpoint_type: "direct_udp".to_string(),
        lan_endpoint,
        nat_type: "unknown".to_string(),
        bind_address: local_addr.to_string(),
        updated_at_ms: current_timestamp_ms(),
    });
}

fn persist_direct_udp_endpoint_report_value(report: DirectUdpEndpointReport) {
    if let Ok(mut current) = DIRECT_UDP_ENDPOINT_REPORT
        .get_or_init(|| Mutex::new(None))
        .lock()
    {
        *current = Some(report.clone());
    }
    let path = direct_udp_endpoint_file_path();
    let Some(parent) = path.parent() else {
        return;
    };
    if fs::create_dir_all(parent).is_err() {
        return;
    }
    let Ok(payload) = serde_json::to_vec_pretty(&report) else {
        return;
    };
    let _ = fs::write(path, payload);
}

pub fn clear_direct_udp_endpoint_report() {
    if let Ok(mut current) = DIRECT_UDP_ENDPOINT_REPORT
        .get_or_init(|| Mutex::new(None))
        .lock()
    {
        *current = None;
    }
    let path = direct_udp_endpoint_file_path();
    if path.exists() {
        let _ = fs::remove_file(path);
    }
}

pub fn current_direct_udp_endpoint_report() -> Option<DirectUdpEndpointReport> {
    DIRECT_UDP_ENDPOINT_REPORT
        .get_or_init(|| Mutex::new(None))
        .lock()
        .ok()
        .and_then(|current| current.clone())
}

fn direct_udp_lan_host() -> Option<String> {
    let socket = UdpSocket::bind("0.0.0.0:0").ok()?;
    socket.connect("8.8.8.8:80").ok()?;
    let local_addr = socket.local_addr().ok()?;
    let ip = local_addr.ip();
    (!ip.is_unspecified() && !ip.is_loopback()).then(|| ip.to_string())
}

fn app_data_dir() -> PathBuf {
    std::env::var_os("SLAN_STATE_DIR")
        .map(PathBuf::from)
        .or_else(|| {
            cfg!(target_os = "macos").then(|| PathBuf::from("/Library/Application Support"))
        })
        .or_else(|| std::env::var_os("ProgramData").map(PathBuf::from))
        .unwrap_or_else(|| PathBuf::from("/var/lib"))
}

fn current_timestamp_ms() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|duration| duration.as_millis() as u64)
        .unwrap_or_default()
}

#[cfg(test)]
mod tests {
    use super::*;
    use client_core::{PathCandidate, PathState};

    #[test]
    fn direct_udp_control_packets_are_recognized() {
        assert_eq!(
            direct_udp_control_packet(DIRECT_UDP_PROBE_PACKET).map(|packet| packet.kind),
            Some(DirectUdpControlKind::Probe)
        );
        assert_eq!(
            direct_udp_control_packet(DIRECT_UDP_PONG_PACKET).map(|packet| packet.kind),
            Some(DirectUdpControlKind::Pong)
        );
        let packet = direct_udp_control_payload(DirectUdpControlKind::Probe, "node-a");
        let parsed = direct_udp_control_packet(packet.as_bytes()).unwrap();
        assert_eq!(parsed.kind, DirectUdpControlKind::Probe);
        assert_eq!(parsed.peer_node_id(), Some("node-a"));
    }

    #[test]
    fn direct_udp_address_normalization_accepts_udp_schemes() {
        assert_eq!(
            normalize_direct_udp_address("127.0.0.1:3478").as_deref(),
            Some("127.0.0.1:3478")
        );
        assert_eq!(
            normalize_direct_udp_address("udp://127.0.0.1:3478").as_deref(),
            Some("127.0.0.1:3478")
        );
        assert_eq!(
            normalize_direct_udp_address("direct+udp://127.0.0.1:3478").as_deref(),
            Some("127.0.0.1:3478")
        );
        assert_eq!(
            normalize_direct_udp_address("relay+udp://127.0.0.1:3478"),
            None
        );
        assert_eq!(normalize_direct_udp_address("https://127.0.0.1:3478"), None);
    }

    #[test]
    fn direct_udp_port_defaults_to_slan_fixed_port() {
        assert_eq!(configured_direct_udp_port_from(None, None), 41642);
        assert_eq!(
            configured_direct_udp_port_from(Some("false"), Some("42000")),
            42000
        );
    }

    #[test]
    fn direct_udp_port_supports_random_mode_and_rejects_tailscale_port() {
        assert_eq!(
            configured_direct_udp_port_from(Some("true"), Some("42000")),
            0
        );
        assert_eq!(
            configured_direct_udp_port_from(Some("false"), Some("41641")),
            41642
        );
    }

    #[test]
    fn direct_udp_transport_matches_peer_by_inner_source_ip() {
        let local = UdpSocket::bind("127.0.0.1:0").unwrap();
        let known_remote = UdpSocket::bind("127.0.0.1:0").unwrap();
        let remote = UdpSocket::bind("127.0.0.1:0").unwrap();
        local.set_nonblocking(true).unwrap();
        let mut transport = DirectUdpTransport {
            socket: local,
            local_node_id: "node-local".to_string(),
            peers: vec![DirectUdpPeer {
                peer_node_id: "node-a".to_string(),
                peer_virtual_ips: vec!["10.0.0.9/32".to_string()],
                path_kind: PathKind::DirectUdp,
                address: known_remote.local_addr().unwrap().to_string(),
                socket_addr: known_remote.local_addr().unwrap(),
                last_rx_seq: 0,
                ready: false,
            }],
        };
        let packet = [
            0x45, 0, 0, 20, 0, 0, 0, 0, 64, 17, 0, 0, 10, 0, 0, 9, 10, 0, 0, 8,
        ];
        let frame = client_core::relay_frame::encode_slan_relay_data_frame(1, 2, &packet).unwrap();
        remote
            .send_to(&frame, transport.socket.local_addr().unwrap())
            .unwrap();
        let mut buffer = [0_u8; 256];
        let mut received = None;
        for _ in 0..20 {
            match transport.recv_from_peer(&mut buffer) {
                Ok(value) => {
                    received = value;
                    break;
                }
                Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                    std::thread::sleep(Duration::from_millis(10));
                }
                Err(error) => panic!("recv direct udp frame: {error}"),
            }
        }
        let received = received.expect("expected direct udp frame");
        assert_eq!(received.peer_index, 0);
        assert!(received.endpoint_changed);
    }

    #[test]
    fn direct_udp_attach_uses_first_udp_candidate() {
        let socket = UdpSocket::bind("127.0.0.1:0").unwrap();
        let address = socket.local_addr().unwrap().to_string();
        let paths = vec![PeerPathConfig {
            peer_node_id: "node-a".to_string(),
            peer_virtual_ips: vec!["10.0.0.9".to_string()],
            candidates: vec![PathCandidate {
                kind: PathKind::DirectUdp,
                state: PathState::Standby,
                endpoint_id: None,
                address: Some(address),
                session_id: None,
                transport: None,
                rtt_ms: None,
                path_score: None,
                last_ok_at_ms: None,
                last_error: None,
            }],
        }];
        let transport = DirectUdpTransport::attach_with_socket("node-local", &paths, || {
            UdpSocket::bind("127.0.0.1:0")
        })
        .unwrap();
        assert_eq!(transport.peers.len(), 1);
    }

    #[test]
    fn direct_udp_attach_prefers_better_quality_port_within_same_path_kind() {
        let slow = UdpSocket::bind("127.0.0.1:0").unwrap();
        let fast = UdpSocket::bind("127.0.0.1:0").unwrap();
        let fast_address = fast.local_addr().unwrap();
        let candidate = |address: String, path_score: u32, rtt_ms: u32| PathCandidate {
            kind: PathKind::DirectUdp,
            state: PathState::Ready,
            endpoint_id: None,
            address: Some(address),
            session_id: None,
            transport: Some("udp".to_string()),
            rtt_ms: Some(rtt_ms),
            path_score: Some(path_score),
            last_ok_at_ms: Some(1),
            last_error: None,
        };
        let paths = vec![PeerPathConfig {
            peer_node_id: "node-a".to_string(),
            peer_virtual_ips: vec!["10.0.0.9".to_string()],
            candidates: vec![
                candidate(slow.local_addr().unwrap().to_string(), 180, 120),
                candidate(fast_address.to_string(), 35, 20),
            ],
        }];

        let transport = DirectUdpTransport::attach_with_socket("node-local", &paths, || {
            UdpSocket::bind("127.0.0.1:0")
        })
        .unwrap();

        assert_eq!(transport.peers[0].socket_addr, fast_address);
    }

    #[test]
    fn direct_udp_attach_prefers_lan_candidate_regardless_of_input_order() {
        let public = UdpSocket::bind("127.0.0.1:0").unwrap();
        let lan = UdpSocket::bind("127.0.0.1:0").unwrap();
        let paths = vec![PeerPathConfig {
            peer_node_id: "node-a".to_string(),
            peer_virtual_ips: vec!["10.0.0.9".to_string()],
            candidates: vec![
                PathCandidate {
                    kind: PathKind::DirectUdp,
                    state: PathState::Probing,
                    endpoint_id: None,
                    address: Some(public.local_addr().unwrap().to_string()),
                    session_id: None,
                    transport: Some("udp".to_string()),
                    rtt_ms: None,
                    path_score: None,
                    last_ok_at_ms: None,
                    last_error: None,
                },
                PathCandidate {
                    kind: PathKind::LanUdp,
                    state: PathState::Probing,
                    endpoint_id: None,
                    address: Some(lan.local_addr().unwrap().to_string()),
                    session_id: None,
                    transport: Some("udp".to_string()),
                    rtt_ms: None,
                    path_score: None,
                    last_ok_at_ms: None,
                    last_error: None,
                },
            ],
        }];

        let transport = DirectUdpTransport::attach_with_socket("node-local", &paths, || {
            UdpSocket::bind("127.0.0.1:0")
        })
        .unwrap();

        assert_eq!(transport.peers[0].path_kind, PathKind::LanUdp);
        assert_eq!(transport.peers[0].socket_addr, lan.local_addr().unwrap());
    }

    #[test]
    fn punch_probe_uses_only_canonical_direct_discovery_nodes() {
        let punch = UdpSocket::bind("127.0.0.1:0").unwrap();
        punch
            .set_read_timeout(Some(Duration::from_secs(1)))
            .unwrap();
        let transport = DirectUdpTransport::attach_with_socket("node-local", &[], || {
            UdpSocket::bind("127.0.0.1:0")
        })
        .unwrap();
        let nodes = vec![
            NodeConfig {
                node_id: "punch-1".to_string(),
                connection_type: "direct".to_string(),
                transport: "udp".to_string(),
                path_kind: "direct_udp".to_string(),
                address: punch.local_addr().unwrap().to_string(),
                country_code: String::new(),
                city_code: String::new(),
                priority: 100,
            },
            NodeConfig {
                node_id: "relay-1".to_string(),
                connection_type: "relay".to_string(),
                transport: "udp".to_string(),
                path_kind: "relay_udp".to_string(),
                address: punch.local_addr().unwrap().to_string(),
                country_code: String::new(),
                city_code: String::new(),
                priority: 200,
            },
        ];

        assert_eq!(transport.send_punch_endpoint_probes("network-1", &nodes), 1);
        let mut buffer = [0_u8; 512];
        let (len, _) = punch.recv_from(&mut buffer).unwrap();
        let payload: serde_json::Value = serde_json::from_slice(&buffer[..len]).unwrap();
        assert_eq!(payload["kind"], "endpoint_probe");
        assert_eq!(payload["networkId"], "network-1");
        assert_eq!(payload["nodeId"], "node-local");
    }

    #[test]
    fn direct_udp_attach_keeps_socket_without_candidates_for_endpoint_report() {
        let transport = DirectUdpTransport::attach_with_socket("node-local", &[], || {
            UdpSocket::bind("127.0.0.1:0")
        })
        .unwrap();
        assert!(transport.peers.is_empty());
    }

    #[test]
    fn direct_udp_attach_keeps_peer_without_candidate_for_roaming_probe() {
        let paths = vec![PeerPathConfig {
            peer_node_id: "node-a".to_string(),
            peer_virtual_ips: vec!["10.0.0.9".to_string()],
            candidates: Vec::new(),
        }];
        let mut transport = DirectUdpTransport::attach_with_socket("node-local", &paths, || {
            UdpSocket::bind("127.0.0.1:0")
        })
        .unwrap();
        assert_eq!(transport.peers.len(), 1);
        assert_eq!(transport.peers[0].socket_addr.port(), 0);
        assert_eq!(
            transport.mark_peer_ready(0, "127.0.0.1:32123".parse().unwrap()),
            Some("node-a".to_string())
        );
        assert_eq!(transport.peers[0].address, "127.0.0.1:32123");
        assert!(transport.peers[0].ready);
    }
}
