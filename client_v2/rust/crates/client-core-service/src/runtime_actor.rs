use std::{
    collections::VecDeque,
    panic::{catch_unwind, AssertUnwindSafe},
    sync::{
        atomic::{AtomicBool, AtomicU64, AtomicU8, AtomicUsize, Ordering},
        mpsc::{self, Receiver, SyncSender, TrySendError},
        Arc, Condvar, Mutex,
    },
    thread,
    time::{Duration, SystemTime, UNIX_EPOCH},
};

use anyhow::{anyhow, Result};
use client_core::{ClientRuntime, ClientViewState};
use client_core_platform::PlatformNetworkImpl;
use serde_json::Value;

use crate::runtime_event_hub::{RuntimeBusinessEvent, RuntimeEventHub};

const DEFAULT_COMMAND_CAPACITY: usize = 128;
const DEFAULT_COMMAND_TIMEOUT: Duration = Duration::from_secs(60);
const COMMAND_COMPLETION_CAPACITY: usize = 256;
const COMMAND_QUEUED: u8 = 0;
const COMMAND_RUNNING: u8 = 1;
const COMMAND_CANCELLED: u8 = 2;
const COMMAND_COMPLETED: u8 = 3;

type PlatformRuntime = ClientRuntime<PlatformNetworkImpl>;
type RuntimeJob = Box<dyn FnOnce(&mut PlatformRuntime) + Send + 'static>;

