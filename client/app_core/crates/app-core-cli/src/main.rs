use std::env;
use std::fs;
use std::io::{self, Write};
use std::path::{Path, PathBuf};
use std::sync::Arc;

use clap::{Args, Parser, Subcommand};
use control_mqtt_client::{ControlMqttClient, ControlMqttConfig, ControlSessionBootstrap};
use controller_client::{HttpControllerClient, TcpJsonHttpTransport};
use ffi_bridge::{
    AppCoreFacade, AppCoreSnapshot, DataPlaneErrorCode, DataPlaneProbe, DefaultAppCoreFacade,
    FileTunnelKeyProvider,
};
use p2p::SocketP2PConnector;
use relay_client::{InMemoryDerpPool, InMemoryPathManager, SocketRelayClient};
use serde_json::{json, Value};
use slan_app_core::{ConnectionPath, ConnectionState, NetworkJoinResult};
use tunnel::{
    build_platform_tunnel_manager, detect_platform_tunnel_driver, TunnelDriverKind,
    TunnelDriverSelection, TunnelManager,
};

type CliFacade = DefaultAppCoreFacade<
    HttpControllerClient<TcpJsonHttpTransport>,
    Arc<SocketP2PConnector>,
    Arc<SocketRelayClient>,
    Arc<InMemoryDerpPool>,
    InMemoryPathManager<Arc<InMemoryDerpPool>, Arc<SocketRelayClient>, Arc<SocketP2PConnector>>,
    Arc<dyn TunnelManager>,
>;

fn main() {
    if let Err(err) = run() {
        let _ = writeln!(io::stderr(), "{err}");
        std::process::exit(1);
    }
}

fn run() -> Result<(), String> {
    let cli = Cli::parse();
    let state_path = cli.state_file.unwrap_or_else(default_state_file);

    if matches!(
        cli.command,
        Command::State {
            command: StateCommand::Clear
        }
    ) {
        clear_state_file(&state_path)?;
        print_output(
            cli.json,
            &json!({
                "status": "cleared",
                "stateFile": state_path.display().to_string(),
            }),
            &format!("cleared state file {}", state_path.display()),
        )?;
        return Ok(());
    }

    let mut snapshot = load_snapshot(&state_path)?;
    if let Command::Control { command } = &cli.command {
        let (json_output, text_output) = run_control_command(command, &mut snapshot)?;
        save_snapshot(&state_path, &snapshot)?;
        print_output(cli.json, &json_output, &text_output)?;
        return Ok(());
    }
    let command_requires_backend = !matches!(
        cli.command,
        Command::State {
            command: StateCommand::Show
        } | Command::State {
            command: StateCommand::Path
        } | Command::Connection {
            command: ConnectionCommand::Status
        } | Command::Connection {
            command: ConnectionCommand::Path
        } | Command::Tunnel {
            command: TunnelCommand::Status
        } | Command::Tunnel {
            command: TunnelCommand::Keys
        } | Command::Tunnel {
            command: TunnelCommand::Driver
        } | Command::Platform {
            command: PlatformCommand::Detect
        } | Command::Platform {
            command: PlatformCommand::Plan
        } | Command::Doctor
            | Command::BootstrapStatus
            | Command::ProbeLast
            | Command::ErrorLast
            | Command::Control { .. }
    );

    if !command_requires_backend {
        return run_local_state_command(cli.json, cli.command, &state_path, &snapshot);
    }

    let base_url = cli
        .control_base_url
        .or_else(|| env::var("SLAN_CONTROL_BASE_URL").ok())
        .ok_or_else(|| {
            "missing control base URL; pass --control-base-url or set SLAN_CONTROL_BASE_URL"
                .to_string()
        })?;

    let facade = build_facade(base_url, &state_path)?;
    facade.restore_snapshot(snapshot.clone())?;

    let (json_output, text_output) = execute_command(&cli.command, &facade, &snapshot)?;

    snapshot = facade.snapshot()?;
    save_snapshot(&state_path, &snapshot)?;
    print_output(cli.json, &json_output, &text_output)?;
    Ok(())
}

fn run_local_state_command(
    json_mode: bool,
    command: Command,
    state_path: &Path,
    snapshot: &AppCoreSnapshot,
) -> Result<(), String> {
    match command {
        Command::State {
            command: StateCommand::Show,
        } => print_output(
            json_mode,
            &serde_json::to_value(snapshot).map_err(|err| err.to_string())?,
            &format_state(snapshot),
        ),
        Command::State {
            command: StateCommand::Path,
        } => print_output(
            json_mode,
            &json!({ "stateFile": state_path.display().to_string() }),
            &state_path.display().to_string(),
        ),
        Command::State {
            command: StateCommand::Clear,
        } => unreachable!(),
        Command::Connection {
            command: ConnectionCommand::Status,
        } => print_output(
            json_mode,
            &connection_status_snapshot_json(snapshot),
            &format_connection_status(snapshot),
        ),
        Command::Connection {
            command: ConnectionCommand::Path,
        } => print_output(
            json_mode,
            &connection_path_snapshot_json(snapshot),
            &format_connection_path(snapshot),
        ),
        Command::Tunnel {
            command: TunnelCommand::Status,
        } => print_output(
            json_mode,
            &tunnel_status_snapshot_json(snapshot),
            &format_tunnel_status(snapshot),
        ),
        Command::Tunnel {
            command: TunnelCommand::Keys,
        } => print_output(
            json_mode,
            &tunnel_keys_snapshot_json(snapshot),
            &format_tunnel_keys(snapshot),
        ),
        Command::Tunnel {
            command: TunnelCommand::Driver,
        } => {
            let report = detect_tunnel_driver()?;
            print_output(
                json_mode,
                &tunnel_driver_report_json(&report),
                &format_tunnel_driver_report(&report),
            )
        }
        Command::Platform {
            command: PlatformCommand::Detect,
        } => {
            let report = detect_platform_report()?;
            print_output(
                json_mode,
                &platform_report_json(&report),
                &format_platform_report(&report),
            )
        }
        Command::Platform {
            command: PlatformCommand::Plan,
        } => {
            let plan = build_platform_install_plan()?;
            print_output(
                json_mode,
                &platform_install_plan_json(&plan),
                &format_platform_install_plan(&plan),
            )
        }
        Command::Doctor => {
            let report = doctor_report(state_path)?;
            print_output(
                json_mode,
                &doctor_report_json(&report),
                &format_doctor_report(&report),
            )
        }
        Command::BootstrapStatus => print_output(
            json_mode,
            &bootstrap_status_snapshot_json(snapshot),
            &format_bootstrap_status(snapshot),
        ),
        Command::ProbeLast => print_output(
            json_mode,
            &probe_last_snapshot_json(snapshot),
            &format_last_probe(snapshot),
        ),
        Command::ErrorLast => print_output(
            json_mode,
            &last_error_snapshot_json(snapshot),
            &format_last_error(snapshot),
        ),
        Command::Control { command } => match command {
            ControlCommand::Status => print_output(
                json_mode,
                &control_status_snapshot_json(snapshot),
                &format_control_status(snapshot),
            ),
            ControlCommand::Sync => Err("control sync requires mutable local state".to_string()),
        },
        _ => Err("unsupported local-only command".to_string()),
    }
}

fn run_control_command(
    command: &ControlCommand,
    snapshot: &mut AppCoreSnapshot,
) -> Result<(Value, String), String> {
    match command {
        ControlCommand::Status => Ok((
            control_status_snapshot_json(snapshot),
            format_control_status(snapshot),
        )),
        ControlCommand::Sync => {
            let config = control_mqtt_config_from_snapshot(snapshot)?;
            let mut client = ControlMqttClient::connect(&config)?;
            let bootstrap = client.bootstrap_session(&config)?;
            if let Some(current_bootstrap) = snapshot.current_bootstrap.as_mut() {
                current_bootstrap.network_map = Some(bootstrap.network_map.clone());
            }
            snapshot.current_network_id = Some(bootstrap.network_map.network_id.clone());
            Ok((
                control_sync_json(&bootstrap),
                format_control_sync(&bootstrap),
            ))
        }
    }
}

