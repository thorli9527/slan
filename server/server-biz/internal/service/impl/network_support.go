package impl

import (
	"context"
	"fmt"
	"net/netip"
	"sort"
	"strings"
	"time"

	"github.com/slan/server/server-biz/api/dto"
	controlmsg "github.com/slan/server/server-biz/internal/controlmsg"
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

func (s *dbState) preferredOwnerIP(ctx context.Context, subnet dto.Subnet) (string, bool, error) {
	attachments, err := s.pg.ListAttachmentsBySubnet(ctx, subnet.SubnetID)
	if err != nil {
		return "", false, err
	}
	prefix, start, end, err := subnetRange(subnet.CIDR, subnet.GatewayIP, subnet.AllocationStartIP, subnet.AllocationEndIP)
	if err != nil {
		return "", false, err
	}
	candidate := networkBase(prefix) + 2
	if candidate < start || candidate > end || !prefix.Contains(uint32ToAddr(candidate)) {
		return "", false, nil
	}
	ip := uint32ToAddr(candidate).String()
	for _, attachment := range attachments {
		if attachment.VirtualIP == ip {
			return "", false, nil
		}
	}
	return ip, true, nil
}

func (s *dbState) reassignDefaultSubnetLeasePool(ctx context.Context, networkID string, subnet dto.Subnet) error {
	attachments, err := s.pg.ListAttachmentsBySubnet(ctx, subnet.SubnetID)
	if err != nil {
		return err
	}
	prefix, start, end, err := subnetRange(subnet.CIDR, subnet.GatewayIP, subnet.AllocationStartIP, subnet.AllocationEndIP)
	if err != nil {
		return err
	}
	if len(attachments) > int(end-start+1) {
		return fmt.Errorf("%w: subnet is too small for current members", ErrConflict)
	}

	sort.Slice(attachments, func(i, j int) bool {
		leftOwner := false
		if member, err := s.pg.GetMemberByNetworkDevice(ctx, attachments[i].NetworkID, attachments[i].DeviceID); err == nil {
			leftOwner = member.Role == "owner"
		}
		rightOwner := false
		if member, err := s.pg.GetMemberByNetworkDevice(ctx, attachments[j].NetworkID, attachments[j].DeviceID); err == nil {
			rightOwner = member.Role == "owner"
		}
		if leftOwner != rightOwner {
			return leftOwner
		}
		return attachments[i].AttachmentID < attachments[j].AttachmentID
	})
	for index, attachment := range attachments {
		candidate := start + uint32(index)
		if candidate > end || !prefix.Contains(uint32ToAddr(candidate)) {
			return fmt.Errorf("%w: subnet is exhausted", ErrConflict)
		}
		if err := s.pg.UpdateAttachmentVirtualIP(ctx, attachment.AttachmentID, uint32ToAddr(candidate).String()); err != nil {
			return err
		}
	}
	return s.pg.UpdateSubnetRange(ctx, subnet)
}

func (s *dbState) publishNetworkRestartRequired(networkID, cidr string) {
	if strings.TrimSpace(networkID) == "" || s.tokens == nil {
		return
	}

	ctx := context.Background()
	revision, err := s.tokens.NextNetworkRevision(ctx, networkID)
	if err != nil || revision == 0 {
		revision = 1
	}
	_ = s.tokens.PublishControlSyncEvent(ctx, controlmsg.ControlSyncEvent{
		Type:      "network_restart_required",
		NetworkID: networkID,
		Revision:  revision,
		Restart: &controlmsg.NetworkRestartRequired{
			NetworkID:         networkID,
			Revision:          revision,
			Reason:            "network configuration changed, tunnel restart required",
			DefaultSubnetCIDR: cidr,
		},
	})
}

func (s *dbState) publishDeviceIPReassigned(networkID, deviceID, attachmentID, virtualIP string) {
	if strings.TrimSpace(networkID) == "" || strings.TrimSpace(deviceID) == "" || s.tokens == nil {
		return
	}

	_ = s.tokens.PublishControlSyncEvent(context.Background(), controlmsg.ControlSyncEvent{
		Type:      "device_ip_reassigned",
		NetworkID: networkID,
		DeviceIP: &controlmsg.DeviceIPReassigned{
			NetworkID:    networkID,
			DeviceID:     deviceID,
			AttachmentID: attachmentID,
			VirtualIP:    virtualIP,
			Reason:       "attachment virtual ip updated",
		},
	})
}

func (s *dbState) publishActiveNetworkEnabled(userID, networkID, reason string) {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(networkID) == "" || s.tokens == nil {
		return
	}

	_ = s.tokens.PublishControlSyncEvent(context.Background(), controlmsg.ControlSyncEvent{
		Type:         "active_network_enabled",
		NetworkID:    networkID,
		TargetUserID: userID,
		ActiveNetwork: &controlmsg.ActiveNetworkEnabled{
			UserID:    userID,
			NetworkID: networkID,
			Reason:    reason,
		},
	})
}

func (s *dbState) publishPeerRemove(networkID, nodeID string) {
	if strings.TrimSpace(networkID) == "" || strings.TrimSpace(nodeID) == "" || s.tokens == nil {
		return
	}

	ctx := context.Background()
	revision, err := s.tokens.NextNetworkRevision(ctx, networkID)
	if err != nil || revision == 0 {
		revision = 1
	}
	_ = s.tokens.PublishControlSyncEvent(ctx, controlmsg.ControlSyncEvent{
		Type:         "peer_remove",
		NetworkID:    networkID,
		SourceNodeID: nodeID,
		Revision:     revision,
		PeerNodeID:   nodeID,
	})
}

func (s *dbState) controlPlaneConfig() dto.ControlPlaneConfig {
	return dto.ControlPlaneConfig{
		ControlURL:       s.controlURL(),
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
		DNS:              s.buildNetworkMapDNS(ctx, userID, networkID),
		Policy:           s.buildAccessPolicy(ctx, userID),
		MTU:              defaultTunnelMTU,
	}
}

func (s *dbState) buildAccessPolicy(ctx context.Context, userID string) dto.AccessPolicy {
	policy := dto.AccessPolicy{
		PlanCode:                "free",
		MaxActiveDevices:        fixedDeviceLimit(),
		RelayBandwidthLimitKbps: defaultRelayBandwidthLimitKbps,
		P2PUnlimited:            true,
		DNSAvailable:            true,
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
	if isDefault {
		ownerIP := uint32ToAddr(networkBase(prefix) + 2).String()
		ownerValue := networkBase(prefix) + 2
		gatewayValue := addrToUint32(mustParseAddr(gatewayIP))
		if gatewayValue == ownerValue || ownerValue < start || ownerValue > end {
			return dto.Subnet{}, fmt.Errorf(
				"%w: default subnet DHCP range must include owner ip %s",
				ErrInvalidArgument,
				ownerIP,
			)
		}
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
	startRaw := strings.TrimSpace(startIP)
	endRaw := strings.TrimSpace(endIP)
	if (startRaw == "") != (endRaw == "") {
		return netip.Prefix{}, 0, 0, fmt.Errorf(
			"%w: allocationStartIp and allocationEndIp must be provided together",
			ErrInvalidArgument,
		)
	}

	if gatewayIP != "" {
		gatewayAddr, err := parseIPv4InPrefix(gatewayIP, prefix, "gatewayIp")
		if err != nil {
			return netip.Prefix{}, 0, 0, err
		}
		if gatewayAddr == base || gatewayAddr == broadcast {
			return netip.Prefix{}, 0, 0, fmt.Errorf("%w: gatewayIp cannot be network or broadcast address", ErrInvalidArgument)
		}
		gateway = gatewayAddr
		if start <= gateway {
			start = gateway + 1
		}
	}
	if startRaw != "" {
		addr, err := parseIPv4InPrefix(startIP, prefix, "allocationStartIp")
		if err != nil {
			return netip.Prefix{}, 0, 0, err
		}
		if addr == base || addr == broadcast {
			return netip.Prefix{}, 0, 0, fmt.Errorf("%w: allocationStartIp cannot be network or broadcast address", ErrInvalidArgument)
		}
		start = addr
	}
	if endRaw != "" {
		addr, err := parseIPv4InPrefix(endIP, prefix, "allocationEndIp")
		if err != nil {
			return netip.Prefix{}, 0, 0, err
		}
		if addr == base || addr == broadcast {
			return netip.Prefix{}, 0, 0, fmt.Errorf("%w: allocationEndIp cannot be network or broadcast address", ErrInvalidArgument)
		}
		end = addr
	}
	if start <= gateway || end <= start {
		return netip.Prefix{}, 0, 0, fmt.Errorf(
			"%w: invalid allocation range, ensure gateway < allocationStartIp < allocationEndIp",
			ErrInvalidArgument,
		)
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

func mustParseAddr(raw string) netip.Addr {
	addr, _ := netip.ParseAddr(strings.TrimSpace(raw))
	return addr
}

type subnetTemplate struct {
	name   string
	remark string
}

var defaultSubnetTemplates = []subnetTemplate{
	{name: "总网络", remark: "默认主子网，适合未分组设备和通用接入"},
	{name: "开发部", remark: "开发、测试、运维相关设备"},
	{name: "营销部", remark: "销售、市场、外勤相关设备"},
	{name: "人事部", remark: "人事、行政、财务相关设备"},
}

func createNetworkCIDR(cidr string) string {
	if strings.TrimSpace(cidr) != "" {
		return strings.TrimSpace(cidr)
	}
	return "10.0.0.0/22"
}

func defaultSubnetsForNetwork(networkID, cidr string, newID func() string) ([]dto.Subnet, error) {
	cidrs, err := subdivideSubnetCIDRs(cidr, len(defaultSubnetTemplates))
	if err != nil {
		return nil, err
	}
	templates := defaultSubnetTemplates
	if len(cidrs) == 1 {
		templates = templates[:1]
	}
	subnets := make([]dto.Subnet, 0, len(cidrs))
	for index, subnetCIDR := range cidrs {
		template := templates[index]
		subnet, err := newSubnet(networkID, newID(), template.name, subnetCIDR, "", "", "", index == 0)
		if err != nil {
			return nil, err
		}
		subnet.Remark = template.remark
		subnets = append(subnets, subnet)
	}
	return subnets, nil
}

func subdivideSubnetCIDRs(cidr string, count int) ([]string, error) {
	prefix, _, _, err := subnetRange(cidr, "", "", "")
	if err != nil {
		return nil, err
	}
	if count <= 1 {
		return []string{prefix.String()}, nil
	}
	extraBits := 0
	for slots := 1; slots < count; slots <<= 1 {
		extraBits++
	}
	childBits := prefix.Bits() + extraBits
	if childBits > 29 {
		return []string{prefix.String()}, nil
	}
	base := networkBase(prefix)
	step := uint32(1) << uint(32-childBits)
	out := make([]string, 0, count)
	for i := 0; i < count; i++ {
		child := netip.PrefixFrom(uint32ToAddr(base+uint32(i)*step), childBits).Masked()
		out = append(out, child.String())
	}
	return out, nil
}
