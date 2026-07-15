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

	items, err := listVisibleManagedDevices(context.Background(), devices, relations, "inviter")
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
	}); err != ErrUnauthorized {
		t.Fatalf("non-owner delete error = %v, want %v", err, ErrUnauthorized)
	}
}
