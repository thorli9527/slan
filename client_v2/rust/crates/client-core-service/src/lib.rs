#![allow(dead_code)]

mod acl_policy;
mod client_config;
mod client_message_mqtt;
mod control_plane;
mod control_tasks;
mod control_transport;
mod local_api;
mod mobile_platform_config;
mod network_event;
mod network_event_apply;
mod network_module;
mod network_runtime_state;
mod relay_candidates;
mod relay_models;
mod relay_store;
mod resolver_apply;
mod resolver_authority;
mod resolver_forwarder;
mod resolver_runtime_state;
mod resolver_server;
mod session_store;
mod time_utils;

pub mod embedded;

#[cfg(test)]
pub(crate) fn test_env_lock() -> std::sync::MutexGuard<'static, ()> {
    static LOCK: std::sync::OnceLock<std::sync::Mutex<()>> = std::sync::OnceLock::new();
    LOCK.get_or_init(|| std::sync::Mutex::new(()))
        .lock()
        .unwrap_or_else(|error| error.into_inner())
}
