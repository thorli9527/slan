use std::collections::BTreeSet;

use serde::{Deserialize, Serialize};
use serde_json::{json, Value};

use crate::session_store::PersistedSession;

fn null_default<'de, D, T>(deserializer: D) -> Result<T, D::Error>
where
    D: serde::Deserializer<'de>,
    T: Deserialize<'de> + Default,
{
    Option::<T>::deserialize(deserializer).map(Option::unwrap_or_default)
}

pub(crate) fn network_event_targets_session(
    envelope: &NetworkEventEnvelope,
    session: &PersistedSession,
    runtime_active_network_id: Option<&str>,
) -> bool {
    if !session.network_ids.is_empty() {
        return session
            .network_ids
            .iter()
            .any(|network_id| network_id.trim() == envelope.network_id.trim());
    }
    let expected_network_id = runtime_active_network_id
        .map(str::trim)
        .filter(|value| !value.is_empty())
        .or_else(|| {
            session
                .active_network_id
                .as_deref()
                .map(str::trim)
                .filter(|value| !value.is_empty())
        });
    expected_network_id.is_none_or(|network_id| network_id == envelope.network_id.trim())
}

pub(crate) fn network_event_topics_for_session(session: &PersistedSession) -> Vec<String> {
    let Some(mqtt) = session.mqtt.as_ref() else {
        return Vec::new();
    };
    let mut prefix = mqtt.topic_prefix.trim_end_matches('/').to_string();
    if let Some(device_id) = session
        .device_id
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
    {
        let suffix = format!("/devices/{device_id}");
        if prefix.ends_with(&suffix) {
            prefix.truncate(prefix.len() - suffix.len());
        }
    }
    let mut network_ids = session
        .network_ids
        .iter()
        .map(|value| value.trim())
        .filter(|value| !value.is_empty())
        .map(str::to_string)
        .collect::<BTreeSet<_>>();
    if let Some(network_id) = session
        .active_network_id
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
    {
        network_ids.insert(network_id.to_string());
    }
    network_ids
        .into_iter()
        .map(|network_id| format!("{prefix}/networks/{network_id}/broadcast"))
        .collect()
}

#[derive(Debug, Clone, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub(crate) struct DeviceNetworkMembershipChangedPayload {
    pub(crate) device_id: String,
    #[serde(default)]
    pub(crate) network_ids: Vec<String>,
    #[serde(default)]
    pub(crate) changed_network_id: Option<String>,
    #[serde(default)]
    pub(crate) operation: Option<String>,
    #[serde(default)]
    pub(crate) membership_version: u64,
}

pub(crate) fn decode_device_network_membership_payload(
    mut value: Value,
) -> serde_json::Result<DeviceNetworkMembershipChangedPayload> {
    if let Some(payload) = value.as_object_mut() {
        let canonical = payload.remove("changedNetworkId");
        let legacy = payload.remove("networkId");
        if let Some(changed_network_id) = canonical.or(legacy) {
            payload.insert("changedNetworkId".to_string(), changed_network_id);
        }
    }
    serde_json::from_value(value)
}

