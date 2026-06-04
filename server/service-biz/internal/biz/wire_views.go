package biz

import (
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

func wireRelayNodeViews(nodes []OpsRelayNode) []map[string]any {
	out := make([]map[string]any, 0)
	for _, node := range nodes {
		if node.Transport != "relay_udp" {
			continue
		}
		out = append(out, wireRelayNodeView(node))
	}
	return out
}

func wireRelayNodeView(node OpsRelayNode) map[string]any {
	host, port := splitHostPort(node.PublicAddr)
	_, adminPort := splitHostPort(node.InternalAddr)
	stale := wireNodeStale(node, time.Now().Unix())
	return map[string]any{
		"regionId":          node.Region,
		"nodeId":            node.NodeID,
		"host":              host,
		"udpPort":           port,
		"adminPort":         adminPort,
		"enabled":           node.Status == "active",
		"healthy":           node.Health == "healthy",
		"stale":             stale,
		"priority":          defaultInt(node.Priority, 100),
		"updatedAtMs":       node.UpdatedAt * 1000,
		"ticketKeyRotation": node.TicketKeyRotation,
	}
}

func wireDerpNodeViews(nodes []OpsRelayNode) []map[string]any {
	out := make([]map[string]any, 0)
	for _, node := range nodes {
		if node.Transport != "derp_tcp_tls_443" {
			continue
		}
		out = append(out, wireDerpNodeView(node))
	}
	return out
}

func wireDerpNodeView(node OpsRelayNode) map[string]any {
	host, port := splitHostPort(node.PublicAddr)
	stale := wireNodeStale(node, time.Now().Unix())
	return map[string]any{
		"regionId":          node.Region,
		"nodeId":            node.NodeID,
		"name":              node.Name,
		"host":              host,
		"port":              port,
		"enabled":           node.Status == "active",
		"healthy":           node.Health == "healthy",
		"stale":             stale,
		"priority":          defaultInt(node.Priority, 100),
		"updatedAtMs":       node.UpdatedAt * 1000,
		"ticketKeyRotation": node.TicketKeyRotation,
	}
}

func derpMapFromRelayNodes(nodes []OpsRelayNode) map[string]any {
	regions := make(map[string][]map[string]any)
	preferred := ""
	now := time.Now().Unix()
	for _, node := range nodes {
		if node.Transport != "derp_tcp_tls_443" || node.Status != "active" || node.Health != "healthy" || wireNodeStale(node, now) {
			continue
		}
		host, port := splitHostPort(node.PublicAddr)
		if preferred == "" {
			preferred = node.Region
		}
		regions[node.Region] = append(regions[node.Region], map[string]any{
			"regionId": node.Region,
			"nodeId":   node.NodeID,
			"host":     host,
			"port":     port,
		})
	}
	items := make([]map[string]any, 0, len(regions))
	for regionID, nodes := range regions {
		items = append(items, map[string]any{
			"regionId": regionID,
			"name":     regionID,
			"nodes":    nodes,
		})
	}
	return map[string]any{"preferredRegionId": preferred, "regions": items}
}

func wireNodeStale(node OpsRelayNode, now int64) bool {
	if node.UpdatedAt <= 0 {
		return false
	}
	freshness := int64(120)
	if value, err := strconv.ParseInt(strings.TrimSpace(os.Getenv("SLAN_WIRE_NODE_HEARTBEAT_FRESHNESS_SECONDS")), 10, 64); err == nil && value > 0 {
		freshness = value
	}
	return now-node.UpdatedAt > freshness
}

func hostPort(host string, port int) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	if port <= 0 {
		return host
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
}

func splitHostPort(value string) (string, int) {
	value = strings.TrimSpace(strings.TrimPrefix(value, "udp://"))
	if value == "" {
		return "", 0
	}
	host, portText, err := net.SplitHostPort(value)
	if err != nil {
		return value, 0
	}
	port, _ := strconv.Atoi(portText)
	return host, port
}

func wirePeerID(networkID, deviceID string) string {
	return strings.TrimSpace(networkID) + ":" + strings.TrimSpace(deviceID)
}

func parseWirePeerID(peerID string) (string, string) {
	peerID = strings.TrimSpace(peerID)
	if strings.Contains(peerID, ":") {
		parts := strings.SplitN(peerID, ":", 2)
		return strings.TrimSpace(parts[0]), strings.TrimPrefix(strings.TrimSpace(parts[1]), "node-")
	}
	deviceID := deviceIDFromNodeID(peerID)
	if deviceID == "" {
		deviceID = peerID
	}
	return "", deviceID
}
