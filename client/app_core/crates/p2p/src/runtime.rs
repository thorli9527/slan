use std::io::ErrorKind;
use std::net::{TcpStream, ToSocketAddrs};
use std::sync::Mutex;
use std::time::Duration;

use slan_app_core::{ConnectionPath, ConnectionState};

use crate::{P2PConnector, PeerCandidate};

pub struct SocketP2PConnector {
    connect_timeout: Duration,
    state: Mutex<P2PState>,
}

#[derive(Default)]
struct P2PState {
    active_stream: Option<TcpStream>,
    peer_node_id: Option<String>,
}

impl SocketP2PConnector {
    pub fn new(connect_timeout: Duration) -> Self {
        Self {
            connect_timeout,
            state: Mutex::new(P2PState::default()),
        }
    }

    fn connect_tcp(&self, endpoint: &str) -> Result<TcpStream, String> {
        let addr = endpoint
            .to_socket_addrs()
            .map_err(|err| format!("resolve tcp endpoint {endpoint}: {err}"))?
            .next()
            .ok_or_else(|| format!("no tcp address resolved for {endpoint}"))?;
        TcpStream::connect_timeout(&addr, self.connect_timeout)
            .map_err(|err| format!("tcp connect to {endpoint} failed: {err}"))
    }
}

impl Default for SocketP2PConnector {
    fn default() -> Self {
        Self::new(Duration::from_secs(1))
    }
}

impl P2PConnector for SocketP2PConnector {
    fn connect(&self, peer: &PeerCandidate) -> Result<ConnectionState, String> {
        match peer.candidate_type.as_str() {
            "tcp" => match self.connect_tcp(&peer.endpoint) {
                Ok(stream) => {
                    let mut state = self
                        .state
                        .lock()
                        .map_err(|_| "p2p state poisoned".to_string())?;
                    state.active_stream = Some(stream);
                    state.peer_node_id = Some(peer.peer_node_id.clone());
                    Ok(ConnectionState::Connected(ConnectionPath::P2P))
                }
                Err(reason) => Ok(ConnectionState::Failed(reason)),
            },
            "udp" => Ok(ConnectionState::Failed(format!(
                "udp p2p connect not implemented for {} via {}",
                peer.peer_node_id, peer.endpoint
            ))),
            other => Ok(ConnectionState::Failed(format!(
                "unsupported p2p candidate type for {}: {}",
                peer.peer_node_id, other
            ))),
        }
    }

    fn send_transport_packet(&self, packet: &[u8]) -> Result<(), String> {
        if packet.is_empty() {
            return Err("cannot send empty packet via p2p".to_string());
        }
        let mut state = self
            .state
            .lock()
            .map_err(|_| "p2p state poisoned".to_string())?;
        let stream = state
            .active_stream
            .as_mut()
            .ok_or_else(|| "p2p connector has no active connection".to_string())?;
        use std::io::Write;
        stream
            .write_all(packet)
            .map_err(|err| format!("send packet to p2p socket failed: {err}"))
    }

    fn poll_transport_packet(&self) -> Result<Option<Vec<u8>>, String> {
        let mut state = self
            .state
            .lock()
            .map_err(|_| "p2p state poisoned".to_string())?;
        let stream = state
            .active_stream
            .as_mut()
            .ok_or_else(|| "p2p connector has no active connection".to_string())?;
        use std::io::Read;
        stream
            .set_nonblocking(true)
            .map_err(|err| format!("set p2p socket nonblocking failed: {err}"))?;
        let mut buf = vec![0_u8; 2048];
        let result = match stream.read(&mut buf) {
            Ok(0) => Ok(None),
            Ok(n) => {
                buf.truncate(n);
                Ok(Some(buf))
            }
            Err(err) if err.kind() == ErrorKind::WouldBlock => Ok(None),
            Err(err) => Err(format!("read p2p socket failed: {err}")),
        };
        let _ = stream.set_nonblocking(false);
        result
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn invalid_tcp_candidate_reports_failed_state() {
        let connector = SocketP2PConnector::default();
        let result = connector
            .connect(&PeerCandidate {
                peer_node_id: "peer-1".into(),
                endpoint: "127.0.0.1:1".into(),
                candidate_type: "tcp".into(),
            })
            .unwrap();

        assert!(matches!(result, ConnectionState::Failed(_)));
    }

    #[test]
    fn unsupported_candidate_type_fails() {
        let connector = SocketP2PConnector::default();
        let result = connector
            .connect(&PeerCandidate {
                peer_node_id: "peer-1".into(),
                endpoint: "127.0.0.1:1".into(),
                candidate_type: "quic".into(),
            })
            .unwrap();

        assert!(
            matches!(result, ConnectionState::Failed(reason) if reason.contains("unsupported p2p candidate type"))
        );
    }
}
