use super::{
    android_data_plane_relay_candidate, data_plane_relay_candidate,
    diagnostic_connect_plan_summaries, parse_rfc3339_utc_ms, path_diagnose_active_path_counts,
    path_diagnose_dns, path_diagnose_health, peer_path_configs,
    relay_candidate_matching_connect_plan_path, relay_maintenance_reconfigure_reason,
    relay_path_candidate_from_connect_plan, relay_reconfigure_backoff_applies,
    relay_session_from_connect_plan_ticket, relay_sessions_missing, relay_ticket_should_renew,
    relay_ticket_timing, relay_transport_for_path_type, routes_with_peer_virtual_ips,
    status_is_managed_disabled, ControlPeer, PersistedConnectPlan, PersistedConnectPlanPath,
    PersistedConnectPlanStore, RelayMaintenanceState, RELAY_NO_RX_RECONFIGURE_INTERVALS,
    RELAY_RESPONSE_GAP_DEGRADED_PACKETS,
};
use crate::control_plane::{PunchConnectSession, PunchEndpoint};
use crate::{
    relay_candidates::select_relay_candidates,
    relay_models::{
        PathDiagnoseDns, PathDiagnoseMtu, PathDiagnoseRelay, PersistedRelayCandidate,
        RelayCandidateSelection, RelayRuntimeStats,
    },
    relay_store::relay_runtime_failure_total,
};
use client_core::{PathKind, PeerPathRuntime, PlatformNetworkDiagnostics, RelayTicket, RouteSpec};
use std::net::{TcpListener, UdpSocket};

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
    let relay = UdpSocket::bind("127.0.0.1:0").expect("bind test relay udp socket");
    let address = relay.local_addr().unwrap().to_string();
    std::thread::spawn(move || {
        let mut buffer = [0_u8; 512];
        if let Ok((_, peer)) = relay.recv_from(&mut buffer) {
            let _ = relay.send_to(br#"{"kind":"pong"}"#, peer);
        }
    });
    let selections = select_relay_candidates(&[PersistedRelayCandidate {
        endpoint_id: "relay-udp".to_string(),
        transport: "udp".to_string(),
        address,
        country_code: Some("CN".to_string()),
        region_id: None,
        cluster_id: None,
    }]);
    assert_eq!(selections[0].endpoint_id, "relay-udp");
    assert!(selections[0].selected);
}

#[test]
fn android_data_plane_does_not_select_non_udp_relay() {
    let selected = android_data_plane_relay_candidate(&[PersistedRelayCandidate {
        endpoint_id: "relay-tcp".to_string(),
        transport: "tcp".to_string(),
        address: "127.0.0.1:9001".to_string(),
        country_code: Some("CN".to_string()),
        region_id: None,
        cluster_id: None,
    }]);

    assert!(selected.is_none());
}

#[test]
fn android_data_plane_selects_derp_when_udp_is_unavailable() {
    let listener = TcpListener::bind("127.0.0.1:0").expect("bind derp probe listener");
    let address = listener.local_addr().expect("derp listener address");
    let selected = android_data_plane_relay_candidate(&[PersistedRelayCandidate {
        endpoint_id: "derp-local".to_string(),
        transport: "derp_tcp_tls_443".to_string(),
        address: format!("derp://{address}"),
        country_code: Some("CN".to_string()),
        region_id: Some("dev".to_string()),
        cluster_id: Some("dev".to_string()),
    }]);

    assert_eq!(
        selected.map(|candidate| candidate.transport),
        Some("derp_tcp_tls_443".to_string())
    );
}

#[test]
fn macos_data_plane_uses_derp_candidate_when_probe_fails() {
    let selected = data_plane_relay_candidate(&[PersistedRelayCandidate {
        endpoint_id: "derp-remote".to_string(),
        transport: "derp_tcp_tls_443".to_string(),
        address: "derp://203.0.113.10:29120".to_string(),
        country_code: Some("CN".to_string()),
        region_id: Some("dev".to_string()),
        cluster_id: Some("dev".to_string()),
    }]);

    let selected = selected.expect("DERP candidate should be retained after probe failure");
    assert_eq!(selected.endpoint_id, "derp-remote");
    assert_eq!(selected.transport, "derp_tcp_tls_443");
    assert!(!selected.reachable);
    assert!(selected.selected);
}

