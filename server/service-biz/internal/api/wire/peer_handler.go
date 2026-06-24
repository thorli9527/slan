package wire

import (
	"net/http"

	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type PeerHandler struct {
	Wire servicepkg.WireUseCase
}

func (h PeerHandler) Routes() []serviceapi.Route {
	return []serviceapi.Route{
		serviceapi.NewRoute(http.MethodGet, "/internal/wire/peers/{peerId}/authz", h.GetPeerAuthz),
		serviceapi.NewRoute(http.MethodGet, "/internal/wire/peers/{peerId}/runtime-config", h.GetPeerRuntimeConfig),
		serviceapi.NewRoute(http.MethodGet, "/internal/wire/networks/{networkId}/topology", h.GetNetworkTopology),
		serviceapi.NewRoute(http.MethodPost, "/internal/wire/peers/path-health", h.ReportPeerPathHealth),
	}
}

func (h PeerHandler) GetPeerAuthz(w http.ResponseWriter, r *http.Request) {
	if !authorizeOrError(w, r, h.Wire) {
		return
	}
	view, err := h.Wire.PeerAuthz(r.Context(), r.PathValue("peerId"))
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, wirePeerAuthzPayload(view))
}

func (h PeerHandler) GetPeerRuntimeConfig(w http.ResponseWriter, r *http.Request) {
	if !authorizeOrError(w, r, h.Wire) {
		return
	}
	view, err := h.Wire.PeerRuntimeConfig(r.Context(), r.PathValue("peerId"))
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, wirePeerRuntimeConfigPayload(view))
}

func (h PeerHandler) GetNetworkTopology(w http.ResponseWriter, r *http.Request) {
	if !authorizeOrError(w, r, h.Wire) {
		return
	}
	view, err := h.Wire.NetworkTopology(r.Context(), r.PathValue("networkId"))
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, wireTopologyPayload(view))
}

func (h PeerHandler) ReportPeerPathHealth(w http.ResponseWriter, r *http.Request) {
	if !authorizeOrError(w, r, h.Wire) {
		return
	}
	var req struct {
		PeerID string                          `json:"peerId"`
		Probes []servicepkg.WirePathProbeInput `json:"probes"`
	}
	if err := serviceapi.DecodeJSON(r, &req); err != nil {
		serviceapi.WriteError(w, servicepkg.ErrInvalidArgument)
		return
	}
	if err := h.Wire.ReportPeerPathHealth(r.Context(), servicepkg.WirePeerPathHealthInput{
		PeerID: req.PeerID,
		Probes: req.Probes,
	}); err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusAccepted, wirePeerPathHealthAcceptedPayload(req.PeerID))
}