fn execute_command(
    command: &Command,
    facade: &CliFacade,
    snapshot: &AppCoreSnapshot,
) -> Result<(Value, String), String> {
    match command {
        Command::Auth {
            command: AuthCommand::Register(args),
        } => {
            let session = facade.register(args.email.clone(), args.password.clone())?;
            Ok((
                serde_json::to_value(&session).map_err(|err| err.to_string())?,
                format!(
                    "registered session for {} with access token {}",
                    session.user_id, session.access_token
                ),
            ))
        }
        Command::Auth {
            command: AuthCommand::Login(args),
        } => {
            let session = facade.login(args.email.clone(), args.password.clone())?;
            Ok((
                serde_json::to_value(&session).map_err(|err| err.to_string())?,
                format!(
                    "logged in as {} with access token {}",
                    session.user_id, session.access_token
                ),
            ))
        }
        Command::Device {
            command: DeviceCommand::Register(args),
        } => {
            let device = facade.register_device(
                args.name.clone(),
                args.platform.clone(),
                args.machine_id.clone(),
                args.public_key.clone(),
            )?;
            Ok((
                serde_json::to_value(&device).map_err(|err| err.to_string())?,
                format!(
                    "registered device {} ({}) on {}",
                    device.device_id, device.name, device.platform
                ),
            ))
        }
        Command::Node {
            command: NodeCommand::Register(args),
        } => {
            let device_id = args
                .device_id
                .clone()
                .or_else(|| {
                    snapshot
                        .current_device
                        .as_ref()
                        .map(|device| device.device_id.clone())
                })
                .ok_or_else(|| {
                    "missing device id; pass --device-id or register a device first".to_string()
                })?;
            let node = facade.register_node(
                device_id,
                args.node_id.clone(),
                args.node_public_key.clone(),
                args.capabilities.clone(),
            )?;
            Ok((
                serde_json::to_value(&node).map_err(|err| err.to_string())?,
                format!(
                    "registered node {} for device {}",
                    node.node_id, node.device_id
                ),
            ))
        }
        Command::Network {
            command: NetworkCommand::List,
        } => {
            let networks = facade.list_networks()?;
            Ok((json!({ "items": networks }), format_network_list(&networks)))
        }
        Command::Network {
            command: NetworkCommand::Create(args),
        } => {
            let network = facade.create_network(
                args.name.clone(),
                Some(args.cidr.clone()),
                args.allocation_start_ip.clone(),
                args.allocation_end_ip.clone(),
            )?;
            Ok((
                serde_json::to_value(&network).map_err(|err| err.to_string())?,
                format!(
                    "created network {} ({}) {}",
                    network.network_id, network.name, network.cidr
                ),
            ))
        }
        Command::Network {
            command: NetworkCommand::Join(args),
        } => {
            let device_id = resolve_device_id(args.device_id.clone(), snapshot)?;
            let joined = facade.join_network(args.network_id.clone(), device_id)?;
            Ok((
                serde_json::to_value(&joined).map_err(|err| err.to_string())?,
                format_network_join("joined network", &joined),
            ))
        }
        Command::Network {
            command: NetworkCommand::JoinByKey(args),
        } => {
            let device_id = resolve_device_id(args.device_id.clone(), snapshot)?;
            let joined = facade.join_network_by_key(args.join_key.clone(), device_id)?;
            maybe_update_join_alias(facade, &joined, args.alias.as_deref())?;
            Ok((
                serde_json::to_value(&joined).map_err(|err| err.to_string())?,
                format_network_join("joined network by key", &joined),
            ))
        }
        Command::Network {
            command: NetworkCommand::Remark(args),
        } => {
            let assignment = facade.update_attachment_remark(
                args.network_id.clone(),
                args.attachment_id.clone(),
                args.remark.clone(),
            )?;
            Ok((
                serde_json::to_value(&assignment).map_err(|err| err.to_string())?,
                "updated attachment remark".to_string(),
            ))
        }
        Command::Network {
            command: NetworkCommand::Activate(args),
        } => {
            let device_id = resolve_device_id(args.device_id.clone(), snapshot)?;
            let joined = facade.activate_network(args.network_id.clone(), device_id)?;
            Ok((
                serde_json::to_value(&joined).map_err(|err| err.to_string())?,
                format_network_join("activated network", &joined),
            ))
        }
        Command::Network {
            command: NetworkCommand::Switch(args),
        } => {
            let device_id = resolve_device_id(args.device_id.clone(), snapshot)?;
            let joined = facade.switch_network(args.network_id.clone(), device_id)?;
            Ok((
                serde_json::to_value(&joined).map_err(|err| err.to_string())?,
                format_network_join("switched network", &joined),
            ))
        }
        Command::Network {
            command: NetworkCommand::Deactivate(args),
        } => {
            let device_id = resolve_device_id(args.device_id.clone(), snapshot)?;
            facade.deactivate_network(args.network_id.clone(), device_id)?;
            Ok((
                json!({
                    "status": "deactivated",
                    "networkId": args.network_id,
                }),
                "deactivated network".to_string(),
            ))
        }
        Command::Bootstrap(args) => {
            let node_id = resolve_node_id(args.node_id.clone(), snapshot)?;
            let bootstrap = facade.bootstrap(node_id.clone(), args.network_id.clone())?;
            Ok((
                serde_json::to_value(&bootstrap).map_err(|err| err.to_string())?,
                format!(
                    "loaded bootstrap for node {} on network {}; relay cluster {}",
                    node_id, args.network_id, bootstrap.relay.default_cluster_id
                ),
            ))
        }
        Command::RelayTicket(args) => {
            let src_node_id = resolve_node_id(args.src_node_id.clone(), snapshot)?;
            let ticket = facade.issue_relay_ticket(
                args.network_id.clone(),
                src_node_id.clone(),
                args.dst_node_id.clone(),
                args.derp_cluster_id.clone(),
                args.preferred_derp_node_ids.clone(),
                args.reason.clone(),
                args.relay_region_id.clone(),
            )?;
            Ok((
                serde_json::to_value(&ticket).map_err(|err| err.to_string())?,
                format!(
                    "issued relay ticket {} for {} -> {} on {}",
                    ticket.ticket_id, src_node_id, args.dst_node_id, args.network_id
                ),
            ))
        }
        Command::Connect(args) => {
            let network_id = resolve_network_id(args.network_id.clone(), snapshot)?;
            let state = facade.connect(network_id.clone(), args.peer_node_id.clone())?;
            Ok((
                connection_state_json(&state),
                format!(
                    "connect {} via {} on {}",
                    args.peer_node_id,
                    describe_connection_state(&state),
                    network_id
                ),
            ))
        }
        Command::Send(args) => {
            let network_id = resolve_network_id(args.network_id.clone(), snapshot)?;
            let state = facade.connect(network_id.clone(), args.peer_node_id.clone())?;
            let bytes_sent = facade
                .send(args.payload.clone().into_bytes())
                .map_err(|error| error.message)?;
            Ok((
                json!({
                    "status": "sent",
                    "bytesSent": bytes_sent,
                    "path": describe_connection_state(&state),
                }),
                format!(
                    "sent {} bytes to {} via {} on {}",
                    bytes_sent,
                    args.peer_node_id,
                    describe_connection_state(&state),
                    network_id
                ),
            ))
        }
        Command::Probe(args) => {
            let network_id = resolve_network_id(args.network_id.clone(), snapshot)?;
            facade.connect(network_id.clone(), args.peer_node_id.clone())?;
            let probe = facade
                .probe_with_timeout(args.payload.clone().into_bytes(), args.probe_timeout_ms)
                .map_err(|error| error.message)?;
            Ok((
                serde_json::to_value(&probe).map_err(|err| err.to_string())?,
                format_probe(&args.peer_node_id, &network_id, &probe),
            ))
        }
        Command::Disconnect => {
            facade.disconnect()?;
            Ok((
                json!({ "status": "disconnected" }),
                "disconnected".to_string(),
            ))
        }
        Command::Connection {
            command: ConnectionCommand::Status,
        } => Ok((
            connection_status_snapshot_json(snapshot),
            format_connection_status(snapshot),
        )),
        Command::Connection {
            command: ConnectionCommand::Path,
        } => Ok((
            connection_path_snapshot_json(snapshot),
            format_connection_path(snapshot),
        )),
        Command::Tunnel {
            command: TunnelCommand::Status,
        } => Ok((
            tunnel_status_snapshot_json(snapshot),
            format_tunnel_status(snapshot),
        )),
        Command::Tunnel {
            command: TunnelCommand::Keys,
        } => Ok((
            tunnel_keys_snapshot_json(snapshot),
            format_tunnel_keys(snapshot),
        )),
        Command::Tunnel {
            command: TunnelCommand::Driver,
        } => {
            let report = detect_tunnel_driver()?;
            Ok((
                tunnel_driver_report_json(&report),
                format_tunnel_driver_report(&report),
            ))
        }
        Command::Platform { .. } => Err("unsupported backend platform command".to_string()),
        Command::Doctor => Err("unsupported backend doctor command".to_string()),
        Command::BootstrapStatus => Ok((
            bootstrap_status_snapshot_json(snapshot),
            format_bootstrap_status(snapshot),
        )),
        Command::ProbeLast => Ok((
            probe_last_snapshot_json(snapshot),
            format_last_probe(snapshot),
        )),
        Command::ErrorLast => Ok((
            last_error_snapshot_json(snapshot),
            format_last_error(snapshot),
        )),
        Command::Control { command } => match command {
            ControlCommand::Status => Ok((
                control_status_snapshot_json(snapshot),
                format_control_status(snapshot),
            )),
            ControlCommand::Sync => {
                Err("control sync is handled before backend facade initialization".to_string())
            }
        },
        Command::Status => Ok((
            serde_json::to_value(snapshot).map_err(|err| err.to_string())?,
            format_state(snapshot),
        )),
        Command::State { .. } => Err("unsupported backend state command".to_string()),
    }
}

