use super::{
    parse_rfc3339_utc_ms, path_diagnose_active_path_counts, path_diagnose_dns,
    relay_maintenance_reconfigure_reason, relay_ticket_should_renew, routes_with_peer_virtual_ips,
    status_is_managed_disabled, ControlPeer, RelayMaintenanceState,
};
use crate::{
    relay_candidates::select_relay_candidates,
    relay_models::{PersistedRelayCandidate, RelayRuntimeStats},
    relay_store::relay_runtime_failure_total,
};
use client_core::{PathKind, PeerPathRuntime, RouteSpec};
use std::net::TcpListener;

#[test]
fn online_presence_statuses_do_not_disable_local_network() {
    for status in [
        None,
        Some(""),
        Some("active"),
        Some("online"),
        Some("offline"),
    ] {
        assert!(!status_is_managed_disabled(status));
    }
}

#[test]
fn managed_disable_statuses_disable_local_network() {
    for status in [
        Some("disabled"),
        Some("suspended"),
        Some("blocked"),
        Some("revoked"),
        Some("deleted"),
        Some("removed"),
    ] {
        assert!(status_is_managed_disabled(status));
    }
}

#[test]
fn relay_selection_prefers_reachable_low_score_candidate() {
    let listener = TcpListener::bind("127.0.0.1:0").expect("bind test relay tcp listener");
    let address = listener.local_addr().unwrap().to_string();
    let selections = select_relay_candidates(&[
        PersistedRelayCandidate {
            endpoint_id: "relay-http3".to_string(),
            transport: "http3".to_string(),
            address: address.clone(),
            country_code: None,
            region_id: None,
            cluster_id: None,
        },
        PersistedRelayCandidate {
            endpoint_id: "relay-tls".to_string(),
            transport: "tls".to_string(),
            address: address.clone(),
            country_code: Some("CN".to_string()),
            region_id: None,
            cluster_id: None,
        },
        PersistedRelayCandidate {
            endpoint_id: "relay-tcp".to_string(),
            transport: "tcp".to_string(),
            address,
            country_code: Some("CN".to_string()),
            region_id: None,
            cluster_id: None,
        },
    ]);
    assert_eq!(selections[0].endpoint_id, "relay-tcp");
    assert!(selections[0].selected);
    assert!(selections
        .iter()
        .filter(|item| item.transport == "tls" || item.transport == "http3")
        .all(|item| !item.reachable && !item.selected));
}

#[test]
fn routes_include_peer_virtual_ip_host_routes_for_multi_device_mesh() {
    let routes = routes_with_peer_virtual_ips(
        vec![RouteSpec {
            destination: "10.0.0.0/24".to_string(),
            gateway: None,
        }],
        &[
            test_peer("node-a", &["10.0.0.2/32"]),
            test_peer("node-b", &["10.0.0.9"]),
        ],
        "10.0.0.2",
    );

    assert!(routes
        .iter()
        .any(|route| route.destination == "10.0.0.0/24"));
    assert!(routes
        .iter()
        .any(|route| route.destination == "10.0.0.9/32"));
    assert!(!routes
        .iter()
        .any(|route| route.destination == "10.0.0.2/32"));
}

#[test]
fn routes_do_not_duplicate_existing_peer_host_routes() {
    let routes = routes_with_peer_virtual_ips(
        vec![RouteSpec {
            destination: "10.0.0.9/32".to_string(),
            gateway: None,
        }],
        &[test_peer("node-b", &["10.0.0.9"])],
        "10.0.0.2",
    );

    assert_eq!(
        routes
            .iter()
            .filter(|route| route.destination == "10.0.0.9/32")
            .count(),
        1
    );
}

#[test]
fn parses_rfc3339_utc_ticket_expiration() {
    assert_eq!(parse_rfc3339_utc_ms("1970-01-01T00:00:01Z"), Some(1_000));
    assert_eq!(
        parse_rfc3339_utc_ms("1970-01-01T00:00:01.500Z"),
        Some(1_000)
    );
    assert_eq!(parse_rfc3339_utc_ms("not-a-date"), None);
}

#[test]
fn relay_ticket_renews_inside_expiration_window() {
    let now = parse_rfc3339_utc_ms("2026-05-03T10:00:00Z").unwrap();

    assert!(relay_ticket_should_renew(now, Some("2026-05-03T10:01:30Z")));
    assert!(!relay_ticket_should_renew(
        now,
        Some("2026-05-03T10:05:00Z")
    ));
}

