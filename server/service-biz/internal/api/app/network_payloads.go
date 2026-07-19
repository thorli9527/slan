package app

import (
	"context"
	"net"
	"sort"
	"strings"

	servicepkg "github.com/slan/service-biz/internal/service"
)

func appMQTTCredentialPayload(view servicepkg.DeviceMQTTProfileView) any {
	if view.Credential != nil {
		return view.Credential
	}
	if strings.TrimSpace(view.BrokerURL) == "" &&
		strings.TrimSpace(view.ClientID) == "" &&
		strings.TrimSpace(view.Username) == "" &&
		strings.TrimSpace(view.Password) == "" &&
		strings.TrimSpace(view.TopicPrefix) == "" {
		return nil
	}
	return map[string]any{
		"brokerUrl":   view.BrokerURL,
		"clientId":    view.ClientID,
		"username":    view.Username,
		"password":    view.Password,
		"topicPrefix": view.TopicPrefix,
		"expiresAt":   view.ExpiresAt,
	}
}

func networkConfigsPayload(items []map[string]any) map[string]any {
	return map[string]any{"items": items}
}

func buildDeviceNetworkConfigPayloads(ctx context.Context, useCase servicepkg.NetworkCoreUseCase, deviceID string) []map[string]any {
	resolved, err := useCase.ResolvedDeviceNetworkConfigs(ctx, deviceID)
	if err != nil {
		return []map[string]any{}
	}
	items := make([]map[string]any, 0, len(resolved))
	for _, item := range resolved {
		items = append(items, networkResolvedConfigPayload(item))
	}
	return items
}

func networkResolvedConfigPayload(resolved servicepkg.NetworkResolvedConfigView) map[string]any {
	view := resolved.Config
	view.DNS = servicepkg.BuildNetworkDNSConfigView(view)
	view.SecurityRules = expandedSecurityRules(view)
	deviceIDsByIP := buildDeviceIDsByIP(view)
	zoneNamesByID := buildZoneNamesByID(view.DNSZones)
	orderedRelayCandidates := orderRelayCandidates(view.RuntimePath, resolved.RelayCandidates)
	aclPolicies := aclPoliciesPayload(view)
	return map[string]any{
		"networkId":           view.Network.NetworkID,
		"networkName":         view.Network.Name,
		"networkCode":         appNetworkCode(view.Network),
		"intraGroupPolicy":    firstNonEmpty(view.Network.IntraGroupPolicy, "allow"),
		"networkCreatedAt":    view.Network.CreatedAt,
		"configVersion":       view.ConfigVersion,
		"deviceId":            view.DeviceID,
		"nodeId":              view.NodeID,
		"selfNodeId":          view.NodeID,
		"globalIp":            view.GlobalIP,
		"prefixLen":           view.PrefixLen,
		"globalName":          view.GlobalName,
		"resolver":            networkDNSPayload(view),
		"runtimePath":         runtimePathPayload(view.RuntimePath),
		"peerCount":           len(view.Peers),
		"dnsRecordCount":      len(view.DNSRecords),
		"securityRuleCount":   len(view.SecurityRules),
		"relayCandidateCount": len(orderedRelayCandidates),
		"securityGroups":      securityGroupPayloads(view.SecurityGroups),
		"rules":               securityRulePayloadsForView(view),
		"aclPolicies":         aclPolicies,
		"resolverZones":       dnsZonePayloads(view.DNSZones),
		"resolverRecords":     dnsRecordPayloads(view.DNSRecords, zoneNamesByID, deviceIDsByIP),
		"peers":               networkPeerPayloads(view.Peers),
		"relayCandidates":     relayCandidatePayloads(view.RuntimePath, orderedRelayCandidates),
	}
}

func expandedSecurityRules(view servicepkg.NetworkConfigView) []servicepkg.SecurityRuleView {
	items := make([]servicepkg.SecurityRuleView, 0, len(view.SecurityRules))
	for _, rule := range view.SecurityRules {
		if strings.TrimSpace(rule.PeerType) != "device_group" {
			items = append(items, rule)
			continue
		}
		groupID := strings.TrimSpace(rule.PeerValue)
		if groupID == "" {
			continue
		}
		deviceIDs := securityRuleDeviceGroupMembers(view, groupID)
		for _, deviceID := range deviceIDs {
			cloned := rule
			cloned.PeerType = "device"
			cloned.PeerValue = deviceID
			items = append(items, cloned)
		}
	}
	return items
}

