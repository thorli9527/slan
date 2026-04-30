#![cfg_attr(
    not(target_os = "windows"),
    allow(dead_code, unused_imports, unused_variables)
)]

#[cfg(target_os = "windows")]
use libloading::Library;
use std::collections::HashMap;
#[cfg(target_os = "windows")]
use std::ffi::OsStr;
#[cfg(target_os = "windows")]
use std::iter;
#[cfg(target_os = "windows")]
use std::os::windows::ffi::OsStrExt;
#[cfg(target_os = "windows")]
use std::path::{Path, PathBuf};
#[cfg(target_os = "windows")]
use std::process::Command;
use std::sync::Mutex;
#[cfg(target_os = "windows")]
use std::thread;
use std::time::{Duration, SystemTime, UNIX_EPOCH};

#[cfg(target_os = "windows")]
use windows_sys::Win32::Devices::DeviceAndDriverInstallation::{
    SetupDiCallClassInstaller, SetupDiCreateDeviceInfoList, SetupDiCreateDeviceInfoW,
    SetupDiDestroyDeviceInfoList, SetupDiSetDeviceRegistryPropertyW,
    UpdateDriverForPlugAndPlayDevicesW, DICD_GENERATE_ID, DIF_REGISTERDEVICE, GUID_DEVCLASS_NET,
    HDEVINFO, INSTALLFLAG_FORCE, SPDRP_FRIENDLYNAME, SPDRP_HARDWAREID, SP_DEVINFO_DATA,
};
#[cfg(target_os = "windows")]
use windows_sys::Win32::Foundation::{GetLastError, HWND, INVALID_HANDLE_VALUE};

use slan_app_core::{
    TunnelTransport, WireGuardInterfaceConfig, WireGuardPeerConfig, WireGuardRuntimeStats,
};

use crate::{TunnelBackend, TunnelBackendDiagnostics, TunnelConfig};

const WINDOWS_LOOPBACK_INTERFACE_ALIAS: &str = "Loopback Pseudo-Interface 1";
const WINDOWS_KMTEST_LOOPBACK_DESCRIPTION: &str = "Microsoft KM-TEST Loopback Adapter";
const WINDOWS_DEDICATED_INTERFACE_ALIAS: &str = "SLAN LAN Adapter";
const WINDOWS_NETLOOP_INF: &str = r"C:\Windows\INF\netloop.inf";
#[cfg(target_os = "windows")]
const WINDOWS_MSLOOP_HARDWARE_ID: &str = "*MSLOOP";
#[cfg(target_os = "windows")]
const WINDOWS_WINTUN_DRIVER_TYPE: &str = "Wintun";

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum WindowsDedicatedDriverKind {
    KmTest,
    Wintun,
}

#[cfg(target_os = "windows")]
#[repr(C)]
struct RawGuid {
    data1: u32,
    data2: u16,
    data3: u16,
    data4: [u8; 8],
}

#[cfg(target_os = "windows")]
type WintunAdapterHandle = *mut std::ffi::c_void;
#[cfg(target_os = "windows")]
type WintunCreateAdapterFunc =
    unsafe extern "system" fn(*const u16, *const u16, *const RawGuid) -> WintunAdapterHandle;
#[cfg(target_os = "windows")]
type WintunOpenAdapterFunc = unsafe extern "system" fn(*const u16) -> WintunAdapterHandle;
#[cfg(target_os = "windows")]
type WintunCloseAdapterFunc = unsafe extern "system" fn(WintunAdapterHandle);
#[cfg(target_os = "windows")]
type WintunSessionHandle = *mut std::ffi::c_void;
#[cfg(target_os = "windows")]
type WintunStartSessionFunc =
    unsafe extern "system" fn(WintunAdapterHandle, u32) -> WintunSessionHandle;
#[cfg(target_os = "windows")]
type WintunEndSessionFunc = unsafe extern "system" fn(WintunSessionHandle);

#[cfg(target_os = "windows")]
struct WindowsWintunRuntimeAdapter {
    _library: Library,
    handle: WintunAdapterHandle,
    session: WintunSessionHandle,
    end_session: WintunEndSessionFunc,
    close_adapter: WintunCloseAdapterFunc,
}

#[cfg(target_os = "windows")]
unsafe impl Send for WindowsWintunRuntimeAdapter {}

#[cfg(target_os = "windows")]
impl Drop for WindowsWintunRuntimeAdapter {
    fn drop(&mut self) {
        if !self.session.is_null() {
            unsafe {
                (self.end_session)(self.session);
            }
            self.session = std::ptr::null_mut();
        }
        if !self.handle.is_null() {
            unsafe {
                (self.close_adapter)(self.handle);
            }
            self.handle = std::ptr::null_mut();
        }
    }
}

#[derive(Debug, Clone)]
struct WindowsInterfaceRuntime {
    interface_name: String,
    interface_index: Option<u32>,
    is_dedicated_adapter: bool,
    local_virtual_ip: String,
    local_prefix_len: u8,
    dns_servers: Vec<String>,
    is_up: bool,
}

#[derive(Debug, Clone)]
struct WindowsPeerRuntime {
    peer: WireGuardPeerConfig,
    latest_handshake_at_ms: Option<u64>,
}

/// Windows tunnel backend.
///
/// Current behavior:
/// - keeps interface / peer runtime in memory
/// - on Windows, tries to project the local virtual IP onto a system interface
///   with `netsh`, so the address is visible from `ipconfig`
/// - currently supports either a legacy KM-TEST path or a Wintun/WireGuardNT-backed
///   dedicated adapter path selected via `SLAN_WINDOWS_TUNNEL_DRIVER`
///
/// The target interface defaults to the dedicated `SLAN LAN Adapter` alias on
/// Windows and can be overridden with `SLAN_WINDOWS_TUNNEL_INTERFACE_ALIAS`.
#[derive(Default)]
pub struct WindowsEmbeddableServiceBackend {
    interface: Mutex<Option<WindowsInterfaceRuntime>>,
    peers: Mutex<HashMap<String, WindowsPeerRuntime>>,
    #[cfg(target_os = "windows")]
    wintun_adapter: Mutex<Option<WindowsWintunRuntimeAdapter>>,
}

impl WindowsEmbeddableServiceBackend {
    pub fn new() -> Self {
        Self::default()
    }

    pub fn planned_peer_ips(&self) -> Vec<String> {
        self.peers
            .lock()
            .map(|peers| peers.keys().cloned().collect())
            .unwrap_or_default()
    }

    pub fn interface_name(&self) -> Option<String> {
        self.interface
            .lock()
            .ok()
            .and_then(|state| state.as_ref().map(|runtime| runtime.interface_name.clone()))
    }

    pub fn is_up(&self) -> bool {
        self.interface
            .lock()
            .ok()
            .and_then(|state| state.as_ref().map(|runtime| runtime.is_up))
            .unwrap_or(false)
    }

    fn validate_interface(interface: &WireGuardInterfaceConfig) -> Result<(), String> {
        if interface.key_pair.public_key.trim().is_empty() {
            return Err("windows backend requires non-empty wireguard public key".to_string());
        }
        if interface.key_pair.private_key.trim().is_empty() {
            return Err("windows backend requires non-empty wireguard private key".to_string());
        }
        if interface.addresses.is_empty() {
            return Err(
                "windows backend requires at least one wireguard interface address".to_string(),
            );
        }
        Ok(())
    }

    fn validate_peer(peer_virtual_ip: &str, peer: &WireGuardPeerConfig) -> Result<(), String> {
        if peer_virtual_ip.trim().is_empty() {
            return Err("windows backend requires non-empty peer virtual ip".to_string());
        }
        if peer.public_key.trim().is_empty() {
            return Err("windows backend requires non-empty peer public key".to_string());
        }
        if peer.allowed_ips.is_empty() {
            return Err("windows backend requires at least one allowed ip".to_string());
        }
        Ok(())
    }

