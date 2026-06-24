package app

import (
	"net/http"

	serviceapi "github.com/slan/service-biz/internal/api"
	devicerequest "github.com/slan/service-biz/internal/api/devicerequest"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type DeviceHandler struct {
	Devices     servicepkg.DeviceCoreUseCase
	NetworkCore servicepkg.NetworkCoreUseCase
}

func (h DeviceHandler) Routes() []serviceapi.Route {
	return []serviceapi.Route{
		serviceapi.NewRoute(http.MethodGet, "/api/devices", h.ListDevices),
		serviceapi.NewRoute(http.MethodPost, "/api/devices/register", h.RegisterDevice),
		serviceapi.NewRoute(http.MethodPost, "/api/devices/{deviceId}/runtime", h.UpdateDeviceRuntime),
		serviceapi.NewRoute(http.MethodPost, "/api/device/runtime/report", h.UpdateDeviceRuntime),
		serviceapi.NewRoute(http.MethodPost, "/api/devices/{deviceId}/renew", h.RenewDevice),
	}
}

func (h DeviceHandler) ListDevices(w http.ResponseWriter, r *http.Request) {
	items, err := h.Devices.ListDeviceProfiles(r.Context(), requestOwnerID(r))
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	payloads := serviceapi.MapPayloads(items, appControlDeviceProfilePayload)
	serviceapi.WriteItems(w, payloads)
}

func (h DeviceHandler) RegisterDevice(w http.ResponseWriter, r *http.Request) {
	var req devicerequest.RegisterDevice
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.ToInput()
	serviceapi.SetIfEmpty(&input.OwnerID, requestOwnerID(r))
	view, err := h.Devices.RegisterDevice(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteEnvelope(w, http.StatusCreated, "device", appControlDeviceProfilePayload(view))
}

func (h DeviceHandler) RenewDevice(w http.ResponseWriter, r *http.Request) {
	view, err := h.Devices.RenewDevice(r.Context(), requestDeviceID(r))
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, renewedDeviceRuntimePayload(r.Context(), h.NetworkCore, view))
}

func (h DeviceHandler) UpdateDeviceRuntime(w http.ResponseWriter, r *http.Request) {
	var req devicerequest.UpdateDeviceRuntime
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.ToInput()
	serviceapi.SetIfEmpty(&input.DeviceID, requestDeviceID(r))
	view, err := h.Devices.UpdateDeviceRuntime(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, renewedDeviceRuntimePayload(r.Context(), h.NetworkCore, view))
}
