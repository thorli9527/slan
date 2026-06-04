package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

var httpClient = &http.Client{
	Transport: &http.Transport{
		Proxy: nil,
	},
	Timeout: 5 * time.Second,
}

type authResponse struct {
	AccessToken    string          `json:"accessToken,omitempty"`
	Auth           authPayload     `json:"auth,omitempty"`
	DefaultNetwork networkResponse `json:"defaultNetwork,omitempty"`
}

type authPayload struct {
	User    authUser    `json:"user"`
	Session authSession `json:"session"`
}

type authUser struct {
	UserID string `json:"userId"`
}

type authSession struct {
	Token string `json:"token"`
}

type deviceResponse struct {
	DeviceID             string              `json:"deviceId,omitempty"`
	CurrentVirtualIP     string              `json:"currentVirtualIp,omitempty"`
	Device               devicePayload       `json:"device,omitempty"`
	DefaultNetworkDevice networkDeviceRecord `json:"defaultNetworkDevice,omitempty"`
}

type devicePayload struct {
	DeviceID string `json:"deviceId"`
}

type networkDeviceRecord struct {
	NetworkDeviceID string `json:"networkDeviceId"`
	NetworkID       string `json:"networkId"`
	DeviceID        string `json:"deviceId"`
	Enabled         bool   `json:"enabled"`
	Status          string `json:"status"`
}

type networkHomeResponse struct {
	ActiveNetwork *networkResponse `json:"activeNetwork,omitempty"`
	OwnedNetwork  *networkResponse `json:"ownedNetwork,omitempty"`
}

type networkResponse struct {
	NetworkID string `json:"networkId"`
}

type activationResponse struct {
	Attachment attachmentResponse `json:"attachment"`
}

type attachmentResponse struct {
	AttachmentID string `json:"attachmentId"`
	NetworkID    string `json:"networkId"`
	DeviceID     string `json:"deviceId"`
	Status       string `json:"status,omitempty"`
}

type nodeResponse struct {
	NodeID     string   `json:"nodeId"`
	NetworkIDs []string `json:"networkIds,omitempty"`
}

type wireRegisterResponse struct {
	Peer wirePeer `json:"peer"`
}

type wirePeer struct {
	PeerID     string   `json:"peerId"`
	NetworkID  string   `json:"networkId"`
	NodeID     string   `json:"nodeId"`
	VirtualIPs []string `json:"virtualIps,omitempty"`
	AllowedIPs []string `json:"allowedIps,omitempty"`
}

type relayTicket struct {
	TicketID  string    `json:"ticketId"`
	PeerID    string    `json:"peerId"`
	SessionID string    `json:"sessionId"`
	Path      string    `json:"path"`
	RegionID  string    `json:"regionId"`
	NodeID    string    `json:"nodeId"`
	Host      string    `json:"host"`
	UDPPort   int       `json:"udpPort"`
	ExpiresAt time.Time `json:"expiresAt"`
	Signature string    `json:"signature"`
}

type derpTicket struct {
	TicketID  string    `json:"ticketId"`
	PeerID    string    `json:"peerId"`
	NetworkID string    `json:"networkId"`
	Path      string    `json:"path"`
	RegionID  string    `json:"regionId"`
	NodeID    string    `json:"nodeId"`
	ExpiresAt time.Time `json:"expiresAt"`
	Signature string    `json:"signature"`
}

type pathPlan struct {
	PreferredPath   string      `json:"preferredPath"`
	DegradedReason  string      `json:"degradedReason,omitempty"`
	FallbackOrder   []string    `json:"fallbackOrder"`
	RelayCandidates []relayNode `json:"relayCandidates,omitempty"`
	DerpCandidates  []derpNode  `json:"derpCandidates,omitempty"`
}

type relayNode struct {
	RegionID          string          `json:"regionId"`
	NodeID            string          `json:"nodeId"`
	Host              string          `json:"host"`
	UDPPort           int             `json:"udpPort"`
	Enabled           bool            `json:"enabled"`
	Healthy           bool            `json:"healthy"`
	Stale             bool            `json:"stale,omitempty"`
	TicketKeyRotation ticketKeyStatus `json:"ticketKeyRotation,omitempty"`
}

type derpNode struct {
	RegionID          string          `json:"regionId"`
	NodeID            string          `json:"nodeId"`
	Host              string          `json:"host"`
	Port              int             `json:"port"`
	Enabled           bool            `json:"enabled,omitempty"`
	Healthy           bool            `json:"healthy,omitempty"`
	Stale             bool            `json:"stale,omitempty"`
	TicketKeyRotation ticketKeyStatus `json:"ticketKeyRotation,omitempty"`
}

