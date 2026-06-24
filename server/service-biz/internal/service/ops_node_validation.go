package service

import (
	"context"
	"net"
	"strconv"
	"strings"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

const (
	relayTransportUDP      = "relay_udp"
	relayTransportDerpTLS  = "derp_tcp_tls_443"
	nodeStatusActive       = "active"
	nodeStatusMaintenance  = "maintenance"
	nodeStatusDisabled     = "disabled"
	nodeHealthHealthy      = "healthy"
	nodeHealthWarning      = "warning"
	nodeHealthDown         = "down"
)

func validateRelayNodeModel(ctx context.Context, catalog repository.OpsRepository, item model.RelayNode) error {
	if strings.TrimSpace(item.Name) == "" {
		return invalidArgumentError("请输入节点名称")
	}
	if !validRelayTransport(item.Transport) {
		return invalidArgumentError("中继传输类型不支持")
	}
	host, port, ok := parseValidatedEndpoint(item.Endpoint)
	if !ok || !isIPv4Address(host) {
		return invalidArgumentError("公网 IP 必须使用 IPv4 地址，不能使用域名")
	}
	if port <= 0 || port > 65535 {
		return invalidArgumentError("公网端口必须在 1-65535 范围内")
	}
	if !validNodeStatus(item.Status) {
		return invalidArgumentError("节点状态不支持")
	}
	if !validNodeHealth(item.Health) {
		return invalidArgumentError("节点健康状态不支持")
	}
	if item.MaxBandwidthMbps < 0 || item.MonthlyTrafficGB < 0 || item.UsedTrafficGB < 0 {
		return invalidArgumentError("带宽和流量不能为负数")
	}
	if item.MaxSessions < 0 || item.ActiveSessions < 0 || item.Priority < 0 {
		return invalidArgumentError("会话数和优先级不能为负数")
	}
	if item.MaxSessions > 0 && item.ActiveSessions > item.MaxSessions {
		return invalidArgumentError("当前会话数不能大于最大会话数")
	}
	return ensureRelayEndpointUnique(ctx, catalog, item.NodeID, net.JoinHostPort(host, strconv.Itoa(port)))
}

func validatePunchNodeModel(ctx context.Context, catalog repository.OpsRepository, item model.PunchNode) error {
	if strings.TrimSpace(item.Name) == "" {
		return invalidArgumentError("请输入打洞节点名称")
	}
	host, port, ok := parseValidatedEndpoint(item.Endpoint)
	if !ok || !isIPv4Address(host) {
		return invalidArgumentError("公网 UDP IP 必须使用 IPv4 地址，不能使用域名")
	}
	if port <= 0 || port > 65534 {
		return invalidArgumentError("公网 UDP 端口必须在 1-65534 范围内")
	}
	if !validNodeStatus(item.Status) {
		return invalidArgumentError("节点状态不支持")
	}
	if !validNodeHealth(item.Health) {
		return invalidArgumentError("节点健康状态不支持")
	}
	if item.MaxSessions < 0 || item.ActiveSessions < 0 || item.Priority < 0 {
		return invalidArgumentError("会话数和优先级不能为负数")
	}
	if item.MaxSessions > 0 && item.ActiveSessions > item.MaxSessions {
		return invalidArgumentError("当前会话数不能大于最大会话数")
	}
	return ensurePunchEndpointUnique(ctx, catalog, item.NodeID, net.JoinHostPort(host, strconv.Itoa(port)))
}

func validRelayTransport(value string) bool {
	switch strings.TrimSpace(value) {
	case "", relayTransportUDP, relayTransportDerpTLS:
		return true
	default:
		return false
	}
}

func validNodeStatus(value string) bool {
	switch strings.TrimSpace(value) {
	case "", nodeStatusActive, nodeStatusMaintenance, nodeStatusDisabled:
		return true
	default:
		return false
	}
}

func validNodeHealth(value string) bool {
	switch strings.TrimSpace(value) {
	case "", nodeHealthHealthy, nodeHealthWarning, nodeHealthDown:
		return true
	default:
		return false
	}
}

func parseValidatedEndpoint(endpoint string) (string, int, bool) {
	host, port := splitOpsEndpoint(endpoint)
	host = strings.TrimSpace(host)
	return host, port, host != "" && port > 0
}

func isIPv4Address(value string) bool {
	ip := net.ParseIP(strings.TrimSpace(value))
	return ip != nil && ip.To4() != nil
}

func ensureRelayEndpointUnique(ctx context.Context, catalog repository.OpsRepository, currentNodeID, endpoint string) error {
	items, err := catalog.ListRelayNodes(ctx)
	if err != nil {
		return err
	}
	for _, item := range items {
		if normalizeNodeID(item.NodeID) == normalizeNodeID(currentNodeID) {
			continue
		}
		if normalizeNodeEndpoint(item.Endpoint) == normalizeNodeEndpoint(endpoint) {
			return conflictError("公网地址已存在，不能重复配置到多个中继节点")
		}
	}
	return nil
}

func ensurePunchEndpointUnique(ctx context.Context, catalog repository.OpsRepository, currentNodeID, endpoint string) error {
	items, err := catalog.ListPunchNodes(ctx)
	if err != nil {
		return err
	}
	for _, item := range items {
		if normalizeNodeID(item.NodeID) == normalizeNodeID(currentNodeID) {
			continue
		}
		if normalizeNodeEndpoint(item.Endpoint) == normalizeNodeEndpoint(endpoint) {
			return conflictError("公网 UDP IP 和端口已存在，不能重复配置到多个打洞节点")
		}
	}
	return nil
}
