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
	"os"
	"strings"
	"time"
)

type loginAuthResponse struct {
	AccessToken string `json:"accessToken,omitempty"`
	Auth        struct {
		Session struct {
			Token string `json:"token"`
		} `json:"session"`
	} `json:"auth,omitempty"`
}

var httpClient = &http.Client{Timeout: 30 * time.Second}

func main() {
	var bizURL string
	var address string
	var email string
	var password string
	var registerUser bool
	var loginUser bool
	var enableNetwork bool
	var sendTarget string
	var sendBody string
	var expectFrom string
	var expectBody string
	var timeout time.Duration
	flag.StringVar(&bizURL, "biz-url", envDefault("SLAN_BIZ_URL", "http://127.0.0.1:28080"), "service-biz base URL")
	flag.StringVar(&address, "address", envDefault("SLAN_CLIENT_CORE_SERVICE_HOST", "127.0.0.1:46392"), "client-core-service local API address")
	flag.StringVar(&email, "email", envDefault("SLAN_TEST_EMAIL", ""), "login email")
	flag.StringVar(&password, "password", envDefault("SLAN_TEST_PASSWORD", "Password123!"), "login password")
	flag.BoolVar(&registerUser, "register", envBoolDefault("SLAN_TEST_REGISTER_USER", false), "register user before login")
	flag.BoolVar(&loginUser, "login", envBoolDefault("SLAN_TEST_LOGIN_USER", true), "login with password before checks")
	flag.BoolVar(&enableNetwork, "enable-network", envBoolDefault("SLAN_TEST_ENABLE_NETWORK", false), "enable local network after login")
	flag.StringVar(&sendTarget, "send-target", envDefault("SLAN_TEST_SEND_TARGET_DEVICE_ID", ""), "target device ID to send a client message to after login")
	flag.StringVar(&sendBody, "send-body", envDefault("SLAN_TEST_SEND_BODY", ""), "client message body to send after login")
	flag.StringVar(&expectFrom, "expect-from", envDefault("SLAN_TEST_EXPECT_MESSAGE_FROM_DEVICE_ID", ""), "source device ID expected in an inbound client message")
	flag.StringVar(&expectBody, "expect-body", envDefault("SLAN_TEST_EXPECT_MESSAGE_BODY", ""), "expected inbound client message body")
	flag.DurationVar(&timeout, "timeout", 25*time.Second, "check timeout")
	flag.Parse()

	bizURL = strings.TrimRight(strings.TrimSpace(bizURL), "/")
	email = strings.TrimSpace(email)
	if email == "" {
		fail("email is required; pass -email or SLAN_TEST_EMAIL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if registerUser {
		register(ctx, bizURL, email, password)
	}
	waitServiceReady(ctx, address)
	var state map[string]any
	if loginUser {
		state = login(ctx, address, email, password)
	} else {
		var err error
		state, err = localRequest(address, "localStatus", map[string]any{}, 2*time.Second)
		if err != nil {
			fail("local status request failed: %v", err)
		}
		if state["signedIn"] != true {
			fail("local status is not signed in: %#v", state)
		}
	}
	deviceID := strings.TrimSpace(stringField(state, "deviceId"))
	if deviceID == "" {
		fail("login response returned empty deviceId: %#v", state)
	}
	waitControlReady(ctx, address)
	if enableNetwork {
		state = enableLocalNetwork(ctx, address)
		virtualIP := strings.TrimSpace(stringField(state, "virtualIp"))
		if virtualIP == "" {
			fail("enable network returned empty virtualIp: %#v", state)
		}
		fmt.Printf("clientCoreServiceNetwork: enabled virtualIp=%s\n", virtualIP)
	}
	if strings.TrimSpace(sendTarget) != "" || strings.TrimSpace(sendBody) != "" {
		if strings.TrimSpace(sendTarget) == "" || strings.TrimSpace(sendBody) == "" {
			fail("send-target and send-body must be set together")
		}
		sendClientMessage(ctx, address, strings.TrimSpace(sendTarget), strings.TrimSpace(sendBody))
	}
	if strings.TrimSpace(expectFrom) != "" || strings.TrimSpace(expectBody) != "" {
		if strings.TrimSpace(expectFrom) == "" || strings.TrimSpace(expectBody) == "" {
			fail("expect-from and expect-body must be set together")
		}
		waitClientMessage(ctx, address, strings.TrimSpace(expectFrom), strings.TrimSpace(expectBody))
	}
	fmt.Printf("clientCoreServiceLoginCheck: ok email=%s deviceId=%s address=%s\n", email, deviceID, address)
}

func enableLocalNetwork(ctx context.Context, address string) map[string]any {
	response, err := localRequest(address, "localNetworkActivate", map[string]any{}, 45*time.Second)
	if err != nil {
		fail("enable local network failed: %v", err)
	}
	if message := strings.TrimSpace(stringField(response, "error")); message != "" {
		fail("enable local network returned error: %s state=%#v", message, response)
	}
	if response["networkEnabled"] != true {
		fail("enable local network did not enable network: %#v", response)
	}
	select {
	case <-ctx.Done():
		fail("enable local network timeout: %v", ctx.Err())
	default:
	}
	return response
}

func waitServiceReady(ctx context.Context, address string) {
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := localRequest(address, "localStatus", map[string]any{}, 800*time.Millisecond); err == nil {
			return
		}
		select {
		case <-ctx.Done():
			fail("wait service ready: %v", ctx.Err())
		case <-time.After(200 * time.Millisecond):
		}
	}
	fail("client-core-service did not become ready at %s", address)
}