    fn validate_config(config: &TunnelConfig) -> Result<(), String> {
        if config.local_virtual_ip.trim().is_empty() {
            return Err("windows backend requires non-empty local virtual ip".to_string());
        }
        if config.peer_virtual_ip.trim().is_empty() {
            return Err("windows backend requires non-empty peer virtual ip".to_string());
        }
        if config.local_virtual_ip == config.peer_virtual_ip {
            return Err("windows backend requires distinct local and peer virtual ip".to_string());
        }
        Self::validate_interface(&config.wireguard_interface)?;
        Self::validate_peer(&config.peer_virtual_ip, &config.wireguard_peer)
    }

    fn current_timestamp_ms() -> u64 {
        SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .map(|duration| duration.as_millis() as u64)
            .unwrap_or_default()
    }

    fn trim_ascii_whitespace(raw: &str) -> &str {
        raw.trim_matches(|char| matches!(char, ' ' | '\r' | '\n' | '\t'))
    }

    #[cfg(target_os = "windows")]
    fn wide_null(raw: &str) -> Vec<u16> {
        OsStr::new(raw).encode_wide().chain(iter::once(0)).collect()
    }

    fn dedicated_driver_kind() -> WindowsDedicatedDriverKind {
        match std::env::var("SLAN_WINDOWS_TUNNEL_DRIVER")
            .ok()
            .map(|value| value.trim().to_ascii_lowercase())
            .as_deref()
        {
            Some("kmtest") | Some("loopback") | Some("msloop") => {
                WindowsDedicatedDriverKind::KmTest
            }
            Some("wireguardnt") | Some("wintun") | None | Some("") => {
                WindowsDedicatedDriverKind::Wintun
            }
            Some(_) => WindowsDedicatedDriverKind::Wintun,
        }
    }

    #[cfg(target_os = "windows")]
    fn command_stdout(program: &str, args: &[&str]) -> Result<String, String> {
        let output = Command::new(program)
            .args(args)
            .output()
            .map_err(|err| format!("failed to launch {program}: {err}"))?;
        if output.status.success() {
            return Ok(String::from_utf8_lossy(&output.stdout).to_string());
        }
        let stderr = String::from_utf8_lossy(&output.stderr).trim().to_string();
        let stdout = String::from_utf8_lossy(&output.stdout).trim().to_string();
        let detail = if !stderr.is_empty() {
            stderr
        } else if !stdout.is_empty() {
            stdout
        } else {
            format!("exit status {}", output.status)
        };
        Err(format!("{program} {} failed: {detail}", args.join(" ")))
    }

    fn resolve_interface_target(requested: Option<&str>) -> (String, Option<u32>, bool) {
        if let Some(configured_alias) = std::env::var("SLAN_WINDOWS_TUNNEL_INTERFACE_ALIAS")
            .ok()
            .map(|value| value.trim().to_string())
            .filter(|value| !value.is_empty())
        {
            return (configured_alias, None, false);
        }

        let wants_dedicated_alias = requested
            .map(str::trim)
            .filter(|value| !value.is_empty())
            .map(|value| {
                value.eq_ignore_ascii_case("slan0")
                    || value.eq_ignore_ascii_case(WINDOWS_DEDICATED_INTERFACE_ALIAS)
            })
            .unwrap_or(true);

        if let Some(requested_alias) = requested.map(str::trim).filter(|value| {
            !value.is_empty()
                && *value != "slan0"
                && !value.eq_ignore_ascii_case(WINDOWS_DEDICATED_INTERFACE_ALIAS)
        }) {
            return (requested_alias.to_string(), None, false);
        }

        #[cfg(target_os = "windows")]
        match Self::dedicated_driver_kind() {
            WindowsDedicatedDriverKind::Wintun => {
                if let Some(target) = Self::find_wintun_adapter() {
                    let (_, interface_index, is_dedicated_adapter) = target;
                    return (
                        WINDOWS_DEDICATED_INTERFACE_ALIAS.to_string(),
                        interface_index,
                        is_dedicated_adapter,
                    );
                }
            }
            WindowsDedicatedDriverKind::KmTest => {
                if let Some(target) = Self::find_kmtest_loopback_adapter() {
                    let (_, interface_index, is_dedicated_adapter) = target;
                    return (
                        WINDOWS_DEDICATED_INTERFACE_ALIAS.to_string(),
                        interface_index,
                        is_dedicated_adapter,
                    );
                }

                if let Err(err) = Self::ensure_kmtest_loopback_adapter() {
                    eprintln!("windows backend failed to ensure KM-TEST loopback adapter: {err}");
                }
                if let Some(target) = Self::find_kmtest_loopback_adapter() {
                    let (_, interface_index, is_dedicated_adapter) = target;
                    return (
                        WINDOWS_DEDICATED_INTERFACE_ALIAS.to_string(),
                        interface_index,
                        is_dedicated_adapter,
                    );
                }
            }
        }

        #[cfg(target_os = "windows")]
        if std::env::var("SLAN_WINDOWS_REQUIRE_DEDICATED_ADAPTER")
            .ok()
            .as_deref()
            == Some("1")
        {
            return (WINDOWS_DEDICATED_INTERFACE_ALIAS.to_string(), None, true);
        }

        #[cfg(target_os = "windows")]
        if wants_dedicated_alias {
            return (WINDOWS_DEDICATED_INTERFACE_ALIAS.to_string(), None, true);
        }

        (WINDOWS_LOOPBACK_INTERFACE_ALIAS.to_string(), None, false)
    }

    #[cfg(target_os = "windows")]
    fn find_kmtest_loopback_adapter() -> Option<(String, Option<u32>, bool)> {
        #[cfg(target_os = "windows")]
        {
            let script = format!(
                "$adapter = Get-NetAdapter -IncludeHidden -ErrorAction SilentlyContinue | Where-Object {{ $_.InterfaceDescription -like '*KM-TEST*Loopback Adapter*' }} | Select-Object -First 1 Name, InterfaceIndex\n\
                 if ($adapter) {{ Write-Output ($adapter.Name + '|' + $adapter.InterfaceIndex) }}\n"
            );
            if let Ok(output) = Self::command_stdout(
                "powershell.exe",
                &[
                    "-NoProfile",
                    "-NonInteractive",
                    "-ExecutionPolicy",
                    "Bypass",
                    "-Command",
                    &script,
                ],
            ) {
                let line = output.lines().map(str::trim).find(|line| !line.is_empty());
                if let Some(line) = line {
                    let mut parts = line.split('|');
                    let alias = parts.next().map(str::trim).unwrap_or_default();
                    let interface_index = parts
                        .next()
                        .and_then(|value| value.trim().parse::<u32>().ok());
                    if !alias.is_empty() {
                        return Some((alias.to_string(), interface_index, true));
                    }
                }
            }

            if let Ok(route_output) = Self::command_stdout("route", &["print"]) {
                let kmtest_index = route_output.lines().find_map(|line| {
                    if !line.contains("Microsoft KM-TEST")
                        && !line.contains(WINDOWS_DEDICATED_INTERFACE_ALIAS)
                    {
                        return None;
                    }
                    let trimmed = Self::trim_ascii_whitespace(line);
                    let first = trimmed.split_whitespace().next()?;
                    first.parse::<u32>().ok()
                });
                if let Some(interface_index) = kmtest_index {
                    if let Ok(netsh_output) =
                        Self::command_stdout("netsh", &["interface", "ipv4", "show", "interfaces"])
                    {
                        for line in netsh_output.lines() {
                            let trimmed = Self::trim_ascii_whitespace(line);
                            let columns = trimmed.split_whitespace().collect::<Vec<_>>();
                            if columns.len() < 5 {
                                continue;
                            }
                            let Ok(parsed_index) = columns[0].parse::<u32>() else {
                                continue;
                            };
                            if parsed_index != interface_index {
                                continue;
                            }
                            if let Some(alias_start) = trimmed.find(columns[4]) {
                                let alias = trimmed[alias_start..].trim().to_string();
                                if !alias.is_empty() {
                                    return Some((alias, Some(interface_index), true));
                                }
                            }
                        }
                    }
                    return Some((
                        WINDOWS_KMTEST_LOOPBACK_DESCRIPTION.to_string(),
                        Some(interface_index),
                        true,
                    ));
                }
            }
        }
        None
    }

