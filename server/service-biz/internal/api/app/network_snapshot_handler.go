package app

import (
	"net/http"

	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type NetworkSnapshotHandler struct {
	Snapshots      servicepkg.NetworkSnapshotUseCase
	DeviceSessions servicepkg.DeviceSessionUseCase
}

func (h NetworkSnapshotHandler) Routes() []serviceapi.Route {
	return []serviceapi.Route{
		serviceapi.NewRoute(http.MethodGet, "/api/app/networks/{networkId}/snapshot", h.NetworkSnapshot),
	}
}

func (h NetworkSnapshotHandler) NetworkSnapshot(w http.ResponseWriter, r *http.Request) {
	deviceID, ok := authenticatedDeviceID(w, r, h.DeviceSessions, requestDeviceID(r))
	if !ok {
		return
	}
	item, err := h.Snapshots.NetworkSnapshot(r.Context(), requestNetworkID(r), deviceID)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, item)
}
