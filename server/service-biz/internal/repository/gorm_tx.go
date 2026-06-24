package repository

import "gorm.io/gorm"

func (s *GormStore) Transaction(fn func(store *GormStore) error) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		return fn(&GormStore{db: tx})
	})
}

func (s *GormStore) AcquireAdvisoryLock(lockID int64) error {
	if s.db.Dialector.Name() != "postgres" {
		return nil
	}
	return s.db.Exec("SELECT pg_advisory_xact_lock(?)", lockID).Error
}

func (s *GormStore) DialectName() string {
	return s.db.Dialector.Name()
}

