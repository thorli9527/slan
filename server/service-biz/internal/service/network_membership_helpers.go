package service

import (
	"time"

	"github.com/slan/service-biz/internal/model"
)

const deviceOnlineFreshnessWindow = 2 * time.Minute

func normalizeNetworkMember(item model.NetworkDevice) model.NetworkDevice {
	if item.MemberStatus == "" {
		item.MemberStatus = model.NetworkMemberStatusActive
	}
	if item.PresenceStatus == "" {
		item.PresenceStatus = model.DevicePresenceStatusOffline
	}
	return item
}

func networkMemberActive(item model.NetworkDevice) bool {
	item = normalizeNetworkMember(item)
	return item.Enabled && item.MemberStatus == model.NetworkMemberStatusActive
}

func networkMemberActivePtr(item *model.NetworkDevice) bool {
	if item == nil {
		return false
	}
	return networkMemberActive(*item)
}

func networkMemberOnlineAt(item model.NetworkDevice, now time.Time) bool {
	item = normalizeNetworkMember(item)
	if !networkMemberActive(item) {
		return false
	}
	if item.MQTTConnected {
		return true
	}
	latest := maxTimestamp(
		item.LastSeenAt,
		item.LastHeartbeatAt,
		item.LastRuntimeStateAt,
		item.LastEndpointAt,
		item.LastPathHealthAt,
	)
	for _, endpoint := range item.Endpoints {
		if endpoint.UpdatedAt > latest {
			latest = endpoint.UpdatedAt
		}
	}
	return latest > 0 && latest >= now.Add(-deviceOnlineFreshnessWindow).Unix()
}

func maxTimestamp(values ...int64) int64 {
	var latest int64
	for _, value := range values {
		if value > latest {
			latest = value
		}
	}
	return latest
}
