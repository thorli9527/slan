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
}

impl StaticTicketValidator {
    /// 创建带 relay URL 前缀约束的校验器。
    pub fn new(required_relay_url_prefix: impl Into<String>) -> Self {
        Self {
            required_relay_url_prefix: Some(required_relay_url_prefix.into()),
            signature_validator: None,
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
        }
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
}

impl TicketValidator for StaticTicketValidator {
    fn validate(&self, ticket: &RelayTicket) -> Result<(), RelayError> {
        self.validate_required_fields(ticket)?;
        self.validate_relay_prefix(ticket)?;
        self.validate_expiration(ticket)?;
        self.validate_signature(ticket)
    }
}
