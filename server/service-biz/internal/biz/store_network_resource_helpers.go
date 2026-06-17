package biz

import "strings"

func (s *Store) ensureDefaultNetworkForUserLocked(userID string, now int64) Network {
	network, _ := s.ensureDefaultNetworkResourcesForUserLocked(userID, now)
	return network
}

func (s *Store) ensureDefaultNetworkResourcesForUserLocked(userID string, now int64) (Network, SecurityGroup) {
	for _, network := range s.networks {
		if network.OwnerUserID == userID && network.Default {
			group := s.defaultSecurityGroupLocked(network.NetworkID)
			return network, group
		}
	}
	legacyID := "default-" + userID
	if network, ok := s.networks[legacyID]; ok {
		var group SecurityGroup
		for _, item := range s.securityGroups {
			if item.NetworkID == network.NetworkID {
				group = item
				break
			}
		}
		return network, group
	}
	id := newCompactUUID()
	network := Network{NetworkID: id, OwnerUserID: userID, Name: "默认网络", Code: "default", TemplateKey: "default", IntraGroupPolicy: "allow", Default: true, CreatedAt: now, UpdatedAt: now}
	s.networks[id] = network
	group := s.addSecurityGroupLocked(id, "", "默认网络安全组", now)
	return network, group
}

func (s *Store) addNetworkDeviceLocked(networkID, deviceID, ownerUserID, alias string, enabled bool, now int64) NetworkDevice {
	status := "active"
	if !enabled {
		status = "disabled"
	}
	networkDevice := NetworkDevice{NetworkDeviceID: newCompactUUID(), NetworkID: networkID, DeviceID: deviceID, OwnerUserID: ownerUserID, Alias: strings.TrimSpace(alias), Enabled: enabled, Status: status, CreatedAt: now, UpdatedAt: now}
	s.networkDevices[networkID+"|"+deviceID] = networkDevice
	return networkDevice
}

func (s *Store) addDNSZoneLocked(networkID, zoneName string, exposeGlobal bool, now int64) NetworkDNSZone {
	zoneName = strings.TrimSpace(zoneName)
	if zoneName == "" {
		zoneName = "default.lan"
	}
	zone := NetworkDNSZone{ZoneID: newCompactUUID(), NetworkID: networkID, ZoneName: zoneName, ExposeGlobal: exposeGlobal, Status: "active", CreatedAt: now}
	s.nextZoneSeq++
	s.dnsZones[zone.ZoneID] = zone
	return zone
}

func (s *Store) addSecurityGroupLocked(networkID, name, description string, now int64) SecurityGroup {
	group := SecurityGroup{
		SecurityGroupID: newCompactUUID(),
		NetworkID:       networkID,
		Name:            strings.TrimSpace(name),
		Description:     strings.TrimSpace(description),
		Status:          "active",
		CreatedAt:       now,
	}
	s.nextSecuritySeq++
	s.securityGroups[group.SecurityGroupID] = group
	return group
}

func defaultSecurityGroupPolicy(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "deny", "isolated":
		return "deny"
	default:
		return "allow"
	}
}

func (s *Store) listDNSZonesLocked(networkID string) []NetworkDNSZone {
	out := make([]NetworkDNSZone, 0)
	for _, zone := range s.dnsZones {
		if zone.NetworkID == networkID {
			out = append(out, zone)
		}
	}
	return out
}

func (s *Store) listDNSRecordsLocked(networkID string) []NetworkDNSRecord {
	out := make([]NetworkDNSRecord, 0)
	for _, record := range s.dnsRecords {
		if record.NetworkID == networkID {
			out = append(out, record)
		}
	}
	return out
}

func networkZoneName(network Network) string {
	if network.Code == "default" {
		return "default.lan"
	}
	return sanitizeDNSLabel(network.Code) + ".internal"
}
