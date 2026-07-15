use std::{
    collections::BTreeMap,
    sync::{Mutex, OnceLock},
};

use anyhow::Result;
use serde::Serialize;

use crate::{
    control_plane::{
        ControlPlaneClient, DeviceNetworkConfig, DeviceNetworkPeer, DeviceResolverConfig,
        DeviceResolverRecord, DeviceResolverZone, DeviceSecurityRule,
    },
    network_event::{
        NetworkEventAclChangedPayload, NetworkEventConfigChangedPayload, NetworkEventEnvelope,
        NetworkEventMemberPayload, NetworkEventMemberRemovedPayload, NetworkEventPresencePayload,
        NetworkEventResolverChangedPayload, NetworkEventType, NetworkSnapshotPayload,
    },
    session_store::{session_device_api_token, PersistedSession},
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
    pub(crate) resolver_record_count: usize,
    pub(crate) security_group_count: usize,
    pub(crate) security_rule_count: usize,
    pub(crate) relay_candidate_count: usize,
    pub(crate) configs: Vec<DeviceNetworkConfig>,
}

impl ClientNetworkModule {
    fn replace_all(&mut self, configs: Vec<DeviceNetworkConfig>) {
        self.configs = configs
            .into_iter()
            .filter(|config| !config.network_id.trim().is_empty())
            .map(|config| (config.network_id.clone(), config))
            .collect();
    }

    fn clear(&mut self) {
        self.configs.clear();
    }

    fn get_or_insert_network(
        &mut self,
        network_id: &str,
        local_device_id: &str,
    ) -> &mut DeviceNetworkConfig {
        self.configs
            .entry(network_id.to_string())
            .or_insert_with(|| DeviceNetworkConfig {
                network_id: network_id.to_string(),
                device_id: local_device_id.to_string(),
                ..DeviceNetworkConfig::default()
            })
    }

