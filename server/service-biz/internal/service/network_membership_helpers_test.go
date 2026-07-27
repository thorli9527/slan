package service

import (
	"testing"
	"time"

	"github.com/slan/service-biz/internal/model"
)

func TestNetworkMemberOnlineAtRequiresFreshPresence(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	active := model.NetworkDevice{
		Enabled:      true,
		MemberStatus: model.NetworkMemberStatusActive,
	}

	tests := []struct {
		name   string
		member model.NetworkDevice
		want   bool
	}{
		{
			name: "connected mqtt session",
			member: func() model.NetworkDevice {
				item := active
				item.MQTTConnected = true
				return item
			}(),
			want: true,
		},
		{
			name: "fresh runtime state",
			member: func() model.NetworkDevice {
				item := active
				item.LastRuntimeStateAt = now.Add(-time.Minute).Unix()
				return item
			}(),
			want: true,
		},
		{
			name: "stale active presence",
			member: func() model.NetworkDevice {
				item := active
				item.PresenceStatus = model.DevicePresenceStatusActive
				item.LastRuntimeStateAt = now.Add(-3 * time.Minute).Unix()
				item.Endpoints = []model.DeviceEndpoint{{UpdatedAt: now.Add(-3 * time.Minute).Unix()}}
				return item
			}(),
			want: false,
		},
		{
			name: "fresh endpoint",
			member: func() model.NetworkDevice {
				item := active
				item.Endpoints = []model.DeviceEndpoint{{UpdatedAt: now.Add(-time.Minute).Unix()}}
				return item
			}(),
			want: true,
		},
		{
			name: "disabled membership",
			member: model.NetworkDevice{
				Enabled:            false,
				MemberStatus:       model.NetworkMemberStatusActive,
				LastRuntimeStateAt: now.Unix(),
			},
			want: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := networkMemberOnlineAt(test.member, now); got != test.want {
				t.Fatalf("networkMemberOnlineAt() = %v, want %v", got, test.want)
			}
		})
	}
}