func securityRuleDeviceGroupMembers(view servicepkg.NetworkConfigView, groupID string) []string {
	deviceIDs := make([]string, 0)
	seen := make(map[string]struct{})
	appendIfMatch := func(deviceID string) {
		deviceID = strings.TrimSpace(deviceID)
		if deviceID == "" {
			return
		}
		for _, current := range view.DeviceGroupsByDevice[deviceID] {
			if !strings.EqualFold(strings.TrimSpace(current), groupID) {
				continue
			}
			if _, ok := seen[deviceID]; ok {
				return
			}
			seen[deviceID] = struct{}{}
			deviceIDs = append(deviceIDs, deviceID)
			return
		}
	}
	appendIfMatch(view.DeviceID)
	for _, peer := range view.Peers {
		appendIfMatch(peer.DeviceID)
	}
	return deviceIDs
}

func networkDNSPayload(view servicepkg.NetworkConfigView) map[string]any {
	config := servicepkg.BuildNetworkDNSConfigView(view)
	return map[string]any{
		"servers":                   config.Servers,
		"searchDomains":             config.SearchDomains,
		"splitDomains":              config.SplitDomains,
		"fallbackToSystemResolvers": config.FallbackToSystemResolvers,
	}
}

func runtimeEndpointsPayload(view servicepkg.DeviceMQTTProfileView, punchNodes []servicepkg.PunchNodeView, networkConfigs []map[string]any, refreshedAt int64) map[string]any {
	nodeConfigs := runtimeNodeConfigs(punchNodes, networkConfigs)
	return map[string]any{
		"mqtt":        appMQTTCredentialPayload(view),
		"nodeConfigs": nodeConfigs,
		"refreshedAt": refreshedAt,
	}
}

func runtimeNodeConfigs(punchNodes []servicepkg.PunchNodeView, networkConfigs []map[string]any) []map[string]any {
	nodes := make([]map[string]any, 0, len(punchNodes))
	for index, node := range punchNodes {
		priority := node.Priority
		if priority <= 0 {
			priority = index + 1
		}
		if priority > 99 {
			priority = 99
		}
		nodes = append(nodes, map[string]any{
			"nodeId":         node.NodeID,
			"connectionType": "direct",
			"transport":      "udp",
			"pathKind":       "direct_udp",
			"address":        node.Address,
			"priority":       100 + priority,
			"networkIds":     []string{},
		})
	}
	seenRelayNodes := make(map[string]int)
	for _, config := range networkConfigs {
		networkID, _ := config["networkId"].(string)
		candidates, _ := config["relayCandidates"].([]map[string]any)
		for _, candidate := range candidates {
			key := relayCandidateRuntimeKey(candidate)
			if key == "" {
				continue
			}
			if existing, ok := seenRelayNodes[key]; ok {
				networkIDs, _ := nodes[existing]["networkIds"].([]string)
				nodes[existing]["networkIds"] = uniqueStrings(append(networkIDs, networkID))
				continue
			}
			transport, _ := candidate["transport"].(string)
			address, _ := candidate["address"].(string)
			nodeID, _ := candidate["endpointId"].(string)
			pathKind := "relay_udp"
			nodeTransport := "udp"
			priority := 200 + len(seenRelayNodes)
			if transport == "derp_tcp_tls_443" {
				pathKind = "relay_tcp"
				nodeTransport = "tcp"
				priority = 300 + len(seenRelayNodes)
			}
			seenRelayNodes[key] = len(nodes)
			nodes = append(nodes, map[string]any{
				"nodeId":         nodeID,
				"connectionType": "relay",
				"transport":      nodeTransport,
				"pathKind":       pathKind,
				"address":        address,
				"priority":       priority,
				"networkIds":     compactStrings(networkID),
			})
		}
	}
	sort.SliceStable(nodes, func(i, j int) bool {
		left, _ := nodes[i]["priority"].(int)
		right, _ := nodes[j]["priority"].(int)
		return left < right
	})
	return nodes
}

func relayCandidateRuntimeKey(candidate map[string]any) string {
	transport, _ := candidate["transport"].(string)
	address, _ := candidate["address"].(string)
	transport = strings.TrimSpace(transport)
	address = strings.TrimSpace(address)
	if transport == "" || address == "" {
		return ""
	}
	return transport + "|" + address
}

func relayCandidatePathScore(candidate servicepkg.RelayCandidateView) int {
	switch candidate.Transport {
	case "derp_tcp_tls_443":
		return 600
	case "udp":
		return 700
	default:
		return 500
	}
}

