use std::{
    collections::{BTreeMap, VecDeque},
    sync::{Mutex, OnceLock},
};

use crate::network_event::{
    NetworkEventAclRuleView, NetworkEventDeviceGroupView, NetworkEventMemberView,
    NetworkEventNetworkView, NetworkEventPeerPathView, NetworkEventResolverRecordView,
};
use crate::session_store::PersistedSession;

#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub enum NetworkSyncStatus {
    #[default]
    Idle,
    SyncingSnapshot,
    Live,
    OutOfSync,
}

#[derive(Debug, Clone, Default)]
pub struct RuntimeNetworkState {
    pub active_network_id: Option<String>,
    pub version: u64,
    pub network: Option<NetworkEventNetworkView>,
    pub self_device_id: Option<String>,
    pub self_virtual_ip: Option<String>,

    pub members_by_device_id: BTreeMap<String, NetworkEventMemberView>,
    pub groups_by_group_id: BTreeMap<String, NetworkEventDeviceGroupView>,
    pub records_by_record_id: BTreeMap<String, NetworkEventResolverRecordView>,
    pub acl_by_rule_id: BTreeMap<String, NetworkEventAclRuleView>,
    pub peer_paths_by_device_id: BTreeMap<String, NetworkEventPeerPathView>,

    pub sync_status: NetworkSyncStatus,
    pub last_event_id: Option<String>,
    pub last_event_at: Option<u64>,
    pub recent_event_ids: VecDeque<String>,
}

impl RuntimeNetworkState {
    pub fn bind_network_id(&mut self, network_id: &str) {
        let network_id = network_id.trim();
        if network_id.is_empty() {
            return;
        }
        self.active_network_id = Some(network_id.to_string());
    }

    pub fn bind_session_identity(&mut self, device_id: Option<&str>, virtual_ip: Option<&str>) {
        self.self_device_id = device_id
            .map(str::trim)
            .filter(|value| !value.is_empty())
            .map(str::to_string);
        self.self_virtual_ip = virtual_ip
            .map(str::trim)
            .filter(|value| !value.is_empty())
            .map(str::to_string);
    }

    pub fn bind_persisted_session(&mut self, session: &PersistedSession) {
        self.bind_session_identity(session.device_id.as_deref(), session.virtual_ip.as_deref());
    }

    pub fn mark_syncing_snapshot(&mut self) {
        self.sync_status = NetworkSyncStatus::SyncingSnapshot;
    }

    pub fn remember_event(&mut self, event_id: String, occurred_at: u64) {
        if event_id.is_empty() {
            return;
        }
        self.last_event_at = Some(occurred_at);
        self.last_event_id = Some(event_id.clone());
        self.recent_event_ids.push_back(event_id);
        while self.recent_event_ids.len() > 64 {
            self.recent_event_ids.pop_front();
        }
    }

    pub fn has_seen_event(&self, event_id: &str) -> bool {
        self.recent_event_ids.iter().any(|item| item == event_id)
    }
}

#[derive(Debug, Clone)]
pub struct RuntimeNetworkSnapshot {
    pub revision: u64,
    pub state: RuntimeNetworkState,
}

#[derive(Debug, Default)]
struct RuntimeNetworkStateStoreInner {
    revision: u64,
    state: RuntimeNetworkState,
}

#[derive(Debug, Default)]
pub struct RuntimeNetworkStateStore {
    inner: Mutex<RuntimeNetworkStateStoreInner>,
}

impl RuntimeNetworkStateStore {
    pub fn snapshot(&self) -> RuntimeNetworkSnapshot {
        let inner = self.inner.lock().unwrap_or_else(|error| error.into_inner());
        RuntimeNetworkSnapshot {
            revision: inner.revision,
            state: inner.state.clone(),
        }
    }

    pub fn snapshot_for_session(
        &self,
        session: Option<&PersistedSession>,
    ) -> RuntimeNetworkSnapshot {
        let mut snapshot = self.snapshot();
        if let Some(session) = session {
            snapshot.state.bind_persisted_session(session);
        }
        snapshot
    }

