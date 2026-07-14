package web

import (
	"net/http"

	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type DeviceGroupHandler struct {
	DeviceGroups servicepkg.DeviceGroupUseCase
}

func (h DeviceGroupHandler) Routes() []serviceapi.Route {
	return []serviceapi.Route{
		serviceapi.NewRoute(http.MethodGet, "/api/users/{userId}/device-groups", h.ListDeviceGroups),
		serviceapi.NewRoute(http.MethodGet, "/api/networks/{networkId}/device-groups", h.ListNetworkDeviceGroups),
		serviceapi.NewRoute(http.MethodPost, "/api/networks/{networkId}/device-groups", h.AddNetworkDeviceGroup),
		serviceapi.NewRoute(http.MethodDelete, "/api/networks/{networkId}/device-groups/{groupId}", h.RemoveNetworkDeviceGroup),
		serviceapi.NewRoute(http.MethodPost, "/api/users/{userId}/device-groups", h.CreateDeviceGroup),
		serviceapi.NewRoute(http.MethodPatch, "/api/users/{userId}/device-groups/{groupId}", h.UpdateDeviceGroup),
		serviceapi.NewRoute(http.MethodDelete, "/api/users/{userId}/device-groups/{groupId}", h.DeleteDeviceGroup),
		serviceapi.NewRoute(http.MethodPut, "/api/users/{userId}/devices/{deviceId}/groups", h.SetDeviceGroups),
	}
}

func (h DeviceGroupHandler) AddNetworkDeviceGroup(w http.ResponseWriter, r *http.Request) {
	var req networkDeviceGroupRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	view, err := h.DeviceGroups.AddNetworkDeviceGroup(r.Context(), servicepkg.AddNetworkDeviceGroupInput{
		NetworkID:   requestNetworkID(r),
		GroupID:     req.GroupID,
		ActorUserID: firstNonEmpty(requestActorUserID(r), req.ActorUserID),
	})
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	writeDeviceGroupCollection(w, view)
}

func (h DeviceGroupHandler) RemoveNetworkDeviceGroup(w http.ResponseWriter, r *http.Request) {
	view, err := h.DeviceGroups.RemoveNetworkDeviceGroup(r.Context(), servicepkg.RemoveNetworkDeviceGroupInput{
		NetworkID:   requestNetworkID(r),
		GroupID:     requestGroupID(r),
		ActorUserID: requestActorUserID(r),
	})
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	writeDeviceGroupCollection(w, view)
}

func (h DeviceGroupHandler) ListDeviceGroups(w http.ResponseWriter, r *http.Request) {
	view, err := h.DeviceGroups.ListDeviceGroups(r.Context(), requestUserID(r))
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	writeDeviceGroupCollection(w, view)
}

func (h DeviceGroupHandler) ListNetworkDeviceGroups(w http.ResponseWriter, r *http.Request) {
	view, err := h.DeviceGroups.ListNetworkDeviceGroups(r.Context(), requestNetworkID(r))
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	writeDeviceGroupCollection(w, view)
}

func writeDeviceGroupCollection(w http.ResponseWriter, view servicepkg.DeviceGroupCollectionView) {
	serviceapi.WriteItemsWithMembers(w, serviceapi.MapPayloads(view.Items, deviceGroupPayload), view.Members)
}

func (h DeviceGroupHandler) CreateDeviceGroup(w http.ResponseWriter, r *http.Request) {
	var req createDeviceGroupRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.toInput()
	setUserAndActor(r, &input.UserID, &input.ActorUserID)
	item, err := h.DeviceGroups.CreateDeviceGroup(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusCreated, deviceGroupPayload(item))
}

func (h DeviceGroupHandler) UpdateDeviceGroup(w http.ResponseWriter, r *http.Request) {
	var req updateDeviceGroupRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.toInput()
	setGroupAndActor(r, &input.GroupID, &input.ActorUserID)
	item, err := h.DeviceGroups.UpdateDeviceGroup(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, deviceGroupPayload(item))
}

func (h DeviceGroupHandler) DeleteDeviceGroup(w http.ResponseWriter, r *http.Request) {
	input := servicepkg.DeleteDeviceGroupInput{
		GroupID:     requestGroupID(r),
		ActorUserID: requestActorUserID(r),
	}
	if err := h.DeviceGroups.DeleteDeviceGroup(r.Context(), input); err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteOK(w)
}

func (h DeviceGroupHandler) SetDeviceGroups(w http.ResponseWriter, r *http.Request) {
	var req setDeviceGroupsRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.toInput()
	setUserAndActor(r, &input.UserID, &input.ActorUserID)
	serviceapi.SetIfEmpty(&input.DeviceID, requestDeviceID(r))
	if err := h.DeviceGroups.SetDeviceGroups(r.Context(), input); err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteOK(w)
}
