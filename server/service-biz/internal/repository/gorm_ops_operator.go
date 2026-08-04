package repository

import (
	"context"
	"strings"

	"github.com/slan/service-biz/internal/model"
)

func (s *GormStore) ListOperators(_ context.Context) ([]model.Operator, error) {
	return listModels(s.db.Order("operator_id asc"), func(row gormOperatorRecord) model.Operator {
		return row.model()
	})
}

func (s *GormStore) GetOperator(_ context.Context, operatorID string) (model.Operator, bool, error) {
	return firstModel(s.db.Where("operator_id = ?", operatorID), func(row gormOperatorRecord) model.Operator {
		return row.model()
	})
}

func (s *GormStore) GetOperatorByEmail(_ context.Context, email string) (model.Operator, bool, error) {
	return firstModel(s.db.Where("email = ?", normalizeEmail(email)), func(row gormOperatorRecord) model.Operator {
		return row.model()
	})
}

func (s *GormStore) SaveOperator(_ context.Context, operator model.Operator) error {
	row := operatorRecordFromModel(operator)
	return upsertByColumns(s.db, &row, []string{"operator_id"}, []string{"email", "name", "password_hash", "role", "status", "created_at", "updated_at"})
}

func (s *GormStore) GetOperatorSessionByAccessToken(_ context.Context, accessToken string) (model.OperatorSession, bool, error) {
	return firstModel(s.db.Where("access_token = ?", strings.TrimSpace(accessToken)), func(row gormOperatorSessionRecord) model.OperatorSession {
		return row.model()
	})
}

func (s *GormStore) SaveOperatorSession(_ context.Context, session model.OperatorSession) error {
	row := operatorSessionRecordFromModel(session)
	return upsertByColumns(s.db, &row, []string{"session_id"}, []string{"operator_id", "access_token", "expires_at", "created_at"})
}

func (s *GormStore) DeleteOperatorSessionsByOperatorID(_ context.Context, operatorID string) error {
	return s.db.Where("operator_id = ?", strings.TrimSpace(operatorID)).Delete(&gormOperatorSessionRecord{}).Error
}

func (s *GormStore) DeleteOperatorSessionByAccessToken(_ context.Context, accessToken string) error {
	return s.db.Where("access_token = ?", strings.TrimSpace(accessToken)).Delete(&gormOperatorSessionRecord{}).Error
}

func (s *GormStore) DeleteExpiredOperatorSessions(_ context.Context, now int64) error {
	return s.db.Where("expires_at <= ?", now).Delete(&gormOperatorSessionRecord{}).Error
}
