use relay_core::{RelayError, RelayTicket, TicketValidator};

mod fields;
mod signature;

pub use signature::HmacTicketSignatureValidator;

/// Relay 票据校验器。
///
/// 默认执行基本字段、前缀与过期校验；配置 `ticket_signing_secret` 后，
/// 会进一步按控制面相同的 HMAC-SHA256 规则验证签名。
#[derive(Debug, Clone, Default)]
pub struct StaticTicketValidator {
    required_relay_url_prefix: Option<String>,
    signature_validator: Option<HmacTicketSignatureValidator>,
    local_relay_node_ids: Vec<String>,
}

impl StaticTicketValidator {
    /// 创建带 relay URL 前缀约束的校验器。
    pub fn new(required_relay_url_prefix: impl Into<String>) -> Self {
        Self {
            required_relay_url_prefix: Some(required_relay_url_prefix.into()),
            signature_validator: None,
            local_relay_node_ids: Vec::new(),
        }
    }

    /// 创建同时带 relay URL 前缀和 HMAC secret 的校验器。
    pub fn with_hmac_secret(
        required_relay_url_prefix: Option<String>,
        ticket_signing_secret: impl Into<String>,
    ) -> Self {
        Self {
            required_relay_url_prefix,
            signature_validator: Some(HmacTicketSignatureValidator::new(ticket_signing_secret)),
            local_relay_node_ids: Vec::new(),
        }
    }

    /// 创建完整可配置的校验器。
    pub fn with_options(
        required_relay_url_prefix: Option<String>,
        ticket_signing_secret: Option<String>,
    ) -> Self {
        Self {
            required_relay_url_prefix,
            signature_validator: ticket_signing_secret.map(HmacTicketSignatureValidator::new),
            local_relay_node_ids: Vec::new(),
        }
    }

    pub fn with_allowed_relay_node_ids(
        required_relay_url_prefix: Option<String>,
        ticket_signing_secret: Option<String>,
        local_relay_node_ids: impl IntoIterator<Item = impl Into<String>>,
    ) -> Self {
        let mut validator = Self::with_options(required_relay_url_prefix, ticket_signing_secret);
        validator.local_relay_node_ids = local_relay_node_ids
            .into_iter()
            .map(Into::into)
            .map(|value| value.trim().to_string())
            .filter(|value| !value.is_empty())
            .collect();
        validator
    }

    fn validate_required_fields(&self, ticket: &RelayTicket) -> Result<(), RelayError> {
        fields::validate_required_fields(ticket)
    }

    fn validate_relay_prefix(&self, ticket: &RelayTicket) -> Result<(), RelayError> {
        fields::validate_relay_prefix(ticket, self.required_relay_url_prefix.as_deref())
    }

    fn validate_expiration(&self, ticket: &RelayTicket) -> Result<(), RelayError> {
        fields::validate_expiration(ticket)
    }

    fn validate_signature(&self, ticket: &RelayTicket) -> Result<(), RelayError> {
        match &self.signature_validator {
            Some(validator) => validator.validate(ticket),
            None => Ok(()),
        }
    }

    fn validate_allowed_relay_node(&self, ticket: &RelayTicket) -> Result<(), RelayError> {
        if self.local_relay_node_ids.is_empty() || ticket.allowed_derp_node_ids.is_empty() {
            return Ok(());
        }
        if ticket.allowed_derp_node_ids.iter().any(|allowed| {
            self.local_relay_node_ids
                .iter()
                .any(|local| local == allowed)
        }) {
            return Ok(());
        }
        Err(RelayError::InvalidTicket)
    }
}

impl TicketValidator for StaticTicketValidator {
    fn validate(&self, ticket: &RelayTicket) -> Result<(), RelayError> {
        self.validate_required_fields(ticket)?;
        self.validate_relay_prefix(ticket)?;
        self.validate_expiration(ticket)?;
        self.validate_signature(ticket)?;
        self.validate_allowed_relay_node(ticket)
    }
}

#[cfg(test)]
mod tests {
    use relay_core::{RelayTicket, TicketValidator};

    use super::StaticTicketValidator;

    #[test]
    fn accepts_ticket_when_allowed_relay_node_matches() {
        let mut ticket = test_ticket();
        ticket.allowed_derp_node_ids = vec!["relay-cn-a".to_string()];
        let validator =
            StaticTicketValidator::with_allowed_relay_node_ids(None, None, ["relay-cn-a"]);

        assert!(validator.validate(&ticket).is_ok());
    }

    #[test]
    fn rejects_ticket_when_allowed_relay_node_does_not_match() {
        let mut ticket = test_ticket();
        ticket.allowed_derp_node_ids = vec!["relay-cn-a".to_string()];
        let validator =
            StaticTicketValidator::with_allowed_relay_node_ids(None, None, ["relay-us-a"]);

        assert!(validator.validate(&ticket).is_err());
    }

    fn test_ticket() -> RelayTicket {
        RelayTicket {
            ticket_id: "ticket-test".to_string(),
            network_id: "net-test".to_string(),
            session_id: "session-test".to_string(),
            src_node_id: "node-a".to_string(),
            dst_node_id: "node-b".to_string(),
            derp_cluster_id: None,
            country_code: None,
            city_code: None,
            allowed_derp_node_ids: Vec::new(),
            relay_url: "udp://127.0.0.1:9000".to_string(),
            expires_at: "2099-01-01T00:00:00Z".to_string(),
            session_key: "session-key".to_string(),
            signature: "signature".to_string(),
        }
    }
}
