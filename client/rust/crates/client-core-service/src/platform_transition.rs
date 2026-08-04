use std::{
    any::Any,
    collections::VecDeque,
    panic::{catch_unwind, AssertUnwindSafe},
    sync::{
        atomic::{AtomicBool, AtomicU64, AtomicU8, Ordering},
        mpsc::{sync_channel, Receiver, SyncSender, TrySendError},
        Arc, Condvar, Mutex, OnceLock,
    },
    thread,
    time::{Duration, SystemTime, UNIX_EPOCH},
};

use anyhow::{Context, Result};
use client_core::{
    NetworkRuntimeState, PlatformNetwork, PlatformNetworkDiagnostics, PlatformResolverConfig,
    PlatformResolverRecord, PlatformResolverZone, RelayDataPlaneConfig, RouteSpec,
};
use client_core_platform::PlatformNetworkImpl;

static PLATFORM_OPERATION_LOCK: OnceLock<Mutex<()>> = OnceLock::new();
static PLATFORM_OPERATION_STATE: OnceLock<Mutex<PlatformTransitionDiagnostics>> = OnceLock::new();
static PLATFORM_SNAPSHOT_STATE: OnceLock<Mutex<PlatformSnapshot>> = OnceLock::new();
static PLATFORM_EXECUTOR: OnceLock<PlatformExecutor> = OnceLock::new();
static PLATFORM_LATE_COMPLETION_STATE: OnceLock<Mutex<PlatformLateCompletionStore>> =
    OnceLock::new();
static PLATFORM_LATE_COMPLETION_CHANGED: Condvar = Condvar::new();
static NEXT_PLATFORM_OPERATION_ID: AtomicU64 = AtomicU64::new(1);
const PLATFORM_TRANSITION_STALLED_AFTER_MS: u64 = 15_000;
const PLATFORM_WRITE_TIMEOUT: Duration = Duration::from_secs(30);
const PLATFORM_RUNTIME_READ_TIMEOUT: Duration = Duration::from_secs(5);
const PLATFORM_DIAGNOSTICS_TIMEOUT: Duration = Duration::from_secs(15);
const PLATFORM_OPERATION_QUEUE_CAPACITY: usize = 32;
const PLATFORM_LATE_COMPLETION_CAPACITY: usize = 32;
const OPERATION_QUEUED: u8 = 0;
const OPERATION_RUNNING: u8 = 1;
const OPERATION_CANCELLED: u8 = 2;
const OPERATION_COMPLETED: u8 = 3;

type ErasedPlatformResult = Result<Box<dyn Any + Send>>;
type ErasedPlatformOperation =
    Box<dyn FnOnce(&PlatformNetworkImpl) -> ErasedPlatformResult + Send + 'static>;

struct PlatformOperationRequest {
    operation_id: u64,
    kind: String,
    correlation_id: Option<String>,
    state: Arc<AtomicU8>,
    caller_timed_out: Arc<AtomicBool>,
    caller_timed_out_at_ms: Arc<AtomicU64>,
    operation: ErasedPlatformOperation,
    response: SyncSender<ErasedPlatformResult>,
}

struct PlatformExecutor {
    sender: SyncSender<PlatformOperationRequest>,
}

#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub(crate) struct PlatformTransitionDiagnostics {
    pub(crate) running: bool,
    pub(crate) queue_depth: usize,
    pub(crate) accepted_total: u64,
    pub(crate) completed_total: u64,
    pub(crate) failed_total: u64,
    pub(crate) rejected_total: u64,
    pub(crate) timed_out_total: u64,
    pub(crate) cancelled_before_start_total: u64,
    pub(crate) active_operation_id: Option<u64>,
    pub(crate) active_kind: Option<String>,
    pub(crate) active_correlation_id: Option<String>,
    pub(crate) active_caller_timed_out: bool,
    pub(crate) active_started_at_ms: Option<u64>,
    pub(crate) active_duration_ms: Option<u64>,
    pub(crate) stalled: bool,
    pub(crate) last_operation_id: Option<u64>,
    pub(crate) last_kind: Option<String>,
    pub(crate) last_correlation_id: Option<String>,
    pub(crate) last_caller_timed_out: bool,
    pub(crate) last_started_at_ms: Option<u64>,
    pub(crate) last_finished_at_ms: Option<u64>,
    pub(crate) last_duration_ms: Option<u64>,
    pub(crate) last_error: Option<String>,
    pub(crate) last_timed_out_operation_id: Option<u64>,
    pub(crate) last_timed_out_kind: Option<String>,
    pub(crate) last_timed_out_correlation_id: Option<String>,
    pub(crate) late_completion_total: u64,
    pub(crate) late_completion_buffered: usize,
    pub(crate) late_completion_evicted_total: u64,
    pub(crate) last_late_completion_operation_id: Option<u64>,
    pub(crate) last_late_completion_kind: Option<String>,
    pub(crate) last_late_completion_correlation_id: Option<String>,
    pub(crate) last_late_completion_succeeded: Option<bool>,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub(crate) struct PlatformLateCompletion {
    pub(crate) revision: u64,
    pub(crate) operation_id: u64,
    pub(crate) kind: String,
    pub(crate) correlation_id: Option<String>,
    pub(crate) succeeded: bool,
    pub(crate) error: Option<String>,
    pub(crate) finished_at_ms: u64,
    pub(crate) caller_timed_out_at_ms: u64,
}

