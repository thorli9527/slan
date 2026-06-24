package service

import "context"

type OpsNodeUseCase interface {
	ListRelayNodes(ctx context.Context) ([]OpsRelayNodeView, error)
	UpsertRelayNode(ctx context.Context, input UpsertNodeInput) (OpsRelayNodeView, error)
	UpdateRelayNodeStatus(ctx context.Context, nodeID, status string) (OpsRelayNodeView, error)
	DeleteRelayNode(ctx context.Context, nodeID string) error
	ListPunchNodes(ctx context.Context) ([]OpsPunchNodeView, error)
	UpsertPunchNode(ctx context.Context, input UpsertNodeInput) (OpsPunchNodeView, error)
	UpdatePunchNodeStatus(ctx context.Context, nodeID, status string) (OpsPunchNodeView, error)
	DeletePunchNode(ctx context.Context, nodeID string) error
}
