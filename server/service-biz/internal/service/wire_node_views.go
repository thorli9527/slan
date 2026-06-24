package service

import (
	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/pkg/wirekit"
)

func relayNodeView(item model.RelayNode) WireNodeView {
	host, port := wirekit.SplitHostPort(item.Endpoint)
	return WireNodeView{
		RegionID:          item.Region,
		NodeID:            item.NodeID,
		Name:              item.Name,
		Host:              host,
		UDPPort:           port,
		AdminPort:         0,
		Enabled:           item.Status == "active",
		Healthy:           item.Health == "healthy",
		Stale:             wireNodeStale(item.UpdatedAt, wirekit.NowUnix()),
		Priority:          firstPositive(item.Priority, 100),
		UpdatedAtMS:       item.UpdatedAt * 1000,
		TicketKeyRotation: wireTicketKeyStatus(item),
	}
}

func derpNodeView(item model.RelayNode) WireNodeView {
	host, port := wirekit.SplitHostPort(item.Endpoint)
	return WireNodeView{
		RegionID:          item.Region,
		NodeID:            item.NodeID,
		Name:              item.Name,
		Host:              host,
		Port:              port,
		Enabled:           item.Status == "active",
		Healthy:           item.Health == "healthy",
		Stale:             wireNodeStale(item.UpdatedAt, wirekit.NowUnix()),
		Priority:          firstPositive(item.Priority, 100),
		UpdatedAtMS:       item.UpdatedAt * 1000,
		TicketKeyRotation: wireTicketKeyStatus(item),
	}
}

func derpMapFromNodes(items []model.RelayNode) WireDerpMapView {
	regions := make(map[string][]wirekit.DerpNode)
	preferred := ""
	now := wirekit.NowUnix()
	for _, item := range items {
		if item.Transport != "" && item.Transport != "derp_tcp_tls_443" {
			continue
		}
		if item.Status != "active" || item.Health != "healthy" || wireNodeStale(item.UpdatedAt, now) {
			continue
		}
		host, port := wirekit.SplitHostPort(item.Endpoint)
		if preferred == "" {
			preferred = item.Region
		}
		regions[item.Region] = append(regions[item.Region], wirekit.DerpNode{
			RegionID: item.Region,
			NodeID:   item.NodeID,
			Host:     host,
			Port:     port,
		})
	}
	out := make([]wirekit.DerpRegion, 0, len(regions))
	for regionID, nodes := range regions {
		out = append(out, wirekit.DerpRegion{RegionID: regionID, Name: regionID, Nodes: nodes})
	}
	return WireDerpMapView{PreferredRegionID: preferred, Regions: out}
}
