package biz

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const punchRequestTimeout = 5 * time.Second

type punchDeviceAuth struct {
	DeviceID  string
	Username  string
	Signature string
}

func (s *Store) AuthorizePunchConnect(networkID, requesterNodeID, peerNodeID string, auth punchDeviceAuth, mqtt MQTTConfig) error {
	networkID = strings.TrimSpace(networkID)
	requesterDeviceID := deviceIDFromNodeID(requesterNodeID)
	peerDeviceID := deviceIDFromNodeID(peerNodeID)
	auth.DeviceID = strings.TrimSpace(auth.DeviceID)
	auth.Username = strings.TrimSpace(auth.Username)
	auth.Signature = strings.ToLower(strings.TrimSpace(auth.Signature))
	if networkID == "" || requesterDeviceID == "" || peerDeviceID == "" || auth.DeviceID == "" || auth.Username == "" || auth.Signature == "" {
		return errBadRequest
	}
	if auth.DeviceID != requesterDeviceID {
		return errUnauthorized
	}
	if !verifyPunchMQTTSignature(mqtt, auth.DeviceID, auth.Username, auth.Signature, time.Now()) {
		return errUnauthorized
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		return err
	}
	if _, err := s.networkConfigLocked(networkID, requesterDeviceID); err != nil {
		return err
	}
	if _, err := s.networkConfigLocked(networkID, peerDeviceID); err != nil {
		return err
	}
	return nil
}

func (s *Server) createPunchConnectSession(w http.ResponseWriter, r *http.Request) {
	var req CreatePunchConnectSessionRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	networkID := r.PathValue("networkId")
	auth := punchDeviceAuth{
		DeviceID:  r.Header.Get("X-Slan-Device-ID"),
		Username:  r.Header.Get("X-Slan-MQTT-Username"),
		Signature: r.Header.Get("X-Slan-Punch-Signature"),
	}
	response, status, err := s.services.Punch.CreateConnectSession(r.Context(), networkID, req, auth)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, status, response)
}

func requestPunchConnectSession(ctx context.Context, nodes []OpsPunchNode, networkID, requesterNodeID, peerNodeID string, ttlSeconds int, auth punchDeviceAuth) (map[string]any, int, error) {
	if strings.TrimSpace(auth.DeviceID) == "" || strings.TrimSpace(auth.Username) == "" || strings.TrimSpace(auth.Signature) == "" {
		return nil, 0, errUnauthorized
	}
	if len(nodes) == 0 {
		return nil, 0, errUnavailable
	}
	payload := map[string]any{
		"networkId":       strings.TrimSpace(networkID),
		"requesterNodeId": strings.TrimSpace(requesterNodeID),
		"peerNodeId":      strings.TrimSpace(peerNodeID),
	}
	if ttlSeconds > 0 {
		payload["ttlSeconds"] = ttlSeconds
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, err
	}
	var lastErr error = errUnavailable
	for _, node := range nodes {
		baseURL := punchNodeBaseURL(node)
		if baseURL == "" {
			continue
		}
		decoded, status, err := postPunchConnectSession(ctx, baseURL, body, auth)
		if err != nil {
			lastErr = err
			continue
		}
		if status >= http.StatusInternalServerError {
			lastErr = errUnavailable
			continue
		}
		if node.NodeID != "" {
			decoded["punchNodeId"] = node.NodeID
		}
		return decoded, status, nil
	}
	return nil, 0, lastErr
}

func punchNodeBaseURL(node OpsPunchNode) string {
	host := strings.TrimSpace(node.PublicUDPIP)
	if host == "" || node.PublicUDPPort <= 0 || node.PublicUDPPort > 65534 {
		return ""
	}
	return "http://" + net.JoinHostPort(host, strconv.Itoa(node.PublicUDPPort+1))
}

