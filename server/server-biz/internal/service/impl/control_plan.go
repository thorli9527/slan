package impl

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/configs"
	"github.com/slan/server/server-biz/internal/repo"
	controlws "github.com/slan/server/server-biz/internal/ws"
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

// endpointRank is the normalized comparison tuple used to order direct
// endpoints from best to worst.
// endpointRank 是 direct endpoint 排序时使用的归一化比较值。
type endpointRank struct {
	score     float64
	rttMs     float64
	hasScore  bool
	hasRtt    bool
	sampledAt uint64
	samples   int
}

// pathHealthWindow is the aggregated recent sample view for one logical path
// candidate, such as a direct endpoint or relay node.
// pathHealthWindow 是一条逻辑路径在窗口期内聚合后的健康视图。
type pathHealthWindow struct {
	pathType   string
	endpoint   string
	derpNodeID string
	scoreAvg   float64
	rttAvg     float64
	hasScore   bool
	hasRtt     bool
	sampledAt  uint64
	samples    int
}

// buildConnectPlan gathers peer state, NAT information and recent health
// samples before delegating to the final plan composer.
func (s dbControlChannelService) buildConnectPlan(ctx context.Context, userID, nodeID, networkID, peerNodeID string) (controlws.ConnectPlan, error) {
	if _, err := s.state.requireNodeSession(ctx, userID, nodeID, networkID); err != nil {
		return controlws.ConnectPlan{}, err
	}

	peer, err := s.PeerSnapshot(userID, nodeID, networkID, peerNodeID)
	if err != nil {
		return controlws.ConnectPlan{}, err
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
func (s dbControlChannelService) composeConnectPlan(ctx context.Context, userID, nodeID, networkID, peerNodeID string, inputs connectPlanInputs) controlws.ConnectPlan {
	paths, nextPriority := directPathOptions(inputs.peer.Endpoints, inputs.pathHealth)
	preferDirect, needRelayTicket := connectPlanPolicy(inputs.sourceNatType, inputs.peerNatType, inputs.connectionState, len(paths) > 0)

	preferredRelayNodeID := preferredRelayNodeID(inputs.pathHealth, inputs.connectionState)
	avoidedRelayNodeID := avoidedRelayNodeID(inputs.connectionState)
	relayCluster := s.relayClusterForPreferredNode(preferredRelayNodeID, avoidedRelayNodeID, inputs.relayCluster)
	var relayTicket *controlws.RelayTicket
	if needRelayTicket {
		relayTicket, relayCluster = s.relayTicketForConnectPlan(ctx, userID, nodeID, networkID, peerNodeID, relayCluster, inputs.connectionState)
	}

	relayPaths, preferredNodeIDs := relayPathOptions(relayCluster.nodes, preferredRelayNodeID, avoidedRelayNodeID, nextPriority)
	paths = append(paths, relayPaths...)

	return controlws.ConnectPlan{
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

// directPathOptions converts visible non-relay endpoints into ordered path
// options and returns the next priority slot for relay paths.
func directPathOptions(endpoints []dto.Endpoint, pathHealth []pathHealthWindow) ([]controlws.PathOption, int) {
	sortedEndpoints := prioritizeDirectEndpoints(endpoints, pathHealth)
	paths := make([]controlws.PathOption, 0, len(sortedEndpoints)+1)
	priority := 10
	for _, endpoint := range sortedEndpoints {
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
	return paths, priority
}

// prioritizeDirectEndpoints reorders direct endpoints using recent path-health
// samples while keeping a stable fallback order when there is no signal.
func prioritizeDirectEndpoints(endpoints []dto.Endpoint, pathHealth []pathHealthWindow) []dto.Endpoint {
	if len(endpoints) <= 1 || len(pathHealth) == 0 {
		return append([]dto.Endpoint(nil), endpoints...)
	}

	bestByEndpoint := make(map[string]endpointRank, len(pathHealth))
	for _, health := range pathHealth {
		if !isDirectPathType(health.pathType) || strings.TrimSpace(health.endpoint) == "" {
			continue
		}
		rank := endpointRank{sampledAt: health.sampledAt, samples: health.samples}
		if health.hasScore {
			rank.score = health.scoreAvg
			rank.hasScore = true
		}
		if health.hasRtt {
			rank.rttMs = health.rttAvg
			rank.hasRtt = true
		}
		current, ok := bestByEndpoint[health.endpoint]
		if !ok || betterEndpointRank(rank, current) {
			bestByEndpoint[health.endpoint] = rank
		}
	}

	out := append([]dto.Endpoint(nil), endpoints...)
	sort.SliceStable(out, func(i, j int) bool {
		left, leftOK := bestByEndpoint[out[i].Address]
		right, rightOK := bestByEndpoint[out[j].Address]
		switch {
		case leftOK && !rightOK:
			return true
		case !leftOK && rightOK:
			return false
		case !leftOK && !rightOK:
			return false
		default:
			return betterEndpointRank(left, right)
		}
	})
	return out
}

// betterEndpointRank compares two aggregated direct endpoint scores and returns
// whether the left endpoint should be tried first.
func betterEndpointRank(left, right endpointRank) bool {
	switch {
	case left.hasScore && right.hasScore && left.score != right.score:
		return left.score > right.score
	case left.hasScore != right.hasScore:
		return left.hasScore
	case left.hasRtt && right.hasRtt && left.rttMs != right.rttMs:
		return left.rttMs < right.rttMs
	case left.hasRtt != right.hasRtt:
		return left.hasRtt
	case left.samples != right.samples:
		return left.samples > right.samples
	default:
		return left.sampledAt > right.sampledAt
	}
}

// connectPlanPolicy decides whether direct paths should still be preferred and
// whether a relay ticket must be prepared eagerly.
func connectPlanPolicy(sourceNatType, peerNatType string, connectionState repo.NodeConnectionState, hasDirectPaths bool) (preferDirect bool, needRelayTicket bool) {
	preferDirect = hasDirectPaths
	if sourceNatType == "symmetric" || peerNatType == "symmetric" {
		preferDirect = false
		needRelayTicket = true
	}
	if connectionState.State == "failed" || connectionState.State == "closed" {
		preferDirect = false
		needRelayTicket = true
	}
	return preferDirect, needRelayTicket
}

// relayPathOptions converts ordered relay nodes into connect-plan path options
// and returns the preferred relay node id list alongside them.
func relayPathOptions(nodes []configs.RelayNodeConfig, preferredNodeID, avoidedNodeID string, startPriority int) ([]controlws.PathOption, []string) {
	nodes = prioritizeRelayNodes(nodes, preferredNodeID, avoidedNodeID, nil)
	paths := make([]controlws.PathOption, 0, len(nodes))
	preferredNodeIDs := make([]string, 0, len(nodes))
	priority := startPriority
	for _, node := range nodes {
		paths = append(paths, controlws.PathOption{
			PathType: "relay_" + node.Transport,
			Endpoint: node.Address,
			Priority: priority,
		})
		priority += 10
		preferredNodeIDs = append(preferredNodeIDs, node.NodeID)
	}
	return paths, preferredNodeIDs
}

// prioritizeRelayNodes ranks relay nodes using explicit preference, avoided
// nodes, live health data and static config priority.
func prioritizeRelayNodes(nodes []configs.RelayNodeConfig, preferredNodeID, avoidedNodeID string, health map[string]relayNodeRank) []configs.RelayNodeConfig {
	out := append([]configs.RelayNodeConfig(nil), nodes...)
	sort.SliceStable(out, func(i, j int) bool {
		leftPreferred := out[i].NodeID == preferredNodeID
		rightPreferred := out[j].NodeID == preferredNodeID
		switch {
		case leftPreferred && !rightPreferred:
			return true
		case !leftPreferred && rightPreferred:
			return false
		}

		leftAvoided := out[i].NodeID == avoidedNodeID
		rightAvoided := out[j].NodeID == avoidedNodeID
		switch {
		case !leftAvoided && rightAvoided:
			return true
		case leftAvoided && !rightAvoided:
			return false
		}

		leftHealth, leftOK := health[out[i].NodeID]
		rightHealth, rightOK := health[out[j].NodeID]
		switch {
		case leftOK && !rightOK:
			return true
		case !leftOK && rightOK:
			return false
		case leftOK && rightOK && betterRelayNodeRank(leftHealth, rightHealth):
			return true
		case leftOK && rightOK && betterRelayNodeRank(rightHealth, leftHealth):
			return false
		}

		if out[i].Priority != out[j].Priority {
			return out[i].Priority < out[j].Priority
		}
		return out[i].NodeID < out[j].NodeID
	})
	return out
}

func isDirectPathType(pathType string) bool {
	switch strings.TrimSpace(pathType) {
	case "lan", "wan", "reflexive", "p2p", "direct":
		return true
	default:
		return false
	}
}

func (s dbControlChannelService) relayTicketForConnectPlan(ctx context.Context, userID, nodeID, networkID, peerNodeID string, relayCluster relayClusterView, connectionState repo.NodeConnectionState) (*controlws.RelayTicket, relayClusterView) {
	ticket, err := s.state.issueRelayTicket(ctx, userID, dto.RelayTicketRequest{
		NetworkID:     networkID,
		SrcNodeID:     nodeID,
		DstNodeID:     peerNodeID,
		DerpClusterID: relayCluster.clusterID,
		Reason:        relayReason(connectionState),
	})
	if err != nil {
		return nil, relayCluster
	}

	return relayTicketDTOToWS(ticket), s.state.relayClusterForRequest(ticket.DerpClusterID)
}

func relayTicketDTOToWS(ticket dto.RelayTicket) *controlws.RelayTicket {
	return &controlws.RelayTicket{
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

func relayReason(state repo.NodeConnectionState) string {
	if strings.TrimSpace(state.Reason) != "" {
		return state.Reason
	}
	if state.State == "failed" || state.State == "closed" {
		return state.State
	}
	return "nat_restricted"
}

func preferredRelayNodeIDFromHistory(state repo.NodeConnectionState) string {
	if state.State != "connected" {
		return ""
	}
	if strings.TrimSpace(state.Path) != "relay" && strings.TrimSpace(state.Path) != "derp" {
		return ""
	}
	return strings.TrimSpace(state.DerpNodeID)
}

func preferredRelayNodeID(pathHealth []pathHealthWindow, state repo.NodeConnectionState) string {
	if nodeID := preferredRelayNodeIDFromPathHealth(pathHealth); nodeID != "" {
		return nodeID
	}
	return preferredRelayNodeIDFromHistory(state)
}

func avoidedRelayNodeID(state repo.NodeConnectionState) string {
	switch strings.TrimSpace(state.State) {
	case "failed", "closed":
		return strings.TrimSpace(state.DerpNodeID)
	default:
		return ""
	}
}

func preferredRelayNodeIDFromPathHealth(pathHealth []pathHealthWindow) string {
	var best pathHealthWindow
	found := false
	for _, health := range pathHealth {
		if strings.TrimSpace(health.derpNodeID) == "" {
			continue
		}
		pathType := strings.TrimSpace(health.pathType)
		if pathType != "relay" && pathType != "derp" && !strings.HasPrefix(pathType, "relay_") {
			continue
		}
		if !found || betterPathHealth(health, best) {
			best = health
			found = true
		}
	}
	if !found {
		return ""
	}
	return strings.TrimSpace(best.derpNodeID)
}

func betterPathHealth(left, right pathHealthWindow) bool {
	switch {
	case left.hasScore && right.hasScore && left.scoreAvg != right.scoreAvg:
		return left.scoreAvg > right.scoreAvg
	case left.hasScore != right.hasScore:
		return left.hasScore
	case left.hasRtt && right.hasRtt && left.rttAvg != right.rttAvg:
		return left.rttAvg < right.rttAvg
	case left.hasRtt != right.hasRtt:
		return left.hasRtt
	case left.samples != right.samples:
		return left.samples > right.samples
	case left.sampledAt != right.sampledAt:
		return left.sampledAt > right.sampledAt
	default:
		return false
	}
}

func aggregatePathHealthWindow(values []repo.NodePathHealth) []pathHealthWindow {
	type accumulator struct {
		pathHealthWindow
		scoreSum   float64
		scoreCount int
		rttSum     float64
		rttCount   int
	}

	grouped := make(map[string]*accumulator, len(values))
	for _, value := range values {
		key := strings.Join([]string{
			strings.TrimSpace(value.PathType),
			strings.TrimSpace(value.Endpoint),
			strings.TrimSpace(value.DerpNodeID),
		}, "|")
		acc, ok := grouped[key]
		if !ok {
			acc = &accumulator{
				pathHealthWindow: pathHealthWindow{
					pathType:   strings.TrimSpace(value.PathType),
					endpoint:   strings.TrimSpace(value.Endpoint),
					derpNodeID: strings.TrimSpace(value.DerpNodeID),
				},
			}
			grouped[key] = acc
		}
		acc.samples++
		if value.SampledAtMs > acc.sampledAt {
			acc.sampledAt = value.SampledAtMs
		}
		if value.PathScore != nil {
			acc.scoreSum += float64(*value.PathScore)
			acc.scoreCount++
			acc.hasScore = true
		}
		if value.ObservedRttMs != nil {
			acc.rttSum += float64(*value.ObservedRttMs)
			acc.rttCount++
			acc.hasRtt = true
		}
	}

	out := make([]pathHealthWindow, 0, len(grouped))
	for _, acc := range grouped {
		if acc.hasScore && acc.scoreCount > 0 {
			acc.scoreAvg = acc.scoreSum / float64(acc.scoreCount)
		}
		if acc.hasRtt && acc.rttCount > 0 {
			acc.rttAvg = acc.rttSum / float64(acc.rttCount)
		}
		out = append(out, acc.pathHealthWindow)
	}

	sort.SliceStable(out, func(i, j int) bool {
		return betterPathHealth(out[i], out[j])
	})
	return out
}

// relayNodeRank 是 relay 节点排序时使用的聚合健康评分。
type relayNodeRank struct {
	scoreAvg  float64
	rttAvg    float64
	hasScore  bool
	hasRtt    bool
	samples   int
	sampledAt uint64
}

func betterRelayNodeRank(left, right relayNodeRank) bool {
	switch {
	case left.hasScore && right.hasScore && left.scoreAvg != right.scoreAvg:
		return left.scoreAvg > right.scoreAvg
	case left.hasScore != right.hasScore:
		return left.hasScore
	case left.hasRtt && right.hasRtt && left.rttAvg != right.rttAvg:
		return left.rttAvg < right.rttAvg
	case left.hasRtt != right.hasRtt:
		return left.hasRtt
	case left.samples != right.samples:
		return left.samples > right.samples
	default:
		return left.sampledAt > right.sampledAt
	}
}
