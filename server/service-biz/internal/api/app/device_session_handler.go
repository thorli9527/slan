package app

import (
	"net/http"
	"time"

	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type DeviceSessionHandler struct {
	DeviceSessions    servicepkg.DeviceSessionUseCase
	NetworkRuntime    servicepkg.NetworkRuntimeUseCase
	NetworkConfigView servicepkg.NetworkCoreUseCase
	ServerNodes       servicepkg.OpsServerNodeUseCase
	Limiter           *deviceRequestLimiter
}

func (h DeviceSessionHandler) Routes() []serviceapi.Route {
	return []serviceapi.Route{
		serviceapi.NewRoute(http.MethodPost, "/api/app/device/session/renew", h.RenewDeviceSession),
	}
}

func (h DeviceSessionHandler) RenewDeviceSession(w http.ResponseWriter, r *http.Request) {
	var req renewDeviceSessionRequest
	if !serviceapi.DecodeJSONIfPresentOrError(w, r, &req) {
		return
	}
	accessToken := serviceapi.AccessTokenFromRequest(r)
	if h.Limiter != nil && !h.Limiter.Allow(
		deviceRequestRemoteIP(r), deviceRequestIdentity(accessToken, req.RefreshToken), time.Now(),
	) {
		serviceapi.WriteError(w, servicepkg.ErrRateLimited)
		return
	}
	input := req.toInput()
	input.RemoteIP = serviceapi.RemoteIP(r)
	item, err := h.DeviceSessions.RenewDeviceSession(r.Context(), accessToken, input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, runtimeSessionPayload(
		r.Context(),
		h.NetworkRuntime,
		h.NetworkConfigView,
		h.ServerNodes,
		item.Profile.Device.DeviceID,
		func(punchNodes []servicepkg.PunchNodeView, proxyNodes []servicepkg.OpsServerNodeView, networkConfigs []map[string]any) map[string]any {
			return boundDeviceSessionPayload(item, punchNodes, proxyNodes, networkConfigs)
		},
	))
}
