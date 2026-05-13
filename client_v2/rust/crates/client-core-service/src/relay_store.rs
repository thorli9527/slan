use std::{fs, path::PathBuf};

use crate::{
    relay_models::{RelayPayloadPolicy, RelayRuntimeStats},
    session_store::app_data_dir,
    session_store::current_timestamp_ms,
    time_utils::ticket_timing_with_window,
};
use client_core::PathPolicy;

const RELAY_TICKET_RENEW_WINDOW_MS: u64 = 2 * 60 * 1000;

pub(crate) fn relay_payload_policy(
    relay_address: &str,
    network_id: &str,
    device_id: Option<&str>,
    path_type: Option<&str>,
) -> RelayPayloadPolicy {
    let _ = (network_id, device_id, path_type);
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
    let _ = (network_id, device_id, path_type);
    PathPolicy::default()
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
    let mut stats = serde_json::from_slice::<RelayRuntimeStats>(&payload).ok()?;
    refresh_relay_runtime_ticket_timing(&mut stats, current_timestamp_ms());
    Some(stats)
}

pub(crate) fn refresh_relay_runtime_ticket_timing(stats: &mut RelayRuntimeStats, now_ms: u64) {
    let timing = ticket_timing_with_window(
        now_ms,
        stats.ticket_expires_at.as_deref(),
        RELAY_TICKET_RENEW_WINDOW_MS,
    );
    stats.ticket_expires_in_ms = timing.expires_in_ms;
    stats.ticket_renew_due = timing.renew_due;
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

pub(crate) fn diagnostics_export_file_path() -> PathBuf {
    let file_name = format!("client-v2-diagnostics-{}.json", current_timestamp_ms());
    service_state_dir().join("diagnostics").join(file_name)
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

pub(crate) fn relay_stats_file_path() -> PathBuf {
    service_state_dir().join("client-v2-relay-stats.json")
}

fn service_state_dir() -> PathBuf {
    app_data_dir().join("SLAN")
}

#[cfg(test)]
mod tests {
    use super::*;

    fn relay_stats(expires_at: Option<&str>) -> RelayRuntimeStats {
        RelayRuntimeStats {
            relay_address: "127.0.0.1:3479".to_string(),
            relay_transport: Some("udp".to_string()),
            active_path: Some("relay_udp".to_string()),
            requested_relay_session_count: 1,
            relay_session_count: 1,
            attached_peer_session_count: 1,
            attached_transport_count: 1,
            ticket_expires_at: expires_at.map(str::to_string),
            ticket_expires_in_ms: None,
            ticket_renew_due: false,
            relay_attach_failures: 0,
            last_relay_attach_error: None,
            peers: Vec::new(),
            relay_mtu: Some(1280),
            max_frame_payload: Some(1200),
            tun_packets_sent: 0,
            relay_packets_received: 0,
            relay_decode_failures: 0,
            relay_config_hash_mismatches: 0,
            relay_error_responses: 0,
            last_relay_error: None,
            relay_send_failures: 0,
            relay_receive_failures: 0,
            unroutable_tun_packets: 0,
            last_unroutable_destination: None,
            oversized_tun_packets: 0,
            last_oversized_tun_packet_size: None,
            wintun_write_failures: 0,
            updated_at_ms: 0,
        }
    }

    #[test]
    fn refresh_relay_runtime_ticket_timing_recomputes_derived_fields() {
        let now = crate::time_utils::parse_rfc3339_utc_ms("2026-05-03T10:00:00Z").unwrap();
        let mut stats = relay_stats(Some("2026-05-03T10:01:30Z"));

        refresh_relay_runtime_ticket_timing(&mut stats, now);

        assert_eq!(stats.ticket_expires_in_ms, Some(90_000));
        assert!(stats.ticket_renew_due);
    }

    #[test]
    fn refresh_relay_runtime_ticket_timing_clears_missing_ticket() {
        let mut stats = relay_stats(None);
        stats.ticket_expires_in_ms = Some(1);
        stats.ticket_renew_due = true;

        refresh_relay_runtime_ticket_timing(&mut stats, 0);

        assert_eq!(stats.ticket_expires_in_ms, None);
        assert!(!stats.ticket_renew_due);
    }
}
