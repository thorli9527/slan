//! Relay session 存储 crate。
//!
//! 该 crate 定义 relay 运行时依赖的 session 存储抽象，并提供一个
//! 适合单进程开发的内存实现。

mod memory;
mod store;

/// InMemorySessionStore 提供基于 `RwLock<HashMap<...>>` 的默认实现。
pub use memory::InMemorySessionStore;
/// SessionStore 定义 relay runtime 读写会话状态时依赖的最小接口。
pub use store::SessionStore;
