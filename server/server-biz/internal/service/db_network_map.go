package service

import (
	"context"
	"strings"
	"time"

	"github.com/slan/server/server-biz/api/dto"
)

func (s *dbState) controlPlaneConfig() dto.ControlPlaneConfig {
	return dto.ControlPlaneConfig{
		WSURL:            s.wsURL(),
		HeartbeatSeconds: 15,
	}
}

func (s *dbState) relayConfig() dto.RelayConfig {
	return dto.RelayConfig{
		Region:      s.cfg.Relay.Region,
		UDPEndpoint: s.cfg.Relay.UDPEndpoint,
		TCPEndpoint: s.cfg.Relay.TCPEndpoint,
	}
}

func (s *dbState) derpMap() dto.DerpMap {
	return dto.DerpMap{
		ProbeIntervalSeconds: 5,
		Clusters: []dto.DerpCluster{
			{
				ClusterID:         "cluster-ap-east",
				RegionID:          "ap-east",
				RegionName:        "Asia Pacific East",
				RecommendedFanout: 3,
				Nodes: []dto.DerpNode{
					{NodeID: "tokyo", Host: "tokyo.derp.local", Port: 9000, Transport: "udp", Priority: 10},
					{NodeID: "singapore", Host: "singapore.derp.local", Port: 9000, Transport: "udp", Priority: 20},
					{NodeID: "hongkong", Host: "hongkong.derp.local", Port: 9000, Transport: "udp", Priority: 30},
				},
			},
		},
	}
}

func (s *dbState) buildNetworkMap(ctx context.Context, userID string, self dto.Node, networkID string) dto.NetworkMap {
	revision := uint64(1)
	if current, err := s.tokens.CurrentNetworkRevision(ctx, networkID); err == nil && current > 0 {
		revision = current
	}
	nodes, _ := s.pg.ListNodesByNetwork(ctx, networkID)
	var peers []dto.Peer
	for _, record := range nodes {
		if record.NodeID == self.NodeID {
			continue
		}
		device, err := s.pg.GetDeviceByID(ctx, record.DeviceID)
		if err != nil {
			continue
		}
		if device.Status != "online" {
			continue
		}
		peers = append(peers, dto.Peer{
			NodeID:        record.NodeID,
			DeviceID:      record.DeviceID,
			PublicKey:     record.NodePublicKey,
			Status:        device.Status,
			RelayAllowed:  true,
			VirtualIPs:    s.virtualIPsForDeviceInNetwork(ctx, record.DeviceID, networkID),
			Endpoints:     s.endpointsForNodeInNetwork(ctx, record.NodeID, networkID),
			AllowedRoutes: s.allowedRoutesForDeviceInNetwork(ctx, record.DeviceID, networkID),
		})
	}
	return dto.NetworkMap{
		SelfUserID:       userID,
		SelfDeviceID:     self.DeviceID,
		SelfNodeID:       self.NodeID,
		NetworkID:        networkID,
		Revision:         revision,
		HeartbeatSeconds: 15,
		STUNServers:      append([]string(nil), s.cfg.Bootstrap.STUNServers...),
		Peers:            peers,
		Routes:           s.routesForNetwork(ctx, networkID),
		RelayRegions: []dto.RelayRegion{
			{
				RegionID:   s.cfg.Relay.Region,
				RegionName: s.cfg.Relay.Region,
				Endpoints: []dto.RelayEndpoint{
					{EndpointID: "relay-udp", Transport: "udp", Address: s.cfg.Relay.UDPEndpoint},
				},
			},
		},
		DNS: dto.DNSConfig{Servers: []string{}, SearchDomains: []string{}},
		MTU: 1280,
	}
}

func (s *dbState) endpointsForNodeInNetwork(ctx context.Context, nodeID, networkID string) []dto.Endpoint {
	records, err := s.pg.ListNodeEndpoints(ctx, nodeID, networkID)
	endpoints := make([]dto.Endpoint, 0, len(records)+1)
	if err == nil {
		for _, record := range records {
			endpoints = append(endpoints, record.ToDTO())
		}
	}
	endpoints = append(endpoints, dto.Endpoint{
		Type:      "relay",
		Address:   s.cfg.Relay.UDPEndpoint,
		UpdatedAt: time.Now().Unix(),
	})
	return endpoints
}

func (s *dbState) virtualIPsForDeviceInNetwork(ctx context.Context, deviceID, networkID string) []string {
	attachments, err := s.pg.ListAttachmentsByDevice(ctx, deviceID)
	if err != nil {
		return nil
	}
	var out []string
	for _, attachment := range attachments {
		if attachment.NetworkID == networkID && attachment.VirtualIP != "" {
			out = append(out, attachment.VirtualIP)
		}
	}
	return out
}

func (s *dbState) allowedRoutesForDeviceInNetwork(ctx context.Context, deviceID, networkID string) []string {
	attachments, err := s.pg.ListAttachmentsByDevice(ctx, deviceID)
	if err != nil {
		return nil
	}
	var out []string
	for _, attachment := range attachments {
		if attachment.NetworkID != networkID {
			continue
		}
		subnet, err := s.pg.GetSubnetByID(ctx, attachment.SubnetID)
		if err == nil {
			out = append(out, subnet.CIDR)
		}
	}
	return out
}

func (s *dbState) routesForNetwork(ctx context.Context, networkID string) []dto.Route {
	subnets, err := s.pg.ListSubnetsByNetwork(ctx, networkID)
	if err != nil {
		return nil
	}
	members, err := s.pg.ListMembersByNetwork(ctx, networkID)
	if err != nil {
		return nil
	}
	var routes []dto.Route
	for _, subnet := range subnets {
		for _, member := range members {
			if _, err := s.pg.GetAttachmentBySubnetDevice(ctx, subnet.SubnetID, member.DeviceID); err != nil {
				continue
			}
			nodes, err := s.pg.ListNodesByDevice(ctx, member.DeviceID)
			if err != nil || len(nodes) == 0 {
				continue
			}
			routes = append(routes, dto.Route{CIDR: subnet.CIDR, ViaNodeID: nodes[0].NodeID})
			break
		}
	}
	return routes
}

func (s *dbState) wsURL() string {
	host := s.cfg.HTTP.Address
	if strings.HasPrefix(host, ":") {
		host = "localhost" + host
	}
	if !strings.Contains(host, ":") {
		host += ":80"
	}
	return "ws://" + host + s.cfg.WS.Path
}
