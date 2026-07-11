use crate::{
    dns_runtime_state::{normalize_fqdn, RuntimeDnsState},
    network_runtime_state::RuntimeNetworkState,
};

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ResolveAuthoritativeResult {
    AnswerA { ttl: u32, ip: String },
    AnswerCname { ttl: u32, cname: String },
    NxDomain,
    NoData,
    NotManaged,
}

pub(crate) fn resolve_authoritative(
    runtime: &RuntimeNetworkState,
    dns: &RuntimeDnsState,
    requester_device_id: &str,
    qname: &str,
    qtype: &str,
) -> ResolveAuthoritativeResult {
    let fqdn = normalize_fqdn(qname);
    if fqdn.is_empty() {
        return ResolveAuthoritativeResult::NoData;
    }
    let Some(resolved_name) = resolve_managed_name(dns, &fqdn) else {
        return ResolveAuthoritativeResult::NotManaged;
    };
    let Some(record_ids) = dns.record_ids_by_fqdn.get(&resolved_name) else {
        return ResolveAuthoritativeResult::NxDomain;
    };
    let wanted = qtype.trim().to_ascii_uppercase();
    for record_id in record_ids {
        let Some(record) = dns.records_by_id.get(record_id) else {
            continue;
        };
        if !record.enabled {
            continue;
        }
        if !record_visible_to_requester(
            runtime,
            requester_device_id,
            record.target_device_id.as_str(),
        ) {
            continue;
        }
        let actual = record.record_type.trim().to_ascii_uppercase();
        if actual == "A" && wanted == "A" {
            if let Some(ip) = resolve_record_target_ip(runtime, record) {
                return ResolveAuthoritativeResult::AnswerA {
                    ttl: normalized_ttl(record.ttl),
                    ip,
                };
            }
            return ResolveAuthoritativeResult::NoData;
        }
        if actual == "CNAME" && wanted == "CNAME" && !record.cname.trim().is_empty() {
            return ResolveAuthoritativeResult::AnswerCname {
                ttl: normalized_ttl(record.ttl),
                cname: normalize_fqdn(&record.cname),
            };
        }
        if actual == wanted {
            return ResolveAuthoritativeResult::NoData;
        }
    }
    ResolveAuthoritativeResult::NxDomain
}

fn resolve_managed_name(dns: &RuntimeDnsState, fqdn: &str) -> Option<String> {
    if dns.record_ids_by_fqdn.contains_key(fqdn) {
        return Some(fqdn.to_string());
    }
    if fqdn.contains('.') {
        return dns.zones_by_id.values().find_map(|zone| {
            let zone_name = normalize_fqdn(&zone.zone_name);
            (!zone_name.is_empty()
                && (fqdn == zone_name || fqdn.ends_with(&format!(".{zone_name}"))))
            .then(|| fqdn.to_string())
        });
    }
    dns.zones_by_id.values().find_map(|zone| {
        let zone_name = normalize_fqdn(&zone.zone_name);
        if zone_name.is_empty() {
            return None;
        }
        let candidate = format!("{fqdn}.{zone_name}");
        dns.record_ids_by_fqdn
            .contains_key(&candidate)
            .then_some(candidate)
    })
}

fn normalized_ttl(ttl: u32) -> u32 {
    if ttl == 0 {
        30
    } else {
        ttl
    }
}

fn record_visible_to_requester(
    runtime: &RuntimeNetworkState,
    requester_device_id: &str,
    target_device_id: &str,
) -> bool {
    if requester_device_id.trim().is_empty() || target_device_id.trim().is_empty() {
        return false;
    }
    if requester_device_id == target_device_id {
        return true;
    }
    runtime
        .members_by_device_id
        .contains_key(requester_device_id)
        && runtime.members_by_device_id.contains_key(target_device_id)
}

fn resolve_record_target_ip(
    runtime: &RuntimeNetworkState,
    record: &crate::dns_runtime_state::DnsRecordView,
) -> Option<String> {
    let target_ip = record.target_ip.trim();
    if !target_ip.is_empty() {
        return Some(target_ip.to_string());
    }
    let target_device_id = record.target_device_id.trim();
    if target_device_id.is_empty() {
        return None;
    }
    runtime
        .members_by_device_id
        .get(target_device_id)
        .and_then(|member| {
            let ip = member.virtual_ip.trim();
            (!ip.is_empty()).then(|| ip.to_string())
        })
}

#[cfg(test)]
mod tests {
    use super::{resolve_authoritative, ResolveAuthoritativeResult};
    use crate::{
        dns_runtime_state::{DnsRecordView, DnsZoneView, RuntimeDnsState},
        network_event::NetworkEventMemberView,
        network_runtime_state::RuntimeNetworkState,
    };

    #[test]
    fn short_name_resolves_via_zone_search_suffix() {
        let mut dns = RuntimeDnsState::default();
        dns.replace_zones(vec![DnsZoneView {
            zone_id: "zone-1".to_string(),
            network_id: "net-1".to_string(),
            zone_name: "example.lan".to_string(),
            expose_global: false,
            updated_at: 0,
        }]);
        dns.replace_records(vec![DnsRecordView {
            record_id: "rec-1".to_string(),
            zone_id: "zone-1".to_string(),
            network_id: "net-1".to_string(),
            name: "self".to_string(),
            fqdn: "self.example.lan".to_string(),
            record_type: "A".to_string(),
            target_device_id: "device-1".to_string(),
            target_ip: String::new(),
            cname: String::new(),
            port: 0,
            ttl: 60,
            enabled: true,
            updated_at: 0,
        }]);

        let mut runtime = RuntimeNetworkState::default();
        runtime.members_by_device_id.insert(
            "device-1".to_string(),
            NetworkEventMemberView {
                device_id: "device-1".to_string(),
                virtual_ip: "10.0.0.99".to_string(),
                ..NetworkEventMemberView::default()
            },
        );

        let result = resolve_authoritative(&runtime, &dns, "device-1", "self", "A");
        assert_eq!(
            result,
            ResolveAuthoritativeResult::AnswerA {
                ttl: 60,
                ip: "10.0.0.99".to_string()
            }
        );
    }

    #[test]
    fn full_fqdn_still_resolves_normally() {
        let mut dns = RuntimeDnsState::default();
        dns.replace_zones(vec![DnsZoneView {
            zone_id: "zone-1".to_string(),
            network_id: "net-1".to_string(),
            zone_name: "example.lan".to_string(),
            expose_global: false,
            updated_at: 0,
        }]);
        dns.replace_records(vec![DnsRecordView {
            record_id: "rec-1".to_string(),
            zone_id: "zone-1".to_string(),
            network_id: "net-1".to_string(),
            name: "self".to_string(),
            fqdn: "self.example.lan".to_string(),
            record_type: "A".to_string(),
            target_device_id: "device-1".to_string(),
            target_ip: String::new(),
            cname: String::new(),
            port: 0,
            ttl: 60,
            enabled: true,
            updated_at: 0,
        }]);

        let mut runtime = RuntimeNetworkState::default();
        runtime.members_by_device_id.insert(
            "device-1".to_string(),
            NetworkEventMemberView {
                device_id: "device-1".to_string(),
                virtual_ip: "10.0.0.99".to_string(),
                ..NetworkEventMemberView::default()
            },
        );

        let result = resolve_authoritative(&runtime, &dns, "device-1", "self.example.lan", "A");
        assert_eq!(
            result,
            ResolveAuthoritativeResult::AnswerA {
                ttl: 60,
                ip: "10.0.0.99".to_string()
            }
        );
    }
}
