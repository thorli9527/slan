use std::sync::Arc;

use slan_app_core::{ConnectionState, RelayTicket};

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct RelayClientError {
    pub code: Option<String>,
    pub message: String,
}

impl RelayClientError {
    pub fn new(code: Option<String>, message: impl Into<String>) -> Self {
        Self {
            code,
            message: message.into(),
        }
    }

    pub fn message(message: impl Into<String>) -> Self {
        Self::new(None, message)
    }
}

impl std::fmt::Display for RelayClientError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self.code.as_deref() {
            Some(code) if !code.trim().is_empty() => write!(f, "[{code}] {}", self.message),
            _ => f.write_str(&self.message),
        }
    }
}

impl std::error::Error for RelayClientError {}

impl From<String> for RelayClientError {
    fn from(value: String) -> Self {
        Self::message(value)
    }
}

/// Relay 连接能力。
pub trait RelayClient: Send + Sync {
    /// 使用 relay 票据建立回退连接。
    fn connect(&self, ticket: &RelayTicket) -> Result<ConnectionState, RelayClientError>;
    /// 通过已建立的 relay 路径发送 transport packet。
    fn send_transport_packet(&self, packet: &[u8]) -> Result<(), RelayClientError>;
    /// 非阻塞轮询接收已建立 relay 路径上的 transport packet。
    fn poll_transport_packet(&self) -> Result<Option<Vec<u8>>, RelayClientError>;
}

impl<T> RelayClient for Arc<T>
where
    T: RelayClient,
{
    fn connect(&self, ticket: &RelayTicket) -> Result<ConnectionState, RelayClientError> {
        self.as_ref().connect(ticket)
    }

    fn send_transport_packet(&self, packet: &[u8]) -> Result<(), RelayClientError> {
        self.as_ref().send_transport_packet(packet)
    }

    fn poll_transport_packet(&self) -> Result<Option<Vec<u8>>, RelayClientError> {
        self.as_ref().poll_transport_packet()
    }
}
