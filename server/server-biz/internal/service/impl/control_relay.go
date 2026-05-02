package impl

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/configs"
	"github.com/slan/server/server-biz/internal/repo"
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

func (s *dbState) relayConfig() dto.RelayConfig {
	countries := make([]dto.RelayCountry, 0, len(s.cfg.Relay.Countries))
	for _, country := range s.cfg.Relay.Countries {
		dtoCountry := dto.RelayCountry{
			CountryCode: country.CountryCode,
			CountryName: country.CountryName,
			Cities:      make([]dto.RelayCity, 0, len(country.Cities)),
		}
		for _, city := range country.Cities {
			dtoCity := dto.RelayCity{
				CityCode: city.CityCode,
				CityName: city.CityName,
				Clusters: make([]dto.RelayCluster, 0, len(city.Clusters)),
			}
			for _, cluster := range city.Clusters {
				dtoCluster := dto.RelayCluster{
					ClusterID:   cluster.ClusterID,
					ClusterName: util.FirstNonEmpty(cluster.ClusterName, cluster.ClusterID),
					Nodes:       make([]dto.RelayNode, 0, len(cluster.Nodes)),
				}
				for _, node := range cluster.Nodes {
					dtoCluster.Nodes = append(dtoCluster.Nodes, dto.RelayNode{
						NodeID:    node.NodeID,
						Transport: node.Transport,
						Address:   node.Address,
						Priority:  node.Priority,
						Tags:      append([]string(nil), node.Tags...),
					})
				}
				dtoCity.Clusters = append(dtoCity.Clusters, dtoCluster)
			}
			dtoCountry.Cities = append(dtoCountry.Cities, dtoCity)
		}
		countries = append(countries, dtoCountry)
	}

	return dto.RelayConfig{
		DefaultClusterID: s.cfg.Relay.DefaultClusterID,
		Countries:        countries,
	}
}

func (s *dbState) derpMap() dto.DerpMap {
	clusters := s.relayClusters()
	out := make([]dto.DerpCluster, 0, len(clusters))
	for _, cluster := range clusters {
		nodes := make([]dto.DerpNode, 0, len(cluster.nodes))
		for _, node := range cluster.nodes {
			host, port := util.SplitRelayAddress(node.Address)
			nodes = append(nodes, dto.DerpNode{
				NodeID:    node.NodeID,
				Host:      host,
				Port:      port,
				Transport: node.Transport,
				Priority:  node.Priority,
				Tags:      append([]string(nil), node.Tags...),
			})
		}
		out = append(out, dto.DerpCluster{
			ClusterID:         cluster.clusterID,
			ClusterName:       cluster.clusterName,
			RegionID:          cluster.cityCode,
			RegionName:        cluster.cityName,
			CountryCode:       cluster.countryCode,
			CountryName:       cluster.countryName,
			CityCode:          cluster.cityCode,
			CityName:          cluster.cityName,
			RecommendedFanout: util.MinInt(len(nodes), 3),
			Nodes:             nodes,
		})
	}
	return dto.DerpMap{
		ProbeIntervalSeconds: 5,
		Clusters:             out,
	}
}

func (s *dbState) relayRegions() []dto.RelayRegion {
	clusters := s.relayClusters()
	return relayRegionsFromClusters(clusters)
}

func (s *dbState) relayRegionsForCountries(countryCodes []string) []dto.RelayRegion {
	clusters := relayClustersForCountries(s.relayClusters(), countryCodes)
	if len(clusters) == 0 {
		clusters = relayClustersForCountries(s.relayClusters(), s.defaultRelayCountryCodes())
	}
	if len(clusters) == 0 {
		clusters = s.relayClusters()
	}
	return relayRegionsFromClusters(clusters)
}

func relayRegionsFromClusters(clusters []relayClusterView) []dto.RelayRegion {
	out := make([]dto.RelayRegion, 0, len(clusters))
	for _, cluster := range clusters {
		endpoints := make([]dto.RelayEndpoint, 0, len(cluster.nodes))
		for _, node := range cluster.nodes {
			endpoints = append(endpoints, dto.RelayEndpoint{
				EndpointID: node.NodeID,
				Transport:  node.Transport,
				Address:    node.Address,
			})
		}
		out = append(out, dto.RelayRegion{
			RegionID:    cluster.cityCode,
			RegionName:  cluster.cityName,
			CountryCode: cluster.countryCode,
			CountryName: cluster.countryName,
			CityCode:    cluster.cityCode,
			CityName:    cluster.cityName,
			ClusterID:   cluster.clusterID,
			ClusterName: cluster.clusterName,
			Endpoints:   endpoints,
		})
	}
	return out
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

func (s *dbState) relayFallbackEndpoints() []dto.Endpoint {
	cluster := s.defaultRelayCluster()
	if len(cluster.nodes) == 0 {
		return nil
	}
	now := time.Now().Unix()
	out := make([]dto.Endpoint, 0, len(cluster.nodes))
	for _, node := range cluster.nodes {
		out = append(out, dto.Endpoint{
			Type:      "relay",
			Address:   node.Address,
			UpdatedAt: now,
		})
	}
	return out
}

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
