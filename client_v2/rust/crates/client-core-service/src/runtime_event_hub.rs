use std::{
    collections::{HashMap, VecDeque},
    sync::{Condvar, Mutex},
    time::{Duration, SystemTime, UNIX_EPOCH},
};

use serde_json::Value;

const DEFAULT_EVENT_CAPACITY: usize = 256;
const MIN_DEDUP_CAPACITY: usize = 1_024;

#[derive(Debug, Clone, PartialEq)]
pub(crate) struct RuntimeBusinessEvent {
    pub(crate) revision: u64,
    pub(crate) event_id: String,
    pub(crate) business_type: String,
    pub(crate) business_data: Value,
    pub(crate) published_at_ms: u64,
}

#[derive(Debug, Clone, PartialEq)]
pub(crate) struct RuntimeEventRead {
    pub(crate) event: Option<RuntimeBusinessEvent>,
    pub(crate) oldest_available_revision: Option<u64>,
    pub(crate) latest_revision: u64,
    pub(crate) replay_gap: bool,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub(crate) struct RuntimeEventHubDiagnostics {
    pub(crate) stream_id: String,
    pub(crate) queue_capacity: usize,
    pub(crate) queue_depth: usize,
    pub(crate) dedup_capacity: usize,
    pub(crate) dedup_entries: usize,
    pub(crate) oldest_available_revision: Option<u64>,
    pub(crate) latest_revision: u64,
    pub(crate) published_total: u64,
    pub(crate) duplicate_suppressed_total: u64,
    pub(crate) evicted_total: u64,
    pub(crate) read_total: u64,
    pub(crate) delivered_total: u64,
    pub(crate) wait_total: u64,
    pub(crate) wait_timeout_total: u64,
    pub(crate) replay_gap_total: u64,
}

#[derive(Debug)]
struct RuntimeEventStore {
    revision: u64,
    published_total: u64,
    duplicate_suppressed_total: u64,
    evicted_total: u64,
    read_total: u64,
    delivered_total: u64,
    wait_total: u64,
    wait_timeout_total: u64,
    replay_gap_total: u64,
    events: VecDeque<RuntimeBusinessEvent>,
    dedup_order: VecDeque<(String, String)>,
    dedup_events: HashMap<(String, String), RuntimeBusinessEvent>,
}

/// Process-local event bus shared by every platform transport.
///
/// The bounded replay queue prevents a slow subscriber from silently skipping
/// intermediate events while keeping event production independent of Flutter.
#[derive(Debug)]
pub(crate) struct RuntimeEventHub {
    capacity: usize,
    dedup_capacity: usize,
    source_id: String,
    store: Mutex<RuntimeEventStore>,
    changed: Condvar,
}

impl Default for RuntimeEventHub {
    fn default() -> Self {
        Self::with_capacity(DEFAULT_EVENT_CAPACITY)
    }
}

impl RuntimeEventHub {
    pub(crate) fn with_capacity(capacity: usize) -> Self {
        Self {
            capacity: capacity.max(1),
            dedup_capacity: capacity.saturating_mul(4).max(MIN_DEDUP_CAPACITY),
            source_id: format!("{}-{}", std::process::id(), now_ms()),
            store: Mutex::new(RuntimeEventStore {
                revision: 0,
                published_total: 0,
                duplicate_suppressed_total: 0,
                evicted_total: 0,
                read_total: 0,
                delivered_total: 0,
                wait_total: 0,
                wait_timeout_total: 0,
                replay_gap_total: 0,
                events: VecDeque::with_capacity(capacity.max(1)),
                dedup_order: VecDeque::new(),
                dedup_events: HashMap::new(),
            }),
            changed: Condvar::new(),
        }
    }

