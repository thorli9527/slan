use anyhow::Result;

use crate::{
    command::{AssignedIpPayload, ClientCommand},
    platform::{
        PlatformNetwork, PlatformResolverConfig, PlatformResolverRecord, PlatformResolverZone,
        RelayDataPlaneConfig, RouteSpec,
    },
    state::ClientViewState,
};

pub struct ClientRuntime<P> {
    platform: P,
    state: ClientViewState,
}

impl<P: PlatformNetwork> ClientRuntime<P> {
    pub fn new(platform: P) -> Self {
        Self {
            platform,
            state: ClientViewState::default(),
        }
    }

    pub fn state(&self) -> &ClientViewState {
        &self.state
    }

    pub fn set_error(&mut self, error: impl Into<String>) -> ClientViewState {
        self.state.syncing = false;
        self.state.sync_reason = None;
        self.state.switch_enabled = true;
        self.state.error = Some(error.into());
        self.state.clone()
    }

    pub fn request_browser_login(&mut self, device_id: Option<String>) -> ClientViewState {
        self.state.error = None;
        self.state.device_id = device_id;
        self.state.notice = Some("loginBrowserRequested".to_string());
        self.state.clone()
    }

    pub fn apply_network_disabled_state(&mut self) -> ClientViewState {
        self.state.error = None;
        self.state.syncing = false;
        self.state.sync_reason = None;
        self.state.switch_enabled = true;
        self.state.network_enabled = false;
        self.state.notice = Some("networkDisabled".to_string());
        self.state.clone()
    }

    pub fn apply_network_enabled_state(&mut self, virtual_ip: String) -> ClientViewState {
        self.state.error = None;
        self.state.syncing = false;
        self.state.sync_reason = None;
        self.state.switch_enabled = true;
        self.state.network_enabled = true;
        self.state.virtual_ip = normalize_virtual_ip(&virtual_ip);
        self.state.notice = Some("networkEnabled".to_string());
        self.state.clone()
    }

    pub fn apply_assigned_ip_state(&mut self, payload: AssignedIpPayload) -> ClientViewState {
        self.state.error = None;
        if let Some(virtual_ip) = normalize_virtual_ip(&payload.virtual_ip) {
            self.state.virtual_ip = Some(virtual_ip);
            self.state.notice = Some("assignedIpSynced".to_string());
        } else {
            self.state.error = Some("assigned virtual IP is empty".to_string());
        }
        self.state.clone()
    }

    pub fn apply_logout_state(&mut self) -> ClientViewState {
        self.state = ClientViewState::default();
        self.state.notice = Some("signedOut".to_string());
        self.state.clone()
    }

    pub fn dispatch(&mut self, command: ClientCommand) -> Result<ClientViewState> {
        self.state.error = None;
        match command {
            ClientCommand::OpenClientLogin => {
                self.state.notice = Some("loginBrowserRequested".to_string());
            }
            ClientCommand::LoginWithPassword(_) => {
                self.state.notice = Some("passwordLoginRequested".to_string());
            }
            ClientCommand::ApplyDeviceUserLogin(payload) => {
                self.state.signed_in = true;
                self.state.user_label = Some(payload.user_label);
                self.state.device_id = payload.device_id;
                self.state.virtual_ip = payload
                    .virtual_ip
                    .and_then(|value| normalize_virtual_ip(&value))
                    .or(self.state.virtual_ip.take());
                self.state.notice = Some("signedIn".to_string());
            }
            ClientCommand::EnableNetwork => {
                self.enable_network_with_config(
                    32,
                    &PlatformResolverConfig::default(),
                    &[],
                    &[],
                    &[RouteSpec {
                        destination: "mesh".to_string(),
                        gateway: None,
                    }],
                    None,
                )?;
            }
            ClientCommand::DisableNetwork => {
                self.with_syncing("disableNetwork", |runtime| {
                    runtime.platform.disable_network()?;
                    runtime.apply_network_disabled_state();
                    Ok(())
                })?;
            }
            ClientCommand::SyncAssignedIp(payload) => {
                if let Some(virtual_ip) = normalize_virtual_ip(&payload.virtual_ip) {
                    if self.state.network_enabled {
                        self.platform
                            .configure_ip(&virtual_ip, payload.prefix_len.unwrap_or(32))?;
                    }
                }
                self.apply_assigned_ip_state(payload);
            }
            ClientCommand::ApplyPlatformRuntimeState(runtime_state) => {
                self.state.network_enabled = runtime_state.network_enabled;
                if let Some(virtual_ip) = runtime_state
                    .virtual_ip
                    .and_then(|value| normalize_virtual_ip(&value))
                {
                    self.state.virtual_ip = Some(virtual_ip);
                }
                self.state.notice = Some("platformRuntimeStateSynced".to_string());
            }
            ClientCommand::ApplyTrafficStats(payload) => {
                self.state.traffic_tx_bytes = Some(payload.tx_bytes);
                self.state.traffic_rx_bytes = Some(payload.rx_bytes);
                self.state.traffic_tx_bytes_per_minute = Some(payload.tx_bytes_per_minute);
                self.state.traffic_rx_bytes_per_minute = Some(payload.rx_bytes_per_minute);
                self.state.traffic_updated_at_ms = Some(payload.updated_at_ms);
                self.state.notice = Some("trafficStatsSynced".to_string());
            }
            ClientCommand::ApplyClientMessage(payload) => {
                self.state.last_client_message_id = payload.message_id;
                self.state.last_client_message_from_device_id = payload.from_device_id;
                self.state.last_client_message_body = payload.body;
                self.state.notice = Some("clientMessageReceived".to_string());
            }
            ClientCommand::Logout => {
                let _ = self.platform.disable_network();
                self.apply_logout_state();
            }
            ClientCommand::Refresh => self.refresh()?,
            ClientCommand::OpenWebConsole => {
                self.state.notice = Some("webConsoleRequested".to_string());
            }
        }
        Ok(self.state.clone())
    }

