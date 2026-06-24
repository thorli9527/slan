package wire

import (
	"net/http"

	serviceapi "github.com/slan/service-biz/internal/api"
	"github.com/slan/service-biz/internal/pkg/wirekit"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type NodeHandler struct {
	Wire servicepkg.WireUseCase
}

type nodeRequest struct {
	RegionID          string                   `json:"regionId"`
	NodeID            string                   `json:"nodeId"`
	Name              string                   `json:"name,omitempty"`
	Host              string                   `json:"host"`
	UDPPort           int                      `json:"udpPort,omitempty"`
	AdminPort         int                      `json:"adminPort,omitempty"`
	Port              int                      `json:"port,omitempty"`
	Priority          int                      `json:"priority,omitempty"`
	TicketKeyRotation *wirekit.TicketKeyStatus `json:"ticketKeyRotation,omitempty"`
	Enabled           *bool                    `json:"enabled,omitempty"`
	Healthy           *bool                    `json:"healthy,omitempty"`
}

type nodeHeartbeatRequest struct {
	Healthy           bool                     `json:"healthy"`
	TicketKeyRotation *wirekit.TicketKeyStatus `json:"ticketKeyRotation,omitempty"`
}

type nodeStatusRequest struct {
	Enabled *bool `json:"enabled,omitempty"`
	Healthy *bool `json:"healthy,omitempty"`
}

func (h NodeHandler) Routes() []serviceapi.Route {
	return []serviceapi.Route{
		serviceapi.NewRoute(http.MethodGet, "/internal/wire/admin/relay-nodes", h.ListRelayNodes),
		serviceapi.NewRoute(http.MethodPut, "/internal/wire/admin/relay-nodes", h.UpsertRelayNode),
		serviceapi.NewRoute(http.MethodPost, "/internal/wire/admin/relay-nodes/{regionId}/{nodeId}/heartbeat", h.HeartbeatRelayNode),
		serviceapi.NewRoute(http.MethodPatch, "/internal/wire/admin/relay-nodes/{regionId}/{nodeId}/status", h.UpdateRelayNodeStatus),
		serviceapi.NewRoute(http.MethodDelete, "/internal/wire/admin/relay-nodes/{regionId}/{nodeId}", h.DeleteRelayNode),
		serviceapi.NewRoute(http.MethodGet, "/internal/wire/admin/derp-nodes", h.ListDerpNodes),
		serviceapi.NewRoute(http.MethodPut, "/internal/wire/admin/derp-nodes", h.UpsertDerpNode),
		serviceapi.NewRoute(http.MethodPost, "/internal/wire/admin/derp-nodes/{regionId}/{nodeId}/heartbeat", h.HeartbeatDerpNode),
		serviceapi.NewRoute(http.MethodPatch, "/internal/wire/admin/derp-nodes/{regionId}/{nodeId}/status", h.UpdateDerpNodeStatus),
		serviceapi.NewRoute(http.MethodDelete, "/internal/wire/admin/derp-nodes/{regionId}/{nodeId}", h.DeleteDerpNode),
		serviceapi.NewRoute(http.MethodGet, "/internal/wire/derp-map", h.GetDerpMap),
	}
}

func Routes(useCase servicepkg.WireUseCase) []serviceapi.Route {
	return serviceapi.CombineRoutes(
		NodeHandler{Wire: useCase}.Routes(),
		PeerHandler{Wire: useCase}.Routes(),
	)
}

func (h NodeHandler) ListRelayNodes(w http.ResponseWriter, r *http.Request) {
	if !authorizeOrError(w, r, h.Wire) {
		return
	}
	items, err := h.Wire.ListRelayNodes(r.Context())
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteItems(w, serviceapi.MapPayloads(items, wireNodePayload))
}

func (h NodeHandler) ListDerpNodes(w http.ResponseWriter, r *http.Request) {
	if !authorizeOrError(w, r, h.Wire) {
		return
	}
	items, err := h.Wire.ListDerpNodes(r.Context())
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteItems(w, serviceapi.MapPayloads(items, wireNodePayload))
}

func (h NodeHandler) UpsertRelayNode(w http.ResponseWriter, r *http.Request) {
	if !authorizeOrError(w, r, h.Wire) {
		return
	}
	var req nodeRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	item, err := h.Wire.UpsertRelayNode(r.Context(), servicepkg.WireUpsertNodeInput{
		RegionID:          req.RegionID,
		NodeID:            req.NodeID,
		Name:              req.Name,
		Host:              req.Host,
		UDPPort:           req.UDPPort,
		AdminPort:         req.AdminPort,
		Priority:          req.Priority,
		TicketKeyRotation: req.TicketKeyRotation,
		Enabled:           req.Enabled,
		Healthy:           req.Healthy,
	})
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, wireNodePayload(item))
}

