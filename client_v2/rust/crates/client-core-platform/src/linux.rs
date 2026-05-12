//! Linux platform bridge for SLAN mesh networking.
//!
//! Linux uses `client-core-service` for control/session/MQTT/path decisions and
//! keeps OS privileges behind this platform layer. The default backend manages a
//! persistent `slan0` TUN interface through `iproute2` and DNS through
//! `systemd-resolved` when available.

use std::{
    net::Ipv4Addr,
    process::Command,
    sync::{Mutex, OnceLock},
};

use anyhow::{anyhow, bail, Context, Result};
use client_core::{
    NetworkRuntimeState, PeerPathRuntime, PlatformDiagnosticCheck, PlatformNetwork,
    PlatformNetworkDiagnostics, RelayDataPlaneConfig, RouteSpec,
};

const DEFAULT_INTERFACE_NAME: &str = "slan0";
const DEFAULT_MTU: u32 = 1280;

#[derive(Debug, Clone, Default)]
pub struct LinuxPlatformNetwork;

#[derive(Debug, Clone, Default)]
struct LinuxRuntime {
    interface_name: String,
    virtual_ip: Option<String>,
    prefix_len: Option<u8>,
    dns_servers: Vec<String>,
    routes: Vec<RouteSpec>,
    relay_config: Option<RelayDataPlaneConfig>,
    mock_enabled: bool,
    network_enabled: bool,
}

impl LinuxRuntime {
    fn interface_name(&self) -> &str {
        if self.interface_name.trim().is_empty() {
            DEFAULT_INTERFACE_NAME
        } else {
            self.interface_name.as_str()
        }
    }
}

fn runtime() -> &'static Mutex<LinuxRuntime> {
    static RUNTIME: OnceLock<Mutex<LinuxRuntime>> = OnceLock::new();
    RUNTIME.get_or_init(|| {
        Mutex::new(LinuxRuntime {
            interface_name: interface_name(),
            ..LinuxRuntime::default()
        })
    })
}

impl PlatformNetwork for LinuxPlatformNetwork {
    fn install_adapter(&self) -> Result<()> {
        let mut runtime = runtime().lock().expect("linux runtime mutex poisoned");
        runtime.mock_enabled = mock_enabled();
        if runtime.mock_enabled {
            runtime.network_enabled = true;
            return Ok(());
        }

        ensure_ip_command()?;
        let interface_name = runtime.interface_name().to_string();
        if !link_exists(&interface_name) {
            run_ip(&["tuntap", "add", "dev", &interface_name, "mode", "tun"])
                .with_context(|| format!("create Linux TUN interface {interface_name}"))?;
        }
        run_ip(&[
            "link",
            "set",
            "dev",
            &interface_name,
            "mtu",
            &DEFAULT_MTU.to_string(),
        ])
        .with_context(|| format!("set Linux TUN MTU on {interface_name}"))?;
        run_ip(&["link", "set", "dev", &interface_name, "up"])
            .with_context(|| format!("bring Linux TUN interface {interface_name} up"))?;
        runtime.network_enabled = true;
        Ok(())
    }

    fn configure_ip(&self, virtual_ip: &str, prefix_len: u8) -> Result<()> {
        let mut runtime = runtime().lock().expect("linux runtime mutex poisoned");
        let virtual_ip = normalize_ipv4(virtual_ip)?;
        runtime.virtual_ip = Some(virtual_ip.to_string());
        runtime.prefix_len = Some(prefix_len);
        if runtime.mock_enabled {
            return Ok(());
        }
        let interface_name = runtime.interface_name().to_string();
        run_ip(&[
            "addr",
            "replace",
            &format!("{virtual_ip}/{prefix_len}"),
            "dev",
            &interface_name,
        ])
        .with_context(|| format!("configure Linux TUN IP {virtual_ip}/{prefix_len}"))?;
        Ok(())
    }

