use std::{
    fs,
    net::{SocketAddr, ToSocketAddrs, UdpSocket},
    path::PathBuf,
    sync::{Mutex, OnceLock},
    time::{SystemTime, UNIX_EPOCH},
};

use anyhow::{Context, Result};
use client_core::{
    compare_path_candidates, ipv4_source, normalize_virtual_ip,
    path::{PathProbeController, PathProbeRole},
    relay_frame::decode_slan_relay_data_frame_full,
    relay_peer_index_for_packet, NodeConfig, PathKind, PeerPathConfig,
};
use serde::{Deserialize, Serialize};

use crate::log_platform_error;

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
    /// 发送方节点 ID。
    pub node_id: String,
}

impl DirectUdpControlPacket {
    pub fn peer_node_id(&self) -> &str {
        &self.node_id
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
    /// 需要并行探测的全部直连候选，避免首个 LAN 候选不可达时阻断公网打洞。
    probe_targets: Vec<DirectUdpProbeTarget>,
    /// 最近接收的序列号，预留给重放或乱序检测。
    pub last_rx_seq: u64,
    /// 是否已经通过 probe/pong 或有效数据包确认可用。
    pub ready: bool,
    /// 客户端本地路径探测状态，不接受服务端策略覆盖。
    pub probe_controller: PathProbeController,
}

#[derive(Debug, Clone, PartialEq, Eq)]
struct DirectUdpProbeTarget {
    path_kind: PathKind,
    address: String,
    socket_addr: SocketAddr,
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
            let probe_targets = udp_probe_targets_for_peer(path);
            let Some(primary) = probe_targets.first() else {
                peers.push(DirectUdpPeer {
                    peer_node_id: path.peer_node_id.clone(),
                    peer_virtual_ips: path.peer_virtual_ips.clone(),
                    path_kind: PathKind::DirectUdp,
                    address: String::new(),
                    socket_addr: SocketAddr::from(([0, 0, 0, 0], 0)),
                    probe_targets,
                    last_rx_seq: 0,
                    ready: false,
                    probe_controller: PathProbeController::new(),
                });
                continue;
            };
            peers.push(DirectUdpPeer {
                peer_node_id: path.peer_node_id.clone(),
                peer_virtual_ips: path.peer_virtual_ips.clone(),
                path_kind: primary.path_kind,
                address: primary.address.clone(),
                socket_addr: primary.socket_addr,
                probe_targets,
                last_rx_seq: 0,
                ready: false,
                probe_controller: PathProbeController::new(),
            });
        }
        if let Err(error) = socket.set_nonblocking(true) {
            let message =
                format!("client-core-platform direct udp nonblocking setup failed error={error:#}");
            eprintln!("{message}");
            log_platform_error(message);
            clear_direct_udp_endpoint_report();
            return None;
        }
        let local_addr = socket.local_addr().ok();
        eprintln!(
            "SLAN_DIRECT_UDP_TRANSPORT_ATTACHED localNodeId={} bindAddress={} peerCount={}",
            local_node_id,
            local_addr
                .map(|address| address.to_string())
                .unwrap_or_else(|| "unknown".to_string()),
            peers.len()
        );
        for peer in &peers {
            let candidates = peer
                .probe_targets
                .iter()
                .map(|target| format!("{}@{}", target.path_kind.as_str(), target.address))
                .collect::<Vec<_>>()
                .join(",");
            eprintln!(
                "SLAN_DIRECT_UDP_PEER_CANDIDATES peer={} virtualIps={} candidates={}",
                peer.peer_node_id,
                peer.peer_virtual_ips.join(","),
                if candidates.is_empty() {
                    "none"
                } else {
                    candidates.as_str()
                }
            );
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
        if let Some(peer_index) = self.peers.iter().position(|peer| {
            peer.socket_addr == remote_addr
                || peer
                    .probe_targets
                    .iter()
                    .any(|target| target.socket_addr == remote_addr)
        }) {
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

    /// 按客户端本地状态机向到期的 peer 发送 probe。
    pub fn send_probe_packets(&mut self) -> usize {
        let now_ms = current_timestamp_ms();
        let payload = direct_udp_control_payload(DirectUdpControlKind::Probe, &self.local_node_id);
        let mut sent = 0_usize;
        for peer in &mut self.peers {
            if peer.probe_targets.is_empty() && peer.socket_addr.port() == 0 {
                continue;
            }
            let role = if peer.ready {
                PathProbeRole::Active
            } else {
                PathProbeRole::Standby
            };
            if !peer.probe_controller.should_probe(now_ms, role) {
                peer.ready = peer.probe_controller.usable();
                continue;
            }
            let targets = if peer.probe_targets.is_empty() {
                vec![(peer.path_kind, peer.socket_addr)]
            } else {
                peer.probe_targets
                    .iter()
                    .map(|target| (target.path_kind, target.socket_addr))
                    .collect()
            };
            let mut sent_any = false;
            for (path_kind, target) in targets {
                match self.socket.send_to(payload.as_bytes(), target) {
                    Ok(size) => {
                        sent_any = true;
                        eprintln!(
                            "SLAN_DIRECT_UDP_PROBE_SENT peer={} path={} target={} bytes={} role={role:?} health={:?}",
                            peer.peer_node_id,
                            path_kind.as_str(),
                            target,
                            size,
                            peer.probe_controller.health()
                        );
                    }
                    Err(error) => {
                        let message = format!(
                            "SLAN_DIRECT_UDP_PROBE_SEND_FAILED peer={} path={} target={} role={role:?} health={:?} error={error}",
                            peer.peer_node_id,
                            path_kind.as_str(),
                            target,
                            peer.probe_controller.health()
                        );
                        eprintln!("{message}");
                        log_platform_error(message);
                    }
                }
            }
            if sent_any {
                peer.probe_controller.on_probe_sent(now_ms);
                sent = sent.saturating_add(1);
            }
            peer.ready = peer.probe_controller.usable();
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
        if persist_direct_udp_reflexive_endpoint(&self.socket, endpoint) {
            eprintln!(
                "direct udp punch reflexive endpoint node={} endpoint={}",
                self.local_node_id, endpoint
            );
        }
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

    /// Demote direct paths that have received no valid peer traffic for the timeout window.
    /// Probes continue after demotion, so a later pong restores the path without rebuilding TUN.
    pub fn poll_probe_health(&mut self, now_ms: u64) -> Vec<String> {
        let mut expired = Vec::new();
        for peer in &mut self.peers {
            let was_ready = peer.ready;
            let role = if peer.ready {
                PathProbeRole::Active
            } else {
                PathProbeRole::Standby
            };
            let _ = peer.probe_controller.should_probe(now_ms, role);
            peer.ready = peer.probe_controller.usable();
            if was_ready && !peer.ready {
                peer.ready = false;
                eprintln!(
                    "SLAN_DIRECT_UDP_PATH_FAILED peer={} path={} endpoint={} health={:?} reason=probe_timeout",
                    peer.peer_node_id,
                    peer.path_kind.as_str(),
                    peer.address,
                    peer.probe_controller.health()
                );
                expired.push(peer.peer_node_id.clone());
            }
        }
        expired
    }

    /// 标记 peer 已就绪，并在观察到地址变化时更新远端地址。
    pub fn mark_peer_ready(
        &mut self,
        peer_index: usize,
        remote_addr: SocketAddr,
    ) -> Option<String> {
        let peer = self.peers.get_mut(peer_index)?;
        let was_ready = peer.ready;
        let previous_path_kind = peer.path_kind;
        let previous_addr = peer.socket_addr;
        if let Some(target) = peer
            .probe_targets
            .iter()
            .find(|target| target.socket_addr == remote_addr)
        {
            peer.path_kind = target.path_kind;
        } else if peer.socket_addr != remote_addr {
            peer.path_kind = PathKind::DirectUdp;
        }
        if peer.socket_addr != remote_addr {
            peer.socket_addr = remote_addr;
            peer.address = remote_addr.to_string();
        }
        let now_ms = current_timestamp_ms();
        peer.probe_controller.on_inbound(now_ms);
        peer.ready = peer.probe_controller.usable();
        if !was_ready || previous_path_kind != peer.path_kind || previous_addr != peer.socket_addr {
            eprintln!(
                "SLAN_DIRECT_UDP_PATH_READY peer={} path={} endpoint={} previousPath={} previousEndpoint={} health={:?}",
                peer.peer_node_id,
                peer.path_kind.as_str(),
                peer.socket_addr,
                previous_path_kind.as_str(),
                previous_addr,
                peer.probe_controller.health()
            );
        }
        Some(peer.peer_node_id.clone())
    }

    fn peer_index_for_unknown_inbound_packet(&self, frame: &[u8]) -> Option<usize> {
        if let Some(control_packet) = direct_udp_control_packet(frame) {
            return self
                .peers
                .iter()
                .position(|peer| peer.peer_node_id == control_packet.peer_node_id());
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
        match UdpSocket::bind(("0.0.0.0", port)) {
            Ok(socket) => return Ok(socket),
            Err(error) => {
                let message = format!(
                    "direct udp preferred port unavailable port={port}; allocating random port: {error}"
                );
                eprintln!("{message}");
                log_platform_error(message);
            }
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
        .and_then(serde_json::Value::as_str)
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .map(str::to_string)?;
    Some(DirectUdpControlPacket { kind, node_id })
}

fn udp_probe_targets_for_peer(path: &PeerPathConfig) -> Vec<DirectUdpProbeTarget> {
    let mut candidates = path
        .candidates
        .iter()
        .filter(|candidate| candidate.kind.is_direct_udp())
        .filter(|candidate| {
            candidate
                .address
                .as_deref()
                .is_some_and(|value| !value.trim().is_empty())
        })
        .collect::<Vec<_>>();
    candidates.sort_by(|left, right| compare_path_candidates(left, right));

    let mut targets = Vec::new();
    for candidate in candidates {
        let address = candidate
            .address
            .as_deref()
            .unwrap_or_default()
            .trim()
            .to_string();
        match resolve_direct_udp_peer_address(&address) {
            Ok(socket_addr)
                if !targets
                    .iter()
                    .any(|target: &DirectUdpProbeTarget| target.socket_addr == socket_addr) =>
            {
                targets.push(DirectUdpProbeTarget {
                    path_kind: candidate.kind,
                    address,
                    socket_addr,
                });
            }
            Ok(_) => {}
            Err(error) => {
                let message = format!(
                    "client-core-platform direct udp candidate skipped peer={} address={} error={error:#}",
                    path.peer_node_id, address
                );
                eprintln!("{message}");
                log_platform_error(message);
            }
        }
    }
    targets
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

pub fn persist_direct_udp_reflexive_endpoint(socket: &UdpSocket, endpoint: &str) -> bool {
    let Ok(local_addr) = socket.local_addr() else {
        return false;
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
    })
}

fn persist_direct_udp_endpoint_report_value(report: DirectUdpEndpointReport) -> bool {
    let mut changed = true;
    if let Ok(mut current) = DIRECT_UDP_ENDPOINT_REPORT
        .get_or_init(|| Mutex::new(None))
        .lock()
    {
        changed = current
            .as_ref()
            .is_none_or(|value| !same_direct_udp_endpoint(value, &report));
        *current = Some(report.clone());
    }
    if !changed {
        return false;
    }
    let path = direct_udp_endpoint_file_path();
    let Some(parent) = path.parent() else {
        return true;
    };
    if fs::create_dir_all(parent).is_err() {
        return true;
    }
    let Ok(payload) = serde_json::to_vec_pretty(&report) else {
        return true;
    };
    let _ = fs::write(path, payload);
    true
}

fn same_direct_udp_endpoint(
    left: &DirectUdpEndpointReport,
    right: &DirectUdpEndpointReport,
) -> bool {
    left.endpoint == right.endpoint
        && left.endpoint_type == right.endpoint_type
        && left.lan_endpoint == right.lan_endpoint
        && left.nat_type == right.nat_type
        && left.bind_address == right.bind_address
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
    use std::time::Duration;

    #[test]
    fn direct_udp_control_packets_are_recognized() {
        let packet = direct_udp_control_payload(DirectUdpControlKind::Probe, "node-a");
        let parsed = direct_udp_control_packet(packet.as_bytes()).unwrap();
        assert_eq!(parsed.kind, DirectUdpControlKind::Probe);
        assert_eq!(parsed.peer_node_id(), "node-a");
        assert_eq!(
            direct_udp_control_packet(br#"{"kind":"direct_udp","type":"probe"}"#),
            None
        );
    }

    #[test]
    fn endpoint_report_identity_ignores_timestamp_only_changes() {
        let report = DirectUdpEndpointReport {
            endpoint: "203.0.113.10:41642".to_string(),
            endpoint_type: "direct_udp".to_string(),
            lan_endpoint: "192.168.5.101:41642".to_string(),
            nat_type: "unknown".to_string(),
            bind_address: "0.0.0.0:41642".to_string(),
            updated_at_ms: 1,
        };
        let mut refreshed = report.clone();
        refreshed.updated_at_ms = 2;
        assert!(same_direct_udp_endpoint(&report, &refreshed));

        refreshed.endpoint = "203.0.113.11:41642".to_string();
        assert!(!same_direct_udp_endpoint(&report, &refreshed));
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
                probe_targets: Vec::new(),
                last_rx_seq: 0,
                ready: false,
                probe_controller: PathProbeController::new(),
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
        assert_eq!(transport.peers[0].probe_targets.len(), 2);
    }

    #[test]
    fn direct_udp_probe_sends_to_every_candidate_until_one_responds() {
        let lan = UdpSocket::bind("127.0.0.1:0").unwrap();
        let public = UdpSocket::bind("127.0.0.1:0").unwrap();
        lan.set_read_timeout(Some(Duration::from_secs(1))).unwrap();
        public
            .set_read_timeout(Some(Duration::from_secs(1)))
            .unwrap();
        let candidate = |kind, address: SocketAddr| PathCandidate {
            kind,
            state: PathState::Probing,
            endpoint_id: None,
            address: Some(address.to_string()),
            session_id: None,
            transport: Some("udp".to_string()),
            rtt_ms: None,
            path_score: None,
            last_ok_at_ms: None,
            last_error: None,
        };
        let paths = vec![PeerPathConfig {
            peer_node_id: "node-a".to_string(),
            peer_virtual_ips: vec!["10.0.0.9".to_string()],
            candidates: vec![
                candidate(PathKind::LanUdp, lan.local_addr().unwrap()),
                candidate(PathKind::DirectUdp, public.local_addr().unwrap()),
            ],
        }];
        let mut transport = DirectUdpTransport::attach_with_socket("node-local", &paths, || {
            UdpSocket::bind("127.0.0.1:0")
        })
        .unwrap();

        assert_eq!(transport.send_probe_packets(), 1);
        let mut buffer = [0_u8; 256];
        assert!(lan.recv_from(&mut buffer).is_ok());
        assert!(public.recv_from(&mut buffer).is_ok());

        transport.mark_peer_ready(0, public.local_addr().unwrap());
        assert_eq!(transport.peers[0].path_kind, PathKind::DirectUdp);
        assert_eq!(transport.peers[0].socket_addr, public.local_addr().unwrap());
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

    #[test]
    fn direct_udp_peer_uses_burst_probes_before_expiring() {
        let paths = vec![PeerPathConfig {
            peer_node_id: "node-a".to_string(),
            peer_virtual_ips: vec!["10.0.0.9".to_string()],
            candidates: Vec::new(),
        }];
        let mut transport = DirectUdpTransport::attach_with_socket("node-local", &paths, || {
            UdpSocket::bind("127.0.0.1:0")
        })
        .unwrap();
        transport.peers[0].probe_controller.on_inbound(10_000);
        transport.peers[0].ready = true;

        assert!(transport.poll_probe_health(15_000).is_empty());
        transport.peers[0].probe_controller.on_probe_sent(15_000);
        for now_ms in [16_000, 17_000] {
            assert!(transport.poll_probe_health(now_ms).is_empty());
            transport.peers[0].probe_controller.on_probe_sent(now_ms);
        }
        assert_eq!(
            transport.poll_probe_health(18_000),
            vec!["node-a".to_string()]
        );
        assert!(!transport.peers[0].ready);
        assert!(transport.poll_probe_health(20_000).is_empty());
    }
}
