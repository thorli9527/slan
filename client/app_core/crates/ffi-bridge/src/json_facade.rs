use serde_json::{json, Value};

use crate::facade::AppCoreFacade;
use crate::json_facade_args::{
    AuthArgs, BootstrapArgs, ConnectArgs, CreateNetworkArgs, DeviceNetworkStateArgs,
    JoinNetworkArgs, JoinNetworkByKeyArgs, LocalNetworkArgs, RefreshSessionArgs,
    RegisterDeviceArgs, RegisterNodeArgs, RelayTicketArgs, RestoreSessionArgs, SendArgs,
    UpdateAttachmentRemarkArgs,
};
use crate::json_facade_runtime::{
    connection_state_value, data_plane_error_string, parse_args, to_value,
};
use crate::AppCoreSnapshot;

pub struct JsonAppCoreFacade<F>
where
    F: AppCoreFacade,
{
    inner: F,
}

#[cfg(test)]
mod tests {
    use std::sync::Mutex;

    use serde_json::json;
    use slan_app_core::{
        BootstrapConfig, ConnectionState, Device, Network, NetworkAssignment, NetworkJoinResult,
        Node, RelayTicket, Session,
    };

    use crate::facade::{AppCoreFacade, ControlStatusView, DataPlaneError, DataPlaneProbe};
    use crate::AppCoreSnapshot;

    use super::JsonAppCoreFacade;

    #[test]
    fn restore_session_routes_to_inner_facade() {
        let inner = RecordingFacade::default();
        let facade = JsonAppCoreFacade::new(inner);

        let result = facade
            .invoke(
                "restoreSession",
                json!({
                    "userId": "user-1",
                    "accessToken": "token-1",
                    "refreshToken": "refresh-1",
                    "expiresIn": 3600,
                    "deviceId": "dev-1",
                    "userLabel": "ignored@example.com"
                }),
            )
            .expect("restore session should succeed");

        assert_eq!(result, json!({ "restored": true }));
        let restored = facade.inner.restored.lock().expect("restore lock").clone();
        let restored = restored.expect("restored session");
        assert_eq!(restored.user_id, "user-1");
        assert_eq!(restored.access_token, "token-1");
        assert_eq!(restored.refresh_token.as_deref(), Some("refresh-1"));
        assert_eq!(restored.device_id.as_deref(), Some("dev-1"));
    }

    #[test]
    fn snapshot_routes_to_inner_facade() {
        let inner = RecordingFacade::default();
        let expected = AppCoreSnapshot {
            current_network_id: Some("net-1".to_string()),
            ..AppCoreSnapshot::default()
        };
        *inner.snapshot.lock().expect("snapshot lock") = expected.clone();
        let facade = JsonAppCoreFacade::new(inner);

        assert_eq!(
            facade.snapshot().expect("snapshot").current_network_id,
            expected.current_network_id,
        );

        let restored = AppCoreSnapshot {
            current_network_id: Some("net-2".to_string()),
            ..AppCoreSnapshot::default()
        };
        facade
            .restore_snapshot(restored.clone())
            .expect("restore snapshot");

        assert_eq!(
            facade
                .inner
                .restored_snapshot
                .lock()
                .expect("restored snapshot lock")
                .as_ref()
                .and_then(|snapshot| snapshot.current_network_id.clone()),
            restored.current_network_id,
        );
    }

    #[derive(Default)]
    struct RecordingFacade {
        restored: Mutex<Option<Session>>,
        snapshot: Mutex<AppCoreSnapshot>,
        restored_snapshot: Mutex<Option<AppCoreSnapshot>>,
    }

    impl AppCoreFacade for RecordingFacade {
        fn snapshot(&self) -> Result<AppCoreSnapshot, String> {
            self.snapshot
                .lock()
                .map_err(|_| "lock poisoned".to_string())
                .map(|snapshot| snapshot.clone())
        }

        fn restore_snapshot(&self, snapshot: AppCoreSnapshot) -> Result<(), String> {
            *self
                .restored_snapshot
                .lock()
                .map_err(|_| "lock poisoned".to_string())? = Some(snapshot);
            Ok(())
        }

        fn restore_session(&self, session: Session) -> Result<(), String> {
            *self
                .restored
                .lock()
                .map_err(|_| "lock poisoned".to_string())? = Some(session);
            Ok(())
        }

        fn register(&self, _email: String, _password: String) -> Result<Session, String> {
            unimplemented!()
        }

        fn login(&self, _email: String, _password: String) -> Result<Session, String> {
            unimplemented!()
        }

        fn refresh_session(
            &self,
            _refresh_token: String,
            _device_id: Option<String>,
        ) -> Result<Session, String> {
            unimplemented!()
        }

        fn register_device(
            &self,
            _name: String,
            _platform: String,
            _machine_id: String,
            _public_key: String,
        ) -> Result<Device, String> {
            unimplemented!()
        }

        fn list_devices(&self) -> Result<Vec<Device>, String> {
            unimplemented!()
        }

        fn register_node(
            &self,
            _device_id: String,
            _node_id: String,
            _node_public_key: String,
            _capabilities: Vec<String>,
        ) -> Result<Node, String> {
            unimplemented!()
        }

