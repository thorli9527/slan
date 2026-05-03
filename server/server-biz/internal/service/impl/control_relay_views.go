package impl

import (
	"time"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/util"
)

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
