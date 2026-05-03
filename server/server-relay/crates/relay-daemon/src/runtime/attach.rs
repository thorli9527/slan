use relay_core::{RelayError, RelaySession};

use crate::errors::{relay_runtime_error, to_relay_ticket};
use crate::protocol::{AttachAck, RelayTicketWire, ServerResponse};

use super::{RelayEndpoint, RelayRuntime, RelayRuntimeError};

impl RelayRuntime {
    pub(super) fn handle_attach(
        &mut self,
        source: RelayEndpoint,
        participant_id: String,
        transport: Option<String>,
        ticket: RelayTicketWire,
    ) -> Result<(ServerResponse, Option<(RelayEndpoint, ServerResponse)>), RelayRuntimeError> {
        let ticket = to_relay_ticket(ticket);
        validate_attach_transport(source, transport.as_deref(), ticket.relay_url.as_str())?;
        if participant_id != ticket.src_node_id && participant_id != ticket.dst_node_id {
            return Err(RelayRuntimeError::new(
                "participant_not_in_ticket",
                "participant is not part of relay ticket",
            ));
        }
        if let Some((existing_session_id, existing_participant_id)) =
            self.source_index.get(&source).cloned()
        {
            if existing_session_id != ticket.session_id || existing_participant_id != participant_id
            {
                return Err(RelayRuntimeError::new(
                    "source_already_attached",
                    "relay source is already attached to another relay participant",
                ));
            }
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

        let participants = self.endpoints.entry(ticket.session_id.clone()).or_default();
        if let Some(previous_source) = participants.insert(participant_id.clone(), source) {
            if previous_source != source {
                self.source_index.remove(&previous_source);
            }
        }
        self.remove_participant_replay(&ticket.session_id, &participant_id);
        self.source_index
            .insert(source, (ticket.session_id.clone(), participant_id.clone()));

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

fn validate_attach_transport(
    source: RelayEndpoint,
    transport: Option<&str>,
    relay_url: &str,
) -> Result<(), RelayRuntimeError> {
    let Some(transport) = transport.map(str::trim).filter(|value| !value.is_empty()) else {
        return Ok(());
    };
    let normalized = transport.to_ascii_lowercase();
    let supported = match source {
        RelayEndpoint::Udp(_) => matches!(normalized.as_str(), "udp" | "relay_udp"),
        RelayEndpoint::Tcp(_) => matches!(
            normalized.as_str(),
            "tcp" | "relay_tcp" | "tls" | "relay_tls" | "http3" | "relay_http3"
        ),
    };
    if supported {
        validate_attach_transport_matches_ticket_url(normalized.as_str(), relay_url)?;
        return Ok(());
    }
    Err(RelayRuntimeError::new(
        "unsupported_transport",
        format!("attach transport {transport} is not supported for this relay endpoint"),
    ))
}

fn validate_attach_transport_matches_ticket_url(
    normalized_transport: &str,
    relay_url: &str,
) -> Result<(), RelayRuntimeError> {
    let scheme = relay_url
        .trim()
        .split_once("://")
        .map(|(scheme, _)| scheme.to_ascii_lowercase());
    let Some(scheme) = scheme else {
        return Ok(());
    };
    let expected = match normalized_transport {
        "udp" | "relay_udp" => "udp",
        "tcp" | "relay_tcp" => "tcp",
        "tls" | "relay_tls" => "tls",
        "http3" | "relay_http3" => "http3",
        _ => return Ok(()),
    };
    if scheme == expected {
        return Ok(());
    }
    Err(RelayRuntimeError::new(
        "transport_ticket_mismatch",
        format!(
            "attach transport {normalized_transport} does not match ticket relayUrl {relay_url}"
        ),
    ))
}
