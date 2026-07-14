package web

import "testing"

func TestNetworkRoutesExposeOnlyGroupBasedMembershipManagement(t *testing.T) {
	routes := Routes(RouteDependencies{})
	keys := make(map[string]struct{}, len(routes))
	for _, route := range routes {
		keys[route.Key()] = struct{}{}
	}
	for _, required := range []string{
		"GET /api/web/networks/{networkId}/devices",
		"GET /api/web/networks/{networkId}/device-groups",
		"POST /api/web/networks/{networkId}/device-groups",
		"DELETE /api/web/networks/{networkId}/device-groups/{groupId}",
	} {
		if _, ok := keys[required]; !ok {
			t.Fatalf("required route is missing: %s", required)
		}
	}
	for _, retired := range []string{
		"POST /api/web/networks/{networkId}/devices",
		"PATCH /api/web/networks/{networkId}/devices/{deviceId}",
		"DELETE /api/web/networks/{networkId}/devices/{deviceId}",
		"GET /api/web/networks/{networkId}/public-mappings",
		"POST /api/web/networks/{networkId}/public-mappings",
		"PATCH /api/web/networks/{networkId}/public-mappings/{mappingId}",
		"DELETE /api/web/networks/{networkId}/public-mappings/{mappingId}",
	} {
		if _, ok := keys[retired]; ok {
			t.Fatalf("retired route is still registered: %s", retired)
		}
	}
}
