package app

import "testing"

func TestAppRoutesExposeDeviceScopedNetworkConfigsOnly(t *testing.T) {
	routes := Routes(RouteDependencies{})
	foundDeviceConfigs := false
	foundRuntimeEndpoints := false
	foundCreateInvite := false
	foundAcceptInvite := false
	foundDeviceLogs := false
	for _, route := range routes {
		switch route.Path {
		case "/api/app/devices/{deviceId}/network-configs":
			foundDeviceConfigs = true
		case "/api/app/runtime/endpoints":
			foundRuntimeEndpoints = route.Method == "GET"
		case "/api/app/network-invites":
			foundCreateInvite = route.Method == "POST"
		case "/api/app/network-invites/accept":
			foundAcceptInvite = route.Method == "POST"
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
	if !foundCreateInvite || !foundAcceptInvite {
		t.Fatal("network invite create/accept routes are missing")
	}
	if !foundDeviceLogs {
		t.Fatal("device-authenticated log upload route is missing")
	}
}