    #[cfg(target_os = "windows")]
    fn find_wintun_adapter() -> Option<(String, Option<u32>, bool)> {
        let escaped_alias = WINDOWS_DEDICATED_INTERFACE_ALIAS.replace('\'', "''");
        let script = format!(
            concat!(
                "$adapter = Get-NetAdapter -IncludeHidden -ErrorAction SilentlyContinue | Where-Object {{ ",
                "$_.Name -eq '{escaped_alias}' -or ",
                "$_.InterfaceDescription -like '*Wintun*' -or ",
                "$_.InterfaceDescription -like '*WireGuard*Tunnel*' -or ",
                "$_.InterfaceDescription -like '*WireGuardNT*' ",
                "}} | Select-Object -First 1 Name, InterfaceIndex\n\
                 if ($adapter) {{ Write-Output ($adapter.Name + '|' + $adapter.InterfaceIndex) }}\n"
            ),
            escaped_alias = escaped_alias,
        );
        if let Ok(output) = Self::command_stdout(
            "powershell.exe",
            &[
                "-NoProfile",
                "-NonInteractive",
                "-ExecutionPolicy",
                "Bypass",
                "-Command",
                &script,
            ],
        ) {
            let line = output.lines().map(str::trim).find(|line| !line.is_empty());
            if let Some(line) = line {
                let mut parts = line.split('|');
                let alias = parts.next().map(str::trim).unwrap_or_default();
                let interface_index = parts
                    .next()
                    .and_then(|value| value.trim().parse::<u32>().ok());
                if !alias.is_empty() {
                    return Some((alias.to_string(), interface_index, true));
                }
            }
        }
        None
    }

    #[cfg(target_os = "windows")]
    fn wait_for_wintun_adapter() -> Option<(String, Option<u32>, bool)> {
        for _ in 0..20 {
            if let Some(adapter) = Self::find_wintun_adapter() {
                return Some(adapter);
            }
            thread::sleep(Duration::from_millis(250));
        }
        None
    }

    #[cfg(target_os = "windows")]
    fn wintun_dll_candidates() -> Vec<PathBuf> {
        let mut candidates = Vec::new();
        if let Ok(current_exe) = std::env::current_exe() {
            if let Some(parent) = current_exe.parent() {
                candidates.push(parent.join("wintun.dll"));
            }
        }
        candidates.push(
            Path::new(env!("CARGO_MANIFEST_DIR"))
                .join("../../../drivers/wintun/bin/amd64/wintun.dll"),
        );
        candidates
    }

    #[cfg(target_os = "windows")]
    fn load_wintun_library() -> Result<Library, String> {
        let mut attempted = Vec::new();
        for candidate in Self::wintun_dll_candidates() {
            attempted.push(candidate.display().to_string());
            if candidate.exists() {
                let library = unsafe { Library::new(&candidate) }.map_err(|err| {
                    format!(
                        "failed to load wintun.dll from {}: {err}",
                        candidate.display()
                    )
                })?;
                return Ok(library);
            }
        }
        Err(format!(
            "wintun.dll not found; searched: {}",
            attempted.join(", ")
        ))
    }

    #[cfg(target_os = "windows")]
    fn ensure_wintun_adapter() -> Result<(), String> {
        if Self::find_wintun_adapter().is_some() {
            Self::rename_wintun_adapter(WINDOWS_DEDICATED_INTERFACE_ALIAS)?;
            return Ok(());
        }

        let library = Self::load_wintun_library()?;
        let create_adapter = unsafe {
            let symbol: libloading::Symbol<WintunCreateAdapterFunc> = library
                .get(b"WintunCreateAdapter\0")
                .map_err(|err| format!("failed to resolve WintunCreateAdapter: {err}"))?;
            *symbol
        };
        let open_adapter = unsafe {
            let symbol: libloading::Symbol<WintunOpenAdapterFunc> = library
                .get(b"WintunOpenAdapter\0")
                .map_err(|err| format!("failed to resolve WintunOpenAdapter: {err}"))?;
            *symbol
        };
        let close_adapter = unsafe {
            let symbol: libloading::Symbol<WintunCloseAdapterFunc> = library
                .get(b"WintunCloseAdapter\0")
                .map_err(|err| format!("failed to resolve WintunCloseAdapter: {err}"))?;
            *symbol
        };
        let adapter_name = Self::wide_null(WINDOWS_DEDICATED_INTERFACE_ALIAS);
        let tunnel_type = Self::wide_null(WINDOWS_WINTUN_DRIVER_TYPE);
        let mut handle = unsafe { open_adapter(adapter_name.as_ptr()) };
        let mut created_adapter = false;
        if handle.is_null() {
            handle = unsafe {
                create_adapter(
                    adapter_name.as_ptr(),
                    tunnel_type.as_ptr(),
                    std::ptr::null(),
                )
            };
            created_adapter = !handle.is_null();
        }
        if handle.is_null() {
            return Err(format!(
                "failed to create or open Wintun adapter '{}': {}",
                WINDOWS_DEDICATED_INTERFACE_ALIAS,
                Self::last_error_message()
            ));
        }
        unsafe {
            close_adapter(handle);
        }
        if Self::find_wintun_adapter().is_some() {
            Self::rename_wintun_adapter(WINDOWS_DEDICATED_INTERFACE_ALIAS)?;
        } else if !created_adapter {
            return Err(format!(
                "opened Wintun adapter '{}' but could not locate it in Windows adapter inventory",
                WINDOWS_DEDICATED_INTERFACE_ALIAS
            ));
        }
        Ok(())
    }

    #[cfg(target_os = "windows")]
    fn open_runtime_wintun_adapter() -> Result<WindowsWintunRuntimeAdapter, String> {
        let library = Self::load_wintun_library()?;
        let create_adapter = unsafe {
            let symbol: libloading::Symbol<WintunCreateAdapterFunc> = library
                .get(b"WintunCreateAdapter\0")
                .map_err(|err| format!("failed to resolve WintunCreateAdapter: {err}"))?;
            *symbol
        };
        let open_adapter = unsafe {
            let symbol: libloading::Symbol<WintunOpenAdapterFunc> = library
                .get(b"WintunOpenAdapter\0")
                .map_err(|err| format!("failed to resolve WintunOpenAdapter: {err}"))?;
            *symbol
        };
        let close_adapter = unsafe {
            let symbol: libloading::Symbol<WintunCloseAdapterFunc> = library
                .get(b"WintunCloseAdapter\0")
                .map_err(|err| format!("failed to resolve WintunCloseAdapter: {err}"))?;
            *symbol
        };
        let start_session = unsafe {
            let symbol: libloading::Symbol<WintunStartSessionFunc> = library
                .get(b"WintunStartSession\0")
                .map_err(|err| format!("failed to resolve WintunStartSession: {err}"))?;
            *symbol
        };
        let end_session = unsafe {
            let symbol: libloading::Symbol<WintunEndSessionFunc> = library
                .get(b"WintunEndSession\0")
                .map_err(|err| format!("failed to resolve WintunEndSession: {err}"))?;
            *symbol
        };

        let adapter_name = Self::wide_null(WINDOWS_DEDICATED_INTERFACE_ALIAS);
        let tunnel_type = Self::wide_null(WINDOWS_WINTUN_DRIVER_TYPE);
        let mut handle = unsafe { open_adapter(adapter_name.as_ptr()) };
        if handle.is_null() {
            handle = unsafe {
                create_adapter(
                    adapter_name.as_ptr(),
                    tunnel_type.as_ptr(),
                    std::ptr::null(),
                )
            };
        }
        if handle.is_null() {
            return Err(format!(
                "failed to create or open runtime Wintun adapter '{}': {}",
                WINDOWS_DEDICATED_INTERFACE_ALIAS,
                Self::last_error_message()
            ));
        }
        let session = unsafe { start_session(handle, 0x400000) };
        if session.is_null() {
            let error = Self::last_error_message();
            unsafe {
                close_adapter(handle);
            }
            return Err(format!(
                "failed to start runtime Wintun session for '{}': {error}",
                WINDOWS_DEDICATED_INTERFACE_ALIAS
            ));
        }

        Ok(WindowsWintunRuntimeAdapter {
            _library: library,
            handle,
            session,
            end_session,
            close_adapter,
        })
    }

