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

type wireNodesOpsView struct {
	TicketKeyHealth ticketKeyHealth `json:"ticketKeyHealth"`
}

type ticketKeyHealth struct {
	BaselineKeyRingID string              `json:"baselineKeyRingId"`
	Drifted           bool                `json:"drifted"`
	UnavailableCount  int                 `json:"unavailableCount"`
	Instances         []ticketKeyInstance `json:"instances"`
}

type ticketKeyInstance struct {
	Kind     string          `json:"kind"`
	RegionID string          `json:"regionId,omitempty"`
	NodeID   string          `json:"nodeId,omitempty"`
	Status   ticketKeyStatus `json:"status"`
	Drifted  bool            `json:"drifted"`
}

func main() {
	bizURL := env("SLAN_BIZ_E2E_BIZ_URL", "http://127.0.0.1:28080")
	opsURL := env("SLAN_BIZ_E2E_OPS_URL", "http://127.0.0.1:28081")
	opsToken := env("SLAN_BIZ_E2E_OPS_TOKEN", "change-me-ops-token")
	internalToken := env("SLAN_INTERNAL_WIRE_TOKEN", "change-me-wire-internal-token")
	relayBAdminURL := env("SLAN_BIZ_E2E_RELAY_B_ADMIN_URL", "http://127.0.0.1:29113")

	disableKnownSmokeNodes(bizURL, internalToken)

	var correct ticketKeyStatus
	getJSON(relayBAdminURL+"/v1/ticket-key-status", "", &correct)
	if correct.KeyRingID == "" {
		fail("relay-b returned empty keyRingId: %+v", correct)
	}

	node := chooseRelayNode(bizURL, internalToken)
	node.TicketKeyRotation = correct
	upsertRelayNode(bizURL, internalToken, node)
	expectBizDrift(opsURL, opsToken, false, node.NodeID)

	driftedNode := node
	driftedStatus := correct
	driftedStatus.KeyRingID = "mismatch-" + correct.KeyRingID
	driftedNode.TicketKeyRotation = driftedStatus
	upsertRelayNode(bizURL, internalToken, driftedNode)
	expectBizDrift(opsURL, opsToken, true, node.NodeID)

	upsertRelayNode(bizURL, internalToken, node)
	expectBizDrift(opsURL, opsToken, false, node.NodeID)

	fmt.Println("wire biz ticket key drift smoke passed")
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

func disableKnownSmokeNodes(bizURL, internalToken string) {
	headers := map[string]string{"X-Slan-Internal-Token": internalToken}
	payload := map[string]any{"enabled": false, "healthy": false}
	patchJSONWithHeaders(bizURL+"/internal/wire/admin/relay-nodes/smoke-region/relay-smoke/status", headers, payload, nil)
	patchJSONWithHeaders(bizURL+"/internal/wire/admin/derp-nodes/smoke-region/derp-smoke/status", headers, payload, nil)
}

func expectBizDrift(opsURL, opsToken string, wantDrift bool, nodeID string) {
	var view wireNodesOpsView
	getJSON(opsURL+"/wire-nodes", opsToken, &view)
	health := view.TicketKeyHealth
	if health.UnavailableCount != 0 {
		fail("expected no unavailable ticket key instances: %+v", health)
	}
	if health.Drifted != wantDrift {
		fail("ticket key drift=%v want=%v health=%+v", health.Drifted, wantDrift, health)
	}
	if wantDrift && !hasDriftedNode(health.Instances, nodeID) {
		fail("expected node %s to be marked drifted: %+v", nodeID, health.Instances)
	}
	if !wantDrift && hasDriftedNode(health.Instances, nodeID) {
		fail("expected node %s drift to be cleared: %+v", nodeID, health.Instances)
	}
}

func hasDriftedNode(instances []ticketKeyInstance, nodeID string) bool {
	for _, instance := range instances {
		if instance.NodeID == nodeID && instance.Drifted {
			return true
		}
	}
	return false
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