    fn configure_routes(&self, routes: &[RouteSpec]) -> Result<()> {
        let mut runtime = runtime().lock().expect("linux runtime mutex poisoned");
        runtime.routes = routes.to_vec();
        if runtime.mock_enabled {
            return Ok(());
        }
        let interface_name = runtime.interface_name().to_string();
        for route in routes.iter().filter(|route| route.destination != "mesh") {
            let destination = normalize_route_destination(&route.destination)?;
            if let Some(gateway) = route
                .gateway
                .as_deref()
                .filter(|value| !value.trim().is_empty())
            {
                run_ip(&[
                    "route",
                    "replace",
                    &destination,
                    "via",
                    gateway,
                    "dev",
                    &interface_name,
                ])
                .with_context(|| format!("replace Linux route {destination} via {gateway}"))?;
            } else {
                run_ip(&["route", "replace", &destination, "dev", &interface_name])
                    .with_context(|| format!("replace Linux route {destination}"))?;
            }
        }
        Ok(())
    }

    fn configure_dns(&self, dns_servers: &[String]) -> Result<()> {
        let mut runtime = runtime().lock().expect("linux runtime mutex poisoned");
        runtime.dns_servers = dns_servers
            .iter()
            .map(|value| value.trim().to_string())
            .filter(|value| !value.is_empty())
            .collect();
        if runtime.mock_enabled || runtime.dns_servers.is_empty() {
            return Ok(());
        }
        let interface_name = runtime.interface_name().to_string();
        if command_available("resolvectl") {
            let mut dns_args = vec!["dns".to_string(), interface_name.clone()];
            dns_args.extend(runtime.dns_servers.iter().cloned());
            run_command("resolvectl", &dns_args)?;
            let _ = run_command(
                "resolvectl",
                &[
                    "domain".to_string(),
                    interface_name.clone(),
                    "~slan".to_string(),
                ],
            );
            let _ = run_command(
                "resolvectl",
                &[
                    "default-route".to_string(),
                    interface_name,
                    "false".to_string(),
                ],
            );
        }
        Ok(())
    }

    fn configure_relay(&self, relay_config: Option<&RelayDataPlaneConfig>) -> Result<()> {
        let mut runtime = runtime().lock().expect("linux runtime mutex poisoned");
        runtime.relay_config = relay_config.cloned();
        Ok(())
    }

    fn diagnostics(&self) -> Result<PlatformNetworkDiagnostics> {
        let runtime = runtime().lock().expect("linux runtime mutex poisoned");
        let interface_name = runtime.interface_name().to_string();
        let adapter_present = runtime.mock_enabled || link_exists(&interface_name);
        let mut checks = vec![
            PlatformDiagnosticCheck {
                name: "iproute2".to_string(),
                ok: command_available("ip"),
                message: Some("ip command is required for Linux TUN setup".to_string()),
            },
            PlatformDiagnosticCheck {
                name: "tunDevice".to_string(),
                ok: std::path::Path::new("/dev/net/tun").exists() || runtime.mock_enabled,
                message: Some(
                    "/dev/net/tun is required unless SLAN_LINUX_NETWORK_MOCK=1".to_string(),
                ),
            },
        ];
        checks.push(PlatformDiagnosticCheck {
            name: "dns".to_string(),
            ok: command_available("resolvectl") || runtime.dns_servers.is_empty(),
            message: Some("resolvectl is used when Linux DNS servers are configured".to_string()),
        });

        Ok(PlatformNetworkDiagnostics {
            platform: "linux".to_string(),
            adapter_present,
            adapter_name: Some(interface_name),
            admin_status: Some(if runtime.network_enabled {
                "up".to_string()
            } else {
                "down".to_string()
            }),
            interface_index: None,
            virtual_ip: runtime.virtual_ip.clone(),
            mtu: Some(DEFAULT_MTU),
            mss: None,
            dns_servers: runtime.dns_servers.clone(),
            routes: runtime
                .routes
                .iter()
                .map(|route| route.destination.clone())
                .collect(),
            checks,
        })
    }

    fn disable_network(&self) -> Result<()> {
        let mut runtime = runtime().lock().expect("linux runtime mutex poisoned");
        if !runtime.mock_enabled {
            let interface_name = runtime.interface_name().to_string();
            for route in runtime
                .routes
                .iter()
                .filter(|route| route.destination != "mesh")
            {
                if let Ok(destination) = normalize_route_destination(&route.destination) {
                    let _ = run_ip(&["route", "del", &destination, "dev", &interface_name]);
                }
            }
            if command_available("resolvectl") {
                let _ = run_command(
                    "resolvectl",
                    &["revert".to_string(), interface_name.clone()],
                );
            }
            let _ = run_ip(&["addr", "flush", "dev", &interface_name]);
            let _ = run_ip(&["link", "set", "dev", &interface_name, "down"]);
        }
        runtime.network_enabled = false;
        runtime.virtual_ip = None;
        runtime.prefix_len = None;
        runtime.routes.clear();
        runtime.relay_config = None;
        Ok(())
    }