pub(crate) fn apply_device_network_membership(
    session: &mut PersistedSession,
    event: DeviceNetworkMembershipChangedPayload,
) -> anyhow::Result<()> {
    let expected_device_id = session.device_id.as_deref().unwrap_or_default().trim();
    anyhow::ensure!(
        !expected_device_id.is_empty() && event.device_id.trim() == expected_device_id,
        "device_network_membership_changed target device mismatch"
    );
    session.network_ids = event
        .network_ids
        .into_iter()
        .map(|value| value.trim().to_string())
        .filter(|value| !value.is_empty())
        .collect::<BTreeSet<_>>()
        .into_iter()
        .collect();
    if session
        .active_network_id
        .as_ref()
        .is_none_or(|network_id| !session.network_ids.contains(network_id))
    {
        session.active_network_id = session.network_ids.first().cloned();
    }
    Ok(())
}

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
    ResolverChanged,
    NetworkConfigChanged,
    PeerPathChanged,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct NetworkEventNetworkView {
    pub network_id: String,
    pub name: String,
    #[serde(default, deserialize_with = "null_default")]
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
    #[serde(default, deserialize_with = "null_default")]
    pub tags: Vec<String>,
    #[serde(default, deserialize_with = "null_default")]
    pub group_ids: Vec<String>,
    pub device_version: String,
    pub platform: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct NetworkEventDeviceGroupView {
    pub group_id: String,
    pub name: String,
    #[serde(default, deserialize_with = "null_default")]
    pub tags: Vec<String>,
    #[serde(default, deserialize_with = "null_default")]
    pub member_device_ids: Vec<String>,
    pub updated_at: u64,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct NetworkEventResolverConfigView {
    #[serde(default, deserialize_with = "null_default")]
    pub servers: Vec<String>,
    #[serde(default, deserialize_with = "null_default")]
    pub search_domains: Vec<String>,
    #[serde(default, deserialize_with = "null_default")]
    pub split_domains: Vec<String>,
    #[serde(default)]
    pub fallback_to_system_resolvers: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct NetworkEventResolverZoneView {
    #[serde(default)]
    pub zone_id: String,
    #[serde(default)]
    pub network_id: String,
    #[serde(default)]
    pub zone_name: String,
    #[serde(default)]
    pub expose_global: bool,
    #[serde(default)]
    pub updated_at: u64,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct NetworkEventResolverRecordView {
    #[serde(default)]
    pub record_id: String,
    #[serde(default)]
    pub zone_id: String,
    #[serde(default)]
    pub network_id: String,
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub fqdn: String,
    #[serde(default)]
    pub record_type: String,
    #[serde(default)]
    pub value: String,
    #[serde(default)]
    pub target_device_id: String,
    #[serde(default)]
    pub target_ip: String,
    #[serde(default)]
    pub cname: String,
    #[serde(default)]
    pub port: i32,
    #[serde(default)]
    pub ttl: i32,
    #[serde(default)]
    pub enabled: bool,
    #[serde(default)]
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
    #[serde(default, deserialize_with = "null_default")]
    pub port_ranges: Vec<String>,
    pub source_type: String,
    #[serde(default, deserialize_with = "null_default")]
    pub source_values: Vec<String>,
    #[serde(default, deserialize_with = "null_default")]
    pub source_device_ids: Vec<String>,
    #[serde(default, deserialize_with = "null_default")]
    pub source_group_ids: Vec<String>,
    pub target_type: String,
    #[serde(default, deserialize_with = "null_default")]
    pub target_values: Vec<String>,
    #[serde(default, deserialize_with = "null_default")]
    pub target_device_ids: Vec<String>,
    #[serde(default, deserialize_with = "null_default")]
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
    #[serde(default, deserialize_with = "null_default")]
    pub members: Vec<NetworkEventMemberView>,
    #[serde(default, deserialize_with = "null_default")]
    pub device_groups: Vec<NetworkEventDeviceGroupView>,
    #[serde(default)]
    pub resolver_config: NetworkEventResolverConfigView,
    #[serde(default, deserialize_with = "null_default")]
    pub resolver_zones: Vec<NetworkEventResolverZoneView>,
    #[serde(default, deserialize_with = "null_default")]
    pub resolver_records: Vec<NetworkEventResolverRecordView>,
    #[serde(default, deserialize_with = "null_default")]
    pub acl_rules: Vec<NetworkEventAclRuleView>,
    #[serde(default, deserialize_with = "null_default")]
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
    #[serde(default, deserialize_with = "null_default")]
    pub rules: Vec<NetworkEventAclRuleView>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct NetworkEventResolverChangedPayload {
    #[serde(default)]
    pub config: NetworkEventResolverConfigView,
    #[serde(default, deserialize_with = "null_default")]
    pub zones: Vec<NetworkEventResolverZoneView>,
    #[serde(default, deserialize_with = "null_default")]
    pub records: Vec<NetworkEventResolverRecordView>,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct NetworkEventConfigChangedPayload {
    pub network: NetworkEventNetworkView,
}

#[derive(Debug, Clone, Serialize, Deserialize, Default)]
#[serde(rename_all = "camelCase")]
pub struct NetworkEventPeerPathChangedPayload {
    #[serde(default, deserialize_with = "null_default")]
    pub paths: Vec<NetworkEventPeerPathView>,
}

pub(crate) fn network_event_business_data(
    event_id: &str,
    network_id: &str,
    event_type: &NetworkEventType,
    config_version: u64,
    reconfigure_required: bool,
    sync_mode: &str,
) -> Value {
    json!({
        "eventId": event_id,
        "messageType": "network_event",
        "networkId": network_id,
        "eventType": format!("{:?}", event_type),
        "configVersion": config_version,
        "reconfigureRequired": reconfigure_required,
        "syncMode": sync_mode,
    })
}

pub(crate) fn synthetic_snapshot_event_id(scope: &str, network_id: &str, version: u64) -> String {
    format!("{scope}-snapshot-{network_id}-{version}")
}

#[cfg(test)]
mod tests {
    use super::{
        apply_device_network_membership, decode_device_network_membership_payload,
        network_event_business_data, synthetic_snapshot_event_id,
        DeviceNetworkMembershipChangedPayload, NetworkEventType,
    };
    use crate::session_store::PersistedSession;

    #[test]
    fn membership_payload_prefers_canonical_field_when_legacy_field_is_also_present() {
        let payload = decode_device_network_membership_payload(serde_json::json!({
            "deviceId": "device-1",
            "networkId": "network-legacy",
            "changedNetworkId": "network-current",
            "networkIds": ["network-current"],
            "operation": "joined"
        }))
        .expect("decode duplicate-compatible membership payload");

        assert_eq!(
            payload.changed_network_id.as_deref(),
            Some("network-current")
        );
    }

    #[test]
    fn membership_payload_accepts_legacy_network_id() {
        let payload = decode_device_network_membership_payload(serde_json::json!({
            "deviceId": "device-1",
            "networkId": "network-legacy",
            "networkIds": ["network-legacy"]
        }))
        .expect("decode legacy membership payload");

        assert_eq!(
            payload.changed_network_id.as_deref(),
            Some("network-legacy")
        );
    }

    #[test]
    fn membership_update_normalizes_networks_and_replaces_removed_active_network() {
        let mut session = PersistedSession::empty();
        session.device_id = Some("device-1".to_string());
        session.network_ids = vec!["network-old".to_string()];
        session.active_network_id = Some("network-old".to_string());

        apply_device_network_membership(
            &mut session,
            DeviceNetworkMembershipChangedPayload {
                device_id: "device-1".to_string(),
                network_ids: vec![
                    " network-b ".to_string(),
                    "network-a".to_string(),
                    "network-b".to_string(),
                    String::new(),
                ],
                changed_network_id: Some("network-b".to_string()),
                operation: Some("joined".to_string()),
                membership_version: 7,
            },
        )
        .expect("apply membership");

        assert_eq!(session.network_ids, vec!["network-a", "network-b"]);
        assert_eq!(session.active_network_id.as_deref(), Some("network-a"));
    }

    #[test]
    fn membership_update_rejects_another_device() {
        let mut session = PersistedSession::empty();
        session.device_id = Some("device-1".to_string());

        let result = apply_device_network_membership(
            &mut session,
            DeviceNetworkMembershipChangedPayload {
                device_id: "device-2".to_string(),
                network_ids: vec!["network-a".to_string()],
                changed_network_id: None,
                operation: None,
                membership_version: 0,
            },
        );

        assert!(result.is_err());
        assert!(session.network_ids.is_empty());
    }

    #[test]
    fn first_membership_selects_the_first_joined_network() {
        let mut session = PersistedSession::empty();
        session.device_id = Some("device-1".to_string());

        apply_device_network_membership(
            &mut session,
            DeviceNetworkMembershipChangedPayload {
                device_id: "device-1".to_string(),
                network_ids: vec!["network-b".to_string(), "network-a".to_string()],
                changed_network_id: Some("network-b".to_string()),
                operation: Some("joined".to_string()),
                membership_version: 1,
            },
        )
        .expect("apply first membership");

        assert_eq!(session.network_ids, vec!["network-a", "network-b"]);
        assert_eq!(session.active_network_id.as_deref(), Some("network-a"));
    }

    #[test]
    fn business_data_preserves_network_event_id_for_runtime_deduplication() {
        let data = network_event_business_data(
            "event-1",
            "network-1",
            &NetworkEventType::AclChanged,
            7,
            true,
            "event",
        );

        assert_eq!(data["eventId"], "event-1");
        assert_eq!(data["networkId"], "network-1");
        assert_eq!(data["configVersion"], 7);
    }

    #[test]
    fn synthetic_snapshot_ids_are_scoped_by_network() {
        assert_eq!(
            synthetic_snapshot_event_id("startup", "network-a", 1),
            "startup-snapshot-network-a-1"
        );
        assert_ne!(
            synthetic_snapshot_event_id("recovery", "network-a", 1),
            synthetic_snapshot_event_id("recovery", "network-b", 1)
        );
    }
}