#[derive(Debug, Default)]
struct PlatformLateCompletionStore {
    revision: u64,
    events: VecDeque<PlatformLateCompletion>,
    evicted_total: u64,
}

impl PlatformLateCompletionStore {
    fn push(
        &mut self,
        operation_id: u64,
        kind: String,
        correlation_id: Option<String>,
        succeeded: bool,
        error: Option<String>,
        finished_at_ms: u64,
        caller_timed_out_at_ms: u64,
    ) {
        self.revision = self.revision.saturating_add(1);
        if self.events.len() == PLATFORM_LATE_COMPLETION_CAPACITY {
            self.events.pop_front();
            self.evicted_total = self.evicted_total.saturating_add(1);
        }
        self.events.push_back(PlatformLateCompletion {
            revision: self.revision,
            operation_id,
            kind,
            correlation_id,
            succeeded,
            error,
            finished_at_ms,
            caller_timed_out_at_ms,
        });
    }

    fn next_after(&self, last_revision: u64) -> Option<PlatformLateCompletion> {
        self.events
            .iter()
            .find(|completion| completion.revision > last_revision)
            .cloned()
    }
}

#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub(crate) struct PlatformSnapshot {
    pub(crate) runtime_state: NetworkRuntimeState,
    pub(crate) runtime_state_available: bool,
    pub(crate) runtime_state_updated_at_ms: Option<u64>,
    pub(crate) runtime_state_age_ms: Option<u64>,
    pub(crate) runtime_state_last_error: Option<String>,
    pub(crate) platform_diagnostics: Option<PlatformNetworkDiagnostics>,
    pub(crate) platform_diagnostics_updated_at_ms: Option<u64>,
    pub(crate) platform_diagnostics_age_ms: Option<u64>,
    pub(crate) platform_diagnostics_last_error: Option<String>,
}

pub(crate) struct PlatformNetworkActivation<'a> {
    pub(crate) virtual_ip: &'a str,
    pub(crate) prefix_len: u8,
    pub(crate) resolver: &'a PlatformResolverConfig,
    pub(crate) resolver_zones: &'a [PlatformResolverZone],
    pub(crate) resolver_records: &'a [PlatformResolverRecord],
    pub(crate) routes: &'a [RouteSpec],
    pub(crate) relay_config: Option<&'a RelayDataPlaneConfig>,
}

pub(crate) fn diagnostics() -> PlatformTransitionDiagnostics {
    let mut diagnostics = operation_state()
        .lock()
        .unwrap_or_else(|error| error.into_inner())
        .clone();
    diagnostics.active_duration_ms = diagnostics
        .active_started_at_ms
        .map(|started_at_ms| now_ms().saturating_sub(started_at_ms));
    diagnostics.stalled = diagnostics
        .active_duration_ms
        .is_some_and(|duration_ms| duration_ms >= PLATFORM_TRANSITION_STALLED_AFTER_MS);
    let late_completions = late_completion_state()
        .lock()
        .unwrap_or_else(|error| error.into_inner());
    diagnostics.late_completion_buffered = late_completions.events.len();
    diagnostics.late_completion_evicted_total = late_completions.evicted_total;
    diagnostics
}

