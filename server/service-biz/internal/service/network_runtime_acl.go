package service

import (
	"context"
	"net"
	"sort"
	"strings"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

type relayTicketACLContext struct {
	networkID       string
	localDeviceID   string
	localGlobalIP   string
	localGlobalName string
	peers           map[string]relayTicketACLPeer
}

type relayTicketACLPeer struct {
	globalIP   string
	globalName string
	alias      string
}

func ensureRelayTicketAllowed(
	ctx context.Context,
	devices repository.DeviceRepository,
	networks repository.NetworkRepository,
	network model.Network,
	srcNodeID string,
	dstNodeID string,
) error {
	srcDeviceID := relayTicketDeviceID(srcNodeID)
	dstDeviceID := relayTicketDeviceID(dstNodeID)
	if srcDeviceID == "" || dstDeviceID == "" {
		return ErrInvalidArgument
	}

	memberships, err := networks.ListNetworkDevices(ctx, network.NetworkID)
	if err != nil {
		return err
	}
	activeMemberships := relayTicketActiveMemberships(memberships)
	if _, ok := activeMemberships[srcDeviceID]; !ok {
		return ErrNotFound
	}
	if _, ok := activeMemberships[dstDeviceID]; !ok {
		return ErrNotFound
	}

	groups, err := networks.ListSecurityGroups(ctx, network.NetworkID)
	if err != nil {
		return err
	}
	rules, err := buildNetworkSecurityRuleViews(ctx, networks, groups)
	if err != nil {
		return err
	}
	if len(rules) == 0 {
		return nil
	}

	deviceIDs := relayTicketDeviceIDs(activeMemberships)
	deviceMap, err := relayTicketDevices(ctx, devices, deviceIDs)
	if err != nil {
		return err
	}
	globalIPs, _ := assignedNetworkIPMap(network.CIDR, deviceIDs)
	dstIP := strings.TrimSpace(globalIPs[dstDeviceID])

	srcACL := newRelayTicketACLContext(network.NetworkID, srcDeviceID, deviceMap, globalIPs)
	if relayTicketBroadDeny(rules, srcACL, "egress", dstIP) {
		return ErrForbidden
	}
	dstACL := newRelayTicketACLContext(network.NetworkID, dstDeviceID, deviceMap, globalIPs)
	if relayTicketBroadDeny(rules, dstACL, "ingress", dstIP) {
		return ErrForbidden
	}
	return nil
}

func relayTicketDeviceID(nodeID string) string {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return ""
	}
	return strings.TrimPrefix(nodeID, "node-")
}

func relayTicketActiveMemberships(items []model.NetworkDevice) map[string]model.NetworkDevice {
	out := make(map[string]model.NetworkDevice, len(items))
	for _, item := range items {
		if item.DeviceID == "" || !item.Enabled || item.Status != "active" {
			continue
		}
		out[item.DeviceID] = item
	}
	return out
}

func relayTicketDeviceIDs(items map[string]model.NetworkDevice) []string {
	out := make([]string, 0, len(items))
	for deviceID := range items {
		if strings.TrimSpace(deviceID) == "" {
			continue
		}
		out = append(out, deviceID)
	}
	return out
}

func relayTicketDevices(ctx context.Context, devices repository.DeviceRepository, deviceIDs []string) (map[string]model.Device, error) {
	out := make(map[string]model.Device, len(deviceIDs))
	if devices == nil {
		return out, nil
	}
	for _, deviceID := range deviceIDs {
		item, ok, err := devices.GetDevice(ctx, deviceID)
		if err != nil {
			return nil, err
		}
		if ok {
			out[deviceID] = item
		}
	}
	return out, nil
}

func newRelayTicketACLContext(networkID, localDeviceID string, devices map[string]model.Device, globalIPs map[string]string) relayTicketACLContext {
	local := devices[localDeviceID]
	ctx := relayTicketACLContext{
		networkID:       networkID,
		localDeviceID:   localDeviceID,
		localGlobalIP:   strings.TrimSpace(globalIPs[localDeviceID]),
		localGlobalName: networkGlobalName(localDeviceID, local.Alias, local.Name),
		peers:           make(map[string]relayTicketACLPeer, len(globalIPs)),
	}
	for deviceID, globalIP := range globalIPs {
		if deviceID == localDeviceID {
			continue
		}
		item := devices[deviceID]
		ctx.peers[deviceID] = relayTicketACLPeer{
			globalIP:   strings.TrimSpace(globalIP),
			globalName: networkGlobalName(deviceID, item.Alias, item.Name),
			alias:      strings.TrimSpace(item.Alias),
		}
	}
	return ctx
}

