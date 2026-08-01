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
	endpoint := DefaultPunchEndpoint()
	if endpoint == "" {
		return model.PunchNode{}, false
	}
	return model.PunchNode{
		NodeID:    "punch000000000000000000000000000001",
		Name:      "Punch 1",
		Region:    defaultOpsRegion("SLAN_WIRE_PUNCH_REGION_ID", "default"),
		Endpoint:  endpoint,
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	}, true
}

func defaultPunchEnabled() bool {
	_, ok := defaultPunchNode(0)
	return ok
}

func defaultSeedCounters() []counterSeed {
	counters := []counterSeed{
		{Name: "operator", Value: 1},
		{Name: "relay_node", Value: 1},
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
	if host := strings.TrimSpace(os.Getenv("SLAN_WIRE_RELAY_PUBLIC_HOST")); host != "" {
		port := strings.TrimSpace(os.Getenv("SLAN_WIRE_RELAY_PUBLIC_UDP_PORT"))
		if port == "" {
			port = "29110"
		}
		return host + ":" + port
	}
	return "127.0.0.1:29110"
}

func DefaultPunchEndpoint() string {
	raw := strings.TrimSpace(os.Getenv("SLAN_WIRE_PUNCH_NODES"))
	if raw == "" {
		return ""
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == ';' || r == '\n' })
	if len(parts) == 0 {
		return ""
	}
	return normalizeRelayAddress(parts[0])
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
