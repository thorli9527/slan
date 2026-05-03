use std::{fs, path::PathBuf};

use crate::{
    relay_models::{RelayDataPlanePolicy, RelayPayloadPolicy, RelayRuntimeStats},
    session_store::current_timestamp_ms,
};
use client_core::{PathKind, PathPolicy};

pub(crate) fn relay_payload_policy(
    relay_address: &str,
    network_id: &str,
    device_id: Option<&str>,
    path_type: Option<&str>,
) -> RelayPayloadPolicy {
    if let Some(policy) =
        load_recent_relay_data_plane_policy_for_path(network_id, device_id, path_type)
    {
        if let (Some(relay_mtu), Some(max_frame_payload)) =
            (policy.relay_mtu, policy.max_frame_payload)
        {
            return RelayPayloadPolicy {
                relay_mtu: relay_mtu.clamp(576, 1500),
                max_frame_payload: max_frame_payload.clamp(512, 1400),
            };
        }
    }
    let Some(stats) =
        load_recent_relay_runtime_stats().filter(|stats| stats.relay_address == relay_address)
    else {
        return RelayPayloadPolicy {
            relay_mtu: 1280,
            max_frame_payload: 1200,
        };
    };
    let loss = runtime_packet_loss_ppm(&stats).unwrap_or(0);
    if loss >= 50_000 {
        return RelayPayloadPolicy {
            relay_mtu: 1100,
            max_frame_payload: 1020,
        };
    }
    if loss >= 10_000 {
        return RelayPayloadPolicy {
            relay_mtu: 1200,
            max_frame_payload: 1120,
        };
    }
    RelayPayloadPolicy {
        relay_mtu: 1280,
        max_frame_payload: 1200,
    }
}

pub(crate) fn relay_path_policy(
    network_id: &str,
    device_id: Option<&str>,
    path_type: Option<&str>,
) -> PathPolicy {
    let Some(policy) =
        load_recent_relay_data_plane_policy_for_path(network_id, device_id, path_type)
    else {
        return PathPolicy::default();
    };
    let preferred = policy
        .preferred_path_types
        .iter()
        .filter_map(|value| path_kind_from_policy_value(value))
        .fold(Vec::new(), |mut acc, kind| {
            if !acc.contains(&kind) {
                acc.push(kind);
            }
            acc
        });
    if preferred.is_empty() {
        return PathPolicy::default();
    }
    PathPolicy {
        preferred,
        ..PathPolicy::default()
    }
}

fn path_kind_from_policy_value(value: &str) -> Option<PathKind> {
    match value.trim() {
        "direct_udp" => Some(PathKind::DirectUdp),
        "relay_udp" => Some(PathKind::RelayUdp),
        "relay_tcp" => Some(PathKind::RelayTcp),
        "relay_http3" => Some(PathKind::RelayHttp3),
        "relay_tls" => Some(PathKind::RelayTls),
        _ => None,
    }
}

pub(crate) fn load_recent_relay_runtime_stats() -> Option<RelayRuntimeStats> {
    let stats = load_relay_runtime_stats()?;
    if current_timestamp_ms().saturating_sub(stats.updated_at_ms) > 15 * 60 * 1000 {
        return None;
    }
    Some(stats)
}

pub(crate) fn load_relay_runtime_stats() -> Option<RelayRuntimeStats> {
    let payload = fs::read(relay_stats_file_path()).ok()?;
    serde_json::from_slice::<RelayRuntimeStats>(&payload).ok()
}

pub(crate) fn relay_runtime_failure_total(stats: &RelayRuntimeStats) -> u64 {
    stats
        .relay_send_failures
        .saturating_add(stats.relay_receive_failures)
        .saturating_add(stats.relay_attach_failures)
        .saturating_add(stats.relay_decode_failures)
        .saturating_add(stats.relay_error_responses)
        .saturating_add(stats.relay_config_hash_mismatches)
        .saturating_add(stats.unroutable_tun_packets)
        .saturating_add(stats.oversized_tun_packets)
        .saturating_add(stats.wintun_write_failures)
}

