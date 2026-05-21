package biz

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Server 是 service-biz 的 HTTP/MQTT 业务入口。
// 它只依赖 BusinessStore 抽象，避免入口层直接绑定具体存储实现。
type Server struct {
	store BusinessStore
	mqtt  MQTTConfig
}

// NewServer 使用默认 Store 创建服务，适合本地测试或无显式数据库注入的场景。
func NewServer() *Server {
	return NewServerWithStore(NewStore())
}

// NewServerWithPostgres 使用 Postgres 初始化默认 Store，并返回业务服务。
func NewServerWithPostgres(db *sql.DB) *Server {
	return NewServerWithStore(NewStoreWithPostgres(db))
}

// NewServerWithStore 使用调用方提供的业务存储实现创建服务。
func NewServerWithStore(store BusinessStore) *Server {
	if store == nil {
		store = NewStore()
	}
	server := &Server{store: store, mqtt: mqttConfigFromEnv()}
	server.startMQTTControlSubscriber()
	server.startMQTTDeliveryRetryWorker()
	return server
}

// Routes 注册 service-biz 对外 HTTP、内部 wire、ops 和 MQTT webhook 路由。
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /api/client-downloads", s.listClientDownloads)
	mux.HandleFunc("GET /downloads/clients/{fileName}", s.downloadClientFile)

	mux.HandleFunc("POST /api/auth/register", s.registerUser)
	mux.HandleFunc("POST /api/auth/login", s.loginUser)
	mux.HandleFunc("POST /api/auth/renew", s.renewUserSession)
	mux.HandleFunc("POST /api/auth/logout", s.logoutUser)
	mux.HandleFunc("POST /api/auth/console-login-keys", s.createConsoleLoginKey)
	mux.HandleFunc("POST /api/auth/console-login", s.consoleLogin)
	mux.HandleFunc("POST /api/auth/device-login-devices", s.prepareDeviceLoginDevice)
	mux.HandleFunc("POST /api/auth/device-login-devices/{deviceId}/complete", s.completeDeviceLoginDevice)
	mux.HandleFunc("POST /api/web/device-bootstrap-keys", s.createDeviceBootstrapKey)
	mux.HandleFunc("GET /api/web/device-bootstrap-keys", s.listDeviceBootstrapKeys)
	mux.HandleFunc("POST /api/web/device-bootstrap-keys/{keyId}/revoke", s.revokeDeviceBootstrapKey)
	mux.HandleFunc("GET /api/users", s.listUsers)
	mux.HandleFunc("GET /api/users/{userId}/entitlement", s.userEntitlement)
	mux.HandleFunc("PATCH /api/users/{userId}/password", s.changeUserPassword)
	mux.HandleFunc("GET /api/user-aliases", s.listUserAliases)
	mux.HandleFunc("PATCH /api/user-aliases", s.upsertUserAlias)

	mux.HandleFunc("GET /api/devices", s.listDevices)
	mux.HandleFunc("GET /api/devices/visible", s.listVisibleDevices)
	mux.HandleFunc("POST /api/devices/register", s.registerDevice)
	mux.HandleFunc("POST /api/devices/{deviceId}/renew", s.renewDevice)
	mux.HandleFunc("POST /api/device/session/bootstrap", s.bootstrapDeviceSession)
	mux.HandleFunc("POST /api/device/session/bind", s.bindDeviceSession)
	mux.HandleFunc("POST /api/device/session/renew", s.renewDeviceSession)
	mux.HandleFunc("GET /api/devices/{deviceId}/network-configs", s.deviceNetworkConfigs)
	mux.HandleFunc("GET /api/devices/{deviceId}/mqtt-credential", s.deviceMQTTCredential)
	mux.HandleFunc("PATCH /api/devices/{deviceId}", s.updateDeviceAlias)
	mux.HandleFunc("DELETE /api/devices/{deviceId}", s.deleteDevice)
	mux.HandleFunc("POST /api/device-invites", s.createDeviceInvite)
	mux.HandleFunc("GET /api/device-invites", s.listDeviceInvites)
	mux.HandleFunc("POST /api/device-invites/accept", s.acceptDeviceInvite)

	mux.HandleFunc("GET /api/networks", s.listNetworks)
	mux.HandleFunc("POST /api/networks", s.createNetwork)
	mux.HandleFunc("PATCH /api/networks/{networkId}", s.updateNetwork)
	mux.HandleFunc("GET /api/networks/{networkId}/devices", s.listNetworkDevices)
	mux.HandleFunc("POST /api/networks/{networkId}/devices", s.addNetworkDevice)
	mux.HandleFunc("PATCH /api/networks/{networkId}/devices/{deviceId}", s.updateNetworkDevice)
	mux.HandleFunc("DELETE /api/networks/{networkId}/devices/{deviceId}", s.removeNetworkDevice)
	mux.HandleFunc("GET /api/networks/{networkId}/dns/zones", s.listDNSZones)
	mux.HandleFunc("POST /api/networks/{networkId}/dns/zones", s.addDNSZone)
	mux.HandleFunc("PATCH /api/networks/{networkId}/dns/zones/{zoneId}", s.updateDNSZone)
	mux.HandleFunc("DELETE /api/networks/{networkId}/dns/zones/{zoneId}", s.deleteDNSZone)
	mux.HandleFunc("GET /api/networks/{networkId}/dns/records", s.listDNSRecords)
	mux.HandleFunc("POST /api/networks/{networkId}/dns/records", s.addDNSRecord)
	mux.HandleFunc("PATCH /api/networks/{networkId}/dns/records/{recordId}", s.updateDNSRecord)
	mux.HandleFunc("DELETE /api/networks/{networkId}/dns/records/{recordId}", s.deleteDNSRecord)
	mux.HandleFunc("GET /api/networks/{networkId}/public-mappings", s.listPublicMappings)
	mux.HandleFunc("POST /api/networks/{networkId}/public-mappings", s.createPublicMapping)
	mux.HandleFunc("PATCH /api/networks/{networkId}/public-mappings/{mappingId}", s.updatePublicMapping)
	mux.HandleFunc("DELETE /api/networks/{networkId}/public-mappings/{mappingId}", s.deletePublicMapping)
	mux.HandleFunc("GET /api/networks/{networkId}/security-groups", s.listSecurityGroups)
	mux.HandleFunc("POST /api/networks/{networkId}/security-groups", s.createSecurityGroup)
	mux.HandleFunc("DELETE /api/networks/{networkId}/security-groups/{securityGroupId}", s.deleteSecurityGroup)
	mux.HandleFunc("GET /api/security-groups/{securityGroupId}/rules", s.listSecurityRules)
	mux.HandleFunc("POST /api/security-groups/{securityGroupId}/rules", s.addSecurityRule)
	mux.HandleFunc("PATCH /api/security-groups/rules/{ruleId}", s.updateSecurityRule)
	mux.HandleFunc("DELETE /api/security-groups/rules/{ruleId}", s.deleteSecurityRule)
	mux.HandleFunc("GET /api/networks/{networkId}/network-config", s.networkConfig)
	mux.HandleFunc("GET /api/networks/{networkId}/relay-candidates", s.relayCandidates)
	mux.HandleFunc("POST /api/networks/{networkId}/relay-candidates", s.relayCandidates)
	mux.HandleFunc("POST /api/networks/{networkId}/punch/connect-sessions", s.createPunchConnectSession)
	mux.HandleFunc("POST /api/relay/tickets", s.issueRelayTicket)

	s.registerInternalWireRoutes(mux)
	s.registerOpsRoutes(mux)

	mux.HandleFunc("POST /mqtt/bifromq/auth", s.bifroMQAuth)
	mux.HandleFunc("POST /mqtt/bifromq/check", s.bifroMQCheck)
	return withCORS(mux)
}

