package biz

import (
	"net/http"
	"strings"
)

func (s *Server) listNetworkDevices(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": s.services.Network.ListNetworkDevices(r.PathValue("networkId"))})
}

func (s *Server) addNetworkDevice(w http.ResponseWriter, r *http.Request) {
	var req AddNetworkDeviceRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	device, enabled, err := s.services.Network.AddNetworkDevice(r.PathValue("networkId"), req)
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
	device, err := s.services.Network.UpdateNetworkDevice(r.PathValue("networkId"), r.PathValue("deviceId"), req)
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
	if err := s.services.Network.RemoveNetworkDevice(networkID, deviceID); err != nil {
		s.recordNetworkMutationAudit(r, "network_device.delete", "network_device", deviceID, networkID, "", "failed", map[string]string{"error": err.Error()})
		writeError(w, err)
		return
	}
	s.recordNetworkMutationAudit(r, "network_device.delete", "network_device", deviceID, networkID, "", "succeeded", map[string]string{"deviceId": deviceID})
	s.notifyNetworkConfigChanged(networkID, "network_device_removed", "network_device", "remove", deviceID, deviceID)
	s.notifyNetworkMemberState(networkID, deviceID, "disabled", "network_device_removed")
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
