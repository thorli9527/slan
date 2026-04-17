use relay_core::{RelayError, RelaySession};

/// Relay session 存储接口。
pub trait SessionStore: Send + Sync {
    fn create(&self, session: RelaySession) -> Result<(), RelayError>;
    fn get(&self, session_id: &str) -> Option<RelaySession>;
    fn remove(&self, session_id: &str) -> Result<(), RelayError>;
}
