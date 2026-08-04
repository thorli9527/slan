package mqtt

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestMQTTAuthLimiterBlocksIdentityFailuresAndRecovers(t *testing.T) {
	limiter := newMQTTAuthLimiter()
	now := time.Unix(1_700_000_000, 0)
	identity := mqttAuthIdentity("client-1", "device-user")
	for attempt := 0; attempt < mqttAuthIdentityFailureLimit; attempt++ {
		if !limiter.Allow("broker-1", identity, now) {
			t.Fatalf("attempt %d blocked before limit", attempt+1)
		}
		limiter.RecordFailure("broker-1", identity, now)
	}
	if limiter.Allow("broker-2", identity, now) {
		t.Fatal("expected identity to remain blocked across broker sources")
	}
	if !limiter.Allow("broker-1", identity, now.Add(mqttAuthFailureWindow)) {
		t.Fatal("expected limiter to recover after window")
	}
	limiter.RecordFailure("broker-1", identity, now)
	limiter.RecordSuccess(identity)
	if !limiter.Allow("broker-1", identity, now) {
		t.Fatal("expected successful authentication to clear identity failures")
	}
}

func TestMQTTAuthIdentityDoesNotContainCredentials(t *testing.T) {
	const clientID = "secret-client-id"
	const username = "secret-username"
	identity := mqttAuthIdentity(clientID, username)
	if identity == "" || strings.Contains(identity, clientID) || strings.Contains(identity, username) {
		t.Fatalf("unsafe MQTT limiter identity: %q", identity)
	}
}

func TestMQTTAuthLimiterBlocksSourceWithRotatingIdentities(t *testing.T) {
	limiter := newMQTTAuthLimiter()
	now := time.Unix(1_700_000_000, 0)
	for attempt := 0; attempt < mqttAuthSourceFailureLimit; attempt++ {
		identity := mqttAuthIdentity("client-"+strconv.Itoa(attempt), "device-user")
		if !limiter.Allow("broker-1", identity, now) {
			t.Fatalf("source attempt %d blocked before limit", attempt+1)
		}
		limiter.RecordFailure("broker-1", identity, now)
	}
	if limiter.Allow("broker-1", mqttAuthIdentity("client-next", "device-user"), now) {
		t.Fatal("expected source aggregate failure limit")
	}
}