#[derive(Debug, Clone, PartialEq, Eq)]
pub(crate) struct RuntimeActorDiagnostics {
    pub(crate) queue_capacity: usize,
    pub(crate) queue_depth: usize,
    pub(crate) running: bool,
    pub(crate) accepted_total: u64,
    pub(crate) completed_total: u64,
    pub(crate) failed_total: u64,
    pub(crate) rejected_total: u64,
    pub(crate) timed_out_total: u64,
    pub(crate) cancelled_before_start_total: u64,
    pub(crate) active_command: Option<RuntimeActiveCommandDiagnostic>,
    pub(crate) last_command: Option<RuntimeCommandDiagnostic>,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub(crate) struct RuntimeActiveCommandDiagnostic {
    pub(crate) command_id: u64,
    pub(crate) kind: String,
    pub(crate) correlation_id: Option<String>,
    pub(crate) queued_at_ms: u64,
    pub(crate) started_at_ms: u64,
    pub(crate) caller_timed_out: bool,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub(crate) struct RuntimeCommandDiagnostic {
    pub(crate) command_id: u64,
    pub(crate) kind: String,
    pub(crate) correlation_id: Option<String>,
    pub(crate) queued_at_ms: u64,
    pub(crate) started_at_ms: Option<u64>,
    pub(crate) finished_at_ms: u64,
    pub(crate) duration_ms: Option<u64>,
    pub(crate) outcome: String,
    pub(crate) caller_timed_out: bool,
}

#[derive(Debug)]
struct RuntimeActorMetrics {
    queue_capacity: usize,
    queue_depth: AtomicUsize,
    running: AtomicBool,
    accepted_total: AtomicU64,
    completed_total: AtomicU64,
    failed_total: AtomicU64,
    rejected_total: AtomicU64,
    timed_out_total: AtomicU64,
    cancelled_before_start_total: AtomicU64,
    command_sequence: AtomicU64,
    active_command: Mutex<Option<RuntimeActiveCommandDiagnostic>>,
    last_command: Mutex<Option<RuntimeCommandDiagnostic>>,
    command_completions: RuntimeCommandCompletionHub,
}

#[derive(Debug, Default)]
struct RuntimeCommandCompletionStore {
    events: VecDeque<RuntimeCommandDiagnostic>,
}

#[derive(Debug, Default)]
struct RuntimeCommandCompletionHub {
    store: Mutex<RuntimeCommandCompletionStore>,
    changed: Condvar,
}

impl RuntimeCommandCompletionHub {
    fn publish(&self, diagnostic: RuntimeCommandDiagnostic) {
        let mut store = self
            .store
            .lock()
            .expect("runtime command completion mutex poisoned");
        if store.events.len() == COMMAND_COMPLETION_CAPACITY {
            store.events.pop_front();
        }
        store.events.push_back(diagnostic);
        drop(store);
        self.changed.notify_all();
    }

    fn wait_for(
        &self,
        kind: &str,
        correlation_id: Option<&str>,
        not_before_ms: u64,
        timeout: Duration,
    ) -> bool {
        let matches = |event: &RuntimeCommandDiagnostic| {
            event.kind == kind
                && event.correlation_id.as_deref() == correlation_id
                && event.finished_at_ms >= not_before_ms
        };
        let store = self
            .store
            .lock()
            .expect("runtime command completion mutex poisoned");
        let store = self
            .changed
            .wait_timeout_while(store, timeout, |current| {
                !current.events.iter().any(matches)
            })
            .expect("runtime command completion condvar poisoned")
            .0;
        store.events.iter().any(matches)
    }
}

impl RuntimeActorMetrics {
    fn new(queue_capacity: usize) -> Self {
        Self {
            queue_capacity,
            queue_depth: AtomicUsize::new(0),
            running: AtomicBool::new(false),
            accepted_total: AtomicU64::new(0),
            completed_total: AtomicU64::new(0),
            failed_total: AtomicU64::new(0),
            rejected_total: AtomicU64::new(0),
            timed_out_total: AtomicU64::new(0),
            cancelled_before_start_total: AtomicU64::new(0),
            command_sequence: AtomicU64::new(0),
            active_command: Mutex::new(None),
            last_command: Mutex::new(None),
            command_completions: RuntimeCommandCompletionHub::default(),
        }
    }

    fn snapshot(&self) -> RuntimeActorDiagnostics {
        RuntimeActorDiagnostics {
            queue_capacity: self.queue_capacity,
            queue_depth: self.queue_depth.load(Ordering::Relaxed),
            running: self.running.load(Ordering::Relaxed),
            accepted_total: self.accepted_total.load(Ordering::Relaxed),
            completed_total: self.completed_total.load(Ordering::Relaxed),
            failed_total: self.failed_total.load(Ordering::Relaxed),
            rejected_total: self.rejected_total.load(Ordering::Relaxed),
            timed_out_total: self.timed_out_total.load(Ordering::Relaxed),
            cancelled_before_start_total: self.cancelled_before_start_total.load(Ordering::Relaxed),
            active_command: self
                .active_command
                .lock()
                .expect("runtime actor active command mutex poisoned")
                .clone(),
            last_command: self
                .last_command
                .lock()
                .expect("runtime actor diagnostic mutex poisoned")
                .clone(),
        }
    }

    fn next_command_id(&self) -> u64 {
        self.command_sequence.fetch_add(1, Ordering::Relaxed) + 1
    }

    fn record_command(&self, diagnostic: RuntimeCommandDiagnostic) {
        self.command_completions.publish(diagnostic.clone());
        *self
            .last_command
            .lock()
            .expect("runtime actor diagnostic mutex poisoned") = Some(diagnostic);
    }

    fn set_active_command(&self, command: Option<RuntimeActiveCommandDiagnostic>) {
        *self
            .active_command
            .lock()
            .expect("runtime actor active command mutex poisoned") = command;
    }

    fn mark_active_command_timed_out(&self, command_id: u64) {
        let mut active = self
            .active_command
            .lock()
            .expect("runtime actor active command mutex poisoned");
        if let Some(command) = active
            .as_mut()
            .filter(|value| value.command_id == command_id)
        {
            command.caller_timed_out = true;
        }
    }
}

#[derive(Debug, Clone)]
pub(crate) struct RuntimeSnapshot {
    pub(crate) revision: u64,
    pub(crate) state: ClientViewState,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub(crate) struct RuntimeSnapshotHubDiagnostics {
    pub(crate) revision: u64,
    pub(crate) publish_attempt_total: u64,
    pub(crate) changed_total: u64,
    pub(crate) read_total: u64,
    pub(crate) wait_total: u64,
    pub(crate) wait_timeout_total: u64,
}

#[derive(Debug)]
struct RuntimeSnapshotStore {
    revision: u64,
    state: ClientViewState,
    publish_attempt_total: u64,
    changed_total: u64,
    read_total: u64,
    wait_total: u64,
    wait_timeout_total: u64,
}

#[derive(Debug)]
pub(crate) struct RuntimeSnapshotHub {
    store: Mutex<RuntimeSnapshotStore>,
    changed: Condvar,
}

impl RuntimeSnapshotHub {
    fn new(state: ClientViewState) -> Self {
        Self {
            store: Mutex::new(RuntimeSnapshotStore {
                revision: 1,
                state,
                publish_attempt_total: 0,
                changed_total: 0,
                read_total: 0,
                wait_total: 0,
                wait_timeout_total: 0,
            }),
            changed: Condvar::new(),
        }
    }

    fn publish(&self, state: ClientViewState) -> RuntimeSnapshot {
        let mut store = self
            .store
            .lock()
            .expect("runtime snapshot hub mutex poisoned");
        store.publish_attempt_total = store.publish_attempt_total.saturating_add(1);
        if store.state != state {
            store.revision = store.revision.saturating_add(1);
            store.changed_total = store.changed_total.saturating_add(1);
            store.state = state;
            self.changed.notify_all();
        }
        RuntimeSnapshot {
            revision: store.revision,
            state: store.state.clone(),
        }
    }

    pub(crate) fn latest(&self) -> RuntimeSnapshot {
        let mut store = self
            .store
            .lock()
            .expect("runtime snapshot hub mutex poisoned");
        store.read_total = store.read_total.saturating_add(1);
        RuntimeSnapshot {
            revision: store.revision,
            state: store.state.clone(),
        }
    }

    pub(crate) fn wait_after(&self, last_revision: u64, timeout: Duration) -> RuntimeSnapshot {
        let store = self
            .store
            .lock()
            .expect("runtime snapshot hub mutex poisoned");
        let (mut store, wait_timed_out) = if store.revision <= last_revision {
            let (store, wait_result) = self
                .changed
                .wait_timeout_while(store, timeout, |current| current.revision <= last_revision)
                .expect("runtime snapshot hub condvar poisoned");
            (store, wait_result.timed_out())
        } else {
            (store, false)
        };
        store.read_total = store.read_total.saturating_add(1);
        store.wait_total = store.wait_total.saturating_add(1);
        if wait_timed_out && store.revision <= last_revision {
            store.wait_timeout_total = store.wait_timeout_total.saturating_add(1);
        }
        RuntimeSnapshot {
            revision: store.revision,
            state: store.state.clone(),
        }
    }

    pub(crate) fn diagnostics(&self) -> RuntimeSnapshotHubDiagnostics {
        let store = self
            .store
            .lock()
            .expect("runtime snapshot hub mutex poisoned");
        RuntimeSnapshotHubDiagnostics {
            revision: store.revision,
            publish_attempt_total: store.publish_attempt_total,
            changed_total: store.changed_total,
            read_total: store.read_total,
            wait_total: store.wait_total,
            wait_timeout_total: store.wait_timeout_total,
        }
    }
}

struct RuntimeActorInner {
    commands: SyncSender<RuntimeJob>,
    snapshots: Arc<RuntimeSnapshotHub>,
    events: Arc<RuntimeEventHub>,
    metrics: Arc<RuntimeActorMetrics>,
}

#[derive(Clone)]
pub(crate) struct RuntimeActorHandle {
    inner: Arc<RuntimeActorInner>,
}

impl RuntimeActorHandle {
    pub(crate) fn spawn(runtime: PlatformRuntime) -> Self {
        Self::spawn_with_capacity(runtime, DEFAULT_COMMAND_CAPACITY)
    }

    fn spawn_with_capacity(runtime: PlatformRuntime, capacity: usize) -> Self {
        let initial_state = runtime.state().clone();
        let capacity = capacity.max(1);
        let (commands, receiver) = mpsc::sync_channel(capacity);
        let snapshots = Arc::new(RuntimeSnapshotHub::new(initial_state));
        let events = Arc::new(RuntimeEventHub::default());
        let metrics = Arc::new(RuntimeActorMetrics::new(capacity));
        let handle = Self {
            inner: Arc::new(RuntimeActorInner {
                commands,
                snapshots,
                events,
                metrics,
            }),
        };
        spawn_actor_thread(runtime, receiver);
        handle
    }

    #[cfg(test)]
    pub(crate) fn call<R, F>(&self, operation: F) -> Result<R>
    where
        R: Send + 'static,
        F: FnOnce(&mut PlatformRuntime) -> Result<R> + Send + 'static,
    {
        self.call_named_timeout("runtime.call", None, DEFAULT_COMMAND_TIMEOUT, operation)
    }

    #[cfg(test)]
    pub(crate) fn call_timeout<R, F>(&self, timeout: Duration, operation: F) -> Result<R>
    where
        R: Send + 'static,
        F: FnOnce(&mut PlatformRuntime) -> Result<R> + Send + 'static,
    {
        self.call_named_timeout("runtime.call", None, timeout, operation)
    }

    pub(crate) fn call_named<R, F>(
        &self,
        kind: impl Into<String>,
        correlation_id: Option<String>,
        operation: F,
    ) -> Result<R>
    where
        R: Send + 'static,
        F: FnOnce(&mut PlatformRuntime) -> Result<R> + Send + 'static,
    {
        self.call_named_timeout(kind, correlation_id, DEFAULT_COMMAND_TIMEOUT, operation)
    }

    pub(crate) fn call_named_if_revision<R, F>(
        &self,
        kind: impl Into<String>,
        correlation_id: Option<String>,
        expected_revision: u64,
        operation: F,
    ) -> Result<Option<R>>
    where
        R: Send + 'static,
        F: FnOnce(&mut PlatformRuntime) -> Result<R> + Send + 'static,
    {
        let snapshots = Arc::clone(&self.inner.snapshots);
        self.call_named(kind, correlation_id, move |runtime| {
            if snapshots.latest().revision != expected_revision {
                return Ok(None);
            }
            operation(runtime).map(Some)
        })
    }

    pub(crate) fn call_named_timeout<R, F>(
        &self,
        kind: impl Into<String>,
        correlation_id: Option<String>,
        timeout: Duration,
        operation: F,
    ) -> Result<R>
    where
        R: Send + 'static,
        F: FnOnce(&mut PlatformRuntime) -> Result<R> + Send + 'static,
    {
        let kind = kind.into();
        let command_id = self.inner.metrics.next_command_id();
        let queued_at_ms = now_ms();
        let (reply, response) = mpsc::sync_channel(1);
        let snapshots = Arc::clone(&self.inner.snapshots);
        let metrics = Arc::clone(&self.inner.metrics);
        let command_state = Arc::new(AtomicU8::new(COMMAND_QUEUED));
        let caller_timed_out = Arc::new(AtomicBool::new(false));
        let job_state = Arc::clone(&command_state);
        let job_caller_timed_out = Arc::clone(&caller_timed_out);
        let job_kind = kind.clone();
        let job_correlation_id = correlation_id.clone();
        let job = Box::new(move |runtime: &mut PlatformRuntime| {
            metrics.queue_depth.fetch_sub(1, Ordering::Relaxed);
            if job_state
                .compare_exchange(
                    COMMAND_QUEUED,
                    COMMAND_RUNNING,
                    Ordering::AcqRel,
                    Ordering::Acquire,
                )
                .is_err()
            {
                metrics
                    .cancelled_before_start_total
                    .fetch_add(1, Ordering::Relaxed);
                metrics.record_command(RuntimeCommandDiagnostic {
                    command_id,
                    kind: job_kind,
                    correlation_id: job_correlation_id,
                    queued_at_ms,
                    started_at_ms: None,
                    finished_at_ms: now_ms(),
                    duration_ms: None,
                    outcome: "cancelled".to_string(),
                    caller_timed_out: job_caller_timed_out.load(Ordering::Acquire),
                });
                return;
            }
            metrics.running.store(true, Ordering::Release);
            let started_at_ms = now_ms();
            metrics.set_active_command(Some(RuntimeActiveCommandDiagnostic {
                command_id,
                kind: job_kind.clone(),
                correlation_id: job_correlation_id.clone(),
                queued_at_ms,
                started_at_ms,
                caller_timed_out: false,
            }));
            let result = catch_unwind(AssertUnwindSafe(|| operation(runtime)))
                .unwrap_or_else(|_| Err(anyhow!("runtime command panicked")));
            snapshots.publish(runtime.state().clone());
            metrics.running.store(false, Ordering::Release);
            metrics.set_active_command(None);
            metrics.completed_total.fetch_add(1, Ordering::Relaxed);
            if result.is_err() {
                metrics.failed_total.fetch_add(1, Ordering::Relaxed);
            }
            let finished_at_ms = now_ms();
            metrics.record_command(RuntimeCommandDiagnostic {
                command_id,
                kind: job_kind,
                correlation_id: job_correlation_id,
                queued_at_ms,
                started_at_ms: Some(started_at_ms),
                finished_at_ms,
                duration_ms: Some(finished_at_ms.saturating_sub(started_at_ms)),
                outcome: if result.is_ok() {
                    "succeeded"
                } else {
                    "failed"
                }
                .to_string(),
                caller_timed_out: job_caller_timed_out.load(Ordering::Acquire),
            });
            job_state.store(COMMAND_COMPLETED, Ordering::Release);
            let _ = reply.send(result);
        });
        self.inner
            .metrics
            .queue_depth
            .fetch_add(1, Ordering::Relaxed);
        match self.inner.commands.try_send(job) {
            Ok(()) => {
                self.inner
                    .metrics
                    .accepted_total
                    .fetch_add(1, Ordering::Relaxed);
            }
            Err(TrySendError::Full(_)) => {
                self.inner
                    .metrics
                    .queue_depth
                    .fetch_sub(1, Ordering::Relaxed);
                self.inner
                    .metrics
                    .rejected_total
                    .fetch_add(1, Ordering::Relaxed);
                self.inner.metrics.record_command(RuntimeCommandDiagnostic {
                    command_id,
                    kind,
                    correlation_id,
                    queued_at_ms,
                    started_at_ms: None,
                    finished_at_ms: now_ms(),
                    duration_ms: None,
                    outcome: "rejected".to_string(),
                    caller_timed_out: false,
                });
                return Err(anyhow!("runtime command queue is full"));
            }
            Err(TrySendError::Disconnected(_)) => {
                self.inner
                    .metrics
                    .queue_depth
                    .fetch_sub(1, Ordering::Relaxed);
                self.inner
                    .metrics
                    .rejected_total
                    .fetch_add(1, Ordering::Relaxed);
                self.inner.metrics.record_command(RuntimeCommandDiagnostic {
                    command_id,
                    kind,
                    correlation_id,
                    queued_at_ms,
                    started_at_ms: None,
                    finished_at_ms: now_ms(),
                    duration_ms: None,
                    outcome: "rejected".to_string(),
                    caller_timed_out: false,
                });
                return Err(anyhow!("runtime actor is not running"));
            }
        }
        match response.recv_timeout(timeout) {
            Ok(result) => result,
            Err(mpsc::RecvTimeoutError::Timeout) => {
                caller_timed_out.store(true, Ordering::Release);
                self.inner.metrics.mark_active_command_timed_out(command_id);
                self.inner
                    .metrics
                    .timed_out_total
                    .fetch_add(1, Ordering::Relaxed);
                let _ = command_state.compare_exchange(
                    COMMAND_QUEUED,
                    COMMAND_CANCELLED,
                    Ordering::AcqRel,
                    Ordering::Acquire,
                );
                Err(anyhow!(
                    "runtime command timed out after {}ms",
                    timeout.as_millis()
                ))
            }
            Err(mpsc::RecvTimeoutError::Disconnected) => {
                Err(anyhow!("runtime actor stopped before command reply"))
            }
        }
    }

    pub(crate) fn snapshot(&self) -> RuntimeSnapshot {
        self.inner.snapshots.latest()
    }

    pub(crate) fn snapshots(&self) -> Arc<RuntimeSnapshotHub> {
        Arc::clone(&self.inner.snapshots)
    }

    pub(crate) fn events(&self) -> Arc<RuntimeEventHub> {
        Arc::clone(&self.inner.events)
    }

    pub(crate) fn diagnostics(&self) -> RuntimeActorDiagnostics {
        self.inner.metrics.snapshot()
    }

    pub(crate) fn wait_for_command_completion(
        &self,
        kind: &str,
        correlation_id: Option<&str>,
        not_before_ms: u64,
        timeout: Duration,
    ) -> bool {
        self.inner.metrics.command_completions.wait_for(
            kind,
            correlation_id,
            not_before_ms,
            timeout,
        )
    }

    #[allow(dead_code)]
    pub(crate) fn publish_event(
        &self,
        business_type: impl Into<String>,
        business_data: Value,
    ) -> RuntimeBusinessEvent {
        self.inner.events.publish(business_type, business_data)
    }
}

fn spawn_actor_thread(mut runtime: PlatformRuntime, receiver: Receiver<RuntimeJob>) {
    thread::Builder::new()
        .name("slan-runtime-actor".to_string())
        .spawn(move || {
            while let Ok(job) = receiver.recv() {
                job(&mut runtime);
            }
        })
        .expect("spawn runtime actor thread");
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
    use super::{now_ms, RuntimeActorHandle, RuntimeSnapshotHub};
    use client_core::{AuthPayload, ClientCommand, ClientRuntime, ClientViewState};
    use client_core_platform::PlatformNetworkImpl;
    use std::{
        sync::{
            atomic::{AtomicBool, Ordering},
            Arc,
        },
        time::Duration,
    };

    #[test]
    fn actor_serializes_runtime_writes_and_publishes_snapshot() {
        let actor = RuntimeActorHandle::spawn(ClientRuntime::new(PlatformNetworkImpl));
        actor
            .call(|runtime| {
                runtime.dispatch(ClientCommand::ApplyDeviceUserLogin(AuthPayload {
                    user_authenticated: Some(true),
                    access_token: "token".to_string(),
                    refresh_token: None,
                    user_id: "user".to_string(),
                    user_label: "user@example.com".to_string(),
                    device_id: Some("device".to_string()),
                    active_network_id: None,
                    virtual_ip: None,
                    expires_in: None,
                }))?;
                Ok(())
            })
            .expect("dispatch login through actor");

        let snapshot = actor.snapshot();
        assert_eq!(snapshot.revision, 2);
        assert!(snapshot.state.signed_in);
        assert_eq!(snapshot.state.device_id.as_deref(), Some("device"));
    }

    #[test]
    fn command_completion_wait_matches_correlation_and_wakes() {
        let actor = RuntimeActorHandle::spawn(ClientRuntime::new(PlatformNetworkImpl));
        let waiting_actor = actor.clone();
        let started_at_ms = now_ms();
        let waiter = std::thread::spawn(move || {
            waiting_actor.wait_for_command_completion(
                "test.completion",
                Some("request-completion"),
                started_at_ms,
                Duration::from_secs(1),
            )
        });

        actor
            .call_named(
                "test.completion",
                Some("request-completion".to_string()),
                |_| Ok(()),
            )
            .expect("complete correlated command");

        assert!(waiter.join().expect("join completion waiter"));
        assert!(actor.wait_for_command_completion(
            "test.completion",
            Some("request-completion"),
            started_at_ms,
            Duration::ZERO,
        ));
        assert!(!actor.wait_for_command_completion(
            "test.completion",
            Some("different-request"),
            started_at_ms,
            Duration::ZERO,
        ));
    }

    #[test]
    fn snapshot_wait_observes_actor_update() {
        let actor = RuntimeActorHandle::spawn(ClientRuntime::new(PlatformNetworkImpl));
        let snapshots = actor.snapshots();
        actor
            .call(|runtime| {
                runtime.request_browser_login(Some("device".to_string()));
                Ok(())
            })
            .expect("request browser login through actor");

        let snapshot = snapshots.wait_after(1, Duration::from_millis(10));
        assert_eq!(snapshot.revision, 2);
        assert_eq!(
            snapshot.state.notice.as_deref(),
            Some("loginBrowserRequested")
        );
    }

    #[test]
    fn snapshot_diagnostics_track_changes_reads_and_wait_timeouts() {
        let initial = ClientViewState::default();
        let snapshots = RuntimeSnapshotHub::new(initial.clone());
        snapshots.latest();
        snapshots.publish(initial);
        let mut changed = ClientViewState::default();
        changed.notice = Some("changed".to_string());
        snapshots.publish(changed);
        snapshots.wait_after(2, Duration::from_millis(1));

        let diagnostics = snapshots.diagnostics();
        assert_eq!(diagnostics.revision, 2);
        assert_eq!(diagnostics.publish_attempt_total, 2);
        assert_eq!(diagnostics.changed_total, 1);
        assert_eq!(diagnostics.read_total, 2);
        assert_eq!(diagnostics.wait_total, 1);
        assert_eq!(diagnostics.wait_timeout_total, 1);
    }

    #[test]
    fn conditional_command_rejects_stale_revision() {
        let actor = RuntimeActorHandle::spawn(ClientRuntime::new(PlatformNetworkImpl));
        let initial_revision = actor.snapshot().revision;
        actor
            .call(|runtime| {
                runtime.request_browser_login(Some("new-device".to_string()));
                Ok(())
            })
            .expect("advance runtime revision");

        let result = actor
            .call_named_if_revision(
                "network.activate.commit",
                None,
                initial_revision,
                |runtime| {
                    runtime.apply_network_enabled_state("10.0.0.2".to_string());
                    Ok(())
                },
            )
            .expect("conditional actor command");

        assert_eq!(result, None);
        let state = actor.snapshot().state;
        assert!(!state.network_enabled);
        assert_eq!(state.device_id.as_deref(), Some("new-device"));
    }

    #[test]
    fn actor_owns_business_event_hub() {
        let actor = RuntimeActorHandle::spawn(ClientRuntime::new(PlatformNetworkImpl));
        let published = actor.publish_event(
            "network.runtime.changed",
            serde_json::json!({"eventId": "event-1"}),
        );
        let consumed = actor.events().next_after(0).expect("runtime event");

        assert_eq!(consumed, published);
    }

    #[test]
    fn named_command_records_correlation_and_timing() {
        let actor = RuntimeActorHandle::spawn(ClientRuntime::new(PlatformNetworkImpl));
        actor
            .call_named(
                "network.activate",
                Some("request-1".to_string()),
                |_| Ok(()),
            )
            .expect("named actor command");

        let diagnostics = actor.diagnostics();
        assert_eq!(diagnostics.active_command, None);
        let command = diagnostics.last_command.expect("last command diagnostic");
        assert_eq!(command.command_id, 1);
        assert_eq!(command.kind, "network.activate");
        assert_eq!(command.correlation_id.as_deref(), Some("request-1"));
        assert_eq!(command.outcome, "succeeded");
        assert!(!command.caller_timed_out);
        assert!(command.started_at_ms.is_some());
        assert!(command.duration_ms.is_some());
    }

    #[test]
    fn active_named_command_is_visible_while_running() {
        let actor = RuntimeActorHandle::spawn(ClientRuntime::new(PlatformNetworkImpl));
        let worker = actor.clone();
        let running = std::thread::spawn(move || {
            worker.call_named(
                "network.deactivate",
                Some("request-running".to_string()),
                |_| {
                    std::thread::sleep(Duration::from_millis(30));
                    Ok(())
                },
            )
        });

        let mut active = None;
        for _ in 0..100 {
            active = actor.diagnostics().active_command;
            if active.is_some() {
                break;
            }
            std::thread::sleep(Duration::from_millis(1));
        }
        let active = active.expect("active command diagnostic");
        assert_eq!(active.kind, "network.deactivate");
        assert_eq!(active.correlation_id.as_deref(), Some("request-running"));

        running.join().expect("named command thread").unwrap();
        assert_eq!(actor.diagnostics().active_command, None);
    }

    #[test]
    fn caller_timeout_does_not_cancel_actor_state_transition() {
        let actor = RuntimeActorHandle::spawn(ClientRuntime::new(PlatformNetworkImpl));
        let worker = actor.clone();
        let started = Arc::new(AtomicBool::new(false));
        let worker_started = Arc::clone(&started);
        let caller = std::thread::spawn(move || {
            worker.call_timeout(Duration::from_millis(20), move |runtime| {
                worker_started.store(true, Ordering::Release);
                std::thread::sleep(Duration::from_millis(60));
                runtime.request_browser_login(Some("device".to_string()));
                Ok(())
            })
        });
        while !started.load(Ordering::Acquire) {
            std::thread::sleep(Duration::from_millis(1));
        }

        let result = caller.join().expect("timed out caller thread");
        assert!(result.is_err());
        let active = actor
            .diagnostics()
            .active_command
            .expect("timed out command remains active");
        assert!(active.caller_timed_out);
        std::thread::sleep(Duration::from_millis(50));
        assert_eq!(
            actor.snapshot().state.notice.as_deref(),
            Some("loginBrowserRequested")
        );
        let diagnostics = actor.diagnostics();
        assert_eq!(diagnostics.timed_out_total, 1);
        assert_eq!(diagnostics.cancelled_before_start_total, 0);
        assert!(
            diagnostics
                .last_command
                .expect("completed timed out command")
                .caller_timed_out
        );
    }

    #[test]
    fn queued_command_is_cancelled_when_caller_times_out() {
        let actor = RuntimeActorHandle::spawn(ClientRuntime::new(PlatformNetworkImpl));
        let blocker = actor.clone();
        let running = std::thread::spawn(move || {
            blocker.call(|_| {
                std::thread::sleep(Duration::from_millis(30));
                Ok(())
            })
        });
        for _ in 0..100 {
            if actor.diagnostics().running {
                break;
            }
            std::thread::sleep(Duration::from_millis(1));
        }

        let result = actor.call_timeout(Duration::from_millis(1), |runtime| {
            runtime.request_browser_login(Some("must-not-run".to_string()));
            Ok(())
        });
        assert!(result.is_err());
        running.join().expect("blocking command thread").unwrap();
        std::thread::sleep(Duration::from_millis(5));

        assert_eq!(actor.snapshot().state.notice, None);
        let diagnostics = actor.diagnostics();
        assert_eq!(diagnostics.cancelled_before_start_total, 1);
        let command = diagnostics
            .last_command
            .expect("cancelled command diagnostic");
        assert_eq!(command.outcome, "cancelled");
        assert!(command.caller_timed_out);
    }

    #[test]
    fn command_panic_does_not_stop_actor() {
        let actor = RuntimeActorHandle::spawn(ClientRuntime::new(PlatformNetworkImpl));
        let failed = actor.call::<(), _>(|_| panic!("test panic"));
        assert!(failed
            .expect_err("panic must become command error")
            .to_string()
            .contains("panicked"));

        actor
            .call(|runtime| {
                runtime.request_browser_login(Some("device".to_string()));
                Ok(())
            })
            .expect("actor remains available");
        assert_eq!(actor.diagnostics().failed_total, 1);
    }
}
