package service

import (
	"context"

	"github.com/slan/server/server-biz/internal/infra"
	"github.com/slan/server/server-biz/internal/repo"
)

type dbState struct {
	cfg    infra.Config
	pg     *repo.PostgresRepository
	tokens *repo.RedisTokenStore
}

type dbAuthService struct{ state *dbState }
type dbDeviceService struct{ state *dbState }
type dbNetworkService struct{ state *dbState }
type dbNodeService struct{ state *dbState }
type dbBootstrapService struct{ state *dbState }
type dbTokenVerifier struct{ state *dbState }
type dbControlChannelService struct{ state *dbState }
type dbControlSyncService struct{ state *dbState }

func NewDBServices(cfg infra.Config, runtime *infra.Runtime) *Services {
	state := &dbState{
		cfg:    cfg,
		pg:     repo.NewPostgresRepository(runtime.Postgres),
		tokens: repo.NewRedisTokenStore(runtime.Redis),
	}
	return &Services{
		Auth:           dbAuthService{state: state},
		Device:         dbDeviceService{state: state},
		Network:        dbNetworkService{state: state},
		Node:           dbNodeService{state: state},
		Bootstrap:      dbBootstrapService{state: state},
		Tokens:         dbTokenVerifier{state: state},
		ControlChannel: dbControlChannelService{state: state},
		ControlSync:    dbControlSyncService{state: state},
	}
}

func (v dbTokenVerifier) Authenticate(accessToken string) (string, error) {
	userID, err := v.state.tokens.Authenticate(context.Background(), accessToken)
	if err != nil {
		return "", ErrUnauthorized
	}
	return userID, nil
}
