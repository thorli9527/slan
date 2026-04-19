//! 隧道管理抽象定义。

mod android_backend;
mod backend;
mod config;
mod factory;
mod linux_adapter;
mod linux_backend;
mod linux_mapper;
mod linux_native;
mod linux_stats;
mod macos;
mod macos_adapter;
mod macos_mapper;
mod macos_stats;
mod manager;
mod platform;
mod runtime;
mod windows_backend;

pub use android_backend::AndroidWireGuardTunnelBackend;
pub use backend::{InMemoryTunnelBackend, TunnelBackend};
pub use config::TunnelConfig;
pub use factory::TunnelBackendFactory;
pub use linux_backend::LinuxKernelWireGuardBackend;
pub use macos::MacosWireGuardKitBackend;
pub use manager::TunnelManager;
pub use platform::{
    build_platform_tunnel_manager, detect_platform_tunnel_driver, TunnelDriverKind,
    TunnelDriverSelection,
};
pub use runtime::{InMemoryTunnelManager, SystemTunnelManager};
pub use windows_backend::WindowsEmbeddableServiceBackend;
