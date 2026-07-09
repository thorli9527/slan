package bootstrap

import "testing"

func TestDefaultRelayEndpointPrefersRelayEndpointsEnv(t *testing.T) {
	t.Setenv("SLAN_RELAY_ENDPOINTS", "47.245.40.231:29110,47.245.40.231:29112")
	t.Setenv("SLAN_RELAY_UDP_ADDR", "")
	t.Setenv("SLAN_WIRE_RELAY_PUBLIC_HOST", "")
	t.Setenv("SLAN_WIRE_RELAY_PUBLIC_UDP_PORT", "")
	if got := DefaultRelayEndpoint(); got != "47.245.40.231:29110" {
		t.Fatalf("DefaultRelayEndpoint() = %q, want %q", got, "47.245.40.231:29110")
	}
}

func TestDefaultRelayEndpointFallsBackToPublicRelayHost(t *testing.T) {
	t.Setenv("SLAN_RELAY_ENDPOINTS", "")
	t.Setenv("SLAN_RELAY_UDP_ADDR", "")
	t.Setenv("SLAN_WIRE_RELAY_PUBLIC_HOST", "47.245.40.231")
	t.Setenv("SLAN_WIRE_RELAY_PUBLIC_UDP_PORT", "29110")
	if got := DefaultRelayEndpoint(); got != "47.245.40.231:29110" {
		t.Fatalf("DefaultRelayEndpoint() = %q, want %q", got, "47.245.40.231:29110")
	}
}
