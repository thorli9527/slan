package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
	controlws "github.com/slan/server/server-biz/internal/ws"
)

func (s dbControlChannelService) Handshake(hello controlws.NodeHello) (controlws.NodeHelloAck, dto.NetworkMap, error) {
	if strings.TrimSpace(hello.SessionToken) == "" || strings.TrimSpace(hello.NodeID) == "" || strings.TrimSpace(hello.NetworkID) == "" {
		return controlws.NodeHelloAck{}, dto.NetworkMap{}, fmt.Errorf("%w: sessionToken, nodeId, and networkId are required", ErrInvalidArgument)
	}

	ctx := context.Background()
	userID, err := s.state.tokens.AuthenticateControlSessionToken(ctx, hello.SessionToken)
	if err != nil {
		return controlws.NodeHelloAck{}, dto.NetworkMap{}, ErrUnauthorized
	}
	if hello.UserID != "" && hello.UserID != userID {
		return controlws.NodeHelloAck{}, dto.NetworkMap{}, ErrForbidden
	}

	session, err := s.state.pg.GetControlSessionByToken(ctx, hello.SessionToken)
	if err != nil {
		if repo.IsNotFound(err) {
			return controlws.NodeHelloAck{}, dto.NetworkMap{}, ErrUnauthorized
		}
		return controlws.NodeHelloAck{}, dto.NetworkMap{}, err
	}
	if session.UserID != userID || session.NodeID != hello.NodeID || session.NetworkID != hello.NetworkID {
		return controlws.NodeHelloAck{}, dto.NetworkMap{}, ErrForbidden
	}
	if hello.DeviceID != "" && session.DeviceID != hello.DeviceID {
		return controlws.NodeHelloAck{}, dto.NetworkMap{}, ErrForbidden
	}
	if err := s.state.pg.UpdateDeviceStatus(ctx, session.DeviceID, "online"); err != nil {
		return controlws.NodeHelloAck{}, dto.NetworkMap{}, err
	}

	networkMap, err := s.NetworkMap(userID, hello.NodeID, hello.NetworkID)
	if err != nil {
		return controlws.NodeHelloAck{}, dto.NetworkMap{}, err
	}

	return controlws.NodeHelloAck{
		ControlSessionID: session.ControlSessionID,
		HeartbeatSeconds: networkMap.HeartbeatSeconds,
		NetworkRevision:  networkMap.Revision,
	}, networkMap, nil
}

func (s dbControlChannelService) NetworkMap(userID, nodeID, networkID string) (dto.NetworkMap, error) {
	ctx := context.Background()
	node, err := s.state.requireNodeSession(ctx, userID, nodeID, networkID)
	if err != nil {
		return dto.NetworkMap{}, err
	}
	return s.state.buildNetworkMap(ctx, userID, node.ToDTO(nil), networkID), nil
}

func (s dbControlChannelService) ReportEndpoints(userID string, report controlws.EndpointReport) (dto.NetworkMap, error) {
	if strings.TrimSpace(report.NodeID) == "" || strings.TrimSpace(report.NetworkID) == "" {
		return dto.NetworkMap{}, fmt.Errorf("%w: nodeId and networkId are required", ErrInvalidArgument)
	}

	ctx := context.Background()
	node, err := s.state.requireNodeSession(ctx, userID, report.NodeID, report.NetworkID)
	if err != nil {
		return dto.NetworkMap{}, err
	}

	endpoints := make([]repo.NodeEndpoint, 0, len(report.Endpoints))
	for _, endpoint := range report.Endpoints {
		if strings.TrimSpace(endpoint.Type) == "" || strings.TrimSpace(endpoint.Address) == "" {
			return dto.NetworkMap{}, fmt.Errorf("%w: endpoint type and address are required", ErrInvalidArgument)
		}
		endpoints = append(endpoints, repo.NodeEndpoint{
			EndpointID: newID("ep"),
			Type:       endpoint.Type,
			Address:    endpoint.Address,
			UpdatedAt:  endpoint.UpdatedAt,
		})
	}
	if err := s.state.pg.ReplaceNodeEndpoints(ctx, node.NodeID, report.NetworkID, report.NatType, endpoints); err != nil {
		return dto.NetworkMap{}, err
	}
	if err := s.state.pg.UpdateDeviceStatus(ctx, node.DeviceID, "online"); err != nil {
		return dto.NetworkMap{}, err
	}
	return s.state.buildNetworkMap(ctx, userID, node.ToDTO(nil), report.NetworkID), nil
}