pub(crate) fn snapshot() -> PlatformSnapshot {
    let mut snapshot = snapshot_state()
        .lock()
        .unwrap_or_else(|error| error.into_inner())
        .clone();
    let now = now_ms();
    snapshot.runtime_state_age_ms = snapshot
        .runtime_state_updated_at_ms
        .map(|updated_at_ms| now.saturating_sub(updated_at_ms));
    snapshot.platform_diagnostics_age_ms = snapshot
        .platform_diagnostics_updated_at_ms
        .map(|updated_at_ms| now.saturating_sub(updated_at_ms));
    snapshot
}

pub(crate) fn wait_late_completion_after(
    last_revision: u64,
    timeout: Duration,
) -> Option<PlatformLateCompletion> {
    let store = late_completion_state()
        .lock()
        .unwrap_or_else(|error| error.into_inner());
    let store = if store.revision <= last_revision {
        PLATFORM_LATE_COMPLETION_CHANGED
            .wait_timeout_while(store, timeout, |current| current.revision <= last_revision)
            .unwrap_or_else(|error| error.into_inner())
            .0
    } else {
        store
    };
    store.next_after(last_revision)
}

#[cfg(test)]
fn late_completion_revision() -> u64 {
    late_completion_state()
        .lock()
        .unwrap_or_else(|error| error.into_inner())
        .revision
}

pub(crate) fn refresh_runtime_state() -> Result<NetworkRuntimeState> {
    let result = run_serialized_timeout(
        "platform.runtime.read",
        None,
        PLATFORM_RUNTIME_READ_TIMEOUT,
        |platform| {
            platform
                .read_runtime_state()
                .context("read platform runtime state")
        },
    );
    record_runtime_state_result(&result);
    result
}

pub(crate) fn refresh_platform_diagnostics() -> Result<PlatformNetworkDiagnostics> {
    let result = run_serialized_timeout(
        "platform.diagnostics.read",
        None,
        PLATFORM_DIAGNOSTICS_TIMEOUT,
        |platform| platform.diagnostics().context("read platform diagnostics"),
    );
    record_platform_diagnostics_result(&result);
    result
}

pub(crate) fn run_serialized<T>(
    kind: impl Into<String>,
    operation: impl FnOnce(&PlatformNetworkImpl) -> Result<T> + Send + 'static,
) -> Result<T>
where
    T: Send + 'static,
{
    run_serialized_correlated(kind, None, operation)
}

pub(crate) fn run_serialized_correlated<T>(
    kind: impl Into<String>,
    correlation_id: Option<String>,
    operation: impl FnOnce(&PlatformNetworkImpl) -> Result<T> + Send + 'static,
) -> Result<T>
where
    T: Send + 'static,
{
    run_serialized_timeout(kind, correlation_id, PLATFORM_WRITE_TIMEOUT, operation)
}

fn run_serialized_timeout<T>(
    kind: impl Into<String>,
    correlation_id: Option<String>,
    timeout: Duration,
    operation: impl FnOnce(&PlatformNetworkImpl) -> Result<T> + Send + 'static,
) -> Result<T>
where
    T: Send + 'static,
{
    let kind = kind.into();
    let operation_id = next_operation_id();
    let lifecycle = Arc::new(AtomicU8::new(OPERATION_QUEUED));
    let caller_timed_out = Arc::new(AtomicBool::new(false));
    let caller_timed_out_at_ms = Arc::new(AtomicU64::new(0));
    let (response_tx, response_rx) = sync_channel(1);
    {
        let mut state = operation_state()
            .lock()
            .unwrap_or_else(|error| error.into_inner());
        state.accepted_total = state.accepted_total.saturating_add(1);
        state.queue_depth = state.queue_depth.saturating_add(1);
    }
    let request = PlatformOperationRequest {
        operation_id,
        kind: kind.clone(),
        correlation_id: correlation_id.clone(),
        state: Arc::clone(&lifecycle),
        caller_timed_out: Arc::clone(&caller_timed_out),
        caller_timed_out_at_ms: Arc::clone(&caller_timed_out_at_ms),
        operation: Box::new(move |platform| {
            operation(platform).map(|value| Box::new(value) as Box<dyn Any + Send>)
        }),
        response: response_tx,
    };
    if let Err(error) = executor().sender.try_send(request) {
        let message = match error {
            TrySendError::Full(_) => "platform operation queue is full",
            TrySendError::Disconnected(_) => "platform operation worker is unavailable",
        };
        let mut state = operation_state()
            .lock()
            .unwrap_or_else(|error| error.into_inner());
        state.queue_depth = state.queue_depth.saturating_sub(1);
        state.failed_total = state.failed_total.saturating_add(1);
        state.rejected_total = state.rejected_total.saturating_add(1);
        state.last_operation_id = Some(operation_id);
        state.last_kind = Some(kind);
        state.last_correlation_id = correlation_id;
        state.last_caller_timed_out = false;
        state.last_error = Some(message.to_string());
        anyhow::bail!(message);
    }
    let erased = match response_rx.recv_timeout(timeout) {
        Ok(result) => result?,
        Err(error) => {
            caller_timed_out.store(true, Ordering::Release);
            caller_timed_out_at_ms.store(now_ms(), Ordering::Release);
            let _ = lifecycle.compare_exchange(
                OPERATION_QUEUED,
                OPERATION_CANCELLED,
                Ordering::AcqRel,
                Ordering::Acquire,
            );
            let mut state = operation_state()
                .lock()
                .unwrap_or_else(|error| error.into_inner());
            state.timed_out_total = state.timed_out_total.saturating_add(1);
            state.last_timed_out_operation_id = Some(operation_id);
            state.last_timed_out_kind = Some(kind.clone());
            state.last_timed_out_correlation_id = correlation_id;
            if state.active_operation_id == Some(operation_id) {
                state.active_caller_timed_out = true;
            }
            anyhow::bail!("platform operation {kind} timed out: {error}");
        }
    };
    erased
        .downcast::<T>()
        .map(|value| *value)
        .map_err(|_| anyhow::anyhow!("platform operation {kind} returned an invalid result type"))
}

