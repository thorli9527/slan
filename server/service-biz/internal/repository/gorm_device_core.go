package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"hash/fnv"
	"strings"

	"github.com/slan/service-biz/internal/model"
	"gorm.io/gorm"
)

func (s *GormStore) ListDevicesByOwner(_ context.Context, ownerID string) ([]model.Device, error) {
	var rows []gormDeviceRecord
	err := s.db.Table("gorm_device_records AS device").
		Joins("JOIN gorm_device_user_relation_records AS relation ON relation.device_id = device.device_id").
		Where("relation.user_id = ? AND relation.role = ? AND relation.status = ?", strings.TrimSpace(ownerID), model.DeviceRelationRoleOwner, model.DeviceRelationStatusActive).
		Order("device.device_id asc").
		Select("device.*").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	items := make([]model.Device, 0, len(rows))
	for _, row := range rows {
		item := row.model()
		item.OwnerID = strings.TrimSpace(ownerID)
		items = append(items, item)
	}
	return items, nil
}

func (s *GormStore) GetDevice(_ context.Context, deviceID string) (model.Device, bool, error) {
	item, ok, err := firstModel(s.db.Where("device_id = ?", deviceID), func(row gormDeviceRecord) model.Device { return row.model() })
	if err != nil || !ok {
		return item, ok, err
	}
	owner, ownerOK, err := s.getDeviceOwnerRelation(deviceID)
	if err != nil {
		return model.Device{}, false, err
	}
	if ownerOK {
		item.OwnerID = owner.UserID
	}
	return item, true, nil
}

func (s *GormStore) SaveDevice(_ context.Context, device model.Device) error {
	row := deviceRecordFromModel(device)
	return s.db.Transaction(func(tx *gorm.DB) error {
		ownerID := strings.TrimSpace(device.OwnerID)
		previousOwner, hasPreviousOwner, err := getDeviceOwnerRelation(tx, device.DeviceID)
		if err != nil {
			return err
		}
		ownerChanged := ownerID != "" && hasPreviousOwner && previousOwner.UserID != ownerID
		if ownerChanged {
			if err := clearDeviceOwnershipScope(tx, device.DeviceID); err != nil {
				return err
			}
		}
		if err := upsertByColumns(tx, &row, []string{"device_id"}, []string{"virtual_ip", "name", "platform", "alias", "os_name", "os_version", "public_key", "device_version", "country_code", "rx_bytes_total", "tx_bytes_total", "status", "created_at", "updated_at", "last_seen_at"}); err != nil {
			return err
		}
		if ownerID == "" {
			return nil
		}
		relation := model.DeviceUserRelation{
			RelationID: deviceUserRelationID(device.DeviceID, ownerID), DeviceID: device.DeviceID, UserID: ownerID,
			Role: model.DeviceRelationRoleOwner, SourceType: "registration", Status: model.DeviceRelationStatusActive,
			CreatedBy: ownerID, CreatedAt: device.CreatedAt, UpdatedAt: device.UpdatedAt,
		}
		return saveDeviceUserRelation(tx, relation)
	})
}

func clearDeviceOwnershipScope(tx *gorm.DB, deviceID string) error {
	if err := tx.Delete(&gormDeviceUserRelationRecord{}, "device_id = ?", deviceID).Error; err != nil {
		return err
	}
	if err := tx.Model(&gormDeviceInviteRecord{}).
		Where("device_id = ? AND status IN ?", deviceID, []string{"pending", "accepted"}).
		Updates(map[string]any{"status": "revoked"}).Error; err != nil {
		return err
	}
	if err := tx.Delete(&gormDeviceGroupAssignmentRecord{}, "device_id = ?", deviceID).Error; err != nil {
		return err
	}
	if err := tx.Delete(&gormNetworkDeviceRecord{}, "device_id = ?", deviceID).Error; err != nil {
		return err
	}
	return tx.Delete(&gormDeviceSessionRecord{}, "device_id = ?", deviceID).Error
}

func (s *GormStore) DeleteDevice(_ context.Context, deviceID string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&gormDeviceGroupAssignmentRecord{}, "device_id = ?", deviceID).Error; err != nil {
			return err
		}
		if err := tx.Delete(&gormDeviceLoginRecord{}, "device_id = ?", deviceID).Error; err != nil {
			return err
		}
		if err := tx.Delete(&gormDeviceUserRelationRecord{}, "device_id = ?", deviceID).Error; err != nil {
			return err
		}
		return tx.Delete(&gormDeviceRecord{}, "device_id = ?", deviceID).Error
	})
}

func deviceUserRelationID(deviceID, userID string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(deviceID) + "\x00" + strings.TrimSpace(userID)))
	return "dur" + hex.EncodeToString(sum[:16])
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

func deviceSessionLockID(deviceID string) int64 {
	hash := fnv.New64a()
	_, _ = hash.Write([]byte("device-session:" + strings.TrimSpace(deviceID)))
	return int64(hash.Sum64())
}

func (s *GormStore) DeleteDeviceSessionByAccessToken(_ context.Context, accessToken string) error {
	return s.db.Delete(&gormDeviceSessionRecord{}, "access_token = ?", strings.TrimSpace(accessToken)).Error
}
