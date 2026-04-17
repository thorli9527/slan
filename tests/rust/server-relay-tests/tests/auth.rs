use auth::StaticTicketValidator;
use relay_core::{RelayError, RelayTicket, TicketValidator};

#[test]
fn valid_ticket_passes() {
    let validator = StaticTicketValidator::new("udp://");
    let ticket = RelayTicket {
        ticket_id: "ticket-1".into(),
        network_id: "net-1".into(),
        session_id: "session-1".into(),
        src_node_id: "node-a".into(),
        dst_node_id: "node-b".into(),
        derp_cluster_id: None,
        allowed_derp_node_ids: vec![],
        relay_url: "udp://127.0.0.1:9000".into(),
        expires_at: "4102444800".into(),
        signature: "sig".into(),
    };

    validator.validate(&ticket).unwrap();
}

#[test]
fn expired_ticket_fails() {
    let validator = StaticTicketValidator::default();
    let ticket = RelayTicket {
        ticket_id: "ticket-1".into(),
        network_id: "net-1".into(),
        session_id: "session-1".into(),
        src_node_id: "node-a".into(),
        dst_node_id: "node-b".into(),
        derp_cluster_id: None,
        allowed_derp_node_ids: vec![],
        relay_url: "udp://127.0.0.1:9000".into(),
        expires_at: "1".into(),
        signature: "sig".into(),
    };

    assert_eq!(
        validator.validate(&ticket).unwrap_err(),
        RelayError::TicketExpired
    );
}
