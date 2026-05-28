package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

var httpClient = &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: 5 * time.Second}

const (
	smokeRegionID    = "smoke-region"
	smokeRelayNodeID = "relay-ticket-key-smoke"
)

type ticketKeyStatus struct {
	Source             string `json:"source,omitempty"`
	KeyRingID          string `json:"keyRingId,omitempty"`
	SigningConfigured  bool   `json:"signingConfigured"`
	KeyRingConfigured  bool   `json:"keyRingConfigured"`
	EffectiveKeyCount  int    `json:"effectiveKeyCount,omitempty"`
	RotationReady      bool   `json:"rotationReady"`
	AcceptsDevFallback bool   `json:"acceptsDevFallback,omitempty"`
}

type relayNodeList struct {
	Items []relayNode `json:"items"`
}

type relayNode struct {
	RegionID          string          `json:"regionId"`
	NodeID            string          `json:"nodeId"`
	Host              string          `json:"host"`
	UDPPort           int             `json:"udpPort"`
	AdminPort         int             `json:"adminPort,omitempty"`
	Enabled           bool            `json:"enabled"`
	Healthy           bool            `json:"healthy"`
	Stale             bool            `json:"stale"`
	Priority          int             `json:"priority"`
	TicketKeyRotation ticketKeyStatus `json:"ticketKeyRotation,omitempty"`
}

type smokeFailure struct {
	message string
}

func main() {
	defer exitOnFailure()

	bizURL := env("SLAN_BIZ_E2E_BIZ_URL", "http://127.0.0.1:28080")
	internalToken := env("SLAN_INTERNAL_WIRE_TOKEN", "change-me-wire-internal-token")
	relayBAdminURL := env("SLAN_BIZ_E2E_RELAY_B_ADMIN_URL", "http://127.0.0.1:29113")

	defer cleanupKnownSmokeNodes(bizURL, internalToken)
	cleanupKnownSmokeNodes(bizURL, internalToken)

	var correct ticketKeyStatus
	getJSON(relayBAdminURL+"/v1/ticket-key-status", "", &correct)
	if correct.KeyRingID == "" {
		fail("relay-b returned empty keyRingId: %+v", correct)
	}

	template := chooseRelayNode(bizURL, internalToken)
	node := relayNode{
		RegionID:          smokeRegionID,
		NodeID:            smokeRelayNodeID,
		Host:              template.Host,
		UDPPort:           template.UDPPort,
		AdminPort:         template.AdminPort,
		Enabled:           false,
		Healthy:           false,
		Priority:          10000,
		TicketKeyRotation: correct,
	}
	upsertRelayNode(bizURL, internalToken, node)
	expectRelayTicketKeyRotation(bizURL, internalToken, node.NodeID, correct)

	driftedNode := node
	driftedStatus := correct
	driftedStatus.KeyRingID = "mismatch-" + correct.KeyRingID
	driftedNode.TicketKeyRotation = driftedStatus
	upsertRelayNode(bizURL, internalToken, driftedNode)
	expectRelayTicketKeyRotation(bizURL, internalToken, node.NodeID, driftedStatus)

	upsertRelayNode(bizURL, internalToken, node)
	expectRelayTicketKeyRotation(bizURL, internalToken, node.NodeID, correct)
	disableRelayNode(bizURL, internalToken, node.RegionID, node.NodeID)
	deleteRelayNode(bizURL, internalToken, node.RegionID, node.NodeID)

	fmt.Println("wire biz ticket key metadata smoke passed")
}

func chooseRelayNode(bizURL, internalToken string) relayNode {
	var list relayNodeList
	getJSONWithHeaders(bizURL+"/internal/wire/admin/relay-nodes", map[string]string{"X-Slan-Internal-Token": internalToken}, &list)
	for _, node := range list.Items {
		if node.Enabled && node.Healthy && !node.Stale && strings.TrimSpace(node.Host) != "" && node.UDPPort > 0 {
			return node
		}
	}
	fail("no active relay node found: %+v", list.Items)
	return relayNode{}
}

func upsertRelayNode(bizURL, internalToken string, node relayNode) {
	payload := map[string]any{
		"regionId":          node.RegionID,
		"nodeId":            node.NodeID,
		"host":              node.Host,
		"udpPort":           node.UDPPort,
		"adminPort":         node.AdminPort,
		"enabled":           node.Enabled,
		"healthy":           node.Healthy,
		"priority":          node.Priority,
		"ticketKeyRotation": node.TicketKeyRotation,
	}
	putJSONWithHeaders(bizURL+"/internal/wire/admin/relay-nodes", map[string]string{"X-Slan-Internal-Token": internalToken}, payload, nil)
}

