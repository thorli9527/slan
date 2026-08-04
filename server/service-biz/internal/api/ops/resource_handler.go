package ops

import (
	"net/http"

	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type ResourceHandler struct{ Resources servicepkg.OpsResourceUseCase }

func (h ResourceHandler) Routes() []serviceapi.Route {
	return withOptAliases([]serviceapi.Route{
		serviceapi.NewRoute(http.MethodGet, "/api/ops/networks", h.ListNetworks),
		serviceapi.NewRoute(http.MethodPost, "/api/ops/networks", h.CreateNetwork),
		serviceapi.NewRoute(http.MethodPatch, "/api/ops/networks/{networkId}", h.UpdateNetwork),
		serviceapi.NewRoute(http.MethodDelete, "/api/ops/networks/{networkId}", h.DeleteNetwork),
		serviceapi.NewRoute(http.MethodPost, "/api/ops/networks/{networkId}/device-groups", h.AddNetworkDeviceGroup),
		serviceapi.NewRoute(http.MethodDelete, "/api/ops/networks/{networkId}/device-groups/{groupId}", h.RemoveNetworkDeviceGroup),
		serviceapi.NewRoute(http.MethodGet, "/api/ops/device-groups", h.ListDeviceGroups),
		serviceapi.NewRoute(http.MethodPost, "/api/ops/device-groups", h.CreateDeviceGroup),
		serviceapi.NewRoute(http.MethodPatch, "/api/ops/device-groups/{groupId}", h.UpdateDeviceGroup),
		serviceapi.NewRoute(http.MethodDelete, "/api/ops/device-groups/{groupId}", h.DeleteDeviceGroup),
		serviceapi.NewRoute(http.MethodPost, "/api/ops/device-groups/{groupId}/devices", h.AddDeviceGroupMember),
		serviceapi.NewRoute(http.MethodDelete, "/api/ops/device-groups/{groupId}/devices/{deviceId}", h.RemoveDeviceGroupMember),
	})
}

func (h ResourceHandler) ListNetworks(w http.ResponseWriter, r *http.Request) {
	items, err := h.Resources.ListOpsNetworks(r.Context())
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteItems(w, items)
}
func (h ResourceHandler) CreateNetwork(w http.ResponseWriter, r *http.Request) {
	var input servicepkg.OpsNetworkInput
	if !serviceapi.DecodeJSONOrError(w, r, &input) {
		return
	}
	item, err := h.Resources.CreateOpsNetwork(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusCreated, item)
}
func (h ResourceHandler) UpdateNetwork(w http.ResponseWriter, r *http.Request) {
	var input servicepkg.OpsNetworkInput
	if !serviceapi.DecodeJSONOrError(w, r, &input) {
		return
	}
	item, err := h.Resources.UpdateOpsNetwork(r.Context(), r.PathValue("networkId"), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, item)
}
func (h ResourceHandler) DeleteNetwork(w http.ResponseWriter, r *http.Request) {
	if err := h.Resources.DeleteOpsNetwork(r.Context(), r.PathValue("networkId")); err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteOK(w)
}
func (h ResourceHandler) AddNetworkDeviceGroup(w http.ResponseWriter, r *http.Request) {
	var input struct {
		GroupID string `json:"groupId"`
	}
	if !serviceapi.DecodeJSONOrError(w, r, &input) {
		return
	}
	if err := h.Resources.AddOpsNetworkDeviceGroup(r.Context(), r.PathValue("networkId"), input.GroupID); err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteOK(w)
}
func (h ResourceHandler) RemoveNetworkDeviceGroup(w http.ResponseWriter, r *http.Request) {
	if err := h.Resources.RemoveOpsNetworkDeviceGroup(r.Context(), r.PathValue("networkId"), r.PathValue("groupId")); err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteOK(w)
}
func (h ResourceHandler) ListDeviceGroups(w http.ResponseWriter, r *http.Request) {
	view, err := h.Resources.ListOpsDeviceGroups(r.Context())
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, view)
}
func (h ResourceHandler) CreateDeviceGroup(w http.ResponseWriter, r *http.Request) {
	var input servicepkg.OpsDeviceGroupInput
	if !serviceapi.DecodeJSONOrError(w, r, &input) {
		return
	}
	item, err := h.Resources.CreateOpsDeviceGroup(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusCreated, item)
}
func (h ResourceHandler) UpdateDeviceGroup(w http.ResponseWriter, r *http.Request) {
	var input servicepkg.OpsDeviceGroupInput
	if !serviceapi.DecodeJSONOrError(w, r, &input) {
		return
	}
	item, err := h.Resources.UpdateOpsDeviceGroup(r.Context(), r.PathValue("groupId"), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, item)
}
func (h ResourceHandler) DeleteDeviceGroup(w http.ResponseWriter, r *http.Request) {
	if err := h.Resources.DeleteOpsDeviceGroup(r.Context(), r.PathValue("groupId")); err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteOK(w)
}
func (h ResourceHandler) AddDeviceGroupMember(w http.ResponseWriter, r *http.Request) {
	var input struct {
		DeviceID string `json:"deviceId"`
	}
	if !serviceapi.DecodeJSONOrError(w, r, &input) {
		return
	}
	if err := h.Resources.AddOpsDeviceGroupMember(r.Context(), r.PathValue("groupId"), input.DeviceID); err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteOK(w)
}
func (h ResourceHandler) RemoveDeviceGroupMember(w http.ResponseWriter, r *http.Request) {
	if err := h.Resources.RemoveOpsDeviceGroupMember(r.Context(), r.PathValue("groupId"), r.PathValue("deviceId")); err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteOK(w)
}
