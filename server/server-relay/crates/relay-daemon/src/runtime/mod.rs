mod attach;
mod detach;
mod forward;

use std::collections::HashMap;
use std::net::SocketAddr;

use auth::StaticTicketValidator;
use session::InMemorySessionStore;
use udp_relay::UdpRelay;

use crate::protocol::{ClientRequest, ServerResponse};

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct RelayRuntimeError {
    pub code: &'static str,
    pub message: String,
}

impl RelayRuntimeError {
    pub(crate) fn new(code: &'static str, message: impl Into<String>) -> Self {
        Self {
            code,
            message: message.into(),
        }
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum RelayEndpoint {
    Udp(SocketAddr),
    Tcp(u64),
}

impl From<SocketAddr> for RelayEndpoint {
    fn from(value: SocketAddr) -> Self {
        Self::Udp(value)
    }
}

impl RelayEndpoint {
    pub fn udp_addr(self) -> Option<SocketAddr> {
        match self {
            Self::Udp(addr) => Some(addr),
            Self::Tcp(_) => None,
        }
    }
}

type SessionEndpointsV2 = HashMap<String, HashMap<String, RelayEndpoint>>;
type SourceIndexV2 = HashMap<RelayEndpoint, (String, String)>;

pub struct RelayRuntime {
    relay: UdpRelay<InMemorySessionStore, StaticTicketValidator>,
    endpoints: SessionEndpointsV2,
    source_index: SourceIndexV2,
}

impl RelayRuntime {
    pub fn new(relay_url_prefix: Option<String>, ticket_signing_secret: Option<String>) -> Self {
        let validator =
            StaticTicketValidator::with_options(relay_url_prefix, ticket_signing_secret);
        Self {
            relay: UdpRelay::new(InMemorySessionStore::new(), validator),
            endpoints: HashMap::new(),
            source_index: HashMap::new(),
        }
    }

    pub fn handle_request(
        &mut self,
        source: SocketAddr,
        request: ClientRequest,
    ) -> Result<(ServerResponse, Option<(SocketAddr, ServerResponse)>), RelayRuntimeError> {
        self.handle_request_from(source.into(), request)
            .map(|(response, peer)| {
                (
                    response,
                    peer.and_then(|(endpoint, packet)| {
                        endpoint.udp_addr().map(|addr| (addr, packet))
                    }),
                )
            })
    }

    pub fn handle_request_from(
        &mut self,
        source: RelayEndpoint,
        request: ClientRequest,
    ) -> Result<(ServerResponse, Option<(RelayEndpoint, ServerResponse)>), RelayRuntimeError> {
        match request {
            ClientRequest::Ping => Ok((ServerResponse::Pong, None)),
            ClientRequest::Attach {
                participant_id,
                ticket,
            } => self.handle_attach(source, participant_id, ticket),
            ClientRequest::Forward {
                session_id,
                from_participant_id,
                payload_b64,
            } => self.handle_forward(source, session_id, from_participant_id, payload_b64),
            ClientRequest::Detach {
                session_id,
                participant_id,
            } => self.handle_detach(source, session_id, participant_id),
        }
    }

    pub fn active_session_count(&self) -> usize {
        self.endpoints.len()
    }

    pub fn detach_endpoint(&mut self, endpoint: RelayEndpoint) {
        let Some((session_id, participant_id)) = self.source_index.remove(&endpoint) else {
            return;
        };
        let should_remove_session = self
            .endpoints
            .get_mut(&session_id)
            .map(|participants| {
                participants.remove(&participant_id);
                participants.is_empty()
            })
            .unwrap_or(false);
        if should_remove_session {
            self.endpoints.remove(&session_id);
            let _ = self.relay.detach(&session_id);
        }
    }
}

#[cfg(test)]
mod tests {
    use std::net::SocketAddr;

    use client_core::relay_frame as client_relay_frame;

    use crate::protocol::binary;
    use crate::protocol::{ClientRequest, RelayTicketWire, ServerResponse};

    use super::{RelayEndpoint, RelayRuntime};

    #[test]
    fn binary_data_frame_forwards_after_json_attach() {
        let mut runtime = RelayRuntime::new(None, None);
        let peer_a: SocketAddr = "127.0.0.1:31001".parse().unwrap();
        let peer_b: SocketAddr = "127.0.0.1:31002".parse().unwrap();
        let ticket = test_ticket();

        let (response, _) = runtime
            .handle_request(
                peer_a,
                ClientRequest::Attach {
                    participant_id: "node-a".to_string(),
                    ticket: ticket.clone(),
                },
            )
            .unwrap();
        assert!(matches!(response, ServerResponse::Attached(_)));
        let (response, _) = runtime
            .handle_request(
                peer_b,
                ClientRequest::Attach {
                    participant_id: "node-b".to_string(),
                    ticket,
                },
            )
            .unwrap();
        assert!(matches!(response, ServerResponse::Attached(_)));

        let frame = binary::encode_data_frame(11, 22, b"ip-packet").unwrap();
        let forwarded = runtime
            .handle_binary_data_frame(peer_a, &frame)
            .unwrap()
            .unwrap();
        let decoded = binary::decode_data_frame(&forwarded.1).unwrap();

        assert_eq!(forwarded.0, peer_b.into());
        assert_eq!(decoded.seq, 11);
        assert_eq!(decoded.config_hash, 22);
        assert_eq!(decoded.payload, b"ip-packet");
    }

    #[test]
    fn client_core_binary_frame_forwards_after_json_attach() {
        let mut runtime = RelayRuntime::new(None, None);
        let peer_a: SocketAddr = "127.0.0.1:31001".parse().unwrap();
        let peer_b: SocketAddr = "127.0.0.1:31002".parse().unwrap();
        let ticket = test_ticket();

        runtime
            .handle_request(
                peer_a,
                ClientRequest::Attach {
                    participant_id: "node-a".to_string(),
                    ticket: ticket.clone(),
                },
            )
            .unwrap();
        runtime
            .handle_request(
                peer_b,
                ClientRequest::Attach {
                    participant_id: "node-b".to_string(),
                    ticket,
                },
            )
            .unwrap();

        let frame = client_relay_frame::encode_slan_relay_data_frame(11, 22, b"ip-packet").unwrap();
        let forwarded = runtime
            .handle_binary_data_frame(peer_a, &frame)
            .unwrap()
            .unwrap();
        let decoded = binary::decode_data_frame(&forwarded.1).unwrap();

        assert_eq!(forwarded.0, peer_b.into());
        assert_eq!(decoded.seq, 11);
        assert_eq!(decoded.config_hash, 22);
        assert_eq!(decoded.payload, b"ip-packet");
    }

    #[test]
    fn daemon_and_client_core_binary_frame_encoders_are_wire_compatible() {
        let daemon_frame = binary::encode_data_frame(44, 55, b"ip-packet").unwrap();
        let client_frame =
            client_relay_frame::encode_slan_relay_data_frame(44, 55, b"ip-packet").unwrap();

        assert_eq!(daemon_frame, client_frame);
        assert_eq!(
            client_relay_frame::decode_slan_relay_data_frame(&daemon_frame),
            Some(b"ip-packet".as_slice())
        );
    }

    #[test]
    fn binary_data_frame_requires_attach_first() {
        let mut runtime = RelayRuntime::new(None, None);
        let peer: SocketAddr = "127.0.0.1:31001".parse().unwrap();
        let frame = binary::encode_data_frame(1, 2, b"packet").unwrap();

        let err = runtime.handle_binary_data_frame(peer, &frame).unwrap_err();

        assert_eq!(err.code, "participant_not_attached");
    }

    #[test]
    fn binary_data_frame_reports_peer_not_attached() {
        let mut runtime = RelayRuntime::new(None, None);
        let peer_a: SocketAddr = "127.0.0.1:31001".parse().unwrap();
        let ticket = test_ticket();

        runtime
            .handle_request(
                peer_a,
                ClientRequest::Attach {
                    participant_id: "node-a".to_string(),
                    ticket,
                },
            )
            .unwrap();

        let frame = binary::encode_data_frame(11, 22, b"ip-packet").unwrap();
        let err = runtime
            .handle_binary_data_frame(peer_a, &frame)
            .unwrap_err();

        assert_eq!(err.code, "peer_not_attached");
        assert_eq!(
            err.message,
            "target participant has not attached to relay session"
        );
    }

    #[test]
    fn reattach_participant_moves_udp_source_binding() {
        let mut runtime = RelayRuntime::new(None, None);
        let old_peer_a: SocketAddr = "127.0.0.1:31001".parse().unwrap();
        let new_peer_a: SocketAddr = "127.0.0.1:31003".parse().unwrap();
        let peer_b: SocketAddr = "127.0.0.1:31002".parse().unwrap();
        let ticket = test_ticket();

        runtime
            .handle_request(
                old_peer_a,
                ClientRequest::Attach {
                    participant_id: "node-a".to_string(),
                    ticket: ticket.clone(),
                },
            )
            .unwrap();
        runtime
            .handle_request(
                new_peer_a,
                ClientRequest::Attach {
                    participant_id: "node-a".to_string(),
                    ticket: ticket.clone(),
                },
            )
            .unwrap();
        runtime
            .handle_request(
                peer_b,
                ClientRequest::Attach {
                    participant_id: "node-b".to_string(),
                    ticket,
                },
            )
            .unwrap();

        let frame = binary::encode_data_frame(11, 22, b"ip-packet").unwrap();
        assert_eq!(
            runtime
                .handle_binary_data_frame(old_peer_a, &frame)
                .unwrap_err()
                .code,
            "participant_not_attached"
        );
        let forwarded = runtime
            .handle_binary_data_frame(new_peer_a, &frame)
            .unwrap()
            .unwrap();
        assert_eq!(forwarded.0, peer_b.into());
    }

    #[test]
    fn attach_rejects_source_reused_by_other_participant() {
        let mut runtime = RelayRuntime::new(None, None);
        let source: SocketAddr = "127.0.0.1:31001".parse().unwrap();
        let ticket = test_ticket();

        runtime
            .handle_request(
                source,
                ClientRequest::Attach {
                    participant_id: "node-a".to_string(),
                    ticket: ticket.clone(),
                },
            )
            .unwrap();
        let err = runtime
            .handle_request(
                source,
                ClientRequest::Attach {
                    participant_id: "node-b".to_string(),
                    ticket,
                },
            )
            .unwrap_err();

        assert_eq!(err.code, "source_already_attached");
    }

    #[test]
    fn detach_removes_source_binding_for_binary_frames() {
        let mut runtime = RelayRuntime::new(None, None);
        let peer_a: SocketAddr = "127.0.0.1:31001".parse().unwrap();
        let peer_b: SocketAddr = "127.0.0.1:31002".parse().unwrap();
        let ticket = test_ticket();

        runtime
            .handle_request(
                peer_a,
                ClientRequest::Attach {
                    participant_id: "node-a".to_string(),
                    ticket: ticket.clone(),
                },
            )
            .unwrap();
        runtime
            .handle_request(
                peer_b,
                ClientRequest::Attach {
                    participant_id: "node-b".to_string(),
                    ticket: ticket.clone(),
                },
            )
            .unwrap();
        assert_eq!(runtime.active_session_count(), 1);

        runtime
            .handle_request(
                peer_a,
                ClientRequest::Detach {
                    session_id: ticket.session_id.clone(),
                    participant_id: "node-a".to_string(),
                },
            )
            .unwrap();

        let frame = binary::encode_data_frame(11, 22, b"ip-packet").unwrap();
        let err = runtime
            .handle_binary_data_frame(peer_a, &frame)
            .unwrap_err();

        assert_eq!(err.code, "participant_not_attached");
        assert_eq!(runtime.active_session_count(), 1);
    }

    #[test]
    fn detach_last_participant_removes_active_session() {
        let mut runtime = RelayRuntime::new(None, None);
        let peer_a: SocketAddr = "127.0.0.1:31001".parse().unwrap();
        let peer_b: SocketAddr = "127.0.0.1:31002".parse().unwrap();
        let ticket = test_ticket();

        runtime
            .handle_request(
                peer_a,
                ClientRequest::Attach {
                    participant_id: "node-a".to_string(),
                    ticket: ticket.clone(),
                },
            )
            .unwrap();
        runtime
            .handle_request(
                peer_b,
                ClientRequest::Attach {
                    participant_id: "node-b".to_string(),
                    ticket: ticket.clone(),
                },
            )
            .unwrap();

        runtime
            .handle_request(
                peer_a,
                ClientRequest::Detach {
                    session_id: ticket.session_id.clone(),
                    participant_id: "node-a".to_string(),
                },
            )
            .unwrap();
        runtime
            .handle_request(
                peer_b,
                ClientRequest::Detach {
                    session_id: ticket.session_id,
                    participant_id: "node-b".to_string(),
                },
            )
            .unwrap();

        assert_eq!(runtime.active_session_count(), 0);
    }

    #[test]
    fn detach_endpoint_removes_tcp_binding() {
        let mut runtime = RelayRuntime::new(None, None);
        let tcp_a = RelayEndpoint::Tcp(1);
        let tcp_b = RelayEndpoint::Tcp(2);
        let ticket = test_ticket();

        runtime
            .handle_request_from(
                tcp_a,
                ClientRequest::Attach {
                    participant_id: "node-a".to_string(),
                    ticket: ticket.clone(),
                },
            )
            .unwrap();
        runtime
            .handle_request_from(
                tcp_b,
                ClientRequest::Attach {
                    participant_id: "node-b".to_string(),
                    ticket,
                },
            )
            .unwrap();
        assert_eq!(runtime.active_session_count(), 1);

        runtime.detach_endpoint(tcp_a);
        let frame = binary::encode_data_frame(11, 22, b"ip-packet").unwrap();
        let err = runtime.handle_binary_data_frame(tcp_a, &frame).unwrap_err();

        assert_eq!(err.code, "participant_not_attached");
        assert_eq!(runtime.active_session_count(), 1);
    }

    fn test_ticket() -> RelayTicketWire {
        RelayTicketWire {
            ticket_id: "ticket-test".to_string(),
            network_id: "net-test".to_string(),
            session_id: "relay-session-test".to_string(),
            src_node_id: "node-a".to_string(),
            dst_node_id: "node-b".to_string(),
            derp_cluster_id: None,
            country_code: None,
            city_code: None,
            allowed_derp_node_ids: Vec::new(),
            relay_url: "udp://127.0.0.1:9100".to_string(),
            expires_at: "2099-01-01T00:00:00Z".to_string(),
            session_key: "session-key-test".to_string(),
            signature: "test-signature".to_string(),
        }
    }
}
