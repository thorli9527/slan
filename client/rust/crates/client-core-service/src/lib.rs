#![allow(dead_code)]

mod acl_policy;
mod client_config;
mod client_message_mqtt;
mod control_plane;
mod control_tasks;
mod control_transport;
mod diagnostic_upload;
mod local_api;
mod mobile_platform_config;
mod network_event;
mod network_event_apply;
mod network_event_projection;
mod network_module;
mod network_runtime_state;
mod platform_runtime_report;
mod relay_candidates;
mod relay_models;
mod relay_store;
mod resolver_apply;
mod resolver_authority;
mod resolver_forwarder;
mod resolver_runtime_state;
mod resolver_server;
mod runtime_actor;
mod runtime_event_hub;
mod session_store;
mod time_utils;

pub mod embedded;

pub(crate) fn log_service_error(message: impl AsRef<str>) {
    use std::{fs, fs::OpenOptions, io::Write};

    let dir = session_store::app_data_dir().join("SLAN");
    let _ = fs::create_dir_all(&dir);
    let path = dir.join("client-core-service.log");
    if let Ok(mut file) = OpenOptions::new().create(true).append(true).open(path) {
        let _ = writeln!(
            file,
            "{} {}",
            session_store::current_timestamp_ms(),
            message.as_ref()
        );
    }
}

#[cfg(test)]
pub(crate) fn test_env_lock() -> std::sync::MutexGuard<'static, ()> {
    static LOCK: std::sync::OnceLock<std::sync::Mutex<()>> = std::sync::OnceLock::new();
    LOCK.get_or_init(|| std::sync::Mutex::new(()))
        .lock()
        .unwrap_or_else(|error| error.into_inner())
}
