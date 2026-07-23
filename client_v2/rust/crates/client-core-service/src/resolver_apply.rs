use std::collections::{BTreeMap, BTreeSet};

use client_core::PlatformResolverRecord;

use crate::{
    control_plane::{DeviceNetworkConfig, DeviceResolverConfig, DeviceSecurityRule},
    network_event::{
        NetworkEventAclRuleView, NetworkEventDeviceGroupView, NetworkEventMemberView,
        NetworkEventNetworkView,
    },
    network_runtime_state::{NetworkSyncStatus, RuntimeNetworkState},
    resolver_runtime_state::{ResolverRecordView, ResolverZoneView, RuntimeResolverState},
    session_store::current_timestamp_ms,
};

pub(crate) fn apply_resolver_from_device_network_configs(
    resolver: &mut RuntimeResolverState,
    active_network_id: &str,
    activation_config: &DeviceResolverConfig,
    configs: &[DeviceNetworkConfig],
) {
    let selected = configs
        .iter()
        .find(|config| config.network_id.trim() == active_network_id.trim())
        .or_else(|| configs.first());
    let config = selected
        .map(|item| &item.resolver)
        .unwrap_or(activation_config);

    resolver.bind_network_id(active_network_id);
    resolver.set_upstream_servers(unique_config_values(configs, activation_config, |value| {
        &value.servers
    }));
    resolver.set_search_domains(unique_config_values(configs, activation_config, |value| {
        &value.search_domains
    }));
    let mut split_domains =
        unique_config_values(configs, activation_config, |value| &value.split_domains);
    if split_domains.is_empty() {
        split_domains =
            unique_config_values(configs, activation_config, |value| &value.search_domains);
    }
    resolver.set_split_domains(split_domains);
    resolver.set_fallback_to_system_resolvers(
        config.fallback_to_system_resolvers
            || activation_config.fallback_to_system_resolvers
            || configs
                .iter()
                .any(|item| item.resolver.fallback_to_system_resolvers),
    );

    let mut zones = Vec::new();
    let mut records = Vec::new();
    let mut network_states = Vec::new();
    for network in configs {
        zones.extend(network.resolver_zones.iter().map(|zone| ResolverZoneView {
            zone_id: zone.zone_id.clone(),
            zone_name: zone.zone_name.clone(),
        }));
        records.extend(network.resolver_records.iter().map(|record| {
            ResolverRecordView {
                record_id: record.record_id.clone(),
                network_id: if record.network_id.trim().is_empty() {
                    network.network_id.clone()
                } else {
                    record.network_id.clone()
                },
                name: record.name.clone(),
                fqdn: record
                    .fqdn
                    .clone()
                    .filter(|value| !value.trim().is_empty())
                    .unwrap_or_else(|| record.name.clone()),
                record_type: if record.record_type.trim().is_empty() {
                    "A".to_string()
                } else {
                    record.record_type.clone()
                },
                value: String::new(),
                target_device_id: record.target_device_id.clone().unwrap_or_default(),
                target_ip: record.target_ip.clone().unwrap_or_default(),
                cname: record.cname.clone().unwrap_or_default(),
                port: record
                    .port
                    .as_deref()
                    .and_then(|value| value.parse::<i32>().ok())
                    .unwrap_or_default(),
                ttl: record
                    .ttl
                    .filter(|value| *value > 0)
                    .map(|value| value as u32)
                    .unwrap_or(60),
                enabled: true,
            }
        }));
        network_states.push(runtime_network_state_from_config(network));
    }
    resolver.replace_zones(zones);
    resolver.replace_records(records);
    resolver.replace_network_states(network_states);
    resolver.clear_cache();
    resolver.last_reload_at_ms = Some(current_timestamp_ms());
}

pub(crate) fn platform_resolver_records_from_configs(
    configs: &[DeviceNetworkConfig],
) -> Vec<PlatformResolverRecord> {
    configs
        .iter()
        .flat_map(|config| {
            config.resolver_records.iter().map(|record| {
                let target_ip = record
                    .target_ip
                    .clone()
                    .filter(|value| !value.trim().is_empty())
                    .or_else(|| {
                        let target_device_id = record.target_device_id.as_deref()?.trim();
                        if target_device_id == config.device_id.trim() {
                            return config.global_ip.clone();
                        }
                        config
                            .peers
                            .iter()
                            .find(|peer| peer.device_id.trim() == target_device_id)
                            .and_then(|peer| peer.global_ip.clone())
                    });
                PlatformResolverRecord {
                    record_id: record.record_id.clone(),
                    zone_id: record.zone_id.clone(),
                    network_id: record.network_id.clone(),
                    name: record.name.clone(),
                    fqdn: record.fqdn.clone(),
                    record_type: record.record_type.clone(),
                    target_device_id: record.target_device_id.clone(),
                    target_ip,
                    cname: record.cname.clone(),
                    port: record.port.clone(),
                    ttl: record.ttl,
                }
            })
        })
        .collect()
}