pub(crate) fn load_recent_relay_data_plane_policy_for_path(
    network_id: &str,
    device_id: Option<&str>,
    path_type: Option<&str>,
) -> Option<RelayDataPlanePolicy> {
    let payload = fs::read(relay_policy_file_path()).ok()?;
    let policy = serde_json::from_slice::<RelayDataPlanePolicy>(&payload).ok()?;
    if let Some(scope) = policy.scope.as_deref() {
        match scope {
            "global" | "region" | "network" | "device" | "device_override" => {}
            _ => return None,
        }
    }
    if let Some(policy_network_id) = policy.network_id.as_deref() {
        if !policy_network_id.trim().is_empty() && policy_network_id.trim() != network_id {
            return None;
        }
    }
    let targets = policy
        .target_device_ids
        .iter()
        .map(|value| value.trim())
        .filter(|value| !value.is_empty())
        .collect::<Vec<_>>();
    if !targets.is_empty() {
        let current_device_id = device_id?.trim();
        if current_device_id.is_empty()
            || !targets.iter().any(|target| *target == current_device_id)
        {
            return None;
        }
    }
    if let Some(policy_path_type) = policy
        .path_type
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
    {
        let current_path_type = path_type.map(str::trim).filter(|value| !value.is_empty())?;
        if policy_path_type != current_path_type
            && !(policy_path_type == "relay" && current_path_type.starts_with("relay_"))
        {
            return None;
        }
    }
    let updated_at_ms = policy.updated_at_ms?;
    let ttl_ms = policy.ttl_ms.unwrap_or(60 * 60 * 1000);
    if current_timestamp_ms().saturating_sub(updated_at_ms) > ttl_ms {
        return None;
    }
    Some(policy)
}

pub(crate) fn diagnostics_export_file_path() -> PathBuf {
    let file_name = format!("client-v2-diagnostics-{}.json", current_timestamp_ms());
    if let Some(dir) = std::env::var_os("ProgramData") {
        return PathBuf::from(dir)
            .join("SLAN")
            .join("diagnostics")
            .join(file_name);
    }
    if let Some(dir) = std::env::var_os("SLAN_STATE_DIR") {
        return PathBuf::from(dir).join("diagnostics").join(file_name);
    }
    PathBuf::from(file_name)
}

pub(crate) fn runtime_packet_loss_ppm(stats: &RelayRuntimeStats) -> Option<u32> {
    let failures = stats
        .unroutable_tun_packets
        .saturating_add(stats.relay_decode_failures)
        .saturating_add(stats.oversized_tun_packets)
        .saturating_add(stats.wintun_write_failures);
    let total = stats
        .tun_packets_sent
        .saturating_add(stats.relay_packets_received)
        .saturating_add(failures);
    if total == 0 {
        return None;
    }
    let ppm = failures.saturating_mul(1_000_000) / total;
    Some(ppm.min(1_000_000) as u32)
}

fn relay_stats_file_path() -> PathBuf {
    if let Some(dir) = std::env::var_os("ProgramData") {
        return PathBuf::from(dir)
            .join("SLAN")
            .join("client-v2-relay-stats.json");
    }
    if let Some(dir) = std::env::var_os("SLAN_STATE_DIR") {
        return PathBuf::from(dir).join("client-v2-relay-stats.json");
    }
    PathBuf::from("client-v2-relay-stats.json")
}

fn relay_policy_file_path() -> PathBuf {
    if let Some(dir) = std::env::var_os("ProgramData") {
        return PathBuf::from(dir)
            .join("SLAN")
            .join("client-v2-relay-policy.json");
    }
    if let Some(dir) = std::env::var_os("SLAN_STATE_DIR") {
        return PathBuf::from(dir).join("client-v2-relay-policy.json");
    }
    PathBuf::from("client-v2-relay-policy.json")
}
