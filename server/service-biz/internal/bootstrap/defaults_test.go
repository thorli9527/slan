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
