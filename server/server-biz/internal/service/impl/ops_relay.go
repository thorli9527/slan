package impl

import (
	"context"
	"math"
	"time"

	"github.com/slan/server/server-biz/api/dto"
)

type opsRelayOverview struct {
	clusterCount    int
	nodeCount       int
	onlineNodeCount int
}

func (s dbOpsService) relayOverview(ctx context.Context) opsRelayOverview {
	clusters := s.state.relayClusters()
	heartbeats := s.state.relayNodeHeartbeats(ctx, time.Now())
	out := opsRelayOverview{clusterCount: len(clusters)}
	for _, cluster := range clusters {
		out.nodeCount += len(cluster.nodes)
		for _, node := range cluster.nodes {
			if heartbeat, ok := heartbeats[node.NodeID]; ok && heartbeat.Healthy {
				out.onlineNodeCount++
			}
		}
	}
	return out
}

func (s dbOpsService) RelayTopology() (dto.OpsRelayTopology, error) {
	ctx := context.Background()
	now := time.Now()
	health := s.state.relayNodeHealth(ctx, now)
	heartbeats := s.state.relayNodeHeartbeats(ctx, now)
	nodes := make([]dto.OpsRelayNode, 0)
	for _, cluster := range s.state.relayClusters() {
		for _, node := range cluster.nodes {
			item := dto.OpsRelayNode{
				NodeID:      node.NodeID,
				ClusterID:   cluster.clusterID,
				ClusterName: cluster.clusterName,
				CountryCode: cluster.countryCode,
				CountryName: cluster.countryName,
				CityCode:    cluster.cityCode,
				CityName:    cluster.cityName,
				Transport:   node.Transport,
				Address:     node.Address,
				Priority:    node.Priority,
			}
			if rank, ok := health[node.NodeID]; ok {
				if rank.hasRtt {
					item.ObservedRttMs = uint32(math.Round(rank.rttAvg))
				}
				if rank.hasScore {
					item.PathScore = uint32(math.Round(rank.scoreAvg))
				}
				item.SampleCount = rank.samples
			}
			if heartbeat, ok := heartbeats[node.NodeID]; ok {
				item.HeartbeatOnline = heartbeat.Healthy
				item.HeartbeatLastSeenAt = heartbeat.UpdatedAt
				item.ActiveSessions = heartbeat.ActiveSessions
			}
			nodes = append(nodes, item)
		}
	}
	return dto.OpsRelayTopology{
		DefaultClusterID: s.state.cfg.Relay.DefaultClusterID,
		Regions:          s.state.relayRegions(),
		Nodes:            nodes,
	}, nil
}
