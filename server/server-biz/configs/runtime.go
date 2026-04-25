package configs

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/slan/server/server-biz/internal/repo"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Runtime 持有 server-biz 启动后依赖的基础设施连接。
type Runtime struct {
	// Postgres 是业务主数据存储连接。
	Postgres *gorm.DB
	// Redis 是令牌、修订号和跨实例同步状态存储连接。
	Redis *redis.Client
}

// Close 关闭基础设施连接。
func (r *Runtime) Close() {
	if r == nil {
		return
	}
	if r.Redis != nil {
		_ = r.Redis.Close()
	}
	if r.Postgres != nil {
		if sqlDB, err := r.Postgres.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}
}

// InitRuntime 根据配置初始化 PostgreSQL 和 Redis 连接。
func InitRuntime(ctx context.Context, cfg Config) (*Runtime, error) {
	pgPool, err := connectPostgres(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := migratePostgres(ctx, pgPool); err != nil {
		if sqlDB, dbErr := pgPool.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
		return nil, err
	}

	redisClient, err := connectRedis(ctx, cfg)
	if err != nil {
		if sqlDB, dbErr := pgPool.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
		return nil, err
	}

	return &Runtime{
		Postgres: pgPool,
		Redis:    redisClient,
	}, nil
}

func connectPostgres(ctx context.Context, cfg Config) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.Postgres.DSN()), &gorm.Config{
		Logger: logger.New(
			log.New(os.Stdout, "\r\n", log.LstdFlags),
			logger.Config{
				LogLevel:                  logger.Warn,
				IgnoreRecordNotFoundError: true,
				Colorful:                  false,
			},
		),
	})
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("postgres db handle: %w", err)
	}
	sqlDB.SetMaxOpenConns(cfg.Postgres.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.Postgres.MinIdleConns)
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(pingCtx); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return db.WithContext(ctx), nil
}

func connectRedis(ctx context.Context, cfg Config) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Redis.Address(),
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.Database,
		PoolSize:     cfg.Redis.PoolSize,
		MinIdleConns: cfg.Redis.MinIdleConns,
	})

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	return client, nil
}

func migratePostgres(ctx context.Context, db *gorm.DB) error {
	if err := db.WithContext(ctx).AutoMigrate(
		&repo.User{},
		&repo.AdminInfo{},
		&repo.Role{},
		&repo.UserRole{},
		&repo.Menu{},
		&repo.RoleMenu{},
		&repo.Device{},
		&repo.Node{},
		&repo.NodeEndpoint{},
		&repo.NodeConnectionState{},
		&repo.NodePathHealth{},
		&repo.Network{},
		&repo.Subnet{},
		&repo.NetworkMember{},
		&repo.SubnetAttachment{},
		&repo.ControlSession{},
		&repo.ControlOutboundMessage{},
		&repo.ControlMessageHistory{},
	); err != nil {
		return err
	}
	if err := migrateSubnetAttachmentIndexes(ctx, db); err != nil {
		return err
	}
	return migrateDeviceIndexes(ctx, db)
}

func migrateSubnetAttachmentIndexes(ctx context.Context, db *gorm.DB) error {
	tx := db.WithContext(ctx)
	if err := tx.Exec(`DROP INDEX IF EXISTS idx_subnet_ip`).Error; err != nil {
		return fmt.Errorf("drop idx_subnet_ip: %w", err)
	}
	if err := tx.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_subnet_ip ON subnet_attachments (subnet_id, virtual_ip) WHERE virtual_ip <> ''`).Error; err != nil {
		return fmt.Errorf("create idx_subnet_ip: %w", err)
	}
	return nil
}

func migrateDeviceIndexes(ctx context.Context, db *gorm.DB) error {
	tx := db.WithContext(ctx)
	if err := tx.Exec(`DROP INDEX IF EXISTS idx_user_machine`).Error; err != nil {
		return fmt.Errorf("drop idx_user_machine: %w", err)
	}
	if err := tx.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_user_machine ON devices (user_id, machine_id)`).Error; err != nil {
		return fmt.Errorf("create idx_user_machine: %w", err)
	}
	return nil
}
