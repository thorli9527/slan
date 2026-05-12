package biz

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type Server struct {
	store *Store
	mqtt  MQTTConfig
}

func NewServer() *Server {
	server := &Server{store: NewStore(), mqtt: mqttConfigFromEnv()}
	server.startMQTTControlSubscriber()
	return server
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("POST /api/auth/register", s.registerUser)
	mux.HandleFunc("POST /api/auth/login", s.loginUser)
	mux.HandleFunc("POST /api/auth/device-login-callbacks", s.createDeviceLoginCallback)
	mux.HandleFunc("GET /api/auth/device-login-callbacks/{callbackId}", s.deviceLoginCallbackStatus)
	mux.HandleFunc("POST /api/auth/device-login-callbacks/{callbackId}/complete", s.completeDeviceLoginCallback)
	mux.HandleFunc("GET /api/users", s.listUsers)
	mux.HandleFunc("GET /api/users/{userId}/entitlement", s.userEntitlement)
	mux.HandleFunc("PATCH /api/users/{userId}/password", s.changeUserPassword)
	mux.HandleFunc("GET /api/user-aliases", s.listUserAliases)
	mux.HandleFunc("PATCH /api/user-aliases", s.upsertUserAlias)

	mux.HandleFunc("GET /api/devices", s.listDevices)
	mux.HandleFunc("GET /api/devices/visible", s.listVisibleDevices)
	mux.HandleFunc("POST /api/devices/register", s.registerDevice)
	mux.HandleFunc("POST /api/devices/{deviceId}/renew", s.renewDevice)
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
	mux.HandleFunc("POST /api/relay/tickets", s.issueRelayTicket)

	s.registerInternalWireRoutes(mux)
	s.registerOpsRoutes(mux)

	mux.HandleFunc("POST /mqtt/bifromq/auth", s.bifroMQAuth)
	mux.HandleFunc("POST /mqtt/bifromq/check", s.bifroMQCheck)
	return withCORS(mux)
}

func (s *Server) registerUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Name     string `json:"name"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	auth, network, err := s.store.RegisterUser(req.Email, req.Password, req.Name)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"auth": auth, "defaultNetwork": network})
}

func (s *Server) loginUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	auth, err := s.store.LoginUser(req.Email, req.Password)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"auth": auth})
}

func (s *Server) createDeviceLoginCallback(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CallbackID string `json:"callbackId"`
		DeviceID   string `json:"deviceId"`
		Platform   string `json:"platform"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	callback, err := s.store.CreateDeviceLoginCallback(req.CallbackID, req.DeviceID, req.Platform, 10*time.Minute)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"callbackId": callback.CallbackID,
		"deviceId":   callback.DeviceID,
		"expiresIn":  callback.ExpiresAt - timeNow().Unix(),
		"loginUrl":   deviceLoginURL(callback),
	})
}

func (s *Server) deviceLoginCallbackStatus(w http.ResponseWriter, r *http.Request) {
	callback, ready, err := s.store.DeviceLoginCallbackStatus(r.PathValue("callbackId"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"callbackId": callback.CallbackID,
		"ready":      ready,
		"payload":    callback.Payload,
		"status":     callback.Status,
		"expiresAt":  callback.ExpiresAt,
	})
}