#[test]
fn relay_maintenance_reconfigures_immediately_when_ticket_is_expiring() {
    let now = parse_rfc3339_utc_ms("2026-05-03T10:00:00Z").unwrap();
    let stats = test_relay_stats("2026-05-03T10:01:00Z", now);
    let mut maintenance = RelayMaintenanceState::default();

    assert_eq!(
        relay_maintenance_reconfigure_reason(now, Some(&stats), &mut maintenance),
        Some("ticket_expiring")
    );
}

#[test]
fn relay_maintenance_reconfigures_when_peer_sessions_are_missing() {
    let now = parse_rfc3339_utc_ms("2026-05-03T10:00:00Z").unwrap();
    let mut stats = test_relay_stats("2026-05-03T10:10:00Z", now);
    stats.requested_relay_session_count = 2;
    stats.relay_session_count = 1;
    let mut maintenance = RelayMaintenanceState {
        last_reconfigure_ms: now.saturating_sub(5 * 60 * 1000),
        last_failure_total: 0,
        last_attach_failures: 0,
    };

    assert_eq!(
        relay_maintenance_reconfigure_reason(now, Some(&stats), &mut maintenance),
        Some("relay_session_missing")
    );
}

#[test]
fn relay_maintenance_reconfigures_when_attach_failures_increase() {
    let now = parse_rfc3339_utc_ms("2026-05-03T10:00:00Z").unwrap();
    let mut stats = test_relay_stats("2026-05-03T10:10:00Z", now);
    stats.relay_attach_failures = 2;
    let mut maintenance = RelayMaintenanceState {
        last_reconfigure_ms: now.saturating_sub(5 * 60 * 1000),
        last_failure_total: 0,
        last_attach_failures: 1,
    };

    assert_eq!(
        relay_maintenance_reconfigure_reason(now, Some(&stats), &mut maintenance),
        Some("relay_attach_failure")
    );
    assert_eq!(maintenance.last_attach_failures, 2);
}

#[test]
fn relay_failure_total_includes_oversized_tun_packets() {
    let now = parse_rfc3339_utc_ms("2026-05-03T10:00:00Z").unwrap();
    let mut stats = test_relay_stats("2026-05-03T10:10:00Z", now);
    stats.oversized_tun_packets = 3;
    stats.relay_decode_failures = 2;
    stats.unroutable_tun_packets = 1;

    assert_eq!(relay_runtime_failure_total(&stats), 6);
}

#[test]
fn path_diagnose_dns_reports_missing_expected_servers() {
    let dns = path_diagnose_dns(
        &["10.0.0.1".to_string(), "8.8.8.8".to_string()],
        &["10.0.0.1".to_string()],
    );

    assert!(dns.checked);
    assert_eq!(dns.ok, Some(false));
    assert_eq!(dns.missing_servers, vec!["8.8.8.8".to_string()]);
}

#[test]
fn path_diagnose_counts_active_paths_by_peer() {
    let counts = path_diagnose_active_path_counts(&[
        test_peer_path("node-a", Some(PathKind::DirectUdp)),
        test_peer_path("node-b", Some(PathKind::RelayUdp)),
        test_peer_path("node-c", Some(PathKind::DirectUdp)),
        test_peer_path("node-d", None),
    ]);

    assert_eq!(counts.len(), 3);
    assert_eq!(counts[0].path_type, "direct_udp");
    assert_eq!(counts[0].count, 2);
    assert!(counts
        .iter()
        .any(|item| item.path_type == "direct_udp" && item.count == 2));
    assert!(counts
        .iter()
        .any(|item| item.path_type == "relay_udp" && item.count == 1));
    assert!(counts
        .iter()
        .any(|item| item.path_type == "unknown" && item.count == 1));
}

fn test_peer(node_id: &str, virtual_ips: &[&str]) -> ControlPeer {
    ControlPeer {
        node_id: node_id.to_string(),
        virtual_ips: virtual_ips.iter().map(|value| value.to_string()).collect(),
        relay_allowed: true,
        endpoints: Vec::new(),
    }
}

fn test_peer_path(node_id: &str, active_path: Option<PathKind>) -> PeerPathRuntime {
    PeerPathRuntime {
        peer_node_id: node_id.to_string(),
        peer_virtual_ips: Vec::new(),
        active_path,
        candidates: Vec::new(),
    }
}

fn test_relay_stats(ticket_expires_at: &str, updated_at_ms: u64) -> RelayRuntimeStats {
    RelayRuntimeStats {
        relay_address: "127.0.0.1:3478".to_string(),
        active_path: Some("relay_udp".to_string()),
        requested_relay_session_count: 1,
        relay_session_count: 1,
        ticket_expires_at: Some(ticket_expires_at.to_string()),
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
        updated_at_ms,
    }
}
