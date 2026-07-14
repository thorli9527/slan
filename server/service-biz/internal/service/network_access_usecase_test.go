package service

import (
	"context"
	"testing"
	"time"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

type networkAccessTestUsers struct {
	users map[string]model.User
}

func (s *networkAccessTestUsers) GetByEmail(context.Context, string) (model.User, bool, error) {
	return model.User{}, false, nil
}

func (s *networkAccessTestUsers) GetUser(_ context.Context, userID string) (model.User, bool, error) {
	item, ok := s.users[userID]
	return item, ok, nil
}

func (s *networkAccessTestUsers) SaveUser(_ context.Context, item model.User) error {
	if s.users == nil {
		s.users = make(map[string]model.User)
	}
	s.users[item.UserID] = item
	return nil
}

func (s *networkAccessTestUsers) ListUsers(context.Context) ([]model.User, error) {
	return nil, nil
}

type networkAccessTestDevices struct {
	devices                map[string]model.Device
	deviceGroups           map[string]model.DeviceGroup
	deviceGroupAssignments []model.DeviceGroupAssignment
}

func (s *networkAccessTestDevices) ListDevicesByOwner(_ context.Context, ownerID string) ([]model.Device, error) {
	items := make([]model.Device, 0)
	for _, item := range s.devices {
		if item.OwnerID == ownerID {
			items = append(items, item)
		}
	}
	return items, nil
}

func (s *networkAccessTestDevices) GetDevice(_ context.Context, deviceID string) (model.Device, bool, error) {
	item, ok := s.devices[deviceID]
	return item, ok, nil
}

func (s *networkAccessTestDevices) SaveDevice(context.Context, model.Device) error { return nil }
func (s *networkAccessTestDevices) DeleteDevice(context.Context, string) error     { return nil }
func (s *networkAccessTestDevices) NewDeviceVirtualIPID() string                   { return "vip-test-1" }
func (s *networkAccessTestDevices) GetDeviceLoginDevice(context.Context, string) (model.DeviceLoginDevice, bool, error) {
	return model.DeviceLoginDevice{}, false, nil
}
func (s *networkAccessTestDevices) SaveDeviceLoginDevice(context.Context, model.DeviceLoginDevice) error {
	return nil
}
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
func (s *networkAccessTestDevices) DeleteDeviceSessionByAccessToken(context.Context, string) error {
	return nil
}
func (s *networkAccessTestDevices) ListDeviceBootstrapKeys(context.Context, string) ([]model.DeviceBootstrapKey, error) {
	return nil, nil
}
func (s *networkAccessTestDevices) GetDeviceBootstrapKey(context.Context, string) (model.DeviceBootstrapKey, bool, error) {
	return model.DeviceBootstrapKey{}, false, nil
}
func (s *networkAccessTestDevices) GetDeviceBootstrapKeyByToken(context.Context, string) (model.DeviceBootstrapKey, bool, error) {
	return model.DeviceBootstrapKey{}, false, nil
}
func (s *networkAccessTestDevices) SaveDeviceBootstrapKey(context.Context, model.DeviceBootstrapKey) error {
	return nil
}
func (s *networkAccessTestDevices) ListDeviceGroups(context.Context, string) ([]model.DeviceGroup, error) {
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
func (s *networkAccessTestDevices) SetDeviceGroups(context.Context, model.DeviceGroupAssignment) error {
	return nil
}
func (s *networkAccessTestDevices) ListDeviceGroupAssignments(context.Context, string) ([]model.DeviceGroupAssignment, error) {
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

func (s *networkAccessTestNetworks) ListNetworksByOwner(_ context.Context, ownerID string) ([]model.Network, error) {
	items := make([]model.Network, 0)
	for _, item := range s.networks {
		if item.OwnerID == ownerID {
			items = append(items, item)
		}
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
func (s *networkAccessTestNetworks) GetNetworkDevice(context.Context, string, string) (model.NetworkDevice, bool, error) {
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
func (s *networkAccessTestNetworks) ListDeviceInvitesByUser(context.Context, string) ([]model.DeviceInvite, error) {
	return nil, nil
}
func (s *networkAccessTestNetworks) ListDeviceInvitesByNetwork(context.Context, string) ([]model.DeviceInvite, error) {
	return nil, nil
}
func (s *networkAccessTestNetworks) GetDeviceInvite(context.Context, string) (model.DeviceInvite, bool, error) {
	return model.DeviceInvite{}, false, nil
}
func (s *networkAccessTestNetworks) GetDeviceInviteByCode(context.Context, string) (model.DeviceInvite, bool, error) {
	return model.DeviceInvite{}, false, nil
}
func (s *networkAccessTestNetworks) SaveDeviceInvite(context.Context, model.DeviceInvite) error {
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

var _ repository.UserRepository = (*networkAccessTestUsers)(nil)
var _ repository.DeviceRepository = (*networkAccessTestDevices)(nil)
var _ repository.NetworkRepository = (*networkAccessTestNetworks)(nil)

type networkAccessTestBroadcaster struct {
	events []NetworkEventEnvelope
}

func (s *networkAccessTestBroadcaster) PublishNetworkEvent(_ context.Context, event NetworkEventEnvelope) error {
	s.events = append(s.events, event)
	return nil
}

func TestAddSecurityRuleAllowsOwnedDeviceGroupPeer(t *testing.T) {
	users := &networkAccessTestUsers{
		users: map[string]model.User{
			"user-1": {UserID: "user-1", Email: "u@example.com", Status: "active"},
		},
	}
	devices := &networkAccessTestDevices{
		deviceGroups: map[string]model.DeviceGroup{
			"dgrp-1": {GroupID: "dgrp-1", UserID: "user-1", Name: "Group 1"},
		},
	}
	networks := &networkAccessTestNetworks{
		networks: map[string]model.Network{
			"net-1": {NetworkID: "net-1", OwnerID: "user-1", Name: "Default Network", Status: "active"},
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
		{UserID: "user-1", DeviceID: "dev-1", GroupIDs: []string{"dgrp-1"}},
	}
	service := NetworkAccessService{
		Users:    users,
		Devices:  devices,
		Networks: networks,
		Now:      func() time.Time { return time.Unix(1700000000, 0) },
		NewSecurityRuleID: func() string {
			return "sgr-1"
		},
	}

	view, err := service.AddSecurityRule(context.Background(), CreateSecurityRuleInput{
		SecurityGroupID: "sg-1",
		ActorUserID:     "user-1",
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

func TestAddSecurityRuleAllowsCurrentNetworkDevicePeer(t *testing.T) {
	users := &networkAccessTestUsers{users: map[string]model.User{
		"user-1": {UserID: "user-1", Email: "u@example.com", Status: "active"},
	}}
	devices := &networkAccessTestDevices{devices: map[string]model.Device{
		"dev-1": {DeviceID: "dev-1", OwnerID: "user-1", Name: "Device 1", Status: "active"},
	}}
	networks := &networkAccessTestNetworks{
		networks: map[string]model.Network{
			"net-1": {NetworkID: "net-1", OwnerID: "user-1", Name: "Default Network", Status: "active"},
		},
		securityGroup: map[string]model.SecurityGroup{
			"sg-1": {SecurityGroupID: "sg-1", NetworkID: "net-1", Name: "Default Security Group"},
		},
		networkDevices: map[string][]model.NetworkDevice{
			"net-1": {{NetworkID: "net-1", DeviceID: "dev-1", Enabled: true, MemberStatus: model.NetworkMemberStatusActive}},
		},
	}
	service := NetworkAccessService{
		Users: users, Devices: devices, Networks: networks,
		Now:               func() time.Time { return time.Unix(1700000000, 0) },
		NewSecurityRuleID: func() string { return "sgr-device-1" },
	}

	view, err := service.AddSecurityRule(context.Background(), CreateSecurityRuleInput{
		SecurityGroupID: "sg-1",
		ActorUserID:     "user-1",
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
	users := &networkAccessTestUsers{
		users: map[string]model.User{
			"user-1": {UserID: "user-1", Email: "u@example.com", Status: "active"},
		},
	}
	devices := &networkAccessTestDevices{}
	networks := &networkAccessTestNetworks{
		networks: map[string]model.Network{
			"net-1": {NetworkID: "net-1", OwnerID: "user-1", Name: "Default Network", Status: "active"},
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
		Users:    users,
		Devices:  devices,
		Networks: networks,
		Now:      func() time.Time { return time.Unix(1700000000, 0) },
		NewSecurityRuleID: func() string {
			return "sgr-1"
		},
	}

	_, err := service.AddSecurityRule(context.Background(), CreateSecurityRuleInput{
		SecurityGroupID: "sg-1",
		ActorUserID:     "user-1",
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
	users := &networkAccessTestUsers{
		users: map[string]model.User{
			"user-1": {UserID: "user-1", Email: "u@example.com", Status: "active"},
		},
	}
	devices := &networkAccessTestDevices{
		deviceGroups: map[string]model.DeviceGroup{
			"dgrp-1": {GroupID: "dgrp-1", UserID: "user-1", Name: "Group 1"},
		},
		deviceGroupAssignments: []model.DeviceGroupAssignment{
			{UserID: "user-1", DeviceID: "dev-outside", GroupIDs: []string{"dgrp-1"}},
		},
	}
	networks := &networkAccessTestNetworks{
		networks: map[string]model.Network{
			"net-1": {NetworkID: "net-1", OwnerID: "user-1", Name: "Default Network", Status: "active"},
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
		Users:    users,
		Devices:  devices,
		Networks: networks,
		Now:      func() time.Time { return time.Unix(1700000000, 0) },
		NewSecurityRuleID: func() string {
			return "sgr-1"
		},
	}

	_, err := service.AddSecurityRule(context.Background(), CreateSecurityRuleInput{
		SecurityGroupID: "sg-1",
		ActorUserID:     "user-1",
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
	users := &networkAccessTestUsers{
		users: map[string]model.User{
			"user-1": {UserID: "user-1", Email: "u@example.com", Status: "active"},
		},
	}
	devices := &networkAccessTestDevices{
		deviceGroups: map[string]model.DeviceGroup{
			"dgrp-1": {GroupID: "dgrp-1", UserID: "user-1", Name: "Group 1"},
			"dgrp-2": {GroupID: "dgrp-2", UserID: "user-1", Name: "Group 2"},
		},
		deviceGroupAssignments: []model.DeviceGroupAssignment{
			{UserID: "user-1", DeviceID: "dev-1", GroupIDs: []string{"dgrp-1"}, UpdatedAt: 100},
			{UserID: "user-1", DeviceID: "dev-2", GroupIDs: []string{"dgrp-2"}, UpdatedAt: 200},
		},
	}
	networks := &networkAccessTestNetworks{
		networks: map[string]model.Network{
			"net-1": {NetworkID: "net-1", OwnerID: "user-1", Name: "Default Network", Status: "active"},
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
		Users:    users,
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
	users := &networkAccessTestUsers{users: map[string]model.User{
		"user-1": {UserID: "user-1", Email: "u@example.com", Status: "active"},
	}}
	devices := &networkAccessTestDevices{
		devices: map[string]model.Device{
			"dev-1": {DeviceID: "dev-1", OwnerID: "user-1", Name: "Device 1", Status: "active"},
		},
		deviceGroups: map[string]model.DeviceGroup{
			"dgrp-1": {GroupID: "dgrp-1", UserID: "user-1", Name: "Group 1"},
		},
		deviceGroupAssignments: []model.DeviceGroupAssignment{
			{UserID: "user-1", DeviceID: "dev-1", GroupIDs: []string{"dgrp-1"}},
		},
	}
	networks := &networkAccessTestNetworks{
		networks: map[string]model.Network{
			"net-1": {NetworkID: "net-1", OwnerID: "user-1", Name: "Network 1", Status: "active"},
		},
		networkDevices:  map[string][]model.NetworkDevice{},
		groupReferences: map[string][]model.NetworkDeviceGroupReference{},
	}
	service := DeviceGroupService{
		Users:         users,
		Devices:       devices,
		Networks:      networks,
		NetworkGroups: networks,
		Now:           func() time.Time { return time.Unix(1700000000, 0) },
	}

	view, err := service.AddNetworkDeviceGroup(context.Background(), AddNetworkDeviceGroupInput{
		NetworkID: "net-1", GroupID: "dgrp-1", ActorUserID: "user-1",
	})
	if err != nil {
		t.Fatalf("AddNetworkDeviceGroup returned error: %v", err)
	}
	if len(view.Items) != 1 || len(networks.networkDevices["net-1"]) != 1 {
		t.Fatalf("expected referenced group and one materialized member, view=%+v members=%+v", view, networks.networkDevices["net-1"])
	}

	view, err = service.RemoveNetworkDeviceGroup(context.Background(), RemoveNetworkDeviceGroupInput{
		NetworkID: "net-1", GroupID: "dgrp-1", ActorUserID: "user-1",
	})
	if err != nil {
		t.Fatalf("RemoveNetworkDeviceGroup returned error: %v", err)
	}
	if len(view.Items) != 0 || len(networks.networkDevices["net-1"]) != 0 {
		t.Fatalf("expected reference and materialized member removal, view=%+v members=%+v", view, networks.networkDevices["net-1"])
	}
}

func TestListNetworkDeviceGroupsRejectsMissingNetwork(t *testing.T) {
	svc := DeviceGroupService{
		Users:    &networkAccessTestUsers{},
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
	users := &networkAccessTestUsers{
		users: map[string]model.User{
			"user-1": {UserID: "user-1", Email: "u@example.com", Status: "active"},
		},
	}
	devices := &networkAccessTestDevices{
		devices: map[string]model.Device{
			"dev-1": {DeviceID: "dev-1", OwnerID: "user-1", Alias: "Device 1", VirtualIP: "10.0.0.2", Status: "active"},
		},
	}
	networks := &networkAccessTestNetworks{
		networks: map[string]model.Network{
			"net-1": {NetworkID: "net-1", OwnerID: "user-1", Name: "Default", CIDR: "10.0.0.0/24", Status: "active"},
		},
		networkDevices: map[string][]model.NetworkDevice{
			"net-1": {
				{NetworkID: "net-1", DeviceID: "dev-1", Enabled: true, MemberStatus: model.NetworkMemberStatusActive},
			},
		},
	}
	svc := DeviceGroupService{
		Users:          users,
		Devices:        devices,
		Networks:       networks,
		EventPublisher: eventPublisher,
		Now:            func() time.Time { return now },
	}

	_, err := svc.CreateDeviceGroup(context.Background(), CreateDeviceGroupInput{
		UserID:      "user-1",
		ActorUserID: "user-1",
		Name:        "Ops",
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
	users := &networkAccessTestUsers{
		users: map[string]model.User{
			"user-1": {UserID: "user-1", Email: "u@example.com", Status: "active"},
		},
	}
	devices := &networkAccessTestDevices{
		devices: map[string]model.Device{
			"dev-1": {DeviceID: "dev-1", OwnerID: "user-1", Alias: "Device 1", VirtualIP: "10.0.0.2", Status: "active"},
		},
		deviceGroups: map[string]model.DeviceGroup{
			"dgrp-1": {GroupID: "dgrp-1", UserID: "user-1", Name: "Group 1"},
		},
	}
	networks := &networkAccessTestNetworks{
		networks: map[string]model.Network{
			"net-1": {NetworkID: "net-1", OwnerID: "user-1", Name: "Default", CIDR: "10.0.0.0/24", Status: "active"},
		},
		networkDevices: map[string][]model.NetworkDevice{
			"net-1": {
				{NetworkID: "net-1", DeviceID: "dev-1", Enabled: true, MemberStatus: model.NetworkMemberStatusActive},
			},
		},
	}
	svc := DeviceGroupService{
		Users:          users,
		Devices:        devices,
		Networks:       networks,
		EventPublisher: eventPublisher,
		Now:            func() time.Time { return now },
	}

	err := svc.SetDeviceGroups(context.Background(), SetDeviceGroupsInput{
		UserID:      "user-1",
		ActorUserID: "user-1",
		DeviceID:    "dev-1",
		GroupIDs:    []string{"dgrp-1"},
	})
	if err != nil {
		t.Fatalf("SetDeviceGroups returned error: %v", err)
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
