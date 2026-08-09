package service

import (
	"context"

	"github.com/slan/service-biz/internal/model"
)

type OpsServerNodeUseCase interface {
	ListServerNodes(ctx context.Context) ([]OpsServerNodeView, error)
	UpsertServerNode(ctx context.Context, input UpsertServerNodeInput) (OpsServerNodeView, error)
	InspectServerNodeHostKey(ctx context.Context, nodeID string) (ServerNodeHostKeyView, error)
	DeployServerNode(ctx context.Context, nodeID, confirmedHostKey string) (OpsServerNodeView, error)
	DeleteServerNode(ctx context.Context, nodeID string) error
}

type ServerNodeDeployer interface {
	ProbeHostKey(ctx context.Context, node model.ServerNode, password string) (string, error)
	Deploy(ctx context.Context, node model.ServerNode, password string) (ServerNodeDeployResult, error)
}

type SecretCipher interface {
	Encrypt(plaintext string) (string, error)
	Decrypt(ciphertext string) (string, error)
}
