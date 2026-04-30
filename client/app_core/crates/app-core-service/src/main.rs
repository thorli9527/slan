use std::ffi::OsString;
use std::fs::OpenOptions;
use std::io::{BufRead, BufReader, Write};
use std::net::TcpStream;
use std::path::{Path, PathBuf};
use std::process::{Child, Command};
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;
use std::thread;
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};

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

mod mqtt_control_tasks;
mod tasks;

use mqtt_control_tasks::{
    read_mqtt_control_tasks, write_mqtt_control_tasks, MqttControlTask, MqttControlTaskPolicy,
};
use tasks::{ServiceTask, ServiceTaskRunner};

const DEFAULT_TCP_HOST: &str = "127.0.0.1:46391";
const DEFAULT_CONTROL_BASE_URL: &str = "http://127.0.0.1:28080";
const CONTROL_SYNC_AGENT_INTERVAL: Duration = Duration::from_secs(5);
const MQTT_CONTROL_TASK_RUNNER_INTERVAL: Duration = Duration::from_secs(2);
const NETWORK_STATE_REPORT_INTERVAL: Duration = Duration::from_secs(15);
const LOCAL_DNS_ENSURE_INTERVAL: Duration = Duration::from_secs(15);
const HELPER_READY_TIMEOUT: Duration = Duration::from_secs(10);
const HELPER_READY_POLL_INTERVAL: Duration = Duration::from_millis(200);
const MQTT_CONTROL_TASK_MAX_ATTEMPTS: u32 = 5;
const MQTT_CONTROL_TASK_RUNNING_STALE_MS: u64 = 30_000;
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
    if let Some(base_url) = read_persisted_control_base_url() {
        let trimmed = base_url.trim();
        if !trimmed.is_empty() {
            write_service_log(&format!(
                "control_base_url restored from persisted app-core state: {trimmed}"
            ));
            return Ok(trimmed.to_string());
        }
    }
    Ok(DEFAULT_CONTROL_BASE_URL.to_string())
}

fn read_persisted_control_base_url() -> Option<String> {
    let path = persisted_app_core_state_path();
    let payload = std::fs::read(&path).ok()?;
    let value: serde_json::Value = serde_json::from_slice(&payload)
        .map_err(|err| {
            write_service_log(&format!(
                "decode persisted app-core state failed path={} error={err}",
                path.display()
            ));
            err
        })
        .ok()?;
    value
        .get("controlBaseUrl")
        .and_then(|value| value.as_str())
        .map(str::to_string)
}

