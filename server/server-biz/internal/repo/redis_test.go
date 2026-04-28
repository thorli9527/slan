package repo

import (
	"testing"
)

func TestParseAccessTokenSessionSupportsJSONSession(t *testing.T) {
	session, err := parseAccessTokenSession([]byte(`{"userId":"user-1","deviceId":"dev-1","issuedAt":123}`))
	if err != nil {
		t.Fatalf("parse json session: %v", err)
	}
	if session.UserID != "user-1" || session.DeviceID != "dev-1" || session.IssuedAt != 123 {
		t.Fatalf("unexpected session: %+v", session)
	}
}

func TestParseAccessTokenSessionRejectsNonJSONSession(t *testing.T) {
	if _, err := parseAccessTokenSession([]byte(`user-1`)); err == nil {
		t.Fatal("expected non-json token session to be rejected")
	}
}