#[test]
fn connect_plan_relay_path_becomes_path_candidate() {
    let candidate = relay_path_candidate_from_connect_plan(
        &PersistedConnectPlanPath {
            path_type: "relay_udp".to_string(),
            endpoint: "udp://relay.example:3478".to_string(),
            priority: 42,
        },
        None,
        &test_relay_selection("relay-other", "udp", "127.0.0.1:3478"),
    )
    .expect("relay_udp connect plan path should become candidate");

    assert_eq!(candidate.kind, PathKind::RelayUdp);
    assert_eq!(candidate.address.as_deref(), Some("relay.example:3478"));
    assert_eq!(candidate.transport.as_deref(), Some("udp"));
    assert_eq!(candidate.path_score, Some(42));
}

#[test]
fn connect_plan_rejects_relay_protocol_aliases() {
    assert!(relay_path_candidate_from_connect_plan(
        &PersistedConnectPlanPath {
            path_type: "relay_udp".to_string(),
            endpoint: "tcp://relay.example:443".to_string(),
            priority: 1,
        },
        None,
        &test_relay_selection("relay-other", "udp", "127.0.0.1:3478"),
    )
    .is_none());
    assert_eq!(relay_transport_for_path_type("relay_udp"), Some("udp"));
    assert_eq!(relay_transport_for_path_type("relay_http3"), None);
    assert_eq!(relay_transport_for_path_type("h3"), None);
    assert_eq!(relay_transport_for_path_type("quic"), None);
}

#[test]
fn connect_plan_path_selects_matching_reachable_relay_candidate() {
    let selected = relay_candidate_matching_connect_plan_path(
        &PersistedConnectPlanPath {
            path_type: "relay_udp".to_string(),
            endpoint: "relay+udp://relay.example:3478".to_string(),
            priority: 1,
        },
        &[test_relay_selection(
            "relay-udp",
            "udp",
            "relay.example:3478",
        )],
    )
    .expect("connect_plan relay path should match candidate");

    assert_eq!(selected.endpoint_id, "relay-udp");
    assert_eq!(selected.transport, "udp");
}

#[test]
fn connect_plan_path_ignores_unreachable_relay_candidate() {
    let mut candidate = test_relay_selection("relay-udp", "udp", "relay.example:3478");
    candidate.reachable = false;

    assert!(relay_candidate_matching_connect_plan_path(
        &PersistedConnectPlanPath {
            path_type: "relay_udp".to_string(),
            endpoint: "udp://relay.example:3478".to_string(),
            priority: 1,
        },
        &[candidate],
    )
    .is_none());
}

#[test]
fn connect_plan_relay_ticket_becomes_peer_session() {
    let peer = test_peer("node-peer", &["10.0.0.9"]);
    let plan = PersistedConnectPlan {
        peer_node_id: peer.node_id.clone(),
        prefer_direct: false,
        paths: Vec::new(),
        relay_ticket: Some(test_relay_ticket("net-1", "node-local", "node-peer")),
        updated_at_ms: 0,
    };

    let session = relay_session_from_connect_plan_ticket(&plan, "net-1", "node-local", &peer)
        .expect("valid connect_plan ticket should become relay session");

    assert_eq!(session.session_id, "session-1");
    assert_eq!(session.peer_node_id, "node-peer");
    assert_eq!(session.peer_virtual_ips, vec!["10.0.0.9".to_string()]);
}

#[test]
fn connect_plan_relay_ticket_must_match_peer() {
    let peer = test_peer("node-peer", &["10.0.0.9"]);
    let plan = PersistedConnectPlan {
        peer_node_id: peer.node_id.clone(),
        prefer_direct: false,
        paths: Vec::new(),
        relay_ticket: Some(test_relay_ticket("net-1", "node-local", "other-peer")),
        updated_at_ms: 0,
    };

    assert!(relay_session_from_connect_plan_ticket(&plan, "net-1", "node-local", &peer).is_none());
}

