package impl

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/slan/server/server-biz/configs"
	controlmsg "github.com/slan/server/server-biz/internal/controlmsg"
	"github.com/slan/server/server-biz/internal/repo"
	"github.com/slan/server/server-biz/internal/service"
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
	AuthenticateRefreshToken(ctx context.Context, token string) (string, error)
	DeleteRefreshToken(ctx context.Context, token string) error
	StoreOpsAccessToken(ctx context.Context, token, adminID string, ttl time.Duration) error
	StoreControlSessionToken(ctx context.Context, token, userID string, ttl time.Duration) error
	DeleteControlSessionToken(ctx context.Context, token string) error
	StoreAuthCallbackPayload(ctx context.Context, callbackID string, payload any, ttl time.Duration) error
	LoadAuthCallbackPayload(ctx context.Context, callbackID string, target any) (bool, error)
	Authenticate(ctx context.Context, accessToken string) (repo.AccessTokenSession, error)
	AuthenticateControlSessionToken(ctx context.Context, token string) (string, error)
	AuthenticateOpsAccessToken(ctx context.Context, token string) (string, error)
	DeleteOpsAccessToken(ctx context.Context, token string) error
	PublishControlSyncEvent(ctx context.Context, event controlmsg.ControlSyncEvent) error
	SubscribeControlSyncEvents(ctx context.Context, handler func(controlmsg.ControlSyncEvent)) error
	NextNetworkRevision(ctx context.Context, networkID string) (uint64, error)
	CurrentNetworkRevision(ctx context.Context, networkID string) (uint64, error)
	AcquireConnectPlanRetry(ctx context.Context, networkID, nodeID, peerNodeID string) (bool, error)
	ResetConnectPlanRetry(ctx context.Context, networkID, nodeID, peerNodeID string) error
	AcquirePeerCandidateDelivery(ctx context.Context, networkID, sourceNodeID, targetNodeID string, candidate controlmsg.PeerCandidate, ttl time.Duration) (bool, error)
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
	if err := state.seedDefaultProducts(context.Background()); err != nil {
		panic(fmt.Errorf("seed default products: %w", err))
	}
	state.cleanupExpiredControlPlaneState(context.Background(), time.Now())
	state.syncAllUserEntitlements(context.Background(), "startup entitlement sync")
	state.startControlStateCleanupLoop()
	state.startEntitlementSyncLoop()
	state.startMQTTNetworkStateSubscriber()
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
