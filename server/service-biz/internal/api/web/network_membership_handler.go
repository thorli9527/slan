package web

import (
	"net/http"

	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type NetworkMembershipHandler struct {
	NetworkInvite servicepkg.NetworkInviteUseCase
}

func (h NetworkMembershipHandler) Routes() []serviceapi.Route {
	return []serviceapi.Route{
		serviceapi.NewRoute(http.MethodGet, "/api/networks/{networkId}/devices", h.ListNetworkDevices),
		serviceapi.NewRoute(http.MethodPost, "/api/networks/{networkId}/devices", h.AddNetworkDevice),
		serviceapi.NewRoute(http.MethodPatch, "/api/networks/{networkId}/devices/{deviceId}", h.UpdateNetworkDevice),
		serviceapi.NewRoute(http.MethodDelete, "/api/networks/{networkId}/devices/{deviceId}", h.RemoveNetworkDevice),
	}
}

func (h NetworkMembershipHandler) ListNetworkDevices(w http.ResponseWriter, r *http.Request) {
	items, err := h.NetworkInvite.ListNetworkDevices(r.Context(), requestNetworkID(r))
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	payloads := serviceapi.MapPayloads(items, networkDevicePayload)
	serviceapi.WriteItems(w, payloads)
}

func (h NetworkMembershipHandler) AddNetworkDevice(w http.ResponseWriter, r *http.Request) {
	var req addNetworkDeviceRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.toInput()
	setNetworkAndActor(r, &input.NetworkID, &input.ActorUserID)
	item, err := h.NetworkInvite.AddNetworkDevice(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusCreated, networkDevicePayload(item))
}

func (h NetworkMembershipHandler) UpdateNetworkDevice(w http.ResponseWriter, r *http.Request) {
	var req updateNetworkDeviceRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.toInput()
	serviceapi.SetIfEmpty(&input.NetworkID, requestNetworkID(r))
	setDeviceAndActor(r, &input.DeviceID, &input.ActorUserID)
	item, err := h.NetworkInvite.UpdateNetworkDevice(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, networkDevicePayload(item))
}

func (h NetworkMembershipHandler) RemoveNetworkDevice(w http.ResponseWriter, r *http.Request) {
	input := servicepkg.RemoveNetworkDeviceInput{
		NetworkID:   requestNetworkID(r),
		DeviceID:    requestDeviceID(r),
		ActorUserID: requestActorUserID(r),
	}
	if err := h.NetworkInvite.RemoveNetworkDevice(r.Context(), input); err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteOK(w)
}
