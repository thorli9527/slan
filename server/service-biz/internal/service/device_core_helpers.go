package service

import (
	"context"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

func listOwnedManagedDevices(ctx context.Context, devices repository.DeviceRepository, ownerID string) ([]model.Device, error) {
	return devices.ListDevicesByOwner(ctx, normalizeDeviceOwnerID(ownerID))
}

func listVisibleManagedDevices(
	ctx context.Context,
	devices repository.DeviceRepository,
	relations repository.DeviceRelationRepository,
	userID string,
) ([]model.Device, error) {
	userID = normalizeDeviceOwnerID(userID)
	relationItems, err := relations.ListDeviceRelationsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	items := make([]model.Device, 0, len(relationItems))
	for _, relation := range relationItems {
		device, ok, err := devices.GetDevice(ctx, relation.DeviceID)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		items = append(items, device)
	}
	return items, nil
}
