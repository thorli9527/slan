package impl

import (
	"context"
	"testing"
	"time"

	"github.com/slan/server/server-biz/internal/repo"
)

func TestOpsOverviewCountsFreshNetworkOnlineDevices(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()
	now := time.Now()

	for _, device := range []repo.Device{
		{DeviceID: "dev-online", UserID: "user-1", MachineID: "machine-1", Name: "online", Platform: "windows", Status: "reachable", CreatedAt: now.Unix()},
		{DeviceID: "dev-reachable", UserID: "user-1", MachineID: "machine-2", Name: "reachable", Platform: "windows", Status: "reachable", CreatedAt: now.Unix()},
		{DeviceID: "dev-stale", UserID: "user-1", MachineID: "machine-3", Name: "stale", Platform: "windows", Status: "online", CreatedAt: now.Unix()},
	} {
		if err := state.pg.InsertDevice(ctx, device); err != nil {
			t.Fatalf("insert device %s: %v", device.DeviceID, err)
		}
	}
	for _, stateRecord := range []repo.DeviceNetworkState{
		{DeviceID: "dev-online", NetworkID: "net-1", ControlReachable: true, NetworkOnline: true, TunnelUp: true, LastProbeOK: true, LastSeenAt: now.Unix(), UpdatedAt: now.Unix()},
		{DeviceID: "dev-reachable", NetworkID: "net-1", ControlReachable: true, NetworkOnline: false, TunnelUp: false, LastProbeOK: false, LastSeenAt: now.Unix(), UpdatedAt: now.Unix()},
		{DeviceID: "dev-stale", NetworkID: "net-1", ControlReachable: true, NetworkOnline: true, TunnelUp: true, LastProbeOK: true, LastSeenAt: now.Add(-2 * time.Minute).Unix(), UpdatedAt: now.Add(-2 * time.Minute).Unix()},
	} {
		if err := state.pg.UpsertDeviceNetworkState(ctx, stateRecord); err != nil {
			t.Fatalf("upsert state for %s: %v", stateRecord.DeviceID, err)
		}
	}

	overview, err := (dbOpsService{state: state}).Overview()
	if err != nil {
		t.Fatalf("overview: %v", err)
	}
	if overview.OnlineDeviceCount != 1 {
		t.Fatalf("expected only fresh network-online device to be counted, got %+v", overview)
	}
}
