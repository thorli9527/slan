use crate::{
    network_runtime_state::RuntimeNetworkState,
    resolver_runtime_state::{
        normalize_fqdn, CachedResolverAnswer, CachedResolverResultKind, ResolverRecordView,
        RuntimeResolverState,
    },
    session_store::current_timestamp_ms,
};
use serde_json::{json, Value};

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ResolveAuthoritativeResult {
    AnswerA { ttl: u32, ips: Vec<String> },
    AnswerAaaa { ttl: u32, ips: Vec<String> },
    AnswerCname { ttl: u32, cname: String },
    AnswerPtr { ttl: u32, name: String },
    AnswerTxt { ttl: u32, texts: Vec<String> },
    AnswerSrv { ttl: u32, port: u16, target: String },
    NxDomain,
    NoData,
    NotManaged,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub(crate) enum ResolverRouteDecision {
    Authoritative,
    ForwardUpstream,
    NoData,
}

pub(crate) fn resolve_authoritative_result_json(result: ResolveAuthoritativeResult) -> Value {
    match result {
        ResolveAuthoritativeResult::AnswerA { ttl, ips } => json!({
            "result": "answer_a",
            "ttl": ttl,
            "ips": ips,
        }),
        ResolveAuthoritativeResult::AnswerAaaa { ttl, ips } => json!({
            "result": "answer_aaaa",
            "ttl": ttl,
            "ips": ips,
        }),
        ResolveAuthoritativeResult::AnswerCname { ttl, cname } => json!({
            "result": "answer_cname",
            "ttl": ttl,
            "cname": cname,
        }),
        ResolveAuthoritativeResult::AnswerPtr { ttl, name } => json!({
            "result": "answer_ptr",
            "ttl": ttl,
            "name": name,
        }),
        ResolveAuthoritativeResult::AnswerTxt { ttl, texts } => json!({
            "result": "answer_txt",
            "ttl": ttl,
            "texts": texts,
        }),
        ResolveAuthoritativeResult::AnswerSrv { ttl, port, target } => json!({
            "result": "answer_srv",
            "ttl": ttl,
            "port": port,
            "target": target,
        }),
        ResolveAuthoritativeResult::NxDomain => json!({ "result": "nxdomain" }),
        ResolveAuthoritativeResult::NoData => json!({ "result": "nodata" }),
        ResolveAuthoritativeResult::NotManaged => json!({ "result": "not_managed" }),
    }
}

pub(crate) fn route_resolver_query(
    resolver: &RuntimeResolverState,
    qname: &str,
) -> ResolverRouteDecision {
    let fqdn = normalize_fqdn(qname);
    if fqdn.is_empty() {
        return ResolverRouteDecision::NoData;
    }
    if resolve_managed_name(resolver, &fqdn).is_some() {
        return ResolverRouteDecision::Authoritative;
    }
    if resolver.matches_split_domain(&fqdn) || !resolver.effective_upstream_resolvers().is_empty() {
        return ResolverRouteDecision::ForwardUpstream;
    }
    ResolverRouteDecision::NoData
}

pub(crate) fn resolve_authoritative(
    runtime: &RuntimeNetworkState,
    dns: &mut RuntimeResolverState,
    requester_device_id: &str,
    qname: &str,
    qtype: &str,
) -> ResolveAuthoritativeResult {
    let fqdn = normalize_fqdn(qname);
    if fqdn.is_empty() {
        return ResolveAuthoritativeResult::NoData;
    }
    let wanted = qtype.trim().to_ascii_uppercase();
    let now_ms = current_timestamp_ms();
    if let Some(cached) = dns.cached_answer(&fqdn, &wanted, now_ms) {
        return cached_answer_to_result(cached);
    }
    let Some(resolved_name) = resolve_managed_name(dns, &fqdn) else {
        return ResolveAuthoritativeResult::NotManaged;
    };
    let Some(record_ids) = dns.authority.record_ids_by_fqdn.get(&resolved_name) else {
        let result = ResolveAuthoritativeResult::NxDomain;
        cache_result(dns, &fqdn, &wanted, &result);
        return result;
    };
    let mut matched_type = false;
    let mut a_answers = Vec::new();
    let mut aaaa_answers = Vec::new();
    let mut txt_answers = Vec::new();
    let mut answer_ttl = None;
    for record_id in record_ids {
        let Some(record) = dns.authority.records_by_id.get(record_id) else {
            continue;
        };
        if !record.enabled {
            continue;
        }
        if !record_visible_to_requester(
            runtime,
            requester_device_id,
            record.target_device_id.as_str(),
            record.fqdn.as_str(),
            record.name.as_str(),
        ) {
            continue;
        }
        let actual = record.record_type.trim().to_ascii_uppercase();
        if actual == "A" && wanted == "A" {
            matched_type = true;
            if let Some(ip) = resolve_record_target_ipv4(runtime, record) {
                answer_ttl.get_or_insert(normalized_ttl(record.ttl));
                if !a_answers.iter().any(|item| item == &ip) {
                    a_answers.push(ip);
                }
            }
            continue;
        }
        if actual == "AAAA" && wanted == "AAAA" {
            matched_type = true;
            if let Some(ip) = resolve_record_target_ipv6(record) {
                answer_ttl.get_or_insert(normalized_ttl(record.ttl));
                if !aaaa_answers.iter().any(|item| item == &ip) {
                    aaaa_answers.push(ip);
                }
            }
            continue;
        }
        if actual == "CNAME" && wanted == "CNAME" && !record.cname.trim().is_empty() {
            let result = ResolveAuthoritativeResult::AnswerCname {
                ttl: normalized_ttl(record.ttl),
                cname: normalize_fqdn(&record.cname),
            };
            cache_result(dns, &fqdn, &wanted, &result);
            return result;
        }
        if actual == "PTR" && wanted == "PTR" {
            let ptr_name = normalize_fqdn(&record.value);
            if !ptr_name.is_empty() {
                let result = ResolveAuthoritativeResult::AnswerPtr {
                    ttl: normalized_ttl(record.ttl),
                    name: ptr_name,
                };
                cache_result(dns, &fqdn, &wanted, &result);
                return result;
            }
            let result = ResolveAuthoritativeResult::NoData;
            cache_result(dns, &fqdn, &wanted, &result);
            return result;
        }
        if actual == "TXT" && wanted == "TXT" {
            matched_type = true;
            let text = record.value.trim().to_string();
            if !text.is_empty() {
                answer_ttl.get_or_insert(normalized_ttl(record.ttl));
                if !txt_answers.iter().any(|item| item == &text) {
                    txt_answers.push(text);
                }
            }
            continue;
        }
        if actual == "SRV" && wanted == "SRV" {
            let target = normalize_fqdn(&record.value);
            let port = u16::try_from(record.port).ok().filter(|value| *value > 0);
            if !target.is_empty() && port.is_some() {
                let result = ResolveAuthoritativeResult::AnswerSrv {
                    ttl: normalized_ttl(record.ttl),
                    port: port.unwrap_or_default(),
                    target,
                };
                cache_result(dns, &fqdn, &wanted, &result);
                return result;
            }
            let result = ResolveAuthoritativeResult::NoData;
            cache_result(dns, &fqdn, &wanted, &result);
            return result;
        }
        if actual == wanted {
            let result = ResolveAuthoritativeResult::NoData;
            cache_result(dns, &fqdn, &wanted, &result);
            return result;
        }
    }
    if wanted == "A" && !a_answers.is_empty() {
        let result = ResolveAuthoritativeResult::AnswerA {
            ttl: answer_ttl.unwrap_or(30),
            ips: a_answers,
        };
        cache_result(dns, &fqdn, &wanted, &result);
        return result;
    }
    if wanted == "AAAA" && !aaaa_answers.is_empty() {
        let result = ResolveAuthoritativeResult::AnswerAaaa {
            ttl: answer_ttl.unwrap_or(30),
            ips: aaaa_answers,
        };
        cache_result(dns, &fqdn, &wanted, &result);
        return result;
    }
    if wanted == "TXT" && !txt_answers.is_empty() {
        let result = ResolveAuthoritativeResult::AnswerTxt {
            ttl: answer_ttl.unwrap_or(30),
            texts: txt_answers,
        };
        cache_result(dns, &fqdn, &wanted, &result);
        return result;
    }
    if matched_type {
        let result = ResolveAuthoritativeResult::NoData;
        cache_result(dns, &fqdn, &wanted, &result);
        return result;
    }
    let result = ResolveAuthoritativeResult::NxDomain;
    cache_result(dns, &fqdn, &wanted, &result);
    result
}

fn resolve_managed_name(dns: &RuntimeResolverState, fqdn: &str) -> Option<String> {
    if dns.authority.record_ids_by_fqdn.contains_key(fqdn) {
        return Some(fqdn.to_string());
    }
    if fqdn.contains('.') {
        return dns.authority.zones_by_id.values().find_map(|zone| {
            let zone_name = normalize_fqdn(&zone.zone_name);
            (!zone_name.is_empty()
                && (fqdn == zone_name || fqdn.ends_with(&format!(".{zone_name}"))))
            .then(|| fqdn.to_string())
        });
    }
    dns.authority.zones_by_id.values().find_map(|zone| {
        let zone_name = normalize_fqdn(&zone.zone_name);
        if zone_name.is_empty() {
            return None;
        }
        let candidate = format!("{fqdn}.{zone_name}");
        dns.authority
            .record_ids_by_fqdn
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
    record_fqdn: &str,
    record_name: &str,
) -> bool {
    if requester_device_id.trim().is_empty() || target_device_id.trim().is_empty() {
        return false;
    }
    if requester_device_id == target_device_id {
        return true;
    }
    if !runtime
        .members_by_device_id
        .contains_key(requester_device_id)
        || !runtime.members_by_device_id.contains_key(target_device_id)
    {
        return false;
    }
    acl_direction_allows(
        runtime,
        requester_device_id,
        target_device_id,
        record_fqdn,
        record_name,
        "egress",
    ) && acl_direction_allows(
        runtime,
        requester_device_id,
        target_device_id,
        record_fqdn,
        record_name,
        "ingress",
    )
}

fn resolve_record_target_ipv4(
    runtime: &RuntimeNetworkState,
    record: &ResolverRecordView,
) -> Option<String> {
    let target_ip = record.target_ip.trim();
    if !target_ip.is_empty() && target_ip.parse::<std::net::Ipv4Addr>().is_ok() {
        return Some(target_ip.to_string());
    }
    let target_device_id = record.target_device_id.trim();
    if target_device_id.is_empty() {
        return None;
    }
    runtime_virtual_ip_for_device(runtime, target_device_id)
}

fn resolve_record_target_ipv6(record: &ResolverRecordView) -> Option<String> {
    let target_ip = record.target_ip.trim();
    (!target_ip.is_empty() && target_ip.parse::<std::net::Ipv6Addr>().is_ok())
        .then(|| target_ip.to_string())
}

fn acl_direction_allows(
    runtime: &RuntimeNetworkState,
    requester_device_id: &str,
    target_device_id: &str,
    record_fqdn: &str,
    record_name: &str,
    direction: &str,
) -> bool {
    let default_allow = runtime
        .network
        .as_ref()
        .map(|network| {
            network
                .default_acl_policy
                .trim()
                .eq_ignore_ascii_case("allow")
        })
        .unwrap_or(true);
    let mut matched = runtime
        .acl_by_rule_id
        .values()
        .filter(|rule| rule.enabled)
        .filter(|rule| rule.direction.trim().eq_ignore_ascii_case(direction))
        .collect::<Vec<_>>();
    matched.sort_by(|left, right| {
        left.priority
            .cmp(&right.priority)
            .then_with(|| left.rule_id.cmp(&right.rule_id))
    });
    for rule in matched {
        if acl_rule_matches(
            runtime,
            rule,
            requester_device_id,
            target_device_id,
            record_fqdn,
            record_name,
            direction,
        ) {
            return !rule.action.trim().eq_ignore_ascii_case("deny");
        }
    }
    default_allow
}

fn acl_rule_matches(
    runtime: &RuntimeNetworkState,
    rule: &crate::network_event::NetworkEventAclRuleView,
    requester_device_id: &str,
    target_device_id: &str,
    record_fqdn: &str,
    record_name: &str,
    direction: &str,
) -> bool {
    if !matches!(direction, "egress" | "ingress") {
        return false;
    }
    if rule
        .target_type
        .trim()
        .eq_ignore_ascii_case("current_device")
    {
        return acl_side_matches(
            runtime,
            rule.source_type.as_str(),
            &rule.source_values,
            &rule.source_device_ids,
            &rule.source_group_ids,
            target_device_id,
            record_fqdn,
            record_name,
        ) && acl_current_device_matches(requester_device_id);
    }
    if rule
        .source_type
        .trim()
        .eq_ignore_ascii_case("current_device")
    {
        return acl_current_device_matches(requester_device_id)
            && acl_side_matches(
                runtime,
                rule.target_type.as_str(),
                &rule.target_values,
                &rule.target_device_ids,
                &rule.target_group_ids,
                target_device_id,
                record_fqdn,
                record_name,
            );
    }
    acl_side_matches(
        runtime,
        rule.source_type.as_str(),
        &rule.source_values,
        &rule.source_device_ids,
        &rule.source_group_ids,
        requester_device_id,
        record_fqdn,
        record_name,
    ) && acl_side_matches(
        runtime,
        rule.target_type.as_str(),
        &rule.target_values,
        &rule.target_device_ids,
        &rule.target_group_ids,
        target_device_id,
        record_fqdn,
        record_name,
    )
}

fn acl_current_device_matches(device_id: &str) -> bool {
    !device_id.trim().is_empty()
}

fn acl_side_matches(
    runtime: &RuntimeNetworkState,
    side_type: &str,
    values: &[String],
    device_ids: &[String],
    group_ids: &[String],
    device_id: &str,
    record_fqdn: &str,
    record_name: &str,
) -> bool {
    let normalized = side_type.trim().to_ascii_lowercase();
    match normalized.as_str() {
        "" | "all" | "any" | "network" | "workspace" | "current_device" => true,
        "device" => device_ids.iter().any(|item| item == device_id),
        "device_group" => group_ids.iter().any(|group_id| {
            runtime
                .groups_by_group_id
                .get(group_id)
                .is_some_and(|group| group.member_device_ids.iter().any(|item| item == device_id))
        }),
        "ip" | "cidr" | "subnet" => runtime_virtual_ip_for_device(runtime, device_id)
            .is_some_and(|member_ip| values.iter().any(|item| acl_ip_matches(item, &member_ip))),
        "domain" => {
            let fqdn = normalize_fqdn(record_fqdn);
            let name = normalize_fqdn(record_name);
            values.iter().any(|item| {
                let expected = normalize_fqdn(item);
                !expected.is_empty() && (expected == fqdn || expected == name)
            })
        }
        _ => false,
    }
}

fn runtime_virtual_ip_for_device(runtime: &RuntimeNetworkState, device_id: &str) -> Option<String> {
    let device_id = device_id.trim();
    if device_id.is_empty() {
        return None;
    }
    if let Some(member) = runtime.members_by_device_id.get(device_id) {
        let ip = normalize_virtual_ip(member.virtual_ip.as_str());
        if !ip.is_empty() {
            return Some(ip);
        }
    }
    let self_device_id = runtime
        .self_device_id
        .as_deref()
        .map(str::trim)
        .unwrap_or_default();
    if self_device_id == device_id {
        let self_ip = runtime
            .self_virtual_ip
            .as_deref()
            .map(normalize_virtual_ip)
            .unwrap_or_default();
        if !self_ip.is_empty() {
            return Some(self_ip);
        }
    }
    None
}

fn normalize_virtual_ip(value: &str) -> String {
    value
        .trim()
        .split_once('/')
        .map(|(ip, _)| ip)
        .unwrap_or_else(|| value.trim())
        .trim()
        .to_string()
}

fn acl_ip_matches(pattern: &str, ip: &str) -> bool {
    let value = pattern.trim();
    let ip = normalize_virtual_ip(ip);
    if value.is_empty() || value.eq_ignore_ascii_case("all") || value == "*" {
        return true;
    }
    if let Some((network, prefix)) = value.split_once('/') {
        return ipv4_cidr_contains(network.trim(), prefix.trim(), ip.as_str());
    }
    normalize_virtual_ip(value) == ip
}

fn ipv4_cidr_contains(network: &str, prefix: &str, ip: &str) -> bool {
    let Ok(prefix) = prefix.parse::<u32>() else {
        return false;
    };
    if prefix > 32 {
        return false;
    }
    let Ok(network) = network.parse::<std::net::Ipv4Addr>() else {
        return false;
    };
    let Ok(ip) = ip.parse::<std::net::Ipv4Addr>() else {
        return false;
    };
    let mask = if prefix == 0 {
        0
    } else {
        u32::MAX << (32 - prefix)
    };
    (u32::from(network) & mask) == (u32::from(ip) & mask)
}

fn cached_answer_to_result(cached: &CachedResolverAnswer) -> ResolveAuthoritativeResult {
    match cached.result_kind {
        CachedResolverResultKind::AnswerA => ResolveAuthoritativeResult::AnswerA {
            ttl: cached.ttl.unwrap_or(30),
            ips: cached.answers.clone(),
        },
        CachedResolverResultKind::AnswerAaaa => ResolveAuthoritativeResult::AnswerAaaa {
            ttl: cached.ttl.unwrap_or(30),
            ips: cached.answers.clone(),
        },
        CachedResolverResultKind::AnswerCname => ResolveAuthoritativeResult::AnswerCname {
            ttl: cached.ttl.unwrap_or(30),
            cname: cached.answers.first().cloned().unwrap_or_default(),
        },
        CachedResolverResultKind::AnswerPtr => ResolveAuthoritativeResult::AnswerPtr {
            ttl: cached.ttl.unwrap_or(30),
            name: cached.answers.first().cloned().unwrap_or_default(),
        },
        CachedResolverResultKind::AnswerTxt => ResolveAuthoritativeResult::AnswerTxt {
            ttl: cached.ttl.unwrap_or(30),
            texts: cached.answers.clone(),
        },
        CachedResolverResultKind::AnswerSrv => {
            let target = cached.answers.first().cloned().unwrap_or_default();
            let port = cached
                .answers
                .get(1)
                .and_then(|value| value.parse::<u16>().ok())
                .unwrap_or_default();
            ResolveAuthoritativeResult::AnswerSrv {
                ttl: cached.ttl.unwrap_or(30),
                port,
                target,
            }
        }
        CachedResolverResultKind::NxDomain => ResolveAuthoritativeResult::NxDomain,
        CachedResolverResultKind::NoData => ResolveAuthoritativeResult::NoData,
    }
}

fn cache_result(
    dns: &mut RuntimeResolverState,
    qname: &str,
    qtype: &str,
    result: &ResolveAuthoritativeResult,
) {
    let now_ms = current_timestamp_ms();
    let (result_kind, ttl, answers) = match result {
        ResolveAuthoritativeResult::AnswerA { ttl, ips } => {
            (CachedResolverResultKind::AnswerA, Some(*ttl), ips.clone())
        }
        ResolveAuthoritativeResult::AnswerAaaa { ttl, ips } => (
            CachedResolverResultKind::AnswerAaaa,
            Some(*ttl),
            ips.clone(),
        ),
        ResolveAuthoritativeResult::AnswerCname { ttl, cname } => (
            CachedResolverResultKind::AnswerCname,
            Some(*ttl),
            vec![cname.clone()],
        ),
        ResolveAuthoritativeResult::AnswerPtr { ttl, name } => (
            CachedResolverResultKind::AnswerPtr,
            Some(*ttl),
            vec![name.clone()],
        ),
        ResolveAuthoritativeResult::AnswerTxt { ttl, texts } => (
            CachedResolverResultKind::AnswerTxt,
            Some(*ttl),
            texts.clone(),
        ),
        ResolveAuthoritativeResult::AnswerSrv { ttl, port, target } => (
            CachedResolverResultKind::AnswerSrv,
            Some(*ttl),
            vec![target.clone(), port.to_string()],
        ),
        ResolveAuthoritativeResult::NxDomain => {
            (CachedResolverResultKind::NxDomain, Some(30), Vec::new())
        }
        ResolveAuthoritativeResult::NoData => {
            (CachedResolverResultKind::NoData, Some(30), Vec::new())
        }
        ResolveAuthoritativeResult::NotManaged => return,
    };
    let ttl_value = ttl.unwrap_or(30).max(1);
    dns.put_cached_answer(CachedResolverAnswer {
        qname: qname.to_string(),
        qtype: qtype.to_string(),
        result_kind,
        ttl: Some(ttl_value),
        answers,
        expires_at_ms: now_ms.saturating_add(u64::from(ttl_value) * 1_000),
    });
}

#[cfg(test)]
mod tests {
    use super::{
        acl_side_matches, resolve_authoritative, route_resolver_query, ResolveAuthoritativeResult,
        ResolverRouteDecision,
    };
    use crate::{
        network_event::NetworkEventMemberView,
        network_runtime_state::RuntimeNetworkState,
        resolver_runtime_state::{ResolverRecordView, ResolverZoneView, RuntimeResolverState},
    };

    #[test]
    fn short_name_resolves_via_zone_search_suffix() {
        let mut dns = RuntimeResolverState::default();
        dns.replace_zones(vec![ResolverZoneView {
            zone_id: "zone-1".to_string(),
            network_id: "net-1".to_string(),
            zone_name: "example.lan".to_string(),
            expose_global: false,
            updated_at: 0,
        }]);
        dns.replace_records(vec![ResolverRecordView {
            record_id: "rec-1".to_string(),
            zone_id: "zone-1".to_string(),
            network_id: "net-1".to_string(),
            name: "self".to_string(),
            fqdn: "self.example.lan".to_string(),
            record_type: "A".to_string(),
            value: String::new(),
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

        let result = resolve_authoritative(&runtime, &mut dns, "device-1", "self", "A");
        assert_eq!(
            result,
            ResolveAuthoritativeResult::AnswerA {
                ttl: 60,
                ips: vec!["10.0.0.99".to_string()]
            }
        );
    }

    #[test]
    fn full_fqdn_still_resolves_normally() {
        let mut dns = RuntimeResolverState::default();
        dns.replace_zones(vec![ResolverZoneView {
            zone_id: "zone-1".to_string(),
            network_id: "net-1".to_string(),
            zone_name: "example.lan".to_string(),
            expose_global: false,
            updated_at: 0,
        }]);
        dns.replace_records(vec![ResolverRecordView {
            record_id: "rec-1".to_string(),
            zone_id: "zone-1".to_string(),
            network_id: "net-1".to_string(),
            name: "self".to_string(),
            fqdn: "self.example.lan".to_string(),
            record_type: "A".to_string(),
            value: String::new(),
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

        let result = resolve_authoritative(&runtime, &mut dns, "device-1", "self.example.lan", "A");
        assert_eq!(
            result,
            ResolveAuthoritativeResult::AnswerA {
                ttl: 60,
                ips: vec!["10.0.0.99".to_string()]
            }
        );
    }

    #[test]
    fn explicit_ipv6_record_resolves_as_aaaa() {
        let mut dns = RuntimeResolverState::default();
        dns.replace_zones(vec![ResolverZoneView {
            zone_id: "zone-1".to_string(),
            network_id: "net-1".to_string(),
            zone_name: "example.lan".to_string(),
            expose_global: false,
            updated_at: 0,
        }]);
        dns.replace_records(vec![ResolverRecordView {
            record_id: "rec-1".to_string(),
            zone_id: "zone-1".to_string(),
            network_id: "net-1".to_string(),
            name: "v6".to_string(),
            fqdn: "v6.example.lan".to_string(),
            record_type: "AAAA".to_string(),
            value: String::new(),
            target_device_id: "device-1".to_string(),
            target_ip: "2001:db8::10".to_string(),
            cname: String::new(),
            port: 0,
            ttl: 120,
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

        let result = resolve_authoritative(&runtime, &mut dns, "device-1", "v6", "AAAA");
        assert_eq!(
            result,
            ResolveAuthoritativeResult::AnswerAaaa {
                ttl: 120,
                ips: vec!["2001:db8::10".to_string()]
            }
        );
    }

    #[test]
    fn answer_is_cached_after_first_lookup() {
        let mut dns = RuntimeResolverState::default();
        dns.replace_zones(vec![ResolverZoneView {
            zone_id: "zone-1".to_string(),
            network_id: "net-1".to_string(),
            zone_name: "example.lan".to_string(),
            expose_global: false,
            updated_at: 0,
        }]);
        dns.replace_records(vec![ResolverRecordView {
            record_id: "rec-1".to_string(),
            zone_id: "zone-1".to_string(),
            network_id: "net-1".to_string(),
            name: "self".to_string(),
            fqdn: "self.example.lan".to_string(),
            record_type: "A".to_string(),
            value: String::new(),
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

        let first = resolve_authoritative(&runtime, &mut dns, "device-1", "self", "A");
        assert!(matches!(first, ResolveAuthoritativeResult::AnswerA { .. }));
        assert_eq!(dns.cache.cache_by_question.len(), 1);

        dns.authority.records_by_id.clear();
        dns.authority.record_ids_by_fqdn.clear();

        let second = resolve_authoritative(&runtime, &mut dns, "device-1", "self", "A");
        assert_eq!(second, first);
    }

    #[test]
    fn route_resolver_query_prefers_authoritative_for_managed_domain() {
        let mut dns = RuntimeResolverState::default();
        dns.replace_zones(vec![ResolverZoneView {
            zone_id: "zone-1".to_string(),
            network_id: "net-1".to_string(),
            zone_name: "example.lan".to_string(),
            expose_global: false,
            updated_at: 0,
        }]);
        dns.replace_records(vec![ResolverRecordView {
            record_id: "rec-1".to_string(),
            zone_id: "zone-1".to_string(),
            network_id: "net-1".to_string(),
            name: "peer".to_string(),
            fqdn: "peer.example.lan".to_string(),
            record_type: "A".to_string(),
            value: String::new(),
            target_device_id: "device-1".to_string(),
            target_ip: "10.0.0.9".to_string(),
            cname: String::new(),
            port: 0,
            ttl: 60,
            enabled: true,
            updated_at: 0,
        }]);

        assert_eq!(
            route_resolver_query(&dns, "peer.example.lan"),
            ResolverRouteDecision::Authoritative
        );
    }

    #[test]
    fn route_resolver_query_falls_back_to_upstream_for_unmanaged_domain() {
        let mut dns = RuntimeResolverState::default();
        dns.set_upstream_servers(vec!["1.1.1.1:53".to_string()]);

        assert_eq!(
            route_resolver_query(&dns, "www.example.com"),
            ResolverRouteDecision::ForwardUpstream
        );
    }

    #[test]
    fn deny_acl_hides_record_from_requester() {
        let mut dns = RuntimeResolverState::default();
        dns.replace_zones(vec![ResolverZoneView {
            zone_id: "zone-1".to_string(),
            network_id: "net-1".to_string(),
            zone_name: "example.lan".to_string(),
            expose_global: false,
            updated_at: 0,
        }]);
        dns.replace_records(vec![ResolverRecordView {
            record_id: "rec-1".to_string(),
            zone_id: "zone-1".to_string(),
            network_id: "net-1".to_string(),
            name: "peer".to_string(),
            fqdn: "peer.example.lan".to_string(),
            record_type: "A".to_string(),
            value: String::new(),
            target_device_id: "device-2".to_string(),
            target_ip: String::new(),
            cname: String::new(),
            port: 0,
            ttl: 60,
            enabled: true,
            updated_at: 0,
        }]);
        let mut runtime = RuntimeNetworkState::default();
        runtime.network = Some(crate::network_event::NetworkEventNetworkView {
            network_id: "net-1".to_string(),
            default_acl_policy: "allow".to_string(),
            ..crate::network_event::NetworkEventNetworkView::default()
        });
        runtime.members_by_device_id.insert(
            "device-1".to_string(),
            NetworkEventMemberView {
                device_id: "device-1".to_string(),
                virtual_ip: "10.0.0.10".to_string(),
                ..NetworkEventMemberView::default()
            },
        );
        runtime.members_by_device_id.insert(
            "device-2".to_string(),
            NetworkEventMemberView {
                device_id: "device-2".to_string(),
                virtual_ip: "10.0.0.20".to_string(),
                ..NetworkEventMemberView::default()
            },
        );
        runtime.acl_by_rule_id.insert(
            "rule-1".to_string(),
            crate::network_event::NetworkEventAclRuleView {
                rule_id: "rule-1".to_string(),
                priority: 10,
                action: "deny".to_string(),
                direction: "egress".to_string(),
                source_type: "device".to_string(),
                source_device_ids: vec!["device-1".to_string()],
                target_type: "device".to_string(),
                target_device_ids: vec!["device-2".to_string()],
                enabled: true,
                ..crate::network_event::NetworkEventAclRuleView::default()
            },
        );

        let result = resolve_authoritative(&runtime, &mut dns, "device-1", "peer", "A");
        assert_eq!(result, ResolveAuthoritativeResult::NxDomain);
    }

    #[test]
    fn device_group_allow_acl_exposes_record_to_requester() {
        let mut dns = RuntimeResolverState::default();
        dns.replace_zones(vec![ResolverZoneView {
            zone_id: "zone-1".to_string(),
            network_id: "net-1".to_string(),
            zone_name: "example.lan".to_string(),
            expose_global: false,
            updated_at: 0,
        }]);
        dns.replace_records(vec![ResolverRecordView {
            record_id: "rec-1".to_string(),
            zone_id: "zone-1".to_string(),
            network_id: "net-1".to_string(),
            name: "peer".to_string(),
            fqdn: "peer.example.lan".to_string(),
            record_type: "A".to_string(),
            value: String::new(),
            target_device_id: "device-2".to_string(),
            target_ip: String::new(),
            cname: String::new(),
            port: 0,
            ttl: 60,
            enabled: true,
            updated_at: 0,
        }]);
        let mut runtime = RuntimeNetworkState::default();
        runtime.network = Some(crate::network_event::NetworkEventNetworkView {
            network_id: "net-1".to_string(),
            default_acl_policy: "deny".to_string(),
            ..crate::network_event::NetworkEventNetworkView::default()
        });
        runtime.members_by_device_id.insert(
            "device-1".to_string(),
            NetworkEventMemberView {
                device_id: "device-1".to_string(),
                virtual_ip: "10.0.0.10".to_string(),
                ..NetworkEventMemberView::default()
            },
        );
        runtime.members_by_device_id.insert(
            "device-2".to_string(),
            NetworkEventMemberView {
                device_id: "device-2".to_string(),
                virtual_ip: "10.0.0.20".to_string(),
                ..NetworkEventMemberView::default()
            },
        );
        runtime.groups_by_group_id.insert(
            "group-1".to_string(),
            crate::network_event::NetworkEventDeviceGroupView {
                group_id: "group-1".to_string(),
                member_device_ids: vec!["device-1".to_string()],
                ..crate::network_event::NetworkEventDeviceGroupView::default()
            },
        );
        runtime.groups_by_group_id.insert(
            "group-2".to_string(),
            crate::network_event::NetworkEventDeviceGroupView {
                group_id: "group-2".to_string(),
                member_device_ids: vec!["device-2".to_string()],
                ..crate::network_event::NetworkEventDeviceGroupView::default()
            },
        );
        runtime.acl_by_rule_id.insert(
            "rule-1".to_string(),
            crate::network_event::NetworkEventAclRuleView {
                rule_id: "rule-1".to_string(),
                priority: 10,
                action: "allow".to_string(),
                direction: "egress".to_string(),
                source_type: "device_group".to_string(),
                source_group_ids: vec!["group-1".to_string()],
                target_type: "device_group".to_string(),
                target_group_ids: vec!["group-2".to_string()],
                enabled: true,
                ..crate::network_event::NetworkEventAclRuleView::default()
            },
        );
        runtime.acl_by_rule_id.insert(
            "rule-2".to_string(),
            crate::network_event::NetworkEventAclRuleView {
                rule_id: "rule-2".to_string(),
                priority: 10,
                action: "allow".to_string(),
                direction: "ingress".to_string(),
                source_type: "device_group".to_string(),
                source_group_ids: vec!["group-1".to_string()],
                target_type: "device_group".to_string(),
                target_group_ids: vec!["group-2".to_string()],
                enabled: true,
                ..crate::network_event::NetworkEventAclRuleView::default()
            },
        );

        let result = resolve_authoritative(&runtime, &mut dns, "device-1", "peer", "A");
        assert_eq!(
            result,
            ResolveAuthoritativeResult::AnswerA {
                ttl: 60,
                ips: vec!["10.0.0.20".to_string()]
            }
        );
    }

    #[test]
    fn current_device_acl_rule_exposes_target_peer_record() {
        let mut dns = RuntimeResolverState::default();
        dns.replace_zones(vec![ResolverZoneView {
            zone_id: "zone-1".to_string(),
            network_id: "net-1".to_string(),
            zone_name: "example.lan".to_string(),
            expose_global: false,
            updated_at: 0,
        }]);
        dns.replace_records(vec![ResolverRecordView {
            record_id: "rec-1".to_string(),
            zone_id: "zone-1".to_string(),
            network_id: "net-1".to_string(),
            name: "peer".to_string(),
            fqdn: "peer.example.lan".to_string(),
            record_type: "A".to_string(),
            value: String::new(),
            target_device_id: "device-2".to_string(),
            target_ip: String::new(),
            cname: String::new(),
            port: 0,
            ttl: 60,
            enabled: true,
            updated_at: 0,
        }]);
        let mut runtime = RuntimeNetworkState::default();
        runtime.network = Some(crate::network_event::NetworkEventNetworkView {
            network_id: "net-1".to_string(),
            default_acl_policy: "deny".to_string(),
            ..crate::network_event::NetworkEventNetworkView::default()
        });
        runtime.members_by_device_id.insert(
            "device-1".to_string(),
            NetworkEventMemberView {
                device_id: "device-1".to_string(),
                virtual_ip: "10.0.0.10".to_string(),
                ..NetworkEventMemberView::default()
            },
        );
        runtime.members_by_device_id.insert(
            "device-2".to_string(),
            NetworkEventMemberView {
                device_id: "device-2".to_string(),
                virtual_ip: "10.0.0.20".to_string(),
                ..NetworkEventMemberView::default()
            },
        );
        runtime.acl_by_rule_id.insert(
            "rule-egress".to_string(),
            crate::network_event::NetworkEventAclRuleView {
                rule_id: "rule-egress".to_string(),
                priority: 10,
                action: "allow".to_string(),
                direction: "egress".to_string(),
                source_type: "device".to_string(),
                source_device_ids: vec!["device-2".to_string()],
                target_type: "current_device".to_string(),
                enabled: true,
                ..crate::network_event::NetworkEventAclRuleView::default()
            },
        );
        runtime.acl_by_rule_id.insert(
            "rule-ingress".to_string(),
            crate::network_event::NetworkEventAclRuleView {
                rule_id: "rule-ingress".to_string(),
                priority: 10,
                action: "allow".to_string(),
                direction: "ingress".to_string(),
                source_type: "device".to_string(),
                source_device_ids: vec!["device-2".to_string()],
                target_type: "current_device".to_string(),
                enabled: true,
                ..crate::network_event::NetworkEventAclRuleView::default()
            },
        );

        let result = resolve_authoritative(&runtime, &mut dns, "device-1", "peer", "A");
        assert_eq!(
            result,
            ResolveAuthoritativeResult::AnswerA {
                ttl: 60,
                ips: vec!["10.0.0.20".to_string()]
            }
        );
    }

    #[test]
    fn cidr_acl_rule_matches_target_virtual_ip() {
        let mut dns = RuntimeResolverState::default();
        dns.replace_zones(vec![ResolverZoneView {
            zone_id: "zone-1".to_string(),
            network_id: "net-1".to_string(),
            zone_name: "example.lan".to_string(),
            expose_global: false,
            updated_at: 0,
        }]);
        dns.replace_records(vec![ResolverRecordView {
            record_id: "rec-1".to_string(),
            zone_id: "zone-1".to_string(),
            network_id: "net-1".to_string(),
            name: "peer".to_string(),
            fqdn: "peer.example.lan".to_string(),
            record_type: "A".to_string(),
            value: String::new(),
            target_device_id: "device-2".to_string(),
            target_ip: String::new(),
            cname: String::new(),
            port: 0,
            ttl: 60,
            enabled: true,
            updated_at: 0,
        }]);
        let mut runtime = RuntimeNetworkState::default();
        runtime.network = Some(crate::network_event::NetworkEventNetworkView {
            network_id: "net-1".to_string(),
            default_acl_policy: "deny".to_string(),
            ..crate::network_event::NetworkEventNetworkView::default()
        });
        runtime.members_by_device_id.insert(
            "device-1".to_string(),
            NetworkEventMemberView {
                device_id: "device-1".to_string(),
                virtual_ip: "10.0.0.10".to_string(),
                ..NetworkEventMemberView::default()
            },
        );
        runtime.members_by_device_id.insert(
            "device-2".to_string(),
            NetworkEventMemberView {
                device_id: "device-2".to_string(),
                virtual_ip: "10.0.0.20/24".to_string(),
                ..NetworkEventMemberView::default()
            },
        );
        for (rule_id, direction) in [("rule-egress", "egress"), ("rule-ingress", "ingress")] {
            runtime.acl_by_rule_id.insert(
                rule_id.to_string(),
                crate::network_event::NetworkEventAclRuleView {
                    rule_id: rule_id.to_string(),
                    priority: 10,
                    action: "allow".to_string(),
                    direction: direction.to_string(),
                    source_type: "cidr".to_string(),
                    source_values: vec!["10.0.0.0/24".to_string()],
                    target_type: "current_device".to_string(),
                    enabled: true,
                    ..crate::network_event::NetworkEventAclRuleView::default()
                },
            );
        }

        let result = resolve_authoritative(&runtime, &mut dns, "device-1", "peer", "A");
        assert_eq!(
            result,
            ResolveAuthoritativeResult::AnswerA {
                ttl: 60,
                ips: vec!["10.0.0.20".to_string()]
            }
        );
    }

    #[test]
    fn domain_acl_rule_matches_record_fqdn() {
        let mut dns = RuntimeResolverState::default();
        dns.replace_zones(vec![ResolverZoneView {
            zone_id: "zone-1".to_string(),
            network_id: "net-1".to_string(),
            zone_name: "example.lan".to_string(),
            expose_global: false,
            updated_at: 0,
        }]);
        dns.replace_records(vec![ResolverRecordView {
            record_id: "rec-1".to_string(),
            zone_id: "zone-1".to_string(),
            network_id: "net-1".to_string(),
            name: "peer".to_string(),
            fqdn: "peer.example.lan".to_string(),
            record_type: "A".to_string(),
            value: String::new(),
            target_device_id: "device-2".to_string(),
            target_ip: String::new(),
            cname: String::new(),
            port: 0,
            ttl: 60,
            enabled: true,
            updated_at: 0,
        }]);
        let mut runtime = RuntimeNetworkState::default();
        runtime.network = Some(crate::network_event::NetworkEventNetworkView {
            network_id: "net-1".to_string(),
            default_acl_policy: "deny".to_string(),
            ..crate::network_event::NetworkEventNetworkView::default()
        });
        runtime.members_by_device_id.insert(
            "device-1".to_string(),
            NetworkEventMemberView {
                device_id: "device-1".to_string(),
                virtual_ip: "10.0.0.10".to_string(),
                ..NetworkEventMemberView::default()
            },
        );
        runtime.members_by_device_id.insert(
            "device-2".to_string(),
            NetworkEventMemberView {
                device_id: "device-2".to_string(),
                virtual_ip: "10.0.0.20".to_string(),
                ..NetworkEventMemberView::default()
            },
        );
        for (rule_id, direction) in [("rule-egress", "egress"), ("rule-ingress", "ingress")] {
            runtime.acl_by_rule_id.insert(
                rule_id.to_string(),
                crate::network_event::NetworkEventAclRuleView {
                    rule_id: rule_id.to_string(),
                    priority: 10,
                    action: "allow".to_string(),
                    direction: direction.to_string(),
                    source_type: "domain".to_string(),
                    source_values: vec!["peer.example.lan".to_string()],
                    target_type: "current_device".to_string(),
                    enabled: true,
                    ..crate::network_event::NetworkEventAclRuleView::default()
                },
            );
        }

        let result = resolve_authoritative(&runtime, &mut dns, "device-1", "peer", "A");
        assert_eq!(
            result,
            ResolveAuthoritativeResult::AnswerA {
                ttl: 60,
                ips: vec!["10.0.0.20".to_string()]
            }
        );
    }

    #[test]
    fn domain_acl_rule_does_not_match_other_record_name() {
        let mut dns = RuntimeResolverState::default();
        dns.replace_zones(vec![ResolverZoneView {
            zone_id: "zone-1".to_string(),
            network_id: "net-1".to_string(),
            zone_name: "example.lan".to_string(),
            expose_global: false,
            updated_at: 0,
        }]);
        dns.replace_records(vec![ResolverRecordView {
            record_id: "rec-1".to_string(),
            zone_id: "zone-1".to_string(),
            network_id: "net-1".to_string(),
            name: "peer".to_string(),
            fqdn: "peer.example.lan".to_string(),
            record_type: "A".to_string(),
            value: String::new(),
            target_device_id: "device-2".to_string(),
            target_ip: String::new(),
            cname: String::new(),
            port: 0,
            ttl: 60,
            enabled: true,
            updated_at: 0,
        }]);
        let mut runtime = RuntimeNetworkState::default();
        runtime.network = Some(crate::network_event::NetworkEventNetworkView {
            network_id: "net-1".to_string(),
            default_acl_policy: "deny".to_string(),
            ..crate::network_event::NetworkEventNetworkView::default()
        });
        runtime.members_by_device_id.insert(
            "device-1".to_string(),
            NetworkEventMemberView {
                device_id: "device-1".to_string(),
                virtual_ip: "10.0.0.10".to_string(),
                ..NetworkEventMemberView::default()
            },
        );
        runtime.members_by_device_id.insert(
            "device-2".to_string(),
            NetworkEventMemberView {
                device_id: "device-2".to_string(),
                virtual_ip: "10.0.0.20".to_string(),
                ..NetworkEventMemberView::default()
            },
        );
        for (rule_id, direction) in [("rule-egress", "egress"), ("rule-ingress", "ingress")] {
            runtime.acl_by_rule_id.insert(
                rule_id.to_string(),
                crate::network_event::NetworkEventAclRuleView {
                    rule_id: rule_id.to_string(),
                    priority: 10,
                    action: "allow".to_string(),
                    direction: direction.to_string(),
                    source_type: "domain".to_string(),
                    source_values: vec!["other.example.lan".to_string()],
                    target_type: "current_device".to_string(),
                    enabled: true,
                    ..crate::network_event::NetworkEventAclRuleView::default()
                },
            );
        }

        let result = resolve_authoritative(&runtime, &mut dns, "device-1", "peer", "A");
        assert_eq!(result, ResolveAuthoritativeResult::NxDomain);
    }

    #[test]
    fn ptr_record_resolves_from_value() {
        let mut dns = RuntimeResolverState::default();
        dns.replace_zones(vec![ResolverZoneView {
            zone_id: "zone-1".to_string(),
            network_id: "net-1".to_string(),
            zone_name: "arpa".to_string(),
            expose_global: false,
            updated_at: 0,
        }]);
        dns.replace_records(vec![ResolverRecordView {
            record_id: "rec-ptr".to_string(),
            zone_id: "zone-1".to_string(),
            network_id: "net-1".to_string(),
            name: "4.3.2.1.in-addr".to_string(),
            fqdn: "4.3.2.1.in-addr.arpa".to_string(),
            record_type: "PTR".to_string(),
            value: "peer.example.lan".to_string(),
            target_device_id: "device-1".to_string(),
            target_ip: String::new(),
            cname: String::new(),
            port: 0,
            ttl: 45,
            enabled: true,
            updated_at: 0,
        }]);
        let mut runtime = RuntimeNetworkState::default();
        runtime.members_by_device_id.insert(
            "device-1".to_string(),
            NetworkEventMemberView {
                device_id: "device-1".to_string(),
                virtual_ip: "10.0.0.10".to_string(),
                ..NetworkEventMemberView::default()
            },
        );

        let result = resolve_authoritative(
            &runtime,
            &mut dns,
            "device-1",
            "4.3.2.1.in-addr.arpa",
            "PTR",
        );
        assert_eq!(
            result,
            ResolveAuthoritativeResult::AnswerPtr {
                ttl: 45,
                name: "peer.example.lan".to_string()
            }
        );
    }

    #[test]
    fn txt_record_resolves_from_value() {
        let mut dns = RuntimeResolverState::default();
        dns.replace_zones(vec![ResolverZoneView {
            zone_id: "zone-1".to_string(),
            network_id: "net-1".to_string(),
            zone_name: "example.lan".to_string(),
            expose_global: false,
            updated_at: 0,
        }]);
        dns.replace_records(vec![ResolverRecordView {
            record_id: "rec-txt".to_string(),
            zone_id: "zone-1".to_string(),
            network_id: "net-1".to_string(),
            name: "txt".to_string(),
            fqdn: "txt.example.lan".to_string(),
            record_type: "TXT".to_string(),
            value: "hello-slan".to_string(),
            target_device_id: "device-1".to_string(),
            target_ip: String::new(),
            cname: String::new(),
            port: 0,
            ttl: 90,
            enabled: true,
            updated_at: 0,
        }]);
        let mut runtime = RuntimeNetworkState::default();
        runtime.members_by_device_id.insert(
            "device-1".to_string(),
            NetworkEventMemberView {
                device_id: "device-1".to_string(),
                virtual_ip: "10.0.0.10".to_string(),
                ..NetworkEventMemberView::default()
            },
        );

        let result = resolve_authoritative(&runtime, &mut dns, "device-1", "txt", "TXT");
        assert_eq!(
            result,
            ResolveAuthoritativeResult::AnswerTxt {
                ttl: 90,
                texts: vec!["hello-slan".to_string()]
            }
        );
    }

    #[test]
    fn multiple_a_records_resolve_as_multi_answer() {
        let mut dns = RuntimeResolverState::default();
        dns.replace_zones(vec![ResolverZoneView {
            zone_id: "zone-1".to_string(),
            network_id: "net-1".to_string(),
            zone_name: "example.lan".to_string(),
            expose_global: false,
            updated_at: 0,
        }]);
        dns.replace_records(vec![
            ResolverRecordView {
                record_id: "rec-1".to_string(),
                zone_id: "zone-1".to_string(),
                network_id: "net-1".to_string(),
                name: "peer".to_string(),
                fqdn: "peer.example.lan".to_string(),
                record_type: "A".to_string(),
                value: String::new(),
                target_device_id: "device-1".to_string(),
                target_ip: "10.0.0.21".to_string(),
                cname: String::new(),
                port: 0,
                ttl: 60,
                enabled: true,
                updated_at: 0,
            },
            ResolverRecordView {
                record_id: "rec-2".to_string(),
                zone_id: "zone-1".to_string(),
                network_id: "net-1".to_string(),
                name: "peer".to_string(),
                fqdn: "peer.example.lan".to_string(),
                record_type: "A".to_string(),
                value: String::new(),
                target_device_id: "device-1".to_string(),
                target_ip: "10.0.0.22".to_string(),
                cname: String::new(),
                port: 0,
                ttl: 60,
                enabled: true,
                updated_at: 0,
            },
        ]);

        let mut runtime = RuntimeNetworkState::default();
        runtime.members_by_device_id.insert(
            "device-1".to_string(),
            NetworkEventMemberView {
                device_id: "device-1".to_string(),
                virtual_ip: "10.0.0.10".to_string(),
                ..NetworkEventMemberView::default()
            },
        );

        let result = resolve_authoritative(&runtime, &mut dns, "device-1", "peer", "A");
        assert_eq!(
            result,
            ResolveAuthoritativeResult::AnswerA {
                ttl: 60,
                ips: vec!["10.0.0.21".to_string(), "10.0.0.22".to_string()]
            }
        );
    }

    #[test]
    fn multiple_txt_records_resolve_as_multi_answer() {
        let mut dns = RuntimeResolverState::default();
        dns.replace_zones(vec![ResolverZoneView {
            zone_id: "zone-1".to_string(),
            network_id: "net-1".to_string(),
            zone_name: "example.lan".to_string(),
            expose_global: false,
            updated_at: 0,
        }]);
        dns.replace_records(vec![
            ResolverRecordView {
                record_id: "rec-1".to_string(),
                zone_id: "zone-1".to_string(),
                network_id: "net-1".to_string(),
                name: "txt".to_string(),
                fqdn: "txt.example.lan".to_string(),
                record_type: "TXT".to_string(),
                value: "hello".to_string(),
                target_device_id: "device-1".to_string(),
                target_ip: String::new(),
                cname: String::new(),
                port: 0,
                ttl: 60,
                enabled: true,
                updated_at: 0,
            },
            ResolverRecordView {
                record_id: "rec-2".to_string(),
                zone_id: "zone-1".to_string(),
                network_id: "net-1".to_string(),
                name: "txt".to_string(),
                fqdn: "txt.example.lan".to_string(),
                record_type: "TXT".to_string(),
                value: "world".to_string(),
                target_device_id: "device-1".to_string(),
                target_ip: String::new(),
                cname: String::new(),
                port: 0,
                ttl: 60,
                enabled: true,
                updated_at: 0,
            },
        ]);

        let mut runtime = RuntimeNetworkState::default();
        runtime.members_by_device_id.insert(
            "device-1".to_string(),
            NetworkEventMemberView {
                device_id: "device-1".to_string(),
                virtual_ip: "10.0.0.10".to_string(),
                ..NetworkEventMemberView::default()
            },
        );

        let result = resolve_authoritative(&runtime, &mut dns, "device-1", "txt", "TXT");
        assert_eq!(
            result,
            ResolveAuthoritativeResult::AnswerTxt {
                ttl: 60,
                texts: vec!["hello".to_string(), "world".to_string()]
            }
        );
    }

    #[test]
    fn srv_record_resolves_from_value_and_port() {
        let mut dns = RuntimeResolverState::default();
        dns.replace_zones(vec![ResolverZoneView {
            zone_id: "zone-1".to_string(),
            network_id: "net-1".to_string(),
            zone_name: "example.lan".to_string(),
            expose_global: false,
            updated_at: 0,
        }]);
        dns.replace_records(vec![ResolverRecordView {
            record_id: "rec-srv".to_string(),
            zone_id: "zone-1".to_string(),
            network_id: "net-1".to_string(),
            name: "_sip._tcp".to_string(),
            fqdn: "_sip._tcp.example.lan".to_string(),
            record_type: "SRV".to_string(),
            value: "peer.example.lan".to_string(),
            target_device_id: "device-1".to_string(),
            target_ip: String::new(),
            cname: String::new(),
            port: 5060,
            ttl: 120,
            enabled: true,
            updated_at: 0,
        }]);
        let mut runtime = RuntimeNetworkState::default();
        runtime.members_by_device_id.insert(
            "device-1".to_string(),
            NetworkEventMemberView {
                device_id: "device-1".to_string(),
                virtual_ip: "10.0.0.10".to_string(),
                ..NetworkEventMemberView::default()
            },
        );

        let result = resolve_authoritative(
            &runtime,
            &mut dns,
            "device-1",
            "_sip._tcp.example.lan",
            "SRV",
        );
        assert_eq!(
            result,
            ResolveAuthoritativeResult::AnswerSrv {
                ttl: 120,
                port: 5060,
                target: "peer.example.lan".to_string()
            }
        );
    }

    #[test]
    fn self_target_a_record_falls_back_to_session_virtual_ip() {
        let mut dns = RuntimeResolverState::default();
        dns.replace_zones(vec![ResolverZoneView {
            zone_id: "zone-1".to_string(),
            network_id: "net-1".to_string(),
            zone_name: "example.lan".to_string(),
            expose_global: false,
            updated_at: 0,
        }]);
        dns.replace_records(vec![ResolverRecordView {
            record_id: "rec-self".to_string(),
            zone_id: "zone-1".to_string(),
            network_id: "net-1".to_string(),
            name: "self".to_string(),
            fqdn: "self.example.lan".to_string(),
            record_type: "A".to_string(),
            value: String::new(),
            target_device_id: "device-1".to_string(),
            target_ip: String::new(),
            cname: String::new(),
            port: 0,
            ttl: 30,
            enabled: true,
            updated_at: 0,
        }]);
        let mut runtime = RuntimeNetworkState::default();
        runtime.bind_session_identity(Some("device-1"), Some("10.0.0.151"));

        let result = resolve_authoritative(&runtime, &mut dns, "device-1", "self", "A");
        assert_eq!(
            result,
            ResolveAuthoritativeResult::AnswerA {
                ttl: 30,
                ips: vec!["10.0.0.151".to_string()],
            }
        );
    }

    #[test]
    fn cidr_acl_matches_self_virtual_ip_from_session_state() {
        let runtime = RuntimeNetworkState {
            self_device_id: Some("device-1".to_string()),
            self_virtual_ip: Some("10.0.0.151".to_string()),
            ..RuntimeNetworkState::default()
        };

        assert!(acl_side_matches(
            &runtime,
            "cidr",
            &["10.0.0.0/24".to_string()],
            &[],
            &[],
            "device-1",
            "self.example.lan",
            "self",
        ));
    }
}