fn connection_state_json(state: &ConnectionState) -> Value {
    match state {
        ConnectionState::Disconnected => json!({ "status": "disconnected" }),
        ConnectionState::Connecting => json!({ "status": "connecting" }),
        ConnectionState::Connected(path) => json!({
            "status": "connected",
            "path": match path {
                ConnectionPath::P2P => "p2p",
                ConnectionPath::Relay => "relay",
                ConnectionPath::Derp => "derp",
            },
        }),
        ConnectionState::Failed(reason) => json!({
            "status": "failed",
            "reason": reason,
        }),
    }
}

fn describe_connection_state(state: &ConnectionState) -> String {
    match state {
        ConnectionState::Disconnected => "disconnected".to_string(),
        ConnectionState::Connecting => "connecting".to_string(),
        ConnectionState::Connected(ConnectionPath::P2P) => "p2p".to_string(),
        ConnectionState::Connected(ConnectionPath::Relay) => "relay".to_string(),
        ConnectionState::Connected(ConnectionPath::Derp) => "derp".to_string(),
        ConnectionState::Failed(reason) => format!("failed ({reason})"),
    }
}

fn format_network_list(networks: &[slan_app_core::Network]) -> String {
    if networks.is_empty() {
        return "no networks".to_string();
    }
    networks
        .iter()
        .map(|network| format!("{} {} {}", network.network_id, network.name, network.cidr))
        .collect::<Vec<_>>()
        .join("\n")
}

fn format_network_join(action: &str, joined: &NetworkJoinResult) -> String {
    format!(
        "{} {} for device {} attachment {} ip {}",
        action,
        joined.network_id,
        joined.device_id,
        joined.attachment_id.as_deref().unwrap_or("none"),
        joined.virtual_ip.as_deref().unwrap_or("none")
    )
}

fn maybe_update_join_alias(
    facade: &CliFacade,
    joined: &NetworkJoinResult,
    alias: Option<&str>,
) -> Result<(), String> {
    let Some(alias) = alias.map(str::trim).filter(|value| !value.is_empty()) else {
        return Ok(());
    };
    let Some(attachment_id) = joined.attachment_id.as_ref() else {
        return Ok(());
    };
    let _ = facade.update_attachment_remark(
        joined.network_id.clone(),
        attachment_id.clone(),
        Some(alias.to_string()),
    )?;
    Ok(())
}

fn format_state(snapshot: &AppCoreSnapshot) -> String {
    let mut lines = vec![
        format!(
            "session: {}",
            snapshot
                .session
                .as_ref()
                .map(|session| session.user_id.as_str())
                .unwrap_or("none")
        ),
        format!(
            "device: {}",
            snapshot
                .current_device
                .as_ref()
                .map(|device| device.device_id.as_str())
                .unwrap_or("none")
        ),
        format!(
            "node: {}",
            snapshot
                .current_node
                .as_ref()
                .map(|node| node.node_id.as_str())
                .unwrap_or("none")
        ),
        format!(
            "network: {}",
            snapshot.current_network_id.as_deref().unwrap_or("none")
        ),
        format!(
            "connection: {}",
            snapshot
                .connection_state
                .as_ref()
                .map(describe_connection_state)
                .unwrap_or_else(|| "none".to_string())
        ),
        format!("connect plans: {}", snapshot.current_connect_plans.len()),
    ];
    if let Some(tunnel) = &snapshot.tunnel_runtime {
        let transport = match tunnel.transport {
            slan_app_core::TunnelTransport::P2P => "p2p",
            slan_app_core::TunnelTransport::Relay => "relay",
            slan_app_core::TunnelTransport::Derp => "derp",
        };
        lines.push(format!(
            "tunnel: configured via {} peer {} endpoint {}",
            transport,
            tunnel.peer_virtual_ip,
            tunnel.selected_endpoint.as_deref().unwrap_or("none")
        ));
    } else {
        lines.push("tunnel: none".to_string());
    }
    if let Some(error) = &snapshot.last_data_plane_error {
        lines.push(format!(
            "last data plane error: {} {}{}",
            error.code.as_str(),
            error.message,
            format_data_plane_error_hint(error.code)
        ));
    }
    if let Some(probe) = &snapshot.last_probe {
        lines.push(format!(
            "last probe: {} {} bytes via {} at {}",
            probe.probe_id,
            probe.bytes_sent,
            match &probe.active_path {
                slan_app_core::ActivePath::None => "none",
                slan_app_core::ActivePath::P2P { .. } => "p2p",
                slan_app_core::ActivePath::Relay { .. } => "relay",
                slan_app_core::ActivePath::Derp { .. } => "derp",
            },
            probe.sampled_at_ms
        ));
    }
    if !snapshot.current_connect_plans.is_empty() {
        let mut plan_lines = snapshot
            .current_connect_plans
            .iter()
            .map(|(peer_node_id, plan)| {
                let preferred_path = plan
                    .paths
                    .first()
                    .map(|path| format!("{} {}", path.path_type, path.endpoint))
                    .unwrap_or_else(|| "none".to_string());
                let relay = if let Some(ticket) = &plan.relay_ticket {
                    format!(
                        "ticket {} cluster {}",
                        ticket.ticket_id,
                        ticket.derp_cluster_id.as_deref().unwrap_or("none")
                    )
                } else if !plan.derp_cluster_id.trim().is_empty() {
                    format!(
                        "cluster {} nodes {}",
                        plan.derp_cluster_id,
                        plan.preferred_derp_node_ids.len()
                    )
                } else {
                    "none".to_string()
                };
                format!(
                    "connect-plan {} direct {} relay {}",
                    peer_node_id, preferred_path, relay
                )
            })
            .collect::<Vec<_>>();
        plan_lines.sort();
        lines.extend(plan_lines);
    }
    lines.join("\n")
}

fn format_probe(peer_node_id: &str, network_id: &str, probe: &DataPlaneProbe) -> String {
    let path = match &probe.active_path {
        slan_app_core::ActivePath::None => "none".to_string(),
        slan_app_core::ActivePath::P2P { .. } => "p2p".to_string(),
        slan_app_core::ActivePath::Relay { .. } => "relay".to_string(),
        slan_app_core::ActivePath::Derp { .. } => "derp".to_string(),
    };
    let tunnel = probe.tunnel_peer_virtual_ip.as_deref().unwrap_or("none");
    let reply = match (
        probe.reply_observed,
        probe.reply_bytes_received,
        probe.reply_rtt_ms,
    ) {
        (true, Some(bytes), Some(rtt_ms)) => {
            format!("; reply observed {} bytes after {} ms", bytes, rtt_ms)
        }
        (true, Some(bytes), None) => format!("; reply observed {} bytes", bytes),
        (true, None, Some(rtt_ms)) => format!("; reply observed after {} ms", rtt_ms),
        (true, None, None) => "; reply observed".to_string(),
        (false, _, _) => "; no reply observed".to_string(),
    };
    let observation = match (
        probe.observed_rtt_ms,
        probe.packet_loss_ppm,
        probe.path_score,
        probe.derp_node_id.as_deref(),
    ) {
        (Some(rtt), Some(loss), Some(score), Some(node_id)) => {
            format!("; observed rtt {rtt} ms loss {loss} ppm score {score} node {node_id}")
        }
        (Some(rtt), Some(loss), Some(score), None) => {
            format!("; observed rtt {rtt} ms loss {loss} ppm score {score}")
        }
        _ => String::new(),
    };
    format!(
        "probed {} on {} via {}; sent {} bytes; tunnel {}{}{}",
        peer_node_id, network_id, path, probe.bytes_sent, tunnel, reply, observation
    )
}

fn connection_status_snapshot_json(snapshot: &AppCoreSnapshot) -> Value {
    match snapshot.connection_state.as_ref() {
        Some(state) => connection_state_json(state),
        None => json!({ "status": "none" }),
    }
}

fn active_path_json(path: &slan_app_core::ActivePath) -> Value {
    match path {
        slan_app_core::ActivePath::None => json!({ "path": "none" }),
        slan_app_core::ActivePath::P2P { peer_node_id } => json!({
            "path": "p2p",
            "peerNodeId": peer_node_id,
        }),
        slan_app_core::ActivePath::Relay { peer_node_id } => json!({
            "path": "relay",
            "peerNodeId": peer_node_id,
        }),
        slan_app_core::ActivePath::Derp {
            cluster_id,
            node_id,
        } => json!({
            "path": "derp",
            "clusterId": cluster_id,
            "nodeId": node_id,
        }),
    }
}

fn describe_active_path(path: &slan_app_core::ActivePath) -> String {
    match path {
        slan_app_core::ActivePath::None => "none".to_string(),
        slan_app_core::ActivePath::P2P { peer_node_id } => format!("p2p peer {peer_node_id}"),
        slan_app_core::ActivePath::Relay { peer_node_id } => format!("relay peer {peer_node_id}"),
        slan_app_core::ActivePath::Derp {
            cluster_id,
            node_id,
        } => format!("derp cluster {cluster_id} node {node_id}"),
    }
}

