use relay_core::{RelayError, RelaySession, RelayTicket};
use session::InMemorySessionStore;
use udp_relay::{ForwardedPacket, RelayPacket, UdpRelay};

mod support;

#[test]
fn attach_with_ticket_and_forward_works() {
    let relay = UdpRelay::new(
        InMemorySessionStore::new(),
        auth::StaticTicketValidator::new(support::DEV_RELAY_URL_PREFIX),
    );
    let ticket = RelayTicket {
        ticket_id: "ticket-1".into(),
        network_id: "net-1".into(),
        session_id: "s-1".into(),
        src_node_id: "node-a".into(),
        dst_node_id: "node-b".into(),
        derp_cluster_id: None,
        country_code: None,
        city_code: None,
        allowed_derp_node_ids: vec![],
        relay_url: support::DEV_RELAY_UDP_URL.into(),
        expires_at: "4102444800".into(),
        session_key: "session-key-1".into(),
        signature: "sig".into(),
    };
    let session = RelaySession {
        session_id: "s-1".into(),
        source_device_id: "dev-a".into(),
        target_device_id: "dev-b".into(),
        network_id: "net-1".into(),
    };

    relay.attach_with_ticket(&ticket, session).unwrap();
    let forwarded = relay
        .forward(RelayPacket {
            session_id: "s-1".into(),
            from_device_id: "dev-a".into(),
            payload: b"ping".to_vec(),
        })
        .unwrap();

    assert_eq!(
        forwarded,
        ForwardedPacket {
            session_id: "s-1".into(),
            to_device_id: "dev-b".into(),
            payload: b"ping".to_vec(),
        }
    );
}

#[test]
fn non_participant_is_rejected() {
    let relay = UdpRelay::new(
        InMemorySessionStore::new(),
        auth::StaticTicketValidator::default(),
    );
    relay
        .attach(RelaySession {
            session_id: "s-1".into(),
            source_device_id: "dev-a".into(),
            target_device_id: "dev-b".into(),
            network_id: "net-1".into(),
        })
        .unwrap();

    assert_eq!(
        relay
            .forward(RelayPacket {
                session_id: "s-1".into(),
                from_device_id: "dev-c".into(),
                payload: b"ping".to_vec(),
            })
            .unwrap_err(),
        RelayError::UnauthorizedPeer
    );
}
