package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

type networkAccessTestDevices struct {
	devices                map[string]model.Device
	deviceGroups           map[string]model.DeviceGroup
	deviceGroupAssignments []model.DeviceGroupAssignment
}

func (s *networkAccessTestDevices) GetDevice(_ context.Context, deviceID string) (model.Device, bool, error) {
	item, ok := s.devices[deviceID]
	return item, ok, nil
}

func (s *networkAccessTestDevices) SaveDevice(_ context.Context, device model.Device) error {
	if s.devices == nil {
		s.devices = map[string]model.Device{}
	}
	s.devices[device.DeviceID] = device
	return nil
}
func (s *networkAccessTestDevices) DeleteDevice(context.Context, string) error { return nil }
func (s *networkAccessTestDevices) NewDeviceVirtualIPID() string               { return "vip-000001" }
func (s *networkAccessTestDevices) GetDeviceSessionByAccessToken(context.Context, string) (model.DeviceSession, bool, error) {
	return model.DeviceSession{}, false, nil
}
func (s *networkAccessTestDevices) GetDeviceSessionByRefreshToken(context.Context, string) (model.DeviceSession, bool, error) {
	return model.DeviceSession{}, false, nil
}
func (s *networkAccessTestDevices) ListDeviceSessionsByDeviceID(context.Context, string) ([]model.DeviceSession, error) {
	return nil, nil
}
func (s *networkAccessTestDevices) SaveDeviceSession(context.Context, model.DeviceSession) error {
	return nil
}

func (s *networkAccessTestDevices) RotateDeviceSession(context.Context, string, model.DeviceSession) (bool, error) {
	return false, nil
}
func (s *networkAccessTestDevices) DeleteDeviceSessionForRefreshReuse(context.Context, string, string, int64) (bool, error) {
	return false, nil
}
func (s *networkAccessTestDevices) DeleteDeviceSessionByAccessToken(context.Context, string) error {
	return nil
}
func (s *networkAccessTestDevices) ListDeviceGroups(context.Context) ([]model.DeviceGroup, error) {
	items := make([]model.DeviceGroup, 0, len(s.deviceGroups))
	for _, item := range s.deviceGroups {
		items = append(items, item)
	}
	return items, nil
}
func (s *networkAccessTestDevices) GetDeviceGroup(_ context.Context, groupID string) (model.DeviceGroup, bool, error) {
	item, ok := s.deviceGroups[groupID]
	return item, ok, nil
}
func (s *networkAccessTestDevices) SaveDeviceGroup(_ context.Context, item model.DeviceGroup) error {
	if s.deviceGroups == nil {
		s.deviceGroups = make(map[string]model.DeviceGroup)
	}
	s.deviceGroups[item.GroupID] = item
	return nil
}
func (s *networkAccessTestDevices) DeleteDeviceGroup(context.Context, string) error { return nil }
func (s *networkAccessTestDevices) SetDeviceGroups(_ context.Context, assignment model.DeviceGroupAssignment) error {
	for index, item := range s.deviceGroupAssignments {
		if item.DeviceID == assignment.DeviceID {
			s.deviceGroupAssignments[index] = assignment
			return nil
		}
	}
	s.deviceGroupAssignments = append(s.deviceGroupAssignments, assignment)
	return nil
}
func (s *networkAccessTestDevices) ListDeviceGroupAssignments(context.Context) ([]model.DeviceGroupAssignment, error) {
	out := make([]model.DeviceGroupAssignment, len(s.deviceGroupAssignments))
	copy(out, s.deviceGroupAssignments)
	return out, nil
}

type networkAccessTestNetworks struct {
	networks        map[string]model.Network
	securityGroup   map[string]model.SecurityGroup
	securityRules   map[string]model.SecurityRule
	versions        map[string]model.NetworkConfigVersion
	networkDevices  map[string][]model.NetworkDevice
	groupReferences map[string][]model.NetworkDeviceGroupReference
}

func (s *networkAccessTestNetworks) ListNetworkDeviceGroupReferences(_ context.Context, networkID string) ([]model.NetworkDeviceGroupReference, error) {
	return append([]model.NetworkDeviceGroupReference(nil), s.groupReferences[networkID]...), nil
}

func (s *networkAccessTestNetworks) SaveNetworkDeviceGroupReference(_ context.Context, item model.NetworkDeviceGroupReference) error {
	if s.groupReferences == nil {
		s.groupReferences = map[string][]model.NetworkDeviceGroupReference{}
	}
	items := s.groupReferences[item.NetworkID]
	for index := range items {
		if items[index].GroupID == item.GroupID {
			items[index] = item
			s.groupReferences[item.NetworkID] = items
			return nil
		}
	}
	s.groupReferences[item.NetworkID] = append(items, item)
	return nil
}

func (s *networkAccessTestNetworks) DeleteNetworkDeviceGroupReference(_ context.Context, networkID, groupID string) error {
	items := s.groupReferences[networkID]
	filtered := items[:0]
	for _, item := range items {
		if item.GroupID != groupID {
			filtered = append(filtered, item)
		}
	}
	s.groupReferences[networkID] = filtered
	return nil
}

