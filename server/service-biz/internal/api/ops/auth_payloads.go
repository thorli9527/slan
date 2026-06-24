package ops

import servicepkg "github.com/slan/service-biz/internal/service"

func authPayload(view servicepkg.OpsSessionView) map[string]any {
	return map[string]any{
		"operator": map[string]any{
			"operatorId": view.Operator.OperatorID,
			"name":       view.Operator.Name,
			"email":      view.Operator.Email,
			"role":       view.Operator.Role,
			"status":     view.Operator.Status,
			"createdAt":  view.Operator.CreatedAt,
			"updatedAt":  view.Operator.UpdatedAt,
		},
		"session": map[string]any{
			"token":      view.Session.AccessToken,
			"sessionId":  view.Session.SessionID,
			"operatorId": view.Session.OperatorID,
			"expiresAt":  view.Session.ExpiresAt,
			"createdAt":  view.Session.CreatedAt,
		},
	}
}

func operatorPayload(view servicepkg.OpsOperatorView) map[string]any {
	item := view.Operator
	return map[string]any{
		"operatorId":  item.OperatorID,
		"name":        item.Name,
		"email":       item.Email,
		"role":        item.Role,
		"status":      item.Status,
		"lastLoginAt": view.LastLoginAt,
		"createdAt":   item.CreatedAt,
		"updatedAt":   item.UpdatedAt,
	}
}
