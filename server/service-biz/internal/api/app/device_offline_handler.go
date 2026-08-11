package app

import (
	"net/http"

	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type DeviceOfflineHandler struct {
	DeviceSessions servicepkg.DeviceSessionUseCase
	Offline        servicepkg.DeviceOfflineUseCase
}

func (h DeviceOfflineHandler) Routes() []serviceapi.Route {
	return []serviceapi.Route{
		serviceapi.NewRoute(http.MethodPost, "/api/app/device/offline-report", h.ReportOffline),
	}
}

func (h DeviceOfflineHandler) ReportOffline(w http.ResponseWriter, r *http.Request) {
	session, ok := authenticatedDeviceSession(w, r, h.DeviceSessions)
	if !ok {
		return
	}
	if err := h.Offline.ReportDeviceOffline(r.Context(), session.DeviceID, session.CredentialID); err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteEnvelope(w, http.StatusOK, "offline", map[string]any{
		"deviceId": session.DeviceID,
	})
}
