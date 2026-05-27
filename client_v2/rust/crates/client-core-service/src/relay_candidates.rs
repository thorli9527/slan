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

static RUNTIME_RELAY_CANDIDATES: OnceLock<Mutex<Vec<PersistedRelayCandidate>>> = OnceLock::new();

fn runtime_store() -> &'static Mutex<Vec<PersistedRelayCandidate>> {
    RUNTIME_RELAY_CANDIDATES.get_or_init(|| Mutex::new(Vec::new()))
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
    let mut selections = candidates
        .iter()
        .filter(|candidate| {
            !candidate.endpoint_id.trim().is_empty()
                && !candidate.transport.trim().is_empty()
                && !candidate.address.trim().is_empty()
        })
        .map(score_relay_candidate)
        .collect::<Vec<_>>();
    selections.sort_by(|left, right| {
        right
            .reachable
            .cmp(&left.reachable)
            .then_with(|| left.path_score.cmp(&right.path_score))
            .then_with(|| left.endpoint_id.cmp(&right.endpoint_id))
    });
    for (index, selection) in selections.iter_mut().enumerate() {
        selection.selected = index == 0 && selection.reachable;
    }
    selections
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

pub(crate) fn extract_persisted_relay_candidates_from_network_map(
    map: &Value,
) -> Vec<PersistedRelayCandidate> {
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
                    })
                })
                .collect::<Vec<_>>()
        })
        .collect()
}

fn score_relay_candidate(candidate: &PersistedRelayCandidate) -> RelayCandidateSelection {
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
            reachable: false,
            rtt_ms: None,
            path_score: 11_000,
            selected: false,
        };
    };
    let mut reachable = true;
    let mut rtt_ms = None;
    let path_score = match transport.as_str() {
        "udp" => match probe_relay_udp_rtt_ms(&address) {
            Some(rtt) => {
                rtt_ms = Some(rtt);
                rtt.saturating_add(30)
            }
            None => {
                reachable = false;
                10_000
            }
        },
        "derp_tcp_tls_443" => match probe_relay_tcp_rtt_ms(&address) {
            Some(rtt) => {
                rtt_ms = Some(rtt);
                rtt.saturating_add(100)
            }
            None => {
                reachable = false;
                10_500
            }
        },
        _ => {
            reachable = false;
            11_000
        }
    };
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
    use super::normalize_relay_candidate_address;

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
}
