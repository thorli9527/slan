package service

import "context"

func (s NetworkCoreService) NetworkSnapshot(
	ctx context.Context,
	networkID, deviceID string,
) (NetworkSnapshotResponse, error) {
	networkID = normalizeNetworkID(networkID)
	deviceID = normalizeDeviceID(deviceID)
	if networkID == "" || deviceID == "" {
		return NetworkSnapshotResponse{}, ErrInvalidArgument
	}
	if _, err := requireManagedNetwork(ctx, s.Networks, networkID); err != nil {
		return NetworkSnapshotResponse{}, err
	}
	if _, err := requireManagedDevice(ctx, s.Devices, deviceID); err != nil {
		return NetworkSnapshotResponse{}, err
	}
	resolved, err := s.ResolvedNetworkConfig(ctx, networkID, deviceID)
	if err != nil {
		return NetworkSnapshotResponse{}, err
	}
	return NetworkSnapshotResponse{
		NetworkID: networkID,
		Version:   uint64(resolved.Config.ConfigVersion),
		Snapshot:  buildNetworkEventSnapshotPayload(resolved),
	}, nil
}

func buildNetworkEventSnapshotPayload(
	resolved NetworkResolvedConfigView,
) NetworkSnapshotPayload {
	view := resolved.Config

	members := make([]NetworkEventMemberView, 0, len(view.Peers))
	peerPaths := make([]NetworkEventPeerPathView, 0, len(view.Peers))
	for _, peer := range view.Peers {
		members = append(members, NetworkEventMemberView{
			DeviceID:   peer.DeviceID,
			DeviceName: peer.Alias,
			VirtualIP:  peer.GlobalIP,
			Online:     stringsEqualFold(peer.Status, "online"),
		})
		peerPaths = append(peerPaths, NetworkEventPeerPathView{
			PeerDeviceID: peer.DeviceID,
			PathType:     resolvedPeerPathType(peer),
			Reachable:    !stringsEqualFold(peer.Status, "offline"),
		})
	}

	deviceGroups := make([]NetworkEventDeviceGroupView, 0, len(view.DeviceGroupsByDevice))
	for deviceID, groupIDs := range view.DeviceGroupsByDevice {
		for _, groupID := range groupIDs {
			deviceGroups = append(deviceGroups, NetworkEventDeviceGroupView{
				GroupID:         groupID,
				Name:            groupID,
				MemberDeviceIDs: []string{deviceID},
			})
		}
	}

	dnsRecords := make([]NetworkEventDNSRecordView, 0, len(view.DNSRecords))
	for _, record := range view.DNSRecords {
		dnsRecords = append(dnsRecords, NetworkEventDNSRecordView{
			RecordID:       record.RecordID,
			ZoneID:         record.ZoneID,
			Name:           record.Name,
			FQDN:           record.Name,
			TargetDeviceID: record.TargetDeviceID,
			TargetIP:       record.TargetIP,
			Enabled:        !stringsEqualFold(record.Status, "disabled"),
			UpdatedAt:      record.UpdatedAt,
		})
	}

	aclRules := make([]NetworkEventACLRuleView, 0, len(view.SecurityRules))
	for _, rule := range view.SecurityRules {
		aclRules = append(aclRules, NetworkEventACLRuleView{
			RuleID:     rule.RuleID,
			Priority:   int(rule.Priority),
			Action:     rule.Action,
			Direction:  rule.Direction,
			Protocol:   rule.Protocol,
			PortRanges: []string{rule.PortRange},
			SourceType: rule.PeerType,
			TargetType: "current_device",
			Enabled:    rule.Enabled,
			UpdatedAt:  rule.UpdatedAt,
		})
	}

	return NetworkSnapshotPayload{
		Network: NetworkEventNetworkView{
			NetworkID:        view.Network.NetworkID,
			Name:             view.Network.Name,
			DefaultACLPolicy: defaultACLPolicyFromResolved(view),
			UpdatedAt:        int64(view.Network.UpdatedAt),
		},
		Members:      members,
		DeviceGroups: deviceGroups,
		DNSRecords:   dnsRecords,
		ACLRules:     aclRules,
		PeerPaths:    peerPaths,
	}
}

func defaultACLPolicyFromResolved(view NetworkConfigView) string {
	if value := view.Network.IntraGroupPolicy; value != "" {
		return value
	}
	return "allow"
}

func resolvedPeerPathType(peer NetworkConfigPeerView) string {
	if len(peer.Endpoints) > 0 {
		return "direct"
	}
	return "relay"
}

func stringsEqualFold(left, right string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		a := left[i]
		b := right[i]
		if a == b {
			continue
		}
		if 'A' <= a && a <= 'Z' {
			a += 'a' - 'A'
		}
		if 'A' <= b && b <= 'Z' {
			b += 'a' - 'A'
		}
		if a != b {
			return false
		}
	}
	return true
}