func (s dbControlChannelService) ReportConnectionState(userID, nodeID string, state controlws.ConnectionState) error {
	if strings.TrimSpace(state.NetworkID) == "" || strings.TrimSpace(state.PeerNodeID) == "" || strings.TrimSpace(state.State) == "" {
		return fmt.Errorf("%w: networkId, peerNodeId, and state are required", ErrInvalidArgument)
	}

	ctx := context.Background()
	sourceNode, err := s.state.requireNodeSession(ctx, userID, nodeID, state.NetworkID)
	if err != nil {
		return err
	}
	peerNode, err := s.state.pg.GetNodeByID(ctx, state.PeerNodeID)
	if err != nil {
		if repo.IsNotFound(err) {
			return ErrNotFound
		}
		return err
	}
	if _, err := s.state.pg.GetMemberByNetworkDevice(ctx, state.NetworkID, peerNode.DeviceID); err != nil {
		if repo.IsNotFound(err) {
			return ErrForbidden
		}
		return err
	}
	return s.state.pg.UpsertNodeConnectionState(ctx, repo.NodeConnectionState{
		StateID:    newID("conn"),
		NetworkID:  state.NetworkID,
		NodeID:     sourceNode.NodeID,
		PeerNodeID: state.PeerNodeID,
		Path:       state.Path,
		State:      state.State,
		Reason:     state.Reason,
		UpdatedAt:  time.Now().Unix(),
	})
}

func (s dbControlChannelService) Disconnect(userID, nodeID string, notice controlws.DisconnectNotice) error {
	if strings.TrimSpace(nodeID) == "" || strings.TrimSpace(notice.NetworkID) == "" || strings.TrimSpace(notice.PeerNodeID) == "" {
		return fmt.Errorf("%w: nodeId, networkId, and peerNodeId are required", ErrInvalidArgument)
	}

	ctx := context.Background()
	node, err := s.state.requireNodeSession(ctx, userID, nodeID, notice.NetworkID)
	if err != nil {
		return err
	}
	if err := s.state.pg.UpsertNodeConnectionState(ctx, repo.NodeConnectionState{
		StateID:    newID("conn"),
		NetworkID:  notice.NetworkID,
		NodeID:     nodeID,
		PeerNodeID: notice.PeerNodeID,
		Path:       "disconnect",
		State:      "closed",
		Reason:     notice.Reason,
		UpdatedAt:  time.Now().Unix(),
	}); err != nil {
		return err
	}
	return s.state.pg.UpdateDeviceStatus(ctx, node.DeviceID, "offline")
}

func (s dbControlChannelService) CloseSession(userID, nodeID, networkID string) error {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(nodeID) == "" || strings.TrimSpace(networkID) == "" {
		return nil
	}

	ctx := context.Background()
	node, err := s.state.requireNodeSession(ctx, userID, nodeID, networkID)
	if err != nil {
		if err == ErrForbidden || err == ErrNotFound {
			return nil
		}
		return err
	}
	if err := s.state.pg.DeleteNodeEndpoints(ctx, nodeID, networkID); err != nil {
		return err
	}
	if err := s.state.pg.DeleteNodeConnectionStates(ctx, nodeID, networkID); err != nil {
		return err
	}
	return s.state.pg.UpdateDeviceStatus(ctx, node.DeviceID, "offline")
}

func (s dbControlChannelService) PeerSnapshot(userID, nodeID, networkID, peerNodeID string) (dto.Peer, error) {
	ctx := context.Background()
	if _, err := s.state.requireNodeSession(ctx, userID, nodeID, networkID); err != nil {
		return dto.Peer{}, err
	}

	record, err := s.state.pg.GetNodeByID(ctx, peerNodeID)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.Peer{}, ErrNotFound
		}
		return dto.Peer{}, err
	}
	if _, err := s.state.pg.GetMemberByNetworkDevice(ctx, networkID, record.DeviceID); err != nil {
		if repo.IsNotFound(err) {
			return dto.Peer{}, ErrForbidden
		}
		return dto.Peer{}, err
	}
	device, err := s.state.pg.GetDeviceByID(ctx, record.DeviceID)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.Peer{}, ErrNotFound
		}
		return dto.Peer{}, err
	}
	return dto.Peer{
		NodeID:        record.NodeID,
		DeviceID:      record.DeviceID,
		PublicKey:     record.NodePublicKey,
		Status:        device.Status,
		RelayAllowed:  true,
		VirtualIPs:    s.state.virtualIPsForDeviceInNetwork(ctx, record.DeviceID, networkID),
		Endpoints:     s.state.endpointsForNodeInNetwork(ctx, record.NodeID, networkID),
		AllowedRoutes: s.state.allowedRoutesForDeviceInNetwork(ctx, record.DeviceID, networkID),
	}, nil
}

