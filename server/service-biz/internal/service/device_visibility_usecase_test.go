package service

import (
	"context"
	"testing"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

type deviceVisibilityTestDevices struct {
	repository.DeviceRepository
	items map[string]model.Device
}

func (s deviceVisibilityTestDevices) ListDevicesByOwner(_ context.Context, ownerID string) ([]model.Device, error) {
	items := make([]model.Device, 0)
	for _, item := range s.items {
		if item.OwnerID == ownerID {
			items = append(items, item)
		}
	}
	return items, nil
}

func (s deviceVisibilityTestDevices) GetDevice(_ context.Context, deviceID string) (model.Device, bool, error) {
	item, ok := s.items[deviceID]
	return item, ok, nil
}

func (s deviceVisibilityTestDevices) SaveDevice(_ context.Context, device model.Device) error {
	s.items[device.DeviceID] = device
	return nil
}

type deviceVisibilityTestRelations struct {
	repository.DeviceRelationRepository
	relations []model.DeviceUserRelation
}

func (s deviceVisibilityTestRelations) ListDeviceRelationsByUser(_ context.Context, userID string) ([]model.DeviceUserRelation, error) {
	items := make([]model.DeviceUserRelation, 0)
	for _, relation := range s.relations {
		if relation.UserID == userID && relation.Status == model.DeviceRelationStatusActive {
			items = append(items, relation)
		}
	}
	return items, nil
}

func (s *deviceVisibilityTestRelations) GetDeviceUserRelation(_ context.Context, deviceID, userID string) (model.DeviceUserRelation, bool, error) {
	for _, relation := range s.relations {
		if relation.DeviceID == deviceID && relation.UserID == userID {
			return relation, true, nil
		}
	}
	return model.DeviceUserRelation{}, false, nil
}

func (s *deviceVisibilityTestRelations) SaveDeviceUserRelation(_ context.Context, updated model.DeviceUserRelation) error {
	for i, relation := range s.relations {
		if relation.DeviceID == updated.DeviceID && relation.UserID == updated.UserID {
			s.relations[i] = updated
			return nil
		}
	}
	s.relations = append(s.relations, updated)
	return nil
}

func TestListVisibleManagedDevicesIncludesAcceptedInvitesForInviter(t *testing.T) {
	devices := deviceVisibilityTestDevices{items: map[string]model.Device{
		"owned-device":   {DeviceID: "owned-device", OwnerID: "inviter"},
		"invited-device": {DeviceID: "invited-device", OwnerID: "acceptor"},
		"pending-device": {DeviceID: "pending-device", OwnerID: "acceptor"},
		"other-device":   {DeviceID: "other-device", OwnerID: "inviter-2"},
	}}
	relations := deviceVisibilityTestRelations{relations: []model.DeviceUserRelation{
		{UserID: "inviter", DeviceID: "invited-device", Role: model.DeviceRelationRoleShared, Status: model.DeviceRelationStatusActive},
		{UserID: "inviter", DeviceID: "pending-device", Role: model.DeviceRelationRoleShared, Status: model.DeviceRelationStatusRevoked},
		{UserID: "inviter-2", DeviceID: "other-device", Role: model.DeviceRelationRoleShared, Status: model.DeviceRelationStatusActive},
		{UserID: "inviter", DeviceID: "owned-device", Role: model.DeviceRelationRoleOwner, Status: model.DeviceRelationStatusActive},
	}}

	items, err := listVisibleManagedDevices(context.Background(), devices, &relations, "inviter")
	if err != nil {
		t.Fatalf("list visible devices: %v", err)
	}
	visible := make(map[string]bool, len(items))
	for _, item := range items {
		visible[item.DeviceID] = true
	}

	if len(items) != 2 || !visible["owned-device"] || !visible["invited-device"] {
		t.Fatalf("expected owned and accepted invited devices, got %#v", visible)
	}
}

func TestUserDeviceDeletionIsRejectedForOwnerAndNonOwner(t *testing.T) {
	service := DeviceProvisioningService{deviceCoreDependencies: deviceCoreDependencies{
		Devices: deviceVisibilityTestDevices{items: map[string]model.Device{
			"device-1": {DeviceID: "device-1", OwnerID: "owner"},
		}},
	}}

	if err := service.DeleteDevice(context.Background(), DeleteDeviceInput{
		DeviceID: "device-1", ActorUserID: "owner",
	}); err != ErrForbidden {
		t.Fatalf("owner delete error = %v, want %v", err, ErrForbidden)
	}
	if err := service.DeleteDevice(context.Background(), DeleteDeviceInput{
		DeviceID: "device-1", ActorUserID: "other",
	}); err != ErrForbidden {
		t.Fatalf("non-owner delete error = %v, want %v", err, ErrForbidden)
	}
}

func TestSharedUserUpdatesOnlyTheirDeviceAlias(t *testing.T) {
	devices := deviceVisibilityTestDevices{items: map[string]model.Device{
		"device-1": {DeviceID: "device-1", OwnerID: "owner", Alias: "Owner alias"},
	}}
	relations := &deviceVisibilityTestRelations{relations: []model.DeviceUserRelation{
		{RelationID: "owner-relation", DeviceID: "device-1", UserID: "owner", Role: model.DeviceRelationRoleOwner, Status: model.DeviceRelationStatusActive},
		{RelationID: "shared-relation", DeviceID: "device-1", UserID: "shared-user", Role: model.DeviceRelationRoleShared, Status: model.DeviceRelationStatusActive},
	}}
	service := DeviceProvisioningService{deviceCoreDependencies: deviceCoreDependencies{
		Devices: devices, Relations: relations,
	}}

	device, alias, err := service.updateDeviceAliasEntity(context.Background(), UpdateDeviceAliasInput{
		DeviceID: "device-1", ActorUserID: "shared-user", Alias: "My private alias",
	})
	if err != nil {
		t.Fatalf("update shared alias: %v", err)
	}
	if alias != "My private alias" || device.Alias != "Owner alias" || devices.items["device-1"].Alias != "Owner alias" {
		t.Fatalf("shared alias changed device alias: alias=%q device=%q stored=%q", alias, device.Alias, devices.items["device-1"].Alias)
	}
	stored, ok, err := relations.GetDeviceUserRelation(context.Background(), "device-1", "shared-user")
	if err != nil || !ok || stored.Alias != "My private alias" {
		t.Fatalf("shared relation alias not saved: relation=%+v ok=%t err=%v", stored, ok, err)
	}
}

func TestVisibleDevicesUseCurrentUsersPrivateAlias(t *testing.T) {
	devices := deviceVisibilityTestDevices{items: map[string]model.Device{
		"device-1": {DeviceID: "device-1", OwnerID: "owner", Alias: "Owner alias"},
	}}
	relations := &deviceVisibilityTestRelations{relations: []model.DeviceUserRelation{
		{DeviceID: "device-1", UserID: "shared-user", Alias: "Shared alias", Role: model.DeviceRelationRoleShared, Status: model.DeviceRelationStatusActive},
	}}
	service := DeviceCatalogService{deviceCoreDependencies: deviceCoreDependencies{
		Devices: devices, Relations: relations,
	}}

	items, err := service.ListVisibleDevices(context.Background(), "shared-user")
	if err != nil {
		t.Fatalf("list visible devices: %v", err)
	}
	if len(items) != 1 || items[0].Alias != "Shared alias" {
		t.Fatalf("visible aliases = %+v, want shared alias", items)
	}
	if devices.items["device-1"].Alias != "Owner alias" {
		t.Fatalf("visible alias changed owner alias to %q", devices.items["device-1"].Alias)
	}
}
