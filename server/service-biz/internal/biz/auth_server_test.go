package biz

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestConsoleLoginHTTPFlowConsumesServerIssuedKeyOnce(t *testing.T) {
	server := NewServer()
	auth, _, err := server.store.RegisterUser("console-http@example.com", "secret", "Console HTTP")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	if _, _, err := server.store.RegisterDevice(auth.User.UserID, "mac-http-1", "Mac", "macos", "macOS", "14", "", "pk_console_http_1"); err != nil {
		t.Fatalf("register device: %v", err)
	}
	handler := server.Routes()

	var keyResponse struct {
		LoginKey string `json:"loginKey"`
		UserID   string `json:"userId"`
		DeviceID string `json:"deviceId"`
		Status   string `json:"status"`
	}
	postJSON(t, handler, "/api/auth/console-login-keys", "Bearer "+auth.Session.Token, map[string]any{
		"deviceId": "mac-http-1",
	}, http.StatusCreated, &keyResponse)
	if keyResponse.LoginKey == "" || keyResponse.UserID != auth.User.UserID || keyResponse.DeviceID != "mac-http-1" || keyResponse.Status != "unused" {
		t.Fatalf("unexpected console login key response: %+v", keyResponse)
	}

	var loginResponse struct {
		Auth AuthResponse `json:"auth"`
	}
	postJSON(t, handler, "/api/auth/console-login", "", map[string]any{
		"loginKey": keyResponse.LoginKey,
	}, http.StatusOK, &loginResponse)
	if loginResponse.Auth.User.UserID != auth.User.UserID || loginResponse.Auth.Session.Token == "" {
		t.Fatalf("unexpected console login response: %+v", loginResponse)
	}

	postJSON(t, handler, "/api/auth/console-login", "", map[string]any{
		"loginKey": keyResponse.LoginKey,
	}, http.StatusNotFound, nil)

	events := server.store.ListAuditEvents()
	var sawCreate, sawConsume, sawConsumeFailed bool
	for _, event := range events {
		switch {
		case event.Action == "console_login_key.create" && event.Status == "succeeded" && event.ResourceID == "mac-http-1":
			sawCreate = true
		case event.Action == "console_login_key.consume" && event.Status == "succeeded" && event.ActorID == auth.User.UserID:
			sawConsume = true
		case event.Action == "console_login_key.consume" && event.Status == "failed":
			sawConsumeFailed = true
		}
		for _, value := range event.Details {
			if value == keyResponse.LoginKey || value == auth.Session.Token {
				t.Fatalf("audit event leaked sensitive value: %+v", event)
			}
		}
	}
	if !sawCreate || !sawConsume || !sawConsumeFailed {
		t.Fatalf("expected console login audit events, got %+v", events)
	}
}

func TestConsoleLoginHTTPRejectsInvalidCredentials(t *testing.T) {
	server := NewServer()
	handler := server.Routes()

	postJSON(t, handler, "/api/auth/console-login-keys", "Bearer bad-token", map[string]any{
		"deviceId": "mac-http-1",
	}, http.StatusNotFound, nil)
	auth, _, err := server.store.RegisterUser("console-invalid-http@example.com", "secret", "Console Invalid")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	postJSON(t, handler, "/api/auth/console-login-keys", "Bearer "+auth.Session.Token, map[string]any{
		"deviceId": "missing-device",
	}, http.StatusNotFound, nil)
	postJSON(t, handler, "/api/auth/console-login-keys", "Bearer "+auth.Session.Token, map[string]any{
		"deviceId": "",
	}, http.StatusBadRequest, nil)
	postJSON(t, handler, "/api/auth/console-login", "", map[string]any{
		"loginKey": "not-a-real-key",
	}, http.StatusNotFound, nil)
}