    pub fn refresh(&mut self) -> Result<()> {
        if !self.state.signed_in {
            self.state.network_enabled = false;
            self.state.virtual_ip = None;
            return Ok(());
        }
        let runtime_state = self.platform.read_runtime_state()?;
        self.state.network_enabled = runtime_state.network_enabled;
        if let Some(virtual_ip) = runtime_state
            .virtual_ip
            .and_then(|value| normalize_virtual_ip(&value))
        {
            self.state.virtual_ip = Some(virtual_ip);
        }
        Ok(())
    }

    pub fn enable_network_with_config(
        &mut self,
        prefix_len: u8,
        resolver: &PlatformResolverConfig,
        resolver_zones: &[PlatformResolverZone],
        resolver_records: &[PlatformResolverRecord],
        routes: &[RouteSpec],
        relay_config: Option<&RelayDataPlaneConfig>,
    ) -> Result<ClientViewState> {
        self.with_syncing("enableNetwork", |runtime| {
            let virtual_ip = runtime
                .state
                .virtual_ip
                .clone()
                .filter(|value| !value.trim().is_empty())
                .ok_or_else(|| {
                    anyhow::anyhow!("device unavailable: missing assigned virtual IP")
                })?;
            runtime.platform.install_adapter()?;
            runtime.platform.configure_ip(&virtual_ip, prefix_len)?;
            runtime.platform.configure_resolver(resolver)?;
            runtime
                .platform
                .configure_resolver_map(resolver_zones, resolver_records)?;
            runtime.platform.configure_routes(routes)?;
            runtime.platform.configure_relay(relay_config)?;
            runtime.apply_network_enabled_state(virtual_ip);
            Ok(())
        })?;
        Ok(self.state.clone())
    }

    fn with_syncing(
        &mut self,
        reason: &str,
        action: impl FnOnce(&mut Self) -> Result<()>,
    ) -> Result<()> {
        self.state.syncing = true;
        self.state.sync_reason = Some(reason.to_string());
        self.state.switch_enabled = false;
        let result = action(self);
        self.state.syncing = false;
        self.state.sync_reason = None;
        self.state.switch_enabled = true;
        if let Err(error) = &result {
            self.state.error = Some(error.to_string());
        }
        result
    }
}

fn normalize_virtual_ip(value: &str) -> Option<String> {
    let trimmed = value.trim();
    let ip = trimmed.split_once('/').map_or(trimmed, |(ip, _)| ip.trim());
    ip.split_whitespace()
        .next()
        .map(str::to_string)
        .filter(|value| !value.is_empty())
}

#[cfg(test)]
mod tests {
    use super::normalize_virtual_ip;

    #[test]
    fn normalizes_cidr_virtual_ip() {
        assert_eq!(
            normalize_virtual_ip(" 10.0.0.1/32 "),
            Some("10.0.0.1".to_string())
        );
        assert_eq!(
            normalize_virtual_ip("10.0.0.6/32 extra"),
            Some("10.0.0.6".to_string())
        );
        assert_eq!(
            normalize_virtual_ip("10.0.0.9"),
            Some("10.0.0.9".to_string())
        );
        assert_eq!(normalize_virtual_ip(" /32 "), None);
        assert_eq!(normalize_virtual_ip(" "), None);
    }
}