fn unique_config_values<'a>(
    configs: &'a [DeviceNetworkConfig],
    activation: &'a DeviceResolverConfig,
    values: impl Fn(&'a DeviceResolverConfig) -> &'a Vec<String>,
) -> Vec<String> {
    configs
        .iter()
        .flat_map(|config| values(&config.resolver).iter())
        .chain(values(activation).iter())
        .map(|value| value.trim())
        .filter(|value| !value.is_empty())
        .map(str::to_string)
        .collect::<BTreeSet<_>>()
        .into_iter()
        .collect()
}

fn runtime_network_state_from_config(config: &DeviceNetworkConfig) -> RuntimeNetworkState {
    let network_id = config.network_id.trim().to_string();
    let mut state = RuntimeNetworkState::default();
    state.bind_network_id(&network_id);
    state.bind_session_identity(Some(&config.device_id), config.global_ip.as_deref());
    state.network = Some(NetworkEventNetworkView {
        network_id,
        name: config.network_name.clone().unwrap_or_default(),
        default_acl_policy: config
            .intra_group_policy
            .clone()
            .unwrap_or_else(|| "deny".to_string()),
        ..NetworkEventNetworkView::default()
    });
    let mut members = Vec::with_capacity(config.peers.len() + 1);
    members.push(NetworkEventMemberView {
        device_id: config.device_id.clone(),
        device_name: config.global_name.clone().unwrap_or_default(),
        virtual_ip: config.global_ip.clone().unwrap_or_default(),
        online: true,
        group_ids: config
            .device_groups_by_device
            .get(&config.device_id)
            .cloned()
            .unwrap_or_default(),
        ..NetworkEventMemberView::default()
    });
    members.extend(config.peers.iter().map(|peer| {
        NetworkEventMemberView {
            device_id: peer.device_id.clone(),
            device_name: peer
                .alias
                .clone()
                .or_else(|| peer.global_name.clone())
                .unwrap_or_default(),
            virtual_ip: peer.global_ip.clone().unwrap_or_default(),
            online: peer.status.as_deref().is_some_and(|status| {
                status.eq_ignore_ascii_case("active") || status.eq_ignore_ascii_case("online")
            }),
            group_ids: config
                .device_groups_by_device
                .get(&peer.device_id)
                .cloned()
                .unwrap_or_default(),
            ..NetworkEventMemberView::default()
        }
    }));
    state.members_by_device_id = members
        .into_iter()
        .map(|member| (member.device_id.clone(), member))
        .collect();

    let mut members_by_group = BTreeMap::<String, Vec<String>>::new();
    for (device_id, group_ids) in &config.device_groups_by_device {
        for group_id in group_ids {
            members_by_group
                .entry(group_id.clone())
                .or_default()
                .push(device_id.clone());
        }
    }
    state.groups_by_group_id = members_by_group
        .into_iter()
        .map(|(group_id, mut member_device_ids)| {
            member_device_ids.sort();
            member_device_ids.dedup();
            (
                group_id.clone(),
                NetworkEventDeviceGroupView {
                    group_id,
                    member_device_ids,
                    ..NetworkEventDeviceGroupView::default()
                },
            )
        })
        .collect();
    state.acl_by_rule_id = config
        .rules
        .iter()
        .enumerate()
        .map(|(index, rule)| {
            let rule = resolver_acl_rule(rule, index);
            (rule.rule_id.clone(), rule)
        })
        .collect();
    state.sync_status = NetworkSyncStatus::Live;
    state
}

