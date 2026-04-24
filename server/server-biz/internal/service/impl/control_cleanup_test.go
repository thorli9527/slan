package impl

import (
	"context"
	"testing"
	"time"

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
