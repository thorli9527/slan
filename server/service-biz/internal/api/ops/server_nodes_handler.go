package ops

import (
	"context"
	"net/http"
	"time"

	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type ServerNodeHandler struct {
	ServerNodes servicepkg.OpsServerNodeUseCase
}

func (h ServerNodeHandler) Routes() []serviceapi.Route {
	return withOptAliases([]serviceapi.Route{
		serviceapi.NewRoute(http.MethodGet, "/api/ops/server-nodes", h.List),
		serviceapi.NewRoute(http.MethodPost, "/api/ops/server-nodes", h.Create),
		serviceapi.NewRoute(http.MethodPatch, "/api/ops/server-nodes/{nodeId}", h.Update),
		serviceapi.NewRoute(http.MethodPost, "/api/ops/server-nodes/{nodeId}/ssh-host-key", h.InspectHostKey),
		serviceapi.NewRoute(http.MethodPost, "/api/ops/server-nodes/{nodeId}/deploy", h.Deploy),
		serviceapi.NewRoute(http.MethodDelete, "/api/ops/server-nodes/{nodeId}", h.Delete),
	})
}

func (h ServerNodeHandler) List(w http.ResponseWriter, r *http.Request) {
	items, err := h.ServerNodes.ListServerNodes(r.Context())
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteItems(w, items)
}

func (h ServerNodeHandler) Create(w http.ResponseWriter, r *http.Request) {
	h.upsert(w, r, http.StatusCreated, "")
}

func (h ServerNodeHandler) Update(w http.ResponseWriter, r *http.Request) {
	h.upsert(w, r, http.StatusOK, requestNodeID(r))
}

func (h ServerNodeHandler) upsert(w http.ResponseWriter, r *http.Request, status int, nodeID string) {
	var req upsertServerNodeRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.toInput()
	if input.NodeID == "" {
		input.NodeID = nodeID
	}
	item, err := h.ServerNodes.UpsertServerNode(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, status, item)
}

func (h ServerNodeHandler) InspectHostKey(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	item, err := h.ServerNodes.InspectServerNodeHostKey(ctx, requestNodeID(r))
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, item)
}

func (h ServerNodeHandler) Deploy(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SSHHostKeyFingerprint string `json:"sshHostKeyFingerprint"`
	}
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Minute)
	defer cancel()
	item, err := h.ServerNodes.DeployServerNode(ctx, requestNodeID(r), req.SSHHostKeyFingerprint)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, item)
}

func (h ServerNodeHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.ServerNodes.DeleteServerNode(r.Context(), requestNodeID(r)); err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteOK(w)
}

type upsertServerNodeRequest struct {
	NodeID         string `json:"nodeId"`
	Name           string `json:"name"`
	Host           string `json:"host"`
	SSHPort        int    `json:"sshPort"`
	SSHUsername    string `json:"sshUsername"`
	SSHPassword    string `json:"sshPassword"`
	RelayUDPPort   int    `json:"relayUdpPort"`
	RelayAdminPort int    `json:"relayAdminPort"`
	RelayTCPPort   int    `json:"relayTcpPort"`
	PunchUDPPort   int    `json:"punchUdpPort"`
	PunchHTTPPort  int    `json:"punchHttpPort"`
	APIProxyPort   int    `json:"apiProxyPort"`
	MQTTProxyPort  int    `json:"mqttProxyPort"`
	RelayEnabled   bool   `json:"relayEnabled"`
	PunchEnabled   bool   `json:"punchEnabled"`
	ProxyEnabled   bool   `json:"proxyEnabled"`
}

func (r upsertServerNodeRequest) toInput() servicepkg.UpsertServerNodeInput {
	return servicepkg.UpsertServerNodeInput{NodeID: r.NodeID, Name: r.Name, Host: r.Host, SSHPort: r.SSHPort, SSHUsername: r.SSHUsername, SSHPassword: r.SSHPassword, RelayUDPPort: r.RelayUDPPort, RelayAdminPort: r.RelayAdminPort, RelayTCPPort: r.RelayTCPPort, PunchUDPPort: r.PunchUDPPort, PunchHTTPPort: r.PunchHTTPPort, APIProxyPort: r.APIProxyPort, MQTTProxyPort: r.MQTTProxyPort, RelayEnabled: r.RelayEnabled, PunchEnabled: r.PunchEnabled, ProxyEnabled: r.ProxyEnabled}
}
