package repository

import (
	"context"
)

func (s *GormStore) Migrate() error {
	return s.migrate()
}

func (s *GormStore) OperatorCount(_ context.Context) (int64, error) {
	var count int64
	if err := s.db.Model(&gormOperatorRecord{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func (s *GormStore) SaveCounter(name string, value int64) error {
	return s.db.Save(&gormCounter{Name: name, Value: value}).Error
}