pub(crate) fn run_inline_serialized<T>(
    kind: impl Into<String>,
    operation: impl FnOnce(&PlatformNetworkImpl) -> Result<T>,
) -> Result<T> {
    let kind = kind.into();
    let operation_id = next_operation_id();
    {
        let mut state = operation_state()
            .lock()
            .unwrap_or_else(|error| error.into_inner());
        state.accepted_total = state.accepted_total.saturating_add(1);
        state.queue_depth = state.queue_depth.saturating_add(1);
    }
    execute_operation(
        operation_id,
        kind,
        None,
        Arc::new(AtomicBool::new(false)),
        Arc::new(AtomicU64::new(0)),
        operation,
    )
}

fn executor() -> &'static PlatformExecutor {
    PLATFORM_EXECUTOR.get_or_init(|| {
        let (sender, receiver) = sync_channel(PLATFORM_OPERATION_QUEUE_CAPACITY);
        thread::Builder::new()
            .name("slan-platform-actor".to_string())
            .spawn(move || platform_worker(receiver))
            .expect("spawn platform operation worker");
        PlatformExecutor { sender }
    })
}

fn platform_worker(receiver: Receiver<PlatformOperationRequest>) {
    while let Ok(request) = receiver.recv() {
        if request
            .state
            .compare_exchange(
                OPERATION_QUEUED,
                OPERATION_RUNNING,
                Ordering::AcqRel,
                Ordering::Acquire,
            )
            .is_err()
        {
            let mut state = operation_state()
                .lock()
                .unwrap_or_else(|error| error.into_inner());
            state.queue_depth = state.queue_depth.saturating_sub(1);
            state.cancelled_before_start_total =
                state.cancelled_before_start_total.saturating_add(1);
            continue;
        }
        let result = execute_operation(
            request.operation_id,
            request.kind,
            request.correlation_id,
            request.caller_timed_out,
            request.caller_timed_out_at_ms,
            request.operation,
        );
        request.state.store(OPERATION_COMPLETED, Ordering::Release);
        let _ = request.response.send(result);
    }
}

