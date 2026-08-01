package service

import (
	"context"
	"testing"
	"time"

	"github.com/slan/service-biz/internal/model"
)

func TestOpsDeleteNetworkPublishesRemovalAndDirectLeave(t *testing.T) {
	users := &networkAccessTestUsers{users: map[string]model.User{
		"user-1": {UserID: "user-1", Status: "active"},
	}}
	devices := &networkAccessTestDevices{devices: map[string]model.Device{
		"dev-1": {DeviceID: "dev-1", OwnerID: "user-1", Status: "active"},
		"dev-2": {DeviceID: "dev-2", OwnerID: "user-1", Status: "active"},
	}}
	networks := &networkAccessTestNetworks{
		networks: map[string]model.Network{
			"net-delete": {NetworkID: "net-delete", OwnerID: "user-1", Status: "active"},
			"net-keep":   {NetworkID: "net-keep", OwnerID: "user-1", Status: "active"},
		},
		versions: map[string]model.NetworkConfigVersion{
			"net-delete": {NetworkID: "net-delete", Version: 7},
		},
		networkDevices: map[string][]model.NetworkDevice{
			"net-delete": {
				{NetworkID: "net-delete", DeviceID: "dev-1", Enabled: true, MemberStatus: model.NetworkMemberStatusActive},
				{NetworkID: "net-delete", DeviceID: "dev-2", Enabled: true, MemberStatus: model.NetworkMemberStatusActive},
				{NetworkID: "net-delete", DeviceID: "dev-old", Enabled: false, MembershipSource: model.NetworkMembershipSourceExcluded},
			},
			"net-keep": {
				{NetworkID: "net-keep", DeviceID: "dev-1", Enabled: true, MemberStatus: model.NetworkMemberStatusActive},
			},
		},
	}
	events := &networkAccessTestBroadcaster{}
	deviceEvents := &networkAccessTestDevicePublisher{}
	service := NetworkCoreService{
		Users: users, Devices: devices, Networks: networks,
		EventPublisher: events, DevicePublisher: deviceEvents,
		Now: func() time.Time { return time.Unix(1700020000, 0) },
	}

	if err := service.DeleteNetwork(context.Background(), DeleteNetworkInput{
		NetworkID: "net-delete", ActorUserID: "user-1",
	}); err != nil {
		t.Fatalf("DeleteNetwork returned error: %v", err)
	}
	if _, exists := networks.networks["net-delete"]; exists {
		t.Fatal("network was not deleted")
	}
	if len(events.events) != 3 {
		t.Fatalf("expected config and two member removal events, got %#v", events.events)
	}
	if events.events[0].EventType != NetworkEventConfigChanged || events.events[0].Version != 8 {
		t.Fatalf("unexpected config event: %#v", events.events[0])
	}
	for index := 1; index < len(events.events); index++ {
		if events.events[index].EventType != NetworkEventMemberRemoved || events.events[index].Version != 8 {
			t.Fatalf("unexpected removal event %d: %#v", index, events.events[index])
		}
	}
	if events.events[1].EventID == events.events[2].EventID {
		t.Fatalf("distinct member removals share event ID %q", events.events[1].EventID)
	}
	removedDeviceIDs := map[string]bool{}
	for _, event := range events.events[1:] {
		payload, ok := event.Payload.(NetworkEventMemberRemovedPayload)
		if !ok {
			t.Fatalf("unexpected removal payload type: %#v", event.Payload)
		}
		removedDeviceIDs[payload.DeviceID] = true
	}
	if !removedDeviceIDs["dev-1"] || !removedDeviceIDs["dev-2"] {
		t.Fatalf("missing member removal payloads: %#v", removedDeviceIDs)
	}
	if len(deviceEvents.events) != 2 {
		t.Fatalf("expected two direct leave events, got %#v", deviceEvents.events)
	}
	for _, event := range deviceEvents.events {
		if event.Payload["operation"] != "left" || event.Payload["changedNetworkId"] != "net-delete" || event.Payload["membershipVersion"] != int64(8) {
			t.Fatalf("unexpected direct leave event: %#v", event)
		}
	}
	if got := deviceEvents.events[0].Payload["networkIds"].([]string); len(got) != 1 || got[0] != "net-keep" {
		t.Fatalf("expected dev-1 to retain net-keep, got %#v", got)
	}
}
