package repo

import (
	"errors"

	"gorm.io/gorm"
)

// PostgresRepository 是 server-biz 对 PostgreSQL 的统一访问入口。
type PostgresRepository struct {
	// db 是底层 gorm 连接。
	db *gorm.DB
}

func NewPostgresRepository(db *gorm.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func IsNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
