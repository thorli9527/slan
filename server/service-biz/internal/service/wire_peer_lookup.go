package service

import (
	"context"

	"github.com/slan/service-biz/internal/pkg/wirekit"
)

func (s WirePeerService) lookupPeer(ctx context.Context, peerID string) (string, string, error) {
	peerID = normalizeWirePeerID(peerID)
	if peerID == "" {
		return "", "", ErrInvalidArgument
	}
	networkID, deviceID := wirekit.ParsePeerID(peerID)
	if deviceID == "" {
		return "", "", ErrInvalidArgument
	}
	if networkID != "" {
		items, err := s.Networks.ListNetworkDevices(ctx, networkID)
		if err != nil {
			return "", "", err
		}
		for _, item := range items {
			if item.DeviceID == deviceID {
				return networkID, deviceID, nil
			}
		}
		return "", "", ErrNotFound
	}
	networks, err := s.Networks.ListNetworksByDevice(ctx, deviceID)
	if err != nil {
		return "", "", err
	}
	for _, network := range networks {
		items, err := s.Networks.ListNetworkDevices(ctx, network.NetworkID)
		if err != nil {
			continue
		}
		for _, item := range items {
			if item.DeviceID == deviceID && networkMemberActive(item) {
				return network.NetworkID, deviceID, nil
			}
		}
	}
	return "", "", ErrNotFound
}
