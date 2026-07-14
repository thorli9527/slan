package service

import (
	"context"
)

func (s NetworkInviteService) ListNetworkDevices(ctx context.Context, networkID string) ([]NetworkDeviceView, error) {
	networkID = normalizeNetworkID(networkID)
	if networkID == "" {
		return []NetworkDeviceView{}, nil
	}
	if _, err := requireManagedNetwork(ctx, s.Networks, networkID); err != nil {
		return nil, err
	}
	items, err := s.Networks.ListNetworkDevices(ctx, networkID)
	if err != nil {
		return nil, err
	}
	return buildNetworkDeviceViews(ctx, s.Devices, items)
}
