package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

var staleHTTPClient = &http.Client{
	Transport: &http.Transport{Proxy: nil},
	Timeout:   5 * time.Second,
}

type staleAuthResponse struct {
	AccessToken string `json:"accessToken"`
}

type staleCleanupAuthResponse struct {
	Auth struct {
		User struct {
			UserID string `json:"userId"`
		} `json:"user"`
	} `json:"auth"`
}

type staleDeviceResponse struct {
	DeviceID string `json:"deviceId"`
}

type staleNetworkHomeResponse struct {
	ActiveNetwork *staleNetworkResponse `json:"activeNetwork,omitempty"`
	OwnedNetwork  *staleNetworkResponse `json:"ownedNetwork,omitempty"`
}

type staleNetworkResponse struct {
	NetworkID string `json:"networkId"`
}

type staleActivationResponse struct {
	Attachment struct {
		AttachmentID string `json:"attachmentId"`
		NetworkID    string `json:"networkId"`
		DeviceID     string `json:"deviceId"`
	} `json:"attachment"`
}

type staleNodeResponse struct {
	NodeID string `json:"nodeId"`
}

type staleWireRegisterResponse struct {
	Peer struct {
		PeerID    string `json:"peerId"`
		NetworkID string `json:"networkId"`
		NodeID    string `json:"nodeId"`
	} `json:"peer"`
}

type stalePathPlan struct {
	RelayCandidates []staleRelayNode `json:"relayCandidates,omitempty"`
	DerpCandidates  []staleDerpNode  `json:"derpCandidates,omitempty"`
}

type staleRelayNode struct {
	RegionID string `json:"regionId"`
	NodeID   string `json:"nodeId"`
	Host     string `json:"host"`
	UDPPort  int    `json:"udpPort"`
	Enabled  bool   `json:"enabled"`
	Healthy  bool   `json:"healthy"`
	Stale    bool   `json:"stale"`
}