func (h NodeHandler) UpsertDerpNode(w http.ResponseWriter, r *http.Request) {
	if !authorizeOrError(w, r, h.Wire) {
		return
	}
	var req nodeRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	item, err := h.Wire.UpsertDerpNode(r.Context(), servicepkg.WireUpsertNodeInput{
		RegionID:          req.RegionID,
		NodeID:            req.NodeID,
		Name:              req.Name,
		Host:              req.Host,
		Port:              req.Port,
		Priority:          req.Priority,
		TicketKeyRotation: req.TicketKeyRotation,
		Enabled:           req.Enabled,
		Healthy:           req.Healthy,
	})
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, wireNodePayload(item))
}

func (h NodeHandler) HeartbeatRelayNode(w http.ResponseWriter, r *http.Request) {
	if !authorizeOrError(w, r, h.Wire) {
		return
	}
	var req nodeHeartbeatRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	item, err := h.Wire.HeartbeatRelayNode(r.Context(), servicepkg.WireNodeStatusInput{
		RegionID:          r.PathValue("regionId"),
		NodeID:            r.PathValue("nodeId"),
		Healthy:           &req.Healthy,
		TicketKeyRotation: req.TicketKeyRotation,
	})
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, wireNodePayload(item))
}

func (h NodeHandler) HeartbeatDerpNode(w http.ResponseWriter, r *http.Request) {
	if !authorizeOrError(w, r, h.Wire) {
		return
	}
	var req nodeHeartbeatRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	item, err := h.Wire.HeartbeatDerpNode(r.Context(), servicepkg.WireNodeStatusInput{
		RegionID:          r.PathValue("regionId"),
		NodeID:            r.PathValue("nodeId"),
		Healthy:           &req.Healthy,
		TicketKeyRotation: req.TicketKeyRotation,
	})
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, wireNodePayload(item))
}

func (h NodeHandler) UpdateRelayNodeStatus(w http.ResponseWriter, r *http.Request) {
	if !authorizeOrError(w, r, h.Wire) {
		return
	}
	var req nodeStatusRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	item, err := h.Wire.UpdateRelayNodeStatus(r.Context(), servicepkg.WireNodeStatusInput{
		RegionID: r.PathValue("regionId"),
		NodeID:   r.PathValue("nodeId"),
		Enabled:  req.Enabled,
		Healthy:  req.Healthy,
	})
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, wireNodePayload(item))
}

func (h NodeHandler) UpdateDerpNodeStatus(w http.ResponseWriter, r *http.Request) {
	if !authorizeOrError(w, r, h.Wire) {
		return
	}
	var req nodeStatusRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	item, err := h.Wire.UpdateDerpNodeStatus(r.Context(), servicepkg.WireNodeStatusInput{
		RegionID: r.PathValue("regionId"),
		NodeID:   r.PathValue("nodeId"),
		Enabled:  req.Enabled,
		Healthy:  req.Healthy,
	})
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, wireNodePayload(item))
}

func (h NodeHandler) DeleteRelayNode(w http.ResponseWriter, r *http.Request) {
	if !authorizeOrError(w, r, h.Wire) {
		return
	}
	if err := h.Wire.DeleteRelayNode(r.Context(), servicepkg.WireNodeDeleteInput{
		RegionID: r.PathValue("regionId"),
		NodeID:   r.PathValue("nodeId"),
	}); err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteNoContent(w)
}

func (h NodeHandler) DeleteDerpNode(w http.ResponseWriter, r *http.Request) {
	if !authorizeOrError(w, r, h.Wire) {
		return
	}
	if err := h.Wire.DeleteDerpNode(r.Context(), servicepkg.WireNodeDeleteInput{
		RegionID: r.PathValue("regionId"),
		NodeID:   r.PathValue("nodeId"),
	}); err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteNoContent(w)
}

func (h NodeHandler) GetDerpMap(w http.ResponseWriter, r *http.Request) {
	if !authorizeOrError(w, r, h.Wire) {
		return
	}
	items, err := h.Wire.DerpMap(r.Context())
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, wireDerpMapPayload(items))
}
