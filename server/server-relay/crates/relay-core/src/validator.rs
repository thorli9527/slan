use crate::{RelayError, RelayTicket};

/// Relay 票据校验器接口。
pub trait TicketValidator: Send + Sync {
    fn validate(&self, ticket: &RelayTicket) -> Result<(), RelayError>;
}
