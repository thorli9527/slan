package app

import (
	"net/http"
	"strings"
	"time"

	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type NetworkRuntimeHandler struct {
	NetworkRuntime    servicepkg.NetworkRuntimeUseCase
	DeviceSessions    servicepkg.DeviceSessionUseCase
	NetworkConfigView servicepkg.NetworkCoreUseCase
}

func (h NetworkRuntimeHandler) Routes() []serviceapi.Route {
	return []serviceapi.Route{
		serviceapi.NewRoute(http.MethodGet, "/api/app/runtime/endpoints", h.RuntimeEndpoints),
		serviceapi.NewRoute(http.MethodGet, "/api/app/networks/{networkId}/relay-candidates", h.RelayCandidates),
		serviceapi.NewRoute(http.MethodPost, "/api/app/networks/{networkId}/relay-candidates", h.RelayCandidates),
		serviceapi.NewRoute(http.MethodPost, "/api/app/networks/{networkId}/punch/connect-sessions", h.CreatePunchConnectSession),
		serviceapi.NewRoute(http.MethodPost, "/api/app/relay/tickets", h.IssueRelayTicket),
	}
}

func (h NetworkRuntimeHandler) RuntimeEndpoints(w http.ResponseWriter, r *http.Request) {
	deviceID, ok := authenticatedDeviceID(w, r, h.DeviceSessions)
	if !ok {
		return
	}
	items, err := h.NetworkRuntime.ListPunchNodes(r.Context())
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	networkConfigs := buildDeviceNetworkConfigPayloads(r.Context(), h.NetworkConfigView, deviceID)
	serviceapi.WriteJSON(w, http.StatusOK, map[string]any{
		"nodeConfigs": runtimeNodeConfigs(items, networkConfigs),
		"refreshedAt": time.Now().UTC().Unix(),
	})
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
	deviceID, ok := authenticatedDeviceID(w, r, h.DeviceSessions, input.DeviceID)
	if !ok {
		return
	}
	input.DeviceID = deviceID
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
	deviceID, ok := authenticatedDeviceID(w, r, h.DeviceSessions)
	if !ok {
		return
	}
	if strings.TrimSpace(input.RequesterNodeID) != "node-"+deviceID {
		serviceapi.WriteError(w, servicepkg.ErrUnauthorized)
		return
	}
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
	input := req.toInput()
	deviceID, ok := authenticatedDeviceID(w, r, h.DeviceSessions)
	if !ok {
		return
	}
	if strings.TrimSpace(input.SrcNodeID) != "node-"+deviceID {
		serviceapi.WriteError(w, servicepkg.ErrUnauthorized)
		return
	}
	item, err := h.NetworkRuntime.IssueRelayTicket(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusCreated, relayTicketPayload(item))
}
