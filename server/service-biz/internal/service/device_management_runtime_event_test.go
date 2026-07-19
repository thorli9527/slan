package service

import (
	"context"
	"testing"
	"time"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/pkg/mqttkit"
)

type deviceRuntimeTestEventPublisher struct {
	events []NetworkEventEnvelope
}

func (s *deviceRuntimeTestEventPublisher) PublishNetworkEvent(_ context.Context, event NetworkEventEnvelope) error {
	s.events = append(s.events, event)
	return nil
}

func TestUpdateDeviceRuntimePublishesNetworkMemberEvent(t *testing.T) {
	now := time.Unix(1700003000, 0)
	eventPublisher := &deviceRuntimeTestEventPublisher{}
	devices := &deviceRegistrationTestDevices{
		networkRuntimeTestDevices: networkRuntimeTestDevices{
			devices: map[string]model.Device{
				"device-a": {
					DeviceID:   "device-a",
					OwnerID:    "user-1",
					VirtualIP:  "10.0.0.1",
					Platform:   "linux",
					Status:     "active",
					LastSeenAt: now.Unix(),
				},
			},
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
			networkDevices: map[string][]model.NetworkDevice{
				"net-1": {{
					NetworkID:      "net-1",
					DeviceID:       "device-a",
					Enabled:        true,
					MemberStatus:   model.NetworkMemberStatusActive,
					PresenceStatus: model.DevicePresenceStatusActive,
				}},
			},
		},
	}
	service := DeviceRuntimeAccessService{
		deviceCoreDependencies: deviceCoreDependencies{
			Users:          &deviceRegistrationTestUsers{users: map[string]model.User{"user-1": {UserID: "user-1", Email: "user-1@example.test", Status: "active"}}},
			Devices:        devices,
			Networks:       networks,
			MQTT:           mqttkit.DefaultConfig(),
			EventPublisher: eventPublisher,
			Now:            func() time.Time { return now },
		},
	}

	_, err := service.UpdateDeviceRuntime(context.Background(), UpdateDeviceRuntimeInput{
		DeviceID:       "device-a",
		NetworkID:      "net-1",
		ActivePath:     "relay_tcp",
		PathObservedAt: now.UnixMilli(),
	})
	if err != nil {
		t.Fatalf("UpdateDeviceRuntime returned error: %v", err)
	}
	if len(eventPublisher.events) != 1 {
		t.Fatalf("expected 1 network event, got %d", len(eventPublisher.events))
	}
	if eventPublisher.events[0].EventType != NetworkEventMemberOnline {
		t.Fatalf("expected event type %q, got %q", NetworkEventMemberOnline, eventPublisher.events[0].EventType)
	}
}

func TestUpdateDeviceRuntimeAppliesPresenceToEveryActiveNetwork(t *testing.T) {
	now := time.Unix(1700003000, 0)
	eventPublisher := &deviceRuntimeTestEventPublisher{}
	devices := &deviceRegistrationTestDevices{
		networkRuntimeTestDevices: networkRuntimeTestDevices{
			devices: map[string]model.Device{
				"device-a": {DeviceID: "device-a", OwnerID: "user-1", Status: "active"},
			},
		},
	}
	networks := &deviceRegistrationTestNetworks{
		networkRuntimeTestNetworks: networkRuntimeTestNetworks{
			networks: map[string]model.Network{
				"net-a": {NetworkID: "net-a", Status: "active"},
				"net-b": {NetworkID: "net-b", Status: "active"},
			},
			networkDevices: map[string][]model.NetworkDevice{
				"net-a": {{NetworkID: "net-a", DeviceID: "device-a", Enabled: true, MemberStatus: model.NetworkMemberStatusActive}},
				"net-b": {{NetworkID: "net-b", DeviceID: "device-a", Enabled: true, MemberStatus: model.NetworkMemberStatusActive}},
			},
		},
	}
	service := DeviceRuntimeAccessService{
		deviceCoreDependencies: deviceCoreDependencies{
			Users:          &deviceRegistrationTestUsers{users: map[string]model.User{"user-1": {UserID: "user-1", Status: "active"}}},
			Devices:        devices,
			Networks:       networks,
			MQTT:           mqttkit.DefaultConfig(),
			EventPublisher: eventPublisher,
			Now:            func() time.Time { return now },
		},
	}

	_, err := service.UpdateDeviceRuntime(context.Background(), UpdateDeviceRuntimeInput{
		DeviceID:     "device-a",
		NetworkID:    "net-a",
		ActivePath:   "relay_udp",
		ReportedAtMS: now.UnixMilli(),
	})
	if err != nil {
		t.Fatalf("UpdateDeviceRuntime returned error: %v", err)
	}
	if got := len(networks.savedNetworkDevices); got != 2 {
		t.Fatalf("expected runtime state saved to 2 networks, got %d", got)
	}
	if got := len(eventPublisher.events); got != 2 {
		t.Fatalf("expected presence events for 2 networks, got %d", got)
	}
}

