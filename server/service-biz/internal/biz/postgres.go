package biz

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const bizSchemaMigration = "001_initial_client_web"

func InitPostgresFromEnv(ctx context.Context) (*sql.DB, error) {
	dsn := strings.TrimSpace(os.Getenv("SLAN_BIZ_POSTGRES_DSN"))
	if dsn == "" {
		return nil, nil
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := migratePostgres(ctx, db, migrationsDirFromEnv()); err != nil {
		_ = db.Close()
		return nil, err
	}
	log.Printf("service-biz using postgres schema dsn=%s", redactPostgresDSN(dsn))
	return db, nil
}

func migratePostgres(ctx context.Context, db *sql.DB, dir string) error {
	if strings.TrimSpace(dir) == "" {
		return errors.New("migration directory is empty")
	}
	if _, err := db.ExecContext(ctx, `create table if not exists schema_migrations (
		version varchar(128) primary key,
		applied_at timestamptz not null default now()
	)`); err != nil {
		return err
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	if len(files) == 0 {
		return fmt.Errorf("no migration files found in %s", dir)
	}
	for _, path := range files {
		version := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		var exists bool
		if err := db.QueryRowContext(ctx, `select exists(select 1 from schema_migrations where version = $1)`, version).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}
		payload, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, string(payload)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %s: %w", version, err)
		}
		if _, err := tx.ExecContext(ctx, `insert into schema_migrations(version) values($1)`, version); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		log.Printf("service-biz applied postgres migration %s", version)
	}
	return nil
}

func migrationsDirFromEnv() string {
	if value := strings.TrimSpace(os.Getenv("SLAN_BIZ_MIGRATIONS_DIR")); value != "" {
		return value
	}
	if _, err := os.Stat("migrations"); err == nil {
		return "migrations"
	}
	return filepath.Join("server", "service-biz", "migrations")
}

func redactPostgresDSN(dsn string) string {
	if at := strings.LastIndex(dsn, "@"); at >= 0 {
		prefix := dsn[:at]
		if scheme := strings.Index(prefix, "://"); scheme >= 0 {
			return prefix[:scheme+3] + "redacted@" + dsn[at+1:]
		}
	}
	return "redacted"
}

var _ = bizSchemaMigration
