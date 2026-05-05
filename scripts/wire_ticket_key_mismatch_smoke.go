package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"time"
)

var httpClient = &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: 5 * time.Second}

type ticketKeyStatus struct {
	Source             string `json:"source"`
	KeyRingID          string `json:"keyRingId"`
	SigningConfigured  bool   `json:"signingConfigured"`
	KeyRingConfigured  bool   `json:"keyRingConfigured"`
	EffectiveKeyCount  int    `json:"effectiveKeyCount"`
	RotationReady      bool   `json:"rotationReady"`
	AcceptsDevFallback bool   `json:"acceptsDevFallback"`
}

func main() {
	wireURL := env("SLAN_BIZ_E2E_WIRE_URL", "http://127.0.0.1:29100")
	wireBURL := env("SLAN_BIZ_E2E_WIRE_B_URL", "http://127.0.0.1:29101")
	relayAdminURL := env("SLAN_BIZ_E2E_RELAY_ADMIN_URL", "http://127.0.0.1:29111")
	relayBAdminURL := env("SLAN_BIZ_E2E_RELAY_B_ADMIN_URL", "http://127.0.0.1:29113")
	derpAdminURL := env("SLAN_BIZ_E2E_DERP_ADMIN_URL", "http://127.0.0.1:29121")
	derpBAdminURL := env("SLAN_BIZ_E2E_DERP_B_ADMIN_URL", "http://127.0.0.1:29123")

	endpoints := map[string]string{
		"wire":    wireURL + "/internal/wire/ticket-key-status",
		"wire-b":  wireBURL + "/internal/wire/ticket-key-status",
		"relay":   relayAdminURL + "/v1/ticket-key-status",
		"relay-b": relayBAdminURL + "/v1/ticket-key-status",
		"derp":    derpAdminURL + "/v1/ticket-key-status",
		"derp-b":  derpBAdminURL + "/v1/ticket-key-status",
	}
	if err := expectConsistentTicketKeyStatus(endpoints); err != nil {
		fail("expected real ticket key statuses to be consistent: %v", err)
	}

	var baseline ticketKeyStatus
	getJSON(wireURL+"/internal/wire/ticket-key-status", &baseline)
	if baseline.KeyRingID == "" {
		fail("wire returned empty keyRingId: %+v", baseline)
	}
	mismatched := baseline
	mismatched.KeyRingID = "mismatch-" + baseline.KeyRingID
	mismatchServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(mismatched)
	}))
	defer mismatchServer.Close()

	endpoints["mismatched-relay"] = mismatchServer.URL
	if err := expectConsistentTicketKeyStatus(endpoints); err == nil {
		fail("expected mismatched keyRingId to fail consistency check")
	}
	fmt.Println("wire ticket key mismatch smoke passed")
}

func expectConsistentTicketKeyStatus(endpoints map[string]string) error {
	var baselineName string
	var baseline ticketKeyStatus
	for name, url := range endpoints {
		var current ticketKeyStatus
		getJSON(url, &current)
		if current.EffectiveKeyCount <= 0 || current.KeyRingID == "" {
			return fmt.Errorf("%s returned invalid ticket key status: %+v", name, current)
		}
		if baselineName == "" {
			baselineName = name
			baseline = current
			continue
		}
		if !sameTicketKeyStatus(baseline, current) {
			return fmt.Errorf("ticket key status mismatch: %s=%+v %s=%+v", baselineName, baseline, name, current)
		}
	}
	return nil
}

func sameTicketKeyStatus(a, b ticketKeyStatus) bool {
	return a.Source == b.Source &&
		a.KeyRingID == b.KeyRingID &&
		a.SigningConfigured == b.SigningConfigured &&
		a.KeyRingConfigured == b.KeyRingConfigured &&
		a.EffectiveKeyCount == b.EffectiveKeyCount &&
		a.RotationReady == b.RotationReady &&
		a.AcceptsDevFallback == b.AcceptsDevFallback
}

func getJSON(url string, out any) {
	resp, err := httpClient.Get(url)
	must(err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		fail("GET %s status=%d body=%s", url, resp.StatusCode, string(body))
	}
	must(json.Unmarshal(body, out))
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func must(err error) {
	if err != nil {
		fail("%v", err)
	}
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