fn persisted_app_core_state_path() -> PathBuf {
    if let Ok(configured) = std::env::var("SLAN_APP_CORE_STATE_FILE") {
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
                .join("app-core-state.json");
        }
    }
    #[cfg(target_os = "windows")]
    return PathBuf::from(r"C:\ProgramData\SLAN\app-core-state.json");
    #[cfg(not(target_os = "windows"))]
    {
        if let Ok(current_exe) = std::env::current_exe() {
            if let Some(parent) = current_exe.parent() {
                return parent.join("app-core-state.json");
            }
        }
        std::env::temp_dir().join("slan-app-core-state.json")
    }
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
    command.env("SLAN_APP_CORE_HELPER_LOG", helper_log_path());
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
        if let Err(err) = wait_for_helper_rpc_ready(
            &config.tcp_host,
            &mut child,
            HELPER_READY_TIMEOUT,
            stop_signal.as_ref(),
        ) {
            write_service_log(&format!("helper rpc not ready after spawn: {err}"));
            let _ = child.kill();
            let _ = child.wait();
            if is_stop_requested(stop_signal.as_ref()) {
                return Ok(());
            }
            if config.once {
                return Err(err);
            }
            thread::sleep(Duration::from_secs(1));
            continue;
        }
        write_service_log("helper rpc ready; starting service tasks");
        let service_tasks = start_service_tasks(config.tcp_host.clone());
        let mut helper_rpc_probe_failures = 0_u32;
        let mut next_helper_rpc_probe = Instant::now() + Duration::from_secs(5);

        loop {
            if is_stop_requested(stop_signal.as_ref()) {
                write_service_log("stop requested; terminating helper");
                let _ = child.kill();
                let _ = child.wait();
                service_tasks.stop_and_join();
                return Ok(());
            }

            match child.try_wait() {
                Ok(Some(status)) => {
                    service_tasks.stop_and_join();
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

            if Instant::now() >= next_helper_rpc_probe {
                next_helper_rpc_probe = Instant::now() + Duration::from_secs(5);
                match helper_rpc_is_healthy(&config.tcp_host) {
                    Ok(()) => {
                        if helper_rpc_probe_failures > 0 {
                            write_service_log("helper rpc probe recovered");
                        }
                        helper_rpc_probe_failures = 0;
                    }
                    Err(err) => {
                        helper_rpc_probe_failures = helper_rpc_probe_failures.saturating_add(1);
                        write_service_log(&format!(
                            "helper rpc probe failed count={} error={}",
                            helper_rpc_probe_failures, err
                        ));
                        if helper_rpc_probe_failures >= 3 {
                            write_service_log(
                                "helper rpc unavailable; terminating helper for restart",
                            );
                            let _ = child.kill();
                            let _ = child.wait();
                            service_tasks.stop_and_join();
                            break;
                        }
                    }
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

fn helper_rpc_is_healthy(tcp_host: &str) -> Result<(), String> {
    let result = invoke_helper_method_result(tcp_host, "helperStatus")?;
    if result
        .get("helperReachable")
        .and_then(|value| value.as_bool())
        == Some(true)
    {
        return Ok(());
    }
    Err("helperStatus reported helperReachable=false".to_string())
}

fn wait_for_helper_rpc_ready(
    tcp_host: &str,
    child: &mut Child,
    timeout: Duration,
    stop_signal: Option<&Arc<AtomicBool>>,
) -> Result<(), String> {
    let deadline = Instant::now() + timeout;
    let mut last_error = None;
    while !is_stop_requested(stop_signal) {
        match child.try_wait() {
            Ok(Some(status)) => {
                return Err(format!(
                    "app-core-helper exited before rpc was ready: {status}"
                ));
            }
            Ok(None) => {}
            Err(err) => {
                return Err(format!("failed while waiting for helper readiness: {err}"));
            }
        }
        match helper_rpc_is_healthy(tcp_host) {
            Ok(()) => return Ok(()),
            Err(err) => last_error = Some(err),
        }
        if Instant::now() >= deadline {
            break;
        }
        let remaining = deadline.saturating_duration_since(Instant::now());
        thread::sleep(remaining.min(HELPER_READY_POLL_INTERVAL));
    }
    if is_stop_requested(stop_signal) {
        return Err("stop requested while waiting for helper rpc".to_string());
    }
    Err(format!(
        "helper rpc did not become ready within {}ms: {}",
        timeout.as_millis(),
        last_error.unwrap_or_else(|| "no probe attempted".to_string())
    ))
}

fn start_service_tasks(tcp_host: String) -> ServiceTaskRunner {
    let control_sync_host = tcp_host.clone();
    let mqtt_control_task_host = tcp_host.clone();
    let network_state_host = tcp_host.clone();
    let local_dns_host = tcp_host;
    ServiceTaskRunner::start(
        vec![
            ServiceTask::periodic("control-sync", CONTROL_SYNC_AGENT_INTERVAL, move || {
                invoke_helper_control_sync(&control_sync_host)
            }),
            ServiceTask::periodic(
                "mqtt-control-task-runner",
                MQTT_CONTROL_TASK_RUNNER_INTERVAL,
                move || run_mqtt_control_task_job(&mqtt_control_task_host),
            ),
            ServiceTask::periodic(
                "network-state-report",
                NETWORK_STATE_REPORT_INTERVAL,
                move || invoke_helper_report_device_network_state(&network_state_host),
            ),
            ServiceTask::periodic("local-dns-ensure", LOCAL_DNS_ENSURE_INTERVAL, move || {
                invoke_helper_ensure_local_dns(&local_dns_host)
            }),
        ],
        Arc::new(|message| write_service_log(&message)),
    )
}

fn invoke_helper_control_sync(tcp_host: &str) -> Result<(), String> {
    invoke_helper_method(tcp_host, "controlSync")
}

fn invoke_helper_report_device_network_state(tcp_host: &str) -> Result<(), String> {
    invoke_helper_method(tcp_host, "reportDeviceNetworkState")
}

fn invoke_helper_ensure_local_dns(tcp_host: &str) -> Result<(), String> {
    invoke_helper_method(tcp_host, "ensureLocalDns")
}

fn run_mqtt_control_task_job(tcp_host: &str) -> Result<(), String> {
    let mut tasks = read_mqtt_control_tasks()?;
    let Some(index) = next_mqtt_control_task_index(&tasks) else {
        return Ok(());
    };
    let task_id = tasks[index].id.clone();
    tasks[index].status = "running".to_string();
    tasks[index].attempts = tasks[index].attempts.saturating_add(1);
    tasks[index].updated_at_ms = current_timestamp_ms();
    tasks[index].error.clear();
    write_mqtt_control_tasks(&tasks)?;

    let task = tasks[index].clone();
    let result = execute_mqtt_control_task(tcp_host, &task);
    let mut latest = read_mqtt_control_tasks()?;
    if let Some(item) = latest.iter_mut().find(|item| item.id == task_id) {
        item.updated_at_ms = current_timestamp_ms();
        match result {
            Ok(()) => {
                item.status = "succeeded".to_string();
                item.error.clear();
            }
            Err(err) => {
                item.status = "failed".to_string();
                item.error = err;
            }
        }
    }
    write_mqtt_control_tasks(&latest)
}

fn next_mqtt_control_task_index(tasks: &[MqttControlTask]) -> Option<usize> {
    MqttControlTaskPolicy {
        max_attempts: MQTT_CONTROL_TASK_MAX_ATTEMPTS,
        running_stale_ms: MQTT_CONTROL_TASK_RUNNING_STALE_MS,
    }
    .next_runnable_index(tasks, current_timestamp_ms())
}

fn execute_mqtt_control_task(tcp_host: &str, task: &MqttControlTask) -> Result<(), String> {
    if task.network_id.trim().is_empty() {
        return Err("mqtt control task missing networkId".to_string());
    }
    match task.task_type.as_str() {
        "enable_network" => invoke_helper_method_with_args(
            tcp_host,
            "enableLocalNetwork",
            serde_json::json!({ "networkId": task.network_id }),
        )
        .map(|_| ()),
        "disable_network" => invoke_helper_method_with_args(
            tcp_host,
            "disableLocalNetwork",
            serde_json::json!({ "networkId": task.network_id }),
        )
        .map(|_| ()),
        other => Err(format!("unsupported mqtt control task type: {other}")),
    }
}

fn invoke_helper_method(tcp_host: &str, method: &str) -> Result<(), String> {
    invoke_helper_method_result(tcp_host, method).map(|_| ())
}

fn invoke_helper_method_with_args(
    tcp_host: &str,
    method: &str,
    args: serde_json::Value,
) -> Result<serde_json::Value, String> {
    invoke_helper_method_result_with_args(tcp_host, method, args)
}

fn invoke_helper_method_result(tcp_host: &str, method: &str) -> Result<serde_json::Value, String> {
    invoke_helper_method_result_with_args(tcp_host, method, serde_json::json!({}))
}

fn invoke_helper_method_result_with_args(
    tcp_host: &str,
    method: &str,
    args: serde_json::Value,
) -> Result<serde_json::Value, String> {
    let mut stream = TcpStream::connect(tcp_host)
        .map_err(|err| format!("connect helper rpc {tcp_host}: {err}"))?;
    stream
        .set_read_timeout(Some(Duration::from_secs(4)))
        .map_err(|err| format!("set helper read timeout: {err}"))?;
    stream
        .set_write_timeout(Some(Duration::from_secs(4)))
        .map_err(|err| format!("set helper write timeout: {err}"))?;
    let request = serde_json::json!({
        "method": method,
        "args": args,
    });
    stream
        .write_all(request.to_string().as_bytes())
        .map_err(|err| format!("write {method} rpc: {err}"))?;
    stream
        .write_all(b"\n")
        .map_err(|err| format!("write {method} rpc newline: {err}"))?;
    stream
        .flush()
        .map_err(|err| format!("flush {method} rpc: {err}"))?;
    let reader_stream = stream
        .try_clone()
        .map_err(|err| format!("clone helper rpc stream: {err}"))?;
    let mut reader = BufReader::new(reader_stream);
    let mut line = String::new();
    let bytes = reader
        .read_line(&mut line)
        .map_err(|err| format!("read {method} rpc response: {err}"))?;
    if bytes == 0 {
        return Err(format!("helper rpc closed before {method} response"));
    }
    let response: serde_json::Value = serde_json::from_str(line.trim())
        .map_err(|err| format!("decode {method} rpc response: {err}"))?;
    if response.get("ok").and_then(|value| value.as_bool()) == Some(true) {
        return Ok(response
            .get("result")
            .cloned()
            .unwrap_or_else(|| serde_json::json!({})));
    }
    let error = response
        .get("error")
        .and_then(|value| value.as_str())
        .unwrap_or("helper rpc failed");
    Err(error.to_string())
}

fn is_stop_requested(stop_signal: Option<&Arc<AtomicBool>>) -> bool {
    stop_signal
        .map(|signal| signal.load(Ordering::SeqCst))
        .unwrap_or(false)
}

fn write_service_log(message: &str) {
    let timestamp = current_timestamp_ms() / 1000;
    let log_path = service_log_path();
    if let Some(parent) = log_path.parent() {
        let _ = std::fs::create_dir_all(parent);
    }
    if let Ok(mut file) = OpenOptions::new().create(true).append(true).open(&log_path) {
        let _ = writeln!(file, "[{timestamp}] {message}");
    }
}

fn current_timestamp_ms() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|duration| duration.as_millis() as u64)
        .unwrap_or_default()
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

fn helper_log_path() -> PathBuf {
    if let Ok(configured) = std::env::var("SLAN_APP_CORE_HELPER_LOG") {
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
                .join("app-core-helper.log");
        }
    }
    #[cfg(target_os = "windows")]
    return PathBuf::from(r"C:\ProgramData\SLAN\app-core-helper.log");
    #[cfg(not(target_os = "windows"))]
    {
        if let Ok(current_exe) = std::env::current_exe() {
            if let Some(parent) = current_exe.parent() {
                return parent.join("app-core-helper.log");
            }
        }
        std::env::temp_dir().join("app-core-helper.log")
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

#[cfg(test)]
mod tests {
    use super::*;
    use std::net::TcpListener;
    use std::path::PathBuf;
    use std::sync::mpsc;
    use std::sync::{Mutex, OnceLock};

    fn env_lock() -> std::sync::MutexGuard<'static, ()> {
        static LOCK: OnceLock<Mutex<()>> = OnceLock::new();
        LOCK.get_or_init(|| Mutex::new(()))
            .lock()
            .expect("env lock")
    }

    fn unique_state_path(name: &str) -> PathBuf {
        let now_ms = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .expect("system time before unix epoch")
            .as_millis();
        std::env::temp_dir().join(format!(
            "slan-service-{name}-{}-{}.json",
            std::process::id(),
            now_ms
        ))
    }

    fn run_single_rpc_server(response: &'static str) -> (String, mpsc::Receiver<String>) {
        let listener = TcpListener::bind("127.0.0.1:0").expect("bind test helper rpc server");
        let address = listener.local_addr().expect("read listener address");
        let (tx, rx) = mpsc::channel();
        thread::spawn(move || {
            let (stream, _) = listener.accept().expect("accept helper rpc client");
            let reader_stream = stream.try_clone().expect("clone helper rpc stream");
            let mut reader = BufReader::new(reader_stream);
            let mut line = String::new();
            reader.read_line(&mut line).expect("read helper rpc line");
            tx.send(line).expect("send captured helper rpc line");
            let mut writer = stream;
            writer
                .write_all(response.as_bytes())
                .expect("write helper rpc response");
            writer
                .write_all(b"\n")
                .expect("write helper rpc response newline");
        });
        (address.to_string(), rx)
    }

    #[cfg(target_os = "windows")]
    fn spawn_sleep_child() -> Child {
        Command::new("cmd")
            .args(["/C", "timeout", "/T", "5", "/NOBREAK"])
            .stdout(std::process::Stdio::null())
            .stderr(std::process::Stdio::null())
            .spawn()
            .expect("spawn sleep child")
    }

    #[cfg(not(target_os = "windows"))]
    fn spawn_sleep_child() -> Child {
        Command::new("sh")
            .args(["-c", "sleep 5"])
            .stdout(std::process::Stdio::null())
            .stderr(std::process::Stdio::null())
            .spawn()
            .expect("spawn sleep child")
    }

    #[cfg(target_os = "windows")]
    fn spawn_exiting_child() -> Child {
        Command::new("cmd")
            .args(["/C", "exit", "17"])
            .stdout(std::process::Stdio::null())
            .stderr(std::process::Stdio::null())
            .spawn()
            .expect("spawn exiting child")
    }

    #[cfg(not(target_os = "windows"))]
    fn spawn_exiting_child() -> Child {
        Command::new("sh")
            .args(["-c", "exit 17"])
            .stdout(std::process::Stdio::null())
            .stderr(std::process::Stdio::null())
            .spawn()
            .expect("spawn exiting child")
    }

    #[test]
    fn invoke_helper_method_sends_method_name_and_accepts_ok_response() {
        let (address, rx) = run_single_rpc_server(r#"{"ok":true,"result":{"done":true}}"#);

        invoke_helper_method(&address, "controlSync").expect("control sync rpc should succeed");

        let request = rx
            .recv_timeout(Duration::from_secs(1))
            .expect("helper rpc request");
        let json: serde_json::Value =
            serde_json::from_str(request.trim()).expect("decode helper rpc request");
        assert_eq!(json["method"], "controlSync");
        assert_eq!(json["args"], serde_json::json!({}));
    }

    #[test]
    fn invoke_helper_method_returns_helper_error() {
        let (address, rx) =
            run_single_rpc_server(r#"{"ok":false,"error":"missing active session"}"#);

        let error = invoke_helper_method(&address, "reportDeviceNetworkState")
            .expect_err("helper rpc should return error");

        let request = rx
            .recv_timeout(Duration::from_secs(1))
            .expect("helper rpc request");
        let json: serde_json::Value =
            serde_json::from_str(request.trim()).expect("decode helper rpc request");
        assert_eq!(json["method"], "reportDeviceNetworkState");
        assert_eq!(error, "missing active session");
    }

    #[test]
    fn helper_rpc_health_uses_helper_status_rpc() {
        let (address, rx) =
            run_single_rpc_server(r#"{"ok":true,"result":{"helperReachable":true}}"#);

        helper_rpc_is_healthy(&address).expect("helper status rpc should succeed");

        let request = rx
            .recv_timeout(Duration::from_secs(1))
            .expect("helper rpc request");
        let json: serde_json::Value =
            serde_json::from_str(request.trim()).expect("decode helper rpc request");
        assert_eq!(json["method"], "helperStatus");
        assert_eq!(json["args"], serde_json::json!({}));
    }

    #[test]
    fn helper_rpc_health_rejects_unreachable_helper_status_payload() {
        let (address, rx) =
            run_single_rpc_server(r#"{"ok":true,"result":{"helperReachable":false}}"#);

        let error = helper_rpc_is_healthy(&address).expect_err("helper status should be unhealthy");

        let request = rx
            .recv_timeout(Duration::from_secs(1))
            .expect("helper rpc request");
        let json: serde_json::Value =
            serde_json::from_str(request.trim()).expect("decode helper rpc request");
        assert_eq!(json["method"], "helperStatus");
        assert_eq!(error, "helperStatus reported helperReachable=false");
    }

    #[test]
    fn wait_for_helper_rpc_ready_times_out_when_helper_never_listens() {
        let mut child = spawn_sleep_child();
        let error =
            wait_for_helper_rpc_ready("127.0.0.1:9", &mut child, Duration::from_millis(20), None)
                .expect_err("helper readiness should time out");
        let _ = child.kill();
        let _ = child.wait();

        assert!(error.contains("helper rpc did not become ready"));
    }

    #[test]
    fn wait_for_helper_rpc_ready_reports_early_helper_exit() {
        let mut child = spawn_exiting_child();
        thread::sleep(Duration::from_millis(50));

        let error =
            wait_for_helper_rpc_ready("127.0.0.1:9", &mut child, Duration::from_secs(1), None)
                .expect_err("helper readiness should detect early exit");

        assert!(error.contains("exited before rpc was ready"));
    }

    #[test]
    fn resolve_control_base_url_prefers_config_over_persisted_state() {
        let _guard = env_lock();
        let path = unique_state_path("config-first");
        std::fs::write(&path, r#"{"controlBaseUrl":"http://persisted.example"}"#)
            .expect("write persisted service state");
        unsafe {
            std::env::set_var("SLAN_APP_CORE_STATE_FILE", &path);
        }
        let mut config = ServiceConfig::default();
        config.control_base_url = Some(" http://config.example ".into());

        assert_eq!(
            resolve_control_base_url(&config).expect("resolve control base url"),
            "http://config.example"
        );

        unsafe {
            std::env::remove_var("SLAN_APP_CORE_STATE_FILE");
        }
        let _ = std::fs::remove_file(path);
    }

    #[test]
    fn resolve_control_base_url_falls_back_to_persisted_state() {
        let _guard = env_lock();
        let path = unique_state_path("persisted");
        std::fs::write(&path, r#"{"controlBaseUrl":" http://persisted.example "}"#)
            .expect("write persisted service state");
        unsafe {
            std::env::set_var("SLAN_APP_CORE_STATE_FILE", &path);
        }

        assert_eq!(
            resolve_control_base_url(&ServiceConfig::default()).expect("resolve control base url"),
            "http://persisted.example"
        );

        unsafe {
            std::env::remove_var("SLAN_APP_CORE_STATE_FILE");
        }
        let _ = std::fs::remove_file(path);
    }
}
