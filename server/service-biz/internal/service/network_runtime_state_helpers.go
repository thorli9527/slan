package service

import (
	"context"
	"github.com/slan/service-biz/internal/repository"
	"strings"
)

func runtimePathForDevice(ctx context.Context, networks repository.NetworkRepository, networkID, deviceID string) (NetworkRuntimePathView, bool, error) {
	networkID = normalizeNetworkID(networkID)
	deviceID = normalizeDeviceID(deviceID)
	if networkID == "" || deviceID == "" {
		return NetworkRuntimePathView{}, false, ErrInvalidArgument
	}
	items, err := networks.ListNetworkDevices(ctx, networkID)
	if err != nil {
		return NetworkRuntimePathView{}, false, err
	}
	for _, item := range items {
		if item.DeviceID != deviceID {
			continue
		}
		return networkRuntimePathView(item, true), true, nil
	}
	return NetworkRuntimePathView{}, false, nil
}

func runtimePathForNode(ctx context.Context, networks repository.NetworkRepository, networkID, nodeID string) (NetworkRuntimePathView, bool, error) {
	nodeID = strings.TrimSpace(nodeID)
	deviceID := strings.TrimPrefix(nodeID, "node-")
	return runtimePathForDevice(ctx, networks, networkID, deviceID)
}