#[test]
fn punch_connect_session_peer_endpoint_becomes_direct_udp_candidate() {
    let mut peer = test_peer("node-peer", &["10.0.0.9"]);
    peer.endpoints.push(crate::control_plane::ControlEndpoint {
        endpoint_type: "lan".to_string(),
        address: "192.168.1.20:49152".to_string(),
        updated_at: 0,
    });
    let mut punch_sessions = std::collections::BTreeMap::new();
    punch_sessions.insert(
        peer.node_id.clone(),
        PunchConnectSession {
            session_id: "punch-1".to_string(),
            network_id: "net-1".to_string(),
            requester_node_id: "node-local".to_string(),
            peer_node_id: peer.node_id.clone(),
            requester: None,
            peer: Some(PunchEndpoint {
                network_id: "net-1".to_string(),
                node_id: peer.node_id.clone(),
                endpoint_type: "reflexive".to_string(),
                address: "10.1.1.20:49152".to_string(),
                reflexive: "203.0.113.20:49152".to_string(),
                nat_type: "unknown".to_string(),
            }),
        },
    );

    let paths = peer_path_configs(
        &[peer],
        "node-local",
        &test_relay_selection("relay-udp", "udp", "relay.example:3478"),
        &[],
        &[],
        Some(std::collections::BTreeMap::new()),
        Some(punch_sessions),
    );

    assert_eq!(paths.len(), 1);
    assert_eq!(paths[0].candidates[0].kind, PathKind::DirectUdp);
    assert_eq!(
        paths[0].candidates[0].address.as_deref(),
        Some("203.0.113.20:49152")
    );
    assert_eq!(
        paths[0].candidates[1].address.as_deref(),
        Some("192.168.1.20:49152")
    );
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
    assert!(relay_ticket_should_renew(now, Some("2026-05-03T10:05:00Z")));
    assert!(!relay_ticket_should_renew(
        now,
        Some("2026-05-03T10:05:01Z")
    ));
}

#[test]
fn relay_ticket_timing_reports_remaining_time_and_due_state() {
    let now = parse_rfc3339_utc_ms("2026-05-03T10:00:00Z").unwrap();

    let timing = relay_ticket_timing(now, Some("2026-05-03T10:01:30Z"));
    assert_eq!(timing.expires_in_ms, Some(90_000));
    assert!(timing.renew_due);

    let expired = relay_ticket_timing(now, Some("2026-05-03T09:59:59Z"));
    assert_eq!(expired.expires_in_ms, Some(-1_000));
    assert!(expired.renew_due);

    let missing = relay_ticket_timing(now, None);
    assert_eq!(missing.expires_in_ms, None);
    assert!(!missing.renew_due);
}

#[test]
fn relay_maintenance_reconfigures_immediately_when_ticket_is_expiring() {
    let now = parse_rfc3339_utc_ms("2026-05-03T10:00:00Z").unwrap();
    let stats = test_relay_stats("2026-05-03T10:01:00Z", now);
    let mut maintenance = RelayMaintenanceState::default();

    assert_eq!(
        relay_maintenance_reconfigure_reason(now, Some(&stats), &mut maintenance, 0),
        Some("ticket_expiring")
    );
}

#[test]
fn relay_maintenance_marks_expired_ticket_as_urgent() {
    let now = parse_rfc3339_utc_ms("2026-05-03T10:00:00Z").unwrap();
    let stats = test_relay_stats("2026-05-03T09:59:59Z", now);
    let mut maintenance = RelayMaintenanceState {
        last_reconfigure_ms: now.saturating_sub(10_000),
        last_failure_total: 0,
        last_attach_failures: 0,
        last_connect_plan_ms: 0,
        ..RelayMaintenanceState::default()
    };

    assert_eq!(
        relay_maintenance_reconfigure_reason(now, Some(&stats), &mut maintenance, 0),
        Some("ticket_expired")
    );
    assert!(!relay_reconfigure_backoff_applies("ticket_expired"));
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
        last_connect_plan_ms: 0,
        ..RelayMaintenanceState::default()
    };

    assert_eq!(
        relay_maintenance_reconfigure_reason(now, Some(&stats), &mut maintenance, 0),
        Some("relay_session_missing")
    );
}

#[test]
fn relay_session_missing_uses_attached_peer_sessions_not_transport_total() {
    let now = parse_rfc3339_utc_ms("2026-05-03T10:00:00Z").unwrap();
    let mut stats = test_relay_stats("2026-05-03T10:10:00Z", now);
    stats.requested_relay_session_count = 2;
    stats.relay_session_count = 2;
    stats.attached_peer_session_count = 1;
    stats.attached_transport_count = 3;

    assert!(relay_sessions_missing(&stats));

    stats.attached_peer_session_count = 2;
    assert!(!relay_sessions_missing(&stats));
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
        last_connect_plan_ms: 0,
        ..RelayMaintenanceState::default()
    };

    assert_eq!(
        relay_maintenance_reconfigure_reason(now, Some(&stats), &mut maintenance, 0),
        Some("relay_attach_failure")
    );
    assert_eq!(maintenance.last_attach_failures, 2);
}

