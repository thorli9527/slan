package service

import (
	"context"

	"github.com/slan/service-biz/internal/pkg/wirekit"
)

type WireNodeView = wirekit.NodeView
type WireDerpMapView = wirekit.DerpMap

type WireNodeUseCase interface {
	Authorize(ctx context.Context, token string) error
	ListRelayNodes(ctx context.Context) ([]WireNodeView, error)
	ListDerpNodes(ctx context.Context) ([]WireNodeView, error)
	ListPunchNodes(ctx context.Context) ([]RuntimePunchNodeView, error)
	UpsertRelayNode(ctx context.Context, input WireUpsertNodeInput) (WireNodeView, error)
	UpsertDerpNode(ctx context.Context, input WireUpsertNodeInput) (WireNodeView, error)
	UpsertPunchNode(ctx context.Context, input WireUpsertNodeInput) (RuntimePunchNodeView, error)
	HeartbeatRelayNode(ctx context.Context, input WireNodeStatusInput) (WireNodeView, error)
	HeartbeatDerpNode(ctx context.Context, input WireNodeStatusInput) (WireNodeView, error)
	HeartbeatPunchNode(ctx context.Context, input WireNodeStatusInput) (RuntimePunchNodeView, error)
	UpdateRelayNodeStatus(ctx context.Context, input WireNodeStatusInput) (WireNodeView, error)
	UpdateDerpNodeStatus(ctx context.Context, input WireNodeStatusInput) (WireNodeView, error)
	DeleteRelayNode(ctx context.Context, input WireNodeDeleteInput) error
	DeleteDerpNode(ctx context.Context, input WireNodeDeleteInput) error
	DeletePunchNode(ctx context.Context, input WireNodeDeleteInput) error
	DerpMap(ctx context.Context) (WireDerpMapView, error)
}
