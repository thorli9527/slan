package app

import (
	"net/http"

	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type NetworkConfigHandler struct {
	NetworkCore    servicepkg.NetworkCoreUseCase
	DeviceSessions servicepkg.DeviceSessionUseCase
}

func (h NetworkConfigHandler) Routes() []serviceapi.Route {
	return []serviceapi.Route{
		serviceapi.NewRoute(http.MethodGet, "/api/app/networks/{networkId}/network-config", h.NetworkConfig),
	}
}

func (h NetworkConfigHandler) NetworkConfig(w http.ResponseWriter, r *http.Request) {
	deviceID, ok := authenticatedDeviceID(w, r, h.DeviceSessions, requestDeviceID(r))
	if !ok {
		return
	}
	view, err := h.NetworkCore.ResolvedNetworkConfig(r.Context(), requestNetworkID(r), deviceID)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, networkResolvedConfigPayload(view))
}
