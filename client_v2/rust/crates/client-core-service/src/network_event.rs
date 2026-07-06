use serde::{Deserialize, Serialize};
use serde_json::{json, Value};

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum NetworkEventType {
    NetworkSnapshot,
    MemberAdded,
    MemberRemoved,
    MemberUpdated,
    MemberOnline,
    MemberOffline,
    DeviceGroupAdded,
    DeviceGroupRemoved,
    DeviceGroupUpdated,
    AclChanged,
    DnsChanged,
    NetworkConfigChanged,
    PeerPathChanged,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct NetworkEventNetworkView {
    pub network_id: String,
    pub name: String,
    pub tags: Vec<String>,
    pub default_acl_policy: String,
    pub updated_at: u64,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct NetworkEventMemberView {
    pub device_id: String,
    pub device_name: String,
    pub virtual_ip: String,
    pub online: bool,
    pub last_seen_at: u64,
    pub tags: Vec<String>,
    pub group_ids: Vec<String>,
    pub device_version: String,
    pub platform: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct NetworkEventDeviceGroupView {
    pub group_id: String,
    pub name: String,
    pub tags: Vec<String>,
    pub member_device_ids: Vec<String>,
    pub updated_at: u64,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct NetworkEventDnsRecordView {
    pub record_id: String,
    pub zone_id: String,
    pub name: String,
    pub fqdn: String,
    pub target_device_id: String,
    pub target_ip: String,
    pub enabled: bool,
    pub updated_at: u64,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct NetworkEventAclRuleView {
    pub rule_id: String,
    pub priority: i32,
    pub action: String,
    pub direction: String,
    pub protocol: String,
    pub port_ranges: Vec<String>,
    pub source_type: String,
    pub source_device_ids: Vec<String>,
    pub source_group_ids: Vec<String>,
    pub target_type: String,
    pub target_device_ids: Vec<String>,
    pub target_group_ids: Vec<String>,
    pub enabled: bool,
    pub updated_at: u64,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct NetworkEventPeerPathView {
    pub peer_device_id: String,
    pub path_type: String,
    pub relay_region_id: String,
    pub reachable: bool,
    pub updated_at: u64,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct NetworkSnapshotPayload {
    pub network: NetworkEventNetworkView,
    pub members: Vec<NetworkEventMemberView>,
    pub device_groups: Vec<NetworkEventDeviceGroupView>,
    pub dns_records: Vec<NetworkEventDnsRecordView>,
    pub acl_rules: Vec<NetworkEventAclRuleView>,
    pub peer_paths: Vec<NetworkEventPeerPathView>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct NetworkSnapshotResponse {
    pub network_id: String,
    pub version: u64,
    pub snapshot: NetworkSnapshotPayload,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct NetworkEventEnvelope {
    pub r#type: String,
    pub network_id: String,
    pub version: u64,
    pub event_id: String,
    pub event_type: NetworkEventType,
    pub occurred_at: u64,
    pub payload: serde_json::Value,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct NetworkEventMemberPayload {
    pub member: NetworkEventMemberView,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct NetworkEventMemberRemovedPayload {
    pub device_id: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct NetworkEventPresencePayload {
    pub device_id: String,
    pub online: bool,
    pub last_seen_at: u64,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct NetworkEventDeviceGroupPayload {
    pub group: NetworkEventDeviceGroupView,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct NetworkEventDeviceGroupRemovedPayload {
    pub group_id: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct NetworkEventAclChangedPayload {
    pub rules: Vec<NetworkEventAclRuleView>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct NetworkEventDnsChangedPayload {
    pub records: Vec<NetworkEventDnsRecordView>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct NetworkEventConfigChangedPayload {
    pub network: NetworkEventNetworkView,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct NetworkEventPeerPathChangedPayload {
    pub paths: Vec<NetworkEventPeerPathView>,
}

pub(crate) fn network_event_business_data(
    network_id: &str,
    event_type: &NetworkEventType,
    config_version: u64,
    reconfigure_required: bool,
    sync_mode: &str,
) -> Value {
    json!({
        "messageType": "network_event",
        "networkId": network_id,
        "eventType": format!("{:?}", event_type),
        "configVersion": config_version,
        "reconfigureRequired": reconfigure_required,
        "syncMode": sync_mode,
    })
}
