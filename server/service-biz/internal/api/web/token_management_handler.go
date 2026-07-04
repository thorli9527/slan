package web

import (
	"net/http"

	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type TokenManagementHandler struct {
	UserTokens   servicepkg.UserTokenManagementUseCase
	DeviceTokens servicepkg.DeviceTokenManagementUseCase
}

func (h TokenManagementHandler) Routes() []serviceapi.Route {
	return []serviceapi.Route{
		serviceapi.NewRoute(http.MethodGet, "/api/users/{userId}/sessions", h.ListUserSessions),
		serviceapi.NewRoute(http.MethodPost, "/api/users/{userId}/sessions/{sessionId}/revoke", h.RevokeUserSession),
		serviceapi.NewRoute(http.MethodGet, "/api/devices/{deviceId}/sessions", h.ListDeviceSessions),
		serviceapi.NewRoute(http.MethodPost, "/api/devices/{deviceId}/sessions/{sessionId}/revoke", h.RevokeDeviceSession),
	}
}

func (h TokenManagementHandler) ListUserSessions(w http.ResponseWriter, r *http.Request) {
	items, err := h.UserTokens.ListUserSessions(r.Context(), requestUserID(r))
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteItems(w, serviceapi.MapPayloads(items, userManagedSessionPayload))
}

func (h TokenManagementHandler) RevokeUserSession(w http.ResponseWriter, r *http.Request) {
	item, err := h.UserTokens.RevokeUserSession(r.Context(), servicepkg.RevokeUserManagedSessionInput{
		UserID:      requestUserID(r),
		ActorUserID: requestActorUserID(r),
		SessionID:   serviceapi.PathOrQuery(r, "sessionId", "sessionId"),
	})
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, userManagedSessionPayload(item))
}

func (h TokenManagementHandler) ListDeviceSessions(w http.ResponseWriter, r *http.Request) {
	items, err := h.DeviceTokens.ListDeviceSessions(r.Context(), requestDeviceID(r))
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteItems(w, serviceapi.MapPayloads(items, deviceManagedSessionPayload))
}

func (h TokenManagementHandler) RevokeDeviceSession(w http.ResponseWriter, r *http.Request) {
	item, err := h.DeviceTokens.RevokeDeviceSession(r.Context(), servicepkg.RevokeDeviceManagedSessionInput{
		DeviceID:    requestDeviceID(r),
		ActorUserID: requestActorUserID(r),
		SessionID:   serviceapi.PathOrQuery(r, "sessionId", "sessionId"),
	})
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, deviceManagedSessionPayload(item))
}
