use relay_core::{RelayError, RelayTicket, TicketValidator};

/// 静态票据校验器。
///
/// 适合 Phase 1 或本地开发场景，做最基本的字段、前缀与过期校验。
#[derive(Debug, Clone)]
pub struct StaticTicketValidator {
    required_relay_url_prefix: Option<String>,
}

impl StaticTicketValidator {
    /// 创建带 relay URL 前缀约束的校验器。
    pub fn new(required_relay_url_prefix: impl Into<String>) -> Self {
        Self {
            required_relay_url_prefix: Some(required_relay_url_prefix.into()),
        }
    }
}

impl Default for StaticTicketValidator {
    fn default() -> Self {
        Self {
            required_relay_url_prefix: None,
        }
    }
}

impl TicketValidator for StaticTicketValidator {
    fn validate(&self, ticket: &RelayTicket) -> Result<(), RelayError> {
        if ticket.ticket_id.trim().is_empty()
            || ticket.signature.trim().is_empty()
            || ticket.relay_url.trim().is_empty()
            || ticket.expires_at.trim().is_empty()
        {
            return Err(RelayError::InvalidTicket);
        }

        if let Some(prefix) = &self.required_relay_url_prefix {
            if !ticket.relay_url.starts_with(prefix) {
                return Err(RelayError::InvalidTicket);
            }
        }

        if ticket.is_expired()? {
            return Err(RelayError::TicketExpired);
        }

        Ok(())
    }
}