func TestUpdateDeviceRuntimeDoesNotRepublishPresenceHeartbeatWithoutStateChange(t *testing.T) {
	now := time.Unix(1700003000, 0)
	eventPublisher := &deviceRuntimeTestEventPublisher{}
	devices := &deviceRegistrationTestDevices{
		networkRuntimeTestDevices: networkRuntimeTestDevices{
			devices: map[string]model.Device{
				"device-a": {
					DeviceID:   "device-a",
					OwnerID:    "user-1",
					VirtualIP:  "10.0.0.1",
					Platform:   "linux",
					Status:     "active",
					LastSeenAt: now.Unix(),
				},
			},
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
			networkDevices: map[string][]model.NetworkDevice{
				"net-1": {{
					NetworkID:          "net-1",
					DeviceID:           "device-a",
					Enabled:            true,
					MemberStatus:       model.NetworkMemberStatusActive,
					PresenceStatus:     model.DevicePresenceStatusActive,
					ActivePath:         "relay_tcp",
					LastSeenAt:         now.Unix(),
					LastRuntimeStateAt: now.Unix(),
				}},
			},
		},
	}
	service := DeviceRuntimeAccessService{
		deviceCoreDependencies: deviceCoreDependencies{
			Users:          &deviceRegistrationTestUsers{users: map[string]model.User{"user-1": {UserID: "user-1", Email: "user-1@example.test", Status: "active"}}},
			Devices:        devices,
			Networks:       networks,
			MQTT:           mqttkit.DefaultConfig(),
			EventPublisher: eventPublisher,
			Now:            func() time.Time { return now.Add(time.Second) },
		},
	}

	_, err := service.UpdateDeviceRuntime(context.Background(), UpdateDeviceRuntimeInput{
		DeviceID:       "device-a",
		NetworkID:      "net-1",
		ActivePath:     "relay_tcp",
		PathObservedAt: now.Add(time.Second).UnixMilli(),
		ReportedAtMS:   now.Add(time.Second).UnixMilli(),
	})
	if err != nil {
		t.Fatalf("UpdateDeviceRuntime returned error: %v", err)
	}
	if len(eventPublisher.events) != 0 {
		t.Fatalf("expected 0 network events, got %d", len(eventPublisher.events))
	}
}

func TestUpdateDeviceRuntimePublishesPresenceWhenPathChanges(t *testing.T) {
	now := time.Unix(1700003000, 0)
	eventPublisher := &deviceRuntimeTestEventPublisher{}
	devices := &deviceRegistrationTestDevices{
		networkRuntimeTestDevices: networkRuntimeTestDevices{
			devices: map[string]model.Device{
				"device-a": {
					DeviceID:   "device-a",
					OwnerID:    "user-1",
					VirtualIP:  "10.0.0.1",
					Platform:   "linux",
					Status:     "active",
					LastSeenAt: now.Unix(),
				},
			},
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
			networkDevices: map[string][]model.NetworkDevice{
				"net-1": {{
					NetworkID:          "net-1",
					DeviceID:           "device-a",
					Enabled:            true,
					MemberStatus:       model.NetworkMemberStatusActive,
					PresenceStatus:     model.DevicePresenceStatusActive,
					ActivePath:         "relay_udp",
					LastSeenAt:         now.Unix(),
					LastRuntimeStateAt: now.Unix(),
				}},
			},
		},
	}
	service := DeviceRuntimeAccessService{
		deviceCoreDependencies: deviceCoreDependencies{
			Users:          &deviceRegistrationTestUsers{users: map[string]model.User{"user-1": {UserID: "user-1", Email: "user-1@example.test", Status: "active"}}},
			Devices:        devices,
			Networks:       networks,
			MQTT:           mqttkit.DefaultConfig(),
			EventPublisher: eventPublisher,
			Now:            func() time.Time { return now.Add(time.Second) },
		},
	}

	_, err := service.UpdateDeviceRuntime(context.Background(), UpdateDeviceRuntimeInput{
		DeviceID:       "device-a",
		NetworkID:      "net-1",
		ActivePath:     "relay_tcp",
		PathObservedAt: now.Add(time.Second).UnixMilli(),
		ReportedAtMS:   now.Add(time.Second).UnixMilli(),
	})
	if err != nil {
		t.Fatalf("UpdateDeviceRuntime returned error: %v", err)
	}
	if len(eventPublisher.events) != 1 {
		t.Fatalf("expected 1 network event, got %d", len(eventPublisher.events))
	}
}