func (s *networkAccessTestNetworks) DeleteNetworkDeviceGroupReferencesByGroup(_ context.Context, groupID string) error {
	for networkID := range s.groupReferences {
		_ = s.DeleteNetworkDeviceGroupReference(context.Background(), networkID, groupID)
	}
	return nil
}

func (s *networkAccessTestNetworks) GetNetwork(_ context.Context, networkID string) (model.Network, bool, error) {
	item, ok := s.networks[networkID]
	return item, ok, nil
}

func (s *networkAccessTestNetworks) ListNetworksByOwner(context.Context, string) ([]model.Network, error) {
	items := make([]model.Network, 0)
	for _, item := range s.networks {
		items = append(items, item)
	}
	return items, nil
}

func (s *networkAccessTestNetworks) ListNetworks(_ context.Context) ([]model.Network, error) {
	items := make([]model.Network, 0, len(s.networks))
	for _, item := range s.networks {
		items = append(items, item)
	}
	return items, nil
}

func (s *networkAccessTestNetworks) ListNetworksByDevice(_ context.Context, deviceID string) ([]model.Network, error) {
	items := make([]model.Network, 0)
	for networkID, memberships := range s.networkDevices {
		for _, membership := range memberships {
			if membership.DeviceID != deviceID {
				continue
			}
			if item, ok := s.networks[networkID]; ok {
				items = append(items, item)
			}
			break
		}
	}
	return items, nil
}

func (s *networkAccessTestNetworks) SaveNetwork(_ context.Context, item model.Network) error {
	if s.networks == nil {
		s.networks = make(map[string]model.Network)
	}
	s.networks[item.NetworkID] = item
	return nil
}

func (s *networkAccessTestNetworks) DeleteNetwork(context.Context, string) error { return nil }
func (s *networkAccessTestNetworks) ListNetworkDevices(_ context.Context, networkID string) ([]model.NetworkDevice, error) {
	items := s.networkDevices[networkID]
	out := make([]model.NetworkDevice, len(items))
	copy(out, items)
	return out, nil
}
func (s *networkAccessTestNetworks) GetNetworkDevice(_ context.Context, networkID, deviceID string) (model.NetworkDevice, bool, error) {
	for _, item := range s.networkDevices[networkID] {
		if item.DeviceID == deviceID {
			return item, true, nil
		}
	}
	return model.NetworkDevice{}, false, nil
}
func (s *networkAccessTestNetworks) SaveNetworkDevice(_ context.Context, item model.NetworkDevice) error {
	if s.networkDevices == nil {
		s.networkDevices = map[string][]model.NetworkDevice{}
	}
	items := s.networkDevices[item.NetworkID]
	for index := range items {
		if items[index].DeviceID == item.DeviceID {
			items[index] = item
			s.networkDevices[item.NetworkID] = items
			return nil
		}
	}
	s.networkDevices[item.NetworkID] = append(items, item)
	return nil
}
func (s *networkAccessTestNetworks) GetNetworkVersion(_ context.Context, networkID string) (model.NetworkConfigVersion, bool, error) {
	item, ok := s.versions[networkID]
	return item, ok, nil
}
func (s *networkAccessTestNetworks) SaveNetworkVersion(_ context.Context, item model.NetworkConfigVersion) error {
	if s.versions == nil {
		s.versions = make(map[string]model.NetworkConfigVersion)
	}
	s.versions[item.NetworkID] = item
	return nil
}
func (s *networkAccessTestNetworks) DeleteNetworkDevice(_ context.Context, networkID, deviceID string) error {
	items := s.networkDevices[networkID]
	filtered := items[:0]
	for _, item := range items {
		if item.DeviceID != deviceID {
			filtered = append(filtered, item)
		}
	}
	s.networkDevices[networkID] = filtered
	return nil
}
func (s *networkAccessTestNetworks) ListDNSZones(context.Context, string) ([]model.DNSZone, error) {
	return nil, nil
}
func (s *networkAccessTestNetworks) GetDNSZone(context.Context, string) (model.DNSZone, bool, error) {
	return model.DNSZone{}, false, nil
}
func (s *networkAccessTestNetworks) SaveDNSZone(context.Context, model.DNSZone) error { return nil }
func (s *networkAccessTestNetworks) DeleteDNSZone(context.Context, string) error      { return nil }
func (s *networkAccessTestNetworks) ListDNSRecords(context.Context, string) ([]model.DNSRecord, error) {
	return nil, nil
}
func (s *networkAccessTestNetworks) GetDNSRecord(context.Context, string) (model.DNSRecord, bool, error) {
	return model.DNSRecord{}, false, nil
}
func (s *networkAccessTestNetworks) SaveDNSRecord(context.Context, model.DNSRecord) error { return nil }
func (s *networkAccessTestNetworks) DeleteDNSRecord(context.Context, string) error        { return nil }
func (s *networkAccessTestNetworks) ListSecurityGroups(context.Context, string) ([]model.SecurityGroup, error) {
	return nil, nil
}
func (s *networkAccessTestNetworks) GetSecurityGroup(_ context.Context, securityGroupID string) (model.SecurityGroup, bool, error) {
	item, ok := s.securityGroup[securityGroupID]
	return item, ok, nil
}
func (s *networkAccessTestNetworks) SaveSecurityGroup(_ context.Context, item model.SecurityGroup) error {
	if s.securityGroup == nil {
		s.securityGroup = make(map[string]model.SecurityGroup)
	}
	s.securityGroup[item.SecurityGroupID] = item
	return nil
}
func (s *networkAccessTestNetworks) DeleteSecurityGroup(context.Context, string) error { return nil }
func (s *networkAccessTestNetworks) ListSecurityRules(context.Context, string) ([]model.SecurityRule, error) {
	return nil, nil
}
func (s *networkAccessTestNetworks) GetSecurityRule(_ context.Context, ruleID string) (model.SecurityRule, bool, error) {
	item, ok := s.securityRules[ruleID]
	return item, ok, nil
}
func (s *networkAccessTestNetworks) SaveSecurityRule(_ context.Context, item model.SecurityRule) error {
	if s.securityRules == nil {
		s.securityRules = make(map[string]model.SecurityRule)
	}
	s.securityRules[item.RuleID] = item
	return nil
}
func (s *networkAccessTestNetworks) DeleteSecurityRule(context.Context, string) error { return nil }