type ticketKeyStatus struct {
	Source             string `json:"source"`
	KeyRingID          string `json:"keyRingId"`
	SigningConfigured  bool   `json:"signingConfigured"`
	KeyRingConfigured  bool   `json:"keyRingConfigured"`
	EffectiveKeyCount  int    `json:"effectiveKeyCount"`
	RotationReady      bool   `json:"rotationReady"`
	AcceptsDevFallback bool   `json:"acceptsDevFallback"`
}

type relayNodeList struct {
	Items []relayNode `json:"items"`
}

type derpNodeList struct {
	Items []derpNode `json:"items"`
}

type smokeFailure struct {
	message string
}

func main() {
	defer exitOnFailure()

	bizURL := env("SLAN_BIZ_E2E_BIZ_URL", "http://127.0.0.1:28080")
	wireURL := env("SLAN_BIZ_E2E_WIRE_URL", "http://127.0.0.1:29100")
	wireBURL := env("SLAN_BIZ_E2E_WIRE_B_URL", "http://127.0.0.1:29101")
	relayAdminURL := env("SLAN_BIZ_E2E_RELAY_ADMIN_URL", "http://127.0.0.1:29111")
	relayBAdminURL := env("SLAN_BIZ_E2E_RELAY_B_ADMIN_URL", "http://127.0.0.1:29113")
	derpAdminURL := env("SLAN_BIZ_E2E_DERP_ADMIN_URL", "http://127.0.0.1:29121")
	derpBAdminURL := env("SLAN_BIZ_E2E_DERP_B_ADMIN_URL", "http://127.0.0.1:29123")
	relayAddr := env("SLAN_BIZ_E2E_RELAY_ADDR", "127.0.0.1:29110")
	derpAddr := env("SLAN_BIZ_E2E_DERP_ADDR", "127.0.0.1:29120")
	internalToken := env("SLAN_INTERNAL_WIRE_TOKEN", "change-me-wire-internal-token")

	waitHTTP(bizURL + "/healthz")
	waitHTTP(wireURL + "/healthz")
	waitHTTP(wireBURL + "/healthz")
	waitHTTP(relayAdminURL + "/healthz")
	waitHTTP(relayBAdminURL + "/healthz")
	waitHTTP(derpAdminURL + "/healthz")
	waitHTTP(derpBAdminURL + "/healthz")
	expectConsistentTicketKeyStatus(map[string]string{
		"wire":    wireURL + "/internal/wire/ticket-key-status",
		"wire-b":  wireBURL + "/internal/wire/ticket-key-status",
		"relay":   relayAdminURL + "/ticket-key-status",
		"relay-b": relayBAdminURL + "/ticket-key-status",
		"derp":    derpAdminURL + "/ticket-key-status",
		"derp-b":  derpBAdminURL + "/ticket-key-status",
	})
	if shouldRestoreWireDataPlaneNodes(bizURL) {
		restoreLocalWireDataPlaneNodes(bizURL, internalToken)
	}

	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	smokeRegionID := "wire-e2e-smoke-" + suffix
	smokeRelayNodeIDs := []string{"relay-e2e-a-" + suffix, "relay-e2e-b-" + suffix}
	smokeDerpNodeIDs := []string{"derp-e2e-a-" + suffix, "derp-e2e-b-" + suffix}
	defer cleanupSmokeWireNodes(bizURL, internalToken, smokeRegionID, smokeRelayNodeIDs, smokeDerpNodeIDs)
	createSmokeWireNodes(bizURL, internalToken, relayAdminURL, derpAdminURL, relayAddr, derpAddr, smokeRegionID, smokeRelayNodeIDs, smokeDerpNodeIDs)

	email := "wire-e2e-" + suffix + "@local.slan"
	password := "Password123!"
	deviceID := "dev-wire-e2e-" + suffix
	nodeID := "node-" + deviceID

	var auth authResponse
	postJSON(bizURL+"/api/auth/register", "", map[string]any{
		"email":    email,
		"password": password,
	}, &auth)
	if auth.AccessToken == "" {
		auth.AccessToken = auth.Auth.Session.Token
	}
	if auth.AccessToken == "" {
		fail("missing access token from register")
	}
	userID := auth.Auth.User.UserID
	if userID == "" {
		fail("missing userId from register: %+v", auth)
	}
	defer cleanupSmokeDevice(bizURL, userID, deviceID)

	var device deviceResponse
	postJSON(bizURL+"/api/devices/register", auth.AccessToken, map[string]any{
		"userId":    userID,
		"deviceId":  deviceID,
		"name":      "wire e2e device",
		"platform":  "smoke",
		"osName":    "smoke",
		"osVersion": "1",
		"publicKey": "device-public-key-" + suffix,
	}, &device)
	if device.DeviceID == "" {
		device.DeviceID = device.Device.DeviceID
	}
	if device.DeviceID != deviceID {
		fail("unexpected device response: %+v", device)
	}

	networkID := auth.DefaultNetwork.NetworkID
	if networkID == "" {
		networkID = device.DefaultNetworkDevice.NetworkID
	}
	if networkID == "" {
		fail("missing default network: auth=%+v device=%+v", auth, device)
	}

	expectGETStatus(bizURL+"/internal/wire/peers/"+nodeID+"/authz", nil, http.StatusUnauthorized)
	expectGETStatus(bizURL+"/internal/wire/peers/"+nodeID+"/authz", map[string]string{
		"X-Slan-Internal-Token": "wrong-token",
	}, http.StatusUnauthorized)

	var authz wirePeer
	getJSONWithHeader(bizURL+"/internal/wire/peers/"+nodeID+"/authz", map[string]string{
		"X-Slan-Internal-Token": internalToken,
	}, &authz)
	if authz.NetworkID != networkID || authz.NodeID != nodeID || len(authz.VirtualIPs) == 0 {
		fail("unexpected biz wire authz: %+v want network=%s node=%s", authz, networkID, nodeID)
	}

	expectPostStatus(wireURL+"/peers/register", "", map[string]any{
		"peer": map[string]any{
			"peerId":                "missing-" + nodeID,
			"networkId":             networkID,
			"nodeId":                "missing-" + nodeID,
			"publicKey":             "wire-public-key-missing-" + suffix,
			"supportsRelayUdp":      true,
			"supportsDerpTcpTls443": true,
		},
	}, http.StatusBadRequest)

	var wireReg wireRegisterResponse
	postJSON(wireURL+"/peers/register", "", map[string]any{
		"peer": map[string]any{
			"peerId":                  nodeID,
			"networkId":               "client-forged-network",
			"nodeId":                  "client-forged-node",
			"publicKey":               "wire-public-key-" + suffix,
			"supportsLanDirect":       true,
			"supportsIpv6Direct":      true,
			"supportsDirectUdp":       true,
			"supportsRelayUdp":        true,
			"supportsDerpTcpTls443":   true,
			"allowEndpointRoaming":    true,
			"allowFastReselection":    true,
			"allowRelayTicketRenewal": true,
		},
	}, &wireReg)
	if wireReg.Peer.NetworkID != networkID || wireReg.Peer.NodeID != nodeID || len(wireReg.Peer.VirtualIPs) == 0 {
		fail("wire did not apply biz authz: %+v want network=%s node=%s", wireReg.Peer, networkID, nodeID)
	}
	var wireBReg wireRegisterResponse
	postJSON(wireBURL+"/peers/register", "", map[string]any{
		"peer": map[string]any{
			"peerId":                  nodeID,
			"networkId":               "client-forged-network-b",
			"nodeId":                  "client-forged-node-b",
			"publicKey":               "wire-public-key-b-" + suffix,
			"supportsLanDirect":       true,
			"supportsIpv6Direct":      true,
			"supportsDirectUdp":       true,
			"supportsRelayUdp":        true,
			"supportsDerpTcpTls443":   true,
			"allowEndpointRoaming":    true,
			"allowFastReselection":    true,
			"allowRelayTicketRenewal": true,
		},
	}, &wireBReg)
	if wireBReg.Peer.NetworkID != networkID || wireBReg.Peer.NodeID != nodeID || len(wireBReg.Peer.VirtualIPs) == 0 {
		fail("wire-b did not apply biz authz: %+v want network=%s node=%s", wireBReg.Peer, networkID, nodeID)
	}

	postJSON(wireURL+"/peers/path-health", "", map[string]any{
		"peerId": nodeID,
		"probes": []map[string]any{
			{"path": "relay_udp", "reachable": true, "rttMs": 20, "mtu": 1280},
			{"path": "derp_tcp_tls_443", "reachable": true, "rttMs": 70, "mtu": 1240},
		},
	}, nil)
	postJSON(wireBURL+"/peers/path-health", "", map[string]any{
		"peerId": nodeID,
		"probes": []map[string]any{
			{"path": "relay_udp", "reachable": true, "rttMs": 20, "mtu": 1280},
			{"path": "derp_tcp_tls_443", "reachable": true, "rttMs": 70, "mtu": 1240},
		},
	}, nil)

	var relayResp struct {
		Ticket relayTicket `json:"ticket"`
	}
	postJSON(wireURL+"/relay/tickets", "", map[string]any{"peerId": nodeID, "ttlSeconds": 300}, &relayResp)
	if relayResp.Ticket.PeerID != nodeID || relayResp.Ticket.Path != "relay_udp" || relayResp.Ticket.Signature == "" {
		fail("invalid relay ticket: %+v", relayResp.Ticket)
	}
	if relayResp.Ticket.NodeID == "" || relayResp.Ticket.Host == "" || relayResp.Ticket.UDPPort <= 0 {
		fail("relay ticket did not include biz relay node: %+v", relayResp.Ticket)
	}

	var derpResp struct {
		Ticket derpTicket `json:"ticket"`
	}
	postJSON(wireURL+"/derp/tickets", "", map[string]any{"peerId": nodeID, "ttlSeconds": 300}, &derpResp)
	if derpResp.Ticket.PeerID != nodeID || derpResp.Ticket.NetworkID != networkID || derpResp.Ticket.Path != "derp_tcp_tls_443" || derpResp.Ticket.Signature == "" {
		fail("invalid derp ticket: %+v want network=%s", derpResp.Ticket, networkID)
	}
	if derpResp.Ticket.RegionID == "" || derpResp.Ticket.NodeID == "" {
		fail("derp ticket did not include biz derp node: %+v", derpResp.Ticket)
	}

	var plan pathPlan
	postJSON(wireURL+"/path-plan", "", map[string]any{"peerId": nodeID}, &plan)
	if len(plan.RelayCandidates) == 0 || plan.RelayCandidates[0].NodeID != relayResp.Ticket.NodeID {
		fail("path plan missing biz relay candidates: plan=%+v ticket=%+v", plan, relayResp.Ticket)
	}
	if len(plan.DerpCandidates) == 0 || plan.DerpCandidates[0].NodeID != derpResp.Ticket.NodeID {
		fail("path plan missing biz derp candidates: plan=%+v ticket=%+v", plan, derpResp.Ticket)
	}
	if len(plan.RelayCandidates) < 2 {
		fail("expected at least two relay candidates for multi-instance scheduling, got %+v", plan.RelayCandidates)
	}
	if len(plan.DerpCandidates) < 2 {
		fail("expected at least two derp candidates for multi-instance scheduling, got %+v", plan.DerpCandidates)
	}
	var planB pathPlan
	postJSON(wireBURL+"/path-plan", "", map[string]any{"peerId": nodeID}, &planB)
	if !sameRelayCandidates(plan.RelayCandidates, planB.RelayCandidates) || !sameDerpCandidates(plan.DerpCandidates, planB.DerpCandidates) {
		fail("wire instances returned inconsistent candidates: a=%+v b=%+v", plan, planB)
	}

	disabledFlag := false
	patchJSONWithInternalToken(
		bizURL+"/internal/wire/admin/relay-nodes/"+relayResp.Ticket.RegionID+"/"+relayResp.Ticket.NodeID+"/status",
		internalToken,
		map[string]any{"enabled": disabledFlag},
		nil,
	)
	patchJSONWithInternalToken(
		bizURL+"/internal/wire/admin/derp-nodes/"+derpResp.Ticket.RegionID+"/"+derpResp.Ticket.NodeID+"/status",
		internalToken,
		map[string]any{"enabled": disabledFlag},
		nil,
	)
	defer patchJSONWithInternalToken(
		bizURL+"/internal/wire/admin/relay-nodes/"+relayResp.Ticket.RegionID+"/"+relayResp.Ticket.NodeID+"/status",
		internalToken,
		map[string]any{"enabled": true, "healthy": true},
		nil,
	)
	defer patchJSONWithInternalToken(
		bizURL+"/internal/wire/admin/derp-nodes/"+derpResp.Ticket.RegionID+"/"+derpResp.Ticket.NodeID+"/status",
		internalToken,
		map[string]any{"enabled": true, "healthy": true},
		nil,
	)

	var afterDisable pathPlan
	postJSON(wireURL+"/path-plan", "", map[string]any{"peerId": nodeID}, &afterDisable)
	var afterDisableB pathPlan
	postJSON(wireBURL+"/path-plan", "", map[string]any{"peerId": nodeID}, &afterDisableB)
	if containsRelayNode(afterDisable.RelayCandidates, relayResp.Ticket.NodeID) {
		fail("disabled relay node still selected: disabled=%s candidates=%+v", relayResp.Ticket.NodeID, afterDisable.RelayCandidates)
	}
	if containsDerpNode(afterDisable.DerpCandidates, derpResp.Ticket.NodeID) {
		fail("disabled derp node still selected: disabled=%s candidates=%+v", derpResp.Ticket.NodeID, afterDisable.DerpCandidates)
	}
	if !sameRelayCandidates(afterDisable.RelayCandidates, afterDisableB.RelayCandidates) || !sameDerpCandidates(afterDisable.DerpCandidates, afterDisableB.DerpCandidates) {
		fail("wire instances returned inconsistent candidates after disable: a=%+v b=%+v", afterDisable, afterDisableB)
	}

	disabledRelayNodes := smokeRelayCandidates(afterDisable.RelayCandidates, smokeRegionID)
	disabledDerpNodes := smokeDerpCandidates(afterDisable.DerpCandidates, smokeRegionID)
	for _, node := range disabledRelayNodes {
		patchJSONWithInternalToken(
			bizURL+"/internal/wire/admin/relay-nodes/"+node.RegionID+"/"+node.NodeID+"/status",
			internalToken,
			map[string]any{"enabled": false},
			nil,
		)
		defer patchJSONWithInternalToken(
			bizURL+"/internal/wire/admin/relay-nodes/"+node.RegionID+"/"+node.NodeID+"/status",
			internalToken,
			map[string]any{"enabled": true, "healthy": true},
			nil,
		)
	}
	for _, node := range disabledDerpNodes {
		patchJSONWithInternalToken(
			bizURL+"/internal/wire/admin/derp-nodes/"+node.RegionID+"/"+node.NodeID+"/status",
			internalToken,
			map[string]any{"enabled": false},
			nil,
		)
		defer patchJSONWithInternalToken(
			bizURL+"/internal/wire/admin/derp-nodes/"+node.RegionID+"/"+node.NodeID+"/status",
			internalToken,
			map[string]any{"enabled": true, "healthy": true},
			nil,
		)
	}
	var directOnly pathPlan
	postJSON(wireURL+"/path-plan", "", map[string]any{"peerId": nodeID}, &directOnly)
	var directOnlyB pathPlan
	postJSON(wireBURL+"/path-plan", "", map[string]any{"peerId": nodeID}, &directOnlyB)
	if containsRelayRegion(directOnly.RelayCandidates, smokeRegionID) {
		fail("disabled smoke relay nodes still selected: region=%s candidates=%+v", smokeRegionID, directOnly.RelayCandidates)
	}
	if containsDerpRegion(directOnly.DerpCandidates, smokeRegionID) {
		fail("disabled smoke derp nodes still selected: region=%s candidates=%+v", smokeRegionID, directOnly.DerpCandidates)
	}
	if !sameRelayCandidates(directOnly.RelayCandidates, directOnlyB.RelayCandidates) || !sameDerpCandidates(directOnly.DerpCandidates, directOnlyB.DerpCandidates) {
		fail("wire instances returned inconsistent candidates after disabling smoke nodes: a=%+v b=%+v", directOnly, directOnlyB)
	}

	for _, node := range disabledRelayNodes {
		patchJSONWithInternalToken(
			bizURL+"/internal/wire/admin/relay-nodes/"+node.RegionID+"/"+node.NodeID+"/status",
			internalToken,
			map[string]any{"enabled": true, "healthy": true},
			nil,
		)
	}
	for _, node := range disabledDerpNodes {
		patchJSONWithInternalToken(
			bizURL+"/internal/wire/admin/derp-nodes/"+node.RegionID+"/"+node.NodeID+"/status",
			internalToken,
			map[string]any{"enabled": true, "healthy": true},
			nil,
		)
	}
	expectWireAdminNodeViews(bizURL, internalToken)

	smokeRelay(relayAddr, relayResp.Ticket, nodeID)
	smokeDerp(derpAddr, derpResp.Ticket, nodeID)

	disabledValue := false
	var disabled networkDeviceRecord
	patchJSON(bizURL+"/api/networks/"+networkID+"/devices/"+deviceID, auth.AccessToken, map[string]any{
		"enabled": disabledValue,
	}, &disabled)
	if disabled.DeviceID != deviceID || disabled.Enabled {
		fail("unexpected disabled network device response: %+v want device=%s enabled=false", disabled, deviceID)
	}

	expectPostStatus(wireURL+"/relay/tickets", "", map[string]any{
		"peerId":     nodeID,
		"ttlSeconds": 300,
	}, http.StatusBadRequest)
	expectPostStatus(wireURL+"/derp/tickets", "", map[string]any{
		"peerId":     nodeID,
		"ttlSeconds": 300,
	}, http.StatusBadRequest)
	expectPostStatus(wireURL+"/path-plan", "", map[string]any{
		"peerId": nodeID,
	}, http.StatusBadRequest)

	fmt.Println("wire biz e2e smoke passed")
}

