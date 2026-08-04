package service

import (
	"context"
	"testing"
	"time"

	"github.com/slan/service-biz/internal/model"
)

func TestOpsResourcesCreateWithoutUserOwnership(t *testing.T) {
	eventPublisher := &networkAccessTestBroadcaster{}
	devices := &networkAccessTestDevices{devices: map[string]model.Device{
		"device-1": {DeviceID: "device-1", Name: "gateway", Status: "active"},
	}, deviceGroups: map[string]model.DeviceGroup{}}
	networks := &networkAccessTestNetworks{
		networks: map[string]model.Network{}, networkDevices: map[string][]model.NetworkDevice{},
		groupReferences: map[string][]model.NetworkDeviceGroupReference{},
	}
	service := OpsResourceService{
		Devices: devices, Networks: networks, NetworkGroups: networks,
		EventPublisher: eventPublisher,
		GroupRuntime:   DeviceGroupService{EventPublisher: eventPublisher},
		NewNetworkID:   func() string { return "net-1" }, Now: func() time.Time { return time.Unix(1_700_000_000, 0) },
	}

	network, err := service.CreateOpsNetwork(context.Background(), OpsNetworkInput{Name: "Production"})
	if err != nil {
		t.Fatalf("CreateOpsNetwork returned error: %v", err)
	}

	group, err := service.CreateOpsDeviceGroup(context.Background(), OpsDeviceGroupInput{Name: "Gateways"})
	if err != nil {
		t.Fatalf("CreateOpsDeviceGroup returned error: %v", err)
	}
	if err := service.AddOpsDeviceGroupMember(context.Background(), group.GroupID, "device-1"); err != nil {
		t.Fatalf("AddOpsDeviceGroupMember returned error: %v", err)
	}
	if err := service.AddOpsNetworkDeviceGroup(context.Background(), network.NetworkID, group.GroupID); err != nil {
		t.Fatalf("AddOpsNetworkDeviceGroup returned error: %v", err)
	}

	listed, err := service.ListOpsNetworks(context.Background())
	if err != nil {
		t.Fatalf("ListOpsNetworks returned error: %v", err)
	}
	if len(listed) != 1 || len(listed[0].DeviceIDs) != 1 || len(listed[0].DeviceGroupIDs) != 1 {
		t.Fatalf("unexpected ops network view: %+v", listed)
	}
	groups, err := service.ListOpsDeviceGroups(context.Background())
	if err != nil {
		t.Fatalf("ListOpsDeviceGroups returned error: %v", err)
	}
	if len(groups.Items) != 1 || len(groups.Members) != 1 || groups.Members[0].DeviceID != "device-1" {
		t.Fatalf("unexpected ops device group view: %+v", groups)
	}
	membership := networks.networkDevices[network.NetworkID][0]
	if membership.Direct || len(membership.DeviceGroupIDs) != 1 || membership.DeviceGroupIDs[0] != group.GroupID {
		t.Fatalf("expected group-derived membership only, got %+v", membership)
	}
	if err := service.RemoveOpsNetworkDeviceGroup(context.Background(), network.NetworkID, group.GroupID); err != nil {
		t.Fatalf("RemoveOpsNetworkDeviceGroup returned error: %v", err)
	}
	members := networks.networkDevices[network.NetworkID]
	if len(members) != 0 {
		t.Fatalf("removing the only group reference must remove derived membership, got %+v", members)
	}
}

func TestUpdateOpsNetworkPublishesConfigAndSnapshot(t *testing.T) {
	eventPublisher := &networkAccessTestBroadcaster{}
	devices := &networkAccessTestDevices{devices: map[string]model.Device{}}
	networks := &networkAccessTestNetworks{
		networks: map[string]model.Network{
			"net-1": {NetworkID: "net-1", Name: "Before", Status: "active"},
		},
		networkDevices:  map[string][]model.NetworkDevice{},
		groupReferences: map[string][]model.NetworkDeviceGroupReference{},
	}
	service := OpsResourceService{
		Devices: devices, Networks: networks, NetworkGroups: networks,
		EventPublisher: eventPublisher,
		Now:            func() time.Time { return time.Unix(1_700_000_100, 0) },
	}

	if _, err := service.UpdateOpsNetwork(context.Background(), "net-1", OpsNetworkInput{Name: "After"}); err != nil {
		t.Fatalf("UpdateOpsNetwork returned error: %v", err)
	}
	if len(eventPublisher.events) != 2 {
		t.Fatalf("expected config and snapshot events, got %d", len(eventPublisher.events))
	}
	if eventPublisher.events[0].EventType != NetworkEventConfigChanged || eventPublisher.events[1].EventType != NetworkEventSnapshot {
		t.Fatalf("unexpected network update event sequence: %#v", eventPublisher.events)
	}
}
