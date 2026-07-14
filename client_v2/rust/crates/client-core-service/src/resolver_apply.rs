use anyhow::Result;

use crate::{
    control_plane::{DeviceNetworkConfig, DeviceResolverConfig},
    network_event::{
        NetworkEventEnvelope, NetworkEventResolverChangedPayload, NetworkEventResolverConfigView,
        NetworkEventType, NetworkSnapshotPayload,
    },
    resolver_runtime_state::{
        resolver_runtime_state, ResolverRecordView, ResolverZoneView, RuntimeResolverState,
    },
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
    resolver.set_upstream_servers(if config.servers.is_empty() {
        activation_config.servers.clone()
    } else {
        config.servers.clone()
    });
    resolver.set_search_domains(if config.search_domains.is_empty() {
        activation_config.search_domains.clone()
    } else {
        config.search_domains.clone()
    });
    let split_domains = if config.split_domains.is_empty() {
        if !config.search_domains.is_empty() {
            config.search_domains.clone()
        } else if !activation_config.split_domains.is_empty() {
            activation_config.split_domains.clone()
        } else {
            activation_config.search_domains.clone()
        }
    } else {
        config.split_domains.clone()
    };
    resolver.set_split_domains(split_domains);
    resolver.set_fallback_to_system_resolvers(
        config.fallback_to_system_resolvers || activation_config.fallback_to_system_resolvers,
    );

    let active_configs = configs
        .iter()
        .filter(|config| config.network_id.trim() == active_network_id.trim());
    let mut zones = Vec::new();
    let mut records = Vec::new();
    for network in active_configs {
        zones.extend(network.resolver_zones.iter().map(|zone| ResolverZoneView {
            zone_id: zone.zone_id.clone(),
            network_id: zone.network_id.clone(),
            zone_name: zone.zone_name.clone(),
            expose_global: false,
            updated_at: 0,
        }));
        records.extend(network.resolver_records.iter().map(|record| {
            ResolverRecordView {
                record_id: record.record_id.clone(),
                zone_id: record.zone_id.clone(),
                network_id: record.network_id.clone(),
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
                updated_at: 0,
            }
        }));
    }
    resolver.replace_zones(zones);
    resolver.replace_records(records);
    resolver.clear_cache();
    resolver.last_reload_at_ms = Some(current_timestamp_ms());
}

pub(crate) fn apply_resolver_runtime_event(envelope: &NetworkEventEnvelope) -> Result<()> {
    let mut resolver = resolver_runtime_state()
        .lock()
        .expect("resolver runtime mutex poisoned");
    match envelope.event_type {
        NetworkEventType::NetworkSnapshot => {
            let payload: NetworkSnapshotPayload = serde_json::from_value(envelope.payload.clone())?;
            apply_resolver_from_network_snapshot(&mut resolver, &payload);
        }
        NetworkEventType::ResolverChanged => {
            let payload: NetworkEventResolverChangedPayload =
                serde_json::from_value(envelope.payload.clone())?;
            apply_resolver_changed(&mut resolver, payload);
        }
        NetworkEventType::AclChanged
        | NetworkEventType::MemberAdded
        | NetworkEventType::MemberUpdated
        | NetworkEventType::MemberRemoved
        | NetworkEventType::MemberOnline
        | NetworkEventType::MemberOffline
        | NetworkEventType::DeviceGroupAdded
        | NetworkEventType::DeviceGroupUpdated
        | NetworkEventType::DeviceGroupRemoved => {
            resolver.clear_cache();
        }
        NetworkEventType::NetworkConfigChanged | NetworkEventType::PeerPathChanged => {}
    }
    Ok(())
}

pub(crate) fn apply_resolver_from_network_snapshot(
    dns: &mut RuntimeResolverState,
    payload: &NetworkSnapshotPayload,
) {
    dns.bind_network_id(&payload.network.network_id);
    apply_resolver_config(dns, &payload.resolver_config);
    let zones = payload
        .resolver_zones
        .iter()
        .map(|item| ResolverZoneView {
            zone_id: item.zone_id.clone(),
            network_id: item.network_id.clone(),
            zone_name: item.zone_name.clone(),
            expose_global: item.expose_global,
            updated_at: item.updated_at,
        })
        .collect();
    let records = payload
        .resolver_records
        .iter()
        .map(build_resolver_record_view)
        .collect();
    dns.replace_zones(zones);
    dns.replace_records(records);
    dns.clear_cache();
    dns.last_reload_at_ms = Some(current_timestamp_ms());
}

pub(crate) fn apply_resolver_changed(
    dns: &mut RuntimeResolverState,
    payload: NetworkEventResolverChangedPayload,
) {
    apply_resolver_config(dns, &payload.config);
    let zones = payload
        .zones
        .into_iter()
        .map(|item| ResolverZoneView {
            zone_id: item.zone_id,
            network_id: item.network_id,
            zone_name: item.zone_name,
            expose_global: item.expose_global,
            updated_at: item.updated_at,
        })
        .collect();
    let records = payload
        .records
        .into_iter()
        .map(|item| build_resolver_record_view(&item))
        .collect();
    dns.replace_zones(zones);
    dns.replace_records(records);
    dns.clear_cache();
    dns.last_reload_at_ms = Some(current_timestamp_ms());
}