func waitHTTP(url string) {
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := httpClient.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	fail("timeout waiting for %s", url)
}

func restoreLocalWireDataPlaneNodes(bizURL, internalToken string) {
	headers := map[string]string{"X-Slan-Internal-Token": internalToken}
	var relayList relayNodeList
	getJSONWithHeader(bizURL+"/internal/wire/admin/relay-nodes", headers, &relayList)
	for _, node := range relayList.Items {
		patchJSONWithInternalToken(
			bizURL+"/internal/wire/admin/relay-nodes/"+node.RegionID+"/"+node.NodeID+"/status",
			internalToken,
			map[string]any{"enabled": true, "healthy": true},
			nil,
		)
	}
	var derpList derpNodeList
	getJSONWithHeader(bizURL+"/internal/wire/admin/derp-nodes", headers, &derpList)
	for _, node := range derpList.Items {
		patchJSONWithInternalToken(
			bizURL+"/internal/wire/admin/derp-nodes/"+node.RegionID+"/"+node.NodeID+"/status",
			internalToken,
			map[string]any{"enabled": true, "healthy": true},
			nil,
		)
	}
}

func shouldRestoreWireDataPlaneNodes(bizURL string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("SLAN_BIZ_E2E_RESTORE_NODES"))) {
	case "1", "true", "yes":
		return true
	case "0", "false", "no":
		return false
	}
	return strings.Contains(bizURL, "127.0.0.1") || strings.Contains(bizURL, "localhost")
}