fn resolver_acl_rule(rule: &DeviceSecurityRule, index: usize) -> NetworkEventAclRuleView {
    let mut view = NetworkEventAclRuleView {
        rule_id: format!("{}:{index}", rule.rule_id),
        priority: i32::try_from(rule.priority).unwrap_or_default(),
        action: rule.action.clone(),
        direction: rule.direction.clone(),
        protocol: rule.protocol.clone(),
        enabled: rule.enabled,
        ..NetworkEventAclRuleView::default()
    };
    if rule.direction.eq_ignore_ascii_case("egress") {
        view.source_type = "current_device".to_string();
        view.target_type = rule.peer_type.clone();
        assign_acl_side(
            &rule.peer_type,
            &rule.peer_value,
            &mut view.target_values,
            &mut view.target_device_ids,
            &mut view.target_group_ids,
        );
    } else {
        view.source_type = rule.peer_type.clone();
        view.target_type = "current_device".to_string();
        assign_acl_side(
            &rule.peer_type,
            &rule.peer_value,
            &mut view.source_values,
            &mut view.source_device_ids,
            &mut view.source_group_ids,
        );
    }
    view
}

fn assign_acl_side(
    side_type: &str,
    side_value: &str,
    values: &mut Vec<String>,
    device_ids: &mut Vec<String>,
    group_ids: &mut Vec<String>,
) {
    let value = side_value.trim();
    if value.is_empty() {
        return;
    }
    match side_type.trim().to_ascii_lowercase().as_str() {
        "device" => device_ids.push(value.to_string()),
        "device_group" => group_ids.push(value.to_string()),
        _ => values.push(value.to_string()),
    }
}

#[cfg(test)]
mod tests {
    use super::{
        apply_resolver_from_device_network_configs, platform_resolver_records_from_configs,
    };
    use crate::{
        control_plane::{
            DeviceNetworkConfig, DeviceNetworkPeer, DeviceResolverConfig, DeviceResolverRecord,
            DeviceResolverZone,
        },
        resolver_authority::{resolve_authoritative, ResolveAuthoritativeResult},
        resolver_runtime_state::RuntimeResolverState,
    };

    #[test]
    fn device_network_configs_aggregate_dns_across_all_joined_networks() {
        let mut dns = RuntimeResolverState::default();
        let configs = vec![
            DeviceNetworkConfig {
                network_id: "net-active".to_string(),
                device_id: "device-self".to_string(),
                global_ip: Some("10.0.0.1".to_string()),
                intra_group_policy: Some("allow".to_string()),
                ..DeviceNetworkConfig::default()
            },
            DeviceNetworkConfig {
                network_id: "net-dns".to_string(),
                device_id: "device-self".to_string(),
                global_ip: Some("10.0.0.1".to_string()),
                intra_group_policy: Some("allow".to_string()),
                resolver: DeviceResolverConfig {
                    search_domains: vec!["tt.com".to_string()],
                    split_domains: vec!["tt.com".to_string()],
                    ..DeviceResolverConfig::default()
                },
                peers: vec![DeviceNetworkPeer {
                    device_id: "device-target".to_string(),
                    global_ip: Some("10.0.0.12".to_string()),
                    status: Some("active".to_string()),
                    ..DeviceNetworkPeer::default()
                }],
                resolver_zones: vec![DeviceResolverZone {
                    zone_id: "zone-tt".to_string(),
                    network_id: "net-dns".to_string(),
                    zone_name: "tt.com".to_string(),
                }],
                resolver_records: vec![DeviceResolverRecord {
                    record_id: "record-api".to_string(),
                    zone_id: "zone-tt".to_string(),
                    network_id: "net-dns".to_string(),
                    name: "api".to_string(),
                    fqdn: Some("api.tt.com".to_string()),
                    record_type: "A".to_string(),
                    target_device_id: Some("device-target".to_string()),
                    ttl: Some(60),
                    ..DeviceResolverRecord::default()
                }],
                ..DeviceNetworkConfig::default()
            },
        ];

        apply_resolver_from_device_network_configs(
            &mut dns,
            "net-active",
            &DeviceResolverConfig::default(),
            &configs,
        );

        assert_eq!(dns.active_network_id(), Some("net-active"));
        assert_eq!(dns.zone_count(), 1);
        assert_eq!(dns.record_count(), 1);
        assert!(dns.matches_split_domain("api.tt.com"));
        let platform_records = platform_resolver_records_from_configs(&configs);
        assert_eq!(platform_records.len(), 1);
        assert_eq!(platform_records[0].target_ip.as_deref(), Some("10.0.0.12"));
        let fallback_runtime = crate::network_runtime_state::RuntimeNetworkState::default();
        assert_eq!(
            resolve_authoritative(
                &fallback_runtime,
                &mut dns,
                "device-self",
                "api.tt.com",
                "A",
            ),
            ResolveAuthoritativeResult::AnswerA {
                ttl: 60,
                ips: vec!["10.0.0.12".to_string()],
            }
        );
    }
}
