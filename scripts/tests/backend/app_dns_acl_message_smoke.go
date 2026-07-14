package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type appSmokeClient struct {
	name     string
	address  string
	deviceID string
	cmd      *exec.Cmd
	logFile  *os.File
}

type authEnvelope struct {
	Auth struct {
		User struct {
			UserID string `json:"userId"`
		} `json:"user"`
		Session struct {
			Token string `json:"token"`
		} `json:"session"`
	} `json:"auth"`
	DefaultNetwork struct {
		NetworkID string `json:"networkId"`
	} `json:"defaultNetwork"`
}

type itemsEnvelope[T any] struct {
	Items []T `json:"items"`
}

type networkRow struct {
	NetworkID string `json:"networkId"`
}

type securityGroupRow struct {
	SecurityGroupID string `json:"securityGroupId"`
}

type dnsZoneRow struct {
	ZoneID string `json:"zoneId"`
}

type dnsRecordRow struct {
	RecordID string `json:"recordId"`
}

type securityRuleRow struct {
	RuleID string `json:"ruleId"`
}

type provisionedResources struct {
	zoneIDs   []string
	recordIDs []string
	ruleIDs   []string
}

type networkModuleSnapshot struct {
	NetworkCount        int `json:"networkCount"`
	PeerCount           int `json:"peerCount"`
	ResolverRecordCount int `json:"resolverRecordCount"`
	SecurityRuleCount   int `json:"securityRuleCount"`
}

type mqttCredentialEnvelope struct {
	MQTT struct {
		BrokerURL string `json:"brokerUrl"`
	} `json:"mqtt"`
}

type smokeFailure struct {
	message string
}

var httpClient = &http.Client{Timeout: 30 * time.Second}

