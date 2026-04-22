use std::collections::HashMap;
use std::env;
#[cfg(target_os = "linux")]
use std::io::Write;
#[cfg(target_os = "linux")]
use std::process::{Command, Stdio};
use std::sync::Mutex;

use crate::linux_mapper::{LinuxKernelInterfacePlan, LinuxKernelPeerPlan};
use crate::linux_native::NativeLinuxCommandExecutor;
use crate::linux_stats::LinuxKernelRuntime;

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct LinuxCommandSpec {
    pub program: String,
    pub args: Vec<String>,
    pub stdin: Option<String>,
    pub ignored_stderr_substrings: Vec<String>,
}

impl LinuxCommandSpec {
    fn new(program: impl Into<String>, args: Vec<String>) -> Self {
        Self {
            program: program.into(),
            args,
            stdin: None,
            ignored_stderr_substrings: Vec::new(),
        }
    }

    fn with_stdin(mut self, stdin: impl Into<String>) -> Self {
        self.stdin = Some(stdin.into());
        self
    }

    fn ignore_stderr(mut self, fragment: impl Into<String>) -> Self {
        self.ignored_stderr_substrings.push(fragment.into());
        self
    }
}

pub trait LinuxKernelCommandExecutor: Send + Sync {
    fn run(&self, spec: &LinuxCommandSpec) -> Result<(), String>;
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum LinuxExecutionMode {
    System,
    DryRun,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum LinuxExecutionBackend {
    Shell,
    Native,
}

struct NoopLinuxCommandExecutor;

impl LinuxKernelCommandExecutor for NoopLinuxCommandExecutor {
    fn run(&self, _spec: &LinuxCommandSpec) -> Result<(), String> {
        Ok(())
    }
}

#[cfg(target_os = "linux")]
struct SystemLinuxCommandExecutor;

#[cfg(target_os = "linux")]
impl LinuxKernelCommandExecutor for SystemLinuxCommandExecutor {
    fn run(&self, spec: &LinuxCommandSpec) -> Result<(), String> {
        let mut command = Command::new(&spec.program);
        command.args(&spec.args);
        if spec.stdin.is_some() {
            command.stdin(Stdio::piped());
        }
        let mut child = command
            .stdout(Stdio::null())
            .stderr(Stdio::piped())
            .spawn()
            .map_err(|err| format!("failed to spawn {}: {err}", spec.program))?;
        if let Some(stdin) = &spec.stdin {
            let mut input = child
                .stdin
                .take()
                .ok_or_else(|| format!("{} stdin unavailable", spec.program))?;
            input
                .write_all(stdin.as_bytes())
                .map_err(|err| format!("failed to write {} stdin: {err}", spec.program))?;
        }
        let output = child
            .wait_with_output()
            .map_err(|err| format!("failed to wait {}: {err}", spec.program))?;
        if output.status.success() {
            return Ok(());
        }
        let stderr = String::from_utf8_lossy(&output.stderr).trim().to_string();
        if spec
            .ignored_stderr_substrings
            .iter()
            .any(|fragment| stderr.contains(fragment))
        {
            return Ok(());
        }
        Err(format!(
            "{} {} failed: {}",
            spec.program,
            spec.args.join(" "),
            if stderr.is_empty() {
                output.status.to_string()
            } else {
                stderr
            }
        ))
    }
}

#[derive(Debug, Clone)]
pub struct LinuxKernelPeerRuntime {
    pub peer: LinuxKernelPeerPlan,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct LinuxDriverDiagnostics {
    pub execution_mode: LinuxExecutionMode,
    pub execution_backend: LinuxExecutionBackend,
    pub interface_name: Option<String>,
    pub is_up: bool,
    pub planned_peer_count: usize,
    pub recent_command_count: usize,
}

pub struct LinuxKernelWireGuardAdapter {
    executor: Box<dyn LinuxKernelCommandExecutor>,
    execution_mode: LinuxExecutionMode,
    execution_backend: LinuxExecutionBackend,
    interface: Mutex<Option<LinuxKernelRuntime>>,
    peers: Mutex<HashMap<String, LinuxKernelPeerRuntime>>,
    recent_commands: Mutex<Vec<LinuxCommandSpec>>,
}

impl Default for LinuxKernelWireGuardAdapter {
    fn default() -> Self {
        let dry_run = env::var("SLAN_TUNNEL_DRY_RUN")
            .ok()
            .map(|value| matches!(value.as_str(), "1" | "true" | "TRUE" | "yes" | "on"))
            .unwrap_or(false);
        let requested_backend = env::var("SLAN_LINUX_EXECUTOR")
            .ok()
            .unwrap_or_else(|| "shell".to_string());
        if dry_run {
            return Self::new_with_executor(
                match requested_backend.as_str() {
                    "native" | "netlink" => Box::new(NativeLinuxCommandExecutor),
                    _ => Box::new(NoopLinuxCommandExecutor),
                },
                LinuxExecutionMode::DryRun,
                match requested_backend.as_str() {
                    "native" | "netlink" => LinuxExecutionBackend::Native,
                    _ => LinuxExecutionBackend::Shell,
                },
            );
        }
        #[cfg(target_os = "linux")]
        {
            match requested_backend.as_str() {
                "native" | "netlink" => Self::new_with_executor(
                    Box::new(NativeLinuxCommandExecutor),
                    LinuxExecutionMode::System,
                    LinuxExecutionBackend::Native,
                ),
                _ => Self::new_with_executor(
                    Box::new(SystemLinuxCommandExecutor),
                    LinuxExecutionMode::System,
                    LinuxExecutionBackend::Shell,
                ),
            }
        }
        #[cfg(not(target_os = "linux"))]
        {
            Self::new_with_executor(
                Box::new(NoopLinuxCommandExecutor),
                LinuxExecutionMode::DryRun,
                LinuxExecutionBackend::Shell,
            )
        }
    }
}

impl LinuxKernelWireGuardAdapter {
    pub fn new_with_executor(
        executor: Box<dyn LinuxKernelCommandExecutor>,
        execution_mode: LinuxExecutionMode,
        execution_backend: LinuxExecutionBackend,
    ) -> Self {
        Self {
            executor,
            execution_mode,
            execution_backend,
            interface: Mutex::new(None),
            peers: Mutex::new(HashMap::new()),
            recent_commands: Mutex::new(Vec::new()),
        }
    }

    fn interface_name_from_plan(interface: &LinuxKernelInterfacePlan) -> String {
        interface
            .interface_name
            .clone()
            .unwrap_or_else(|| "wg0".to_string())
    }

    fn interface_name_from_runtime(runtime: &LinuxKernelRuntime) -> String {
        runtime
            .interface_name
            .clone()
            .unwrap_or_else(|| "wg0".to_string())
    }

    fn apply_interface_commands(interface: &LinuxKernelInterfacePlan) -> Vec<LinuxCommandSpec> {
        let interface_name = Self::interface_name_from_plan(interface);
        let mut commands = vec![LinuxCommandSpec::new(
            "ip",
            vec![
                "link".into(),
                "add".into(),
                "dev".into(),
                interface_name.clone(),
                "type".into(),
                "wireguard".into(),
            ],
        )
        .ignore_stderr("File exists")];
        let mut wg_args = vec![
            "set".into(),
            interface_name.clone(),
            "private-key".into(),
            "/dev/stdin".into(),
        ];
        if let Some(listen_port) = interface.listen_port {
            wg_args.push("listen-port".into());
            wg_args.push(listen_port.to_string());
        }
        commands
            .push(LinuxCommandSpec::new("wg", wg_args).with_stdin(interface.private_key.clone()));
        if let Some(mtu) = interface.mtu {
            commands.push(LinuxCommandSpec::new(
                "ip",
                vec![
                    "link".into(),
                    "set".into(),
                    "dev".into(),
                    interface_name.clone(),
                    "mtu".into(),
                    mtu.to_string(),
                ],
            ));
        }
        commands.extend(interface.addresses.iter().cloned().map(|address| {
            LinuxCommandSpec::new(
                "ip",
                vec![
                    "address".into(),
                    "replace".into(),
                    address,
                    "dev".into(),
                    interface_name.clone(),
                ],
            )
        }));
        commands
    }

    fn apply_peer_commands(
        interface_name: &str,
        peer: &LinuxKernelPeerPlan,
    ) -> Vec<LinuxCommandSpec> {
        let mut wg_args = vec![
            "set".into(),
            interface_name.to_string(),
            "peer".into(),
            peer.public_key.clone(),
        ];
        if let Some(endpoint) = &peer.endpoint {
            wg_args.push("endpoint".into());
            wg_args.push(endpoint.clone());
        }
        wg_args.push("allowed-ips".into());
        wg_args.push(peer.allowed_ips.join(","));
        if let Some(keepalive) = peer.persistent_keepalive_seconds {
            wg_args.push("persistent-keepalive".into());
            wg_args.push(keepalive.to_string());
        }
        let wg_command = match &peer.preshared_key {
            Some(preshared_key) => LinuxCommandSpec::new("wg", {
                let mut args = wg_args;
                args.push("preshared-key".into());
                args.push("/dev/stdin".into());
                args
            })
            .with_stdin(preshared_key.clone()),
            None => LinuxCommandSpec::new("wg", wg_args),
        };
        let mut commands = vec![wg_command];
        commands.extend(peer.allowed_ips.iter().cloned().map(|allowed_ip| {
            LinuxCommandSpec::new(
                "ip",
                vec![
                    "route".into(),
                    "replace".into(),
                    allowed_ip,
                    "dev".into(),
                    interface_name.to_string(),
                ],
            )
        }));
        commands
    }

    fn remove_peer_commands(
        interface_name: &str,
        peer: &LinuxKernelPeerPlan,
    ) -> Vec<LinuxCommandSpec> {
        let mut commands = vec![LinuxCommandSpec::new(
            "wg",
            vec![
                "set".into(),
                interface_name.to_string(),
                "peer".into(),
                peer.public_key.clone(),
                "remove".into(),
            ],
        )];
        commands.extend(peer.allowed_ips.iter().cloned().map(|allowed_ip| {
            LinuxCommandSpec::new(
                "ip",
                vec![
                    "route".into(),
                    "del".into(),
                    allowed_ip,
                    "dev".into(),
                    interface_name.to_string(),
                ],
            )
            .ignore_stderr("No such process")
            .ignore_stderr("Cannot find device")
        }));
        commands
    }

    fn bring_up_command(interface_name: &str) -> LinuxCommandSpec {
        LinuxCommandSpec::new(
            "ip",
            vec![
                "link".into(),
                "set".into(),
                "up".into(),
                "dev".into(),
                interface_name.to_string(),
            ],
        )
    }

    fn bring_down_command(interface_name: &str) -> LinuxCommandSpec {
        LinuxCommandSpec::new(
            "ip",
            vec![
                "link".into(),
                "set".into(),
                "down".into(),
                "dev".into(),
                interface_name.to_string(),
            ],
        )
    }

    fn run_all(&self, commands: &[LinuxCommandSpec]) -> Result<(), String> {
        self.recent_commands
            .lock()
            .map_err(|_| "linux adapter command state poisoned".to_string())?
            .extend(commands.iter().cloned());
        for command in commands {
            self.executor.run(command)?;
        }
        Ok(())
    }

    pub fn apply_interface(
        &self,
        interface: LinuxKernelInterfacePlan,
        runtime: LinuxKernelRuntime,
    ) -> Result<(), String> {
        let commands = Self::apply_interface_commands(&interface);
        self.run_all(&commands)?;
        let mut current = self
            .interface
            .lock()
            .map_err(|_| "linux adapter interface state poisoned".to_string())?;
        *current = Some(runtime);
        Ok(())
    }

    pub fn apply_peer(
        &self,
        peer_virtual_ip: &str,
        peer: LinuxKernelPeerPlan,
    ) -> Result<(), String> {
        let interface_name = self
            .interface_runtime()
            .map(|runtime| Self::interface_name_from_runtime(&runtime))
            .ok_or_else(|| "linux adapter interface runtime unavailable".to_string())?;
        let commands = Self::apply_peer_commands(&interface_name, &peer);
        self.run_all(&commands)?;
        let mut peers = self
            .peers
            .lock()
            .map_err(|_| "linux adapter peer state poisoned".to_string())?;
        peers.insert(peer_virtual_ip.to_string(), LinuxKernelPeerRuntime { peer });
        Ok(())
    }

    pub fn remove_peer(&self, peer_virtual_ip: &str) -> Result<(), String> {
        let interface_name = self
            .interface_runtime()
            .map(|runtime| Self::interface_name_from_runtime(&runtime))
            .ok_or_else(|| "linux adapter interface runtime unavailable".to_string())?;
        let mut peers = self
            .peers
            .lock()
            .map_err(|_| "linux adapter peer state poisoned".to_string())?;
        let Some(peer) = peers.remove(peer_virtual_ip) else {
            return Ok(());
        };
        let commands = Self::remove_peer_commands(&interface_name, &peer.peer);
        drop(peers);
        self.run_all(&commands)
    }

    pub fn bring_up(&self) -> Result<(), String> {
        let mut interface = self
            .interface
            .lock()
            .map_err(|_| "linux adapter interface state poisoned".to_string())?;
        let Some(runtime) = interface.as_mut() else {
            return Err("linux adapter interface runtime unavailable".to_string());
        };
        let command = Self::bring_up_command(&Self::interface_name_from_runtime(runtime));
        self.recent_commands
            .lock()
            .map_err(|_| "linux adapter command state poisoned".to_string())?
            .push(command.clone());
        self.executor.run(&command)?;
        runtime.is_up = true;
        Ok(())
    }

    pub fn bring_down(&self) -> Result<(), String> {
        let mut interface = self
            .interface
            .lock()
            .map_err(|_| "linux adapter interface state poisoned".to_string())?;
        if let Some(runtime) = interface.as_mut() {
            let command = Self::bring_down_command(&Self::interface_name_from_runtime(runtime));
            self.recent_commands
                .lock()
                .map_err(|_| "linux adapter command state poisoned".to_string())?
                .push(command.clone());
            self.executor.run(&command)?;
            runtime.is_up = false;
        }
        Ok(())
    }

    pub fn planned_peer_ips(&self) -> Vec<String> {
        self.peers
            .lock()
            .map(|state| state.keys().cloned().collect())
            .unwrap_or_default()
    }

    pub fn interface_runtime(&self) -> Option<LinuxKernelRuntime> {
        self.interface
            .lock()
            .ok()
            .and_then(|runtime| runtime.clone())
    }

    pub fn peer_runtime(&self, peer_virtual_ip: &str) -> Option<LinuxKernelPeerRuntime> {
        self.peers
            .lock()
            .ok()
            .and_then(|peers| peers.get(peer_virtual_ip).cloned())
    }

    pub fn recent_commands(&self) -> Vec<LinuxCommandSpec> {
        self.recent_commands
            .lock()
            .map(|commands| commands.clone())
            .unwrap_or_default()
    }

    pub fn diagnostics(&self) -> LinuxDriverDiagnostics {
        let interface_runtime = self.interface_runtime();
        LinuxDriverDiagnostics {
            execution_mode: self.execution_mode,
            execution_backend: self.execution_backend,
            interface_name: interface_runtime
                .as_ref()
                .and_then(|runtime| runtime.interface_name.clone()),
            is_up: interface_runtime
                .map(|runtime| runtime.is_up)
                .unwrap_or(false),
            planned_peer_count: self.planned_peer_ips().len(),
            recent_command_count: self.recent_commands().len(),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[allow(dead_code)]
    #[derive(Default)]
    pub struct RecordingLinuxCommandExecutor {
        pub commands: Mutex<Vec<LinuxCommandSpec>>,
    }

    impl LinuxKernelCommandExecutor for RecordingLinuxCommandExecutor {
        fn run(&self, spec: &LinuxCommandSpec) -> Result<(), String> {
            self.commands
                .lock()
                .map_err(|_| "recording linux command executor poisoned".to_string())?
                .push(spec.clone());
            Ok(())
        }
    }
}
