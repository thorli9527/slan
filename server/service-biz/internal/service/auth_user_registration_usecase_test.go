package service

import (
	"context"
	"testing"
	"time"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

type authUserRegistrationTestUsers struct {
	byEmail map[string]model.User
	byID    map[string]model.User
}

func (s *authUserRegistrationTestUsers) GetByEmail(_ context.Context, email string) (model.User, bool, error) {
	item, ok := s.byEmail[email]
	return item, ok, nil
}

func (s *authUserRegistrationTestUsers) GetUser(_ context.Context, userID string) (model.User, bool, error) {
	item, ok := s.byID[userID]
	return item, ok, nil
}

func (s *authUserRegistrationTestUsers) SaveUser(_ context.Context, item model.User) error {
	if s.byEmail == nil {
		s.byEmail = make(map[string]model.User)
	}
	if s.byID == nil {
		s.byID = make(map[string]model.User)
	}
	s.byEmail[item.Email] = item
	s.byID[item.UserID] = item
	return nil
}

func (s *authUserRegistrationTestUsers) ListUsers(_ context.Context) ([]model.User, error) {
	out := make([]model.User, 0, len(s.byID))
	for _, item := range s.byID {
		out = append(out, item)
	}
	return out, nil
}

type authUserRegistrationTestSessions struct {
	items []model.UserSession
}

func (s *authUserRegistrationTestSessions) GetUserSessionByAccessToken(context.Context, string) (model.UserSession, bool, error) {
	return model.UserSession{}, false, nil
}

func (s *authUserRegistrationTestSessions) GetUserSessionByRefreshToken(context.Context, string) (model.UserSession, bool, error) {
	return model.UserSession{}, false, nil
}

func (s *authUserRegistrationTestSessions) ListUserSessionsByUserID(context.Context, string) ([]model.UserSession, error) {
	return nil, nil
}

func (s *authUserRegistrationTestSessions) SaveUserSession(_ context.Context, item model.UserSession) error {
	s.items = append(s.items, item)
	return nil
}

func (s *authUserRegistrationTestSessions) ReplaceUserSessionForClient(_ context.Context, item model.UserSession) error {
	s.items = append(s.items, item)
	return nil
}

func (s *authUserRegistrationTestSessions) ReplaceUserSession(_ context.Context, _ string, item model.UserSession) error {
	s.items = append(s.items, item)
	return nil
}

func (s *authUserRegistrationTestSessions) DeleteUserSessionByAccessToken(context.Context, string) error {
	return nil
}

func (s *authUserRegistrationTestSessions) GetConsoleLoginKeyByKey(context.Context, string) (model.ConsoleLoginKey, bool, error) {
	return model.ConsoleLoginKey{}, false, nil
}

func (s *authUserRegistrationTestSessions) SaveConsoleLoginKey(context.Context, model.ConsoleLoginKey) error {
	return nil
}

type authUserRegistrationTestNetworks struct {
	networks       map[string]model.Network
	securityGroups map[string]model.SecurityGroup
	securityRules  map[string]model.SecurityRule
	groupRefs      map[string][]model.NetworkDeviceGroupReference
	versions       map[string]model.NetworkConfigVersion
	nextSecurityID int
	nextRuleID     int
}

func (s *authUserRegistrationTestNetworks) GetNetwork(_ context.Context, networkID string) (model.Network, bool, error) {
	item, ok := s.networks[networkID]
	return item, ok, nil
}

func (s *authUserRegistrationTestNetworks) ListNetworksByOwner(_ context.Context, ownerID string) ([]model.Network, error) {
	out := []model.Network{}
	for _, item := range s.networks {
		if item.OwnerID == ownerID {
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *authUserRegistrationTestNetworks) ListNetworksByDevice(context.Context, string) ([]model.Network, error) {
	return nil, nil
}

func (s *authUserRegistrationTestNetworks) SaveNetwork(_ context.Context, item model.Network) error {
	if s.networks == nil {
		s.networks = make(map[string]model.Network)
	}
	s.networks[item.NetworkID] = item
	return nil
}

func (s *authUserRegistrationTestNetworks) DeleteNetwork(context.Context, string) error {
	return nil
}

func (s *authUserRegistrationTestNetworks) ListNetworkDevices(context.Context, string) ([]model.NetworkDevice, error) {
	return nil, nil
}

func (s *authUserRegistrationTestNetworks) GetNetworkDevice(_ context.Context, networkID, deviceID string) (model.NetworkDevice, bool, error) {
	return model.NetworkDevice{}, false, nil
}

func (s *authUserRegistrationTestNetworks) SaveNetworkDevice(context.Context, model.NetworkDevice) error {
	return nil
}

func (s *authUserRegistrationTestNetworks) GetNetworkVersion(_ context.Context, networkID string) (model.NetworkConfigVersion, bool, error) {
	item, ok := s.versions[networkID]
	return item, ok, nil
}

func (s *authUserRegistrationTestNetworks) SaveNetworkVersion(_ context.Context, item model.NetworkConfigVersion) error {
	if s.versions == nil {
		s.versions = make(map[string]model.NetworkConfigVersion)
	}
	s.versions[item.NetworkID] = item
	return nil
}

func (s *authUserRegistrationTestNetworks) DeleteNetworkDevice(context.Context, string, string) error {
	return nil
}

func (s *authUserRegistrationTestNetworks) ListDeviceInvitesByUser(context.Context, string) ([]model.DeviceInvite, error) {
	return nil, nil
}

func (s *authUserRegistrationTestNetworks) ListDeviceInvitesByNetwork(context.Context, string) ([]model.DeviceInvite, error) {
	return nil, nil
}

func (s *authUserRegistrationTestNetworks) GetDeviceInvite(context.Context, string) (model.DeviceInvite, bool, error) {
	return model.DeviceInvite{}, false, nil
}

func (s *authUserRegistrationTestNetworks) GetDeviceInviteByCode(context.Context, string) (model.DeviceInvite, bool, error) {
	return model.DeviceInvite{}, false, nil
}

func (s *authUserRegistrationTestNetworks) SaveDeviceInvite(context.Context, model.DeviceInvite) error {
	return nil
}

func (s *authUserRegistrationTestNetworks) ListDNSZones(context.Context, string) ([]model.DNSZone, error) {
	return nil, nil
}

func (s *authUserRegistrationTestNetworks) GetDNSZone(context.Context, string) (model.DNSZone, bool, error) {
	return model.DNSZone{}, false, nil
}

func (s *authUserRegistrationTestNetworks) SaveDNSZone(context.Context, model.DNSZone) error {
	return nil
}

func (s *authUserRegistrationTestNetworks) DeleteDNSZone(context.Context, string) error {
	return nil
}

func (s *authUserRegistrationTestNetworks) ListDNSRecords(context.Context, string) ([]model.DNSRecord, error) {
	return nil, nil
}

func (s *authUserRegistrationTestNetworks) GetDNSRecord(context.Context, string) (model.DNSRecord, bool, error) {
	return model.DNSRecord{}, false, nil
}

func (s *authUserRegistrationTestNetworks) SaveDNSRecord(context.Context, model.DNSRecord) error {
	return nil
}

func (s *authUserRegistrationTestNetworks) DeleteDNSRecord(context.Context, string) error {
	return nil
}

func (s *authUserRegistrationTestNetworks) ListSecurityGroups(_ context.Context, networkID string) ([]model.SecurityGroup, error) {
	out := []model.SecurityGroup{}
	for _, item := range s.securityGroups {
		if item.NetworkID == networkID {
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *authUserRegistrationTestNetworks) GetSecurityGroup(_ context.Context, securityGroupID string) (model.SecurityGroup, bool, error) {
	item, ok := s.securityGroups[securityGroupID]
	return item, ok, nil
}

func (s *authUserRegistrationTestNetworks) SaveSecurityGroup(_ context.Context, item model.SecurityGroup) error {
	if s.securityGroups == nil {
		s.securityGroups = make(map[string]model.SecurityGroup)
	}
	s.securityGroups[item.SecurityGroupID] = item
	return nil
}

func (s *authUserRegistrationTestNetworks) DeleteSecurityGroup(context.Context, string) error {
	return nil
}

func (s *authUserRegistrationTestNetworks) ListSecurityRules(_ context.Context, securityGroupID string) ([]model.SecurityRule, error) {
	out := []model.SecurityRule{}
	for _, item := range s.securityRules {
		if item.SecurityGroupID == securityGroupID {
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *authUserRegistrationTestNetworks) GetSecurityRule(_ context.Context, ruleID string) (model.SecurityRule, bool, error) {
	item, ok := s.securityRules[ruleID]
	return item, ok, nil
}

func (s *authUserRegistrationTestNetworks) SaveSecurityRule(_ context.Context, item model.SecurityRule) error {
	if s.securityRules == nil {
		s.securityRules = make(map[string]model.SecurityRule)
	}
	s.securityRules[item.RuleID] = item
	return nil
}

func (s *authUserRegistrationTestNetworks) DeleteSecurityRule(context.Context, string) error {
	return nil
}

func (s *authUserRegistrationTestNetworks) NewSecurityGroupID() string {
	s.nextSecurityID++
	return "sg-test-" + string(rune('0'+s.nextSecurityID))
}

func (s *authUserRegistrationTestNetworks) NewSecurityRuleID() string {
	s.nextRuleID++
	return "sgr-test-" + string(rune('0'+s.nextRuleID))
}

func (s *authUserRegistrationTestNetworks) ListNetworkDeviceGroupReferences(_ context.Context, networkID string) ([]model.NetworkDeviceGroupReference, error) {
	return append([]model.NetworkDeviceGroupReference(nil), s.groupRefs[networkID]...), nil
}

func (s *authUserRegistrationTestNetworks) SaveNetworkDeviceGroupReference(_ context.Context, item model.NetworkDeviceGroupReference) error {
	if s.groupRefs == nil {
		s.groupRefs = make(map[string][]model.NetworkDeviceGroupReference)
	}
	s.groupRefs[item.NetworkID] = append(s.groupRefs[item.NetworkID], item)
	return nil
}

func (s *authUserRegistrationTestNetworks) DeleteNetworkDeviceGroupReference(context.Context, string, string) error {
	return nil
}

func (s *authUserRegistrationTestNetworks) DeleteNetworkDeviceGroupReferencesByGroup(context.Context, string) error {
	return nil
}

type authUserRegistrationTestDevices struct {
	networkRuntimeTestDevices
	groups      map[string]model.DeviceGroup
	assignments map[string]model.DeviceGroupAssignment
	nextGroupID int
}

func (s *authUserRegistrationTestDevices) ListDeviceGroups(_ context.Context, userID string) ([]model.DeviceGroup, error) {
	out := []model.DeviceGroup{}
	for _, item := range s.groups {
		if item.UserID == userID {
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *authUserRegistrationTestDevices) GetDeviceGroup(_ context.Context, groupID string) (model.DeviceGroup, bool, error) {
	item, ok := s.groups[groupID]
	return item, ok, nil
}

func (s *authUserRegistrationTestDevices) SaveDeviceGroup(_ context.Context, item model.DeviceGroup) error {
	if s.groups == nil {
		s.groups = make(map[string]model.DeviceGroup)
	}
	s.groups[item.GroupID] = item
	return nil
}

func (s *authUserRegistrationTestDevices) SetDeviceGroups(_ context.Context, item model.DeviceGroupAssignment) error {
	if s.assignments == nil {
		s.assignments = make(map[string]model.DeviceGroupAssignment)
	}
	s.assignments[item.DeviceID] = item
	return nil
}

func (s *authUserRegistrationTestDevices) ListDeviceGroupAssignments(_ context.Context, userID string) ([]model.DeviceGroupAssignment, error) {
	out := []model.DeviceGroupAssignment{}
	for _, item := range s.assignments {
		if item.UserID == userID {
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *authUserRegistrationTestDevices) NewDeviceGroupID() string {
	s.nextGroupID++
	return "dgrp-test-" + string(rune('0'+s.nextGroupID))
}

var _ repository.NetworkRepository = (*authUserRegistrationTestNetworks)(nil)
var _ repository.UserRepository = (*authUserRegistrationTestUsers)(nil)
var _ repository.UserSessionRepository = (*authUserRegistrationTestSessions)(nil)

func TestRegisterUserDoesNotCreateNetworkResources(t *testing.T) {
	networks := &authUserRegistrationTestNetworks{}
	devices := &authUserRegistrationTestDevices{}
	service := AuthUserRegistrationService{
		authUserDependencies: authUserDependencies{
			Users:     &authUserRegistrationTestUsers{},
			Sessions:  &authUserRegistrationTestSessions{},
			Devices:   devices,
			Networks:  networks,
			NewUserID: func() string { return "user-test-1" },
			NewSessID: func(string) string { return "sess-test-1" },
			Now: func() time.Time {
				return time.Unix(1700000000, 0)
			},
		},
	}

	view, err := service.RegisterUser(context.Background(), RegisterUserInput{
		Email:    "smoke@example.com",
		Password: "Password123!",
		Name:     "Smoke",
	})
	if err != nil {
		t.Fatalf("RegisterUser returned error: %v", err)
	}
	if view.User.UserID == "" {
		t.Fatalf("RegisterUser missing user view: %+v", view)
	}

	deviceGroups, err := devices.ListDeviceGroups(context.Background(), "user-test-1")
	if err != nil {
		t.Fatalf("ListDeviceGroups returned error: %v", err)
	}
	if len(deviceGroups) != 0 {
		t.Fatalf("registration created device groups: %+v", deviceGroups)
	}
	if len(networks.networks) != 0 {
		t.Fatalf("registration created networks: %+v", networks.networks)
	}
	if len(networks.groupRefs) != 0 {
		t.Fatalf("registration created network group references: %+v", networks.groupRefs)
	}
	if len(networks.securityGroups) != 0 {
		t.Fatalf("registration created security groups: %+v", networks.securityGroups)
	}
	if len(networks.securityRules) != 0 {
		t.Fatalf("registration created security rules: %+v", networks.securityRules)
	}
	if len(networks.versions) != 0 {
		t.Fatalf("registration created network versions: %+v", networks.versions)
	}
}
