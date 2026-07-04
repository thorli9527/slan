package app

import (
	"net/http"

	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type DeviceSessionHandler struct {
	DeviceBootstrap   servicepkg.DeviceBootstrapUseCase
	DeviceSessions    servicepkg.DeviceSessionUseCase
	AuthSessions      servicepkg.AuthUserSessionUseCase
	NetworkRuntime    servicepkg.NetworkRuntimeUseCase
	NetworkConfigView servicepkg.NetworkCoreUseCase
}

func (h DeviceSessionHandler) Routes() []serviceapi.Route {
	return []serviceapi.Route{
		serviceapi.NewRoute(http.MethodPost, "/api/app/device/session/bootstrap", h.BootstrapDeviceSession),
		serviceapi.NewRoute(http.MethodPost, "/api/app/device/session/bind", h.BindDeviceSession),
		serviceapi.NewRoute(http.MethodPost, "/api/app/device/session/renew", h.RenewDeviceSession),
	}
}

func (h DeviceSessionHandler) BootstrapDeviceSession(w http.ResponseWriter, r *http.Request) {
	var req bootstrapDeviceSessionRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.toInput()
	item, err := h.DeviceBootstrap.BootstrapDeviceSession(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, runtimeSessionPayload(
		r.Context(),
		h.NetworkRuntime,
		h.NetworkConfigView,
		item.Device.DeviceID,
		func(punchNodes []servicepkg.PunchNodeView, networkConfigs []map[string]any) map[string]any {
			return deviceSessionPayload(item, punchNodes, networkConfigs)
		},
	))
}

func (h DeviceSessionHandler) BindDeviceSession(w http.ResponseWriter, r *http.Request) {
	var req bindDeviceSessionRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.toInput()
	serviceapi.ResolveUserIDFromRequest(r, h.AuthSessions, &input.UserID)
	item, err := h.DeviceSessions.BindDeviceSession(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, runtimeSessionPayload(
		r.Context(),
		h.NetworkRuntime,
		h.NetworkConfigView,
		item.Profile.Device.DeviceID,
		func(punchNodes []servicepkg.PunchNodeView, networkConfigs []map[string]any) map[string]any {
			return boundDeviceSessionPayload(item, punchNodes, networkConfigs)
		},
	))
}

func (h DeviceSessionHandler) RenewDeviceSession(w http.ResponseWriter, r *http.Request) {
	var req renewDeviceSessionRequest
	if !serviceapi.DecodeJSONIfPresentOrError(w, r, &req) {
		return
	}
	item, err := h.DeviceSessions.RenewDeviceSession(
		r.Context(),
		serviceapi.AccessTokenFromRequest(r),
		req.toInput(),
	)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, runtimeSessionPayload(
		r.Context(),
		h.NetworkRuntime,
		h.NetworkConfigView,
		item.Profile.Device.DeviceID,
		func(punchNodes []servicepkg.PunchNodeView, networkConfigs []map[string]any) map[string]any {
			return boundDeviceSessionPayload(item, punchNodes, networkConfigs)
		},
	))
}