var _ repository.DeviceRepository = (*networkAccessTestDevices)(nil)
var _ repository.NetworkRepository = (*networkAccessTestNetworks)(nil)

type networkAccessTestBroadcaster struct {
	events []NetworkEventEnvelope
}

type networkAccessDeliveryPublisher struct {
	events  []NetworkEventEnvelope
	tracker *MqttNetworkEventPublisher
}

func (p *networkAccessDeliveryPublisher) PublishNetworkEvent(ctx context.Context, event NetworkEventEnvelope) error {
	p.events = append(p.events, event)
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return p.tracker.trackMembershipEvent(ctx, event, payload)
}

type networkAccessTestDevicePublisher struct {
	deviceIDs []string
	events    []DeviceControlEnvelope
}

func (p *networkAccessTestDevicePublisher) PublishDeviceControl(_ context.Context, deviceID string, event DeviceControlEnvelope) error {
	p.deviceIDs = append(p.deviceIDs, deviceID)
	p.events = append(p.events, event)
	return nil
}

func (s *networkAccessTestBroadcaster) PublishNetworkEvent(_ context.Context, event NetworkEventEnvelope) error {
	s.events = append(s.events, event)
	return nil
}

func TestAddSecurityRuleAllowsOwnedDeviceGroupPeer(t *testing.T) {
	devices := &networkAccessTestDevices{
		deviceGroups: map[string]model.DeviceGroup{
			"dgrp-1": {GroupID: "dgrp-1", Name: "Group 1"},
		},
	}
	networks := &networkAccessTestNetworks{
		networks: map[string]model.Network{
			"net-1": {NetworkID: "net-1", Name: "Default Network", Status: "active"},
		},
		securityGroup: map[string]model.SecurityGroup{
			"sg-1": {SecurityGroupID: "sg-1", NetworkID: "net-1", Name: "Default Security Group"},
		},
		networkDevices: map[string][]model.NetworkDevice{
			"net-1": {
				{NetworkID: "net-1", DeviceID: "dev-1", Enabled: true, MemberStatus: model.NetworkMemberStatusActive},
			},
		},
		groupReferences: map[string][]model.NetworkDeviceGroupReference{
			"net-1": {{NetworkID: "net-1", GroupID: "dgrp-1"}},
		},
	}
	devices.deviceGroupAssignments = []model.DeviceGroupAssignment{
		{DeviceID: "dev-1", GroupIDs: []string{"dgrp-1"}},
	}
	service := NetworkAccessService{
		Devices:  devices,
		Networks: networks,
		Now:      func() time.Time { return time.Unix(1700000000, 0) },
		NewSecurityRuleID: func() string {
			return "sgr-1"
		},
	}

	view, err := service.AddSecurityRule(context.Background(), CreateSecurityRuleInput{
		SecurityGroupID: "sg-1",
		Direction:       "ingress",
		Protocol:        "tcp",
		PortRange:       "443",
		PeerType:        "device_group",
		PeerValue:       "dgrp-1",
		Action:          "allow",
		Priority:        4,
		Description:     "allow group",
		Enabled:         true,
	})
	if err != nil {
		t.Fatalf("AddSecurityRule returned error: %v", err)
	}
	if view.RuleID != "sgr-1" {
		t.Fatalf("unexpected rule id: %+v", view)
	}
	saved, ok := networks.securityRules["sgr-1"]
	if !ok {
		t.Fatalf("expected security rule to be saved")
	}
	if saved.PeerType != "device_group" || saved.PeerValue != "dgrp-1" {
		t.Fatalf("unexpected saved rule peer: %+v", saved)
	}
	version, ok, err := networks.GetNetworkVersion(context.Background(), "net-1")
	if err != nil {
		t.Fatalf("GetNetworkVersion returned error: %v", err)
	}
	if !ok || version.Version != 1 {
		t.Fatalf("expected network version bump, got ok=%v version=%+v", ok, version)
	}
}

