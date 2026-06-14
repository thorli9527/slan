package biz

import (
	"net/http"
	"strings"
	"time"
)

func (s *Server) registerDevice(w http.ResponseWriter, r *http.Request) {
	var req RegisterDeviceRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	device, membership, err := s.services.Devices.RegisterDevice(bearerToken(r), req)
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
	device, configs, err := s.services.Devices.RenewDevice(bearerToken(r), r.PathValue("deviceId"), req)
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
	device, session, configs, err := s.services.Devices.BootstrapDeviceSession(req)
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
	writeJSON(w, http.StatusCreated, s.services.Runtime.DeviceSessionResponse(device, session, configs, timeNow()))
}

func (s *Server) bindDeviceSession(w http.ResponseWriter, r *http.Request) {
	var req DeviceSessionBindRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	device, session, configs, err := s.services.Devices.BindDeviceSession(bearerToken(r), req)
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
	writeJSON(w, http.StatusCreated, s.services.Runtime.DeviceSessionResponse(device, session, configs, timeNow()))
}

func (s *Server) renewDeviceSession(w http.ResponseWriter, r *http.Request) {
	var req DeviceRuntimeCountersRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	device, session, configs, err := s.services.Devices.RenewDeviceSession(bearerToken(r), req)
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
	writeJSON(w, http.StatusOK, s.services.Runtime.DeviceSessionResponse(device, session, configs, timeNow()))
}

func (s *Server) listDevices(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.services.Devices.ListDevices(r.URL.Query().Get("userId"))})
}

func (s *Server) listVisibleDevices(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("userId")
	if userID == "" {
		userID = r.URL.Query().Get("userId")
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": s.services.Devices.ListVisibleDevices(userID)})
}

func (s *Server) updateDeviceAlias(w http.ResponseWriter, r *http.Request) {
	var req UpdateDeviceAliasRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	device, err := s.services.Devices.UpdateAlias(r.PathValue("deviceId"), req)
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
	if err := s.services.Devices.RemoveVisibleDevice(r.PathValue("deviceId"), actorUserID); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) deviceNetworkConfigs(w http.ResponseWriter, r *http.Request) {
	configs, err := s.services.Devices.NetworkConfigsForDevice(r.PathValue("deviceId"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"deviceId": r.PathValue("deviceId"),
		"items":    configs,
	})
}
