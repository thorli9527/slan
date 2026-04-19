use std::sync::Arc;

use crate::{
    InMemoryTunnelManager, LinuxKernelWireGuardBackend, SystemTunnelManager, TunnelManager,
};

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum TunnelDriverKind {
    Auto,
    InMemory,
    LinuxKernel,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct TunnelDriverSelection {
    pub requested: TunnelDriverKind,
    pub selected: TunnelDriverKind,
    pub execution_mode: &'static str,
    pub execution_backend: &'static str,
    pub platform: &'static str,
}

impl TunnelDriverKind {
    pub fn parse(raw: Option<&str>) -> Result<Self, String> {
        match raw
            .map(str::trim)
            .filter(|value| !value.is_empty())
            .unwrap_or("auto")
        {
            "auto" => Ok(Self::Auto),
            "in-memory" | "memory" | "inmemory" => Ok(Self::InMemory),
            "linux-kernel" | "linux" | "wg-kernel" => Ok(Self::LinuxKernel),
            other => Err(format!(
                "unsupported tunnel driver '{other}'; expected auto, in-memory, or linux-kernel"
            )),
        }
    }
}

pub fn detect_platform_tunnel_driver(
    requested: Option<&str>,
    dry_run: bool,
) -> Result<TunnelDriverSelection, String> {
    let requested = TunnelDriverKind::parse(requested)?;
    let selected = match requested {
        TunnelDriverKind::Auto => {
            #[cfg(target_os = "linux")]
            {
                TunnelDriverKind::LinuxKernel
            }
            #[cfg(not(target_os = "linux"))]
            {
                TunnelDriverKind::InMemory
            }
        }
        other => other,
    };
    let execution_mode = match selected {
        TunnelDriverKind::LinuxKernel if !dry_run => "system",
        TunnelDriverKind::LinuxKernel => "dry-run",
        TunnelDriverKind::InMemory => "memory",
        TunnelDriverKind::Auto => "unknown",
    };
    let requested_linux_executor = std::env::var("SLAN_LINUX_EXECUTOR")
        .ok()
        .unwrap_or_else(|| "shell".to_string());
    let execution_backend = match selected {
        TunnelDriverKind::LinuxKernel => match requested_linux_executor.as_str() {
            "native" | "netlink" => "native",
            _ => "shell",
        },
        TunnelDriverKind::InMemory => "memory",
        TunnelDriverKind::Auto => "unknown",
    };
    let platform = if cfg!(target_os = "linux") {
        "linux"
    } else if cfg!(target_os = "macos") {
        "macos"
    } else if cfg!(target_os = "windows") {
        "windows"
    } else {
        "other"
    };
    Ok(TunnelDriverSelection {
        requested,
        selected,
        execution_mode,
        execution_backend,
        platform,
    })
}

pub fn build_platform_tunnel_manager(
    selection: &TunnelDriverSelection,
) -> Result<Arc<dyn TunnelManager>, String> {
    match selection.selected {
        TunnelDriverKind::InMemory => Ok(Arc::new(InMemoryTunnelManager::default())),
        TunnelDriverKind::LinuxKernel => Ok(Arc::new(SystemTunnelManager::new(
            LinuxKernelWireGuardBackend::new(),
        ))),
        TunnelDriverKind::Auto => Err("auto tunnel driver selection is unresolved".to_string()),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn parses_linux_driver_alias() {
        assert_eq!(
            TunnelDriverKind::parse(Some("linux")).unwrap(),
            TunnelDriverKind::LinuxKernel
        );
    }

    #[test]
    fn detects_dry_run_linux_mode() {
        let selection =
            detect_platform_tunnel_driver(Some("linux-kernel"), true).expect("selection");
        assert_eq!(selection.requested, TunnelDriverKind::LinuxKernel);
        assert_eq!(selection.selected, TunnelDriverKind::LinuxKernel);
        assert_eq!(selection.execution_mode, "dry-run");
        assert_eq!(selection.execution_backend, "shell");
    }
}
