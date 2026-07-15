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

type DeviceRelationRepository interface {
	ListDeviceRelationsByUser(ctx context.Context, userID string) ([]model.DeviceUserRelation, error)
	ListDeviceRelationsByDevice(ctx context.Context, deviceID string) ([]model.DeviceUserRelation, error)
	GetDeviceUserRelation(ctx context.Context, deviceID, userID string) (model.DeviceUserRelation, bool, error)
	SaveDeviceUserRelation(ctx context.Context, relation model.DeviceUserRelation) error
	RevokeDeviceUserRelation(ctx context.Context, deviceID, userID, revokedBy string, revokedAt int64) error
	SaveDeviceInviteWithRelation(ctx context.Context, invite model.DeviceInvite, relation model.DeviceUserRelation) error
	RevokeDeviceInviteWithRelation(ctx context.Context, invite model.DeviceInvite, sharedUserID, revokedBy string, revokedAt int64) error
}
