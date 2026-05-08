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

// InitPostgresRuntime initializes only PostgreSQL-backed state. It is used by
// standalone data-plane-adjacent processes such as the UDP ICE probe server,
// which need credential lookups but must not start Redis-backed control loops.
func InitPostgresRuntime(ctx context.Context, cfg Config) (*Runtime, error) {
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
	return &Runtime{Postgres: pgPool}, nil
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
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.Postgres.ConnMaxLifetimeSeconds) * time.Second)
	sqlDB.SetConnMaxIdleTime(time.Duration(cfg.Postgres.ConnMaxIdleTimeSeconds) * time.Second)
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
		DialTimeout:  time.Duration(cfg.Redis.DialTimeoutSeconds) * time.Second,
		ReadTimeout:  time.Duration(cfg.Redis.ReadTimeoutSeconds) * time.Second,
		WriteTimeout: time.Duration(cfg.Redis.WriteTimeoutSeconds) * time.Second,
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
		&repo.PlanConfig{},
		&repo.UserPlanOverride{},
		&repo.Menu{},
		&repo.RoleMenu{},
		&repo.Device{},
		&repo.DeviceUser{},
		&repo.DeviceInstallRegistration{},
		&repo.DeviceNetworkState{},
		&repo.Node{},
		&repo.NodeEndpoint{},
		&repo.NodeConnectionState{},
		&repo.NodePathHealth{},
		&repo.NodePathHealthSample{},
		&repo.RelayPolicyExecution{},
		&repo.RelayPolicyTemplate{},
		&repo.RelayNodeHeartbeat{},
		&repo.IceServer{},
		&repo.PeerCandidate{},
		&repo.PunchSession{},
		&repo.PunchResult{},
		&repo.IceServerStat{},
		&repo.WireDerpNode{},
		&repo.WireRelayNode{},
		&repo.WireNodeEvent{},
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
	if err := migrateNetworkMemberIndexes(ctx, db); err != nil {
		return err
	}
	if err := migrateDeviceIDOnly(ctx, db); err != nil {
		return err
	}
	return migrateDeviceUserBinding(ctx, db)
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

func migrateNetworkMemberIndexes(ctx context.Context, db *gorm.DB) error {
	tx := db.WithContext(ctx)
	if err := tx.Exec(`
		DELETE FROM network_members a
		USING network_members b
		WHERE a.network_id = b.network_id
		  AND a.device_id = b.device_id
		  AND (
		    a.created_at > b.created_at
		    OR (a.created_at = b.created_at AND a.member_id > b.member_id)
		  )
	`).Error; err != nil {
		return fmt.Errorf("dedupe network members: %w", err)
	}
	if err := tx.Exec(`DROP INDEX IF EXISTS idx_network_device`).Error; err != nil {
		return fmt.Errorf("drop idx_network_device: %w", err)
	}
	if err := tx.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_network_device ON network_members (network_id, device_id)`).Error; err != nil {
		return fmt.Errorf("create idx_network_device: %w", err)
	}
	return nil
}

func migrateDeviceIDOnly(ctx context.Context, db *gorm.DB) error {
	tx := db.WithContext(ctx)
	if err := tx.Exec(`ALTER TABLE devices DROP COLUMN IF EXISTS machine_id`).Error; err != nil {
		return fmt.Errorf("drop devices.machine_id: %w", err)
	}
	return nil
}

func migrateDeviceUserBinding(ctx context.Context, db *gorm.DB) error {
	tx := db.WithContext(ctx)
	if tx.Migrator().HasColumn(&repo.Device{}, "user_id") {
		if err := tx.Exec(`
			INSERT INTO device_users (device_id, user_id, bound_at)
			SELECT device_id, user_id, COALESCE(NULLIF(created_at, 0), EXTRACT(EPOCH FROM NOW())::bigint)
			FROM devices
			WHERE COALESCE(user_id, '') <> ''
			ON CONFLICT (device_id, user_id) DO NOTHING
		`).Error; err != nil {
			return fmt.Errorf("backfill device_users: %w", err)
		}
	}
	if err := tx.Exec(`ALTER TABLE devices DROP COLUMN IF EXISTS user_id`).Error; err != nil {
		return fmt.Errorf("drop devices.user_id: %w", err)
	}
	return nil
}