func expectConsistentTicketKeyStatus(endpoints map[string]string) {
	var baselineName string
	var baseline ticketKeyStatus
	for name, url := range endpoints {
		var current ticketKeyStatus
		getJSONWithHeader(url, nil, &current)
		if current.EffectiveKeyCount <= 0 || current.KeyRingID == "" {
			fail("%s returned invalid ticket key status: %+v", name, current)
		}
		if baselineName == "" {
			baselineName = name
			baseline = current
			continue
		}
		if !sameTicketKeyStatus(baseline, current) {
			fail("ticket key status mismatch: %s=%+v %s=%+v", baselineName, baseline, name, current)
		}
	}
}

func expectWireAdminNodeViews(bizURL, internalToken string) {
	headers := map[string]string{"X-Slan-Internal-Token": internalToken}
	var relayList relayNodeList
	getJSONWithHeader(bizURL+"/internal/wire/admin/relay-nodes", headers, &relayList)
	if !hasActiveRelayNode(relayList.Items) {
		fail("biz internal relay node view has no active node: %+v", relayList.Items)
	}
	var derpList derpNodeList
	getJSONWithHeader(bizURL+"/internal/wire/admin/derp-nodes", headers, &derpList)
	if !hasActiveDerpNode(derpList.Items) {
		fail("biz internal derp node view has no active node: %+v", derpList.Items)
	}
}

