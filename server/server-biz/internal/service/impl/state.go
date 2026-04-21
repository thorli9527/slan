package impl

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/slan/server/server-biz/configs"
	"github.com/slan/server/server-biz/internal/repo"
	"github.com/slan/server/server-biz/internal/service"
)

// dbState 持有各 service 实现共享的基础依赖和进程内状态。
//
// 这里集中放：
// - 静态配置
// - PostgreSQL / Redis repository
// - relay ticket cache
// - control-plane 清理协程共享状态
type dbState struct {
	cfg    configs.Config
	pg     *repo.PostgresRepository
	tokens *repo.RedisTokenStore

	controlMu         sync.Mutex
	cachedRelayTicket map[string]cachedRelayTicket
}

// NewDBServices 基于同一份共享状态构造全部 service 实现。
//
// 这样各业务实现可以共享：
// - 同一个 repository 视图
// - 同一份控制面缓存
// - 同一套后台清理循环
func NewDBServices(cfg configs.Config, runtime *configs.Runtime) (
	service.Auth,
	service.Device,
	service.Network,
	service.Node,
	service.Bootstrap,
	service.TokenVerifier,
	service.ControlChannel,
	service.ControlSync,
	service.MessageDelivery,
	service.Ops,
) {
	state := &dbState{
		cfg:               cfg,
		pg:                repo.NewPostgresRepository(runtime.Postgres),
		tokens:            repo.NewRedisTokenStore(runtime.Redis),
		cachedRelayTicket: make(map[string]cachedRelayTicket),
	}
	if err := state.seedBuiltinOpsRBAC(context.Background()); err != nil {
		panic(fmt.Errorf("seed builtin ops rbac: %w", err))
	}
	if err := state.seedDefaultAdmin(context.Background()); err != nil {
		panic(fmt.Errorf("seed default admin: %w", err))
	}
	state.cleanupExpiredControlPlaneState(context.Background(), time.Now())
	state.startControlStateCleanupLoop()
	return dbAuthService{state: state},
		dbDeviceService{state: state},
		dbNetworkService{state: state},
		dbNodeService{state: state},
		dbBootstrapService{state: state},
		dbTokenVerifier{state: state},
		dbControlChannelService{state: state},
		dbControlSyncService{state: state},
		dbMessageDeliveryService{state: state},
		dbOpsService{state: state}
}