fn execute_operation<T>(
    operation_id: u64,
    kind: String,
    correlation_id: Option<String>,
    caller_timed_out: Arc<AtomicBool>,
    caller_timed_out_at_ms: Arc<AtomicU64>,
    operation: impl FnOnce(&PlatformNetworkImpl) -> Result<T>,
) -> Result<T> {
    let _guard = operation_lock()
        .lock()
        .unwrap_or_else(|error| error.into_inner());
    let started_at_ms = now_ms();
    {
        let mut state = operation_state()
            .lock()
            .unwrap_or_else(|error| error.into_inner());
        state.running = true;
        state.queue_depth = state.queue_depth.saturating_sub(1);
        state.active_operation_id = Some(operation_id);
        state.active_kind = Some(kind.clone());
        state.active_correlation_id = correlation_id.clone();
        state.active_caller_timed_out = caller_timed_out.load(Ordering::Acquire);
        state.active_started_at_ms = Some(started_at_ms);
        state.active_duration_ms = Some(0);
        state.stalled = false;
    }
    let result = catch_unwind(AssertUnwindSafe(|| operation(&PlatformNetworkImpl)))
        .unwrap_or_else(|_| Err(anyhow::anyhow!("platform transition panicked")));
    let finished_at_ms = now_ms();
    let caller_timed_out = caller_timed_out.load(Ordering::Acquire);
    let caller_timed_out_at_ms = caller_timed_out_at_ms.load(Ordering::Acquire);
    let completion_kind = kind.clone();
    let completion_correlation_id = correlation_id.clone();
    let completion_error = result.as_ref().err().map(|error| format!("{error:#}"));
    {
        let mut state = operation_state()
            .lock()
            .unwrap_or_else(|error| error.into_inner());
        state.running = false;
        state.completed_total = state.completed_total.saturating_add(1);
        if result.is_err() {
            state.failed_total = state.failed_total.saturating_add(1);
        }
        state.active_operation_id = None;
        state.active_kind = None;
        state.active_correlation_id = None;
        state.active_caller_timed_out = false;
        state.active_started_at_ms = None;
        state.active_duration_ms = None;
        state.stalled = false;
        state.last_operation_id = Some(operation_id);
        state.last_kind = Some(kind);
        state.last_correlation_id = correlation_id;
        state.last_caller_timed_out = caller_timed_out;
        state.last_started_at_ms = Some(started_at_ms);
        state.last_finished_at_ms = Some(finished_at_ms);
        state.last_duration_ms = Some(finished_at_ms.saturating_sub(started_at_ms));
        state.last_error = result.as_ref().err().map(|error| format!("{error:#}"));
        if caller_timed_out {
            state.late_completion_total = state.late_completion_total.saturating_add(1);
            state.last_late_completion_operation_id = Some(operation_id);
            state.last_late_completion_kind = Some(completion_kind.clone());
            state.last_late_completion_correlation_id = completion_correlation_id.clone();
            state.last_late_completion_succeeded = Some(result.is_ok());
        }
    }
    if caller_timed_out {
        publish_late_completion(
            operation_id,
            completion_kind,
            completion_correlation_id,
            result.is_ok(),
            completion_error,
            finished_at_ms,
            caller_timed_out_at_ms,
        );
    }
    result
}

fn next_operation_id() -> u64 {
    NEXT_PLATFORM_OPERATION_ID.fetch_add(1, Ordering::Relaxed)
}

/// Phase 1 (fast): install/enable adapter only — no IP/routes/DNS configuration.
/// Returns quickly so the UI can show "network enabled" before the heavier
/// IP/routes/DNS/relay configuration runs in phase 2 ([configure_network_full]).
pub(crate) fn activate_network(
    platform: &PlatformNetworkImpl,
    activation: PlatformNetworkActivation<'_>,
) -> Result<()> {
    // Fast path: if the adapter is already up with the correct IP,
    // skip the install step entirely (~1-2s saved).
    let cache_matches = platform.read_runtime_state().ok().is_some_and(|state| {
        state.network_enabled
            && state
                .virtual_ip
                .as_deref()
                .is_some_and(|ip| ip == activation.virtual_ip)
    });
    let adapter_verified = cache_matches
        && platform
            .verify_adapter_ip(activation.virtual_ip)
            .unwrap_or(false);
    if adapter_verified {
        crate::log_service_error(format!(
            "activate_network: fast path — adapter verified with {}",
            activation.virtual_ip
        ));
    } else {
        if cache_matches {
            crate::log_service_error(format!(
                "activate_network: cache says {} but adapter verification failed — reinstalling",
                activation.virtual_ip
            ));
        }
        // Phase 1 only: install/enable the Wintun adapter.  IP, routes, DNS
        // and relay are configured asynchronously in configure_network_full.
        platform.install_adapter()?;
    }
    platform.mark_network_enabled(activation.virtual_ip)?;
    record_network_enabled(activation.virtual_ip);
    Ok(())
}

