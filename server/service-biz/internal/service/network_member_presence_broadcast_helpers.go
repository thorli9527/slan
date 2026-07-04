package service

import (
	"context"
	"strings"
	"time"

	"github.com/slan/service-biz/internal/model"
)

func publishNetworkMemberChanged(
	ctx context.Context,
	broadcaster networkBroadcastPublisher,
	nowFn func() time.Time,
	networkID string,
	deviceID string,
	action string,
	member model.NetworkDevice,
	version int64,
	reason string,
) error {
	if broadcaster == nil {
		return nil
	}
	return broadcaster.PublishNetworkMemberChanged(ctx, networkBroadcastMemberChanged{
		NetworkID:  strings.TrimSpace(networkID),
		DeviceID:   strings.TrimSpace(deviceID),
		Action:     strings.TrimSpace(action),
		Member:     member,
		Version:    version,
		Reason:     strings.TrimSpace(reason),
		OccurredAt: currentTime(nowFn),
	})
}

func publishDevicePresenceChanged(
	ctx context.Context,
	broadcaster networkBroadcastPublisher,
	now time.Time,
	networkID string,
	deviceID string,
	member model.NetworkDevice,
) error {
	if broadcaster == nil {
		return nil
	}
	member = normalizeNetworkMember(member)
	return broadcaster.PublishDevicePresenceChanged(ctx, networkBroadcastDevicePresenceChanged{
		NetworkID:          strings.TrimSpace(networkID),
		DeviceID:           strings.TrimSpace(deviceID),
		PresenceStatus:     member.PresenceStatus,
		MQTTConnected:      member.MQTTConnected,
		ActivePath:         member.ActivePath,
		LastSeenAt:         member.LastSeenAt,
		LastRuntimeStateAt: member.LastRuntimeStateAt,
		OccurredAt:         now,
	})
}
