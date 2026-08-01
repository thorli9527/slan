package service

import (
	"context"
	"testing"
	"time"

	"github.com/slan/service-biz/internal/model"
)

func TestOpsNetworkMembershipPublishesBroadcastSnapshotAndDeviceControl(t *testing.T) {
	users := &networkAccessTestUsers{users: map[string]model.User{
		"user-1": {UserID: "user-1", Email: "owner@example.com", Status: "active"},
	}}
	devices := &networkAccessTestDevices{devices: map[string]model.Device{
		"dev-1": {DeviceID: "dev-1", OwnerID: "user-1", Name: "Device 1", Status: "active", VirtualIP: "10.0.1.20"},
	}}
	networks := &networkAccessTestNetworks{
		networks: map[string]model.Network{
			"net-1": {NetworkID: "net-1", OwnerID: "user-1", Name: "Network 1", Status: "active"},
		},
		networkDevices: map[string][]model.NetworkDevice{},
	}
	events := &networkAccessTestBroadcaster{}
	deviceEvents := &networkAccessTestDevicePublisher{}
	svc := NetworkInviteService{
		Users: users, Devices: devices, Networks: networks,
		EventPublisher: events, DevicePublisher: deviceEvents,
		Now: func() time.Time { return time.Unix(1700010000, 0) },
	}

	memberView, err := svc.AddNetworkDevice(context.Background(), AddNetworkDeviceInput{
		NetworkID: "net-1", DeviceID: "dev-1", ActorUserID: "user-1",
	})
	if err != nil {
		t.Fatalf("AddNetworkDevice returned error: %v", err)
	}
	if member := networks.networkDevices["net-1"][0]; member.MembershipSource != model.NetworkMembershipSourceDirect {
		t.Fatalf("expected direct membership source, got %q (view=%+v)", member.MembershipSource, memberView)
	}
	assertMembershipEventSequence(t, events.events, NetworkEventMemberAdded)
	if len(deviceEvents.events) != 1 || deviceEvents.events[0].Payload["operation"] != "joined" {
		t.Fatalf("expected joined device control event, got %#v", deviceEvents.events)
	}
	groupService := DeviceGroupService{
		Users: users, Devices: devices, Networks: networks, NetworkGroups: networks,
		EventPublisher: events, DevicePublisher: deviceEvents,
		Now: func() time.Time { return time.Unix(1700010001, 0) },
	}
	if err := groupService.syncNetworkDeviceGroupMemberships(context.Background(), networks.networks["net-1"], "test_group_sync"); err != nil {
		t.Fatalf("group membership sync returned error: %v", err)
	}
	if _, ok, err := networks.GetNetworkDevice(context.Background(), "net-1", "dev-1"); err != nil || !ok {
		t.Fatalf("direct member must survive group sync: ok=%v err=%v", ok, err)
	}

	events.events = nil
	if err := svc.RemoveNetworkDevice(context.Background(), RemoveNetworkDeviceInput{
		NetworkID: "net-1", DeviceID: "dev-1", ActorUserID: "user-1",
	}); err != nil {
		t.Fatalf("RemoveNetworkDevice returned error: %v", err)
	}
	assertMembershipEventSequence(t, events.events, NetworkEventMemberRemoved)
	if len(deviceEvents.events) != 2 || deviceEvents.events[1].Payload["operation"] != "left" {
		t.Fatalf("expected left device control event, got %#v", deviceEvents.events)
	}
	if got := deviceEvents.events[1].Payload["networkIds"]; len(got.([]string)) != 0 {
		t.Fatalf("expected remaining network list to be empty, got %#v", got)
	}
	excluded, ok, err := networks.GetNetworkDevice(context.Background(), "net-1", "dev-1")
	if err != nil || !ok || excluded.MembershipSource != model.NetworkMembershipSourceExcluded || networkMemberActive(excluded) {
		t.Fatalf("expected persistent excluded membership, item=%+v ok=%v err=%v", excluded, ok, err)
	}
	if err := groupService.syncNetworkDeviceGroupMemberships(context.Background(), networks.networks["net-1"], "test_group_sync_after_remove"); err != nil {
		t.Fatalf("group membership sync after removal returned error: %v", err)
	}
	excluded, ok, err = networks.GetNetworkDevice(context.Background(), "net-1", "dev-1")
	if err != nil || !ok || networkMemberActive(excluded) {
		t.Fatalf("excluded member must not be reactivated by group sync, item=%+v ok=%v err=%v", excluded, ok, err)
	}
}

func TestOpsNetworkMembershipRejectsDeviceOwnedByAnotherUser(t *testing.T) {
	svc := NetworkInviteService{
		Users: &networkAccessTestUsers{users: map[string]model.User{
			"user-1": {UserID: "user-1", Status: "active"},
		}},
		Devices: &networkAccessTestDevices{devices: map[string]model.Device{
			"dev-2": {DeviceID: "dev-2", OwnerID: "user-2", Status: "active"},
		}},
		Networks: &networkAccessTestNetworks{networks: map[string]model.Network{
			"net-1": {NetworkID: "net-1", OwnerID: "user-1", Status: "active"},
		}},
	}
	_, err := svc.AddNetworkDevice(context.Background(), AddNetworkDeviceInput{
		NetworkID: "net-1", DeviceID: "dev-2", ActorUserID: "user-1",
	})
	if err != ErrForbidden {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func assertMembershipEventSequence(t *testing.T, events []NetworkEventEnvelope, memberType NetworkEventType) {
	t.Helper()
	if len(events) != 3 {
		t.Fatalf("expected config, member and snapshot events, got %#v", events)
	}
	want := []NetworkEventType{NetworkEventConfigChanged, memberType, NetworkEventSnapshot}
	for i, eventType := range want {
		if events[i].EventType != eventType {
			t.Fatalf("event %d: expected %s, got %s", i, eventType, events[i].EventType)
		}
	}
}
