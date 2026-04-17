//! Relay 核心模型与错误定义。

mod error;
mod model;
mod time;
mod validator;

pub use error::RelayError;
pub use model::{RelaySession, RelayTicket};
pub use time::parse_timestamp;
pub use validator::TicketValidator;