    fn read_runtime_state(&self) -> Result<NetworkRuntimeState> {
        let runtime = runtime().lock().expect("linux runtime mutex poisoned");
        let adapter_present = runtime.mock_enabled || link_exists(runtime.interface_name());
        Ok(NetworkRuntimeState {
            adapter_present,
            network_enabled: adapter_present && runtime.network_enabled,
            virtual_ip: runtime.virtual_ip.clone(),
            active_path: None,
            peer_paths: runtime
                .relay_config
                .as_ref()
                .map(|config| {
                    config
                        .peer_paths
                        .iter()
                        .map(|path| PeerPathRuntime {
                            peer_node_id: path.peer_node_id.clone(),
                            peer_virtual_ips: path.peer_virtual_ips.clone(),
                            active_path: None,
                            candidates: path.candidates.clone(),
                        })
                        .collect()
                })
                .unwrap_or_default(),
        })
    }
}

fn interface_name() -> String {
    std::env::var("SLAN_LINUX_TUN_NAME")
        .ok()
        .map(|value| value.trim().to_string())
        .filter(|value| !value.is_empty())
        .unwrap_or_else(|| DEFAULT_INTERFACE_NAME.to_string())
}

fn mock_enabled() -> bool {
    matches!(
        std::env::var("SLAN_LINUX_NETWORK_MOCK")
            .unwrap_or_default()
            .to_ascii_lowercase()
            .as_str(),
        "1" | "true" | "yes" | "on"
    )
}

fn normalize_ipv4(value: &str) -> Result<Ipv4Addr> {
    value
        .trim()
        .parse::<Ipv4Addr>()
        .with_context(|| format!("invalid Linux virtual IPv4 address: {value}"))
}

fn normalize_route_destination(destination: &str) -> Result<String> {
    let value = destination.trim();
    if value.is_empty() {
        bail!("Linux route destination is empty");
    }
    if value == "mesh" {
        return Ok(value.to_string());
    }
    if value.contains('/') {
        let (ip, prefix) = value
            .split_once('/')
            .ok_or_else(|| anyhow!("invalid Linux route destination: {value}"))?;
        normalize_ipv4(ip)?;
        let prefix_len = prefix
            .parse::<u8>()
            .with_context(|| format!("invalid Linux route prefix: {value}"))?;
        if prefix_len > 32 {
            bail!("invalid Linux route prefix: {value}");
        }
        return Ok(format!("{ip}/{prefix_len}"));
    }
    normalize_ipv4(value)?;
    Ok(format!("{value}/32"))
}

fn ensure_ip_command() -> Result<()> {
    if command_available("ip") {
        Ok(())
    } else {
        bail!("Linux iproute2 command 'ip' is required")
    }
}

fn link_exists(interface_name: &str) -> bool {
    Command::new("ip")
        .args(["link", "show", "dev", interface_name])
        .output()
        .map(|output| output.status.success())
        .unwrap_or(false)
}

fn run_ip(args: &[&str]) -> Result<()> {
    let owned: Vec<String> = args.iter().map(|value| value.to_string()).collect();
    run_command("ip", &owned)
}

fn run_command(program: &str, args: &[String]) -> Result<()> {
    let output = Command::new(program)
        .args(args)
        .output()
        .with_context(|| format!("run {program} {}", args.join(" ")))?;
    if output.status.success() {
        return Ok(());
    }
    let stderr = String::from_utf8_lossy(&output.stderr).trim().to_string();
    let stdout = String::from_utf8_lossy(&output.stdout).trim().to_string();
    let message = if !stderr.is_empty() { stderr } else { stdout };
    bail!(
        "{} {} failed: {}",
        program,
        args.join(" "),
        if message.is_empty() {
            output.status.to_string()
        } else {
            message
        }
    )
}

fn command_available(program: &str) -> bool {
    Command::new(program)
        .arg("-V")
        .output()
        .map(|output| output.status.success())
        .unwrap_or(false)
        || Command::new("which")
            .arg(program)
            .output()
            .map(|output| output.status.success())
            .unwrap_or(false)
}
