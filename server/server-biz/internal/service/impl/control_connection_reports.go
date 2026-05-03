package impl

import (
	"context"
	"fmt"
	"strings"
	"time"

	controlmsg "github.com/slan/server/server-biz/internal/controlmsg"
	"github.com/slan/server/server-biz/internal/repo"
	"github.com/slan/server/server-biz/internal/util"
)

// ReportConnectionState stores the latest path outcome reported by the client
// for a specific peer pair.
func (s dbControlChannelService) ReportConnectionState(userID, nodeID string, state controlmsg.ConnectionState) error {
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
	if _, err := s.state.requireActiveNetworkMember(ctx, state.NetworkID, peerNode.DeviceID, ErrForbidden, "peer node device"); err != nil {
		return err
	}
	if _, err := s.state.requireActiveNetworkAttachment(ctx, state.NetworkID, peerNode.DeviceID, ErrForbidden, "peer node device"); err != nil {
		return err
	}
	return s.state.pg.UpsertNodeConnectionState(ctx, repo.NodeConnectionState{
		StateID:       util.NewID("conn"),
		NetworkID:     state.NetworkID,
		NodeID:        sourceNode.NodeID,
		PeerNodeID:    state.PeerNodeID,
		Path:          state.Path,
		State:         state.State,
		Reason:        state.Reason,
		ObservedRttMs: state.ObservedRttMs,
		PacketLossPpm: state.PacketLossPpm,
		PathScore:     state.PathScore,
		DerpNodeID:    strings.TrimSpace(state.DerpNodeID),
		UpdatedAt:     time.Now().Unix(),
	})
}

// Disconnect records a closed connection state and marks the device offline.
func (s dbControlChannelService) Disconnect(userID, nodeID string, notice controlmsg.DisconnectNotice) error {
	if strings.TrimSpace(nodeID) == "" || strings.TrimSpace(notice.NetworkID) == "" || strings.TrimSpace(notice.PeerNodeID) == "" {
		return fmt.Errorf("%w: nodeId, networkId, and peerNodeId are required", ErrInvalidArgument)
	}

	ctx := context.Background()
	node, err := s.state.requireNodeSession(ctx, userID, nodeID, notice.NetworkID)
	if err != nil {
		return err
	}
	if err := s.state.pg.UpsertNodeConnectionState(ctx, repo.NodeConnectionState{
		StateID:    util.NewID("conn"),
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
