use anyhow::{bail, Result};

use crate::{
    network_event::{
        NetworkEventAclChangedPayload, NetworkEventConfigChangedPayload,
        NetworkEventDeviceGroupPayload, NetworkEventDeviceGroupRemovedPayload,
        NetworkEventDnsChangedPayload, NetworkEventEnvelope, NetworkEventMemberPayload,
        NetworkEventMemberRemovedPayload, NetworkEventMemberView,
        NetworkEventPeerPathChangedPayload, NetworkEventPresencePayload, NetworkEventType,
        NetworkSnapshotPayload,
    },
    network_runtime_state::{NetworkSyncStatus, RuntimeNetworkState},
};

#[derive(Debug, Clone, PartialEq, Eq)]
pub enum ApplyResult {
    Applied,
    IgnoredDuplicate,
    IgnoredStale,
    NeedsSnapshot,
}

pub fn apply_network_event(
    state: &mut RuntimeNetworkState,
    envelope: NetworkEventEnvelope,
) -> Result<ApplyResult> {
    if envelope.r#type != "network_event" {
        bail!("unsupported network event type");
    }
    if let Some(active_network_id) = state
        .active_network_id
        .as_deref()
        .map(str::trim)
        .filter(|value| !value.is_empty())
    {
        if active_network_id != envelope.network_id.trim() {
            state.sync_status = NetworkSyncStatus::OutOfSync;
            return Ok(ApplyResult::NeedsSnapshot);
        }
    } else {
        state.bind_network_id(&envelope.network_id);
    }
    let versionless_runtime_event = is_versionless_runtime_event(&envelope);
    if !versionless_runtime_event {
        match check_version(state, envelope.version) {
            VersionCheck::IgnoreStale => return Ok(ApplyResult::IgnoredStale),
            VersionCheck::NeedSnapshot => {
                state.sync_status = NetworkSyncStatus::OutOfSync;
                return Ok(ApplyResult::NeedsSnapshot);
            }
            VersionCheck::Apply => {}
        }
    }
    if state.has_seen_event(&envelope.event_id) {
        return Ok(ApplyResult::IgnoredDuplicate);
    }

    match envelope.event_type {
        NetworkEventType::NetworkSnapshot => {
            let payload: NetworkSnapshotPayload = serde_json::from_value(envelope.payload)?;
            apply_snapshot(state, payload);
        }
        NetworkEventType::MemberAdded | NetworkEventType::MemberUpdated => {
            let payload: NetworkEventMemberPayload = serde_json::from_value(envelope.payload)?;
            state
                .members_by_device_id
                .insert(payload.member.device_id.clone(), payload.member);
        }
        NetworkEventType::MemberRemoved => {
            let payload: NetworkEventMemberRemovedPayload =
                serde_json::from_value(envelope.payload)?;
            state.members_by_device_id.remove(&payload.device_id);
        }
        NetworkEventType::MemberOnline | NetworkEventType::MemberOffline => {
            let payload: NetworkEventPresencePayload = serde_json::from_value(envelope.payload)?;
            let member = state
                .members_by_device_id
                .entry(payload.device_id.clone())
                .or_insert_with(|| NetworkEventMemberView {
                    device_id: payload.device_id.clone(),
                    ..NetworkEventMemberView::default()
                });
            member.online = payload.online;
            member.last_seen_at = payload.last_seen_at;
        }
        NetworkEventType::DeviceGroupAdded | NetworkEventType::DeviceGroupUpdated => {
            let payload: NetworkEventDeviceGroupPayload = serde_json::from_value(envelope.payload)?;
            state
                .groups_by_group_id
                .insert(payload.group.group_id.clone(), payload.group);
        }
        NetworkEventType::DeviceGroupRemoved => {
            let payload: NetworkEventDeviceGroupRemovedPayload =
                serde_json::from_value(envelope.payload)?;
            state.groups_by_group_id.remove(&payload.group_id);
        }
        NetworkEventType::AclChanged => {
            let payload: NetworkEventAclChangedPayload = serde_json::from_value(envelope.payload)?;
            state.acl_by_rule_id.clear();
            for rule in payload.rules {
                state.acl_by_rule_id.insert(rule.rule_id.clone(), rule);
            }
        }
        NetworkEventType::DnsChanged => {
            let payload: NetworkEventDnsChangedPayload = serde_json::from_value(envelope.payload)?;
            state.dns_by_record_id.clear();
            for record in payload.records {
                state
                    .dns_by_record_id
                    .insert(record.record_id.clone(), record);
            }
        }
        NetworkEventType::NetworkConfigChanged => {
            let payload: NetworkEventConfigChangedPayload =
                serde_json::from_value(envelope.payload)?;
            let incoming = payload.network;
            if let Some(existing) = state.network.as_mut() {
                if !incoming.network_id.trim().is_empty() {
                    existing.network_id = incoming.network_id;
                }
                if !incoming.name.trim().is_empty() {
                    existing.name = incoming.name;
                }
                if !incoming.tags.is_empty() {
                    existing.tags = incoming.tags;
                }
                if !incoming.default_acl_policy.trim().is_empty() {
                    existing.default_acl_policy = incoming.default_acl_policy;
                }
                if incoming.updated_at > 0 {
                    existing.updated_at = incoming.updated_at;
                }
            } else {
                state.network = Some(incoming);
            }
        }
        NetworkEventType::PeerPathChanged => {
            let payload: NetworkEventPeerPathChangedPayload =
                serde_json::from_value(envelope.payload)?;
            state.peer_paths_by_device_id.clear();
            for path in payload.paths {
                state
                    .peer_paths_by_device_id
                    .insert(path.peer_device_id.clone(), path);
            }
        }
    }

    if !versionless_runtime_event {
        state.version = envelope.version;
    }
    state.sync_status = NetworkSyncStatus::Live;
    state.remember_event(envelope.event_id, envelope.occurred_at);
    Ok(ApplyResult::Applied)
}

