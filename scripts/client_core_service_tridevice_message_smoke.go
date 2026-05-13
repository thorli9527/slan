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

type triClient struct {
	name     string
	address  string
	deviceID string
	cmd      *exec.Cmd
	logFile  *os.File
}

type triAuthResponse struct {
	AccessToken string `json:"accessToken,omitempty"`
	Auth        struct {
		Session struct {
			Token string `json:"token"`
		} `json:"session"`
	} `json:"auth,omitempty"`
}

func main() {
	var bizURL string
	var serviceBin string
	var password string
	var timeout time.Duration
	flag.StringVar(&bizURL, "biz-url", envDefault("SLAN_BIZ_URL", "http://127.0.0.1:28080"), "server-biz base URL")
	flag.StringVar(&serviceBin, "service-bin", envDefault("SLAN_CLIENT_CORE_SERVICE_BIN", "client_v2/rust/target/debug/client-core-service"), "client-core-service binary")
	flag.StringVar(&password, "password", "Password123!", "test user password")
	flag.DurationVar(&timeout, "timeout", 45*time.Second, "smoke timeout")
	flag.Parse()

	bizURL = strings.TrimRight(bizURL, "/")
	serviceBin = filepath.Clean(serviceBin)
	if _, err := os.Stat(serviceBin); err != nil {
		fail("client-core-service binary is not available at %s: %v\nrun: cd client_v2/rust && cargo build -p client-core-service", serviceBin, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	email := fmt.Sprintf("client-tridevice-smoke-%d@example.test", time.Now().UnixNano())
	register(ctx, bizURL, email, password)

	root := filepath.Join(os.TempDir(), "slan-client-tridevice-smoke-"+uniqueSuffix())
	defer os.RemoveAll(root)

	clients := []*triClient{
		startTriService(ctx, serviceBin, bizURL, filepath.Join(root, "mac"), "smoke-mac-"+uniqueSuffix(), "mac"),
		startTriService(ctx, serviceBin, bizURL, filepath.Join(root, "android"), "smoke-android-"+uniqueSuffix(), "android"),
		startTriService(ctx, serviceBin, bizURL, filepath.Join(root, "ios"), "smoke-ios-"+uniqueSuffix(), "ios"),
	}
	defer func() {
		for _, client := range clients {
			client.stop()
		}
	}()

	for _, client := range clients {
		login(ctx, client, email, password)
	}
	for _, client := range clients {
		waitControlReady(ctx, client)
	}
	time.Sleep(6 * time.Second)

	edges := [][2]int{
		{0, 1}, // mac -> android
		{1, 2}, // android -> ios
		{2, 0}, // ios -> mac
	}
	for index, edge := range edges {
		from := clients[edge[0]]
		target := clients[edge[1]]
		body := fmt.Sprintf("hello-%s-to-%s-%s", from.name, target.name, uniqueSuffix())
		sendClientMessage(ctx, from, target.deviceID, body)
		waitClientMessage(ctx, target, from.deviceID, body)
		fmt.Printf("triDeviceMessageSmoke: delivered step=%d from=%s target=%s body=%s\n", index+1, from.deviceID, target.deviceID, body)
	}

	fmt.Printf(
		"triDeviceMessageSmoke: ok email=%s mac=%s android=%s ios=%s\n",
		email,
		clients[0].deviceID,
		clients[1].deviceID,
		clients[2].deviceID,
	)
}

func startTriService(ctx context.Context, serviceBin, bizURL, stateDir, deviceID, name string) *triClient {
	if err := os.MkdirAll(filepath.Join(stateDir, "SLAN"), 0o755); err != nil {
		fail("create %s state dir: %v", name, err)
	}
	address := freeLocalAddress()
	logFilePath := filepath.Join(stateDir, "SLAN", "client-core-service-tridevice.log")
	logFile, err := os.Create(logFilePath)
	if err != nil {
		fail("create %s log: %v", name, err)
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
	client := &triClient{
		name:     name,
		address:  address,
		deviceID: deviceID,
		cmd:      cmd,
		logFile:  logFile,
	}
	waitServiceReady(ctx, client)
	return client
}

func (client *triClient) stop() {
	if client.cmd != nil && client.cmd.Process != nil {
		_ = client.cmd.Process.Kill()
		_, _ = client.cmd.Process.Wait()
	}
	if client.logFile != nil {
		_ = client.logFile.Close()
	}
}

func waitServiceReady(ctx context.Context, client *triClient) {
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

func login(ctx context.Context, client *triClient, email, password string) {
	response, err := localRequest(client.address, "dispatch", map[string]any{
		"type": "loginWithPassword",
		"payload": map[string]any{
			"email":    email,
			"password": password,
		},
	}, 8*time.Second)
	if err != nil {
		fail("%s login request failed: %v", client.name, err)
	}
	if response["signedIn"] != true {
		fail("%s login did not sign in: %#v", client.name, response)
	}
	if response["deviceId"] != client.deviceID {
		fail("%s device id mismatch: response=%v expected=%s", client.name, response["deviceId"], client.deviceID)
	}
	select {
	case <-ctx.Done():
		fail("%s login timeout: %v", client.name, ctx.Err())
	default:
	}
}

func waitControlReady(ctx context.Context, client *triClient) {
	deadline := time.Now().Add(10 * time.Second)
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

func sendClientMessage(ctx context.Context, from *triClient, targetDeviceID, body string) {
	response, err := localRequest(from.address, "localSendClientMessage", map[string]any{
		"targetDeviceId": targetDeviceID,
		"body":           body,
		"metadata": map[string]any{
			"smoke": "tridevice",
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

func waitClientMessage(ctx context.Context, target *triClient, fromDeviceID, body string) {
	var lastRevision float64
	deadline := time.Now().Add(20 * time.Second)
	var lastSnapshot map[string]any
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
		lastSnapshot = snapshot
		if snapshot["lastClientMessageFromDeviceId"] == fromDeviceID && snapshot["lastClientMessageBody"] == body {
			return
		}
		select {
		case <-ctx.Done():
			fail("wait client message timeout: %v", ctx.Err())
		default:
		}
	}
	status, _ := localRequest(target.address, "localControlStatus", map[string]any{}, 2*time.Second)
	fail("%s did not consume client_message from %s body=%s lastRevision=%.0f lastSnapshot=%#v controlStatus=%#v", target.name, fromDeviceID, body, lastRevision, lastSnapshot, status)
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

func register(ctx context.Context, bizURL, email, password string) {
	var out triAuthResponse
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
	resp, err := http.DefaultClient.Do(req)
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
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
