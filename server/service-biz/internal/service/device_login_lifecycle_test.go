package service

import (
	"context"
	"testing"
	"time"

	"github.com/slan/service-biz/internal/model"
)

type deviceLoginTestSessions struct {
	sessions map[string]model.UserSession
}

func (s *deviceLoginTestSessions) GetUserSessionByAccessToken(_ context.Context, token string) (model.UserSession, bool, error) {
	item, ok := s.sessions[token]
	return item, ok, nil
}

func (s *deviceLoginTestSessions) GetUserSessionByRefreshToken(context.Context, string) (model.UserSession, bool, error) {
	return model.UserSession{}, false, nil
}

func (s *deviceLoginTestSessions) ListUserSessionsByUserID(context.Context, string) ([]model.UserSession, error) {
	return nil, nil
}

func (s *deviceLoginTestSessions) SaveUserSession(context.Context, model.UserSession) error {
	return nil
}
func (s *deviceLoginTestSessions) ReplaceUserSessionForClient(_ context.Context, next model.UserSession) error {
	if s.sessions == nil {
		s.sessions = make(map[string]model.UserSession)
	}
	for accessToken, session := range s.sessions {
		if session.UserID == next.UserID && session.ClientType == next.ClientType && session.DeviceID == next.DeviceID {
			delete(s.sessions, accessToken)
		}
	}
	s.sessions[next.AccessToken] = next
	return nil
}
func (s *deviceLoginTestSessions) ReplaceUserSession(context.Context, string, model.UserSession) error {
	return nil
}
func (s *deviceLoginTestSessions) DeleteUserSessionByAccessToken(context.Context, string) error {
	return nil
}
func (s *deviceLoginTestSessions) GetConsoleLoginKeyByKey(context.Context, string) (model.ConsoleLoginKey, bool, error) {
	return model.ConsoleLoginKey{}, false, nil
}
func (s *deviceLoginTestSessions) SaveConsoleLoginKey(context.Context, model.ConsoleLoginKey) error {
	return nil
}

type deviceLoginTestDevices struct {
	deviceRegistrationTestDevices
	logins         map[string]model.DeviceLoginDevice
	groups         map[string]model.DeviceGroup
	assignments    map[string]model.DeviceGroupAssignment
	onOwnerChanged func(deviceID string)
}

func (s *deviceLoginTestDevices) GetDeviceLoginDevice(_ context.Context, deviceID string) (model.DeviceLoginDevice, bool, error) {
	item, ok := s.logins[deviceID]
	return item, ok, nil
}

func (s *deviceLoginTestDevices) SaveDeviceLoginDevice(_ context.Context, item model.DeviceLoginDevice) error {
	s.logins[item.DeviceID] = item
	return nil
}

func (s *deviceLoginTestDevices) SaveDevice(ctx context.Context, item model.Device) error {
	previous, ok := s.devices[item.DeviceID]
	if err := s.deviceRegistrationTestDevices.SaveDevice(ctx, item); err != nil {
		return err
	}
	if ok && previous.OwnerID != "" && previous.OwnerID != item.OwnerID && s.onOwnerChanged != nil {
		s.onOwnerChanged(item.DeviceID)
	}
	return nil
}

func (s *deviceLoginTestDevices) ListDeviceGroups(_ context.Context, userID string) ([]model.DeviceGroup, error) {
	out := []model.DeviceGroup{}
	for _, group := range s.groups {
		if group.UserID == userID {
			out = append(out, group)
		}
	}
	return out, nil
}

func (s *deviceLoginTestDevices) GetDeviceGroup(_ context.Context, groupID string) (model.DeviceGroup, bool, error) {
	group, ok := s.groups[groupID]
	return group, ok, nil
}

func (s *deviceLoginTestDevices) SetDeviceGroups(_ context.Context, assignment model.DeviceGroupAssignment) error {
	if s.assignments == nil {
		s.assignments = make(map[string]model.DeviceGroupAssignment)
	}
	s.assignments[assignment.DeviceID] = assignment
	return nil
}