func main() {
	defer exitOnFailure()

	var bizURL string
	var webBaseURL string
	var serviceBin string
	var password string
	var expectMQTTHost string
	var checkMessages bool
	var clientCount int
	var timeout time.Duration
	flag.StringVar(&bizURL, "biz-url", envDefault("SLAN_BIZ_URL", "http://127.0.0.1:28080"), "service-biz base URL")
	flag.StringVar(&webBaseURL, "web-base-url", envDefault("SLAN_WEB_BASE_URL", strings.TrimRight(envDefault("SLAN_BIZ_WEB_BASE_URL", "http://127.0.0.1:28081"), "/")), "web console base URL")
	flag.StringVar(&serviceBin, "service-bin", envDefault("SLAN_CLIENT_CORE_SERVICE_BIN", "client_v2/rust/target/release/client-core-service"), "client-core-service binary")
	flag.StringVar(&password, "password", "Password123!", "test user password")
	flag.StringVar(&expectMQTTHost, "expect-mqtt-host", envDefault("SLAN_EXPECT_MQTT_HOST", ""), "expected public MQTT broker host")
	flag.BoolVar(&checkMessages, "check-messages", true, "also verify MQTT client_message delivery")
	flag.IntVar(&clientCount, "clients", 2, "number of logical app clients to start")
	flag.DurationVar(&timeout, "timeout", 90*time.Second, "integration timeout")
	flag.Parse()
	if clientCount < 2 {
		fail("-clients must be >= 2")
	}

	bizURL = strings.TrimRight(bizURL, "/")
	webBaseURL = strings.TrimRight(webBaseURL, "/")
	serviceBin = filepath.Clean(serviceBin)
	if _, err := os.Stat(serviceBin); err != nil {
		fail("client-core-service binary is not available at %s: %v\nrun: cd client_v2/rust && cargo build -p client-core-service", serviceBin, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	email := fmt.Sprintf("app-dns-acl-message-%d@example.test", time.Now().UnixNano())
	auth := register(ctx, bizURL, email, password)
	userID := auth.Auth.User.UserID
	userToken := auth.Auth.Session.Token
	networkID := auth.DefaultNetwork.NetworkID
	if networkID == "" {
		networkID = firstNetworkID(ctx, webBaseURL, auth.Auth.User.UserID)
	}
	if networkID == "" {
		fail("registered user has no default network")
	}
	resources := &provisionedResources{}

	root := filepath.Join(os.TempDir(), "slan-app-dns-acl-message-"+uniqueSuffix())
	if os.Getenv("SLAN_KEEP_APP_DNS_ACL_WORK_DIR") == "" {
		defer os.RemoveAll(root)
	} else {
		fmt.Printf("appDnsAclMessageSmoke: workDir=%s\n", root)
	}
	clients := make([]*appSmokeClient, 0, clientCount)
	defer cleanupIntegration(webBaseURL, &networkID, userID, resources, &clients)
	for index := 0; index < clientCount; index++ {
		name := fmt.Sprintf("app-%c", 'a'+rune(index))
		deviceID := fmt.Sprintf("app-smoke-%c-%s", 'a'+rune(index), uniqueSuffix())
		clients = append(clients, startService(ctx, serviceBin, bizURL, filepath.Join(root, name), deviceID, name))
	}
	defer func() {
		for _, client := range clients {
			client.stop()
		}
	}()

	for _, client := range clients {
		login(ctx, client, email, password)
		assertMQTTBrokerHost(ctx, bizURL, client.deviceID, expectMQTTHost)
		waitControlReady(ctx, client)
	}

	provisionDNSAndACL(ctx, webBaseURL, networkID, clients, resources)
	for _, client := range clients {
		assertAppNetworkConfig(ctx, bizURL, userToken, networkID, client.deviceID, clientCount)
	}

	minPeers := clientCount - 1
	minResolverRecords := clientCount
	minSecurityRules := clientCount * (clientCount - 1) * 2
	for _, client := range clients {
		waitModule(ctx, client, minPeers, minResolverRecords, minSecurityRules)
	}

	if checkMessages {
		for _, client := range clients {
			enableNetwork(ctx, client)
		}
		time.Sleep(6 * time.Second)
		for _, from := range clients {
			for _, target := range clients {
				if from == target {
					continue
				}
				body := from.name + "-to-" + target.name + "-" + uniqueSuffix()
				sendClientMessage(ctx, from, target.deviceID, body)
				waitClientMessage(ctx, target, from.deviceID, body)
			}
		}
	}

	deviceIDs := make([]string, 0, len(clients))
	for _, client := range clients {
		deviceIDs = append(deviceIDs, client.name+"="+client.deviceID)
	}
	fmt.Printf("appDnsAclMessageSmoke: ok email=%s network=%s clients=%s\n", email, networkID, strings.Join(deviceIDs, ","))
}

func startService(ctx context.Context, serviceBin, bizURL, stateDir, deviceID, name string) *appSmokeClient {
	if err := os.MkdirAll(filepath.Join(stateDir, "SLAN"), 0o755); err != nil {
		fail("create %s state dir: %v", name, err)
	}
	address := freeLocalAddress()
	logFilePath := filepath.Join(stateDir, "SLAN", "client-core-service.log")
	logFile, err := os.Create(logFilePath)
	if err != nil {
		fail("create %s log: %v", name, err)
	}
	cmd := exec.CommandContext(ctx, serviceBin)
	cmd.Env = append(os.Environ(),
		"SLAN_CONTROL_BASE_URL="+bizURL,
		"SLAN_CLIENT_CORE_SERVICE_HOST="+address,
		"SLAN_CLIENT_DEVICE_ID="+deviceID,
		"SLAN_MACOS_NETWORK_MOCK=1",
		"SLAN_STATE_DIR="+stateDir,
	)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		fail("start %s service: %v", name, err)
	}
	client := &appSmokeClient{name: name, address: address, deviceID: deviceID, cmd: cmd, logFile: logFile}
	waitServiceReady(ctx, client)
	return client
}

func (client *appSmokeClient) stop() {
	if client.cmd != nil && client.cmd.Process != nil {
		_ = client.cmd.Process.Kill()
		_, _ = client.cmd.Process.Wait()
	}
	if client.logFile != nil {
		_ = client.logFile.Close()
	}
}

func assertAppNetworkConfig(ctx context.Context, bizURL, userToken, networkID, deviceID string, clientCount int) {
	var out map[string]any
	getAuthorizedJSON(
		ctx,
		bizURL+"/api/app/networks/"+url.PathEscape(networkID)+"/network-config?deviceId="+url.QueryEscape(deviceID),
		userToken,
		&out,
	)
	resolverRecords := jsonArrayLen(out["resolverRecords"])
	securityRules := jsonArrayLen(out["securityRules"])
	if securityRules == 0 {
		securityRules = jsonArrayLen(out["rules"])
	}
	if securityRules == 0 {
		securityRules = jsonIntValue(out["securityRuleCount"])
	}
	peers := jsonArrayLen(out["peers"])
	if resolverRecords < clientCount {
		fail("app network-config resolverRecords too small for %s: got=%d want>=%d payload=%#v", deviceID, resolverRecords, clientCount, out)
	}
	if securityRules < clientCount*2-2 {
		fail("app network-config securityRules too small for %s: got=%d payload=%#v", deviceID, securityRules, out)
	}
	if peers < clientCount-1 {
		fail("app network-config peers too small for %s: got=%d want>=%d payload=%#v", deviceID, peers, clientCount-1, out)
	}
	fmt.Printf("appDnsAclMessageSmoke: networkConfig device=%s peers=%d resolverRecords=%d securityRules=%d\n", deviceID, peers, resolverRecords, securityRules)
}

func jsonArrayLen(value any) int {
	switch typed := value.(type) {
	case []any:
		return len(typed)
	case []map[string]any:
		return len(typed)
	default:
		return 0
	}
}

func jsonIntValue(value any) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case float32:
		return int(typed)
	case int:
		return typed
	case int64:
		return int(typed)
	default:
		return 0
	}
}

