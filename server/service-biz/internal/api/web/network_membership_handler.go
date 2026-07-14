package web

import (
	"net/http"

	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type NetworkMembershipHandler struct {
	NetworkDevices servicepkg.NetworkDeviceQueryUseCase
}

func (h NetworkMembershipHandler) Routes() []serviceapi.Route {
	return []serviceapi.Route{
		serviceapi.NewRoute(http.MethodGet, "/api/networks/{networkId}/devices", h.ListNetworkDevices),
	}
}

func (h NetworkMembershipHandler) ListNetworkDevices(w http.ResponseWriter, r *http.Request) {
	items, err := h.NetworkDevices.ListNetworkDevices(r.Context(), requestNetworkID(r))
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	payloads := serviceapi.MapPayloads(items, networkDevicePayload)
	serviceapi.WriteItems(w, payloads)
}
