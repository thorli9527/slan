package mqttkit

import (
	"testing"
	"time"
)

func TestValidateServerCredentialAcceptsServiceBizClientID(t *testing.T) {
	cfg := DefaultConfig()
	now := time.Unix(1_782_912_800, 0)

	cred := CredentialForSystem(cfg, ServerID, now)
	if cred.ClientID == "" || cred.Username == "" || cred.Password == "" {
		t.Fatalf("expected complete system credential, got %+v", cred)
	}

	result, ok := ValidateCredential(
		cfg,
		cred.ClientID,
		cred.Username,
		cred.Password,
		now.Add(30*time.Second),
	)
	if !ok {
		t.Fatalf("expected service-biz system credential to validate")
	}
	if result.Principal != "server" {
		t.Fatalf("expected principal server, got %q", result.Principal)
	}
}

func TestValidateDeviceCredentialBindsCredentialIDAndExpiry(t *testing.T) {
	cfg := DefaultConfig()
	now := time.Unix(1_782_912_800, 0)
	limit := now.Add(time.Minute).Unix()
	cred := CredentialForDevice(cfg, "device-1", "dcred-1", now, limit)
	if cred == nil || cred.ExpiresAt != limit {
		t.Fatalf("expected credential capped at %d, got %+v", limit, cred)
	}
	result, ok := ValidateCredential(cfg, cred.ClientID, cred.Username, cred.Password, now)
	if !ok || result.DeviceID != "device-1" || result.CredentialID != "dcred-1" {
		t.Fatalf("unexpected device credential result: result=%+v ok=%t", result, ok)
	}
	if _, ok := ValidateCredential(cfg, cred.ClientID, cred.Username, cred.Password, time.Unix(limit, 0)); ok {
		t.Fatal("credential must be invalid at its exact expiry")
	}
}
