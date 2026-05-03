package httpapi

import (
	"encoding/json"
	"expvar"
	"sort"
	"strings"
	"sync"
	"time"

	controlmsg "github.com/slan/server/server-biz/internal/controlmsg"
	"github.com/slan/server/server-biz/internal/util"
)

// The expvar metrics below expose Control MQTT observability data.
//
// Design notes:
// - keep total counters and categorized counters separate
// - keep a small ring buffer for recent samples
// - summarize hot peer pairs for first-pass troubleshooting
var controlmsgMetrics = expvar.NewMap("control_mqtt_metrics")
var controlmsgMessageTypeMetrics = expvar.NewMap("control_mqtt_message_type_total")
var controlmsgErrorCodeMetrics = expvar.NewMap("control_mqtt_error_code_total")
var controlmsgConnectionStateMetrics = expvar.NewMap("control_mqtt_connection_state_total")
var controlmsgConnectionPathMetrics = expvar.NewMap("control_mqtt_connection_path_total")
var controlmsgPathHealthMetrics = expvar.NewMap("control_mqtt_path_health_total")
var controlmsgConnectPlanClusterMetrics = expvar.NewMap("control_mqtt_connect_plan_cluster_total")
var controlmsgConnectPlanNodeMetrics = expvar.NewMap("control_mqtt_connect_plan_node_total")
var controlmsgConnectPlanClusterByPeerMetrics = expvar.NewMap("control_mqtt_connect_plan_cluster_by_peer_total")
var controlmsgConnectPlanNodeByPeerMetrics = expvar.NewMap("control_mqtt_connect_plan_node_by_peer_total")
var controlmsgConnectionStateByPeerMetrics = expvar.NewMap("control_mqtt_connection_state_by_peer_total")
var controlmsgPathHealthByPeerMetrics = expvar.NewMap("control_mqtt_path_health_by_peer_total")
var controlmsgActiveSessions = expvar.NewInt("control_mqtt_active_sessions")
var controlmsgLastConnectionState = expvar.NewString("control_mqtt_last_connection_state")
var controlmsgLastPathHealth = expvar.NewString("control_mqtt_last_path_health")
var controlmsgLastConnectPlan = expvar.NewString("control_mqtt_last_connect_plan")
var controlmsgRecentConnectionStates = expvar.NewString("control_mqtt_recent_connection_states")
var controlmsgRecentPathHealth = expvar.NewString("control_mqtt_recent_path_health")
var controlmsgRecentConnectPlans = expvar.NewString("control_mqtt_recent_connect_plans")
var controlmsgRecentRelayPairs = expvar.NewString("control_mqtt_recent_relay_pairs")
var controlmsgRecentUnhealthyPairs = expvar.NewString("control_mqtt_recent_unhealthy_pairs")
var controlmsgRecentFailedPairs = expvar.NewString("control_mqtt_recent_failed_pairs")
var controlmsgRecentDegradedPairs = expvar.NewString("control_mqtt_recent_degraded_pairs")

const controlmsgRecentSampleLimit = 8

var controlmsgRecentSamplesMu sync.Mutex
var controlmsgRecentConnectionStateItems []any
var controlmsgRecentPathHealthItems []any
var controlmsgRecentConnectPlanItems []any
var controlmsgRelayPairCounts = map[string]int64{}
var controlmsgRelayPairClusterIDs = map[string]string{}
var controlmsgRelayPairNodeIDs = map[string]string{}
var controlmsgUnhealthyPairCounts = map[string]int64{}
var controlmsgUnhealthyPairReasons = map[string]string{}
var controlmsgFailedPairCounts = map[string]int64{}
var controlmsgFailedPairReasons = map[string]string{}
var controlmsgDegradedPairCounts = map[string]int64{}
var controlmsgDegradedPairReasons = map[string]string{}

// metricAdd increments a top-level control_mqtt_metrics counter.
func metricAdd(name string, delta int64) {
	controlmsgMetrics.Add(name, delta)
}

// metricAddByType increments an expvar.Map counter keyed by a string label.
func metricAddByType(metrics *expvar.Map, name string, delta int64) {
	if metrics == nil {
		return
	}
	name = util.FirstNonEmpty(name, "unknown")
	metrics.Add(name, delta)
}

