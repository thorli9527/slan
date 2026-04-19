package httpapi

import (
	"encoding/json"
	"expvar"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/slan/server/server-biz/internal/util"
	controlws "github.com/slan/server/server-biz/internal/ws"
)

// 下面这组 expvar 指标统一承载 control WS 的观测信息。
//
// 设计原则：
// - 总量和分类量分开
// - 最近样本保留少量 ring buffer
// - 对热点 pair 给出摘要，便于一期排障
var controlWSMetrics = expvar.NewMap("control_ws_metrics")
var controlWSMessageTypeMetrics = expvar.NewMap("control_ws_message_type_total")
var controlWSErrorCodeMetrics = expvar.NewMap("control_ws_error_code_total")
var controlWSConnectionStateMetrics = expvar.NewMap("control_ws_connection_state_total")
var controlWSConnectionPathMetrics = expvar.NewMap("control_ws_connection_path_total")
var controlWSPathHealthMetrics = expvar.NewMap("control_ws_path_health_total")
var controlWSConnectPlanClusterMetrics = expvar.NewMap("control_ws_connect_plan_cluster_total")
var controlWSConnectPlanNodeMetrics = expvar.NewMap("control_ws_connect_plan_node_total")
var controlWSConnectPlanClusterByPeerMetrics = expvar.NewMap("control_ws_connect_plan_cluster_by_peer_total")
var controlWSConnectPlanNodeByPeerMetrics = expvar.NewMap("control_ws_connect_plan_node_by_peer_total")
var controlWSConnectionStateByPeerMetrics = expvar.NewMap("control_ws_connection_state_by_peer_total")
var controlWSPathHealthByPeerMetrics = expvar.NewMap("control_ws_path_health_by_peer_total")
var controlWSActiveSessions = expvar.NewInt("control_ws_active_sessions")
var controlWSLastConnectionState = expvar.NewString("control_ws_last_connection_state")
var controlWSLastPathHealth = expvar.NewString("control_ws_last_path_health")
var controlWSLastConnectPlan = expvar.NewString("control_ws_last_connect_plan")
var controlWSRecentConnectionStates = expvar.NewString("control_ws_recent_connection_states")
var controlWSRecentPathHealth = expvar.NewString("control_ws_recent_path_health")
var controlWSRecentConnectPlans = expvar.NewString("control_ws_recent_connect_plans")
var controlWSRecentRelayPairs = expvar.NewString("control_ws_recent_relay_pairs")
var controlWSRecentUnhealthyPairs = expvar.NewString("control_ws_recent_unhealthy_pairs")
var controlWSRecentFailedPairs = expvar.NewString("control_ws_recent_failed_pairs")
var controlWSRecentDegradedPairs = expvar.NewString("control_ws_recent_degraded_pairs")

const controlWSRecentSampleLimit = 8

var controlWSRecentSamplesMu sync.Mutex
var controlWSRecentConnectionStateItems []any
var controlWSRecentPathHealthItems []any
var controlWSRecentConnectPlanItems []any
var controlWSRelayPairCounts = map[string]int64{}
var controlWSRelayPairClusterIDs = map[string]string{}
var controlWSRelayPairNodeIDs = map[string]string{}
var controlWSUnhealthyPairCounts = map[string]int64{}
var controlWSUnhealthyPairReasons = map[string]string{}
var controlWSFailedPairCounts = map[string]int64{}
var controlWSFailedPairReasons = map[string]string{}
var controlWSDegradedPairCounts = map[string]int64{}
var controlWSDegradedPairReasons = map[string]string{}

// metricAdd 为顶层 control_ws_metrics 增加一个整型指标。
func metricAdd(name string, delta int64) {
	controlWSMetrics.Add(name, delta)
}

// metricAddByType 按字符串标签对 expvar.Map 做累计。
func metricAddByType(metrics *expvar.Map, name string, delta int64) {
	if metrics == nil {
		return
	}
	name = util.FirstNonEmpty(name, "unknown")
	metrics.Add(name, delta)
}

// metricAddByPeer 以 network|node|peer 维度累计 pair 指标。
func metricAddByPeer(metrics *expvar.Map, networkID, nodeID, peerNodeID string, delta int64) {
	if metrics == nil {
		return
	}
	key := util.FirstNonEmpty(networkID, "unknown") + "|" +
		util.FirstNonEmpty(nodeID, "unknown") + "|" +
		util.FirstNonEmpty(peerNodeID, "unknown")
	metrics.Add(key, delta)
}

// metricAddConnectPlanByPeer 以 network|node|peer|value 维度累计 connect-plan 偏向。
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

// metricSetJSON 将任意 payload 序列化为 JSON 后写入 expvar.String。
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

// metricAppendRecentJSON 把 payload 插入最近样本队列头部，并回写 JSON 数组。
func metricAppendRecentJSON(target *expvar.String, items *[]any, payload any) {
	if target == nil || items == nil {
		return
	}
	controlWSRecentSamplesMu.Lock()
	defer controlWSRecentSamplesMu.Unlock()

	next := append([]any{payload}, (*items)...)
	if len(next) > controlWSRecentSampleLimit {
		next = next[:controlWSRecentSampleLimit]
	}
	*items = next
	metricSetJSON(target, next)
}

