package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
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
	payloadDigest := sha256.Sum256(marshalNetworkEventPayload(payload))
	return NetworkEventEnvelope{
		Type:       "network_event",
		NetworkID:  networkID,
		Version:    version,
		EventID:    fmt.Sprintf("%s-%s-%d-%d-%s", networkID, eventType, version, occurredAt, hex.EncodeToString(payloadDigest[:6])),
		EventType:  eventType,
		OccurredAt: occurredAt,
		Payload:    payload,
	}
}

func marshalNetworkEventPayload(payload any) []byte {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return []byte(fmt.Sprintf("%T:%v", payload, payload))
	}
	return encoded
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
	devices repository.DeviceRepository,
	networks repository.NetworkRepository,
	runtimeNodes repository.RuntimeNodeRepository,
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
		Devices:      devices,
		Networks:     networks,
		RuntimeNodes: runtimeNodes,
		Now:          nowFn,
	}
	resolved, err := core.ResolvedNetworkConfig(ctx, networkID, primaryDeviceID)
	if err != nil {
		return NetworkSnapshotPayload{}, err
	}
	payload := buildNetworkEventSnapshotPayload(resolved)
	deviceGroups, err := buildNetworkEventDeviceGroups(ctx, devices, networks, networkID)
	if err != nil {
		return NetworkSnapshotPayload{}, err
	}
	payload.DeviceGroups = deviceGroups
	return payload, nil
}

func buildNetworkEventDeviceGroups(
	ctx context.Context,
	devices repository.DeviceRepository,
	networks repository.NetworkRepository,
	networkID string,
) ([]NetworkEventDeviceGroupView, error) {
	groupRepository, ok := networks.(repository.NetworkDeviceGroupRepository)
	if !ok || devices == nil {
		return []NetworkEventDeviceGroupView{}, nil
	}
	references, err := groupRepository.ListNetworkDeviceGroupReferences(ctx, networkID)
	if err != nil {
		return nil, err
	}
	assignments, err := devices.ListDeviceGroupAssignments(ctx)
	if err != nil {
		return nil, err
	}
	membersByGroupID := make(map[string][]string, len(references))
	for _, assignment := range assignments {
		for _, groupID := range assignment.GroupIDs {
			membersByGroupID[groupID] = append(membersByGroupID[groupID], assignment.DeviceID)
		}
	}
	result := make([]NetworkEventDeviceGroupView, 0, len(references))
	for _, reference := range references {
		group, exists, err := devices.GetDeviceGroup(ctx, reference.GroupID)
		if err != nil {
			return nil, err
		}
		if !exists {
			continue
		}
		memberDeviceIDs := append([]string(nil), membersByGroupID[group.GroupID]...)
		slices.Sort(memberDeviceIDs)
		result = append(result, NetworkEventDeviceGroupView{
			GroupID: group.GroupID, Name: group.Name, Tags: []string{},
			MemberDeviceIDs: memberDeviceIDs, UpdatedAt: group.UpdatedAt,
		})
	}
	slices.SortFunc(result, func(left, right NetworkEventDeviceGroupView) int {
		return strings.Compare(left.GroupID, right.GroupID)
	})
	return result, nil
}

func networkEventMemberView(member model.NetworkDevice, now time.Time) NetworkEventMemberView {
	member = normalizeNetworkMember(member)
	return NetworkEventMemberView{
		DeviceID:   strings.TrimSpace(member.DeviceID),
		VirtualIP:  strings.TrimSpace(member.VirtualIP),
		Online:     networkMemberOnlineAt(member, now),
		LastSeenAt: member.LastSeenAt,
		Tags:       []string{},
		GroupIDs:   []string{},
	}
}

func networkEventDNSRecords(
	records []model.DNSRecord,
	zones []model.DNSZone,
) []NetworkEventDNSRecordView {
	zoneNamesByID := networkEventZoneNamesByID(zones)
	out := make([]NetworkEventDNSRecordView, 0, len(records))
	for _, record := range records {
		port, _ := strconv.Atoi(normalizeDNSRecordPort(record.Type, record.Port))
		value := strings.TrimSpace(record.Value)
		targetDeviceID := ""
		cname := ""
		switch strings.ToUpper(strings.TrimSpace(record.Type)) {
		case "CNAME":
			cname = value
		case "A", "AAAA":
			targetDeviceID = value
		}
		out = append(out, NetworkEventDNSRecordView{
			RecordID:       record.RecordID,
			ZoneID:         record.ZoneID,
			NetworkID:      record.NetworkID,
			Name:           record.Name,
			FQDN:           networkEventRecordFQDN(record.Name, zoneNamesByID[record.ZoneID]),
			RecordType:     strings.TrimSpace(record.Type),
			Value:          value,
			TargetDeviceID: targetDeviceID,
			CNAME:          cname,
			Port:           port,
			TTL:            record.TTL,
			Enabled:        true,
			UpdatedAt:      record.UpdatedAt,
		})
	}
	return out
}

func networkEventZoneNamesByID(zones []model.DNSZone) map[string]string {
	out := make(map[string]string, len(zones))
	for _, zone := range zones {
		zoneID := strings.TrimSpace(zone.ZoneID)
		if zoneID == "" {
			continue
		}
		out[zoneID] = strings.TrimSpace(zone.Name)
	}
	return out
}

func networkEventRecordFQDN(name, zoneName string) string {
	fqdn := strings.TrimSpace(name)
	zoneName = strings.TrimSpace(zoneName)
	if fqdn == "" {
		return ""
	}
	if zoneName == "" {
		return fqdn
	}
	lowerFQDN := strings.ToLower(fqdn)
	lowerZone := strings.ToLower(zoneName)
	if strings.EqualFold(fqdn, zoneName) || strings.HasSuffix(lowerFQDN, "."+lowerZone) {
		return fqdn
	}
	return fqdn + "." + zoneName
}

func networkEventDNSZones(zones []model.DNSZone) []NetworkEventDNSZoneView {
	out := make([]NetworkEventDNSZoneView, 0, len(zones))
	for _, zone := range zones {
		out = append(out, NetworkEventDNSZoneView{
			ZoneID:    zone.ZoneID,
			NetworkID: zone.NetworkID,
			ZoneName:  zone.Name,
			UpdatedAt: zone.UpdatedAt,
		})
	}
	return out
}

func networkEventACLRules(rules []model.SecurityRule) []NetworkEventACLRuleView {
	out := make([]NetworkEventACLRuleView, 0, len(rules))
	for _, rule := range rules {
		sourceType := strings.TrimSpace(rule.PeerType)
		sourceValue := strings.TrimSpace(rule.PeerValue)
		sourceValues := []string{}
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
		default:
			if sourceValue != "" {
				sourceValues = []string{sourceValue}
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
			SourceValues:    sourceValues,
			SourceDeviceIDs: sourceDeviceIDs,
			SourceGroupIDs:  sourceGroupIDs,
			TargetType:      "current_device",
			TargetValues:    []string{},
			TargetDeviceIDs: []string{},
			TargetGroupIDs:  []string{},
			Enabled:         rule.Enabled,
			UpdatedAt:       rule.UpdatedAt,
		})
	}
	return out
}
