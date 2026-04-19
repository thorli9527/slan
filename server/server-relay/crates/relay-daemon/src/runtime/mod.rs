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

type SessionEndpoints = HashMap<String, HashMap<String, SocketAddr>>;

pub struct RelayRuntime {
    relay: UdpRelay<InMemorySessionStore, StaticTicketValidator>,
    endpoints: SessionEndpoints,
}

impl RelayRuntime {
    pub fn new(relay_url_prefix: Option<String>, ticket_signing_secret: Option<String>) -> Self {
        let validator =
            StaticTicketValidator::with_options(relay_url_prefix, ticket_signing_secret);
        Self {
            relay: UdpRelay::new(InMemorySessionStore::new(), validator),
            endpoints: HashMap::new(),
        }
    }

    pub fn handle_request(
        &mut self,
        source: SocketAddr,
        request: ClientRequest,
    ) -> Result<(ServerResponse, Option<(SocketAddr, ServerResponse)>), RelayRuntimeError> {
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
}

#[cfg(test)]
mod tests {
    use super::*;
    use base64::{engine::general_purpose::STANDARD, Engine as _};

    use crate::protocol::{ForwardAck, RelayPacketMessage, RelayTicketWire};

    fn sample_ticket() -> RelayTicketWire {
        RelayTicketWire {
            ticket_id: "ticket-1".into(),
            network_id: "net-1".into(),
            session_id: "session-1".into(),
            src_node_id: "node-a".into(),
            dst_node_id: "node-b".into(),
            derp_cluster_id: None,
            country_code: None,
            city_code: None,
            allowed_derp_node_ids: vec![],
            relay_url: "udp://127.0.0.1:9000".into(),
            expires_at: "4102444800".into(),
            session_key: "session-key-1".into(),
            signature: "sig".into(),
        }
    }

    #[test]
    fn attach_then_forward_emits_packet_to_peer() {
        let mut runtime = RelayRuntime::new(Some("udp://".into()), None);
        let a: SocketAddr = "127.0.0.1:30001".parse().unwrap();
        let b: SocketAddr = "127.0.0.1:30002".parse().unwrap();

        runtime
            .handle_request(
                a,
                ClientRequest::Attach {
                    participant_id: "node-a".into(),
                    ticket: sample_ticket(),
                },
            )
            .unwrap();
        runtime
            .handle_request(
                b,
                ClientRequest::Attach {
                    participant_id: "node-b".into(),
                    ticket: sample_ticket(),
                },
            )
            .unwrap();

        let (ack, peer_packet) = runtime
            .handle_request(
                a,
                ClientRequest::Forward {
                    session_id: "session-1".into(),
                    from_participant_id: "node-a".into(),
                    payload_b64: STANDARD.encode(b"ping"),
                },
            )
            .unwrap();

        assert_eq!(
            ack,
            ServerResponse::Forwarded(ForwardAck {
                session_id: "session-1".into(),
                to_participant_id: "node-b".into(),
                bytes_forwarded: 4,
            })
        );
        assert_eq!(
            peer_packet,
            Some((
                b,
                ServerResponse::Packet(RelayPacketMessage {
                    session_id: "session-1".into(),
                    from_participant_id: "node-a".into(),
                    payload_b64: STANDARD.encode(b"ping"),
                })
            ))
        );
    }

    #[test]
    fn forward_rejects_sender_bound_to_other_address() {
        let mut runtime = RelayRuntime::new(Some("udp://".into()), None);
        let a: SocketAddr = "127.0.0.1:30001".parse().unwrap();
        let rogue: SocketAddr = "127.0.0.1:30099".parse().unwrap();

        runtime
            .handle_request(
                a,
                ClientRequest::Attach {
                    participant_id: "node-a".into(),
                    ticket: sample_ticket(),
                },
            )
            .unwrap();

        let err = runtime
            .handle_request(
                rogue,
                ClientRequest::Forward {
                    session_id: "session-1".into(),
                    from_participant_id: "node-a".into(),
                    payload_b64: STANDARD.encode(b"ping"),
                },
            )
            .unwrap_err();

        assert_eq!(
            err,
            RelayRuntimeError::new(
                "participant_address_mismatch",
                "participant is bound to a different udp address",
            )
        );
    }

    #[test]
    fn detach_last_participant_removes_session() {
        let mut runtime = RelayRuntime::new(Some("udp://".into()), None);
        let a: SocketAddr = "127.0.0.1:30001".parse().unwrap();

        runtime
            .handle_request(
                a,
                ClientRequest::Attach {
                    participant_id: "node-a".into(),
                    ticket: sample_ticket(),
                },
            )
            .unwrap();
        runtime
            .handle_request(
                a,
                ClientRequest::Detach {
                    session_id: "session-1".into(),
                    participant_id: "node-a".into(),
                },
            )
            .unwrap();

        let err = runtime
            .handle_request(
                a,
                ClientRequest::Forward {
                    session_id: "session-1".into(),
                    from_participant_id: "node-a".into(),
                    payload_b64: STANDARD.encode(b"ping"),
                },
            )
            .unwrap_err();

        assert_eq!(
            err,
            RelayRuntimeError::new(
                "session_not_attached",
                "relay session has no attached participants",
            )
        );
    }
}
