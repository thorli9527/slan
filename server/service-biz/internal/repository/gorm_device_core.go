package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"hash/fnv"
	"strings"

	"github.com/slan/service-biz/internal/model"
	"gorm.io/gorm"
)

func (s *GormStore) ListAllDevices(_ context.Context) ([]model.Device, error) {
	return listModels(s.db.Order("created_at desc, device_id asc"), func(row gormDeviceRecord) model.Device { return row.model() })
}

func (s *GormStore) GetDevice(_ context.Context, deviceID string) (model.Device, bool, error) {
	return firstModel(s.db.Where("device_id = ?", deviceID), func(row gormDeviceRecord) model.Device { return row.model() })
}

func (s *GormStore) SaveDevice(_ context.Context, device model.Device) error {
	row := deviceRecordFromModel(device)
	err := upsertByColumns(s.db, &row, []string{"device_id"}, []string{"virtual_ip", "name", "platform", "alias", "os_name", "os_version", "public_key", "device_version", "public_ip", "country_code", "city_code", "geo_updated_at", "rx_bytes_total", "tx_bytes_total", "status", "created_at", "updated_at", "last_seen_at"})
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return ErrDeviceVirtualIPConflict
	}
	return err
}

func (s *GormStore) UpdateDeviceVirtualIP(_ context.Context, deviceID, virtualIP string, updatedAt int64) error {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&gormDeviceRecord{}).
			Where("device_id = ?", strings.TrimSpace(deviceID)).
			Updates(map[string]any{"virtual_ip": strings.TrimSpace(virtualIP), "updated_at": updatedAt})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		return tx.Model(&gormNetworkDeviceRecord{}).
			Where("device_id = ?", strings.TrimSpace(deviceID)).
			Updates(map[string]any{"virtual_ip": strings.TrimSpace(virtualIP), "updated_at": updatedAt}).Error
	})
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return ErrDeviceVirtualIPConflict
	}
	return err
}

func (s *GormStore) DeleteDevice(_ context.Context, deviceID string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		steps := []func() error{
			func() error { return tx.Delete(&gormDeviceCredentialRecord{}, "device_id = ?", deviceID).Error },
			func() error { return tx.Delete(&gormDeviceSessionRecord{}, "device_id = ?", deviceID).Error },
			func() error { return tx.Delete(&gormDeviceGroupAssignmentRecord{}, "device_id = ?", deviceID).Error },
			func() error { return tx.Delete(&gormNetworkDeviceRecord{}, "device_id = ?", deviceID).Error },
		}
		for _, step := range steps {
			if err := step(); err != nil {
				return err
			}
		}
		return tx.Delete(&gormDeviceRecord{}, "device_id = ?", deviceID).Error
	})
}

func (s *GormStore) GetDeviceSessionByAccessToken(_ context.Context, accessToken string) (model.DeviceSession, bool, error) {
	return firstModel(s.db.Where("access_token = ?", strings.TrimSpace(accessToken)), func(row gormDeviceSessionRecord) model.DeviceSession {
		return row.model()
	})
}

func (s *GormStore) GetDeviceSessionByRefreshToken(_ context.Context, refreshToken string) (model.DeviceSession, bool, error) {
	refreshToken = strings.TrimSpace(refreshToken)
	digest := sha256.Sum256([]byte(refreshToken))
	refreshTokenHash := hex.EncodeToString(digest[:])
	return firstModel(s.db.Where("refresh_token = ? OR previous_refresh_token_hash = ?", refreshToken, refreshTokenHash), func(row gormDeviceSessionRecord) model.DeviceSession {
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
	return s.db.Transaction(func(tx *gorm.DB) error {
		lockedStore := &GormStore{db: tx}
		if err := lockedStore.AcquireAdvisoryLock(deviceSessionLockID(item.DeviceID)); err != nil {
			return err
		}
		if err := tx.Delete(
			&gormDeviceSessionRecord{},
			"device_id = ? AND session_id <> ?",
			strings.TrimSpace(item.DeviceID),
			strings.TrimSpace(item.SessionID),
		).Error; err != nil {
			return err
		}
		return upsertByColumns(tx, &row, []string{"session_id"}, []string{"device_id", "access_token", "refresh_token", "status", "session_mode", "previous_refresh_token_hash", "refresh_rotation_grace_expiry", "expires_at", "refresh_expiry", "created_at", "updated_at", "revoked_at"})
	})
}

func (s *GormStore) RotateDeviceSession(_ context.Context, currentRefreshToken string, next model.DeviceSession) (bool, error) {
	currentRefreshToken = strings.TrimSpace(currentRefreshToken)
	if currentRefreshToken == "" {
		return false, nil
	}
	row := deviceSessionRecordFromModel(next)
	rotated := false
	err := s.db.Transaction(func(tx *gorm.DB) error {
		lockedStore := &GormStore{db: tx}
		if err := lockedStore.AcquireAdvisoryLock(deviceSessionLockID(next.DeviceID)); err != nil {
			return err
		}
		result := tx.Delete(
			&gormDeviceSessionRecord{},
			"device_id = ? AND refresh_token = ? AND status = ? AND revoked_at = 0",
			strings.TrimSpace(next.DeviceID), currentRefreshToken, "active",
		)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		rotated = true
		return nil
	})
	return rotated, err
}

func (s *GormStore) DeleteDeviceSessionForRefreshReuse(_ context.Context, sessionID, previousRefreshTokenHash string, now int64) (bool, error) {
	result := s.db.Delete(
		&gormDeviceSessionRecord{},
		"session_id = ? AND previous_refresh_token_hash = ? AND refresh_rotation_grace_expiry < ?",
		strings.TrimSpace(sessionID), strings.TrimSpace(previousRefreshTokenHash), now,
	)
	return result.RowsAffected > 0, result.Error
}

func deviceSessionLockID(deviceID string) int64 {
	hash := fnv.New64a()
	_, _ = hash.Write([]byte("device-session:" + strings.TrimSpace(deviceID)))
	return int64(hash.Sum64())
}

func (s *GormStore) DeleteDeviceSessionByAccessToken(_ context.Context, accessToken string) error {
	return s.db.Delete(&gormDeviceSessionRecord{}, "access_token = ?", strings.TrimSpace(accessToken)).Error
}
