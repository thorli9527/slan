package repository

import (
	"context"

	"github.com/slan/service-biz/internal/model"
)

type DeviceCoreRepository interface {
	ListDevicesByOwner(ctx context.Context, ownerID string) ([]model.Device, error)
	GetDevice(ctx context.Context, deviceID string) (model.Device, bool, error)
	SaveDevice(ctx context.Context, device model.Device) error
	DeleteDevice(ctx context.Context, deviceID string) error
	NewDeviceVirtualIPID() string
	GetDeviceLoginDevice(ctx context.Context, deviceID string) (model.DeviceLoginDevice, bool, error)
	SaveDeviceLoginDevice(ctx context.Context, item model.DeviceLoginDevice) error
	GetDeviceSessionByAccessToken(ctx context.Context, accessToken string) (model.DeviceSession, bool, error)
	GetDeviceSessionByRefreshToken(ctx context.Context, refreshToken string) (model.DeviceSession, bool, error)
	ListDeviceSessionsByDeviceID(ctx context.Context, deviceID string) ([]model.DeviceSession, error)
	SaveDeviceSession(ctx context.Context, item model.DeviceSession) error
	DeleteDeviceSessionByAccessToken(ctx context.Context, accessToken string) error
}
