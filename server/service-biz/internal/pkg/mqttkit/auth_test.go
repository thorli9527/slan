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