func TestNetworkConfigHTTPMutationsWriteAuditEvents(t *testing.T) {
	server := NewServer()
	auth, network, err := server.store.RegisterUser("audit-net@example.com", "secret", "Audit Net")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	device, _, err := server.store.RegisterDevice(auth.User.UserID, "audit-device-1", "Mac", "macos", "macOS", "15.0", "", "pub")
	if err != nil {
		t.Fatalf("register device: %v", err)
	}
	handler := server.Routes()
	zones := server.store.ListDNSZones(network.NetworkID)
	if len(zones) == 0 {
		t.Fatalf("expected default dns zone")
	}
	var record NetworkDNSRecord
	postJSON(t, handler, "/api/networks/"+network.NetworkID+"/dns/records", "", map[string]any{
		"zoneId":         zones[0].ZoneID,
		"name":           "audit",
		"recordType":     "A",
		"targetDeviceId": device.DeviceID,
		"ttl":            60,
	}, http.StatusCreated, &record)

	groups := server.store.ListSecurityGroups(network.NetworkID)
	if len(groups) == 0 {
		t.Fatalf("expected default security group")
	}
	var rule SecurityGroupRule
	postJSON(t, handler, "/api/security-groups/"+groups[0].SecurityGroupID+"/rules", "", map[string]any{
		"direction": "ingress",
		"action":    "allow",
		"protocol":  "tcp",
		"peerType":  "device",
		"peerValue": device.DeviceID,
		"priority":  100,
		"portFrom":  22,
		"portTo":    22,
	}, http.StatusCreated, &rule)

	events := server.store.ListAuditEvents()
	if !hasAuditEvent(events, "dns_record.add", "succeeded", record.RecordID) {
		t.Fatalf("expected dns_record.add audit event, got %+v", events)
	}
	if !hasAuditEvent(events, "security_rule.add", "succeeded", rule.RuleID) {
		t.Fatalf("expected security_rule.add audit event, got %+v", events)
	}
}

func TestOpsMutationsWriteOperatorAuditEvents(t *testing.T) {
	server := NewServer()
	auth, _, err := server.store.RegisterUser("ops-audit@example.com", "secret", "Ops Audit")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	device, _, err := server.store.RegisterDevice(auth.User.UserID, "ops-audit-device", "Mac", "macos", "macOS", "15.0", "", "pub")
	if err != nil {
		t.Fatalf("register device: %v", err)
	}
	operatorAuth, err := server.store.LoginOperator("admin1", "admin1")
	if err != nil {
		t.Fatalf("login operator: %v", err)
	}
	handler := server.Routes()
	authorization := "Bearer " + operatorAuth.Session.Token
	requestJSON(t, handler, http.MethodPatch, "/api/ops/customers/"+auth.User.UserID, authorization, map[string]any{
		"email":  auth.User.Email,
		"name":   auth.User.Name,
		"status": "disabled",
	}, http.StatusOK, nil)
	disabled := false
	requestJSON(t, handler, http.MethodPatch, "/api/ops/devices/"+device.DeviceID, authorization, map[string]any{
		"enabled": disabled,
	}, http.StatusOK, nil)

	events := server.store.ListAuditEvents()
	if !hasAuditEvent(events, "ops.customer.update", "succeeded", auth.User.UserID) {
		t.Fatalf("expected ops.customer.update audit event, got %+v", events)
	}
	if !hasAuditEvent(events, "ops.device.update", "succeeded", device.DeviceID) {
		t.Fatalf("expected ops.device.update audit event, got %+v", events)
	}
	for _, event := range events {
		if strings.HasPrefix(event.Action, "ops.") && event.ActorID != operatorAuth.Operator.OperatorID {
			t.Fatalf("expected ops audit actor to be operator, got %+v", event)
		}
	}
}

