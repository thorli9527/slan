package biz

import (
	"net/http"
	"strings"
)

func (s *Server) opsListCustomers(w http.ResponseWriter, r *http.Request) {
	if _, err := s.requireOperator(r); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": s.services.Ops.ListCustomers()})
}

func (s *Server) opsUpdateCustomer(w http.ResponseWriter, r *http.Request) {
	operator, err := s.requireOperator(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var req CustomerProfile
	if !decodeJSON(w, r, &req) {
		return
	}
	req.CustomerID = r.PathValue("customerId")
	statusChanged := strings.TrimSpace(req.Status) != ""
	customer, err := s.services.Ops.UpdateCustomer(req)
	if err != nil {
		s.recordOpsAudit(r, operator, "ops.customer.update", "customer", req.CustomerID, "failed", map[string]string{"error": err.Error(), "status": req.Status})
		writeError(w, err)
		return
	}
	s.recordOpsAudit(r, operator, "ops.customer.update", "customer", customer.CustomerID, "succeeded", map[string]string{"status": customer.Status, "email": customer.Email})
	if statusChanged && strings.EqualFold(customer.Status, "disabled") {
		for _, membership := range s.services.Network.ListNetworkDevicesForUser(customer.CustomerID) {
			s.notifyNetworkConfigChanged(membership.NetworkID, "ops_customer_disabled", "ops_customer", "update", customer.CustomerID, membership.DeviceID)
			s.notifyNetworkMemberState(membership.NetworkID, membership.DeviceID, "disabled", "ops_customer_disabled")
		}
	}
	writeJSON(w, http.StatusOK, customer)
}

func (s *Server) opsListDevices(w http.ResponseWriter, r *http.Request) {
	if _, err := s.requireOperator(r); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": s.services.Ops.ListDevices()})
}

func (s *Server) opsUpdateDevice(w http.ResponseWriter, r *http.Request) {
	operator, err := s.requireOperator(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var req OpsUpdateDeviceRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	memberships := s.services.Network.ListNetworkDevicesForDevice(r.PathValue("deviceId"))
	device, err := s.services.Ops.UpdateDevice(r.PathValue("deviceId"), req)
	if err != nil {
		s.recordOpsAudit(r, operator, "ops.device.update", "device", r.PathValue("deviceId"), "failed", map[string]string{"error": err.Error(), "status": req.Status})
		writeError(w, err)
		return
	}
	s.recordOpsAudit(r, operator, "ops.device.update", "device", device.DeviceID, "succeeded", map[string]string{"status": device.Status, "deviceEnabled": boolString(device.DeviceEnabled)})
	statusChanged := strings.TrimSpace(req.Status) != ""
	if req.Enabled != nil || statusChanged {
		memberState := "enabled"
		reason := "ops_device_enabled"
		disabled := req.Enabled != nil && !*req.Enabled
		if statusChanged && statusIsManagedDisabled(req.Status) {
			disabled = true
		}
		if disabled {
			memberState = "disabled"
			reason = "ops_device_disabled"
		}
		for _, membership := range memberships {
			s.notifyNetworkConfigChanged(membership.NetworkID, reason, "ops_device", "update", device.DeviceID, device.DeviceID)
			s.notifyNetworkMemberState(membership.NetworkID, device.DeviceID, memberState, reason)
		}
	}
	writeJSON(w, http.StatusOK, device)
}

func (s *Server) opsDeleteDevice(w http.ResponseWriter, r *http.Request) {
	operator, err := s.requireOperator(r)
	if err != nil {
		writeError(w, err)
		return
	}
	deviceID := r.PathValue("deviceId")
	if err := s.services.Ops.DeleteDevice(deviceID); err != nil {
		s.recordOpsAudit(r, operator, "ops.device.delete", "device", deviceID, "failed", map[string]string{"error": err.Error()})
		writeError(w, err)
		return
	}
	s.recordOpsAudit(r, operator, "ops.device.delete", "device", deviceID, "succeeded", nil)
	w.WriteHeader(http.StatusNoContent)
}

func statusIsManagedDisabled(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "disabled", "suspended", "blocked", "revoked", "deleted", "removed":
		return true
	default:
		return false
	}
}
