package app

import (
	"strings"
	"testing"
	"time"
)

func TestDeviceRequestLimiterEnforcesIdentityAndIPLimits(t *testing.T) {
	limiter := newDeviceRequestLimiter(2, 2)
	now := time.Unix(1_700_000_000, 0)
	identity := deviceRequestIdentity("access-secret", "refresh-secret")
	for attempt := 0; attempt < 2; attempt++ {
		if !limiter.Allow("127.0.0.1", identity, now) {
			t.Fatalf("attempt %d blocked before limit", attempt+1)
		}
	}
	if limiter.Allow("127.0.0.2", identity, now) {
		t.Fatal("expected identity limit across source IPs")
	}
	if limiter.Allow("127.0.0.1", deviceRequestIdentity("other-token"), now) {
		t.Fatal("expected source IP aggregate limit")
	}
	if !limiter.Allow("127.0.0.1", identity, now.Add(deviceRequestLimitWindow)) {
		t.Fatal("expected limiter recovery after window")
	}
}

func TestDeviceRequestIdentityDoesNotContainCredential(t *testing.T) {
	const secret = "device-token-secret"
	identity := deviceRequestIdentity(secret)
	if identity == "" || strings.Contains(identity, secret) {
		t.Fatalf("unsafe limiter identity: %q", identity)
	}
}