func TestUpdateSecurityRulePersistsReferencedDeviceGroupPeer(t *testing.T) {
	devices := &networkAccessTestDevices{deviceGroups: map[string]model.DeviceGroup{
		"dgrp-1": {GroupID: "dgrp-1", Name: "Members"},
	}}
	networks := &networkAccessTestNetworks{
		networks: map[string]model.Network{
			"net-1": {NetworkID: "net-1", Name: "Default Network", Status: "active"},
		},
		securityGroup: map[string]model.SecurityGroup{
			"sg-1": {SecurityGroupID: "sg-1", NetworkID: "net-1", Name: "Default Security Group"},
		},
		securityRules: map[string]model.SecurityRule{
			"sgr-1": {
				RuleID:          "sgr-1",
				SecurityGroupID: "sg-1",
				Direction:       "ingress",
				Protocol:        "tcp",
				PortRange:       "22",
				PeerType:        "device_group",
				PeerValue:       "dgrp-1",
				Action:          "allow",
				Priority:        100,
				Enabled:         true,
			},
		},
		groupReferences: map[string][]model.NetworkDeviceGroupReference{
			"net-1": {{NetworkID: "net-1", GroupID: "dgrp-1"}},
		},
	}
	service := NetworkAccessService{
		Devices:  devices,
		Networks: networks,
		Now:      func() time.Time { return time.Unix(1700000000, 0) },
	}
	priority := 5
	enabled := false

	view, err := service.UpdateSecurityRule(context.Background(), UpdateSecurityRuleInput{
		RuleID:      "sgr-1",
		Direction:   "egress",
		Protocol:    "udp",
		PortRange:   "53",
		PeerType:    "device_group",
		PeerValue:   "dgrp-1",
		Action:      "deny",
		Priority:    &priority,
		Description: "updated rule",
		Enabled:     &enabled,
	})
	if err != nil {
		t.Fatalf("UpdateSecurityRule returned error: %v", err)
	}
	if view.Priority != priority || view.Description != "updated rule" || view.Enabled {
		t.Fatalf("unexpected updated view: %+v", view)
	}
	saved := networks.securityRules["sgr-1"]
	if saved.Direction != "egress" || saved.Protocol != "udp" || saved.PortRange != "53" || saved.Action != "deny" {
		t.Fatalf("unexpected saved rule: %+v", saved)
	}
	version, ok, err := networks.GetNetworkVersion(context.Background(), "net-1")
	if err != nil {
		t.Fatalf("GetNetworkVersion returned error: %v", err)
	}
	if !ok || version.Version != 1 || version.Reason != "security_rule_updated" {
		t.Fatalf("expected update version bump, got ok=%v version=%+v", ok, version)
	}
}

func TestAddSecurityRuleAllowsCurrentNetworkDevicePeer(t *testing.T) {
	devices := &networkAccessTestDevices{devices: map[string]model.Device{
		"dev-1": {DeviceID: "dev-1", Name: "Device 1", Status: "active"},
	}}
	networks := &networkAccessTestNetworks{
		networks: map[string]model.Network{
			"net-1": {NetworkID: "net-1", Name: "Default Network", Status: "active"},
		},
		securityGroup: map[string]model.SecurityGroup{
			"sg-1": {SecurityGroupID: "sg-1", NetworkID: "net-1", Name: "Default Security Group"},
		},
		networkDevices: map[string][]model.NetworkDevice{
			"net-1": {{NetworkID: "net-1", DeviceID: "dev-1", Enabled: true, MemberStatus: model.NetworkMemberStatusActive}},
		},
	}
	service := NetworkAccessService{
		Devices: devices, Networks: networks,
		Now:               func() time.Time { return time.Unix(1700000000, 0) },
		NewSecurityRuleID: func() string { return "sgr-device-1" },
	}

	view, err := service.AddSecurityRule(context.Background(), CreateSecurityRuleInput{
		SecurityGroupID: "sg-1",
		Direction:       "ingress",
		Protocol:        "tcp",
		PortRange:       "22",
		PeerType:        "device",
		PeerValue:       "dev-1",
		Action:          "allow",
		Priority:        10,
		Enabled:         true,
	})
	if err != nil {
		t.Fatalf("AddSecurityRule returned error: %v", err)
	}
	if view.PeerType != "device" || view.PeerValue != "dev-1" {
		t.Fatalf("unexpected device peer: %+v", view)
	}
}

func TestAddSecurityRuleRejectsDevicePeerOutsideCurrentNetwork(t *testing.T) {
	devices := &networkAccessTestDevices{}
	networks := &networkAccessTestNetworks{
		networks: map[string]model.Network{
			"net-1": {NetworkID: "net-1", Name: "Default Network", Status: "active"},
		},
		securityGroup: map[string]model.SecurityGroup{
			"sg-1": {SecurityGroupID: "sg-1", NetworkID: "net-1", Name: "Default Security Group"},
		},
		networkDevices: map[string][]model.NetworkDevice{
			"net-1": {
				{NetworkID: "net-1", DeviceID: "dev-1", Enabled: true, MemberStatus: model.NetworkMemberStatusActive},
			},
		},
	}
	service := NetworkAccessService{
		Devices:  devices,
		Networks: networks,
		Now:      func() time.Time { return time.Unix(1700000000, 0) },
		NewSecurityRuleID: func() string {
			return "sgr-1"
		},
	}

	_, err := service.AddSecurityRule(context.Background(), CreateSecurityRuleInput{
		SecurityGroupID: "sg-1",
		Direction:       "ingress",
		Protocol:        "tcp",
		PortRange:       "443",
		PeerType:        "device",
		PeerValue:       "dev-outside",
		Action:          "allow",
		Priority:        4,
		Description:     "reject outside device",
		Enabled:         true,
	})
	if err == nil {
		t.Fatalf("expected AddSecurityRule to reject device outside current network")
	}
}

