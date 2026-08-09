package service

import "context"

type RuntimeNodeRegistryUseCase interface {
	ListRelayNodes(ctx context.Context) ([]RuntimeRelayNodeView, error)
	UpsertRelayNode(ctx context.Context, input UpsertRuntimeNodeInput) (RuntimeRelayNodeView, error)
	UpdateRelayNodeStatus(ctx context.Context, nodeID, status string) (RuntimeRelayNodeView, error)
	DeleteRelayNode(ctx context.Context, nodeID string) error
	ListPunchNodes(ctx context.Context) ([]RuntimePunchNodeView, error)
	UpsertPunchNode(ctx context.Context, input UpsertRuntimeNodeInput) (RuntimePunchNodeView, error)
	UpdatePunchNodeStatus(ctx context.Context, nodeID, status string) (RuntimePunchNodeView, error)
	DeletePunchNode(ctx context.Context, nodeID string) error
}