func waitServiceReady(ctx context.Context, client *appSmokeClient) {
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := localRequest(client.address, "localStatus", map[string]any{}, 800*time.Millisecond); err == nil {
			return
		}
		select {
		case <-ctx.Done():
			fail("wait %s service ready: %v", client.name, ctx.Err())
		case <-time.After(200 * time.Millisecond):
		}
	}
	fail("%s service did not become ready at %s", client.name, client.address)
}

func waitControlReady(ctx context.Context, client *appSmokeClient) {
	deadline := time.Now().Add(15 * time.Second)
	var last map[string]any
	for time.Now().Before(deadline) {
		response, err := localRequest(client.address, "localControlStatus", map[string]any{}, 2*time.Second)
		if err == nil {
			last = response
			if response["ready"] == true {
				return
			}
		}
		select {
		case <-ctx.Done():
			fail("wait %s control ready: %v", client.name, ctx.Err())
		case <-time.After(300 * time.Millisecond):
		}
	}
	fail("%s control transport is not ready: %#v", client.name, last)
}

func provisionDNSAndACL(ctx context.Context, webBaseURL, networkID string, clients []*appSmokeClient, resources *provisionedResources) {
	var groups itemsEnvelope[securityGroupRow]
	getJSON(ctx, webBaseURL+"/api/web/networks/"+url.PathEscape(networkID)+"/security-groups", &groups)
	if len(groups.Items) == 0 {
		fail("network %s has no security groups", networkID)
	}
	zoneName := "app-smoke-" + uniqueSuffix() + ".lan"
	var zone dnsZoneRow
	postJSON(ctx, webBaseURL+"/api/web/networks/"+url.PathEscape(networkID)+"/dns/zones", map[string]any{
		"zoneName": zoneName,
	}, &zone)
	if zone.ZoneID == "" {
		fail("created dns zone returned empty zone id")
	}
	resources.zoneIDs = append(resources.zoneIDs, zone.ZoneID)
	for _, client := range clients {
		var record dnsRecordRow
		postJSON(ctx, webBaseURL+"/api/web/networks/"+url.PathEscape(networkID)+"/dns/records", map[string]any{
			"zoneId":         zone.ZoneID,
			"name":           client.name,
			"recordType":     "A",
			"targetDeviceId": client.deviceID,
			"port":           "443",
			"ttl":            60,
		}, &record)
		if record.RecordID != "" {
			resources.recordIDs = append(resources.recordIDs, record.RecordID)
		}
	}
	for _, from := range clients {
		for _, target := range clients {
			if from == target {
				continue
			}
			addRule(ctx, webBaseURL, groups.Items[0].SecurityGroupID, "ingress", "device", from.deviceID, 443, resources)
			addRule(ctx, webBaseURL, groups.Items[0].SecurityGroupID, "egress", "device", target.deviceID, 443, resources)
		}
	}
	deviceIDs := make([]string, 0, len(clients))
	for _, client := range clients {
		deviceIDs = append(deviceIDs, client.name+"="+client.deviceID)
	}
	fmt.Printf("appDnsAclMessageSmoke: provisioned dnsZone=%s clients=%s\n", zoneName, strings.Join(deviceIDs, ","))
}