func (s dbControlChannelService) ConnectPlan(userID, nodeID, networkID, peerNodeID string) (controlws.ConnectPlan, error) {
	ctx := context.Background()
	if _, err := s.state.requireNodeSession(ctx, userID, nodeID, networkID); err != nil {
		return controlws.ConnectPlan{}, err
	}
	peer, err := s.PeerSnapshot(userID, nodeID, networkID, peerNodeID)
	if err != nil {
		return controlws.ConnectPlan{}, err
	}

	sourceNatType := s.nodeNatType(ctx, nodeID, networkID)
	peerNatType := s.nodeNatType(ctx, peerNodeID, networkID)
	connectionState := s.connectionState(ctx, networkID, nodeID, peerNodeID)

	paths := make([]controlws.PathOption, 0, len(peer.Endpoints)+1)
	priority := 10
	for _, endpoint := range peer.Endpoints {
		if endpoint.Type == "relay" {
			continue
		}
		pathPriority := priority
		switch endpoint.Type {
		case "lan":
			pathPriority = 10
		case "wan":
			pathPriority = 20
		case "reflexive":
			pathPriority = 30
		}
		paths = append(paths, controlws.PathOption{
			PathType: endpoint.Type,
			Endpoint: endpoint.Address,
			Priority: pathPriority,
		})
		priority += 10
	}

	preferDirect := len(paths) > 0
	needRelayTicket := false
	if sourceNatType == "symmetric" || peerNatType == "symmetric" {
		preferDirect = false
		needRelayTicket = true
	}
	if connectionState.State == "failed" || connectionState.State == "closed" {
		preferDirect = false
		needRelayTicket = true
	}

	var relayTicket *controlws.RelayTicket
	if needRelayTicket {
		ticket, err := s.state.issueRelayTicket(ctx, userID, dto.RelayTicketRequest{
			NetworkID:     networkID,
			SrcNodeID:     nodeID,
			DstNodeID:     peerNodeID,
			DerpClusterID: s.state.cfg.Relay.Region,
			Reason:        relayReason(connectionState),
		})
		if err == nil {
			relayTicket = &controlws.RelayTicket{
				TicketID:           ticket.TicketID,
				NetworkID:          ticket.NetworkID,
				SessionID:          ticket.SessionID,
				SrcNodeID:          ticket.SrcNodeID,
				DstNodeID:          ticket.DstNodeID,
				DerpClusterID:      ticket.DerpClusterID,
				AllowedDerpNodeIDs: append([]string(nil), ticket.AllowedDerpNodeIDs...),
				RelayURL:           ticket.RelayURL,
				ExpiresAt:          ticket.ExpiresAt,
				SessionKey:         ticket.SessionKey,
				Signature:          ticket.Signature,
			}
		}
	}

	paths = append(paths, controlws.PathOption{
		PathType: "relay",
		Endpoint: s.state.cfg.Relay.UDPEndpoint,
		Priority: priority,
	})

	return controlws.ConnectPlan{
		PeerNodeID:           peerNodeID,
		PreferDirect:         preferDirect,
		Paths:                paths,
		DerpClusterID:        s.state.cfg.Relay.Region,
		PreferredDerpNodeIDs: nil,
		RelayTicket:          relayTicket,
	}, nil
}

func (s dbControlChannelService) nodeNatType(ctx context.Context, nodeID, networkID string) string {
	value, err := s.state.pg.GetNodeNatType(ctx, nodeID, networkID)
	if err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(value))
}

func (s dbControlChannelService) connectionState(ctx context.Context, networkID, nodeID, peerNodeID string) repo.NodeConnectionState {
	value, err := s.state.pg.GetNodeConnectionState(ctx, networkID, nodeID, peerNodeID)
	if err != nil {
		return repo.NodeConnectionState{}
	}
	return value
}

func relayReason(state repo.NodeConnectionState) string {
	if strings.TrimSpace(state.Reason) != "" {
		return state.Reason
	}
	if state.State == "failed" || state.State == "closed" {
		return state.State
	}
	return "nat_restricted"
}