fn is_versionless_runtime_event(envelope: &NetworkEventEnvelope) -> bool {
    envelope.version == 0
        && matches!(
            envelope.event_type,
            NetworkEventType::MemberOnline | NetworkEventType::MemberOffline
        )
}

fn apply_snapshot(state: &mut RuntimeNetworkState, payload: NetworkSnapshotPayload) {
    state.bind_network_id(&payload.network.network_id);
    state.network = Some(payload.network);
    state.members_by_device_id = payload
        .members
        .into_iter()
        .map(|item| (item.device_id.clone(), item))
        .collect();
    state.groups_by_group_id = payload
        .device_groups
        .into_iter()
        .map(|item| (item.group_id.clone(), item))
        .collect();
    state.dns_by_record_id = payload
        .dns_records
        .into_iter()
        .map(|item| (item.record_id.clone(), item))
        .collect();
    state.acl_by_rule_id = payload
        .acl_rules
        .into_iter()
        .map(|item| (item.rule_id.clone(), item))
        .collect();
    state.peer_paths_by_device_id = payload
        .peer_paths
        .into_iter()
        .map(|item| (item.peer_device_id.clone(), item))
        .collect();
}

enum VersionCheck {
    Apply,
    IgnoreStale,
    NeedSnapshot,
}

fn check_version(state: &RuntimeNetworkState, incoming: u64) -> VersionCheck {
    if incoming <= state.version {
        return VersionCheck::IgnoreStale;
    }
    if state.version > 0 && incoming > state.version + 1 {
        return VersionCheck::NeedSnapshot;
    }
    VersionCheck::Apply
}

#[cfg(test)]
mod tests {
    use super::{apply_network_event, ApplyResult};
    use crate::{
        network_event::{
            NetworkEventAclChangedPayload, NetworkEventEnvelope, NetworkEventMemberView,
            NetworkEventType,
        },
        network_runtime_state::{NetworkSyncStatus, RuntimeNetworkState},
    };

    #[test]
    fn first_event_binds_active_network_id() {
        let mut state = RuntimeNetworkState::default();
        let result = apply_network_event(
            &mut state,
            NetworkEventEnvelope {
                r#type: "network_event".to_string(),
                network_id: "net-1".to_string(),
                version: 1,
                event_id: "evt-1".to_string(),
                event_type: NetworkEventType::DnsChanged,
                occurred_at: 1,
                payload: serde_json::json!({ "records": [] }),
            },
        )
        .expect("apply first event");

        assert_eq!(result, ApplyResult::Applied);
        assert_eq!(state.active_network_id.as_deref(), Some("net-1"));
        assert_eq!(state.sync_status, NetworkSyncStatus::Live);
    }

    #[test]
    fn mismatched_network_requires_snapshot() {
        let mut state = RuntimeNetworkState::default();
        state.active_network_id = Some("net-1".to_string());

        let result = apply_network_event(
            &mut state,
            NetworkEventEnvelope {
                r#type: "network_event".to_string(),
                network_id: "net-2".to_string(),
                version: 1,
                event_id: "evt-2".to_string(),
                event_type: NetworkEventType::DnsChanged,
                occurred_at: 2,
                payload: serde_json::json!({ "records": [] }),
            },
        )
        .expect("apply mismatched event");

        assert_eq!(result, ApplyResult::NeedsSnapshot);
        assert_eq!(state.sync_status, NetworkSyncStatus::OutOfSync);
    }

    #[test]
    fn network_event_payload_treats_null_sequences_as_empty() {
        let member: NetworkEventMemberView = serde_json::from_value(serde_json::json!({
            "deviceId": "device-1",
            "deviceName": "Device 1",
            "virtualIp": "10.0.1.10",
            "online": true,
            "lastSeenAt": 1,
            "tags": null,
            "groupIds": null,
            "deviceVersion": "1.0.0",
            "platform": "linux"
        }))
        .expect("decode member");
        assert!(member.tags.is_empty());
        assert!(member.group_ids.is_empty());

        let acl: NetworkEventAclChangedPayload = serde_json::from_value(serde_json::json!({
            "rules": null
        }))
        .expect("decode acl");
        assert!(acl.rules.is_empty());
    }

    #[test]
    fn applies_versionless_member_presence_without_advancing_snapshot_version() {
        let mut state = RuntimeNetworkState::default();
        state.bind_network_id("net-1");
        state.version = 480;
        state.members_by_device_id.insert(
            "dev-2".to_string(),
            NetworkEventMemberView {
                device_id: "dev-2".to_string(),
                online: false,
                ..NetworkEventMemberView::default()
            },
        );

        let result = apply_network_event(
            &mut state,
            NetworkEventEnvelope {
                r#type: "network_event".to_string(),
                network_id: "net-1".to_string(),
                version: 0,
                event_id: "presence-dev-2-1".to_string(),
                event_type: NetworkEventType::MemberOnline,
                occurred_at: 1,
                payload: serde_json::json!({
                    "deviceId": "dev-2",
                    "online": true,
                    "lastSeenAt": 123
                }),
            },
        )
        .expect("apply versionless presence event");

        assert_eq!(result, ApplyResult::Applied);
        assert_eq!(state.version, 480);
        assert_eq!(
            state
                .members_by_device_id
                .get("dev-2")
                .expect("member exists")
                .online,
            true
        );
    }
}
