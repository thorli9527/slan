package app

import (
	"net/http"

	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type NetworkRuntimeHandler struct {
	NetworkRuntime servicepkg.NetworkRuntimeUseCase
}

func (h NetworkRuntimeHandler) Routes() []serviceapi.Route {
	return []serviceapi.Route{
		serviceapi.NewRoute(http.MethodGet, "/api/networks/{networkId}/relay-candidates", h.RelayCandidates),
		serviceapi.NewRoute(http.MethodPost, "/api/networks/{networkId}/relay-candidates", h.RelayCandidates),
		serviceapi.NewRoute(http.MethodPost, "/api/networks/{networkId}/punch/connect-sessions", h.CreatePunchConnectSession),
		serviceapi.NewRoute(http.MethodPost, "/api/relay/tickets", h.IssueRelayTicket),
	}
}

func (h NetworkRuntimeHandler) RelayCandidates(w http.ResponseWriter, r *http.Request) {
	input := servicepkg.RelayCandidatesInput{
		NetworkID: requestNetworkID(r),
		DeviceID:  requestDeviceID(r),
	}
	if r.Method == http.MethodPost {
		var req relayCandidatesRequest
		if !serviceapi.DecodeJSONIfPresentOrError(w, r, &req) {
			return
		}
		bodyInput := req.toInput()
		serviceapi.SetIfEmpty(&bodyInput.DeviceID, input.DeviceID)
		input = bodyInput
		input.NetworkID = requestNetworkID(r)
	}
	items, err := h.NetworkRuntime.RelayCandidates(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteItems(w, serviceapi.MapPayloads(items, relayCandidateViewPayload))
}

func (h NetworkRuntimeHandler) CreatePunchConnectSession(w http.ResponseWriter, r *http.Request) {
	var req createPunchConnectSessionRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.toInput()
	serviceapi.SetIfEmpty(&input.NetworkID, requestNetworkID(r))
	item, err := h.NetworkRuntime.CreatePunchConnectSession(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusCreated, punchConnectSessionPayload(item))
}

func (h NetworkRuntimeHandler) IssueRelayTicket(w http.ResponseWriter, r *http.Request) {
	var req issueRelayTicketRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	item, err := h.NetworkRuntime.IssueRelayTicket(r.Context(), req.toInput())
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusCreated, relayTicketPayload(item))
}
