package web

import (
	"net/http"

	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type NetworkCoreHandler struct {
	NetworkCore servicepkg.NetworkCoreUseCase
}

func (h NetworkCoreHandler) Routes() []serviceapi.Route {
	return []serviceapi.Route{
		serviceapi.NewRoute(http.MethodGet, "/api/users/{userId}/networks", h.ListNetworks),
		serviceapi.NewRoute(http.MethodGet, "/api/networks", h.ListNetworks),
		serviceapi.NewRoute(http.MethodPost, "/api/networks", h.CreateNetwork),
		serviceapi.NewRoute(http.MethodPatch, "/api/networks/{networkId}", h.UpdateNetwork),
		serviceapi.NewRoute(http.MethodDelete, "/api/networks/{networkId}", h.DeleteNetwork),
	}
}

func (h NetworkCoreHandler) ListNetworks(w http.ResponseWriter, r *http.Request) {
	items, err := h.NetworkCore.ListNetworks(r.Context(), requestOwnerID(r))
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteItems(w, serviceapi.MapPayloads(items, networkPayload))
}

func (h NetworkCoreHandler) CreateNetwork(w http.ResponseWriter, r *http.Request) {
	var req createNetworkRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.toInput()
	setOwnerAndActor(r, &input.OwnerID, &input.ActorUserID)
	item, err := h.NetworkCore.CreateNetwork(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusCreated, networkPayload(item))
}

func (h NetworkCoreHandler) UpdateNetwork(w http.ResponseWriter, r *http.Request) {
	var req updateNetworkRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.toInput()
	serviceapi.SetIfEmpty(&input.NetworkID, requestNetworkID(r))
	serviceapi.SetIfEmpty(&input.ActorUserID, firstNonEmpty(req.ActorUserID, requestActorUserID(r)))
	item, err := h.NetworkCore.UpdateNetwork(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, networkPayload(item))
}

func (h NetworkCoreHandler) DeleteNetwork(w http.ResponseWriter, r *http.Request) {
	input := servicepkg.DeleteNetworkInput{
		NetworkID:   requestNetworkID(r),
		ActorUserID: requestActorUserID(r),
	}
	if err := h.NetworkCore.DeleteNetwork(r.Context(), input); err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteOK(w)
}