func postPunchConnectSession(ctx context.Context, baseURL string, body []byte, auth punchDeviceAuth) (map[string]any, int, error) {
	reqCtx, cancel := context.WithTimeout(ctx, punchRequestTimeout)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(reqCtx, http.MethodPost, baseURL+"/connect-sessions", bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if token := strings.TrimSpace(os.Getenv("SLAN_INTERNAL_WIRE_TOKEN")); token != "" {
		httpReq.Header.Set("X-Slan-Internal-Token", token)
	}
	httpReq.Header.Set("X-Slan-Device-ID", strings.TrimSpace(auth.DeviceID))
	httpReq.Header.Set("X-Slan-MQTT-Username", strings.TrimSpace(auth.Username))
	httpReq.Header.Set("X-Slan-Punch-Signature", strings.TrimSpace(auth.Signature))
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, 0, errUnavailable
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, 0, errUnavailable
	}
	var decoded map[string]any
	if len(bytes.TrimSpace(raw)) > 0 {
		if err := json.Unmarshal(raw, &decoded); err != nil {
			return nil, 0, fmt.Errorf("%w: punch returned non-json response", errUnavailable)
		}
	} else {
		decoded = map[string]any{}
	}
	return decoded, resp.StatusCode, nil
}

func verifyPunchMQTTSignature(cfg MQTTConfig, deviceID, username, signature string, now time.Time) bool {
	if !cfg.Enabled {
		return false
	}
	deviceID = strings.TrimSpace(deviceID)
	username = strings.TrimSpace(username)
	signature = strings.ToLower(strings.TrimSpace(signature))
	parsedDeviceID, expiresAt, ok := parseMQTTUsername(cfg, username)
	if !ok || parsedDeviceID != deviceID || expiresAt < now.Unix() {
		return false
	}
	clientID := mqttClientID(cfg, deviceID)
	password := mqttPassword(cfg, clientID, username, deviceID)
	expected := punchMQTTSignature(deviceID, password)
	return subtle.ConstantTimeCompare([]byte(expected), []byte(signature)) == 1
}

func punchMQTTSignature(deviceID, mqttPassword string) string {
	sum := md5.Sum([]byte(strings.TrimSpace(deviceID) + strings.TrimSpace(mqttPassword)))
	return hex.EncodeToString(sum[:])
}

func configuredPunchNodes() []OpsPunchNode {
	raw := strings.TrimSpace(os.Getenv("SLAN_WIRE_PUNCH_NODES"))
	if raw == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == ';' || r == '\n' })
	nodes := make([]OpsPunchNode, 0, len(parts))
	for index, part := range parts {
		addr := strings.TrimSpace(part)
		if addr == "" {
			continue
		}
		name := fmt.Sprintf("Punch %d", index+1)
		if before, after, ok := strings.Cut(addr, "="); ok {
			name = strings.TrimSpace(before)
			addr = strings.TrimSpace(after)
		}
		host, port, ok := splitPunchNodeAddr(addr)
		if !ok {
			continue
		}
		nodes = append(nodes, OpsPunchNode{
			Name:          defaultString(name, fmt.Sprintf("Punch %d", index+1)),
			Region:        strings.TrimSpace(os.Getenv("SLAN_WIRE_PUNCH_REGION_ID")),
			PublicUDPIP:   host,
			PublicUDPPort: port,
			Status:        "active",
			Health:        "healthy",
			Priority:      index + 1,
		})
	}
	return nodes
}

func splitPunchNodeAddr(raw string) (string, int, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", 0, false
	}
	if strings.Contains(raw, "://") {
		return "", 0, false
	}
	host, portText, err := net.SplitHostPort(raw)
	if err != nil {
		if h, p, ok := strings.Cut(raw, ":"); ok && !strings.Contains(p, ":") {
			host, portText = h, p
		} else {
			return "", 0, false
		}
	}
	host = strings.Trim(host, "[]")
	port, err := strconv.Atoi(strings.TrimSpace(portText))
	if err != nil || strings.TrimSpace(host) == "" || port <= 0 || port > 65535 {
		return "", 0, false
	}
	return strings.TrimSpace(host), port, true
}