func createSmokeWireNodes(bizURL, internalToken, relayAdminURL, derpAdminURL, relayAddr, derpAddr, regionID string, relayNodeIDs, derpNodeIDs []string) {
	relayHost, relayPort := splitAddress(relayAddr)
	derpHost, derpPort := splitAddress(derpAddr)
	var relayTicketKey ticketKeyStatus
	getJSONWithHeader(relayAdminURL+"/ticket-key-status", nil, &relayTicketKey)
	var derpTicketKey ticketKeyStatus
	getJSONWithHeader(derpAdminURL+"/ticket-key-status", nil, &derpTicketKey)
	for idx, nodeID := range relayNodeIDs {
		putJSONWithInternalToken(bizURL+"/internal/wire/admin/relay-nodes", internalToken, map[string]any{
			"regionId":          regionID,
			"nodeId":            nodeID,
			"host":              relayHost,
			"udpPort":           relayPort,
			"adminPort":         relayPort + 1,
			"enabled":           true,
			"healthy":           true,
			"priority":          idx + 1,
			"ticketKeyRotation": relayTicketKey,
		}, nil)
	}
	for idx, nodeID := range derpNodeIDs {
		putJSONWithInternalToken(bizURL+"/internal/wire/admin/derp-nodes", internalToken, map[string]any{
			"regionId":          regionID,
			"nodeId":            nodeID,
			"name":              "Wire E2E Smoke",
			"host":              derpHost,
			"port":              derpPort,
			"enabled":           true,
			"healthy":           true,
			"priority":          idx + 1,
			"ticketKeyRotation": derpTicketKey,
		}, nil)
	}
}