func (s *Server) registerUser(w http.ResponseWriter, r *http.Request) {
	var req RegisterUserRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	auth, network, err := s.store.RegisterUser(req.Email, req.Password, req.Name)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, RegisterUserResponse{Auth: auth, DefaultNetwork: network})
}

func (s *Server) loginUser(w http.ResponseWriter, r *http.Request) {
	var req LoginUserRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	auth, err := s.store.LoginUserWithRateLimit(req.Email, req.Password, clientIPFromRequest(r))
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
	auth, _ := s.store.AuthByToken(bearerToken(r))
	if err := s.store.LogoutSessions(bearerToken(r), req.DeviceToken); err != nil {
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
	token := bearerToken(r)
	key, err := s.store.CreateConsoleLoginKey(token, req.DeviceID, 2*time.Minute)
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
	auth, err := s.store.ConsumeConsoleLoginKey(req.LoginKey)
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
	if err := s.store.CheckDeviceLoginPrepareRateLimit(req.DeviceID, clientIPFromRequest(r)); err != nil {
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
	device, err := s.store.PrepareDeviceLoginDevice(req.DeviceID, req.Name, req.Platform, req.OSName, req.OSVersion, req.Alias, req.PublicKey)
	if err != nil {
		writeError(w, err)
		return
	}
	log.Printf("device login prepared device=%s platform=%s", device.DeviceID, strings.TrimSpace(req.Platform))
	writeJSON(w, http.StatusCreated, PrepareDeviceLoginResponse{DeviceID: device.DeviceID, LoginURL: deviceLoginDeviceURL(device.DeviceID), MQTT: deviceMQTTCredential(s.mqtt, device.DeviceID, timeNow())})
}

func (s *Server) completeDeviceLoginDevice(w http.ResponseWriter, r *http.Request) {
	var req CompleteDeviceLoginRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	token := req.AccessToken
	if strings.TrimSpace(token) == "" {
		token = req.Token
	}
	deviceID := r.PathValue("deviceId")
	payload, err := s.store.CompleteDeviceLoginForDevice(deviceID, token, req.Action)
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
	if configs, err := s.store.NetworkConfigsForDevice(deviceID); err == nil {
		s.notifyDeviceNetworkConfigsChanged(configs, "device_login_completed", "network_device", "add", deviceID)
	}
	writeJSON(w, http.StatusOK, CompleteDeviceLoginResponse{Status: "ok", DeviceID: deviceID, DeliveryID: deliveryID})
}

func (s *Server) createDeviceBootstrapKey(w http.ResponseWriter, r *http.Request) {
	var req CreateDeviceBootstrapKeyRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	userID := strings.TrimSpace(req.UserID)
	if userID == "" {
		if auth, err := s.store.AuthByToken(bearerToken(r)); err == nil {
			userID = auth.User.UserID
		}
	}
	key, err := s.store.CreateDeviceBootstrapKey(userID, req.NetworkID, req.DeviceAlias, req.TTLSeconds)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, key)
}

func (s *Server) listDeviceBootstrapKeys(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.URL.Query().Get("userId"))
	if userID == "" {
		if auth, err := s.store.AuthByToken(bearerToken(r)); err == nil {
			userID = auth.User.UserID
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListDeviceBootstrapKeys(userID)})
}