func addRule(ctx context.Context, webBaseURL, securityGroupID, direction, peerType, peerValue string, port int, resources *provisionedResources) {
	var out securityRuleRow
	postJSON(ctx, webBaseURL+"/api/web/security-groups/"+url.PathEscape(securityGroupID)+"/rules", map[string]any{
		"direction": direction,
		"priority":  100,
		"action":    "allow",
		"protocol":  "tcp",
		"portFrom":  port,
		"portTo":    port,
		"peerType":  peerType,
		"peerValue": peerValue,
		"enabled":   true,
	}, &out)
	if out.RuleID != "" {
		resources.ruleIDs = append(resources.ruleIDs, out.RuleID)
	}
}

func waitModule(ctx context.Context, client *appSmokeClient, minPeers, minResolverRecords, minSecurityRules int) {
	deadline := time.Now().Add(25 * time.Second)
	var last networkModuleSnapshot
	var lastResponse map[string]any
	var lastRefreshErr error
	for time.Now().Before(deadline) {
		_, lastRefreshErr = localRequest(client.address, "localPlatformNetworkConfig", map[string]any{}, 5*time.Second)
		response, err := localRequest(client.address, "localNetworkModule", map[string]any{}, 2*time.Second)
		if err == nil {
			lastResponse = response
			payload, _ := json.Marshal(response)
			_ = json.Unmarshal(payload, &last)
			if last.PeerCount >= minPeers && last.ResolverRecordCount >= minResolverRecords && last.SecurityRuleCount >= minSecurityRules {
				fmt.Printf("appDnsAclMessageSmoke: module %s peers=%d resolverRecords=%d securityRules=%d\n", client.name, last.PeerCount, last.ResolverRecordCount, last.SecurityRuleCount)
				return
			}
		}
		select {
		case <-ctx.Done():
			fail("wait %s network module: %v", client.name, ctx.Err())
		case <-time.After(500 * time.Millisecond):
		}
	}
	fail("%s network module did not receive dns/acl config: %#v refreshErr=%v snapshot=%#v", client.name, last, lastRefreshErr, lastResponse)
}

func enableNetwork(ctx context.Context, client *appSmokeClient) {
	response, err := localRequest(client.address, "localNetworkActivate", map[string]any{}, 45*time.Second)
	if err != nil {
		fail("%s enable network request failed: %v", client.name, err)
	}
	if response["networkEnabled"] != true {
		fail("%s network did not enable: %#v", client.name, response)
	}
	if response["virtualIp"] == nil || strings.TrimSpace(fmt.Sprint(response["virtualIp"])) == "" {
		fail("%s network enabled without virtualIp: %#v", client.name, response)
	}
}

func sendClientMessage(ctx context.Context, from *appSmokeClient, targetDeviceID, body string) {
	response, err := localRequest(from.address, "localSendClientMessage", map[string]any{
		"targetDeviceId": targetDeviceID,
		"body":           body,
		"metadata":       map[string]any{"smoke": "app-dns-acl-message"},
	}, 8*time.Second)
	if err != nil {
		fail("%s send client message failed: %v", from.name, err)
	}
	if response["messageId"] == "" || response["messageId"] == nil {
		fail("%s send client message returned no messageId: %#v", from.name, response)
	}
	select {
	case <-ctx.Done():
		fail("send client message timeout: %v", ctx.Err())
	default:
	}
}

