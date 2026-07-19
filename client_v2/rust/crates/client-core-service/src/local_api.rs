use client_core::ClientViewState;
use serde::{Deserialize, Serialize};
use serde_json::Value;

pub(crate) const BUSINESS_SESSION_CHANGED: &str = "session.changed";
pub(crate) const BUSINESS_NETWORK_SWITCH_FINISHED: &str = "network.switch.finished";
pub(crate) const BUSINESS_NETWORK_SWITCH_FAILED: &str = "network.switch.failed";
pub(crate) const BUSINESS_NETWORK_RUNTIME_CHANGED: &str = "network.runtime.changed";
pub(crate) const BUSINESS_CONTROL_SYNC_CHANGED: &str = "control.sync.changed";
pub(crate) const BUSINESS_STATE_CHANGED: &str = "state.changed";

#[derive(Debug, Deserialize)]
pub(crate) struct ServiceRequest {
    pub(crate) method: String,
    #[serde(default)]
    pub(crate) args: Value,
}

pub(crate) fn request_correlation_id(args: &Value) -> Option<String> {
    ["requestId", "messageId", "eventId", "deliveryId"]
        .into_iter()
        .find_map(|key| {
            args.get(key)
                .and_then(Value::as_str)
                .map(str::trim)
                .filter(|value| !value.is_empty())
                .map(str::to_string)
        })
}

