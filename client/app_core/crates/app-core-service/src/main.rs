use std::ffi::OsString;
use std::fs::OpenOptions;
use std::io::Write;
use std::path::{Path, PathBuf};
use std::process::{Child, Command};
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;
use std::thread;
use std::time::{Duration, SystemTime, UNIX_EPOCH};

use slan_app_core::{WireGuardInterfaceConfig, WireGuardKeyPair};
#[cfg(target_os = "windows")]
use windows_service::define_windows_service;
#[cfg(target_os = "windows")]
use windows_service::service::{
    ServiceControl, ServiceControlAccept, ServiceExitCode, ServiceState, ServiceStatus, ServiceType,
};
#[cfg(target_os = "windows")]
use windows_service::service_control_handler::{self, ServiceControlHandlerResult};
#[cfg(target_os = "windows")]
use windows_service::service_dispatcher;

use tunnel::{TunnelBackend, WindowsEmbeddableServiceBackend};

const DEFAULT_TCP_HOST: &str = "127.0.0.1:46391";
const DEFAULT_CONTROL_BASE_URL: &str = "http://127.0.0.1:28080";
#[cfg(target_os = "windows")]
const WINDOWS_SERVICE_NAME: &str = "SLANAppCoreService";

#[cfg(target_os = "windows")]
define_windows_service!(ffi_service_main, service_main);

#[derive(Debug, Clone)]
struct ServiceConfig {
    tcp_host: String,
    helper_path: Option<PathBuf>,
    control_base_url: Option<String>,
    driver: Option<String>,
    bind_ip: Option<String>,
    bind_prefix_len: Option<u8>,
    bind_interface_name: Option<String>,
    once: bool,
    prepare_adapter: bool,
    windows_service: bool,
}

impl Default for ServiceConfig {
    fn default() -> Self {
        Self {
            tcp_host: DEFAULT_TCP_HOST.to_string(),
            helper_path: None,
            control_base_url: None,
            driver: None,
            bind_ip: None,
            bind_prefix_len: None,
            bind_interface_name: None,
            once: false,
            prepare_adapter: false,
            windows_service: false,
        }
    }
}

fn main() {
    write_service_log(&format!(
        "process main start args={:?}",
        std::env::args_os()
            .map(|arg| arg.to_string_lossy().into_owned())
            .collect::<Vec<_>>()
    ));
    if let Err(err) = run() {
        write_service_log(&format!("fatal error: {err}"));
        eprintln!("{err}");
        std::process::exit(1);
    }
}

fn run() -> Result<(), String> {
    write_service_log("run entry");
    let config = parse_args(std::env::args_os().skip(1))?;
    apply_driver_override(&config);

    if config.prepare_adapter {
        write_service_log("prepare-adapter requested");
        WindowsEmbeddableServiceBackend::prepare_dedicated_adapter()?;
        write_service_log("prepare-adapter completed successfully");
        return Ok(());
    }

    if config.bind_ip.is_some() {
        return run_direct_bind(&config);
    }

    #[cfg(target_os = "windows")]
    if config.windows_service {
        write_service_log(&format!(
            "starting Windows service dispatcher service_name={WINDOWS_SERVICE_NAME}"
        ));
        return service_dispatcher::start(WINDOWS_SERVICE_NAME, ffi_service_main)
            .map_err(|err| format!("failed to start Windows service dispatcher: {err}"));
    }

    run_supervisor(config, None)
}