/// Phase 2: configure IP, routes, DNS and relay after the adapter is up.
/// Called after [activate_network] + state commit so the UI already shows
/// "enabled" while the heavier configuration runs in the background.
pub(crate) fn configure_network_full(
    platform: &PlatformNetworkImpl,
    activation: PlatformNetworkActivation<'_>,
) -> Result<()> {
    platform.configure_ip(activation.virtual_ip, activation.prefix_len)?;
    platform.configure_resolver(activation.resolver)?;
    platform.configure_resolver_map(activation.resolver_zones, activation.resolver_records)?;
    platform.configure_routes(activation.routes)?;
    platform.configure_relay(activation.relay_config)?;
    Ok(())
}

pub(crate) fn replace_data_plane(
    platform: &PlatformNetworkImpl,
    relay_config: Option<&RelayDataPlaneConfig>,
) -> Result<()> {
    platform
        .configure_relay(relay_config)
        .context("replace platform data plane")
}

pub(crate) fn disable_network(platform: &PlatformNetworkImpl) -> Result<()> {
    platform
        .disable_network()
        .context("disable platform network")?;
    crate::resolver_runtime_state::clear_resolver_runtime_state();
    record_runtime_state_result(&Ok(NetworkRuntimeState::default()));
    Ok(())
}

pub(crate) fn configure_ip(
    platform: &PlatformNetworkImpl,
    virtual_ip: &str,
    prefix_len: u8,
) -> Result<()> {
    platform
        .configure_ip(virtual_ip, prefix_len)
        .context("configure assigned platform IP")?;
    let mut runtime_state = snapshot().runtime_state;
    runtime_state.adapter_present = true;
    runtime_state.virtual_ip = Some(virtual_ip.to_string());
    record_runtime_state_result(&Ok(runtime_state));
    Ok(())
}

fn operation_lock() -> &'static Mutex<()> {
    PLATFORM_OPERATION_LOCK.get_or_init(|| Mutex::new(()))
}

fn operation_state() -> &'static Mutex<PlatformTransitionDiagnostics> {
    PLATFORM_OPERATION_STATE.get_or_init(|| Mutex::new(PlatformTransitionDiagnostics::default()))
}

fn snapshot_state() -> &'static Mutex<PlatformSnapshot> {
    PLATFORM_SNAPSHOT_STATE.get_or_init(|| Mutex::new(PlatformSnapshot::default()))
}

fn late_completion_state() -> &'static Mutex<PlatformLateCompletionStore> {
    PLATFORM_LATE_COMPLETION_STATE
        .get_or_init(|| Mutex::new(PlatformLateCompletionStore::default()))
}

fn publish_late_completion(
    operation_id: u64,
    kind: String,
    correlation_id: Option<String>,
    succeeded: bool,
    error: Option<String>,
    finished_at_ms: u64,
    caller_timed_out_at_ms: u64,
) {
    let mut store = late_completion_state()
        .lock()
        .unwrap_or_else(|error| error.into_inner());
    store.push(
        operation_id,
        kind,
        correlation_id,
        succeeded,
        error,
        finished_at_ms,
        caller_timed_out_at_ms,
    );
    drop(store);
    PLATFORM_LATE_COMPLETION_CHANGED.notify_all();
}

fn record_runtime_state_result(result: &Result<NetworkRuntimeState>) {
    let mut snapshot = snapshot_state()
        .lock()
        .unwrap_or_else(|error| error.into_inner());
    match result {
        Ok(runtime_state) => {
            snapshot.runtime_state = runtime_state.clone();
            snapshot.runtime_state_available = true;
            snapshot.runtime_state_updated_at_ms = Some(now_ms());
            snapshot.runtime_state_last_error = None;
        }
        Err(error) => snapshot.runtime_state_last_error = Some(format!("{error:#}")),
    }
}

fn record_platform_diagnostics_result(result: &Result<PlatformNetworkDiagnostics>) {
    let mut snapshot = snapshot_state()
        .lock()
        .unwrap_or_else(|error| error.into_inner());
    match result {
        Ok(diagnostics) => {
            snapshot.platform_diagnostics = Some(diagnostics.clone());
            snapshot.platform_diagnostics_updated_at_ms = Some(now_ms());
            snapshot.platform_diagnostics_last_error = None;
        }
        Err(error) => snapshot.platform_diagnostics_last_error = Some(format!("{error:#}")),
    }
}

fn record_network_enabled(virtual_ip: &str) {
    let mut runtime_state = snapshot().runtime_state;
    runtime_state.adapter_present = true;
    runtime_state.network_enabled = true;
    runtime_state.virtual_ip = Some(virtual_ip.to_string());
    record_runtime_state_result(&Ok(runtime_state));
}

