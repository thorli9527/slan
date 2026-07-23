package service

import "testing"

func TestNormalizeDNSRecordInputKeepsPortOnlyForSRV(t *testing.T) {
	aRecord := normalizeCreateDNSRecordInput(CreateDNSRecordInput{
		Type: " a ",
		Port: " 443 ",
	})
	if aRecord.Type != "A" {
		t.Fatalf("expected normalized A type, got %q", aRecord.Type)
	}
	if aRecord.Port != "" {
		t.Fatalf("expected A record port to be cleared, got %q", aRecord.Port)
	}

	srvRecord := normalizeUpdateDNSRecordInput(UpdateDNSRecordInput{
		Type: " srv ",
		Port: " 5060 ",
	})
	if srvRecord.Type != "SRV" {
		t.Fatalf("expected normalized SRV type, got %q", srvRecord.Type)
	}
	if srvRecord.Port != "5060" {
		t.Fatalf("expected SRV port 5060, got %q", srvRecord.Port)
	}
}
