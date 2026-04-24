package impl

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lib/pq"
	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/configs"
	"github.com/slan/server/server-biz/internal/repo"
	"github.com/slan/server/server-biz/internal/service"
)

func TestRuntimeControlRejectsPendingNetworkMember(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()
	seedRuntimeMembershipFixture(t, state, "pending")

	_, err := state.requireNodeSession(ctx, "user-1", "node-1", "net-1")
	if !errors.Is(err, service.ErrForbidden) {
		t.Fatalf("expected forbidden for pending member, got %v", err)
	}
}

func TestRuntimeNodeListExcludesPendingNetworkMembers(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()
	seedRuntimeMembershipFixture(t, state, "pending")

	nodes, err := state.pg.ListNodesByNetwork(ctx, "net-1")
	if err != nil {
		t.Fatalf("list nodes: %v", err)
	}
	if len(nodes) != 0 {
		t.Fatalf("expected pending member node to be hidden from runtime map, got %+v", nodes)
	}
}

func TestRelayTicketRejectsPendingDestinationMember(t *testing.T) {
	state := newNetworkTestState(t)
	state.cfg = configs.DefaultConfig()
	ctx := context.Background()
	seedRuntimeMembershipFixture(t, state, "active")

	if err := state.pg.InsertDevice(ctx, repo.Device{
		DeviceID:  "dev-2",
		UserID:    "user-2",
		MachineID: "machine-2",
		Name:      "pending peer",
		Platform:  "windows",
		Status:    "offline",
		CreatedAt: time.Now().Unix(),
	}); err != nil {
		t.Fatalf("insert peer device: %v", err)
	}
	if err := state.pg.UpsertNode(ctx, repo.Node{
		NodeID:        "node-2",
		UserID:        "user-2",
		DeviceID:      "dev-2",
		NodePublicKey: "node-pub-2",
		Capabilities:  pq.StringArray{},
	}); err != nil {
		t.Fatalf("upsert peer node: %v", err)
	}
	if err := state.pg.CreateMember(ctx, dto.NetworkMember{
		MemberID:  "member-2",
		NetworkID: "net-1",
		DeviceID:  "dev-2",
		Role:      "member",
		Status:    "pending",
		CreatedAt: time.Now().Unix(),
	}); err != nil {
		t.Fatalf("create pending peer member: %v", err)
	}

	_, err := state.issueRelayTicket(ctx, "user-1", dto.RelayTicketRequest{
		NetworkID: "net-1",
		SrcNodeID: "node-1",
		DstNodeID: "node-2",
		Reason:    "runtime-test",
	})
	if !errors.Is(err, service.ErrForbidden) {
		t.Fatalf("expected forbidden for pending destination member, got %v", err)
	}
}

func seedRuntimeMembershipFixture(t *testing.T, state *dbState, status string) {
	t.Helper()
	ctx := context.Background()

	if err := state.pg.CreateUser(ctx, repo.User{
		UserID:       "user-1",
		Email:        "runtime-owner@local.slan",
		PasswordHash: "hash",
	}); err != nil {
		t.Fatalf("create owner user: %v", err)
	}
	createNetworkFixture(t, state, "user-1", "net-1", "subnet-1", "100.64.0.0/24")
	if err := state.pg.InsertDevice(ctx, repo.Device{
		DeviceID:  "dev-1",
		UserID:    "user-1",
		MachineID: "machine-1",
		Name:      "runtime device",
		Platform:  "windows",
		Status:    "offline",
		CreatedAt: time.Now().Unix(),
	}); err != nil {
		t.Fatalf("insert device: %v", err)
	}
	if err := state.pg.UpsertNode(ctx, repo.Node{
		NodeID:        "node-1",
		UserID:        "user-1",
		DeviceID:      "dev-1",
		NodePublicKey: "node-pub-1",
		Capabilities:  pq.StringArray{},
	}); err != nil {
		t.Fatalf("upsert node: %v", err)
	}
	if err := state.pg.CreateMember(ctx, dto.NetworkMember{
		MemberID:  "member-1",
		NetworkID: "net-1",
		DeviceID:  "dev-1",
		Role:      "owner",
		Status:    status,
		CreatedAt: time.Now().Unix(),
	}); err != nil {
		t.Fatalf("create member: %v", err)
	}
}
