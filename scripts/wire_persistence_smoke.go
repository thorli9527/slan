package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"
)

const (
	stateFile     = "/private/tmp/slan-wire-persistence-smoke.json"
	smokeRegionID = "smoke-region"
)

var httpClient = &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: 5 * time.Second}

type persistenceState struct {
	PeerID      string `json:"peerId"`
	NetworkID   string `json:"networkId"`
	Token       string `json:"token"`
	RegionID    string `json:"regionId"`
	RelayNodeID string `json:"relayNodeId"`
	DerpNodeID  string `json:"derpNodeId"`
	DeviceID    string `json:"deviceId,omitempty"`
	UserEmail   string `json:"userEmail,omitempty"`
}

type authResponse struct {
	AccessToken string `json:"accessToken"`
}

type cleanupAuthResponse struct {
	Auth struct {
		User struct {
			UserID string `json:"userId"`
		} `json:"user"`
	} `json:"auth"`
}

type networkHomeResponse struct {
	ActiveNetwork *networkResponse `json:"activeNetwork,omitempty"`
	OwnedNetwork  *networkResponse `json:"ownedNetwork,omitempty"`
}

type networkResponse struct {
	NetworkID string `json:"networkId"`
}

type wireRegisterResponse struct {
	Peer wirePeer `json:"peer"`
}

type wirePeer struct {
	PeerID          string             `json:"peerId"`
	NetworkID       string             `json:"networkId"`
	NodeID          string             `json:"nodeId"`
	VirtualIPs      []string           `json:"virtualIps,omitempty"`
	ActivePath      string             `json:"activePath,omitempty"`
	EndpointChanged bool               `json:"endpointChanged,omitempty"`
	Probes          []probe            `json:"probes,omitempty"`
	DerpHealth      []derpHealthSample `json:"derpHealth,omitempty"`
	RelayTicket     relayTicket        `json:"relayTicket,omitempty"`
}

type runtimeConfig struct {
	PeerID        string `json:"peerId"`
	NetworkID     string `json:"networkId"`
	PreferredPath string `json:"preferredPath"`
}

type pathPlan struct {
	PreferredPath   string      `json:"preferredPath"`
	RelayCandidates []relayNode `json:"relayCandidates,omitempty"`
	DerpCandidates  []derpNode  `json:"derpCandidates,omitempty"`
}

type probe struct {
	Path      string `json:"path"`
	Reachable bool   `json:"reachable"`
	RTTMs     int    `json:"rttMs,omitempty"`
	MTU       int    `json:"mtu,omitempty"`
}

type derpHealthSample struct {
	RegionID  string `json:"regionId"`
	NodeID    string `json:"nodeId"`
	Reachable bool   `json:"reachable"`
	RTTMs     int    `json:"rttMs,omitempty"`
}

type relayTicket struct {
	TicketID  string `json:"ticketId,omitempty"`
	PeerID    string `json:"peerId,omitempty"`
	SessionID string `json:"sessionId,omitempty"`
	Path      string `json:"path,omitempty"`
	RegionID  string `json:"regionId,omitempty"`
	NodeID    string `json:"nodeId,omitempty"`
	Host      string `json:"host,omitempty"`
	UDPPort   int    `json:"udpPort,omitempty"`
	Present   bool   `json:"present"`
	Signature string `json:"signature,omitempty"`
}

type relayTicketResponse struct {
	Ticket relayTicket `json:"ticket"`
}

type relayNode struct {
	RegionID string `json:"regionId"`
	NodeID   string `json:"nodeId"`
	Host     string `json:"host"`
	UDPPort  int    `json:"udpPort"`
}

type derpMapResponse struct {
	Map derpMap `json:"map"`
}

type derpMap struct {
	Regions []derpRegion `json:"regions"`
}

type derpRegion struct {
	RegionID string     `json:"regionId"`
	Nodes    []derpNode `json:"nodes"`
}

