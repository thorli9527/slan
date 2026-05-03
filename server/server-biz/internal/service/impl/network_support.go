package impl

import (
	"context"
	"strings"
	"time"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
)

func (s *dbState) controlPlaneConfig() dto.ControlPlaneConfig {
	return dto.ControlPlaneConfig{
		ControlURL:       s.controlURL(),
		HeartbeatSeconds: defaultControlHeartbeatSeconds,
	}
}

func (s *dbState) buildNetworkMap(ctx context.Context, userID string, self dto.Node, networkID string) dto.NetworkMap {
	peers := s.buildNetworkMapPeers(ctx, self, networkID)
	relayCountries := s.relayCandidateCountryCodes(ctx, self, peers)
	return dto.NetworkMap{
		SelfUserID:              userID,
		SelfDeviceID:            self.DeviceID,
		SelfNodeID:              self.NodeID,
		NetworkID:               networkID,
		Revision:                s.currentNetworkRevision(ctx, networkID),
		HeartbeatSeconds:        defaultControlHeartbeatSeconds,
		STUNServers:             append([]string(nil), s.cfg.Bootstrap.STUNServers...),
		Peers:                   peers,
		Routes:                  s.routesForNetwork(ctx, networkID),
		RelayRegions:            s.relayRegionsForCountries(relayCountries),
		RelayCandidateCountries: append([]string(nil), relayCountries...),
		DNS:                     s.buildNetworkMapDNS(ctx, userID, networkID),
		Policy:                  s.buildAccessPolicy(ctx, userID),
		MTU:                     defaultTunnelMTU,
	}
}

func (s *dbState) relayCandidateCountryCodes(ctx context.Context, self dto.Node, peers []dto.Peer) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, 4)
	appendCountry := func(countryCode string) {
		value := normalizeRelayCountryCode(countryCode)
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	if device, err := s.pg.GetDeviceByID(ctx, self.DeviceID); err == nil {
		appendCountry(device.CountryCode)
	}
	for _, peer := range peers {
		if device, err := s.pg.GetDeviceByID(ctx, peer.DeviceID); err == nil {
			appendCountry(device.CountryCode)
		}
	}
	for _, countryCode := range s.defaultRelayCountryCodes() {
		appendCountry(countryCode)
	}
	return out
}

func (s *dbState) buildAccessPolicy(ctx context.Context, userID string) dto.AccessPolicy {
	plan := s.planStatusForUser(ctx, userID)
	policy := dto.AccessPolicy{
		PlanCode:                plan.PlanName,
		MaxActiveDevices:        plan.MaxActiveDevices,
		RelayBandwidthLimitKbps: plan.RelayBandwidthLimitKbps,
		RelayIngressKbps:        plan.RelayIngressKbps,
		RelayEgressKbps:         plan.RelayEgressKbps,
		UDPIngressKbps:          plan.UDPIngressKbps,
		UDPEgressKbps:           plan.UDPEgressKbps,
		P2PUnlimited:            plan.P2PUnlimited,
		DNSAvailable:            plan.DNSAvailable,
	}
	return policy
}

func (s *dbState) endpointsForNodeInNetwork(ctx context.Context, nodeID, networkID string) []dto.Endpoint {
	now := time.Now()
	cutoff := endpointCutoffUnix(now)
	_ = s.pg.DeleteNodeEndpointsBefore(ctx, nodeID, networkID, cutoff)

	records, err := s.pg.ListNodeEndpoints(ctx, nodeID, networkID)
	endpoints := make([]dto.Endpoint, 0, len(records)+1)
	if err == nil {
		for _, record := range records {
			if record.UpdatedAt < cutoff {
				continue
			}
			endpoints = append(endpoints, record.ToDTO())
		}
	}
	endpoints = append(endpoints, s.relayFallbackEndpoints()...)
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
			if member.Status != "active" {
				continue
			}
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

func (s *dbState) controlURL() string {
	if strings.TrimSpace(s.cfg.MQTT.PublicBrokerURL) != "" {
		return strings.TrimSpace(s.cfg.MQTT.PublicBrokerURL)
	}
	return strings.TrimSpace(s.cfg.MQTT.BrokerURL)
}

func (s *dbState) currentNetworkRevision(ctx context.Context, networkID string) uint64 {
	revision := uint64(1)
	if s.tokens == nil {
		return revision
	}
	if current, err := s.tokens.CurrentNetworkRevision(ctx, networkID); err == nil && current > 0 {
		revision = current
	}
	return revision
}

func (s *dbState) buildNetworkMapPeers(ctx context.Context, self dto.Node, networkID string) []dto.Peer {
	nodes, err := s.pg.ListNodesByNetwork(ctx, networkID)
	if err != nil {
		return nil
	}

	now := time.Now()
	peers := make([]dto.Peer, 0, len(nodes))
	for _, record := range nodes {
		peer, ok := s.networkPeerDTO(ctx, self, networkID, record, now)
		if !ok {
			continue
		}
		peers = append(peers, peer)
	}
	return peers
}

func (s *dbState) networkPeerDTO(ctx context.Context, self dto.Node, networkID string, record repo.Node, now time.Time) (dto.Peer, bool) {
	if record.NodeID == self.NodeID {
		return dto.Peer{}, false
	}

	device, err := s.pg.GetDeviceByID(ctx, record.DeviceID)
	if err != nil {
		return dto.Peer{}, false
	}
	if _, err := s.requireActiveNetworkAttachment(ctx, networkID, record.DeviceID, ErrForbidden, "peer node device"); err != nil {
		return dto.Peer{}, false
	}
	if !s.hasFreshControlSession(ctx, record.NodeID, networkID, now) {
		return dto.Peer{}, false
	}

	return dto.Peer{
		NodeID:        record.NodeID,
		DeviceID:      record.DeviceID,
		PublicKey:     record.NodePublicKey,
		Status:        device.Status,
		RelayAllowed:  true,
		VirtualIPs:    s.virtualIPsForDeviceInNetwork(ctx, record.DeviceID, networkID),
		Endpoints:     s.endpointsForNodeInNetwork(ctx, record.NodeID, networkID),
		AllowedRoutes: s.allowedRoutesForDeviceInNetwork(ctx, record.DeviceID, networkID),
	}, true
}

func (s *dbState) buildNetworkMapDNS(ctx context.Context, userID, networkID string) dto.DNSConfig {
	record, err := s.pg.GetNetworkByID(ctx, networkID)
	if err != nil {
		return dto.DNSConfig{
			Servers:       []string{},
			SearchDomains: []string{},
		}
	}
	return record.DNSConfig()
}

func visibleJoinKey(network repo.Network, userID string) string {
	if network.OwnerUserID != userID {
		return ""
	}
	return strings.TrimSpace(network.JoinKey)
}