pub(crate) fn response_with_correlation_id(response: &str, correlation_id: Option<&str>) -> String {
    let Some(correlation_id) = correlation_id
        .map(str::trim)
        .filter(|value| !value.is_empty())
    else {
        return response.to_string();
    };
    let Ok(mut value) = serde_json::from_str::<Value>(response) else {
        return response.to_string();
    };
    let Value::Object(data) = &mut value else {
        return response.to_string();
    };
    data.entry("requestId".to_string())
        .or_insert_with(|| Value::String(correlation_id.to_string()));
    serde_json::to_string(&value).unwrap_or_else(|_| response.to_string())
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub(crate) enum LocalServiceMethod {
    LocalState,
    LocalStateWatch,
    LocalBusinessEventWatch,
    LocalStatus,
    LocalSession,
    LocalNetworkModule,
    LocalResolverState,
    LocalResolverResolve,
    LocalPeers,
    LocalPathPlan,
    LocalPathDiagnose,
    LocalRelayCandidates,
    LocalRefreshRelayCandidates,
    LocalSetRelayTransportAllowlist,
    LocalRelayPrepare,
    LocalControlStatus,
    LocalEnsureDevice,
    LocalConnectControlMqtt,
    LocalSendClientMessage,
    LocalRegisterTestUser,
    LocalControlPlan,
    LocalControlCadence,
    LocalControlTickPlan,
    LocalControlOutbox,
    LocalPendingControlAcks,
    LocalMarkControlAcked,
    LocalMarkTransportPublished,
    LocalNetworkActivate,
    LocalNetworkDeactivate,
    LocalNetworkShutdown,
    LocalPlatformNetworkConfig,
    IngestPlatformRuntimeState,
    LocalDiagnosticsExport,
    LocalLogout,
    Start,
    Refresh,
    Dispatch,
    EnqueueControlTask,
    EnqueueDownstreamControlTask,
    IngestDownstreamControlMessage,
    ConsoleLoginKey,
    Other,
}

impl LocalServiceMethod {
    pub(crate) fn parse(method: &str) -> Self {
        match method {
            "localState" => Self::LocalState,
            "localStateWatch" => Self::LocalStateWatch,
            "localBusinessEventWatch" => Self::LocalBusinessEventWatch,
            "localStatus" => Self::LocalStatus,
            "localSession" => Self::LocalSession,
            "localNetworkModule" => Self::LocalNetworkModule,
            "localResolverState" => Self::LocalResolverState,
            "localResolverResolve" => Self::LocalResolverResolve,
            "localPeers" => Self::LocalPeers,
            "localPathPlan" => Self::LocalPathPlan,
            "localPathDiagnose" => Self::LocalPathDiagnose,
            "localRelayCandidates" => Self::LocalRelayCandidates,
            "localRefreshRelayCandidates" => Self::LocalRefreshRelayCandidates,
            "localSetRelayTransportAllowlist" => Self::LocalSetRelayTransportAllowlist,
            "localRelayPrepare" => Self::LocalRelayPrepare,
            "localControlStatus" => Self::LocalControlStatus,
            "localEnsureDevice" => Self::LocalEnsureDevice,
            "localConnectControlMqtt" => Self::LocalConnectControlMqtt,
            "localSendClientMessage" => Self::LocalSendClientMessage,
            "localRegisterTestUser" => Self::LocalRegisterTestUser,
            "localControlPlan" => Self::LocalControlPlan,
            "localControlCadence" => Self::LocalControlCadence,
            "localControlTickPlan" => Self::LocalControlTickPlan,
            "localControlOutbox" => Self::LocalControlOutbox,
            "localPendingControlAcks" => Self::LocalPendingControlAcks,
            "localMarkControlAcked" => Self::LocalMarkControlAcked,
            "localMarkTransportPublished" => Self::LocalMarkTransportPublished,
            "localNetworkActivate" => Self::LocalNetworkActivate,
            "localNetworkDeactivate" => Self::LocalNetworkDeactivate,
            "localNetworkShutdown" => Self::LocalNetworkShutdown,
            "localPlatformNetworkConfig" => Self::LocalPlatformNetworkConfig,
            "ingestPlatformRuntimeState" => Self::IngestPlatformRuntimeState,
            "localDiagnosticsExport" => Self::LocalDiagnosticsExport,
            "localLogout" => Self::LocalLogout,
            "start" => Self::Start,
            "refresh" => Self::Refresh,
            "dispatch" => Self::Dispatch,
            "enqueueControlTask" => Self::EnqueueControlTask,
            "enqueueDownstreamControlTask" => Self::EnqueueDownstreamControlTask,
            "ingestDownstreamControlMessage" => Self::IngestDownstreamControlMessage,
            "consoleLoginKey" => Self::ConsoleLoginKey,
            _ => Self::Other,
        }
    }

    pub(crate) fn should_notify(self) -> bool {
        matches!(
            self,
            Self::Start
                | Self::Dispatch
                | Self::Refresh
                | Self::LocalNetworkShutdown
                | Self::LocalNetworkActivate
                | Self::LocalNetworkDeactivate
                | Self::LocalLogout
        )
    }

    pub(crate) fn control_sync_event_method(self) -> Option<&'static str> {
        match self {
            Self::EnqueueControlTask => Some("enqueueControlTask"),
            Self::EnqueueDownstreamControlTask => Some("enqueueDownstreamControlTask"),
            Self::IngestDownstreamControlMessage => Some("ingestDownstreamControlMessage"),
            _ => None,
        }
    }
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct LocalStatusResponse {
    pub(crate) service: String,
    pub(crate) version: String,
    pub(crate) signed_in: bool,
    pub(crate) device_id: Option<String>,
    pub(crate) self_node_id: Option<String>,
    pub(crate) active_network_id: Option<String>,
    pub(crate) virtual_ip: Option<String>,
    pub(crate) network_enabled: bool,
    pub(crate) switch_enabled: bool,
    pub(crate) syncing: bool,
    pub(crate) sync_reason: Option<String>,
    pub(crate) active_path: Option<Value>,
    pub(crate) peer_count: usize,
    pub(crate) relay_candidate_count: usize,
    pub(crate) connect_plan_count: usize,
    pub(crate) local_request_concurrency_limit: usize,
    pub(crate) local_watch_concurrency_limit: usize,
    pub(crate) local_request_active: usize,
    pub(crate) local_watch_active: usize,
    pub(crate) local_request_accepted_total: u64,
    pub(crate) local_request_completed_total: u64,
    pub(crate) local_request_rejected_total: u64,
    pub(crate) local_watch_accepted_total: u64,
    pub(crate) local_watch_rejected_total: u64,
    pub(crate) runtime_command_queue_capacity: usize,
    pub(crate) runtime_command_queue_depth: usize,
    pub(crate) runtime_command_running: bool,
    pub(crate) runtime_command_accepted_total: u64,
    pub(crate) runtime_command_completed_total: u64,
    pub(crate) runtime_command_failed_total: u64,
    pub(crate) runtime_command_rejected_total: u64,
    pub(crate) runtime_command_timed_out_total: u64,
    pub(crate) runtime_command_cancelled_before_start_total: u64,
    pub(crate) runtime_active_command_id: Option<u64>,
    pub(crate) runtime_active_command_kind: Option<String>,
    pub(crate) runtime_active_command_correlation_id: Option<String>,
    pub(crate) runtime_active_command_queued_at_ms: Option<u64>,
    pub(crate) runtime_active_command_started_at_ms: Option<u64>,
    pub(crate) runtime_active_command_caller_timed_out: Option<bool>,
    pub(crate) runtime_last_command_id: Option<u64>,
    pub(crate) runtime_last_command_kind: Option<String>,
    pub(crate) runtime_last_command_correlation_id: Option<String>,
    pub(crate) runtime_last_command_queued_at_ms: Option<u64>,
    pub(crate) runtime_last_command_started_at_ms: Option<u64>,
    pub(crate) runtime_last_command_finished_at_ms: Option<u64>,
    pub(crate) runtime_last_command_duration_ms: Option<u64>,
    pub(crate) runtime_last_command_outcome: Option<String>,
    pub(crate) runtime_last_command_caller_timed_out: Option<bool>,
    pub(crate) runtime_snapshot_revision: u64,
    pub(crate) runtime_snapshot_publish_attempt_total: u64,
    pub(crate) runtime_snapshot_changed_total: u64,
    pub(crate) runtime_snapshot_read_total: u64,
    pub(crate) runtime_snapshot_wait_total: u64,
    pub(crate) runtime_snapshot_wait_timeout_total: u64,
    pub(crate) event_stream_id: String,
    pub(crate) event_queue_capacity: usize,
    pub(crate) event_queue_depth: usize,
    pub(crate) event_dedup_capacity: usize,
    pub(crate) event_dedup_entries: usize,
    pub(crate) event_oldest_available_revision: Option<u64>,
    pub(crate) event_latest_revision: u64,
    pub(crate) event_published_total: u64,
    pub(crate) event_duplicate_suppressed_total: u64,
    pub(crate) event_evicted_total: u64,
    pub(crate) event_read_total: u64,
    pub(crate) event_delivered_total: u64,
    pub(crate) event_wait_total: u64,
    pub(crate) event_wait_timeout_total: u64,
    pub(crate) event_replay_gap_total: u64,
    pub(crate) platform_transition_running: bool,
    pub(crate) platform_transition_queue_depth: usize,
    pub(crate) platform_transition_accepted_total: u64,
    pub(crate) platform_transition_completed_total: u64,
    pub(crate) platform_transition_failed_total: u64,
    pub(crate) platform_transition_rejected_total: u64,
    pub(crate) platform_transition_timed_out_total: u64,
    pub(crate) platform_transition_cancelled_before_start_total: u64,
    pub(crate) platform_transition_active_operation_id: Option<u64>,
    pub(crate) platform_transition_active_kind: Option<String>,
    pub(crate) platform_transition_active_correlation_id: Option<String>,
    pub(crate) platform_transition_active_caller_timed_out: bool,
    pub(crate) platform_transition_active_started_at_ms: Option<u64>,
    pub(crate) platform_transition_active_duration_ms: Option<u64>,
    pub(crate) platform_transition_stalled: bool,
    pub(crate) platform_transition_last_operation_id: Option<u64>,
    pub(crate) platform_transition_last_kind: Option<String>,
    pub(crate) platform_transition_last_correlation_id: Option<String>,
    pub(crate) platform_transition_last_caller_timed_out: bool,
    pub(crate) platform_transition_last_started_at_ms: Option<u64>,
    pub(crate) platform_transition_last_finished_at_ms: Option<u64>,
    pub(crate) platform_transition_last_duration_ms: Option<u64>,
    pub(crate) platform_transition_last_error: Option<String>,
    pub(crate) platform_transition_last_timed_out_operation_id: Option<u64>,
    pub(crate) platform_transition_last_timed_out_kind: Option<String>,
    pub(crate) platform_transition_last_timed_out_correlation_id: Option<String>,
    pub(crate) platform_transition_late_completion_total: u64,
    pub(crate) platform_transition_late_completion_buffered: usize,
    pub(crate) platform_transition_late_completion_evicted_total: u64,
    pub(crate) platform_transition_last_late_completion_operation_id: Option<u64>,
    pub(crate) platform_transition_last_late_completion_kind: Option<String>,
    pub(crate) platform_transition_last_late_completion_correlation_id: Option<String>,
    pub(crate) platform_transition_last_late_completion_succeeded: Option<bool>,
    pub(crate) platform_runtime_state_available: bool,
    pub(crate) platform_runtime_state_updated_at_ms: Option<u64>,
    pub(crate) platform_runtime_state_age_ms: Option<u64>,
    pub(crate) platform_runtime_state_last_error: Option<String>,
    pub(crate) platform_diagnostics_updated_at_ms: Option<u64>,
    pub(crate) platform_diagnostics_age_ms: Option<u64>,
    pub(crate) platform_diagnostics_last_error: Option<String>,
    pub(crate) error: Option<String>,
    pub(crate) runtime_error: Option<String>,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct LocalSessionResponse {
    pub(crate) signed_in: bool,
    pub(crate) expired: bool,
    pub(crate) user_id: Option<String>,
    pub(crate) user_label: Option<String>,
    pub(crate) device_id: Option<String>,
    pub(crate) self_node_id: Option<String>,
    pub(crate) active_network_id: Option<String>,
    pub(crate) virtual_ip: Option<String>,
    pub(crate) relay_candidate_count: usize,
    pub(crate) mqtt_configured: bool,
    pub(crate) expires_in: Option<u64>,
    pub(crate) authenticated_at_ms: Option<u64>,
    pub(crate) expires_at_ms: Option<u64>,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct LocalPeersResponse {
    pub(crate) items: Vec<LocalPeerView>,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct LocalPathPlanResponse {
    pub(crate) items: Vec<Value>,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct LocalPeerView {
    pub(crate) peer_node_id: String,
    #[serde(default)]
    pub(crate) peer_virtual_ips: Vec<String>,
    pub(crate) active_path: Option<Value>,
    #[serde(default)]
    pub(crate) candidates: Vec<Value>,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct MarkControlAckedRequest {
    pub(crate) task_id: String,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct PlatformRuntimeStateReportRequest {
    #[serde(default)]
    pub(crate) platform: Option<String>,
    #[serde(default)]
    pub(crate) runtime_state: Value,
    #[serde(default)]
    pub(crate) traffic: Option<Value>,
    #[serde(default)]
    pub(crate) error: Option<String>,
    #[serde(default)]
    pub(crate) reported_at_ms: Option<u64>,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct SendClientMessageRequest {
    pub(crate) target_device_id: String,
    pub(crate) body: String,
    #[serde(default)]
    pub(crate) metadata: Option<Value>,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct RegisterTestUserRequest {
    pub(crate) email: String,
    pub(crate) password: String,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct LocalResolverResolveRequest {
    pub(crate) requester_device_id: String,
    pub(crate) qname: String,
    #[serde(default = "default_resolver_qtype")]
    pub(crate) qtype: String,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct SetRelayTransportAllowlistRequest {
    #[serde(default)]
    pub(crate) transports: Vec<String>,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct WatchStateRequest {
    #[serde(default)]
    pub(crate) last_revision: u64,
    #[serde(default = "default_watch_timeout_ms")]
    pub(crate) timeout_ms: u64,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct WatchStateResponse {
    pub(crate) revision: u64,
    pub(crate) state: ClientViewState,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct WatchBusinessEventRequest {
    #[serde(default)]
    pub(crate) last_revision: u64,
    #[serde(default)]
    pub(crate) stream_id: Option<String>,
    #[serde(default = "default_watch_timeout_ms")]
    pub(crate) timeout_ms: u64,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct WatchBusinessEventResponse {
    pub(crate) revision: u64,
    pub(crate) stream_id: String,
    pub(crate) stream_reset: bool,
    pub(crate) oldest_available_revision: Option<u64>,
    pub(crate) latest_revision: u64,
    pub(crate) replay_gap: bool,
    pub(crate) event_id: Option<String>,
    pub(crate) business_type: String,
    pub(crate) business_data: Value,
    pub(crate) published_at_ms: Option<u64>,
    pub(crate) replayed: bool,
    pub(crate) snapshot: ClientViewState,
}

fn default_watch_timeout_ms() -> u64 {
    30_000
}

fn default_resolver_qtype() -> String {
    "A".to_string()
}

#[cfg(test)]
mod tests {
    use super::LocalServiceMethod;

    #[test]
    fn parses_local_interface_methods() {
        assert_eq!(
            LocalServiceMethod::parse("localState"),
            LocalServiceMethod::LocalState
        );
        assert_eq!(
            LocalServiceMethod::parse("localStateWatch"),
            LocalServiceMethod::LocalStateWatch
        );
        assert_eq!(
            LocalServiceMethod::parse("localBusinessEventWatch"),
            LocalServiceMethod::LocalBusinessEventWatch
        );
        assert_eq!(
            LocalServiceMethod::parse("localStatus"),
            LocalServiceMethod::LocalStatus
        );
        assert_eq!(
            LocalServiceMethod::parse("localSession"),
            LocalServiceMethod::LocalSession
        );
        assert_eq!(
            LocalServiceMethod::parse("localPeers"),
            LocalServiceMethod::LocalPeers
        );
        assert_eq!(
            LocalServiceMethod::parse("localNetworkModule"),
            LocalServiceMethod::LocalNetworkModule
        );
        assert_eq!(
            LocalServiceMethod::parse("localResolverState"),
            LocalServiceMethod::LocalResolverState
        );
        assert_eq!(
            LocalServiceMethod::parse("localResolverResolve"),
            LocalServiceMethod::LocalResolverResolve
        );
        assert_eq!(
            LocalServiceMethod::parse("localPathPlan"),
            LocalServiceMethod::LocalPathPlan
        );
        assert_eq!(
            LocalServiceMethod::parse("localPathDiagnose"),
            LocalServiceMethod::LocalPathDiagnose
        );
        assert_eq!(
            LocalServiceMethod::parse("localRelayCandidates"),
            LocalServiceMethod::LocalRelayCandidates
        );
        assert_eq!(
            LocalServiceMethod::parse("localRefreshRelayCandidates"),
            LocalServiceMethod::LocalRefreshRelayCandidates
        );
        assert_eq!(
            LocalServiceMethod::parse("localRelayPrepare"),
            LocalServiceMethod::LocalRelayPrepare
        );
        assert_eq!(
            LocalServiceMethod::parse("localControlStatus"),
            LocalServiceMethod::LocalControlStatus
        );
        assert_eq!(
            LocalServiceMethod::parse("localEnsureDevice"),
            LocalServiceMethod::LocalEnsureDevice
        );
        assert_eq!(
            LocalServiceMethod::parse("localConnectControlMqtt"),
            LocalServiceMethod::LocalConnectControlMqtt
        );
        assert_eq!(
            LocalServiceMethod::parse("localSendClientMessage"),
            LocalServiceMethod::LocalSendClientMessage
        );
        assert_eq!(
            LocalServiceMethod::parse("localControlPlan"),
            LocalServiceMethod::LocalControlPlan
        );
        assert_eq!(
            LocalServiceMethod::parse("localControlOutbox"),
            LocalServiceMethod::LocalControlOutbox
        );
        assert_eq!(
            LocalServiceMethod::parse("localMarkControlAcked"),
            LocalServiceMethod::LocalMarkControlAcked
        );
        assert_eq!(
            LocalServiceMethod::parse("localNetworkActivate"),
            LocalServiceMethod::LocalNetworkActivate
        );
        assert_eq!(
            LocalServiceMethod::parse("localPlatformNetworkConfig"),
            LocalServiceMethod::LocalPlatformNetworkConfig
        );
        assert_eq!(
            LocalServiceMethod::parse("ingestPlatformRuntimeState"),
            LocalServiceMethod::IngestPlatformRuntimeState
        );
        assert_eq!(
            LocalServiceMethod::parse("localReportDeviceRuntime"),
            LocalServiceMethod::Other
        );
        assert_eq!(
            LocalServiceMethod::parse("localDiagnosticsExport"),
            LocalServiceMethod::LocalDiagnosticsExport
        );
    }
}
