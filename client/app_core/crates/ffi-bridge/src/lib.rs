//! Flutter 与 Rust AppCore 之间的门面抽象。

mod connect_runtime;
mod default_facade;
mod diagnostics;
mod facade;
mod json_facade;
mod json_facade_args;
mod json_facade_runtime;
mod key_provider;
mod mqtt_control_tasks;
mod probe_runtime;
mod snapshot;
mod snapshot_updates;
mod state_helpers;
mod tunnel_runtime;

pub use default_facade::DefaultAppCoreFacade;
pub use facade::{
    AppCoreFacade, DataPlaneError, DataPlaneErrorCode, DataPlaneProbe, TunnelRuntimeView,
    TunnelState,
};
pub use json_facade::JsonAppCoreFacade;
pub use key_provider::{FileTunnelKeyProvider, InMemoryTunnelKeyProvider, TunnelKeyProvider};
pub use snapshot::AppCoreSnapshot;