    #[cfg(target_os = "windows")]
    fn ensure_runtime_wintun_adapter_index(&self) -> Result<u32, String> {
        if let Some((_, Some(interface_index), _)) = Self::find_wintun_adapter() {
            return Ok(interface_index);
        }

        let mut adapter = self
            .wintun_adapter
            .lock()
            .map_err(|_| "windows backend wintun adapter state poisoned".to_string())?;
        if adapter.is_none() {
            *adapter = Some(Self::open_runtime_wintun_adapter()?);
        }
        drop(adapter);

        if let Some((alias, interface_index, _)) = Self::wait_for_wintun_adapter() {
            Self::rename_wintun_adapter(WINDOWS_DEDICATED_INTERFACE_ALIAS)?;
            return interface_index.ok_or_else(|| {
                format!("runtime Wintun adapter '{alias}' was created but has no interface index")
            });
        }

        Err(format!(
            "runtime Wintun adapter '{}' was created but was not visible in Windows adapter inventory",
            WINDOWS_DEDICATED_INTERFACE_ALIAS
        ))
    }

    #[cfg(target_os = "windows")]
    fn rename_dedicated_adapter_by_description_patterns(
        patterns: &[&str],
        alias: &str,
    ) -> Result<(), String> {
        let escaped_alias = alias.replace('\'', "''");
        let filter = patterns
            .iter()
            .map(|pattern| {
                format!(
                    "$_.InterfaceDescription -like '{}'",
                    pattern.replace('\'', "''")
                )
            })
            .collect::<Vec<_>>()
            .join(" -or ");
        let script = format!(
            "$ErrorActionPreference='Stop'\n\
             $adapter = Get-NetAdapter -IncludeHidden -ErrorAction SilentlyContinue | Where-Object {{ {filter} }} | Select-Object -First 1\n\
             if (-not $adapter) {{ throw 'Dedicated adapter not found for rename.' }}\n\
             if ($adapter.Name -ne '{escaped_alias}') {{ Rename-NetAdapter -Name $adapter.Name -NewName '{escaped_alias}' -Confirm:$false -ErrorAction Stop | Out-Null }}\n"
        );
        Self::run_powershell_script(&script, true)
    }

    #[cfg(target_os = "windows")]
    fn rename_kmtest_loopback_adapter(alias: &str) -> Result<(), String> {
        Self::rename_dedicated_adapter_by_description_patterns(
            &["*KM-TEST*Loopback Adapter*"],
            alias,
        )
    }

    #[cfg(target_os = "windows")]
    fn rename_wintun_adapter(alias: &str) -> Result<(), String> {
        Self::rename_dedicated_adapter_by_description_patterns(
            &["*Wintun*", "*WireGuard*Tunnel*", "*WireGuardNT*"],
            alias,
        )
    }

    #[cfg(target_os = "windows")]
    fn cleanup_kmtest_loopback_adapters() -> Result<(), String> {
        let output = Self::command_stdout("pnputil.exe", &["/enum-devices", "/class", "Net"])?;
        let mut instance_ids = Vec::new();
        let mut pending_instance_id: Option<String> = None;
        for raw_line in output.lines() {
            let line = raw_line.trim();
            if let Some(rest) = line.strip_prefix("Instance ID:") {
                pending_instance_id = Some(rest.trim().to_string());
                continue;
            }
            if line.contains("KM-TEST")
                || line.contains("Loopback Adapter")
                || line.contains("MICROSOFT_KM-TEST_LOOPBACK_ADAPTER")
            {
                if let Some(instance_id) = pending_instance_id.take() {
                    if instance_id
                        .to_ascii_uppercase()
                        .contains("MICROSOFT_KM-TEST_LOOPBACK_ADAPTER")
                    {
                        instance_ids.push(instance_id);
                    }
                }
            }
        }

        let mut failures = Vec::new();
        for instance_id in instance_ids {
            let output = Command::new("pnputil.exe")
                .args(["/remove-device", instance_id.as_str()])
                .output()
                .map_err(|err| {
                    format!("failed to launch pnputil /remove-device for {instance_id}: {err}")
                })?;
            if !output.status.success() {
                let stderr = String::from_utf8_lossy(&output.stderr).trim().to_string();
                let stdout = String::from_utf8_lossy(&output.stdout).trim().to_string();
                let detail = if !stderr.is_empty() {
                    stderr
                } else if !stdout.is_empty() {
                    stdout
                } else {
                    format!("exit status {}", output.status)
                };
                failures.push(format!("{instance_id}: {detail}"));
            }
        }
        if !failures.is_empty() {
            return Err(format!(
                "failed to remove one or more KM-TEST adapters: {}",
                failures.join("; ")
            ));
        }
        Ok(())
    }

    #[cfg(target_os = "windows")]
    fn kmtest_adapter_instance_ids() -> Result<Vec<String>, String> {
        let output = Self::command_stdout("pnputil.exe", &["/enum-devices", "/class", "Net"])?;
        let mut instance_ids = Vec::new();
        let mut pending_instance_id: Option<String> = None;
        for raw_line in output.lines() {
            let line = raw_line.trim();
            if let Some(rest) = line.strip_prefix("Instance ID:") {
                pending_instance_id = Some(rest.trim().to_string());
                continue;
            }
            if line.contains("KM-TEST")
                || line.contains("Loopback Adapter")
                || line.contains("MICROSOFT_KM-TEST_LOOPBACK_ADAPTER")
            {
                if let Some(instance_id) = pending_instance_id.take() {
                    if instance_id
                        .to_ascii_uppercase()
                        .contains("MICROSOFT_KM-TEST_LOOPBACK_ADAPTER")
                    {
                        instance_ids.push(instance_id);
                    }
                }
            }
        }
        Ok(instance_ids)
    }

