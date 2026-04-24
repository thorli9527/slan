package impl

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
	"github.com/slan/server/server-biz/internal/service"
	"github.com/slan/server/server-biz/internal/util"
	controlws "github.com/slan/server/server-biz/internal/ws"
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
// online, and returns the latest network map snapshot for the WS session.
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
	s.state.touchControlSessionByToken(ctx, hello.SessionToken)
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

// NetworkMap rebuilds the topology snapshot currently visible to the node.
func (s dbControlChannelService) NetworkMap(userID, nodeID, networkID string) (dto.NetworkMap, error) {
	ctx := context.Background()
	node, err := s.state.requireNodeSession(ctx, userID, nodeID, networkID)
	if err != nil {
		return dto.NetworkMap{}, err
	}
	return s.state.buildNetworkMap(ctx, userID, node.ToDTO(nil), networkID), nil
}

// ReportEndpoints replaces the node's advertised endpoints and NAT observation
// for the target network, then returns an updated network map.
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
			EndpointID: util.NewID("ep"),
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

// ReportConnectionState stores the latest path outcome reported by the client
// for a specific peer pair.
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

// ReportPathHealth persists direct or relay path quality samples that later
// influence connect-plan sorting.
func (s dbControlChannelService) ReportPathHealth(userID, nodeID string, report controlws.PathHealthReport) error {
	if strings.TrimSpace(report.NetworkID) == "" || strings.TrimSpace(report.PeerNodeID) == "" || strings.TrimSpace(report.PathType) == "" {
		return fmt.Errorf("%w: networkId, peerNodeId, and pathType are required", ErrInvalidArgument)
	}

	ctx := context.Background()
	sourceNode, err := s.state.requireNodeSession(ctx, userID, nodeID, report.NetworkID)
	if err != nil {
		return err
	}
	peerNode, err := s.state.pg.GetNodeByID(ctx, report.PeerNodeID)
	if err != nil {
		if repo.IsNotFound(err) {
			return ErrNotFound
		}
		return err
	}
	if _, err := s.state.requireActiveNetworkMember(ctx, report.NetworkID, peerNode.DeviceID, ErrForbidden, "peer node device"); err != nil {
		return err
	}
	if _, err := s.state.requireActiveNetworkAttachment(ctx, report.NetworkID, peerNode.DeviceID, ErrForbidden, "peer node device"); err != nil {
		return err
	}

	sampledAtMs := report.SampledAtMs
	if sampledAtMs == 0 {
		sampledAtMs = uint64(time.Now().UnixMilli())
	}

	return s.state.pg.UpsertNodePathHealth(ctx, repo.NodePathHealth{
		HealthID:      util.NewID("path"),
		NetworkID:     report.NetworkID,
		NodeID:        sourceNode.NodeID,
		PeerNodeID:    report.PeerNodeID,
		PathType:      strings.TrimSpace(report.PathType),
		Endpoint:      strings.TrimSpace(report.Endpoint),
		DerpNodeID:    strings.TrimSpace(report.DerpNodeID),
		ObservedRttMs: report.ObservedRttMs,
		PacketLossPpm: report.PacketLossPpm,
		PathScore:     report.PathScore,
		SampledAtMs:   sampledAtMs,
		UpdatedAt:     time.Now().Unix(),
	})
}

// Disconnect records a closed connection state and marks the device offline.
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
func (s dbControlChannelService) ConnectPlan(userID, nodeID, networkID, peerNodeID string) (controlws.ConnectPlan, error) {
	return s.buildConnectPlan(context.Background(), userID, nodeID, networkID, peerNodeID)
}

