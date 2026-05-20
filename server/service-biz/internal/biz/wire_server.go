package biz

import (
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type wireTicketKeyStatus struct {
	Source             string `json:"source,omitempty"`
	KeyRingID          string `json:"keyRingId,omitempty"`
	SigningConfigured  bool   `json:"signingConfigured"`
	KeyRingConfigured  bool   `json:"keyRingConfigured"`
	EffectiveKeyCount  int    `json:"effectiveKeyCount,omitempty"`
	RotationReady      bool   `json:"rotationReady"`
	AcceptsDevFallback bool   `json:"acceptsDevFallback,omitempty"`
}

type wireRelayNodeRequest struct {
	RegionID          string              `json:"regionId"`
	NodeID            string              `json:"nodeId"`
	Host              string              `json:"host"`
	UDPPort           int                 `json:"udpPort"`
	AdminPort         int                 `json:"adminPort,omitempty"`
	Enabled           *bool               `json:"enabled,omitempty"`
	Healthy           *bool               `json:"healthy,omitempty"`
	Priority          int                 `json:"priority,omitempty"`
	TicketKeyRotation wireTicketKeyStatus `json:"ticketKeyRotation,omitempty"`
}

type wireDerpNodeRequest struct {
	RegionID          string              `json:"regionId"`
	NodeID            string              `json:"nodeId"`
	Name              string              `json:"name,omitempty"`
	Host              string              `json:"host"`
	Port              int                 `json:"port,omitempty"`
	Enabled           *bool               `json:"enabled,omitempty"`
	Healthy           *bool               `json:"healthy,omitempty"`
	Priority          int                 `json:"priority,omitempty"`
	TicketKeyRotation wireTicketKeyStatus `json:"ticketKeyRotation,omitempty"`
}

type wireNodeHeartbeatRequest struct {
	Healthy           bool                `json:"healthy"`
	TicketKeyRotation wireTicketKeyStatus `json:"ticketKeyRotation,omitempty"`
}

type wireNodeStatusRequest struct {
	Enabled *bool `json:"enabled,omitempty"`
	Healthy *bool `json:"healthy,omitempty"`
}

type wireEndpoint struct {
	Kind       string `json:"kind"`
	Address    string `json:"address"`
	Port       int    `json:"port,omitempty"`
	Reachable  bool   `json:"reachable,omitempty"`
	ObservedAt int64  `json:"observedAt,omitempty"`
}

type wirePeerRecord struct {
	PeerID                  string         `json:"peerId"`
	NetworkID               string         `json:"networkId,omitempty"`
	NodeID                  string         `json:"nodeId,omitempty"`
	PublicKey               string         `json:"publicKey,omitempty"`
	VirtualIPs              []string       `json:"virtualIps,omitempty"`
	AllowedIPs              []string       `json:"allowedIps,omitempty"`
	SupportsLanDirect       bool           `json:"supportsLanDirect"`
	SupportsIpv6Direct      bool           `json:"supportsIpv6Direct"`
	SupportsDirectUdp       bool           `json:"supportsDirectUdp"`
	SupportsRelayUdp        bool           `json:"supportsRelayUdp"`
	SupportsDerpTcpTls443   bool           `json:"supportsDerpTcpTls443"`
	PreferIpv6              bool           `json:"preferIpv6"`
	PreferLan               bool           `json:"preferLan"`
	AllowEndpointRoaming    bool           `json:"allowEndpointRoaming"`
	AllowFastReselection    bool           `json:"allowFastReselection"`
	AllowRelayTicketRenewal bool           `json:"allowRelayTicketRenewal"`
	KeepaliveIntervalSecs   int            `json:"keepaliveIntervalSecs,omitempty"`
	Endpoints               []wireEndpoint `json:"endpoints,omitempty"`
	ActivePath              string         `json:"activePath,omitempty"`
	RequireMtuRefresh       bool           `json:"requireMtuRefresh"`
	EndpointChanged         bool           `json:"endpointChanged"`
	UpdatedAt               int64          `json:"updatedAt"`
}

type wirePeerContext struct {
	networkDevice NetworkDevice
	device        Device
	runtime       DeviceRuntimeStatus
}

func (s *Server) registerInternalWireRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /internal/wire/peers/{peerId}/authz", s.internalWirePeerAuthz)
	mux.HandleFunc("GET /internal/wire/peers/{peerId}/runtime-config", s.internalWirePeerRuntimeConfig)
	mux.HandleFunc("GET /internal/wire/networks/{networkId}/topology", s.internalWireNetworkTopology)
	mux.HandleFunc("GET /internal/wire/admin/relay-nodes", s.internalWireRelayNodes)
	mux.HandleFunc("PUT /internal/wire/admin/relay-nodes", s.internalWireUpsertRelayNode)
	mux.HandleFunc("POST /internal/wire/admin/relay-nodes/{regionId}/{nodeId}/heartbeat", s.internalWireRelayNodeHeartbeat)
	mux.HandleFunc("PATCH /internal/wire/admin/relay-nodes/{regionId}/{nodeId}/status", s.internalWireRelayNodeStatus)
	mux.HandleFunc("GET /internal/wire/admin/derp-nodes", s.internalWireDerpNodes)
	mux.HandleFunc("PUT /internal/wire/admin/derp-nodes", s.internalWireUpsertDerpNode)
	mux.HandleFunc("POST /internal/wire/admin/derp-nodes/{regionId}/{nodeId}/heartbeat", s.internalWireDerpNodeHeartbeat)
	mux.HandleFunc("PATCH /internal/wire/admin/derp-nodes/{regionId}/{nodeId}/status", s.internalWireDerpNodeStatus)
	mux.HandleFunc("GET /internal/wire/derp-map", s.internalWireDerpMap)
}

