use std::{
    ffi::{c_void, OsStr},
    fs,
    os::windows::{ffi::OsStrExt, process::CommandExt},
    path::{Path, PathBuf},
    process::Command,
    sync::{Mutex, OnceLock},
    thread,
    time::Duration,
};

use anyhow::{bail, Context, Result};
use client_core::{NetworkRuntimeState, PlatformNetwork, RouteSpec};
use libloading::Library;

const DEFAULT_INTERFACE_NAME: &str = "SLAN LAN Adapter";
const WINDOWS_WINTUN_DRIVER_TYPE: &str = "Wintun";
const CREATE_NO_WINDOW: u32 = 0x08000000;

type WintunAdapterHandle = *mut c_void;
type WintunCreateAdapterFunc =
    unsafe extern "system" fn(*const u16, *const u16, *const RawGuid) -> WintunAdapterHandle;
type WintunOpenAdapterFunc = unsafe extern "system" fn(*const u16) -> WintunAdapterHandle;
type WintunCloseAdapterFunc = unsafe extern "system" fn(WintunAdapterHandle);
type WintunSessionHandle = *mut c_void;
type WintunStartSessionFunc =
    unsafe extern "system" fn(WintunAdapterHandle, u32) -> WintunSessionHandle;
type WintunEndSessionFunc = unsafe extern "system" fn(WintunSessionHandle);

#[repr(C)]
struct RawGuid {
    data1: u32,
    data2: u16,
    data3: u16,
    data4: [u8; 8],
}

struct WintunRuntime {
    _library: Library,
    handle: WintunAdapterHandle,
    session: WintunSessionHandle,
    end_session: WintunEndSessionFunc,
    close_adapter: WintunCloseAdapterFunc,
}

static WINTUN_RUNTIME: OnceLock<Mutex<Option<WintunRuntime>>> = OnceLock::new();

unsafe impl Send for WintunRuntime {}

impl Drop for WintunRuntime {
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

#[derive(Debug, Clone, Default)]
pub struct WindowsPlatformNetwork;

impl PlatformNetwork for WindowsPlatformNetwork {
    fn install_adapter(&self) -> Result<()> {
        ensure_adapter_present(DEFAULT_INTERFACE_NAME)?;
        persist_state(&NetworkRuntimeState {
            adapter_present: true,
            ..load_cached_runtime_state().unwrap_or_default()
        })
    }

    fn configure_ip(&self, virtual_ip: &str, prefix_len: u8) -> Result<()> {
        ensure_installed_adapter_ready(DEFAULT_INTERFACE_NAME)
            .context("ensure installed Wintun adapter before IP")?;
        configure_adapter_ip(DEFAULT_INTERFACE_NAME, virtual_ip, prefix_len)
            .context("configure Wintun adapter IP")?;
        verify_adapter_ip(DEFAULT_INTERFACE_NAME, virtual_ip)
            .context("verify Wintun adapter IP")?;
        let mut state = load_cached_runtime_state().unwrap_or_default();
        state.adapter_present = true;
        state.network_enabled = true;
        state.virtual_ip = Some(virtual_ip.to_string());
        persist_state(&state)
    }

    fn configure_routes(&self, routes: &[RouteSpec]) -> Result<()> {
        configure_routes(DEFAULT_INTERFACE_NAME, routes).context("configure Wintun routes")?;
        persist_state(&load_cached_runtime_state().unwrap_or_default())
    }

    fn configure_dns(&self, dns_servers: &[String]) -> Result<()> {
        configure_dns(DEFAULT_INTERFACE_NAME, dns_servers).context("configure Wintun DNS")?;
        persist_state(&load_cached_runtime_state().unwrap_or_default())
    }

    fn disable_network(&self) -> Result<()> {
        disable_adapter(DEFAULT_INTERFACE_NAME)?;
        let mut state = load_cached_runtime_state().unwrap_or_default();
        state.network_enabled = false;
        state.virtual_ip = None;
        persist_state(&state)
    }

