package ops

import (
	"testing"
	"time"
)

func TestOpsLoginLimiterBlocksIPAndAccount(t *testing.T) {
	limiter := newOpsLoginLimiter()
	now := time.Unix(1_700_000_000, 0)
	for attempt := 0; attempt < opsLoginFailureLimit; attempt++ {
		if !limiter.Allow("127.0.0.1", "operator@example.com", now) {
			t.Fatalf("attempt %d blocked before limit", attempt+1)
		}
		limiter.RecordFailure("127.0.0.1", "operator@example.com", now)
	}
	if limiter.Allow("127.0.0.1", "other@example.com", now) {
		t.Fatal("source IP was not blocked")
	}
	if limiter.Allow("127.0.0.2", "operator@example.com", now) {
		t.Fatal("account was not blocked across source IPs")
	}
	limiter.RecordSuccess("127.0.0.1", "operator@example.com")
	if !limiter.Allow("127.0.0.1", "operator@example.com", now) {
		t.Fatal("successful login did not clear failures")
	}
}
