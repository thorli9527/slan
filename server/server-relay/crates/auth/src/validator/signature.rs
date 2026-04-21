use base64::{engine::general_purpose::URL_SAFE_NO_PAD, Engine as _};
use hmac::{Hmac, Mac};
use relay_core::{RelayError, RelayTicket};
use sha2::Sha256;

type HmacSha256 = Hmac<Sha256>;

/// HmacTicketSignatureValidator 负责按控制面约定校验票据签名。
#[derive(Debug, Clone)]
pub struct HmacTicketSignatureValidator {
    secret: String,
}

impl HmacTicketSignatureValidator {
    /// new 创建一个使用共享 secret 的 HMAC-SHA256 签名校验器。
    pub fn new(secret: impl Into<String>) -> Self {
        Self {
            secret: secret.into(),
        }
    }

    /// validate 校验传入票据的 signature 是否与 signing_payload 匹配。
    pub fn validate(&self, ticket: &RelayTicket) -> Result<(), RelayError> {
        let mut mac = HmacSha256::new_from_slice(self.secret.as_bytes())
            .map_err(|_| RelayError::InvalidTicket)?;
        mac.update(ticket.signing_payload().as_bytes());
        let expected = URL_SAFE_NO_PAD.encode(mac.finalize().into_bytes());
        if expected != ticket.signature {
            return Err(RelayError::InvalidSignature);
        }
        Ok(())
    }
}
