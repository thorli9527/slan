package biz

import (
	"fmt"
	"strings"
)

func (s *Store) ensureDefaultNetworkForUserLocked(userID string, now int64) Network {
	network, _, _ := s.ensureDefaultNetworkResourcesForUserLocked(userID, now)
	return network
}

func (s *Store) ensureDefaultNetworkResourcesForUserLocked(userID string, now int64) (Network, SecurityGroup, NetworkDNSZone) {
	id := "default-" + userID
	if network, ok := s.networks[id]; ok {
		var group SecurityGroup
		for _, item := range s.securityGroups {
			if item.NetworkID == network.NetworkID {
				group = item
				break
			}
		}
		var zone NetworkDNSZone
		for _, item := range s.dnsZones {
			if item.NetworkID == network.NetworkID {
				zone = item
				break
			}
		}
		return network, group, zone
	}
	network := Network{NetworkID: id, OwnerUserID: userID, Name: "默认网络", Code: "default", TemplateKey: "default", Status: "enabled", Default: true, CreatedAt: now, UpdatedAt: now}
	s.networks[id] = network
	group := s.addSecurityGroupLocked(id, "默认安全组", "默认网络安全组", "deny", now)
	zone := s.addDNSZoneLocked(id, networkZoneName(network), false, now)
	return network, group, zone
}

func (s *Store) addNetworkDeviceLocked(networkID, deviceID, ownerUserID, alias string, enabled bool, now int64) NetworkDevice {
	status := "active"
	if !enabled {
		status = "disabled"
	}
	networkDevice := NetworkDevice{NetworkDeviceID: fmt.Sprintf("network-device-%s-%s", networkID, deviceID), NetworkID: networkID, DeviceID: deviceID, OwnerUserID: ownerUserID, Alias: strings.TrimSpace(alias), Enabled: enabled, Status: status, CreatedAt: now, UpdatedAt: now}
	s.networkDevices[networkID+"|"+deviceID] = networkDevice
	return networkDevice
}

func (s *Store) addDNSZoneLocked(networkID, zoneName string, exposeGlobal bool, now int64) NetworkDNSZone {
	zoneName = strings.TrimSpace(zoneName)
	if zoneName == "" {
		zoneName = "default.lan"
	}
	zone := NetworkDNSZone{ZoneID: fmt.Sprintf("zone-%06d", s.nextZoneSeq), NetworkID: networkID, ZoneName: zoneName, ExposeGlobal: exposeGlobal, Status: "active", CreatedAt: now}
	s.nextZoneSeq++
	s.dnsZones[zone.ZoneID] = zone
	return zone
}

func (s *Store) addSecurityGroupLocked(networkID, name, description, defaultPolicy string, now int64) SecurityGroup {
	group := SecurityGroup{SecurityGroupID: fmt.Sprintf("sg-%06d", s.nextSecuritySeq), NetworkID: networkID, Name: defaultString(name, "默认安全组"), Description: description, DefaultPolicy: defaultString(defaultPolicy, "deny"), Status: "active", CreatedAt: now}
	s.nextSecuritySeq++
	s.securityGroups[group.SecurityGroupID] = group
	return group
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
