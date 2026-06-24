package ops

import (
	"net/http"

	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type DeviceHandler struct {
	OpsDevices servicepkg.OpsManagedDeviceUseCase
}

func (h DeviceHandler) Routes() []serviceapi.Route {
	return withOptAliases([]serviceapi.Route{
		serviceapi.NewRoute(http.MethodGet, "/api/ops/devices", h.OpsListDevices),
		serviceapi.NewRoute(http.MethodPatch, "/api/ops/devices/{deviceId}", h.OpsUpdateDevice),
		serviceapi.NewRoute(http.MethodDelete, "/api/ops/devices/{deviceId}", h.OpsDeleteDevice),
	})
}

func (h DeviceHandler) OpsListDevices(w http.ResponseWriter, r *http.Request) {
	items, err := h.OpsDevices.ListDevices(r.Context())
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteItems(w, serviceapi.MapPayloads(items, managedDevicePayload))
}

func (h DeviceHandler) OpsUpdateDevice(w http.ResponseWriter, r *http.Request) {
	var req updateManagedDeviceRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.toInput()
	serviceapi.SetIfEmpty(&input.DeviceID, requestDeviceID(r))
	item, err := h.OpsDevices.UpdateDevice(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, managedDevicePayload(item))
}

func (h DeviceHandler) OpsDeleteDevice(w http.ResponseWriter, r *http.Request) {
	if err := h.OpsDevices.DeleteDevice(r.Context(), requestDeviceID(r)); err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteOK(w)
}
