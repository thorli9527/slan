package impl

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/lib/pq"
	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/configs"
	"github.com/slan/server/server-biz/internal/repo"
	"github.com/slan/server/server-biz/internal/service"
)

func TestBootstrapPreconditions(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()
	bootstrap := dbBootstrapService{state: state}

	if _, err := bootstrap.Bootstrap("user-1", dto.BootstrapRequest{NetworkID: "net-1"}); !errors.Is(err, service.ErrInvalidArgument) {
		t.Fatalf("expected invalid argument for missing node id, got %v", err)
	}
	if _, err := bootstrap.Bootstrap("user-1", dto.BootstrapRequest{NodeID: "missing-node", NetworkID: "net-1"}); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("expected not found for unknown node, got %v", err)
	}

	if err := state.pg.CreateUser(ctx, repo.User{
		UserID:       "user-1",
		Email:        "user-1@local.slan",
		PasswordHash: "hash",
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := state.pg.InsertDevice(ctx, repo.Device{
		DeviceID:  "dev-1",
		UserID:    "user-1",
		MachineID: "machine-1",
		Name:      "device",
		Platform:  "windows",
		Status:    "offline",
		CreatedAt: time.Now().Unix(),
	}); err != nil {
		t.Fatalf("create device: %v", err)
	}
	if err := state.pg.UpsertNode(ctx, repo.Node{
		NodeID:        "node-1",
		UserID:        "user-1",
		DeviceID:      "dev-1",
		NodePublicKey: "node-pub-1",
		Capabilities:  pq.StringArray{},
	}); err != nil {
		t.Fatalf("create node: %v", err)
	}
	if _, err := bootstrap.Bootstrap("user-1", dto.BootstrapRequest{NodeID: "node-1", NetworkID: "missing-net"}); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("expected not found for unknown network, got %v", err)
	}

	createNetworkFixture(t, state, "user-1", "net-1", "subnet-1", "100.64.0.0/24")
	if err := state.pg.CreateMember(ctx, dto.NetworkMember{
		MemberID:  "member-1",
		NetworkID: "net-1",
		DeviceID:  "dev-1",
		Role:      "owner",
		Status:    "active",
		CreatedAt: time.Now().Unix(),
	}); err != nil {
		t.Fatalf("create member: %v", err)
	}
	if _, err := bootstrap.Bootstrap("user-1", dto.BootstrapRequest{NodeID: "node-1", NetworkID: "net-1"}); !errors.Is(err, service.ErrForbidden) {
		t.Fatalf("expected forbidden without active attachment, got %v", err)
	}
}

func TestRelayTicketValidatesAccessAndCachesNormalizedRequest(t *testing.T) {
	state := newNetworkTestState(t)
	state.cfg = configs.DefaultConfig()
	state.cachedRelayTicket = make(map[string]cachedRelayTicket)
	ctx := context.Background()
	seedRuntimeMembershipFixture(t, state, "active")
	seedRuntimeAttachment(t, state, "att-1", "dev-1", "100.64.0.2")
	seedRuntimePeer(t, state, "active")
	seedRuntimeAttachment(t, state, "att-2", "dev-2", "100.64.0.3")

	req := dto.RelayTicketRequest{
		NetworkID:            "net-1",
		SrcNodeID:            "node-1",
		DstNodeID:            "node-2",
		DerpClusterID:        "cn-local-a",
		PreferredDerpNodeIDs: []string{"relay-cn-local-tcp", "missing-node", "relay-cn-local-udp", "relay-cn-local-tcp"},
		Reason:               "direct path failed",
	}
	first, err := state.issueRelayTicket(ctx, "user-1", req)
	if err != nil {
		t.Fatalf("issue first ticket: %v", err)
	}
	second, err := state.issueRelayTicket(ctx, "user-1", req)
	if err != nil {
		t.Fatalf("issue cached ticket: %v", err)
	}
	if first.TicketID == "" || first.Signature == "" || first.SessionKey == "" {
		t.Fatalf("expected signed relay ticket, got %+v", first)
	}
	if first.TicketID != second.TicketID || first.SessionID != second.SessionID {
		t.Fatalf("expected cached ticket reuse, got first=%+v second=%+v", first, second)
	}
	wantNodes := []string{"relay-cn-local-tcp", "relay-cn-local-udp"}
	if !reflect.DeepEqual(first.AllowedDerpNodeIDs, wantNodes) {
		t.Fatalf("expected normalized relay node ids %v, got %v", wantNodes, first.AllowedDerpNodeIDs)
	}

	if _, err := state.issueRelayTicket(ctx, "user-2", req); !errors.Is(err, service.ErrForbidden) {
		t.Fatalf("expected forbidden when source node belongs to another user, got %v", err)
	}
	if _, err := state.issueRelayTicket(ctx, "user-1", dto.RelayTicketRequest{
		NetworkID: "net-1",
		SrcNodeID: "node-1",
		DstNodeID: "missing-node",
		Reason:    "direct path failed",
	}); !errors.Is(err, service.ErrNotFound) {
		t.Fatalf("expected not found for missing destination node, got %v", err)
	}
}
