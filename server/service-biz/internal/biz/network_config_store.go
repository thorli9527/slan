package biz

import (
	"context"
	"log"
	"net"
	"sort"
	"strings"
	"time"
)

func (s *Store) GlobalDNS() []GlobalDNSRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		log.Printf("service-biz refresh postgres global dns failed: %v", err)
	}
	out := make([]GlobalDNSRecord, 0, len(s.devices))
	for _, device := range s.devices {
		out = append(out, GlobalDNSRecord{Name: device.GlobalName, DeviceID: device.DeviceID, Value: device.GlobalIP})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (s *Store) NetworkConfig(networkID, deviceID string) (NetworkConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		return NetworkConfig{}, err
	}
	return s.networkConfigLocked(networkID, deviceID)
}

func (s *Store) NetworkConfigsForDevice(deviceID string) ([]NetworkConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		return nil, err
	}
	return s.networkConfigsForDeviceLocked(deviceID)
}

func (s *Store) networkConfigsForDeviceLocked(deviceID string) ([]NetworkConfig, error) {
	device, ok := s.devices[deviceID]
	if !ok {
		return nil, errNotFound
	}
	networkIDs := make([]string, 0)
	for _, membership := range s.networkDevices {
		if membership.DeviceID != deviceID || !membership.Enabled || membership.Status != "active" {
			continue
		}
		network, ok := s.networks[membership.NetworkID]
		if !ok || network.Status != "enabled" {
			continue
		}
		networkIDs = append(networkIDs, membership.NetworkID)
	}
	if len(networkIDs) == 0 && strings.TrimSpace(device.OwnerID) != "" {
		defaultNetwork := s.ensureDefaultNetworkForUserLocked(device.OwnerID, time.Now().Unix())
		membership := s.addNetworkDeviceLocked(defaultNetwork.NetworkID, deviceID, device.OwnerID, device.Alias, true, time.Now().Unix())
		if membership.Status == "active" && membership.Enabled {
			networkIDs = append(networkIDs, defaultNetwork.NetworkID)
		}
	}
	sort.Strings(networkIDs)
	out := make([]NetworkConfig, 0, len(networkIDs))
	for _, networkID := range networkIDs {
		config, err := s.networkConfigLocked(networkID, deviceID)
		if err != nil {
			return nil, err
		}
		out = append(out, config)
	}
	return out, nil
}

func (s *Store) networkConfigLocked(networkID, deviceID string) (NetworkConfig, error) {
	now := time.Now().Unix()
	device, ok := s.devices[deviceID]
	if !ok {
		return NetworkConfig{}, errNotFound
	}
	network, ok := s.networks[networkID]
	if !ok {
		return NetworkConfig{}, errNotFound
	}
	membership, ok := s.networkDevices[networkID+"|"+deviceID]
	if !ok || !membership.Enabled || membership.Status != "active" {
		if network.OwnerUserID != "" && network.OwnerUserID == device.OwnerID {
			membership = s.addNetworkDeviceLocked(networkID, deviceID, device.OwnerID, device.Alias, true, now)
		}
		if !membership.Enabled || membership.Status != "active" {
			return NetworkConfig{}, errNotFound
		}
	}
	s.ensureUniqueGlobalIPsLocked(now)
	device = s.devices[deviceID]
	peers := make([]Device, 0)
	for _, membership := range s.networkDevices {
		if membership.NetworkID == networkID && membership.Enabled && membership.Status == "active" && membership.DeviceID != deviceID {
			peers = append(peers, s.devices[membership.DeviceID])
		}
	}
	groups, rules := s.activeSecurityPolicyLocked(networkID)
	peers = s.securityPolicyFilterPeersLocked(networkID, device, peers, groups, rules)
	relayCandidates := s.activeRelayCandidatesLocked()
	globalIP := hostIP(device.GlobalIP)
	subnet, _ := s.globalIPSubnetLocked(globalIP)
	return NetworkConfig{
		NetworkID:       networkID,
		NetworkName:     network.Name,
		NetworkCode:     network.Code,
		ConfigVersion:   s.currentNetworkConfigVersionLocked(networkID),
		DeviceID:        deviceID,
		GlobalIP:        globalIP,
		PrefixLen:       subnet.PrefixLength,
		GlobalCIDR:      ipamGlobalCIDR,
		SubnetID:        subnet.SubnetID,
		SubnetCIDR:      subnet.CIDRBlock,
		SubnetPrefixLen: subnet.PrefixLength,
		GlobalName:      device.GlobalName,
		Peers:           s.devicesWithSubnetLocked(networkID, peers),
		SecurityGroups:  groups,
		Rules:           rules,
		DNSZones:        s.listDNSZonesLocked(networkID),
		DNSRecords:      s.listDNSRecordsLocked(networkID),
		RelayCandidates: relayCandidates,
	}, nil
}

func (s *Store) activeSecurityPolicyLocked(networkID string) ([]SecurityGroup, []SecurityGroupRule) {
	groups := make([]SecurityGroup, 0)
	groupIDs := make(map[string]bool)
	for _, group := range s.securityGroups {
		if group.NetworkID == networkID && group.Status == "active" {
			groups = append(groups, group)
			groupIDs[group.SecurityGroupID] = true
		}
	}
	rules := make([]SecurityGroupRule, 0)
	for _, rule := range s.securityGroupRules {
		if rule.Enabled && groupIDs[rule.SecurityGroupID] {
			rules = append(rules, rule)
		}
	}
	sort.Slice(rules, func(i, j int) bool {
		if rules[i].Priority == rules[j].Priority {
			return rules[i].RuleID < rules[j].RuleID
		}
		return rules[i].Priority < rules[j].Priority
	})
	return groups, rules
}