func (s *Server) revokeDeviceBootstrapKey(w http.ResponseWriter, r *http.Request) {
	var req RevokeDeviceBootstrapKeyRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	userID := strings.TrimSpace(req.UserID)
	if userID == "" {
		if auth, err := s.store.AuthByToken(bearerToken(r)); err == nil {
			userID = auth.User.UserID
		}
	}
	key, err := s.store.RevokeDeviceBootstrapKey(r.PathValue("keyId"), userID)
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
	if err := s.store.ChangeUserPassword(r.PathValue("userId"), req.OldPassword, req.NewPassword); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) renewUserSession(w http.ResponseWriter, r *http.Request) {
	auth, err := s.store.RenewUserSession(bearerToken(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"auth": auth})
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListUsers()})
}

func (s *Server) userEntitlement(w http.ResponseWriter, r *http.Request) {
	quota, err := s.store.DeviceQuota(r.PathValue("userId"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, quota)
}

func (s *Server) listUserAliases(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListUserAliases(r.URL.Query().Get("ownerUserId"))})
}

func (s *Server) upsertUserAlias(w http.ResponseWriter, r *http.Request) {
	var req UpsertUserAliasRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	alias, err := s.store.UpsertUserAlias(req.OwnerUserID, req.Email, req.Alias)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, alias)
}

func (s *Server) registerDevice(w http.ResponseWriter, r *http.Request) {
	var req RegisterDeviceRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	device, membership, err := s.store.RegisterDevice(req.UserID, req.DeviceID, req.Name, req.Platform, req.OSName, req.OSVersion, req.Alias, req.PublicKey)
	if err != nil {
		s.recordRequestAudit(r, AuditEvent{
			ActorType:    "user",
			ActorID:      strings.TrimSpace(req.UserID),
			Action:       "device.register",
			ResourceType: "device",
			ResourceID:   strings.TrimSpace(req.DeviceID),
			Status:       "failed",
			Details:      map[string]string{"error": err.Error(), "platform": req.Platform},
		})
		writeError(w, err)
		return
	}
	s.recordRequestAudit(r, AuditEvent{
		ActorType:    "user",
		ActorID:      device.OwnerID,
		Action:       "device.register",
		ResourceType: "device",
		ResourceID:   device.DeviceID,
		Status:       "succeeded",
		Details:      map[string]string{"platform": device.Platform, "networkId": membership.NetworkID},
	})
	s.notifyNetworkConfigChanged(membership.NetworkID, "device_registered", "network_device", "add", membership.NetworkDeviceID, device.DeviceID)
	s.notifyNetworkMemberState(membership.NetworkID, device.DeviceID, "enabled", "device_registered")
	writeJSON(w, http.StatusCreated, map[string]any{"device": device, "defaultNetworkDevice": membership, "mqtt": deviceMQTTCredential(s.mqtt, device.DeviceID, timeNow())})
}

func (s *Server) renewDevice(w http.ResponseWriter, r *http.Request) {
	var req RenewDeviceRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	device, configs, err := s.store.RenewDevice(r.PathValue("deviceId"), req.UserID, req.NetworkEnabled, req.RxBytesTotal, req.TxBytesTotal)
	if err != nil {
		writeError(w, err)
		return
	}
	now := timeNow()
	writeJSON(w, http.StatusOK, map[string]any{
		"device":         device,
		"mqtt":           deviceMQTTCredential(s.mqtt, device.DeviceID, now),
		"networkConfigs": map[string]any{"items": configs},
		"leaseExpiresAt": now.Add(3 * time.Minute).Unix(),
	})
}

func (s *Server) bootstrapDeviceSession(w http.ResponseWriter, r *http.Request) {
	var req DeviceSessionBootstrapRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	device, session, configs, err := s.store.BootstrapDeviceSession(req.SessionKey, req.DeviceID, req.Name, req.Platform, req.OSName, req.OSVersion, req.Alias, req.PublicKey)
	if err != nil {
		s.recordRequestAudit(r, AuditEvent{
			ActorType:    "device",
			Action:       "device_session.bootstrap",
			ResourceType: "device",
			ResourceID:   strings.TrimSpace(req.DeviceID),
			Status:       "failed",
			Details:      map[string]string{"error": err.Error(), "platform": req.Platform},
		})
		writeError(w, err)
		return
	}
	s.recordRequestAudit(r, AuditEvent{
		ActorType:    "device",
		ActorID:      device.DeviceID,
		Action:       "device_session.bootstrap",
		ResourceType: "device",
		ResourceID:   device.DeviceID,
		Status:       "succeeded",
		Details:      map[string]string{"userId": session.UserID, "platform": device.Platform},
	})
	s.notifyDeviceNetworkConfigsChanged(configs, "device_session_bootstrapped", "network_device", "add", device.DeviceID)
	writeJSON(w, http.StatusCreated, DeviceSessionResponse{Device: device, DeviceSession: session, MQTT: deviceMQTTCredential(s.mqtt, device.DeviceID, timeNow()), NetworkConfigs: ItemsResponse{Items: configs}})
}