fn parse_args<I>(args: I) -> Result<ServiceConfig, String>
where
    I: IntoIterator<Item = OsString>,
{
    let mut config = ServiceConfig::default();
    let mut iter = args.into_iter();
    while let Some(arg) = iter.next() {
        let arg_string = arg.to_string_lossy().into_owned();
        match arg_string.as_str() {
            "--tcp-host" => {
                let value = iter
                    .next()
                    .ok_or_else(|| "missing host:port after --tcp-host".to_string())?;
                config.tcp_host = value.to_string_lossy().into_owned();
            }
            "--helper" => {
                let value = iter
                    .next()
                    .ok_or_else(|| "missing executable path after --helper".to_string())?;
                config.helper_path = Some(PathBuf::from(value));
            }
            "--control-base-url" => {
                let value = iter
                    .next()
                    .ok_or_else(|| "missing URL after --control-base-url".to_string())?;
                config.control_base_url = Some(value.to_string_lossy().into_owned());
            }
            "--driver" => {
                let value = iter
                    .next()
                    .ok_or_else(|| "missing driver kind after --driver".to_string())?;
                config.driver = Some(value.to_string_lossy().into_owned());
            }
            "--bind-ip" => {
                let value = iter
                    .next()
                    .ok_or_else(|| "missing IPv4 address after --bind-ip".to_string())?;
                config.bind_ip = Some(value.to_string_lossy().into_owned());
            }
            "--bind-prefix-len" => {
                let value = iter
                    .next()
                    .ok_or_else(|| "missing prefix length after --bind-prefix-len".to_string())?;
                let parsed = value
                    .to_string_lossy()
                    .parse::<u8>()
                    .map_err(|err| format!("invalid prefix length for --bind-prefix-len: {err}"))?;
                config.bind_prefix_len = Some(parsed);
            }
            "--bind-interface" => {
                let value = iter
                    .next()
                    .ok_or_else(|| "missing interface name after --bind-interface".to_string())?;
                config.bind_interface_name = Some(value.to_string_lossy().into_owned());
            }
            "--once" => {
                config.once = true;
            }
            "--prepare-adapter" => {
                config.prepare_adapter = true;
            }
            "--windows-service" => {
                config.windows_service = true;
            }
            other if !other.starts_with('-') => {
                #[cfg(target_os = "windows")]
                if other.eq_ignore_ascii_case(WINDOWS_SERVICE_NAME)
                    || other.eq_ignore_ascii_case("SLAN AppCore Service")
                    || other
                        .rsplit(['\\', '/'])
                        .next()
                        .map(|name| name.eq_ignore_ascii_case("app-core-service.exe"))
                        .unwrap_or(false)
                {
                    continue;
                }
                #[cfg(not(target_os = "windows"))]
                {
                    let _ = other;
                }
                return Err(format!(
                    "unsupported app-core-service argument '{other}'; expected --tcp-host, --helper, --control-base-url, --driver, --bind-ip, --bind-prefix-len, --bind-interface, --once, --prepare-adapter, or --windows-service"
                ));
            }
            other => {
                return Err(format!(
                    "unsupported app-core-service argument '{other}'; expected --tcp-host, --helper, --control-base-url, --driver, --bind-ip, --bind-prefix-len, --bind-interface, --once, --prepare-adapter, or --windows-service"
                ));
            }
        }
    }
    Ok(config)
}

fn apply_driver_override(config: &ServiceConfig) {
    if let Some(driver) = config
        .driver
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
    {
        unsafe {
            std::env::set_var("SLAN_WINDOWS_TUNNEL_DRIVER", driver);
        }
        write_service_log(&format!("driver override applied driver={driver}"));
    }
}

fn run_direct_bind(config: &ServiceConfig) -> Result<(), String> {
    let bind_ip = config
        .bind_ip
        .as_deref()
        .filter(|value| !value.trim().is_empty())
        .ok_or_else(|| "missing bind IP; pass --bind-ip <IPv4>".to_string())?;
    let prefix_len = config.bind_prefix_len.unwrap_or(32);
    let interface_name = config
        .bind_interface_name
        .clone()
        .unwrap_or_else(|| "SLAN LAN Adapter".to_string());

    write_service_log(&format!(
        "direct-bind requested interface={} ip={}/{}",
        interface_name, bind_ip, prefix_len
    ));
    let interface = WireGuardInterfaceConfig {
        interface_name: Some(interface_name.clone()),
        key_pair: WireGuardKeyPair {
            public_key: "diag-public-key".to_string(),
            private_key: "diag-private-key".to_string(),
        },
        listen_port: Some(51820),
        mtu: Some(1280),
        addresses: vec![format!("{bind_ip}/{prefix_len}")],
        dns_servers: vec![],
        peers: vec![],
    };

    let mut last_error: Option<String> = None;
    for attempt in 1..=2 {
        if attempt == 2 {
            write_service_log("direct-bind retrying after prepare-adapter");
            match WindowsEmbeddableServiceBackend::prepare_dedicated_adapter() {
                Ok(()) => write_service_log("direct-bind prepare-adapter completed"),
                Err(err) => write_service_log(&format!(
                    "direct-bind prepare-adapter failed but continuing to bind attempt: {err}"
                )),
            }
        }

        let backend = WindowsEmbeddableServiceBackend::new();
        backend
            .apply_interface_config(&interface)
            .map_err(|err| format!("direct-bind apply_interface_config failed: {err}"))?;
        write_service_log(&format!(
            "direct-bind apply_interface_config accepted interface={} address={}/{} attempt={}",
            interface_name, bind_ip, prefix_len, attempt
        ));

        match backend.bring_up() {
            Ok(()) => {
                write_service_log(&format!(
                    "direct-bind bring_up succeeded interface={} address={}/{} attempt={}",
                    interface_name, bind_ip, prefix_len, attempt
                ));
                return Ok(());
            }
            Err(err) => {
                write_service_log(&format!(
                    "direct-bind bring_up failed interface={} address={}/{} attempt={} error={}",
                    interface_name, bind_ip, prefix_len, attempt, err
                ));
                last_error = Some(err);
            }
        }
    }

    Err(format!(
        "direct-bind failed after retries: {}",
        last_error.unwrap_or_else(|| "unknown error".to_string())
    ))
}

