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
	versions       map[string]model.NetworkConfigVersion
	nextSecurityID int
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

func (s *authUserRegistrationTestNetworks) ListPublicMappings(context.Context, string) ([]model.PublicMapping, error) {
	return nil, nil
}

func (s *authUserRegistrationTestNetworks) GetPublicMapping(context.Context, string) (model.PublicMapping, bool, error) {
	return model.PublicMapping{}, false, nil
}

func (s *authUserRegistrationTestNetworks) SavePublicMapping(context.Context, model.PublicMapping) error {
	return nil
}

func (s *authUserRegistrationTestNetworks) DeletePublicMapping(context.Context, string) error {
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

func (s *authUserRegistrationTestNetworks) ListSecurityRules(context.Context, string) ([]model.SecurityRule, error) {
	return nil, nil
}

func (s *authUserRegistrationTestNetworks) GetSecurityRule(context.Context, string) (model.SecurityRule, bool, error) {
	return model.SecurityRule{}, false, nil
}

func (s *authUserRegistrationTestNetworks) SaveSecurityRule(context.Context, model.SecurityRule) error {
	return nil
}

func (s *authUserRegistrationTestNetworks) DeleteSecurityRule(context.Context, string) error {
	return nil
}

func (s *authUserRegistrationTestNetworks) NewSecurityGroupID() string {
	s.nextSecurityID++
	return "sg-test-" + string(rune('0'+s.nextSecurityID))
}

var _ repository.NetworkRepository = (*authUserRegistrationTestNetworks)(nil)
var _ repository.UserRepository = (*authUserRegistrationTestUsers)(nil)
var _ repository.UserSessionRepository = (*authUserRegistrationTestSessions)(nil)

func TestRegisterUserCreatesDefaultSecurityGroup(t *testing.T) {
	networks := &authUserRegistrationTestNetworks{}
	service := AuthUserRegistrationService{
		authUserDependencies: authUserDependencies{
			Users:              &authUserRegistrationTestUsers{},
			Sessions:           &authUserRegistrationTestSessions{},
			Networks:           networks,
			NewUserID:          func() string { return "user-test-1" },
			NewNetID:           func() string { return "net-test-1" },
			NewSessID:          func(string) string { return "sess-test-1" },
			NewSecurityGroupID: func() string { return "sg-test-1" },
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

	groups, err := networks.ListSecurityGroups(context.Background(), "net-test-1")
	if err != nil {
		t.Fatalf("ListSecurityGroups returned error: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("expected 1 default security group, got %d", len(groups))
	}
	if groups[0].Name != "Default Security Group" {
		t.Fatalf("unexpected default security group: %+v", groups[0])
	}
	version, ok, err := networks.GetNetworkVersion(context.Background(), "net-test-1")
	if err != nil {
		t.Fatalf("GetNetworkVersion returned error: %v", err)
	}
	if !ok {
		t.Fatalf("expected default network version to be created")
	}
	if version.Version != 1 {
		t.Fatalf("expected default network version 1, got %+v", version)
	}
	if version.Reason != "user_default_network_created" {
		t.Fatalf("unexpected default network version reason: %+v", version)
	}
}