fn connection_path_snapshot_json(snapshot: &AppCoreSnapshot) -> Value {
    match snapshot.active_path.as_ref() {
        Some(path) => active_path_json(path),
        None => json!({ "path": "none" }),
    }
}

fn tunnel_status_snapshot_json(snapshot: &AppCoreSnapshot) -> Value {
    match snapshot.tunnel_runtime.as_ref() {
        Some(runtime) => {
            serde_json::to_value(runtime).unwrap_or_else(|_| json!({ "status": "invalid" }))
        }
        None => json!({ "status": "none" }),
    }
}

fn format_connection_status(snapshot: &AppCoreSnapshot) -> String {
    match snapshot.connection_state.as_ref() {
        Some(state) => format!("connection {}", describe_connection_state(state)),
        None => "connection none".to_string(),
    }
}

fn format_connection_path(snapshot: &AppCoreSnapshot) -> String {
    match snapshot.active_path.as_ref() {
        Some(path) => format!("connection path {}", describe_active_path(path)),
        None => "connection path none".to_string(),
    }
}

fn format_tunnel_status(snapshot: &AppCoreSnapshot) -> String {
    match snapshot.tunnel_runtime.as_ref() {
        Some(runtime) => format!(
            "tunnel {:?} via {:?} peer {} endpoint {}",
            runtime.state,
            runtime.transport,
            runtime.peer_virtual_ip,
            runtime.selected_endpoint.as_deref().unwrap_or("none")
        ),
        None => "tunnel none".to_string(),
    }
}

fn tunnel_keys_snapshot_json(snapshot: &AppCoreSnapshot) -> Value {
    match snapshot.tunnel_key_material.as_ref() {
        Some(material) => json!({
            "status": "present",
            "deviceId": material.device_id,
            "nodeId": material.node_id,
            "publicKey": material.key_pair.public_key,
            "createdAtMs": material.created_at_ms,
        }),
        None => json!({ "status": "none" }),
    }
}

fn bootstrap_status_snapshot_json(snapshot: &AppCoreSnapshot) -> Value {
    match snapshot.current_bootstrap.as_ref() {
        Some(bootstrap) => {
            serde_json::to_value(bootstrap).unwrap_or_else(|_| json!({ "status": "invalid" }))
        }
        None => json!({ "status": "none" }),
    }
}

fn control_status_snapshot_json(snapshot: &AppCoreSnapshot) -> Value {
    match snapshot.current_bootstrap.as_ref() {
        Some(bootstrap) => {
            let network_map = bootstrap.network_map.as_ref();
            json!({
                "status": "configured",
                "wsUrl": bootstrap.control_plane.ws_url,
                "heartbeatSeconds": bootstrap.control_plane.heartbeat_seconds,
                "sessionTokenPresent": bootstrap.control_plane.session_token.is_some(),
                "networkMapPresent": network_map.is_some(),
                "networkId": snapshot.current_network_id.as_deref().or(network_map.map(|map| map.network_id.as_str())),
                "nodeId": snapshot.current_node.as_ref().map(|node| node.node_id.as_str()).or(network_map.map(|map| map.self_node_id.as_str())),
                "deviceId": snapshot.current_device.as_ref().map(|device| device.device_id.as_str()).or(network_map.map(|map| map.self_device_id.as_str())),
                "peerCount": network_map.map(|map| map.peers.len()).unwrap_or(0),
                "connectPlanCount": snapshot.current_connect_plans.len(),
                "connectPlans": snapshot.current_connect_plans.iter().map(|(peer_node_id, plan)| {
                    json!({
                        "peerNodeId": peer_node_id,
                        "preferDirect": plan.prefer_direct,
                        "pathCount": plan.paths.len(),
                        "preferredPath": plan.paths.first().map(|path| json!({
                            "pathType": path.path_type,
                            "endpoint": path.endpoint,
                            "priority": path.priority,
                        })),
                        "derpClusterId": if plan.derp_cluster_id.trim().is_empty() { None::<String> } else { Some(plan.derp_cluster_id.clone()) },
                        "preferredDerpNodeIds": plan.preferred_derp_node_ids,
                        "relayTicketId": plan.relay_ticket.as_ref().map(|ticket| ticket.ticket_id.clone()),
                    })
                }).collect::<Vec<_>>(),
            })
        }
        None => json!({ "status": "none" }),
    }
}

fn control_sync_json(bootstrap: &ControlSessionBootstrap) -> Value {
    json!({
        "status": "synced",
        "controlSessionId": bootstrap.ack.control_session_id,
        "heartbeatSeconds": bootstrap.ack.heartbeat_seconds,
        "networkRevision": bootstrap.ack.network_revision,
        "networkId": bootstrap.network_map.network_id,
        "peerCount": bootstrap.network_map.peers.len(),
    })
}

fn probe_last_snapshot_json(snapshot: &AppCoreSnapshot) -> Value {
    match snapshot.last_probe.as_ref() {
        Some(probe) => {
            serde_json::to_value(probe).unwrap_or_else(|_| json!({ "status": "invalid" }))
        }
        None => json!({ "status": "none" }),
    }
}

fn last_error_snapshot_json(snapshot: &AppCoreSnapshot) -> Value {
    match snapshot.last_data_plane_error.as_ref() {
        Some(error) => json!({
            "code": error.code,
            "message": error.message,
            "hint": data_plane_error_hint(error.code),
        }),
        None => json!({ "status": "none" }),
    }
}

fn format_bootstrap_status(snapshot: &AppCoreSnapshot) -> String {
    match snapshot.current_bootstrap.as_ref() {
        Some(bootstrap) => format!(
            "bootstrap node {} network {} relay {} peers {} control-session {}",
            bootstrap
                .network_map
                .as_ref()
                .map(|map| map.self_node_id.as_str())
                .unwrap_or("none"),
            snapshot.current_network_id.as_deref().unwrap_or("none"),
            bootstrap.relay.default_cluster_id,
            bootstrap
                .network_map
                .as_ref()
                .map(|map| map.peers.len())
                .unwrap_or(0),
            bootstrap
                .control_plane
                .session_token
                .as_deref()
                .map(|_| "present")
                .unwrap_or("missing")
        ),
        None => "bootstrap none".to_string(),
    }
}

fn format_control_status(snapshot: &AppCoreSnapshot) -> String {
    match snapshot.current_bootstrap.as_ref() {
        Some(bootstrap) => {
            let network_map = bootstrap.network_map.as_ref();
            let mut summary = format!(
                "control mqtt {} heartbeat {} token {} network-map {} peers {}",
                bootstrap.control_plane.ws_url,
                bootstrap.control_plane.heartbeat_seconds,
                bootstrap
                    .control_plane
                    .session_token
                    .as_deref()
                    .map(|_| "present")
                    .unwrap_or("missing"),
                if network_map.is_some() {
                    "present"
                } else {
                    "missing"
                },
                network_map.map(|map| map.peers.len()).unwrap_or(0),
            );
            if !snapshot.current_connect_plans.is_empty() {
                let mut plan_summaries = snapshot
                    .current_connect_plans
                    .iter()
                    .map(|(peer_node_id, plan)| {
                        let preferred_path = plan
                            .paths
                            .first()
                            .map(|path| format!("{} {}", path.path_type, path.endpoint))
                            .unwrap_or_else(|| "none".to_string());
                        format!("{peer_node_id}=>{preferred_path}")
                    })
                    .collect::<Vec<_>>();
                plan_summaries.sort();
                summary.push_str(&format!(
                    "; connect-plans {} [{}]",
                    snapshot.current_connect_plans.len(),
                    plan_summaries.join(", ")
                ));
            }
            summary
        }
        None => "control none".to_string(),
    }
}

fn format_control_sync(bootstrap: &ControlSessionBootstrap) -> String {
    format!(
        "control synced session {} heartbeat {} revision {} peers {}",
        bootstrap.ack.control_session_id,
        bootstrap.ack.heartbeat_seconds,
        bootstrap.ack.network_revision,
        bootstrap.network_map.peers.len(),
    )
}

fn format_last_probe(snapshot: &AppCoreSnapshot) -> String {
    match snapshot.last_probe.as_ref() {
        Some(probe) => {
            let peer = probe.tunnel_peer_virtual_ip.as_deref().unwrap_or("none");
            let path = match &probe.active_path {
                slan_app_core::ActivePath::None => "none".to_string(),
                slan_app_core::ActivePath::P2P { .. } => "p2p".to_string(),
                slan_app_core::ActivePath::Relay { .. } => "relay".to_string(),
                slan_app_core::ActivePath::Derp { .. } => "derp".to_string(),
            };
            format!(
                "last probe {} via {} sent {} bytes tunnel {}",
                probe.probe_id, path, probe.bytes_sent, peer
            )
        }
        None => "probe none".to_string(),
    }
}

