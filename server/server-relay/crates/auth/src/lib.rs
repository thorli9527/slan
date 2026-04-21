//! Relay 票据校验 crate。
//!
//! 该 crate 负责把控制面签发的 `RelayTicket` 转换成 daemon 可消费的校验能力。
//! 当前主要提供一个静态配置驱动的 `StaticTicketValidator`，用于执行：
//! - 必填字段检查
//! - relay URL 前缀检查
//! - 过期时间检查
//! - 可选的 HMAC-SHA256 签名检查

mod validator;

/// StaticTicketValidator 是当前 relay 侧默认使用的票据校验实现。
pub use validator::StaticTicketValidator;
