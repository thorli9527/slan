package repository

import (
	"context"
	"strings"

	"github.com/slan/service-biz/internal/model"
	"gorm.io/gorm"
)

func (s *GormStore) ListDevicesByOwner(_ context.Context, ownerID string) ([]model.Device, error) {
	return listModels(s.db.Where("owner_id = ?", ownerID).Order("device_id asc"), func(row gormDeviceRecord) model.Device {
		return row.model()
	})
}

func (s *GormStore) GetDevice(_ context.Context, deviceID string) (model.Device, bool, error) {
	return firstModel(s.db.Where("device_id = ?", deviceID), func(row gormDeviceRecord) model.Device {
		return row.model()
	})
}

func (s *GormStore) SaveDevice(_ context.Context, device model.Device) error {
	row := deviceRecordFromModel(device)
	return upsertByColumns(s.db, &row, []string{"device_id"}, []string{"owner_id", "virtual_ip", "name", "platform", "alias", "os_name", "os_version", "public_key", "device_version", "country_code", "rx_bytes_total", "tx_bytes_total", "status", "created_at", "updated_at", "last_seen_at"})
}

func (s *GormStore) DeleteDevice(_ context.Context, deviceID string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&gormDeviceGroupAssignmentRecord{}, "device_id = ?", deviceID).Error; err != nil {
			return err
		}
		if err := tx.Delete(&gormDeviceLoginRecord{}, "device_id = ?", deviceID).Error; err != nil {
			return err
		}
		return tx.Delete(&gormDeviceRecord{}, "device_id = ?", deviceID).Error
	})
}

func (s *GormStore) GetDeviceLoginDevice(_ context.Context, deviceID string) (model.DeviceLoginDevice, bool, error) {
	return firstModel(s.db.Where("device_id = ?", deviceID), func(row gormDeviceLoginRecord) model.DeviceLoginDevice {
		return row.model()
	})
}

func (s *GormStore) SaveDeviceLoginDevice(_ context.Context, item model.DeviceLoginDevice) error {
	row := deviceLoginRecordFromModel(item)
	return upsertByColumns(s.db, &row, []string{"device_id"}, []string{"user_id", "name", "platform", "verify_code", "status", "expires_at", "created_at", "updated_at"})
}

func (s *GormStore) GetDeviceSessionByAccessToken(_ context.Context, accessToken string) (model.DeviceSession, bool, error) {
	return firstModel(s.db.Where("access_token = ?", strings.TrimSpace(accessToken)), func(row gormDeviceSessionRecord) model.DeviceSession {
		return row.model()
	})
}

func (s *GormStore) GetDeviceSessionByRefreshToken(_ context.Context, refreshToken string) (model.DeviceSession, bool, error) {
	return firstModel(s.db.Where("refresh_token = ?", strings.TrimSpace(refreshToken)), func(row gormDeviceSessionRecord) model.DeviceSession {
		return row.model()
	})
}

func (s *GormStore) ListDeviceSessionsByDeviceID(_ context.Context, deviceID string) ([]model.DeviceSession, error) {
	return listModels(s.db.Where("device_id = ?", strings.TrimSpace(deviceID)).Order("created_at desc"), func(row gormDeviceSessionRecord) model.DeviceSession {
		return row.model()
	})
}

func (s *GormStore) SaveDeviceSession(_ context.Context, item model.DeviceSession) error {
	row := deviceSessionRecordFromModel(item)
	return upsertByColumns(s.db, &row, []string{"session_id"}, []string{"device_id", "access_token", "refresh_token", "status", "session_mode", "expires_at", "refresh_expiry", "created_at", "updated_at", "revoked_at"})
}

func (s *GormStore) DeleteDeviceSessionByAccessToken(_ context.Context, accessToken string) error {
	return s.db.Delete(&gormDeviceSessionRecord{}, "access_token = ?", strings.TrimSpace(accessToken)).Error
}
