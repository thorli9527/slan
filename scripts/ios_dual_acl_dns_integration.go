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

type dualClient struct {
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
	NetworkCount      int `json:"networkCount"`
	PeerCount         int `json:"peerCount"`
	DNSRecordCount    int `json:"dnsRecordCount"`
	SecurityRuleCount int `json:"securityRuleCount"`
}

type mqttCredentialEnvelope struct {
	MQTT struct {
		BrokerURL string `json:"brokerUrl"`
	} `json:"mqtt"`
}

type integrationFailure struct {
	message string
}

var httpClient = &http.Client{Timeout: 30 * time.Second}

func main() {
	defer exitOnFailure()

	var bizURL string
	var serviceBin string
	var password string
	var expectMQTTHost string
	var checkMessages bool
	var clientCount int
	var timeout time.Duration
	flag.StringVar(&bizURL, "biz-url", envDefault("SLAN_BIZ_URL", "http://127.0.0.1:28080"), "service-biz base URL")
	flag.StringVar(&serviceBin, "service-bin", envDefault("SLAN_CLIENT_CORE_SERVICE_BIN", "client_v2/rust/target/debug/client-core-service"), "client-core-service binary")
	flag.StringVar(&password, "password", "Password123!", "test user password")
	flag.StringVar(&expectMQTTHost, "expect-mqtt-host", envDefault("SLAN_EXPECT_MQTT_HOST", ""), "expected public MQTT broker host returned by service-biz")
	flag.BoolVar(&checkMessages, "check-messages", false, "also verify MQTT client_message delivery")
	flag.IntVar(&clientCount, "clients", 2, "number of logical iOS clients to start")
	flag.DurationVar(&timeout, "timeout", 75*time.Second, "integration timeout")
	flag.Parse()
	if clientCount < 2 {
		fail("-clients must be >= 2")
	}

	bizURL = strings.TrimRight(bizURL, "/")
	serviceBin = filepath.Clean(serviceBin)
	if _, err := os.Stat(serviceBin); err != nil {
		fail("client-core-service binary is not available at %s: %v\nrun: cd client_v2/rust && cargo build -p client-core-service", serviceBin, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	email := fmt.Sprintf("ios-dual-acl-dns-%d@example.test", time.Now().UnixNano())
	auth := register(ctx, bizURL, email, password)
	userID := auth.Auth.User.UserID
	networkID := auth.DefaultNetwork.NetworkID
	if networkID == "" {
		networkID = firstNetworkID(ctx, bizURL, auth.Auth.User.UserID)
	}
	if networkID == "" {
		fail("registered user has no default network")
	}
	resources := &provisionedResources{}

	root := filepath.Join(os.TempDir(), "slan-ios-dual-acl-dns-"+uniqueSuffix())
	if os.Getenv("SLAN_KEEP_IOS_DUAL_WORK_DIR") == "" {
		defer os.RemoveAll(root)
	} else {
		fmt.Printf("iosDualAclDnsIntegration: workDir=%s\n", root)
	}
	clients := make([]*dualClient, 0, clientCount)
	defer cleanupIntegration(bizURL, &networkID, userID, resources, &clients)
	for index := 0; index < clientCount; index++ {
		name := fmt.Sprintf("ios-%c", 'a'+rune(index))
		clients = append(clients, startService(ctx, serviceBin, bizURL, filepath.Join(root, name), "ios-sim-"+string('a'+rune(index))+"-"+uniqueSuffix(), name))
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

	provisionDNSAndACL(ctx, bizURL, networkID, clients, resources)
	minPeers := clientCount - 1
	minDNSRecords := clientCount
	minSecurityRules := clientCount * (clientCount - 1) * 2
	for _, client := range clients {
		waitModule(ctx, client, minPeers, minDNSRecords, minSecurityRules)
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
	fmt.Printf("iosDualAclDnsIntegration: ok email=%s network=%s clients=%s\n", email, networkID, strings.Join(deviceIDs, ","))
}

func startService(ctx context.Context, serviceBin, bizURL, stateDir, deviceID, name string) *dualClient {
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
	client := &dualClient{name: name, address: address, deviceID: deviceID, cmd: cmd, logFile: logFile}
	waitServiceReady(ctx, client)
	return client
}

func (client *dualClient) stop() {
	if client.cmd != nil && client.cmd.Process != nil {
		_ = client.cmd.Process.Kill()
		_, _ = client.cmd.Process.Wait()
	}
	if client.logFile != nil {
		_ = client.logFile.Close()
	}
}

func login(ctx context.Context, client *dualClient, email, password string) {
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

func assertMQTTBrokerHost(ctx context.Context, bizURL, deviceID, expectedHost string) {
	if strings.TrimSpace(expectedHost) == "" {
		return
	}
	var out mqttCredentialEnvelope
	getJSON(ctx, bizURL+"/api/devices/"+url.PathEscape(deviceID)+"/mqtt-credential", &out)
	parsed, err := url.Parse(out.MQTT.BrokerURL)
	if err != nil {
		fail("parse mqtt broker url for %s: %v url=%s", deviceID, err, out.MQTT.BrokerURL)
	}
	if !strings.EqualFold(parsed.Hostname(), expectedHost) {
		fail("mqtt broker host mismatch for %s: got=%s expected=%s url=%s", deviceID, parsed.Hostname(), expectedHost, out.MQTT.BrokerURL)
	}
	fmt.Printf("iosDualAclDnsIntegration: mqtt device=%s broker=%s\n", deviceID, out.MQTT.BrokerURL)
}

func waitServiceReady(ctx context.Context, client *dualClient) {
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

func waitControlReady(ctx context.Context, client *dualClient) {
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

func provisionDNSAndACL(ctx context.Context, bizURL, networkID string, clients []*dualClient, resources *provisionedResources) {
	var groups itemsEnvelope[securityGroupRow]
	getJSON(ctx, bizURL+"/api/networks/"+url.PathEscape(networkID)+"/security-groups", &groups)
	if len(groups.Items) == 0 {
		fail("network %s has no security groups", networkID)
	}
	zoneName := "itest-" + uniqueSuffix() + ".lan"
	var zone dnsZoneRow
	postJSON(ctx, bizURL+"/api/networks/"+url.PathEscape(networkID)+"/dns/zones", map[string]any{
		"zoneName": zoneName,
	}, &zone)
	if zone.ZoneID == "" {
		fail("created dns zone returned empty zone id")
	}
	resources.zoneIDs = append(resources.zoneIDs, zone.ZoneID)
	for _, client := range clients {
		var record dnsRecordRow
		postJSON(ctx, bizURL+"/api/networks/"+url.PathEscape(networkID)+"/dns/records", map[string]any{
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
			addRule(ctx, bizURL, groups.Items[0].SecurityGroupID, "ingress", "device", from.deviceID, 443, resources)
			addRule(ctx, bizURL, groups.Items[0].SecurityGroupID, "egress", "device", target.deviceID, 443, resources)
		}
	}
	deviceIDs := make([]string, 0, len(clients))
	for _, client := range clients {
		deviceIDs = append(deviceIDs, client.name+"="+client.deviceID)
	}
	fmt.Printf("iosDualAclDnsIntegration: provisioned dnsZone=%s clients=%s\n", zoneName, strings.Join(deviceIDs, ","))
}

func addRule(ctx context.Context, bizURL, securityGroupID, direction, peerType, peerValue string, port int, resources *provisionedResources) {
	var out securityRuleRow
	postJSON(ctx, bizURL+"/api/security-groups/"+url.PathEscape(securityGroupID)+"/rules", map[string]any{
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

func waitModule(ctx context.Context, client *dualClient, minPeers, minDNSRecords, minSecurityRules int) {
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
			if last.PeerCount >= minPeers && last.DNSRecordCount >= minDNSRecords && last.SecurityRuleCount >= minSecurityRules {
				fmt.Printf("iosDualAclDnsIntegration: module %s peers=%d dnsRecords=%d securityRules=%d\n", client.name, last.PeerCount, last.DNSRecordCount, last.SecurityRuleCount)
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

func enableNetwork(ctx context.Context, client *dualClient) {
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
	select {
	case <-ctx.Done():
		fail("%s enable network timeout: %v", client.name, ctx.Err())
	default:
	}
}

func sendClientMessage(ctx context.Context, from *dualClient, targetDeviceID, body string) {
	response, err := localRequest(from.address, "localSendClientMessage", map[string]any{
		"targetDeviceId": targetDeviceID,
		"body":           body,
		"metadata": map[string]any{
			"smoke": "ios-dual-acl-dns",
		},
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

func waitClientMessage(ctx context.Context, target *dualClient, fromDeviceID, body string) {
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
			fmt.Printf("iosDualAclDnsIntegration: delivered from=%s target=%s body=%s\n", fromDeviceID, target.deviceID, body)
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
	postJSON(ctx, bizURL+"/api/auth/register", map[string]any{"email": email, "password": password}, &out)
	if out.Auth.Session.Token == "" {
		fail("register returned empty session token")
	}
	if strings.TrimSpace(out.Auth.User.UserID) == "" {
		fail("register returned empty user id")
	}
	return out
}

func cleanupIntegration(bizURL string, networkID *string, userID string, resources *provisionedResources, clients *[]*dualClient) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if resources != nil {
		for index := len(resources.ruleIDs) - 1; index >= 0; index-- {
			deleteJSON(ctx, bizURL+"/api/security-groups/rules/"+url.PathEscape(resources.ruleIDs[index]))
		}
		if networkID != nil && strings.TrimSpace(*networkID) != "" {
			for index := len(resources.recordIDs) - 1; index >= 0; index-- {
				deleteJSON(ctx, bizURL+"/api/networks/"+url.PathEscape(*networkID)+"/dns/records/"+url.PathEscape(resources.recordIDs[index]))
			}
			for index := len(resources.zoneIDs) - 1; index >= 0; index-- {
				deleteJSON(ctx, bizURL+"/api/networks/"+url.PathEscape(*networkID)+"/dns/zones/"+url.PathEscape(resources.zoneIDs[index]))
			}
		}
	}
	cleanupDevicesForUser(ctx, bizURL, userID, clients)
}

func cleanupDevicesForUser(ctx context.Context, bizURL, userID string, clients *[]*dualClient) {
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
	getBestEffortJSON(ctx, bizURL+"/api/devices?userId="+url.QueryEscape(userID), &listed)
	for _, item := range listed.Items {
		if strings.TrimSpace(item.DeviceID) != "" {
			deviceIDSet[item.DeviceID] = struct{}{}
		}
	}
	for deviceID := range deviceIDSet {
		deleteJSON(ctx, bizURL+"/api/devices/"+url.PathEscape(deviceID)+"?actorUserId="+url.QueryEscape(userID))
	}
}

func firstNetworkID(ctx context.Context, bizURL, userID string) string {
	var out itemsEnvelope[networkRow]
	getJSON(ctx, bizURL+"/api/networks?userId="+url.QueryEscape(userID), &out)
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
		fail("%s %s: %v", req.Method, req.URL, err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		fail("%s %s: HTTP %d: %s", req.Method, req.URL, resp.StatusCode, string(respBody))
	}
	if out != nil && len(bytes.TrimSpace(respBody)) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			fail("decode %s: %v body=%s", req.URL, err, string(respBody))
		}
	}
}

func freeLocalAddress() string {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fail("allocate local port: %v", err)
	}
	defer listener.Close()
	return listener.Addr().String()
}

func uniqueSuffix() string {
	return strings.ReplaceAll(url.QueryEscape(fmt.Sprintf("%d", time.Now().UnixNano())), "%", "")
}

func envDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func fail(format string, args ...any) {
	panic(integrationFailure{message: fmt.Sprintf(format, args...)})
}

func exitOnFailure() {
	recovered := recover()
	if recovered == nil {
		return
	}
	if failure, ok := recovered.(integrationFailure); ok {
		fmt.Fprintln(os.Stderr, failure.message)
		os.Exit(1)
	}
	panic(recovered)
}
