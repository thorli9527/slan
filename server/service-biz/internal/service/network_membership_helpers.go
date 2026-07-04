package service

import (
	"strings"

	"github.com/slan/service-biz/internal/model"
)

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

func networkMemberOnline(item model.NetworkDevice) bool {
	item = normalizeNetworkMember(item)
	if !networkMemberActive(item) {
		return false
	}
	return item.MQTTConnected ||
		item.PresenceStatus == model.DevicePresenceStatusConnected ||
		item.PresenceStatus == model.DevicePresenceStatusActive ||
		item.LastSeenAt > 0 ||
		item.LastHeartbeatAt > 0 ||
		item.LastRuntimeStateAt > 0 ||
		item.LastEndpointAt > 0 ||
		item.LastPathHealthAt > 0 ||
		strings.TrimSpace(item.ActivePath) != "" ||
		len(item.Endpoints) > 0
}