func TestAddSecurityRuleRejectsDeviceGroupWithoutCurrentNetworkMembers(t *testing.T) {
	devices := &networkAccessTestDevices{
		deviceGroups: map[string]model.DeviceGroup{
			"dgrp-1": {GroupID: "dgrp-1", Name: "Group 1"},
		},
		deviceGroupAssignments: []model.DeviceGroupAssignment{
			{DeviceID: "dev-outside", GroupIDs: []string{"dgrp-1"}},
		},
	}
	networks := &networkAccessTestNetworks{
		networks: map[string]model.Network{
			"net-1": {NetworkID: "net-1", Name: "Default Network", Status: "active"},
		},
		securityGroup: map[string]model.SecurityGroup{
			"sg-1": {SecurityGroupID: "sg-1", NetworkID: "net-1", Name: "Default Security Group"},
		},
		networkDevices: map[string][]model.NetworkDevice{
			"net-1": {
				{NetworkID: "net-1", DeviceID: "dev-1", Enabled: true, MemberStatus: model.NetworkMemberStatusActive},
			},
		},
	}
	service := NetworkAccessService{
		Devices:  devices,
		Networks: networks,
		Now:      func() time.Time { return time.Unix(1700000000, 0) },
		NewSecurityRuleID: func() string {
			return "sgr-1"
		},
	}

	_, err := service.AddSecurityRule(context.Background(), CreateSecurityRuleInput{
		SecurityGroupID: "sg-1",
		Direction:       "ingress",
		Protocol:        "tcp",
		PortRange:       "443",
		PeerType:        "device_group",
		PeerValue:       "dgrp-1",
		Action:          "allow",
		Priority:        4,
		Description:     "reject outside group",
		Enabled:         true,
	})
	if err == nil {
		t.Fatalf("expected AddSecurityRule to reject device group without current network members")
	}
}

func TestListNetworkDeviceGroupsReturnsReferencedGroupsAndTheirMembers(t *testing.T) {
	devices := &networkAccessTestDevices{
		deviceGroups: map[string]model.DeviceGroup{
			"dgrp-1": {GroupID: "dgrp-1", Name: "Group 1"},
			"dgrp-2": {GroupID: "dgrp-2", Name: "Group 2"},
		},
		deviceGroupAssignments: []model.DeviceGroupAssignment{
			{DeviceID: "dev-1", GroupIDs: []string{"dgrp-1"}, UpdatedAt: 100},
			{DeviceID: "dev-2", GroupIDs: []string{"dgrp-2"}, UpdatedAt: 200},
		},
	}
	networks := &networkAccessTestNetworks{
		networks: map[string]model.Network{
			"net-1": {NetworkID: "net-1", Name: "Default Network", Status: "active"},
		},
		networkDevices: map[string][]model.NetworkDevice{
			"net-1": {
				{NetworkID: "net-1", DeviceID: "dev-1", Enabled: true, MemberStatus: model.NetworkMemberStatusActive},
			},
		},
		groupReferences: map[string][]model.NetworkDeviceGroupReference{
			"net-1": {
				{NetworkID: "net-1", GroupID: "dgrp-1"},
				{NetworkID: "net-1", GroupID: "dgrp-2"},
			},
		},
	}
	svc := DeviceGroupService{
		Devices:  devices,
		Networks: networks,
	}

	view, err := svc.ListNetworkDeviceGroups(context.Background(), "net-1")
	if err != nil {
		t.Fatalf("ListNetworkDeviceGroups returned error: %v", err)
	}
	if len(view.Items) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(view.Items))
	}
	if len(view.Members) != 2 {
		t.Fatalf("expected 2 referenced group member assignments, got %d", len(view.Members))
	}
	if view.Members[0].GroupID != "dgrp-1" || view.Members[0].DeviceID != "dev-1" {
		t.Fatalf("unexpected member payload: %#v", view.Members[0])
	}
}

