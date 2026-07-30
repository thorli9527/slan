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

const MAX_ACTIVE_RELAY_PROBES: usize = 3;

static RUNTIME_RELAY_CANDIDATES: OnceLock<Mutex<Vec<PersistedRelayCandidate>>> = OnceLock::new();
static TEST_RELAY_TRANSPORT_ALLOWLIST: OnceLock<Mutex<Option<Vec<String>>>> = OnceLock::new();

fn runtime_store() -> &'static Mutex<Vec<PersistedRelayCandidate>> {
    RUNTIME_RELAY_CANDIDATES.get_or_init(|| Mutex::new(Vec::new()))
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
            .then_with(|| left.path_score.cmp(&right.path_score))
            .then_with(|| {
                left.rtt_ms
                    .unwrap_or(u32::MAX)
                    .cmp(&right.rtt_ms.unwrap_or(u32::MAX))
            })
            .then_with(|| right.selected.cmp(&left.selected))
            .then_with(|| left.endpoint_id.cmp(&right.endpoint_id))
    });
    let has_explicit_selected = selections.iter().any(|selection| selection.selected);
    for (index, selection) in selections.iter_mut().enumerate() {
        selection.selected = if has_explicit_selected {
            index == 0
        } else {
            index == 0 && selection.reachable
        };
    }
    selections
}

fn compare_relay_probe_priority(
    left: &PersistedRelayCandidate,
    right: &PersistedRelayCandidate,
) -> std::cmp::Ordering {
    right
        .selected_hint
        .cmp(&left.selected_hint)
        .then_with(|| right.reachable_hint.cmp(&left.reachable_hint))
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
        .then_with(|| {
            relay_transport_rank(&left.transport).cmp(&relay_transport_rank(&right.transport))
        })
        .then_with(|| left.endpoint_id.cmp(&right.endpoint_id))
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
                        region_id: region_id.clone(),
                        cluster_id: cluster_id.clone(),
                        reachable_hint: false,
                        observed_rtt_ms_hint: None,
                        path_score_hint: None,
                        selected_hint: false,
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
            region_id: candidate.region_id.clone(),
            cluster_id: candidate.cluster_id.clone(),
            reachable: candidate.reachable_hint,
            rtt_ms: candidate.observed_rtt_ms_hint,
            path_score: candidate.path_score_hint.unwrap_or(11_000),
            selected: candidate.selected_hint,
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
        region_id: candidate.region_id.clone(),
        cluster_id: candidate.cluster_id.clone(),
        reachable,
        rtt_ms,
        path_score,
        selected: candidate.selected_hint,
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
        active_relay_probe_endpoint_ids, normalize_relay_candidate_address,
        select_relay_candidates, set_test_relay_transport_allowlist,
    };
    use crate::relay_models::PersistedRelayCandidate;

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
                region_id: None,
                cluster_id: None,
                reachable_hint: true,
                observed_rtt_ms_hint: Some(10),
                path_score_hint: Some(10),
                selected_hint: true,
            },
            PersistedRelayCandidate {
                endpoint_id: "relay-derp".to_string(),
                transport: "derp_tcp_tls_443".to_string(),
                address: "derp://127.0.0.1:29120".to_string(),
                country_code: None,
                region_id: None,
                cluster_id: None,
                reachable_hint: true,
                observed_rtt_ms_hint: Some(20),
                path_score_hint: Some(20),
                selected_hint: false,
            },
        ]);
        set_test_relay_transport_allowlist(None);

        assert_eq!(selections.len(), 1);
        assert_eq!(selections[0].endpoint_id, "relay-derp");
    }

    #[test]
    fn local_quality_overrides_stale_server_selected_hint() {
        let _lock = crate::test_env_lock();
        set_test_relay_transport_allowlist(None);
        let selections = select_relay_candidates(&[
            PersistedRelayCandidate {
                endpoint_id: "derp-server-selected".to_string(),
                transport: "derp_tcp_tls_443".to_string(),
                address: "derp://127.0.0.1:1".to_string(),
                country_code: None,
                region_id: None,
                cluster_id: None,
                reachable_hint: true,
                observed_rtt_ms_hint: Some(500),
                path_score_hint: Some(600),
                selected_hint: true,
            },
            PersistedRelayCandidate {
                endpoint_id: "derp-locally-faster".to_string(),
                transport: "derp_tcp_tls_443".to_string(),
                address: "derp://127.0.0.1:2".to_string(),
                country_code: None,
                region_id: None,
                cluster_id: None,
                reachable_hint: true,
                observed_rtt_ms_hint: Some(10),
                path_score_hint: Some(110),
                selected_hint: false,
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
                region_id: None,
                cluster_id: None,
                reachable_hint: true,
                observed_rtt_ms_hint: Some(80),
                path_score_hint: Some(110),
                selected_hint: false,
            },
            PersistedRelayCandidate {
                endpoint_id: "udp-fast".to_string(),
                transport: "udp".to_string(),
                address: "udp://127.0.0.1:2".to_string(),
                country_code: None,
                region_id: None,
                cluster_id: None,
                reachable_hint: true,
                observed_rtt_ms_hint: Some(15),
                path_score_hint: Some(45),
                selected_hint: false,
            },
            PersistedRelayCandidate {
                endpoint_id: "derp-fast".to_string(),
                transport: "derp_tcp_tls_443".to_string(),
                address: "derp://127.0.0.1:3".to_string(),
                country_code: None,
                region_id: None,
                cluster_id: None,
                reachable_hint: true,
                observed_rtt_ms_hint: Some(5),
                path_score_hint: Some(5),
                selected_hint: false,
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

        assert_eq!(selected.len(), 3);
        assert!(selected.contains("udp-1"));
        assert!(selected.contains("udp-2"));
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
            region_id: None,
            cluster_id: None,
            reachable_hint: true,
            observed_rtt_ms_hint: Some(path_score),
            path_score_hint: Some(path_score),
            selected_hint: false,
        }
    }
}
