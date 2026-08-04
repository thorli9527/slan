package ops

import "testing"

func TestOpsRoutesExposeNetworkPolicyManagement(t *testing.T) {
	want := map[string]bool{
		"GET /api/ops/networks/{networkId}/policy":              false,
		"GET /api/ops/networks/{networkId}/dns/zones":           false,
		"POST /api/ops/networks/{networkId}/dns/zones":          false,
		"PATCH /api/ops/dns/zones/{zoneId}":                     false,
		"DELETE /api/ops/dns/zones/{zoneId}":                    false,
		"GET /api/ops/networks/{networkId}/dns/records":         false,
		"POST /api/ops/networks/{networkId}/dns/records":        false,
		"PATCH /api/ops/dns/records/{recordId}":                 false,
		"DELETE /api/ops/dns/records/{recordId}":                false,
		"GET /api/ops/networks/{networkId}/security-groups":     false,
		"POST /api/ops/networks/{networkId}/security-groups":    false,
		"PATCH /api/ops/security-groups/{securityGroupId}":      false,
		"DELETE /api/ops/security-groups/{securityGroupId}":     false,
		"GET /api/ops/security-groups/{securityGroupId}/rules":  false,
		"POST /api/ops/security-groups/{securityGroupId}/rules": false,
		"PATCH /api/ops/security-rules/{ruleId}":                false,
		"DELETE /api/ops/security-rules/{ruleId}":               false,
	}
	for _, route := range Routes(RouteDependencies{}) {
		if _, ok := want[route.Key()]; ok {
			want[route.Key()] = true
		}
	}
	for key, found := range want {
		if !found {
			t.Errorf("missing Ops network policy route: %s", key)
		}
	}
}