func (s *Server) bindDeviceSession(w http.ResponseWriter, r *http.Request) {
	var req DeviceSessionBindRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	device, session, configs, err := s.store.BindDeviceSession(bearerToken(r), req.DeviceID, req.Name, req.Platform, req.OSName, req.OSVersion, req.Alias, req.PublicKey)
	if err != nil {
		s.recordRequestAudit(r, AuditEvent{
			ActorType:    "user",
			Action:       "device_session.bind",
			ResourceType: "device",
			ResourceID:   strings.TrimSpace(req.DeviceID),
			Status:       "failed",
			Details:      map[string]string{"error": err.Error(), "platform": req.Platform},
		})
		writeError(w, err)
		return
	}
	s.recordRequestAudit(r, AuditEvent{
		ActorType:    "user",
		ActorID:      session.UserID,
		Action:       "device_session.bind",
		ResourceType: "device",
		ResourceID:   device.DeviceID,
		Status:       "succeeded",
		Details:      map[string]string{"platform": device.Platform},
	})
	s.notifyDeviceNetworkConfigsChanged(configs, "device_session_bound", "network_device", "add", device.DeviceID)
	writeJSON(w, http.StatusCreated, DeviceSessionResponse{Device: device, DeviceSession: session, MQTT: deviceMQTTCredential(s.mqtt, device.DeviceID, timeNow()), NetworkConfigs: ItemsResponse{Items: configs}})
}

func (s *Server) renewDeviceSession(w http.ResponseWriter, r *http.Request) {
	var req DeviceRuntimeCountersRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	device, session, configs, err := s.store.RenewDeviceSession(bearerToken(r), req.NetworkEnabled, req.RxBytesTotal, req.TxBytesTotal)
	if err != nil {
		s.recordRequestAudit(r, AuditEvent{
			ActorType:    "device",
			Action:       "device_session.renew",
			ResourceType: "device_session",
			Status:       "failed",
			Details:      map[string]string{"error": err.Error()},
		})
		writeError(w, err)
		return
	}
	s.recordRequestAudit(r, AuditEvent{
		ActorType:    "device",
		ActorID:      device.DeviceID,
		Action:       "device_session.renew",
		ResourceType: "device_session",
		ResourceID:   session.SessionID,
		Status:       "succeeded",
		Details:      map[string]string{"userId": session.UserID, "networkEnabled": boolString(req.NetworkEnabled)},
	})
	writeJSON(w, http.StatusOK, DeviceSessionResponse{Device: device, DeviceSession: session, MQTT: deviceMQTTCredential(s.mqtt, device.DeviceID, timeNow()), NetworkConfigs: ItemsResponse{Items: configs}})
}

func (s *Server) listDevices(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListDevices(r.URL.Query().Get("userId"))})
}

func (s *Server) listVisibleDevices(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListVisibleDevices(r.URL.Query().Get("userId"))})
}

func (s *Server) updateDeviceAlias(w http.ResponseWriter, r *http.Request) {
	var req UpdateDeviceAliasRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	device, err := s.store.UpdateDeviceAlias(r.PathValue("deviceId"), req.ActorUserID, req.Alias)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, device)
}

func (s *Server) deleteDevice(w http.ResponseWriter, r *http.Request) {
	actorUserID := r.URL.Query().Get("actorUserId")
	if actorUserID == "" {
		var req DeleteDeviceRequest
		if decodeJSON(w, r, &req) {
			actorUserID = req.ActorUserID
		} else {
			return
		}
	}
	if err := s.store.RemoveVisibleDevice(r.PathValue("deviceId"), actorUserID); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) deviceNetworkConfigs(w http.ResponseWriter, r *http.Request) {
	configs, err := s.store.NetworkConfigsForDevice(r.PathValue("deviceId"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"deviceId": r.PathValue("deviceId"),
		"items":    configs,
	})
}

func (s *Server) listNetworks(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListNetworks(r.URL.Query().Get("userId"))})
}

func (s *Server) createNetwork(w http.ResponseWriter, r *http.Request) {
	var req CreateNetworkRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	network, group, zone, err := s.store.CreateNetwork(req.OwnerUserID, req.Name, req.Code, req.TemplateKey)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"network": network, "defaultSecurityGroup": group, "defaultDNSZone": zone})
}

func (s *Server) updateNetwork(w http.ResponseWriter, r *http.Request) {
	var req UpdateNetworkRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	network, err := s.store.UpdateNetworkFull(r.PathValue("networkId"), req.Name, req.Code, req.Status)
	if err != nil {
		writeError(w, err)
		return
	}
	reason := "network_updated"
	if network.Status == "enabled" {
		reason = "network_enabled"
	} else if network.Status == "disabled" {
		reason = "network_disabled"
	}
	s.notifyNetworkConfigChanged(network.NetworkID, reason, "network", "update", network.NetworkID, "")
	writeJSON(w, http.StatusOK, network)
}

func (s *Server) createDeviceInvite(w http.ResponseWriter, r *http.Request) {
	var req CreateDeviceInviteRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	invite, err := s.store.CreateDeviceInvite(req.InviterUserID, req.TTLSeconds)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, invite)
}

func (s *Server) listDeviceInvites(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListDeviceInvites(r.URL.Query().Get("userId"))})
}

func (s *Server) acceptDeviceInvite(w http.ResponseWriter, r *http.Request) {
	var req AcceptDeviceInviteRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	grant, invite, err := s.store.AcceptDeviceInvite(req.InviteCode, req.DeviceID, req.ActorUserID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"grant": grant, "invite": invite})
}

func (s *Server) listNetworkDevices(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListNetworkDevices(r.PathValue("networkId"))})
}

