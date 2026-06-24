package ops

import servicepkg "github.com/slan/service-biz/internal/service"

func dashboardPayload(view servicepkg.OpsDashboardView) any {
	return view
}

func auditEventPayload(view servicepkg.OpsAuditEventView) map[string]any {
	return map[string]any{
		"eventId":      view.EventID,
		"actorType":    view.ActorType,
		"actorId":      view.ActorID,
		"action":       view.Action,
		"resourceType": view.ResourceType,
		"resourceId":   view.ResourceID,
		"status":       view.Status,
		"createdAt":    view.CreatedAt,
	}
}
