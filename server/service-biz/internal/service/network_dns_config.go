package service

import "strings"

func BuildNetworkDNSConfigView(view NetworkConfigView) NetworkDNSConfigView {
	searchDomains := dnsSearchDomains(view.DNSZones)
	return NetworkDNSConfigView{
		Servers:                   []string{},
		SearchDomains:             searchDomains,
		SplitDomains:              append([]string(nil), searchDomains...),
		FallbackToSystemResolvers: false,
	}
}

func dnsSearchDomains(items []DNSZoneView) []string {
	values := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		values = append(values, name)
	}
	return values
}
