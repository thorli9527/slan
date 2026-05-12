package biz

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

type networkChangePayload struct {
	NetworkID      string `json:"networkId"`
	ConfigVersion  int64  `json:"configVersion"`
	Reason         string `json:"reason"`
	ChangedAt      int64  `json:"changedAt"`
	ResourceType   string `json:"resourceType,omitempty"`
	Action         string `json:"action,omitempty"`
	ResourceID     string `json:"resourceId,omitempty"`
	DeviceID       string `json:"deviceId,omitempty"`
	EffectiveState string `json:"effectiveState,omitempty"`
}

func timeNow() time.Time {
	return time.Now()
}

func (s *Server) deviceMQTTCredential(w http.ResponseWriter, r *http.Request) {
	device, err := s.store.GetDevice(r.PathValue("deviceId"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"mqtt": deviceMQTTCredential(s.mqtt, device.DeviceID, timeNow())})
}

func (s *Server) bifroMQAuth(w http.ResponseWriter, r *http.Request) {
	var req map[string]any
	if !decodeJSON(w, r, &req) {
		return
	}
	clientID := mqttStringValue(req, "clientId", "client_id")
	username := mqttStringValue(req, "username", "userName")
	password := mqttStringValue(req, "password")
	result, ok := validateMQTTCredential(s.mqtt, clientID, username, password, timeNow())
	if !ok {
		if decoded, err := base64.StdEncoding.DecodeString(password); err == nil {
			result, ok = validateMQTTCredential(s.mqtt, clientID, username, string(decoded), timeNow())
		}
	}
	mqtt5 := mqttAuthRequestIsV5(req)
	if !ok {
		log.Printf("mqtt auth rejected clientId=%s username=%s mqtt5=%v keys=%v", clientID, username, mqtt5, mqttRequestKeys(req))
		if mqtt5 {
			writeJSON(w, http.StatusForbidden, MQTT5AuthResponse{Failed: &MQTT5AuthFailed{Code: "NotAuthorized"}})
			return
		}
		writeJSON(w, http.StatusForbidden, MQTTAuthResponse{Reject: "NotAuthorized"})
		return
	}
	userID := result.DeviceID
	if result.Principal == "server" {
		userID = mqttServerID
	}
	authOK := &MQTTAuthOK{
		TenantID: "slan",
		UserID:   userID,
		Attrs: map[string]string{
			"principal": result.Principal,
			"deviceId":  result.DeviceID,
			"userId":    userID,
		},
	}
	if mqtt5 {
		writeJSON(w, http.StatusOK, MQTT5AuthResponse{Success: authOK})
		return
	}
	writeJSON(w, http.StatusOK, MQTTAuthResponse{OK: authOK})
}

func (s *Server) bifroMQCheck(w http.ResponseWriter, r *http.Request) {
	var req map[string]any
	if !decodeJSON(w, r, &req) {
		return
	}
	principal := mqttStringValue(req, "principal")
	deviceID := mqttStringValue(req, "deviceId", "device_id")
	userID := mqttStringValue(req, "userId", "user_id")
	if userID == "" {
		userID = mqttHeaderValue(r, "user_id", "user-id", "userid", "userId")
	}
	if deviceID == "" {
		deviceID = mqttHeaderValue(r, "device_id", "device-id", "deviceId")
	}
	if principal == "" {
		if userID == mqttServerID {
			principal = "server"
		} else if userID != "" {
			principal = "device"
			deviceID = userID
		}
	}
	topic, subscribe, connect := mqttCheckTopic(req)
	allowed := connect || mqttAllowTopicAccess(s.mqtt, principal, deviceID, topic, subscribe)
	if allowed && principal == "device" && isNetworkTopic(s.mqtt, topic) {
		networkID := networkIDFromMQTTTopic(s.mqtt, topic)
		allowed = s.store.HasActiveNetworkDevice(networkID, deviceID)
	}
	w.Header().Set("Content-Type", "text/plain")
	_, _ = fmt.Fprint(w, allowed)
}

func (s *Server) notifyNetworkConfigChanged(networkID, reason, resourceType, action, resourceID, deviceID string) {
	if !s.mqtt.Enabled || strings.TrimSpace(networkID) == "" {
		return
	}
	now := timeNow().Unix()
	version := s.store.NextNetworkConfigVersion(networkID, now)
	payload := networkChangePayload{
		NetworkID:     networkID,
		ConfigVersion: version,
		Reason:        reason,
		ChangedAt:     now,
		ResourceType:  resourceType,
		Action:        action,
		ResourceID:    resourceID,
		DeviceID:      deviceID,
	}
	deviceIDs := s.store.ListActiveNetworkDeviceIDs(networkID)
	if deviceID != "" && !containsString(deviceIDs, deviceID) {
		deviceIDs = append(deviceIDs, deviceID)
	}
	go s.publishNetworkConfigChanged(deviceIDs, payload)
}

func (s *Server) notifyAuthCallback(deviceID string, payload AuthCallbackPayload) {
	if !s.mqtt.Enabled || strings.TrimSpace(deviceID) == "" {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.mqtt.PublishTimeoutMilliseconds)*time.Millisecond)
		defer cancel()
		if err := publishControlMQTT(ctx, s.mqtt, deviceID, "auth_callback", payload); err != nil {
			log.Printf("mqtt publish auth_callback failed device=%s callback=%s: %v", deviceID, payload.CallbackID, err)
		}
	}()
}

