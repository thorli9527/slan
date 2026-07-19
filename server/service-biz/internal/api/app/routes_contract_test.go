package app

import "testing"

func TestAppRoutesExposeDeviceScopedNetworkConfigsOnly(t *testing.T) {
	routes := Routes(RouteDependencies{})
	foundDeviceConfigs := false
	foundRuntimeEndpoints := false
	for _, route := range routes {
		switch route.Path {
		case "/api/app/devices/{deviceId}/network-configs":
			foundDeviceConfigs = true
		case "/api/app/runtime/endpoints":
			foundRuntimeEndpoints = route.Method == "GET"
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
}