    fn read_runtime_state(&self) -> Result<NetworkRuntimeState> {
        match read_windows_runtime_state(DEFAULT_INTERFACE_NAME) {
            Ok(state) => {
                persist_state(&state)?;
                Ok(state)
            }
            Err(_) => load_cached_runtime_state(),
        }
    }
}

fn state_file_path() -> PathBuf {
    app_data_dir()
        .join("SLAN")
        .join("client-v2-network-state.json")
}

fn app_data_dir() -> PathBuf {
    std::env::var_os("ProgramData")
        .map(PathBuf::from)
        .unwrap_or_else(|| PathBuf::from(r"C:\ProgramData"))
}

fn load_cached_runtime_state() -> Result<NetworkRuntimeState> {
    let path = state_file_path();
    if !path.exists() {
        return Ok(NetworkRuntimeState::default());
    }
    let payload = fs::read(&path).with_context(|| format!("read {}", path.display()))?;
    serde_json::from_slice(&payload).with_context(|| format!("decode {}", path.display()))
}

fn persist_state(state: &NetworkRuntimeState) -> Result<()> {
    let path = state_file_path();
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent).with_context(|| format!("create {}", parent.display()))?;
    }
    let payload = serde_json::to_vec_pretty(state).context("encode network state")?;
    fs::write(&path, payload).with_context(|| format!("write {}", path.display()))
}

fn ensure_adapter_present(interface_name: &str) -> Result<()> {
    ensure_adapter_created(interface_name)?;
    ensure_installed_adapter_ready(interface_name)
}

fn ensure_installed_adapter_ready(interface_name: &str) -> Result<()> {
    let script = format!(
        "$name = '{}'; \
         $adapter = Get-NetAdapter -IncludeHidden -Name $name -ErrorAction SilentlyContinue; \
         if (-not $adapter) {{ throw \"SLAN local network adapter '$name' was not found. Please reinstall or repair SLAN Client.\" }}; \
         Enable-NetAdapter -Name $name -Confirm:$false -ErrorAction Stop | Out-Null; \
         Write-Output $adapter.Name",
        escape_powershell_single_quoted(interface_name),
    );
    run_powershell(&script).map(|_| ())
}

fn ensure_adapter_created(interface_name: &str) -> Result<()> {
    let runtime = WINTUN_RUNTIME.get_or_init(|| Mutex::new(None));
    let mut runtime = runtime.lock().expect("wintun runtime mutex poisoned");
    if runtime.is_some() {
        rename_wintun_adapter(interface_name)?;
        return Ok(());
    }

    let library = load_wintun_library()?;
    let create_adapter = unsafe {
        let symbol: libloading::Symbol<WintunCreateAdapterFunc> = library
            .get(b"WintunCreateAdapter\0")
            .map_err(|err| anyhow::anyhow!("failed to resolve WintunCreateAdapter: {err}"))?;
        *symbol
    };
    let open_adapter = unsafe {
        let symbol: libloading::Symbol<WintunOpenAdapterFunc> = library
            .get(b"WintunOpenAdapter\0")
            .map_err(|err| anyhow::anyhow!("failed to resolve WintunOpenAdapter: {err}"))?;
        *symbol
    };
    let close_adapter = unsafe {
        let symbol: libloading::Symbol<WintunCloseAdapterFunc> = library
            .get(b"WintunCloseAdapter\0")
            .map_err(|err| anyhow::anyhow!("failed to resolve WintunCloseAdapter: {err}"))?;
        *symbol
    };
    let start_session = unsafe {
        let symbol: libloading::Symbol<WintunStartSessionFunc> = library
            .get(b"WintunStartSession\0")
            .map_err(|err| anyhow::anyhow!("failed to resolve WintunStartSession: {err}"))?;
        *symbol
    };
    let end_session = unsafe {
        let symbol: libloading::Symbol<WintunEndSessionFunc> =
            library
                .get(b"WintunEndSession\0")
                .map_err(|err| anyhow::anyhow!("failed to resolve WintunEndSession: {err}"))?;
        *symbol
    };

    let existing_name = find_wintun_adapter();
    let adapter_name = wide_null(existing_name.as_deref().unwrap_or(interface_name));
    let tunnel_type = wide_null(WINDOWS_WINTUN_DRIVER_TYPE);
    let mut handle = unsafe { open_adapter(adapter_name.as_ptr()) };
    if handle.is_null() {
        let adapter_name = wide_null(interface_name);
        handle = unsafe {
            create_adapter(
                adapter_name.as_ptr(),
                tunnel_type.as_ptr(),
                std::ptr::null(),
            )
        };
    }
    if handle.is_null() {
        bail!(
            "failed to create or open Wintun adapter '{}': {}",
            interface_name,
            std::io::Error::last_os_error()
        );
    }
    let session = unsafe { start_session(handle, 0x400000) };
    if session.is_null() {
        unsafe {
            close_adapter(handle);
        }
        bail!(
            "failed to start Wintun session for '{}': {}",
            interface_name,
            std::io::Error::last_os_error()
        );
    }
    wait_for_wintun_adapter().ok_or_else(|| {
        anyhow::anyhow!(
            "Wintun adapter '{}' was created but not visible",
            interface_name
        )
    })?;
    rename_wintun_adapter(interface_name)?;
    *runtime = Some(WintunRuntime {
        _library: library,
        handle,
        session,
        end_session,
        close_adapter,
    });
    Ok(())
}