    #[cfg(target_os = "windows")]
    fn ensure_kmtest_loopback_adapter() -> Result<(), String> {
        if !std::path::Path::new(WINDOWS_NETLOOP_INF).exists() {
            return Err(format!(
                "windows backend could not find loopback driver inf at {WINDOWS_NETLOOP_INF}"
            ));
        }
        if Self::find_kmtest_loopback_adapter().is_some() {
            Self::rename_kmtest_loopback_adapter(WINDOWS_DEDICATED_INTERFACE_ALIAS)?;
            return Ok(());
        }
        let setupapi_error = Self::create_kmtest_loopback_adapter().err();
        if Self::find_kmtest_loopback_adapter().is_some() {
            Self::rename_kmtest_loopback_adapter(WINDOWS_DEDICATED_INTERFACE_ALIAS)?;
            return Ok(());
        }
        let script = format!(
            "$ErrorActionPreference='Stop'\n\
             $existing = Get-NetAdapter -IncludeHidden -ErrorAction SilentlyContinue | Where-Object {{ $_.InterfaceDescription -like '*KM-TEST*Loopback Adapter*' }}\n\
             if (-not $existing) {{\n\
               pnputil.exe /add-driver '{}' /install | Out-Null\n\
               Start-Sleep -Seconds 2\n\
            }}\n",
            WINDOWS_NETLOOP_INF.replace('\\', "\\\\")
        );
        let powershell_error = Self::run_powershell_script(&script, true).err();
        if Self::find_kmtest_loopback_adapter().is_some() {
            Self::rename_kmtest_loopback_adapter(WINDOWS_DEDICATED_INTERFACE_ALIAS)?;
            return Ok(());
        }

        let mut errors = Vec::new();
        if let Some(error) = setupapi_error {
            errors.push(format!("setupapi: {error}"));
        }
        if let Some(error) = powershell_error {
            errors.push(format!("powershell: {error}"));
        }
        if errors.is_empty() {
            return Err(
                "windows backend could not locate Microsoft KM-TEST Loopback Adapter after preparation"
                    .to_string(),
            );
        }
        Err(format!(
            "windows backend could not locate Microsoft KM-TEST Loopback Adapter after preparation ({})",
            errors.join("; ")
        ))
    }

    pub fn prepare_dedicated_adapter() -> Result<(), String> {
        #[cfg(target_os = "windows")]
        {
            match Self::dedicated_driver_kind() {
                WindowsDedicatedDriverKind::KmTest => {
                    Self::ensure_kmtest_loopback_adapter()?;
                    Self::rename_kmtest_loopback_adapter(WINDOWS_DEDICATED_INTERFACE_ALIAS)?;
                    if Self::find_kmtest_loopback_adapter().is_some() {
                        return Ok(());
                    }
                    return Err(
                        "windows backend could not locate Microsoft KM-TEST Loopback Adapter after preparation"
                            .to_string(),
                    );
                }
                WindowsDedicatedDriverKind::Wintun => {
                    Self::cleanup_kmtest_loopback_adapters()?;
                    let remaining_kmtest = Self::kmtest_adapter_instance_ids()?;
                    if !remaining_kmtest.is_empty() {
                        return Err(format!(
                            "legacy KM-TEST adapters are still present after cleanup: {}",
                            remaining_kmtest.join(", ")
                        ));
                    }
                    Self::ensure_wintun_adapter()?;
                    if let Some((alias, interface_index, _)) = Self::find_wintun_adapter() {
                        Self::rename_wintun_adapter(WINDOWS_DEDICATED_INTERFACE_ALIAS)?;
                        if interface_index.is_none() {
                            return Err(format!(
                                "Wintun adapter '{alias}' was created but has no interface index"
                            ));
                        }
                        return Ok(());
                    }
                    return Ok(());
                }
            }
        }

        #[cfg(not(target_os = "windows"))]
        {
            Ok(())
        }
    }

    #[cfg(target_os = "windows")]
    fn create_kmtest_loopback_adapter() -> Result<(), String> {
        unsafe {
            let device_info_set = SetupDiCreateDeviceInfoList(&GUID_DEVCLASS_NET, 0 as HWND);
            if Self::is_invalid_device_info_set(device_info_set) {
                return Err(format!(
                    "failed to create device info list for KM-TEST adapter: {}",
                    Self::last_error_message()
                ));
            }

            let mut device_info_data = SP_DEVINFO_DATA {
                cbSize: std::mem::size_of::<SP_DEVINFO_DATA>() as u32,
                ClassGuid: GUID_DEVCLASS_NET,
                DevInst: 0,
                Reserved: 0,
            };

            let class_name = Self::wide_null(WINDOWS_KMTEST_LOOPBACK_DESCRIPTION);
            if SetupDiCreateDeviceInfoW(
                device_info_set,
                class_name.as_ptr(),
                &GUID_DEVCLASS_NET,
                std::ptr::null(),
                0 as HWND,
                DICD_GENERATE_ID,
                &mut device_info_data,
            ) == 0
            {
                let error = Self::last_error_message();
                SetupDiDestroyDeviceInfoList(device_info_set);
                return Err(format!(
                    "failed to create KM-TEST loopback device info: {error}"
                ));
            }

            let hardware_id = Self::wide_multi_sz(&[WINDOWS_MSLOOP_HARDWARE_ID]);
            if SetupDiSetDeviceRegistryPropertyW(
                device_info_set,
                &mut device_info_data,
                SPDRP_HARDWAREID,
                hardware_id.as_ptr() as *const u8,
                (hardware_id.len() * std::mem::size_of::<u16>()) as u32,
            ) == 0
            {
                let error = Self::last_error_message();
                SetupDiDestroyDeviceInfoList(device_info_set);
                return Err(format!(
                    "failed to set KM-TEST loopback hardware id: {error}"
                ));
            }

            let friendly_name = Self::wide_null(WINDOWS_DEDICATED_INTERFACE_ALIAS);
            if SetupDiSetDeviceRegistryPropertyW(
                device_info_set,
                &mut device_info_data,
                SPDRP_FRIENDLYNAME,
                friendly_name.as_ptr() as *const u8,
                (friendly_name.len() * std::mem::size_of::<u16>()) as u32,
            ) == 0
            {
                let error = Self::last_error_message();
                SetupDiDestroyDeviceInfoList(device_info_set);
                return Err(format!(
                    "failed to set KM-TEST loopback friendly name: {error}"
                ));
            }

            if SetupDiCallClassInstaller(DIF_REGISTERDEVICE, device_info_set, &mut device_info_data)
                == 0
            {
                let error = Self::last_error_message();
                SetupDiDestroyDeviceInfoList(device_info_set);
                return Err(format!(
                    "failed to register KM-TEST loopback device: {error}"
                ));
            }

            let inf_path = Self::wide_null(WINDOWS_NETLOOP_INF);
            let hardware_id = Self::wide_null(WINDOWS_MSLOOP_HARDWARE_ID);
            let mut reboot_required = 0;
            let update_ok = UpdateDriverForPlugAndPlayDevicesW(
                0 as HWND,
                hardware_id.as_ptr(),
                inf_path.as_ptr(),
                INSTALLFLAG_FORCE,
                &mut reboot_required,
            );
            let update_error = if update_ok == 0 {
                Some(Self::last_error_message())
            } else {
                None
            };

            SetupDiDestroyDeviceInfoList(device_info_set);

            if let Some(error) = update_error {
                return Err(format!(
                    "failed to install KM-TEST loopback driver on registered device: {error}"
                ));
            }
        }

        Ok(())
    }

    #[cfg(target_os = "windows")]
    fn wide_multi_sz(values: &[&str]) -> Vec<u16> {
        let mut wide = Vec::new();
        for value in values {
            wide.extend(OsStr::new(value).encode_wide());
            wide.push(0);
        }
        wide.push(0);
        wide
    }

    #[cfg(target_os = "windows")]
    fn is_invalid_device_info_set(handle: HDEVINFO) -> bool {
        handle == INVALID_HANDLE_VALUE as HDEVINFO
    }

    #[cfg(target_os = "windows")]
    fn last_error_message() -> String {
        format!("win32 error {}", unsafe { GetLastError() })
    }

    fn netmask_from_prefix_len(prefix_len: u8) -> Result<String, String> {
        if prefix_len > 32 {
            return Err(format!(
                "windows backend requires ipv4 prefix length <= 32, got {prefix_len}"
            ));
        }
        let mask = if prefix_len == 0 {
            0
        } else {
            u32::MAX << (32 - prefix_len)
        };
        Ok(format!(
            "{}.{}.{}.{}",
            (mask >> 24) & 0xff,
            (mask >> 16) & 0xff,
            (mask >> 8) & 0xff,
            mask & 0xff
        ))
    }