// metricRecordConnectPlan 记录一次 connect-plan 下发及其 relay 偏向。
func metricRecordConnectPlan(networkID, sourceNodeID, targetNodeID string, plan controlws.ConnectPlan) {
	metricAdd("connect_plan_sent_total", 1)
	clusterID := util.FirstNonEmpty(plan.DerpClusterID, "none")
	metricAddByType(controlWSConnectPlanClusterMetrics, clusterID, 1)
	metricAddConnectPlanByPeer(controlWSConnectPlanClusterByPeerMetrics, networkID, sourceNodeID, plan.PeerNodeID, clusterID, 1)

	suggestedRelayNodeID := "none"
	if len(plan.PreferredDerpNodeIDs) > 0 && strings.TrimSpace(plan.PreferredDerpNodeIDs[0]) != "" {
		suggestedRelayNodeID = strings.TrimSpace(plan.PreferredDerpNodeIDs[0])
	}
	metricAddByType(controlWSConnectPlanNodeMetrics, suggestedRelayNodeID, 1)
	metricAddConnectPlanByPeer(controlWSConnectPlanNodeByPeerMetrics, networkID, sourceNodeID, plan.PeerNodeID, suggestedRelayNodeID, 1)

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
	metricSetJSON(controlWSLastConnectPlan, sample)
	metricAppendRecentJSON(controlWSRecentConnectPlans, &controlWSRecentConnectPlanItems, sample)
	if suggestedRelayNodeID != "none" || strings.HasPrefix(suggestedPathType, "relay") || clusterID != "none" {
		metricRecordRelayPair(networkID, sourceNodeID, plan.PeerNodeID, clusterID, suggestedRelayNodeID)
	}
}

// metricRecordConnectionState 记录客户端上报的连接状态样本。
func metricRecordConnectionState(session *wsSession, state controlws.ConnectionState) {
	metricAdd("connection_state_report_total", 1)
	metricAddByType(controlWSConnectionStateMetrics, state.State, 1)
	metricAddByType(controlWSConnectionPathMetrics, util.FirstNonEmpty(state.Path, "unknown"), 1)
	metricAddByPeer(
		controlWSConnectionStateByPeerMetrics,
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
	metricSetJSON(controlWSLastConnectionState, connectionSample)
	metricAppendRecentJSON(
		controlWSRecentConnectionStates,
		&controlWSRecentConnectionStateItems,
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

// metricRecordPathHealth 记录客户端上报的路径健康度样本。
func metricRecordPathHealth(session *wsSession, report controlws.PathHealthReport) {
	metricAddByType(controlWSPathHealthMetrics, util.FirstNonEmpty(report.PathType, "unknown"), 1)
	metricAdd("path_health_report_total", 1)
	metricAddByPeer(
		controlWSPathHealthByPeerMetrics,
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
		"endpoint":       report.Endpoint,
		"derpNodeId":     report.DerpNodeID,
		"observedRttMs":  report.ObservedRttMs,
		"packetLossPpm":  report.PacketLossPpm,
		"pathScore":      report.PathScore,
		"sampledAtMs":    report.SampledAtMs,
		"authorizedUser": session.userID,
	}
	metricSetJSON(controlWSLastPathHealth, pathHealthSample)
	metricAppendRecentJSON(
		controlWSRecentPathHealth,
		&controlWSRecentPathHealthItems,
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
	controlWSRecentSamplesMu.Lock()
	defer controlWSRecentSamplesMu.Unlock()

	pairKey := util.FirstNonEmpty(networkID, "unknown") + "|" +
		util.FirstNonEmpty(nodeID, "unknown") + "|" +
		util.FirstNonEmpty(peerNodeID, "unknown")
	controlWSRelayPairCounts[pairKey]++
	controlWSRelayPairClusterIDs[pairKey] = util.FirstNonEmpty(clusterID, "none")
	controlWSRelayPairNodeIDs[pairKey] = util.FirstNonEmpty(relayNodeID, "none")

	type relayPairSummary struct {
		PairKey       string `json:"pairKey"`
		Count         int64  `json:"count"`
		DerpClusterID string `json:"derpClusterId"`
		RelayNodeID   string `json:"relayNodeId"`
	}

	items := make([]relayPairSummary, 0, len(controlWSRelayPairCounts))
	for key, count := range controlWSRelayPairCounts {
		items = append(items, relayPairSummary{
			PairKey:       key,
			Count:         count,
			DerpClusterID: controlWSRelayPairClusterIDs[key],
			RelayNodeID:   controlWSRelayPairNodeIDs[key],
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].PairKey < items[j].PairKey
		}
		return items[i].Count > items[j].Count
	})
	if len(items) > controlWSRecentSampleLimit {
		items = items[:controlWSRecentSampleLimit]
	}
	metricSetJSON(controlWSRecentRelayPairs, items)
}

func metricRecordUnhealthyPair(networkID, nodeID, peerNodeID, reason string) {
	recordPairSummary(
		controlWSRecentUnhealthyPairs,
		controlWSUnhealthyPairCounts,
		controlWSUnhealthyPairReasons,
		networkID,
		nodeID,
		peerNodeID,
		reason,
	)
}

func metricRecordFailedPair(networkID, nodeID, peerNodeID, reason string) {
	recordPairSummary(
		controlWSRecentFailedPairs,
		controlWSFailedPairCounts,
		controlWSFailedPairReasons,
		networkID,
		nodeID,
		peerNodeID,
		reason,
	)
}

func metricRecordDegradedPair(networkID, nodeID, peerNodeID, reason string) {
	recordPairSummary(
		controlWSRecentDegradedPairs,
		controlWSDegradedPairCounts,
		controlWSDegradedPairReasons,
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
	controlWSRecentSamplesMu.Lock()
	defer controlWSRecentSamplesMu.Unlock()

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
	if len(items) > controlWSRecentSampleLimit {
		items = items[:controlWSRecentSampleLimit]
	}
	metricSetJSON(target, items)
}
