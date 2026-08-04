package bootstrap

import (
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestDefaultOperatorUsesDefaultAdminCredentials(t *testing.T) {
	t.Setenv("SLAN_OPS_DEFAULT_ADMIN_EMAIL", "")
	t.Setenv("SLAN_OPS_DEFAULT_ADMIN_PASSWORD", "")
	operator, err := defaultOperator(1_700_000_000)
	if err != nil {
		t.Fatalf("defaultOperator returned error: %v", err)
	}
	if operator.Email != "admin1" || operator.Role != "admin" || !strings.HasPrefix(operator.PasswordHash, "$2") {
		t.Fatalf("unexpected bootstrap operator: %#v", operator)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(operator.PasswordHash), []byte("admin1")); err != nil {
		t.Fatalf("default operator password mismatch: %v", err)
	}
}

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

func TestDefaultPunchEndpointUsesPunchNodesEnv(t *testing.T) {
	t.Setenv("SLAN_WIRE_PUNCH_NODES", "local=47.245.40.231:29130")
	if got := DefaultPunchEndpoint(); got != "47.245.40.231:29130" {
		t.Fatalf("DefaultPunchEndpoint() = %q, want %q", got, "47.245.40.231:29130")
	}
}
