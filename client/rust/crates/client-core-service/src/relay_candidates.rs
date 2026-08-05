use std::{
    net::{TcpStream, ToSocketAddrs, UdpSocket},
    sync::{Mutex, OnceLock},
    time::{Duration, Instant},
};

use serde_json::Value;

use client_core::{normalize_relay_transport, relay_path_kind_for_transport};

use crate::{
    control_plane::{ControlEndpoint, ControlPeer},
    relay_models::{PathDiagnoseDirectCandidate, PersistedRelayCandidate, RelayCandidateSelection},
};

const MAX_ACTIVE_RELAY_PROBES: usize = 4;

static RUNTIME_RELAY_CANDIDATES: OnceLock<Mutex<Vec<PersistedRelayCandidate>>> = OnceLock::new();
static RUNTIME_DEVICE_LOCATION: OnceLock<Mutex<RuntimeDeviceLocation>> = OnceLock::new();
static TEST_RELAY_TRANSPORT_ALLOWLIST: OnceLock<Mutex<Option<Vec<String>>>> = OnceLock::new();

#[derive(Clone, Default)]
struct RuntimeDeviceLocation {
    country_code: Option<String>,
    city_code: Option<String>,
}

fn runtime_store() -> &'static Mutex<Vec<PersistedRelayCandidate>> {
    RUNTIME_RELAY_CANDIDATES.get_or_init(|| Mutex::new(Vec::new()))
}

fn device_location_store() -> &'static Mutex<RuntimeDeviceLocation> {
    RUNTIME_DEVICE_LOCATION.get_or_init(|| Mutex::new(RuntimeDeviceLocation::default()))
}

pub(crate) fn update_runtime_device_location(country_code: &str, city_code: &str) {
    let normalized = RuntimeDeviceLocation {
        country_code: normalized_location_value(country_code, true),
        city_code: normalized_location_value(city_code, false),
    };
    *device_location_store()
        .lock()
        .expect("runtime device location mutex poisoned") = normalized;
}

pub(crate) fn runtime_device_country_code() -> Option<String> {
    device_location_store()
        .lock()
        .expect("runtime device location mutex poisoned")
        .country_code
        .clone()
}

pub(crate) fn infrastructure_location_rank(
    country_code: &str,
    city_code: &str,
    peer_country_code: &str,
    peer_city_code: &str,
) -> u8 {
    let location = device_location_store()
        .lock()
        .expect("runtime device location mutex poisoned")
        .clone();
    let local_country = location.country_code.as_deref().unwrap_or_default();
    let local_city = location.city_code.as_deref().unwrap_or_default();
    let country = country_code.trim();
    let city = city_code.trim();
    let peer_country = peer_country_code.trim();
    let peer_city = peer_city_code.trim();
    let crosses_mainland_border = !local_country.is_empty()
        && !peer_country.is_empty()
        && local_country.eq_ignore_ascii_case("CN") != peer_country.eq_ignore_ascii_case("CN");
    if crosses_mainland_border && country.eq_ignore_ascii_case("HK") {
        return 0;
    }
    if (!local_city.is_empty() && city.eq_ignore_ascii_case(local_city))
        || (!peer_city.is_empty() && city.eq_ignore_ascii_case(peer_city))
    {
        return 1;
    }
    if (!local_country.is_empty() && country.eq_ignore_ascii_case(local_country))
        || (!peer_country.is_empty() && country.eq_ignore_ascii_case(peer_country))
    {
        return 2;
    }
    3
}