func TestNetworkDeviceGroupReferenceMaterializesMemberships(t *testing.T) {
	devicePublisher := &networkAccessTestDevicePublisher{}
	eventPublisher := &networkAccessTestBroadcaster{}
	devices := &networkAccessTestDevices{
		devices: map[string]model.Device{
			"dev-1": {DeviceID: "dev-1", Name: "Device 1", Status: "active"},
		},
		deviceGroups: map[string]model.DeviceGroup{
			"dgrp-1": {GroupID: "dgrp-1", Name: "Group 1"},
		},
		deviceGroupAssignments: []model.DeviceGroupAssignment{
			{DeviceID: "dev-1", GroupIDs: []string{"dgrp-1"}},
		},
	}
	networks := &networkAccessTestNetworks{
		networks: map[string]model.Network{
			"net-1": {NetworkID: "net-1", Name: "Network 1", Status: "active"},
		},
		networkDevices:  map[string][]model.NetworkDevice{},
		groupReferences: map[string][]model.NetworkDeviceGroupReference{},
	}
	service := DeviceGroupService{
		Devices:         devices,
		Networks:        networks,
		NetworkGroups:   networks,
		EventPublisher:  eventPublisher,
		DevicePublisher: devicePublisher,
		Now:             func() time.Time { return time.Unix(1700000000, 0) },
	}

	view, err := service.AddNetworkDeviceGroup(context.Background(), AddNetworkDeviceGroupInput{
		NetworkID: "net-1", GroupID: "dgrp-1",
	})
	if err != nil {
		t.Fatalf("AddNetworkDeviceGroup returned error: %v", err)
	}
	if len(view.Items) != 1 || len(networks.networkDevices["net-1"]) != 1 {
		t.Fatalf("expected referenced group and one materialized member, view=%+v members=%+v", view, networks.networkDevices["net-1"])
	}
	if got := devices.devices["dev-1"].VirtualIP; got != "10.0.1.1" {
		t.Fatalf("expected group materialization to allocate 10.0.1.1, got %q", got)
	}
	if len(devicePublisher.events) != 1 {
		t.Fatalf("expected one joined membership event, got %d", len(devicePublisher.events))
	}
	joined := devicePublisher.events[0].Payload
	if joined["operation"] != "joined" || joined["changedNetworkId"] != "net-1" {
		t.Fatalf("unexpected joined membership payload: %#v", joined)
	}
	if _, exists := joined["networkId"]; exists {
		t.Fatalf("joined membership payload must not duplicate changedNetworkId as networkId: %#v", joined)
	}
	if joined["membershipVersion"] == uint64(0) {
		t.Fatalf("expected joined membership version, payload=%#v", joined)
	}
	if len(eventPublisher.events) != 5 {
		t.Fatalf("expected five add events, got %d", len(eventPublisher.events))
	}
	if eventPublisher.events[1].EventType != NetworkEventMemberAdded || eventPublisher.events[4].EventType != NetworkEventDeviceGroupAdded {
		t.Fatalf("unexpected group add event sequence: %#v", eventPublisher.events)
	}

	view, err = service.RemoveNetworkDeviceGroup(context.Background(), RemoveNetworkDeviceGroupInput{
		NetworkID: "net-1", GroupID: "dgrp-1",
	})
	if err != nil {
		t.Fatalf("RemoveNetworkDeviceGroup returned error: %v", err)
	}
	if len(view.Items) != 0 || len(networks.networkDevices["net-1"]) != 0 {
		t.Fatalf("expected reference and materialized member removal, view=%+v members=%+v", view, networks.networkDevices["net-1"])
	}
	if len(devicePublisher.events) != 2 {
		t.Fatalf("expected joined and left membership events, got %d", len(devicePublisher.events))
	}
	left := devicePublisher.events[1].Payload
	if left["operation"] != "left" || left["changedNetworkId"] != "net-1" {
		t.Fatalf("unexpected left membership payload: %#v", left)
	}
	if _, exists := left["networkId"]; exists {
		t.Fatalf("left membership payload must not duplicate changedNetworkId as networkId: %#v", left)
	}
	if len(eventPublisher.events) != 10 {
		t.Fatalf("expected five add and five remove events, got %d", len(eventPublisher.events))
	}
	if eventPublisher.events[6].EventType != NetworkEventMemberRemoved || eventPublisher.events[9].EventType != NetworkEventDeviceGroupRemoved {
		t.Fatalf("unexpected group remove event sequence: %#v", eventPublisher.events[5:])
	}
}

func TestNetworkSnapshotAggregatesDeviceGroupMembers(t *testing.T) {
	devices := &networkAccessTestDevices{
		devices: map[string]model.Device{
			"dev-1": {DeviceID: "dev-1", Alias: "Device 1", VirtualIP: "10.0.1.1", Status: "active"},
			"dev-2": {DeviceID: "dev-2", Alias: "Device 2", VirtualIP: "10.0.1.2", Status: "active"},
		},
		deviceGroups: map[string]model.DeviceGroup{
			"dgrp-1": {GroupID: "dgrp-1", Name: "Development", UpdatedAt: 1700000000},
		},
		deviceGroupAssignments: []model.DeviceGroupAssignment{
			{DeviceID: "dev-1", GroupIDs: []string{"dgrp-1"}},
			{DeviceID: "dev-2", GroupIDs: []string{"dgrp-1"}},
		},
	}
	networks := &networkAccessTestNetworks{
		networks: map[string]model.Network{
			"net-1": {NetworkID: "net-1", Name: "Network 1", Status: "active"},
		},
		networkDevices: map[string][]model.NetworkDevice{
			"net-1": {
				{NetworkID: "net-1", DeviceID: "dev-1", DeviceGroupIDs: []string{"dgrp-1"}, Enabled: true, MemberStatus: model.NetworkMemberStatusActive},
				{NetworkID: "net-1", DeviceID: "dev-2", DeviceGroupIDs: []string{"dgrp-1"}, Enabled: true, MemberStatus: model.NetworkMemberStatusActive},
			},
		},
		groupReferences: map[string][]model.NetworkDeviceGroupReference{
			"net-1": {{NetworkID: "net-1", GroupID: "dgrp-1"}},
		},
	}

	groups, err := buildNetworkEventDeviceGroups(context.Background(), devices, networks, "net-1")
	if err != nil {
		t.Fatalf("buildNetworkEventDeviceGroups returned error: %v", err)
	}
	if len(groups) != 1 || groups[0].Name != "Development" {
		t.Fatalf("unexpected aggregated groups: %#v", groups)
	}
	if got := groups[0].MemberDeviceIDs; len(got) != 2 || got[0] != "dev-1" || got[1] != "dev-2" {
		t.Fatalf("unexpected aggregated group members: %#v", got)
	}
}

