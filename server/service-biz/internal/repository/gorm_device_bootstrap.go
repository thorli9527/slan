package repository

import (
	"context"

	"github.com/slan/service-biz/internal/model"
)

func (s *GormStore) ListDeviceBootstrapKeys(_ context.Context, userID string) ([]model.DeviceBootstrapKey, error) {
	return listModels(s.db.Where("user_id = ?", userID).Order("key_id asc"), func(row gormBootstrapKeyRecord) model.DeviceBootstrapKey {
		return row.model()
	})
}

func (s *GormStore) GetDeviceBootstrapKey(_ context.Context, keyID string) (model.DeviceBootstrapKey, bool, error) {
	return firstModel(s.db.Where("key_id = ?", keyID), func(row gormBootstrapKeyRecord) model.DeviceBootstrapKey {
		return row.model()
	})
}

func (s *GormStore) GetDeviceBootstrapKeyByToken(_ context.Context, token string) (model.DeviceBootstrapKey, bool, error) {
	return firstModel(s.db.Where("token = ?", token), func(row gormBootstrapKeyRecord) model.DeviceBootstrapKey {
		return row.model()
	})
}

func (s *GormStore) SaveDeviceBootstrapKey(_ context.Context, key model.DeviceBootstrapKey) error {
	row := bootstrapKeyRecordFromModel(key)
	return upsertByColumns(s.db, &row, []string{"key_id"}, []string{"user_id", "name", "token", "status", "expires_at", "used_at", "used_by_device_id", "created_at", "updated_at", "revoked_at"})
}