type derpNode struct {
	RegionID string `json:"regionId"`
	NodeID   string `json:"nodeId"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
}

type ticketKeyStatus struct {
	Source             string `json:"source,omitempty"`
	KeyRingID          string `json:"keyRingId,omitempty"`
	SigningConfigured  bool   `json:"signingConfigured"`
	KeyRingConfigured  bool   `json:"keyRingConfigured"`
	EffectiveKeyCount  int    `json:"effectiveKeyCount,omitempty"`
	RotationReady      bool   `json:"rotationReady"`
	AcceptsDevFallback bool   `json:"acceptsDevFallback,omitempty"`
}

type smokeFailure struct {
	message string
}

func main() {
	defer exitOnFailure()

	if len(os.Args) != 2 {
		fail("usage: wire_persistence_smoke.go seed|verify|cleanup")
	}
	switch os.Args[1] {
	case "seed":
		seed()
	case "verify":
		verify()
	case "cleanup":
		cleanup()
	default:
		fail("usage: wire_persistence_smoke.go seed|verify|cleanup")
	}
}

func seed() {
	bizURL := env("SLAN_BIZ_E2E_BIZ_URL", "http://127.0.0.1:28080")
	wireURL := env("SLAN_BIZ_E2E_WIRE_URL", "http://127.0.0.1:29100")
	relayAdminURL := env("SLAN_BIZ_E2E_RELAY_ADMIN_URL", "http://127.0.0.1:29111")
	derpAdminURL := env("SLAN_BIZ_E2E_DERP_ADMIN_URL", "http://127.0.0.1:29121")
	internalToken := env("SLAN_INTERNAL_WIRE_TOKEN", "change-me-wire-internal-token")
	waitHTTP(bizURL + "/healthz")
	waitHTTP(wireURL + "/healthz")
	waitHTTP(relayAdminURL + "/healthz")
	waitHTTP(derpAdminURL + "/healthz")

	var relayTicketKey ticketKeyStatus
	getJSON(relayAdminURL+"/v1/ticket-key-status", "", &relayTicketKey)
	if relayTicketKey.KeyRingID == "" {
		fail("relay returned empty ticket key status: %+v", relayTicketKey)
	}
	var derpTicketKey ticketKeyStatus
	getJSON(derpAdminURL+"/v1/ticket-key-status", "", &derpTicketKey)
	if derpTicketKey.KeyRingID == "" {
		fail("derp returned empty ticket key status: %+v", derpTicketKey)
	}

	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	deviceID := "dev-wire-persist-" + suffix
	nodeID := "node-wire-persist-" + suffix
	regionID := smokeRegionID
	relayNodeID := "relay-smoke-" + suffix
	derpNodeID := "derp-smoke-" + suffix
	seedCompleted := false
	defer func() {
		if !seedCompleted {
			disableSmokeNodesBestEffort(bizURL, internalToken, regionID, relayNodeID, derpNodeID)
			_ = os.Remove(stateFile)
		}
	}()

	email := "wire-persist-" + suffix + "@local.slan"
	password := "Password123!"
	var auth authResponse
	postJSON(bizURL+"/auth/register", "", map[string]any{
		"email":    email,
		"password": password,
	}, &auth)
	if auth.AccessToken == "" {
		fail("missing access token")
	}
	defer func() {
		if !seedCompleted {
			cleanupSmokeDeviceBestEffort(bizURL, email, password, deviceID)
		}
	}()

	postJSON(bizURL+"/devices/register", auth.AccessToken, map[string]any{
		"deviceId":    deviceID,
		"name":        "wire persistence device",
		"platform":    "smoke",
		"countryCode": "CN",
		"publicKey":   "device-public-key-" + suffix,
	}, nil)

	var home networkHomeResponse
	getJSON(bizURL+"/networks/home", auth.AccessToken, &home)
	networkID := ""
	if home.ActiveNetwork != nil {
		networkID = home.ActiveNetwork.NetworkID
	}
	if networkID == "" && home.OwnedNetwork != nil {
		networkID = home.OwnedNetwork.NetworkID
	}
	if networkID == "" {
		fail("missing network")
	}

	postJSON(bizURL+"/networks/"+networkID+"/activate", auth.AccessToken, map[string]any{"deviceId": deviceID}, nil)
	postJSON(bizURL+"/nodes/register", auth.AccessToken, map[string]any{
		"deviceId":      deviceID,
		"nodeId":        nodeID,
		"nodePublicKey": "node-public-key-" + suffix,
		"capabilities":  []string{"wireguard", "relay_udp", "derp_tcp_tls_443"},
	}, nil)

	var reg wireRegisterResponse
	postJSON(wireURL+"/v1/peers/register", "", map[string]any{
		"peer": map[string]any{
			"peerId":                nodeID,
			"networkId":             "client-forged-network",
			"nodeId":                "client-forged-node",
			"publicKey":             "wire-public-key-" + suffix,
			"supportsDirectUdp":     true,
			"supportsRelayUdp":      true,
			"supportsDerpTcpTls443": true,
		},
	}, &reg)
	if reg.Peer.PeerID != nodeID || reg.Peer.NetworkID != networkID || len(reg.Peer.VirtualIPs) == 0 {
		fail("unexpected wire register response: %+v", reg.Peer)
	}

	postJSON(wireURL+"/v1/peers/path-health", "", map[string]any{
		"peerId": nodeID,
		"probes": []map[string]any{
			{"path": "direct_udp", "reachable": true, "rttMs": 10, "mtu": 1420},
			{"path": "relay_udp", "reachable": true, "rttMs": 50, "mtu": 1280},
		},
	}, nil)
	postJSON(wireURL+"/v1/peers/active-path", "", map[string]any{"peerId": nodeID, "path": "direct_udp"}, nil)

	putJSONWithInternalToken(bizURL+"/internal/wire/admin/derp-nodes", internalToken, map[string]any{
		"regionId":          regionID,
		"nodeId":            derpNodeID,
		"name":              "Smoke Region",
		"host":              "derp-smoke.local",
		"port":              443,
		"healthy":           true,
		"priority":          1,
		"ticketKeyRotation": derpTicketKey,
	}, nil)
	postJSONWithInternalToken(bizURL+"/internal/wire/admin/derp-nodes/"+regionID+"/"+derpNodeID+"/heartbeat", internalToken, map[string]any{"healthy": true, "ticketKeyRotation": derpTicketKey}, nil)
	putJSONWithInternalToken(bizURL+"/internal/wire/admin/relay-nodes", internalToken, map[string]any{
		"regionId":          regionID,
		"nodeId":            relayNodeID,
		"host":              "relay-smoke.local",
		"udpPort":           29110,
		"adminPort":         29111,
		"healthy":           true,
		"priority":          1,
		"ticketKeyRotation": relayTicketKey,
	}, nil)
	postJSONWithInternalToken(bizURL+"/internal/wire/admin/relay-nodes/"+regionID+"/"+relayNodeID+"/heartbeat", internalToken, map[string]any{"healthy": true, "ticketKeyRotation": relayTicketKey}, nil)

	postJSON(wireURL+"/v1/peers/derp-health", "", map[string]any{
		"peerId": nodeID,
		"samples": []map[string]any{
			{"regionId": regionID, "nodeId": derpNodeID, "reachable": true, "rttMs": 35},
		},
	}, nil)
	var relayResp relayTicketResponse
	postJSON(wireURL+"/v1/relay/tickets", "", map[string]any{"peerId": nodeID, "ttlSeconds": 300, "renewAfterMs": 60000}, &relayResp)
	if relayResp.Ticket.NodeID != relayNodeID || relayResp.Ticket.SessionID == "" || relayResp.Ticket.Signature == "" {
		fail("unexpected relay ticket during seed: %+v", relayResp.Ticket)
	}

	writeState(persistenceState{
		PeerID:      nodeID,
		NetworkID:   networkID,
		Token:       auth.AccessToken,
		RegionID:    regionID,
		RelayNodeID: relayNodeID,
		DerpNodeID:  derpNodeID,
		DeviceID:    deviceID,
		UserEmail:   email,
	})
	seedCompleted = true
	fmt.Println("wire persistence seed passed; run cleanup after verify to disable smoke nodes")
}

func verify() {
	wireURL := env("SLAN_BIZ_E2E_WIRE_URL", "http://127.0.0.1:29100")
	waitHTTP(wireURL + "/healthz")
	state := readState()
	regionID, relayNodeID, derpNodeID := smokeIDs(state)

	var peerResp struct {
		Peer wirePeer `json:"peer"`
	}
	getJSON(wireURL+"/v1/peers/"+state.PeerID, "", &peerResp)
	if peerResp.Peer.PeerID != state.PeerID || peerResp.Peer.NetworkID != state.NetworkID || peerResp.Peer.ActivePath != "direct_udp" {
		fail("persisted peer not restored: %+v want peer=%s network=%s active=direct_udp", peerResp.Peer, state.PeerID, state.NetworkID)
	}
	if !hasProbe(peerResp.Peer.Probes, "direct_udp") || !hasProbe(peerResp.Peer.Probes, "relay_udp") {
		fail("persisted path probes not restored: %+v", peerResp.Peer.Probes)
	}
	if !hasDerpHealth(peerResp.Peer.DerpHealth, regionID, derpNodeID) {
		fail("persisted derp health not restored: %+v", peerResp.Peer.DerpHealth)
	}
	if peerResp.Peer.RelayTicket.NodeID != relayNodeID || peerResp.Peer.RelayTicket.SessionID == "" || peerResp.Peer.RelayTicket.Signature == "" {
		fail("persisted relay ticket not restored: %+v", peerResp.Peer.RelayTicket)
	}

	var runtime runtimeConfig
	getJSON(wireURL+"/v1/peers/"+state.PeerID+"/runtime-config", "", &runtime)
	if runtime.PeerID != state.PeerID || runtime.NetworkID != state.NetworkID || runtime.PreferredPath == "" {
		fail("runtime config not restored: %+v", runtime)
	}

	var plan pathPlan
	postJSON(wireURL+"/v1/path-plan", "", map[string]any{"peerId": state.PeerID}, &plan)
	if plan.PreferredPath == "" {
		fail("path plan not restored: %+v", plan)
	}
	if !hasRelayCandidate(plan.RelayCandidates, relayNodeID) {
		fail("path plan did not restore biz relay candidate: %+v", plan.RelayCandidates)
	}
	if !hasDerpCandidate(plan.DerpCandidates, regionID, derpNodeID) {
		fail("path plan did not restore biz derp candidate: %+v", plan.DerpCandidates)
	}

	var derp derpMapResponse
	getJSON(wireURL+"/v1/derp/map", "", &derp)
	if !hasDerpNode(derp.Map, regionID, derpNodeID) {
		fail("server-wire did not load biz derp map: %+v", derp.Map)
	}
	fmt.Println("wire persistence verify passed")
}

func cleanup() {
	bizURL := env("SLAN_BIZ_E2E_BIZ_URL", "http://127.0.0.1:28080")
	internalToken := env("SLAN_INTERNAL_WIRE_TOKEN", "change-me-wire-internal-token")
	waitHTTP(bizURL + "/healthz")
	if _, err := os.Stat(stateFile); err == nil {
		state := readState()
		regionID, relayNodeID, derpNodeID := smokeIDs(state)
		defer cleanupSmokeDeviceBestEffort(bizURL, state.UserEmail, "Password123!", state.DeviceID)
		disableSmokeNodes(bizURL, internalToken, regionID, relayNodeID, derpNodeID)
		must(os.Remove(stateFile))
	}
	disableSmokeNodes(bizURL, internalToken, smokeRegionID, "relay-smoke", "derp-smoke")
	fmt.Println("wire persistence cleanup passed")
}

func hasProbe(probes []probe, path string) bool {
	for _, probe := range probes {
		if probe.Path == path && probe.Reachable {
			return true
		}
	}
	return false
}

func hasDerpHealth(samples []derpHealthSample, regionID, nodeID string) bool {
	for _, sample := range samples {
		if sample.RegionID == regionID && sample.NodeID == nodeID && sample.Reachable {
			return true
		}
	}
	return false
}

func hasRelayCandidate(nodes []relayNode, nodeID string) bool {
	for _, node := range nodes {
		if node.NodeID == nodeID {
			return true
		}
	}
	return false
}

func hasDerpCandidate(nodes []derpNode, regionID, nodeID string) bool {
	for _, node := range nodes {
		if node.RegionID == regionID && node.NodeID == nodeID {
			return true
		}
	}
	return false
}

func hasDerpNode(m derpMap, regionID, nodeID string) bool {
	for _, region := range m.Regions {
		if region.RegionID != regionID {
			continue
		}
		for _, node := range region.Nodes {
			if node.NodeID == nodeID {
				return true
			}
		}
	}
	return false
}

func smokeIDs(state persistenceState) (string, string, string) {
	return defaultString(state.RegionID, smokeRegionID),
		defaultString(state.RelayNodeID, "relay-smoke"),
		defaultString(state.DerpNodeID, "derp-smoke")
}

func disableSmokeNodes(bizURL, internalToken, regionID, relayNodeID, derpNodeID string) {
	payload := map[string]any{"enabled": false, "healthy": false}
	patchJSONWithInternalToken(bizURL+"/internal/wire/admin/relay-nodes/"+regionID+"/"+relayNodeID+"/status", internalToken, payload, nil)
	patchJSONWithInternalToken(bizURL+"/internal/wire/admin/derp-nodes/"+regionID+"/"+derpNodeID+"/status", internalToken, payload, nil)
	deleteWithInternalToken(bizURL+"/internal/wire/admin/relay-nodes/"+regionID+"/"+relayNodeID, internalToken)
	deleteWithInternalToken(bizURL+"/internal/wire/admin/derp-nodes/"+regionID+"/"+derpNodeID, internalToken)
}

func disableSmokeNodesBestEffort(bizURL, internalToken, regionID, relayNodeID, derpNodeID string) {
	defer func() {
		_ = recover()
	}()
	disableSmokeNodes(bizURL, internalToken, regionID, relayNodeID, derpNodeID)
}

func cleanupSmokeDeviceBestEffort(bizURL, email, password, deviceID string) {
	if email == "" || password == "" || deviceID == "" {
		return
	}
	defer func() {
		_ = recover()
	}()
	client := &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: 30 * time.Second}
	var auth cleanupAuthResponse
	payload, err := json.Marshal(map[string]any{"email": email, "password": password})
	must(err)
	req, err := http.NewRequest(http.MethodPost, bizURL+"/api/auth/login", bytes.NewReader(payload))
	must(err)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	must(err)
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return
	}
	must(json.Unmarshal(body, &auth))
	userID := auth.Auth.User.UserID
	if userID == "" {
		return
	}
	req, err = http.NewRequest(
		http.MethodDelete,
		bizURL+"/api/devices/"+url.PathEscape(deviceID)+"?actorUserId="+url.QueryEscape(userID),
		nil,
	)
	must(err)
	resp, err = client.Do(req)
	if err == nil {
		_ = resp.Body.Close()
	}
}

func waitHTTP(url string) {
	deadline := time.Now().Add(30 * time.Second)
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

func getJSON(url, token string, out any) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	must(err)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	doJSON(req, out)
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
	doJSON(req, out)
}

func putJSONWithInternalToken(url, token string, in, out any) {
	payload, err := json.Marshal(in)
	must(err)
	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(payload))
	must(err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Slan-Internal-Token", token)
	doJSON(req, out)
}

func postJSONWithInternalToken(url, token string, in, out any) {
	payload, err := json.Marshal(in)
	must(err)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	must(err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Slan-Internal-Token", token)
	doJSON(req, out)
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
	if resp.StatusCode == http.StatusNotFound {
		return
	}
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		fail("%s %s status=%d body=%s", req.Method, req.URL.String(), resp.StatusCode, string(body))
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
		fail("%s %s status=%d body=%s", req.Method, req.URL.String(), resp.StatusCode, string(body))
	}
}

func doJSON(req *http.Request, out any) {
	resp, err := httpClient.Do(req)
	must(err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		fail("%s %s status=%d body=%s", req.Method, req.URL.String(), resp.StatusCode, string(body))
	}
	if out != nil {
		must(json.Unmarshal(body, out))
	}
}

func writeState(state persistenceState) {
	payload, err := json.Marshal(state)
	must(err)
	must(os.WriteFile(stateFile, payload, 0o600))
}

func readState() persistenceState {
	payload, err := os.ReadFile(stateFile)
	must(err)
	var state persistenceState
	must(json.Unmarshal(payload, &state))
	if state.PeerID == "" || state.NetworkID == "" {
		fail("invalid persistence state: %+v", state)
	}
	return state
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func defaultString(value, fallback string) string {
	if value != "" {
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