func TestNetworkEventIDIncludesPayloadIdentity(t *testing.T) {
	first := newNetworkEventEnvelope(
		NetworkEventMemberUpdated,
		"net-1",
		7,
		1700000000000,
		NetworkEventMemberRemovedPayload{DeviceID: "device-1"},
	)
	second := newNetworkEventEnvelope(
		NetworkEventMemberUpdated,
		"net-1",
		7,
		1700000000000,
		NetworkEventMemberRemovedPayload{DeviceID: "device-2"},
	)
	if first.EventID == second.EventID {
		t.Fatalf("events with different payloads must have different IDs: %q", first.EventID)
	}
}

func TestListNetworkDeviceGroupsRejectsMissingNetwork(t *testing.T) {
	svc := DeviceGroupService{
		Devices:  &networkAccessTestDevices{},
		Networks: &networkAccessTestNetworks{},
	}

	_, err := svc.ListNetworkDeviceGroups(context.Background(), "net-missing")
	if err == nil {
		t.Fatalf("expected ListNetworkDeviceGroups to fail for missing network")
	}
}

func TestCreateDeviceGroupPublishesNetworkChange(t *testing.T) {
	now := time.Unix(1700005000, 0)
	eventPublisher := &networkAccessTestBroadcaster{}
	devices := &networkAccessTestDevices{
		devices: map[string]model.Device{
			"dev-1": {DeviceID: "dev-1", Alias: "Device 1", VirtualIP: "10.0.0.2", Status: "active"},
		},
	}
	networks := &networkAccessTestNetworks{
		networks: map[string]model.Network{
			"net-1": {NetworkID: "net-1", Name: "Default", CIDR: "10.0.0.0/24", Status: "active"},
		},
		networkDevices: map[string][]model.NetworkDevice{
			"net-1": {
				{NetworkID: "net-1", DeviceID: "dev-1", Enabled: true, MemberStatus: model.NetworkMemberStatusActive},
			},
		},
	}
	svc := DeviceGroupService{
		Devices:        devices,
		Networks:       networks,
		EventPublisher: eventPublisher,
		Now:            func() time.Time { return now },
	}

	_, err := svc.CreateDeviceGroup(context.Background(), CreateDeviceGroupInput{
		Name: "Ops",
	})
	if err != nil {
		t.Fatalf("CreateDeviceGroup returned error: %v", err)
	}
	if len(eventPublisher.events) != 3 {
		t.Fatalf("expected 3 network events, got %d", len(eventPublisher.events))
	}
	if eventPublisher.events[0].EventType != NetworkEventConfigChanged {
		t.Fatalf("expected first event type %q, got %q", NetworkEventConfigChanged, eventPublisher.events[0].EventType)
	}
	if eventPublisher.events[1].EventType != NetworkEventACLChanged {
		t.Fatalf("expected second event type %q, got %q", NetworkEventACLChanged, eventPublisher.events[1].EventType)
	}
	if eventPublisher.events[2].EventType != NetworkEventSnapshot {
		t.Fatalf("expected third event type %q, got %q", NetworkEventSnapshot, eventPublisher.events[2].EventType)
	}
}

func TestSetDeviceGroupsPublishesImpactedNetworkChange(t *testing.T) {
	now := time.Unix(1700006000, 0)
	eventPublisher := &networkAccessTestBroadcaster{}
	devices := &networkAccessTestDevices{
		devices: map[string]model.Device{
			"dev-1": {DeviceID: "dev-1", Alias: "Device 1", VirtualIP: "10.0.0.2", Status: "active"},
		},
		deviceGroups: map[string]model.DeviceGroup{
			"dgrp-1": {GroupID: "dgrp-1", Name: "Group 1"},
		},
	}
	networks := &networkAccessTestNetworks{
		networks: map[string]model.Network{
			"net-1": {NetworkID: "net-1", Name: "Default", CIDR: "10.0.0.0/24", Status: "active"},
		},
		networkDevices: map[string][]model.NetworkDevice{
			"net-1": {
				{NetworkID: "net-1", DeviceID: "dev-1", Enabled: true, MemberStatus: model.NetworkMemberStatusActive},
			},
		},
		groupReferences: map[string][]model.NetworkDeviceGroupReference{
			"net-1": {{NetworkID: "net-1", GroupID: "dgrp-1"}},
		},
	}
	svc := DeviceGroupService{
		Devices:        devices,
		Networks:       networks,
		EventPublisher: eventPublisher,
		Now:            func() time.Time { return now },
	}

	err := svc.SetDeviceGroups(context.Background(), SetDeviceGroupsInput{
		DeviceID: "dev-1",
		GroupIDs: []string{"dgrp-1"},
	})
	if err != nil {
		t.Fatalf("SetDeviceGroups returned error: %v", err)
	}
	if len(eventPublisher.events) != 4 {
		t.Fatalf("expected 4 network events, got %d", len(eventPublisher.events))
	}
	if eventPublisher.events[0].EventType != NetworkEventConfigChanged {
		t.Fatalf("expected first event type %q, got %q", NetworkEventConfigChanged, eventPublisher.events[0].EventType)
	}
	if eventPublisher.events[1].EventType != NetworkEventMemberUpdated {
		t.Fatalf("expected second event type %q, got %q", NetworkEventMemberUpdated, eventPublisher.events[1].EventType)
	}
	if eventPublisher.events[2].EventType != NetworkEventACLChanged {
		t.Fatalf("expected third event type %q, got %q", NetworkEventACLChanged, eventPublisher.events[2].EventType)
	}
	if eventPublisher.events[3].EventType != NetworkEventSnapshot {
		t.Fatalf("expected fourth event type %q, got %q", NetworkEventSnapshot, eventPublisher.events[3].EventType)
	}
}

