package biz

import (
	"fmt"
	"net/netip"
	"sort"
	"strings"
)

func (s *Store) allocateGlobalIPLocked(deviceID string, now int64) string {
	return s.allocateGlobalIPExcludingLocked(deviceID, now, nil)
}

func (s *Store) allocateGlobalIPExcludingLocked(deviceID string, now int64, reserved map[string]bool) string {
	s.ensureIPPoolLocked(now)
	var selected GlobalIPAddress
	for _, address := range s.globalIPs {
		if address.Status != "available" {
			continue
		}
		if reserved != nil && reserved[address.IP] {
			continue
		}
		if selected.IP == "" || address.Offset < selected.Offset {
			selected = address
		}
	}
	if selected.IP == "" {
		return ""
	}
	selected.DeviceID = deviceID
	selected.Status = "assigned"
	selected.AssignedAt = now
	selected.ReleasedAt = 0
	s.globalIPs[selected.IP] = selected
	s.ensureIPPoolLocked(now)
	return selected.IP
}

func (s *Store) ensureUniqueGlobalIPsLocked(now int64) {
	deviceIDs := make([]string, 0, len(s.devices))
	for deviceID := range s.devices {
		deviceIDs = append(deviceIDs, deviceID)
	}
	sort.Strings(deviceIDs)

	used := make(map[string]bool, len(deviceIDs))
	reassign := make([]string, 0)
	for _, deviceID := range deviceIDs {
		device := s.devices[deviceID]
		ip := strings.TrimSpace(device.GlobalIP)
		if ip == "" || used[ip] {
			reassign = append(reassign, deviceID)
			continue
		}
		used[ip] = true
		if address, ok := s.globalIPs[ip]; ok {
			address.DeviceID = deviceID
			address.Status = "assigned"
			if address.AssignedAt == 0 {
				address.AssignedAt = now
			}
			address.ReleasedAt = 0
			s.globalIPs[ip] = address
		}
	}

	for _, deviceID := range reassign {
		device := s.devices[deviceID]
		oldIP := strings.TrimSpace(device.GlobalIP)
		if oldIP != "" {
			if address, ok := s.globalIPs[oldIP]; ok && address.DeviceID == deviceID {
				address.DeviceID = ""
				address.Status = "available"
				address.ReleasedAt = now
				s.globalIPs[oldIP] = address
			}
		}
		newIP := s.allocateGlobalIPExcludingLocked(deviceID, now, used)
		device.GlobalIP = newIP
		device.UpdatedAt = now
		s.devices[deviceID] = device
		if newIP != "" {
			used[newIP] = true
		}
	}

	for ip, address := range s.globalIPs {
		if address.Status != "assigned" || address.DeviceID == "" {
			continue
		}
		device, ok := s.devices[address.DeviceID]
		if ok && strings.TrimSpace(device.GlobalIP) == ip {
			continue
		}
		address.DeviceID = ""
		address.Status = "available"
		address.ReleasedAt = now
		s.globalIPs[ip] = address
	}
}

func (s *Store) ensureIPPoolLocked(now int64) {
	for s.availableIPCountLocked() < ipamPoolLowWatermark {
		if !s.generateNextIPSubnetLocked(now) {
			return
		}
	}
}

func (s *Store) availableIPCountLocked() int {
	count := 0
	for _, address := range s.globalIPs {
		if address.Status == "available" {
			count++
		}
	}
	return count
}

func (s *Store) generateNextIPSubnetLocked(now int64) bool {
	startOffset := s.nextIPOffset
	if startOffset >= ipamMaxOffsetIn10CIDR {
		return false
	}
	endOffset := startOffset + ipamSubnetSize - 1
	if endOffset >= ipamMaxOffsetIn10CIDR {
		endOffset = ipamMaxOffsetIn10CIDR - 1
	}
	baseIP := ipFrom10Offset(startOffset)
	subnetID := fmt.Sprintf("ipam-subnet-%06d", s.nextIPSubnetSeq)
	s.nextIPSubnetSeq++
	subnet := IPAMSubnet{
		SubnetID:     subnetID,
		CIDRBlock:    netip.PrefixFrom(netip.MustParseAddr(baseIP), ipamSubnetPrefix).String(),
		BaseIP:       baseIP,
		PrefixLength: ipamSubnetPrefix,
		StartOffset:  startOffset,
		EndOffset:    endOffset,
		Status:       "active",
		CreatedAt:    now,
	}
	for offset := startOffset; offset <= endOffset; offset++ {
		if offset == startOffset || offset == startOffset+1 || offset == endOffset {
			continue
		}
		ip := ipFrom10Offset(offset)
		s.globalIPs[ip] = GlobalIPAddress{
			AddressID: fmt.Sprintf("ipam-address-%06d", s.nextIPAddressSeq),
			SubnetID:  subnetID,
			IP:        ip,
			CIDRBlock: subnet.CIDRBlock,
			Offset:    offset,
			Status:    "available",
			CreatedAt: now,
		}
		s.nextIPAddressSeq++
		subnet.GeneratedCapacity++
	}
	s.ipamSubnets[subnetID] = subnet
	s.nextIPOffset = endOffset + 1
	return true
}

func ipFrom10Offset(offset uint32) string {
	return netip.AddrFrom4([4]byte{10, byte(offset >> 16), byte(offset >> 8), byte(offset)}).String()
}
