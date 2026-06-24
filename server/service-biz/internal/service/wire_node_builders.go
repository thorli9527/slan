package service

import "github.com/slan/service-biz/internal/model"

func relayWireNodeViews(items []model.RelayNode) []WireNodeView {
	views := make([]WireNodeView, 0, len(items))
	for _, item := range items {
		if item.Transport != "" && item.Transport != "relay_udp" {
			continue
		}
		views = append(views, relayNodeView(item))
	}
	return views
}

func derpWireNodeViews(items []model.RelayNode) []WireNodeView {
	views := make([]WireNodeView, 0, len(items))
	for _, item := range items {
		if item.Transport != "derp_tcp_tls_443" {
			continue
		}
		views = append(views, derpNodeView(item))
	}
	return views
}

func derpMapFromOpsNodes(items []model.RelayNode) WireDerpMapView {
	return derpMapFromNodes(derpRelayNodes(items))
}

func derpRelayNodes(items []model.RelayNode) []model.RelayNode {
	out := make([]model.RelayNode, 0, len(items))
	for _, item := range items {
		if item.Transport != "derp_tcp_tls_443" {
			continue
		}
		out = append(out, item)
	}
	return out
}
