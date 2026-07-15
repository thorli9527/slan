package web

import (
	"net/http"

	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type NetworkInviteHandler struct {
	NetworkInvite servicepkg.NetworkInviteUseCase
}

func (h NetworkInviteHandler) Routes() []serviceapi.Route {
	return []serviceapi.Route{
		serviceapi.NewRoute(http.MethodGet, "/api/users/{userId}/device-invites", h.ListDeviceInvites),
		serviceapi.NewRoute(http.MethodPost, "/api/device-invites", h.CreateDeviceInvite),
		serviceapi.NewRoute(http.MethodGet, "/api/device-invites", h.ListDeviceInvites),
		serviceapi.NewRoute(http.MethodPost, "/api/device-invites/accept", h.AcceptDeviceInvite),
		serviceapi.NewRoute(http.MethodPost, "/api/device-invites/{inviteId}/revoke", h.RevokeDeviceInvite),
	}
}

func (h NetworkInviteHandler) CreateDeviceInvite(w http.ResponseWriter, r *http.Request) {
	var req createDeviceInviteRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.toInput()
	item, err := h.NetworkInvite.CreateDeviceInvite(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusCreated, deviceInvitePayload(item))
}

func (h NetworkInviteHandler) ListDeviceInvites(w http.ResponseWriter, r *http.Request) {
	items, err := h.NetworkInvite.ListDeviceInvites(r.Context(), requestUserID(r), requestNetworkID(r))
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	payloads := serviceapi.MapPayloads(items, deviceInvitePayload)
	serviceapi.WriteItems(w, payloads)
}

func (h NetworkInviteHandler) AcceptDeviceInvite(w http.ResponseWriter, r *http.Request) {
	var req acceptDeviceInviteRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.toInput()
	setActorUserID(r, &input.ActorUserID)
	item, err := h.NetworkInvite.AcceptDeviceInvite(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteEnvelope(w, http.StatusOK, "invite", deviceInvitePayload(item))
}

func (h NetworkInviteHandler) RevokeDeviceInvite(w http.ResponseWriter, r *http.Request) {
	var req revokeDeviceInviteRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.toInput()
	input.InviteID = requestInviteID(r)
	setActorUserID(r, &input.ActorUserID)
	item, err := h.NetworkInvite.RevokeDeviceInvite(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteEnvelope(w, http.StatusOK, "invite", deviceInvitePayload(item))
}
