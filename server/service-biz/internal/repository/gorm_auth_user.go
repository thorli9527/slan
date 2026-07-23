package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/slan/service-biz/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *GormStore) ListUsers(_ context.Context) ([]model.User, error) {
	return listModels(s.db.Order("user_id asc"), func(row gormUserRecord) model.User {
		return row.model()
	})
}

func (s *GormStore) GetUser(_ context.Context, userID string) (model.User, bool, error) {
	return firstModel(s.db.Where("user_id = ?", userID), func(row gormUserRecord) model.User {
		return row.model()
	})
}

func (s *GormStore) GetByEmail(_ context.Context, email string) (model.User, bool, error) {
	return firstModel(s.db.Where("email = ?", normalizeEmail(email)), func(row gormUserRecord) model.User {
		return row.model()
	})
}

func (s *GormStore) SaveUser(_ context.Context, user model.User) error {
	row := userRecordFromModel(user)
	return upsertByColumns(s.db, &row, []string{"user_id"}, []string{"email", "name", "country", "province", "city", "ip_region", "password_hash", "status", "created_at", "updated_at"})
}

func (s *GormStore) GetUserSessionByAccessToken(_ context.Context, accessToken string) (model.UserSession, bool, error) {
	return firstModel(s.db.Where("access_token = ?", strings.TrimSpace(accessToken)), func(row gormUserSessionRecord) model.UserSession {
		return row.model()
	})
}

func (s *GormStore) GetUserSessionByRefreshToken(_ context.Context, refreshToken string) (model.UserSession, bool, error) {
	refreshToken = strings.TrimSpace(refreshToken)
	digest := sha256.Sum256([]byte(refreshToken))
	refreshTokenHash := hex.EncodeToString(digest[:])
	return firstModel(s.db.Where("refresh_token = ? OR previous_refresh_token_hash = ?", refreshToken, refreshTokenHash), func(row gormUserSessionRecord) model.UserSession {
		return row.model()
	})
}

func (s *GormStore) ListUserSessionsByUserID(_ context.Context, userID string) ([]model.UserSession, error) {
	return listModels(s.db.Where("user_id = ?", strings.TrimSpace(userID)).Order("created_at desc"), func(row gormUserSessionRecord) model.UserSession {
		return row.model()
	})
}

func (s *GormStore) SaveUserSession(_ context.Context, session model.UserSession) error {
	return saveUserSession(s.db, session)
}

func (s *GormStore) ReplaceUserSessionForClient(_ context.Context, session model.UserSession) error {
	row := userSessionRecordFromModel(session)
	return s.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "client_type"}, {Name: "device_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"session_id", "access_token", "refresh_token", "status", "session_mode",
			"previous_refresh_token_hash", "refresh_rotation_grace_expiry",
			"expires_at", "refresh_expiry", "created_at", "updated_at", "revoked_at",
		}),
	}).Create(&row).Error
}

func saveUserSession(db *gorm.DB, session model.UserSession) error {
	row := userSessionRecordFromModel(session)
	return upsertByColumns(db, &row, []string{"session_id"}, []string{"user_id", "access_token", "refresh_token", "status", "session_mode", "client_type", "device_id", "previous_refresh_token_hash", "refresh_rotation_grace_expiry", "expires_at", "refresh_expiry", "created_at", "updated_at", "revoked_at"})
}

func (s *GormStore) ReplaceUserSession(_ context.Context, oldAccessToken string, session model.UserSession) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&gormUserSessionRecord{}, "access_token = ?", strings.TrimSpace(oldAccessToken)).Error; err != nil {
			return err
		}
		return saveUserSession(tx, session)
	})
}

func (s *GormStore) DeleteUserSessionByAccessToken(_ context.Context, accessToken string) error {
	return s.db.Delete(&gormUserSessionRecord{}, "access_token = ?", strings.TrimSpace(accessToken)).Error
}

func (s *GormStore) ListUserAliases(_ context.Context, userID string) ([]model.UserAlias, error) {
	return listModels(s.db.Where("user_id = ?", userID).Order("id asc"), func(row gormUserAliasRecord) model.UserAlias {
		return row.model()
	})
}

func (s *GormStore) SaveUserAlias(_ context.Context, alias model.UserAlias) error {
	row := userAliasRecordFromModel(alias)
	return upsertByColumns(s.db, &row, []string{"user_id", "alias"}, []string{"created_at", "updated_at"})
}