func compactStrings(values ...string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		out = append(out, value)
	}
	return out
}

func relayCandidatePayloads(runtime servicepkg.NetworkRuntimePathView, items []servicepkg.RelayCandidateView) []map[string]any {
	payloads := make([]map[string]any, 0, len(items))
	for _, item := range items {
		payloads = append(payloads, appRelayCandidatePayload(runtime, item))
	}
	return payloads
}

func appRelayCandidatePayload(runtime servicepkg.NetworkRuntimePathView, item servicepkg.RelayCandidateView) map[string]any {
	selected := item.Selected || relayCandidateMatchesRuntime(runtime, item)
	payload := relayCandidateBasePayload(item)
	payload["selected"] = selected
	if item.Reachable || selected {
		payload["reachable"] = true
	}
	if item.ObservedRttMs > 0 {
		payload["rttMs"] = item.ObservedRttMs
		payload["observedRttMs"] = item.ObservedRttMs
	} else if selected && runtime.ObservedRttMs > 0 {
		payload["rttMs"] = runtime.ObservedRttMs
		payload["observedRttMs"] = runtime.ObservedRttMs
	}
	if item.PathScore > 0 {
		payload["pathScore"] = item.PathScore
	} else if selected && runtime.PathScore > 0 {
		payload["pathScore"] = runtime.PathScore
	}
	return payload
}

func relayCandidateBasePayload(item servicepkg.RelayCandidateView) map[string]any {
	return map[string]any{
		"endpointId":  item.EndpointID,
		"transport":   item.Transport,
		"address":     item.Address,
		"countryCode": item.CountryCode,
		"regionId":    item.RegionID,
		"clusterId":   item.ClusterID,
	}
}

func runtimePathPayload(view servicepkg.NetworkRuntimePathView) any {
	if view == (servicepkg.NetworkRuntimePathView{}) {
		return nil
	}
	return map[string]any{
		"natType":         view.NATType,
		"activePath":      view.ActivePath,
		"relayTransport":  view.RelayTransport,
		"relayEndpoint":   view.RelayEndpoint,
		"derpNodeId":      view.DerpNodeID,
		"peerNodeId":      view.PeerNodeID,
		"pathScore":       view.PathScore,
		"rttMs":           view.ObservedRttMs,
		"observedRttMs":   view.ObservedRttMs,
		"packetLossPpm":   view.PacketLossPpm,
		"relayMtu":        view.RelayMtu,
		"maxFramePayload": view.MaxFramePayload,
		"ticketExpiresAt": view.TicketExpiresAt,
		"ticketRenewDue":  view.TicketRenewDue,
		"pathDowngrades":  view.PathDowngrades,
		"pathUpgrades":    view.PathUpgrades,
		"lastPathChange":  view.LastPathChange,
		"observedAt":      view.ObservedAt,
	}
}

func orderRelayCandidates(runtime servicepkg.NetworkRuntimePathView, items []servicepkg.RelayCandidateView) []servicepkg.RelayCandidateView {
	if len(items) < 2 {
		return items
	}
	ordered := append([]servicepkg.RelayCandidateView(nil), items...)
	sort.SliceStable(ordered, func(i, j int) bool {
		left := relayCandidateRank(runtime, ordered[i])
		right := relayCandidateRank(runtime, ordered[j])
		if left != right {
			return left < right
		}
		if ordered[i].RegionID != ordered[j].RegionID {
			return ordered[i].RegionID < ordered[j].RegionID
		}
		return ordered[i].EndpointID < ordered[j].EndpointID
	})
	return ordered
}

func relayCandidateRank(runtime servicepkg.NetworkRuntimePathView, item servicepkg.RelayCandidateView) int {
	if relayCandidateMatchesRuntime(runtime, item) {
		return 0
	}
	if runtime.ActivePath == "relay_udp" && item.Transport == "udp" {
		return 10
	}
	if runtime.ActivePath == "derp_tcp_tls_443" && item.Transport == "derp_tcp_tls_443" {
		return 10
	}
	return 100
}