// metricAddByPeer increments a pair metric keyed by network, node, and peer.
func metricAddByPeer(metrics *expvar.Map, networkID, nodeID, peerNodeID string, delta int64) {
	if metrics == nil {
		return
	}
	key := util.FirstNonEmpty(networkID, "unknown") + "|" +
		util.FirstNonEmpty(nodeID, "unknown") + "|" +
		util.FirstNonEmpty(peerNodeID, "unknown")
	metrics.Add(key, delta)
}

// metricAddConnectPlanByPeer increments a connect-plan preference by peer.
func metricAddConnectPlanByPeer(metrics *expvar.Map, networkID, nodeID, peerNodeID, value string, delta int64) {
	if metrics == nil {
		return
	}
	key := util.FirstNonEmpty(networkID, "unknown") + "|" +
		util.FirstNonEmpty(nodeID, "unknown") + "|" +
		util.FirstNonEmpty(peerNodeID, "unknown") + "|" +
		util.FirstNonEmpty(value, "none")
	metrics.Add(key, delta)
}

// metricSetJSON serializes a payload as JSON and stores it in expvar.String.
func metricSetJSON(target *expvar.String, payload any) {
	if target == nil {
		return
	}
	data, err := json.Marshal(payload)
	if err != nil {
		target.Set(`{"error":"marshal_failed"}`)
		return
	}
	target.Set(string(data))
}

// metricAppendRecentJSON prepends a payload to the recent sample queue.
func metricAppendRecentJSON(target *expvar.String, items *[]any, payload any) {
	if target == nil || items == nil {
		return
	}
	controlmsgRecentSamplesMu.Lock()
	defer controlmsgRecentSamplesMu.Unlock()

	next := append([]any{payload}, (*items)...)
	if len(next) > controlmsgRecentSampleLimit {
		next = next[:controlmsgRecentSampleLimit]
	}
	*items = next
	metricSetJSON(target, next)
}

// metricRecordConnectPlan records a delivered connect plan and relay preference.
func metricRecordConnectPlan(networkID, sourceNodeID, targetNodeID string, plan controlmsg.ConnectPlan) {
	metricAdd("connect_plan_sent_total", 1)
	clusterID := util.FirstNonEmpty(plan.DerpClusterID, "none")
	metricAddByType(controlmsgConnectPlanClusterMetrics, clusterID, 1)
	metricAddConnectPlanByPeer(controlmsgConnectPlanClusterByPeerMetrics, networkID, sourceNodeID, plan.PeerNodeID, clusterID, 1)

	suggestedRelayNodeID := "none"
	if len(plan.PreferredDerpNodeIDs) > 0 && strings.TrimSpace(plan.PreferredDerpNodeIDs[0]) != "" {
		suggestedRelayNodeID = strings.TrimSpace(plan.PreferredDerpNodeIDs[0])
	}
	metricAddByType(controlmsgConnectPlanNodeMetrics, suggestedRelayNodeID, 1)
	metricAddConnectPlanByPeer(controlmsgConnectPlanNodeByPeerMetrics, networkID, sourceNodeID, plan.PeerNodeID, suggestedRelayNodeID, 1)

	suggestedPathType := "none"
	suggestedEndpoint := ""
	if len(plan.Paths) > 0 {
		suggestedPathType = util.FirstNonEmpty(plan.Paths[0].PathType, "unknown")
		suggestedEndpoint = plan.Paths[0].Endpoint
	}

	sample := map[string]any{
		"sentAtMs":              time.Now().UnixMilli(),
		"networkId":             networkID,
		"sourceNodeId":          sourceNodeID,
		"targetNodeId":          targetNodeID,
		"peerNodeId":            plan.PeerNodeID,
		"preferDirect":          plan.PreferDirect,
		"derpClusterId":         clusterID,
		"preferredDerpNodeIds":  append([]string(nil), plan.PreferredDerpNodeIDs...),
		"suggestedRelayNodeId":  suggestedRelayNodeID,
		"suggestedPathType":     suggestedPathType,
		"suggestedPathEndpoint": suggestedEndpoint,
		"pathCount":             len(plan.Paths),
		"hasRelayTicket":        plan.RelayTicket != nil,
	}
	metricSetJSON(controlmsgLastConnectPlan, sample)
	metricAppendRecentJSON(controlmsgRecentConnectPlans, &controlmsgRecentConnectPlanItems, sample)
	if suggestedRelayNodeID != "none" || strings.HasPrefix(suggestedPathType, "relay") || clusterID != "none" {
		metricRecordRelayPair(networkID, sourceNodeID, plan.PeerNodeID, clusterID, suggestedRelayNodeID)
	}
}

