package biz

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

func (s *Server) registerUser(w http.ResponseWriter, r *http.Request) {
	var req RegisterUserRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	response, err := s.services.Auth.RegisterUser(req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, response)
}

func (s *Server) loginUser(w http.ResponseWriter, r *http.Request) {
	var req LoginUserRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	auth, err := s.services.Auth.LoginUser(req, clientIPFromRequest(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, AuthEnvelopeResponse{Auth: auth})
}

func (s *Server) logoutUser(w http.ResponseWriter, r *http.Request) {
	var req LogoutUserRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	auth, err := s.services.Auth.Logout(bearerToken(r), req.DeviceToken)
	if err != nil {
		s.recordRequestAudit(r, AuditEvent{
			ActorType:    "user",
			ActorID:      auth.User.UserID,
			ActorEmail:   auth.User.Email,
			Action:       "auth.logout",
			ResourceType: "auth",
			Status:       "failed",
			Details:      map[string]string{"error": err.Error(), "hasDeviceToken": boolString(strings.TrimSpace(req.DeviceToken) != "")},
		})
		writeError(w, err)
		return
	}
	s.recordRequestAudit(r, AuditEvent{
		ActorType:    "user",
		ActorID:      auth.User.UserID,
		ActorEmail:   auth.User.Email,
		Action:       "auth.logout",
		ResourceType: "auth",
		Status:       "succeeded",
		Details:      map[string]string{"hasDeviceToken": boolString(strings.TrimSpace(req.DeviceToken) != "")},
	})
	writeJSON(w, http.StatusOK, StatusResponse{Status: "ok"})
}

func (s *Server) createConsoleLoginKey(w http.ResponseWriter, r *http.Request) {
	var req CreateConsoleLoginKeyRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	key, err := s.services.Auth.CreateConsoleLoginKey(bearerToken(r), req)
	if err != nil {
		s.recordRequestAudit(r, AuditEvent{
			ActorType:    "user",
			Action:       "console_login_key.create",
			ResourceType: "console_login_key",
			ResourceID:   strings.TrimSpace(req.DeviceID),
			Status:       "failed",
			Details:      map[string]string{"error": err.Error()},
		})
		writeError(w, err)
		return
	}
	s.recordRequestAudit(r, AuditEvent{
		ActorType:    "user",
		ActorID:      key.UserID,
		Action:       "console_login_key.create",
		ResourceType: "console_login_key",
		ResourceID:   key.DeviceID,
		Status:       "succeeded",
		Details:      map[string]string{"deviceId": key.DeviceID},
	})
	writeJSON(w, http.StatusCreated, key)
}

func (s *Server) consoleLogin(w http.ResponseWriter, r *http.Request) {
	var req ConsoleLoginRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	auth, err := s.services.Auth.ConsoleLogin(req)
	if err != nil {
		s.recordRequestAudit(r, AuditEvent{
			ActorType:    "user",
			Action:       "console_login_key.consume",
			ResourceType: "console_login_key",
			Status:       "failed",
			Details:      map[string]string{"error": err.Error()},
		})
		writeError(w, err)
		return
	}
	s.recordRequestAudit(r, AuditEvent{
		ActorType:    "user",
		ActorID:      auth.User.UserID,
		ActorEmail:   auth.User.Email,
		Action:       "console_login_key.consume",
		ResourceType: "console_login_key",
		Status:       "succeeded",
	})
	writeJSON(w, http.StatusOK, AuthEnvelopeResponse{Auth: auth})
}

func (s *Server) prepareDeviceLoginDevice(w http.ResponseWriter, r *http.Request) {
	var req DeviceIdentityRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	device, err := s.services.Auth.PrepareDeviceLogin(req, clientIPFromRequest(r))
	if err != nil {
		s.recordRequestAudit(r, AuditEvent{
			ActorType:    "device",
			Action:       "device_login.prepare",
			ResourceType: "device",
			ResourceID:   strings.TrimSpace(req.DeviceID),
			Status:       "rate_limited",
			Details:      map[string]string{"platform": req.Platform},
		})
		writeError(w, err)
		return
	}
	log.Printf("device login prepared device=%s platform=%s", device.DeviceID, strings.TrimSpace(req.Platform))
	writeJSON(w, http.StatusCreated, PrepareDeviceLoginResponse{
		DeviceID: device.DeviceID,
		LoginURL: deviceLoginDeviceURL(device.DeviceID),
		MQTT:     deviceMQTTCredential(s.mqtt, device.DeviceID, timeNow()),
	})
}

func (s *Server) completeDeviceLoginDevice(w http.ResponseWriter, r *http.Request) {
	var req CompleteDeviceLoginRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	deviceID := r.PathValue("deviceId")
	payload, err := s.services.Auth.CompleteDeviceLogin(deviceID, req)
	if err != nil {
		log.Printf("device login complete failed device=%s err=%v", deviceID, err)
		s.recordRequestAudit(r, AuditEvent{
			ActorType:    "user",
			Action:       "device_login.complete",
			ResourceType: "device",
			ResourceID:   deviceID,
			Status:       "failed",
			Details:      map[string]string{"error": err.Error(), "action": req.Action},
		})
		writeError(w, err)
		return
	}
	log.Printf("device login completed device=%s user=%s", deviceID, payload.UserID)
	s.recordRequestAudit(r, AuditEvent{
		ActorType:    "user",
		ActorID:      payload.UserID,
		ActorEmail:   payload.UserLabel,
		Action:       "device_login.complete",
		ResourceType: "device",
		ResourceID:   deviceID,
		Status:       "succeeded",
		Details:      map[string]string{"action": payload.Action},
	})
	deliveryID, err := s.notifyDeviceUserLoginSucceeded(deviceID, payload)
	if err != nil {
		log.Printf("device login mqtt enqueue failed device=%s user=%s err=%v", deviceID, payload.UserID, err)
		s.recordRequestAudit(r, AuditEvent{
			ActorType:    "user",
			ActorID:      payload.UserID,
			ActorEmail:   payload.UserLabel,
			Action:       "device_login.notify",
			ResourceType: "device",
			ResourceID:   deviceID,
			Status:       "failed",
			Details:      map[string]string{"error": err.Error(), "action": payload.Action},
		})
		writeError(w, err)
		return
	}
	if configs, err := s.services.Devices.NetworkConfigsForDevice(deviceID); err == nil {
		s.notifyDeviceNetworkConfigsChanged(configs, "device_login_completed", "network_device", "add", deviceID)
	}
	writeJSON(w, http.StatusOK, CompleteDeviceLoginResponse{Status: "ok", DeviceID: deviceID, DeliveryID: deliveryID})
}

func (s *Server) createDeviceBootstrapKey(w http.ResponseWriter, r *http.Request) {
	var req CreateDeviceBootstrapKeyRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	key, err := s.services.Auth.CreateDeviceBootstrapKey(bearerToken(r), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, key)
}

func (s *Server) listDeviceBootstrapKeys(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.services.Auth.ListDeviceBootstrapKeys(bearerToken(r), r.URL.Query().Get("userId"))})
}

func (s *Server) revokeDeviceBootstrapKey(w http.ResponseWriter, r *http.Request) {
	var req RevokeDeviceBootstrapKeyRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	key, err := s.services.Auth.RevokeDeviceBootstrapKey(bearerToken(r), r.PathValue("keyId"), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, key)
}

func (s *Server) changeUserPassword(w http.ResponseWriter, r *http.Request) {
	var req ChangeUserPasswordRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.services.Auth.ChangeUserPassword(r.PathValue("userId"), req); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) renewUserSession(w http.ResponseWriter, r *http.Request) {
	auth, err := s.services.Auth.RenewUserSession(bearerToken(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"auth": auth})
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.services.Auth.ListUsers()})
}

func (s *Server) userEntitlement(w http.ResponseWriter, r *http.Request) {
	quota, err := s.services.Auth.UserEntitlement(r.PathValue("userId"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, quota)
}

func (s *Server) listUserAliases(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.services.Auth.ListUserAliases(r.URL.Query().Get("ownerUserId"))})
}

func (s *Server) upsertUserAlias(w http.ResponseWriter, r *http.Request) {
	var req UpsertUserAliasRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	alias, err := s.services.Auth.UpsertUserAlias(req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, alias)
}
