package wire

import servicepkg "github.com/slan/service-biz/internal/service"

func wireNodePayload(item servicepkg.WireNodeView) map[string]any {
	return map[string]any{
		"regionId":          item.RegionID,
		"nodeId":            item.NodeID,
		"name":              item.Name,
		"host":              item.Host,
		"udpPort":           item.UDPPort,
		"adminPort":         item.AdminPort,
		"port":              item.Port,
		"enabled":           item.Enabled,
		"healthy":           item.Healthy,
		"stale":             item.Stale,
		"priority":          item.Priority,
		"updatedAtMs":       item.UpdatedAtMS,
		"ticketKeyRotation": item.TicketKeyRotation,
	}
}

func wirePeerAuthzPayload(view servicepkg.WirePeerAuthzView) map[string]any {
	return map[string]any{
		"peerId":      view.PeerID,
		"networkId":   view.NetworkID,
		"nodeId":      view.NodeID,
		"enabled":     view.Enabled,
		"virtualIps":  view.VirtualIPs,
		"allowedIps":  view.AllowedIPs,
		"quotaPolicy": view.QuotaPolicy,
	}
}

func wirePeerRuntimeConfigPayload(view servicepkg.WirePeerRuntimeConfigView) map[string]any {
	return map[string]any{
		"peerId":                view.PeerID,
		"networkId":             view.NetworkID,
		"nodeId":                view.NodeID,
		"virtualIps":            view.VirtualIPs,
		"allowedIps":            view.AllowedIPs,
		"keepaliveIntervalSecs": view.KeepaliveIntervalSecs,
		"networkEnabled":        view.NetworkEnabled,
		"preferredPath":         view.PreferredPath,
		"endpoints":             view.Endpoints,
	}
}

func wirePeerPathHealthAcceptedPayload(peerID string) map[string]any {
	return map[string]any{
		"accepted": true,
		"peerId":   peerID,
	}
}

func wireTopologyPayload(view servicepkg.WireTopologyView) map[string]any {
	peers := make([]map[string]any, 0, len(view.Peers))
	for _, item := range view.Peers {
		peers = append(peers, map[string]any{
			"peerId":                  item.PeerID,
			"networkId":               item.NetworkID,
			"nodeId":                  item.NodeID,
			"publicKey":               item.PublicKey,
			"virtualIPs":              item.VirtualIPs,
			"allowedIPs":              item.AllowedIPs,
			"supportsLanDirect":       item.SupportsLanDirect,
			"supportsIpv6Direct":      item.SupportsIpv6Direct,
			"supportsDirectUdp":       item.SupportsDirectUdp,
			"supportsRelayUdp":        item.SupportsRelayUdp,
			"supportsDerpTcpTls443":   item.SupportsDerpTcpTls443,
			"preferIpv6":              item.PreferIpv6,
			"preferLan":               item.PreferLan,
			"allowEndpointRoaming":    item.AllowEndpointRoaming,
			"allowFastReselection":    item.AllowFastReselection,
			"allowRelayTicketRenewal": item.AllowRelayTicketRenewal,
			"keepaliveIntervalSecs":   item.KeepaliveIntervalSecs,
			"endpoints":               item.Endpoints,
			"activePath":              item.ActivePath,
			"requireMtuRefresh":       item.RequireMtuRefresh,
			"endpointChanged":         item.EndpointChanged,
			"updatedAt":               item.UpdatedAt,
		})
	}
	return map[string]any{
		"networkId": view.NetworkID,
		"peers":     peers,
	}
}

func wireDerpMapPayload(view servicepkg.WireDerpMapView) map[string]any {
	regions := make([]map[string]any, 0, len(view.Regions))
	for _, region := range view.Regions {
		nodes := make([]map[string]any, 0, len(region.Nodes))
		for _, node := range region.Nodes {
			nodes = append(nodes, map[string]any{
				"regionId": node.RegionID,
				"nodeId":   node.NodeID,
				"host":     node.Host,
				"port":     node.Port,
			})
		}
		regions = append(regions, map[string]any{
			"regionId": region.RegionID,
			"name":     region.Name,
			"nodes":    nodes,
		})
	}
	return map[string]any{
		"preferredRegionId": view.PreferredRegionID,
		"regions":           regions,
	}
}