func (s *Store) securityPolicyFilterPeersLocked(networkID string, local Device, peers []Device, groups []SecurityGroup, rules []SecurityGroupRule) []Device {
	if len(peers) == 0 {
		return peers
	}
	out := make([]Device, 0, len(peers))
	for _, peer := range peers {
		if s.securityPolicyAllowsPeerLocked(networkID, local, peer, groups, rules) {
			out = append(out, peer)
		}
	}
	return out
}

func (s *Store) securityPolicyAllowsPeerLocked(networkID string, local, peer Device, groups []SecurityGroup, rules []SecurityGroupRule) bool {
	if len(rules) == 0 {
		return true
	}
	for _, rule := range rules {
		if !securityRuleMatchesPeerAccess(networkID, rule, local, peer) {
			continue
		}
		return strings.EqualFold(strings.TrimSpace(rule.Action), "allow")
	}
	for _, group := range groups {
		if strings.EqualFold(strings.TrimSpace(group.DefaultPolicy), "allow") {
			return true
		}
	}
	return false
}

func (s *Store) networkPeerAccessAllowedLocked(networkID, localDeviceID, peerDeviceID string) bool {
	local, ok := s.devices[localDeviceID]
	if !ok {
		return false
	}
	peer, ok := s.devices[peerDeviceID]
	if !ok {
		return false
	}
	localMembership, ok := s.networkDevices[networkID+"|"+localDeviceID]
	if !ok || !localMembership.Enabled || localMembership.Status != "active" {
		return false
	}
	peerMembership, ok := s.networkDevices[networkID+"|"+peerDeviceID]
	if !ok || !peerMembership.Enabled || peerMembership.Status != "active" {
		return false
	}
	groups, rules := s.activeSecurityPolicyLocked(networkID)
	return s.securityPolicyAllowsPeerLocked(networkID, local, peer, groups, rules)
}

func securityRuleMatchesPeerAccess(networkID string, rule SecurityGroupRule, local, peer Device) bool {
	switch strings.ToLower(strings.TrimSpace(rule.Direction)) {
	case "ingress", "inbound", "in":
		return securityRuleSubjectMatches(networkID, rule, peer)
	case "egress", "outbound", "out":
		return securityRuleSubjectMatches(networkID, rule, peer)
	default:
		return securityRuleSubjectMatches(networkID, rule, local) || securityRuleSubjectMatches(networkID, rule, peer)
	}
}

func securityRuleSubjectMatches(networkID string, rule SecurityGroupRule, target Device) bool {
	peerType := strings.ToLower(strings.TrimSpace(rule.PeerType))
	peerValue := strings.TrimSpace(rule.PeerValue)
	switch peerType {
	case "", "all", "any":
		return peerValue == "" || strings.EqualFold(peerValue, "all") || peerValue == "*"
	case "network", "workspace":
		return peerValue == "" || strings.EqualFold(peerValue, "self") || strings.EqualFold(peerValue, "all") || peerValue == networkID
	case "device":
		return peerValue == target.DeviceID
	case "cidr", "ip":
		return securityRuleCIDRMatches(peerValue, target.GlobalIP)
	case "domain":
		return strings.EqualFold(peerValue, target.GlobalName) || strings.EqualFold(peerValue, target.Name) || strings.EqualFold(peerValue, target.Alias)
	default:
		return false
	}
}

func securityRuleCIDRMatches(cidr, ipValue string) bool {
	ipValue = hostIP(ipValue)
	if cidr == "" || strings.EqualFold(cidr, "all") || cidr == "*" {
		return true
	}
	ip := net.ParseIP(ipValue)
	if ip == nil {
		return false
	}
	if _, network, err := net.ParseCIDR(cidr); err == nil {
		return network.Contains(ip)
	}
	exact := net.ParseIP(hostIP(cidr))
	return exact != nil && exact.Equal(ip)
}

func (s *Store) globalIPSubnetLocked(ip string) (IPAMSubnet, bool) {
	ip = hostIP(ip)
	if ip == "" {
		return IPAMSubnet{}, false
	}
	address, ok := s.globalIPs[ip]
	if !ok {
		return IPAMSubnet{}, false
	}
	subnet, ok := s.ipamSubnets[address.SubnetID]
	return subnet, ok
}

func (s *Store) devicesWithSubnetLocked(networkID string, devices []Device) []Device {
	out := make([]Device, 0, len(devices))
	for _, device := range devices {
		withSubnet := s.deviceWithSubnetLocked(device)
		withSubnet.Endpoints = s.deviceEndpointsForNetworkDeviceLocked(networkID, device.DeviceID, time.Now().Unix())
		withSubnet.RelayAllowed = true
		out = append(out, withSubnet)
	}
	return out
}

func (s *Store) deviceEndpointsForNetworkDeviceLocked(networkID, deviceID string, now int64) []DeviceEndpoint {
	const endpointTTLSeconds = 5 * 60
	out := make([]DeviceEndpoint, 0)
	for _, endpoint := range s.deviceEndpoints[networkID+"|"+deviceID] {
		if endpoint.UpdatedAt > 0 && now-endpoint.UpdatedAt > endpointTTLSeconds {
			continue
		}
		out = append(out, endpoint)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Type == out[j].Type {
			return out[i].Address < out[j].Address
		}
		return out[i].Type < out[j].Type
	})
	return out
}

func hostIP(value string) string {
	value = strings.TrimSpace(value)
	if ip, _, ok := strings.Cut(value, "/"); ok {
		return strings.TrimSpace(ip)
	}
	return value
}