func cleanupKnownSmokeNodes(bizURL, internalToken string) {
	headers := map[string]string{"X-Slan-Internal-Token": internalToken}
	payload := map[string]any{"enabled": false, "healthy": false}
	patchJSONWithHeadersBestEffort(bizURL+"/internal/wire/admin/relay-nodes/smoke-region/relay-smoke/status", headers, payload, nil)
	patchJSONWithHeadersBestEffort(bizURL+"/internal/wire/admin/relay-nodes/"+smokeRegionID+"/"+smokeRelayNodeID+"/status", headers, payload, nil)
	patchJSONWithHeadersBestEffort(bizURL+"/internal/wire/admin/derp-nodes/smoke-region/derp-smoke/status", headers, payload, nil)
	deleteJSONWithHeadersBestEffort(bizURL+"/internal/wire/admin/relay-nodes/"+smokeRegionID+"/"+smokeRelayNodeID, headers)
}

func disableRelayNode(bizURL, internalToken, regionID, nodeID string) {
	headers := map[string]string{"X-Slan-Internal-Token": internalToken}
	payload := map[string]any{"enabled": false, "healthy": false}
	patchJSONWithHeaders(bizURL+"/internal/wire/admin/relay-nodes/"+regionID+"/"+nodeID+"/status", headers, payload, nil)
}

func deleteRelayNode(bizURL, internalToken, regionID, nodeID string) {
	headers := map[string]string{"X-Slan-Internal-Token": internalToken}
	deleteJSONWithHeaders(bizURL+"/internal/wire/admin/relay-nodes/"+regionID+"/"+nodeID, headers)
}

func expectRelayTicketKeyRotation(bizURL, internalToken, nodeID string, want ticketKeyStatus) {
	var list relayNodeList
	getJSONWithHeaders(bizURL+"/internal/wire/admin/relay-nodes", map[string]string{"X-Slan-Internal-Token": internalToken}, &list)
	for _, node := range list.Items {
		if node.NodeID != nodeID {
			continue
		}
		if node.TicketKeyRotation.KeyRingID != want.KeyRingID ||
			node.TicketKeyRotation.SigningConfigured != want.SigningConfigured ||
			node.TicketKeyRotation.KeyRingConfigured != want.KeyRingConfigured ||
			node.TicketKeyRotation.RotationReady != want.RotationReady {
			fail("relay node ticket key metadata mismatch: got=%+v want=%+v", node.TicketKeyRotation, want)
		}
		return
	}
	fail("relay node %s not found in internal view: %+v", nodeID, list.Items)
}

func getJSON(url, token string, out any) {
	headers := map[string]string{}
	if token != "" {
		headers["Authorization"] = "Bearer " + token
	}
	getJSONWithHeaders(url, headers, out)
}

func getJSONWithHeaders(url string, headers map[string]string, out any) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	must(err)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := httpClient.Do(req)
	must(err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		fail("GET %s status=%d body=%s", url, resp.StatusCode, string(body))
	}
	must(json.Unmarshal(body, out))
}

func patchJSONWithHeaders(url string, headers map[string]string, in, out any) {
	payload, err := json.Marshal(in)
	must(err)
	req, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(payload))
	must(err)
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return
	}
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		fail("PATCH %s status=%d body=%s", url, resp.StatusCode, string(body))
	}
	if out != nil {
		must(json.Unmarshal(body, out))
	}
}

func patchJSONWithHeadersBestEffort(url string, headers map[string]string, in, out any) {
	defer func() {
		_ = recover()
	}()
	patchJSONWithHeaders(url, headers, in, out)
}

func putJSONWithHeaders(url string, headers map[string]string, in, out any) {
	payload, err := json.Marshal(in)
	must(err)
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(payload))
	must(err)
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := httpClient.Do(req)
	must(err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		fail("PUT %s status=%d body=%s", url, resp.StatusCode, string(body))
	}
	if out != nil {
		must(json.Unmarshal(body, out))
	}
}

func deleteJSONWithHeadersBestEffort(url string, headers map[string]string) {
	defer func() {
		_ = recover()
	}()
	deleteJSONWithHeaders(url, headers)
}

func deleteJSONWithHeaders(url string, headers map[string]string) {
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	must(err)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := httpClient.Do(req)
	must(err)
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return
	}
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		fail("DELETE %s status=%d body=%s", url, resp.StatusCode, string(body))
	}
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
	panic(smokeFailure{message: fmt.Sprintf(format, args...)})
}

func exitOnFailure() {
	recovered := recover()
	if recovered == nil {
		return
	}
	if failure, ok := recovered.(smokeFailure); ok {
		fmt.Fprintln(os.Stderr, failure.message)
		os.Exit(1)
	}
	panic(recovered)
}
