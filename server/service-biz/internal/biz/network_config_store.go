package biz

import (
	"context"
	"log"
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
