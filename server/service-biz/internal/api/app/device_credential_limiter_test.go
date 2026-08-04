package app

import (
	"testing"
	"time"
)

func TestDeviceCredentialExchangeLimiterBlocksAndRecovers(t *testing.T) {
	limiter := newDeviceCredentialExchangeLimiter()
	now := time.Unix(1_700_000_000, 0)
	for attempt := 0; attempt < deviceCredentialFailureLimit; attempt++ {
		if !limiter.Allow("127.0.0.1", "key-a", now) {
			t.Fatalf("attempt %d blocked before limit", attempt+1)
		}
		limiter.RecordFailure("127.0.0.1", "key-a", now)
	}
	if limiter.Allow("127.0.0.1", "key-a", now) {
		t.Fatal("expected key to be blocked at failure limit")
	}
	if limiter.Allow("127.0.0.1", "key-b", now) {
		t.Fatal("expected source IP aggregate limit to block key rotation")
	}
	if !limiter.Allow("127.0.0.1", "key-a", now.Add(deviceCredentialLimitWindow)) {
		t.Fatal("expected limiter to recover after window")
	}
	limiter.RecordFailure("127.0.0.2", "key-c", now)
	limiter.RecordSuccess("127.0.0.2", "key-c")
	if !limiter.Allow("127.0.0.2", "key-d", now) {
		t.Fatal("expected successful exchange to clear source IP failures")
	}
}