func (s *Server) addNetworkDevice(w http.ResponseWriter, r *http.Request) {
	var req AddNetworkDeviceRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	device, err := s.store.AddNetworkDevice(r.PathValue("networkId"), req.DeviceID, req.ActorUserID, req.Alias, enabled)
	if err != nil {
		s.recordNetworkMutationAudit(r, "network_device.add", "network_device", strings.TrimSpace(req.DeviceID), r.PathValue("networkId"), req.ActorUserID, "failed", map[string]string{"error": err.Error(), "enabled": boolString(enabled)})
		writeError(w, err)
		return
	}
	s.recordNetworkMutationAudit(r, "network_device.add", "network_device", device.NetworkDeviceID, device.NetworkID, req.ActorUserID, "succeeded", map[string]string{"deviceId": device.DeviceID, "enabled": boolString(device.Enabled)})
	reason := "network_device_added"
	memberState := "enabled"
	if !device.Enabled || device.Status != "active" {
		memberState = "disabled"
	}
	s.notifyNetworkConfigChanged(device.NetworkID, reason, "network_device", "add", device.NetworkDeviceID, device.DeviceID)
	s.notifyNetworkMemberState(device.NetworkID, device.DeviceID, memberState, reason)
	writeJSON(w, http.StatusCreated, device)
}

func (s *Server) updateNetworkDevice(w http.ResponseWriter, r *http.Request) {
	var req UpdateNetworkDeviceRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	device, err := s.store.UpdateNetworkDevice(r.PathValue("networkId"), r.PathValue("deviceId"), req.Alias, req.Enabled)
	if err != nil {
		s.recordNetworkMutationAudit(r, "network_device.update", "network_device", r.PathValue("deviceId"), r.PathValue("networkId"), "", "failed", map[string]string{"error": err.Error()})
		writeError(w, err)
		return
	}
	s.recordNetworkMutationAudit(r, "network_device.update", "network_device", device.NetworkDeviceID, device.NetworkID, device.OwnerUserID, "succeeded", map[string]string{"deviceId": device.DeviceID, "enabled": boolString(device.Enabled)})
	reason := "network_device_updated"
	if req.Enabled != nil && *req.Enabled {
		reason = "network_device_enabled"
	} else if req.Enabled != nil && !*req.Enabled {
		reason = "network_device_disabled"
	}
	s.notifyNetworkConfigChanged(device.NetworkID, reason, "network_device", "update", device.NetworkDeviceID, device.DeviceID)
	if req.Enabled != nil {
		memberState := "enabled"
		if !*req.Enabled {
			memberState = "disabled"
		}
		s.notifyNetworkMemberState(device.NetworkID, device.DeviceID, memberState, reason)
	}
	writeJSON(w, http.StatusOK, device)
}

func (s *Server) removeNetworkDevice(w http.ResponseWriter, r *http.Request) {
	networkID := r.PathValue("networkId")
	deviceID := r.PathValue("deviceId")
	if err := s.store.RemoveNetworkDevice(networkID, deviceID); err != nil {
		s.recordNetworkMutationAudit(r, "network_device.delete", "network_device", deviceID, networkID, "", "failed", map[string]string{"error": err.Error()})
		writeError(w, err)
		return
	}
	s.recordNetworkMutationAudit(r, "network_device.delete", "network_device", deviceID, networkID, "", "succeeded", map[string]string{"deviceId": deviceID})
	s.notifyNetworkConfigChanged(networkID, "network_device_removed", "network_device", "remove", deviceID, deviceID)
	s.notifyNetworkMemberState(networkID, deviceID, "disabled", "network_device_removed")
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) listDNSZones(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListDNSZones(r.PathValue("networkId"))})
}

func (s *Server) addDNSZone(w http.ResponseWriter, r *http.Request) {
	var req DNSZoneRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	zone, err := s.store.AddDNSZone(r.PathValue("networkId"), req.ZoneName, req.ExposeGlobal)
	if err != nil {
		s.recordNetworkMutationAudit(r, "dns_zone.add", "dns_zone", "", r.PathValue("networkId"), "", "failed", map[string]string{"error": err.Error(), "zoneName": req.ZoneName})
		writeError(w, err)
		return
	}
	s.recordNetworkMutationAudit(r, "dns_zone.add", "dns_zone", zone.ZoneID, zone.NetworkID, "", "succeeded", map[string]string{"zoneName": zone.ZoneName, "exposeGlobal": boolString(zone.ExposeGlobal)})
	s.notifyNetworkConfigChanged(zone.NetworkID, "dns_zone_added", "dns_zone", "add", zone.ZoneID, "")
	writeJSON(w, http.StatusCreated, zone)
}

func (s *Server) updateDNSZone(w http.ResponseWriter, r *http.Request) {
	var req DNSZoneRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	zone, err := s.store.UpdateDNSZone(r.PathValue("networkId"), r.PathValue("zoneId"), req.ZoneName, req.ExposeGlobal)
	if err != nil {
		s.recordNetworkMutationAudit(r, "dns_zone.update", "dns_zone", r.PathValue("zoneId"), r.PathValue("networkId"), "", "failed", map[string]string{"error": err.Error(), "zoneName": req.ZoneName})
		writeError(w, err)
		return
	}
	s.recordNetworkMutationAudit(r, "dns_zone.update", "dns_zone", zone.ZoneID, zone.NetworkID, "", "succeeded", map[string]string{"zoneName": zone.ZoneName, "exposeGlobal": boolString(zone.ExposeGlobal)})
	s.notifyNetworkConfigChanged(zone.NetworkID, "dns_zone_updated", "dns_zone", "update", zone.ZoneID, "")
	writeJSON(w, http.StatusOK, zone)
}

