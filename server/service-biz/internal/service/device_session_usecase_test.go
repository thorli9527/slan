package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/pkg/mqttkit"
)

type deviceSessionTestUsers struct {
	users map[string]model.User
}

func (s *deviceSessionTestUsers) ListUsers(context.Context) ([]model.User, error) { return nil, nil }

func (s *deviceSessionTestUsers) GetUser(_ context.Context, userID string) (model.User, bool, error) {
	item, ok := s.users[userID]
	return item, ok, nil
}

func (s *deviceSessionTestUsers) GetByEmail(_ context.Context, email string) (model.User, bool, error) {
	for _, item := range s.users {
		if item.Email == email {
			return item, true, nil
		}
	}
	return model.User{}, false, nil
}

func (s *deviceSessionTestUsers) SaveUser(_ context.Context, user model.User) error {
	if s.users == nil {
		s.users = map[string]model.User{}
	}
	s.users[user.UserID] = user
	return nil
}

type deviceSessionTestDevices struct {
	networkRuntimeTestDevices
	savedSessions []model.DeviceSession
	nextVirtualIP int
}

func (s *deviceSessionTestDevices) SaveDevice(_ context.Context, device model.Device) error {
	if s.devices == nil {
		s.devices = map[string]model.Device{}
	}
	s.devices[device.DeviceID] = device
	return nil
}

func (s *deviceSessionTestDevices) SaveDeviceSession(_ context.Context, session model.DeviceSession) error {
	s.savedSessions = append(s.savedSessions, session)
	return nil
}

func (s *deviceSessionTestDevices) NewDeviceVirtualIPID() string {
	s.nextVirtualIP += 1
	return fmt.Sprintf("vip-%06d", s.nextVirtualIP)
}

type deviceSessionTestNetworks struct {
	networkRuntimeTestNetworks
}

func (s *deviceSessionTestNetworks) ListNetworksByOwner(_ context.Context, ownerID string) ([]model.Network, error) {
	out := []model.Network{}
	for _, item := range s.networks {
		if item.OwnerID == ownerID {
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *deviceSessionTestNetworks) SaveNetworkDevice(_ context.Context, item model.NetworkDevice) error {
	items := s.networkDevices[item.NetworkID]
	for index, current := range items {
		if current.DeviceID == item.DeviceID {
			items[index] = item
			s.networkDevices[item.NetworkID] = items
			return nil
		}
	}
	s.networkDevices[item.NetworkID] = append(items, item)
	return nil
}

func TestBindDeviceSessionAutoRegistersMissingDevice(t *testing.T) {
	now := time.Unix(1700000000, 0)
	devices := &deviceSessionTestDevices{
		networkRuntimeTestDevices: networkRuntimeTestDevices{
			devices: map[string]model.Device{},
		},
	}
	networks := &deviceSessionTestNetworks{
		networkRuntimeTestNetworks: networkRuntimeTestNetworks{
			networks: map[string]model.Network{
				"net-1": {
					NetworkID: "net-1",
					OwnerID:   "user-1",
					Name:      "Default",
					CIDR:      "10.0.0.0/24",
					Default:   true,
					Status:    "active",
				},
			},
			networkDevices: map[string][]model.NetworkDevice{},
		},
	}
	service := DeviceSessionService{
		Users: &deviceSessionTestUsers{
			users: map[string]model.User{
				"user-1": {UserID: "user-1", Email: "user-1@example.test", Status: "active"},
			},
		},
		Devices:   devices,
		Networks:  networks,
		MQTT:      mqttkit.DefaultConfig(),
		NewSessID: func(scope string) string { return scope + "-1" },
		Now:       func() time.Time { return now },
	}

	view, err := service.BindDeviceSession(context.Background(), BindDeviceSessionInput{
		UserID:        "user-1",
		DeviceID:      "android-1",
		Name:          "Pixel 3a",
		Platform:      "android",
		Alias:         "android-phone",
		OSName:        "android",
		OSVersion:     "14",
		PublicKey:     "pub-1",
		DeviceVersion: "1.0.0",
		CountryCode:   "CN",
	})
	if err != nil {
		t.Fatalf("BindDeviceSession returned error: %v", err)
	}
	device, ok := devices.devices["android-1"]
	if !ok {
		t.Fatalf("expected device to be auto-registered")
	}
	if device.OwnerID != "user-1" {
		t.Fatalf("expected device owner user-1, got %s", device.OwnerID)
	}
	if len(devices.savedSessions) != 1 {
		t.Fatalf("expected one saved device session, got %d", len(devices.savedSessions))
	}
	if len(networks.networkDevices["net-1"]) != 1 {
		t.Fatalf("expected default network membership to be created, got %+v", networks.networkDevices["net-1"])
	}
	if view.Profile.Device.DeviceID != "android-1" {
		t.Fatalf("expected view device android-1, got %+v", view.Profile.Device)
	}
	if view.Profile.OwnerEmail != "user-1@example.test" {
		t.Fatalf("expected owner email to be populated, got %s", view.Profile.OwnerEmail)
	}
	if view.Session.DeviceID != "android-1" || view.Session.AccessToken == "" {
		t.Fatalf("expected bound session to be returned, got %+v", view.Session)
	}
}
