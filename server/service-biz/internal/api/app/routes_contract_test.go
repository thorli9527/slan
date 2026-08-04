package app

import "testing"

func TestAppRoutesExposeDeviceScopedNetworkConfigsOnly(t *testing.T) {
	routes := Routes(RouteDependencies{})
	foundDeviceConfigs := false
	foundRuntimeEndpoints := false
	foundDeviceLogs := false
	for _, route := range routes {
		if route.Path == "/api/app/devices" || route.Path == "/api/app/devices/register" ||
			len(route.Path) >= len("/api/app/auth/") && route.Path[:len("/api/app/auth/")] == "/api/app/auth/" {
			t.Fatalf("client user route must not be exposed: %s", route.Key())
		}
		switch route.Path {
		case "/api/app/devices/{deviceId}/network-configs":
			foundDeviceConfigs = true
		case "/api/app/runtime/endpoints":
			foundRuntimeEndpoints = route.Method == "GET"
		case "/api/app/network-invites", "/api/app/network-invites/accept":
			t.Fatal("network invite routes must not be exposed to app clients")
		case "/api/app/devices/{deviceId}/logs":
			foundDeviceLogs = route.Method == "POST"
		case "/api/app/networks/{networkId}/network-config":
			t.Fatal("single-network config route must not be exposed to app clients")
		}
	}
	if !foundDeviceConfigs {
		t.Fatal("device-scoped network configs route is missing")
	}
	if !foundRuntimeEndpoints {
		t.Fatal("device-authenticated runtime endpoints route is missing")
	}
	if !foundDeviceLogs {
		t.Fatal("device-authenticated log upload route is missing")
	}
}
