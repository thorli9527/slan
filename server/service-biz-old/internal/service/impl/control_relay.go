package impl

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/slan/server/server-biz/configs"
	"github.com/slan/server/server-biz/internal/util"
)

// relayClusterView 是控制面在运行时使用的扁平 relay 集群视图。
type relayClusterView struct {
	countryCode string
	countryName string
	cityCode    string
	cityName    string
	clusterID   string
	clusterName string
	nodes       []configs.RelayNodeConfig
}

func relayClustersForCountries(clusters []relayClusterView, countryCodes []string) []relayClusterView {
	allowed := make(map[string]struct{}, len(countryCodes))
	for _, countryCode := range countryCodes {
		value := normalizeRelayCountryCode(countryCode)
		if value == "" {
			continue
		}
		allowed[value] = struct{}{}
	}
	if len(allowed) == 0 {
		return nil
	}
	out := make([]relayClusterView, 0, len(clusters))
	for _, cluster := range clusters {
		if _, ok := allowed[normalizeRelayCountryCode(cluster.countryCode)]; ok {
			out = append(out, cluster)
		}
	}
	return out
}

func (s *dbState) defaultRelayCountryCodes() []string {
	if cluster, ok := s.relayClusterByID(s.cfg.Relay.DefaultClusterID); ok {
		return []string{cluster.countryCode}
	}
	for _, country := range s.cfg.Relay.Countries {
		if value := normalizeRelayCountryCode(country.CountryCode); value != "" {
			return []string{value}
		}
	}
	return nil
}

func (s *dbState) relayClusters() []relayClusterView {
	health := s.relayNodeHealth(context.Background(), time.Now())
	out := make([]relayClusterView, 0)
	for _, country := range s.cfg.Relay.Countries {
		for _, city := range country.Cities {
			for _, cluster := range city.Clusters {
				nodes := prioritizeRelayNodes(cluster.Nodes, "", "", health)
				out = append(out, relayClusterView{
					countryCode: country.CountryCode,
					countryName: country.CountryName,
					cityCode:    city.CityCode,
					cityName:    city.CityName,
					clusterID:   cluster.ClusterID,
					clusterName: util.FirstNonEmpty(cluster.ClusterName, cluster.ClusterID),
					nodes:       nodes,
				})
			}
		}
	}
	return out
}

func (s *dbState) bestRelayCluster(preferredNodeID, avoidedNodeID string) relayClusterView {
	clusters := s.relayClusters()
	if len(clusters) == 0 {
		return relayClusterView{}
	}
	sortRelayClusters(clusters, preferredNodeID, avoidedNodeID)
	return clusters[0]
}

func (s *dbState) relayClusterByID(clusterID string) (relayClusterView, bool) {
	for _, cluster := range s.relayClusters() {
		if cluster.clusterID == clusterID {
			return cluster, true
		}
	}
	return relayClusterView{}, false
}

func (s *dbState) relayClusterByNodeID(nodeID string) (relayClusterView, bool) {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return relayClusterView{}, false
	}
	for _, cluster := range s.relayClusters() {
		for _, node := range cluster.nodes {
			if node.NodeID == nodeID {
				return cluster, true
			}
		}
	}
	return relayClusterView{}, false
}

func (s *dbState) defaultRelayCluster() relayClusterView {
	if cluster, ok := s.relayClusterByID(s.cfg.Relay.DefaultClusterID); ok && len(cluster.nodes) > 0 {
		return cluster
	}
	for _, cluster := range s.relayClusters() {
		if len(cluster.nodes) > 0 {
			return cluster
		}
	}
	return relayClusterView{}
}

func (s *dbState) relayClusterForRequest(clusterID string) relayClusterView {
	clusterID = strings.TrimSpace(clusterID)
	if clusterID != "" {
		if cluster, ok := s.relayClusterByID(clusterID); ok && len(cluster.nodes) > 0 {
			return cluster
		}
	}
	return s.defaultRelayCluster()
}

func sortRelayClusters(clusters []relayClusterView, preferredNodeID, avoidedNodeID string) {
	sort.SliceStable(clusters, func(i, j int) bool {
		leftPreferred := clusterContainsRelayNode(clusters[i], preferredNodeID)
		rightPreferred := clusterContainsRelayNode(clusters[j], preferredNodeID)
		switch {
		case leftPreferred && !rightPreferred:
			return true
		case !leftPreferred && rightPreferred:
			return false
		}

		leftAvoided := clusterContainsRelayNode(clusters[i], avoidedNodeID)
		rightAvoided := clusterContainsRelayNode(clusters[j], avoidedNodeID)
		switch {
		case !leftAvoided && rightAvoided:
			return true
		case leftAvoided && !rightAvoided:
			return false
		}

		leftRank := relayClusterRank(clusters[i])
		rightRank := relayClusterRank(clusters[j])
		switch {
		case betterRelayNodeRank(leftRank, rightRank):
			return true
		case betterRelayNodeRank(rightRank, leftRank):
			return false
		}

		return clusters[i].clusterID < clusters[j].clusterID
	})
}

func relayClusterRank(cluster relayClusterView) relayNodeRank {
	if len(cluster.nodes) == 0 {
		return relayNodeRank{}
	}
	// Cluster nodes are already ordered by preferred/avoided/health/priority.
	// Reuse the best node as the main cluster-level signal, then add a small
	// sample bonus for denser clusters.
	best := relayNodeRank{
		samples: len(cluster.nodes),
	}
	first := cluster.nodes[0]
	if first.Priority > 0 {
		best.hasScore = true
		best.scoreAvg = float64(1000 - util.MinInt(first.Priority, 1000))
	}
	return best
}

func clusterContainsRelayNode(cluster relayClusterView, nodeID string) bool {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return false
	}
	for _, node := range cluster.nodes {
		if node.NodeID == nodeID {
			return true
		}
	}
	return false
}

func normalizeRelayCountryCode(countryCode string) string {
	return strings.ToUpper(strings.TrimSpace(countryCode))
}