#[test]
fn relay_maintenance_reconfigures_when_connect_plan_is_newer() {
    let now = parse_rfc3339_utc_ms("2026-05-03T10:00:00Z").unwrap();
    let stats = test_relay_stats("2026-05-03T10:10:00Z", now);
    let mut maintenance = RelayMaintenanceState {
        last_reconfigure_ms: now.saturating_sub(5 * 60 * 1000),
        last_failure_total: 0,
        last_attach_failures: 0,
        last_connect_plan_ms: now.saturating_sub(60 * 1000),
        ..RelayMaintenanceState::default()
    };

    assert_eq!(
        relay_maintenance_reconfigure_reason(now, Some(&stats), &mut maintenance, now),
        Some("connect_plan_updated")
    );
    assert_eq!(maintenance.last_connect_plan_ms, now);
}

#[test]
fn relay_maintenance_reconfigures_when_relay_stops_returning_packets() {
    let now = parse_rfc3339_utc_ms("2026-05-03T10:00:00Z").unwrap();
    let mut stats = test_relay_stats("2026-05-03T10:10:00Z", now);
    stats.tun_packets_sent = 10;
    stats.relay_packets_received = 8;
    let mut maintenance = RelayMaintenanceState {
        last_reconfigure_ms: now.saturating_sub(5 * 60 * 1000),
        last_tun_packets_sent: 5,
        last_relay_packets_received: 8,
        no_rx_intervals: RELAY_NO_RX_RECONFIGURE_INTERVALS - 1,
        ..RelayMaintenanceState::default()
    };

    assert_eq!(
        relay_maintenance_reconfigure_reason(now, Some(&stats), &mut maintenance, 0),
        Some("relay_response_stalled")
    );
}

#[test]
fn relay_maintenance_does_not_count_stall_when_replies_progress() {
    let now = parse_rfc3339_utc_ms("2026-05-03T10:00:00Z").unwrap();
    let mut stats = test_relay_stats("2026-05-03T10:10:00Z", now);
    stats.tun_packets_sent = 10;
    stats.relay_packets_received = 9;
    let mut maintenance = RelayMaintenanceState {
        last_reconfigure_ms: now.saturating_sub(5 * 60 * 1000),
        last_tun_packets_sent: 5,
        last_relay_packets_received: 8,
        no_rx_intervals: RELAY_NO_RX_RECONFIGURE_INTERVALS - 1,
        ..RelayMaintenanceState::default()
    };

    assert_eq!(
        relay_maintenance_reconfigure_reason(now, Some(&stats), &mut maintenance, 0),
        None
    );
    assert_eq!(maintenance.no_rx_intervals, 0);
}

