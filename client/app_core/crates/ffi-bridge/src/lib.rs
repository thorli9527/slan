//! Flutter 与 Rust AppCore 之间的门面抽象。

mod default_facade;
mod facade;

pub use default_facade::DefaultAppCoreFacade;
pub use facade::AppCoreFacade;
