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

type serviceClient struct {
	name     string
	address  string
	deviceID string
	stateDir string
	cmd      *exec.Cmd
	logFile  *os.File
}

type authResponse struct {
	AccessToken string `json:"accessToken,omitempty"`
	Auth        struct {
		User struct {
			UserID string `json:"userId"`
		} `json:"user"`
		Session struct {
			Token string `json:"token"`
		} `json:"session"`
	} `json:"auth,omitempty"`
}

type smokeFailure struct {
	message string
}

var httpClient = &http.Client{Timeout: 12 * time.Second}

func main() {
	defer exitOnFailure()

	var bizURL string
	var serviceBin string
	var password string
	var fromPlatform string
	var targetPlatform string
	var timeout time.Duration
	flag.StringVar(&bizURL, "biz-url", envDefault("SLAN_BIZ_URL", "http://127.0.0.1:28080"), "service-biz base URL")
	flag.StringVar(&serviceBin, "service-bin", envDefault("SLAN_CLIENT_CORE_SERVICE_BIN", "client_v2/rust/target/debug/client-core-service"), "client-core-service binary")
	flag.StringVar(&password, "password", "Password123!", "test user password")
	flag.StringVar(&fromPlatform, "from-platform", "mac", "source service platform label")
	flag.StringVar(&targetPlatform, "target-platform", "ios", "target service platform label")
	flag.DurationVar(&timeout, "timeout", 25*time.Second, "smoke timeout")
	flag.Parse()

	bizURL = strings.TrimRight(bizURL, "/")
	serviceBin = filepath.Clean(serviceBin)
	if _, err := os.Stat(serviceBin); err != nil {
		fail("client-core-service binary is not available at %s: %v\nrun: cd client_v2/rust && cargo build -p client-core-service", serviceBin, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	email := fmt.Sprintf("client-core-service-smoke-%d@example.test", time.Now().UnixNano())
	auth := register(ctx, bizURL, email, password)
	userID := auth.Auth.User.UserID

	root := filepath.Join(os.TempDir(), "slan-client-core-service-message-smoke-"+uniqueSuffix())
	defer func() {
		if os.Getenv("SLAN_KEEP_CLIENT_CORE_SERVICE_WORK_DIR") == "1" {
			fmt.Printf("clientCoreServiceMessageSmoke: kept work dir: %s\n", root)
			return
		}
		_ = os.RemoveAll(root)
	}()

	from := startService(ctx, serviceBin, bizURL, filepath.Join(root, "from"), "smoke-"+fromPlatform+"-"+uniqueSuffix(), fromPlatform)
	target := startService(ctx, serviceBin, bizURL, filepath.Join(root, "target"), "smoke-"+targetPlatform+"-"+uniqueSuffix(), targetPlatform)
	defer func() {
		cleanupDevices(bizURL, userID, []string{from.deviceID, target.deviceID})
	}()
	defer from.stop()
	defer target.stop()

	login(ctx, from, email, password)
	login(ctx, target, email, password)
	waitControlReady(ctx, from)
	waitControlReady(ctx, target)
	time.Sleep(6 * time.Second)

	body := "hello-client-core-service-" + uniqueSuffix()
	sendClientMessage(ctx, from, target.deviceID, body)
	waitClientMessage(ctx, target, from.deviceID, body)

	fmt.Printf(
		"clientCoreServiceMessageSmoke: ok email=%s from=%s target=%s body=%s\n",
		email,
		from.deviceID,
		target.deviceID,
		body,
	)
}

func startService(ctx context.Context, serviceBin, bizURL, stateDir, deviceID, name string) *serviceClient {
	if err := os.MkdirAll(filepath.Join(stateDir, "SLAN"), 0o755); err != nil {
		fail("create state dir: %v", err)
	}
	address := freeLocalAddress()
	logFilePath := filepath.Join(stateDir, "SLAN", "client-core-service-smoke.log")
	logFile, err := os.Create(logFilePath)
	if err != nil {
		fail("create service log: %v", err)
	}
	cmd := exec.CommandContext(ctx, serviceBin)
	cmd.Env = append(os.Environ(),
		"SLAN_CONTROL_BASE_URL="+bizURL,
		"SLAN_CLIENT_CORE_SERVICE_HOST="+address,
		"SLAN_CLIENT_DEVICE_ID="+deviceID,
		"SLAN_STATE_DIR="+stateDir,
	)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		fail("start %s service: %v", name, err)
	}
	client := &serviceClient{
		name:     name,
		address:  address,
		deviceID: deviceID,
		stateDir: stateDir,
		cmd:      cmd,
		logFile:  logFile,
	}
	waitServiceReady(ctx, client)
	return client
}

func (client *serviceClient) stop() {
	if client.cmd != nil && client.cmd.Process != nil {
		_ = client.cmd.Process.Kill()
		_, _ = client.cmd.Process.Wait()
	}
	if client.logFile != nil {
		_ = client.logFile.Close()
	}
}

func waitServiceReady(ctx context.Context, client *serviceClient) {
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

func login(ctx context.Context, client *serviceClient, email, password string) {
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
	select {
	case <-ctx.Done():
		fail("%s login timeout: %v", client.name, ctx.Err())
	default:
	}
}

func waitControlReady(ctx context.Context, client *serviceClient) {
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		response, err := localRequest(client.address, "localControlStatus", map[string]any{}, 2*time.Second)
		if err == nil && response["ready"] == true {
			return
		}
		select {
		case <-ctx.Done():
			fail("wait %s control ready: %v", client.name, ctx.Err())
		case <-time.After(300 * time.Millisecond):
		}
	}
	fail("%s control transport is not ready", client.name)
}

func sendClientMessage(ctx context.Context, from *serviceClient, targetDeviceID, body string) {
	response, err := localRequest(from.address, "localSendClientMessage", map[string]any{
		"targetDeviceId": targetDeviceID,
		"body":           body,
		"metadata": map[string]any{
			"smoke": "client-core-service",
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

func waitClientMessage(ctx context.Context, target *serviceClient, fromDeviceID, body string) {
	var lastRevision float64
	deadline := time.Now().Add(12 * time.Second)
	for time.Now().Before(deadline) {
		response, err := localRequest(target.address, "localBusinessEventWatch", map[string]any{
			"lastRevision": lastRevision,
			"timeoutMs":    2000,
		}, 3*time.Second)
		if err != nil {
			fail("%s watch business event failed: %v", target.name, err)
		}
		if rev, ok := response["revision"].(float64); ok {
			lastRevision = rev
		}
		snapshot, _ := response["snapshot"].(map[string]any)
		if snapshot["lastClientMessageFromDeviceId"] == fromDeviceID && snapshot["lastClientMessageBody"] == body {
			return
		}
		select {
		case <-ctx.Done():
			fail("wait client message timeout: %v", ctx.Err())
		default:
		}
	}
	fail("%s did not consume client_message from %s body=%s", target.name, fromDeviceID, body)
}

func localRequest(address, method string, args map[string]any, timeout time.Duration) (map[string]any, error) {
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))
	request, err := json.Marshal(map[string]any{
		"method": method,
		"args":   args,
	})
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

func register(ctx context.Context, bizURL, email, password string) authResponse {
	var out authResponse
	postJSON(ctx, bizURL+"/api/auth/register", "", map[string]any{
		"email":    email,
		"password": password,
	}, &out)
	if strings.TrimSpace(out.AccessToken) == "" {
		out.AccessToken = out.Auth.Session.Token
	}
	if strings.TrimSpace(out.AccessToken) == "" {
		fail("register returned empty access token")
	}
	if strings.TrimSpace(out.Auth.User.UserID) == "" {
		fail("register returned empty userId")
	}
	return out
}

func cleanupDevices(bizURL, userID string, deviceIDs []string) {
	if strings.TrimSpace(userID) == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	deviceIDSet := make(map[string]struct{}, len(deviceIDs))
	for _, deviceID := range deviceIDs {
		if strings.TrimSpace(deviceID) == "" {
			continue
		}
		deviceIDSet[deviceID] = struct{}{}
	}
	var listed struct {
		Items []struct {
			DeviceID string `json:"deviceId"`
		} `json:"items"`
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, bizURL+"/api/devices?userId="+url.QueryEscape(userID), nil)
	if err == nil {
		if resp, err := httpClient.Do(req); err == nil {
			func() {
				defer resp.Body.Close()
				if resp.StatusCode >= 200 && resp.StatusCode < 300 {
					_ = json.NewDecoder(resp.Body).Decode(&listed)
				}
			}()
		}
	}
	for _, item := range listed.Items {
		if strings.TrimSpace(item.DeviceID) != "" {
			deviceIDSet[item.DeviceID] = struct{}{}
		}
	}
	for deviceID := range deviceIDSet {
		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, bizURL+"/api/devices/"+url.PathEscape(deviceID)+"?actorUserId="+url.QueryEscape(userID), nil)
		if err != nil {
			continue
		}
		resp, err := httpClient.Do(req)
		if err == nil {
			_ = resp.Body.Close()
		}
	}
}

func postJSON(ctx context.Context, url, token string, body any, out any) {
	payload, err := json.Marshal(body)
	if err != nil {
		fail("encode request %s: %v", url, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		fail("build request %s: %v", url, err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		fail("%s %s: %v", req.Method, req.URL, err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		fail("%s %s: HTTP %d: %s", req.Method, req.URL, resp.StatusCode, string(respBody))
	}
	if err := json.Unmarshal(respBody, out); err != nil {
		fail("decode %s: %v body=%s", req.URL, err, string(respBody))
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
