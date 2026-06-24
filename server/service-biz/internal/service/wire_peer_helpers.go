package service

import (
	"context"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

func peerEndpoints(ctx context.Context, networks repository.NetworkRepository, networkID, deviceID string) ([]string, model.NetworkDevice, bool) {
	items, err := networks.ListNetworkDevices(ctx, networkID)
	if err != nil {
		return []string{}, model.NetworkDevice{}, false
	}
	for _, item := range items {
		if item.DeviceID != deviceID || !item.Enabled || item.Status != "active" {
			continue
		}
		return networkDeviceEndpoints(item), item, true
	}
	return []string{}, model.NetworkDevice{}, false
}

func networkDeviceEndpoints(item model.NetworkDevice) []string {
	endpoints := make([]string, 0, len(item.Endpoints))
	for _, endpoint := range item.Endpoints {
		if endpoint.Address == "" {
			continue
		}
		endpoints = append(endpoints, endpoint.Address)
	}
	return endpoints
}