func (s *deviceLoginTestDevices) ListDeviceGroupAssignments(_ context.Context, userID string) ([]model.DeviceGroupAssignment, error) {
	out := []model.DeviceGroupAssignment{}
	for _, assignment := range s.assignments {
		if assignment.UserID == userID {
			out = append(out, assignment)
		}
	}
	return out, nil
}

type deviceLoginTestNetworks struct {
	networkRuntimeTestNetworks
	groupRefs map[string][]model.NetworkDeviceGroupReference
}

func (s *deviceLoginTestNetworks) ListNetworksByOwner(_ context.Context, ownerID string) ([]model.Network, error) {
	items := make([]model.Network, 0)
	for _, network := range s.networks {
		if network.OwnerID == ownerID {
			items = append(items, network)
		}
	}
	return items, nil
}

func (s *deviceLoginTestNetworks) SaveNetworkDevice(_ context.Context, item model.NetworkDevice) error {
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

func (s *deviceLoginTestNetworks) DeleteNetworkDevice(_ context.Context, networkID, deviceID string) error {
	items := s.networkDevices[networkID]
	kept := items[:0]
	for _, item := range items {
		if item.DeviceID != deviceID {
			kept = append(kept, item)
		}
	}
	s.networkDevices[networkID] = kept
	return nil
}

func (s *deviceLoginTestNetworks) ListNetworkDeviceGroupReferences(_ context.Context, networkID string) ([]model.NetworkDeviceGroupReference, error) {
	return append([]model.NetworkDeviceGroupReference(nil), s.groupRefs[networkID]...), nil
}

func (s *deviceLoginTestNetworks) SaveNetworkDeviceGroupReference(_ context.Context, item model.NetworkDeviceGroupReference) error {
	if s.groupRefs == nil {
		s.groupRefs = make(map[string][]model.NetworkDeviceGroupReference)
	}
	s.groupRefs[item.NetworkID] = append(s.groupRefs[item.NetworkID], item)
	return nil
}

func (s *deviceLoginTestNetworks) DeleteNetworkDeviceGroupReference(context.Context, string, string) error {
	return nil
}

func (s *deviceLoginTestNetworks) DeleteNetworkDeviceGroupReferencesByGroup(context.Context, string) error {
	return nil
}

type deviceLoginTestPublisher struct {
	deviceID string
	event    DeviceControlEnvelope
	events   []DeviceControlEnvelope
}

func (p *deviceLoginTestPublisher) PublishDeviceControl(_ context.Context, deviceID string, event DeviceControlEnvelope) error {
	p.deviceID = deviceID
	p.event = event
	p.events = append(p.events, event)
	return nil
}

func TestRegisterInstalledDeviceAllocatesIPWithoutAttachingNetwork(t *testing.T) {
	now := time.Unix(1700004000, 0)
	devices := &deviceRegistrationTestDevices{
		networkRuntimeTestDevices: networkRuntimeTestDevices{devices: map[string]model.Device{}},
	}
	device, err := registerInstalledDevice(
		context.Background(),
		&deviceRegistrationTestUsers{users: map[string]model.User{
			"user-1": {UserID: "user-1", Email: "user@example.test", Status: "active"},
		}},
		devices,
		func() time.Time { return now },
		RegisterDeviceInput{
			OwnerID:  "user-1",
			DeviceID: "device-1",
			Name:     "Mac",
			Platform: "macos",
		},
	)
	if err != nil {
		t.Fatalf("registerInstalledDevice returned error: %v", err)
	}
	if device.VirtualIP != "10.0.1.1" {
		t.Fatalf("installed device must receive IP 10.0.1.1, got %q", device.VirtualIP)
	}
	if devices.nextVirtualIP != 1 {
		t.Fatalf("installed device must allocate exactly one IP, got %d", devices.nextVirtualIP)
	}
}

func TestRegisterInstalledDeviceRepairsMissingIPOnce(t *testing.T) {
	now := time.Unix(1700004000, 0)
	devices := &deviceRegistrationTestDevices{
		networkRuntimeTestDevices: networkRuntimeTestDevices{devices: map[string]model.Device{
			"device-1": {
				DeviceID: "device-1", OwnerID: "user-1", Name: "Linux", Platform: "linux", Status: "active",
			},
		}},
	}
	users := &deviceRegistrationTestUsers{users: map[string]model.User{
		"user-1": {UserID: "user-1", Email: "user@example.test", Status: "active"},
	}}
	input := RegisterDeviceInput{
		OwnerID: "user-1", DeviceID: "device-1", Name: "Linux", Platform: "linux",
	}
	first, err := registerInstalledDevice(context.Background(), users, devices, func() time.Time { return now }, input)
	if err != nil {
		t.Fatalf("first registerInstalledDevice returned error: %v", err)
	}
	second, err := registerInstalledDevice(context.Background(), users, devices, func() time.Time { return now }, input)
	if err != nil {
		t.Fatalf("second registerInstalledDevice returned error: %v", err)
	}
	if first.VirtualIP != "10.0.1.1" || second.VirtualIP != first.VirtualIP {
		t.Fatalf("expected stable repaired IP, first=%q second=%q", first.VirtualIP, second.VirtualIP)
	}
	if devices.nextVirtualIP != 1 {
		t.Fatalf("expected one IP allocation across retries, got %d", devices.nextVirtualIP)
	}
}

func TestCompleteDeviceLoginAllocatesIPAndPublishesPrivateLogin(t *testing.T) {
	now := time.Unix(1700005000, 0)
	devices := &deviceLoginTestDevices{
		deviceRegistrationTestDevices: deviceRegistrationTestDevices{
			networkRuntimeTestDevices: networkRuntimeTestDevices{devices: map[string]model.Device{
				"device-1": {
					DeviceID: "device-1", OwnerID: "user-1", Name: "Mac", Platform: "macos", Status: "active",
				},
			}},
		},
		logins: map[string]model.DeviceLoginDevice{
			"device-1": {
				DeviceID: "device-1", Name: "Mac", Platform: "macos", Status: "pending", ExpiresAt: now.Add(time.Minute).Unix(),
			},
		},
	}
	networks := &deviceLoginTestNetworks{networkRuntimeTestNetworks: networkRuntimeTestNetworks{
		networks: map[string]model.Network{
			"network-1": {NetworkID: "network-1", OwnerID: "user-1", Name: "Default", Default: true, Status: "active"},
		},
		networkDevices: map[string][]model.NetworkDevice{},
	}}
	publisher := &deviceLoginTestPublisher{}
	service := AuthDeviceLoginCompleteService{authDeviceLoginDependencies: authDeviceLoginDependencies{
		Users: &deviceRegistrationTestUsers{users: map[string]model.User{
			"user-1": {UserID: "user-1", Email: "user@example.test", Status: "active"},
		}},
		Sessions: &deviceLoginTestSessions{sessions: map[string]model.UserSession{
			"access-1": {
				UserID: "user-1", AccessToken: "access-1", RefreshToken: "refresh-1", Status: "active", ExpiresAt: now.Add(time.Hour).Unix(),
			},
		}},
		Devices: devices, Networks: networks, DevicePublisher: publisher,
		NewSessID: func(string) string { return "desktop-session-1" }, Now: func() time.Time { return now },
	}}

	_, err := service.CompleteDeviceLoginDevice(context.Background(), CompleteDeviceLoginDeviceInput{
		AccessToken: "access-1",
		DeviceID:    "device-1",
	})
	if err != nil {
		t.Fatalf("CompleteDeviceLoginDevice returned error: %v", err)
	}
	if got := devices.devices["device-1"].VirtualIP; got != "10.0.1.1" {
		t.Fatalf("expected login allocation 10.0.1.1, got %q", got)
	}
	if len(devices.assignments) != 0 {
		t.Fatalf("login must not create implicit group assignments: %+v", devices.assignments)
	}
	if len(networks.networkDevices["network-1"]) != 0 {
		t.Fatalf("login must not create implicit network membership: %+v", networks.networkDevices["network-1"])
	}
	if publisher.deviceID != "device-1" {
		t.Fatalf("unexpected private event target: %q", publisher.deviceID)
	}
	if len(publisher.events) != 1 {
		t.Fatalf("expected only login event, got %#v", publisher.events)
	}
	if publisher.events[0].Type != "device_user_login_succeeded" {
		t.Fatalf("unexpected private event: %#v", publisher.events)
	}
	loginEvent := publisher.events[0]
	desktopAccessToken, _ := loginEvent.Payload["accessToken"].(string)
	if desktopAccessToken == "" || desktopAccessToken == "access-1" {
		t.Fatalf("expected an independent desktop access token, got %#v", desktopAccessToken)
	}
	desktopSession, ok, err := service.Sessions.GetUserSessionByAccessToken(context.Background(), desktopAccessToken)
	if err != nil {
		t.Fatalf("load desktop session: %v", err)
	}
	if !ok || desktopSession.ClientType != UserSessionClientDesktop || desktopSession.DeviceID != "device-1" {
		t.Fatalf("expected device-scoped desktop session, got %#v", desktopSession)
	}
	if got := loginEvent.Payload["virtualIp"]; got != "10.0.1.1" {
		t.Fatalf("expected virtual IP in private login event, got %#v", got)
	}
}

func TestCompleteDeviceLoginChangingOwnerClearsOldNetworkExposure(t *testing.T) {
	now := time.Unix(1700006000, 0)
	networks := &deviceLoginTestNetworks{networkRuntimeTestNetworks: networkRuntimeTestNetworks{
		networks: map[string]model.Network{
			"old-network": {NetworkID: "old-network", OwnerID: "old-user", Name: "Old", Status: "active"},
		},
		networkDevices: map[string][]model.NetworkDevice{
			"old-network": {{NetworkID: "old-network", DeviceID: "device-1", Enabled: true, MemberStatus: model.NetworkMemberStatusActive}},
		},
	}}
	devices := &deviceLoginTestDevices{
		deviceRegistrationTestDevices: deviceRegistrationTestDevices{
			networkRuntimeTestDevices: networkRuntimeTestDevices{devices: map[string]model.Device{
				"device-1": {DeviceID: "device-1", OwnerID: "old-user", VirtualIP: "10.0.0.1", Name: "Mac", Platform: "macos", Status: "active"},
			}},
		},
		logins: map[string]model.DeviceLoginDevice{
			"device-1": {DeviceID: "device-1", Name: "Mac", Platform: "macos", Status: "pending", ExpiresAt: now.Add(time.Minute).Unix()},
		},
	}
	devices.onOwnerChanged = func(deviceID string) {
		_ = networks.DeleteNetworkDevice(context.Background(), "old-network", deviceID)
	}
	devicePublisher := &deviceLoginTestPublisher{}
	networkPublisher := &deviceRuntimeTestEventPublisher{}
	service := AuthDeviceLoginCompleteService{authDeviceLoginDependencies: authDeviceLoginDependencies{
		Users: &deviceRegistrationTestUsers{users: map[string]model.User{
			"new-user": {UserID: "new-user", Email: "new@example.test", Status: "active"},
		}},
		Sessions: &deviceLoginTestSessions{sessions: map[string]model.UserSession{
			"access-new": {UserID: "new-user", AccessToken: "access-new", RefreshToken: "refresh-new", Status: "active", ExpiresAt: now.Add(time.Hour).Unix()},
		}},
		Devices: devices, Networks: networks, DevicePublisher: devicePublisher,
		EventPublisher: networkPublisher, Now: func() time.Time { return now },
	}}

	if _, err := service.CompleteDeviceLoginDevice(context.Background(), CompleteDeviceLoginDeviceInput{
		AccessToken: "access-new", DeviceID: "device-1",
	}); err != nil {
		t.Fatalf("complete owner-changing login: %v", err)
	}
	if got := devices.devices["device-1"].OwnerID; got != "new-user" {
		t.Fatalf("owner = %q, want new-user", got)
	}
	if len(networks.networkDevices["old-network"]) != 0 {
		t.Fatalf("old network still exposes device: %#v", networks.networkDevices["old-network"])
	}
	foundRemoved := false
	for _, event := range networkPublisher.events {
		if event.EventType == NetworkEventMemberRemoved && event.NetworkID == "old-network" {
			foundRemoved = true
		}
	}
	if !foundRemoved {
		t.Fatalf("expected member removed event, got %#v", networkPublisher.events)
	}
	if got := devicePublisher.event.Payload["activeNetworkId"]; got != "" {
		t.Fatalf("login retained old active network: %#v", got)
	}
}
