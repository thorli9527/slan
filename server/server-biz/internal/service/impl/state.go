package impl

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/slan/server/server-biz/configs"
	"github.com/slan/server/server-biz/internal/repo"
	"github.com/slan/server/server-biz/internal/service"
	controlws "github.com/slan/server/server-biz/internal/ws"
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
	tokens tokenStore

	controlMu         sync.Mutex
	cachedRelayTicket map[string]cachedRelayTicket
}

type tokenStore interface {
	StoreAccessToken(ctx context.Context, token, userID string, ttl time.Duration) error
	StoreRefreshToken(ctx context.Context, token, userID string, ttl time.Duration) error
	StoreOpsAccessToken(ctx context.Context, token, adminID string, ttl time.Duration) error
	StoreControlSessionToken(ctx context.Context, token, userID string, ttl time.Duration) error
	MarkAuthCallbackReceived(ctx context.Context, callbackID string, receivedAt int64, ttl time.Duration) error
	AuthCallbackReceivedAt(ctx context.Context, callbackID string) (int64, error)
	StoreAuthCallbackPayload(ctx context.Context, callbackID string, payload any, ttl time.Duration) error
	LoadAuthCallbackPayload(ctx context.Context, callbackID string, target any) (bool, error)
	Authenticate(ctx context.Context, accessToken string) (string, error)
	AuthenticateControlSessionToken(ctx context.Context, token string) (string, error)
	AuthenticateOpsAccessToken(ctx context.Context, token string) (string, error)
	PublishControlSyncEvent(ctx context.Context, event controlws.ControlSyncEvent) error
	SubscribeControlSyncEvents(ctx context.Context, handler func(controlws.ControlSyncEvent)) error
	NextNetworkRevision(ctx context.Context, networkID string) (uint64, error)
	CurrentNetworkRevision(ctx context.Context, networkID string) (uint64, error)
	AcquireConnectPlanRetry(ctx context.Context, networkID, nodeID, peerNodeID string) (bool, error)
	ResetConnectPlanRetry(ctx context.Context, networkID, nodeID, peerNodeID string) error
	AcquirePeerCandidateDelivery(ctx context.Context, networkID, sourceNodeID, targetNodeID string, candidate controlws.PeerCandidate, ttl time.Duration) (bool, error)
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