func relayCandidateMatchesRuntime(runtime servicepkg.NetworkRuntimePathView, item servicepkg.RelayCandidateView) bool {
	if strings.TrimSpace(runtime.RelayEndpoint) != "" && strings.TrimSpace(runtime.RelayEndpoint) != strings.TrimSpace(item.Address) {
		return false
	}
	if strings.TrimSpace(runtime.RelayTransport) != "" && strings.TrimSpace(runtime.RelayTransport) != strings.TrimSpace(item.Transport) {
		return false
	}
	if strings.TrimSpace(runtime.DerpNodeID) != "" && strings.TrimSpace(runtime.DerpNodeID) != strings.TrimSpace(item.EndpointID) {
		return false
	}
	return strings.TrimSpace(runtime.RelayEndpoint) != "" || strings.TrimSpace(runtime.DerpNodeID) != ""
}

func punchNodePayloads(items []servicepkg.PunchNodeView) []map[string]any {
	payloads := make([]map[string]any, 0, len(items))
	for _, item := range items {
		payloads = append(payloads, punchNodeViewPayload(item))
	}
	return payloads
}

func punchNodeViewPayload(item servicepkg.PunchNodeView) map[string]any {
	return map[string]any{
		"nodeId":        item.NodeID,
		"name":          item.Name,
		"region":        item.Region,
		"address":       item.Address,
		"publicUdpIp":   item.PublicUDPIP,
		"publicUdpPort": item.PublicUDPPort,
	}
}

func networkPeerPayloads(items []servicepkg.NetworkConfigPeerView) []map[string]any {
	payloads := make([]map[string]any, 0, len(items))
	for _, item := range items {
		endpoints := peerEndpointPayloads(item)
		payloads = append(payloads, map[string]any{
			"deviceId":     item.DeviceID,
			"nodeId":       "node-" + strings.TrimSpace(item.DeviceID),
			"ownerId":      item.OwnerID,
			"ownerEmail":   item.OwnerEmail,
			"alias":        item.Alias,
			"globalIp":     item.GlobalIP,
			"globalName":   item.GlobalName,
			"status":       item.Status,
			"relayAllowed": true,
			"virtualIps":   compactStrings(item.GlobalIP),
			"endpoints":    endpoints,
		})
	}
	return payloads
}

func peerEndpointPayloads(item servicepkg.NetworkConfigPeerView) []any {
	endpoints := make([]any, 0, len(item.Endpoints)+1)
	for _, endpoint := range item.Endpoints {
		payload := peerEndpointPayload(endpoint.Type, endpoint.Address, endpoint.UpdatedAt)
		if payload == nil {
			continue
		}
		endpoints = append(endpoints, payload)
	}
	if len(endpoints) == 0 && strings.TrimSpace(item.GlobalIP) != "" {
		endpoints = append(endpoints, peerEndpointPayload("direct_udp", net.JoinHostPort(item.GlobalIP, "0"), 0))
	}
	return endpoints
}

func peerEndpointPayload(endpointType, address string, updatedAt int64) any {
	address = strings.TrimSpace(address)
	if address == "" {
		return nil
	}
	return map[string]any{
		"type":      firstNonEmpty(endpointType, "direct_udp"),
		"address":   address,
		"updatedAt": updatedAt,
	}
}

func securityGroupPayloads(items []servicepkg.SecurityGroupView) []map[string]any {
	payloads := make([]map[string]any, 0, len(items))
	for _, item := range items {
		payloads = append(payloads, map[string]any{
			"securityGroupId": item.SecurityGroupID,
			"networkId":       item.NetworkID,
			"name":            item.Name,
			"status":          "active",
			"createdAt":       item.CreatedAt,
		})
	}
	return payloads
}

func aclPoliciesPayload(view servicepkg.NetworkConfigView) []map[string]any {
	return []map[string]any{
		{
			"networkId": view.Network.NetworkID,
			"rules":     securityRulePayloadsForView(view),
		},
	}
}

func securityRulePayloadsForView(view servicepkg.NetworkConfigView) []map[string]any {
	payloads := make([]map[string]any, 0, len(view.SecurityRules))
	for _, item := range view.SecurityRules {
		resolvedNodeID, resolvedIPs := resolveSecurityRulePeer(item, view)
		payloads = append(payloads, map[string]any{
			"ruleId":          item.RuleID,
			"securityGroupId": item.SecurityGroupID,
			"direction":       item.Direction,
			"priority":        item.Priority,
			"action":          item.Action,
			"protocol":        item.Protocol,
			"portFrom":        item.PortFrom,
			"portTo":          item.PortTo,
			"peerType":        item.PeerType,
			"peerValue":       item.PeerValue,
			"sourceType":      item.PeerType,
			"sourceValue":     item.PeerValue,
			"enabled":         item.Enabled,
			"resolvedPeerNodeId": func() any {
				if resolvedNodeID == "" {
					return nil
				}
				return resolvedNodeID
			}(),
			"resolvedPeerVirtualIps": resolvedIPs,
		})
	}
	return payloads
}