// metricRecordConnectionState records a client connection-state sample.
func metricRecordConnectionState(session *controlSession, state controlmsg.ConnectionState) {
	metricAdd("connection_state_report_total", 1)
	metricAddByType(controlmsgConnectionStateMetrics, state.State, 1)
	metricAddByType(controlmsgConnectionPathMetrics, util.FirstNonEmpty(state.Path, "unknown"), 1)
	metricAddByPeer(
		controlmsgConnectionStateByPeerMetrics,
		state.NetworkID,
		session.nodeID,
		state.PeerNodeID,
		1,
	)

	connectionSample := map[string]any{
		"receivedAtMs":   time.Now().UnixMilli(),
		"networkId":      state.NetworkID,
		"sourceNodeId":   session.nodeID,
		"peerNodeId":     state.PeerNodeID,
		"path":           util.FirstNonEmpty(state.Path, "unknown"),
		"state":          state.State,
		"reason":         state.Reason,
		"observedRttMs":  state.ObservedRttMs,
		"packetLossPpm":  state.PacketLossPpm,
		"pathScore":      state.PathScore,
		"derpNodeId":     state.DerpNodeID,
		"authorizedUser": session.userID,
	}
	metricSetJSON(controlmsgLastConnectionState, connectionSample)
	metricAppendRecentJSON(
		controlmsgRecentConnectionStates,
		&controlmsgRecentConnectionStateItems,
		connectionSample,
	)

	if state.State == "failed" || state.State == "closed" {
		reason := util.FirstNonEmpty(state.State, state.Reason)
		metricRecordUnhealthyPair(
			state.NetworkID,
			session.nodeID,
			state.PeerNodeID,
			reason,
		)
		metricRecordFailedPair(
			state.NetworkID,
			session.nodeID,
			state.PeerNodeID,
			reason,
		)
	}
}

// metricRecordPathHealth records a client path-health sample.
func metricRecordPathHealth(session *controlSession, report controlmsg.PathHealthReport) {
	metricAddByType(controlmsgPathHealthMetrics, util.FirstNonEmpty(report.PathType, "unknown"), 1)
	metricAdd("path_health_report_total", 1)
	metricAddByPeer(
		controlmsgPathHealthByPeerMetrics,
		report.NetworkID,
		session.nodeID,
		report.PeerNodeID,
		1,
	)

	pathHealthSample := map[string]any{
		"receivedAtMs":   time.Now().UnixMilli(),
		"networkId":      report.NetworkID,
		"sourceNodeId":   session.nodeID,
		"peerNodeId":     report.PeerNodeID,
		"pathType":       util.FirstNonEmpty(report.PathType, "unknown"),
		"activePath":     report.ActivePath,
		"endpoint":       report.Endpoint,
		"derpNodeId":     report.DerpNodeID,
		"observedRttMs":  report.ObservedRttMs,
		"packetLossPpm":  report.PacketLossPpm,
		"pathScore":      report.PathScore,
		"pathDowngrades": report.PathDowngrades,
		"pathUpgrades":   report.PathUpgrades,
		"lastPathChange": report.LastPathChange,
		"sampledAtMs":    report.SampledAtMs,
		"authorizedUser": session.userID,
	}
	metricSetJSON(controlmsgLastPathHealth, pathHealthSample)
	metricAppendRecentJSON(
		controlmsgRecentPathHealth,
		&controlmsgRecentPathHealthItems,
		pathHealthSample,
	)

	if (report.PathScore != nil && *report.PathScore < 50) ||
		(report.PacketLossPpm != nil && *report.PacketLossPpm > 100000) {
		metricRecordUnhealthyPair(
			report.NetworkID,
			session.nodeID,
			report.PeerNodeID,
			"degraded_path_health",
		)
		metricRecordDegradedPair(
			report.NetworkID,
			session.nodeID,
			report.PeerNodeID,
			"degraded_path_health",
		)
	}
}