    pub(crate) fn publish(
        &self,
        business_type: impl Into<String>,
        business_data: Value,
    ) -> RuntimeBusinessEvent {
        let mut store = self.store.lock().expect("runtime event hub mutex poisoned");
        let business_type = business_type.into();
        let upstream_event_id = upstream_event_id(&business_data);
        let dedup_key = upstream_event_id
            .as_ref()
            .map(|event_id| (business_type.clone(), event_id.clone()));
        if let Some(existing) = dedup_key
            .as_ref()
            .and_then(|key| store.dedup_events.get(key))
            .cloned()
        {
            store.duplicate_suppressed_total = store.duplicate_suppressed_total.saturating_add(1);
            return existing;
        }
        store.revision = store.revision.saturating_add(1);
        store.published_total = store.published_total.saturating_add(1);
        let revision = store.revision;
        let event = RuntimeBusinessEvent {
            revision,
            event_id: upstream_event_id
                .unwrap_or_else(|| format!("runtime-{}-{revision}", self.source_id)),
            business_type,
            business_data,
            published_at_ms: now_ms(),
        };
        store.events.push_back(event.clone());
        while store.events.len() > self.capacity {
            store.events.pop_front();
            store.evicted_total = store.evicted_total.saturating_add(1);
        }
        if let Some(key) = dedup_key {
            store.dedup_order.push_back(key.clone());
            store.dedup_events.insert(key, event.clone());
            while store.dedup_order.len() > self.dedup_capacity {
                if let Some(expired) = store.dedup_order.pop_front() {
                    store.dedup_events.remove(&expired);
                }
            }
        }
        drop(store);
        self.changed.notify_all();
        event
    }

    #[cfg(test)]
    pub(crate) fn latest_revision(&self) -> u64 {
        self.store
            .lock()
            .expect("runtime event hub mutex poisoned")
            .revision
    }

    pub(crate) fn stream_id(&self) -> &str {
        &self.source_id
    }

    pub(crate) fn diagnostics(&self) -> RuntimeEventHubDiagnostics {
        let store = self.store.lock().expect("runtime event hub mutex poisoned");
        RuntimeEventHubDiagnostics {
            stream_id: self.source_id.clone(),
            queue_capacity: self.capacity,
            queue_depth: store.events.len(),
            dedup_capacity: self.dedup_capacity,
            dedup_entries: store.dedup_events.len(),
            oldest_available_revision: store.events.front().map(|event| event.revision),
            latest_revision: store.revision,
            published_total: store.published_total,
            duplicate_suppressed_total: store.duplicate_suppressed_total,
            evicted_total: store.evicted_total,
            read_total: store.read_total,
            delivered_total: store.delivered_total,
            wait_total: store.wait_total,
            wait_timeout_total: store.wait_timeout_total,
            replay_gap_total: store.replay_gap_total,
        }
    }

    #[cfg(test)]
    pub(crate) fn next_after(&self, last_revision: u64) -> Option<RuntimeBusinessEvent> {
        self.read_after(last_revision).event
    }

    pub(crate) fn read_after(&self, last_revision: u64) -> RuntimeEventRead {
        let mut store = self.store.lock().expect("runtime event hub mutex poisoned");
        let read = event_read(&store, last_revision);
        record_read(&mut store, &read);
        read
    }

    #[cfg(test)]
    pub(crate) fn wait_next(
        &self,
        last_revision: u64,
        timeout: Duration,
    ) -> Option<RuntimeBusinessEvent> {
        self.wait_read_after(last_revision, timeout).event
    }

    pub(crate) fn wait_read_after(
        &self,
        last_revision: u64,
        timeout: Duration,
    ) -> RuntimeEventRead {
        let store = self.store.lock().expect("runtime event hub mutex poisoned");
        let (mut store, wait_timed_out) = if store.revision <= last_revision {
            let (store, wait_result) = self
                .changed
                .wait_timeout_while(store, timeout, |current| current.revision <= last_revision)
                .expect("runtime event hub condvar poisoned");
            (store, wait_result.timed_out())
        } else {
            (store, false)
        };
        let read = event_read(&store, last_revision);
        store.wait_total = store.wait_total.saturating_add(1);
        if wait_timed_out && read.event.is_none() {
            store.wait_timeout_total = store.wait_timeout_total.saturating_add(1);
        }
        record_read(&mut store, &read);
        read
    }

