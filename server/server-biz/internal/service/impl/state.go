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

type dbState struct {
	cfg    configs.Config
	pg     *repo.PostgresRepository
	tokens tokenStore

	controlMu         sync.Mutex
	cachedRelayTicket map[string]cachedRelayTicket
}

type tokenStore interface {
	StoreAccessToken(ctx context.Context, token, userID, deviceID string, ttl time.Duration) error
	DeleteAccessToken(ctx context.Context, accessToken string) error
	StoreRefreshToken(ctx context.Context, token, userID string, ttl time.Duration) error
	StoreOpsAccessToken(ctx context.Context, token, adminID string, ttl time.Duration) error
	StoreControlSessionToken(ctx context.Context, token, userID string, ttl time.Duration) error
	MarkAuthCallbackReceived(ctx context.Context, callbackID string, receivedAt int64, ttl time.Duration) error
	AuthCallbackReceivedAt(ctx context.Context, callbackID string) (int64, error)
	StoreAuthCallbackPayload(ctx context.Context, callbackID string, payload any, ttl time.Duration) error
	LoadAuthCallbackPayload(ctx context.Context, callbackID string, target any) (bool, error)
	Authenticate(ctx context.Context, accessToken string) (repo.AccessTokenSession, error)
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
