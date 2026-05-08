package impl

import (
	"context"
	"strings"
	"time"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/configs"
	controlmsg "github.com/slan/server/server-biz/internal/controlmsg"
	"github.com/slan/server/server-biz/internal/repo"
)

// connectPlanInputs 汇总构建 connect plan 所需的全部即时输入。
type connectPlanInputs struct {
	peer            dto.Peer
	sourceNatType   string
	peerNatType     string
	connectionState repo.NodeConnectionState
	pathHealth      []pathHealthWindow
	relayCluster    relayClusterView
}

// buildConnectPlan gathers peer state, NAT information and recent health
// samples before delegating to the final plan composer.
func (s dbControlChannelService) buildConnectPlan(ctx context.Context, userID, nodeID, networkID, peerNodeID string) (controlmsg.ConnectPlan, error) {
	if _, err := s.state.requireNodeSession(ctx, userID, nodeID, networkID); err != nil {
		return controlmsg.ConnectPlan{}, err
	}

	peer, err := s.PeerSnapshot(userID, nodeID, networkID, peerNodeID)
	if err != nil {
		return controlmsg.ConnectPlan{}, err
	}

	inputs := connectPlanInputs{
		peer:            peer,
		sourceNatType:   s.nodeNatType(ctx, nodeID, networkID),
		peerNatType:     s.nodeNatType(ctx, peerNodeID, networkID),
		connectionState: s.connectionState(ctx, networkID, nodeID, peerNodeID),
		pathHealth:      s.pathHealth(ctx, networkID, nodeID, peerNodeID),
		relayCluster:    s.state.bestRelayCluster("", ""),
	}

	return s.composeConnectPlan(ctx, userID, nodeID, networkID, peerNodeID, inputs), nil
}

// composeConnectPlan decides path ordering, relay preference and ticket usage
// for the final control-plane response.
func (s dbControlChannelService) composeConnectPlan(ctx context.Context, userID, nodeID, networkID, peerNodeID string, inputs connectPlanInputs) controlmsg.ConnectPlan {
	paths, nextPriority := directPathOptions(inputs.peer.Endpoints, inputs.pathHealth)
	preferDirect, needRelayTicket := connectPlanPolicy(inputs.sourceNatType, inputs.peerNatType, inputs.connectionState, len(paths) > 0)

	preferredRelayNodeID := preferredRelayNodeID(inputs.pathHealth, inputs.connectionState)
	avoidedRelayNodeID := avoidedRelayNodeID(inputs.connectionState)
	relayCluster := s.relayClusterForPreferredNode(preferredRelayNodeID, avoidedRelayNodeID, inputs.relayCluster)
	var relayTicket *controlmsg.RelayTicket
	if needRelayTicket {
		relayTicket, relayCluster = s.relayTicketForConnectPlan(ctx, userID, nodeID, networkID, peerNodeID, relayCluster, inputs.connectionState)
	}

	relayPaths, preferredNodeIDs := relayPathOptions(relayCluster.nodes, preferredRelayNodeID, avoidedRelayNodeID, nextPriority)
	paths = append(paths, relayPaths...)

	return controlmsg.ConnectPlan{
		PeerNodeID:           peerNodeID,
		PreferDirect:         preferDirect,
		Paths:                paths,
		DerpClusterID:        relayCluster.clusterID,
		PreferredDerpNodeIDs: preferredNodeIDs,
		RelayTicket:          relayTicket,
	}
}

// relayClusterForPreferredNode aligns the chosen relay cluster with the best
// known relay node before falling back to the default cluster view.
func (s dbControlChannelService) relayClusterForPreferredNode(preferredRelayNodeID, avoidedRelayNodeID string, fallback relayClusterView) relayClusterView {
	cluster := s.state.bestRelayCluster(preferredRelayNodeID, avoidedRelayNodeID)
	if len(cluster.nodes) > 0 {
		return cluster
	}
	return fallback
}

func (s dbControlChannelService) relayTicketForConnectPlan(ctx context.Context, userID, nodeID, networkID, peerNodeID string, relayCluster relayClusterView, connectionState repo.NodeConnectionState) (*controlmsg.RelayTicket, relayClusterView) {
	ticket, err := s.state.issueRelayTicket(ctx, userID, dto.RelayTicketRequest{
		NetworkID:            networkID,
		SrcNodeID:            nodeID,
		DstNodeID:            peerNodeID,
		DerpClusterID:        relayCluster.clusterID,
		PreferredDerpNodeIDs: relayNodeIDs(relayCluster.nodes),
		Reason:               relayReason(connectionState),
	})
	if err != nil {
		return nil, relayCluster
	}

	return relayTicketDTOToControl(ticket), s.state.relayClusterForRequest(ticket.DerpClusterID)
}

func relayNodeIDs(nodes []configs.RelayNodeConfig) []string {
	out := make([]string, 0, len(nodes))
	for _, node := range nodes {
		if nodeID := strings.TrimSpace(node.NodeID); nodeID != "" {
			out = append(out, nodeID)
		}
	}
	return out
}

func relayTicketDTOToControl(ticket dto.RelayTicket) *controlmsg.RelayTicket {
	return &controlmsg.RelayTicket{
		TicketID:           ticket.TicketID,
		NetworkID:          ticket.NetworkID,
		SessionID:          ticket.SessionID,
		SrcNodeID:          ticket.SrcNodeID,
		DstNodeID:          ticket.DstNodeID,
		DerpClusterID:      ticket.DerpClusterID,
		CountryCode:        ticket.CountryCode,
		CityCode:           ticket.CityCode,
		AllowedDerpNodeIDs: append([]string(nil), ticket.AllowedDerpNodeIDs...),
		RelayURL:           ticket.RelayURL,
		ExpiresAt:          ticket.ExpiresAt,
		SessionKey:         ticket.SessionKey,
		Signature:          ticket.Signature,
	}
}

func (s dbControlChannelService) nodeNatType(ctx context.Context, nodeID, networkID string) string {
	now := time.Now()
	cutoff := endpointCutoffUnix(now)
	_ = s.state.pg.DeleteNodeEndpointsBefore(ctx, nodeID, networkID, cutoff)

	value, err := s.state.pg.GetNodeNatType(ctx, nodeID, networkID)
	if err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(value))
}

func (s dbControlChannelService) connectionState(ctx context.Context, networkID, nodeID, peerNodeID string) repo.NodeConnectionState {
	now := time.Now()
	cutoff := connectionStateCutoffUnix(now)
	_ = s.state.pg.DeleteNodeConnectionStatesBefore(ctx, nodeID, networkID, cutoff)

	value, err := s.state.pg.GetNodeConnectionState(ctx, networkID, nodeID, peerNodeID)
	if err != nil {
		return repo.NodeConnectionState{}
	}
	if value.UpdatedAt < cutoff {
		return repo.NodeConnectionState{}
	}
	return value
}

func (s dbControlChannelService) pathHealth(ctx context.Context, networkID, nodeID, peerNodeID string) []pathHealthWindow {
	now := time.Now()
	cutoff := pathHealthCutoffUnix(now)
	_ = s.state.pg.DeleteNodePathHealthBefore(ctx, nodeID, networkID, cutoff)

	values, err := s.state.pg.ListNodePathHealth(ctx, networkID, nodeID, peerNodeID)
	if err != nil {
		return nil
	}

	out := make([]repo.NodePathHealth, 0, len(values))
	for _, value := range values {
		if value.UpdatedAt < cutoff {
			continue
		}
		out = append(out, value)
	}
	return aggregatePathHealthWindow(out)
}