type staleDerpNode struct {
	RegionID string `json:"regionId"`
	NodeID   string `json:"nodeId"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Enabled  bool   `json:"enabled"`
	Healthy  bool   `json:"healthy"`
	Stale    bool   `json:"stale"`
}

type staleDerpMap struct {
	Regions []struct {
		RegionID string          `json:"regionId"`
		Nodes    []staleDerpNode `json:"nodes"`
	} `json:"regions"`
}

type staleAuthorizedPeer struct {
	NodeID   string
	Email    string
	Password string
	DeviceID string
}

func main() {
	if err := runStaleSmoke(); err != nil {
		fmt.Fprintf(os.Stderr, "wire stale nodes smoke failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("wire stale nodes smoke passed")
}

func runStaleSmoke() error {
	bizURL := env("SLAN_BIZ_E2E_BIZ_URL", "http://127.0.0.1:28080")
	wireURL := env("SLAN_BIZ_E2E_WIRE_URL", "http://127.0.0.1:29100")
	relayBAdminURL := env("SLAN_BIZ_E2E_RELAY_B_ADMIN_URL", "http://127.0.0.1:29113")
	derpBAdminURL := env("SLAN_BIZ_E2E_DERP_B_ADMIN_URL", "http://127.0.0.1:29123")
	internalToken := env("SLAN_INTERNAL_WIRE_TOKEN", "change-me-wire-internal-token")

	if err := compose([]string{"up", "-d", "server-wire-relay-b", "server-wire-derp-b"}, nil); err != nil {
		return err
	}
	if err := restartBizWithWireTTL("3", "1"); err != nil {
		return err
	}
	defer func() {
		_ = compose([]string{"up", "-d", "server-wire-relay-b", "server-wire-derp-b"}, nil)
		_ = restartBizWithWireTTL("120", "60")
		_ = waitHTTP(bizURL + "/healthz")
		if shouldRestoreWireDataPlaneNodes(bizURL) {
			restoreLocalWireDataPlaneNodes(bizURL, internalToken)
		}
	}()

	if err := waitHTTP(bizURL + "/healthz"); err != nil {
		return err
	}
	if err := waitHTTP(wireURL + "/healthz"); err != nil {
		return err
	}
	if err := waitHTTP(relayBAdminURL + "/healthz"); err != nil {
		return err
	}
	if err := waitHTTP(derpBAdminURL + "/healthz"); err != nil {
		return err
	}
	if shouldRestoreWireDataPlaneNodes(bizURL) {
		restoreLocalWireDataPlaneNodes(bizURL, internalToken)
	}

	peer, err := createAuthorizedWirePeer(bizURL, wireURL)
	if err != nil {
		return err
	}
	defer cleanupStaleSmokeDeviceBestEffort(bizURL, peer.Email, peer.Password, peer.DeviceID)
	if err := waitPathPlanContainsB(wireURL, peer.NodeID); err != nil {
		return err
	}

	if err := compose([]string{"stop", "server-wire-relay-b", "server-wire-derp-b"}, nil); err != nil {
		return err
	}
	return waitBNodesPrunedFromScheduling(bizURL, wireURL, internalToken, peer.NodeID)
}

func restartBizWithWireTTL(freshnessSeconds, cleanupSeconds string) error {
	return compose([]string{"up", "-d", "--no-deps", "--build", "server-biz"}, map[string]string{
		"SLAN_WIRE_NODE_HEARTBEAT_FRESHNESS_SECONDS": freshnessSeconds,
		"SLAN_WIRE_NODE_CLEANUP_INTERVAL_SECONDS":    cleanupSeconds,
	})
}

func compose(args []string, env map[string]string) error {
	if os.Getenv("SLAN_ALLOW_LOCAL_DOCKER") != "1" {
		return fmt.Errorf("local Docker smoke compose is disabled; set SLAN_ALLOW_LOCAL_DOCKER=1 for one-off local debugging")
	}
	contextName := os.Getenv("SLAN_LOCAL_DOCKER_CONTEXT")
	if contextName == "" {
		contextName = "desktop-linux"
	}
	fullArgs := append([]string{"--context", contextName, "compose", "-f", "docker-compose.local.yml"}, args...)
	cmd := exec.Command("docker", fullArgs...)
	cmd.Env = os.Environ()
	for key, value := range env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker %s: %w\n%s", strings.Join(fullArgs, " "), err, string(output))
	}
	return nil
}

func createAuthorizedWirePeer(bizURL, wireURL string) (staleAuthorizedPeer, error) {
	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	email := "wire-stale-" + suffix + "@local.slan"
	password := "Password123!"
	deviceID := "dev-wire-stale-" + suffix
	nodeID := "node-wire-stale-" + suffix
	cleanupOnFailure := true
	defer func() {
		if cleanupOnFailure {
			cleanupStaleSmokeDeviceBestEffort(bizURL, email, password, deviceID)
		}
	}()

	var auth staleAuthResponse
	if err := postJSON(bizURL+"/auth/register", "", map[string]any{"email": email, "password": password}, &auth); err != nil {
		return staleAuthorizedPeer{}, err
	}
	var device staleDeviceResponse
	if err := postJSON(bizURL+"/devices/register", auth.AccessToken, map[string]any{
		"deviceId":    deviceID,
		"name":        "wire stale smoke device",
		"platform":    "smoke",
		"countryCode": "CN",
		"publicKey":   "device-public-key-" + suffix,
	}, &device); err != nil {
		return staleAuthorizedPeer{}, err
	}
	if device.DeviceID != deviceID {
		return staleAuthorizedPeer{}, fmt.Errorf("unexpected device response: %+v", device)
	}
	var home staleNetworkHomeResponse
	if err := getJSON(bizURL+"/networks/home", auth.AccessToken, &home); err != nil {
		return staleAuthorizedPeer{}, err
	}
	networkID := ""
	if home.ActiveNetwork != nil {
		networkID = home.ActiveNetwork.NetworkID
	}
	if networkID == "" && home.OwnedNetwork != nil {
		networkID = home.OwnedNetwork.NetworkID
	}
	if networkID == "" {
		return staleAuthorizedPeer{}, fmt.Errorf("missing network from home: %+v", home)
	}
	var activation staleActivationResponse
	if err := postJSON(bizURL+"/networks/"+networkID+"/activate", auth.AccessToken, map[string]any{"deviceId": deviceID}, &activation); err != nil {
		return staleAuthorizedPeer{}, err
	}
	var node staleNodeResponse
	if err := postJSON(bizURL+"/nodes/register", auth.AccessToken, map[string]any{
		"deviceId":      deviceID,
		"nodeId":        nodeID,
		"nodePublicKey": "node-public-key-" + suffix,
		"capabilities":  []string{"wireguard", "relay_udp", "derp_tcp_tls_443"},
	}, &node); err != nil {
		return staleAuthorizedPeer{}, err
	}
	if node.NodeID != nodeID {
		return staleAuthorizedPeer{}, fmt.Errorf("unexpected node response: %+v", node)
	}
	var reg staleWireRegisterResponse
	if err := postJSON(wireURL+"/v1/peers/register", "", map[string]any{
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
	}, &reg); err != nil {
		return staleAuthorizedPeer{}, err
	}
	if reg.Peer.NetworkID != networkID || reg.Peer.NodeID != nodeID {
		return staleAuthorizedPeer{}, fmt.Errorf("wire did not apply biz authz: %+v", reg.Peer)
	}
	cleanupOnFailure = false
	return staleAuthorizedPeer{NodeID: nodeID, Email: email, Password: password, DeviceID: deviceID}, nil
}

func cleanupStaleSmokeDeviceBestEffort(bizURL, email, password, deviceID string) {
	if email == "" || password == "" || deviceID == "" {
		return
	}
	client := &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: 30 * time.Second}
	var auth staleCleanupAuthResponse
	payload, err := json.Marshal(map[string]any{"email": email, "password": password})
	if err != nil {
		return
	}
	req, err := http.NewRequest(http.MethodPost, bizURL+"/api/auth/login", bytes.NewReader(payload))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return
	}
	if err := json.Unmarshal(body, &auth); err != nil {
		return
	}
	userID := auth.Auth.User.UserID
	if userID == "" {
		return
	}
	req, err = http.NewRequest(
		http.MethodDelete,
		bizURL+"/api/devices/"+url.PathEscape(deviceID)+"?actorUserId="+url.QueryEscape(userID),
		nil,
	)
	if err != nil {
		return
	}
	resp, err = client.Do(req)
	if err == nil {
		_ = resp.Body.Close()
	}
}

func waitPathPlanContainsB(wireURL, nodeID string) error {
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		plan, err := pathPlan(wireURL, nodeID)
		if err == nil && containsRelayNode(plan.RelayCandidates, "relay-local-b") && containsDerpNode(plan.DerpCandidates, "derp-local-b") {
			return nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for path plan to include relay-local-b and derp-local-b")
}

func waitBNodesPrunedFromScheduling(bizURL, wireURL, internalToken, nodeID string) error {
	deadline := time.Now().Add(25 * time.Second)
	for time.Now().Before(deadline) {
		refreshPrimaryLocalWireDataPlaneNodes(bizURL, internalToken)
		relayB, relayFound, relayErr := relayNodeRecord(bizURL, internalToken, "relay-local-b")
		derpB, derpFound, derpErr := derpNodeRecord(bizURL, internalToken, "derp-local-b")
		derpMap, mapErr := derpMap(bizURL, internalToken)
		plan, planErr := pathPlan(wireURL, nodeID)
		if relayErr == nil && derpErr == nil && mapErr == nil && planErr == nil &&
			relayFound && derpFound &&
			(relayB.Stale || !relayB.Healthy) &&
			(derpB.Stale || !derpB.Healthy) &&
			!derpMapContainsNode(derpMap, "derp-local-b") &&
			!containsRelayNode(plan.RelayCandidates, "relay-local-b") &&
			!containsDerpNode(plan.DerpCandidates, "derp-local-b") &&
			containsRelayNode(plan.RelayCandidates, "relay-local") &&
			containsDerpNode(plan.DerpCandidates, "derp-local") {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for relay-local-b/derp-local-b to become stale and leave scheduling")
}

func refreshPrimaryLocalWireDataPlaneNodes(bizURL, internalToken string) {
	_ = patchJSONWithInternalToken(
		bizURL+"/internal/wire/admin/relay-nodes/local/relay-local/status",
		internalToken,
		map[string]any{"enabled": true, "healthy": true},
		nil,
	)
	_ = patchJSONWithInternalToken(
		bizURL+"/internal/wire/admin/derp-nodes/local/derp-local/status",
		internalToken,
		map[string]any{"enabled": true, "healthy": true},
		nil,
	)
}

func relayNodeRecord(bizURL, internalToken, nodeID string) (staleRelayNode, bool, error) {
	var resp struct {
		Items []staleRelayNode `json:"items"`
	}
	if err := getJSONWithHeader(bizURL+"/internal/wire/admin/relay-nodes", map[string]string{"X-Slan-Internal-Token": internalToken}, &resp); err != nil {
		return staleRelayNode{}, false, err
	}
	for _, node := range resp.Items {
		if node.NodeID == nodeID {
			return node, true, nil
		}
	}
	return staleRelayNode{}, false, nil
}

func derpNodeRecord(bizURL, internalToken, nodeID string) (staleDerpNode, bool, error) {
	var resp struct {
		Items []staleDerpNode `json:"items"`
	}
	if err := getJSONWithHeader(bizURL+"/internal/wire/admin/derp-nodes", map[string]string{"X-Slan-Internal-Token": internalToken}, &resp); err != nil {
		return staleDerpNode{}, false, err
	}
	for _, node := range resp.Items {
		if node.NodeID == nodeID {
			return node, true, nil
		}
	}
	return staleDerpNode{}, false, nil
}

func derpMap(bizURL, internalToken string) (staleDerpMap, error) {
	var out staleDerpMap
	err := getJSONWithHeader(bizURL+"/internal/wire/derp-map", map[string]string{"X-Slan-Internal-Token": internalToken}, &out)
	return out, err
}

func pathPlan(wireURL, nodeID string) (stalePathPlan, error) {
	var out stalePathPlan
	err := postJSON(wireURL+"/v1/path-plan", "", map[string]any{"peerId": nodeID}, &out)
	return out, err
}

func restoreLocalWireDataPlaneNodes(bizURL, internalToken string) {
	for _, nodeID := range []string{"relay-local", "relay-local-b"} {
		_ = patchJSONWithInternalToken(
			bizURL+"/internal/wire/admin/relay-nodes/local/"+nodeID+"/status",
			internalToken,
			map[string]any{"enabled": true, "healthy": true},
			nil,
		)
	}
	for _, nodeID := range []string{"derp-local", "derp-local-b"} {
		_ = patchJSONWithInternalToken(
			bizURL+"/internal/wire/admin/derp-nodes/local/"+nodeID+"/status",
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

func waitHTTP(url string) error {
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := staleHTTPClient.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for %s", url)
}

func getJSON(url, token string, out any) error {
	headers := map[string]string{}
	if token != "" {
		headers["Authorization"] = "Bearer " + token
	}
	return getJSONWithHeader(url, headers, out)
}

func getJSONWithHeader(url string, headers map[string]string, out any) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := staleHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("GET %s status=%d body=%s", url, resp.StatusCode, string(body))
	}
	return json.Unmarshal(body, out)
}

func postJSON(url, token string, in, out any) error {
	payload, err := json.Marshal(in)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := staleHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("POST %s status=%d body=%s", url, resp.StatusCode, string(body))
	}
	if out != nil {
		return json.Unmarshal(body, out)
	}
	return nil
}

func patchJSONWithInternalToken(url, token string, in, out any) error {
	payload, err := json.Marshal(in)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPatch, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Slan-Internal-Token", token)
	resp, err := staleHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("PATCH %s status=%d body=%s", url, resp.StatusCode, string(body))
	}
	if out != nil {
		return json.Unmarshal(body, out)
	}
	return nil
}

func containsRelayNode(nodes []staleRelayNode, nodeID string) bool {
	for _, node := range nodes {
		if node.NodeID == nodeID {
			return true
		}
	}
	return false
}

func containsDerpNode(nodes []staleDerpNode, nodeID string) bool {
	for _, node := range nodes {
		if node.NodeID == nodeID {
			return true
		}
	}
	return false
}

func derpMapContainsNode(m staleDerpMap, nodeID string) bool {
	for _, region := range m.Regions {
		if containsDerpNode(region.Nodes, nodeID) {
			return true
		}
	}
	return false
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
