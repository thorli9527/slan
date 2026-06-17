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
    pub(crate) security_group_count: usize,
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
            security_group_count: configs.iter().map(|item| item.security_groups.len()).sum(),
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
    replace_network_module_configs(configs.clone());
    Ok(configs)
}

pub(crate) fn replace_network_module_configs(configs: Vec<DeviceNetworkConfig>) {
    module()
        .lock()
        .expect("client network module mutex poisoned")
        .replace_all(configs);
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

#[cfg(test)]
mod tests {
    use super::{clear_network_module, network_module_snapshot, replace_network_module_configs};
    use crate::control_plane::{DeviceNetworkConfig, DeviceSecurityGroup, DeviceSecurityRule};

    #[test]
    fn snapshot_counts_security_groups_cached_from_network_configs() {
        clear_network_module();
        replace_network_module_configs(vec![DeviceNetworkConfig {
            network_id: "network-1".to_string(),
            device_id: "device-1".to_string(),
            security_groups: vec![DeviceSecurityGroup {
                security_group_id: "sg-1".to_string(),
                network_id: "network-1".to_string(),
                name: "default".to_string(),
                status: "active".to_string(),
                created_at: 1,
            }],
            intra_group_policy: Some("deny".to_string()),
            rules: vec![DeviceSecurityRule {
                rule_id: "rule-1".to_string(),
                security_group_id: "sg-1".to_string(),
                direction: "ingress".to_string(),
                action: "allow".to_string(),
                protocol: "tcp".to_string(),
                peer_type: "device".to_string(),
                peer_value: "device-2".to_string(),
                enabled: true,
                ..DeviceSecurityRule::default()
            }],
            ..DeviceNetworkConfig::default()
        }]);

        let snapshot = network_module_snapshot();
        assert_eq!(snapshot.network_count, 1);
        assert_eq!(snapshot.security_group_count, 1);
        assert_eq!(snapshot.security_rule_count, 1);
        assert_eq!(
            snapshot.configs[0].intra_group_policy.as_deref(),
            Some("deny")
        );
    }
}