    #[cfg(test)]
    #[allow(dead_code)]
    pub(crate) fn reset(&self) {
        let mut store = self.store.lock().expect("runtime event hub mutex poisoned");
        store.revision = 0;
        store.published_total = 0;
        store.duplicate_suppressed_total = 0;
        store.evicted_total = 0;
        store.read_total = 0;
        store.delivered_total = 0;
        store.wait_total = 0;
        store.wait_timeout_total = 0;
        store.replay_gap_total = 0;
        store.events.clear();
        store.dedup_order.clear();
        store.dedup_events.clear();
    }
}

fn upstream_event_id(business_data: &Value) -> Option<String> {
    ["eventId", "messageId"]
        .into_iter()
        .find_map(|key| business_data.get(key).and_then(Value::as_str))
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .map(str::to_string)
}

fn next_event(store: &RuntimeEventStore, last_revision: u64) -> Option<RuntimeBusinessEvent> {
    store
        .events
        .iter()
        .find(|event| event.revision > last_revision)
        .cloned()
}

fn event_read(store: &RuntimeEventStore, last_revision: u64) -> RuntimeEventRead {
    let oldest_available_revision = store.events.front().map(|event| event.revision);
    RuntimeEventRead {
        event: next_event(store, last_revision),
        oldest_available_revision,
        latest_revision: store.revision,
        replay_gap: oldest_available_revision.is_some_and(|oldest| {
            last_revision < store.revision && last_revision.saturating_add(1) < oldest
        }),
    }
}

fn record_read(store: &mut RuntimeEventStore, read: &RuntimeEventRead) {
    store.read_total = store.read_total.saturating_add(1);
    if read.event.is_some() {
        store.delivered_total = store.delivered_total.saturating_add(1);
    }
    if read.replay_gap {
        store.replay_gap_total = store.replay_gap_total.saturating_add(1);
    }
}

fn now_ms() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_millis()
        .min(u64::MAX as u128) as u64
}

#[cfg(test)]
mod tests {
    use super::RuntimeEventHub;
    use std::{sync::Arc, thread, time::Duration};

    #[test]
    fn replays_every_event_in_revision_order() {
        let hub = RuntimeEventHub::with_capacity(4);
        hub.publish("session.changed", serde_json::json!({"step": 1}));
        hub.publish("network.runtime.changed", serde_json::json!({"step": 2}));

        let first = hub.next_after(0).expect("first event");
        let second = hub.next_after(first.revision).expect("second event");

        assert_eq!(first.revision, 1);
        assert_eq!(first.business_type, "session.changed");
        assert_eq!(second.revision, 2);
        assert_eq!(second.business_type, "network.runtime.changed");
    }

    #[test]
    fn bounded_queue_replays_oldest_retained_event() {
        let hub = RuntimeEventHub::with_capacity(2);
        hub.publish("one", serde_json::json!({}));
        hub.publish("two", serde_json::json!({}));
        hub.publish("three", serde_json::json!({}));

        let event = hub.next_after(0).expect("oldest retained event");
        assert_eq!(event.revision, 2);
        assert_eq!(event.business_type, "two");
    }

    #[test]
    fn duplicate_upstream_event_id_is_published_once() {
        let hub = RuntimeEventHub::with_capacity(4);
        let first = hub.publish(
            "control.sync.changed",
            serde_json::json!({"eventId": "network-event-1", "attempt": 1}),
        );
        let duplicate = hub.publish(
            "control.sync.changed",
            serde_json::json!({"eventId": "network-event-1", "attempt": 2}),
        );

        assert_eq!(duplicate.revision, first.revision);
        assert_eq!(hub.latest_revision(), 1);
        assert!(hub.next_after(first.revision).is_none());
    }

