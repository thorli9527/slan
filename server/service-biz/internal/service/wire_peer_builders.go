package service

import (
	"context"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/pkg/wirekit"
	"github.com/slan/service-biz/internal/repository"
)

func wirePeerAuthzView(peerID, networkID, deviceID string) WirePeerAuthzView {
	return WirePeerAuthzView{
		PeerID:      peerID,
		NetworkID:   networkID,
		NodeID:      wirekit.NodeID(deviceID),
		Enabled:     true,
		VirtualIPs:  []string{wirekit.VirtualIP(networkID, deviceID)},
		AllowedIPs:  []string{wirekit.AllowedIP(networkID, deviceID)},
		QuotaPolicy: "default",
	}
}

func wirePeerRuntimeConfigView(peerID, networkID, deviceID string, endpoints []string, membership model.NetworkDevice) WirePeerRuntimeConfigView {
	return WirePeerRuntimeConfigView{
		PeerID:                peerID,
		NetworkID:             networkID,
		NodeID:                wirekit.NodeID(deviceID),
		VirtualIPs:            []string{wirekit.VirtualIP(networkID, deviceID)},
		AllowedIPs:            []string{wirekit.AllowedIP(networkID, deviceID)},
		KeepaliveIntervalSecs: 30,
		NetworkEnabled:        true,
		PreferredPath:         firstNonEmpty(membership.ActivePath, "direct_udp"),
		Endpoints:             endpoints,
	}
}

func wireTopologyPeers(
	ctx context.Context,
	devices repository.DeviceRepository,
	networkID string,
	items []model.NetworkDevice,
) ([]wirekit.TopologyPeer, error) {
	peers := make([]wirekit.TopologyPeer, 0)
	for _, item := range items {
		if !item.Enabled || item.Status != "active" {
			continue
		}
		device, ok, err := devices.GetDevice(ctx, item.DeviceID)
		if err != nil {
			return nil, err
		}
		if !ok || device.Status != "active" {
			continue
		}
		peers = append(peers, topologyPeer(networkID, device, item))
	}
	return peers, nil
}
