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

    pub fn active_session_count(&self) -> usize {
        self.endpoints.len()
    }
}