fn now_ms() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|duration| duration.as_millis() as u64)
        .unwrap_or_default()
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::{sync::mpsc, thread, time::Duration};

    #[test]
    fn late_completion_store_replays_in_order_and_evicts_oldest() {
        let mut store = PlatformLateCompletionStore::default();
        for operation_id in 1..=(PLATFORM_LATE_COMPLETION_CAPACITY as u64 + 2) {
            store.push(
                operation_id,
                format!("test.late.{operation_id}"),
                Some(format!("request-{operation_id}")),
                true,
                None,
                operation_id,
                operation_id,
            );
        }

        assert_eq!(store.events.len(), PLATFORM_LATE_COMPLETION_CAPACITY);
        assert_eq!(store.evicted_total, 2);
        let first = store.next_after(0).expect("oldest retained completion");
        assert_eq!(first.operation_id, 3);
        let second = store
            .next_after(first.revision)
            .expect("next retained completion");
        assert_eq!(second.operation_id, 4);
        assert_eq!(
            store
                .next_after(store.revision - 1)
                .map(|event| event.operation_id),
            Some(PLATFORM_LATE_COMPLETION_CAPACITY as u64 + 2)
        );
        assert_eq!(store.next_after(store.revision), None);
    }

    #[test]
    fn serialized_operation_records_success_and_failure() {
        let _lock = crate::test_env_lock();
        let before = diagnostics();
        run_serialized("test.success", |_| Ok(())).expect("successful platform operation");
        let success = diagnostics();
        assert!(!success.running);
        assert_eq!(success.queue_depth, 0);
        assert_eq!(success.accepted_total, before.accepted_total + 1);
        assert_eq!(success.completed_total, before.completed_total + 1);
        assert_eq!(success.last_kind.as_deref(), Some("test.success"));
        assert_eq!(success.last_error, None);

        let _ = run_serialized::<()>("test.failure", |_| anyhow::bail!("expected failure"));
        let failure = diagnostics();
        assert_eq!(failure.failed_total, success.failed_total + 1);
        assert_eq!(failure.last_kind.as_deref(), Some("test.failure"));
        assert!(failure
            .last_error
            .as_deref()
            .is_some_and(|error| error.contains("expected failure")));
    }

    #[test]
    fn panicked_operation_releases_serial_executor() {
        let _lock = crate::test_env_lock();
        let error = run_serialized::<()>("test.panic", |_| panic!("expected panic"))
            .expect_err("panicked platform operation must become an error");
        assert!(error.to_string().contains("panicked"));

        run_serialized("test.after_panic", |_| Ok(()))
            .expect("serial executor remains usable after panic");
        let state = diagnostics();
        assert!(!state.running);
        assert_eq!(state.last_kind.as_deref(), Some("test.after_panic"));
    }

    #[test]
    fn diagnostics_expose_active_operation_and_waiting_queue() {
        let _lock = crate::test_env_lock();
        let (active_tx, active_rx) = mpsc::sync_channel(1);
        let (release_tx, release_rx) = mpsc::sync_channel(1);
        let active = thread::spawn(move || {
            run_serialized("test.active", move |_| {
                active_tx.send(()).expect("announce active operation");
                release_rx.recv().expect("release active operation");
                Ok(())
            })
            .expect("active operation")
        });
        active_rx.recv().expect("wait for active operation");

        let queued =
            thread::spawn(|| run_serialized("test.queued", |_| Ok(())).expect("queued operation"));
        let mut snapshot = diagnostics();
        for _ in 0..100 {
            if snapshot.queue_depth == 1 {
                break;
            }
            thread::sleep(Duration::from_millis(1));
            snapshot = diagnostics();
        }
        assert!(snapshot.running);
        let active_operation_id = snapshot
            .active_operation_id
            .expect("active operation has an identity");
        assert_eq!(snapshot.active_kind.as_deref(), Some("test.active"));
        assert!(!snapshot.active_caller_timed_out);
        assert_eq!(snapshot.queue_depth, 1);
        assert!(snapshot.active_duration_ms.is_some());
        assert!(!snapshot.stalled);

        release_tx.send(()).expect("release platform operation");
        active.join().expect("join active operation");
        queued.join().expect("join queued operation");
        let finished = diagnostics();
        assert!(!finished.running);
        assert_eq!(finished.queue_depth, 0);
        assert!(finished
            .last_operation_id
            .is_some_and(|operation_id| operation_id > active_operation_id));
        assert_eq!(finished.last_kind.as_deref(), Some("test.queued"));
    }

    #[test]
    fn snapshot_keeps_last_successful_runtime_state_after_read_failure() {
        let _lock = crate::test_env_lock();
        let runtime_state = NetworkRuntimeState {
            adapter_present: true,
            network_enabled: true,
            virtual_ip: Some("10.0.0.7".to_string()),
            ..NetworkRuntimeState::default()
        };
        record_runtime_state_result(&Ok(runtime_state.clone()));
        record_runtime_state_result(&Err(anyhow::anyhow!("expected read failure")));

        let cached = snapshot();
        assert!(cached.runtime_state_available);
        assert_eq!(cached.runtime_state, runtime_state);
        assert!(cached.runtime_state_updated_at_ms.is_some());
        assert!(cached.runtime_state_age_ms.is_some());
        assert!(cached
            .runtime_state_last_error
            .as_deref()
            .is_some_and(|error| error.contains("expected read failure")));
    }

    #[test]
    fn caller_timeout_does_not_break_platform_worker() {
        let _lock = crate::test_env_lock();
        let before = diagnostics();
        let late_revision = late_completion_revision();
        let (release_tx, release_rx) = mpsc::sync_channel(1);
        let error = run_serialized_timeout(
            "test.timeout",
            Some("request-timeout".to_string()),
            Duration::from_millis(10),
            move |_| {
                release_rx.recv().expect("release timed out operation");
                Ok(())
            },
        )
        .expect_err("caller must stop waiting at its timeout");
        assert!(error.to_string().contains("timed out"));
        let timed_out = diagnostics();
        assert_eq!(timed_out.timed_out_total, before.timed_out_total + 1);
        assert!(timed_out.active_operation_id.is_some());
        assert_eq!(
            timed_out.active_operation_id,
            timed_out.last_timed_out_operation_id
        );
        assert_eq!(
            timed_out.last_timed_out_kind.as_deref(),
            Some("test.timeout")
        );
        assert_eq!(
            timed_out.active_correlation_id.as_deref(),
            Some("request-timeout")
        );
        assert_eq!(
            timed_out.last_timed_out_correlation_id.as_deref(),
            Some("request-timeout")
        );
        assert!(timed_out.active_caller_timed_out);

        release_tx.send(()).expect("release platform worker");
        let late_completion = wait_late_completion_after(late_revision, Duration::from_secs(1))
            .expect("timed out operation publishes late completion");
        assert_eq!(late_completion.kind, "test.timeout");
        assert_eq!(
            late_completion.correlation_id.as_deref(),
            Some("request-timeout")
        );
        assert!(late_completion.succeeded);
        assert_eq!(late_completion.error, None);
        run_serialized("test.after_timeout", |_| Ok(()))
            .expect("worker accepts operations after timed out caller releases");
        let recovered = diagnostics();
        assert_eq!(recovered.last_kind.as_deref(), Some("test.after_timeout"));
        assert!(!recovered.last_caller_timed_out);
    }

    #[test]
    fn queued_operation_is_cancelled_before_start_after_timeout() {
        let _lock = crate::test_env_lock();
        let before = diagnostics();
        let (active_tx, active_rx) = mpsc::sync_channel(1);
        let (release_tx, release_rx) = mpsc::sync_channel(1);
        let active = thread::spawn(move || {
            run_serialized("test.cancel.blocker", move |_| {
                active_tx.send(()).expect("announce blocking operation");
                release_rx.recv().expect("release blocking operation");
                Ok(())
            })
            .expect("blocking operation")
        });
        active_rx.recv().expect("wait for blocking operation");

        let executed = Arc::new(AtomicU8::new(0));
        let operation_executed = Arc::clone(&executed);
        let error = run_serialized_timeout(
            "test.cancel.queued",
            None,
            Duration::from_millis(10),
            move |_| {
                operation_executed.store(1, Ordering::Release);
                Ok(())
            },
        )
        .expect_err("queued caller must time out");
        assert!(error.to_string().contains("timed out"));

        release_tx.send(()).expect("release blocking operation");
        active.join().expect("join blocking operation");
        run_serialized("test.cancel.after", |_| Ok(())).expect("drain cancelled operation");
        assert_eq!(executed.load(Ordering::Acquire), 0);
        assert_eq!(
            diagnostics().cancelled_before_start_total,
            before.cancelled_before_start_total + 1
        );
    }
}