fn format_last_error(snapshot: &AppCoreSnapshot) -> String {
    match snapshot.last_data_plane_error.as_ref() {
        Some(error) => format!(
            "last data plane error {} {}{}",
            error.code.as_str(),
            error.message,
            format_data_plane_error_hint(error.code)
        ),
        None => "last data plane error none".to_string(),
    }
}

fn data_plane_error_hint(code: DataPlaneErrorCode) -> &'static str {
    match code {
        DataPlaneErrorCode::Timeout => "probe reply did not arrive before deadline",
        DataPlaneErrorCode::Transport => "socket or transport path failed",
        DataPlaneErrorCode::RelayAuth => {
            "relay ticket was rejected; refresh bootstrap/relay ticket and retry"
        }
        DataPlaneErrorCode::RelaySession => {
            "relay session state is stale or mismatched; reconnect before retrying"
        }
        DataPlaneErrorCode::RelayProtocol => {
            "relay daemon protocol state is inconsistent; inspect relay logs"
        }
        DataPlaneErrorCode::UnsupportedPath => {
            "current active path does not support this operation"
        }
        DataPlaneErrorCode::Unknown => "unclassified data plane failure",
    }
}

fn format_data_plane_error_hint(code: DataPlaneErrorCode) -> String {
    format!("; {}", data_plane_error_hint(code))
}

fn format_tunnel_keys(snapshot: &AppCoreSnapshot) -> String {
    match snapshot.tunnel_key_material.as_ref() {
        Some(material) => format!(
            "tunnel keys public {} device {} node {} created {}",
            material.key_pair.public_key,
            material.device_id.as_deref().unwrap_or("none"),
            material.node_id.as_deref().unwrap_or("none"),
            material.created_at_ms
        ),
        None => "tunnel keys none".to_string(),
    }
}

fn control_mqtt_config_from_snapshot(
    snapshot: &AppCoreSnapshot,
) -> Result<ControlMqttConfig, String> {
    let bootstrap = snapshot
        .current_bootstrap
        .as_ref()
        .ok_or_else(|| "missing bootstrap; run bootstrap first".to_string())?;
    let network_map = bootstrap.network_map.as_ref().ok_or_else(|| {
        "missing bootstrap network map; bootstrap must include network map".to_string()
    })?;
    let session_token = bootstrap
        .control_plane
        .session_token
        .clone()
        .ok_or_else(|| "missing control session token in bootstrap config".to_string())?;
    let node = snapshot
        .current_node
        .as_ref()
        .ok_or_else(|| "missing current node; register a node first".to_string())?;
    let user_id = snapshot
        .session
        .as_ref()
        .map(|session| session.user_id.clone())
        .unwrap_or_else(|| network_map.self_user_id.clone());
    let access_token = snapshot
        .session
        .as_ref()
        .map(|session| session.access_token.clone())
        .ok_or_else(|| "missing session access token".to_string())?;
    let device_id = snapshot
        .current_device
        .as_ref()
        .map(|device| device.device_id.clone())
        .unwrap_or_else(|| network_map.self_device_id.clone());
    let mqtt = snapshot
        .current_device
        .as_ref()
        .and_then(|device| device.mqtt.clone())
        .ok_or_else(|| "missing MQTT credential in current device".to_string())?;

    Ok(ControlMqttConfig {
        access_token,
        session_token,
        user_id,
        device_id,
        node_id: node.node_id.clone(),
        node_public_key: node.node_public_key.clone(),
        network_id: snapshot
            .current_network_id
            .clone()
            .unwrap_or_else(|| network_map.network_id.clone()),
        capabilities: node.capabilities.clone(),
        mqtt,
    })
}

fn tunnel_driver_report_json(report: &TunnelDriverSelection) -> Value {
    json!({
        "requested": match report.requested {
            TunnelDriverKind::Auto => "auto",
            TunnelDriverKind::InMemory => "in-memory",
            TunnelDriverKind::LinuxKernel => "linux-kernel",
            TunnelDriverKind::WindowsEmbeddable => "windows-embeddable",
        },
        "selected": match report.selected {
            TunnelDriverKind::Auto => "auto",
            TunnelDriverKind::InMemory => "in-memory",
            TunnelDriverKind::LinuxKernel => "linux-kernel",
            TunnelDriverKind::WindowsEmbeddable => "windows-embeddable",
        },
        "executionMode": report.execution_mode,
        "executionBackend": report.execution_backend,
        "platform": report.platform,
    })
}

fn format_tunnel_driver_report(report: &TunnelDriverSelection) -> String {
    let requested = match report.requested {
        TunnelDriverKind::Auto => "auto",
        TunnelDriverKind::InMemory => "in-memory",
        TunnelDriverKind::LinuxKernel => "linux-kernel",
        TunnelDriverKind::WindowsEmbeddable => "windows-embeddable",
    };
    let selected = match report.selected {
        TunnelDriverKind::Auto => "auto",
        TunnelDriverKind::InMemory => "in-memory",
        TunnelDriverKind::LinuxKernel => "linux-kernel",
        TunnelDriverKind::WindowsEmbeddable => "windows-embeddable",
    };
    format!(
        "tunnel driver selected {} (requested {}, mode {}, backend {}, platform {})",
        selected, requested, report.execution_mode, report.execution_backend, report.platform
    )
}

#[derive(Debug, Clone, PartialEq, Eq)]
enum LinuxDistroFamily {
    Debian,
    Rhel,
    Arch,
    Unknown,
}

#[derive(Debug, Clone, PartialEq, Eq)]
struct PlatformReport {
    os: String,
    distro_id: Option<String>,
    version_id: Option<String>,
    id_like: Vec<String>,
    family: Option<LinuxDistroFamily>,
    kernel_release: Option<String>,
    package_manager: Option<String>,
}

#[derive(Debug, Clone, PartialEq, Eq)]
struct PlatformInstallPlan {
    report: PlatformReport,
    packages: Vec<String>,
    supported_driver_modes: Vec<String>,
    warnings: Vec<String>,
}

#[derive(Debug, Clone, PartialEq, Eq)]
struct DoctorCheck {
    name: String,
    status: &'static str,
    detail: String,
}

#[derive(Debug, Clone, PartialEq, Eq)]
struct DoctorReport {
    platform: PlatformReport,
    tunnel_driver: TunnelDriverSelection,
    checks: Vec<DoctorCheck>,
}

fn parse_os_release(contents: &str) -> std::collections::BTreeMap<String, String> {
    contents
        .lines()
        .filter_map(|line| {
            let line = line.trim();
            if line.is_empty() || line.starts_with('#') {
                return None;
            }
            let (key, value) = line.split_once('=')?;
            let normalized = value.trim_matches('"').to_string();
            Some((key.trim().to_string(), normalized))
        })
        .collect()
}

fn linux_family(id: Option<&str>, id_like: &[String]) -> LinuxDistroFamily {
    let mut values = id_like.iter().map(String::as_str).collect::<Vec<_>>();
    if let Some(id) = id {
        values.push(id);
    }
    if values
        .iter()
        .any(|value| matches!(*value, "ubuntu" | "debian"))
    {
        LinuxDistroFamily::Debian
    } else if values
        .iter()
        .any(|value| matches!(*value, "rhel" | "centos" | "fedora" | "rocky" | "almalinux"))
    {
        LinuxDistroFamily::Rhel
    } else if values.iter().any(|value| *value == "arch") {
        LinuxDistroFamily::Arch
    } else {
        LinuxDistroFamily::Unknown
    }
}

fn detect_platform_report() -> Result<PlatformReport, String> {
    let os = env::consts::OS.to_string();
    let kernel_release = fs::read_to_string("/proc/sys/kernel/osrelease")
        .ok()
        .map(|value| value.trim().to_string())
        .filter(|value| !value.is_empty());
    if os != "linux" {
        return Ok(PlatformReport {
            os,
            distro_id: None,
            version_id: None,
            id_like: Vec::new(),
            family: None,
            kernel_release,
            package_manager: None,
        });
    }

    let os_release = fs::read_to_string("/etc/os-release")
        .map_err(|err| format!("failed to read /etc/os-release: {err}"))?;
    let parsed = parse_os_release(&os_release);
    let distro_id = parsed.get("ID").cloned();
    let version_id = parsed.get("VERSION_ID").cloned();
    let id_like = parsed
        .get("ID_LIKE")
        .map(|value| {
            value
                .split_whitespace()
                .map(ToString::to_string)
                .collect::<Vec<_>>()
        })
        .unwrap_or_default();
    let family = linux_family(distro_id.as_deref(), &id_like);
    let package_manager = match family {
        LinuxDistroFamily::Debian => Some("apt-get".to_string()),
        LinuxDistroFamily::Rhel => Some("dnf".to_string()),
        LinuxDistroFamily::Arch => Some("pacman".to_string()),
        LinuxDistroFamily::Unknown => None,
    };

    Ok(PlatformReport {
        os,
        distro_id,
        version_id,
        id_like,
        family: Some(family),
        kernel_release,
        package_manager,
    })
}

