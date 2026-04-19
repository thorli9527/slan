package httpapi

import (
	"encoding/json"

	"github.com/slan/server/server-biz/internal/service"
	"github.com/slan/server/server-biz/internal/util"
	controlws "github.com/slan/server/server-biz/internal/ws"
	"golang.org/x/net/websocket"
)

func handleControlWSMessage(conn *websocket.Conn, deps routerDeps, session *wsSession, env wsEnvelope) error {
	switch env.Type {
	case "node_hello":
		return handleNodeHello(conn, deps, session, env)
	case "ping":
		return handlePing(conn, deps, session, env)
	case "network_map_request":
		return handleNetworkMapRequest(conn, deps, session, env)
	case "endpoint_report":
		return handleEndpointReport(conn, deps, session, env)
	case "connection_state":
		return handleConnectionState(conn, deps, session, env)
	case "path_health_report":
		return handlePathHealthReport(conn, deps, session, env)
	case "disconnect_notice":
		return handleDisconnectNotice(conn, deps, session, env)
	case "peer_candidate":
		return handlePeerCandidate(conn, deps, session, env)
	default:
		writeWSError(conn, env.RequestID, service.ErrInvalidArgument)
		return nil
	}
}

func handleNodeHello(conn *websocket.Conn, deps routerDeps, session *wsSession, env wsEnvelope) error {
	var hello controlws.NodeHello
	if err := json.Unmarshal(env.Payload, &hello); err != nil {
		writeWSError(conn, env.RequestID, service.ErrInvalidArgument)
		return nil
	}

	ack, networkMap, err := deps.ControlChannel.Handshake(hello)
	if err != nil {
		metricAdd("handshake_error_total", 1)
		writeWSError(conn, env.RequestID, err)
		return nil
	}

	metricAdd("handshake_success_total", 1)
	metricAddByType(controlWSMessageTypeMetrics, "node_hello_ack", 1)
	*session = wsSession{
		userID:    util.FirstNonEmpty(hello.UserID, networkMap.SelfUserID),
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
		return err
	}
	if err := writeWSEnvelope(conn, "network_map_response", env.RequestID, controlws.NetworkMapResponse{
		Map: dtoNetworkMapToWS(networkMap),
	}); err != nil {
		return err
	}

	fanoutPeerUpdate(deps, *session)
	fanoutConnectPlans(deps, *session)
	return nil
}

func handlePing(conn *websocket.Conn, deps routerDeps, session *wsSession, env wsEnvelope) error {
	var ping controlws.Ping
	if err := json.Unmarshal(env.Payload, &ping); err != nil {
		writeWSError(conn, env.RequestID, service.ErrInvalidArgument)
		return nil
	}
	if session.authorized() {
		_ = deps.ControlChannel.Heartbeat(session.userID, session.nodeID, session.networkID)
	}
	return writeWSEnvelope(conn, "pong", env.RequestID, controlws.Pong{Timestamp: ping.Timestamp})
}

func handleNetworkMapRequest(conn *websocket.Conn, deps routerDeps, session *wsSession, env wsEnvelope) error {
	if !session.authorized() {
		writeWSError(conn, env.RequestID, service.ErrUnauthorized)
		return nil
	}
	_ = deps.ControlChannel.Heartbeat(session.userID, session.nodeID, session.networkID)

	var req controlws.NetworkMapRequest
	if err := json.Unmarshal(env.Payload, &req); err != nil {
		writeWSError(conn, env.RequestID, service.ErrInvalidArgument)
		return nil
	}

	networkID := session.networkID
	if req.NetworkID != "" {
		networkID = req.NetworkID
	}
	networkMap, err := deps.ControlChannel.NetworkMap(session.userID, session.nodeID, networkID)
	if err != nil {
		writeWSError(conn, env.RequestID, err)
		return nil
	}
	session.networkID = networkID
	return writeWSEnvelope(conn, "network_map_response", env.RequestID, controlws.NetworkMapResponse{
		Map: dtoNetworkMapToWS(networkMap),
	})
}

func handleEndpointReport(conn *websocket.Conn, deps routerDeps, session *wsSession, env wsEnvelope) error {
	if !session.authorized() {
		writeWSError(conn, env.RequestID, service.ErrUnauthorized)
		return nil
	}
	_ = deps.ControlChannel.Heartbeat(session.userID, session.nodeID, session.networkID)

	var report controlws.EndpointReport
	if err := json.Unmarshal(env.Payload, &report); err != nil {
		writeWSError(conn, env.RequestID, service.ErrInvalidArgument)
		return nil
	}
	report.NodeID = util.FirstNonEmpty(report.NodeID, session.nodeID)
	report.NetworkID = util.FirstNonEmpty(report.NetworkID, session.networkID)

	networkMap, err := deps.ControlChannel.ReportEndpoints(session.userID, report)
	if err != nil {
		writeWSError(conn, env.RequestID, err)
		return nil
	}
	if err := writeWSEnvelope(conn, "network_map_response", env.RequestID, controlws.NetworkMapResponse{
		Map: dtoNetworkMapToWS(networkMap),
	}); err != nil {
		return err
	}

	fanoutPeerUpdate(deps, *session)
	fanoutConnectPlans(deps, *session)
	return nil
}