fn wide_null(value: &str) -> Vec<u16> {
    OsStr::new(value).encode_wide().chain([0]).collect()
}

fn find_wintun_adapter() -> Option<String> {
    let escaped_alias = escape_powershell_single_quoted(DEFAULT_INTERFACE_NAME);
    let script = format!(
        "$adapter = Get-NetAdapter -IncludeHidden -ErrorAction SilentlyContinue | Where-Object {{ \
         $_.Name -eq '{escaped_alias}' -or \
         $_.InterfaceDescription -like '*Wintun*' -or \
         $_.InterfaceDescription -like '*WireGuard*Tunnel*' -or \
         $_.InterfaceDescription -like '*WireGuardNT*' \
         }} | Select-Object -First 1 Name; \
         if ($adapter) {{ Write-Output $adapter.Name }}"
    );
    run_powershell(&script).ok().and_then(|output| {
        output
            .lines()
            .map(str::trim)
            .find(|line| !line.is_empty())
            .map(str::to_string)
    })
}

fn wait_for_wintun_adapter() -> Option<String> {
    for _ in 0..20 {
        if let Some(adapter) = find_wintun_adapter() {
            return Some(adapter);
        }
        thread::sleep(Duration::from_millis(250));
    }
    None
}

fn rename_wintun_adapter(interface_name: &str) -> Result<()> {
    let escaped_alias = escape_powershell_single_quoted(interface_name);
    let script = format!(
        "$ErrorActionPreference='Stop'; \
         $adapter = Get-NetAdapter -IncludeHidden -ErrorAction SilentlyContinue | Where-Object {{ \
           $_.Name -eq '{escaped_alias}' -or \
           $_.InterfaceDescription -like '*Wintun*' -or \
           $_.InterfaceDescription -like '*WireGuard*Tunnel*' -or \
           $_.InterfaceDescription -like '*WireGuardNT*' \
         }} | Select-Object -First 1; \
         if (-not $adapter) {{ throw \"SLAN Wintun adapter was not found.\" }}; \
         if ($adapter.Name -ne '{escaped_alias}') {{ Rename-NetAdapter -Name $adapter.Name -NewName '{escaped_alias}' -Confirm:$false -ErrorAction Stop | Out-Null }}; \
         Write-Output '{escaped_alias}'"
    );
    run_powershell(&script).map(|_| ())
}

fn wintun_dll_candidates() -> Vec<PathBuf> {
    let mut candidates = Vec::new();
    if let Ok(current_exe) = std::env::current_exe() {
        if let Some(parent) = current_exe.parent() {
            candidates.push(parent.join("wintun.dll"));
        }
    }
    candidates.push(
        Path::new(env!("CARGO_MANIFEST_DIR")).join("../../../drivers/wintun/bin/amd64/wintun.dll"),
    );
    candidates
}

fn load_wintun_library() -> Result<Library> {
    let mut attempted = Vec::new();
    for candidate in wintun_dll_candidates() {
        attempted.push(candidate.display().to_string());
        if candidate.exists() {
            return unsafe { Library::new(&candidate) }
                .with_context(|| format!("load wintun.dll from {}", candidate.display()));
        }
    }
    bail!("wintun.dll not found; searched: {}", attempted.join(", "))
}

