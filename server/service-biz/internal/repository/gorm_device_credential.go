package repository

import (
	"context"

	"github.com/slan/service-biz/internal/model"
	"gorm.io/gorm"
)

func (s *GormStore) ListDeviceCredentials(_ context.Context, deviceID string) ([]model.DeviceCredential, error) {
	db := s.db.Order("created_at desc")
	if deviceID != "" {
		db = db.Where("device_id = ?", deviceID)
	}
	return listModels(db, func(row gormDeviceCredentialRecord) model.DeviceCredential { return row.model() })
}

func (s *GormStore) GetDeviceCredential(_ context.Context, credentialID string) (model.DeviceCredential, bool, error) {
	return firstModel(s.db.Where("credential_id = ?", credentialID), func(row gormDeviceCredentialRecord) model.DeviceCredential { return row.model() })
}

func (s *GormStore) GetDeviceCredentialByKeyID(_ context.Context, keyID string) (model.DeviceCredential, bool, error) {
	return firstModel(s.db.Where("key_id = ?", keyID), func(row gormDeviceCredentialRecord) model.DeviceCredential { return row.model() })
}

func (s *GormStore) BindDeviceCredential(_ context.Context, credentialID, deviceID string, createdAfter, now int64) (bool, error) {
	result := s.db.Model(&gormDeviceCredentialRecord{}).
		Where("credential_id = ? AND device_id = ? AND status = ? AND created_at > ?", credentialID, "", model.DeviceCredentialStatusActive, createdAfter).
		Updates(map[string]any{"device_id": deviceID, "updated_at": now})
	return result.RowsAffected == 1, result.Error
}

func (s *GormStore) MarkDeviceCredentialUsed(_ context.Context, credentialID, deviceID string, now int64, remoteIP string) (bool, error) {
	result := s.db.Model(&gormDeviceCredentialRecord{}).
		Where("credential_id = ? AND device_id = ? AND status = ?", credentialID, deviceID, model.DeviceCredentialStatusActive).
		Updates(map[string]any{"last_used_at": now, "last_used_ip": remoteIP, "updated_at": now})
	return result.RowsAffected == 1, result.Error
}

func (s *GormStore) RevokeDeviceCredential(_ context.Context, credentialID string, now int64) (bool, error) {
	result := s.db.Model(&gormDeviceCredentialRecord{}).
		Where("credential_id = ?", credentialID).
		Updates(map[string]any{"status": model.DeviceCredentialStatusRevoked, "revoked_at": now, "updated_at": now})
	return result.RowsAffected == 1, result.Error
}

func (s *GormStore) MarkDeviceCredentialsDisablePending(_ context.Context, deviceID string, now int64) (int64, error) {
	result := s.db.Model(&gormDeviceCredentialRecord{}).
		Where("device_id = ? AND status = ?", deviceID, model.DeviceCredentialStatusActive).
		Updates(map[string]any{"disable_notified_at": now, "disable_notify_count": 0, "offline_ack_at": 0, "updated_at": now})
	return result.RowsAffected, result.Error
}

func (s *GormStore) AckDeviceOffline(_ context.Context, credentialID string, now int64) (bool, error) {
	result := s.db.Model(&gormDeviceCredentialRecord{}).
		Where("credential_id = ? AND offline_ack_at = 0", credentialID).
		Updates(map[string]any{"offline_ack_at": now, "updated_at": now})
	return result.RowsAffected == 1, result.Error
}

func (s *GormStore) RecordDisableNotify(_ context.Context, credentialID string, now int64) (bool, error) {
	result := s.db.Model(&gormDeviceCredentialRecord{}).
		Where("credential_id = ?", credentialID).
		Updates(map[string]any{"disable_notified_at": now, "disable_notify_count": gorm.Expr("disable_notify_count + 1"), "updated_at": now})
	return result.RowsAffected == 1, result.Error
}

func (s *GormStore) ListPendingOfflineAckCredentials(_ context.Context) ([]model.DeviceCredential, error) {
	return listModels(s.db.Where("device_id <> ? AND status = ? AND offline_ack_at = 0 AND disable_notified_at > 0", "", model.DeviceCredentialStatusActive), func(row gormDeviceCredentialRecord) model.DeviceCredential { return row.model() })
}

func (s *GormStore) DeleteInvalidDeviceCredentialsBefore(_ context.Context, cutoff int64) (int64, error) {
	var deleted int64
	err := s.db.Transaction(func(tx *gorm.DB) error {
		staleCredentials := tx.Model(&gormDeviceCredentialRecord{}).
			Select("credential_id").
			Where("status <> ? AND updated_at <= ?", model.DeviceCredentialStatusActive, cutoff)
		if err := tx.Where("credential_id IN (?)", staleCredentials).Delete(&gormDeviceSessionRecord{}).Error; err != nil {
			return err
		}
		result := tx.Where("status <> ? AND updated_at <= ?", model.DeviceCredentialStatusActive, cutoff).
			Delete(&gormDeviceCredentialRecord{})
		deleted = result.RowsAffected
		return result.Error
	})
	return deleted, err
}

func (s *GormStore) DeleteExpiredUnboundDeviceCredentialsBefore(_ context.Context, cutoff int64) (int64, error) {
	result := s.db.
		Where("status = ? AND device_id = ? AND created_at <= ?", model.DeviceCredentialStatusActive, "", cutoff).
		Delete(&gormDeviceCredentialRecord{})
	return result.RowsAffected, result.Error
}

func (s *GormStore) SaveDeviceCredential(_ context.Context, item model.DeviceCredential) error {
	row := deviceCredentialRecordFromModel(item)
	return upsertByColumns(s.db, &row, []string{"credential_id"}, []string{
		"key_id", "device_id", "name", "secret_hash", "status", "scopes",
		"last_used_at", "last_used_ip", "created_at", "updated_at", "revoked_at",
	})
}

func deviceCredentialRecordFromModel(item model.DeviceCredential) gormDeviceCredentialRecord {
	return gormDeviceCredentialRecord{
		CredentialID: item.CredentialID, KeyID: item.KeyID, DeviceID: item.DeviceID,
		Name: item.Name, SecretHash: item.SecretHash, Status: item.Status, Scopes: item.Scopes,
		LastUsedAt: item.LastUsedAt, LastUsedIP: item.LastUsedIP,
		OfflineAckAt: item.OfflineAckAt, DisableNotifiedAt: item.DisableNotifiedAt, DisableNotifyCount: item.DisableNotifyCount,
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt, RevokedAt: item.RevokedAt,
	}
}

func (r gormDeviceCredentialRecord) model() model.DeviceCredential {
	return model.DeviceCredential{
		CredentialID: r.CredentialID, KeyID: r.KeyID, DeviceID: r.DeviceID,
		Name: r.Name, SecretHash: r.SecretHash, Status: r.Status, Scopes: r.Scopes,
		LastUsedAt: r.LastUsedAt, LastUsedIP: r.LastUsedIP,
		OfflineAckAt: r.OfflineAckAt, DisableNotifiedAt: r.DisableNotifiedAt, DisableNotifyCount: r.DisableNotifyCount,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, RevokedAt: r.RevokedAt,
	}
}