func login(ctx context.Context, address, email, password string) map[string]any {
	response, err := localRequest(address, "dispatch", map[string]any{
		"type": "loginWithPassword",
		"payload": map[string]any{
			"email":    email,
			"password": password,
		},
	}, 45*time.Second)
	if err != nil {
		fail("login request failed: %v", err)
	}
	if response["signedIn"] != true {
		fail("login did not sign in: %#v", response)
	}
	select {
	case <-ctx.Done():
		fail("login timeout: %v", ctx.Err())
	default:
	}
	return response
}

func waitControlReady(ctx context.Context, address string) {
	deadline := time.Now().Add(10 * time.Second)
	var last map[string]any
	for time.Now().Before(deadline) {
		response, err := localRequest(address, "localControlStatus", map[string]any{}, 2*time.Second)
		if err == nil {
			last = response
			if response["ready"] == true {
				return
			}
		}
		select {
		case <-ctx.Done():
			fail("wait control ready: %v", ctx.Err())
		case <-time.After(300 * time.Millisecond):
		}
	}
	fail("control transport is not ready: %#v", last)
}

func sendClientMessage(ctx context.Context, address, targetDeviceID, body string) {
	response, err := localRequest(address, "localSendClientMessage", map[string]any{
		"targetDeviceId": targetDeviceID,
		"body":           body,
		"metadata": map[string]any{
			"smoke": "macos-shared-login-check",
		},
	}, 8*time.Second)
	if err != nil {
		fail("send client message failed: %v", err)
	}
	if response["messageId"] == "" || response["messageId"] == nil {
		fail("send client message returned no messageId: %#v", response)
	}
	select {
	case <-ctx.Done():
		fail("send client message timeout: %v", ctx.Err())
	default:
	}
}

func waitClientMessage(ctx context.Context, address, fromDeviceID, body string) {
	var lastRevision float64
	deadline := time.Now().Add(45 * time.Second)
	if ctxDeadline, ok := ctx.Deadline(); ok {
		deadline = ctxDeadline
	}
	var lastSnapshot map[string]any
	var lastWatchErr error
	for time.Now().Before(deadline) {
		response, err := localRequest(address, "localBusinessEventWatch", map[string]any{
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
			return
		}
		select {
		case <-ctx.Done():
			fail("wait client message timeout: %v", ctx.Err())
		default:
		}
	}
	status, _ := localRequest(address, "localControlStatus", map[string]any{}, 2*time.Second)
	fail("did not consume client_message from %s body=%s lastRevision=%.0f lastSnapshot=%#v lastWatchErr=%v controlStatus=%#v", fromDeviceID, body, lastRevision, lastSnapshot, lastWatchErr, status)
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
	var out loginAuthResponse
	postJSON(ctx, bizURL+"/api/auth/register", map[string]any{
		"email":    email,
		"password": password,
	}, &out)
	if strings.TrimSpace(out.AccessToken) == "" {
		out.AccessToken = out.Auth.Session.Token
	}
	if strings.TrimSpace(out.AccessToken) == "" {
		fail("register returned empty session token")
	}
}

func postJSON(ctx context.Context, url string, body any, out any) {
	payload, err := json.Marshal(body)
	if err != nil {
		fail("encode request %s: %v", url, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		fail("build request %s: %v", url, err)
	}
	req.Header.Set("Content-Type", "application/json")
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

func stringField(values map[string]any, key string) string {
	value, _ := values[key].(string)
	return value
}

func envDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func envBoolDefault(name string, fallback bool) bool {
	value := strings.TrimSpace(strings.ToLower(os.Getenv(name)))
	if value == "" {
		return fallback
	}
	return value == "1" || value == "true" || value == "yes"
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
