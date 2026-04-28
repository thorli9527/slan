package httpapi

import (
	"github.com/slan/server/server-biz/api/dto"
	controlmsg "github.com/slan/server/server-biz/internal/controlmsg"
)

// dtoNetworkMapToControl maps the business DTO network map into the control payload.
func dtoNetworkMapToControl(m dto.NetworkMap) controlmsg.NetworkMap {
	return controlmsg.NetworkMap{
		SelfUserID:       m.SelfUserID,
		SelfDeviceID:     m.SelfDeviceID,
		SelfNodeID:       m.SelfNodeID,
		NetworkID:        m.NetworkID,
		Revision:         m.Revision,
		HeartbeatSeconds: m.HeartbeatSeconds,
		STUNServers:      append([]string(nil), m.STUNServers...),
		Peers:            dtoPeersToControlPeers(m.Peers),
		Routes:           dtoRoutesToControlRoutes(m.Routes),
		RelayRegions:     dtoRelayRegionsToControlRelayRegions(m.RelayRegions),
		DNS: controlmsg.DNSConfig{
			Servers:       append([]string(nil), m.DNS.Servers...),
			SearchDomains: append([]string(nil), m.DNS.SearchDomains...),
			Wildcards:     append([]string(nil), m.DNS.Wildcards...),
		},
		Policy: controlmsg.AccessPolicy{
			PlanCode:                m.Policy.PlanCode,
			MaxActiveDevices:        m.Policy.MaxActiveDevices,
			BandwidthLimitMbps:      m.Policy.BandwidthLimitMbps,
			RelayBandwidthLimitKbps: m.Policy.RelayBandwidthLimitKbps,
			P2PUnlimited:            m.Policy.P2PUnlimited,
			DNSAvailable:            m.Policy.DNSAvailable,
		},
		MTU: m.MTU,
	}
}

// dtoPeersToControlPeers maps peer DTOs into control payload peers.
func dtoPeersToControlPeers(peers []dto.Peer) []controlmsg.Peer {
	out := make([]controlmsg.Peer, 0, len(peers))
	for _, peer := range peers {
		out = append(out, controlmsg.Peer{
			NodeID:        peer.NodeID,
			DeviceID:      peer.DeviceID,
			PublicKey:     peer.PublicKey,
			Status:        peer.Status,
			RelayAllowed:  peer.RelayAllowed,
			VirtualIPs:    append([]string(nil), peer.VirtualIPs...),
			Endpoints:     dtoEndpointsToControlEndpoints(peer.Endpoints),
			AllowedRoutes: append([]string(nil), peer.AllowedRoutes...),
		})
	}
	return out
}

// dtoEndpointsToControlEndpoints maps endpoint DTOs into control payload endpoints.
func dtoEndpointsToControlEndpoints(endpoints []dto.Endpoint) []controlmsg.Endpoint {
	out := make([]controlmsg.Endpoint, 0, len(endpoints))
	for _, endpoint := range endpoints {
		out = append(out, controlmsg.Endpoint{
			Type:      endpoint.Type,
			Address:   endpoint.Address,
			UpdatedAt: endpoint.UpdatedAt,
		})
	}
	return out
}

// dtoRoutesToControlRoutes maps route DTOs into control payload routes.
func dtoRoutesToControlRoutes(routes []dto.Route) []controlmsg.Route {
	out := make([]controlmsg.Route, 0, len(routes))
	for _, route := range routes {
		out = append(out, controlmsg.Route{
			CIDR:      route.CIDR,
			ViaNodeID: route.ViaNodeID,
			Metric:    route.Metric,
		})
	}
	return out
}

// dtoRelayRegionsToControlRelayRegions maps relay region DTOs into control payload regions.
func dtoRelayRegionsToControlRelayRegions(regions []dto.RelayRegion) []controlmsg.RelayRegion {
	out := make([]controlmsg.RelayRegion, 0, len(regions))
	for _, region := range regions {
		out = append(out, controlmsg.RelayRegion{
			RegionID:    region.RegionID,
			RegionName:  region.RegionName,
			CountryCode: region.CountryCode,
			CountryName: region.CountryName,
			CityCode:    region.CityCode,
			CityName:    region.CityName,
			ClusterID:   region.ClusterID,
			ClusterName: region.ClusterName,
			Endpoints:   dtoRelayEndpointsToControlRelayEndpoints(region.Endpoints),
		})
	}
	return out
}

// dtoRelayEndpointsToControlRelayEndpoints maps relay endpoint DTOs into control payload endpoints.
func dtoRelayEndpointsToControlRelayEndpoints(endpoints []dto.RelayEndpoint) []controlmsg.RelayEndpoint {
	out := make([]controlmsg.RelayEndpoint, 0, len(endpoints))
	for _, endpoint := range endpoints {
		out = append(out, controlmsg.RelayEndpoint{
			EndpointID: endpoint.EndpointID,
			Transport:  endpoint.Transport,
			Address:    endpoint.Address,
		})
	}
	return out
}
