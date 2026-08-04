package repository

import (
	"context"
	"errors"

	"github.com/slan/service-biz/internal/model"
)

var ErrDeviceVirtualIPConflict = errors.New("device virtual IP already exists")

type DeviceVirtualIPRepository interface {
	UpdateDeviceVirtualIP(ctx context.Context, deviceID, virtualIP string, updatedAt int64) error
}

type DeviceCoreRepository interface {
	GetDevice(ctx context.Context, deviceID string) (model.Device, bool, error)
	SaveDevice(ctx context.Context, device model.Device) error
	DeleteDevice(ctx context.Context, deviceID string) error
	NewDeviceVirtualIPID() string
	GetDeviceSessionByAccessToken(ctx context.Context, accessToken string) (model.DeviceSession, bool, error)
	GetDeviceSessionByRefreshToken(ctx context.Context, refreshToken string) (model.DeviceSession, bool, error)
	ListDeviceSessionsByDeviceID(ctx context.Context, deviceID string) ([]model.DeviceSession, error)
	SaveDeviceSession(ctx context.Context, item model.DeviceSession) error
	RotateDeviceSession(ctx context.Context, currentRefreshToken string, next model.DeviceSession) (bool, error)
	DeleteDeviceSessionForRefreshReuse(ctx context.Context, sessionID, previousRefreshTokenHash string, now int64) (bool, error)
	DeleteDeviceSessionByAccessToken(ctx context.Context, accessToken string) error
}