// ConnectPlanByNode is a convenience wrapper for call sites that only know the
// source node id and not its owning user id.
func (s dbControlChannelService) ConnectPlanByNode(nodeID, networkID, peerNodeID string) (controlws.ConnectPlan, error) {
	ctx := context.Background()
	node, err := s.state.pg.GetNodeByID(ctx, nodeID)
	if err != nil {
		if repo.IsNotFound(err) {
			return controlws.ConnectPlan{}, ErrNotFound
		}
		return controlws.ConnectPlan{}, err
	}
	return s.ConnectPlan(node.UserID, nodeID, networkID, peerNodeID)
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

// Publish emits a control-sync event into the shared pub/sub channel.
func (s dbControlSyncService) Publish(event controlws.ControlSyncEvent) error {
	return s.state.tokens.PublishControlSyncEvent(context.Background(), event)
}

// Subscribe registers a callback that receives control-sync events published by
// any biz instance.
func (s dbControlSyncService) Subscribe(handler func(controlws.ControlSyncEvent)) error {
	return s.state.tokens.SubscribeControlSyncEvents(context.Background(), handler)
}

// NextRevision increments and returns the current network revision counter.
func (s dbControlSyncService) NextRevision(networkID string) (uint64, error) {
	return s.state.tokens.NextNetworkRevision(context.Background(), networkID)
}

// CurrentRevision reads the current network revision without mutating it.
func (s dbControlSyncService) CurrentRevision(networkID string) (uint64, error) {
	return s.state.tokens.CurrentNetworkRevision(context.Background(), networkID)
}

// AcquireConnectPlanRetry provides a retry window so the same pair is not
// continuously re-planned.
func (s dbControlSyncService) AcquireConnectPlanRetry(networkID, nodeID, peerNodeID string) (bool, error) {
	return s.state.tokens.AcquireConnectPlanRetry(context.Background(), networkID, nodeID, peerNodeID)
}

// ResetConnectPlanRetry clears the retry gate after successful delivery or
// when the pending retry is no longer relevant.
func (s dbControlSyncService) ResetConnectPlanRetry(networkID, nodeID, peerNodeID string) error {
	return s.state.tokens.ResetConnectPlanRetry(context.Background(), networkID, nodeID, peerNodeID)
}

// AcquirePeerCandidateDelivery de-duplicates repeated candidate forwarding for
// a short TTL window.
func (s dbControlSyncService) AcquirePeerCandidateDelivery(networkID, sourceNodeID, targetNodeID string, candidate controlws.PeerCandidate, ttl time.Duration) (bool, error) {
	return s.state.tokens.AcquirePeerCandidateDelivery(context.Background(), networkID, sourceNodeID, targetNodeID, candidate, ttl)
}

// relayTicketFromCache returns a still-valid cached relay ticket for the exact
// same request dimensions.
func (s *dbState) relayTicketFromCache(req dto.RelayTicketRequest) (dto.RelayTicket, bool) {
	key := relayTicketCacheKey(req)
	now := time.Now()

	s.controlMu.Lock()
	defer s.controlMu.Unlock()

	cached, ok := s.cachedRelayTicket[key]
	if !ok {
		return dto.RelayTicket{}, false
	}
	if !cached.validUntil.After(now) {
		delete(s.cachedRelayTicket, key)
		return dto.RelayTicket{}, false
	}
	return cached.ticket, true
}

// storeRelayTicket caches a relay ticket until shortly before its expiry time.
func (s *dbState) storeRelayTicket(req dto.RelayTicketRequest, ticket dto.RelayTicket) {
	expiresAt, err := time.Parse(time.RFC3339, ticket.ExpiresAt)
	if err != nil {
		return
	}

	validUntil := expiresAt.Add(-time.Minute)
	now := time.Now()
	if !validUntil.After(now) {
		validUntil = now.Add(30 * time.Second)
	}

	s.controlMu.Lock()
	defer s.controlMu.Unlock()
	s.cachedRelayTicket[relayTicketCacheKey(req)] = cachedRelayTicket{
		ticket:     ticket,
		validUntil: validUntil,
	}
}

// pruneExpiredRelayTickets clears stale relay tickets from the in-memory cache.
func (s *dbState) pruneExpiredRelayTickets(now time.Time) {
	s.controlMu.Lock()
	defer s.controlMu.Unlock()

	for key, cached := range s.cachedRelayTicket {
		if !cached.validUntil.After(now) {
			delete(s.cachedRelayTicket, key)
		}
	}
}

// relayTicketCacheKey canonicalizes request fields that affect the signed
// ticket so the cache remains stable across equivalent input ordering.
func relayTicketCacheKey(req dto.RelayTicketRequest) string {
	preferredNodeIDs := append([]string(nil), req.PreferredDerpNodeIDs...)
	sort.Strings(preferredNodeIDs)
	return strings.Join([]string{
		req.NetworkID,
		req.SrcNodeID,
		req.DstNodeID,
		req.DerpClusterID,
		strings.Join(preferredNodeIDs, ","),
		req.Reason,
	}, "|")
}

const controlSessionFreshnessWindow = 45 * time.Second
const deviceBoundWebSessionFreshnessWindow = 2 * time.Minute
const nodeEndpointFreshnessWindow = 2 * time.Minute
const nodeConnectionStateFreshnessWindow = 2 * time.Minute
const nodePathHealthFreshnessWindow = 5 * time.Minute
const controlStateCleanupInterval = 1 * time.Minute

// touchControlSessionByToken refreshes the liveness timestamp of a known
// control-session token.
func (s *dbState) touchControlSessionByToken(ctx context.Context, sessionToken string) {
	if sessionToken == "" {
		return
	}
	_ = s.pg.TouchControlSessionByToken(ctx, sessionToken, time.Now().Unix())
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
	issuedAt := time.Unix(session.IssuedAt, 0)
	if session.IssuedAt <= 0 {
		issuedAt = now
	}
	record, err := s.pg.GetLatestControlSessionByDevice(ctx, session.DeviceID)
	if err != nil {
		return now.Sub(issuedAt) <= deviceBoundWebSessionFreshnessWindow
	}
	if record.UserID != session.UserID {
		return false
	}
	lastSeenAt := time.Unix(record.LastSeenAt, 0)
	if lastSeenAt.Before(issuedAt) {
		lastSeenAt = issuedAt
	}
	return now.Sub(lastSeenAt) <= deviceBoundWebSessionFreshnessWindow
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

// endpointCutoffUnix returns the oldest endpoint timestamp still trusted by the
// control plane.
func endpointCutoffUnix(now time.Time) int64 {
	return now.Add(-nodeEndpointFreshnessWindow).Unix()
}

// connectionStateCutoffUnix returns the oldest connection-state timestamp still
// considered for planning.
func connectionStateCutoffUnix(now time.Time) int64 {
	return now.Add(-nodeConnectionStateFreshnessWindow).Unix()
}

// pathHealthCutoffUnix returns the oldest path-health sample still considered
// useful for ranking.
func pathHealthCutoffUnix(now time.Time) int64 {
	return now.Add(-nodePathHealthFreshnessWindow).Unix()
}

// controlSessionCutoffUnix returns the oldest last-seen timestamp still treated
// as an active control session.
func controlSessionCutoffUnix(now time.Time) int64 {
	return now.Add(-controlSessionFreshnessWindow).Unix()
}

// startControlStateCleanupLoop starts the background janitor that periodically
// trims transient control-plane state.
func (s *dbState) startControlStateCleanupLoop() {
	go func() {
		ticker := time.NewTicker(controlStateCleanupInterval)
		defer ticker.Stop()

		for range ticker.C {
			s.cleanupExpiredControlPlaneState(context.Background(), time.Now())
		}
	}()
}

// cleanupExpiredControlPlaneState performs one cleanup pass over transient
// control-plane rows and in-memory relay ticket cache.
func (s *dbState) cleanupExpiredControlPlaneState(ctx context.Context, now time.Time) {
	_ = s.pg.MarkDevicesOfflineWithoutFreshControlSession(ctx, controlSessionCutoffUnix(now))
	_ = s.pg.DeleteControlSessionsBefore(ctx, controlSessionCutoffUnix(now))
	_ = s.pg.DeleteNodeEndpointsBeforeAll(ctx, endpointCutoffUnix(now))
	_ = s.pg.DeleteNodeConnectionStatesBeforeAll(ctx, connectionStateCutoffUnix(now))
	_ = s.pg.DeleteNodePathHealthBeforeAll(ctx, pathHealthCutoffUnix(now))
	s.pruneExpiredRelayTickets(now)
}