func waitClientMessage(ctx context.Context, target *appSmokeClient, fromDeviceID, body string) {
	var lastRevision float64
	deadline := time.Now().Add(30 * time.Second)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	var lastSnapshot map[string]any
	var lastWatchErr error
	for time.Now().Before(deadline) {
		response, err := localRequest(target.address, "localBusinessEventWatch", map[string]any{
			"lastRevision": lastRevision,
			"timeoutMs":    5000,
		}, 8*time.Second)
		if err != nil {
			lastWatchErr = err
			select {
			case <-ctx.Done():
				fail("wait client message timeout: %v", ctx.Err())
			case <-time.After(200 * time.Millisecond):
			}
			continue
		}
		if rev, ok := response["revision"].(float64); ok {
			lastRevision = rev
		}
		snapshot, _ := response["snapshot"].(map[string]any)
		lastSnapshot = snapshot
		if snapshot["lastClientMessageFromDeviceId"] == fromDeviceID && snapshot["lastClientMessageBody"] == body {
			fmt.Printf("appDnsAclMessageSmoke: delivered from=%s target=%s body=%s\n", fromDeviceID, target.deviceID, body)
			return
		}
		select {
		case <-ctx.Done():
			fail("wait client message timeout: %v", ctx.Err())
		default:
		}
	}
	status, _ := localRequest(target.address, "localControlStatus", map[string]any{}, 2*time.Second)
	fail("%s did not consume client_message from %s body=%s lastRevision=%.0f lastSnapshot=%#v lastWatchErr=%v controlStatus=%#v", target.name, fromDeviceID, body, lastRevision, lastSnapshot, lastWatchErr, status)
}

func localRequest(address, method string, args map[string]any, timeout time.Duration) (map[string]any, error) {
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))
	request, err := json.Marshal(map[string]any{"method": method, "args": args})
	if err != nil {
		return nil, err
	}
	if _, err := conn.Write(append(request, '\n')); err != nil {
		return nil, err
	}
	line, err := bufio.NewReader(conn).ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	var response map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(line), &response); err != nil {
		return nil, err
	}
	if errorValue, ok := response["error"].(string); ok && strings.TrimSpace(errorValue) != "" {
		return response, errors.New(errorValue)
	}
	return response, nil
}

func register(ctx context.Context, bizURL, email, password string) authEnvelope {
	var out authEnvelope
	postJSON(ctx, bizURL+"/api/app/auth/register", map[string]any{"email": email, "password": password}, &out)
	if out.Auth.Session.Token == "" {
		fail("register returned empty session token")
	}
	if strings.TrimSpace(out.Auth.User.UserID) == "" {
		fail("register returned empty user id")
	}
	return out
}

func assertMQTTBrokerHost(ctx context.Context, bizURL, deviceID, expectedHost string) {
	if strings.TrimSpace(expectedHost) == "" {
		return
	}
	var out mqttCredentialEnvelope
	getJSON(ctx, bizURL+"/api/app/devices/"+url.PathEscape(deviceID)+"/mqtt-credential", &out)
	parsed, err := url.Parse(out.MQTT.BrokerURL)
	if err != nil {
		fail("parse mqtt broker url for %s: %v url=%s", deviceID, err, out.MQTT.BrokerURL)
	}
	if !strings.EqualFold(parsed.Hostname(), expectedHost) {
		fail("mqtt broker host mismatch for %s: got=%s expected=%s url=%s", deviceID, parsed.Hostname(), expectedHost, out.MQTT.BrokerURL)
	}
	fmt.Printf("appDnsAclMessageSmoke: mqtt device=%s broker=%s\n", deviceID, out.MQTT.BrokerURL)
}

func login(ctx context.Context, client *appSmokeClient, email, password string) {
	response, err := localRequest(client.address, "dispatch", map[string]any{
		"type": "loginWithPassword",
		"payload": map[string]any{
			"email":    email,
			"password": password,
		},
	}, 45*time.Second)
	if err != nil {
		fail("%s login request failed: %v", client.name, err)
	}
	if response["signedIn"] != true {
		fail("%s login did not sign in: %#v", client.name, response)
	}
	actualDeviceID, ok := response["deviceId"].(string)
	if !ok || strings.TrimSpace(actualDeviceID) == "" {
		fail("%s login returned empty device id: %#v", client.name, response)
	}
	if actualDeviceID != client.deviceID {
		client.deviceID = actualDeviceID
	}
}

