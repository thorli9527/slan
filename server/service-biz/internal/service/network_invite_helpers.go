package service

import (
	"context"
	"time"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

func ensureNetworkDeviceAlias(ctx context.Context, devices repository.DeviceRepository, nowFn func() time.Time, actorUserID, deviceID, alias string) error {
	return ensureDeviceAlias(ctx, deviceAliasStore{Devices: devices}, func() int64 { return networkNow(nowFn).Unix() }, actorUserID, deviceID, alias)
}

func ensureDeviceAlias(ctx context.Context, devices deviceAliasStore, nowFn func() int64, actorUserID, deviceID, alias string) error {
	if alias == "" || deviceID == "" {
		return nil
	}
	device, err := requireManagedDevice(ctx, devices.Devices, deviceID)
	if err != nil {
		return err
	}
	if actorUserID != "" && device.OwnerID != actorUserID {
		return ErrForbidden
	}
	if device.Alias == alias {
		return nil
	}
	device.Alias = alias
	device.UpdatedAt = nowFn()
	return devices.Devices.SaveDevice(ctx, device)
}

type deviceAliasStore struct {
	Devices repository.DeviceRepository
}

func findNetworkDeviceMembership(items []model.NetworkDevice, deviceID string) (model.NetworkDevice, bool) {
	for _, item := range items {
		if item.DeviceID == deviceID {
			return item, true
		}
	}
	return model.NetworkDevice{}, false
}
