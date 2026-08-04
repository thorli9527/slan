package bootstrap

import (
	"testing"
	"time"
)

func TestAuditRetentionCutoffDisabledByDefault(t *testing.T) {
	t.Setenv("SLAN_AUDIT_RETENTION_DAYS", "")
	if cutoff, enabled, err := auditRetentionCutoff(time.Unix(1_700_000_000, 0)); err != nil || enabled || cutoff != 0 {
		t.Fatalf("disabled retention = cutoff %d enabled %v err %v", cutoff, enabled, err)
	}
}

func TestAuditRetentionCutoffUsesConfiguredDays(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	t.Setenv("SLAN_AUDIT_RETENTION_DAYS", "180")
	cutoff, enabled, err := auditRetentionCutoff(now)
	if err != nil || !enabled {
		t.Fatalf("configured retention = cutoff %d enabled %v err %v", cutoff, enabled, err)
	}
	want := now.Add(-180 * 24 * time.Hour).Unix()
	if cutoff != want {
		t.Fatalf("retention cutoff = %d, want %d", cutoff, want)
	}
}

func TestAuditRetentionCutoffRejectsInvalidValues(t *testing.T) {
	for _, value := range []string{"-1", "invalid", "3651"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("SLAN_AUDIT_RETENTION_DAYS", value)
			if _, _, err := auditRetentionCutoff(time.Now()); err == nil {
				t.Fatalf("retention value %q was accepted", value)
			}
		})
	}
}
