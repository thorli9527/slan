package impl

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/slan/server/server-biz/internal/repo"
	"github.com/slan/server/server-biz/internal/service"
)

func (s dbControlChannelService) ActiveSessions(networkID, excludeNodeID string) ([]service.ControlSession, error) {
	ctx := context.Background()
	nodes, err := s.state.pg.ListNodesByNetwork(ctx, networkID)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	out := make([]service.ControlSession, 0, len(nodes))
	for _, node := range nodes {
		if node.NodeID == excludeNodeID {
			continue
		}
		session, err := s.state.pg.GetLatestControlSessionByNode(ctx, node.NodeID, networkID)
		if err != nil || !controlSessionIsFresh(session, now) {
			continue
		}
		out = append(out, service.ControlSession{
			UserID:    session.UserID,
			DeviceID:  session.DeviceID,
			NodeID:    session.NodeID,
			NetworkID: session.NetworkID,
		})
	}
	return out, nil
}

func (s dbControlChannelService) LatestSessionByDevice(deviceID string) (service.ControlSession, error) {
	ctx := context.Background()
	session, err := s.state.pg.GetLatestControlSessionByDevice(ctx, strings.TrimSpace(deviceID))
	if err != nil {
		return service.ControlSession{}, err
	}
	if !controlSessionIsFresh(session, time.Now()) {
		return service.ControlSession{}, ErrUnauthorized
	}
	return service.ControlSession{
		UserID:    session.UserID,
		DeviceID:  session.DeviceID,
		NodeID:    session.NodeID,
		NetworkID: session.NetworkID,
	}, nil
}

// requireNodeSession verifies ownership and network membership for a node.
func (s *dbState) requireNodeSession(ctx context.Context, userID, nodeID, networkID string) (repo.Node, error) {
	if strings.TrimSpace(nodeID) == "" || strings.TrimSpace(networkID) == "" {
		return repo.Node{}, fmt.Errorf("%w: nodeId and networkId are required", ErrInvalidArgument)
	}
	node, err := s.pg.GetNodeByID(ctx, nodeID)
	if err != nil {
		if repo.IsNotFound(err) {
			return repo.Node{}, ErrNotFound
		}
		return repo.Node{}, err
	}
	if node.UserID != userID {
		return repo.Node{}, ErrForbidden
	}
	if err := s.ensureNetworkAccess(ctx, userID, networkID); err != nil {
		return repo.Node{}, err
	}
	if _, err := s.requireActiveNetworkMember(ctx, networkID, node.DeviceID, ErrForbidden, "node device"); err != nil {
		return repo.Node{}, err
	}
	if _, err := s.requireActiveNetworkAttachment(ctx, networkID, node.DeviceID, ErrForbidden, "node device"); err != nil {
		return repo.Node{}, err
	}
	return node, nil
}

// touchControlSessionByToken refreshes the liveness timestamp of a known
// control-session token.
func (s *dbState) touchControlSessionByToken(ctx context.Context, sessionToken string) {
	if sessionToken == "" {
		return
	}
	_ = s.pg.TouchControlSessionByToken(ctx, sessionToken, time.Now().Unix())
}

func (s *dbState) deleteControlSessionTokens(ctx context.Context, sessions []repo.ControlSession) {
	if s.tokens == nil {
		return
	}
	for _, session := range sessions {
		if strings.TrimSpace(session.SessionToken) == "" {
			continue
		}
		_ = s.tokens.DeleteControlSessionToken(ctx, session.SessionToken)
	}
}

// touchControlSessionByNode refreshes the latest control session for a node.
func (s *dbState) touchControlSessionByNode(ctx context.Context, nodeID, networkID string) {
	if nodeID == "" || networkID == "" {
		return
	}
	_ = s.pg.TouchControlSessionByNode(ctx, nodeID, networkID, time.Now().Unix())
}

// hasFreshControlSession reports whether the node currently has a recent enough
// control session to appear online in control-plane views.
func (s *dbState) hasFreshControlSession(ctx context.Context, nodeID, networkID string, now time.Time) bool {
	session, err := s.pg.GetLatestControlSessionByNode(ctx, nodeID, networkID)
	if err != nil {
		return false
	}
	return controlSessionIsFresh(session, now)
}

func (s *dbState) hasFreshDeviceBoundWebSession(
	ctx context.Context,
	session repo.AccessTokenSession,
	now time.Time,
) bool {
	if session.UserID == "" || session.DeviceID == "" {
		return true
	}
	record, err := s.pg.GetLatestControlSessionByDevice(ctx, session.DeviceID)
	if err != nil {
		return true
	}
	if record.UserID != session.UserID {
		return false
	}
	return true
}

// markDeviceOfflineIfNoFreshControlSession avoids flipping a device offline
// while another node/network session for the same device is still fresh.
func (s *dbState) markDeviceOfflineIfNoFreshControlSession(ctx context.Context, deviceID string, now time.Time) error {
	session, err := s.pg.GetLatestControlSessionByDevice(ctx, deviceID)
	if err == nil && controlSessionIsFresh(session, now) {
		return nil
	}
	if err != nil && !repo.IsNotFound(err) {
		return err
	}
	return s.pg.UpdateDeviceStatus(ctx, deviceID, "offline")
}

// controlSessionIsFresh applies the in-memory freshness window to a stored
// control session row.
func controlSessionIsFresh(session repo.ControlSession, now time.Time) bool {
	if session.LastSeenAt <= 0 {
		return false
	}
	return now.Unix()-session.LastSeenAt <= int64(controlSessionFreshnessWindow/time.Second)
}
