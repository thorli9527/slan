package repository

import (
	"fmt"
	"os"
	"strings"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func OpenGormStore(cfg GormConfig) (*GormStore, error) {
	dsn := strings.TrimSpace(cfg.DSN)
	if dsn == "" {
		dsn = defaultGormDSN()
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	return &GormStore{db: db}, nil
}

func defaultGormDSN() string {
	if dsn := strings.TrimSpace(os.Getenv("SLAN_SERVICE_BIZ_DSN")); dsn != "" {
		return dsn
	}
	host := envOrDefault("SLAN_SERVICE_BIZ_DB_HOST", "127.0.0.1")
	port := envOrDefault("SLAN_SERVICE_BIZ_DB_PORT", "5432")
	user := envOrDefault("SLAN_SERVICE_BIZ_DB_USER", "slan")
	password := envOrDefault("SLAN_SERVICE_BIZ_DB_PASSWORD", "slan")
	name := envOrDefault("SLAN_SERVICE_BIZ_DB_NAME", "slan")
	sslMode := envOrDefault("SLAN_SERVICE_BIZ_DB_SSLMODE", "disable")
	timeZone := envOrDefault("SLAN_SERVICE_BIZ_DB_TIMEZONE", "UTC")
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=%s",
		host,
		port,
		user,
		password,
		name,
		sslMode,
		timeZone,
	)
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
