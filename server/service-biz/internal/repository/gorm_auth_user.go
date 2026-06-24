package repository

import (
	"context"
	"strings"

	"github.com/slan/service-biz/internal/model"
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

func (s *GormStore) SaveUserSession(_ context.Context, session model.UserSession) error {
	row := userSessionRecordFromModel(session)
	return upsertByColumns(s.db, &row, []string{"session_id"}, []string{"user_id", "access_token", "refresh_token", "expires_at", "refresh_expiry", "created_at"})
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
