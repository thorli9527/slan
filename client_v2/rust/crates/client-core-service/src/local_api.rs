use client_core::{ClientViewState, NetworkRuntimeState};
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

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub(crate) enum LocalServiceMethod {
    LocalState,
    LocalStateWatch,
    LocalBusinessEventWatch,
    LocalStatus,
    LocalSession,
    LocalNetworkModule,
    LocalPeers,
    LocalPathPlan,
    LocalPathDiagnose,
    LocalRelayCandidates,
    LocalRefreshRelayCandidates,
    LocalRelayPrepare,
    LocalControlStatus,
    LocalEnsureDevice,
    LocalConnectControlMqtt,
    LocalSendClientMessage,
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
            "localPeers" => Self::LocalPeers,
            "localPathPlan" => Self::LocalPathPlan,
            "localPathDiagnose" => Self::LocalPathDiagnose,
            "localRelayCandidates" => Self::LocalRelayCandidates,
            "localRefreshRelayCandidates" => Self::LocalRefreshRelayCandidates,
            "localRelayPrepare" => Self::LocalRelayPrepare,
            "localControlStatus" => Self::LocalControlStatus,
            "localEnsureDevice" => Self::LocalEnsureDevice,
            "localConnectControlMqtt" => Self::LocalConnectControlMqtt,
            "localSendClientMessage" => Self::LocalSendClientMessage,
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
    pub(crate) runtime_state: NetworkRuntimeState,
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

#[derive(Debug, Clone, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct StoredBusinessEvent {
    pub(crate) business_type: String,
    pub(crate) business_data: Value,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct WatchBusinessEventRequest {
    #[serde(default)]
    pub(crate) last_revision: u64,
    #[serde(default = "default_watch_timeout_ms")]
    pub(crate) timeout_ms: u64,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
pub(crate) struct WatchBusinessEventResponse {
    pub(crate) revision: u64,
    pub(crate) business_type: String,
    pub(crate) business_data: Value,
    pub(crate) snapshot: ClientViewState,
}

fn default_watch_timeout_ms() -> u64 {
    30_000
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
            LocalServiceMethod::parse("localDiagnosticsExport"),
            LocalServiceMethod::LocalDiagnosticsExport
        );
    }
}