func (s *Server) deleteDNSZone(w http.ResponseWriter, r *http.Request) {
	networkID := r.PathValue("networkId")
	zoneID := r.PathValue("zoneId")
	if err := s.store.DeleteDNSZone(networkID, zoneID); err != nil {
		s.recordNetworkMutationAudit(r, "dns_zone.delete", "dns_zone", zoneID, networkID, "", "failed", map[string]string{"error": err.Error()})
		writeError(w, err)
		return
	}
	s.recordNetworkMutationAudit(r, "dns_zone.delete", "dns_zone", zoneID, networkID, "", "succeeded", nil)
	s.notifyNetworkConfigChanged(networkID, "dns_zone_removed", "dns_zone", "remove", zoneID, "")
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) listDNSRecords(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListDNSRecords(r.PathValue("networkId"))})
}

func (s *Server) addDNSRecord(w http.ResponseWriter, r *http.Request) {
	var req AddDNSRecordRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	record, err := s.store.AddDNSRecord(r.PathValue("networkId"), req.ZoneID, req.Name, req.RecordType, req.TargetDeviceID, req.TargetIP, req.CNAME, req.Port, req.TTL)
	if err != nil {
		s.recordNetworkMutationAudit(r, "dns_record.add", "dns_record", "", r.PathValue("networkId"), "", "failed", map[string]string{"error": err.Error(), "name": req.Name, "recordType": req.RecordType})
		writeError(w, err)
		return
	}
	s.recordNetworkMutationAudit(r, "dns_record.add", "dns_record", record.RecordID, record.NetworkID, "", "succeeded", map[string]string{"fqdn": record.FQDN, "recordType": record.RecordType, "targetDeviceId": record.TargetDeviceID})
	s.notifyNetworkConfigChanged(record.NetworkID, "dns_record_added", "dns_record", "add", record.RecordID, record.TargetDeviceID)
	writeJSON(w, http.StatusCreated, record)
}

func (s *Server) updateDNSRecord(w http.ResponseWriter, r *http.Request) {
	var req DNSRecordRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	record, err := s.store.UpdateDNSRecord(r.PathValue("networkId"), r.PathValue("recordId"), req.Name, req.RecordType, req.TargetDeviceID, req.TargetIP, req.CNAME, req.Port, req.TTL)
	if err != nil {
		s.recordNetworkMutationAudit(r, "dns_record.update", "dns_record", r.PathValue("recordId"), r.PathValue("networkId"), "", "failed", map[string]string{"error": err.Error(), "name": req.Name, "recordType": req.RecordType})
		writeError(w, err)
		return
	}
	s.recordNetworkMutationAudit(r, "dns_record.update", "dns_record", record.RecordID, record.NetworkID, "", "succeeded", map[string]string{"fqdn": record.FQDN, "recordType": record.RecordType, "targetDeviceId": record.TargetDeviceID})
	s.notifyNetworkConfigChanged(record.NetworkID, "dns_record_updated", "dns_record", "update", record.RecordID, record.TargetDeviceID)
	writeJSON(w, http.StatusOK, record)
}

func (s *Server) deleteDNSRecord(w http.ResponseWriter, r *http.Request) {
	networkID := r.PathValue("networkId")
	recordID := r.PathValue("recordId")
	if err := s.store.DeleteDNSRecord(networkID, recordID); err != nil {
		s.recordNetworkMutationAudit(r, "dns_record.delete", "dns_record", recordID, networkID, "", "failed", map[string]string{"error": err.Error()})
		writeError(w, err)
		return
	}
	s.recordNetworkMutationAudit(r, "dns_record.delete", "dns_record", recordID, networkID, "", "succeeded", nil)
	s.notifyNetworkConfigChanged(networkID, "dns_record_removed", "dns_record", "remove", recordID, "")
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) listPublicMappings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListPublicMappings(r.PathValue("networkId"))})
}

func (s *Server) createPublicMapping(w http.ResponseWriter, r *http.Request) {
	mapping, ok := s.decodePublicMapping(w, r, "")
	if !ok {
		return
	}
	s.recordNetworkMutationAudit(r, "public_mapping.add", "public_mapping", mapping.MappingID, mapping.NetworkID, "", "succeeded", map[string]string{"deviceId": mapping.DeviceID, "publicDomain": mapping.PublicDomain, "protocol": mapping.Protocol})
	writeJSON(w, http.StatusCreated, mapping)
}

func (s *Server) updatePublicMapping(w http.ResponseWriter, r *http.Request) {
	mapping, ok := s.decodePublicMapping(w, r, r.PathValue("mappingId"))
	if !ok {
		return
	}
	s.recordNetworkMutationAudit(r, "public_mapping.update", "public_mapping", mapping.MappingID, mapping.NetworkID, "", "succeeded", map[string]string{"deviceId": mapping.DeviceID, "publicDomain": mapping.PublicDomain, "protocol": mapping.Protocol, "status": mapping.Status})
	writeJSON(w, http.StatusOK, mapping)
}

