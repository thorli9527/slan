package impl

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lib/pq"
	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/configs"
	controlmsg "github.com/slan/server/server-biz/internal/controlmsg"
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

func TestRuntimeControlRejectsActiveMemberWithoutAttachment(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()
	seedRuntimeMembershipFixture(t, state, "active")

	_, err := state.requireNodeSession(ctx, "user-1", "node-1", "net-1")
	if !errors.Is(err, service.ErrForbidden) {
		t.Fatalf("expected forbidden for member without active attachment, got %v", err)
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
	seedRuntimeAttachment(t, state, "att-1", "dev-1", "100.64.0.2")

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

func TestRuntimePeerReportsRejectPendingPeerMember(t *testing.T) {
	state := newNetworkTestState(t)
	seedRuntimeMembershipFixture(t, state, "active")
	seedRuntimeAttachment(t, state, "att-1", "dev-1", "100.64.0.2")
	seedRuntimePeer(t, state, "pending")

	channel := dbControlChannelService{state: state}
	if err := channel.ReportConnectionState("user-1", "node-1", controlmsg.ConnectionState{
		NetworkID:  "net-1",
		PeerNodeID: "node-2",
		Path:       "relay",
		State:      "connecting",
	}); !errors.Is(err, service.ErrForbidden) {
		t.Fatalf("expected forbidden for pending peer connection state, got %v", err)
	}

	if err := channel.ReportPathHealth("user-1", "node-1", controlmsg.PathHealthReport{
		NetworkID:  "net-1",
		PeerNodeID: "node-2",
		PathType:   "relay",
	}); !errors.Is(err, service.ErrForbidden) {
		t.Fatalf("expected forbidden for pending peer path health, got %v", err)
	}
}

func TestRuntimePeerSnapshotRejectsPendingPeerMember(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()
	now := time.Now()
	seedRuntimeMembershipFixture(t, state, "active")
	seedRuntimeAttachment(t, state, "att-1", "dev-1", "100.64.0.2")
	seedRuntimePeer(t, state, "pending")

	if err := state.pg.CreateControlSession(ctx, repo.ControlSession{
		ControlSessionID: "ctrl-peer",
		UserID:           "user-2",
		DeviceID:         "dev-2",
		NodeID:           "node-2",
		NetworkID:        "net-1",
		SessionToken:     "session-peer",
		ConnectedAt:      now.Unix(),
		LastSeenAt:       now.Unix(),
	}); err != nil {
		t.Fatalf("create peer session: %v", err)
	}

	_, err := dbControlChannelService{state: state}.PeerSnapshot("user-1", "node-1", "net-1", "node-2")
	if !errors.Is(err, service.ErrForbidden) {
		t.Fatalf("expected forbidden for pending peer snapshot, got %v", err)
	}
}

func TestRuntimeRoutesSkipPendingMembers(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()
	seedRuntimeMembershipFixture(t, state, "pending")
	seedRuntimePeer(t, state, "active")

	if err := state.pg.CreateAttachment(ctx, dto.SubnetAttachment{
		AttachmentID: "att-pending",
		NetworkID:    "net-1",
		SubnetID:     "subnet-1",
		DeviceID:     "dev-1",
		VirtualIP:    "100.64.0.2",
		Status:       "active",
	}); err != nil {
		t.Fatalf("create pending attachment: %v", err)
	}
	if err := state.pg.CreateAttachment(ctx, dto.SubnetAttachment{
		AttachmentID: "att-active",
		NetworkID:    "net-1",
		SubnetID:     "subnet-1",
		DeviceID:     "dev-2",
		VirtualIP:    "100.64.0.3",
		Status:       "active",
	}); err != nil {
		t.Fatalf("create active attachment: %v", err)
	}

	routes := state.routesForNetwork(ctx, "net-1")
	if len(routes) != 1 {
		t.Fatalf("expected one route via active member, got %+v", routes)
	}
	if routes[0].ViaNodeID != "node-2" {
		t.Fatalf("expected route via active node, got %+v", routes[0])
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

func seedRuntimePeer(t *testing.T, state *dbState, status string) {
	t.Helper()
	ctx := context.Background()

	if err := state.pg.CreateUser(ctx, repo.User{
		UserID:       "user-2",
		Email:        "runtime-peer@local.slan",
		PasswordHash: "hash",
	}); err != nil {
		t.Fatalf("create peer user: %v", err)
	}
	if err := state.pg.InsertDevice(ctx, repo.Device{
		DeviceID:  "dev-2",
		UserID:    "user-2",
		MachineID: "machine-2",
		Name:      "runtime peer",
		Platform:  "windows",
		Status:    "online",
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
		Status:    status,
		CreatedAt: time.Now().Unix(),
	}); err != nil {
		t.Fatalf("create peer member: %v", err)
	}
}

func seedRuntimeAttachment(t *testing.T, state *dbState, attachmentID, deviceID, virtualIP string) {
	t.Helper()
	if err := state.pg.CreateAttachment(context.Background(), dto.SubnetAttachment{
		AttachmentID: attachmentID,
		NetworkID:    "net-1",
		SubnetID:     "subnet-1",
		DeviceID:     deviceID,
		VirtualIP:    virtualIP,
		Status:       "active",
	}); err != nil {
		t.Fatalf("create runtime attachment: %v", err)
	}
}
