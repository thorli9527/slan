package biz

import (
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
)

func (s *Server) deviceMQTTCredential(w http.ResponseWriter, r *http.Request) {
	device, err := s.services.MQTT.Device(r.PathValue("deviceId"))
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
		allowed = s.services.MQTT.HasActiveNetworkDevice(networkID, deviceID)
	}
	w.Header().Set("Content-Type", "text/plain")
	_, _ = fmt.Fprint(w, allowed)
}
