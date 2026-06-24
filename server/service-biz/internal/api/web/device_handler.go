package web

import (
	"net/http"

	serviceapi "github.com/slan/service-biz/internal/api"
	devicerequest "github.com/slan/service-biz/internal/api/devicerequest"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type DeviceHandler struct {
	Devices servicepkg.DeviceCoreUseCase
}

func (h DeviceHandler) Routes() []serviceapi.Route {
	return []serviceapi.Route{
		serviceapi.NewRoute(http.MethodGet, "/api/users/{userId}/devices/visible", h.ListVisibleDevices),
		serviceapi.NewRoute(http.MethodGet, "/api/devices", h.ListDevices),
		serviceapi.NewRoute(http.MethodGet, "/api/devices/visible", h.ListVisibleDevices),
		serviceapi.NewRoute(http.MethodPost, "/api/devices/register", h.RegisterDevice),
		serviceapi.NewRoute(http.MethodPatch, "/api/devices/{deviceId}", h.UpdateDeviceAlias),
		serviceapi.NewRoute(http.MethodDelete, "/api/devices/{deviceId}", h.DeleteDevice),
	}
}

func (h DeviceHandler) ListDevices(w http.ResponseWriter, r *http.Request) {
	items, err := h.Devices.ListDeviceProfiles(r.Context(), requestOwnerID(r))
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	payloads := serviceapi.MapPayloads(items, devicePayload)
	serviceapi.WriteItems(w, payloads)
}

func (h DeviceHandler) RegisterDevice(w http.ResponseWriter, r *http.Request) {
	var req devicerequest.RegisterDevice
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.ToInput()
	setOwnerAndActor(r, &input.OwnerID, &input.ActorUserID)
	view, err := h.Devices.RegisterDevice(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusCreated, devicePayload(view))
}

func (h DeviceHandler) ListVisibleDevices(w http.ResponseWriter, r *http.Request) {
	items, err := h.Devices.ListVisibleDeviceProfiles(r.Context(), requestOwnerID(r))
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	payloads := serviceapi.MapPayloads(items, devicePayload)
	serviceapi.WriteItems(w, payloads)
}

func (h DeviceHandler) UpdateDeviceAlias(w http.ResponseWriter, r *http.Request) {
	var req updateDeviceAliasRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.toInput()
	setDeviceAndActor(r, &input.DeviceID, &input.ActorUserID)
	view, err := h.Devices.UpdateDeviceAlias(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, devicePayload(view))
}

func (h DeviceHandler) DeleteDevice(w http.ResponseWriter, r *http.Request) {
	input := servicepkg.DeleteDeviceInput{
		DeviceID:    requestDeviceID(r),
		ActorUserID: requestActorUserID(r),
	}
	if err := h.Devices.DeleteDevice(r.Context(), input); err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteOK(w)
}