fn configure_adapter_ip(interface_name: &str, virtual_ip: &str, prefix_len: u8) -> Result<()> {
    let virtual_ip = virtual_ip.trim();
    if virtual_ip.is_empty() || virtual_ip.eq_ignore_ascii_case("pending") {
        bail!("device unavailable: missing assigned virtual IP");
    }
    let script = format!(
        "$name = '{}'; \
         $ip = '{}'; \
         $prefix = {}; \
         $adapter = Get-NetAdapter -IncludeHidden -Name $name -ErrorAction SilentlyContinue; \
         if (-not $adapter) {{ throw \"SLAN local network adapter '$name' was not found. Please reinstall or repair SLAN Client.\" }}; \
         Enable-NetAdapter -Name $name -Confirm:$false -ErrorAction Stop | Out-Null; \
         Start-Sleep -Milliseconds 800; \
         Set-NetIPInterface -InterfaceAlias $name -AddressFamily IPv4 -Dhcp Disabled -ErrorAction SilentlyContinue | Out-Null; \
         Get-NetIPAddress -InterfaceAlias $name -AddressFamily IPv4 -ErrorAction SilentlyContinue | Remove-NetIPAddress -Confirm:$false -ErrorAction SilentlyContinue; \
         New-NetIPAddress -InterfaceAlias $name -IPAddress $ip -PrefixLength $prefix -ErrorAction Stop | Out-Null; \
         $actual = Get-NetIPAddress -InterfaceAlias $name -AddressFamily IPv4 -ErrorAction Stop | Where-Object {{ $_.IPAddress -eq $ip }} | Select-Object -First 1 -ExpandProperty IPAddress; \
         if ($actual -ne $ip) {{ throw \"SLAN local network adapter '$name' did not apply IP '$ip'.\" }}; \
         Write-Output ($name + '|' + $ip)",
        escape_powershell_single_quoted(interface_name),
        escape_powershell_single_quoted(virtual_ip),
        prefix_len,
    );
    run_powershell(&script).map(|_| ())
}

fn verify_adapter_ip(interface_name: &str, virtual_ip: &str) -> Result<()> {
    let script = format!(
        "$name = '{}'; \
         $ip = '{}'; \
         $adapter = Get-NetAdapter -IncludeHidden -Name $name -ErrorAction Stop; \
         $actual = Get-NetIPAddress -InterfaceAlias $name -AddressFamily IPv4 -ErrorAction SilentlyContinue | Where-Object {{ $_.IPAddress -eq $ip }} | Select-Object -First 1 -ExpandProperty IPAddress; \
         if ($adapter.AdminStatus -ne 'Up') {{ throw \"SLAN local network adapter '$name' is disabled; adminStatus=\" + $adapter.AdminStatus }}; \
         if ($actual -ne $ip) {{ throw \"SLAN local network adapter '$name' expected IP '$ip' but it was not applied.\" }}; \
         Write-Output ([string]$adapter.AdminStatus + '|' + $actual)",
        escape_powershell_single_quoted(interface_name),
        escape_powershell_single_quoted(virtual_ip),
    );
    run_powershell(&script).map(|_| ())
}

fn configure_routes(interface_name: &str, routes: &[RouteSpec]) -> Result<()> {
    if routes.is_empty() {
        return Ok(());
    }
    let mut script = format!(
        "$name = '{}'; \
         $adapter = Get-NetAdapter -IncludeHidden -Name $name -ErrorAction SilentlyContinue; \
         if (-not $adapter) {{ throw \"SLAN local network adapter '$name' was not found. Please reinstall or repair SLAN Client.\" }}; \
         $ifIndex = $adapter.ifIndex; ",
        escape_powershell_single_quoted(interface_name),
    );
    for route in routes {
        let destination = route.destination.trim();
        if destination.is_empty() || destination.eq_ignore_ascii_case("mesh") {
            continue;
        }
        let gateway = route.gateway.as_deref().unwrap_or("0.0.0.0").trim();
        script.push_str(&format!(
            "New-NetRoute -DestinationPrefix '{}' -InterfaceIndex $ifIndex -NextHop '{}' -PolicyStore ActiveStore -ErrorAction SilentlyContinue | Out-Null; ",
            escape_powershell_single_quoted(destination),
            escape_powershell_single_quoted(gateway),
        ));
    }
    run_powershell(&script).map(|_| ())
}

