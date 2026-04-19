package impl

import (
	"context"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
)

func (s *dbState) allocateIP(ctx context.Context, subnet dto.Subnet) (string, error) {
	attachments, err := s.pg.ListAttachmentsBySubnet(ctx, subnet.SubnetID)
	if err != nil {
		return "", err
	}
	prefix, start, end, err := subnetRange(subnet.CIDR, subnet.GatewayIP, subnet.AllocationStartIP, subnet.AllocationEndIP)
	if err != nil {
		return "", err
	}
	used := make(map[string]struct{}, len(attachments))
	for _, attachment := range attachments {
		used[attachment.VirtualIP] = struct{}{}
	}
	for candidate := start; candidate <= end; candidate++ {
		ip := uint32ToAddr(candidate).String()
		if _, exists := used[ip]; exists {
			continue
		}
		if !prefix.Contains(uint32ToAddr(candidate)) {
			continue
		}
		return ip, nil
	}
	return "", fmt.Errorf("%w: subnet is exhausted", ErrConflict)
}

func (s *dbState) controlPlaneConfig() dto.ControlPlaneConfig {
	return dto.ControlPlaneConfig{
		WSURL:            s.wsURL(),
		HeartbeatSeconds: defaultControlHeartbeatSeconds,
	}
}

func (s *dbState) buildNetworkMap(ctx context.Context, userID string, self dto.Node, networkID string) dto.NetworkMap {
	return dto.NetworkMap{
		SelfUserID:       userID,
		SelfDeviceID:     self.DeviceID,
		SelfNodeID:       self.NodeID,
		NetworkID:        networkID,
		Revision:         s.currentNetworkRevision(ctx, networkID),
		HeartbeatSeconds: defaultControlHeartbeatSeconds,
		STUNServers:      append([]string(nil), s.cfg.Bootstrap.STUNServers...),
		Peers:            s.buildNetworkMapPeers(ctx, self, networkID),
		Routes:           s.routesForNetwork(ctx, networkID),
		RelayRegions:     s.relayRegions(),
		DNS:              buildNetworkMapDNS(),
		MTU:              defaultTunnelMTU,
	}
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

func (s *dbState) currentNetworkRevision(ctx context.Context, networkID string) uint64 {
	revision := uint64(1)
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
	if err != nil || device.Status != "online" {
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

func buildNetworkMapDNS() dto.DNSConfig {
	return dto.DNSConfig{
		Servers:       []string{},
		SearchDomains: []string{},
	}
}

func newSubnet(networkID, subnetID, name, cidr, gatewayIP, startIP, endIP string, isDefault bool) (dto.Subnet, error) {
	prefix, start, end, err := subnetRange(cidr, gatewayIP, startIP, endIP)
	if err != nil {
		return dto.Subnet{}, err
	}
	if gatewayIP == "" {
		gatewayIP = uint32ToAddr(networkBase(prefix) + 1).String()
	}
	if startIP == "" {
		startIP = uint32ToAddr(start).String()
	}
	if endIP == "" {
		endIP = uint32ToAddr(end).String()
	}

	return dto.Subnet{
		SubnetID:          subnetID,
		NetworkID:         networkID,
		Name:              strings.TrimSpace(name),
		CIDR:              prefix.String(),
		GatewayIP:         gatewayIP,
		AllocationStartIP: startIP,
		AllocationEndIP:   endIP,
		IsDefault:         isDefault,
		Status:            "active",
	}, nil
}

func subnetRange(cidr, gatewayIP, startIP, endIP string) (netip.Prefix, uint32, uint32, error) {
	prefix, err := netip.ParsePrefix(strings.TrimSpace(cidr))
	if err != nil {
		return netip.Prefix{}, 0, 0, fmt.Errorf("%w: invalid cidr", ErrInvalidArgument)
	}
	if !prefix.Addr().Is4() {
		return netip.Prefix{}, 0, 0, fmt.Errorf("%w: only IPv4 is supported", ErrInvalidArgument)
	}

	base := networkBase(prefix)
	broadcast := base | ^mask(prefix)
	if broadcast-base < 3 {
		return netip.Prefix{}, 0, 0, fmt.Errorf("%w: subnet too small", ErrInvalidArgument)
	}

	gateway := base + 1
	start := base + 2
	end := broadcast - 1

	if gatewayIP != "" {
		gatewayAddr, err := parseIPv4InPrefix(gatewayIP, prefix, "gatewayIp")
		if err != nil {
			return netip.Prefix{}, 0, 0, err
		}
		gateway = gatewayAddr
		if start <= gateway {
			start = gateway + 1
		}
	}
	if startIP != "" {
		addr, err := parseIPv4InPrefix(startIP, prefix, "allocationStartIp")
		if err != nil {
			return netip.Prefix{}, 0, 0, err
		}
		start = addr
	}
	if endIP != "" {
		addr, err := parseIPv4InPrefix(endIP, prefix, "allocationEndIp")
		if err != nil {
			return netip.Prefix{}, 0, 0, err
		}
		end = addr
	}
	if start <= gateway || end <= start {
		return netip.Prefix{}, 0, 0, fmt.Errorf("%w: invalid allocation range", ErrInvalidArgument)
	}
	return prefix, start, end, nil
}

func parseIPv4InPrefix(raw string, prefix netip.Prefix, field string) (uint32, error) {
	addr, err := netip.ParseAddr(strings.TrimSpace(raw))
	if err != nil || !addr.Is4() || !prefix.Contains(addr) {
		return 0, fmt.Errorf("%w: invalid %s", ErrInvalidArgument, field)
	}
	return addrToUint32(addr), nil
}

func networkBase(prefix netip.Prefix) uint32 {
	return addrToUint32(prefix.Masked().Addr())
}

func mask(prefix netip.Prefix) uint32 {
	bits := prefix.Bits()
	if bits == 0 {
		return 0
	}
	return ^uint32(0) << (32 - bits)
}

func addrToUint32(addr netip.Addr) uint32 {
	bytes := addr.As4()
	return uint32(bytes[0])<<24 | uint32(bytes[1])<<16 | uint32(bytes[2])<<8 | uint32(bytes[3])
}

func uint32ToAddr(value uint32) netip.Addr {
	return netip.AddrFrom4([4]byte{
		byte(value >> 24),
		byte(value >> 16),
		byte(value >> 8),
		byte(value),
	})
}