func (s *Server) decodePublicMapping(w http.ResponseWriter, r *http.Request, mappingID string) (PublicDomainMapping, bool) {
	var req PublicMappingRequest
	if !decodeJSON(w, r, &req) {
		return PublicDomainMapping{}, false
	}
	mapping, err := s.store.UpsertPublicMapping(mappingID, r.PathValue("networkId"), req.Alias, req.PublicDomain, req.SourceRecord, req.DeviceID, req.Protocol, req.Port, req.ExternalPort, req.Status)
	if err != nil {
		action := "public_mapping.add"
		if strings.TrimSpace(mappingID) != "" {
			action = "public_mapping.update"
		}
		s.recordNetworkMutationAudit(r, action, "public_mapping", mappingID, r.PathValue("networkId"), "", "failed", map[string]string{"error": err.Error(), "deviceId": req.DeviceID, "publicDomain": req.PublicDomain, "protocol": req.Protocol})
		writeError(w, err)
		return PublicDomainMapping{}, false
	}
	return mapping, true
}

func (s *Server) deletePublicMapping(w http.ResponseWriter, r *http.Request) {
	networkID := r.PathValue("networkId")
	mappingID := r.PathValue("mappingId")
	if err := s.store.DeletePublicMapping(networkID, mappingID); err != nil {
		s.recordNetworkMutationAudit(r, "public_mapping.delete", "public_mapping", mappingID, networkID, "", "failed", map[string]string{"error": err.Error()})
		writeError(w, err)
		return
	}
	s.recordNetworkMutationAudit(r, "public_mapping.delete", "public_mapping", mappingID, networkID, "", "succeeded", nil)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) listSecurityGroups(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListSecurityGroups(r.PathValue("networkId"))})
}

func (s *Server) createSecurityGroup(w http.ResponseWriter, r *http.Request) {
	var req CreateSecurityGroupRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	group, err := s.store.CreateSecurityGroup(r.PathValue("networkId"), req.Name, req.Description, req.DefaultPolicy)
	if err != nil {
		s.recordNetworkMutationAudit(r, "security_group.add", "security_group", "", r.PathValue("networkId"), "", "failed", map[string]string{"error": err.Error(), "name": req.Name, "defaultPolicy": req.DefaultPolicy})
		writeError(w, err)
		return
	}
	s.recordNetworkMutationAudit(r, "security_group.add", "security_group", group.SecurityGroupID, group.NetworkID, "", "succeeded", map[string]string{"name": group.Name, "defaultPolicy": group.DefaultPolicy})
	s.notifyNetworkConfigChanged(group.NetworkID, "security_group_added", "security_group", "add", group.SecurityGroupID, "")
	writeJSON(w, http.StatusCreated, group)
}

func (s *Server) deleteSecurityGroup(w http.ResponseWriter, r *http.Request) {
	networkID := r.PathValue("networkId")
	securityGroupID := r.PathValue("securityGroupId")
	if err := s.store.DeleteSecurityGroup(networkID, securityGroupID); err != nil {
		s.recordNetworkMutationAudit(r, "security_group.delete", "security_group", securityGroupID, networkID, "", "failed", map[string]string{"error": err.Error()})
		writeError(w, err)
		return
	}
	s.recordNetworkMutationAudit(r, "security_group.delete", "security_group", securityGroupID, networkID, "", "succeeded", nil)
	s.notifyNetworkConfigChanged(networkID, "security_group_removed", "security_group", "remove", securityGroupID, "")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listSecurityRules(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListSecurityGroupRules(r.PathValue("securityGroupId"))})
}

func (s *Server) addSecurityRule(w http.ResponseWriter, r *http.Request) {
	var req SecurityRuleRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	rule, err := s.store.AddSecurityGroupRule(r.PathValue("securityGroupId"), req.Direction, req.Action, req.Protocol, req.PeerType, req.PeerValue, req.Description, req.Priority, req.PortFrom, req.PortTo, enabled)
	if err != nil {
		s.recordNetworkMutationAudit(r, "security_rule.add", "security_rule", "", "", "", "failed", map[string]string{"error": err.Error(), "securityGroupId": r.PathValue("securityGroupId"), "direction": req.Direction, "action": req.Action})
		writeError(w, err)
		return
	}
	networkID, err := s.store.SecurityGroupNetworkID(rule.SecurityGroupID)
	if err == nil {
		s.recordNetworkMutationAudit(r, "security_rule.add", "security_rule", rule.RuleID, networkID, "", "succeeded", map[string]string{"securityGroupId": rule.SecurityGroupID, "direction": rule.Direction, "action": rule.Action, "enabled": boolString(rule.Enabled)})
		s.notifyNetworkConfigChanged(networkID, securityRuleReason(rule.Direction, "added"), "security_rule", "add", rule.RuleID, "")
	}
	writeJSON(w, http.StatusCreated, rule)
}

func (s *Server) updateSecurityRule(w http.ResponseWriter, r *http.Request) {
	var req SecurityRuleRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	rule, err := s.store.UpdateSecurityGroupRule(r.PathValue("ruleId"), req.Direction, req.Action, req.Protocol, req.PeerType, req.PeerValue, req.Description, req.Priority, req.PortFrom, req.PortTo, enabled)
	if err != nil {
		s.recordNetworkMutationAudit(r, "security_rule.update", "security_rule", r.PathValue("ruleId"), "", "", "failed", map[string]string{"error": err.Error(), "direction": req.Direction, "action": req.Action})
		writeError(w, err)
		return
	}
	networkID, err := s.store.SecurityGroupNetworkID(rule.SecurityGroupID)
	if err == nil {
		s.recordNetworkMutationAudit(r, "security_rule.update", "security_rule", rule.RuleID, networkID, "", "succeeded", map[string]string{"securityGroupId": rule.SecurityGroupID, "direction": rule.Direction, "action": rule.Action, "enabled": boolString(rule.Enabled)})
		s.notifyNetworkConfigChanged(networkID, securityRuleReason(rule.Direction, "updated"), "security_rule", "update", rule.RuleID, "")
	}
	writeJSON(w, http.StatusOK, rule)
}

