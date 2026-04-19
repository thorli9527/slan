package httpapi

import (
	"github.com/slan/server/server-biz/api/dto"
	controlws "github.com/slan/server/server-biz/internal/ws"
)

// dtoNetworkMapToWS 把业务 DTO 网络图转换成控制通道 WS 负载。
func dtoNetworkMapToWS(m dto.NetworkMap) controlws.NetworkMap {
	return controlws.NetworkMap{
		SelfUserID:       m.SelfUserID,
		SelfDeviceID:     m.SelfDeviceID,
		SelfNodeID:       m.SelfNodeID,
		NetworkID:        m.NetworkID,
		Revision:         m.Revision,
		HeartbeatSeconds: m.HeartbeatSeconds,
		STUNServers:      append([]string(nil), m.STUNServers...),
		Peers:            dtoPeersToWSPeers(m.Peers),
		Routes:           dtoRoutesToWSRoutes(m.Routes),
		RelayRegions:     dtoRelayRegionsToWSRelayRegions(m.RelayRegions),
		DNS: controlws.DNSConfig{
			Servers:       append([]string(nil), m.DNS.Servers...),
			SearchDomains: append([]string(nil), m.DNS.SearchDomains...),
		},
		MTU: m.MTU,
	}
}

// dtoPeersToWSPeers 批量转换 peer 列表。
func dtoPeersToWSPeers(peers []dto.Peer) []controlws.Peer {
	out := make([]controlws.Peer, 0, len(peers))
	for _, peer := range peers {
		out = append(out, controlws.Peer{
			NodeID:        peer.NodeID,
			DeviceID:      peer.DeviceID,
			PublicKey:     peer.PublicKey,
			Status:        peer.Status,
			RelayAllowed:  peer.RelayAllowed,
			VirtualIPs:    append([]string(nil), peer.VirtualIPs...),
			Endpoints:     dtoEndpointsToWSEndpoints(peer.Endpoints),
			AllowedRoutes: append([]string(nil), peer.AllowedRoutes...),
		})
	}
	return out
}

// dtoEndpointsToWSEndpoints 批量转换 endpoint 列表。
func dtoEndpointsToWSEndpoints(endpoints []dto.Endpoint) []controlws.Endpoint {
	out := make([]controlws.Endpoint, 0, len(endpoints))
	for _, endpoint := range endpoints {
		out = append(out, controlws.Endpoint{
			Type:      endpoint.Type,
			Address:   endpoint.Address,
			UpdatedAt: endpoint.UpdatedAt,
		})
	}
	return out
}

// dtoRoutesToWSRoutes 批量转换路由列表。
func dtoRoutesToWSRoutes(routes []dto.Route) []controlws.Route {
	out := make([]controlws.Route, 0, len(routes))
	for _, route := range routes {
		out = append(out, controlws.Route{
			CIDR:      route.CIDR,
			ViaNodeID: route.ViaNodeID,
			Metric:    route.Metric,
		})
	}
	return out
}

// dtoRelayRegionsToWSRelayRegions 批量转换 relay 区域列表。
func dtoRelayRegionsToWSRelayRegions(regions []dto.RelayRegion) []controlws.RelayRegion {
	out := make([]controlws.RelayRegion, 0, len(regions))
	for _, region := range regions {
		out = append(out, controlws.RelayRegion{
			RegionID:    region.RegionID,
			RegionName:  region.RegionName,
			CountryCode: region.CountryCode,
			CountryName: region.CountryName,
			CityCode:    region.CityCode,
			CityName:    region.CityName,
			ClusterID:   region.ClusterID,
			ClusterName: region.ClusterName,
			Endpoints:   dtoRelayEndpointsToWSRelayEndpoints(region.Endpoints),
		})
	}
	return out
}

// dtoRelayEndpointsToWSRelayEndpoints 批量转换 relay 端点列表。
func dtoRelayEndpointsToWSRelayEndpoints(endpoints []dto.RelayEndpoint) []controlws.RelayEndpoint {
	out := make([]controlws.RelayEndpoint, 0, len(endpoints))
	for _, endpoint := range endpoints {
		out = append(out, controlws.RelayEndpoint{
			EndpointID: endpoint.EndpointID,
			Transport:  endpoint.Transport,
			Address:    endpoint.Address,
		})
	}
	return out
}
