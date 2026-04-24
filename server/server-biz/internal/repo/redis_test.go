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

func TestParseAccessTokenSessionSupportsLegacyUserID(t *testing.T) {
	session, err := parseAccessTokenSession([]byte(`user-legacy`))
	if err != nil {
		t.Fatalf("parse legacy session: %v", err)
	}
	if session.UserID != "user-legacy" || session.DeviceID != "" {
		t.Fatalf("unexpected legacy session: %+v", session)
	}
}

func TestParseAccessTokenSessionSupportsQuotedLegacyUserID(t *testing.T) {
	session, err := parseAccessTokenSession([]byte(`"user-quoted"`))
	if err != nil {
		t.Fatalf("parse quoted legacy session: %v", err)
	}
	if session.UserID != "user-quoted" {
		t.Fatalf("unexpected quoted legacy session: %+v", session)
	}
}
