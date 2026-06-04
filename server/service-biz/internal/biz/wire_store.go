package biz

import (
	"strings"
	"time"
)

func (s *Store) UpdateWireNodeHeartbeat(nodeID, transport string, req wireNodeHeartbeatRequest) (OpsRelayNode, error) {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return OpsRelayNode{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	node, ok := s.relayNodes[nodeID]
	if !ok {
		return OpsRelayNode{}, errNotFound
	}
	if node.Transport != transport {
		return OpsRelayNode{}, errNotFound
	}
	if node.Status == "disabled" {
		node.Health = "down"
	} else if req.Healthy {
		node.Health = "healthy"
	} else {
		node.Health = "down"
	}
	node.TicketKeyRotation = req.TicketKeyRotation
	node.UpdatedAt = time.Now().Unix()
	s.relayNodes[nodeID] = node
	return node, nil
}

func (s *Store) UpdateWireNodeStatus(nodeID, transport string, req wireNodeStatusRequest) (OpsRelayNode, error) {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" || (req.Enabled == nil && req.Healthy == nil) {
		return OpsRelayNode{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	node, ok := s.relayNodes[nodeID]
	if !ok {
		return OpsRelayNode{}, errNotFound
	}
	if node.Transport != transport {
		return OpsRelayNode{}, errNotFound
	}
	if req.Enabled != nil {
		if *req.Enabled {
			node.Status = "active"
		} else {
			node.Status = "disabled"
			node.Health = "down"
		}
	}
	if req.Healthy != nil && node.Status != "disabled" {
		if *req.Healthy {
			node.Health = "healthy"
		} else {
			node.Health = "down"
		}
	}
	node.UpdatedAt = time.Now().Unix()
	s.relayNodes[nodeID] = node
	return node, nil
}

func (s *Store) UpsertWireNode(node OpsRelayNode) (OpsRelayNode, error) {
	node.NodeID = strings.TrimSpace(node.NodeID)
	node.Name = strings.TrimSpace(node.Name)
	node.Region = defaultString(node.Region, "default")
	node.Transport = defaultString(node.Transport, "relay_udp")
	node.PublicAddr = strings.TrimSpace(node.PublicAddr)
	node.Status = defaultString(node.Status, "active")
	node.Health = defaultString(node.Health, "healthy")
	if node.NodeID == "" || node.Name == "" || node.PublicAddr == "" {
		return OpsRelayNode{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().Unix()
	if existing, ok := s.relayNodes[node.NodeID]; ok {
		node.CreatedAt = existing.CreatedAt
		node.UpdatedAt = now
	} else {
		node.CreatedAt = now
		node.UpdatedAt = now
	}
	s.relayNodes[node.NodeID] = node
	return node, nil
}

func (s *Store) DeleteWireNode(nodeID, transport string) error {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	node, ok := s.relayNodes[nodeID]
	if !ok || node.Transport != transport {
		return errNotFound
	}
	delete(s.relayNodes, nodeID)
	return nil
}

func (s *Store) WirePeerAuthz(peerID string) (map[string]any, error) {
	ctx, err := s.wirePeerLookup(peerID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"peerId":      peerID,
		"networkId":   ctx.networkDevice.NetworkID,
		"nodeId":      "node-" + ctx.device.DeviceID,
		"enabled":     ctx.networkDevice.Enabled && ctx.networkDevice.Status == "active" && ctx.device.Status == "active",
		"virtualIps":  []string{ctx.device.GlobalIP},
		"allowedIps":  []string{ctx.device.GlobalIP + "/32"},
		"quotaPolicy": "default",
	}, nil
}

func (s *Store) WirePeerRuntimeConfig(peerID string) (map[string]any, error) {
	ctx, err := s.wirePeerLookup(peerID)
	if err != nil {
		return nil, err
	}
	networkEnabled := ctx.networkDevice.Enabled && ctx.networkDevice.Status == "active" && ctx.runtime.NetworkEnabled
	return map[string]any{
		"peerId":                peerID,
		"networkId":             ctx.networkDevice.NetworkID,
		"nodeId":                "node-" + ctx.device.DeviceID,
		"virtualIps":            []string{ctx.device.GlobalIP},
		"allowedIps":            []string{ctx.device.GlobalIP + "/32"},
		"keepaliveIntervalSecs": 30,
		"networkEnabled":        networkEnabled,
		"preferredPath":         "direct_udp",
		"endpoints":             []wireEndpoint{},
	}, nil
}

func (s *Store) WireNetworkTopology(networkID string) (map[string]any, error) {
	networkID = strings.TrimSpace(networkID)
	if networkID == "" {
		return nil, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.networks[networkID]; !ok {
		return nil, errNotFound
	}
	peers := make([]wirePeerRecord, 0)
	for _, networkDevice := range s.networkDevices {
		if networkDevice.NetworkID != networkID || !networkDevice.Enabled || networkDevice.Status != "active" {
			continue
		}
		device, ok := s.devices[networkDevice.DeviceID]
		if !ok || device.Status != "active" {
			continue
		}
		peerEndpoints := s.deviceEndpointsForNetworkDeviceLocked(networkID, device.DeviceID, time.Now().Unix())
		wireEndpoints := make([]wireEndpoint, 0, len(peerEndpoints))
		for _, endpoint := range peerEndpoints {
			wireEndpoints = append(wireEndpoints, wireEndpoint{
				Kind:       endpoint.Type,
				Address:    endpoint.Address,
				Reachable:  true,
				ObservedAt: endpoint.UpdatedAt,
			})
		}
		peers = append(peers, wirePeerRecord{
			PeerID:                  wirePeerID(networkDevice.NetworkID, device.DeviceID),
			NetworkID:               networkDevice.NetworkID,
			NodeID:                  "node-" + device.DeviceID,
			PublicKey:               device.PublicKey,
			VirtualIPs:              []string{device.GlobalIP},
			AllowedIPs:              []string{device.GlobalIP + "/32"},
			SupportsLanDirect:       true,
			SupportsIpv6Direct:      true,
			SupportsDirectUdp:       true,
			SupportsRelayUdp:        true,
			SupportsDerpTcpTls443:   true,
			PreferIpv6:              false,
			PreferLan:               true,
			AllowEndpointRoaming:    true,
			AllowFastReselection:    true,
			AllowRelayTicketRenewal: true,
			KeepaliveIntervalSecs:   30,
			Endpoints:               wireEndpoints,
			ActivePath:              "direct_udp",
			RequireMtuRefresh:       false,
			EndpointChanged:         false,
			UpdatedAt:               networkDevice.UpdatedAt,
		})
	}
	return map[string]any{"networkId": networkID, "peers": peers}, nil
}

func (s *Store) wirePeerLookup(peerID string) (wirePeerContext, error) {
	peerID = strings.TrimSpace(peerID)
	if peerID == "" {
		return wirePeerContext{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	networkID, deviceID := parseWirePeerID(peerID)
	if deviceID == "" {
		return wirePeerContext{}, errBadRequest
	}
	var networkDevice NetworkDevice
	if networkID != "" {
		found, ok := s.networkDevices[networkID+"|"+deviceID]
		if !ok {
			return wirePeerContext{}, errNotFound
		}
		networkDevice = found
	} else {
		bestNetworkID := ""
		for _, candidate := range s.networkDevices {
			if candidate.DeviceID == deviceID && candidate.Enabled && candidate.Status == "active" {
				if bestNetworkID == "" || candidate.NetworkID < bestNetworkID {
					bestNetworkID = candidate.NetworkID
					networkDevice = candidate
				}
			}
		}
	}
	if networkDevice.DeviceID == "" {
		return wirePeerContext{}, errNotFound
	}
	device, ok := s.devices[deviceID]
	if !ok {
		return wirePeerContext{}, errNotFound
	}
	return wirePeerContext{networkDevice: networkDevice, device: device, runtime: s.runtimeStatuses[deviceID]}, nil
}