        fn list_networks(&self) -> Result<Vec<Network>, String> {
            unimplemented!()
        }

        fn create_network(
            &self,
            _name: String,
            _cidr: Option<String>,
            _allocation_start_ip: Option<String>,
            _allocation_end_ip: Option<String>,
        ) -> Result<Network, String> {
            unimplemented!()
        }

        fn join_network(
            &self,
            _network_id: String,
            _device_id: String,
        ) -> Result<NetworkJoinResult, String> {
            unimplemented!()
        }

        fn join_network_by_key(
            &self,
            _join_key: String,
            _device_id: String,
        ) -> Result<NetworkJoinResult, String> {
            unimplemented!()
        }

        fn update_attachment_remark(
            &self,
            _network_id: String,
            _attachment_id: String,
            _remark: Option<String>,
        ) -> Result<NetworkAssignment, String> {
            unimplemented!()
        }

        fn activate_network(
            &self,
            _network_id: String,
            _device_id: String,
        ) -> Result<NetworkJoinResult, String> {
            unimplemented!()
        }

        fn switch_network(
            &self,
            _network_id: String,
            _device_id: String,
        ) -> Result<NetworkJoinResult, String> {
            unimplemented!()
        }

        fn deactivate_network(
            &self,
            _network_id: String,
            _device_id: String,
        ) -> Result<(), String> {
            unimplemented!()
        }

        fn set_device_network_state(
            &self,
            _device_id: String,
            _network_id: String,
            _control_reachable: bool,
            _network_online: bool,
            _tunnel_up: bool,
            _last_probe_ok: bool,
            _virtual_ip: Option<String>,
            _reported_at: Option<i64>,
        ) -> Result<(), String> {
            unimplemented!()
        }

        fn report_device_network_state(&self) -> Result<(), String> {
            unimplemented!()
        }

        fn enable_local_network(
            &self,
            _network_id: Option<String>,
        ) -> Result<BootstrapConfig, String> {
            unimplemented!()
        }

        fn disable_local_network(&self, _network_id: Option<String>) -> Result<(), String> {
            unimplemented!()
        }

        fn ensure_local_dns(&self) -> Result<(), String> {
            unimplemented!()
        }

        fn bootstrap(
            &self,
            _node_id: String,
            _network_id: String,
        ) -> Result<BootstrapConfig, String> {
            unimplemented!()
        }

        fn control_sync(&self) -> Result<BootstrapConfig, String> {
            unimplemented!()
        }

        fn control_status(&self) -> Result<ControlStatusView, String> {
            unimplemented!()
        }

        fn issue_relay_ticket(
            &self,
            _network_id: String,
            _src_node_id: String,
            _dst_node_id: String,
            _derp_cluster_id: Option<String>,
            _preferred_derp_node_ids: Vec<String>,
            _reason: String,
            _relay_region_id: Option<String>,
        ) -> Result<RelayTicket, String> {
            unimplemented!()
        }

        fn connect(
            &self,
            _network_id: String,
            _peer_node_id: String,
        ) -> Result<ConnectionState, String> {
            unimplemented!()
        }

        fn probe_with_timeout(
            &self,
            _packet: Vec<u8>,
            _reply_timeout_ms: Option<u64>,
        ) -> Result<DataPlaneProbe, DataPlaneError> {
            unimplemented!()
        }

        fn send(&self, _packet: Vec<u8>) -> Result<usize, DataPlaneError> {
            unimplemented!()
        }

        fn disconnect(&self) -> Result<(), String> {
            unimplemented!()
        }
    }
}

impl<F> JsonAppCoreFacade<F>
where
    F: AppCoreFacade,
{
    pub fn new(inner: F) -> Self {
        Self { inner }
    }

    pub fn snapshot(&self) -> Result<AppCoreSnapshot, String> {
        self.inner.snapshot()
    }

    pub fn restore_snapshot(&self, snapshot: AppCoreSnapshot) -> Result<(), String> {
        self.inner.restore_snapshot(snapshot)
    }

    pub fn invoke(&self, method: &str, args: Value) -> Result<Value, String> {
        match method {
            "register" => {
                let args: AuthArgs = parse_args(args)?;
                Ok(to_value(self.inner.register(args.email, args.password)?)?)
            }
            "restoreSession" => {
                let args: RestoreSessionArgs = parse_args(args)?;
                self.inner.restore_session(args.session)?;
                Ok(json!({ "restored": true }))
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
            "listDevices" => {
                let items = self.inner.list_devices()?;
                Ok(json!({ "items": items }))
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
            "reportDeviceNetworkState" => {
                self.inner.report_device_network_state()?;
                Ok(json!({}))
            }
            "enableLocalNetwork" => {
                let args: LocalNetworkArgs = parse_args(args)?;
                Ok(to_value(self.inner.enable_local_network(args.network_id)?)?)
            }
            "disableLocalNetwork" => {
                let args: LocalNetworkArgs = parse_args(args)?;
                self.inner.disable_local_network(args.network_id)?;
                Ok(json!({}))
            }
            "ensureLocalDns" => {
                self.inner.ensure_local_dns()?;
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
