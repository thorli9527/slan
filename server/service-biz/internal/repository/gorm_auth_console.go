package repository

import (
	"context"
	"strings"

	"github.com/slan/service-biz/internal/model"
)

func (s *GormStore) GetConsoleLoginKeyByKey(_ context.Context, key string) (model.ConsoleLoginKey, bool, error) {
	return firstModel(s.db.Where("key = ?", strings.TrimSpace(key)), func(row gormConsoleLoginKeyRecord) model.ConsoleLoginKey {
		return row.model()
	})
}

func (s *GormStore) SaveConsoleLoginKey(_ context.Context, item model.ConsoleLoginKey) error {
	row := consoleLoginKeyRecordFromModel(item)
	return upsertByColumns(s.db, &row, []string{"key_id"}, []string{"user_id", "key", "status", "expires_at", "created_at", "updated_at"})
}
