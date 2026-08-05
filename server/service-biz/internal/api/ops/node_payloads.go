package ops

import servicepkg "github.com/slan/service-biz/internal/service"

func relayNodePayload(view servicepkg.OpsRelayNodeView) map[string]any {
	return map[string]any{
		"nodeId":            view.NodeID,
		"name":              view.Name,
		"transport":         view.Transport,
		"publicAddr":        view.PublicAddr,
		"priority":          view.Priority,
		"ticketKeyRotation": view.TicketKeyRotation,
		"maxBandwidthMbps":  view.MaxBandwidthMbps,
		"monthlyTrafficGb":  view.MonthlyTrafficGB,
		"usedTrafficGb":     view.UsedTrafficGB,
		"maxSessions":       view.MaxSessions,
		"activeSessions":    view.ActiveSessions,
		"status":            view.Status,
		"health":            view.Health,
		"endpoint":          view.Endpoint,
		"createdAt":         view.CreatedAt,
		"updatedAt":         view.UpdatedAt,
	}
}

func punchNodePayload(view servicepkg.OpsPunchNodeView) map[string]any {
	return map[string]any{
		"nodeId":         view.NodeID,
		"name":           view.Name,
		"publicUdpIp":    view.PublicUDPIP,
		"publicUdpPort":  view.PublicUDPPort,
		"maxSessions":    view.MaxSessions,
		"activeSessions": view.ActiveSessions,
		"status":         view.Status,
		"health":         view.Health,
		"endpoint":       view.Endpoint,
		"priority":       view.Priority,
		"createdAt":      view.CreatedAt,
		"updatedAt":      view.UpdatedAt,
	}
}
