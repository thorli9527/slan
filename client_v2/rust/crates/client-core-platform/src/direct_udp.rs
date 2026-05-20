use std::{
    fs,
    net::{SocketAddr, ToSocketAddrs, UdpSocket},
    path::PathBuf,
    time::{Duration, SystemTime, UNIX_EPOCH},
};

use anyhow::{Context, Result};
use client_core::{
    ipv4_source, normalize_virtual_ip, relay_frame::decode_slan_relay_data_frame_full,
    relay_peer_index_for_packet, PathKind, PeerPathConfig,
};
use serde::Serialize;

pub const DIRECT_UDP_PROBE_PACKET: &[u8] = b"slan-direct-udp-probe-v1";
pub const DIRECT_UDP_PONG_PACKET: &[u8] = b"slan-direct-udp-pong-v1";

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum DirectUdpControlKind {
    Probe,
    Pong,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct DirectUdpControlPacket {
    pub kind: DirectUdpControlKind,
    pub node_id: Option<String>,
}

impl DirectUdpControlPacket {
    pub fn peer_node_id(&self) -> Option<&str> {
        self.node_id.as_deref().filter(|value| !value.is_empty())
    }
}

#[derive(Debug)]
pub struct DirectUdpPeer {
    pub peer_node_id: String,
    pub peer_virtual_ips: Vec<String>,
    pub path_kind: PathKind,
    pub address: String,
    pub socket_addr: SocketAddr,
    pub last_rx_seq: u64,
    pub ready: bool,
}

#[derive(Debug)]
pub struct DirectUdpTransport {
    pub socket: UdpSocket,
    pub local_node_id: String,
    pub peers: Vec<DirectUdpPeer>,
}

#[derive(Debug, Clone, Copy)]
pub struct DirectUdpReceive {
    pub peer_index: usize,
    pub frame_len: usize,
    pub remote_addr: SocketAddr,
    pub endpoint_changed: bool,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
struct DirectUdpEndpointReport {
    endpoint: String,
    endpoint_type: String,
    nat_type: String,
    bind_address: String,
    updated_at_ms: u64,
}

impl DirectUdpTransport {
    pub fn attach(local_node_id: &str, configured_paths: &[PeerPathConfig]) -> Option<Self> {
        Self::attach_with_socket(local_node_id, configured_paths, || {
            UdpSocket::bind("0.0.0.0:0")
        })
    }

    pub fn attach_with_socket(
        local_node_id: &str,
        configured_paths: &[PeerPathConfig],
        socket_factory: impl FnOnce() -> std::io::Result<UdpSocket>,
    ) -> Option<Self> {
        let socket = match socket_factory() {
            Ok(socket) => socket,
            Err(error) => {
                eprintln!("client-core-platform direct udp attach skipped error={error:#}");
                clear_direct_udp_endpoint_report();
                return None;
            }
        };
        let mut peers = Vec::new();
        for path in configured_paths {
            let Some((path_kind, address)) = udp_address_for_peer(path) else {
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
                    eprintln!(
                        "client-core-platform direct udp attach skipped peer={} error={error:#}",
                        path.peer_node_id
                    );
                }
            }
        }
        if peers.is_empty() {
            clear_direct_udp_endpoint_report();
            return None;
        }
        if socket.set_nonblocking(true).is_err() {
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

    pub fn ready_peer_index_for_packet(&self, payload: &[u8]) -> Option<usize> {
        self.peer_index_for_packet(payload)
            .filter(|index| self.peers.get(*index).is_some_and(|peer| peer.ready))
    }

    pub fn send_to_peer(&self, peer_index: usize, frame: &[u8]) -> std::io::Result<usize> {
        let Some(peer) = self.peers.get(peer_index) else {
            return Err(std::io::Error::new(
                std::io::ErrorKind::NotFound,
                "direct udp peer not found",
            ));
        };
        self.socket.send_to(frame, peer.socket_addr)
    }

    pub fn recv_from_peer(
        &mut self,
        buffer: &mut [u8],
    ) -> std::io::Result<Option<DirectUdpReceive>> {
        let (frame_len, remote_addr) = self.socket.recv_from(buffer)?;
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

    pub fn send_probe_packets(&self) {
        let payload = direct_udp_control_payload(DirectUdpControlKind::Probe, &self.local_node_id);
        for peer in &self.peers {
            let _ = self.socket.send_to(payload.as_bytes(), peer.socket_addr);
        }
    }

    pub fn send_pong_to_peer(&self, peer_index: usize) {
        if let Some(peer) = self.peers.get(peer_index) {
            let payload =
                direct_udp_control_payload(DirectUdpControlKind::Pong, &self.local_node_id);
            let _ = self.socket.send_to(payload.as_bytes(), peer.socket_addr);
        }
    }

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
    for path_kind in [PathKind::LanUdp, PathKind::Ipv6Udp, PathKind::DirectUdp] {
        if let Some(address) = path.candidates.iter().find_map(|candidate| {
            (candidate.kind == path_kind)
                .then(|| candidate.address.as_deref())
                .flatten()
                .map(str::trim)
                .filter(|value| !value.is_empty())
                .map(str::to_string)
        }) {
            return Some((path_kind, address));
        }
    }
    None
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
    app_data_dir()
        .join("SLAN")
        .join("client-v2-direct-udp-endpoint.json")
}

pub fn persist_direct_udp_endpoint_report(socket: &UdpSocket) {
    let Ok(local_addr) = socket.local_addr() else {
        return;
    };
    let report_host = std::env::var("SLAN_DIRECT_UDP_PUBLIC_HOST")
        .ok()
        .map(|value| value.trim().to_string())
        .filter(|value| !value.is_empty())
        .or_else(direct_udp_lan_host)
        .unwrap_or_else(|| local_addr.ip().to_string());
    let report = DirectUdpEndpointReport {
        endpoint: format!("{report_host}:{}", local_addr.port()),
        endpoint_type: std::env::var("SLAN_DIRECT_UDP_ENDPOINT_TYPE")
            .ok()
            .map(|value| value.trim().to_string())
            .filter(|value| !value.is_empty())
            .unwrap_or_else(|| "lan".to_string()),
        nat_type: std::env::var("SLAN_DIRECT_UDP_NAT_TYPE")
            .ok()
            .map(|value| value.trim().to_string())
            .filter(|value| !value.is_empty())
            .unwrap_or_else(|| "unknown".to_string()),
        bind_address: local_addr.to_string(),
        updated_at_ms: current_timestamp_ms(),
    };
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
    let path = direct_udp_endpoint_file_path();
    if path.exists() {
        let _ = fs::remove_file(path);
    }
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
}