fn build_platform_install_plan() -> Result<PlatformInstallPlan, String> {
    let report = detect_platform_report()?;
    let mut warnings = Vec::new();
    let packages = match report.family.clone() {
        Some(LinuxDistroFamily::Debian) => {
            vec!["wireguard-tools".to_string(), "iproute2".to_string()]
        }
        Some(LinuxDistroFamily::Rhel) => {
            warnings.push("wireguard kernel packages may require extra repositories on some RHEL-family systems".to_string());
            vec!["wireguard-tools".to_string(), "iproute".to_string()]
        }
        Some(LinuxDistroFamily::Arch) => {
            vec!["wireguard-tools".to_string(), "iproute2".to_string()]
        }
        Some(LinuxDistroFamily::Unknown) | None => {
            warnings.push("unsupported or unknown linux distribution; package plan may need manual adjustments".to_string());
            Vec::new()
        }
    };
    let supported_driver_modes = if report.os == "linux" {
        vec![
            "in-memory".to_string(),
            "linux-kernel-shell".to_string(),
            "linux-kernel-native".to_string(),
        ]
    } else {
        vec!["in-memory".to_string()]
    };
    Ok(PlatformInstallPlan {
        report,
        packages,
        supported_driver_modes,
        warnings,
    })
}

fn command_on_path(command: &str) -> bool {
    env::var_os("PATH")
        .map(|paths| env::split_paths(&paths).any(|dir| dir.join(command).exists()))
        .unwrap_or(false)
}

fn doctor_report(state_path: &Path) -> Result<DoctorReport, String> {
    let platform = detect_platform_report()?;
    let tunnel_driver = detect_tunnel_driver()?;
    let mut checks = Vec::new();
    checks.push(DoctorCheck {
        name: "state_file".to_string(),
        status: "ok",
        detail: state_path.display().to_string(),
    });
    checks.push(DoctorCheck {
        name: "ip_command".to_string(),
        status: if command_on_path("ip") { "ok" } else { "fail" },
        detail: if command_on_path("ip") {
            "ip command available".to_string()
        } else {
            "ip command missing from PATH".to_string()
        },
    });
    checks.push(DoctorCheck {
        name: "wg_command".to_string(),
        status: if command_on_path("wg") { "ok" } else { "warn" },
        detail: if command_on_path("wg") {
            "wg command available".to_string()
        } else {
            "wg command missing from PATH".to_string()
        },
    });
    checks.push(DoctorCheck {
        name: "tun_device".to_string(),
        status: if Path::new("/dev/net/tun").exists() {
            "ok"
        } else {
            "warn"
        },
        detail: if Path::new("/dev/net/tun").exists() {
            "/dev/net/tun present".to_string()
        } else {
            "/dev/net/tun missing".to_string()
        },
    });
    checks.push(DoctorCheck {
        name: "tunnel_driver".to_string(),
        status: "ok",
        detail: format_tunnel_driver_report(&tunnel_driver),
    });
    Ok(DoctorReport {
        platform,
        tunnel_driver,
        checks,
    })
}

fn platform_report_json(report: &PlatformReport) -> Value {
    json!({
        "os": report.os,
        "distroId": report.distro_id,
        "versionId": report.version_id,
        "idLike": report.id_like,
        "family": report.family.as_ref().map(|family| match family {
            LinuxDistroFamily::Debian => "debian",
            LinuxDistroFamily::Rhel => "rhel",
            LinuxDistroFamily::Arch => "arch",
            LinuxDistroFamily::Unknown => "unknown",
        }),
        "kernelRelease": report.kernel_release,
        "packageManager": report.package_manager,
    })
}

fn platform_install_plan_json(plan: &PlatformInstallPlan) -> Value {
    json!({
        "platform": platform_report_json(&plan.report),
        "packages": plan.packages,
        "supportedDriverModes": plan.supported_driver_modes,
        "warnings": plan.warnings,
    })
}

fn doctor_report_json(report: &DoctorReport) -> Value {
    json!({
        "platform": platform_report_json(&report.platform),
        "tunnelDriver": tunnel_driver_report_json(&report.tunnel_driver),
        "checks": report.checks.iter().map(|check| json!({
            "name": check.name,
            "status": check.status,
            "detail": check.detail,
        })).collect::<Vec<_>>(),
    })
}

fn format_platform_report(report: &PlatformReport) -> String {
    let family = report
        .family
        .as_ref()
        .map(|family| match family {
            LinuxDistroFamily::Debian => "debian",
            LinuxDistroFamily::Rhel => "rhel",
            LinuxDistroFamily::Arch => "arch",
            LinuxDistroFamily::Unknown => "unknown",
        })
        .unwrap_or("n/a");
    format!(
        "platform {} distro {} version {} family {} kernel {} package-manager {}",
        report.os,
        report.distro_id.as_deref().unwrap_or("n/a"),
        report.version_id.as_deref().unwrap_or("n/a"),
        family,
        report.kernel_release.as_deref().unwrap_or("n/a"),
        report.package_manager.as_deref().unwrap_or("n/a"),
    )
}

fn format_platform_install_plan(plan: &PlatformInstallPlan) -> String {
    let mut lines = vec![format_platform_report(&plan.report)];
    lines.push(format!(
        "packages: {}",
        if plan.packages.is_empty() {
            "none".to_string()
        } else {
            plan.packages.join(", ")
        }
    ));
    lines.push(format!(
        "supported drivers: {}",
        plan.supported_driver_modes.join(", ")
    ));
    if !plan.warnings.is_empty() {
        lines.push(format!("warnings: {}", plan.warnings.join("; ")));
    }
    lines.join("\n")
}

fn format_doctor_report(report: &DoctorReport) -> String {
    let mut lines = vec![format_platform_report(&report.platform)];
    lines.push(format_tunnel_driver_report(&report.tunnel_driver));
    lines.extend(
        report
            .checks
            .iter()
            .map(|check| format!("{} [{}] {}", check.name, check.status, check.detail)),
    );
    lines.join("\n")
}

fn resolve_node_id(explicit: Option<String>, snapshot: &AppCoreSnapshot) -> Result<String, String> {
    explicit
        .or_else(|| {
            snapshot
                .current_node
                .as_ref()
                .map(|node| node.node_id.clone())
        })
        .ok_or_else(|| "missing node id; pass --node-id or register a node first".to_string())
}

fn resolve_device_id(
    explicit: Option<String>,
    snapshot: &AppCoreSnapshot,
) -> Result<String, String> {
    explicit
        .or_else(|| {
            snapshot
                .current_device
                .as_ref()
                .map(|device| device.device_id.clone())
        })
        .ok_or_else(|| "missing device id; pass --device-id or register a device first".to_string())
}

fn resolve_network_id(
    explicit: Option<String>,
    snapshot: &AppCoreSnapshot,
) -> Result<String, String> {
    explicit
        .or_else(|| snapshot.current_network_id.clone())
        .ok_or_else(|| "missing network id; pass --network-id or run bootstrap first".to_string())
}

fn print_output(json_mode: bool, value: &Value, text: &str) -> Result<(), String> {
    if json_mode {
        println!(
            "{}",
            serde_json::to_string_pretty(value).map_err(|err| err.to_string())?
        );
    } else {
        println!("{text}");
    }
    Ok(())
}

fn default_state_file() -> PathBuf {
    if let Some(home) = env::var_os("HOME").or_else(|| env::var_os("USERPROFILE")) {
        let mut path = PathBuf::from(home);
        path.push(".slan");
        path.push("app-core-cli-state.json");
        return path;
    }
    PathBuf::from(".slan-app-core-cli-state.json")
}

fn load_snapshot(path: &Path) -> Result<AppCoreSnapshot, String> {
    match fs::read(path) {
        Ok(bytes) => serde_json::from_slice(&bytes).map_err(|err| err.to_string()),
        Err(err) if err.kind() == io::ErrorKind::NotFound => Ok(AppCoreSnapshot::default()),
        Err(err) => Err(err.to_string()),
    }
}

fn save_snapshot(path: &Path, snapshot: &AppCoreSnapshot) -> Result<(), String> {
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent).map_err(|err| err.to_string())?;
    }
    let bytes = serde_json::to_vec_pretty(snapshot).map_err(|err| err.to_string())?;
    fs::write(path, bytes).map_err(|err| err.to_string())
}

fn clear_state_file(path: &Path) -> Result<(), String> {
    match fs::remove_file(path) {
        Ok(()) => Ok(()),
        Err(err) if err.kind() == io::ErrorKind::NotFound => Ok(()),
        Err(err) => Err(err.to_string()),
    }
}

fn detect_tunnel_driver() -> Result<TunnelDriverSelection, String> {
    let dry_run = env::var("SLAN_TUNNEL_DRY_RUN")
        .ok()
        .map(|value| matches!(value.as_str(), "1" | "true" | "TRUE" | "yes" | "on"))
        .unwrap_or(false);
    detect_platform_tunnel_driver(env::var("SLAN_TUNNEL_DRIVER").ok().as_deref(), dry_run)
}

