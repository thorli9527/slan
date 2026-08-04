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

type opsAuthEnvelope struct {
	Auth struct {
		Session struct {
			Token string `json:"token"`
		} `json:"session"`
	} `json:"auth"`
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
var opsAccessToken string

func main() {
	defer exitOnFailure()

	var bizURL string
	var opsBaseURL string
	var serviceBin string
	var opsEmail string
	var opsPassword string
	var authorizationKeys string
	var expectMQTTHost string
	var checkMessages bool
	var clientCount int
	var timeout time.Duration
	flag.StringVar(&bizURL, "biz-url", envDefault("SLAN_BIZ_URL", "http://127.0.0.1:28080"), "service-biz base URL")
	flag.StringVar(&opsBaseURL, "ops-base-url", envDefault("SLAN_OPS_BASE_URL", "http://127.0.0.1:28082"), "Ops API base URL")
	flag.StringVar(&serviceBin, "service-bin", envDefault("SLAN_CLIENT_CORE_SERVICE_BIN", "client/rust/target/release/client-core-service"), "client-core-service binary")
	flag.StringVar(&opsEmail, "ops-email", envDefault("SLAN_OPS_EMAIL", "admin1"), "Ops operator email")
	flag.StringVar(&opsPassword, "ops-password", envDefault("SLAN_OPS_PASSWORD", "admin1"), "Ops operator password")
	flag.StringVar(&authorizationKeys, "authorization-keys", envDefault("SLAN_DEVICE_AUTHORIZATION_KEYS", ""), "comma-separated device authorization keys")
	flag.StringVar(&expectMQTTHost, "expect-mqtt-host", envDefault("SLAN_EXPECT_MQTT_HOST", ""), "expected public MQTT broker host")
	flag.BoolVar(&checkMessages, "check-messages", true, "also verify MQTT client_message delivery")
	flag.IntVar(&clientCount, "clients", 2, "number of logical app clients to start")
	flag.DurationVar(&timeout, "timeout", 90*time.Second, "integration timeout")
	flag.Parse()
	if clientCount < 2 {
		fail("-clients must be >= 2")
	}
	keys := splitRequiredValues(authorizationKeys, clientCount, "-authorization-keys")

	bizURL = strings.TrimRight(bizURL, "/")
	opsBaseURL = strings.TrimRight(opsBaseURL, "/")
	serviceBin = filepath.Clean(serviceBin)
	if _, err := os.Stat(serviceBin); err != nil {
		fail("client-core-service binary is not available at %s: %v\nrun: cd client/rust && cargo build -p client-core-service", serviceBin, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	opsAccessToken = loginOps(ctx, opsBaseURL, opsEmail, opsPassword)
	networkID := ""
	resources := &provisionedResources{}

	root := filepath.Join(os.TempDir(), "slan-app-dns-acl-message-"+uniqueSuffix())
	if os.Getenv("SLAN_KEEP_APP_DNS_ACL_WORK_DIR") == "" {
		defer os.RemoveAll(root)
	} else {
		fmt.Printf("appDnsAclMessageSmoke: workDir=%s\n", root)
	}
	clients := make([]*appSmokeClient, 0, clientCount)
	defer cleanupIntegration(opsBaseURL, &networkID, resources)
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

	for index, client := range clients {
		activate(ctx, client, keys[index])
		assertMQTTBrokerHost(ctx, bizURL, client.deviceID, expectMQTTHost)
		waitControlReady(ctx, client)
	}

	networkID = createOpsNetwork(ctx, opsBaseURL, clients)
	provisionDNSAndACL(ctx, opsBaseURL, networkID, clients, resources)

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
	fmt.Printf("appDnsAclMessageSmoke: ok network=%s clients=%s\n", networkID, strings.Join(deviceIDs, ","))
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

func provisionDNSAndACL(ctx context.Context, opsBaseURL, networkID string, clients []*appSmokeClient, resources *provisionedResources) {
	var group securityGroupRow
	postJSON(ctx, opsBaseURL+"/api/ops/networks/"+url.PathEscape(networkID)+"/security-groups", map[string]any{
		"name": "app-smoke-security-" + uniqueSuffix(),
	}, &group)
	if group.SecurityGroupID == "" {
		fail("created security group returned empty id")
	}
	zoneName := "app-smoke-" + uniqueSuffix() + ".lan"
	var zone dnsZoneRow
	postJSON(ctx, opsBaseURL+"/api/ops/networks/"+url.PathEscape(networkID)+"/dns/zones", map[string]any{
		"name": zoneName,
	}, &zone)
	if zone.ZoneID == "" {
		fail("created dns zone returned empty zone id")
	}
	resources.zoneIDs = append(resources.zoneIDs, zone.ZoneID)
	for _, client := range clients {
		var record dnsRecordRow
		postJSON(ctx, opsBaseURL+"/api/ops/networks/"+url.PathEscape(networkID)+"/dns/records", map[string]any{
			"zoneId": zone.ZoneID,
			"name":   client.name,
			"type":   "A",
			"value":  client.deviceID,
			"port":   "443",
			"ttl":    60,
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
			addRule(ctx, opsBaseURL, group.SecurityGroupID, "ingress", "device", from.deviceID, 443, resources)
			addRule(ctx, opsBaseURL, group.SecurityGroupID, "egress", "device", target.deviceID, 443, resources)
		}
	}
	deviceIDs := make([]string, 0, len(clients))
	for _, client := range clients {
		deviceIDs = append(deviceIDs, client.name+"="+client.deviceID)
	}
	fmt.Printf("appDnsAclMessageSmoke: provisioned dnsZone=%s clients=%s\n", zoneName, strings.Join(deviceIDs, ","))
}

func addRule(ctx context.Context, opsBaseURL, securityGroupID, direction, peerType, peerValue string, port int, resources *provisionedResources) {
	var out securityRuleRow
	postJSON(ctx, opsBaseURL+"/api/ops/security-groups/"+url.PathEscape(securityGroupID)+"/rules", map[string]any{
		"direction": direction,
		"priority":  100,
		"action":    "allow",
		"protocol":  "tcp",
		"portRange": fmt.Sprintf("%d", port),
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

func loginOps(ctx context.Context, opsBaseURL, email, password string) string {
	var out opsAuthEnvelope
	postJSON(ctx, opsBaseURL+"/api/ops/auth/login", map[string]any{"email": email, "password": password}, &out)
	if strings.TrimSpace(out.Auth.Session.Token) == "" {
		fail("Ops login returned empty access token")
	}
	return out.Auth.Session.Token
}

func createOpsNetwork(ctx context.Context, opsBaseURL string, clients []*appSmokeClient) string {
	var network networkRow
	postJSON(ctx, opsBaseURL+"/api/ops/networks", map[string]any{
		"name":   "app-dns-acl-" + uniqueSuffix(),
		"status": "active",
	}, &network)
	if network.NetworkID == "" {
		fail("created Ops network returned empty id")
	}
	for _, client := range clients {
		postJSON(ctx, opsBaseURL+"/api/ops/networks/"+url.PathEscape(network.NetworkID)+"/devices", map[string]any{
			"deviceId": client.deviceID,
		}, nil)
	}
	return network.NetworkID
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

func activate(ctx context.Context, client *appSmokeClient, authorizationKey string) {
	response, err := localRequest(client.address, "dispatch", map[string]any{
		"type": "localActivateDevice",
		"payload": map[string]any{
			"key": authorizationKey,
		},
	}, 45*time.Second)
	if err != nil {
		fail("%s activation request failed: %v", client.name, err)
	}
	if response["activated"] != true {
		fail("%s activation failed: %#v", client.name, response)
	}
	actualDeviceID, ok := response["deviceId"].(string)
	if !ok || strings.TrimSpace(actualDeviceID) == "" {
		fail("%s activation returned empty device id: %#v", client.name, response)
	}
	if actualDeviceID != client.deviceID {
		client.deviceID = actualDeviceID
	}
}

func splitRequiredValues(raw string, expected int, flagName string) []string {
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			values = append(values, value)
		}
	}
	if len(values) != expected {
		fail("%s must contain exactly %d comma-separated values", flagName, expected)
	}
	return values
}

func cleanupIntegration(opsBaseURL string, networkID *string, resources *provisionedResources) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if resources != nil {
		for index := len(resources.ruleIDs) - 1; index >= 0; index-- {
			deleteJSON(ctx, opsBaseURL+"/api/ops/security-rules/"+url.PathEscape(resources.ruleIDs[index]))
		}
		if networkID != nil && strings.TrimSpace(*networkID) != "" {
			for index := len(resources.recordIDs) - 1; index >= 0; index-- {
				deleteJSON(ctx, opsBaseURL+"/api/ops/dns/records/"+url.PathEscape(resources.recordIDs[index]))
			}
			for index := len(resources.zoneIDs) - 1; index >= 0; index-- {
				deleteJSON(ctx, opsBaseURL+"/api/ops/dns/zones/"+url.PathEscape(resources.zoneIDs[index]))
			}
		}
	}
	if networkID != nil && strings.TrimSpace(*networkID) != "" {
		deleteJSON(ctx, opsBaseURL+"/api/ops/networks/"+url.PathEscape(*networkID))
	}
}

func getJSON(ctx context.Context, rawURL string, out any) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		fail("build request %s: %v", rawURL, err)
	}
	setOpsAuthorization(req)
	doJSON(req, out)
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
	setOpsAuthorization(req)
	doJSON(req, out)
}

func deleteJSON(ctx context.Context, rawURL string) {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, rawURL, nil)
	if err != nil {
		return
	}
	setOpsAuthorization(req)
	resp, err := httpClient.Do(req)
	if err == nil {
		_ = resp.Body.Close()
	}
}

func setOpsAuthorization(req *http.Request) {
	if strings.Contains(req.URL.Path, "/api/ops/") && strings.TrimSpace(opsAccessToken) != "" {
		req.Header.Set("Authorization", "Bearer "+opsAccessToken)
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
