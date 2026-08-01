package service

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/slan/service-biz/internal/repository"
)

func publishDirectNetworkMembership(
	ctx context.Context,
	networks repository.NetworkRepository,
	publisher DeviceControlPublisher,
	nowFn func() time.Time,
	deviceID string,
	changedNetworkID string,
	operation string,
	version int64,
) error {
	if publisher == nil {
		return nil
	}
	now := networkNow(nowFn)
	messageID := networkMembershipMessageID(now, deviceID, changedNetworkID, operation, version)
	return publishDirectNetworkMembershipWithMessageID(ctx, networks, publisher, deviceID, changedNetworkID, operation, version, messageID)
}

func publishDirectNetworkMembershipWithMessageID(
	ctx context.Context,
	networks repository.NetworkRepository,
	publisher DeviceControlPublisher,
	deviceID string,
	changedNetworkID string,
	operation string,
	version int64,
	messageID string,
) error {
	if publisher == nil {
		return nil
	}
	remaining, err := networks.ListNetworksByDevice(ctx, deviceID)
	if err != nil {
		return err
	}
	networkIDs := make([]string, 0, len(remaining))
	for _, network := range remaining {
		networkIDs = append(networkIDs, network.NetworkID)
	}
	return publisher.PublishDeviceControl(ctx, deviceID, DeviceControlEnvelope{
		Type:      "device_network_membership_changed",
		MessageID: messageID,
		Payload: map[string]any{
			"deviceId":          deviceID,
			"changedNetworkId":  changedNetworkID,
			"networkIds":        networkIDs,
			"membershipVersion": version,
			"operation":         operation,
			"source":            "ops_network_management",
		},
	})
}

func networkMembershipMessageID(
	now time.Time,
	deviceID string,
	networkID string,
	operation string,
	version int64,
) string {
	seed := fmt.Sprintf("%d\x00%s\x00%s\x00%s\x00%d", now.UnixNano(), deviceID, networkID, operation, version)
	sum := sha256.Sum256([]byte(seed))
	return fmt.Sprintf("opsnet-%x", sum[:12])
}
