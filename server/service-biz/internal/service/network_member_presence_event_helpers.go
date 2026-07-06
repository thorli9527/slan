package service

import (
	"context"
	"strings"
	"time"

	"github.com/slan/service-biz/internal/model"
)

func publishNetworkMemberChanged(
	ctx context.Context,
	eventPublisher NetworkEventPublisher,
	nowFn func() time.Time,
	networkID string,
	deviceID string,
	action string,
	member model.NetworkDevice,
	version int64,
	reason string,
) error {
	if eventPublisher == nil {
		return nil
	}
	occurredAt := currentTime(nowFn)
	eventType := NetworkEventMemberUpdated
	switch strings.TrimSpace(action) {
	case "added", "member_added", "network_member_added":
		eventType = NetworkEventMemberAdded
	case "removed", "member_removed", "network_member_removed":
		eventType = NetworkEventMemberRemoved
	}
	if eventType == NetworkEventMemberRemoved {
		return publishNetworkEvent(
			ctx,
			eventPublisher,
			eventType,
			networkID,
			uint64(version),
			occurredAt.UnixMilli(),
			NetworkEventMemberRemovedPayload{DeviceID: strings.TrimSpace(deviceID)},
		)
	}
	return publishNetworkEvent(
		ctx,
		eventPublisher,
		eventType,
		networkID,
		uint64(version),
		occurredAt.UnixMilli(),
		NetworkEventMemberPayload{Member: networkEventMemberView(member)},
	)
}

func publishDevicePresenceChanged(
	ctx context.Context,
	eventPublisher NetworkEventPublisher,
	now time.Time,
	networkID string,
	deviceID string,
	member model.NetworkDevice,
) error {
	if eventPublisher == nil {
		return nil
	}
	member = normalizeNetworkMember(member)
	eventType := NetworkEventMemberOffline
	if networkMemberOnline(member) {
		eventType = NetworkEventMemberOnline
	}
	return publishNetworkEvent(
		ctx,
		eventPublisher,
		eventType,
		networkID,
		0,
		now.UnixMilli(),
		NetworkEventPresencePayload{
			DeviceID:   strings.TrimSpace(deviceID),
			Online:     networkMemberOnline(member),
			LastSeenAt: member.LastSeenAt,
		},
	)
}