pub(crate) fn relay_candidates_for_peer(
    candidates: &[RelayCandidateSelection],
    peer_country_code: &str,
    peer_city_code: &str,
) -> Vec<RelayCandidateSelection> {
    let mut ranked = candidates.to_vec();
    ranked.sort_by(|left, right| {
        right
            .reachable
            .cmp(&left.reachable)
            .then_with(|| {
                relay_transport_rank(&left.transport).cmp(&relay_transport_rank(&right.transport))
            })
            .then_with(|| {
                infrastructure_location_rank(
                    left.country_code.as_deref().unwrap_or_default(),
                    left.city_code.as_deref().unwrap_or_default(),
                    peer_country_code,
                    peer_city_code,
                )
                .cmp(&infrastructure_location_rank(
                    right.country_code.as_deref().unwrap_or_default(),
                    right.city_code.as_deref().unwrap_or_default(),
                    peer_country_code,
                    peer_city_code,
                ))
            })
            .then_with(|| left.configured_priority.cmp(&right.configured_priority))
            .then_with(|| left.endpoint_id.cmp(&right.endpoint_id))
            .then_with(|| left.path_score.cmp(&right.path_score))
            .then_with(|| {
                left.rtt_ms
                    .unwrap_or(u32::MAX)
                    .cmp(&right.rtt_ms.unwrap_or(u32::MAX))
            })
    });
    ranked
}

fn normalized_location_value(value: &str, uppercase: bool) -> Option<String> {
    let value = value.trim();
    if value.is_empty() {
        return None;
    }
    Some(if uppercase {
        value.to_ascii_uppercase()
    } else {
        value.to_string()
    })
}

fn test_allowlist_store() -> &'static Mutex<Option<Vec<String>>> {
    TEST_RELAY_TRANSPORT_ALLOWLIST.get_or_init(|| Mutex::new(None))
}

pub(crate) fn replace_runtime_relay_candidates(
    candidates: Vec<PersistedRelayCandidate>,
) -> Vec<PersistedRelayCandidate> {
    let candidates = sorted_persisted_relay_candidates(candidates);
    *runtime_store()
        .lock()
        .expect("runtime relay candidates mutex poisoned") = candidates.clone();
    candidates
}

pub(crate) fn runtime_relay_candidates() -> Vec<PersistedRelayCandidate> {
    runtime_store()
        .lock()
        .expect("runtime relay candidates mutex poisoned")
        .clone()
}

pub(crate) fn set_test_relay_transport_allowlist(
    allowlist: Option<Vec<String>>,
) -> Option<Vec<String>> {
    let normalized = allowlist.and_then(|items| {
        let values = items
            .into_iter()
            .map(|value| value.trim().to_string())
            .filter(|value| !value.is_empty())
            .map(|value| {
                normalize_relay_transport(value.as_str())
                    .unwrap_or(value.as_str())
                    .to_string()
            })
            .collect::<Vec<_>>();
        (!values.is_empty()).then_some(values)
    });
    let mut guard = test_allowlist_store()
        .lock()
        .expect("test relay transport allowlist mutex poisoned");
    *guard = normalized.clone();
    guard.clone()
}

pub(crate) fn current_test_relay_transport_allowlist() -> Option<Vec<String>> {
    test_allowlist_store()
        .lock()
        .expect("test relay transport allowlist mutex poisoned")
        .clone()
}

pub(crate) fn sorted_persisted_relay_candidates(
    mut candidates: Vec<PersistedRelayCandidate>,
) -> Vec<PersistedRelayCandidate> {
    let selections = select_relay_candidates(&candidates);
    let rank_by_endpoint = selections
        .iter()
        .enumerate()
        .map(|(index, item)| (item.endpoint_id.clone(), index))
        .collect::<std::collections::HashMap<_, _>>();
    candidates.sort_by_key(|candidate| {
        rank_by_endpoint
            .get(candidate.endpoint_id.as_str())
            .copied()
            .unwrap_or(usize::MAX)
    });
    candidates
}

pub(crate) fn best_relay_candidate(
    candidates: &[PersistedRelayCandidate],
) -> Option<RelayCandidateSelection> {
    select_relay_candidates(candidates)
        .into_iter()
        .find(|candidate| candidate.selected)
}

pub(crate) fn relay_candidate_probe_fallback(
    candidates: &[PersistedRelayCandidate],
) -> Option<RelayCandidateSelection> {
    select_relay_candidates(candidates)
        .into_iter()
        .find(|candidate| {
            normalize_relay_transport(candidate.transport.as_str())
                .and_then(relay_path_kind_for_transport)
                .is_some()
        })
        .map(|mut candidate| {
            candidate.selected = true;
            candidate
        })
}

