use serde_json::{json, Value};

use crate::facade::AppCoreFacade;
use crate::json_facade_args::{
    AuthArgs, BootstrapArgs, ConnectArgs, CreateNetworkArgs, DeviceNetworkStateArgs,
    JoinNetworkArgs, JoinNetworkByKeyArgs, JoinNetworkByOwnerEmailArgs, RefreshSessionArgs,
    RegisterDeviceArgs, RegisterNodeArgs, RelayTicketArgs, SendArgs, UpdateAttachmentRemarkArgs,
};
use crate::json_facade_runtime::{
    connection_state_value, data_plane_error_string, parse_args, to_value,
};

pub struct JsonAppCoreFacade<F>
where
    F: AppCoreFacade,
{
    inner: F,
}

impl<F> JsonAppCoreFacade<F>
where
    F: AppCoreFacade,
{
    pub fn new(inner: F) -> Self {
        Self { inner }
    }

    pub fn invoke(&self, method: &str, args: Value) -> Result<Value, String> {
        match method {
            "register" => {
                let args: AuthArgs = parse_args(args)?;
                Ok(to_value(self.inner.register(args.email, args.password)?)?)
            }
            "login" => {
                let args: AuthArgs = parse_args(args)?;
                Ok(to_value(self.inner.login(args.email, args.password)?)?)
            }
            "refreshSession" => {
                let args: RefreshSessionArgs = parse_args(args)?;
                Ok(to_value(
                    self.inner
                        .refresh_session(args.refresh_token, args.device_id)?,
                )?)
            }
            "registerDevice" => {
                let args: RegisterDeviceArgs = parse_args(args)?;
                Ok(to_value(self.inner.register_device(
                    args.name,
                    args.platform,
                    args.machine_id,
                    args.public_key,
                )?)?)
            }
            "registerNode" => {
                let args: RegisterNodeArgs = parse_args(args)?;
                Ok(to_value(self.inner.register_node(
                    args.device_id,
                    args.node_id,
                    args.node_public_key,
                    args.capabilities,
                )?)?)
            }
            "listNetworks" => {
                let items = self.inner.list_networks()?;
                Ok(json!({ "items": items }))
            }
            "createNetwork" => {
                let args: CreateNetworkArgs = parse_args(args)?;
                Ok(to_value(self.inner.create_network(
                    args.name,
                    args.cidr,
                    args.expected_devices,
                    args.gateway_ip,
                    args.allocation_start_ip,
                    args.allocation_end_ip,
                )?)?)
            }
            "joinNetwork" => {
                let args: JoinNetworkArgs = parse_args(args)?;
                Ok(to_value(
                    self.inner.join_network(args.network_id, args.device_id)?,
                )?)
            }
            "joinNetworkByOwnerEmail" => {
                let args: JoinNetworkByOwnerEmailArgs = parse_args(args)?;
                Ok(to_value(self.inner.join_network_by_owner_email(
                    args.owner_email,
                    args.device_id,
                )?)?)
            }
            "joinNetworkByKey" => {
                let args: JoinNetworkByKeyArgs = parse_args(args)?;
                Ok(to_value(
                    self.inner
                        .join_network_by_key(args.join_key, args.device_id)?,
                )?)
            }
            "updateAttachmentRemark" => {
                let args: UpdateAttachmentRemarkArgs = parse_args(args)?;
                Ok(to_value(self.inner.update_attachment_remark(
                    args.network_id,
                    args.attachment_id,
                    args.remark,
                )?)?)
            }
            "activateNetwork" => {
                let args: JoinNetworkArgs = parse_args(args)?;
                Ok(to_value(
                    self.inner
                        .activate_network(args.network_id, args.device_id)?,
                )?)
            }
            "switchNetwork" => {
                let args: JoinNetworkArgs = parse_args(args)?;
                Ok(to_value(
                    self.inner.switch_network(args.network_id, args.device_id)?,
                )?)
            }
            "deactivateNetwork" => {
                let args: JoinNetworkArgs = parse_args(args)?;
                self.inner
                    .deactivate_network(args.network_id, args.device_id)?;
                Ok(json!({}))
            }
            "setDeviceNetworkState" => {
                let args: DeviceNetworkStateArgs = parse_args(args)?;
                self.inner.set_device_network_state(
                    args.device_id,
                    args.network_id,
                    args.control_reachable,
                    args.network_online,
                    args.tunnel_up,
                    args.last_probe_ok,
                    args.virtual_ip,
                    args.reported_at,
                )?;
                Ok(json!({}))
            }
            "bootstrap" => {
                let args: BootstrapArgs = parse_args(args)?;
                Ok(to_value(
                    self.inner.bootstrap(args.node_id, args.network_id)?,
                )?)
            }
            "controlStatus" => Ok(to_value(self.inner.control_status()?)?),
            "controlSync" => Ok(to_value(self.inner.control_sync()?)?),
            "issueRelayTicket" => {
                let args: RelayTicketArgs = parse_args(args)?;
                Ok(to_value(self.inner.issue_relay_ticket(
                    args.network_id,
                    args.src_node_id,
                    args.dst_node_id,
                    args.derp_cluster_id,
                    args.preferred_derp_node_ids,
                    args.reason,
                    args.relay_region_id,
                )?)?)
            }
            "connect" => {
                let args: ConnectArgs = parse_args(args)?;
                Ok(connection_state_value(
                    self.inner.connect(args.network_id, args.peer_node_id)?,
                ))
            }
            "probe" => {
                let args: SendArgs = parse_args(args)?;
                Ok(to_value(
                    self.inner
                        .probe_with_timeout(args.payload.into_bytes(), args.probe_timeout_ms)
                        .map_err(|error| data_plane_error_string("probe", error))?,
                )?)
            }
            "send" => {
                let args: SendArgs = parse_args(args)?;
                Ok(json!({
                    "bytesSent": self
                        .inner
                        .send(args.payload.into_bytes())
                        .map_err(|error| data_plane_error_string("send", error))?
                }))
            }
            "disconnect" => {
                self.inner.disconnect()?;
                Ok(json!({}))
            }
            _ => Err(format!("unsupported method: {method}")),
        }
    }
}
