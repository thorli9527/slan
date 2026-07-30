package repository

import (
	"context"

	"github.com/slan/service-biz/internal/model"
)

type DeviceBootstrapRepository interface {
	ListDeviceBootstrapKeys(ctx context.Context, userID string) ([]model.DeviceBootstrapKey, error)
	GetDeviceBootstrapKey(ctx context.Context, keyID string) (model.DeviceBootstrapKey, bool, error)
	GetDeviceBootstrapKeyByToken(ctx context.Context, token string) (model.DeviceBootstrapKey, bool, error)
	SaveDeviceBootstrapKey(ctx context.Context, key model.DeviceBootstrapKey) error
}

type DeviceBootstrapCleanupRepository interface {
	DeleteExpiredDeviceBootstrapKeys(ctx context.Context, expiresBefore int64) (int64, error)
}
