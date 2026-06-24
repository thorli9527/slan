package web

import (
	"net/http"

	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type NetworkMappingHandler struct {
	NetworkAccess servicepkg.NetworkAccessUseCase
}

func (h NetworkMappingHandler) Routes() []serviceapi.Route {
	return []serviceapi.Route{
		serviceapi.NewRoute(http.MethodGet, "/api/networks/{networkId}/public-mappings", h.ListPublicMappings),
		serviceapi.NewRoute(http.MethodPost, "/api/networks/{networkId}/public-mappings", h.CreatePublicMapping),
		serviceapi.NewRoute(http.MethodPatch, "/api/networks/{networkId}/public-mappings/{mappingId}", h.UpdatePublicMapping),
		serviceapi.NewRoute(http.MethodDelete, "/api/networks/{networkId}/public-mappings/{mappingId}", h.DeletePublicMapping),
	}
}

func (h NetworkMappingHandler) ListPublicMappings(w http.ResponseWriter, r *http.Request) {
	items, err := h.NetworkAccess.ListPublicMappings(r.Context(), requestNetworkID(r))
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteItems(w, serviceapi.MapPayloads(items, publicMappingPayload))
}

func (h NetworkMappingHandler) CreatePublicMapping(w http.ResponseWriter, r *http.Request) {
	var req createPublicMappingRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.toInput()
	setNetworkAndActor(r, &input.NetworkID, &input.ActorUserID)
	item, err := h.NetworkAccess.CreatePublicMapping(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusCreated, publicMappingPayload(item))
}

func (h NetworkMappingHandler) UpdatePublicMapping(w http.ResponseWriter, r *http.Request) {
	var req updatePublicMappingRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.toInput()
	setMappingAndActor(r, &input.MappingID, &input.ActorUserID)
	item, err := h.NetworkAccess.UpdatePublicMapping(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, publicMappingPayload(item))
}

func (h NetworkMappingHandler) DeletePublicMapping(w http.ResponseWriter, r *http.Request) {
	input := servicepkg.DeletePublicMappingInput{
		MappingID:   requestMappingID(r),
		ActorUserID: requestActorUserID(r),
	}
	if err := h.NetworkAccess.DeletePublicMapping(r.Context(), input); err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteOK(w)
}