func cleanupSmokeWireNodes(bizURL, internalToken, regionID string, relayNodeIDs, derpNodeIDs []string) {
	for _, nodeID := range relayNodeIDs {
		patchJSONWithInternalTokenAllowNotFound(
			bizURL+"/internal/wire/admin/relay-nodes/"+regionID+"/"+nodeID+"/status",
			internalToken,
			map[string]any{"enabled": false, "healthy": false},
			nil,
		)
		deleteWithInternalToken(bizURL+"/internal/wire/admin/relay-nodes/"+regionID+"/"+nodeID, internalToken)
	}
	for _, nodeID := range derpNodeIDs {
		patchJSONWithInternalTokenAllowNotFound(
			bizURL+"/internal/wire/admin/derp-nodes/"+regionID+"/"+nodeID+"/status",
			internalToken,
			map[string]any{"enabled": false, "healthy": false},
			nil,
		)
		deleteWithInternalToken(bizURL+"/internal/wire/admin/derp-nodes/"+regionID+"/"+nodeID, internalToken)
	}
}

func cleanupSmokeDevice(bizURL, userID, deviceID string) {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(deviceID) == "" {
		return
	}
	client := &http.Client{
		Transport: &http.Transport{Proxy: nil},
		Timeout:   30 * time.Second,
	}
	req, err := http.NewRequest(
		http.MethodDelete,
		bizURL+"/api/devices/"+url.PathEscape(deviceID)+"?actorUserId="+url.QueryEscape(userID),
		nil,
	)
	if err != nil {
		return
	}
	resp, err := client.Do(req)
	if err == nil {
		_ = resp.Body.Close()
	}
}