func TestOpsPunchNodesCanBeManaged(t *testing.T) {
	server := NewServer()
	operatorAuth, err := server.store.LoginOperator("admin1", "admin1")
	if err != nil {
		t.Fatalf("login operator: %v", err)
	}
	handler := server.Routes()
	authorization := "Bearer " + operatorAuth.Session.Token
	var node OpsPunchNode
	requestJSON(t, handler, http.MethodPost, "/api/ops/punch-nodes", authorization, map[string]any{
		"name":          "Punch A",
		"region":        "cn-east",
		"publicUdpIp":   "10.10.0.21",
		"publicUdpPort": 29130,
		"maxSessions":   1000,
		"status":        "active",
		"health":        "healthy",
		"priority":      20,
	}, http.StatusCreated, &node)
	if node.NodeID == "" || node.PublicUDPIP != "10.10.0.21" || node.PublicUDPPort != 29130 {
		t.Fatalf("unexpected punch node: %+v", node)
	}
	requestJSON(t, handler, http.MethodPatch, "/api/ops/punch-nodes/"+node.NodeID, authorization, map[string]any{
		"name":          "Punch A Updated",
		"region":        "cn-east",
		"publicUdpIp":   "10.10.0.21",
		"publicUdpPort": 29130,
		"status":        "disabled",
		"health":        "healthy",
		"priority":      30,
	}, http.StatusOK, &node)
	if node.Status != "disabled" || len(server.store.ActivePunchNodes()) != 0 {
		t.Fatalf("expected disabled punch node to be inactive, node=%+v active=%+v", node, server.store.ActivePunchNodes())
	}
	var response struct {
		Items []OpsPunchNode `json:"items"`
	}
	requestJSON(t, handler, http.MethodGet, "/api/ops/punch-nodes", authorization, map[string]any{}, http.StatusOK, &response)
	if len(response.Items) == 0 || response.Items[0].NodeID != node.NodeID {
		t.Fatalf("expected punch node in list: %+v", response.Items)
	}
	requestJSON(t, handler, http.MethodDelete, "/api/ops/punch-nodes/"+node.NodeID, authorization, nil, http.StatusNoContent, nil)
	requestJSON(t, handler, http.MethodDelete, "/api/ops/punch-nodes/"+node.NodeID, authorization, nil, http.StatusNotFound, nil)
}

func TestOpsNodeStatusEndpointsToggleRelayDerpAndPunchNodes(t *testing.T) {
	server := NewServer()
	operatorAuth, err := server.store.LoginOperator("admin1", "admin1")
	if err != nil {
		t.Fatalf("login operator: %v", err)
	}
	handler := server.Routes()
	authorization := "Bearer " + operatorAuth.Session.Token
	store := server.store.(*Store)
	baseActiveRelayNodes := countHealthyActiveRelayNodes(store.ListRelayNodes())
	baseActivePunchNodes := len(store.ActivePunchNodes())

	var relay OpsRelayNode
	requestJSON(t, handler, http.MethodPost, "/api/ops/relay-nodes", authorization, map[string]any{
		"name":             "Relay UDP A",
		"region":           "cn-east",
		"transport":        "relay_udp",
		"publicAddr":       "udp://relay.example.com:29110",
		"maxBandwidthMbps": 1000,
		"monthlyTrafficGb": 100,
		"maxSessions":      1000,
		"status":           "active",
		"health":           "healthy",
	}, http.StatusCreated, &relay)
	requestJSON(t, handler, http.MethodPatch, "/api/ops/relay-nodes/"+relay.NodeID+"/status", authorization, map[string]any{
		"enabled": false,
	}, http.StatusOK, &relay)
	if relay.Status != "disabled" || relay.Health != "down" || countHealthyActiveRelayNodes(store.ListRelayNodes()) != baseActiveRelayNodes {
		t.Fatalf("expected disabled relay node to be inactive, node=%+v relays=%+v", relay, store.ListRelayNodes())
	}
	requestJSON(t, handler, http.MethodPatch, "/api/ops/relay-nodes/"+relay.NodeID+"/status", authorization, map[string]any{
		"enabled": true,
	}, http.StatusOK, &relay)
	if relay.Status != "active" || relay.Health != "healthy" || countHealthyActiveRelayNodes(store.ListRelayNodes()) != baseActiveRelayNodes+1 {
		t.Fatalf("expected enabled relay node to be active, node=%+v relays=%+v", relay, store.ListRelayNodes())
	}
	requestJSON(t, handler, http.MethodDelete, "/api/ops/relay-nodes/"+relay.NodeID, authorization, nil, http.StatusNoContent, nil)
	if countHealthyActiveRelayNodes(store.ListRelayNodes()) != baseActiveRelayNodes {
		t.Fatalf("expected relay node delete to remove node, relays=%+v", store.ListRelayNodes())
	}

	var derp OpsRelayNode
	requestJSON(t, handler, http.MethodPost, "/api/ops/relay-nodes", authorization, map[string]any{
		"name":             "DERP TCP A",
		"region":           "cn-east",
		"transport":        "derp_tcp_tls_443",
		"publicAddr":       "derp://derp.example.com:29120",
		"maxBandwidthMbps": 1000,
		"monthlyTrafficGb": 100,
		"maxSessions":      1000,
		"status":           "active",
		"health":           "healthy",
	}, http.StatusCreated, &derp)
	requestJSON(t, handler, http.MethodPatch, "/api/ops/relay-nodes/"+derp.NodeID+"/status", authorization, map[string]any{
		"enabled": false,
	}, http.StatusOK, &derp)
	if derp.Status != "disabled" || derp.Health != "down" {
		t.Fatalf("expected disabled DERP node, got %+v", derp)
	}
	requestJSON(t, handler, http.MethodDelete, "/api/ops/relay-nodes/"+derp.NodeID, authorization, nil, http.StatusNoContent, nil)

	var punch OpsPunchNode
	requestJSON(t, handler, http.MethodPost, "/api/ops/punch-nodes", authorization, map[string]any{
		"name":          "Punch B",
		"region":        "cn-east",
		"publicUdpIp":   "10.10.0.22",
		"publicUdpPort": 29131,
		"maxSessions":   1000,
		"status":        "active",
		"health":        "healthy",
		"priority":      20,
	}, http.StatusCreated, &punch)
	requestJSON(t, handler, http.MethodPatch, "/api/ops/punch-nodes/"+punch.NodeID+"/status", authorization, map[string]any{
		"enabled": false,
	}, http.StatusOK, &punch)
	if punch.Status != "disabled" || punch.Health != "down" || len(store.ActivePunchNodes()) != baseActivePunchNodes {
		t.Fatalf("expected disabled punch node to be inactive, node=%+v active=%+v", punch, store.ActivePunchNodes())
	}
	requestJSON(t, handler, http.MethodPatch, "/api/ops/punch-nodes/"+punch.NodeID+"/status", authorization, map[string]any{}, http.StatusBadRequest, nil)
}

