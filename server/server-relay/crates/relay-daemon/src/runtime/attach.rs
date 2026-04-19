use std::net::SocketAddr;

use relay_core::{RelayError, RelaySession};

use crate::errors::{relay_runtime_error, to_relay_ticket};
use crate::protocol::{AttachAck, RelayTicketWire, ServerResponse};

use super::{RelayRuntime, RelayRuntimeError};

impl RelayRuntime {
    pub(super) fn handle_attach(
        &mut self,
        source: SocketAddr,
        participant_id: String,
        ticket: RelayTicketWire,
    ) -> Result<(ServerResponse, Option<(SocketAddr, ServerResponse)>), RelayRuntimeError> {
        let ticket = to_relay_ticket(ticket);
        if participant_id != ticket.src_node_id && participant_id != ticket.dst_node_id {
            return Err(RelayRuntimeError::new(
                "participant_not_in_ticket",
                "participant is not part of relay ticket",
            ));
        }

        let session = RelaySession {
            session_id: ticket.session_id.clone(),
            source_device_id: ticket.src_node_id.clone(),
            target_device_id: ticket.dst_node_id.clone(),
            network_id: ticket.network_id.clone(),
        };

        match self.relay.attach_with_ticket(&ticket, session.clone()) {
            Ok(()) => {}
            Err(RelayError::SessionAlreadyExists) => {
                let existing = self
                    .relay
                    .session(&ticket.session_id)
                    .map_err(relay_runtime_error)?;
                if existing != session {
                    return Err(RelayRuntimeError::new(
                        "session_conflict",
                        "relay session already exists with different participants",
                    ));
                }
            }
            Err(err) => return Err(relay_runtime_error(err)),
        }

        self.endpoints
            .entry(ticket.session_id.clone())
            .or_default()
            .insert(participant_id.clone(), source);

        let peer_participant_id = if participant_id == ticket.src_node_id {
            ticket.dst_node_id
        } else {
            ticket.src_node_id
        };

        Ok((
            ServerResponse::Attached(AttachAck {
                session_id: ticket.session_id,
                peer_participant_id,
            }),
            None,
        ))
    }
}