func hasActiveRelayNode(nodes []relayNode) bool {
	for _, node := range nodes {
		if node.Enabled && node.Healthy && !node.Stale && strings.TrimSpace(node.Host) != "" && node.UDPPort > 0 {
			return true
		}
	}
	return false
}

func hasActiveDerpNode(nodes []derpNode) bool {
	for _, node := range nodes {
		if node.Enabled && node.Healthy && !node.Stale && strings.TrimSpace(node.Host) != "" && node.Port > 0 {
			return true
		}
	}
	return false
}

func smokeRelayCandidates(nodes []relayNode, regionID string) []relayNode {
	out := make([]relayNode, 0)
	for _, node := range nodes {
		if node.RegionID == regionID {
			out = append(out, node)
		}
	}
	return out
}

func smokeDerpCandidates(nodes []derpNode, regionID string) []derpNode {
	out := make([]derpNode, 0)
	for _, node := range nodes {
		if node.RegionID == regionID {
			out = append(out, node)
		}
	}
	return out
}

func containsRelayRegion(nodes []relayNode, regionID string) bool {
	for _, node := range nodes {
		if node.RegionID == regionID {
			return true
		}
	}
	return false
}

func containsDerpRegion(nodes []derpNode, regionID string) bool {
	for _, node := range nodes {
		if node.RegionID == regionID {
			return true
		}
	}
	return false
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

func getJSON(url, token string, out any) {
	headers := map[string]string{}
	if token != "" {
		headers["Authorization"] = "Bearer " + token
	}
	getJSONWithHeader(url, headers, out)
}

func getJSONWithHeader(url string, headers map[string]string, out any) {
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

func expectGETStatus(url string, headers map[string]string, want int) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	must(err)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	expectStatus(req, want)
}

func postJSON(url, token string, in, out any) {
	payload, err := json.Marshal(in)
	must(err)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	must(err)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := httpClient.Do(req)
	must(err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		fail("POST %s status=%d body=%s", url, resp.StatusCode, string(body))
	}
	if out != nil {
		must(json.Unmarshal(body, out))
	}
}

func putJSON(url, token string, in, out any) {
	payload, err := json.Marshal(in)
	must(err)
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(payload))
	must(err)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
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

func putJSONWithInternalToken(url, token string, in, out any) {
	payload, err := json.Marshal(in)
	must(err)
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(payload))
	must(err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Slan-Internal-Token", token)
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

func patchJSON(url, token string, in, out any) {
	payload, err := json.Marshal(in)
	must(err)
	req, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(payload))
	must(err)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := httpClient.Do(req)
	must(err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		fail("PATCH %s status=%d body=%s", url, resp.StatusCode, string(body))
	}
	if out != nil {
		must(json.Unmarshal(body, out))
	}
}

func patchJSONWithInternalToken(url, token string, in, out any) {
	payload, err := json.Marshal(in)
	must(err)
	req, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(payload))
	must(err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Slan-Internal-Token", token)
	resp, err := httpClient.Do(req)
	must(err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		fail("PATCH %s status=%d body=%s", url, resp.StatusCode, string(body))
	}
	if out != nil {
		must(json.Unmarshal(body, out))
	}
}

func patchJSONWithInternalTokenAllowNotFound(url, token string, in, out any) {
	payload, err := json.Marshal(in)
	must(err)
	req, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(payload))
	must(err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Slan-Internal-Token", token)
	resp, err := httpClient.Do(req)
	must(err)
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

func deleteWithInternalToken(url, token string) {
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	must(err)
	req.Header.Set("X-Slan-Internal-Token", token)
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

func expectPostStatus(url, token string, in any, want int) {
	payload, err := json.Marshal(in)
	must(err)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	must(err)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	expectStatus(req, want)
}

func containsRelayNode(nodes []relayNode, nodeID string) bool {
	for _, node := range nodes {
		if node.NodeID == nodeID {
			return true
		}
	}
	return false
}

func containsDerpNode(nodes []derpNode, nodeID string) bool {
	for _, node := range nodes {
		if node.NodeID == nodeID {
			return true
		}
	}
	return false
}

func sameRelayCandidates(a, b []relayNode) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].RegionID != b[i].RegionID || a[i].NodeID != b[i].NodeID || a[i].Host != b[i].Host || a[i].UDPPort != b[i].UDPPort {
			return false
		}
	}
	return true
}

