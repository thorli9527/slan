package impl

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/configs"
	"github.com/slan/server/server-biz/internal/repo"
	"github.com/slan/server/server-biz/internal/util"
)

// IssueRelayTicket signs or reuses a relay ticket for a source/destination pair.
func (s dbBootstrapService) IssueRelayTicket(userID string, req dto.RelayTicketRequest) (dto.RelayTicket, error) {
	return s.state.issueRelayTicket(context.Background(), userID, req)
}

// issueRelayTicket validates access, picks the relay cluster and either reuses
// a cached ticket or signs a fresh one.
func (s *dbState) issueRelayTicket(ctx context.Context, userID string, req dto.RelayTicketRequest) (dto.RelayTicket, error) {
	if strings.TrimSpace(req.NetworkID) == "" || strings.TrimSpace(req.SrcNodeID) == "" || strings.TrimSpace(req.DstNodeID) == "" || strings.TrimSpace(req.Reason) == "" {
		return dto.RelayTicket{}, fmt.Errorf("%w: networkId, srcNodeId, dstNodeId, and reason are required", ErrInvalidArgument)
	}
	srcNode, err := s.pg.GetNodeByID(ctx, req.SrcNodeID)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.RelayTicket{}, ErrNotFound
		}
		return dto.RelayTicket{}, err
	}
	if srcNode.UserID != userID {
		return dto.RelayTicket{}, ErrForbidden
	}
	if err := s.ensureNetworkAccess(ctx, userID, req.NetworkID); err != nil {
		return dto.RelayTicket{}, err
	}
	dstNode, err := s.pg.GetNodeByID(ctx, req.DstNodeID)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.RelayTicket{}, ErrNotFound
		}
		return dto.RelayTicket{}, err
	}
	if _, err := s.requireActiveNetworkMember(ctx, req.NetworkID, srcNode.DeviceID, ErrForbidden, "source node device"); err != nil {
		return dto.RelayTicket{}, err
	}
	if _, err := s.requireActiveNetworkAttachment(ctx, req.NetworkID, srcNode.DeviceID, ErrForbidden, "source node device"); err != nil {
		return dto.RelayTicket{}, err
	}
	if _, err := s.requireActiveNetworkMember(ctx, req.NetworkID, dstNode.DeviceID, ErrNotFound, "destination node device"); err != nil {
		return dto.RelayTicket{}, err
	}
	if _, err := s.requireActiveNetworkAttachment(ctx, req.NetworkID, dstNode.DeviceID, ErrNotFound, "destination node device"); err != nil {
		return dto.RelayTicket{}, err
	}
	cluster := s.relayClusterForRequest(req.DerpClusterID)
	if len(cluster.nodes) == 0 {
		return dto.RelayTicket{}, fmt.Errorf("%w: no relay nodes configured", ErrNotFound)
	}
	req = normalizeRelayTicketRequest(req, cluster)
	if ticket, ok := s.relayTicketFromCache(req); ok {
		return ticket, nil
	}
	ticket := s.buildRelayTicket(req, cluster, time.Now().Add(10*time.Minute).UTC())
	s.storeRelayTicket(req, ticket)
	return ticket, nil
}

// buildRelayTicket materializes the relay ticket payload later verified by
// server-relay.
func (s *dbState) buildRelayTicket(req dto.RelayTicketRequest, cluster relayClusterView, expiresAt time.Time) dto.RelayTicket {
	primaryNode := relayTicketPrimaryNode(req, cluster.nodes)
	ticket := dto.RelayTicket{
		TicketID:           util.NewID("ticket"),
		NetworkID:          req.NetworkID,
		SessionID:          util.NewID("session"),
		SrcNodeID:          req.SrcNodeID,
		DstNodeID:          req.DstNodeID,
		DerpClusterID:      cluster.clusterID,
		CountryCode:        cluster.countryCode,
		CityCode:           cluster.cityCode,
		AllowedDerpNodeIDs: append([]string(nil), req.PreferredDerpNodeIDs...),
		RelayURL:           primaryNode.Transport + "://" + primaryNode.Address,
		ExpiresAt:          expiresAt.Format(time.RFC3339),
	}
	ticket.SessionKey = s.newRelaySessionKey(ticket.TicketID, ticket.SrcNodeID, ticket.DstNodeID, expiresAt)
	ticket.Signature = s.signRelayTicket(ticket)
	return ticket
}