func (s *Server) publishNetworkConfigChanged(deviceIDs []string, payload networkChangePayload) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.mqtt.PublishTimeoutMilliseconds)*time.Millisecond)
	defer cancel()
	for _, deviceID := range deviceIDs {
		if err := publishControlMQTT(ctx, s.mqtt, deviceID, "network_config_changed", payload); err != nil {
			log.Printf("mqtt publish network_config_changed failed device=%s network=%s: %v", deviceID, payload.NetworkID, err)
		}
	}
}

func (s *Server) notifyNetworkMemberState(networkID, deviceID, state, reason string) {
	if !s.mqtt.Enabled || strings.TrimSpace(networkID) == "" || strings.TrimSpace(deviceID) == "" {
		return
	}
	effectiveState := state
	if state == "expired" {
		effectiveState = "disabled"
	}
	now := timeNow().Unix()
	payload := networkChangePayload{
		NetworkID:      networkID,
		Reason:         reason,
		ChangedAt:      now,
		ResourceType:   "network_member",
		Action:         effectiveState,
		DeviceID:       deviceID,
		EffectiveState: effectiveState,
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.mqtt.PublishTimeoutMilliseconds)*time.Millisecond)
		defer cancel()
		topic := mqttNetworkBroadcastTopic(s.mqtt, networkID)
		credential := serverMQTTCredential(s.mqtt, timeNow())
		if credential == nil {
			return
		}
		clientID := fmt.Sprintf("%s-member-%d", credential.ClientID, timeNow().UnixNano())
		err := mqttPublishJSON(ctx, s.mqtt, clientID, credential.Username, credential.Password, topic, controlEnvelope{
			Type:      "network_member_state_changed",
			MessageID: fmt.Sprintf("msg-%d", timeNow().UnixNano()),
			Payload:   payload,
		}, mqttQoSExactlyOnce)
		if err != nil {
			log.Printf("mqtt publish network_member_state_changed failed device=%s network=%s: %v", deviceID, networkID, err)
		}
	}()
}

func mqttStringValue(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := values[key]; ok {
			if text, ok := mqttBytesString(value); ok {
				return strings.TrimSpace(text)
			}
			return strings.TrimSpace(fmt.Sprint(value))
		}
	}
	return ""
}

func mqttBytesString(value any) (string, bool) {
	items, ok := value.([]any)
	if !ok {
		return "", false
	}
	out := make([]byte, 0, len(items))
	for _, item := range items {
		switch typed := item.(type) {
		case float64:
			if typed < 0 || typed > 255 {
				return "", false
			}
			out = append(out, byte(typed))
		case int:
			if typed < 0 || typed > 255 {
				return "", false
			}
			out = append(out, byte(typed))
		default:
			return "", false
		}
	}
	return string(out), true
}

func mqttRequestKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}

func mqttHeaderValue(r *http.Request, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(r.Header.Get(key)); value != "" {
			return value
		}
	}
	return ""
}

func mqttAuthRequestIsV5(values map[string]any) bool {
	if _, ok := values["responseInfo"]; ok {
		return true
	}
	if _, ok := values["userProps"]; ok {
		return true
	}
	if version := mqttStringValue(values, "version", "ver", "protocolVersion"); version == "5" || strings.EqualFold(version, "MQTT5") {
		return true
	}
	return false
}

func mqttCheckTopic(req map[string]any) (string, bool, bool) {
	if _, ok := req["conn"]; ok {
		return "", false, true
	}
	if sub, ok := req["sub"].(map[string]any); ok {
		return mqttTopicFromValues(sub), true, false
	}
	if pub, ok := req["pub"].(map[string]any); ok {
		return mqttTopicFromValues(pub), false, false
	}
	action := strings.ToLower(mqttStringValue(req, "action", "operation", "type"))
	if strings.Contains(action, "connect") {
		return "", false, true
	}
	subscribe := strings.Contains(action, "sub")
	if topic := mqttTopicFromValues(req); topic != "" {
		return topic, subscribe, false
	}
	for _, key := range []string{"topics", "topicFilters", "topic_filters"} {
		if values, ok := req[key].([]any); ok && len(values) > 0 {
			return strings.TrimSpace(fmt.Sprint(values[0])), true, false
		}
	}
	return "", subscribe, false
}

func mqttTopicFromValues(values map[string]any) string {
	return mqttStringValue(values, "topic", "topicFilter", "topic_filter")
}

func isNetworkTopic(cfg MQTTConfig, topic string) bool {
	return strings.HasPrefix(trimTopic(topic), mqttTopicRoot(cfg)+"/networks/")
}

func networkIDFromMQTTTopic(cfg MQTTConfig, topic string) string {
	suffix := strings.TrimPrefix(trimTopic(topic), mqttTopicRoot(cfg)+"/networks/")
	parts := strings.Split(suffix, "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func containsString(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

func securityRuleReason(direction, action string) string {
	prefix := "security_rule_ingress"
	if strings.EqualFold(direction, "egress") {
		prefix = "security_rule_egress"
	}
	return prefix + "_" + action
}