fn apply_resolver_config(dns: &mut RuntimeResolverState, config: &NetworkEventResolverConfigView) {
    dns.set_upstream_servers(config.servers.clone());
    dns.set_search_domains(config.search_domains.clone());
    dns.set_split_domains(if config.split_domains.is_empty() {
        config.search_domains.clone()
    } else {
        config.split_domains.clone()
    });
    dns.set_fallback_to_system_resolvers(config.fallback_to_system_resolvers);
}

fn build_resolver_record_view(
    item: &crate::network_event::NetworkEventResolverRecordView,
) -> ResolverRecordView {
    let name = item.name.trim().to_string();
    let fqdn = if item.fqdn.trim().is_empty() {
        name.clone()
    } else {
        item.fqdn.trim().to_string()
    };
    let record_type = if item.record_type.trim().is_empty() {
        "A".to_string()
    } else {
        item.record_type.trim().to_string()
    };
    ResolverRecordView {
        record_id: item.record_id.clone(),
        zone_id: item.zone_id.clone(),
        network_id: item.network_id.clone(),
        name,
        fqdn,
        record_type,
        value: item.value.clone(),
        target_device_id: item.target_device_id.clone(),
        target_ip: item.target_ip.clone(),
        cname: item.cname.clone(),
        port: item.port,
        ttl: normalized_ttl(item.ttl),
        enabled: item.enabled,
        updated_at: item.updated_at,
    }
}

fn normalized_ttl(ttl: i32) -> u32 {
    if ttl > 0 {
        ttl as u32
    } else {
        60
    }
}

#[cfg(test)]
mod tests {
    use super::{
        apply_resolver_changed, apply_resolver_from_network_snapshot, apply_resolver_runtime_event,
    };
    use crate::{
        network_event::{
            NetworkEventAclChangedPayload, NetworkEventEnvelope, NetworkEventNetworkView,
            NetworkEventResolverChangedPayload, NetworkEventResolverConfigView, NetworkEventType,
            NetworkSnapshotPayload,
        },
        resolver_runtime_state::{
            resolver_runtime_state, CachedResolverAnswer, CachedResolverResultKind,
            RuntimeResolverState,
        },
    };

    #[test]
    fn acl_change_clears_resolver_cache() {
        let _lock = crate::test_env_lock();
        {
            let mut dns = resolver_runtime_state()
                .lock()
                .expect("resolver runtime mutex poisoned");
            dns.cache.cache_by_question.clear();
            dns.put_cached_answer(CachedResolverAnswer {
                qname: "peer.example".to_string(),
                qtype: "A".to_string(),
                result_kind: CachedResolverResultKind::AnswerA,
                ttl: Some(60),
                answers: vec!["10.0.0.9".to_string()],
                expires_at_ms: u64::MAX,
            });
        }

        apply_resolver_runtime_event(&NetworkEventEnvelope {
            r#type: "network_event".to_string(),
            network_id: "net-1".to_string(),
            version: 2,
            event_id: "evt-acl-clear-1".to_string(),
            event_type: NetworkEventType::AclChanged,
            occurred_at: 2,
            payload: serde_json::to_value(NetworkEventAclChangedPayload { rules: vec![] })
                .expect("encode acl payload"),
        })
        .expect("apply acl changed event");

        let dns = resolver_runtime_state()
            .lock()
            .expect("resolver runtime mutex poisoned");
        assert!(dns.cache.cache_by_question.is_empty());
    }

    #[test]
    fn snapshot_applies_resolver_config() {
        let mut dns = RuntimeResolverState::default();
        apply_resolver_from_network_snapshot(
            &mut dns,
            &NetworkSnapshotPayload {
                network: NetworkEventNetworkView {
                    network_id: "net-1".to_string(),
                    ..NetworkEventNetworkView::default()
                },
                resolver_config: NetworkEventResolverConfigView {
                    servers: vec!["10.0.0.53".to_string()],
                    search_domains: vec!["example.lan".to_string()],
                    split_domains: vec!["example.lan".to_string()],
                    fallback_to_system_resolvers: false,
                },
                ..NetworkSnapshotPayload::default()
            },
        );

        assert_eq!(dns.active_network_id(), Some("net-1"));
        assert_eq!(
            dns.effective_upstream_resolvers(),
            vec!["10.0.0.53".to_string()]
        );
        assert!(dns.matches_split_domain("peer.example.lan"));
    }

    #[test]
    fn resolver_changed_uses_search_domains_as_split_fallback() {
        let mut dns = RuntimeResolverState::default();
        apply_resolver_changed(
            &mut dns,
            NetworkEventResolverChangedPayload {
                config: NetworkEventResolverConfigView {
                    search_domains: vec!["corp.lan".to_string()],
                    fallback_to_system_resolvers: true,
                    ..NetworkEventResolverConfigView::default()
                },
                ..NetworkEventResolverChangedPayload::default()
            },
        );

        assert!(dns.matches_split_domain("host.corp.lan"));
        assert!(dns.effective_upstream_resolvers().is_empty());
    }
}