func TestSetDeviceGroupsCreatesPendingJoinDeliveryOnlyForMissingNetworkMembership(t *testing.T) {
	now := time.Unix(1700007000, 0)
	deliveries := &deliveryTestStore{items: map[string]model.NetworkEventDelivery{}}
	eventPublisher := &networkAccessDeliveryPublisher{
		tracker: &MqttNetworkEventPublisher{deliveries: deliveries},
	}
	devices := &networkAccessTestDevices{
		devices: map[string]model.Device{
			"dev-1": {DeviceID: "dev-1", VirtualIP: "10.0.1.10", Status: "active"},
		},
		deviceGroups: map[string]model.DeviceGroup{
			"dgrp-1": {GroupID: "dgrp-1", Name: "Development"},
		},
	}
	networks := &networkAccessTestNetworks{
		networks: map[string]model.Network{
			"net-1": {NetworkID: "net-1", Name: "Default", Status: "active"},
			"net-2": {NetworkID: "net-2", Name: "Unrelated", Status: "active"},
		},
		networkDevices: map[string][]model.NetworkDevice{},
		groupReferences: map[string][]model.NetworkDeviceGroupReference{
			"net-1": {{NetworkID: "net-1", GroupID: "dgrp-1"}},
		},
	}
	service := DeviceGroupService{
		Devices: devices, Networks: networks, NetworkGroups: networks,
		EventPublisher: eventPublisher, Now: func() time.Time { return now },
	}

	input := SetDeviceGroupsInput{DeviceID: "dev-1", GroupIDs: []string{" dgrp-1 ", "dgrp-1"}}
	if err := service.SetDeviceGroups(context.Background(), input); err != nil {
		t.Fatalf("SetDeviceGroups returned error: %v", err)
	}
	if len(networks.networkDevices["net-1"]) != 1 {
		t.Fatalf("expected device to join referenced network: %#v", networks.networkDevices["net-1"])
	}
	if len(networks.networkDevices["net-2"]) != 0 || networks.versions["net-2"].Version != 0 {
		t.Fatalf("unrelated network must not be synchronized: members=%#v version=%+v", networks.networkDevices["net-2"], networks.versions["net-2"])
	}
	if len(deliveries.items) != 1 {
		t.Fatalf("expected one pending join delivery, got %d", len(deliveries.items))
	}
	for _, item := range deliveries.items {
		if item.TargetDeviceID != "dev-1" || item.NetworkID != "net-1" || item.Status != "pending" {
			t.Fatalf("unexpected pending join delivery: %+v", item)
		}
	}

	if err := service.SetDeviceGroups(context.Background(), input); err != nil {
		t.Fatalf("repeated SetDeviceGroups returned error: %v", err)
	}
	if len(deliveries.items) != 1 {
		t.Fatalf("existing network membership must not create another delivery, got %d", len(deliveries.items))
	}
}

func TestSetDeviceGroupsAssignsExistingDevice(t *testing.T) {
	devices := &networkAccessTestDevices{
		devices: map[string]model.Device{
			"shared-device": {DeviceID: "shared-device", Status: "active"},
		},
		deviceGroups: map[string]model.DeviceGroup{
			"shared-group": {GroupID: "shared-group", Name: "Development"},
		},
	}
	networks := &networkAccessTestNetworks{networks: map[string]model.Network{}}

	err := (DeviceGroupService{
		Devices: devices, Networks: networks,
	}).SetDeviceGroups(context.Background(), SetDeviceGroupsInput{
		DeviceID: "shared-device",
		GroupIDs: []string{"shared-group"},
	})
	if err != nil {
		t.Fatalf("SetDeviceGroups returned error for shared device: %v", err)
	}
	if len(devices.deviceGroupAssignments) != 1 {
		t.Fatalf("expected one device assignment, got %d", len(devices.deviceGroupAssignments))
	}
	assignment := devices.deviceGroupAssignments[0]
	if assignment.DeviceID != "shared-device" {
		t.Fatalf("unexpected device assignment: %#v", assignment)
	}
}