#[test]
fn relay_failure_total_excludes_local_packet_noise() {
    let now = parse_rfc3339_utc_ms("2026-05-03T10:00:00Z").unwrap();
    let mut stats = test_relay_stats("2026-05-03T10:10:00Z", now);
    stats.oversized_tun_packets = 3;
    stats.relay_decode_failures = 2;
    stats.unroutable_tun_packets = 1;

    assert_eq!(relay_runtime_failure_total(&stats), 2);
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

#[test]
fn path_diagnose_health_fails_on_missing_attached_peer_sessions() {
    let relay = PathDiagnoseRelay {
        address: "127.0.0.1:3478".to_string(),
        transport: Some("udp".to_string()),
        active_path: Some("relay_udp".to_string()),
        requested_relay_session_count: 2,
        relay_session_count: 2,
        attached_peer_session_count: 1,
        attached_transport_count: 3,
        ticket_expires_at: None,
        ticket_expires_in_ms: Some(60_000),
        ticket_renew_due: false,
        relay_attach_failures: 1,
        last_relay_attach_error: Some("attach failed".to_string()),
        peers: Vec::new(),
        relay_mtu: Some(1280),
        max_frame_payload: Some(1200),
        tun_packets_sent: 0,
        relay_packets_received: 0,
        relay_error_responses: 0,
        relay_config_hash_mismatches: 0,
        last_relay_error: None,
        failures: 0,
        unroutable_tun_packets: 0,
        last_unroutable_destination: None,
        oversized_tun_packets: 0,
        last_oversized_tun_packet_size: None,
        started_at_ms: 0,
        last_tun_packet_at_ms: None,
        last_relay_packet_at_ms: None,
        last_relay_keepalive_at_ms: None,
        updated_at_ms: 0,
        stale: false,
    };
    let relay_candidates = vec![test_relay_selection("relay-udp", "udp", "127.0.0.1:3478")];
    let peer_paths = vec![test_peer_path("node-peer", Some(PathKind::RelayUdp))];

    let health = path_diagnose_health(
        Some(&relay),
        &PathDiagnoseMtu::default(),
        &PathDiagnoseDns::default(),
        &PlatformNetworkDiagnostics::default(),
        &relay_candidates,
        &peer_paths,
    );

    assert_eq!(health.status, "failed");
    assert!(health
        .reasons
        .iter()
        .any(|reason| reason.code == "relay_peer_sessions_not_attached"));
}

#[test]
fn path_diagnose_health_reports_ok_for_clean_relay() {
    let relay = PathDiagnoseRelay {
        address: "127.0.0.1:3478".to_string(),
        transport: Some("udp".to_string()),
        active_path: Some("relay_udp".to_string()),
        requested_relay_session_count: 1,
        relay_session_count: 1,
        attached_peer_session_count: 1,
        attached_transport_count: 1,
        ticket_expires_at: None,
        ticket_expires_in_ms: Some(60_000),
        ticket_renew_due: false,
        relay_attach_failures: 0,
        last_relay_attach_error: None,
        peers: Vec::new(),
        relay_mtu: Some(1280),
        max_frame_payload: Some(1200),
        tun_packets_sent: 0,
        relay_packets_received: 0,
        relay_error_responses: 0,
        relay_config_hash_mismatches: 0,
        last_relay_error: None,
        failures: 0,
        unroutable_tun_packets: 0,
        last_unroutable_destination: None,
        oversized_tun_packets: 0,
        last_oversized_tun_packet_size: None,
        started_at_ms: 0,
        last_tun_packet_at_ms: None,
        last_relay_packet_at_ms: None,
        last_relay_keepalive_at_ms: None,
        updated_at_ms: 0,
        stale: false,
    };
    let relay_candidates = vec![test_relay_selection("relay-udp", "udp", "127.0.0.1:3478")];
    let peer_paths = vec![test_peer_path("node-peer", Some(PathKind::RelayUdp))];

    let health = path_diagnose_health(
        Some(&relay),
        &PathDiagnoseMtu::default(),
        &PathDiagnoseDns::default(),
        &PlatformNetworkDiagnostics::default(),
        &relay_candidates,
        &peer_paths,
    );

    assert_eq!(health.status, "ok");
    assert!(health.reasons.is_empty());
}

#[test]
fn path_diagnose_health_reports_degraded_for_relay_response_gap() {
    let relay = PathDiagnoseRelay {
        address: "127.0.0.1:3478".to_string(),
        transport: Some("udp".to_string()),
        active_path: Some("relay_udp".to_string()),
        requested_relay_session_count: 1,
        relay_session_count: 1,
        attached_peer_session_count: 1,
        attached_transport_count: 1,
        ticket_expires_at: None,
        ticket_expires_in_ms: Some(60_000),
        ticket_renew_due: false,
        relay_attach_failures: 0,
        last_relay_attach_error: None,
        peers: Vec::new(),
        relay_mtu: Some(1280),
        max_frame_payload: Some(1200),
        tun_packets_sent: RELAY_RESPONSE_GAP_DEGRADED_PACKETS + 5,
        relay_packets_received: 5,
        relay_error_responses: 0,
        relay_config_hash_mismatches: 0,
        last_relay_error: None,
        failures: 0,
        unroutable_tun_packets: 0,
        last_unroutable_destination: None,
        oversized_tun_packets: 0,
        last_oversized_tun_packet_size: None,
        started_at_ms: 0,
        last_tun_packet_at_ms: Some(2),
        last_relay_packet_at_ms: Some(1),
        last_relay_keepalive_at_ms: None,
        updated_at_ms: 0,
        stale: false,
    };
    let relay_candidates = vec![test_relay_selection("relay-udp", "udp", "127.0.0.1:3478")];
    let peer_paths = vec![test_peer_path("node-peer", Some(PathKind::RelayUdp))];

    let health = path_diagnose_health(
        Some(&relay),
        &PathDiagnoseMtu::default(),
        &PathDiagnoseDns::default(),
        &PlatformNetworkDiagnostics::default(),
        &relay_candidates,
        &peer_paths,
    );

    assert_eq!(health.status, "degraded");
    assert!(health
        .reasons
        .iter()
        .any(|reason| reason.code == "relay_response_gap"));
}

#[test]
fn path_diagnose_health_accepts_runtime_peer_paths_without_stats() {
    let relay_candidates = vec![test_relay_selection("relay-udp", "udp", "127.0.0.1:3478")];
    let peer_paths = vec![test_peer_path("node-peer", Some(PathKind::RelayUdp))];

    let health = path_diagnose_health(
        None,
        &PathDiagnoseMtu::default(),
        &PathDiagnoseDns::default(),
        &PlatformNetworkDiagnostics::default(),
        &relay_candidates,
        &peer_paths,
    );

    assert_eq!(health.status, "ok");
    assert!(health.reasons.is_empty());
}

#[test]
fn diagnostic_connect_plan_summary_redacts_ticket_secret_fields() {
    let store = PersistedConnectPlanStore {
        plans: vec![PersistedConnectPlan {
            peer_node_id: "node-peer".to_string(),
            prefer_direct: true,
            paths: vec![PersistedConnectPlanPath {
                path_type: "relay_udp".to_string(),
                endpoint: "udp://relay.example:3478".to_string(),
                priority: 10,
            }],
            relay_ticket: Some(test_relay_ticket("net-1", "node-local", "node-peer")),
            updated_at_ms: 1_000,
        }],
    };

    let summaries = diagnostic_connect_plan_summaries(&store, 2_000);
    let encoded = serde_json::to_string(&summaries).expect("encode connect plan summaries");

    assert_eq!(summaries.len(), 1);
    assert!(encoded.contains("relayTicketExpiresAt"));
    assert!(encoded.contains("hasRelayTicket"));
    assert!(!encoded.contains("session-key"));
    assert!(!encoded.contains("signature"));
    assert!(!encoded.contains("ticket-1"));
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

fn test_relay_selection(
    endpoint_id: &str,
    transport: &str,
    address: &str,
) -> RelayCandidateSelection {
    RelayCandidateSelection {
        endpoint_id: endpoint_id.to_string(),
        transport: transport.to_string(),
        address: address.to_string(),
        country_code: None,
        region_id: None,
        cluster_id: None,
        reachable: true,
        rtt_ms: None,
        path_score: 0,
        selected: true,
    }
}

fn test_relay_ticket(network_id: &str, src_node_id: &str, dst_node_id: &str) -> RelayTicket {
    RelayTicket {
        ticket_id: "ticket-1".to_string(),
        network_id: network_id.to_string(),
        session_id: "session-1".to_string(),
        src_node_id: src_node_id.to_string(),
        dst_node_id: dst_node_id.to_string(),
        derp_cluster_id: Some("cluster-1".to_string()),
        country_code: None,
        city_code: None,
        allowed_derp_node_ids: vec!["relay-1".to_string()],
        relay_url: "udp://relay.example:3478".to_string(),
        expires_at: "2099-01-01T00:00:00Z".to_string(),
        session_key: "session-key".to_string(),
        signature: "signature".to_string(),
    }
}

fn test_relay_stats(ticket_expires_at: &str, updated_at_ms: u64) -> RelayRuntimeStats {
    RelayRuntimeStats {
        relay_address: "127.0.0.1:3478".to_string(),
        relay_transport: Some("udp".to_string()),
        active_path: Some("relay_udp".to_string()),
        requested_relay_session_count: 1,
        relay_session_count: 1,
        attached_peer_session_count: 1,
        attached_transport_count: 1,
        ticket_expires_at: Some(ticket_expires_at.to_string()),
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
        started_at_ms: updated_at_ms,
        last_tun_packet_at_ms: None,
        last_relay_packet_at_ms: None,
        last_relay_keepalive_at_ms: None,
        updated_at_ms,
    }
}
