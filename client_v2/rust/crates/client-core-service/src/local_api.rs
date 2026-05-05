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

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub(crate) enum LocalServiceMethod {
    WatchState,
    WatchBusinessEvent,
    State,
    LocalStatus,
    LocalPeers,
    LocalPathPlan,
    Start,
    Refresh,
    Dispatch,
    ShutdownNetwork,
    ActivateNetwork,
    DeactivateNetwork,
    Logout,
    EnqueueControlTask,
    EnqueueDownstreamControlTask,
    IngestDownstreamControlMessage,
    PendingControlAcks,
    MarkControlAcked,
    ControlTransportOutbox,
    MarkTransportPublished,
    ControlTransportStatus,
    ControlTransportPlan,
    ControlTransportCadence,
    ControlTransportTickPlan,
    RelayCandidates,
    RefreshRelayCandidates,
    PrepareRelayDataPlane,
    PathDiagnose,
    ExportDiagnostics,
    ConsoleLoginKey,
    AndroidNetworkConfig,
    Other,
}

impl LocalServiceMethod {
    pub(crate) fn parse(method: &str) -> Self {
        match method {
            "watchState" => Self::WatchState,
            "watchBusinessEvent" => Self::WatchBusinessEvent,
            "state" => Self::State,
            "localStatus" => Self::LocalStatus,
            "localPeers" => Self::LocalPeers,
            "localPathPlan" => Self::LocalPathPlan,
            "start" => Self::Start,
            "refresh" => Self::Refresh,
            "dispatch" => Self::Dispatch,
            "shutdownNetwork" => Self::ShutdownNetwork,
            "activateNetwork" => Self::ActivateNetwork,
            "deactivateNetwork" => Self::DeactivateNetwork,
            "logout" => Self::Logout,
            "enqueueControlTask" => Self::EnqueueControlTask,
            "enqueueDownstreamControlTask" => Self::EnqueueDownstreamControlTask,
            "ingestDownstreamControlMessage" => Self::IngestDownstreamControlMessage,
            "pendingControlAcks" => Self::PendingControlAcks,
            "markControlAcked" => Self::MarkControlAcked,
            "controlTransportOutbox" => Self::ControlTransportOutbox,
            "markTransportPublished" => Self::MarkTransportPublished,
            "controlTransportStatus" => Self::ControlTransportStatus,
            "controlTransportPlan" => Self::ControlTransportPlan,
            "controlTransportCadence" => Self::ControlTransportCadence,
            "controlTransportTickPlan" => Self::ControlTransportTickPlan,
            "relayCandidates" => Self::RelayCandidates,
            "refreshRelayCandidates" => Self::RefreshRelayCandidates,
            "prepareRelayDataPlane" => Self::PrepareRelayDataPlane,
            "pathDiagnose" => Self::PathDiagnose,
            "exportDiagnostics" => Self::ExportDiagnostics,
            "consoleLoginKey" => Self::ConsoleLoginKey,
            "androidNetworkConfig" => Self::AndroidNetworkConfig,
            _ => Self::Other,
        }
    }

    pub(crate) fn should_notify(self) -> bool {
        matches!(
            self,
            Self::Start
                | Self::Dispatch
                | Self::Refresh
                | Self::ShutdownNetwork
                | Self::ActivateNetwork
                | Self::DeactivateNetwork
                | Self::Logout
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
            LocalServiceMethod::parse("localStatus"),
            LocalServiceMethod::LocalStatus
        );
        assert_eq!(
            LocalServiceMethod::parse("localPeers"),
            LocalServiceMethod::LocalPeers
        );
        assert_eq!(
            LocalServiceMethod::parse("localPathPlan"),
            LocalServiceMethod::LocalPathPlan
        );
    }
}
