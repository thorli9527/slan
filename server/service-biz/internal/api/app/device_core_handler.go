package app

import (
	"net/http"
	"time"

	serviceapi "github.com/slan/service-biz/internal/api"
	devicerequest "github.com/slan/service-biz/internal/api/devicerequest"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type DeviceHandler struct {
	Devices        servicepkg.DeviceCoreUseCase
	DeviceSessions servicepkg.DeviceSessionUseCase
	NetworkCore    servicepkg.NetworkCoreUseCase
	RuntimeLimiter *deviceRequestLimiter
}

func (h DeviceHandler) Routes() []serviceapi.Route {
	return []serviceapi.Route{
		serviceapi.NewRoute(http.MethodPost, "/api/app/devices/{deviceId}/runtime", h.UpdateDeviceRuntime),
		serviceapi.NewRoute(http.MethodPost, "/api/app/device/runtime/report", h.UpdateDeviceRuntime),
		serviceapi.NewRoute(http.MethodPost, "/api/app/devices/{deviceId}/renew", h.RenewDevice),
	}
}

func (h DeviceHandler) RenewDevice(w http.ResponseWriter, r *http.Request) {
	if !h.allowRuntimeRequest(w, r) {
		return
	}
	deviceID, ok := authenticatedDeviceID(w, r, h.DeviceSessions, requestDeviceID(r))
	if !ok {
		return
	}
	view, err := h.Devices.RenewDevice(r.Context(), deviceID)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, renewedDeviceRuntimePayload(r.Context(), h.NetworkCore, view))
}

func (h DeviceHandler) UpdateDeviceRuntime(w http.ResponseWriter, r *http.Request) {
	if !h.allowRuntimeRequest(w, r) {
		return
	}
	var req devicerequest.UpdateDeviceRuntime
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.ToInput()
	deviceID, ok := authenticatedDeviceID(w, r, h.DeviceSessions, requestDeviceID(r), input.DeviceID)
	if !ok {
		return
	}
	input.DeviceID = deviceID
	view, err := h.Devices.UpdateDeviceRuntime(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, renewedDeviceRuntimePayload(r.Context(), h.NetworkCore, view))
}

func (h DeviceHandler) allowRuntimeRequest(w http.ResponseWriter, r *http.Request) bool {
	if h.RuntimeLimiter == nil || h.RuntimeLimiter.Allow(
		deviceRequestRemoteIP(r), deviceRequestIdentity(serviceapi.AccessTokenFromRequest(r)), time.Now(),
	) {
		return true
	}
	serviceapi.WriteError(w, servicepkg.ErrRateLimited)
	return false
}
