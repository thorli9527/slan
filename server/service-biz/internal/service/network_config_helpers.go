package service

import (
	"net"
	"sort"
	"strings"
)

func assignedNetworkIPMap(cidr string, deviceIDs []string) (map[string]string, int) {
	ipMap := map[string]string{}
	_, ipnet, err := net.ParseCIDR(strings.TrimSpace(cidr))
	if err != nil || ipnet == nil {
		return ipMap, 0
	}
	prefixLen, bits := ipnet.Mask.Size()
	if bits != 32 {
		return ipMap, prefixLen
	}
	base := ipnet.IP.To4()
	if base == nil {
		return ipMap, prefixLen
	}
	sorted := append([]string(nil), deviceIDs...)
	sort.Strings(sorted)
	for i, deviceID := range sorted {
		if deviceID == "" {
			continue
		}
		hostIP := append(net.IP(nil), base...)
		offset := i + 10
		for j := len(hostIP) - 1; j >= 0 && offset > 0; j-- {
			offset += int(hostIP[j])
			hostIP[j] = byte(offset % 256)
			offset /= 256
		}
		if !ipnet.Contains(hostIP) {
			continue
		}
		ipMap[deviceID] = hostIP.String()
	}
	return ipMap, prefixLen
}

func networkGlobalName(deviceID, alias, name string) string {
	if value := strings.TrimSpace(alias); value != "" {
		return value
	}
	if value := strings.TrimSpace(name); value != "" {
		return value
	}
	return strings.TrimSpace(deviceID)
}
