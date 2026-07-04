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
	deviceGroups map[string]model.DeviceGroup
}

func (s *networkAccessTestDevices) ListDevicesByOwner(context.Context, string) ([]model.Device, error) {
	return nil, nil
}

func (s *networkAccessTestDevices) GetDevice(context.Context, string) (model.Device, bool, error) {
	return model.Device{}, false, nil
}

func (s *networkAccessTestDevices) SaveDevice(context.Context, model.Device) error { return nil }
func (s *networkAccessTestDevices) DeleteDevice(context.Context, string) error      { return nil }
func (s *networkAccessTestDevices) NewDeviceVirtualIPID() string                    { return "vip-test-1" }
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
	return nil, nil
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
	return nil, nil
}

type networkAccessTestNetworks struct {
	networks      map[string]model.Network
	securityGroup map[string]model.SecurityGroup
	securityRules map[string]model.SecurityRule
	versions      map[string]model.NetworkConfigVersion
}

func (s *networkAccessTestNetworks) GetNetwork(_ context.Context, networkID string) (model.Network, bool, error) {
	item, ok := s.networks[networkID]
	return item, ok, nil
}

func (s *networkAccessTestNetworks) ListNetworksByOwner(context.Context, string) ([]model.Network, error) {
	return nil, nil
}

func (s *networkAccessTestNetworks) ListNetworksByDevice(context.Context, string) ([]model.Network, error) {
	return nil, nil
}

func (s *networkAccessTestNetworks) SaveNetwork(_ context.Context, item model.Network) error {
	if s.networks == nil {
		s.networks = make(map[string]model.Network)
	}
	s.networks[item.NetworkID] = item
	return nil
}

func (s *networkAccessTestNetworks) DeleteNetwork(context.Context, string) error { return nil }
func (s *networkAccessTestNetworks) ListNetworkDevices(context.Context, string) ([]model.NetworkDevice, error) {
	return nil, nil
}
func (s *networkAccessTestNetworks) GetNetworkDevice(context.Context, string, string) (model.NetworkDevice, bool, error) {
	return model.NetworkDevice{}, false, nil
}
func (s *networkAccessTestNetworks) SaveNetworkDevice(context.Context, model.NetworkDevice) error { return nil }
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
func (s *networkAccessTestNetworks) DeleteNetworkDevice(context.Context, string, string) error { return nil }
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
func (s *networkAccessTestNetworks) SaveDeviceInvite(context.Context, model.DeviceInvite) error { return nil }
func (s *networkAccessTestNetworks) ListDNSZones(context.Context, string) ([]model.DNSZone, error) {
	return nil, nil
}
func (s *networkAccessTestNetworks) GetDNSZone(context.Context, string) (model.DNSZone, bool, error) {
	return model.DNSZone{}, false, nil
}
func (s *networkAccessTestNetworks) SaveDNSZone(context.Context, model.DNSZone) error { return nil }
func (s *networkAccessTestNetworks) DeleteDNSZone(context.Context, string) error       { return nil }
func (s *networkAccessTestNetworks) ListDNSRecords(context.Context, string) ([]model.DNSRecord, error) {
	return nil, nil
}
func (s *networkAccessTestNetworks) GetDNSRecord(context.Context, string) (model.DNSRecord, bool, error) {
	return model.DNSRecord{}, false, nil
}
func (s *networkAccessTestNetworks) SaveDNSRecord(context.Context, model.DNSRecord) error { return nil }
func (s *networkAccessTestNetworks) DeleteDNSRecord(context.Context, string) error         { return nil }
func (s *networkAccessTestNetworks) ListPublicMappings(context.Context, string) ([]model.PublicMapping, error) {
	return nil, nil
}
func (s *networkAccessTestNetworks) GetPublicMapping(context.Context, string) (model.PublicMapping, bool, error) {
	return model.PublicMapping{}, false, nil
}
func (s *networkAccessTestNetworks) SavePublicMapping(context.Context, model.PublicMapping) error { return nil }
func (s *networkAccessTestNetworks) DeletePublicMapping(context.Context, string) error             { return nil }
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
	}
	service := NetworkAccessService{
		Users:     users,
		Devices:   devices,
		Networks:  networks,
		Now:       func() time.Time { return time.Unix(1700000000, 0) },
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