pub(crate) fn best_udp_relay_candidate(
    candidates: &[PersistedRelayCandidate],
) -> Option<RelayCandidateSelection> {
    select_relay_candidates(candidates)
        .into_iter()
        .find(|candidate| candidate.reachable && candidate.transport.eq_ignore_ascii_case("udp"))
}

pub(crate) fn select_relay_candidates(
    candidates: &[PersistedRelayCandidate],
) -> Vec<RelayCandidateSelection> {
    let mut candidates = filtered_relay_candidates_for_testing(candidates)
        .into_iter()
        .filter(|candidate| {
            !candidate.endpoint_id.trim().is_empty()
                && !candidate.transport.trim().is_empty()
                && !candidate.address.trim().is_empty()
        })
        .collect::<Vec<_>>();
    candidates.sort_by(compare_relay_probe_priority);
    let active_probes = active_relay_probe_endpoint_ids(&candidates);
    let mut selections = candidates
        .iter()
        .map(|candidate| {
            score_relay_candidate(candidate, active_probes.contains(&candidate.endpoint_id))
        })
        .collect::<Vec<_>>();
    selections.sort_by(|left, right| {
        right
            .reachable
            .cmp(&left.reachable)
            .then_with(|| {
                relay_transport_rank(&left.transport).cmp(&relay_transport_rank(&right.transport))
            })
            .then_with(|| relay_locality_rank(left).cmp(&relay_locality_rank(right)))
            .then_with(|| left.path_score.cmp(&right.path_score))
            .then_with(|| {
                left.rtt_ms
                    .unwrap_or(u32::MAX)
                    .cmp(&right.rtt_ms.unwrap_or(u32::MAX))
            })
            .then_with(|| right.selected.cmp(&left.selected))
            .then_with(|| left.endpoint_id.cmp(&right.endpoint_id))
    });
    for (index, selection) in selections.iter_mut().enumerate() {
        selection.selected = index == 0 && selection.reachable;
    }
    selections
}

fn compare_relay_probe_priority(
    left: &PersistedRelayCandidate,
    right: &PersistedRelayCandidate,
) -> std::cmp::Ordering {
    right
        .reachable_hint
        .cmp(&left.reachable_hint)
        .then_with(|| {
            relay_transport_rank(&left.transport).cmp(&relay_transport_rank(&right.transport))
        })
        .then_with(|| {
            relay_candidate_locality_rank(left).cmp(&relay_candidate_locality_rank(right))
        })
        .then_with(|| {
            left.path_score_hint
                .unwrap_or(u32::MAX)
                .cmp(&right.path_score_hint.unwrap_or(u32::MAX))
        })
        .then_with(|| {
            left.observed_rtt_ms_hint
                .unwrap_or(u32::MAX)
                .cmp(&right.observed_rtt_ms_hint.unwrap_or(u32::MAX))
        })
        .then_with(|| left.endpoint_id.cmp(&right.endpoint_id))
}

fn relay_locality_rank(candidate: &RelayCandidateSelection) -> u8 {
    locality_rank(
        candidate.country_code.as_deref(),
        candidate.city_code.as_deref(),
    )
}

fn relay_candidate_locality_rank(candidate: &PersistedRelayCandidate) -> u8 {
    locality_rank(
        candidate.country_code.as_deref(),
        candidate.city_code.as_deref(),
    )
}

fn locality_rank(country_code: Option<&str>, city_code: Option<&str>) -> u8 {
    let location = device_location_store()
        .lock()
        .expect("runtime device location mutex poisoned")
        .clone();
    locality_rank_for_location(&location, country_code, city_code)
}

fn locality_rank_for_location(
    location: &RuntimeDeviceLocation,
    country_code: Option<&str>,
    city_code: Option<&str>,
) -> u8 {
    if location.city_code.as_deref().is_some_and(|local| {
        city_code.is_some_and(|candidate| candidate.eq_ignore_ascii_case(local))
    }) {
        return 0;
    }
    if location.country_code.as_deref().is_some_and(|local| {
        country_code.is_some_and(|candidate| candidate.eq_ignore_ascii_case(local))
    }) {
        return 1;
    }
    2
}

