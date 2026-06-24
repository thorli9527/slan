package service

import (
	"context"
	"strings"
	"time"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

func attachDeviceToDefaultNetwork(ctx context.Context, networks repository.NetworkRepository, ownerID, deviceID string, now int64) error {
	items, err := networks.ListNetworksByOwner(ctx, ownerID)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}
	target := items[0]
	for _, network := range items {
		if network.Default && network.Status == "active" {
			target = network
			break
		}
	}
	return networks.SaveNetworkDevice(ctx, model.NetworkDevice{
		NetworkID: target.NetworkID,
		DeviceID:  deviceID,
		Enabled:   true,
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	})
}

func registerManagedDevice(ctx context.Context, users repository.UserRepository, devices repository.DeviceRepository, networks repository.NetworkRepository, nowFn func() time.Time, newID func() string, input RegisterDeviceInput) (model.Device, error) {
	input = normalizeRegisterDeviceInput(input)
	if input.OwnerID == "" || input.Name == "" || input.Platform == "" {
		return model.Device{}, ErrInvalidArgument
	}
	if _, ok, err := users.GetUser(ctx, input.OwnerID); err != nil {
		return model.Device{}, err
	} else if !ok {
		return model.Device{}, ErrNotFound
	}
	if input.ActorUserID != "" {
		if _, ok, err := users.GetUser(ctx, input.ActorUserID); err != nil {
			return model.Device{}, err
		} else if !ok || input.ActorUserID != input.OwnerID {
			return model.Device{}, ErrUnauthorized
		}
	}
	now := deviceNow(nowFn).Unix()
	deviceID := strings.TrimSpace(input.DeviceID)
	if deviceID == "" {
		deviceID = newManagedDeviceID(newID)
	}
	device := model.Device{
		DeviceID:      deviceID,
		OwnerID:       input.OwnerID,
		Name:          input.Name,
		Platform:      input.Platform,
		Alias:         input.Alias,
		OSName:        input.OSName,
		OSVersion:     input.OSVersion,
		PublicKey:     input.PublicKey,
		DeviceVersion: input.DeviceVersion,
		CountryCode:   input.CountryCode,
		Status:        "active",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := devices.SaveDevice(ctx, device); err != nil {
		return model.Device{}, err
	}
	if err := attachDeviceToDefaultNetwork(ctx, networks, device.OwnerID, device.DeviceID, now); err != nil {
		return model.Device{}, err
	}
	return device, nil
}
