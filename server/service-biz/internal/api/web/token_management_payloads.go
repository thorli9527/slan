package web

import servicepkg "github.com/slan/service-biz/internal/service"

func userManagedSessionPayload(view servicepkg.UserManagedSessionView) map[string]any {
	return map[string]any{
		"sessionId":     view.SessionID,
		"userId":        view.UserID,
		"status":        view.Status,
		"sessionMode":   view.SessionMode,
		"expiresAt":     view.ExpiresAt,
		"refreshExpiry": view.RefreshExpiry,
		"createdAt":     view.CreatedAt,
		"updatedAt":     view.UpdatedAt,
		"revokedAt":     view.RevokedAt,
	}
}

func deviceManagedSessionPayload(view servicepkg.DeviceManagedSessionView) map[string]any {
	return map[string]any{
		"sessionId":     view.SessionID,
		"deviceId":      view.DeviceID,
		"status":        view.Status,
		"sessionMode":   view.SessionMode,
		"expiresAt":     view.ExpiresAt,
		"refreshExpiry": view.RefreshExpiry,
		"createdAt":     view.CreatedAt,
		"updatedAt":     view.UpdatedAt,
		"revokedAt":     view.RevokedAt,
	}
}
