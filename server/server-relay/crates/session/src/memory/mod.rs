use relay_core::{RelayError, RelaySession};

use crate::SessionStore;

mod state;

use state::MemorySessionState;

/// 基于内存的 session store，适合本地开发与单进程运行。
#[derive(Debug, Clone, Default)]
pub struct InMemorySessionStore {
    inner: MemorySessionState,
}

impl InMemorySessionStore {
    /// 创建新的内存 session store。
    pub fn new() -> Self {
        Self::default()
    }

    /// 返回当前已存储 session 数量。
    pub fn len(&self) -> usize {
        self.inner.len()
    }

    /// 判断当前 store 是否为空。
    pub fn is_empty(&self) -> bool {
        self.len() == 0
    }
}

impl SessionStore for InMemorySessionStore {
    fn create(&self, session: RelaySession) -> Result<(), RelayError> {
        self.inner.create(session)
    }

    fn get(&self, session_id: &str) -> Option<RelaySession> {
        self.inner.get(session_id)
    }

    fn remove(&self, session_id: &str) -> Result<(), RelayError> {
        self.inner.remove(session_id)
    }
}
