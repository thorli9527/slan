use relay_core::{parse_timestamp, RelaySession, RelayTicket};

#[test]
fn parses_rfc3339_timestamp() {
    assert_eq!(parse_timestamp("1970-01-01T00:00:00Z").unwrap(), 0);
    assert_eq!(parse_timestamp("1970-01-01T08:00:00+08:00").unwrap(), 0);
}

#[test]
fn ticket_expiry_works() {
    let ticket = RelayTicket {
        ticket_id: "ticket-1".into(),
        network_id: "net-1".into(),
        session_id: "session-1".into(),
        src_node_id: "node-a".into(),
        dst_node_id: "node-b".into(),
        derp_cluster_id: None,
        country_code: None,
        city_code: None,
        allowed_derp_node_ids: vec![],
        relay_url: "udp://127.0.0.1:9000".into(),
        expires_at: "100".into(),
        signature: "sig".into(),
    };

    assert!(!ticket.is_expired_at(99).unwrap());
    assert!(ticket.is_expired_at(100).unwrap());
}

#[test]
fn session_peer_lookup_works() {
    let session = RelaySession {
        session_id: "s-1".into(),
        source_device_id: "a".into(),
        target_device_id: "b".into(),
        network_id: "n".into(),
    };

    assert_eq!(session.peer_of("a").unwrap(), "b");
    assert_eq!(session.peer_of("b").unwrap(), "a");
    assert!(session.peer_of("c").is_err());
}