func sameDerpCandidates(a, b []derpNode) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].RegionID != b[i].RegionID || a[i].NodeID != b[i].NodeID || a[i].Host != b[i].Host || a[i].Port != b[i].Port {
			return false
		}
	}
	return true
}

func expectStatus(req *http.Request, want int) {
	resp, err := httpClient.Do(req)
	must(err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != want {
		fail("%s %s status=%d want=%d body=%s", req.Method, req.URL.String(), resp.StatusCode, want, string(body))
	}
}

func smokeRelay(address string, ticket relayTicket, nodeID string) {
	addr, err := net.ResolveUDPAddr("udp", address)
	must(err)
	a := udpConn()
	defer a.Close()
	b := udpConn()
	defer b.Close()
	peerID := nodeID + "-peer"

	udpSend(a, addr, map[string]any{"kind": "attach", "participantId": nodeID, "transport": "relay_udp", "ticket": ticket})
	udpRecv(a)
	udpSend(b, addr, map[string]any{"kind": "attach", "participantId": peerID, "transport": "relay_udp", "ticket": ticket})
	udpRecv(b)

	udpSend(a, addr, map[string]any{"kind": "forward", "sessionId": ticket.SessionID, "participantId": nodeID, "payload": []byte("relay-payload")})
	packet := udpRecv(b)
	if packet["kind"] != "packet" {
		fail("expected relay packet, got %+v", packet)
	}
	forwarded := udpRecv(a)
	if forwarded["kind"] != "forwarded" {
		fail("expected relay forwarded, got %+v", forwarded)
	}
}

func smokeDerp(address string, ticket derpTicket, nodeID string) {
	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	must(err)
	defer conn.Close()
	enc := json.NewEncoder(conn)
	dec := json.NewDecoder(conn)
	must(enc.Encode(map[string]any{
		"kind":     "connect",
		"peerId":   ticket.PeerID,
		"nodeId":   nodeID,
		"regionId": ticket.RegionID,
		"ticket":   ticket,
	}))
	var connected map[string]any
	must(dec.Decode(&connected))
	if connected["kind"] != "connected" {
		fail("expected derp connected, got %+v", connected)
	}
	sessionID, _ := connected["sessionId"].(string)
	if sessionID == "" {
		fail("missing derp session id: %+v", connected)
	}
	must(enc.Encode(map[string]any{"kind": "disconnect", "sessionId": sessionID}))
	var disconnected map[string]any
	must(dec.Decode(&disconnected))
	if disconnected["kind"] != "disconnected" {
		fail("expected derp disconnected, got %+v", disconnected)
	}
}

func udpConn() *net.UDPConn {
	conn, err := net.ListenUDP("udp", &net.UDPAddr{Port: 0})
	must(err)
	return conn
}

func udpSend(conn *net.UDPConn, addr *net.UDPAddr, msg any) {
	payload, err := json.Marshal(msg)
	must(err)
	_, err = conn.WriteToUDP(payload, addr)
	must(err)
}

func udpRecv(conn *net.UDPConn) map[string]any {
	must(conn.SetReadDeadline(time.Now().Add(5 * time.Second)))
	buf := make([]byte, 64*1024)
	n, _, err := conn.ReadFromUDP(buf)
	must(err)
	var msg map[string]any
	must(json.Unmarshal(buf[:n], &msg))
	if msg["kind"] == "error" {
		fail("udp error response: %+v", msg)
	}
	return msg
}

func splitAddress(value string) (string, int) {
	host, portText, err := net.SplitHostPort(strings.TrimSpace(value))
	if err != nil {
		fail("invalid host:port address %q: %v", value, err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port <= 0 || port > 65535 {
		fail("invalid port in address %q", value)
	}
	if strings.TrimSpace(host) == "" {
		fail("missing host in address %q", value)
	}
	return host, port
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
