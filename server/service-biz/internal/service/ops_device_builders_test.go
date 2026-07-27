package service

import (
	"testing"
	"time"

	"github.com/slan/service-biz/internal/model"
)

func TestDeviceHeartbeatOnlineAtSeparatesPresenceFromNetworkState(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)

	tests := []struct {
		name   string
		device model.Device
		want   bool
	}{
		{
			name:   "recent active heartbeat",
			device: model.Device{Status: "active", LastSeenAt: now.Add(-time.Minute).Unix()},
			want:   true,
		},
		{
			name:   "stale active heartbeat",
			device: model.Device{Status: "active", LastSeenAt: now.Add(-3 * time.Minute).Unix()},
			want:   false,
		},
		{
			name:   "recent disabled device",
			device: model.Device{Status: "inactive", LastSeenAt: now.Unix()},
			want:   false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := deviceHeartbeatOnlineAt(test.device, now); got != test.want {
				t.Fatalf("deviceHeartbeatOnlineAt() = %v, want %v", got, test.want)
			}
		})
	}
}
