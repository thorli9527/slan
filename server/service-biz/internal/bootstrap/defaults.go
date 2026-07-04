package bootstrap

import (
	"os"
	"strings"

	"github.com/slan/service-biz/internal/model"
)

type counterSeed struct {
	Name  string
	Value int64
}

func defaultOperator(now int64) model.Operator {
	return model.Operator{
		OperatorID:   "op00000000000000000000000000000001",
		Email:        "admin1",
		Name:         "超级管理员",
		PasswordHash: "plain:admin1",
		Role:         "super_admin",
		Status:       "active",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

func defaultPlans(now int64) []model.Plan {
	return []model.Plan{
		{PlanCode: "free", Name: "免费版", DeviceLimit: 10, Status: "active", UpdatedAt: now},
		{PlanCode: "pro", Name: "专业版", DeviceLimit: 130, Status: "active", UpdatedAt: now},
		{PlanCode: "enterprise", Name: "企业版", DeviceLimit: 1000, Status: "active", UpdatedAt: now},
	}
}

func defaultProducts(now int64) []model.Product {
	return []model.Product{
		{ProductID: "product000000000000000000000000000001", Name: "专业版月付", PlanCode: "pro", Price: 39, Status: "active", CreatedAt: now, UpdatedAt: now},
		{ProductID: "product000000000000000000000000000002", Name: "专业版年付", PlanCode: "pro", Price: 299, Status: "active", CreatedAt: now, UpdatedAt: now},
		{ProductID: "product000000000000000000000000000003", Name: "企业版年付", PlanCode: "enterprise", Price: 2999, Status: "active", CreatedAt: now, UpdatedAt: now},
	}
}

func defaultDownloads(now int64) []model.ClientDownload {
	return []model.ClientDownload{
		{DownloadID: "download0000000000000000000000000001", Name: "slan-client-linux.tar.gz", Platform: "linux", Version: "0.1.0", URL: "/downloads/clients/slan-client-linux.tar.gz", Status: "active", CreatedAt: now, UpdatedAt: now},
		{DownloadID: "download0000000000000000000000000002", Name: "slan-client-macos.tar.gz", Platform: "macos", Version: "0.1.0", URL: "/downloads/clients/slan-client-macos.tar.gz", Status: "active", CreatedAt: now, UpdatedAt: now},
		{DownloadID: "download0000000000000000000000000003", Name: "slan-client-windows.zip", Platform: "windows", Version: "0.1.0", URL: "/downloads/clients/slan-client-windows.zip", Status: "active", CreatedAt: now, UpdatedAt: now},
	}
}

func defaultRelayNode(now int64) model.RelayNode {
	return model.RelayNode{
		NodeID:    "relay000000000000000000000000000001",
		Name:      "默认 UDP Relay",
		Region:    defaultOpsRegion("SLAN_RELAY_REGION_ID", "local"),
		Endpoint:  DefaultRelayEndpoint(),
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func defaultPunchNode(now int64) (model.PunchNode, bool) {
	raw := strings.TrimSpace(os.Getenv("SLAN_WIRE_PUNCH_NODES"))
	if raw == "" {
		return model.PunchNode{}, false
	}
	first := strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == ';' || r == '\n' })
	if len(first) == 0 {
		return model.PunchNode{}, false
	}
	addr := strings.TrimSpace(first[0])
	if before, after, ok := strings.Cut(addr, "="); ok {
		if strings.TrimSpace(after) != "" {
			addr = strings.TrimSpace(after)
		} else {
			addr = strings.TrimSpace(before)
		}
	}
	return model.PunchNode{
		NodeID:    "punch000000000000000000000000000001",
		Name:      "Punch 1",
		Region:    defaultOpsRegion("SLAN_WIRE_PUNCH_REGION_ID", "default"),
		Endpoint:  addr,
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	}, strings.TrimSpace(addr) != ""
}

func defaultPunchEnabled() bool {
	_, ok := defaultPunchNode(0)
	return ok
}

func defaultSeedCounters() []counterSeed {
	counters := []counterSeed{
		{Name: "operator", Value: 1},
		{Name: "relay_node", Value: 1},
		{Name: "client_download", Value: 3},
		{Name: "product", Value: 3},
	}
	if defaultPunchEnabled() {
		counters = append(counters, counterSeed{Name: "punch_node", Value: 1})
	}
	return counters
}

func DefaultRelayEndpoint() string {
	if raw := strings.TrimSpace(os.Getenv("SLAN_RELAY_ENDPOINTS")); raw != "" {
		parts := strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == ';' || r == '\n' })
		if len(parts) > 0 {
			return normalizeRelayAddress(parts[0])
		}
	}
	if raw := strings.TrimSpace(os.Getenv("SLAN_RELAY_UDP_ADDR")); raw != "" {
		return normalizeRelayAddress(raw)
	}
	return "127.0.0.1:3478"
}

func normalizeRelayAddress(value string) string {
	value = strings.TrimSpace(value)
	if before, after, ok := strings.Cut(value, "="); ok {
		if strings.TrimSpace(after) != "" {
			value = strings.TrimSpace(after)
		} else {
			value = strings.TrimSpace(before)
		}
	}
	for _, prefix := range []string{"udp://", "derp://", "derp+tcp+tls://"} {
		if strings.HasPrefix(strings.ToLower(value), prefix) {
			return strings.TrimSpace(value[len(prefix):])
		}
	}
	return value
}

func defaultOpsRegion(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
