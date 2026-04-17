use slan_app_core::{ConnectionState, RelayTicket};

/// Relay 连接能力。
pub trait RelayClient: Send + Sync {
    /// 使用 relay 票据建立回退连接。
    fn connect(&self, ticket: &RelayTicket) -> Result<ConnectionState, String>;
}