fn configure_dns(interface_name: &str, dns_servers: &[String]) -> Result<()> {
    let servers = dns_servers
        .iter()
        .map(|value| value.trim())
        .filter(|value| !value.is_empty())
        .map(|value| format!("'{}'", escape_powershell_single_quoted(value)))
        .collect::<Vec<_>>();
    let script = if servers.is_empty() {
        format!(
            "Set-DnsClientServerAddress -InterfaceAlias '{}' -ResetServerAddresses -ErrorAction SilentlyContinue",
            escape_powershell_single_quoted(interface_name),
        )
    } else {
        format!(
            "Set-DnsClientServerAddress -InterfaceAlias '{}' -ServerAddresses @({}) -ErrorAction Stop",
            escape_powershell_single_quoted(interface_name),
            servers.join(","),
        )
    };
    run_powershell(&script).map(|_| ())
}

fn disable_adapter(interface_name: &str) -> Result<()> {
    let script = format!(
        "$name = '{}'; \
         $adapter = Get-NetAdapter -IncludeHidden -Name $name -ErrorAction SilentlyContinue; \
         if ($adapter) {{ \
           Set-NetIPInterface -InterfaceAlias $name -AddressFamily IPv4 -Dhcp Disabled -ErrorAction SilentlyContinue | Out-Null; \
           Get-NetIPAddress -InterfaceAlias $name -AddressFamily IPv4 -ErrorAction SilentlyContinue | Remove-NetIPAddress -Confirm:$false -ErrorAction SilentlyContinue; \
           Set-DnsClientServerAddress -InterfaceAlias $name -ResetServerAddresses -ErrorAction SilentlyContinue | Out-Null; \
           Disable-NetAdapter -Name $name -Confirm:$false -ErrorAction SilentlyContinue | Out-Null; \
         }}",
        escape_powershell_single_quoted(interface_name),
    );
    run_powershell(&script).map(|_| ())
}

fn read_windows_runtime_state(interface_name: &str) -> Result<NetworkRuntimeState> {
    let script = format!(
        "$name = '{}'; \
         $adapter = Get-NetAdapter -IncludeHidden -Name $name -ErrorAction SilentlyContinue; \
         if (-not $adapter) {{ Write-Output 'missing||'; exit 0 }}; \
         $ip = Get-NetIPAddress -InterfaceAlias $name -AddressFamily IPv4 -ErrorAction SilentlyContinue | \
           Where-Object {{ $_.IPAddress -and $_.IPAddress -ne '0.0.0.0' -and $_.IPAddress -notlike '169.254.*' }} | \
           Select-Object -First 1 -ExpandProperty IPAddress; \
         Write-Output ([string]$adapter.AdminStatus + '|' + $adapter.Name + '|' + $ip)",
        escape_powershell_single_quoted(interface_name),
    );
    let output = run_powershell(&script)?;
    let mut parts = output.split('|');
    let status = parts.next().unwrap_or_default().trim();
    let name = parts.next().unwrap_or_default().trim();
    let ip = parts.next().unwrap_or_default().trim();
    if status.eq_ignore_ascii_case("missing") || name.is_empty() {
        return Ok(NetworkRuntimeState::default());
    }
    let network_enabled = status.eq_ignore_ascii_case("up") && is_usable_virtual_ip(ip);
    Ok(NetworkRuntimeState {
        adapter_present: true,
        network_enabled,
        virtual_ip: if network_enabled {
            Some(ip.to_string())
        } else {
            None
        },
    })
}

fn is_usable_virtual_ip(ip: &str) -> bool {
    let ip = ip.trim();
    !ip.is_empty() && ip != "0.0.0.0" && !ip.starts_with("169.254.")
}

fn run_powershell(script: &str) -> Result<String> {
    let output = Command::new("powershell.exe")
        .args([
            "-NoProfile",
            "-NonInteractive",
            "-ExecutionPolicy",
            "Bypass",
            "-Command",
            script,
        ])
        .creation_flags(CREATE_NO_WINDOW)
        .output()
        .context("run powershell network command")?;
    if output.status.success() {
        return Ok(String::from_utf8_lossy(&output.stdout).trim().to_string());
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
    bail!("windows network command failed: {detail}");
}

fn escape_powershell_single_quoted(value: &str) -> String {
    value.replace('\'', "''")
}

#[cfg(test)]
mod tests {
    use super::is_usable_virtual_ip;

    #[test]
    fn rejects_windows_link_local_autoconfig_ip() {
        assert!(!is_usable_virtual_ip(""));
        assert!(!is_usable_virtual_ip("0.0.0.0"));
        assert!(!is_usable_virtual_ip("169.254.92.148"));
        assert!(is_usable_virtual_ip("100.64.0.10"));
    }
}
