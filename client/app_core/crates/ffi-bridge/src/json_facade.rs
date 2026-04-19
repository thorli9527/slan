use serde_json::{json, Value};

use crate::facade::AppCoreFacade;
use crate::json_facade_args::{
    AuthArgs, BootstrapArgs, ConnectArgs, CreateNetworkArgs, RegisterDeviceArgs,
    RegisterNodeArgs, RelayTicketArgs, SendArgs,
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
                Ok(to_value(self.inner.create_network(args.name, args.cidr)?)?)
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
                    args.reason,
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
