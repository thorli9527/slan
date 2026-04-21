//! Relay 核心模型与通用能力。
//!
//! `relay-core` 只保留纯领域层能力，不依赖具体传输或 daemon 运行时。
//! 其他 crate 通常通过这里共享：
//! - `RelayTicket`
//! - `RelaySession`
//! - `RelayError`
//! - `TicketValidator`

mod error;
mod model;
mod time;
mod validator;

/// RelayError 是 relay 领域层统一错误类型。
pub use error::RelayError;
/// RelaySession 和 RelayTicket 是 relay 侧共享的核心模型。
pub use model::{RelaySession, RelayTicket};
/// parse_timestamp 用于解析票据等对象中的时间字段。
pub use time::parse_timestamp;
/// TicketValidator 描述票据校验器的统一接口。
pub use validator::TicketValidator;