fn resolve_control_base_url(config: &ServiceConfig) -> Result<String, String> {
    if let Some(base_url) = config.control_base_url.as_ref() {
        let trimmed = base_url.trim();
        if !trimmed.is_empty() {
            return Ok(trimmed.to_string());
        }
    }
    if let Ok(base_url) = std::env::var("SLAN_CONTROL_BASE_URL") {
        let trimmed = base_url.trim();
        if !trimmed.is_empty() {
            return Ok(trimmed.to_string());
        }
    }
    Ok(DEFAULT_CONTROL_BASE_URL.to_string())
}

fn resolve_helper_path(config: &ServiceConfig) -> Result<PathBuf, String> {
    if let Some(path) = config.helper_path.as_ref() {
        return Ok(path.clone());
    }
    if let Ok(path) = std::env::var("SLAN_APP_CORE_HELPER") {
        let trimmed = path.trim();
        if !trimmed.is_empty() {
            return Ok(PathBuf::from(trimmed));
        }
    }
    let current_exe = std::env::current_exe()
        .map_err(|err| format!("failed to resolve app-core-service executable path: {err}"))?;
    let parent = current_exe
        .parent()
        .ok_or_else(|| "app-core-service executable path has no parent directory".to_string())?;
    Ok(parent.join("app-core-helper.exe"))
}

fn spawn_helper(
    helper_path: &Path,
    tcp_host: &str,
    control_base_url: &str,
    driver: Option<&str>,
) -> Result<Child, String> {
    if !helper_path.exists() {
        return Err(format!(
            "app-core-helper executable not found at {}",
            helper_path.display()
        ));
    }
    let mut command = Command::new(helper_path);
    command.arg("--tcp-host").arg(tcp_host);
    command.env("SLAN_CONTROL_BASE_URL", control_base_url);
    if let Some(driver) = driver.map(str::trim).filter(|value| !value.is_empty()) {
        command.env("SLAN_WINDOWS_TUNNEL_DRIVER", driver);
    }
    command
        .spawn()
        .map_err(|err| format!("failed to spawn app-core-helper: {err}"))
}

fn run_supervisor(
    config: ServiceConfig,
    stop_signal: Option<Arc<AtomicBool>>,
) -> Result<(), String> {
    let control_base_url = resolve_control_base_url(&config)?;
    let helper_path = resolve_helper_path(&config)?;
    write_service_log(&format!(
        "service start tcp_host={} control_base_url={} driver={} helper_override={} once={}",
        config.tcp_host,
        control_base_url,
        config.driver.as_deref().unwrap_or("<auto>"),
        helper_path.display(),
        config.once
    ));
    write_service_log(&format!("resolved helper path={}", helper_path.display()));

    loop {
        if is_stop_requested(stop_signal.as_ref()) {
            write_service_log("stop requested before helper spawn");
            return Ok(());
        }

        let mut child = spawn_helper(
            &helper_path,
            &config.tcp_host,
            &control_base_url,
            config.driver.as_deref(),
        )?;
        write_service_log(&format!(
            "spawned helper pid={} tcp_host={}",
            child.id(),
            config.tcp_host
        ));

        loop {
            if is_stop_requested(stop_signal.as_ref()) {
                write_service_log("stop requested; terminating helper");
                let _ = child.kill();
                let _ = child.wait();
                return Ok(());
            }

            match child.try_wait() {
                Ok(Some(status)) => {
                    write_service_log(&format!("helper exited status={status}"));
                    if config.once {
                        if status.success() {
                            return Ok(());
                        }
                        return Err(format!("app-core-helper exited with status {status}"));
                    }
                    break;
                }
                Ok(None) => {
                    thread::sleep(Duration::from_millis(500));
                }
                Err(err) => {
                    return Err(format!("failed while waiting on app-core-helper: {err}"));
                }
            }
        }

        if config.once {
            return Ok(());
        }

        if is_stop_requested(stop_signal.as_ref()) {
            write_service_log("stop requested after helper exit");
            return Ok(());
        }

        write_service_log("helper exited unexpectedly; restarting after backoff");
        thread::sleep(Duration::from_secs(1));
    }
}

