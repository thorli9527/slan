use relay_core::{RelayError, RelayTicket};

/// validate_required_fields 校验票据最基础的非空字段。
///
/// 这些字段缺任意一项，都说明票据已经不具备最小语义，
/// 后续的过期、签名或参与者检查都没有继续执行的价值。
pub fn validate_required_fields(ticket: &RelayTicket) -> Result<(), RelayError> {
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
    Ok(())
}

/// validate_relay_prefix 校验票据是否属于当前 relay 地址空间。
///
/// 对静态部署来说，这一步可以避免把为其他 relay 节点签发的票据
/// 错发到当前节点后仍被接受。
pub fn validate_relay_prefix(
    ticket: &RelayTicket,
    required_relay_url_prefix: Option<&str>,
) -> Result<(), RelayError> {
    if let Some(prefix) = required_relay_url_prefix {
        if !ticket.relay_url.starts_with(prefix) {
            return Err(RelayError::InvalidTicket);
        }
    }
    Ok(())
}

/// validate_expiration 校验票据是否已经超过过期时间。
pub fn validate_expiration(ticket: &RelayTicket) -> Result<(), RelayError> {
    if ticket.is_expired()? {
        return Err(RelayError::TicketExpired);
    }
    Ok(())
}
