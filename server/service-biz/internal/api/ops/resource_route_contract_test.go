package ops

import "testing"

func TestResourceRoutesCoverGlobalManagement(t *testing.T) {
	want := map[string]bool{
		"PUT /api/ops/devices/{deviceId}/groups":                       false,
		"GET /api/ops/networks/{networkId}/devices":                    false,
		"POST /api/ops/networks/{networkId}/devices/{deviceId}":        false,
		"DELETE /api/ops/networks/{networkId}/devices/{deviceId}":      false,
		"POST /api/ops/networks/{networkId}/device-groups":             false,
		"DELETE /api/ops/networks/{networkId}/device-groups/{groupId}": false,
		"POST /api/ops/networks/{networkId}/security-groups":           false,
		"PATCH /api/ops/security-groups/{securityGroupId}":             false,
		"DELETE /api/ops/security-groups/{securityGroupId}":            false,
		"POST /api/ops/security-groups/{securityGroupId}/rules":        false,
		"PATCH /api/ops/security-rules/{ruleId}":                       false,
		"DELETE /api/ops/security-rules/{ruleId}":                      false,
		"POST /api/ops/networks/{networkId}/dns/zones":                 false,
		"PATCH /api/ops/dns/zones/{zoneId}":                            false,
		"DELETE /api/ops/dns/zones/{zoneId}":                           false,
		"POST /api/ops/networks/{networkId}/dns/records":               false,
		"PATCH /api/ops/dns/records/{recordId}":                        false,
		"DELETE /api/ops/dns/records/{recordId}":                       false,
	}
	for _, route := range (ResourceHandler{}).Routes() {
		if _, ok := want[route.Key()]; ok {
			want[route.Key()] = true
		}
	}
	for key, found := range want {
		if !found {
			t.Fatalf("resource route missing: %s", key)
		}
	}
}
