package ops

import (
	"net/http"

	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type NodeHandler struct {
	OpsNodes servicepkg.OpsNodeUseCase
}

func (h NodeHandler) Routes() []serviceapi.Route {
	return withOptAliases([]serviceapi.Route{
		serviceapi.NewRoute(http.MethodGet, "/api/ops/relay-nodes", h.OpsListRelayNodes),
		serviceapi.NewRoute(http.MethodPost, "/api/ops/relay-nodes", h.OpsCreateRelayNode),
		serviceapi.NewRoute(http.MethodPatch, "/api/ops/relay-nodes/{nodeId}", h.OpsUpdateRelayNode),
		serviceapi.NewRoute(http.MethodPatch, "/api/ops/relay-nodes/{nodeId}/status", h.OpsUpdateRelayNodeStatus),
		serviceapi.NewRoute(http.MethodDelete, "/api/ops/relay-nodes/{nodeId}", h.OpsDeleteRelayNode),
		serviceapi.NewRoute(http.MethodGet, "/api/ops/punch-nodes", h.OpsListPunchNodes),
		serviceapi.NewRoute(http.MethodPost, "/api/ops/punch-nodes", h.OpsCreatePunchNode),
		serviceapi.NewRoute(http.MethodPatch, "/api/ops/punch-nodes/{nodeId}", h.OpsUpdatePunchNode),
		serviceapi.NewRoute(http.MethodPatch, "/api/ops/punch-nodes/{nodeId}/status", h.OpsUpdatePunchNodeStatus),
		serviceapi.NewRoute(http.MethodDelete, "/api/ops/punch-nodes/{nodeId}", h.OpsDeletePunchNode),
	})
}

func (h NodeHandler) OpsListRelayNodes(w http.ResponseWriter, r *http.Request) {
	items, err := h.OpsNodes.ListRelayNodes(r.Context())
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteItems(w, serviceapi.MapPayloads(items, relayNodePayload))
}

func (h NodeHandler) OpsCreateRelayNode(w http.ResponseWriter, r *http.Request) {
	var req upsertNodeRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.toInput()
	item, err := h.OpsNodes.UpsertRelayNode(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusCreated, relayNodePayload(item))
}

func (h NodeHandler) OpsUpdateRelayNode(w http.ResponseWriter, r *http.Request) {
	var req upsertNodeRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.toInput()
	serviceapi.SetIfEmpty(&input.NodeID, requestNodeID(r))
	item, err := h.OpsNodes.UpsertRelayNode(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, relayNodePayload(item))
}

func (h NodeHandler) OpsUpdateRelayNodeStatus(w http.ResponseWriter, r *http.Request) {
	var req updateNodeStatusRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	item, err := h.OpsNodes.UpdateRelayNodeStatus(r.Context(), requestNodeID(r), req.normalizedStatus())
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, relayNodePayload(item))
}

func (h NodeHandler) OpsDeleteRelayNode(w http.ResponseWriter, r *http.Request) {
	if err := h.OpsNodes.DeleteRelayNode(r.Context(), requestNodeID(r)); err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteOK(w)
}

func (h NodeHandler) OpsListPunchNodes(w http.ResponseWriter, r *http.Request) {
	items, err := h.OpsNodes.ListPunchNodes(r.Context())
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteItems(w, serviceapi.MapPayloads(items, punchNodePayload))
}

func (h NodeHandler) OpsCreatePunchNode(w http.ResponseWriter, r *http.Request) {
	var req upsertNodeRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.toInput()
	item, err := h.OpsNodes.UpsertPunchNode(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusCreated, punchNodePayload(item))
}

func (h NodeHandler) OpsUpdatePunchNode(w http.ResponseWriter, r *http.Request) {
	var req upsertNodeRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.toInput()
	serviceapi.SetIfEmpty(&input.NodeID, requestNodeID(r))
	item, err := h.OpsNodes.UpsertPunchNode(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, punchNodePayload(item))
}

func (h NodeHandler) OpsUpdatePunchNodeStatus(w http.ResponseWriter, r *http.Request) {
	var req updateNodeStatusRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	item, err := h.OpsNodes.UpdatePunchNodeStatus(r.Context(), requestNodeID(r), req.normalizedStatus())
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, punchNodePayload(item))
}

func (h NodeHandler) OpsDeletePunchNode(w http.ResponseWriter, r *http.Request) {
	if err := h.OpsNodes.DeletePunchNode(r.Context(), requestNodeID(r)); err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteOK(w)
}