    #[test]
    fn duplicate_is_suppressed_after_original_leaves_replay_queue() {
        let hub = RuntimeEventHub::with_capacity(2);
        let first = hub.publish(
            "control.sync.changed",
            serde_json::json!({"eventId": "network-event-1"}),
        );
        hub.publish("two", serde_json::json!({}));
        hub.publish("three", serde_json::json!({}));

        let duplicate = hub.publish(
            "control.sync.changed",
            serde_json::json!({"eventId": "network-event-1"}),
        );
        assert_eq!(duplicate.revision, first.revision);
        assert_eq!(hub.latest_revision(), 3);
    }

    #[test]
    fn reports_replay_gap_when_subscriber_is_behind_retained_queue() {
        let hub = RuntimeEventHub::with_capacity(2);
        hub.publish("one", serde_json::json!({}));
        hub.publish("two", serde_json::json!({}));
        hub.publish("three", serde_json::json!({}));

        let read = hub.read_after(0);
        assert!(read.replay_gap);
        assert_eq!(read.oldest_available_revision, Some(2));
        assert_eq!(read.latest_revision, 3);
        assert_eq!(read.event.expect("oldest retained event").revision, 2);
    }

    #[test]
    fn wait_next_releases_store_lock_and_receives_publication() {
        let hub = Arc::new(RuntimeEventHub::with_capacity(4));
        let publisher = Arc::clone(&hub);
        let task = thread::spawn(move || {
            thread::sleep(Duration::from_millis(10));
            publisher.publish("state.changed", serde_json::json!({}));
        });

        let event = hub
            .wait_next(0, Duration::from_secs(1))
            .expect("published event");
        task.join().expect("publisher thread");
        assert_eq!(event.revision, 1);
    }

    #[test]
    fn wait_next_returns_none_after_timeout() {
        let hub = RuntimeEventHub::with_capacity(4);
        assert!(hub.wait_next(0, Duration::from_millis(1)).is_none());
    }

    #[test]
    fn diagnostics_report_deduplication_and_replay_eviction() {
        let hub = RuntimeEventHub::with_capacity(1);
        hub.publish("one", serde_json::json!({"eventId": "event-1"}));
        hub.publish("one", serde_json::json!({"eventId": "event-1"}));
        hub.publish("two", serde_json::json!({"eventId": "event-2"}));

        let diagnostics = hub.diagnostics();
        assert_eq!(diagnostics.queue_capacity, 1);
        assert_eq!(diagnostics.queue_depth, 1);
        assert_eq!(diagnostics.latest_revision, 2);
        assert_eq!(diagnostics.published_total, 2);
        assert_eq!(diagnostics.duplicate_suppressed_total, 1);
        assert_eq!(diagnostics.evicted_total, 1);
        assert_eq!(diagnostics.oldest_available_revision, Some(2));
        assert_eq!(diagnostics.dedup_entries, 2);
    }

    #[test]
    fn diagnostics_report_event_delivery_timeout_and_replay_gap() {
        let hub = RuntimeEventHub::with_capacity(1);
        hub.publish("one", serde_json::json!({}));
        hub.publish("two", serde_json::json!({}));

        let replay = hub.read_after(0);
        assert!(replay.replay_gap);
        assert!(replay.event.is_some());
        assert!(hub
            .wait_read_after(replay.latest_revision, Duration::from_millis(1))
            .event
            .is_none());

        let diagnostics = hub.diagnostics();
        assert_eq!(diagnostics.read_total, 2);
        assert_eq!(diagnostics.delivered_total, 1);
        assert_eq!(diagnostics.wait_total, 1);
        assert_eq!(diagnostics.wait_timeout_total, 1);
        assert_eq!(diagnostics.replay_gap_total, 1);
    }
}
