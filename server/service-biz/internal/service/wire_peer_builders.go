package service

import (
	"context"
	"fmt"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/pkg/wirekit"
	"github.com/slan/service-biz/internal/repository"
)

func wirePeerAuthzView(peerID, networkID string, device model.Device) WirePeerAuthzView {
	globalIP := deviceGlobalIP(device)
	return WirePeerAuthzView{
		PeerID:      peerID,
		NetworkID:   networkID,
		NodeID:      wirekit.NodeID(device.DeviceID),
		Enabled:     true,
		VirtualIPs:  []string{globalIP},
		AllowedIPs:  []string{fmt.Sprintf("%s/32", globalIP)},
		QuotaPolicy: "default",
	}
}

func wirePeerRuntimeConfigView(peerID, networkID string, device model.Device, endpoints []string, membership model.NetworkDevice) WirePeerRuntimeConfigView {
	globalIP := deviceGlobalIP(device)
	return WirePeerRuntimeConfigView{
		PeerID:                peerID,
		NetworkID:             networkID,
		NodeID:                wirekit.NodeID(device.DeviceID),
		VirtualIPs:            []string{globalIP},
		AllowedIPs:            []string{fmt.Sprintf("%s/32", globalIP)},
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
		if !networkMemberActive(item) {
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
