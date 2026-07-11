use anyhow::Result;

use crate::{
    dns_runtime_state::{dns_runtime_state, DnsRecordView, DnsZoneView, RuntimeDnsState},
    network_event::{
        NetworkEventDnsChangedPayload, NetworkEventEnvelope, NetworkEventType,
        NetworkSnapshotPayload,
    },
    session_store::current_timestamp_ms,
};

pub(crate) fn apply_dns_runtime_event(envelope: &NetworkEventEnvelope) -> Result<()> {
    let mut dns = dns_runtime_state()
        .lock()
        .expect("dns runtime mutex poisoned");
    match envelope.event_type {
        NetworkEventType::NetworkSnapshot => {
            let payload: NetworkSnapshotPayload = serde_json::from_value(envelope.payload.clone())?;
            apply_dns_from_network_snapshot(&mut dns, &payload);
        }
        NetworkEventType::DnsChanged => {
            let payload: NetworkEventDnsChangedPayload =
                serde_json::from_value(envelope.payload.clone())?;
            apply_dns_changed(&mut dns, payload);
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
            dns.clear_cache();
        }
        NetworkEventType::NetworkConfigChanged | NetworkEventType::PeerPathChanged => {}
    }
    Ok(())
}

pub(crate) fn apply_dns_from_network_snapshot(
    dns: &mut RuntimeDnsState,
    payload: &NetworkSnapshotPayload,
) {
    dns.bind_network_id(&payload.network.network_id);
    let zones = payload
        .dns_zones
        .iter()
        .map(|item| DnsZoneView {
            zone_id: item.zone_id.clone(),
            network_id: item.network_id.clone(),
            zone_name: item.zone_name.clone(),
            expose_global: item.expose_global,
            updated_at: item.updated_at,
        })
        .collect();
    let records = payload
        .dns_records
        .iter()
        .map(build_dns_record_view)
        .collect();
    dns.replace_zones(zones);
    dns.replace_records(records);
    dns.clear_cache();
    dns.last_reload_at_ms = Some(current_timestamp_ms());
}

pub(crate) fn apply_dns_changed(dns: &mut RuntimeDnsState, payload: NetworkEventDnsChangedPayload) {
    let zones = payload
        .zones
        .into_iter()
        .map(|item| DnsZoneView {
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
        .map(|item| build_dns_record_view(&item))
        .collect();
    dns.replace_zones(zones);
    dns.replace_records(records);
    dns.clear_cache();
    dns.last_reload_at_ms = Some(current_timestamp_ms());
}

fn build_dns_record_view(item: &crate::network_event::NetworkEventDnsRecordView) -> DnsRecordView {
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
    DnsRecordView {
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
    use super::apply_dns_runtime_event;
    use crate::{
        dns_runtime_state::{dns_runtime_state, CachedDnsAnswer, CachedDnsResultKind},
        network_event::{NetworkEventAclChangedPayload, NetworkEventEnvelope, NetworkEventType},
    };

    #[test]
    fn acl_change_clears_dns_cache() {
        let _lock = crate::test_env_lock();
        {
            let mut dns = dns_runtime_state()
                .lock()
                .expect("dns runtime mutex poisoned");
            dns.cache_by_question.clear();
            dns.put_cached_answer(CachedDnsAnswer {
                qname: "peer.example".to_string(),
                qtype: "A".to_string(),
                result_kind: CachedDnsResultKind::AnswerA,
                ttl: Some(60),
                answers: vec!["10.0.0.9".to_string()],
                expires_at_ms: u64::MAX,
            });
        }

        apply_dns_runtime_event(&NetworkEventEnvelope {
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

        let dns = dns_runtime_state()
            .lock()
            .expect("dns runtime mutex poisoned");
        assert!(dns.cache_by_question.is_empty());
    }
}