func cleanupIntegration(webBaseURL string, networkID *string, userID string, resources *provisionedResources, clients *[]*appSmokeClient) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if resources != nil {
		for index := len(resources.ruleIDs) - 1; index >= 0; index-- {
			deleteJSON(ctx, webBaseURL+"/api/web/security-groups/rules/"+url.PathEscape(resources.ruleIDs[index]))
		}
		if networkID != nil && strings.TrimSpace(*networkID) != "" {
			for index := len(resources.recordIDs) - 1; index >= 0; index-- {
				deleteJSON(ctx, webBaseURL+"/api/web/networks/"+url.PathEscape(*networkID)+"/dns/records/"+url.PathEscape(resources.recordIDs[index]))
			}
			for index := len(resources.zoneIDs) - 1; index >= 0; index-- {
				deleteJSON(ctx, webBaseURL+"/api/web/networks/"+url.PathEscape(*networkID)+"/dns/zones/"+url.PathEscape(resources.zoneIDs[index]))
			}
		}
	}
	cleanupDevicesForUser(ctx, webBaseURL, userID, clients)
}

func cleanupDevicesForUser(ctx context.Context, webBaseURL, userID string, clients *[]*appSmokeClient) {
	if strings.TrimSpace(userID) == "" {
		return
	}
	deviceIDSet := make(map[string]struct{})
	if clients != nil {
		for _, client := range *clients {
			if client != nil && strings.TrimSpace(client.deviceID) != "" {
				deviceIDSet[client.deviceID] = struct{}{}
			}
		}
	}
	var listed struct {
		Items []struct {
			DeviceID string `json:"deviceId"`
		} `json:"items"`
	}
	getBestEffortJSON(ctx, webBaseURL+"/api/web/devices?userId="+url.QueryEscape(userID), &listed)
	for _, item := range listed.Items {
		if strings.TrimSpace(item.DeviceID) != "" {
			deviceIDSet[item.DeviceID] = struct{}{}
		}
	}
	for deviceID := range deviceIDSet {
		deleteJSON(ctx, webBaseURL+"/api/web/devices/"+url.PathEscape(deviceID)+"?actorUserId="+url.QueryEscape(userID))
	}
}

func firstNetworkID(ctx context.Context, webBaseURL, userID string) string {
	var out itemsEnvelope[networkRow]
	getJSON(ctx, webBaseURL+"/api/web/networks?userId="+url.QueryEscape(userID), &out)
	if len(out.Items) == 0 {
		return ""
	}
	return out.Items[0].NetworkID
}

func getJSON(ctx context.Context, rawURL string, out any) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		fail("build request %s: %v", rawURL, err)
	}
	doJSON(req, out)
}

func getAuthorizedJSON(ctx context.Context, rawURL, bearerToken string, out any) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		fail("build request %s: %v", rawURL, err)
	}
	req.Header.Set("Authorization", "Bearer "+bearerToken)
	doJSON(req, out)
}

func getBestEffortJSON(ctx context.Context, rawURL string, out any) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return
	}
	_ = json.NewDecoder(resp.Body).Decode(out)
}

func postJSON(ctx context.Context, rawURL string, body any, out any) {
	payload, err := json.Marshal(body)
	if err != nil {
		fail("encode request %s: %v", rawURL, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, bytes.NewReader(payload))
	if err != nil {
		fail("build request %s: %v", rawURL, err)
	}
	req.Header.Set("Content-Type", "application/json")
	doJSON(req, out)
}

func deleteJSON(ctx context.Context, rawURL string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, rawURL, nil)
	if err != nil {
		return
	}
	resp, err := httpClient.Do(req)
	if err == nil {
		_ = resp.Body.Close()
	}
}

func doJSON(req *http.Request, out any) {
	resp, err := httpClient.Do(req)
	if err != nil {
		fail("request %s %s failed: %v", req.Method, req.URL.String(), err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		fail("request %s %s returned %d: %s", req.Method, req.URL.String(), resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if out == nil {
		io.Copy(io.Discard, resp.Body)
		return
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		fail("decode response %s %s: %v", req.Method, req.URL.String(), err)
	}
}

func freeLocalAddress() string {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fail("allocate local address: %v", err)
	}
	defer listener.Close()
	return listener.Addr().String()
}

func uniqueSuffix() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func envDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func fail(format string, args ...any) {
	panic(smokeFailure{message: fmt.Sprintf(format, args...)})
}

func exitOnFailure() {
	if recovered := recover(); recovered != nil {
		if failure, ok := recovered.(smokeFailure); ok {
			fmt.Fprintln(os.Stderr, failure.message)
			os.Exit(1)
		}
		panic(recovered)
	}
}
