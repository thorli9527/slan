use relay_core::{RelayError, RelaySession};
use session::{InMemorySessionStore, SessionStore};

#[test]
fn store_round_trip_works() {
    let store = InMemorySessionStore::new();
    let session = RelaySession {
        session_id: "s-1".into(),
        source_device_id: "a".into(),
        target_device_id: "b".into(),
        network_id: "n-1".into(),
    };

    store.create(session.clone()).unwrap();
    assert_eq!(store.get("s-1"), Some(session));
    store.remove("s-1").unwrap();
    assert!(store.get("s-1").is_none());
}

#[test]
fn duplicate_session_is_rejected() {
    let store = InMemorySessionStore::new();
    let session = RelaySession {
        session_id: "s-1".into(),
        source_device_id: "a".into(),
        target_device_id: "b".into(),
        network_id: "n-1".into(),
    };

    store.create(session.clone()).unwrap();
    assert_eq!(
        store.create(session).unwrap_err(),
        RelayError::SessionAlreadyExists
    );
}
