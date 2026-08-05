package service

import (
	"net"
	"strconv"
	"strings"

	"github.com/slan/service-biz/internal/model"
)

func opsNodeHealth(status string) string {
	if status == "active" {
		return "healthy"
	}
	return "inactive"
}

func splitOpsEndpoint(endpoint string) (string, int) {
	endpoint = strings.TrimSpace(endpoint)
	for _, prefix := range []string{"udp://", "derp://", "derp+tcp+tls://"} {
		endpoint = strings.TrimPrefix(endpoint, prefix)
	}
	host, portText, err := net.SplitHostPort(endpoint)
	if err != nil {
		return endpoint, 0
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		return host, 0
	}
	return host, port
}

func opsPublicAddr(transport, endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return ""
	}
	if strings.Contains(endpoint, "://") {
		return endpoint
	}
	switch strings.TrimSpace(transport) {
	case "derp_tcp_tls_443":
		return "derp://" + endpoint
	default:
		return "udp://" + endpoint
	}
}

func opsRelayNodeViewFromModel(item model.RelayNode) OpsRelayNodeView {
	return OpsRelayNodeView{
		NodeID:            item.NodeID,
		Name:              item.Name,
		Endpoint:          item.Endpoint,
		Status:            item.Status,
		CreatedAt:         item.CreatedAt,
		UpdatedAt:         item.UpdatedAt,
		Transport:         firstNonEmpty(item.Transport, "relay_udp"),
		PublicAddr:        opsPublicAddr(item.Transport, item.Endpoint),
		MaxBandwidthMbps:  item.MaxBandwidthMbps,
		MonthlyTrafficGB:  item.MonthlyTrafficGB,
		UsedTrafficGB:     item.UsedTrafficGB,
		MaxSessions:       item.MaxSessions,
		ActiveSessions:    item.ActiveSessions,
		Health:            firstNonEmpty(item.Health, opsNodeHealth(item.Status)),
		Priority:          item.Priority,
		TicketKeyRotation: wireTicketKeyStatus(item),
	}
}

func opsPunchNodeViewFromModel(item model.PunchNode) OpsPunchNodeView {
	host, port := splitOpsEndpoint(item.Endpoint)
	return OpsPunchNodeView{
		NodeID:         item.NodeID,
		Name:           item.Name,
		Endpoint:       item.Endpoint,
		Status:         item.Status,
		CreatedAt:      item.CreatedAt,
		UpdatedAt:      item.UpdatedAt,
		PublicUDPIP:    host,
		PublicUDPPort:  port,
		MaxSessions:    item.MaxSessions,
		ActiveSessions: item.ActiveSessions,
		Health:         firstNonEmpty(item.Health, opsNodeHealth(item.Status)),
		Priority:       item.Priority,
	}
}
