package app

import (
	"net/http"

	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type NetworkSnapshotHandler struct {
	Snapshots servicepkg.NetworkSnapshotUseCase
}

func (h NetworkSnapshotHandler) Routes() []serviceapi.Route {
	return []serviceapi.Route{
		serviceapi.NewRoute(http.MethodGet, "/api/app/networks/{networkId}/snapshot", h.NetworkSnapshot),
	}
}

func (h NetworkSnapshotHandler) NetworkSnapshot(w http.ResponseWriter, r *http.Request) {
	item, err := h.Snapshots.NetworkSnapshot(r.Context(), requestNetworkID(r), requestDeviceID(r))
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, item)
}
