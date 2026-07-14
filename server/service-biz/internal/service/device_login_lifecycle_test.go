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
	logins map[string]model.DeviceLoginDevice
}

func (s *deviceLoginTestDevices) GetDeviceLoginDevice(_ context.Context, deviceID string) (model.DeviceLoginDevice, bool, error) {
	item, ok := s.logins[deviceID]
	return item, ok, nil
}

func (s *deviceLoginTestDevices) SaveDeviceLoginDevice(_ context.Context, item model.DeviceLoginDevice) error {
	s.logins[item.DeviceID] = item
	return nil
}

type deviceLoginTestNetworks struct {
	networkRuntimeTestNetworks
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

type deviceLoginTestPublisher struct {
	deviceID string
	event    DeviceControlEnvelope
}

func (p *deviceLoginTestPublisher) PublishDeviceControl(_ context.Context, deviceID string, event DeviceControlEnvelope) error {
	p.deviceID = deviceID
	p.event = event
	return nil
}

func TestRegisterInstalledDeviceDoesNotAllocateIPOrAttachNetwork(t *testing.T) {
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
	if device.VirtualIP != "" {
		t.Fatalf("installed device must not receive IP, got %q", device.VirtualIP)
	}
	if devices.nextVirtualIP != 0 {
		t.Fatalf("installed device must not consume IP sequence, got %d", devices.nextVirtualIP)
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
		Devices: devices, Networks: networks, DevicePublisher: publisher, Now: func() time.Time { return now },
	}}

	_, err := service.CompleteDeviceLoginDevice(context.Background(), CompleteDeviceLoginDeviceInput{
		AccessToken: "access-1",
		DeviceID:    "device-1",
	})
	if err != nil {
		t.Fatalf("CompleteDeviceLoginDevice returned error: %v", err)
	}
	if got := devices.devices["device-1"].VirtualIP; got != "10.0.0.1" {
		t.Fatalf("expected login allocation 10.0.0.1, got %q", got)
	}
	if len(networks.networkDevices["network-1"]) != 0 {
		t.Fatalf("expected login not to attach an individual device to the network")
	}
	if publisher.deviceID != "device-1" || publisher.event.Type != "device_user_login_succeeded" {
		t.Fatalf("unexpected private login event: device=%q type=%q", publisher.deviceID, publisher.event.Type)
	}
	if got := publisher.event.Payload["accessToken"]; got != "access-1" {
		t.Fatalf("expected access token in private login event, got %#v", got)
	}
	if got := publisher.event.Payload["virtualIp"]; got != "10.0.0.1" {
		t.Fatalf("expected virtual IP in private login event, got %#v", got)
	}
}
