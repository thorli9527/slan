package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/service"
	controlws "github.com/slan/server/server-biz/internal/ws"
	"golang.org/x/net/websocket"
)

type wsEnvelope struct {
	Type      string          `json:"type"`
	RequestID string          `json:"requestId,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
}

type wsSession struct {
	userID    string
	nodeID    string
	networkID string
}

type controlWSSession struct {
	conn      *websocket.Conn
	writeMu   sync.Mutex
	userID    string
	nodeID    string
	networkID string
}

type controlWSHub struct {
	mu       sync.RWMutex
	sessions map[string]*controlWSSession
}

func newControlWSHub() *controlWSHub {
	return &controlWSHub{
		sessions: make(map[string]*controlWSSession),
	}
}

func (h *controlWSHub) register(session *controlWSSession) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sessions[session.nodeID] = session
}

func (h *controlWSHub) unregister(nodeID string) {
	if nodeID == "" {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.sessions, nodeID)
}

func (h *controlWSHub) session(nodeID string) *controlWSSession {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.sessions[nodeID]
}

func (h *controlWSHub) peersInNetwork(networkID, excludeNodeID string) []*controlWSSession {
	h.mu.RLock()
	defer h.mu.RUnlock()

	out := make([]*controlWSSession, 0, len(h.sessions))
	for _, session := range h.sessions {
		if session.networkID != networkID || session.nodeID == excludeNodeID {
			continue
		}
		out = append(out, session)
	}
	return out
}

func (s *controlWSSession) send(msgType, requestID string, payload any) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	return websocket.JSON.Send(s.conn, controlws.Envelope{
		Type:      msgType,
		RequestID: requestID,
		Payload:   payload,
	})
}

var defaultControlWSHub = newControlWSHub()
var controlWSInstanceID = newControlWSInstanceID()
var controlWSSyncOnce sync.Once

func registerControlWS(router *gin.Engine, path string, services *service.Services) {
	controlWSSyncOnce.Do(func() {
		if services.ControlSync != nil {
			_ = services.ControlSync.Subscribe(func(event controlws.ControlSyncEvent) {
				if event.InstanceID == controlWSInstanceID {
					return
				}
				switch event.Type {
				case "peer_update":
					if event.Peer != nil {
						broadcastPeerUpdateToSessions(event.NetworkID, event.SourceNodeID, event.Revision, *event.Peer)
					}
				case "peer_remove":
					broadcastPeerRemoveToSessions(event.NetworkID, event.SourceNodeID, event.Revision)
				case "peer_candidate":
					if event.Candidate != nil {
						sendPeerCandidateToNode(event.TargetNodeID, *event.Candidate)
					}
				case "connect_plan":
					if event.Plan != nil {
						sendConnectPlanToNode(event.TargetNodeID, *event.Plan)
					}
				}
			})
		}
	})

	router.GET(path, func(c *gin.Context) {
		websocket.Handler(func(conn *websocket.Conn) {
			serveControlWS(conn, services)
		}).ServeHTTP(c.Writer, c.Request)
	})
}

func serveControlWS(conn *websocket.Conn, services *service.Services) {
	defer conn.Close()

	var session wsSession
	defer func() {
		defaultControlWSHub.unregister(session.nodeID)
		_ = services.ControlChannel.CloseSession(session.userID, session.nodeID, session.networkID)
		fanoutPeerRemove(services, session.networkID, session.nodeID)
	}()
	for {
		var env wsEnvelope
		if err := websocket.JSON.Receive(conn, &env); err != nil {
			return
		}

		switch env.Type {
		case "node_hello":
			var hello controlws.NodeHello
			if err := json.Unmarshal(env.Payload, &hello); err != nil {
				writeWSError(conn, env.RequestID, service.ErrInvalidArgument)
				continue
			}
			ack, networkMap, err := services.ControlChannel.Handshake(hello)
			if err != nil {
				writeWSError(conn, env.RequestID, err)
				continue
			}
			session = wsSession{
				userID:    firstNonEmpty(hello.UserID, networkMap.SelfUserID),
				nodeID:    hello.NodeID,
				networkID: hello.NetworkID,
			}
			defaultControlWSHub.register(&controlWSSession{
				conn:      conn,
				userID:    session.userID,
				nodeID:    session.nodeID,
				networkID: session.networkID,
			})
			if err := writeWSEnvelope(conn, "node_hello_ack", env.RequestID, ack); err != nil {
				return
			}
			if err := writeWSEnvelope(conn, "network_map_response", env.RequestID, controlws.NetworkMapResponse{
				Map: dtoNetworkMapToWS(networkMap),
			}); err != nil {
				return
			}
			fanoutPeerUpdate(services, session)
			fanoutConnectPlans(services, session)
		case "ping":
			var ping controlws.Ping
			if err := json.Unmarshal(env.Payload, &ping); err != nil {
				writeWSError(conn, env.RequestID, service.ErrInvalidArgument)
				continue
			}
			if err := writeWSEnvelope(conn, "pong", env.RequestID, controlws.Pong{Timestamp: ping.Timestamp}); err != nil {
				return
			}
		case "network_map_request":
			if session.userID == "" || session.nodeID == "" || session.networkID == "" {
				writeWSError(conn, env.RequestID, service.ErrUnauthorized)
				continue
			}
			var req controlws.NetworkMapRequest
			if err := json.Unmarshal(env.Payload, &req); err != nil {
				writeWSError(conn, env.RequestID, service.ErrInvalidArgument)
				continue
			}
			networkID := session.networkID
			if req.NetworkID != "" {
				networkID = req.NetworkID
			}
			networkMap, err := services.ControlChannel.NetworkMap(session.userID, session.nodeID, networkID)
			if err != nil {
				writeWSError(conn, env.RequestID, err)
				continue
			}
			session.networkID = networkID
			if err := writeWSEnvelope(conn, "network_map_response", env.RequestID, controlws.NetworkMapResponse{
				Map: dtoNetworkMapToWS(networkMap),
			}); err != nil {
				return
			}
		case "endpoint_report":
			if session.userID == "" || session.nodeID == "" || session.networkID == "" {
				writeWSError(conn, env.RequestID, service.ErrUnauthorized)
				continue
			}
			var report controlws.EndpointReport
			if err := json.Unmarshal(env.Payload, &report); err != nil {
				writeWSError(conn, env.RequestID, service.ErrInvalidArgument)
				continue
			}
			report.NodeID = firstNonEmpty(report.NodeID, session.nodeID)
			report.NetworkID = firstNonEmpty(report.NetworkID, session.networkID)
			networkMap, err := services.ControlChannel.ReportEndpoints(session.userID, report)
			if err != nil {
				writeWSError(conn, env.RequestID, err)
				continue
			}
			if err := writeWSEnvelope(conn, "network_map_response", env.RequestID, controlws.NetworkMapResponse{
				Map: dtoNetworkMapToWS(networkMap),
			}); err != nil {
				return
			}
			fanoutPeerUpdate(services, session)
			fanoutConnectPlans(services, session)
		case "connection_state":
			if session.userID == "" || session.nodeID == "" || session.networkID == "" {
				writeWSError(conn, env.RequestID, service.ErrUnauthorized)
				continue
			}
			var state controlws.ConnectionState
			if err := json.Unmarshal(env.Payload, &state); err != nil {
				writeWSError(conn, env.RequestID, service.ErrInvalidArgument)
				continue
			}
			state.NetworkID = firstNonEmpty(state.NetworkID, session.networkID)
			if err := services.ControlChannel.ReportConnectionState(session.userID, session.nodeID, state); err != nil {
				writeWSError(conn, env.RequestID, err)
				continue
			}
		case "disconnect_notice":
			if session.userID == "" || session.nodeID == "" || session.networkID == "" {
				writeWSError(conn, env.RequestID, service.ErrUnauthorized)
				continue
			}
			var notice controlws.DisconnectNotice
			if err := json.Unmarshal(env.Payload, &notice); err != nil {
				writeWSError(conn, env.RequestID, service.ErrInvalidArgument)
				continue
			}
			notice.NetworkID = firstNonEmpty(notice.NetworkID, session.networkID)
			if err := services.ControlChannel.Disconnect(session.userID, session.nodeID, notice); err != nil {
				writeWSError(conn, env.RequestID, err)
				continue
			}
			fanoutPeerRemove(services, session.networkID, session.nodeID)
		case "peer_candidate":
			if session.userID == "" || session.nodeID == "" || session.networkID == "" {
				writeWSError(conn, env.RequestID, service.ErrUnauthorized)
				continue
			}
			var candidate controlws.PeerCandidate
			if err := json.Unmarshal(env.Payload, &candidate); err != nil {
				writeWSError(conn, env.RequestID, service.ErrInvalidArgument)
				continue
			}
			if candidate.PeerNodeID == "" || candidate.Endpoint == "" || candidate.CandidateType == "" {
				writeWSError(conn, env.RequestID, service.ErrInvalidArgument)
				continue
			}
			sendPeerCandidateToNode(candidate.PeerNodeID, candidate)
			if services.ControlSync != nil {
				rev, _ := services.ControlSync.CurrentRevision(session.networkID)
				_ = services.ControlSync.Publish(controlws.ControlSyncEvent{
					InstanceID:   controlWSInstanceID,
					Type:         "peer_candidate",
					NetworkID:    session.networkID,
					SourceNodeID: session.nodeID,
					TargetNodeID: candidate.PeerNodeID,
					Revision:     rev,
					Candidate:    ptr(candidate),
				})
			}
		default:
			writeWSError(conn, env.RequestID, service.ErrInvalidArgument)
		}
	}
}

func writeWSEnvelope(conn *websocket.Conn, msgType, requestID string, payload any) error {
	return websocket.JSON.Send(conn, controlws.Envelope{
		Type:      msgType,
		RequestID: requestID,
		Payload:   payload,
	})
}

func writeWSError(conn *websocket.Conn, requestID string, err error) {
	code := "INTERNAL"
	switch {
	case errors.Is(err, service.ErrInvalidArgument):
		code = "INVALID_ARGUMENT"
	case errors.Is(err, service.ErrUnauthorized):
		code = "UNAUTHORIZED"
	case errors.Is(err, service.ErrForbidden):
		code = "FORBIDDEN"
	case errors.Is(err, service.ErrNotFound):
		code = "NOT_FOUND"
	case errors.Is(err, service.ErrConflict):
		code = "CONFLICT"
	}
	_ = writeWSEnvelope(conn, "error", requestID, controlws.ErrorMessage{
		Code:    code,
		Message: err.Error(),
	})
}

func dtoNetworkMapToWS(m dto.NetworkMap) controlws.NetworkMap {
	peers := make([]controlws.Peer, 0, len(m.Peers))
	for _, peer := range m.Peers {
		endpoints := make([]controlws.Endpoint, 0, len(peer.Endpoints))
		for _, endpoint := range peer.Endpoints {
			endpoints = append(endpoints, controlws.Endpoint{
				Type:      endpoint.Type,
				Address:   endpoint.Address,
				UpdatedAt: endpoint.UpdatedAt,
			})
		}
		peers = append(peers, controlws.Peer{
			NodeID:        peer.NodeID,
			DeviceID:      peer.DeviceID,
			PublicKey:     peer.PublicKey,
			Status:        peer.Status,
			RelayAllowed:  peer.RelayAllowed,
			VirtualIPs:    append([]string(nil), peer.VirtualIPs...),
			Endpoints:     endpoints,
			AllowedRoutes: append([]string(nil), peer.AllowedRoutes...),
		})
	}

	routes := make([]controlws.Route, 0, len(m.Routes))
	for _, route := range m.Routes {
		routes = append(routes, controlws.Route{
			CIDR:      route.CIDR,
			ViaNodeID: route.ViaNodeID,
			Metric:    route.Metric,
		})
	}

	relayRegions := make([]controlws.RelayRegion, 0, len(m.RelayRegions))
	for _, region := range m.RelayRegions {
		endpoints := make([]controlws.RelayEndpoint, 0, len(region.Endpoints))
		for _, endpoint := range region.Endpoints {
			endpoints = append(endpoints, controlws.RelayEndpoint{
				EndpointID: endpoint.EndpointID,
				Transport:  endpoint.Transport,
				Address:    endpoint.Address,
			})
		}
		relayRegions = append(relayRegions, controlws.RelayRegion{
			RegionID:   region.RegionID,
			RegionName: region.RegionName,
			Endpoints:  endpoints,
		})
	}

	return controlws.NetworkMap{
		SelfUserID:       m.SelfUserID,
		SelfDeviceID:     m.SelfDeviceID,
		SelfNodeID:       m.SelfNodeID,
		NetworkID:        m.NetworkID,
		Revision:         m.Revision,
		HeartbeatSeconds: m.HeartbeatSeconds,
		STUNServers:      append([]string(nil), m.STUNServers...),
		Peers:            peers,
		Routes:           routes,
		RelayRegions:     relayRegions,
		DNS: controlws.DNSConfig{
			Servers:       append([]string(nil), m.DNS.Servers...),
			SearchDomains: append([]string(nil), m.DNS.SearchDomains...),
		},
		MTU: m.MTU,
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func fanoutPeerUpdate(services *service.Services, session wsSession) {
	peer, err := services.ControlChannel.PeerSnapshot(session.userID, session.nodeID, session.networkID, session.nodeID)
	if err != nil {
		return
	}
	revision := uint64(1)
	if services.ControlSync != nil {
		if next, err := services.ControlSync.NextRevision(session.networkID); err == nil && next > 0 {
			revision = next
		}
	}
	broadcastPeerUpdateToSessions(session.networkID, session.nodeID, revision, dtoPeerToWSPeer(peer))
	if services.ControlSync != nil {
		_ = services.ControlSync.Publish(controlws.ControlSyncEvent{
			InstanceID:   controlWSInstanceID,
			Type:         "peer_update",
			NetworkID:    session.networkID,
			SourceNodeID: session.nodeID,
			Revision:     revision,
			Peer:         ptr(dtoPeerToWSPeer(peer)),
		})
	}
}

func fanoutPeerRemove(services *service.Services, networkID, sourceNodeID string) {
	revision := uint64(1)
	if services.ControlSync != nil {
		if next, err := services.ControlSync.NextRevision(networkID); err == nil && next > 0 {
			revision = next
		}
	}
	broadcastPeerRemoveToSessions(networkID, sourceNodeID, revision)
	if services.ControlSync != nil {
		_ = services.ControlSync.Publish(controlws.ControlSyncEvent{
			InstanceID:   controlWSInstanceID,
			Type:         "peer_remove",
			NetworkID:    networkID,
			SourceNodeID: sourceNodeID,
			Revision:     revision,
			PeerNodeID:   sourceNodeID,
		})
	}
}

func fanoutConnectPlans(services *service.Services, session wsSession) {
	for _, peerSession := range defaultControlWSHub.peersInNetwork(session.networkID, session.nodeID) {
		planForSource, err := services.ControlChannel.ConnectPlan(session.userID, session.nodeID, session.networkID, peerSession.nodeID)
		if err == nil {
			sendConnectPlanToNode(session.nodeID, planForSource)
			publishConnectPlan(services, session.networkID, session.nodeID, session.nodeID, planForSource)
		}
		planForPeer, err := services.ControlChannel.ConnectPlan(peerSession.userID, peerSession.nodeID, peerSession.networkID, session.nodeID)
		if err == nil {
			sendConnectPlanToNode(peerSession.nodeID, planForPeer)
			publishConnectPlan(services, peerSession.networkID, peerSession.nodeID, peerSession.nodeID, planForPeer)
		}
	}
}

func publishConnectPlan(services *service.Services, networkID, sourceNodeID, targetNodeID string, plan controlws.ConnectPlan) {
	if services.ControlSync == nil {
		return
	}
	rev, _ := services.ControlSync.CurrentRevision(networkID)
	_ = services.ControlSync.Publish(controlws.ControlSyncEvent{
		InstanceID:   controlWSInstanceID,
		Type:         "connect_plan",
		NetworkID:    networkID,
		SourceNodeID: sourceNodeID,
		TargetNodeID: targetNodeID,
		Revision:     rev,
		Plan:         ptr(plan),
	})
}

func sendPeerCandidateToNode(targetNodeID string, candidate controlws.PeerCandidate) {
	if targetNodeID == "" {
		return
	}
	session := defaultControlWSHub.session(targetNodeID)
	if session == nil {
		return
	}
	_ = session.send("peer_candidate", "", candidate)
}

func sendConnectPlanToNode(targetNodeID string, plan controlws.ConnectPlan) {
	if targetNodeID == "" {
		return
	}
	session := defaultControlWSHub.session(targetNodeID)
	if session == nil {
		return
	}
	_ = session.send("connect_plan", "", plan)
}

func broadcastPeerUpdateToSessions(networkID, sourceNodeID string, revision uint64, peer controlws.Peer) {
	for _, peerSession := range defaultControlWSHub.peersInNetwork(networkID, sourceNodeID) {
		_ = peerSession.send("peer_update", "", controlws.PeerUpdate{
			NetworkID: networkID,
			Revision:  revision,
			Peer:      peer,
		})
	}
}

func broadcastPeerRemoveToSessions(networkID, sourceNodeID string, revision uint64) {
	if sourceNodeID == "" || networkID == "" {
		return
	}
	for _, peerSession := range defaultControlWSHub.peersInNetwork(networkID, sourceNodeID) {
		_ = peerSession.send("peer_remove", "", controlws.PeerRemove{
			NetworkID:  networkID,
			Revision:   revision,
			PeerNodeID: sourceNodeID,
		})
	}
}

func newControlWSInstanceID() string {
	var buf [6]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "instance"
	}
	return hex.EncodeToString(buf[:])
}

func ptr[T any](value T) *T {
	return &value
}

func findPeer(peers []dto.Peer, nodeID string) (dto.Peer, bool) {
	for _, peer := range peers {
		if peer.NodeID == nodeID {
			return peer, true
		}
	}
	return dto.Peer{}, false
}

func dtoPeerToWSPeer(peer dto.Peer) controlws.Peer {
	endpoints := make([]controlws.Endpoint, 0, len(peer.Endpoints))
	for _, endpoint := range peer.Endpoints {
		endpoints = append(endpoints, controlws.Endpoint{
			Type:      endpoint.Type,
			Address:   endpoint.Address,
			UpdatedAt: endpoint.UpdatedAt,
		})
	}
	return controlws.Peer{
		NodeID:        peer.NodeID,
		DeviceID:      peer.DeviceID,
		PublicKey:     peer.PublicKey,
		Status:        peer.Status,
		RelayAllowed:  peer.RelayAllowed,
		VirtualIPs:    append([]string(nil), peer.VirtualIPs...),
		Endpoints:     endpoints,
		AllowedRoutes: append([]string(nil), peer.AllowedRoutes...),
	}
}