func (s *Server) completeDeviceLoginCallback(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AccessToken string `json:"accessToken"`
		Token       string `json:"token"`
		DeviceID    string `json:"deviceId"`
		Action      string `json:"action"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	token := req.AccessToken
	if strings.TrimSpace(token) == "" {
		token = req.Token
	}
	callback, err := s.store.CompleteDeviceLoginCallback(r.PathValue("callbackId"), token, req.DeviceID, req.Action)
	if err != nil {
		writeError(w, err)
		return
	}
	if callback.Payload != nil && callback.Payload.DeviceID != nil {
		s.notifyAuthCallback(*callback.Payload.DeviceID, *callback.Payload)
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "callback": callback})
}

func (s *Server) changeUserPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OldPassword string `json:"oldPassword"`
		NewPassword string `json:"newPassword"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.store.ChangeUserPassword(r.PathValue("userId"), req.OldPassword, req.NewPassword); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
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
	var req struct {
		OwnerUserID string `json:"ownerUserId"`
		Email       string `json:"email"`
		Alias       string `json:"alias"`
	}
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
	var req struct {
		UserID    string `json:"userId"`
		DeviceID  string `json:"deviceId"`
		Name      string `json:"name"`
		Platform  string `json:"platform"`
		OSName    string `json:"osName"`
		OSVersion string `json:"osVersion"`
		Alias     string `json:"alias"`
		PublicKey string `json:"publicKey"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	device, membership, err := s.store.RegisterDevice(req.UserID, req.DeviceID, req.Name, req.Platform, req.OSName, req.OSVersion, req.Alias, req.PublicKey)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"device": device, "defaultNetworkDevice": membership, "mqtt": deviceMQTTCredential(s.mqtt, device.DeviceID, timeNow())})
}

func (s *Server) renewDevice(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID         string `json:"userId"`
		NetworkEnabled bool   `json:"networkEnabled"`
		RxBytesTotal   uint64 `json:"rxBytesTotal"`
		TxBytesTotal   uint64 `json:"txBytesTotal"`
	}
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

func (s *Server) listDevices(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListDevices(r.URL.Query().Get("userId"))})
}

func (s *Server) listVisibleDevices(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListVisibleDevices(r.URL.Query().Get("userId"))})
}

func (s *Server) updateDeviceAlias(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ActorUserID string `json:"actorUserId"`
		Alias       string `json:"alias"`
	}
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
		var req struct {
			ActorUserID string `json:"actorUserId"`
		}
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
	var req struct {
		OwnerUserID string `json:"ownerUserId"`
		Name        string `json:"name"`
		Code        string `json:"code"`
		TemplateKey string `json:"templateKey"`
	}
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
	var req struct {
		Name   string `json:"name"`
		Code   string `json:"code"`
		Status string `json:"status"`
	}
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
	var req struct {
		InviterUserID string `json:"inviterUserId"`
		TTLSeconds    int64  `json:"ttlSeconds"`
	}
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
	var req struct {
		InviteCode  string `json:"inviteCode"`
		DeviceID    string `json:"deviceId"`
		ActorUserID string `json:"actorUserId"`
	}
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
	var req struct {
		DeviceID    string `json:"deviceId"`
		ActorUserID string `json:"actorUserId"`
		Alias       string `json:"alias"`
		Enabled     *bool  `json:"enabled"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	device, err := s.store.AddNetworkDevice(r.PathValue("networkId"), req.DeviceID, req.ActorUserID, req.Alias, enabled)
	if err != nil {
		writeError(w, err)
		return
	}
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
	var req struct {
		Alias   string `json:"alias"`
		Enabled *bool  `json:"enabled"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	device, err := s.store.UpdateNetworkDevice(r.PathValue("networkId"), r.PathValue("deviceId"), req.Alias, req.Enabled)
	if err != nil {
		writeError(w, err)
		return
	}
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
		writeError(w, err)
		return
	}
	s.notifyNetworkConfigChanged(networkID, "network_device_removed", "network_device", "remove", deviceID, deviceID)
	s.notifyNetworkMemberState(networkID, deviceID, "disabled", "network_device_removed")
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) listDNSZones(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListDNSZones(r.PathValue("networkId"))})
}

func (s *Server) addDNSZone(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ZoneName     string `json:"zoneName"`
		ExposeGlobal bool   `json:"exposeGlobal"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	zone, err := s.store.AddDNSZone(r.PathValue("networkId"), req.ZoneName, req.ExposeGlobal)
	if err != nil {
		writeError(w, err)
		return
	}
	s.notifyNetworkConfigChanged(zone.NetworkID, "dns_zone_added", "dns_zone", "add", zone.ZoneID, "")
	writeJSON(w, http.StatusCreated, zone)
}

func (s *Server) updateDNSZone(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ZoneName     string `json:"zoneName"`
		ExposeGlobal bool   `json:"exposeGlobal"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	zone, err := s.store.UpdateDNSZone(r.PathValue("networkId"), r.PathValue("zoneId"), req.ZoneName, req.ExposeGlobal)
	if err != nil {
		writeError(w, err)
		return
	}
	s.notifyNetworkConfigChanged(zone.NetworkID, "dns_zone_updated", "dns_zone", "update", zone.ZoneID, "")
	writeJSON(w, http.StatusOK, zone)
}

func (s *Server) deleteDNSZone(w http.ResponseWriter, r *http.Request) {
	networkID := r.PathValue("networkId")
	zoneID := r.PathValue("zoneId")
	if err := s.store.DeleteDNSZone(networkID, zoneID); err != nil {
		writeError(w, err)
		return
	}
	s.notifyNetworkConfigChanged(networkID, "dns_zone_removed", "dns_zone", "remove", zoneID, "")
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) listDNSRecords(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListDNSRecords(r.PathValue("networkId"))})
}

func (s *Server) addDNSRecord(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ZoneID         string `json:"zoneId"`
		Name           string `json:"name"`
		RecordType     string `json:"recordType"`
		TargetDeviceID string `json:"targetDeviceId"`
		TargetIP       string `json:"targetIp"`
		CNAME          string `json:"cname"`
		Port           string `json:"port"`
		TTL            int    `json:"ttl"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	record, err := s.store.AddDNSRecord(r.PathValue("networkId"), req.ZoneID, req.Name, req.RecordType, req.TargetDeviceID, req.TargetIP, req.CNAME, req.Port, req.TTL)
	if err != nil {
		writeError(w, err)
		return
	}
	s.notifyNetworkConfigChanged(record.NetworkID, "dns_record_added", "dns_record", "add", record.RecordID, record.TargetDeviceID)
	writeJSON(w, http.StatusCreated, record)
}

func (s *Server) updateDNSRecord(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name           string `json:"name"`
		RecordType     string `json:"recordType"`
		TargetDeviceID string `json:"targetDeviceId"`
		TargetIP       string `json:"targetIp"`
		CNAME          string `json:"cname"`
		Port           string `json:"port"`
		TTL            int    `json:"ttl"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	record, err := s.store.UpdateDNSRecord(r.PathValue("networkId"), r.PathValue("recordId"), req.Name, req.RecordType, req.TargetDeviceID, req.TargetIP, req.CNAME, req.Port, req.TTL)
	if err != nil {
		writeError(w, err)
		return
	}
	s.notifyNetworkConfigChanged(record.NetworkID, "dns_record_updated", "dns_record", "update", record.RecordID, record.TargetDeviceID)
	writeJSON(w, http.StatusOK, record)
}

func (s *Server) deleteDNSRecord(w http.ResponseWriter, r *http.Request) {
	networkID := r.PathValue("networkId")
	recordID := r.PathValue("recordId")
	if err := s.store.DeleteDNSRecord(networkID, recordID); err != nil {
		writeError(w, err)
		return
	}
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
	writeJSON(w, http.StatusCreated, mapping)
}

func (s *Server) updatePublicMapping(w http.ResponseWriter, r *http.Request) {
	mapping, ok := s.decodePublicMapping(w, r, r.PathValue("mappingId"))
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, mapping)
}

func (s *Server) decodePublicMapping(w http.ResponseWriter, r *http.Request, mappingID string) (PublicDomainMapping, bool) {
	var req struct {
		Alias        string `json:"alias"`
		PublicDomain string `json:"publicDomain"`
		SourceRecord string `json:"sourceRecord"`
		DeviceID     string `json:"deviceId"`
		Protocol     string `json:"protocol"`
		Port         string `json:"port"`
		ExternalPort string `json:"externalPort"`
		Status       string `json:"status"`
	}
	if !decodeJSON(w, r, &req) {
		return PublicDomainMapping{}, false
	}
	mapping, err := s.store.UpsertPublicMapping(mappingID, r.PathValue("networkId"), req.Alias, req.PublicDomain, req.SourceRecord, req.DeviceID, req.Protocol, req.Port, req.ExternalPort, req.Status)
	if err != nil {
		writeError(w, err)
		return PublicDomainMapping{}, false
	}
	return mapping, true
}

func (s *Server) deletePublicMapping(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeletePublicMapping(r.PathValue("networkId"), r.PathValue("mappingId")); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) listSecurityGroups(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListSecurityGroups(r.PathValue("networkId"))})
}

func (s *Server) createSecurityGroup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name          string `json:"name"`
		Description   string `json:"description"`
		DefaultPolicy string `json:"defaultPolicy"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	group, err := s.store.CreateSecurityGroup(r.PathValue("networkId"), req.Name, req.Description, req.DefaultPolicy)
	if err != nil {
		writeError(w, err)
		return
	}
	s.notifyNetworkConfigChanged(group.NetworkID, "security_group_added", "security_group", "add", group.SecurityGroupID, "")
	writeJSON(w, http.StatusCreated, group)
}

func (s *Server) deleteSecurityGroup(w http.ResponseWriter, r *http.Request) {
	networkID := r.PathValue("networkId")
	securityGroupID := r.PathValue("securityGroupId")
	if err := s.store.DeleteSecurityGroup(networkID, securityGroupID); err != nil {
		writeError(w, err)
		return
	}
	s.notifyNetworkConfigChanged(networkID, "security_group_removed", "security_group", "remove", securityGroupID, "")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listSecurityRules(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListSecurityGroupRules(r.PathValue("securityGroupId"))})
}

func (s *Server) addSecurityRule(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Direction   string `json:"direction"`
		Priority    int    `json:"priority"`
		Action      string `json:"action"`
		Protocol    string `json:"protocol"`
		PortFrom    int    `json:"portFrom"`
		PortTo      int    `json:"portTo"`
		PeerType    string `json:"peerType"`
		PeerValue   string `json:"peerValue"`
		Description string `json:"description"`
		Enabled     *bool  `json:"enabled"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	rule, err := s.store.AddSecurityGroupRule(r.PathValue("securityGroupId"), req.Direction, req.Action, req.Protocol, req.PeerType, req.PeerValue, req.Description, req.Priority, req.PortFrom, req.PortTo, enabled)
	if err != nil {
		writeError(w, err)
		return
	}
	networkID, err := s.store.SecurityGroupNetworkID(rule.SecurityGroupID)
	if err == nil {
		s.notifyNetworkConfigChanged(networkID, securityRuleReason(rule.Direction, "added"), "security_rule", "add", rule.RuleID, "")
	}
	writeJSON(w, http.StatusCreated, rule)
}

func (s *Server) updateSecurityRule(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Direction   string `json:"direction"`
		Priority    int    `json:"priority"`
		Action      string `json:"action"`
		Protocol    string `json:"protocol"`
		PortFrom    int    `json:"portFrom"`
		PortTo      int    `json:"portTo"`
		PeerType    string `json:"peerType"`
		PeerValue   string `json:"peerValue"`
		Description string `json:"description"`
		Enabled     *bool  `json:"enabled"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	rule, err := s.store.UpdateSecurityGroupRule(r.PathValue("ruleId"), req.Direction, req.Action, req.Protocol, req.PeerType, req.PeerValue, req.Description, req.Priority, req.PortFrom, req.PortTo, enabled)
	if err != nil {
		writeError(w, err)
		return
	}
	networkID, err := s.store.SecurityGroupNetworkID(rule.SecurityGroupID)
	if err == nil {
		s.notifyNetworkConfigChanged(networkID, securityRuleReason(rule.Direction, "updated"), "security_rule", "update", rule.RuleID, "")
	}
	writeJSON(w, http.StatusOK, rule)
}

func (s *Server) deleteSecurityRule(w http.ResponseWriter, r *http.Request) {
	ruleID := r.PathValue("ruleId")
	rule, err := s.store.GetSecurityRule(ruleID)
	if err != nil {
		writeError(w, err)
		return
	}
	networkID, err := s.store.SecurityGroupNetworkID(rule.SecurityGroupID)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.store.DeleteSecurityGroupRule(ruleID); err != nil {
		writeError(w, err)
		return
	}
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
		var req struct {
			DeviceID string `json:"deviceId"`
		}
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
	var req struct {
		NetworkID                 string   `json:"networkId"`
		SrcNodeID                 string   `json:"srcNodeId"`
		DstNodeID                 string   `json:"dstNodeId"`
		DERPClusterID             string   `json:"derpClusterId"`
		PreferredDERPNodeIDs      []string `json:"preferredDerpNodeIds"`
		PreferredRelayEndpointIDs []string `json:"preferredRelayEndpointIds"`
		Reason                    string   `json:"reason"`
		RelayRegionID             string   `json:"relayRegionId"`
	}
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
	}
	writeJSON(w, status, map[string]string{"error": code})
}

func deviceLoginURL(callback DeviceLoginCallback) string {
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
	query.Set("callbackId", callback.CallbackID)
	if strings.TrimSpace(callback.DeviceID) != "" {
		query.Set("deviceId", callback.DeviceID)
	}
	if strings.TrimSpace(callback.Platform) != "" {
		query.Set("clientPlatform", callback.Platform)
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