func (s *Server) deleteSecurityRule(w http.ResponseWriter, r *http.Request) {
	ruleID := r.PathValue("ruleId")
	rule, err := s.store.GetSecurityRule(ruleID)
	if err != nil {
		s.recordNetworkMutationAudit(r, "security_rule.delete", "security_rule", ruleID, "", "", "failed", map[string]string{"error": err.Error()})
		writeError(w, err)
		return
	}
	networkID, err := s.store.SecurityGroupNetworkID(rule.SecurityGroupID)
	if err != nil {
		s.recordNetworkMutationAudit(r, "security_rule.delete", "security_rule", ruleID, "", "", "failed", map[string]string{"error": err.Error(), "securityGroupId": rule.SecurityGroupID})
		writeError(w, err)
		return
	}
	if err := s.store.DeleteSecurityGroupRule(ruleID); err != nil {
		s.recordNetworkMutationAudit(r, "security_rule.delete", "security_rule", ruleID, networkID, "", "failed", map[string]string{"error": err.Error(), "securityGroupId": rule.SecurityGroupID})
		writeError(w, err)
		return
	}
	s.recordNetworkMutationAudit(r, "security_rule.delete", "security_rule", ruleID, networkID, "", "succeeded", map[string]string{"securityGroupId": rule.SecurityGroupID, "direction": rule.Direction})
	s.notifyNetworkConfigChanged(networkID, securityRuleReason(rule.Direction, "removed"), "security_rule", "remove", ruleID, "")
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) networkConfig(w http.ResponseWriter, r *http.Request) {
	deviceID := strings.TrimSpace(r.URL.Query().Get("deviceId"))
	if deviceID == "" {
		writeError(w, errBadRequest)
		return
	}
	config, err := s.store.NetworkConfig(r.PathValue("networkId"), deviceID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, config)
}

func (s *Server) relayCandidates(w http.ResponseWriter, r *http.Request) {
	deviceID := strings.TrimSpace(r.URL.Query().Get("deviceId"))
	if deviceID == "" && r.Method == http.MethodPost {
		var req RelayCandidatesRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		deviceID = strings.TrimSpace(req.DeviceID)
	}
	if deviceID == "" {
		writeError(w, errBadRequest)
		return
	}
	candidates, err := s.store.RelayCandidates(r.PathValue("networkId"), deviceID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": candidates})
}

func (s *Server) issueRelayTicket(w http.ResponseWriter, r *http.Request) {
	var req IssueRelayTicketRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	preferred := append([]string{}, req.PreferredRelayEndpointIDs...)
	preferred = append(preferred, req.PreferredDERPNodeIDs...)
	ticket, err := s.store.IssueRelayTicket(req.NetworkID, req.SrcNodeID, req.DstNodeID, req.DERPClusterID, preferred)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, ticket)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		writeError(w, errBadRequest)
		return false
	}
	return true
}

func bearerToken(r *http.Request) string {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	return auth
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	code := "INTERNAL"
	switch {
	case errors.Is(err, errBadRequest):
		status, code = http.StatusBadRequest, "BAD_REQUEST"
	case errors.Is(err, errUnauthorized):
		status, code = http.StatusUnauthorized, "UNAUTHORIZED"
	case errors.Is(err, errNotFound):
		status, code = http.StatusNotFound, "NOT_FOUND"
	case errors.Is(err, errConflict):
		status, code = http.StatusConflict, "CONFLICT"
	case errors.Is(err, errUnavailable):
		status, code = http.StatusServiceUnavailable, "UNAVAILABLE"
	case errors.Is(err, errRateLimited):
		status, code = http.StatusTooManyRequests, "RATE_LIMITED"
	}
	writeJSON(w, status, map[string]string{"error": code})
}

func (s *Server) recordRequestAudit(r *http.Request, event AuditEvent) {
	event.RemoteIP = clientIPFromRequest(r)
	s.store.RecordAuditEvent(event)
}

func (s *Server) recordNetworkMutationAudit(r *http.Request, action, resourceType, resourceID, networkID, actorUserID, status string, details map[string]string) {
	if details == nil {
		details = map[string]string{}
	}
	if strings.TrimSpace(networkID) != "" {
		details["networkId"] = strings.TrimSpace(networkID)
	}
	s.recordRequestAudit(r, AuditEvent{
		ActorType:    "user",
		ActorID:      strings.TrimSpace(actorUserID),
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   strings.TrimSpace(resourceID),
		Status:       status,
		Details:      details,
	})
}

func (s *Server) recordOpsAudit(r *http.Request, operator OperatorUser, action, resourceType, resourceID, status string, details map[string]string) {
	s.recordRequestAudit(r, AuditEvent{
		ActorType:    "operator",
		ActorID:      operator.OperatorID,
		ActorEmail:   operator.Email,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   strings.TrimSpace(resourceID),
		Status:       status,
		Details:      details,
	})
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func deviceLoginDeviceURL(deviceID string) string {
	base := strings.TrimSpace(os.Getenv("SLAN_WEB_CONSOLE_URL"))
	if base == "" {
		base = "http://web.dev.staticlss.com/"
	}
	parsed, err := url.Parse(base)
	if err != nil {
		return base
	}
	query := parsed.Query()
	query.Set("auth", "login")
	if strings.TrimSpace(deviceID) != "" {
		query.Set("deviceId", deviceID)
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
