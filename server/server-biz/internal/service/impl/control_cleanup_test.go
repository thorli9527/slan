package impl

import (
	"context"
	"testing"
	"time"

	"github.com/lib/pq"
	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
)

func TestCleanupExpiredControlPlaneStateMarksStaleDeviceOffline(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()
	now := time.Now()

	if err := state.pg.InsertDevice(ctx, repo.Device{
		DeviceID:  "dev-stale",
		UserID:    "user-1",
		MachineID: "machine-stale",
		Name:      "stale",
		Platform:  "windows",
		Status:    "online",
		CreatedAt: now.Unix(),
	}); err != nil {
		t.Fatalf("insert device: %v", err)
	}
	if err := state.pg.CreateControlSession(ctx, repo.ControlSession{
		ControlSessionID: "ctrl-stale",
		UserID:           "user-1",
		DeviceID:         "dev-stale",
		NodeID:           "node-stale",
		NetworkID:        "net-1",
		SessionToken:     "session-stale",
		ConnectedAt:      now.Add(-5 * time.Minute).Unix(),
		LastSeenAt:       now.Add(-5 * time.Minute).Unix(),
	}); err != nil {
		t.Fatalf("create stale session: %v", err)
	}

	state.cleanupExpiredControlPlaneState(ctx, now)

	device, err := state.pg.GetDeviceByID(ctx, "dev-stale")
	if err != nil {
		t.Fatalf("load device: %v", err)
	}
	if device.Status != "offline" {
		t.Fatalf("expected stale device offline, got %s", device.Status)
	}
}

func TestCleanupExpiredControlPlaneStateKeepsDeviceOnlineWithFreshSession(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()
	now := time.Now()

	if err := state.pg.InsertDevice(ctx, repo.Device{
		DeviceID:  "dev-fresh",
		UserID:    "user-1",
		MachineID: "machine-fresh",
		Name:      "fresh",
		Platform:  "windows",
		Status:    "online",
		CreatedAt: now.Unix(),
	}); err != nil {
		t.Fatalf("insert device: %v", err)
	}
	if err := state.pg.CreateControlSession(ctx, repo.ControlSession{
		ControlSessionID: "ctrl-old",
		UserID:           "user-1",
		DeviceID:         "dev-fresh",
		NodeID:           "node-old",
		NetworkID:        "net-1",
		SessionToken:     "session-old",
		ConnectedAt:      now.Add(-5 * time.Minute).Unix(),
		LastSeenAt:       now.Add(-5 * time.Minute).Unix(),
	}); err != nil {
		t.Fatalf("create old session: %v", err)
	}
	if err := state.pg.CreateControlSession(ctx, repo.ControlSession{
		ControlSessionID: "ctrl-fresh",
		UserID:           "user-1",
		DeviceID:         "dev-fresh",
		NodeID:           "node-fresh",
		NetworkID:        "net-2",
		SessionToken:     "session-fresh",
		ConnectedAt:      now.Add(-10 * time.Second).Unix(),
		LastSeenAt:       now.Add(-10 * time.Second).Unix(),
	}); err != nil {
		t.Fatalf("create fresh session: %v", err)
	}

	state.cleanupExpiredControlPlaneState(ctx, now)

	device, err := state.pg.GetDeviceByID(ctx, "dev-fresh")
	if err != nil {
		t.Fatalf("load device: %v", err)
	}
	if device.Status != "online" {
		t.Fatalf("expected fresh device online, got %s", device.Status)
	}
}

