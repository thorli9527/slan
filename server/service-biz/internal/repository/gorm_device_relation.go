package repository

import (
	"context"
	"strings"

	"github.com/slan/service-biz/internal/model"
	"gorm.io/gorm"
)

func (s *GormStore) ListDeviceRelationsByUser(_ context.Context, userID string) ([]model.DeviceUserRelation, error) {
	return listModels(
		s.db.Where("user_id = ? AND status = ?", strings.TrimSpace(userID), model.DeviceRelationStatusActive).Order("device_id asc"),
		func(row gormDeviceUserRelationRecord) model.DeviceUserRelation { return row.model() },
	)
}

func (s *GormStore) ListDeviceRelationsByDevice(_ context.Context, deviceID string) ([]model.DeviceUserRelation, error) {
	return listModels(
		s.db.Where("device_id = ?", strings.TrimSpace(deviceID)).Order("role asc, user_id asc"),
		func(row gormDeviceUserRelationRecord) model.DeviceUserRelation { return row.model() },
	)
}

func (s *GormStore) GetDeviceUserRelation(_ context.Context, deviceID, userID string) (model.DeviceUserRelation, bool, error) {
	return firstModel(
		s.db.Where("device_id = ? AND user_id = ?", strings.TrimSpace(deviceID), strings.TrimSpace(userID)),
		func(row gormDeviceUserRelationRecord) model.DeviceUserRelation { return row.model() },
	)
}

func (s *GormStore) SaveDeviceUserRelation(_ context.Context, relation model.DeviceUserRelation) error {
	return saveDeviceUserRelation(s.db, relation)
}

func saveDeviceUserRelation(db *gorm.DB, relation model.DeviceUserRelation) error {
	if relation.RelationID == "" {
		relation.RelationID = deviceUserRelationID(relation.DeviceID, relation.UserID)
	}
	row := deviceUserRelationRecordFromModel(relation)
	return upsertByColumns(db, &row, []string{"device_id", "user_id"}, []string{
		"relation_id", "role", "source_type", "source_id", "status", "created_by", "created_at", "updated_at", "revoked_by", "revoked_at",
	})
}

func (s *GormStore) SaveDeviceInviteWithRelation(_ context.Context, invite model.DeviceInvite, relation model.DeviceUserRelation) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := saveDeviceUserRelation(tx, relation); err != nil {
			return err
		}
		row := deviceInviteRecordFromModel(invite)
		return upsertByColumns(tx, &row, []string{"invite_id"}, []string{"invite_code", "inviter_user_id", "network_id", "device_id", "user_id", "status", "created_at", "expires_at", "accepted_at"})
	})
}

func (s *GormStore) RevokeDeviceInviteWithRelation(_ context.Context, invite model.DeviceInvite, sharedUserID, revokedBy string, revokedAt int64) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&gormDeviceUserRelationRecord{}).
			Where("device_id = ? AND user_id = ? AND role = ?", invite.DeviceID, strings.TrimSpace(sharedUserID), model.DeviceRelationRoleShared).
			Updates(map[string]any{
				"status": model.DeviceRelationStatusRevoked, "updated_at": revokedAt,
				"revoked_by": strings.TrimSpace(revokedBy), "revoked_at": revokedAt,
			}).Error; err != nil {
			return err
		}
		row := deviceInviteRecordFromModel(invite)
		return upsertByColumns(tx, &row, []string{"invite_id"}, []string{"invite_code", "inviter_user_id", "network_id", "device_id", "user_id", "status", "created_at", "expires_at", "accepted_at"})
	})
}

func (s *GormStore) RevokeDeviceUserRelation(_ context.Context, deviceID, userID, revokedBy string, revokedAt int64) error {
	return s.db.Model(&gormDeviceUserRelationRecord{}).
		Where("device_id = ? AND user_id = ? AND role <> ?", strings.TrimSpace(deviceID), strings.TrimSpace(userID), model.DeviceRelationRoleOwner).
		Updates(map[string]any{
			"status": model.DeviceRelationStatusRevoked, "updated_at": revokedAt,
			"revoked_by": strings.TrimSpace(revokedBy), "revoked_at": revokedAt,
		}).Error
}

func (s *GormStore) getDeviceOwnerRelation(deviceID string) (model.DeviceUserRelation, bool, error) {
	return getDeviceOwnerRelation(s.db, deviceID)
}

func getDeviceOwnerRelation(db *gorm.DB, deviceID string) (model.DeviceUserRelation, bool, error) {
	return firstModel(
		db.Where("device_id = ? AND role = ? AND status = ?", strings.TrimSpace(deviceID), model.DeviceRelationRoleOwner, model.DeviceRelationStatusActive),
		func(row gormDeviceUserRelationRecord) model.DeviceUserRelation { return row.model() },
	)
}
