package impl

import (
	"context"
	"strings"
	"time"

	"github.com/slan/server/server-biz/internal/repo"
)

func (s *dbState) relayNodeHealth(ctx context.Context, now time.Time) map[string]relayNodeRank {
	cutoff := pathHealthCutoffUnix(now)
	values, err := s.pg.ListRecentRelayNodePathHealth(ctx, cutoff)
	if err != nil {
		values = nil
	}

	type accumulator struct {
		rank       relayNodeRank
		scoreSum   float64
		scoreCount int
		rttSum     float64
		rttCount   int
	}
	grouped := make(map[string]*accumulator, len(values))
	for _, value := range values {
		nodeID := strings.TrimSpace(value.DerpNodeID)
		if nodeID == "" {
			continue
		}
		acc, ok := grouped[nodeID]
		if !ok {
			acc = &accumulator{}
			grouped[nodeID] = acc
		}
		acc.rank.samples++
		if value.SampledAtMs > acc.rank.sampledAt {
			acc.rank.sampledAt = value.SampledAtMs
		}
		if value.PathScore != nil {
			acc.scoreSum += float64(*value.PathScore)
			acc.scoreCount++
			acc.rank.hasScore = true
		}
		if value.ObservedRttMs != nil {
			acc.rttSum += float64(*value.ObservedRttMs)
			acc.rttCount++
			acc.rank.hasRtt = true
		}
	}

	out := make(map[string]relayNodeRank, len(grouped))
	for nodeID, acc := range grouped {
		if acc.rank.hasScore && acc.scoreCount > 0 {
			acc.rank.scoreAvg = acc.scoreSum / float64(acc.scoreCount)
		}
		if acc.rank.hasRtt && acc.rttCount > 0 {
			acc.rank.rttAvg = acc.rttSum / float64(acc.rttCount)
		}
		out[nodeID] = acc.rank
	}
	for nodeID, heartbeat := range s.relayNodeHeartbeats(ctx, now) {
		rank := out[nodeID]
		rank.hasHeartbeat = true
		rank.heartbeatHealthy = heartbeat.Healthy
		if heartbeat.ReportedAtMs > rank.sampledAt {
			rank.sampledAt = heartbeat.ReportedAtMs
		}
		out[nodeID] = rank
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (s *dbState) relayNodeHeartbeats(ctx context.Context, now time.Time) map[string]repo.RelayNodeHeartbeat {
	cutoff := now.Add(-relayNodeHeartbeatFreshnessWindow).Unix()
	values, err := s.pg.ListRecentRelayNodeHeartbeats(ctx, cutoff)
	if err != nil || len(values) == 0 {
		return nil
	}
	out := make(map[string]repo.RelayNodeHeartbeat, len(values))
	for _, value := range values {
		nodeID := strings.TrimSpace(value.NodeID)
		if nodeID == "" {
			continue
		}
		out[nodeID] = value
	}
	return out
}