func relayTicketPrimaryNode(req dto.RelayTicketRequest, nodes []configs.RelayNodeConfig) configs.RelayNodeConfig {
	for _, preferredNodeID := range req.PreferredDerpNodeIDs {
		preferredNodeID = strings.TrimSpace(preferredNodeID)
		if preferredNodeID == "" {
			continue
		}
		for _, node := range nodes {
			if node.NodeID == preferredNodeID {
				return node
			}
		}
	}
	return nodes[0]
}

// newRelaySessionKey generates the opaque session secret embedded in relay
// tickets.
func (s *dbState) newRelaySessionKey(ticketID, srcNodeID, dstNodeID string, expiresAt time.Time) string {
	return util.OpaqueToken(
		"relay-session",
		fmt.Sprintf("%s:%s:%s:%d", ticketID, srcNodeID, dstNodeID, expiresAt.UTC().Unix()),
	)
}

// signRelayTicket computes the HMAC signature shared with server-relay.
func (s *dbState) signRelayTicket(ticket dto.RelayTicket) string {
	mac := hmac.New(sha256.New, []byte(s.cfg.Relay.TicketSigningSecret))
	_, _ = mac.Write([]byte(relayTicketSigningPayload(ticket)))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// relayTicketSigningPayload must remain stable across biz and relay so both
// sides hash the same canonical ticket body.
func relayTicketSigningPayload(ticket dto.RelayTicket) string {
	parts := []string{
		ticket.TicketID,
		ticket.NetworkID,
		ticket.SessionID,
		ticket.SrcNodeID,
		ticket.DstNodeID,
		ticket.DerpClusterID,
		ticket.CountryCode,
		ticket.CityCode,
		strings.Join(util.SortedStrings(ticket.AllowedDerpNodeIDs), ","),
		ticket.RelayURL,
		ticket.ExpiresAt,
		ticket.SessionKey,
	}
	return strings.Join(parts, "|")
}

// normalizeRelayTicketRequest fills in omitted cluster/node preferences and
// strips invalid node ids before caching or signing.
func normalizeRelayTicketRequest(req dto.RelayTicketRequest, cluster relayClusterView) dto.RelayTicketRequest {
	req.DerpClusterID = util.FirstNonEmpty(strings.TrimSpace(req.DerpClusterID), cluster.clusterID)
	req.PreferredDerpNodeIDs = normalizePreferredRelayNodeIDs(req.PreferredDerpNodeIDs, cluster.nodes)
	return req
}

// normalizePreferredRelayNodeIDs keeps only nodes that belong to the selected
// cluster and returns them in a stable order.
func normalizePreferredRelayNodeIDs(preferredNodeIDs []string, clusterNodes []configs.RelayNodeConfig) []string {
	if len(clusterNodes) == 0 {
		return nil
	}

	allowed := make(map[string]struct{}, len(clusterNodes))
	defaultNodeIDs := make([]string, 0, len(clusterNodes))
	for _, node := range clusterNodes {
		if strings.TrimSpace(node.NodeID) == "" {
			continue
		}
		allowed[node.NodeID] = struct{}{}
		defaultNodeIDs = append(defaultNodeIDs, node.NodeID)
	}

	if len(preferredNodeIDs) == 0 {
		sort.Strings(defaultNodeIDs)
		return defaultNodeIDs
	}

	seen := make(map[string]struct{}, len(preferredNodeIDs))
	normalized := make([]string, 0, len(preferredNodeIDs))
	for _, nodeID := range preferredNodeIDs {
		nodeID = strings.TrimSpace(nodeID)
		if nodeID == "" {
			continue
		}
		if _, ok := allowed[nodeID]; !ok {
			continue
		}
		if _, ok := seen[nodeID]; ok {
			continue
		}
		seen[nodeID] = struct{}{}
		normalized = append(normalized, nodeID)
	}

	return normalized
}