func requireInternalWireToken(w http.ResponseWriter, r *http.Request) bool {
	expected := strings.TrimSpace(os.Getenv("SLAN_INTERNAL_WIRE_TOKEN"))
	if expected == "" || strings.TrimSpace(r.Header.Get("X-Slan-Internal-Token")) != expected {
		writeError(w, errUnauthorized)
		return false
	}
	return true
}

func (s *Server) internalWireRelayNodes(w http.ResponseWriter, r *http.Request) {
	if !requireInternalWireToken(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": wireRelayNodeViews(s.store.ListRelayNodes())})
}

func (s *Server) internalWirePeerAuthz(w http.ResponseWriter, r *http.Request) {
	if !requireInternalWireToken(w, r) {
		return
	}
	view, err := s.store.WirePeerAuthz(r.PathValue("peerId"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) internalWirePeerRuntimeConfig(w http.ResponseWriter, r *http.Request) {
	if !requireInternalWireToken(w, r) {
		return
	}
	view, err := s.store.WirePeerRuntimeConfig(r.PathValue("peerId"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) internalWireNetworkTopology(w http.ResponseWriter, r *http.Request) {
	if !requireInternalWireToken(w, r) {
		return
	}
	view, err := s.store.WireNetworkTopology(r.PathValue("networkId"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) internalWireUpsertRelayNode(w http.ResponseWriter, r *http.Request) {
	if !requireInternalWireToken(w, r) {
		return
	}
	var req wireRelayNodeRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	node, err := s.store.UpsertWireNode(req.relayNode())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, wireRelayNodeView(node))
}

func (s *Server) internalWireRelayNodeHeartbeat(w http.ResponseWriter, r *http.Request) {
	if !requireInternalWireToken(w, r) {
		return
	}
	var req wireNodeHeartbeatRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	node, err := s.store.UpdateWireNodeHeartbeat(r.PathValue("nodeId"), "relay_udp", req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, wireRelayNodeView(node))
}

func (s *Server) internalWireRelayNodeStatus(w http.ResponseWriter, r *http.Request) {
	if !requireInternalWireToken(w, r) {
		return
	}
	var req wireNodeStatusRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	node, err := s.store.UpdateWireNodeStatus(r.PathValue("nodeId"), "relay_udp", req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, wireRelayNodeView(node))
}

func (s *Server) internalWireDerpNodes(w http.ResponseWriter, r *http.Request) {
	if !requireInternalWireToken(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": wireDerpNodeViews(s.store.ListRelayNodes())})
}

func (s *Server) internalWireUpsertDerpNode(w http.ResponseWriter, r *http.Request) {
	if !requireInternalWireToken(w, r) {
		return
	}
	var req wireDerpNodeRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	node, err := s.store.UpsertWireNode(req.derpNode())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, wireDerpNodeView(node))
}

func (s *Server) internalWireDerpNodeHeartbeat(w http.ResponseWriter, r *http.Request) {
	if !requireInternalWireToken(w, r) {
		return
	}
	var req wireNodeHeartbeatRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	node, err := s.store.UpdateWireNodeHeartbeat(r.PathValue("nodeId"), "derp_tcp_tls_443", req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, wireDerpNodeView(node))
}

func (s *Server) internalWireDerpNodeStatus(w http.ResponseWriter, r *http.Request) {
	if !requireInternalWireToken(w, r) {
		return
	}
	var req wireNodeStatusRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	node, err := s.store.UpdateWireNodeStatus(r.PathValue("nodeId"), "derp_tcp_tls_443", req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, wireDerpNodeView(node))
}

func (s *Server) internalWireDerpMap(w http.ResponseWriter, r *http.Request) {
	if !requireInternalWireToken(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, derpMapFromRelayNodes(s.store.ListRelayNodes()))
}

func (req wireRelayNodeRequest) relayNode() OpsRelayNode {
	status := "active"
	if req.Enabled != nil && !*req.Enabled {
		status = "disabled"
	}
	health := "healthy"
	if req.Healthy != nil && !*req.Healthy {
		health = "down"
	}
	return OpsRelayNode{
		NodeID:            strings.TrimSpace(req.NodeID),
		Name:              defaultString(strings.TrimSpace(req.NodeID), "Relay Node"),
		Region:            defaultString(req.RegionID, "default"),
		Transport:         "relay_udp",
		PublicAddr:        hostPort(req.Host, req.UDPPort),
		InternalAddr:      hostPort(req.Host, req.AdminPort),
		MaxBandwidthMbps:  1000,
		MonthlyTrafficGB:  10240,
		MaxSessions:       10000,
		Status:            status,
		Health:            health,
		Priority:          defaultInt(req.Priority, 100),
		TicketKeyRotation: req.TicketKeyRotation,
	}
}

func (req wireDerpNodeRequest) derpNode() OpsRelayNode {
	status := "active"
	if req.Enabled != nil && !*req.Enabled {
		status = "disabled"
	}
	health := "healthy"
	if req.Healthy != nil && !*req.Healthy {
		health = "down"
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = strings.TrimSpace(req.NodeID)
	}
	return OpsRelayNode{
		NodeID:            strings.TrimSpace(req.NodeID),
		Name:              defaultString(name, "DERP Node"),
		Region:            defaultString(req.RegionID, "default"),
		Transport:         "derp_tcp_tls_443",
		PublicAddr:        hostPort(req.Host, req.Port),
		MaxBandwidthMbps:  1000,
		MonthlyTrafficGB:  10240,
		MaxSessions:       10000,
		Status:            status,
		Health:            health,
		Priority:          defaultInt(req.Priority, 100),
		TicketKeyRotation: req.TicketKeyRotation,
	}
}

func wireRelayNodeViews(nodes []OpsRelayNode) []map[string]any {
	out := make([]map[string]any, 0)
	for _, node := range nodes {
		if node.Transport != "relay_udp" {
			continue
		}
		out = append(out, wireRelayNodeView(node))
	}
	return out
}

func wireRelayNodeView(node OpsRelayNode) map[string]any {
	host, port := splitHostPort(node.PublicAddr)
	_, adminPort := splitHostPort(node.InternalAddr)
	stale := wireNodeStale(node, time.Now().Unix())
	return map[string]any{
		"regionId":          node.Region,
		"nodeId":            node.NodeID,
		"host":              host,
		"udpPort":           port,
		"adminPort":         adminPort,
		"enabled":           node.Status == "active",
		"healthy":           node.Health == "healthy",
		"stale":             stale,
		"priority":          defaultInt(node.Priority, 100),
		"updatedAtMs":       node.UpdatedAt * 1000,
		"ticketKeyRotation": node.TicketKeyRotation,
	}
}

func wireDerpNodeViews(nodes []OpsRelayNode) []map[string]any {
	out := make([]map[string]any, 0)
	for _, node := range nodes {
		if node.Transport != "derp_tcp_tls_443" {
			continue
		}
		out = append(out, wireDerpNodeView(node))
	}
	return out
}

func wireDerpNodeView(node OpsRelayNode) map[string]any {
	host, port := splitHostPort(node.PublicAddr)
	stale := wireNodeStale(node, time.Now().Unix())
	return map[string]any{
		"regionId":          node.Region,
		"nodeId":            node.NodeID,
		"name":              node.Name,
		"host":              host,
		"port":              port,
		"enabled":           node.Status == "active",
		"healthy":           node.Health == "healthy",
		"stale":             stale,
		"priority":          defaultInt(node.Priority, 100),
		"updatedAtMs":       node.UpdatedAt * 1000,
		"ticketKeyRotation": node.TicketKeyRotation,
	}
}

func derpMapFromRelayNodes(nodes []OpsRelayNode) map[string]any {
	regions := make(map[string][]map[string]any)
	preferred := ""
	now := time.Now().Unix()
	for _, node := range nodes {
		if node.Transport != "derp_tcp_tls_443" || node.Status != "active" || node.Health == "down" || wireNodeStale(node, now) {
			continue
		}
		host, port := splitHostPort(node.PublicAddr)
		if preferred == "" {
			preferred = node.Region
		}
		regions[node.Region] = append(regions[node.Region], map[string]any{
			"regionId": node.Region,
			"nodeId":   node.NodeID,
			"host":     host,
			"port":     port,
		})
	}
	items := make([]map[string]any, 0, len(regions))
	for regionID, nodes := range regions {
		items = append(items, map[string]any{
			"regionId": regionID,
			"name":     regionID,
			"nodes":    nodes,
		})
	}
	return map[string]any{"preferredRegionId": preferred, "regions": items}
}

func wireNodeStale(node OpsRelayNode, now int64) bool {
	if node.UpdatedAt <= 0 {
		return false
	}
	freshness := int64(120)
	if value, err := strconv.ParseInt(strings.TrimSpace(os.Getenv("SLAN_WIRE_NODE_HEARTBEAT_FRESHNESS_SECONDS")), 10, 64); err == nil && value > 0 {
		freshness = value
	}
	return now-node.UpdatedAt > freshness
}

func hostPort(host string, port int) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	if port <= 0 {
		return host
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
}

func splitHostPort(value string) (string, int) {
	value = strings.TrimSpace(strings.TrimPrefix(value, "udp://"))
	if value == "" {
		return "", 0
	}
	host, portText, err := net.SplitHostPort(value)
	if err != nil {
		return value, 0
	}
	port, _ := strconv.Atoi(portText)
	return host, port
}

func (s *Store) UpdateWireNodeHeartbeat(nodeID, transport string, req wireNodeHeartbeatRequest) (OpsRelayNode, error) {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return OpsRelayNode{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	node, ok := s.relayNodes[nodeID]
	if !ok {
		return OpsRelayNode{}, errNotFound
	}
	if node.Transport != transport {
		return OpsRelayNode{}, errNotFound
	}
	if req.Healthy {
		node.Health = "healthy"
	} else {
		node.Health = "down"
	}
	node.TicketKeyRotation = req.TicketKeyRotation
	node.UpdatedAt = time.Now().Unix()
	s.relayNodes[nodeID] = node
	return node, nil
}

func (s *Store) UpdateWireNodeStatus(nodeID, transport string, req wireNodeStatusRequest) (OpsRelayNode, error) {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" || (req.Enabled == nil && req.Healthy == nil) {
		return OpsRelayNode{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	node, ok := s.relayNodes[nodeID]
	if !ok {
		return OpsRelayNode{}, errNotFound
	}
	if node.Transport != transport {
		return OpsRelayNode{}, errNotFound
	}
	if req.Enabled != nil {
		if *req.Enabled {
			node.Status = "active"
		} else {
			node.Status = "disabled"
		}
	}
	if req.Healthy != nil {
		if *req.Healthy {
			node.Health = "healthy"
		} else {
			node.Health = "down"
		}
	}
	node.UpdatedAt = time.Now().Unix()
	s.relayNodes[nodeID] = node
	return node, nil
}

func (s *Store) UpsertWireNode(node OpsRelayNode) (OpsRelayNode, error) {
	node.NodeID = strings.TrimSpace(node.NodeID)
	node.Name = strings.TrimSpace(node.Name)
	node.Region = defaultString(node.Region, "default")
	node.Transport = defaultString(node.Transport, "relay_udp")
	node.PublicAddr = strings.TrimSpace(node.PublicAddr)
	node.Status = defaultString(node.Status, "active")
	node.Health = defaultString(node.Health, "healthy")
	if node.NodeID == "" || node.Name == "" || node.PublicAddr == "" {
		return OpsRelayNode{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().Unix()
	if existing, ok := s.relayNodes[node.NodeID]; ok {
		node.CreatedAt = existing.CreatedAt
		node.UpdatedAt = now
	} else {
		node.CreatedAt = now
		node.UpdatedAt = now
	}
	s.relayNodes[node.NodeID] = node
	return node, nil
}

func (s *Store) WirePeerAuthz(peerID string) (map[string]any, error) {
	ctx, err := s.wirePeerLookup(peerID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"peerId":      peerID,
		"networkId":   ctx.networkDevice.NetworkID,
		"nodeId":      "node-" + ctx.device.DeviceID,
		"enabled":     ctx.networkDevice.Enabled && ctx.networkDevice.Status == "active" && ctx.device.Status == "active",
		"virtualIps":  []string{ctx.device.GlobalIP},
		"allowedIps":  []string{ctx.device.GlobalIP + "/32"},
		"quotaPolicy": "default",
	}, nil
}

func (s *Store) WirePeerRuntimeConfig(peerID string) (map[string]any, error) {
	ctx, err := s.wirePeerLookup(peerID)
	if err != nil {
		return nil, err
	}
	networkEnabled := ctx.networkDevice.Enabled && ctx.networkDevice.Status == "active" && ctx.runtime.NetworkEnabled
	return map[string]any{
		"peerId":                peerID,
		"networkId":             ctx.networkDevice.NetworkID,
		"nodeId":                "node-" + ctx.device.DeviceID,
		"virtualIps":            []string{ctx.device.GlobalIP},
		"allowedIps":            []string{ctx.device.GlobalIP + "/32"},
		"keepaliveIntervalSecs": 30,
		"networkEnabled":        networkEnabled,
		"preferredPath":         "direct_udp",
		"endpoints":             []wireEndpoint{},
	}, nil
}

func (s *Store) WireNetworkTopology(networkID string) (map[string]any, error) {
	networkID = strings.TrimSpace(networkID)
	if networkID == "" {
		return nil, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.networks[networkID]; !ok {
		return nil, errNotFound
	}
	peers := make([]wirePeerRecord, 0)
	for _, networkDevice := range s.networkDevices {
		if networkDevice.NetworkID != networkID || !networkDevice.Enabled || networkDevice.Status != "active" {
			continue
		}
		device, ok := s.devices[networkDevice.DeviceID]
		if !ok || device.Status != "active" {
			continue
		}
		peerEndpoints := s.deviceEndpointsForNetworkDeviceLocked(networkID, device.DeviceID, time.Now().Unix())
		wireEndpoints := make([]wireEndpoint, 0, len(peerEndpoints))
		for _, endpoint := range peerEndpoints {
			wireEndpoints = append(wireEndpoints, wireEndpoint{
				Kind:       endpoint.Type,
				Address:    endpoint.Address,
				Reachable:  true,
				ObservedAt: endpoint.UpdatedAt,
			})
		}
		peers = append(peers, wirePeerRecord{
			PeerID:                  wirePeerID(networkDevice.NetworkID, device.DeviceID),
			NetworkID:               networkDevice.NetworkID,
			NodeID:                  "node-" + device.DeviceID,
			PublicKey:               device.PublicKey,
			VirtualIPs:              []string{device.GlobalIP},
			AllowedIPs:              []string{device.GlobalIP + "/32"},
			SupportsLanDirect:       true,
			SupportsIpv6Direct:      true,
			SupportsDirectUdp:       true,
			SupportsRelayUdp:        true,
			SupportsDerpTcpTls443:   true,
			PreferIpv6:              false,
			PreferLan:               true,
			AllowEndpointRoaming:    true,
			AllowFastReselection:    true,
			AllowRelayTicketRenewal: true,
			KeepaliveIntervalSecs:   30,
			Endpoints:               wireEndpoints,
			ActivePath:              "direct_udp",
			RequireMtuRefresh:       false,
			EndpointChanged:         false,
			UpdatedAt:               networkDevice.UpdatedAt,
		})
	}
	return map[string]any{"networkId": networkID, "peers": peers}, nil
}

func (s *Store) wirePeerLookup(peerID string) (wirePeerContext, error) {
	peerID = strings.TrimSpace(peerID)
	if peerID == "" {
		return wirePeerContext{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	networkID, deviceID := parseWirePeerID(peerID)
	if deviceID == "" {
		return wirePeerContext{}, errBadRequest
	}
	var networkDevice NetworkDevice
	if networkID != "" {
		found, ok := s.networkDevices[networkID+"|"+deviceID]
		if !ok {
			return wirePeerContext{}, errNotFound
		}
		networkDevice = found
	} else {
		bestNetworkID := ""
		for _, candidate := range s.networkDevices {
			if candidate.DeviceID == deviceID && candidate.Enabled && candidate.Status == "active" {
				if bestNetworkID == "" || candidate.NetworkID < bestNetworkID {
					bestNetworkID = candidate.NetworkID
					networkDevice = candidate
				}
			}
		}
	}
	if networkDevice.DeviceID == "" {
		return wirePeerContext{}, errNotFound
	}
	device, ok := s.devices[deviceID]
	if !ok {
		return wirePeerContext{}, errNotFound
	}
	return wirePeerContext{networkDevice: networkDevice, device: device, runtime: s.runtimeStatuses[deviceID]}, nil
}

func wirePeerID(networkID, deviceID string) string {
	return strings.TrimSpace(networkID) + ":" + strings.TrimSpace(deviceID)
}

func parseWirePeerID(peerID string) (string, string) {
	peerID = strings.TrimSpace(peerID)
	if strings.Contains(peerID, ":") {
		parts := strings.SplitN(peerID, ":", 2)
		return strings.TrimSpace(parts[0]), strings.TrimPrefix(strings.TrimSpace(parts[1]), "node-")
	}
	deviceID := deviceIDFromNodeID(peerID)
	if deviceID == "" {
		deviceID = peerID
	}
	return "", deviceID
}
