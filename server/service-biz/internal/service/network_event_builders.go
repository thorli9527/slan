package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

func newNetworkEventEnvelope(
	eventType NetworkEventType,
	networkID string,
	version uint64,
	occurredAt int64,
	payload any,
) NetworkEventEnvelope {
	networkID = strings.TrimSpace(networkID)
	return NetworkEventEnvelope{
		Type:       "network_event",
		NetworkID:  networkID,
		Version:    version,
		EventID:    fmt.Sprintf("%s-%s-%d-%d", networkID, eventType, version, occurredAt),
		EventType:  eventType,
		OccurredAt: occurredAt,
		Payload:    payload,
	}
}

func publishNetworkEvent(
	ctx context.Context,
	eventPublisher NetworkEventPublisher,
	eventType NetworkEventType,
	networkID string,
	version uint64,
	occurredAt int64,
	payload any,
) error {
	if eventPublisher == nil {
		return nil
	}
	return eventPublisher.PublishNetworkEvent(
		ctx,
		newNetworkEventEnvelope(eventType, networkID, version, occurredAt, payload),
	)
}

func buildNetworkEventSnapshotFromRepositories(
	ctx context.Context,
	users repository.UserRepository,
	devices repository.DeviceRepository,
	networks repository.NetworkRepository,
	ops repository.OpsRepository,
	nowFn func() time.Time,
	networkID string,
) (NetworkSnapshotPayload, error) {
	networkID = strings.TrimSpace(networkID)
	if networkID == "" {
		return NetworkSnapshotPayload{}, ErrInvalidArgument
	}
	memberships, err := networks.ListNetworkDevices(ctx, networkID)
	if err != nil {
		return NetworkSnapshotPayload{}, err
	}
	primaryDeviceID, err := firstActiveManagedNetworkDeviceID(ctx, devices, memberships)
	if err != nil {
		return NetworkSnapshotPayload{}, err
	}
	if primaryDeviceID == "" {
		network, err := requireManagedNetwork(ctx, networks, networkID)
		if err != nil {
			return NetworkSnapshotPayload{}, err
		}
		return NetworkSnapshotPayload{
			Network: NetworkEventNetworkView{
				NetworkID:        network.NetworkID,
				Name:             network.Name,
				DefaultACLPolicy: network.IntraGroupPolicy,
				UpdatedAt:        network.UpdatedAt,
			},
		}, nil
	}
	core := NetworkCoreService{
		Users:    users,
		Devices:  devices,
		Networks: networks,
		Ops:      ops,
		Now:      nowFn,
	}
	resolved, err := core.ResolvedNetworkConfig(ctx, networkID, primaryDeviceID)
	if err != nil {
		return NetworkSnapshotPayload{}, err
	}
	return buildNetworkEventSnapshotPayload(resolved), nil
}

func networkEventMemberView(member model.NetworkDevice) NetworkEventMemberView {
	member = normalizeNetworkMember(member)
	return NetworkEventMemberView{
		DeviceID:   strings.TrimSpace(member.DeviceID),
		VirtualIP:  strings.TrimSpace(member.VirtualIP),
		Online:     networkMemberOnline(member),
		LastSeenAt: member.LastSeenAt,
		Tags:       []string{},
		GroupIDs:   []string{},
	}
}

func networkEventDNSRecords(records []model.DNSRecord) []NetworkEventDNSRecordView {
	out := make([]NetworkEventDNSRecordView, 0, len(records))
	for _, record := range records {
		out = append(out, NetworkEventDNSRecordView{
			RecordID:  record.RecordID,
			ZoneID:    record.ZoneID,
			Name:      record.Name,
			FQDN:      record.Name,
			TargetIP:  record.Value,
			Enabled:   true,
			UpdatedAt: record.UpdatedAt,
		})
	}
	return out
}

func networkEventACLRules(rules []model.SecurityRule) []NetworkEventACLRuleView {
	out := make([]NetworkEventACLRuleView, 0, len(rules))
	for _, rule := range rules {
		sourceType := strings.TrimSpace(rule.PeerType)
		sourceValue := strings.TrimSpace(rule.PeerValue)
		sourceDeviceIDs := []string{}
		sourceGroupIDs := []string{}
		switch sourceType {
		case "device":
			if sourceValue != "" {
				sourceDeviceIDs = []string{sourceValue}
			}
		case "device_group":
			if sourceValue != "" {
				sourceGroupIDs = []string{sourceValue}
			}
		}
		out = append(out, NetworkEventACLRuleView{
			RuleID:          rule.RuleID,
			Priority:        rule.Priority,
			Action:          rule.Action,
			Direction:       rule.Direction,
			Protocol:        rule.Protocol,
			PortRanges:      []string{rule.PortRange},
			SourceType:      sourceType,
			SourceDeviceIDs: sourceDeviceIDs,
			SourceGroupIDs:  sourceGroupIDs,
			TargetType:      "current_device",
			TargetDeviceIDs: []string{},
			TargetGroupIDs:  []string{},
			Enabled:         rule.Enabled,
			UpdatedAt:       rule.UpdatedAt,
		})
	}
	return out
}