func TestCloseSessionKeepsDeviceOnlineWithAnotherFreshSession(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()
	now := time.Now()

	if err := state.pg.CreateUser(ctx, repo.User{
		UserID:       "user-1",
		Email:        "user@example.com",
		PasswordHash: "hash",
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}
	createNetworkFixture(t, state, "user-1", "net-1", "subnet-1", "100.64.0.0/24")
	createNetworkFixture(t, state, "user-1", "net-2", "subnet-2", "100.65.0.0/24")
	if err := state.pg.InsertDevice(ctx, repo.Device{
		DeviceID:  "dev-1",
		UserID:    "user-1",
		MachineID: "machine-1",
		Name:      "device",
		Platform:  "windows",
		Status:    "online",
		CreatedAt: now.Unix(),
	}); err != nil {
		t.Fatalf("insert device: %v", err)
	}
	for _, member := range []dto.NetworkMember{
		{MemberID: "member-1", NetworkID: "net-1", DeviceID: "dev-1", Role: "owner", Status: "active", CreatedAt: now.Unix()},
		{MemberID: "member-2", NetworkID: "net-2", DeviceID: "dev-1", Role: "owner", Status: "active", CreatedAt: now.Unix()},
	} {
		if err := state.pg.CreateMember(ctx, member); err != nil {
			t.Fatalf("create member %s: %v", member.MemberID, err)
		}
	}
	for _, attachment := range []dto.SubnetAttachment{
		{AttachmentID: "att-1", NetworkID: "net-1", SubnetID: "subnet-1", DeviceID: "dev-1", VirtualIP: "100.64.0.2", Status: "active"},
		{AttachmentID: "att-2", NetworkID: "net-2", SubnetID: "subnet-2", DeviceID: "dev-1", VirtualIP: "100.65.0.2", Status: "active"},
	} {
		if err := state.pg.CreateAttachment(ctx, attachment); err != nil {
			t.Fatalf("create attachment %s: %v", attachment.AttachmentID, err)
		}
	}
	for _, node := range []repo.Node{
		{NodeID: "node-1", UserID: "user-1", DeviceID: "dev-1", NodePublicKey: "node-pub-1", Capabilities: pq.StringArray{}},
		{NodeID: "node-2", UserID: "user-1", DeviceID: "dev-1", NodePublicKey: "node-pub-2", Capabilities: pq.StringArray{}},
	} {
		if err := state.pg.UpsertNode(ctx, node); err != nil {
			t.Fatalf("upsert node %s: %v", node.NodeID, err)
		}
	}
	for _, session := range []repo.ControlSession{
		{
			ControlSessionID: "ctrl-1",
			UserID:           "user-1",
			DeviceID:         "dev-1",
			NodeID:           "node-1",
			NetworkID:        "net-1",
			SessionToken:     "token-1",
			ConnectedAt:      now.Unix(),
			LastSeenAt:       now.Unix(),
		},
		{
			ControlSessionID: "ctrl-2",
			UserID:           "user-1",
			DeviceID:         "dev-1",
			NodeID:           "node-2",
			NetworkID:        "net-2",
			SessionToken:     "token-2",
			ConnectedAt:      now.Unix(),
			LastSeenAt:       now.Unix(),
		},
	} {
		if err := state.pg.CreateControlSession(ctx, session); err != nil {
			t.Fatalf("create session %s: %v", session.ControlSessionID, err)
		}
		if err := state.tokens.StoreControlSessionToken(ctx, session.SessionToken, session.UserID, time.Hour); err != nil {
			t.Fatalf("store session token %s: %v", session.SessionToken, err)
		}
	}

	if err := (dbControlChannelService{state: state}).CloseSession("user-1", "node-1", "net-1"); err != nil {
		t.Fatalf("close session: %v", err)
	}

	device, err := state.pg.GetDeviceByID(ctx, "dev-1")
	if err != nil {
		t.Fatalf("load device: %v", err)
	}
	if device.Status != "online" {
		t.Fatalf("expected device to stay online, got %s", device.Status)
	}
	tokenStore := state.tokens.(*memoryTokenStore)
	if _, err := tokenStore.AuthenticateControlSessionToken(ctx, "token-1"); err == nil {
		t.Fatal("expected closed session token to be revoked")
	}
	if _, err := tokenStore.AuthenticateControlSessionToken(ctx, "token-2"); err != nil {
		t.Fatalf("expected other fresh session token to remain valid: %v", err)
	}
}