func metricRecordRelayPair(networkID, nodeID, peerNodeID, clusterID, relayNodeID string) {
	controlmsgRecentSamplesMu.Lock()
	defer controlmsgRecentSamplesMu.Unlock()

	pairKey := util.FirstNonEmpty(networkID, "unknown") + "|" +
		util.FirstNonEmpty(nodeID, "unknown") + "|" +
		util.FirstNonEmpty(peerNodeID, "unknown")
	controlmsgRelayPairCounts[pairKey]++
	controlmsgRelayPairClusterIDs[pairKey] = util.FirstNonEmpty(clusterID, "none")
	controlmsgRelayPairNodeIDs[pairKey] = util.FirstNonEmpty(relayNodeID, "none")

	type relayPairSummary struct {
		PairKey       string `json:"pairKey"`
		Count         int64  `json:"count"`
		DerpClusterID string `json:"derpClusterId"`
		RelayNodeID   string `json:"relayNodeId"`
	}

	items := make([]relayPairSummary, 0, len(controlmsgRelayPairCounts))
	for key, count := range controlmsgRelayPairCounts {
		items = append(items, relayPairSummary{
			PairKey:       key,
			Count:         count,
			DerpClusterID: controlmsgRelayPairClusterIDs[key],
			RelayNodeID:   controlmsgRelayPairNodeIDs[key],
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].PairKey < items[j].PairKey
		}
		return items[i].Count > items[j].Count
	})
	if len(items) > controlmsgRecentSampleLimit {
		items = items[:controlmsgRecentSampleLimit]
	}
	metricSetJSON(controlmsgRecentRelayPairs, items)
}

func metricRecordUnhealthyPair(networkID, nodeID, peerNodeID, reason string) {
	recordPairSummary(
		controlmsgRecentUnhealthyPairs,
		controlmsgUnhealthyPairCounts,
		controlmsgUnhealthyPairReasons,
		networkID,
		nodeID,
		peerNodeID,
		reason,
	)
}

func metricRecordFailedPair(networkID, nodeID, peerNodeID, reason string) {
	recordPairSummary(
		controlmsgRecentFailedPairs,
		controlmsgFailedPairCounts,
		controlmsgFailedPairReasons,
		networkID,
		nodeID,
		peerNodeID,
		reason,
	)
}

func metricRecordDegradedPair(networkID, nodeID, peerNodeID, reason string) {
	recordPairSummary(
		controlmsgRecentDegradedPairs,
		controlmsgDegradedPairCounts,
		controlmsgDegradedPairReasons,
		networkID,
		nodeID,
		peerNodeID,
		reason,
	)
}

func recordPairSummary(
	target *expvar.String,
	counts map[string]int64,
	reasons map[string]string,
	networkID, nodeID, peerNodeID, reason string,
) {
	controlmsgRecentSamplesMu.Lock()
	defer controlmsgRecentSamplesMu.Unlock()

	key := util.FirstNonEmpty(networkID, "unknown") + "|" +
		util.FirstNonEmpty(nodeID, "unknown") + "|" +
		util.FirstNonEmpty(peerNodeID, "unknown")
	counts[key]++
	reasons[key] = util.FirstNonEmpty(reason, "unknown")

	type unhealthyPairSummary struct {
		PairKey string `json:"pairKey"`
		Count   int64  `json:"count"`
		Reason  string `json:"reason"`
	}

	items := make([]unhealthyPairSummary, 0, len(counts))
	for pairKey, count := range counts {
		items = append(items, unhealthyPairSummary{
			PairKey: pairKey,
			Count:   count,
			Reason:  reasons[pairKey],
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].PairKey < items[j].PairKey
		}
		return items[i].Count > items[j].Count
	})
	if len(items) > controlmsgRecentSampleLimit {
		items = items[:controlmsgRecentSampleLimit]
	}
	metricSetJSON(target, items)
}
