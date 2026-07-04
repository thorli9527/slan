package service

import (
	"context"
	"testing"
	"time"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/pkg/mqttkit"
)

type deviceRuntimeBroadcastTestBroadcaster struct {
	calls         []networkBroadcastMemberState
	presenceCalls []networkBroadcastDevicePresenceChanged
}

func (s *deviceRuntimeBroadcastTestBroadcaster) PublishNetworkMemberStateChanged(_ context.Context, state networkBroadcastMemberState) error {
	s.calls = append(s.calls, state)
	return nil
}

func (s *deviceRuntimeBroadcastTestBroadcaster) PublishNetworkConfigChanged(_ context.Context, _ networkBroadcastConfigChanged) error {
	return nil
}

func (s *deviceRuntimeBroadcastTestBroadcaster) PublishNetworkSnapshot(_ context.Context, _ networkBroadcastSnapshot) error {
	return nil
}

func (s *deviceRuntimeBroadcastTestBroadcaster) PublishDNSChanged(_ context.Context, _ networkBroadcastDNSChanged) error {
	return nil
}

func (s *deviceRuntimeBroadcastTestBroadcaster) PublishACLChanged(_ context.Context, _ networkBroadcastACLChanged) error {
	return nil
}

func (s *deviceRuntimeBroadcastTestBroadcaster) PublishNetworkMemberChanged(_ context.Context, _ networkBroadcastMemberChanged) error {
	return nil
}

func (s *deviceRuntimeBroadcastTestBroadcaster) PublishDevicePresenceChanged(_ context.Context, state networkBroadcastDevicePresenceChanged) error {
	s.presenceCalls = append(s.presenceCalls, state)
	return nil
}

func TestUpdateDeviceRuntimePublishesNetworkMemberStateChanged(t *testing.T) {
	now := time.Unix(1700003000, 0)
	broadcaster := &deviceRuntimeBroadcastTestBroadcaster{}
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
			Users:       &deviceRegistrationTestUsers{users: map[string]model.User{"user-1": {UserID: "user-1", Email: "user-1@example.test", Status: "active"}}},
			Devices:     devices,
			Networks:    networks,
			MQTT:        mqttkit.DefaultConfig(),
			Broadcaster: broadcaster,
			Now:         func() time.Time { return now },
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
	if len(broadcaster.calls) != 1 {
		t.Fatalf("expected 1 broadcast call, got %d", len(broadcaster.calls))
	}
	call := broadcaster.calls[0]
	if call.Network.NetworkID != "net-1" {
		t.Fatalf("expected network net-1, got %q", call.Network.NetworkID)
	}
	if call.Device.DeviceID != "device-a" {
		t.Fatalf("expected device device-a, got %q", call.Device.DeviceID)
	}
	if call.PrefixLen != 24 {
		t.Fatalf("expected prefix len 24, got %d", call.PrefixLen)
	}
	if !call.Online {
		t.Fatalf("expected online=true")
	}
	if len(broadcaster.presenceCalls) != 1 {
		t.Fatalf("expected 1 presence broadcast call, got %d", len(broadcaster.presenceCalls))
	}
	if broadcaster.presenceCalls[0].PresenceStatus != model.DevicePresenceStatusActive {
		t.Fatalf("expected active presence status, got %q", broadcaster.presenceCalls[0].PresenceStatus)
	}
}