fn is_stop_requested(stop_signal: Option<&Arc<AtomicBool>>) -> bool {
    stop_signal
        .map(|signal| signal.load(Ordering::SeqCst))
        .unwrap_or(false)
}

fn write_service_log(message: &str) {
    let timestamp = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|duration| duration.as_secs())
        .unwrap_or_default();
    let log_path = service_log_path();
    if let Some(parent) = log_path.parent() {
        let _ = std::fs::create_dir_all(parent);
    }
    if let Ok(mut file) = OpenOptions::new().create(true).append(true).open(&log_path) {
        let _ = writeln!(file, "[{timestamp}] {message}");
    }
}

fn service_log_path() -> PathBuf {
    if let Ok(configured) = std::env::var("SLAN_APP_CORE_SERVICE_LOG") {
        let trimmed = configured.trim();
        if !trimmed.is_empty() {
            return PathBuf::from(trimmed);
        }
    }
    #[cfg(target_os = "windows")]
    if let Ok(program_data) = std::env::var("ProgramData") {
        let trimmed = program_data.trim();
        if !trimmed.is_empty() {
            return PathBuf::from(trimmed)
                .join("SLAN")
                .join("app-core-service.log");
        }
    }
    #[cfg(target_os = "windows")]
    return PathBuf::from(r"C:\ProgramData\SLAN\app-core-service.log");
    #[cfg(not(target_os = "windows"))]
    {
        if let Ok(current_exe) = std::env::current_exe() {
            if let Some(parent) = current_exe.parent() {
                return parent.join("app-core-service.log");
            }
        }
        std::env::temp_dir().join("app-core-service.log")
    }
}

#[cfg(target_os = "windows")]
fn service_main(arguments: Vec<OsString>) {
    if let Err(err) = run_windows_service(arguments) {
        write_service_log(&format!("windows service fatal error: {err}"));
    }
}

#[cfg(target_os = "windows")]
fn run_windows_service(arguments: Vec<OsString>) -> Result<(), String> {
    let stop_signal = Arc::new(AtomicBool::new(false));
    let stop_signal_for_handler = stop_signal.clone();
    let status_handle =
        service_control_handler::register(WINDOWS_SERVICE_NAME, move |control_event| {
            match control_event {
                ServiceControl::Stop | ServiceControl::Shutdown => {
                    stop_signal_for_handler.store(true, Ordering::SeqCst);
                    ServiceControlHandlerResult::NoError
                }
                _ => ServiceControlHandlerResult::NotImplemented,
            }
        })
        .map_err(|err| format!("failed to register Windows service control handler: {err}"))?;

    write_service_log(&format!(
        "windows service invoked with arguments={:?}",
        arguments
            .iter()
            .map(|arg| arg.to_string_lossy().into_owned())
            .collect::<Vec<_>>()
    ));
    set_windows_service_status(&status_handle, ServiceState::StartPending)?;

    match parse_args(arguments) {
        Ok(mut config) => {
            config.windows_service = false;
            set_windows_service_status(&status_handle, ServiceState::Running)?;
            let run_result = run_supervisor(config, Some(stop_signal));
            set_windows_service_status(&status_handle, ServiceState::StopPending)?;
            set_windows_service_status(&status_handle, ServiceState::Stopped)?;
            run_result
        }
        Err(err) => {
            let _ = set_windows_service_status(&status_handle, ServiceState::StopPending);
            let _ = set_windows_service_status(&status_handle, ServiceState::Stopped);
            Err(err)
        }
    }
}

#[cfg(target_os = "windows")]
fn set_windows_service_status(
    status_handle: &windows_service::service_control_handler::ServiceStatusHandle,
    state: ServiceState,
) -> Result<(), String> {
    let controls = match state {
        ServiceState::Running => ServiceControlAccept::STOP | ServiceControlAccept::SHUTDOWN,
        _ => ServiceControlAccept::empty(),
    };
    status_handle
        .set_service_status(ServiceStatus {
            service_type: ServiceType::OWN_PROCESS,
            current_state: state,
            controls_accepted: controls,
            exit_code: ServiceExitCode::Win32(0),
            checkpoint: 0,
            wait_hint: Duration::from_secs(10),
            process_id: None,
        })
        .map_err(|err| format!("failed to update Windows service status: {err}"))
}
