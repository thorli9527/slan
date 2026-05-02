//! Relay daemon crate。
//!
//! 该 crate 负责把 `relay-core` 的领域对象和 `session/auth/udp-relay`
//! 这些依赖组装成一个可运行的 UDP relay 服务。阅读顺序建议：
//! - `config`：进程级配置
//! - `protocol/*`：线协议模型
//! - `errors/*`：协议与领域错误映射
//! - `runtime/*`：attach / forward / detach 运行时逻辑
//! - `daemon`：UDP server 外壳

mod config;
mod daemon;
mod errors;
mod mqtt;
mod protocol;
mod runtime;

/// DaemonConfig 描述 relay daemon 的启动配置。
pub use config::{DaemonConfig, RelayMqttConfig};
/// RelayDaemon 负责 UDP socket 生命周期和请求收发循环。
pub use daemon::RelayDaemon;
/// 协议层导出供 daemon 和客户端共享的 request/response 模型。
pub use protocol::{
    AttachAck, ClientRequest, ErrorResponse, ForwardAck, RelayPacketMessage, RelayTicketWire,
    ServerResponse,
};
/// RelayRuntime 和 RelayRuntimeError 描述 daemon 的核心运行时行为。
pub use runtime::{RelayRuntime, RelayRuntimeError};
