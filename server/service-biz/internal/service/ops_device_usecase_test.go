package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

type opsDeviceIPStore struct {
	*deviceSessionTestDevices
	networks *deviceSessionTestNetworks
}

func (s *opsDeviceIPStore) ListAllDevices(context.Context) ([]model.Device, error) {
	items := make([]model.Device, 0, len(s.devices))
	for _, item := range s.devices {
		items = append(items, item)
	}
	return items, nil
}

func (s *opsDeviceIPStore) UpdateDeviceVirtualIP(_ context.Context, deviceID, virtualIP string, updatedAt int64) error {
	for existingID, item := range s.devices {
		if existingID != deviceID && item.VirtualIP == virtualIP {
			return repository.ErrDeviceVirtualIPConflict
		}
	}
	item, ok := s.devices[deviceID]
	if !ok {
		return ErrNotFound
	}
	item.VirtualIP = virtualIP
	item.UpdatedAt = updatedAt
	s.devices[deviceID] = item
	for networkID, members := range s.networks.networkDevices {
		for index := range members {
			if members[index].DeviceID == deviceID {
				members[index].VirtualIP = virtualIP
				members[index].UpdatedAt = updatedAt
			}
		}
		s.networks.networkDevices[networkID] = members
	}
	return nil
}

func TestDisablingManagedDeviceDeletesSessionsAndWritesAudit(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	devices := &deviceSessionTestDevices{
		networkRuntimeTestDevices: networkRuntimeTestDevices{devices: map[string]model.Device{
			"device-1": {DeviceID: "device-1", Name: "gateway", Status: "active"},
		}},
		savedSessions: []model.DeviceSession{{SessionID: "session-1", DeviceID: "device-1", AccessToken: "access-1"}},
	}
	audit := &deviceCredentialTestAudit{}
	devicePublisher := &networkAccessTestDevicePublisher{}
	service := OpsManagedDeviceService{
		Devices: devices, Audit: audit, DevicePublisher: devicePublisher,
		Networks: &deviceSessionTestNetworks{networkRuntimeTestNetworks: networkRuntimeTestNetworks{
			networks: map[string]model.Network{}, networkDevices: map[string][]model.NetworkDevice{},
		}},
		Now: func() time.Time { return now },
	}
	ctx := WithAuthenticatedOperator(context.Background(), "operator-1")

	view, err := service.UpdateDevice(ctx, UpdateDeviceInput{DeviceID: "device-1", Status: "disabled"})
	if err != nil {
		t.Fatal(err)
	}
	if view.Device.Status != "disabled" || len(devices.savedSessions) != 0 {
		t.Fatalf("disabled device retained session: view=%+v sessions=%+v", view, devices.savedSessions)
	}
	if len(audit.events) != 1 || audit.events[0].Action != "disable" || audit.events[0].ActorID != "operator-1" || audit.events[0].ResourceID != "device-1" {
		t.Fatalf("unexpected device disable audit: %#v", audit.events)
	}
	if len(devicePublisher.events) != 1 || devicePublisher.deviceIDs[0] != "device-1" || devicePublisher.events[0].Type != "device_disabled" {
		t.Fatalf("expected device disabled MQTT event, got ids=%v events=%#v", devicePublisher.deviceIDs, devicePublisher.events)
	}
	reenabled, err := service.UpdateDevice(ctx, UpdateDeviceInput{DeviceID: "device-1", Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	if reenabled.Device.Status != "active" || len(devices.savedSessions) != 0 {
		t.Fatalf("reenabled device restored old session: view=%+v sessions=%+v", reenabled, devices.savedSessions)
	}
	if len(audit.events) != 2 || audit.events[1].Action != "enable" {
		t.Fatalf("unexpected device enable audit: %#v", audit.events)
	}
}

func TestUpdateManagedDeviceRejectsInvalidStatus(t *testing.T) {
	service := OpsManagedDeviceService{}
	if _, err := service.UpdateDevice(context.Background(), UpdateDeviceInput{DeviceID: "device-1", Status: "pending"}); err != ErrInvalidArgument {
		t.Fatalf("invalid device status error = %v, want %v", err, ErrInvalidArgument)
	}
}

func TestUpdateManagedDeviceRejectsReservedVirtualIP(t *testing.T) {
	devices := newOpsDeviceIPTestStore()
	service := OpsManagedDeviceService{Devices: devices, Inventory: devices, Networks: devices.networks}

	_, err := service.UpdateDevice(context.Background(), UpdateDeviceInput{DeviceID: "device-1", VirtualIP: "10.0.0.53"})
	if !errors.Is(err, ErrInvalidArgument) {
		t.Fatalf("reserved IP error = %v, want %v", err, ErrInvalidArgument)
	}
}

func TestUpdateManagedDeviceRejectsDuplicateVirtualIP(t *testing.T) {
	devices := newOpsDeviceIPTestStore()
	service := OpsManagedDeviceService{Devices: devices, Inventory: devices, Networks: devices.networks}

	_, err := service.UpdateDevice(context.Background(), UpdateDeviceInput{DeviceID: "device-1", VirtualIP: "10.0.1.2"})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("duplicate IP error = %v, want %v", err, ErrConflict)
	}
}

func TestUpdateManagedDeviceSynchronizesVirtualIP(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	devices := newOpsDeviceIPTestStore()
	service := OpsManagedDeviceService{
		Devices: devices, Inventory: devices, Networks: devices.networks,
		Now: func() time.Time { return now },
	}

	view, err := service.UpdateDevice(context.Background(), UpdateDeviceInput{DeviceID: "device-1", VirtualIP: "100.0.1.20"})
	if err != nil {
		t.Fatal(err)
	}
	if view.GlobalIP != "100.0.1.20" || devices.devices["device-1"].VirtualIP != "100.0.1.20" {
		t.Fatalf("device IP was not updated: view=%+v device=%+v", view, devices.devices["device-1"])
	}
	membership := devices.networks.networkDevices["network-1"][0]
	if membership.VirtualIP != "100.0.1.20" {
		t.Fatalf("network membership IP = %q, want 100.0.1.20", membership.VirtualIP)
	}
}

func newOpsDeviceIPTestStore() *opsDeviceIPStore {
	networks := &deviceSessionTestNetworks{networkRuntimeTestNetworks: networkRuntimeTestNetworks{
		networks: map[string]model.Network{
			"network-1": {NetworkID: "network-1", Name: "Default", Status: "active"},
		},
		networkDevices: map[string][]model.NetworkDevice{
			"network-1": {{NetworkID: "network-1", DeviceID: "device-1", VirtualIP: "10.0.1.1", Enabled: true, MemberStatus: model.NetworkMemberStatusActive}},
		},
	}}
	return &opsDeviceIPStore{
		deviceSessionTestDevices: &deviceSessionTestDevices{networkRuntimeTestDevices: networkRuntimeTestDevices{devices: map[string]model.Device{
			"device-1": {DeviceID: "device-1", Name: "one", VirtualIP: "10.0.1.1", Status: "active"},
			"device-2": {DeviceID: "device-2", Name: "two", VirtualIP: "10.0.1.2", Status: "active"},
		}}},
		networks: networks,
	}
}