func relayTicketBroadDeny(rules []SecurityRuleView, ctx relayTicketACLContext, direction string, subjectIP string) bool {
	ordered := append([]SecurityRuleView(nil), rules...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Priority == ordered[j].Priority {
			return ordered[i].RuleID < ordered[j].RuleID
		}
		return ordered[i].Priority < ordered[j].Priority
	})
	for _, rule := range ordered {
		if !rule.Enabled {
			continue
		}
		if !relayTicketDirectionMatches(rule.Direction, direction) {
			continue
		}
		if !relayTicketBroadProtocolMatches(rule.Protocol) {
			continue
		}
		if !relayTicketBroadPortMatches(rule.PortFrom, rule.PortTo) {
			continue
		}
		if !relayTicketPeerMatches(rule, ctx, subjectIP) {
			continue
		}
		return !strings.EqualFold(strings.TrimSpace(rule.Action), "allow")
	}
	return false
}

func relayTicketDirectionMatches(ruleDirection, expected string) bool {
	value := strings.ToLower(strings.TrimSpace(ruleDirection))
	switch {
	case value == "", value == "all", value == "any":
		return true
	case strings.EqualFold(expected, "egress"):
		return value == "egress" || value == "out" || value == "outbound"
	default:
		return value == "ingress" || value == "in" || value == "inbound"
	}
}

func relayTicketBroadProtocolMatches(protocol string) bool {
	value := strings.ToLower(strings.TrimSpace(protocol))
	return value == "" || value == "all" || value == "any"
}

func relayTicketBroadPortMatches(portFrom, portTo int) bool {
	return max(0, portFrom) == 0 && max(0, portTo) == 0
}

func relayTicketPeerMatches(rule SecurityRuleView, ctx relayTicketACLContext, subjectIP string) bool {
	subjectIP = strings.TrimSpace(subjectIP)
	peerType := strings.ToLower(strings.TrimSpace(rule.PeerType))
	peerValue := strings.TrimSpace(rule.PeerValue)

	switch peerType {
	case "", "all", "any":
		return peerValue == "" ||
			strings.EqualFold(peerValue, "all") ||
			strings.EqualFold(peerValue, "any") ||
			peerValue == "*"
	case "network", "workspace":
		return peerValue == "" ||
			strings.EqualFold(peerValue, "self") ||
			strings.EqualFold(peerValue, "all") ||
			peerValue == ctx.networkID
	case "ip", "cidr", "subnet":
		return relayTicketIPMatches(peerValue, subjectIP)
	case "device":
		if subjectIP == "" || peerValue == "" {
			return false
		}
		if peerValue == ctx.localDeviceID {
			return relayTicketSameIP(subjectIP, ctx.localGlobalIP)
		}
		peer, ok := ctx.peers[peerValue]
		return ok && relayTicketSameIP(subjectIP, peer.globalIP)
	case "domain", "dns":
		if subjectIP == "" || peerValue == "" {
			return false
		}
		if strings.EqualFold(ctx.localGlobalName, peerValue) {
			return relayTicketSameIP(subjectIP, ctx.localGlobalIP)
		}
		for _, peer := range ctx.peers {
			if strings.EqualFold(peer.globalName, peerValue) || strings.EqualFold(peer.alias, peerValue) {
				return relayTicketSameIP(subjectIP, peer.globalIP)
			}
		}
		return false
	default:
		return false
	}
}

func relayTicketSameIP(left, right string) bool {
	return strings.TrimSpace(left) != "" && strings.TrimSpace(left) == strings.TrimSpace(right)
}

func relayTicketIPMatches(pattern, ip string) bool {
	pattern = strings.TrimSpace(pattern)
	ip = strings.TrimSpace(ip)
	if pattern == "" || strings.EqualFold(pattern, "all") || pattern == "*" {
		return true
	}
	if ip == "" {
		return false
	}
	if strings.Contains(pattern, "/") {
		_, ipnet, err := net.ParseCIDR(pattern)
		if err != nil || ipnet == nil {
			return false
		}
		parsed := net.ParseIP(ip)
		return parsed != nil && ipnet.Contains(parsed)
	}
	return pattern == ip
}
