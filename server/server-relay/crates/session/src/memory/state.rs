use std::collections::HashMap;
use std::sync::{Arc, RwLock};

use relay_core::{RelayError, RelaySession};

/// MemorySessionState 封装内存 session store 的共享锁状态。
#[derive(Debug, Clone, Default)]
pub struct MemorySessionState {
    sessions: Arc<RwLock<HashMap<String, RelaySession>>>,
}

impl MemorySessionState {
    /// len 返回当前已保存的 session 数量。
    pub fn len(&self) -> usize {
        self.sessions.read().expect("session store poisoned").len()
    }

    /// create 创建一个新的 session 记录。
    pub fn create(&self, session: RelaySession) -> Result<(), RelayError> {
        let mut guard = self
            .sessions
            .write()
            .map_err(|err| RelayError::Store(err.to_string()))?;
        if guard.contains_key(&session.session_id) {
            return Err(RelayError::SessionAlreadyExists);
        }
        guard.insert(session.session_id.clone(), session);
        Ok(())
    }

    /// get 返回指定 session 的副本。
    pub fn get(&self, session_id: &str) -> Option<RelaySession> {
        self.sessions
            .read()
            .ok()
            .and_then(|guard| guard.get(session_id).cloned())
    }

    /// remove 删除指定 session。
    pub fn remove(&self, session_id: &str) -> Result<(), RelayError> {
        let mut guard = self
            .sessions
            .write()
            .map_err(|err| RelayError::Store(err.to_string()))?;
        if guard.remove(session_id).is_none() {
            return Err(RelayError::SessionNotFound);
        }
        Ok(())
    }
}