fn build_tunnel_manager() -> Result<Arc<dyn TunnelManager>, String> {
    let report = detect_tunnel_driver()?;
    build_platform_tunnel_manager(&report)
}

fn build_facade(base_url: String, state_path: &Path) -> Result<CliFacade, String> {
    let derp_pool = Arc::new(InMemoryDerpPool::default());
    let relay_client = Arc::new(SocketRelayClient::default());
    let p2p_connector = Arc::new(SocketP2PConnector::default());
    let tunnel_manager = build_tunnel_manager()?;
    Ok(DefaultAppCoreFacade::new_with_tunnel_key_provider(
        HttpControllerClient::new(base_url, TcpJsonHttpTransport::default()),
        p2p_connector.clone(),
        relay_client.clone(),
        derp_pool.clone(),
        InMemoryPathManager::new(derp_pool, relay_client, p2p_connector),
        tunnel_manager,
        Box::new(FileTunnelKeyProvider::new(tunnel_key_file_path(state_path))),
    ))
}

fn tunnel_key_file_path(state_path: &Path) -> PathBuf {
    state_path.with_extension("wg-keys.json")
}

#[derive(Parser, Debug)]
#[command(name = "app-core-cli")]
#[command(about = "Headless CLI for SLAN app_core flows")]
struct Cli {
    #[arg(long, env = "SLAN_CONTROL_BASE_URL")]
    control_base_url: Option<String>,
    #[arg(long)]
    state_file: Option<PathBuf>,
    #[arg(long)]
    json: bool,
    #[command(subcommand)]
    command: Command,
}

#[derive(Subcommand, Debug)]
enum Command {
    Auth {
        #[command(subcommand)]
        command: AuthCommand,
    },
    Device {
        #[command(subcommand)]
        command: DeviceCommand,
    },
    Node {
        #[command(subcommand)]
        command: NodeCommand,
    },
    Network {
        #[command(subcommand)]
        command: NetworkCommand,
    },
    Bootstrap(BootstrapArgs),
    RelayTicket(RelayTicketArgs),
    Connect(ConnectArgs),
    Send(SendArgs),
    Probe(ProbeArgs),
    BootstrapStatus,
    ProbeLast,
    ErrorLast,
    Platform {
        #[command(subcommand)]
        command: PlatformCommand,
    },
    Doctor,
    Disconnect,
    Connection {
        #[command(subcommand)]
        command: ConnectionCommand,
    },
    Tunnel {
        #[command(subcommand)]
        command: TunnelCommand,
    },
    Control {
        #[command(subcommand)]
        command: ControlCommand,
    },
    Status,
    State {
        #[command(subcommand)]
        command: StateCommand,
    },
}

#[derive(Subcommand, Debug)]
enum AuthCommand {
    Register(AuthArgs),
    Login(AuthArgs),
}

#[derive(Subcommand, Debug)]
enum DeviceCommand {
    Register(RegisterDeviceArgs),
}

#[derive(Subcommand, Debug)]
enum NodeCommand {
    Register(RegisterNodeArgs),
}

#[derive(Subcommand, Debug)]
enum NetworkCommand {
    List,
    Create(CreateNetworkArgs),
    Join(JoinNetworkArgs),
    JoinByKey(JoinNetworkByKeyArgs),
    Remark(UpdateAttachmentRemarkArgs),
    Activate(NetworkDeviceArgs),
    Switch(NetworkDeviceArgs),
    Deactivate(NetworkDeviceArgs),
}

#[derive(Subcommand, Debug)]
enum ConnectionCommand {
    Status,
    Path,
}

#[derive(Subcommand, Debug)]
enum TunnelCommand {
    Status,
    Keys,
    Driver,
}

#[derive(Subcommand, Debug)]
enum ControlCommand {
    Status,
    Sync,
}

#[derive(Subcommand, Debug)]
enum PlatformCommand {
    Detect,
    Plan,
}

#[derive(Subcommand, Debug)]
enum StateCommand {
    Show,
    Path,
    Clear,
}

#[derive(Args, Debug)]
struct AuthArgs {
    #[arg(long)]
    email: String,
    #[arg(long)]
    password: String,
}

#[derive(Args, Debug)]
struct RegisterDeviceArgs {
    #[arg(long)]
    name: String,
    #[arg(long)]
    platform: String,
    #[arg(long)]
    machine_id: String,
    #[arg(long)]
    public_key: String,
}

#[derive(Args, Debug)]
struct RegisterNodeArgs {
    #[arg(long)]
    device_id: Option<String>,
    #[arg(long)]
    node_id: String,
    #[arg(long)]
    node_public_key: String,
    #[arg(long = "capability")]
    capabilities: Vec<String>,
}

#[derive(Args, Debug)]
struct CreateNetworkArgs {
    #[arg(long)]
    name: String,
    #[arg(long, default_value = "100.64.0.0/24")]
    cidr: String,
    #[arg(long)]
    allocation_start_ip: Option<String>,
    #[arg(long)]
    allocation_end_ip: Option<String>,
}

#[derive(Args, Debug)]
struct JoinNetworkArgs {
    #[arg(long)]
    network_id: String,
    #[arg(long)]
    device_id: Option<String>,
}

#[derive(Args, Debug)]
struct JoinNetworkByKeyArgs {
    #[arg(long)]
    join_key: String,
    #[arg(long)]
    device_id: Option<String>,
    #[arg(long)]
    alias: Option<String>,
}

#[derive(Args, Debug)]
struct UpdateAttachmentRemarkArgs {
    #[arg(long)]
    network_id: String,
    #[arg(long)]
    attachment_id: String,
    #[arg(long)]
    remark: Option<String>,
}

#[derive(Args, Debug)]
struct NetworkDeviceArgs {
    #[arg(long)]
    network_id: String,
    #[arg(long)]
    device_id: Option<String>,
}

#[derive(Args, Debug)]
struct BootstrapArgs {
    #[arg(long)]
    node_id: Option<String>,
    #[arg(long)]
    network_id: String,
}

#[derive(Args, Debug)]
struct RelayTicketArgs {
    #[arg(long)]
    network_id: String,
    #[arg(long)]
    src_node_id: Option<String>,
    #[arg(long)]
    dst_node_id: String,
    #[arg(long)]
    derp_cluster_id: Option<String>,
    #[arg(long = "preferred-derp-node-id")]
    preferred_derp_node_ids: Vec<String>,
    #[arg(long)]
    reason: String,
    #[arg(long)]
    relay_region_id: Option<String>,
}

#[derive(Args, Debug)]
struct ConnectArgs {
    #[arg(long)]
    network_id: Option<String>,
    #[arg(long)]
    peer_node_id: String,
}

#[derive(Args, Debug)]
struct SendArgs {
    #[arg(long)]
    network_id: Option<String>,
    #[arg(long)]
    peer_node_id: String,
    #[arg(long)]
    payload: String,
}

#[derive(Args, Debug)]
struct ProbeArgs {
    #[arg(long)]
    network_id: Option<String>,
    #[arg(long)]
    peer_node_id: String,
    #[arg(long)]
    payload: String,
    #[arg(long)]
    probe_timeout_ms: Option<u64>,
}

#[cfg(test)]
mod tests {
    use super::*;
    use ffi_bridge::{DataPlaneError, DataPlaneErrorCode, TunnelRuntimeView, TunnelState};
    use slan_app_core::{
        ActivePath, Device, Session, TunnelKeyMaterial, TunnelTransport, WireGuardKeyPair,
    };

    #[test]
    fn formats_state_summary() {
        let snapshot = AppCoreSnapshot {
            session: Some(Session {
                user_id: "user-1".into(),
                access_token: "token-1".into(),
                refresh_token: None,
                expires_in: 3600,
                device_id: Some("dev-1".into()),
            }),
            current_device: Some(Device {
                device_id: "dev-1".into(),
                name: "thor-mac".into(),
                platform: "macos".into(),
                status: "online".into(),
                virtual_ip: None,
                public_key: None,
                mqtt: None,
            }),
            ..AppCoreSnapshot::default()
        };

        let formatted = format_state(&snapshot);
        assert!(formatted.contains("session: user-1"));
        assert!(formatted.contains("device: dev-1"));
        assert!(formatted.contains("node: none"));
    }

    #[test]
    fn default_state_file_uses_fallback_when_home_missing() {
        let path = default_state_file();
        assert!(!path.as_os_str().is_empty());
    }

    #[test]
    fn tunnel_key_file_path_reuses_state_file_stem() {
        let path = PathBuf::from("/tmp/app-core-state.json");

        assert_eq!(
            tunnel_key_file_path(&path),
            PathBuf::from("/tmp/app-core-state.wg-keys.json")
        );
    }

    #[test]
    fn formats_connection_status_from_snapshot() {
        let snapshot = AppCoreSnapshot {
            connection_state: Some(ConnectionState::Connected(ConnectionPath::Relay)),
            ..AppCoreSnapshot::default()
        };

        assert_eq!(format_connection_status(&snapshot), "connection relay");
        assert_eq!(
            connection_status_snapshot_json(&snapshot)["status"],
            "connected"
        );
    }

