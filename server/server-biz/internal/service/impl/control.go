package impl

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/slan/server/server-biz/api/dto"
	controlmsg "github.com/slan/server/server-biz/internal/controlmsg"
	"github.com/slan/server/server-biz/internal/repo"
	"github.com/slan/server/server-biz/internal/service"
)

type dbControlChannelService struct{ state *dbState }
type dbControlSyncService struct{ state *dbState }

var (
	_ service.ControlChannel = dbControlChannelService{}
	_ service.ControlSync    = dbControlSyncService{}
)

// cachedRelayTicket 表示进程内缓存的一张仍可复用的 relay ticket。
type cachedRelayTicket struct {
	ticket     dto.RelayTicket
	validUntil time.Time
}

// Handshake verifies the control-plane session token, marks the backing device
// reachable, and returns the latest network map snapshot for the MQTT control session.
func (s dbControlChannelService) Handshake(hello controlmsg.NodeHello) (controlmsg.NodeHelloAck, dto.NetworkMap, error) {
	if strings.TrimSpace(hello.SessionToken) == "" || strings.TrimSpace(hello.NodeID) == "" || strings.TrimSpace(hello.NetworkID) == "" {
		return controlmsg.NodeHelloAck{}, dto.NetworkMap{}, fmt.Errorf("%w: sessionToken, nodeId, and networkId are required", ErrInvalidArgument)
	}

	ctx := context.Background()
	userID, err := s.state.tokens.AuthenticateControlSessionToken(ctx, hello.SessionToken)
	if err != nil {
		return controlmsg.NodeHelloAck{}, dto.NetworkMap{}, ErrUnauthorized
	}
	if hello.UserID != "" && hello.UserID != userID {
		return controlmsg.NodeHelloAck{}, dto.NetworkMap{}, ErrForbidden
	}

	session, err := s.state.pg.GetControlSessionByToken(ctx, hello.SessionToken)
	if err != nil {
		if repo.IsNotFound(err) {
			return controlmsg.NodeHelloAck{}, dto.NetworkMap{}, ErrUnauthorized
		}
		return controlmsg.NodeHelloAck{}, dto.NetworkMap{}, err
	}
	if session.UserID != userID || session.NodeID != hello.NodeID || session.NetworkID != hello.NetworkID {
		return controlmsg.NodeHelloAck{}, dto.NetworkMap{}, ErrForbidden
	}
	if hello.DeviceID != "" && session.DeviceID != hello.DeviceID {
		return controlmsg.NodeHelloAck{}, dto.NetworkMap{}, ErrForbidden
	}
	s.state.touchControlSessionByToken(ctx, hello.SessionToken)
	if err := s.state.pg.UpdateDeviceStatus(ctx, session.DeviceID, "reachable"); err != nil {
		return controlmsg.NodeHelloAck{}, dto.NetworkMap{}, err
	}

	networkMap, err := s.NetworkMap(userID, hello.NodeID, hello.NetworkID)
	if err != nil {
		return controlmsg.NodeHelloAck{}, dto.NetworkMap{}, err
	}

	return controlmsg.NodeHelloAck{
		ControlSessionID: session.ControlSessionID,
		HeartbeatSeconds: networkMap.HeartbeatSeconds,
		NetworkRevision:  networkMap.Revision,
	}, networkMap, nil
}

// NetworkMap rebuilds the topology snapshot currently visible to the node.
func (s dbControlChannelService) NetworkMap(userID, nodeID, networkID string) (dto.NetworkMap, error) {
	ctx := context.Background()
	node, err := s.state.requireNodeSession(ctx, userID, nodeID, networkID)
	if err != nil {
		return dto.NetworkMap{}, err
	}
	return s.state.buildNetworkMap(ctx, userID, node.ToDTO(nil), networkID), nil
}

// Heartbeat refreshes the control-session freshness window for the node.
func (s dbControlChannelService) Heartbeat(userID, nodeID, networkID string) error {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(nodeID) == "" || strings.TrimSpace(networkID) == "" {
		return ErrUnauthorized
	}
	ctx := context.Background()
	if _, err := s.state.requireNodeSession(ctx, userID, nodeID, networkID); err != nil {
		return err
	}
	s.state.touchControlSessionByNode(ctx, nodeID, networkID)
	return nil
}

// CloseSession clears transient control-plane state for the node/network pair
// and marks the backing device offline.
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
	if sessions, err := s.state.pg.ListControlSessionsByNode(ctx, nodeID, networkID); err != nil {
		return err
	} else {
		s.state.deleteControlSessionTokens(ctx, sessions)
	}
	if err := s.state.pg.DeleteControlSessionByNode(ctx, nodeID, networkID); err != nil {
		return err
	}
	return s.state.markDeviceOfflineIfNoFreshControlSession(ctx, node.DeviceID, time.Now())
}

// PeerSnapshot returns the peer view that would appear in the current network
// map for the source node.
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
	if _, err := s.state.requireActiveNetworkMember(ctx, networkID, record.DeviceID, ErrForbidden, "peer node device"); err != nil {
		return dto.Peer{}, err
	}
	if _, err := s.state.requireActiveNetworkAttachment(ctx, networkID, record.DeviceID, ErrForbidden, "peer node device"); err != nil {
		return dto.Peer{}, err
	}
	device, err := s.state.pg.GetDeviceByID(ctx, record.DeviceID)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.Peer{}, ErrNotFound
		}
		return dto.Peer{}, err
	}
	if !s.state.hasFreshControlSession(ctx, record.NodeID, networkID, time.Now()) {
		return dto.Peer{}, ErrNotFound
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

// ConnectPlan builds a routing recommendation for the source node to reach the
// peer node.
func (s dbControlChannelService) ConnectPlan(userID, nodeID, networkID, peerNodeID string) (controlmsg.ConnectPlan, error) {
	return s.buildConnectPlan(context.Background(), userID, nodeID, networkID, peerNodeID)
}

// ConnectPlanByNode is a convenience wrapper for call sites that only know the
// source node id and not its owning user id.
func (s dbControlChannelService) ConnectPlanByNode(nodeID, networkID, peerNodeID string) (controlmsg.ConnectPlan, error) {
	ctx := context.Background()
	node, err := s.state.pg.GetNodeByID(ctx, nodeID)
	if err != nil {
		if repo.IsNotFound(err) {
			return controlmsg.ConnectPlan{}, ErrNotFound
		}
		return controlmsg.ConnectPlan{}, err
	}
	return s.ConnectPlan(node.UserID, nodeID, networkID, peerNodeID)
}
