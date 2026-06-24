package service

import (
	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/pkg/wirekit"
)

func topologyPeer(networkID string, device model.Device, membership model.NetworkDevice) wirekit.TopologyPeer {
	endpoints := make([]string, 0, len(membership.Endpoints))
	for _, endpoint := range membership.Endpoints {
		if endpoint.Address == "" {
			continue
		}
		endpoints = append(endpoints, endpoint.Address)
	}
	activePath := membership.ActivePath
	if activePath == "" {
		activePath = "direct_udp"
	}
	updatedAt := membership.UpdatedAt
	if membership.PathObservedAt > updatedAt {
		updatedAt = membership.PathObservedAt
	}
	preferLan := activePath == "lan_udp"
	preferIPv6 := activePath == "ipv6_udp"
	requireMtuRefresh := false
	if (activePath == "relay_udp" || activePath == "derp_tcp_tls_443") && membership.RelayMtu > 0 && membership.MaxFramePayload <= 0 {
		requireMtuRefresh = true
	}
	return wirekit.TopologyPeer{
		PeerID:                  wirekit.PeerID(networkID, device.DeviceID),
		NetworkID:               networkID,
		NodeID:                  wirekit.NodeID(device.DeviceID),
		PublicKey:               "",
		VirtualIPs:              []string{wirekit.VirtualIP(networkID, device.DeviceID)},
		AllowedIPs:              []string{wirekit.AllowedIP(networkID, device.DeviceID)},
		SupportsLanDirect:       true,
		SupportsIpv6Direct:      true,
		SupportsDirectUdp:       true,
		SupportsRelayUdp:        membership.RelayTransport == "" || membership.RelayTransport == "udp" || membership.ActivePath == "relay_udp",
		SupportsDerpTcpTls443:   membership.RelayTransport == "derp_tcp_tls_443" || membership.ActivePath == "derp_tcp_tls_443",
		PreferIpv6:              preferIPv6,
		PreferLan:               preferLan,
		AllowEndpointRoaming:    true,
		AllowFastReselection:    true,
		AllowRelayTicketRenewal: true,
		KeepaliveIntervalSecs:   30,
		Endpoints:               endpoints,
		ActivePath:              activePath,
		RequireMtuRefresh:       requireMtuRefresh,
		EndpointChanged:         len(endpoints) > 0 || membership.PathObservedAt > 0,
		UpdatedAt:               updatedAt,
	}
}
