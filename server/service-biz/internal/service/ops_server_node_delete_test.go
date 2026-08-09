package service

import (
	"context"
	"testing"

	"github.com/slan/service-biz/internal/model"
)

type deleteServerNodeRepo struct {
	item    model.ServerNode
	deleted string
}

func (r *deleteServerNodeRepo) ListServerNodes(context.Context) ([]model.ServerNode, error) {
	return []model.ServerNode{r.item}, nil
}

func (r *deleteServerNodeRepo) GetServerNode(_ context.Context, nodeID string) (model.ServerNode, bool, error) {
	return r.item, nodeID == r.item.NodeID, nil
}

func (r *deleteServerNodeRepo) SaveServerNode(_ context.Context, item model.ServerNode) error {
	r.item = item
	return nil
}

func (r *deleteServerNodeRepo) DeleteServerNode(_ context.Context, nodeID string) error {
	r.deleted = nodeID
	return nil
}

type deleteRuntimeNodeRepo struct {
	relayIDs []string
	punchIDs []string
}

func (*deleteRuntimeNodeRepo) ListRelayNodes(context.Context) ([]model.RelayNode, error) {
	return nil, nil
}

func (*deleteRuntimeNodeRepo) GetRelayNode(context.Context, string) (model.RelayNode, bool, error) {
	return model.RelayNode{}, false, nil
}

func (*deleteRuntimeNodeRepo) SaveRelayNode(context.Context, model.RelayNode) error { return nil }

func (r *deleteRuntimeNodeRepo) DeleteRelayNode(_ context.Context, nodeID string) error {
	r.relayIDs = append(r.relayIDs, nodeID)
	return nil
}

func (*deleteRuntimeNodeRepo) ListPunchNodes(context.Context) ([]model.PunchNode, error) {
	return nil, nil
}

func (*deleteRuntimeNodeRepo) GetPunchNode(context.Context, string) (model.PunchNode, bool, error) {
	return model.PunchNode{}, false, nil
}

func (*deleteRuntimeNodeRepo) SavePunchNode(context.Context, model.PunchNode) error { return nil }

func (r *deleteRuntimeNodeRepo) DeletePunchNode(_ context.Context, nodeID string) error {
	r.punchIDs = append(r.punchIDs, nodeID)
	return nil
}

func TestDeleteServerNodeRemovesManagedRuntimeNodes(t *testing.T) {
	serverNodes := &deleteServerNodeRepo{item: model.ServerNode{
		NodeID: "server1", RelayNodeID: "relay1", RelayTCPNodeID: "derp1", PunchNodeID: "punch1",
	}}
	runtimeNodes := &deleteRuntimeNodeRepo{}
	service := OpsServerNodeService{ServerNodes: serverNodes, Nodes: runtimeNodes}

	if err := service.DeleteServerNode(context.Background(), "server1"); err != nil {
		t.Fatal(err)
	}
	if serverNodes.deleted != "server1" {
		t.Fatalf("deleted server node = %q", serverNodes.deleted)
	}
	if len(runtimeNodes.relayIDs) != 2 || runtimeNodes.relayIDs[0] != "relay1" || runtimeNodes.relayIDs[1] != "derp1" {
		t.Fatalf("deleted relay nodes = %#v", runtimeNodes.relayIDs)
	}
	if len(runtimeNodes.punchIDs) != 1 || runtimeNodes.punchIDs[0] != "punch1" {
		t.Fatalf("deleted punch nodes = %#v", runtimeNodes.punchIDs)
	}
}
