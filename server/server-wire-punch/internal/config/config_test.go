package config

import (
	"strings"
	"testing"
)

func TestProductionConfigRequiresBusinessRegistration(t *testing.T) {
	t.Setenv("SLAN_ENV", "production")
	t.Setenv("SLAN_WIRE_PUNCH_PUBLIC_HOST", "203.0.113.10")
	t.Setenv("SLAN_INTERNAL_WIRE_TOKEN", "production-wire-token")
	t.Setenv("SLAN_WIRE_PUNCH_NODE_ID", "punch-1")
	t.Setenv("SLAN_BIZ_URL", "")

	err := Load().Validate()
	if err == nil || !strings.Contains(err.Error(), "SLAN_BIZ_URL is required") {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestProductionConfigAcceptsRuntimeRegistrationSettings(t *testing.T) {
	t.Setenv("SLAN_ENV", "production")
	t.Setenv("SLAN_WIRE_PUNCH_PUBLIC_HOST", "203.0.113.10")
	t.Setenv("SLAN_INTERNAL_WIRE_TOKEN", "production-wire-token")
	t.Setenv("SLAN_WIRE_PUNCH_NODE_ID", "punch-1")
	t.Setenv("SLAN_BIZ_URL", "https://biz.example.test")

	if err := Load().Validate(); err != nil {
		t.Fatal(err)
	}
}
