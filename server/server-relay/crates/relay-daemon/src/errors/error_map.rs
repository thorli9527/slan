use relay_core::RelayError;

use crate::runtime::RelayRuntimeError;

pub fn relay_runtime_error(err: RelayError) -> RelayRuntimeError {
    match err {
        RelayError::InvalidTicket => {
            RelayRuntimeError::new("invalid_ticket", RelayError::InvalidTicket.to_string())
        }
        RelayError::InvalidSignature => RelayRuntimeError::new(
            "invalid_signature",
            RelayError::InvalidSignature.to_string(),
        ),
        RelayError::InvalidTimestamp => RelayRuntimeError::new(
            "invalid_timestamp",
            RelayError::InvalidTimestamp.to_string(),
        ),
        RelayError::TicketExpired => {
            RelayRuntimeError::new("ticket_expired", RelayError::TicketExpired.to_string())
        }
        RelayError::SessionNotFound => {
            RelayRuntimeError::new("session_not_found", RelayError::SessionNotFound.to_string())
        }
        RelayError::SessionAlreadyExists => RelayRuntimeError::new(
            "session_already_exists",
            RelayError::SessionAlreadyExists.to_string(),
        ),
        RelayError::UnauthorizedPeer => RelayRuntimeError::new(
            "unauthorized_peer",
            RelayError::UnauthorizedPeer.to_string(),
        ),
        RelayError::EmptyPayload => {
            RelayRuntimeError::new("empty_payload", RelayError::EmptyPayload.to_string())
        }
        RelayError::ClockSkew => {
            RelayRuntimeError::new("clock_skew", RelayError::ClockSkew.to_string())
        }
        RelayError::Store(message) => {
            RelayRuntimeError::new("store_error", format!("session store error: {message}"))
        }
    }
}