    #[cfg(target_os = "windows")]
    fn run_netsh(args: &[&str]) -> Result<(), String> {
        if std::env::var("SLAN_WINDOWS_TUNNEL_MODE").ok().as_deref() == Some("dry-run") {
            return Ok(());
        }
        let output = Command::new("netsh")
            .args(args)
            .output()
            .map_err(|err| format!("failed to launch netsh: {err}"))?;
        if output.status.success() {
            return Ok(());
        }
        let stderr = String::from_utf8_lossy(&output.stderr).trim().to_string();
        let stdout = String::from_utf8_lossy(&output.stdout).trim().to_string();
        let detail = if !stderr.is_empty() {
            stderr
        } else if !stdout.is_empty() {
            stdout
        } else {
            format!("exit status {}", output.status)
        };
        Err(format!("netsh {} failed: {}", args.join(" "), detail))
    }

    #[cfg(not(target_os = "windows"))]
    fn run_netsh(_args: &[&str]) -> Result<(), String> {
        Ok(())
    }

    #[cfg(target_os = "windows")]
    fn run_powershell_script(script: &str, elevated: bool) -> Result<(), String> {
        if std::env::var("SLAN_WINDOWS_TUNNEL_MODE").ok().as_deref() == Some("dry-run") {
            return Ok(());
        }
        let output = Command::new("powershell.exe")
            .args([
                "-NoProfile",
                "-NonInteractive",
                "-ExecutionPolicy",
                "Bypass",
                "-Command",
                script,
            ])
            .output()
            .map_err(|err| format!("failed to launch powershell: {err}"))?;
        if output.status.success() {
            return Ok(());
        }
        let stderr = String::from_utf8_lossy(&output.stderr).trim().to_string();
        let stdout = String::from_utf8_lossy(&output.stdout).trim().to_string();
        let detail = if !stderr.is_empty() {
            stderr
        } else if !stdout.is_empty() {
            stdout
        } else {
            format!("exit status {}", output.status)
        };
        let elevation_hint = if elevated { " elevated" } else { "" };
        Err(format!(
            "powershell{elevation_hint} script failed: {detail}"
        ))
    }

    #[cfg(not(target_os = "windows"))]
    fn run_powershell_script(_script: &str, _elevated: bool) -> Result<(), String> {
        Ok(())
    }

    fn apply_system_interface(runtime: &WindowsInterfaceRuntime) -> Result<(), String> {
        #[cfg(target_os = "windows")]
        if std::env::var("SLAN_WINDOWS_TUNNEL_MODE").ok().as_deref() == Some("dry-run") {
            return Ok(());
        }
        #[cfg(target_os = "windows")]
        if runtime.is_dedicated_adapter {
            let interface_index = runtime.interface_index.ok_or_else(|| {
                "windows backend dedicated adapter is missing an interface index".to_string()
            })?;
            let dns_servers = powershell_string_array(&runtime.dns_servers);
            let script = format!(
                "$ErrorActionPreference='Stop'\n\
                 $adapter = Get-NetAdapter -InterfaceIndex {interface_index} -ErrorAction Stop\n\
                 $adapter | Enable-NetAdapter -Confirm:$false -ErrorAction SilentlyContinue | Out-Null\n\
                 if ($adapter.Name -ne '{}') {{ Rename-NetAdapter -Name $adapter.Name -NewName '{}' -Confirm:$false -ErrorAction SilentlyContinue | Out-Null }}\n\
                 Set-NetIPInterface -InterfaceIndex {interface_index} -Dhcp Disabled -ErrorAction SilentlyContinue | Out-Null\n\
                 if (Get-Command Set-DnsClientServerAddress -ErrorAction SilentlyContinue) {{ if (@({dns_servers}).Count -gt 0) {{ Set-DnsClientServerAddress -InterfaceIndex {interface_index} -ServerAddresses @({dns_servers}) -ErrorAction Stop | Out-Null }} elseif (Get-Command Reset-DnsClientServerAddress -ErrorAction SilentlyContinue) {{ Reset-DnsClientServerAddress -InterfaceIndex {interface_index} -ErrorAction SilentlyContinue | Out-Null }} }}\n",
                runtime.interface_name,
                runtime.interface_name,
            );
            Self::run_powershell_script(&script, false)?;
            if let Some(primary_dns) = runtime.dns_servers.first() {
                Self::run_netsh(&[
                    "interface",
                    "ipv4",
                    "set",
                    "dnsservers",
                    &format!("name={}", runtime.interface_name),
                    "source=static",
                    &format!("address={primary_dns}"),
                    "register=none",
                    "validate=no",
                ])?;
                for dns in runtime.dns_servers.iter().skip(1) {
                    Self::run_netsh(&[
                        "interface",
                        "ipv4",
                        "add",
                        "dnsservers",
                        &format!("name={}", runtime.interface_name),
                        &format!("address={dns}"),
                        "validate=no",
                    ])?;
                }
            } else {
                let _ = Self::run_netsh(&[
                    "interface",
                    "ipv4",
                    "set",
                    "dnsservers",
                    &format!("name={}", runtime.interface_name),
                    "source=dhcp",
                ]);
            }
            let address_script = format!(
                "$ErrorActionPreference='Stop'\n\
                 Get-NetIPAddress -InterfaceIndex {interface_index} -AddressFamily IPv4 -ErrorAction SilentlyContinue | Remove-NetIPAddress -Confirm:$false -ErrorAction SilentlyContinue\n\
                 New-NetIPAddress -InterfaceIndex {interface_index} -IPAddress '{}' -PrefixLength {} -AddressFamily IPv4 -Type Unicast -ErrorAction Stop | Out-Null\n",
                runtime.local_virtual_ip,
                runtime.local_prefix_len,
            );
            return Self::run_powershell_script(&address_script, false);
        }
        let netmask = Self::netmask_from_prefix_len(runtime.local_prefix_len)?;
        let _ = Self::run_netsh(&[
            "interface",
            "ipv4",
            "delete",
            "address",
            &format!("name={}", runtime.interface_name),
            &format!("addr={}", runtime.local_virtual_ip),
        ]);
        Self::run_netsh(&[
            "interface",
            "ipv4",
            "add",
            "address",
            &format!("name={}", runtime.interface_name),
            &format!("addr={}", runtime.local_virtual_ip),
            &format!("mask={netmask}"),
        ])?;
        if let Some(primary_dns) = runtime.dns_servers.first() {
            Self::run_netsh(&[
                "interface",
                "ipv4",
                "set",
                "dnsservers",
                &format!("name={}", runtime.interface_name),
                "source=static",
                &format!("address={primary_dns}"),
                "register=none",
                "validate=no",
            ])?;
            for dns in runtime.dns_servers.iter().skip(1) {
                Self::run_netsh(&[
                    "interface",
                    "ipv4",
                    "add",
                    "dnsservers",
                    &format!("name={}", runtime.interface_name),
                    &format!("address={dns}"),
                    "validate=no",
                ])?;
            }
        } else {
            let _ = Self::run_netsh(&[
                "interface",
                "ipv4",
                "set",
                "dnsservers",
                &format!("name={}", runtime.interface_name),
                "source=dhcp",
            ]);
        }
        Ok(())
    }

