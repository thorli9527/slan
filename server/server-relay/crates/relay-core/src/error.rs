use std::error::Error;
use std::fmt::{Display, Formatter};

/// Relay 领域错误定义。
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum RelayError {
    InvalidTicket,
    InvalidTimestamp,
    TicketExpired,
    SessionNotFound,
    SessionAlreadyExists,
    UnauthorizedPeer,
    EmptyPayload,
    ClockSkew,
    Store(String),
}

impl Display for RelayError {
    fn fmt(&self, f: &mut Formatter<'_>) -> std::fmt::Result {
        match self {
            RelayError::InvalidTicket => f.write_str("invalid relay ticket"),
            RelayError::InvalidTimestamp => f.write_str("invalid timestamp"),
            RelayError::TicketExpired => f.write_str("relay ticket expired"),
            RelayError::SessionNotFound => f.write_str("relay session not found"),
            RelayError::SessionAlreadyExists => f.write_str("relay session already exists"),
            RelayError::UnauthorizedPeer => {
                f.write_str("device is not a participant of the relay session")
            }
            RelayError::EmptyPayload => f.write_str("relay payload is empty"),
            RelayError::ClockSkew => f.write_str("system clock is before unix epoch"),
            RelayError::Store(message) => write!(f, "session store error: {message}"),
        }
    }
}

impl Error for RelayError {}
