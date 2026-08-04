package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

func main() {
	var bizURL string
	var address string
	var authorizationKey string
	var activateDevice bool
	var healthOnly bool
	var enableNetwork bool
	var sendTarget string
	var sendBody string
	var expectFrom string
	var expectBody string
	var timeout time.Duration
	flag.StringVar(&bizURL, "biz-url", envDefault("SLAN_BIZ_URL", "http://127.0.0.1:28080"), "service-biz base URL")
	flag.StringVar(&address, "address", envDefault("SLAN_CLIENT_CORE_SERVICE_HOST", "127.0.0.1:46392"), "client-core-service local API address")
	flag.StringVar(&authorizationKey, "authorization-key", envDefault("SLAN_DEVICE_AUTHORIZATION_KEY", ""), "Opt-issued device authorization key")
	flag.BoolVar(&activateDevice, "activate", envBoolDefault("SLAN_TEST_ACTIVATE_DEVICE", true), "activate device before checks")
	flag.BoolVar(&healthOnly, "health-only", envBoolDefault("SLAN_TEST_HEALTH_ONLY", false), "only verify client-core-service local API health")
	flag.BoolVar(&enableNetwork, "enable-network", envBoolDefault("SLAN_TEST_ENABLE_NETWORK", false), "enable local network after activation")
	flag.StringVar(&sendTarget, "send-target", envDefault("SLAN_TEST_SEND_TARGET_DEVICE_ID", ""), "target device ID to send a client message to after activation")
	flag.StringVar(&sendBody, "send-body", envDefault("SLAN_TEST_SEND_BODY", ""), "client message body to send after activation")
	flag.StringVar(&expectFrom, "expect-from", envDefault("SLAN_TEST_EXPECT_MESSAGE_FROM_DEVICE_ID", ""), "source device ID expected in an inbound client message")
	flag.StringVar(&expectBody, "expect-body", envDefault("SLAN_TEST_EXPECT_MESSAGE_BODY", ""), "expected inbound client message body")
	flag.DurationVar(&timeout, "timeout", 25*time.Second, "check timeout")
	flag.Parse()

	bizURL = strings.TrimRight(strings.TrimSpace(bizURL), "/")
	authorizationKey = strings.TrimSpace(authorizationKey)
	if !healthOnly && activateDevice && authorizationKey == "" {
		fail("authorization key is required; pass -authorization-key or SLAN_DEVICE_AUTHORIZATION_KEY")
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if healthOnly {
		waitServiceReady(ctx, address)
		fmt.Printf("clientCoreServiceHealthCheck: ok address=%s\n", address)
		return
	}

	waitServiceReady(ctx, address)
	var state map[string]any
	if activateDevice {
		state = activate(ctx, address, authorizationKey)
	} else {
		var err error
		state, err = localRequest(address, "localStatus", map[string]any{}, 2*time.Second)
		if err != nil {
			fail("local status request failed: %v", err)
		}
		if state["activated"] != true {
			fail("local status is not activated: %#v", state)
		}
	}
	deviceID := strings.TrimSpace(stringField(state, "deviceId"))
	if deviceID == "" {
		fail("activation response returned empty deviceId: %#v", state)
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
	fmt.Printf("clientCoreServiceActivationCheck: ok deviceId=%s address=%s\n", deviceID, address)
}

func enableLocalNetwork(ctx context.Context, address string) map[string]any {
	requestTimeout := envDurationDefault("SLAN_LOCAL_NETWORK_ACTIVATE_TIMEOUT", 120*time.Second)
	if ctxDeadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(ctxDeadline); remaining > 0 && remaining < requestTimeout {
			requestTimeout = remaining
		}
	}
	response, err := localRequest(address, "localNetworkActivate", map[string]any{}, requestTimeout)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "i/o timeout") {
			if settled, ok := waitLocalNetworkSettled(ctx, address); ok {
				response = settled
			} else {
				fail("enable local network timed out waiting for response and network did not settle: %v", err)
			}
		} else {
			fail("enable local network failed: %v", err)
		}
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

func waitLocalNetworkSettled(ctx context.Context, address string) (map[string]any, bool) {
	pollTimeout := envDurationDefault("SLAN_LOCAL_NETWORK_SETTLE_TIMEOUT", 90*time.Second)
	deadline := time.Now().Add(pollTimeout)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	var last map[string]any
	for time.Now().Before(deadline) {
		response, err := localRequest(address, "localStatus", map[string]any{}, 3*time.Second)
		if err == nil {
			last = response
			if response["networkEnabled"] == true && strings.TrimSpace(stringField(response, "virtualIp")) != "" {
				return response, true
			}
		}
		select {
		case <-ctx.Done():
			return last, false
		case <-time.After(500 * time.Millisecond):
		}
	}
	return last, false
}

func waitServiceReady(ctx context.Context, address string) {
	deadline := time.Now().Add(8 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		if _, err := localRequest(address, "localStatus", map[string]any{}, 800*time.Millisecond); err == nil {
			return
		} else {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			fail("wait service ready: %v", ctx.Err())
		case <-time.After(200 * time.Millisecond):
		}
	}
	fail("client-core-service did not become ready at %s: lastErr=%v", address, lastErr)
}

func activate(ctx context.Context, address, authorizationKey string) map[string]any {
	response, err := localRequest(address, "localActivateDevice", map[string]any{"key": authorizationKey}, 45*time.Second)
	if err != nil {
		fail("device activation request failed: %v", err)
	}
	if response["activated"] != true {
		fail("device activation failed: %#v", response)
	}
	select {
	case <-ctx.Done():
		fail("device activation timeout: %v", ctx.Err())
	default:
	}
	return response
}

func waitControlReady(ctx context.Context, address string) {
	waitWindow := envDurationDefault("SLAN_CONTROL_READY_TIMEOUT", 25*time.Second)
	deadline := time.Now().Add(waitWindow)
	var last map[string]any
	var lastErr error
	mqttConnectAttempted := false
	for time.Now().Before(deadline) {
		response, err := localRequest(address, "localControlStatus", map[string]any{}, 2*time.Second)
		if err == nil {
			last = response
			if response["ready"] == true {
				return
			}
			missing := toStringSlice(response["missing"])
			if !mqttConnectAttempted && containsString(missing, "mqtt") {
				_, _ = localRequest(address, "localEnsureDevice", map[string]any{}, 4*time.Second)
				connectResponse, connectErr := localRequest(address, "localConnectControlMqtt", map[string]any{}, 8*time.Second)
				if connectErr == nil {
					last = connectResponse
					mqttConnectAttempted = true
					continue
				}
				lastErr = connectErr
				mqttConnectAttempted = true
			}
		} else {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			fail("wait control ready: %v", ctx.Err())
		case <-time.After(300 * time.Millisecond):
		}
	}
	fail("control transport is not ready: last=%#v lastErr=%v", last, lastErr)
}

func sendClientMessage(ctx context.Context, address, targetDeviceID, body string) {
	response, err := localRequest(address, "localSendClientMessage", map[string]any{
		"targetDeviceId": targetDeviceID,
		"body":           body,
		"metadata": map[string]any{
			"smoke": "macos-shared-activation-check",
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

func envDurationDefault(name string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func toStringSlice(value any) []string {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		text, ok := item.(string)
		if !ok {
			continue
		}
		text = strings.TrimSpace(text)
		if text != "" {
			result = append(result, text)
		}
	}
	return result
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
