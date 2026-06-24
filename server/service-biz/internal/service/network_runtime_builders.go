package service

import (
	"context"
	"sort"
	"time"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

func listRelayNodeEntities(ctx context.Context, networks repository.NetworkRepository, ops repository.OpsRepository, nowFn func() time.Time, networkID string) ([]model.RelayNode, error) {
	networkID = normalizeNetworkID(networkID)
	if networkID == "" {
		return nil, ErrInvalidArgument
	}
	if _, err := requireManagedNetwork(ctx, networks, networkID); err != nil {
		return nil, err
	}
	if ops != nil {
		items, err := ops.ListRelayNodes(ctx)
		if err != nil {
			return nil, err
		}
		nodes := buildActiveRelayNodes(items, networkNow(nowFn).Unix())
		if len(nodes) > 0 {
			return nodes, nil
		}
	}
	return defaultRelayNodes(nowFn), nil
}

func buildActiveRelayNodes(items []model.RelayNode, nowUnix int64) []model.RelayNode {
	nodes := make([]model.RelayNode, 0, len(items))
	for _, item := range items {
		transport := firstNonEmpty(item.Transport, "relay_udp")
		if transport != "relay_udp" && transport != "derp_tcp_tls_443" {
			continue
		}
		if item.Status != "active" || item.Health != "healthy" || wireNodeStale(item.UpdatedAt, nowUnix) {
			continue
		}
		nodes = append(nodes, model.RelayNode{
			NodeID:    item.NodeID,
			Name:      item.Name,
			Region:    item.Region,
			Endpoint:  item.Endpoint,
			Transport: transport,
			Priority:  item.Priority,
			Status:    item.Status,
			Health:    item.Health,
			CreatedAt: item.CreatedAt,
			UpdatedAt: item.UpdatedAt,
		})
	}
	sortRelayNodes(nodes)
	return nodes
}

func defaultRelayNodes(nowFn func() time.Time) []model.RelayNode {
	now := networkNow(nowFn).Unix()
	return []model.RelayNode{{
		NodeID:    "relay-default",
		Name:      "Default Relay",
		Region:    "global",
		Endpoint:  "relay.default.local:3478",
		Transport: "relay_udp",
		Priority:  100,
		Status:    "active",
		Health:    "healthy",
		CreatedAt: now,
		UpdatedAt: now,
	}}
}

func sortRelayNodes(nodes []model.RelayNode) {
	sort.SliceStable(nodes, func(i, j int) bool {
		left := firstPositive(nodes[i].Priority, 100)
		right := firstPositive(nodes[j].Priority, 100)
		if left != right {
			return left < right
		}
		if nodes[i].Region != nodes[j].Region {
			return nodes[i].Region < nodes[j].Region
		}
		return nodes[i].NodeID < nodes[j].NodeID
	})
}

func relayCandidatesFromNodes(items []model.RelayNode) []RelayCandidateView {
	views := make([]RelayCandidateView, 0, len(items))
	for _, item := range items {
		views = append(views, relayCandidateView(item))
	}
	return views
}

func listPunchNodeEntities(ctx context.Context, ops repository.OpsRepository, nowFn func() time.Time) ([]model.PunchNode, error) {
	if ops != nil {
		items, err := ops.ListPunchNodes(ctx)
		if err != nil {
			return nil, err
		}
		nodes := buildActivePunchNodes(items)
		if len(nodes) > 0 {
			return nodes, nil
		}
	}
	return defaultPunchNodes(nowFn), nil
}

func buildActivePunchNodes(items []model.PunchNode) []model.PunchNode {
	nodes := make([]model.PunchNode, 0, len(items))
	for _, item := range items {
		if item.Status != "active" || item.Health != "healthy" {
			continue
		}
		if item.Endpoint == "" {
			continue
		}
		nodes = append(nodes, model.PunchNode{
			NodeID:    item.NodeID,
			Name:      item.Name,
			Region:    item.Region,
			Endpoint:  item.Endpoint,
			Status:    item.Status,
			Health:    item.Health,
			Priority:  item.Priority,
			CreatedAt: item.CreatedAt,
			UpdatedAt: item.UpdatedAt,
		})
	}
	sortPunchNodes(nodes)
	return nodes
}

func defaultPunchNodes(nowFn func() time.Time) []model.PunchNode {
	now := networkNow(nowFn).Unix()
	return []model.PunchNode{{
		NodeID:    "punch-default",
		Name:      "Default Punch",
		Region:    "global",
		Endpoint:  "punch.default.local:3478",
		Status:    "active",
		Health:    "healthy",
		Priority:  100,
		CreatedAt: now,
		UpdatedAt: now,
	}}
}

func sortPunchNodes(nodes []model.PunchNode) {
	sort.SliceStable(nodes, func(i, j int) bool {
		left := firstPositive(nodes[i].Priority, 100)
		right := firstPositive(nodes[j].Priority, 100)
		if left != right {
			return left < right
		}
		if nodes[i].Region != nodes[j].Region {
			return nodes[i].Region < nodes[j].Region
		}
		return nodes[i].NodeID < nodes[j].NodeID
	})
}

func punchNodeViews(items []model.PunchNode) []PunchNodeView {
	views := make([]PunchNodeView, 0, len(items))
	for _, item := range items {
		views = append(views, punchNodeView(item))
	}
	return views
}