    fn remove_system_interface_address(runtime: &WindowsInterfaceRuntime) -> Result<(), String> {
        #[cfg(target_os = "windows")]
        if std::env::var("SLAN_WINDOWS_TUNNEL_MODE").ok().as_deref() == Some("dry-run") {
            return Ok(());
        }
        #[cfg(target_os = "windows")]
        if runtime.is_dedicated_adapter {
            let adapter_selector = if let Some(interface_index) = runtime.interface_index {
                format!(
                    "Get-NetAdapter -InterfaceIndex {interface_index} -ErrorAction SilentlyContinue"
                )
            } else {
                let escaped_name = runtime.interface_name.replace('\'', "''");
                format!("Get-NetAdapter -Name '{escaped_name}' -ErrorAction SilentlyContinue")
            };
            let escaped_ip = runtime.local_virtual_ip.replace('\'', "''");
            let script = format!(
                "$ErrorActionPreference='Stop'\n\
                 $adapter = {adapter_selector}\n\
                 if ($adapter) {{\n\
                   $ifIndex = $adapter.ifIndex\n\
                   if ('{escaped_ip}'.Length -gt 0 -and (Get-Command Remove-NetIPAddress -ErrorAction SilentlyContinue)) {{\n\
                     Get-NetIPAddress -InterfaceIndex $ifIndex -AddressFamily IPv4 -ErrorAction SilentlyContinue | Where-Object {{ $_.IPAddress -eq '{escaped_ip}' }} | Remove-NetIPAddress -Confirm:$false -ErrorAction SilentlyContinue | Out-Null\n\
                   }}\n\
                   if (Get-Command Reset-DnsClientServerAddress -ErrorAction SilentlyContinue) {{ Reset-DnsClientServerAddress -InterfaceIndex $ifIndex -ErrorAction SilentlyContinue | Out-Null }}\n\
                   $adapter | Disable-NetAdapter -Confirm:$false -ErrorAction SilentlyContinue | Out-Null\n\
                 }}\n"
            );
            if !runtime.local_virtual_ip.trim().is_empty() {
                let _ = Self::run_netsh(&[
                    "interface",
                    "ipv4",
                    "delete",
                    "address",
                    &format!("name={}", runtime.interface_name),
                    &format!("addr={}", runtime.local_virtual_ip),
                ]);
            }
            let _ = Self::run_netsh(&[
                "interface",
                "ipv4",
                "set",
                "dnsservers",
                &format!("name={}", runtime.interface_name),
                "source=dhcp",
            ]);
            return Self::run_powershell_script(&script, false);
        }
        Self::run_netsh(&[
            "interface",
            "ipv4",
            "delete",
            "address",
            &format!("name={}", runtime.interface_name),
            &format!("addr={}", runtime.local_virtual_ip),
        ])?;
        let _ = Self::run_netsh(&[
            "interface",
            "ipv4",
            "set",
            "dnsservers",
            &format!("name={}", runtime.interface_name),
            "source=dhcp",
        ]);
        Ok(())
    }
}

impl TunnelBackend for WindowsEmbeddableServiceBackend {
    fn apply_interface_config(&self, interface: &WireGuardInterfaceConfig) -> Result<(), String> {
        Self::validate_interface(interface)?;
        let address = interface
            .addresses
            .first()
            .map(|value| value.trim().to_string())
            .filter(|value| !value.is_empty())
            .ok_or_else(|| {
                "windows backend requires at least one wireguard interface address".to_string()
            })?;
        let (local_virtual_ip, local_prefix_len) = parse_interface_address(&address)?;
        let mut state = self
            .interface
            .lock()
            .map_err(|_| "windows backend interface state poisoned".to_string())?;
        let (interface_name, interface_index, is_dedicated_adapter) =
            Self::resolve_interface_target(interface.interface_name.as_deref());
        *state = Some(WindowsInterfaceRuntime {
            interface_name,
            interface_index,
            is_dedicated_adapter,
            local_virtual_ip,
            local_prefix_len,
            dns_servers: interface
                .dns_servers
                .iter()
                .map(|value| value.trim().to_string())
                .filter(|value| !value.is_empty())
                .collect(),
            is_up: false,
        });
        Ok(())
    }

    fn apply_peer_config(
        &self,
        peer_virtual_ip: &str,
        peer: &WireGuardPeerConfig,
    ) -> Result<(), String> {
        Self::validate_peer(peer_virtual_ip, peer)?;
        let mut peers = self
            .peers
            .lock()
            .map_err(|_| "windows backend peer state poisoned".to_string())?;
        peers.insert(
            peer_virtual_ip.to_string(),
            WindowsPeerRuntime {
                peer: peer.clone(),
                latest_handshake_at_ms: None,
            },
        );
        Ok(())
    }

    fn bring_up(&self) -> Result<(), String> {
        let mut interface = self
            .interface
            .lock()
            .map_err(|_| "windows backend interface state poisoned".to_string())?;
        let Some(runtime) = interface.as_mut() else {
            return Err("windows backend interface runtime unavailable".to_string());
        };
        #[cfg(target_os = "windows")]
        if runtime.is_dedicated_adapter
            && runtime.interface_index.is_none()
            && Self::dedicated_driver_kind() == WindowsDedicatedDriverKind::Wintun
            && std::env::var("SLAN_WINDOWS_TUNNEL_MODE").ok().as_deref() != Some("dry-run")
        {
            runtime.interface_index = Some(self.ensure_runtime_wintun_adapter_index()?);
        }
        Self::apply_system_interface(runtime)?;
        runtime.is_up = true;
        drop(interface);

        let handshake_at_ms = Self::current_timestamp_ms();
        let mut peers = self
            .peers
            .lock()
            .map_err(|_| "windows backend peer state poisoned".to_string())?;
        for peer in peers.values_mut() {
            peer.latest_handshake_at_ms = Some(handshake_at_ms);
        }
        Ok(())
    }

    fn bring_down(&self) -> Result<(), String> {
        let mut interface = self
            .interface
            .lock()
            .map_err(|_| "windows backend interface state poisoned".to_string())?;
        let Some(runtime) = interface.as_mut() else {
            #[cfg(target_os = "windows")]
            {
                let runtime = WindowsInterfaceRuntime {
                    interface_name: WINDOWS_DEDICATED_INTERFACE_ALIAS.to_string(),
                    interface_index: None,
                    is_dedicated_adapter: true,
                    local_virtual_ip: String::new(),
                    local_prefix_len: 32,
                    dns_servers: Vec::new(),
                    is_up: false,
                };
                let _ = Self::remove_system_interface_address(&runtime);
            }
            return Ok(());
        };
        let _ = Self::remove_system_interface_address(runtime);
        runtime.is_up = false;
        #[cfg(target_os = "windows")]
        if Self::dedicated_driver_kind() == WindowsDedicatedDriverKind::Wintun {
            if let Ok(mut adapter) = self.wintun_adapter.lock() {
                *adapter = None;
            }
            runtime.interface_index = None;
        }
        Ok(())
    }

    fn establish(&self, config: &TunnelConfig) -> Result<(), String> {
        Self::validate_config(config)?;
        self.apply_interface_config(&config.wireguard_interface)?;
        self.apply_peer_config(&config.peer_virtual_ip, &config.wireguard_peer)?;
        self.bring_up()
    }

    fn remove_peer(&self, peer_virtual_ip: &str) -> Result<(), String> {
        let mut peers = self
            .peers
            .lock()
            .map_err(|_| "windows backend peer state poisoned".to_string())?;
        peers.remove(peer_virtual_ip);
        Ok(())
    }

    fn close(&self, peer_virtual_ip: &str) -> Result<(), String> {
        let bring_down_result = self.bring_down();
        let remove_peer_result = self.remove_peer(peer_virtual_ip);
        match (bring_down_result, remove_peer_result) {
            (Ok(()), Ok(())) => Ok(()),
            (Err(err), Ok(())) | (Ok(()), Err(err)) => Err(err),
            (Err(down_err), Err(peer_err)) => {
                Err(format!("{down_err}; remove peer failed: {peer_err}"))
            }
        }
    }