    pub fn read<R>(&self, read: impl FnOnce(&RuntimeNetworkState) -> R) -> R {
        let inner = self.inner.lock().unwrap_or_else(|error| error.into_inner());
        read(&inner.state)
    }

    #[cfg(test)]
    pub fn update<R>(&self, update: impl FnOnce(&mut RuntimeNetworkState) -> R) -> R {
        let mut inner = self.inner.lock().unwrap_or_else(|error| error.into_inner());
        let result = update(&mut inner.state);
        inner.revision = inner.revision.saturating_add(1);
        result
    }

    pub fn try_update<R, E>(
        &self,
        update: impl FnOnce(&mut RuntimeNetworkState) -> Result<R, E>,
    ) -> Result<R, E> {
        let mut inner = self.inner.lock().unwrap_or_else(|error| error.into_inner());
        let mut next = inner.state.clone();
        let result = update(&mut next)?;
        inner.state = next;
        inner.revision = inner.revision.saturating_add(1);
        Ok(result)
    }

    pub fn clear(&self) {
        let mut inner = self.inner.lock().unwrap_or_else(|error| error.into_inner());
        inner.state = RuntimeNetworkState::default();
        inner.revision = inner.revision.saturating_add(1);
    }
}

pub fn runtime_network_state_store() -> &'static RuntimeNetworkStateStore {
    static STORE: OnceLock<RuntimeNetworkStateStore> = OnceLock::new();
    STORE.get_or_init(RuntimeNetworkStateStore::default)
}

#[cfg(test)]
mod tests {
    use super::RuntimeNetworkStateStore;

    #[test]
    fn store_updates_revision_and_returns_consistent_snapshots() {
        let store = RuntimeNetworkStateStore::default();
        assert_eq!(store.snapshot().revision, 0);

        store.update(|state| {
            state.bind_network_id("network-1");
            state.bind_session_identity(Some("device-1"), Some("10.0.0.1"));
        });

        let snapshot = store.snapshot();
        assert_eq!(snapshot.revision, 1);
        assert_eq!(
            snapshot.state.active_network_id.as_deref(),
            Some("network-1")
        );
        assert_eq!(
            store.read(|state| state.self_virtual_ip.clone()),
            Some("10.0.0.1".to_string())
        );
    }

    #[test]
    fn failed_transaction_does_not_publish_partial_network_state() {
        let store = RuntimeNetworkStateStore::default();
        store.update(|state| state.bind_network_id("network-1"));

        let result = store.try_update(|state| {
            state.bind_network_id("network-2");
            Err::<(), _>("expected failure")
        });

        assert_eq!(result, Err("expected failure"));
        let snapshot = store.snapshot();
        assert_eq!(snapshot.revision, 1);
        assert_eq!(
            snapshot.state.active_network_id.as_deref(),
            Some("network-1")
        );
    }

    #[test]
    fn session_enriched_snapshot_does_not_advance_store_revision() {
        let store = RuntimeNetworkStateStore::default();
        let mut session = crate::session_store::PersistedSession::empty();
        session.device_id = Some("device-1".to_string());
        session.virtual_ip = Some("10.0.0.1".to_string());

        let snapshot = store.snapshot_for_session(Some(&session));

        assert_eq!(snapshot.revision, 0);
        assert_eq!(snapshot.state.self_device_id.as_deref(), Some("device-1"));
        assert_eq!(snapshot.state.self_virtual_ip.as_deref(), Some("10.0.0.1"));
        assert_eq!(store.snapshot().revision, 0);
        assert_eq!(store.snapshot().state.self_device_id, None);
    }

    #[test]
    fn clear_removes_session_scoped_network_state() {
        let store = RuntimeNetworkStateStore::default();
        store.update(|state| {
            state.bind_network_id("network-1");
            state.bind_session_identity(Some("device-1"), Some("10.0.0.1"));
        });

        store.clear();

        let snapshot = store.snapshot();
        assert_eq!(snapshot.revision, 2);
        assert_eq!(snapshot.state.active_network_id, None);
        assert_eq!(snapshot.state.self_device_id, None);
        assert!(snapshot.state.members_by_device_id.is_empty());
        assert!(snapshot.state.acl_by_rule_id.is_empty());
    }
}
