package service

import (
	"context"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

func listOwnedManagedDevices(ctx context.Context, devices repository.DeviceRepository, ownerID string) ([]model.Device, error) {
	return devices.ListDevicesByOwner(ctx, normalizeDeviceOwnerID(ownerID))
}