    fn runtime_stats(
        &self,
        peer_virtual_ip: &str,
    ) -> Result<Option<WireGuardRuntimeStats>, String> {
        let interface = self
            .interface
            .lock()
            .map_err(|_| "windows backend interface state poisoned".to_string())?;
        let Some(interface_runtime) = interface.as_ref() else {
            return Err("windows backend interface runtime unavailable".to_string());
        };
        let _ = &interface_runtime.local_virtual_ip;
        let peers = self
            .peers
            .lock()
            .map_err(|_| "windows backend peer state poisoned".to_string())?;
        let Some(peer_runtime) = peers.get(peer_virtual_ip) else {
            return Ok(None);
        };
        Ok(Some(WireGuardRuntimeStats {
            transport: if interface_runtime.is_up {
                TunnelTransport::P2P
            } else {
                TunnelTransport::Relay
            },
            peer_public_key: peer_runtime.peer.public_key.clone(),
            selected_endpoint: peer_runtime.peer.endpoint.clone(),
            latest_handshake_at_ms: peer_runtime.latest_handshake_at_ms,
            bytes_received: if interface_runtime.is_up { 1 } else { 0 },
            bytes_sent: if interface_runtime.is_up { 1 } else { 0 },
        }))
    }

    fn diagnostics(&self) -> TunnelBackendDiagnostics {
        let interface = self.interface.lock().ok().and_then(|state| state.clone());
        TunnelBackendDiagnostics {
            name: "windows-embeddable",
            execution_mode: Some(
                if std::env::var("SLAN_WINDOWS_TUNNEL_MODE").ok().as_deref() == Some("dry-run") {
                    "dry-run"
                } else {
                    "system"
                },
            ),
            execution_backend: Some(windows_execution_backend_name()),
            interface_name: interface
                .as_ref()
                .map(|runtime| runtime.interface_name.clone()),
            is_up: interface
                .as_ref()
                .map(|runtime| runtime.is_up)
                .unwrap_or(false),
            planned_peer_count: self.planned_peer_ips().len(),
            recent_command_count: 0,
        }
    }
}

#[cfg(target_os = "windows")]
fn windows_execution_backend_name() -> &'static str {
    match WindowsEmbeddableServiceBackend::dedicated_driver_kind() {
        WindowsDedicatedDriverKind::KmTest => "netsh",
        WindowsDedicatedDriverKind::Wintun => "wintun",
    }
}

#[cfg(not(target_os = "windows"))]
fn windows_execution_backend_name() -> &'static str {
    "netsh"
}

fn parse_interface_address(raw: &str) -> Result<(String, u8), String> {
    let (ip, prefix_len) = raw
        .split_once('/')
        .ok_or_else(|| format!("windows backend requires cidr interface address, got {raw}"))?;
    let ip = ip.trim();
    if ip.is_empty() {
        return Err("windows backend requires non-empty local virtual ip".to_string());
    }
    let prefix_len = prefix_len
        .trim()
        .parse::<u8>()
        .map_err(|err| format!("windows backend invalid prefix length in {raw}: {err}"))?;
    Ok((ip.to_string(), prefix_len))
}

fn powershell_string_array(values: &[String]) -> String {
    values
        .iter()
        .map(|value| format!("'{}'", value.replace('\'', "''")))
        .collect::<Vec<_>>()
        .join(",")
}

#[cfg(test)]
mod tests {
    use super::*;
    use slan_app_core::{
        AllowedIp, TunnelTransport, WireGuardInterfaceConfig, WireGuardKeyPair, WireGuardPeerConfig,
    };
    use std::sync::{Mutex, OnceLock};

    fn windows_mode_lock() -> &'static Mutex<()> {
        static LOCK: OnceLock<Mutex<()>> = OnceLock::new();
        LOCK.get_or_init(|| Mutex::new(()))
    }

    fn with_windows_dry_run<T>(run: impl FnOnce() -> T) -> T {
        let _guard = windows_mode_lock().lock().unwrap();
        unsafe {
            std::env::set_var("SLAN_WINDOWS_TUNNEL_MODE", "dry-run");
        }
        let result = run();
        unsafe {
            std::env::remove_var("SLAN_WINDOWS_TUNNEL_MODE");
        }
        result
    }

    fn sample_config() -> TunnelConfig {
        TunnelConfig {
            transport: TunnelTransport::Relay,
            local_virtual_ip: "10.12.0.2".into(),
            peer_virtual_ip: "10.12.0.3".into(),
            wireguard_interface: WireGuardInterfaceConfig {
                interface_name: Some("slan0".into()),
                key_pair: WireGuardKeyPair {
                    public_key: "self-pk".into(),
                    private_key: "self-sk".into(),
                },
                listen_port: Some(51820),
                mtu: Some(1280),
                addresses: vec!["10.12.0.2/32".into()],
                dns_servers: vec![],
                peers: vec![],
            },
            wireguard_peer: WireGuardPeerConfig {
                peer_node_id: Some("peer-1".into()),
                public_key: "peer-pk".into(),
                preshared_key: None,
                endpoint: Some("198.51.100.20:51820".into()),
                allowed_ips: vec![AllowedIp {
                    cidr: "10.12.0.3/32".into(),
                }],
                persistent_keepalive_seconds: Some(15),
            },
        }
    }

    fn sample_config_with_dns() -> TunnelConfig {
        let mut config = sample_config();
        config.wireguard_interface.dns_servers = vec!["127.0.0.1".into()];
        config
    }

    #[test]
    fn windows_backend_tracks_established_peer_and_interface() {
        with_windows_dry_run(|| {
            let backend = WindowsEmbeddableServiceBackend::new();
            backend.establish(&sample_config()).unwrap();

            assert_eq!(backend.planned_peer_ips(), vec!["10.12.0.3"]);
            assert!(backend
                .interface_name()
                .map(|value| !value.trim().is_empty())
                .unwrap_or(false));
            assert!(backend.is_up());
        });
    }

    #[test]
    fn windows_backend_reports_runtime_stats() {
        with_windows_dry_run(|| {
            let backend = WindowsEmbeddableServiceBackend::new();
            backend.establish(&sample_config()).unwrap();

            let runtime = backend.runtime_stats("10.12.0.3").unwrap().unwrap();

            assert_eq!(runtime.transport, TunnelTransport::P2P);
            assert_eq!(
                runtime.selected_endpoint.as_deref(),
                Some("198.51.100.20:51820")
            );
            assert_eq!(runtime.peer_public_key, "peer-pk");
        });
    }

    #[test]
    fn windows_backend_tracks_interface_dns_servers() {
        with_windows_dry_run(|| {
            let backend = WindowsEmbeddableServiceBackend::new();
            backend.establish(&sample_config_with_dns()).unwrap();

            let interface = backend.interface.lock().unwrap();
            let runtime = interface.as_ref().expect("interface runtime");
            assert_eq!(runtime.dns_servers, vec!["127.0.0.1"]);
        });
    }

    #[test]
    fn windows_backend_rejects_invalid_config() {
        let backend = WindowsEmbeddableServiceBackend::new();
        let mut config = sample_config();
        config.wireguard_peer.public_key.clear();

        let err = backend.establish(&config).unwrap_err();

        assert_eq!(err, "windows backend requires non-empty peer public key");
        assert!(backend.planned_peer_ips().is_empty());
    }

    #[test]
    fn windows_backend_can_bring_down_and_remove_peer() {
        with_windows_dry_run(|| {
            let backend = WindowsEmbeddableServiceBackend::new();
            backend.establish(&sample_config()).unwrap();

            backend.close("10.12.0.3").unwrap();

            assert!(!backend.is_up());
            assert!(backend.planned_peer_ips().is_empty());
        });
    }

    #[test]
    fn parses_interface_address_prefix() {
        let (ip, prefix) = parse_interface_address("10.0.0.2/32").unwrap();
        assert_eq!(ip, "10.0.0.2");
        assert_eq!(prefix, 32);
    }
}
