package service

import (
	"context"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

func hasDeviceRuntimeMembershipUpdate(input UpdateDeviceRuntimeInput) bool {
	return input.NetworkID != "" ||
		input.NATType != "" ||
		input.ActivePath != "" ||
		input.PathObservedAt > 0 ||
		input.RelayTransport != "" ||
		input.RelayEndpoint != "" ||
		input.DerpNodeID != "" ||
		input.PeerNodeID != "" ||
		input.PathScore > 0 ||
		input.ObservedRttMs > 0 ||
		input.PacketLossPpm > 0 ||
		input.RelayMtu > 0 ||
		input.MaxFramePayload > 0 ||
		input.TicketExpiresAt != "" ||
		input.TicketRenewDue != nil ||
		input.PathDowngrades > 0 ||
		input.PathUpgrades > 0 ||
		input.LastPathChange != ""
}

func resolveDeviceRuntimeMembership(
	ctx context.Context,
	networks repository.NetworkRepository,
	networkID string,
	deviceID string,
) (string, model.NetworkDevice, bool, error) {
	deviceID = normalizeDeviceID(deviceID)
	networkID = normalizeNetworkID(networkID)
	if deviceID == "" {
		return "", model.NetworkDevice{}, false, ErrInvalidArgument
	}
	if networkID != "" {
		membership, ok, err := findNetworkMembership(ctx, networks, networkID, deviceID)
		return networkID, membership, ok, err
	}
	items, err := networks.ListNetworksByDevice(ctx, deviceID)
	if err != nil {
		return "", model.NetworkDevice{}, false, err
	}
	for _, item := range items {
		membership, ok, err := findNetworkMembership(ctx, networks, item.NetworkID, deviceID)
		if err != nil {
			return "", model.NetworkDevice{}, false, err
		}
		if ok {
			return item.NetworkID, membership, true, nil
		}
	}
	return "", model.NetworkDevice{}, false, nil
}

func findNetworkMembership(
	ctx context.Context,
	networks repository.NetworkRepository,
	networkID string,
	deviceID string,
) (model.NetworkDevice, bool, error) {
	items, err := networks.ListNetworkDevices(ctx, networkID)
	if err != nil {
		return model.NetworkDevice{}, false, err
	}
	for _, item := range items {
		if item.DeviceID == deviceID {
			return item, true, nil
		}
	}
	return model.NetworkDevice{}, false, nil
}
