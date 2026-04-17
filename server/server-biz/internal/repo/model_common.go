package repo

import (
	"errors"
	"strings"

	"gorm.io/gorm"
)

type PostgresRepository struct {
	db *gorm.DB
}

func NewPostgresRepository(db *gorm.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func IsNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

func normalizeEmail(email string) string {
	return strings.TrimSpace(strings.ToLower(email))
}
