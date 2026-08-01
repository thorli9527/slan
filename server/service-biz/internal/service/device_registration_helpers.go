package service

import (
	"context"
	"strings"
	"time"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

func registerInstalledDevice(
	ctx context.Context,
	users repository.UserRepository,
	devices repository.DeviceRepository,
	nowFn func() time.Time,
	input RegisterDeviceInput,
) (model.Device, error) {
	input = normalizeRegisterDeviceInput(input)
	if input.OwnerID == "" || input.DeviceID == "" || input.Name == "" || input.Platform == "" {
		return model.Device{}, ErrInvalidArgument
	}
	if _, ok, err := users.GetUser(ctx, input.OwnerID); err != nil {
		return model.Device{}, err
	} else if !ok {
		return model.Device{}, ErrNotFound
	}
	now := deviceNow(nowFn).Unix()
	existing, exists, err := devices.GetDevice(ctx, input.DeviceID)
	if err != nil {
		return model.Device{}, err
	}
	device := model.Device{
		DeviceID:      input.DeviceID,
		OwnerID:       input.OwnerID,
		Name:          input.Name,
		Platform:      input.Platform,
		Alias:         "",
		OSName:        input.OSName,
		OSVersion:     input.OSVersion,
		PublicKey:     input.PublicKey,
		DeviceVersion: input.DeviceVersion,
		CountryCode:   input.CountryCode,
		Status:        "active",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if exists {
		device.Alias = existing.Alias
		device.VirtualIP = existing.VirtualIP
		device.CreatedAt = existing.CreatedAt
		device.LastSeenAt = existing.LastSeenAt
		device.RXBytesTotal = existing.RXBytesTotal
		device.TXBytesTotal = existing.TXBytesTotal
	}
	if !managedDeviceVirtualIP(device.VirtualIP) {
		device.VirtualIP, err = allocateDeviceVirtualIP(devices)
		if err != nil {
			return model.Device{}, err
		}
	}
	if err := devices.SaveDevice(ctx, device); err != nil {
		return model.Device{}, err
	}
	return device, nil
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
			return model.Device{}, ErrForbidden
		}
	}
	now := deviceNow(nowFn).Unix()
	deviceID := strings.TrimSpace(input.DeviceID)
	if deviceID == "" {
		deviceID = newManagedDeviceID(newID)
	}
	existing, exists, err := devices.GetDevice(ctx, deviceID)
	if err != nil {
		return model.Device{}, err
	}
	virtualIP := strings.TrimSpace(existing.VirtualIP)
	if !managedDeviceVirtualIP(virtualIP) {
		virtualIP, err = allocateDeviceVirtualIP(devices)
		if err != nil {
			return model.Device{}, err
		}
	}
	device := model.Device{
		DeviceID:      deviceID,
		OwnerID:       input.OwnerID,
		VirtualIP:     virtualIP,
		Name:          input.Name,
		Platform:      input.Platform,
		Alias:         "",
		OSName:        input.OSName,
		OSVersion:     input.OSVersion,
		PublicKey:     input.PublicKey,
		DeviceVersion: input.DeviceVersion,
		CountryCode:   input.CountryCode,
		Status:        "active",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if exists {
		device.Alias = existing.Alias
		device.CreatedAt = existing.CreatedAt
		if device.CreatedAt == 0 {
			device.CreatedAt = now
		}
		device.RXBytesTotal = existing.RXBytesTotal
		device.TXBytesTotal = existing.TXBytesTotal
		device.LastSeenAt = existing.LastSeenAt
	}
	if err := devices.SaveDevice(ctx, device); err != nil {
		return model.Device{}, err
	}
	return device, nil
}
