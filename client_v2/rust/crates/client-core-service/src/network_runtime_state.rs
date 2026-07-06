use std::collections::{BTreeMap, VecDeque};

use crate::network_event::{
    NetworkEventAclRuleView, NetworkEventDeviceGroupView, NetworkEventDnsRecordView,
    NetworkEventMemberView, NetworkEventNetworkView, NetworkEventPeerPathView,
};

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

    pub members_by_device_id: BTreeMap<String, NetworkEventMemberView>,
    pub groups_by_group_id: BTreeMap<String, NetworkEventDeviceGroupView>,
    pub dns_by_record_id: BTreeMap<String, NetworkEventDnsRecordView>,
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
