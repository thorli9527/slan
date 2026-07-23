package service

import (
	"context"
	"testing"
	"time"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/pkg/mqttkit"
)

type deviceRegistrationTestUsers struct {
	users map[string]model.User
}

func (s *deviceRegistrationTestUsers) ListUsers(context.Context) ([]model.User, error) {
	return nil, nil
}

func (s *deviceRegistrationTestUsers) GetUser(_ context.Context, userID string) (model.User, bool, error) {
	item, ok := s.users[userID]
	return item, ok, nil
}

func (s *deviceRegistrationTestUsers) GetByEmail(_ context.Context, email string) (model.User, bool, error) {
	for _, item := range s.users {
		if item.Email == email {
			return item, true, nil
		}
	}
	return model.User{}, false, nil
}

func (s *deviceRegistrationTestUsers) SaveUser(_ context.Context, user model.User) error {
	if s.users == nil {
		s.users = map[string]model.User{}
	}
	s.users[user.UserID] = user
	return nil
}

type deviceRegistrationTestDevices struct {
	networkRuntimeTestDevices
	nextVirtualIP int
}

func (s *deviceRegistrationTestDevices) SaveDevice(_ context.Context, device model.Device) error {
	if s.devices == nil {
		s.devices = map[string]model.Device{}
	}
	s.devices[device.DeviceID] = device
	return nil
}

func (s *deviceRegistrationTestDevices) NewDeviceVirtualIPID() string {
	s.nextVirtualIP += 1
	return newTestVirtualIPSequenceID(s.nextVirtualIP)
}

func newTestVirtualIPSequenceID(index int) string {
	return "vip-" + leftPadInt(index, 6)
}

func leftPadInt(value, width int) string {
	text := ""
	for current := value; current > 0; current /= 10 {
		text = string(rune('0'+(current%10))) + text
	}
	if text == "" {
		text = "0"
	}
	for len(text) < width {
		text = "0" + text
	}
	return text
}

type deviceRegistrationTestNetworks struct {
	networkRuntimeTestNetworks
}

func TestRegisterDeviceAllocatesStableVirtualIP(t *testing.T) {
	now := time.Unix(1700001000, 0)
	devices := &deviceRegistrationTestDevices{
		networkRuntimeTestDevices: networkRuntimeTestDevices{
			devices: map[string]model.Device{},
		},
	}
	networks := &deviceRegistrationTestNetworks{
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
	service := DeviceProvisioningService{
		deviceCoreDependencies: deviceCoreDependencies{
			Users:       &deviceRegistrationTestUsers{users: map[string]model.User{"user-1": {UserID: "user-1", Email: "user-1@example.test", Status: "active"}}},
			Devices:     devices,
			Networks:    networks,
			MQTT:        mqttkit.DefaultConfig(),
			NewDeviceID: func() string { return "dev-1" },
			Now:         func() time.Time { return now },
		},
	}

	first, err := service.RegisterDevice(context.Background(), RegisterDeviceInput{
		OwnerID:  "user-1",
		DeviceID: "device-fixed-1",
		Name:     "Pixel",
		Platform: "android",
		Alias:    "Pixel",
	})
	if err != nil {
		t.Fatalf("first RegisterDevice returned error: %v", err)
	}
	if first.Device.Alias != "" {
		t.Fatalf("expected first registration alias to be empty, got %q", first.Device.Alias)
	}
	storedWithAlias := devices.devices["device-fixed-1"]
	storedWithAlias.Alias = "My phone"
	devices.devices["device-fixed-1"] = storedWithAlias
	second, err := service.RegisterDevice(context.Background(), RegisterDeviceInput{
		OwnerID:  "user-1",
		DeviceID: "device-fixed-1",
		Name:     "Pixel renamed",
		Platform: "android",
	})
	if err != nil {
		t.Fatalf("second RegisterDevice returned error: %v", err)
	}

	if first.VirtualIP == "" {
		t.Fatalf("expected first registration to allocate virtual IP")
	}
	if first.VirtualIP != "10.0.1.1" {
		t.Fatalf("expected first device IP to be 10.0.1.1, got %q", first.VirtualIP)
	}
	if second.VirtualIP != first.VirtualIP {
		t.Fatalf("expected stable virtual IP across re-registration, first=%q second=%q", first.VirtualIP, second.VirtualIP)
	}
	if second.Device.Alias != "My phone" {
		t.Fatalf("expected manual alias to survive re-registration, got %q", second.Device.Alias)
	}
	stored := devices.devices["device-fixed-1"]
	if stored.VirtualIP != first.VirtualIP {
		t.Fatalf("expected stored virtual IP %q, got %q", first.VirtualIP, stored.VirtualIP)
	}
	if devices.nextVirtualIP != 1 {
		t.Fatalf("expected only one server-side IP allocation, got %d", devices.nextVirtualIP)
	}
}

func TestRegisterDifferentDevicesGetDifferentVirtualIPs(t *testing.T) {
	now := time.Unix(1700002000, 0)
	devices := &deviceRegistrationTestDevices{
		networkRuntimeTestDevices: networkRuntimeTestDevices{
			devices: map[string]model.Device{},
		},
	}
	networks := &deviceRegistrationTestNetworks{
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
	service := DeviceProvisioningService{
		deviceCoreDependencies: deviceCoreDependencies{
			Users:       &deviceRegistrationTestUsers{users: map[string]model.User{"user-1": {UserID: "user-1", Email: "user-1@example.test", Status: "active"}}},
			Devices:     devices,
			Networks:    networks,
			MQTT:        mqttkit.DefaultConfig(),
			NewDeviceID: func() string { return "dev-1" },
			Now:         func() time.Time { return now },
		},
	}

	first, err := service.RegisterDevice(context.Background(), RegisterDeviceInput{
		OwnerID:  "user-1",
		DeviceID: "device-a",
		Name:     "Device A",
		Platform: "android",
	})
	if err != nil {
		t.Fatalf("first RegisterDevice returned error: %v", err)
	}
	second, err := service.RegisterDevice(context.Background(), RegisterDeviceInput{
		OwnerID:  "user-1",
		DeviceID: "device-b",
		Name:     "Device B",
		Platform: "ios",
	})
	if err != nil {
		t.Fatalf("second RegisterDevice returned error: %v", err)
	}

	if first.VirtualIP == "" || second.VirtualIP == "" {
		t.Fatalf("expected both devices to receive virtual IPs, first=%q second=%q", first.VirtualIP, second.VirtualIP)
	}
	if first.VirtualIP != "10.0.1.1" || second.VirtualIP != "10.0.1.2" {
		t.Fatalf("expected sequential device IPs 10.0.1.1 and 10.0.1.2, got %q and %q", first.VirtualIP, second.VirtualIP)
	}
	if first.VirtualIP == second.VirtualIP {
		t.Fatalf("expected different devices to receive different virtual IPs, got %q", first.VirtualIP)
	}
}