func resolveSecurityRulePeer(rule servicepkg.SecurityRuleView, view servicepkg.NetworkConfigView) (string, []string) {
	peerType := strings.ToLower(strings.TrimSpace(rule.PeerType))
	peerValue := strings.TrimSpace(rule.PeerValue)
	switch peerType {
	case "device":
		if peerValue == "" {
			return "", []string{}
		}
		if peerValue == strings.TrimSpace(view.DeviceID) {
			return "node-" + peerValue, compactStrings(view.GlobalIP)
		}
		for _, peer := range view.Peers {
			if strings.TrimSpace(peer.DeviceID) == peerValue {
				return "node-" + peerValue, compactStrings(peer.GlobalIP)
			}
		}
		return "node-" + peerValue, []string{}
	case "user":
		ips := make([]string, 0, len(view.Peers)+1)
		if strings.EqualFold(strings.TrimSpace(view.Network.OwnerID), peerValue) {
			ips = append(ips, compactStrings(view.GlobalIP)...)
		}
		for _, peer := range view.Peers {
			if strings.EqualFold(strings.TrimSpace(peer.OwnerID), peerValue) {
				ips = append(ips, compactStrings(peer.GlobalIP)...)
			}
		}
		return "", uniqueStrings(ips)
	default:
		return "", []string{}
	}
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	if len(values) < 2 {
		return values
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func dnsZonePayloads(items []servicepkg.DNSZoneView) []map[string]any {
	payloads := make([]map[string]any, 0, len(items))
	for _, item := range items {
		payloads = append(payloads, map[string]any{
			"zoneId":       item.ZoneID,
			"networkId":    item.NetworkID,
			"zoneName":     item.Name,
			"exposeGlobal": item.ExposeGlobal,
		})
	}
	return payloads
}

func dnsRecordPayloads(items []servicepkg.DNSRecordView, zoneNamesByID map[string]string, deviceIDsByIP map[string]string) []map[string]any {
	payloads := make([]map[string]any, 0, len(items))
	for _, item := range items {
		targetDeviceID := item.TargetDeviceID
		if targetDeviceID == "" && item.TargetIP != "" {
			if deviceID, ok := deviceIDsByIP[strings.TrimSpace(item.TargetIP)]; ok {
				targetDeviceID = deviceID
			}
		}
		fqdn := strings.TrimSpace(item.Name)
		if zoneName := strings.TrimSpace(zoneNamesByID[item.ZoneID]); fqdn != "" && zoneName != "" && !strings.EqualFold(fqdn, zoneName) && !strings.HasSuffix(strings.ToLower(fqdn), "."+strings.ToLower(zoneName)) {
			fqdn += "." + zoneName
		}
		payloads = append(payloads, map[string]any{
			"recordId":       item.RecordID,
			"zoneId":         item.ZoneID,
			"networkId":      item.NetworkID,
			"name":           item.Name,
			"fqdn":           fqdn,
			"recordType":     item.Type,
			"targetDeviceId": targetDeviceID,
			"targetIp":       item.TargetIP,
			"cname":          item.CNAME,
			"port":           item.Port,
			"ttl":            item.TTL,
		})
	}
	return payloads
}

func buildZoneNamesByID(items []servicepkg.DNSZoneView) map[string]string {
	result := make(map[string]string, len(items))
	for _, item := range items {
		if strings.TrimSpace(item.ZoneID) == "" {
			continue
		}
		result[item.ZoneID] = item.Name
	}
	return result
}

func appNetworkCode(item servicepkg.NetworkView) string {
	code := strings.TrimSpace(item.Code)
	if code != "" {
		return code
	}
	code = strings.TrimSpace(item.CIDR)
	if code != "" {
		return code
	}
	code = strings.TrimSpace(item.NetworkID)
	if len(code) > 8 {
		code = code[:8]
	}
	return code
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func buildDeviceIDsByIP(view servicepkg.NetworkConfigView) map[string]string {
	out := map[string]string{}
	if view.GlobalIP != "" && view.DeviceID != "" {
		out[strings.TrimSpace(view.GlobalIP)] = view.DeviceID
	}
	for _, item := range view.Peers {
		if item.GlobalIP == "" || item.DeviceID == "" {
			continue
		}
		out[strings.TrimSpace(item.GlobalIP)] = item.DeviceID
	}
	return out
}
