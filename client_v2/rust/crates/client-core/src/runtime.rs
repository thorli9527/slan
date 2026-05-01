use std::time::{SystemTime, UNIX_EPOCH};

use anyhow::Result;

use crate::{
    command::ClientCommand,
    platform::{PlatformNetwork, RouteSpec},
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

    pub fn dispatch(&mut self, command: ClientCommand) -> Result<ClientViewState> {
        self.state.error = None;
        match command {
            ClientCommand::LoginWithBrowser => {
                self.state.auth_callback_id = Some(
                    self.state
                        .device_id
                        .clone()
                        .filter(|value| !value.trim().is_empty())
                        .unwrap_or_else(|| format!("cb-{}", current_timestamp_ms())),
                );
                self.state.notice = Some("loginBrowserRequested".to_string());
            }
            ClientCommand::ApplyAuthCallback(payload) => {
                self.state.signed_in = true;
                self.state.user_label = Some(payload.user_label);
                self.state.device_id = payload.device_id;
                self.state.virtual_ip = payload
                    .virtual_ip
                    .filter(|value| !value.trim().is_empty())
                    .or(self.state.virtual_ip.take());
                self.state.auth_callback_id = None;
                self.state.notice = Some("signedIn".to_string());
            }
            ClientCommand::EnableNetwork => {
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
                    runtime.platform.configure_ip(&virtual_ip)?;
                    runtime.platform.configure_routes(&[RouteSpec {
                        destination: "mesh".to_string(),
                        gateway: None,
                    }])?;
                    runtime.state.network_enabled = true;
                    runtime.state.virtual_ip = Some(virtual_ip);
                    runtime.state.notice = Some("networkEnabled".to_string());
                    Ok(())
                })?;
            }
            ClientCommand::DisableNetwork => {
                self.with_syncing("disableNetwork", |runtime| {
                    runtime.platform.disable_network()?;
                    runtime.state.network_enabled = false;
                    runtime.state.virtual_ip = None;
                    runtime.state.notice = Some("networkDisabled".to_string());
                    Ok(())
                })?;
            }
            ClientCommand::SyncAssignedIp(payload) => {
                let virtual_ip = payload.virtual_ip.trim();
                if virtual_ip.is_empty() {
                    self.state.error = Some("assigned virtual IP is empty".to_string());
                } else {
                    self.state.virtual_ip = Some(virtual_ip.to_string());
                    self.state.notice = Some("assignedIpSynced".to_string());
                }
            }
            ClientCommand::Logout => {
                self.platform.disable_network()?;
                self.state = ClientViewState::default();
                self.state.notice = Some("signedOut".to_string());
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
        if runtime_state.virtual_ip.is_some() || self.state.virtual_ip.is_none() {
            self.state.virtual_ip = runtime_state.virtual_ip;
        }
        Ok(())
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

fn current_timestamp_ms() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|duration| duration.as_millis() as u64)
        .unwrap_or_default()
}