fn active_relay_probe_endpoint_ids(
    candidates: &[PersistedRelayCandidate],
) -> std::collections::HashSet<String> {
    let mut selected = std::collections::HashSet::new();

    // Probe the best candidate of each supported transport first so that an
    // unhealthy UDP node cannot prevent DERP fallback quality from refreshing.
    for transport in ["udp", "derp_tcp_tls_443"] {
        if let Some(candidate) = candidates.iter().find(|candidate| {
            normalize_relay_transport(candidate.transport.as_str()) == Some(transport)
        }) {
            selected.insert(candidate.endpoint_id.clone());
        }
    }
    for transport in ["udp", "derp_tcp_tls_443"] {
        if let Some(candidate) = candidates.iter().find(|candidate| {
            normalize_relay_transport(candidate.transport.as_str()) == Some(transport)
                && candidate
                    .country_code
                    .as_deref()
                    .is_some_and(|country| country.eq_ignore_ascii_case("HK"))
        }) {
            selected.insert(candidate.endpoint_id.clone());
        }
    }
    for candidate in candidates {
        if selected.len() >= MAX_ACTIVE_RELAY_PROBES {
            break;
        }
        selected.insert(candidate.endpoint_id.clone());
    }
    selected
}

fn relay_transport_rank(transport: &str) -> u8 {
    match normalize_relay_transport(transport) {
        Some("udp") => 0,
        Some("derp_tcp_tls_443") => 1,
        _ => 2,
    }
}

fn filtered_relay_candidates_for_testing(
    candidates: &[PersistedRelayCandidate],
) -> Vec<PersistedRelayCandidate> {
    let Some(allowlist) =
        current_test_relay_transport_allowlist().or_else(relay_transport_allowlist_from_env)
    else {
        return candidates.to_vec();
    };
    candidates
        .iter()
        .filter(|candidate| {
            normalize_relay_transport(candidate.transport.as_str())
                .map(|transport| allowlist.iter().any(|allowed| allowed == transport))
                .unwrap_or_else(|| {
                    allowlist
                        .iter()
                        .any(|allowed| allowed == candidate.transport.trim())
                })
        })
        .cloned()
        .collect()
}

fn relay_transport_allowlist_from_env() -> Option<Vec<String>> {
    let raw = std::env::var("SLAN_TEST_RELAY_TRANSPORT_ALLOWLIST").ok()?;
    let allowlist = raw
        .split(',')
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .map(|value| {
            normalize_relay_transport(value)
                .unwrap_or(value)
                .to_string()
        })
        .collect::<Vec<_>>();
    (!allowlist.is_empty()).then_some(allowlist)
}

pub(crate) fn diagnose_direct_candidates(
    peers: &[ControlPeer],
) -> Vec<PathDiagnoseDirectCandidate> {
    peers
        .iter()
        .flat_map(|peer| {
            peer.endpoints
                .iter()
                .filter(|endpoint| is_direct_endpoint(endpoint))
                .map(move |endpoint| {
                    let rtt_ms = probe_relay_udp_rtt_ms(&endpoint.address);
                    PathDiagnoseDirectCandidate {
                        peer_node_id: peer.node_id.clone(),
                        path_type: "direct_udp_probe".to_string(),
                        endpoint_type: endpoint.endpoint_type.clone(),
                        address: endpoint.address.clone(),
                        updated_at: endpoint.updated_at,
                        reachable: rtt_ms.is_some(),
                        rtt_ms,
                    }
                })
        })
        .collect()
}