    #[test]
    fn formats_connection_path_from_snapshot() {
        let snapshot = AppCoreSnapshot {
            active_path: Some(ActivePath::Relay {
                peer_node_id: "peer-1".into(),
            }),
            ..AppCoreSnapshot::default()
        };

        assert_eq!(
            format_connection_path(&snapshot),
            "connection path relay peer peer-1"
        );
        assert_eq!(connection_path_snapshot_json(&snapshot)["path"], "relay");
    }

    #[test]
    fn formats_tunnel_status_from_snapshot() {
        let snapshot = AppCoreSnapshot {
            tunnel_runtime: Some(TunnelRuntimeView {
                state: TunnelState::Configured,
                transport: TunnelTransport::Relay,
                peer_virtual_ip: "100.64.0.2".into(),
                peer_public_key: "peer-pub".into(),
                selected_endpoint: Some("203.0.113.10:51820".into()),
                interface_name: Some("utun9".into()),
            }),
            ..AppCoreSnapshot::default()
        };

        let formatted = format_tunnel_status(&snapshot);
        assert!(formatted.contains("100.64.0.2"));
        assert!(formatted.contains("203.0.113.10:51820"));
        assert_eq!(
            tunnel_status_snapshot_json(&snapshot)["peerVirtualIp"],
            "100.64.0.2"
        );
    }

    #[test]
    fn formats_bootstrap_status_from_snapshot() {
        let snapshot = AppCoreSnapshot {
            current_network_id: Some("net-1".into()),
            current_bootstrap: Some(
                serde_json::from_value(json!({
                    "device": {
                        "deviceId": "dev-1",
                        "name": "thor-mac",
                        "platform": "macos",
                        "status": "online"
                    },
                    "networks": [],
                    "controlPlane": {
                        "wsUrl": "mqtt://127.0.0.1:1883",
                        "sessionToken": "ctrl-token-1",
                        "heartbeatSeconds": 15
                    },
                    "stunServers": [],
                    "relay": {
                        "defaultClusterId": "cn-local-a",
                        "countries": []
                    },
                    "networkMap": {
                        "selfUserId": "user-1",
                        "selfDeviceId": "dev-1",
                        "networkId": "net-1",
                        "selfNodeId": "node-1",
                        "revision": 1,
                        "heartbeatSeconds": 15,
                        "stunServers": [],
                        "peers": [],
                        "routes": [],
                        "relayRegions": [],
                        "dns": {
                            "servers": [],
                            "searchDomains": []
                        }
                    }
                }))
                .expect("bootstrap json"),
            ),
            ..AppCoreSnapshot::default()
        };

        let formatted = format_bootstrap_status(&snapshot);
        assert!(formatted.contains("network net-1"));
        assert!(formatted.contains("relay cn-local-a"));
        assert!(formatted.contains("control-session present"));
    }

    #[test]
    fn formats_last_probe_from_snapshot() {
        let snapshot = AppCoreSnapshot {
            last_probe: Some(DataPlaneProbe {
                probe_id: "probe-1".into(),
                sampled_at_ms: 1,
                active_path: ActivePath::Relay {
                    peer_node_id: "peer-1".into(),
                },
                bytes_sent: 5,
                reply_observed: false,
                reply_bytes_received: None,
                reply_sampled_at_ms: None,
                reply_rtt_ms: None,
                tunnel_peer_virtual_ip: Some("100.64.0.2".into()),
                observed_rtt_ms: None,
                packet_loss_ppm: None,
                path_score: None,
                derp_cluster_id: None,
                derp_node_id: None,
            }),
            ..AppCoreSnapshot::default()
        };

        let formatted = format_last_probe(&snapshot);
        assert!(formatted.contains("probe-1"));
        assert!(formatted.contains("relay"));
        assert_eq!(probe_last_snapshot_json(&snapshot)["probeId"], "probe-1");
    }

    #[test]
    fn formats_last_error_from_snapshot() {
        let snapshot = AppCoreSnapshot {
            last_data_plane_error: Some(DataPlaneError::new(
                DataPlaneErrorCode::Transport,
                "relay write failed",
            )),
            ..AppCoreSnapshot::default()
        };

        let formatted = format_last_error(&snapshot);
        assert!(formatted.contains("probe_transport_error"));
        assert!(formatted.contains("relay write failed"));
        assert_eq!(last_error_snapshot_json(&snapshot)["code"], "transport");
    }

    #[test]
    fn formats_relay_session_error_with_hint() {
        let snapshot = AppCoreSnapshot {
            last_data_plane_error: Some(DataPlaneError::new(
                DataPlaneErrorCode::RelaySession,
                "relay attach failed [session_not_attached]: relay session has no attached participants",
            )),
            ..AppCoreSnapshot::default()
        };

        let formatted = format_last_error(&snapshot);
        assert!(formatted.contains("probe_relay_session_error"));
        assert!(formatted.contains("session_not_attached"));
        assert!(formatted.contains("reconnect before retrying"));
        assert_eq!(
            last_error_snapshot_json(&snapshot)["hint"],
            "relay session state is stale or mismatched; reconnect before retrying"
        );
    }

    #[test]
    fn formats_tunnel_keys_from_snapshot() {
        let snapshot = AppCoreSnapshot {
            tunnel_key_material: Some(TunnelKeyMaterial {
                device_id: Some("dev-1".into()),
                node_id: Some("node-1".into()),
                key_pair: WireGuardKeyPair {
                    public_key: "wg-pub-1".into(),
                    private_key: "wg-priv-1".into(),
                },
                created_at_ms: 42,
            }),
            ..AppCoreSnapshot::default()
        };

        let formatted = format_tunnel_keys(&snapshot);
        assert!(formatted.contains("wg-pub-1"));
        assert!(!formatted.contains("wg-priv-1"));
        assert_eq!(
            tunnel_keys_snapshot_json(&snapshot)["publicKey"],
            "wg-pub-1"
        );
    }

    #[test]
    fn formats_tunnel_driver_report() {
        let report = TunnelDriverSelection {
            requested: TunnelDriverKind::Auto,
            selected: TunnelDriverKind::LinuxKernel,
            execution_mode: "dry-run",
            execution_backend: "shell",
            platform: "linux",
        };

        let formatted = format_tunnel_driver_report(&report);
        assert!(formatted.contains("linux-kernel"));
        assert!(formatted.contains("dry-run"));
        assert!(formatted.contains("shell"));
        assert_eq!(tunnel_driver_report_json(&report)["platform"], "linux");
    }

    #[test]
    fn parses_os_release_fields() {
        let parsed = parse_os_release(
            r#"
ID=ubuntu
VERSION_ID="22.04"
ID_LIKE="debian"
"#,
        );
        assert_eq!(parsed.get("ID").map(String::as_str), Some("ubuntu"));
        assert_eq!(parsed.get("VERSION_ID").map(String::as_str), Some("22.04"));
        assert_eq!(parsed.get("ID_LIKE").map(String::as_str), Some("debian"));
    }

    #[test]
    fn builds_platform_plan_for_rhel_family() {
        let report = PlatformReport {
            os: "linux".to_string(),
            distro_id: Some("rocky".to_string()),
            version_id: Some("9".to_string()),
            id_like: vec!["rhel".to_string(), "fedora".to_string()],
            family: Some(LinuxDistroFamily::Rhel),
            kernel_release: Some("6.8.0".to_string()),
            package_manager: Some("dnf".to_string()),
        };
        let plan = PlatformInstallPlan {
            report,
            packages: vec!["wireguard-tools".to_string(), "iproute".to_string()],
            supported_driver_modes: vec!["linux-kernel-shell".to_string()],
            warnings: vec!["extra repositories may be required".to_string()],
        };
        let formatted = format_platform_install_plan(&plan);
        assert!(formatted.contains("wireguard-tools"));
        assert!(formatted.contains("linux-kernel-shell"));
        assert!(platform_install_plan_json(&plan)["packages"].is_array());
    }

    #[test]
    fn formats_doctor_report() {
        let report = DoctorReport {
            platform: PlatformReport {
                os: "linux".to_string(),
                distro_id: Some("ubuntu".to_string()),
                version_id: Some("24.04".to_string()),
                id_like: vec!["debian".to_string()],
                family: Some(LinuxDistroFamily::Debian),
                kernel_release: Some("6.8.0".to_string()),
                package_manager: Some("apt-get".to_string()),
            },
            tunnel_driver: TunnelDriverSelection {
                requested: TunnelDriverKind::Auto,
                selected: TunnelDriverKind::LinuxKernel,
                execution_mode: "dry-run",
                execution_backend: "shell",
                platform: "linux",
            },
            checks: vec![DoctorCheck {
                name: "wg_command".to_string(),
                status: "warn",
                detail: "wg command missing from PATH".to_string(),
            }],
        };
        let formatted = format_doctor_report(&report);
        assert!(formatted.contains("wg_command [warn]"));
        assert!(doctor_report_json(&report)["checks"].is_array());
    }
}