func handleConnectionState(conn *websocket.Conn, deps routerDeps, session *wsSession, env wsEnvelope) error {
	if !session.authorized() {
		writeWSError(conn, env.RequestID, service.ErrUnauthorized)
		return nil
	}
	_ = deps.ControlChannel.Heartbeat(session.userID, session.nodeID, session.networkID)

	var state controlws.ConnectionState
	if err := json.Unmarshal(env.Payload, &state); err != nil {
		writeWSError(conn, env.RequestID, service.ErrInvalidArgument)
		return nil
	}
	state.NetworkID = util.FirstNonEmpty(state.NetworkID, session.networkID)

	if err := deps.ControlChannel.ReportConnectionState(session.userID, session.nodeID, state); err != nil {
		writeWSError(conn, env.RequestID, err)
		return nil
	}
	metricRecordConnectionState(session, state)
	if state.State == "connected" {
		resetConnectPlanRetry(deps, session.networkID, session.nodeID, state.PeerNodeID)
	}
	if state.State == "failed" || state.State == "closed" {
		if !allowConnectPlanRetry(deps, session.networkID, session.nodeID, state.PeerNodeID) {
			return nil
		}
		fanoutConnectPlanPair(deps, *session, state.PeerNodeID)
	}
	return nil
}

func handlePathHealthReport(conn *websocket.Conn, deps routerDeps, session *wsSession, env wsEnvelope) error {
	if !session.authorized() {
		writeWSError(conn, env.RequestID, service.ErrUnauthorized)
		return nil
	}
	_ = deps.ControlChannel.Heartbeat(session.userID, session.nodeID, session.networkID)

	var report controlws.PathHealthReport
	if err := json.Unmarshal(env.Payload, &report); err != nil {
		writeWSError(conn, env.RequestID, service.ErrInvalidArgument)
		return nil
	}
	report.NetworkID = util.FirstNonEmpty(report.NetworkID, session.networkID)

	if err := deps.ControlChannel.ReportPathHealth(session.userID, session.nodeID, report); err != nil {
		writeWSError(conn, env.RequestID, err)
		return nil
	}
	metricRecordPathHealth(session, report)
	return nil
}

func handleDisconnectNotice(conn *websocket.Conn, deps routerDeps, session *wsSession, env wsEnvelope) error {
	if !session.authorized() {
		writeWSError(conn, env.RequestID, service.ErrUnauthorized)
		return nil
	}
	_ = deps.ControlChannel.Heartbeat(session.userID, session.nodeID, session.networkID)

	var notice controlws.DisconnectNotice
	if err := json.Unmarshal(env.Payload, &notice); err != nil {
		writeWSError(conn, env.RequestID, service.ErrInvalidArgument)
		return nil
	}
	notice.NetworkID = util.FirstNonEmpty(notice.NetworkID, session.networkID)

	if err := deps.ControlChannel.Disconnect(session.userID, session.nodeID, notice); err != nil {
		writeWSError(conn, env.RequestID, err)
		return nil
	}
	fanoutPeerRemove(deps, session.networkID, session.nodeID)
	return nil
}

func handlePeerCandidate(conn *websocket.Conn, deps routerDeps, session *wsSession, env wsEnvelope) error {
	if !session.authorized() {
		writeWSError(conn, env.RequestID, service.ErrUnauthorized)
		return nil
	}
	_ = deps.ControlChannel.Heartbeat(session.userID, session.nodeID, session.networkID)

	var candidate controlws.PeerCandidate
	if err := json.Unmarshal(env.Payload, &candidate); err != nil {
		writeWSError(conn, env.RequestID, service.ErrInvalidArgument)
		return nil
	}
	if candidate.PeerNodeID == "" || candidate.Endpoint == "" || candidate.CandidateType == "" {
		writeWSError(conn, env.RequestID, service.ErrInvalidArgument)
		return nil
	}

	forwardPeerCandidate(deps, *session, candidate)
	return nil
}

func (s wsSession) authorized() bool {
	return s.userID != "" && s.nodeID != "" && s.networkID != ""
}