pub(crate) fn extract_persisted_relay_candidates_from_control_map(
    map: &Value,
) -> Vec<PersistedRelayCandidate> {
    // The older control map shape is still used as a relay-region snapshot source.
    // It no longer drives general network state updates.
    map.get("relayRegions")
        .and_then(Value::as_array)
        .into_iter()
        .flatten()
        .flat_map(|region| {
            let country_code = optional_trimmed_string(region.get("countryCode"));
            let city_code = optional_trimmed_string(region.get("cityCode"));
            let region_id = optional_trimmed_string(region.get("regionId"));
            let cluster_id = optional_trimmed_string(region.get("clusterId"));
            region
                .get("endpoints")
                .and_then(Value::as_array)
                .into_iter()
                .flatten()
                .filter_map(move |endpoint| {
                    let endpoint_id = optional_trimmed_string(endpoint.get("endpointId"))?;
                    let transport = optional_trimmed_string(endpoint.get("transport"))?;
                    let address = optional_trimmed_string(endpoint.get("address"))?;
                    Some(PersistedRelayCandidate {
                        endpoint_id,
                        transport,
                        address,
                        country_code: country_code.clone(),
                        city_code: city_code.clone(),
                        region_id: region_id.clone(),
                        cluster_id: cluster_id.clone(),
                        reachable_hint: false,
                        observed_rtt_ms_hint: None,
                        path_score_hint: None,
                    })
                })
                .collect::<Vec<_>>()
        })
        .collect()
}

fn score_relay_candidate(
    candidate: &PersistedRelayCandidate,
    should_probe: bool,
) -> RelayCandidateSelection {
    let transport = normalize_relay_transport(&candidate.transport)
        .unwrap_or_else(|| candidate.transport.trim())
        .to_string();
    let Some(address) = normalize_relay_candidate_address(&candidate.address, &transport) else {
        return RelayCandidateSelection {
            endpoint_id: candidate.endpoint_id.clone(),
            transport,
            address: candidate.address.clone(),
            country_code: candidate.country_code.clone(),
            city_code: candidate.city_code.clone(),
            region_id: candidate.region_id.clone(),
            cluster_id: candidate.cluster_id.clone(),
            configured_priority: candidate.path_score_hint.unwrap_or(u32::MAX),
            reachable: candidate.reachable_hint,
            rtt_ms: candidate.observed_rtt_ms_hint,
            path_score: candidate.path_score_hint.unwrap_or(11_000),
            selected: false,
        };
    };
    let mut reachable = candidate.reachable_hint;
    let mut rtt_ms = candidate.observed_rtt_ms_hint;
    let mut path_score = candidate.path_score_hint.unwrap_or(11_000);
    match (transport.as_str(), should_probe) {
        ("udp", true) => match probe_relay_udp_rtt_ms(&address) {
            Some(rtt) => {
                reachable = true;
                rtt_ms = Some(rtt);
                path_score = rtt.saturating_add(30);
            }
            None if candidate.path_score_hint.is_none() => {
                reachable = false;
                path_score = 10_000;
            }
            None => {}
        },
        ("derp_tcp_tls_443", true) => match probe_relay_tcp_rtt_ms(&address) {
            Some(rtt) => {
                reachable = true;
                rtt_ms = Some(rtt);
                path_score = rtt.saturating_add(100);
            }
            None if candidate.path_score_hint.is_none() => {
                reachable = false;
                path_score = 10_500;
            }
            None => {}
        },
        ("udp", false) | ("derp_tcp_tls_443", false) => {}
        _ => {
            if candidate.path_score_hint.is_none() {
                reachable = false;
                path_score = 11_000;
            }
        }
    }
    RelayCandidateSelection {
        endpoint_id: candidate.endpoint_id.clone(),
        transport,
        address,
        country_code: candidate.country_code.clone(),
        city_code: candidate.city_code.clone(),
        region_id: candidate.region_id.clone(),
        cluster_id: candidate.cluster_id.clone(),
        configured_priority: candidate.path_score_hint.unwrap_or(u32::MAX),
        reachable,
        rtt_ms,
        path_score,
        selected: false,
    }
}