    fn snapshot(&self) -> ClientNetworkSnapshot {
        let configs = self.configs.values().cloned().collect::<Vec<_>>();
        ClientNetworkSnapshot {
            network_count: configs.len(),
            peer_count: configs.iter().map(|item| item.peers.len()).sum(),
            resolver_record_count: configs.iter().map(|item| item.resolver_records.len()).sum(),
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
    if let Some(network_id) = session
        .active_network_id
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
    {
        if let Ok(snapshot) =
            client.network_snapshot(session_device_api_token(session), network_id, device_id)
        {
            replace_network_module_from_snapshot(
                &snapshot.network_id,
                device_id,
                &snapshot.snapshot,
            );
            let configs = network_module_snapshot().configs;
            sync_resolver_runtime_state(session, &configs);
            return Ok(configs);
        }
    }
    let configs = client.device_network_configs(session_device_api_token(session), device_id)?;
    replace_network_module_configs(configs.clone());
    sync_resolver_runtime_state(session, &configs);
    Ok(configs)
}

fn sync_resolver_runtime_state(session: &PersistedSession, configs: &[DeviceNetworkConfig]) {
    let network_id = session
        .active_network_id
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .or_else(|| configs.first().map(|config| config.network_id.as_str()));
    let Some(network_id) = network_id else {
        return;
    };
    let activation_resolver = configs
        .iter()
        .find(|config| config.network_id.trim() == network_id)
        .map(|config| config.resolver.clone())
        .unwrap_or_default();
    let mut resolver = crate::resolver_runtime_state::resolver_runtime_state()
        .lock()
        .expect("resolver runtime mutex poisoned");
    crate::resolver_apply::apply_resolver_from_device_network_configs(
        &mut resolver,
        network_id,
        &activation_resolver,
        configs,
    );
}

pub(crate) fn network_module_configs_for_session(
    client: &ControlPlaneClient,
    session: &PersistedSession,
) -> Vec<DeviceNetworkConfig> {
    let cached = network_module_snapshot().configs;
    if !cached.is_empty() {
        return cached;
    }
    refresh_network_module_from_session(client, session).unwrap_or_default()
}

pub(crate) fn replace_network_module_configs(configs: Vec<DeviceNetworkConfig>) {
    module()
        .lock()
        .expect("client network module mutex poisoned")
        .replace_all(configs);
}

pub(crate) fn replace_network_module_from_snapshot(
    network_id: &str,
    local_device_id: &str,
    snapshot: &NetworkSnapshotPayload,
) {
    let network_id = network_id.trim();
    let local_device_id = local_device_id.trim();
    if network_id.is_empty() || local_device_id.is_empty() {
        return;
    }
    let self_member = snapshot
        .members
        .iter()
        .find(|member| member.device_id == local_device_id);
    let config = DeviceNetworkConfig {
        network_id: network_id.to_string(),
        network_name: (!snapshot.network.name.trim().is_empty())
            .then(|| snapshot.network.name.clone()),
        network_code: None,
        intra_group_policy: (!snapshot.network.default_acl_policy.trim().is_empty())
            .then(|| snapshot.network.default_acl_policy.clone()),
        network_created_at: None,
        config_version: None,
        device_id: local_device_id.to_string(),
        node_id: None,
        self_node_id: None,
        prefix_len: None,
        global_ip: self_member.and_then(|member| {
            (!member.virtual_ip.trim().is_empty()).then(|| member.virtual_ip.clone())
        }),
        global_name: self_member.and_then(|member| {
            (!member.device_name.trim().is_empty()).then(|| member.device_name.clone())
        }),
        resolver: DeviceResolverConfig {
            servers: snapshot.resolver_config.servers.clone(),
            search_domains: snapshot.resolver_config.search_domains.clone(),
            split_domains: snapshot.resolver_config.split_domains.clone(),
            fallback_to_system_resolvers: snapshot.resolver_config.fallback_to_system_resolvers,
        },
        peers: snapshot
            .members
            .iter()
            .filter(|member| member.device_id != local_device_id)
            .map(|member| DeviceNetworkPeer {
                device_id: member.device_id.clone(),
                alias: (!member.device_name.trim().is_empty()).then(|| member.device_name.clone()),
                global_ip: (!member.virtual_ip.trim().is_empty())
                    .then(|| member.virtual_ip.clone()),
                status: Some(if member.online { "online" } else { "offline" }.to_string()),
                ..DeviceNetworkPeer::default()
            })
            .collect(),
        security_groups: Vec::new(),
        rules: snapshot
            .acl_rules
            .iter()
            .map(|rule| DeviceSecurityRule {
                rule_id: rule.rule_id.clone(),
                direction: rule.direction.clone(),
                priority: i64::from(rule.priority),
                action: rule.action.clone(),
                protocol: rule.protocol.clone(),
                peer_type: rule.source_type.clone(),
                peer_value: rule
                    .source_device_ids
                    .first()
                    .cloned()
                    .or_else(|| rule.source_group_ids.first().cloned())
                    .unwrap_or_default(),
                enabled: rule.enabled,
                ..DeviceSecurityRule::default()
            })
            .collect(),
        resolver_zones: snapshot
            .resolver_zones
            .iter()
            .map(|zone| DeviceResolverZone {
                zone_id: zone.zone_id.clone(),
                network_id: if zone.network_id.trim().is_empty() {
                    network_id.to_string()
                } else {
                    zone.network_id.clone()
                },
                zone_name: zone.zone_name.clone(),
            })
            .collect(),
        resolver_records: snapshot
            .resolver_records
            .iter()
            .map(|record| DeviceResolverRecord {
                record_id: record.record_id.clone(),
                zone_id: record.zone_id.clone(),
                network_id: network_id.to_string(),
                name: record.name.clone(),
                fqdn: (!record.fqdn.trim().is_empty()).then(|| record.fqdn.clone()),
                record_type: if record.record_type.trim().is_empty() {
                    "A".to_string()
                } else {
                    record.record_type.clone()
                },
                target_device_id: (!record.target_device_id.trim().is_empty())
                    .then(|| record.target_device_id.clone()),
                target_ip: (!record.target_ip.trim().is_empty()).then(|| record.target_ip.clone()),
                cname: (!record.cname.trim().is_empty()).then(|| record.cname.clone()),
                port: (record.port > 0).then(|| record.port.to_string()),
                ttl: (record.ttl > 0).then(|| i64::from(record.ttl)),
                ..DeviceResolverRecord::default()
            })
            .collect(),
        relay_candidates: Vec::new(),
    };
    replace_network_module_configs(vec![config]);
}

pub(crate) fn apply_network_module_event(
    network_id: &str,
    local_device_id: &str,
    envelope: &NetworkEventEnvelope,
) -> anyhow::Result<()> {
    let network_id = network_id.trim();
    let local_device_id = local_device_id.trim();
    if network_id.is_empty() || local_device_id.is_empty() {
        return Ok(());
    }
    let mut guard = module()
        .lock()
        .expect("client network module mutex poisoned");
    let config = guard.get_or_insert_network(network_id, local_device_id);
    match envelope.event_type {
        NetworkEventType::NetworkSnapshot => {
            let payload: NetworkSnapshotPayload = serde_json::from_value(envelope.payload.clone())?;
            drop(guard);
            replace_network_module_from_snapshot(network_id, local_device_id, &payload);
            return Ok(());
        }
        NetworkEventType::MemberAdded | NetworkEventType::MemberUpdated => {
            let payload: NetworkEventMemberPayload =
                serde_json::from_value(envelope.payload.clone())?;
            apply_member_upsert(config, local_device_id, payload);
        }
        NetworkEventType::MemberRemoved => {
            let payload: NetworkEventMemberRemovedPayload =
                serde_json::from_value(envelope.payload.clone())?;
            apply_member_removed(config, local_device_id, &payload.device_id);
        }
        NetworkEventType::MemberOnline | NetworkEventType::MemberOffline => {
            let payload: NetworkEventPresencePayload =
                serde_json::from_value(envelope.payload.clone())?;
            apply_member_presence(config, local_device_id, payload);
        }
        NetworkEventType::AclChanged => {
            let payload: NetworkEventAclChangedPayload =
                serde_json::from_value(envelope.payload.clone())?;
            config.rules = payload
                .rules
                .into_iter()
                .map(to_device_security_rule)
                .collect();
        }
        NetworkEventType::ResolverChanged => {
            let payload: NetworkEventResolverChangedPayload =
                serde_json::from_value(envelope.payload.clone())?;
            config.resolver = DeviceResolverConfig {
                servers: payload.config.servers.clone(),
                search_domains: payload.config.search_domains.clone(),
                split_domains: payload.config.split_domains.clone(),
                fallback_to_system_resolvers: payload.config.fallback_to_system_resolvers,
            };
            config.resolver_zones = payload
                .zones
                .iter()
                .map(|zone| DeviceResolverZone {
                    zone_id: zone.zone_id.clone(),
                    network_id: if zone.network_id.trim().is_empty() {
                        network_id.to_string()
                    } else {
                        zone.network_id.clone()
                    },
                    zone_name: zone.zone_name.clone(),
                })
                .collect();
            config.resolver_records = payload
                .records
                .into_iter()
                .map(|record| to_device_resolver_record(network_id, record))
                .collect();
        }
        NetworkEventType::NetworkConfigChanged => {
            let payload: NetworkEventConfigChangedPayload =
                serde_json::from_value(envelope.payload.clone())?;
            if !payload.network.name.trim().is_empty() {
                config.network_name = Some(payload.network.name);
            }
            if !payload.network.default_acl_policy.trim().is_empty() {
                config.intra_group_policy = Some(payload.network.default_acl_policy);
            }
        }
        NetworkEventType::DeviceGroupAdded
        | NetworkEventType::DeviceGroupRemoved
        | NetworkEventType::DeviceGroupUpdated
        | NetworkEventType::PeerPathChanged => {}
    }
    Ok(())
}

fn apply_member_upsert(
    config: &mut DeviceNetworkConfig,
    local_device_id: &str,
    payload: NetworkEventMemberPayload,
) {
    let member = payload.member;
    if member.device_id == local_device_id {
        if !member.virtual_ip.trim().is_empty() {
            config.global_ip = Some(member.virtual_ip);
        }
        if !member.device_name.trim().is_empty() {
            config.global_name = Some(member.device_name);
        }
        return;
    }
    let peer = DeviceNetworkPeer {
        device_id: member.device_id.clone(),
        alias: (!member.device_name.trim().is_empty()).then(|| member.device_name.clone()),
        global_ip: (!member.virtual_ip.trim().is_empty()).then(|| member.virtual_ip.clone()),
        status: Some(if member.online { "online" } else { "offline" }.to_string()),
        ..DeviceNetworkPeer::default()
    };
    if let Some(existing) = config
        .peers
        .iter_mut()
        .find(|existing| existing.device_id == member.device_id)
    {
        *existing = peer;
    } else {
        config.peers.push(peer);
    }
}

fn apply_member_removed(config: &mut DeviceNetworkConfig, local_device_id: &str, device_id: &str) {
    if device_id == local_device_id {
        config.global_ip = None;
        config.global_name = None;
        return;
    }
    config.peers.retain(|peer| peer.device_id != device_id);
}

fn apply_member_presence(
    config: &mut DeviceNetworkConfig,
    local_device_id: &str,
    payload: NetworkEventPresencePayload,
) {
    if payload.device_id == local_device_id {
        return;
    }
    if let Some(peer) = config
        .peers
        .iter_mut()
        .find(|peer| peer.device_id == payload.device_id)
    {
        peer.status = Some(if payload.online { "online" } else { "offline" }.to_string());
    }
}

fn to_device_security_rule(
    rule: crate::network_event::NetworkEventAclRuleView,
) -> DeviceSecurityRule {
    DeviceSecurityRule {
        rule_id: rule.rule_id,
        direction: rule.direction,
        priority: i64::from(rule.priority),
        action: rule.action,
        protocol: rule.protocol,
        peer_type: rule.source_type,
        peer_value: rule
            .source_values
            .first()
            .cloned()
            .or_else(|| rule.source_device_ids.first().cloned())
            .or_else(|| rule.source_group_ids.first().cloned())
            .unwrap_or_default(),
        enabled: rule.enabled,
        ..DeviceSecurityRule::default()
    }
}

fn to_device_resolver_record(
    network_id: &str,
    record: crate::network_event::NetworkEventResolverRecordView,
) -> DeviceResolverRecord {
    DeviceResolverRecord {
        record_id: record.record_id,
        zone_id: record.zone_id,
        network_id: if record.network_id.trim().is_empty() {
            network_id.to_string()
        } else {
            record.network_id
        },
        name: record.name,
        fqdn: (!record.fqdn.trim().is_empty()).then_some(record.fqdn),
        record_type: if record.record_type.trim().is_empty() {
            "A".to_string()
        } else {
            record.record_type
        },
        target_device_id: (!record.target_device_id.trim().is_empty())
            .then_some(record.target_device_id),
        target_ip: (!record.target_ip.trim().is_empty()).then_some(record.target_ip),
        cname: (!record.cname.trim().is_empty()).then_some(record.cname),
        port: (record.port > 0).then(|| record.port.to_string()),
        ttl: (record.ttl > 0).then(|| i64::from(record.ttl)),
        ..DeviceResolverRecord::default()
    }
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
    use super::{
        apply_network_module_event, clear_network_module, network_module_snapshot,
        replace_network_module_configs, replace_network_module_from_snapshot,
    };
    use crate::control_plane::{DeviceNetworkConfig, DeviceSecurityGroup, DeviceSecurityRule};
    use crate::network_event::{
        NetworkEventAclChangedPayload, NetworkEventAclRuleView, NetworkEventEnvelope,
        NetworkEventMemberPayload, NetworkEventMemberView, NetworkEventNetworkView,
        NetworkEventResolverChangedPayload, NetworkEventResolverRecordView, NetworkEventType,
        NetworkSnapshotPayload,
    };

    #[test]
    fn snapshot_counts_security_groups_cached_from_network_configs() {
        let _lock = crate::test_env_lock();
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

    #[test]
    fn snapshot_payload_can_populate_network_module_cache() {
        let _lock = crate::test_env_lock();
        clear_network_module();
        replace_network_module_from_snapshot(
            "network-2",
            "device-self",
            &NetworkSnapshotPayload {
                network: NetworkEventNetworkView {
                    network_id: "network-2".to_string(),
                    name: "Net Two".to_string(),
                    default_acl_policy: "allow".to_string(),
                    ..NetworkEventNetworkView::default()
                },
                members: vec![
                    NetworkEventMemberView {
                        device_id: "device-self".to_string(),
                        device_name: "Self".to_string(),
                        virtual_ip: "10.0.0.2".to_string(),
                        online: true,
                        ..NetworkEventMemberView::default()
                    },
                    NetworkEventMemberView {
                        device_id: "device-peer".to_string(),
                        device_name: "Peer".to_string(),
                        virtual_ip: "10.0.0.3".to_string(),
                        online: true,
                        ..NetworkEventMemberView::default()
                    },
                ],
                resolver_records: vec![NetworkEventResolverRecordView {
                    record_id: "dns-1".to_string(),
                    zone_id: "zone-1".to_string(),
                    name: "peer".to_string(),
                    fqdn: "peer.example".to_string(),
                    record_type: "AAAA".to_string(),
                    target_device_id: "device-peer".to_string(),
                    target_ip: "2001:db8::20".to_string(),
                    ttl: 120,
                    ..NetworkEventResolverRecordView::default()
                }],
                acl_rules: vec![NetworkEventAclRuleView {
                    rule_id: "rule-2".to_string(),
                    direction: "ingress".to_string(),
                    action: "allow".to_string(),
                    protocol: "tcp".to_string(),
                    source_type: "device".to_string(),
                    source_device_ids: vec!["device-peer".to_string()],
                    enabled: true,
                    ..NetworkEventAclRuleView::default()
                }],
                ..NetworkSnapshotPayload::default()
            },
        );

        let snapshot = network_module_snapshot();
        assert_eq!(snapshot.network_count, 1);
        assert_eq!(snapshot.peer_count, 1);
        assert_eq!(snapshot.resolver_record_count, 1);
        assert_eq!(snapshot.security_rule_count, 1);
        assert_eq!(snapshot.configs[0].device_id, "device-self");
        assert_eq!(snapshot.configs[0].global_ip.as_deref(), Some("10.0.0.2"));
        assert_eq!(snapshot.configs[0].peers[0].device_id, "device-peer");
        assert_eq!(snapshot.configs[0].resolver_records[0].record_type, "AAAA");
        assert_eq!(
            snapshot.configs[0].resolver_records[0].target_ip.as_deref(),
            Some("2001:db8::20")
        );
        assert_eq!(snapshot.configs[0].resolver_records[0].ttl, Some(120));
    }

    #[test]
    fn member_event_updates_peer_cache_incrementally() {
        let _lock = crate::test_env_lock();
        clear_network_module();
        replace_network_module_from_snapshot(
            "network-3",
            "device-self",
            &NetworkSnapshotPayload {
                network: NetworkEventNetworkView {
                    network_id: "network-3".to_string(),
                    ..NetworkEventNetworkView::default()
                },
                members: vec![NetworkEventMemberView {
                    device_id: "device-self".to_string(),
                    device_name: "Self".to_string(),
                    virtual_ip: "10.0.0.2".to_string(),
                    online: true,
                    ..NetworkEventMemberView::default()
                }],
                ..NetworkSnapshotPayload::default()
            },
        );

        apply_network_module_event(
            "network-3",
            "device-self",
            &NetworkEventEnvelope {
                r#type: "network_event".to_string(),
                network_id: "network-3".to_string(),
                version: 2,
                event_id: "evt-member-1".to_string(),
                event_type: NetworkEventType::MemberAdded,
                occurred_at: 2,
                payload: serde_json::to_value(NetworkEventMemberPayload {
                    member: NetworkEventMemberView {
                        device_id: "device-peer".to_string(),
                        device_name: "Peer".to_string(),
                        virtual_ip: "10.0.0.3".to_string(),
                        online: true,
                        ..NetworkEventMemberView::default()
                    },
                })
                .expect("encode member payload"),
            },
        )
        .expect("apply member event");

        let snapshot = network_module_snapshot();
        assert_eq!(snapshot.peer_count, 1);
        assert_eq!(snapshot.configs[0].peers[0].device_id, "device-peer");
        assert_eq!(
            snapshot.configs[0].peers[0].status.as_deref(),
            Some("online")
        );
    }

    #[test]
    fn resolver_and_acl_events_update_module_incrementally() {
        let _lock = crate::test_env_lock();
        clear_network_module();
        replace_network_module_from_snapshot(
            "network-4",
            "device-self",
            &NetworkSnapshotPayload {
                network: NetworkEventNetworkView {
                    network_id: "network-4".to_string(),
                    ..NetworkEventNetworkView::default()
                },
                members: vec![NetworkEventMemberView {
                    device_id: "device-self".to_string(),
                    ..NetworkEventMemberView::default()
                }],
                ..NetworkSnapshotPayload::default()
            },
        );

        apply_network_module_event(
            "network-4",
            "device-self",
            &NetworkEventEnvelope {
                r#type: "network_event".to_string(),
                network_id: "network-4".to_string(),
                version: 2,
                event_id: "evt-resolver-1".to_string(),
                event_type: NetworkEventType::ResolverChanged,
                occurred_at: 2,
                payload: serde_json::to_value(NetworkEventResolverChangedPayload {
                    config: crate::network_event::NetworkEventResolverConfigView {
                        servers: vec!["10.0.0.53".to_string()],
                        search_domains: vec!["example.lan".to_string()],
                        split_domains: vec!["example.lan".to_string()],
                        fallback_to_system_resolvers: false,
                    },
                    zones: vec![],
                    records: vec![NetworkEventResolverRecordView {
                        record_id: "dns-2".to_string(),
                        zone_id: "zone-2".to_string(),
                        name: "api".to_string(),
                        fqdn: "api.example".to_string(),
                        record_type: "CNAME".to_string(),
                        target_device_id: "device-self".to_string(),
                        cname: "backend.example".to_string(),
                        ttl: 90,
                        ..NetworkEventResolverRecordView::default()
                    }],
                })
                .expect("encode resolver payload"),
            },
        )
        .expect("apply resolver event");

        apply_network_module_event(
            "network-4",
            "device-self",
            &NetworkEventEnvelope {
                r#type: "network_event".to_string(),
                network_id: "network-4".to_string(),
                version: 3,
                event_id: "evt-acl-1".to_string(),
                event_type: NetworkEventType::AclChanged,
                occurred_at: 3,
                payload: serde_json::to_value(NetworkEventAclChangedPayload {
                    rules: vec![NetworkEventAclRuleView {
                        rule_id: "rule-4".to_string(),
                        direction: "ingress".to_string(),
                        action: "allow".to_string(),
                        protocol: "udp".to_string(),
                        source_type: "device".to_string(),
                        source_device_ids: vec!["device-peer".to_string()],
                        enabled: true,
                        ..NetworkEventAclRuleView::default()
                    }],
                })
                .expect("encode acl payload"),
            },
        )
        .expect("apply acl event");

        let snapshot = network_module_snapshot();
        assert_eq!(snapshot.resolver_record_count, 1);
        assert_eq!(snapshot.security_rule_count, 1);
        assert_eq!(
            snapshot.configs[0].resolver.servers,
            vec!["10.0.0.53".to_string()]
        );
        assert_eq!(snapshot.configs[0].resolver_records[0].record_id, "dns-2");
        assert_eq!(snapshot.configs[0].resolver_records[0].record_type, "CNAME");
        assert_eq!(
            snapshot.configs[0].resolver_records[0].cname.as_deref(),
            Some("backend.example")
        );
        assert_eq!(snapshot.configs[0].resolver_records[0].ttl, Some(90));
        assert_eq!(snapshot.configs[0].rules[0].rule_id, "rule-4");
    }
}
