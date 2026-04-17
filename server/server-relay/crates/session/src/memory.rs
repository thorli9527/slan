use std::collections::HashMap;
use std::sync::{Arc, RwLock};

use relay_core::{RelayError, RelaySession};

use crate::SessionStore;

/// 基于内存的 session store，适合本地开发与单进程测试。
#[derive(Debug, Clone, Default)]
pub struct InMemorySessionStore {
    inner: Arc<RwLock<HashMap<String, RelaySession>>>,
}

impl InMemorySessionStore {
    /// 创建新的内存 session store。
    pub fn new() -> Self {
        Self::default()
    }

    /// 返回当前已存储 session 数量。
    pub fn len(&self) -> usize {
        self.inner.read().expect("session store poisoned").len()
    }

    /// 判断当前 store 是否为空。
    pub fn is_empty(&self) -> bool {
        self.len() == 0
    }
}

impl SessionStore for InMemorySessionStore {
    fn create(&self, session: RelaySession) -> Result<(), RelayError> {
        let mut guard = self
            .inner
            .write()
            .map_err(|err| RelayError::Store(err.to_string()))?;
        if guard.contains_key(&session.session_id) {
            return Err(RelayError::SessionAlreadyExists);
        }
        guard.insert(session.session_id.clone(), session);
        Ok(())
    }

    fn get(&self, session_id: &str) -> Option<RelaySession> {
        self.inner
            .read()
            .ok()
            .and_then(|guard| guard.get(session_id).cloned())
    }

    fn remove(&self, session_id: &str) -> Result<(), RelayError> {
        let mut guard = self
            .inner
            .write()
            .map_err(|err| RelayError::Store(err.to_string()))?;
        if guard.remove(session_id).is_none() {
            return Err(RelayError::SessionNotFound);
        }
        Ok(())
    }
}
