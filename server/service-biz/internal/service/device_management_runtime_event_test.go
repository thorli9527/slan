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