func countHealthyActiveRelayNodes(nodes []OpsRelayNode) int {
	count := 0
	for _, node := range nodes {
		if node.Status == "active" && node.Health == "healthy" {
			count++
		}
	}
	return count
}

func TestOpsAuditEventsQueryFiltersResults(t *testing.T) {
	server := NewServer()
	operatorAuth, err := server.store.LoginOperator("admin1", "admin1")
	if err != nil {
		t.Fatalf("login operator: %v", err)
	}
	server.store.RecordAuditEvent(AuditEvent{ActorType: "user", ActorID: "user-a", Action: "dns_record.add", ResourceType: "dns_record", ResourceID: "record-a", Status: "succeeded"})
	server.store.RecordAuditEvent(AuditEvent{ActorType: "user", ActorID: "user-a", Action: "dns_record.add", ResourceType: "dns_record", ResourceID: "record-b", Status: "failed"})
	server.store.RecordAuditEvent(AuditEvent{ActorType: "operator", ActorID: operatorAuth.Operator.OperatorID, Action: "ops.device.update", ResourceType: "device", ResourceID: "device-a", Status: "succeeded"})

	handler := server.Routes()
	req := httptest.NewRequest(http.MethodGet, "/api/ops/audit-events?action=dns_record.add&status=succeeded&limit=10", nil)
	req.Header.Set("Authorization", "Bearer "+operatorAuth.Session.Token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET audit events status=%d body=%s", rec.Code, rec.Body.String())
	}
	var response struct {
		Items []AuditEvent `json:"items"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode audit response: %v", err)
	}
	if len(response.Items) != 1 || response.Items[0].ResourceID != "record-a" {
		t.Fatalf("expected filtered audit event record-a, got %+v", response.Items)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/ops/audit-events", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized audit query, got status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func hasAuditEvent(events []AuditEvent, action, status, resourceID string) bool {
	for _, event := range events {
		if event.Action == action && event.Status == status && event.ResourceID == resourceID {
			return true
		}
	}
	return false
}

func postJSON(t *testing.T, handler http.Handler, path, authorization string, body any, want int, out any) {
	t.Helper()
	requestJSON(t, handler, http.MethodPost, path, authorization, body, want, out)
}

func requestJSON(t *testing.T, handler http.Handler, method, path, authorization string, body any, want int, out any) {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != want {
		t.Fatalf("%s %s status=%d want=%d body=%s", method, path, rec.Code, want, rec.Body.String())
	}
	if out != nil {
		if err := json.NewDecoder(rec.Body).Decode(out); err != nil {
			t.Fatalf("decode response: %v body=%s", err, rec.Body.String())
		}
	}
}
