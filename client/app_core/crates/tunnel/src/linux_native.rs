use crate::linux_adapter::{LinuxCommandSpec, LinuxKernelCommandExecutor};

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
enum NativeLinuxOperation {
    CreateInterface,
    ConfigureInterface,
    SetLinkMtu,
    AssignAddress,
    ConfigurePeer,
    ReplaceRoute,
    RemovePeer,
    DeleteRoute,
    BringUp,
    BringDown,
    Unknown,
}

impl NativeLinuxOperation {
    fn classify(spec: &LinuxCommandSpec) -> Self {
        let args = spec.args.iter().map(String::as_str).collect::<Vec<_>>();
        match (spec.program.as_str(), args.as_slice()) {
            ("ip", ["link", "add", ..]) => Self::CreateInterface,
            ("wg", ["set", _, "private-key", ..]) => Self::ConfigureInterface,
            ("ip", ["link", "set", _, "mtu", ..]) => Self::SetLinkMtu,
            ("ip", ["address", "replace", ..]) => Self::AssignAddress,
            ("wg", ["set", _, "peer", _, "remove"]) => Self::RemovePeer,
            ("wg", ["set", _, "peer", ..]) => Self::ConfigurePeer,
            ("ip", ["route", "replace", ..]) => Self::ReplaceRoute,
            ("ip", ["route", "del", ..]) => Self::DeleteRoute,
            ("ip", ["link", "set", "up", ..]) => Self::BringUp,
            ("ip", ["link", "set", "down", ..]) => Self::BringDown,
            _ => Self::Unknown,
        }
    }

    fn description(self) -> &'static str {
        match self {
            Self::CreateInterface => "create interface",
            Self::ConfigureInterface => "configure interface",
            Self::SetLinkMtu => "set link mtu",
            Self::AssignAddress => "assign address",
            Self::ConfigurePeer => "configure peer",
            Self::ReplaceRoute => "replace route",
            Self::RemovePeer => "remove peer",
            Self::DeleteRoute => "delete route",
            Self::BringUp => "bring link up",
            Self::BringDown => "bring link down",
            Self::Unknown => "unknown operation",
        }
    }
}

pub struct NativeLinuxCommandExecutor;

impl LinuxKernelCommandExecutor for NativeLinuxCommandExecutor {
    fn run(&self, spec: &LinuxCommandSpec) -> Result<(), String> {
        let operation = NativeLinuxOperation::classify(spec);
        Err(format!(
            "native linux wireguard executor is not integrated yet; cannot {} via {} {}",
            operation.description(),
            spec.program,
            spec.args.join(" ")
        ))
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn spec(program: &str, args: &[&str]) -> LinuxCommandSpec {
        LinuxCommandSpec {
            program: program.to_string(),
            args: args.iter().map(|value| (*value).to_string()).collect(),
            stdin: None,
            ignored_stderr_substrings: Vec::new(),
        }
    }

    #[test]
    fn classifies_peer_configuration_command() {
        let operation = NativeLinuxOperation::classify(&spec(
            "wg",
            &[
                "set",
                "wg0",
                "peer",
                "peer-pk",
                "allowed-ips",
                "100.64.0.2/32",
            ],
        ));
        assert_eq!(operation, NativeLinuxOperation::ConfigurePeer);
    }

    #[test]
    fn native_executor_surfaces_operation_context() {
        let executor = NativeLinuxCommandExecutor;
        let err = executor
            .run(&spec("ip", &["link", "set", "up", "dev", "wg0"]))
            .unwrap_err();
        assert!(err.contains("bring link up"));
        assert!(err.contains("ip link set up dev wg0"));
    }
}
