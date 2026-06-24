package repository

import (
	"context"

	"github.com/slan/service-biz/internal/model"
)

type OpsNodeRepository interface {
	ListRelayNodes(ctx context.Context) ([]model.RelayNode, error)
	GetRelayNode(ctx context.Context, nodeID string) (model.RelayNode, bool, error)
	SaveRelayNode(ctx context.Context, item model.RelayNode) error
	DeleteRelayNode(ctx context.Context, nodeID string) error
	ListPunchNodes(ctx context.Context) ([]model.PunchNode, error)
	GetPunchNode(ctx context.Context, nodeID string) (model.PunchNode, bool, error)
	SavePunchNode(ctx context.Context, item model.PunchNode) error
	DeletePunchNode(ctx context.Context, nodeID string) error
}
