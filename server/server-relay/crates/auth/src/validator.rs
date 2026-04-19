use base64::{engine::general_purpose::URL_SAFE_NO_PAD, Engine as _};
use hmac::{Hmac, Mac};
use relay_core::{RelayError, RelayTicket, TicketValidator};
use sha2::Sha256;

type HmacSha256 = Hmac<Sha256>;

/// Relay 票据校验器。
///
/// 默认执行基本字段、前缀与过期校验；配置 `ticket_signing_secret` 后，
/// 会进一步按控制面相同的 HMAC-SHA256 规则验证签名。
#[derive(Debug, Clone, Default)]
pub struct StaticTicketValidator {
    required_relay_url_prefix: Option<String>,
    ticket_signing_secret: Option<String>,
}

impl StaticTicketValidator {
    /// 创建带 relay URL 前缀约束的校验器。
    pub fn new(required_relay_url_prefix: impl Into<String>) -> Self {
        Self {
            required_relay_url_prefix: Some(required_relay_url_prefix.into()),
            ticket_signing_secret: None,
        }
    }

    /// 创建同时带 relay URL 前缀和 HMAC secret 的校验器。
    pub fn with_hmac_secret(
        required_relay_url_prefix: Option<String>,
        ticket_signing_secret: impl Into<String>,
    ) -> Self {
        Self {
            required_relay_url_prefix,
            ticket_signing_secret: Some(ticket_signing_secret.into()),
        }
    }

    /// 创建完整可配置的校验器。
    pub fn with_options(
        required_relay_url_prefix: Option<String>,
        ticket_signing_secret: Option<String>,
    ) -> Self {
        Self {
            required_relay_url_prefix,
            ticket_signing_secret,
        }
    }

    fn validate_signature(&self, ticket: &RelayTicket) -> Result<(), RelayError> {
        let Some(secret) = &self.ticket_signing_secret else {
            return Ok(());
        };
        let mut mac =
            HmacSha256::new_from_slice(secret.as_bytes()).map_err(|_| RelayError::InvalidTicket)?;
        mac.update(ticket.signing_payload().as_bytes());
        let expected = URL_SAFE_NO_PAD.encode(mac.finalize().into_bytes());
        if expected != ticket.signature {
            return Err(RelayError::InvalidSignature);
        }
        Ok(())
    }
}

impl TicketValidator for StaticTicketValidator {
    fn validate(&self, ticket: &RelayTicket) -> Result<(), RelayError> {
        if ticket.ticket_id.trim().is_empty()
            || ticket.signature.trim().is_empty()
            || ticket.relay_url.trim().is_empty()
            || ticket.expires_at.trim().is_empty()
            || ticket.network_id.trim().is_empty()
            || ticket.session_id.trim().is_empty()
            || ticket.src_node_id.trim().is_empty()
            || ticket.dst_node_id.trim().is_empty()
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

        self.validate_signature(ticket)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn sample_ticket() -> RelayTicket {
        RelayTicket {
            ticket_id: "ticket-1".into(),
            network_id: "net-1".into(),
            session_id: "session-1".into(),
            src_node_id: "node-a".into(),
            dst_node_id: "node-b".into(),
            derp_cluster_id: Some("cluster-a".into()),
            country_code: Some("CN".into()),
            city_code: Some("SHA".into()),
            allowed_derp_node_ids: vec!["derp-b".into(), "derp-a".into()],
            relay_url: "udp://127.0.0.1:9000".into(),
            expires_at: "4102444800".into(),
            session_key: "session-key-1".into(),
            signature: String::new(),
        }
    }

    fn sign_ticket(secret: &str, ticket: &RelayTicket) -> String {
        let mut mac = HmacSha256::new_from_slice(secret.as_bytes()).unwrap();
        mac.update(ticket.signing_payload().as_bytes());
        URL_SAFE_NO_PAD.encode(mac.finalize().into_bytes())
    }

    #[test]
    fn validates_hmac_signed_ticket() {
        let mut ticket = sample_ticket();
        ticket.signature = sign_ticket("secret-1", &ticket);

        let validator =
            StaticTicketValidator::with_hmac_secret(Some("udp://".to_string()), "secret-1");
        assert_eq!(validator.validate(&ticket), Ok(()));
    }

    #[test]
    fn rejects_invalid_hmac_signature() {
        let mut ticket = sample_ticket();
        ticket.signature = "bad-signature".into();

        let validator = StaticTicketValidator::with_hmac_secret(None, "secret-1");
        assert_eq!(
            validator.validate(&ticket),
            Err(RelayError::InvalidSignature)
        );
    }

    #[test]
    fn signing_payload_sorts_allowed_derp_node_ids() {
        let mut left = sample_ticket();
        left.allowed_derp_node_ids = vec!["node-b".into(), "node-a".into()];
        let mut right = sample_ticket();
        right.allowed_derp_node_ids = vec!["node-a".into(), "node-b".into()];

        assert_eq!(left.signing_payload(), right.signing_payload());
    }
}