pub(crate) fn normalize_relay_candidate_address(address: &str, transport: &str) -> Option<String> {
    let trimmed = address.trim();
    if trimmed.is_empty() {
        return None;
    }
    let transport = normalize_relay_transport(transport)?;
    let stripped = match transport {
        "udp" => trimmed
            .strip_prefix("udp://")
            .or_else(|| trimmed.strip_prefix("relay+udp://")),
        "derp_tcp_tls_443" => trimmed
            .strip_prefix("derp://")
            .or_else(|| trimmed.strip_prefix("derp+tcp+tls://"))
            .or_else(|| trimmed.strip_prefix("derp_tcp_tls_443://")),
        _ => None,
    };
    if stripped.is_none() && trimmed.contains("://") {
        return None;
    }
    let normalized = stripped.unwrap_or(trimmed).trim();
    if normalized.is_empty() {
        return None;
    }
    Some(normalized.to_string())
}

fn is_direct_endpoint(endpoint: &ControlEndpoint) -> bool {
    matches!(
        endpoint.endpoint_type.trim(),
        "lan" | "wan" | "reflexive" | "p2p" | "direct"
    ) && !endpoint.address.trim().is_empty()
}

fn probe_relay_udp_rtt_ms(address: &str) -> Option<u32> {
    let socket = address
        .to_socket_addrs()
        .ok()
        .and_then(|mut values| values.next())?;
    let udp = UdpSocket::bind("0.0.0.0:0").ok()?;
    udp.set_read_timeout(Some(Duration::from_millis(750)))
        .ok()?;
    udp.set_write_timeout(Some(Duration::from_millis(750)))
        .ok()?;
    udp.connect(socket).ok()?;
    let started = Instant::now();
    udp.send(br#"{"kind":"ping"}"#).ok()?;
    let mut response = [0_u8; 512];
    let len = udp.recv(&mut response).ok()?;
    let value = serde_json::from_slice::<Value>(&response[..len]).ok()?;
    if value.get("kind").and_then(Value::as_str) != Some("pong") {
        return None;
    }
    Some(started.elapsed().as_millis().min(u32::MAX as u128) as u32)
}

fn probe_relay_tcp_rtt_ms(address: &str) -> Option<u32> {
    let socket = address
        .to_socket_addrs()
        .ok()
        .and_then(|mut values| values.next())?;
    let started = Instant::now();
    TcpStream::connect_timeout(&socket, Duration::from_millis(750)).ok()?;
    Some(started.elapsed().as_millis().min(u32::MAX as u128) as u32)
}

fn optional_trimmed_string(value: Option<&Value>) -> Option<String> {
    value
        .and_then(Value::as_str)
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .map(str::to_string)
}

#[cfg(test)]
mod tests {
    use super::{
        active_relay_probe_endpoint_ids, locality_rank_for_location,
        normalize_relay_candidate_address, relay_candidates_for_peer, select_relay_candidates,
        set_test_relay_transport_allowlist, update_runtime_device_location, RuntimeDeviceLocation,
    };
    use crate::relay_models::{PersistedRelayCandidate, RelayCandidateSelection};

    fn selection(
        endpoint_id: &str,
        transport: &str,
        country_code: &str,
        score: u32,
    ) -> RelayCandidateSelection {
        RelayCandidateSelection {
            endpoint_id: endpoint_id.to_string(),
            transport: transport.to_string(),
            address: "127.0.0.1:1".to_string(),
            country_code: Some(country_code.to_string()),
            city_code: None,
            region_id: None,
            cluster_id: None,
            configured_priority: score,
            reachable: true,
            rtt_ms: Some(score),
            path_score: score,
            selected: false,
        }
    }

    #[test]
    fn client_prefers_hong_kong_within_transport_for_mainland_cross_border_peer() {
        let _lock = crate::test_env_lock();
        update_runtime_device_location("CN", "");
        let ranked = relay_candidates_for_peer(
            &[
                selection("sg-udp", "udp", "SG", 10),
                selection("hk-udp", "udp", "HK", 30),
                selection("hk-tcp", "derp_tcp_tls_443", "HK", 1),
            ],
            "US",
            "",
        );

        assert_eq!(ranked[0].endpoint_id, "hk-udp");
        assert_eq!(ranked[1].endpoint_id, "sg-udp");
        assert_eq!(ranked[2].endpoint_id, "hk-tcp");
        update_runtime_device_location("", "");
    }

    #[test]
    fn peer_pair_ranking_is_stable_when_local_and_peer_locations_are_reversed() {
        let _lock = crate::test_env_lock();
        let mut shenzhen = selection("relay-shenzhen", "udp", "CN", 5);
        shenzhen.city_code = Some("shenzhen".to_string());
        shenzhen.configured_priority = 100;
        let mut singapore = selection("relay-singapore", "udp", "SG", 50);
        singapore.city_code = Some("singapore".to_string());
        singapore.configured_priority = 80;
        let candidates = [shenzhen, singapore];

        update_runtime_device_location("CN", "shenzhen");
        let forward = relay_candidates_for_peer(&candidates, "SG", "singapore");
        update_runtime_device_location("SG", "singapore");
        let reverse = relay_candidates_for_peer(&candidates, "CN", "shenzhen");

        assert_eq!(forward[0].endpoint_id, "relay-singapore");
        assert_eq!(reverse[0].endpoint_id, forward[0].endpoint_id);
        update_runtime_device_location("", "");
    }

    #[test]
    fn relay_candidate_address_accepts_matching_scheme_or_bare_address() {
        assert_eq!(
            normalize_relay_candidate_address("udp://127.0.0.1:9000", "udp").as_deref(),
            Some("127.0.0.1:9000")
        );
        assert_eq!(
            normalize_relay_candidate_address("127.0.0.1:9000", "udp").as_deref(),
            Some("127.0.0.1:9000")
        );
    }

    #[test]
    fn relay_candidate_address_rejects_mismatched_or_alias_scheme() {
        assert_eq!(
            normalize_relay_candidate_address("tcp://127.0.0.1:9001", "udp"),
            None
        );
        assert_eq!(
            normalize_relay_candidate_address("tls://127.0.0.1:9443", "tls"),
            None
        );
        assert_eq!(
            normalize_relay_candidate_address("http3://127.0.0.1:9443", "http3"),
            None
        );
    }

    #[test]
    fn relay_candidate_selection_can_be_filtered_by_test_allowlist() {
        let _lock = crate::test_env_lock();
        set_test_relay_transport_allowlist(Some(vec!["derp_tcp_tls_443".to_string()]));
        let selections = select_relay_candidates(&[
            PersistedRelayCandidate {
                endpoint_id: "relay-udp".to_string(),
                transport: "udp".to_string(),
                address: "udp://127.0.0.1:29110".to_string(),
                country_code: None,
                city_code: None,
                region_id: None,
                cluster_id: None,
                reachable_hint: true,
                observed_rtt_ms_hint: Some(10),
                path_score_hint: Some(10),
            },
            PersistedRelayCandidate {
                endpoint_id: "relay-derp".to_string(),
                transport: "derp_tcp_tls_443".to_string(),
                address: "derp://127.0.0.1:29120".to_string(),
                country_code: None,
                city_code: None,
                region_id: None,
                cluster_id: None,
                reachable_hint: true,
                observed_rtt_ms_hint: Some(20),
                path_score_hint: Some(20),
            },
        ]);
        set_test_relay_transport_allowlist(None);

        assert_eq!(selections.len(), 1);
        assert_eq!(selections[0].endpoint_id, "relay-derp");
    }

    #[test]
    fn local_quality_selects_the_lower_score_candidate() {
        let _lock = crate::test_env_lock();
        set_test_relay_transport_allowlist(None);
        let selections = select_relay_candidates(&[
            PersistedRelayCandidate {
                endpoint_id: "derp-slow".to_string(),
                transport: "derp_tcp_tls_443".to_string(),
                address: "derp://127.0.0.1:1".to_string(),
                country_code: None,
                city_code: None,
                region_id: None,
                cluster_id: None,
                reachable_hint: true,
                observed_rtt_ms_hint: Some(500),
                path_score_hint: Some(600),
            },
            PersistedRelayCandidate {
                endpoint_id: "derp-locally-faster".to_string(),
                transport: "derp_tcp_tls_443".to_string(),
                address: "derp://127.0.0.1:2".to_string(),
                country_code: None,
                city_code: None,
                region_id: None,
                cluster_id: None,
                reachable_hint: true,
                observed_rtt_ms_hint: Some(10),
                path_score_hint: Some(110),
            },
        ]);

        assert_eq!(selections[0].endpoint_id, "derp-locally-faster");
        assert!(selections[0].selected);
        assert!(!selections[1].selected);
    }

    #[test]
    fn udp_relay_ports_are_ranked_by_quality_before_derp() {
        let _lock = crate::test_env_lock();
        set_test_relay_transport_allowlist(None);
        let selections = select_relay_candidates(&[
            PersistedRelayCandidate {
                endpoint_id: "udp-slow".to_string(),
                transport: "udp".to_string(),
                address: "udp://127.0.0.1:1".to_string(),
                country_code: None,
                city_code: None,
                region_id: None,
                cluster_id: None,
                reachable_hint: true,
                observed_rtt_ms_hint: Some(80),
                path_score_hint: Some(110),
            },
            PersistedRelayCandidate {
                endpoint_id: "udp-fast".to_string(),
                transport: "udp".to_string(),
                address: "udp://127.0.0.1:2".to_string(),
                country_code: None,
                city_code: None,
                region_id: None,
                cluster_id: None,
                reachable_hint: true,
                observed_rtt_ms_hint: Some(15),
                path_score_hint: Some(45),
            },
            PersistedRelayCandidate {
                endpoint_id: "derp-fast".to_string(),
                transport: "derp_tcp_tls_443".to_string(),
                address: "derp://127.0.0.1:3".to_string(),
                country_code: None,
                city_code: None,
                region_id: None,
                cluster_id: None,
                reachable_hint: true,
                observed_rtt_ms_hint: Some(5),
                path_score_hint: Some(5),
            },
        ]);

        assert_eq!(selections[0].endpoint_id, "udp-fast");
        assert_eq!(selections[1].endpoint_id, "udp-slow");
        assert_eq!(selections[2].endpoint_id, "derp-fast");
    }

    #[test]
    fn relay_probe_budget_preserves_transport_diversity() {
        let candidates = [
            relay_candidate("udp-1", "udp", 10),
            relay_candidate("udp-2", "udp", 20),
            relay_candidate("udp-3", "udp", 30),
            relay_candidate("derp-1", "derp_tcp_tls_443", 40),
            relay_candidate("derp-2", "derp_tcp_tls_443", 50),
        ];

        let selected = active_relay_probe_endpoint_ids(&candidates);

        assert_eq!(selected.len(), 4);
        assert!(selected.contains("udp-1"));
        assert!(selected.contains("udp-2"));
        assert!(selected.contains("udp-3"));
        assert!(selected.contains("derp-1"));
    }

    fn relay_candidate(
        endpoint_id: &str,
        transport: &str,
        path_score: u32,
    ) -> PersistedRelayCandidate {
        PersistedRelayCandidate {
            endpoint_id: endpoint_id.to_string(),
            transport: transport.to_string(),
            address: "127.0.0.1:1".to_string(),
            country_code: None,
            city_code: None,
            region_id: None,
            cluster_id: None,
            reachable_hint: true,
            observed_rtt_ms_hint: Some(path_score),
            path_score_hint: Some(path_score),
        }
    }

    #[test]
    fn locality_rank_prefers_city_then_country() {
        let local = RuntimeDeviceLocation {
            country_code: Some("CN".to_string()),
            city_code: Some("1796236".to_string()),
        };

        assert_eq!(
            locality_rank_for_location(&local, Some("CN"), Some("1796236")),
            0
        );
        assert_eq!(
            locality_rank_for_location(&local, Some("CN"), Some("1816670")),
            1
        );
        assert_eq!(
            locality_rank_for_location(&local, Some("HK"), Some("1819729")),
            2
        );
    }
}
