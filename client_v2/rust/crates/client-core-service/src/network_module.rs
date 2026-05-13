use std::{
    collections::BTreeMap,
    sync::{Mutex, OnceLock},
};

use anyhow::Result;
use serde::Serialize;

use crate::{
    control_plane::{ControlPlaneClient, DeviceNetworkConfig},
    session_store::PersistedSession,
};

static NETWORK_MODULE: OnceLock<Mutex<ClientNetworkModule>> = OnceLock::new();

fn module() -> &'static Mutex<ClientNetworkModule> {
    NETWORK_MODULE.get_or_init(|| Mutex::new(ClientNetworkModule::default()))
}

#[derive(Debug, Clone, Default, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct ClientNetworkModule {
    configs: BTreeMap<String, DeviceNetworkConfig>,
}

#[derive(Debug, Clone, Default, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct ClientNetworkSnapshot {
    pub(crate) network_count: usize,
    pub(crate) peer_count: usize,
    pub(crate) dns_record_count: usize,
    pub(crate) security_rule_count: usize,
    pub(crate) relay_candidate_count: usize,
    pub(crate) configs: Vec<DeviceNetworkConfig>,
}

impl ClientNetworkModule {
    fn replace_all(&mut self, configs: Vec<DeviceNetworkConfig>) {
        self.configs = configs
            .into_iter()
            .map(|config| (config.network_id.clone(), config))
            .collect();
    }

    fn clear(&mut self) {
        self.configs.clear();
    }

    fn snapshot(&self) -> ClientNetworkSnapshot {
        let configs = self.configs.values().cloned().collect::<Vec<_>>();
        ClientNetworkSnapshot {
            network_count: configs.len(),
            peer_count: configs.iter().map(|item| item.peers.len()).sum(),
            dns_record_count: configs.iter().map(|item| item.dns_records.len()).sum(),
            security_rule_count: configs.iter().map(|item| item.rules.len()).sum(),
            relay_candidate_count: configs.iter().map(|item| item.relay_candidates.len()).sum(),
            configs,
        }
    }
}

pub(crate) fn refresh_network_module_from_session(
    client: &ControlPlaneClient,
    session: &PersistedSession,
) -> Result<Vec<DeviceNetworkConfig>> {
    let Some(device_id) = session
        .device_id
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
    else {
        return Ok(Vec::new());
    };
    let configs = client.device_network_configs(&session.access_token, device_id)?;
    module()
        .lock()
        .expect("client network module mutex poisoned")
        .replace_all(configs.clone());
    Ok(configs)
}

pub(crate) fn clear_network_module() {
    module()
        .lock()
        .expect("client network module mutex poisoned")
        .clear();
}

pub(crate) fn network_module_snapshot() -> ClientNetworkSnapshot {
    module()
        .lock()
        .expect("client network module mutex poisoned")
        .snapshot()
}
