use std::collections::HashMap;
#[cfg(target_os = "windows")]
use std::process::Command;
use std::sync::Mutex;
use std::time::{SystemTime, UNIX_EPOCH};

use slan_app_core::{
    TunnelTransport, WireGuardInterfaceConfig, WireGuardPeerConfig, WireGuardRuntimeStats,
};

use crate::{TunnelBackend, TunnelConfig};

#[derive(Debug, Clone)]
struct WindowsInterfaceRuntime {
    interface_name: String,
    interface_index: Option<u32>,
    is_dedicated_adapter: bool,
    local_virtual_ip: String,
    local_prefix_len: u8,
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
/// - still does not create a dedicated Wintun / WireGuardNT adapter
///
/// The target interface defaults to `Loopback Pseudo-Interface 1` and can be
/// overridden with `SLAN_WINDOWS_TUNNEL_INTERFACE_ALIAS`.
#[derive(Default)]
pub struct WindowsEmbeddableServiceBackend {
    interface: Mutex<Option<WindowsInterfaceRuntime>>,
    peers: Mutex<HashMap<String, WindowsPeerRuntime>>,
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

        if let Some(requested_alias) = requested
            .map(str::trim)
            .filter(|value| !value.is_empty() && *value != "slan0")
        {
            return (requested_alias.to_string(), None, false);
        }

        #[cfg(target_os = "windows")]
        {
            if let Ok(route_output) = Self::command_stdout("route", &["print"]) {
                let kmtest_index = route_output.lines().find_map(|line| {
                    if !line.contains("Microsoft KM-TEST") {
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
                                    return (alias, Some(interface_index), true);
                                }
                            }
                        }
                    }
                    return (
                        "Microsoft KM-TEST Loopback Adapter".to_string(),
                        Some(interface_index),
                        true,
                    );
                }
            }
        }

        ("Loopback Pseudo-Interface 1".to_string(), None, false)
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
        if stderr.contains("requires elevation")
            || stdout.contains("requires elevation")
            || stderr.contains("Run as administrator")
            || stdout.contains("Run as administrator")
        {
            return Self::run_netsh_elevated(args);
        }
        let detail = if !stderr.is_empty() {
            stderr
        } else if !stdout.is_empty() {
            stdout
        } else {
            format!("exit status {}", output.status)
        };
        Err(format!("netsh {} failed: {}", args.join(" "), detail))
    }

    #[cfg(target_os = "windows")]
    fn run_netsh_elevated(args: &[&str]) -> Result<(), String> {
        let argument_list = args
            .iter()
            .map(|value| format!("'{}'", value.replace('\'', "''")))
            .collect::<Vec<_>>()
            .join(",");
        let command = format!(
            "$p = Start-Process -FilePath 'netsh.exe' -ArgumentList @({argument_list}) -Verb RunAs -WindowStyle Hidden -Wait -PassThru; exit $p.ExitCode"
        );
        let output = Command::new("powershell.exe")
            .args([
                "-NoProfile",
                "-NonInteractive",
                "-ExecutionPolicy",
                "Bypass",
                "-Command",
                &command,
            ])
            .output()
            .map_err(|err| format!("failed to launch elevated netsh: {err}"))?;
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
            "UAC prompt was rejected or the elevated netsh command failed".to_string()
        };
        Err(format!(
            "netsh {} failed after elevation attempt: {}",
            args.join(" "),
            detail
        ))
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
        if elevated {
            return Self::run_powershell_script_elevated(script);
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
        if stderr.contains("requires elevation")
            || stdout.contains("requires elevation")
            || stderr.contains("Access is denied")
            || stdout.contains("Access is denied")
        {
            return Self::run_powershell_script_elevated(script);
        }
        let detail = if !stderr.is_empty() {
            stderr
        } else if !stdout.is_empty() {
            stdout
        } else {
            format!("exit status {}", output.status)
        };
        Err(format!("powershell script failed: {detail}"))
    }

    #[cfg(target_os = "windows")]
    fn run_powershell_script_elevated(script: &str) -> Result<(), String> {
        let escaped = script.replace('\'', "''").replace('\n', "; ");
        let command = format!(
            "$p = Start-Process -FilePath 'powershell.exe' -ArgumentList @('-NoProfile','-ExecutionPolicy','Bypass','-Command','{escaped}') -Verb RunAs -WindowStyle Hidden -Wait -PassThru; exit $p.ExitCode"
        );
        let output = Command::new("powershell.exe")
            .args([
                "-NoProfile",
                "-NonInteractive",
                "-ExecutionPolicy",
                "Bypass",
                "-Command",
                &command,
            ])
            .output()
            .map_err(|err| format!("failed to launch elevated powershell: {err}"))?;
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
            "UAC prompt was rejected or the elevated PowerShell command failed".to_string()
        };
        Err(format!("elevated powershell script failed: {detail}"))
    }

    #[cfg(not(target_os = "windows"))]
    fn run_powershell_script(_script: &str, _elevated: bool) -> Result<(), String> {
        Ok(())
    }

    fn apply_system_interface(runtime: &WindowsInterfaceRuntime) -> Result<(), String> {
        #[cfg(target_os = "windows")]
        if runtime.is_dedicated_adapter {
            let interface_index = runtime.interface_index.ok_or_else(|| {
                "windows backend dedicated adapter is missing an interface index".to_string()
            })?;
            let script = format!(
                "$ErrorActionPreference='Stop'\n\
                 Enable-NetAdapter -InterfaceIndex {interface_index} -Confirm:$false -ErrorAction SilentlyContinue | Out-Null\n\
                 Set-NetIPInterface -InterfaceIndex {interface_index} -Dhcp Disabled -ErrorAction SilentlyContinue | Out-Null\n\
                 Get-NetIPAddress -InterfaceIndex {interface_index} -AddressFamily IPv4 -ErrorAction SilentlyContinue | Remove-NetIPAddress -Confirm:$false -ErrorAction SilentlyContinue\n\
                 New-NetIPAddress -InterfaceIndex {interface_index} -IPAddress {} -PrefixLength {} -AddressFamily IPv4 -Type Unicast | Out-Null\n",
                runtime.local_virtual_ip, runtime.local_prefix_len
            );
            return Self::run_powershell_script(&script, true);
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
        ])
    }

    fn remove_system_interface_address(runtime: &WindowsInterfaceRuntime) -> Result<(), String> {
        #[cfg(target_os = "windows")]
        if runtime.is_dedicated_adapter {
            let interface_index = runtime.interface_index.ok_or_else(|| {
                "windows backend dedicated adapter is missing an interface index".to_string()
            })?;
            let script = format!(
                "$ErrorActionPreference='Stop'\n\
                 Get-NetIPAddress -InterfaceIndex {interface_index} -AddressFamily IPv4 -ErrorAction SilentlyContinue | Remove-NetIPAddress -Confirm:$false -ErrorAction SilentlyContinue\n"
            );
            return Self::run_powershell_script(&script, true);
        }
        Self::run_netsh(&[
            "interface",
            "ipv4",
            "delete",
            "address",
            &format!("name={}", runtime.interface_name),
            &format!("addr={}", runtime.local_virtual_ip),
        ])
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
            return Err("windows backend interface runtime unavailable".to_string());
        };
        let _ = Self::remove_system_interface_address(runtime);
        runtime.is_up = false;
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

            backend.bring_down().unwrap();
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
