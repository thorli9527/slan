#![allow(dead_code)]

mod acl_policy;
mod client_message_mqtt;
mod control_plane;
mod control_tasks;
mod control_transport;
mod local_api;
mod network_module;
mod relay_candidates;
mod relay_models;
mod session_store;
mod time_utils;

pub mod embedded;

#[cfg(test)]
pub(crate) fn test_env_lock() -> std::sync::MutexGuard<'static, ()> {
    static LOCK: std::sync::OnceLock<std::sync::Mutex<()>> = std::sync::OnceLock::new();
    LOCK.get_or_init(|| std::sync::Mutex::new(()))
        .lock()
        .expect("test env mutex poisoned")
}
