package biz

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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

func TestMQTTIsReturnedAfterDeviceLoginPrepareAndRegistration(t *testing.T) {
	server := NewServer()
	server.mqtt = MQTTConfig{
		Enabled:              true,
		PublicBrokerURL:      "mqtt://47.245.40.231:1883",
		UsernamePrefix:       "slan",
		PasswordSecret:       "test-secret",
		TopicPrefix:          "slan",
		CredentialTTLSeconds: 3600,
	}
	handler := server.Routes()

	auth, _, err := server.store.RegisterUser("mqtt-register-http@example.com", "secret", "MQTT Register")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}

	var prepare map[string]json.RawMessage
	postJSON(t, handler, "/api/auth/device-login-devices", "", map[string]any{
		"deviceId":  "mqtt-prepare-1",
		"name":      "Mac",
		"platform":  "macos",
		"osName":    "macOS",
		"osVersion": "15.0",
		"publicKey": "pk_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}, http.StatusCreated, &prepare)
	var preparedMQTT MQTTCredential
	if err := json.Unmarshal(prepare["mqtt"], &preparedMQTT); err != nil {
		t.Fatalf("expected prepare login response to include mqtt credential: %v", err)
	}
	if preparedMQTT.BrokerURL != "mqtt://47.245.40.231:1883" {
		t.Fatalf("unexpected prepare mqtt credential: %+v", preparedMQTT)
	}
	requestJSON(t, handler, http.MethodGet, "/api/devices/mqtt-prepare-1/mqtt-credential", "", nil, http.StatusUnauthorized, nil)

	var registered struct {
		Device Device          `json:"device"`
		MQTT   *MQTTCredential `json:"mqtt"`
	}
	postJSON(t, handler, "/api/devices/register", "Bearer "+auth.Session.Token, map[string]any{
		"userId":    auth.User.UserID,
		"deviceId":  "mqtt-register-1",
		"name":      "Android",
		"platform":  "android",
		"osName":    "Android",
		"osVersion": "15",
		"publicKey": "pk_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}, http.StatusCreated, &registered)
	if registered.Device.DeviceID != "mqtt-register-1" {
		t.Fatalf("unexpected registered device: %+v", registered.Device)
	}
	if registered.MQTT == nil || registered.MQTT.BrokerURL != "mqtt://47.245.40.231:1883" {
		t.Fatalf("expected mqtt credential after device registration, got %+v", registered.MQTT)
	}
}

func TestPrepareDeviceLoginTruncatesBrowserIdentityFields(t *testing.T) {
	server := NewServer()
	handler := server.Routes()

	postJSON(t, handler, "/api/auth/device-login-devices", "", map[string]any{
		"deviceId":  "browser-long-identity",
		"name":      strings.Repeat("n", 180),
		"platform":  strings.Repeat("p", 80),
		"osName":    strings.Repeat("o", 90),
		"osVersion": strings.Repeat("u", 180),
		"alias":     strings.Repeat("a", 180),
		"publicKey": "pk_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}, http.StatusCreated, nil)

	var device Device
	for _, item := range server.store.ListDevices("") {
		if item.DeviceID == "browser-long-identity" {
			device = item
			break
		}
	}
	if device.DeviceID == "" {
		t.Fatal("expected prepared device to be stored")
	}
	if len(device.Name) != deviceNameMaxLength || len(device.Platform) != devicePlatformMaxLength || len(device.OSName) != deviceOSNameMaxLength || len(device.OSVersion) != deviceOSVersionMaxLength || len(device.Alias) != deviceAliasMaxLength {
		t.Fatalf("expected identity fields to be truncated, got name=%d platform=%d osName=%d osVersion=%d alias=%d", len(device.Name), len(device.Platform), len(device.OSName), len(device.OSVersion), len(device.Alias))
	}
}

func TestDeviceLoginHTTPAllowsSameUserMultipleDevices(t *testing.T) {
	server := NewServer()
	server.mqtt = MQTTConfig{
		Enabled:                    true,
		BrokerURL:                  "mqtt://127.0.0.1:1883",
		PublicBrokerURL:            "mqtt://127.0.0.1:1883",
		UsernamePrefix:             "slan",
		PasswordSecret:             "test-secret",
		TopicPrefix:                "slan",
		CredentialTTLSeconds:       3600,
		ControlMessageTTLSeconds:   3600,
		PublishTimeoutMilliseconds: 1,
	}
	auth, _, err := server.store.RegisterUser("multi-device-http@example.com", "secret", "Multi Device")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	handler := server.Routes()
	devices := []struct {
		deviceID  string
		name      string
		platform  string
		publicKey string
	}{
		{deviceID: "multi-http-mac", name: "Mac", platform: "macos", publicKey: "pk_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		{deviceID: "multi-http-android", name: "Android", platform: "android", publicKey: "pk_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
	}

	for _, item := range devices {
		postJSON(t, handler, "/api/auth/device-login-devices", "", map[string]any{
			"deviceId":  item.deviceID,
			"name":      item.name,
			"platform":  item.platform,
			"osName":    item.platform,
			"osVersion": "1.0",
			"publicKey": item.publicKey,
		}, http.StatusCreated, nil)
		var complete CompleteDeviceLoginResponse
		postJSON(t, handler, "/api/auth/device-login-devices/"+item.deviceID+"/complete", "", map[string]any{
			"accessToken": auth.Session.Token,
			"action":      "login",
		}, http.StatusOK, &complete)
		if complete.DeviceID != item.deviceID || complete.Status != "ok" {
			t.Fatalf("unexpected complete response for %s: %+v", item.deviceID, complete)
		}
	}

	for _, item := range devices {
		device, err := server.store.GetDevice(item.deviceID)
		if err != nil {
			t.Fatalf("get device %s: %v", item.deviceID, err)
		}
		if device.OwnerID != auth.User.UserID {
			t.Fatalf("expected %s to bind to %s, got %+v", item.deviceID, auth.User.UserID, device)
		}
	}
}

func TestDeviceLoginHTTPRebindsDeviceToBrowserUser(t *testing.T) {
	server := NewServer()
	server.mqtt = MQTTConfig{
		Enabled:                    true,
		BrokerURL:                  "mqtt://127.0.0.1:1883",
		PublicBrokerURL:            "mqtt://127.0.0.1:1883",
		UsernamePrefix:             "slan",
		PasswordSecret:             "test-secret",
		TopicPrefix:                "slan",
		CredentialTTLSeconds:       3600,
		ControlMessageTTLSeconds:   3600,
		PublishTimeoutMilliseconds: 1,
	}
	alice, _, err := server.store.RegisterUser("rebind-alice@example.com", "secret", "Alice")
	if err != nil {
		t.Fatalf("register alice: %v", err)
	}
	bob, _, err := server.store.RegisterUser("rebind-bob@example.com", "secret", "Bob")
	if err != nil {
		t.Fatalf("register bob: %v", err)
	}
	if _, _, _, err := server.store.BindDeviceSession(alice.Session.Token, "rebind-http-mac", "Mac", "macos", "macOS", "15.0", "", "pk_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"); err != nil {
		t.Fatalf("bind alice device: %v", err)
	}
	handler := server.Routes()

	var complete CompleteDeviceLoginResponse
	postJSON(t, handler, "/api/auth/device-login-devices/rebind-http-mac/complete", "", map[string]any{
		"accessToken": bob.Session.Token,
		"action":      "login",
	}, http.StatusOK, &complete)
	if complete.Status != "ok" || complete.DeviceID != "rebind-http-mac" {
		t.Fatalf("unexpected complete response: %+v", complete)
	}
	device, err := server.store.GetDevice("rebind-http-mac")
	if err != nil {
		t.Fatalf("get device: %v", err)
	}
	if device.OwnerID != bob.User.UserID {
		t.Fatalf("expected device to rebind to bob, got %+v", device)
	}
}

func TestLegacyDeviceRegisterAndRenewRequireBearerIdentity(t *testing.T) {
	server := NewServer()
	alice, _, err := server.store.RegisterUser("legacy-alice@example.com", "secret", "Alice")
	if err != nil {
		t.Fatalf("register alice: %v", err)
	}
	bob, _, err := server.store.RegisterUser("legacy-bob@example.com", "secret", "Bob")
	if err != nil {
		t.Fatalf("register bob: %v", err)
	}
	handler := server.Routes()

	registerBody := map[string]any{
		"userId":    alice.User.UserID,
		"deviceId":  "legacy-secure-device",
		"name":      "Mac",
		"platform":  "macos",
		"osName":    "macOS",
		"osVersion": "15.0",
		"publicKey": "pk_cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
	}
	requestJSON(t, handler, http.MethodPost, "/api/devices/register", "", registerBody, http.StatusUnauthorized, nil)

	var registered struct {
		Device Device `json:"device"`
	}
	postJSON(t, handler, "/api/devices/register", "Bearer "+bob.Session.Token, registerBody, http.StatusCreated, &registered)
	if registered.Device.OwnerID != bob.User.UserID {
		t.Fatalf("expected bearer user to own device, got %+v", registered.Device)
	}

	renewBody := map[string]any{
		"userId":         alice.User.UserID,
		"networkEnabled": true,
		"rxBytesTotal":   7,
		"txBytesTotal":   9,
	}
	requestJSON(t, handler, http.MethodPost, "/api/devices/"+registered.Device.DeviceID+"/renew", "", renewBody, http.StatusUnauthorized, nil)
	requestJSON(t, handler, http.MethodPost, "/api/devices/"+registered.Device.DeviceID+"/renew", "Bearer "+alice.Session.Token, renewBody, http.StatusNotFound, nil)

	var renewed struct {
		Device Device `json:"device"`
	}
	postJSON(t, handler, "/api/devices/"+registered.Device.DeviceID+"/renew", "Bearer "+bob.Session.Token, renewBody, http.StatusOK, &renewed)
	store := server.store.(*Store)
	status := store.runtimeStatuses[registered.Device.DeviceID]
	if renewed.Device.OwnerID != bob.User.UserID || !status.NetworkEnabled || status.RxBytesTotal != 7 || status.TxBytesTotal != 9 {
		t.Fatalf("expected renew to use bearer user and refresh runtime, device=%+v status=%+v", renewed.Device, status)
	}
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
	var zone NetworkDNSZone
	postJSON(t, handler, "/api/networks/"+network.NetworkID+"/dns/zones", "", map[string]any{
		"zoneName": "audit.lan",
	}, http.StatusCreated, &zone)
	var record NetworkDNSRecord
	postJSON(t, handler, "/api/networks/"+network.NetworkID+"/dns/records", "", map[string]any{
		"zoneId":         zone.ZoneID,
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

func TestSecurityGroupMutationsPushNetworkConfigChangedToClients(t *testing.T) {
	server := NewServer()
	store := server.store.(*Store)
	auth, network, err := server.store.RegisterUser("security-push@example.com", "secret", "Security Push")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	device, _, err := server.store.RegisterDevice(auth.User.UserID, "security-push-device", "Mac", "macos", "macOS", "15.0", "", "pub")
	if err != nil {
		t.Fatalf("register device: %v", err)
	}
	server.mqtt = MQTTConfig{
		Enabled:                    true,
		BrokerURL:                  "mqtt://127.0.0.1:1883",
		PublicBrokerURL:            "mqtt://127.0.0.1:1883",
		UsernamePrefix:             "slan",
		PasswordSecret:             "test-secret",
		TopicPrefix:                "slan",
		CredentialTTLSeconds:       3600,
		ControlMessageTTLSeconds:   3600,
		PublishTimeoutMilliseconds: 1,
	}
	handler := server.Routes()

	var group SecurityGroup
	postJSON(t, handler, "/api/networks/"+network.NetworkID+"/security-groups", "", map[string]any{
		"name": "",
	}, http.StatusCreated, &group)
	assertNetworkConfigDeliveryCount(t, store, device.DeviceID, 0)

	requestJSON(t, handler, http.MethodPatch, "/api/networks/"+network.NetworkID+"/security-groups/"+group.SecurityGroupID, "", map[string]any{
		"name": "renamed",
	}, http.StatusOK, &group)
	assertNetworkConfigDeliveryCount(t, store, device.DeviceID, 0)

	requestJSON(t, handler, http.MethodPatch, "/api/networks/"+network.NetworkID, "", map[string]any{
		"name":             "Security Push Renamed",
		"code":             "security-push-renamed",
		"intraGroupPolicy": "deny",
	}, http.StatusOK, &network)
	assertLatestNetworkConfigDelivery(t, store, device.DeviceID, 1, "network", "update", "network_updated", network.NetworkID)

	var rule SecurityGroupRule
	postJSON(t, handler, "/api/security-groups/"+group.SecurityGroupID+"/rules", "", map[string]any{
		"direction": "ingress",
		"action":    "allow",
		"protocol":  "tcp",
		"peerType":  "device",
		"peerValue": device.DeviceID,
		"priority":  100,
		"portFrom":  22,
		"portTo":    22,
		"enabled":   true,
	}, http.StatusCreated, &rule)
	assertLatestNetworkConfigDelivery(t, store, device.DeviceID, 2, "security_rule", "add", "security_rule_ingress_added", rule.RuleID)

	requestJSON(t, handler, http.MethodPatch, "/api/security-groups/rules/"+rule.RuleID, "", map[string]any{
		"direction": "ingress",
		"action":    "deny",
		"protocol":  "tcp",
		"peerType":  "device",
		"peerValue": device.DeviceID,
		"priority":  90,
		"portFrom":  22,
		"portTo":    22,
		"enabled":   true,
	}, http.StatusOK, &rule)
	assertLatestNetworkConfigDelivery(t, store, device.DeviceID, 3, "security_rule", "update", "security_rule_ingress_updated", rule.RuleID)

	requestJSON(t, handler, http.MethodDelete, "/api/security-groups/rules/"+rule.RuleID, "", map[string]any{}, http.StatusOK, nil)
	assertLatestNetworkConfigDelivery(t, store, device.DeviceID, 4, "security_rule", "remove", "security_rule_ingress_removed", rule.RuleID)

	var egressRule SecurityGroupRule
	postJSON(t, handler, "/api/security-groups/"+group.SecurityGroupID+"/rules", "", map[string]any{
		"direction": "egress",
		"action":    "allow",
		"protocol":  "tcp",
		"peerType":  "all",
		"peerValue": "all",
		"priority":  100,
		"portFrom":  443,
		"portTo":    443,
		"enabled":   true,
	}, http.StatusCreated, &egressRule)
	assertLatestNetworkConfigDelivery(t, store, device.DeviceID, 5, "security_rule", "add", "security_rule_egress_added", egressRule.RuleID)

	requestJSON(t, handler, http.MethodPatch, "/api/security-groups/rules/"+egressRule.RuleID, "", map[string]any{
		"direction": "egress",
		"action":    "deny",
		"protocol":  "tcp",
		"peerType":  "all",
		"peerValue": "all",
		"priority":  80,
		"portFrom":  443,
		"portTo":    443,
		"enabled":   true,
	}, http.StatusOK, &egressRule)
	assertLatestNetworkConfigDelivery(t, store, device.DeviceID, 6, "security_rule", "update", "security_rule_egress_updated", egressRule.RuleID)

	requestJSON(t, handler, http.MethodDelete, "/api/security-groups/rules/"+egressRule.RuleID, "", map[string]any{}, http.StatusOK, nil)
	assertLatestNetworkConfigDelivery(t, store, device.DeviceID, 7, "security_rule", "remove", "security_rule_egress_removed", egressRule.RuleID)

	requestJSON(t, handler, http.MethodPatch, "/api/networks/"+network.NetworkID+"/security-groups/"+group.SecurityGroupID, "", map[string]any{
		"name": "renamed",
	}, http.StatusOK, &group)
	assertNetworkConfigDeliveryCount(t, store, device.DeviceID, 7)

	requestJSON(t, handler, http.MethodDelete, "/api/networks/"+network.NetworkID+"/security-groups/"+group.SecurityGroupID, "", map[string]any{}, http.StatusOK, nil)
	assertLatestNetworkConfigDelivery(t, store, device.DeviceID, 8, "security_group", "remove", "security_group_removed", group.SecurityGroupID)

	requestJSON(t, handler, http.MethodPatch, "/api/networks/"+network.NetworkID, "", map[string]any{
		"name":             network.Name + " A",
		"code":             network.Code,
		"intraGroupPolicy": network.IntraGroupPolicy,
	}, http.StatusOK, &network)
	assertLatestNetworkConfigDelivery(t, store, device.DeviceID, 9, "network", "update", "network_updated", network.NetworkID)

	requestJSON(t, handler, http.MethodPatch, "/api/networks/"+network.NetworkID, "", map[string]any{
		"name":             network.Name + " B",
		"code":             network.Code,
		"intraGroupPolicy": network.IntraGroupPolicy,
	}, http.StatusOK, &network)
	assertLatestNetworkConfigDelivery(t, store, device.DeviceID, 10, "network", "update", "network_updated", network.NetworkID)
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

func waitForNetworkConfigDeliveryCount(t *testing.T, store *Store, deviceID string, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		if got := networkConfigDeliveryCount(store, deviceID); got >= want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("network_config_changed delivery count for %s = %d, want at least %d", deviceID, networkConfigDeliveryCount(store, deviceID), want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func assertNetworkConfigDeliveryCount(t *testing.T, store *Store, deviceID string, want int) {
	t.Helper()
	if got := networkConfigDeliveryCount(store, deviceID); got != want {
		t.Fatalf("network_config_changed delivery count for %s = %d, want %d", deviceID, got, want)
	}
}

func assertLatestNetworkConfigDelivery(t *testing.T, store *Store, deviceID string, wantCount int, resourceType, action, reason, resourceID string) {
	t.Helper()
	waitForNetworkConfigDeliveryCount(t, store, deviceID, wantCount)
	payload := latestNetworkConfigDeliveryPayload(t, store, deviceID)
	if payload.ResourceType != resourceType || payload.Action != action || payload.Reason != reason || payload.ResourceID != resourceID {
		t.Fatalf("latest network_config_changed payload = %+v, want resourceType=%s action=%s reason=%s resourceID=%s", payload, resourceType, action, reason, resourceID)
	}
}

func networkConfigDeliveryCount(store *Store, deviceID string) int {
	store.mu.Lock()
	defer store.mu.Unlock()
	count := 0
	for _, delivery := range store.controlDeliveries {
		if delivery.DeviceID == deviceID && delivery.MessageType == "network_config_changed" {
			count++
		}
	}
	return count
}

func latestNetworkConfigDeliveryPayload(t *testing.T, store *Store, deviceID string) networkChangePayload {
	t.Helper()
	store.mu.Lock()
	defer store.mu.Unlock()
	var latest MQTTControlDelivery
	var latestPayload networkChangePayload
	for _, delivery := range store.controlDeliveries {
		if delivery.DeviceID != deviceID || delivery.MessageType != "network_config_changed" {
			continue
		}
		var payload networkChangePayload
		if err := json.Unmarshal(delivery.Payload, &payload); err != nil {
			t.Fatalf("unmarshal network_config_changed payload: %v", err)
		}
		if latest.DeliveryID == "" || payload.ConfigVersion > latestPayload.ConfigVersion {
			latest = delivery
			latestPayload = payload
		}
	}
	if latest.DeliveryID == "" {
		t.Fatalf("no network_config_changed delivery for device %s", deviceID)
	}
	return latestPayload
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
