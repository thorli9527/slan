package biz

import (
	"net/http"
	"strconv"
	"strings"
)

func (s *Server) registerOpsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/ops/auth/login", s.opsLogin)
	mux.HandleFunc("PATCH /api/ops/auth/password", s.opsChangePassword)
	mux.HandleFunc("GET /api/ops/dashboard", s.opsDashboard)
	mux.HandleFunc("GET /api/ops/audit-events", s.opsListAuditEvents)
	mux.HandleFunc("GET /api/ops/operators", s.opsListOperators)
	mux.HandleFunc("POST /api/ops/operators", s.opsCreateOperator)
	mux.HandleFunc("PATCH /api/ops/operators/{operatorId}", s.opsUpdateOperator)
	mux.HandleFunc("POST /api/ops/operators/{operatorId}/password", s.opsSetOperatorPassword)
	mux.HandleFunc("GET /api/ops/relay-nodes", s.opsListRelayNodes)
	mux.HandleFunc("POST /api/ops/relay-nodes", s.opsCreateRelayNode)
	mux.HandleFunc("PATCH /api/ops/relay-nodes/{nodeId}", s.opsUpdateRelayNode)
	mux.HandleFunc("GET /api/ops/customers", s.opsListCustomers)
	mux.HandleFunc("PATCH /api/ops/customers/{customerId}", s.opsUpdateCustomer)
	mux.HandleFunc("POST /api/ops/customers/{customerId}/assign-plan", s.opsAssignCustomerPlan)
	mux.HandleFunc("GET /api/ops/devices", s.opsListDevices)
	mux.HandleFunc("PATCH /api/ops/devices/{deviceId}", s.opsUpdateDevice)
	mux.HandleFunc("DELETE /api/ops/devices/{deviceId}", s.opsDeleteDevice)
	mux.HandleFunc("GET /api/ops/client-downloads", s.opsListClientDownloads)
	mux.HandleFunc("POST /api/ops/client-downloads", s.opsUploadClientDownload)
	mux.HandleFunc("DELETE /api/ops/client-downloads/{downloadId}", s.opsDeleteClientDownload)
	mux.HandleFunc("GET /api/ops/plans", s.opsListPlans)
	mux.HandleFunc("POST /api/ops/plans", s.opsUpsertPlan)
	mux.HandleFunc("PATCH /api/ops/plans/{planCode}", s.opsUpsertPlan)
	mux.HandleFunc("GET /api/ops/products", s.opsListProducts)
	mux.HandleFunc("POST /api/ops/products", s.opsCreateProduct)
	mux.HandleFunc("PATCH /api/ops/products/{productId}", s.opsUpdateProduct)
	mux.HandleFunc("GET /api/ops/orders", s.opsListOrders)
	mux.HandleFunc("POST /api/ops/orders", s.opsCreateOrder)
	mux.HandleFunc("PATCH /api/ops/orders/{orderId}", s.opsUpdateOrder)
	mux.HandleFunc("GET /api/ops/renewals", s.opsListRenewals)
	mux.HandleFunc("PATCH /api/ops/renewals/{renewalId}", s.opsUpdateRenewal)
}

func (s *Server) opsLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	auth, err := s.store.LoginOperatorWithRateLimit(req.Email, req.Password, clientIPFromRequest(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"auth": auth})
}

func (s *Server) opsChangePassword(w http.ResponseWriter, r *http.Request) {
	operator, err := s.requireOperator(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var req struct {
		OldPassword string `json:"oldPassword"`
		NewPassword string `json:"newPassword"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.store.ChangeOwnOperatorPassword(operator.OperatorID, req.OldPassword, req.NewPassword); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) opsDashboard(w http.ResponseWriter, r *http.Request) {
	if _, err := s.requireOperator(r); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.store.OpsDashboard())
}

func (s *Server) opsListAuditEvents(w http.ResponseWriter, r *http.Request) {
	if _, err := s.requireOperator(r); err != nil {
		writeError(w, err)
		return
	}
	query := r.URL.Query()
	limit, _ := strconv.Atoi(query.Get("limit"))
	events, err := s.store.QueryAuditEvents(AuditEventFilter{
		ActorType:    query.Get("actorType"),
		ActorID:      query.Get("actorId"),
		Action:       query.Get("action"),
		ResourceType: query.Get("resourceType"),
		ResourceID:   query.Get("resourceId"),
		Status:       query.Get("status"),
		Limit:        limit,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": events})
}

func (s *Server) opsListOperators(w http.ResponseWriter, r *http.Request) {
	if _, err := s.requireOperator(r); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListOperators()})
}

func (s *Server) opsCreateOperator(w http.ResponseWriter, r *http.Request) {
	if _, err := s.requireOperator(r); err != nil {
		writeError(w, err)
		return
	}
	var req OperatorUser
	if !decodeJSON(w, r, &req) {
		return
	}
	operator, err := s.store.UpsertOperator(req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, operator)
}

func (s *Server) opsUpdateOperator(w http.ResponseWriter, r *http.Request) {
	if _, err := s.requireOperator(r); err != nil {
		writeError(w, err)
		return
	}
	var req OperatorUser
	if !decodeJSON(w, r, &req) {
		return
	}
	req.OperatorID = r.PathValue("operatorId")
	operator, err := s.store.UpsertOperator(req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, operator)
}

func (s *Server) opsSetOperatorPassword(w http.ResponseWriter, r *http.Request) {
	if _, err := s.requireOperator(r); err != nil {
		writeError(w, err)
		return
	}
	var req struct {
		NewPassword string `json:"newPassword"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.store.SetOperatorPassword(r.PathValue("operatorId"), req.NewPassword); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) opsListRelayNodes(w http.ResponseWriter, r *http.Request) {
	if _, err := s.requireOperator(r); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListRelayNodes()})
}

func (s *Server) opsCreateRelayNode(w http.ResponseWriter, r *http.Request) {
	if _, err := s.requireOperator(r); err != nil {
		writeError(w, err)
		return
	}
	var req OpsRelayNode
	if !decodeJSON(w, r, &req) {
		return
	}
	node, err := s.store.UpsertRelayNode(req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, node)
}

func (s *Server) opsUpdateRelayNode(w http.ResponseWriter, r *http.Request) {
	if _, err := s.requireOperator(r); err != nil {
		writeError(w, err)
		return
	}
	var req OpsRelayNode
	if !decodeJSON(w, r, &req) {
		return
	}
	req.NodeID = r.PathValue("nodeId")
	node, err := s.store.UpsertRelayNode(req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, node)
}

func (s *Server) opsListCustomers(w http.ResponseWriter, r *http.Request) {
	if _, err := s.requireOperator(r); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListCustomers()})
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
	customer, err := s.store.UpdateCustomerProfile(req)
	if err != nil {
		s.recordOpsAudit(r, operator, "ops.customer.update", "customer", req.CustomerID, "failed", map[string]string{"error": err.Error(), "status": req.Status})
		writeError(w, err)
		return
	}
	s.recordOpsAudit(r, operator, "ops.customer.update", "customer", customer.CustomerID, "succeeded", map[string]string{"status": customer.Status, "email": customer.Email})
	if statusChanged && strings.EqualFold(customer.Status, "disabled") {
		for _, membership := range s.store.ListNetworkDevicesForUser(customer.CustomerID) {
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
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListOpsDevices()})
}

func (s *Server) opsUpdateDevice(w http.ResponseWriter, r *http.Request) {
	operator, err := s.requireOperator(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var req struct {
		Alias   string `json:"alias"`
		Status  string `json:"status"`
		Enabled *bool  `json:"enabled"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	memberships := s.store.ListNetworkDevicesForDevice(r.PathValue("deviceId"))
	device, err := s.store.UpdateOpsDevice(r.PathValue("deviceId"), req.Alias, req.Status, req.Enabled)
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
	if err := s.store.DeleteOpsDevice(deviceID); err != nil {
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

func (s *Server) opsAssignCustomerPlan(w http.ResponseWriter, r *http.Request) {
	operator, err := s.requireOperator(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var req struct {
		PlanCode  string  `json:"planCode"`
		ExpiresAt int64   `json:"expiresAt"`
		Amount    float64 `json:"amount"`
		Period    string  `json:"period"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	customer, renewal, err := s.store.AssignCustomerPlan(r.PathValue("customerId"), req.PlanCode, req.ExpiresAt, req.Amount, req.Period, operator.Email)
	if err != nil {
		s.recordOpsAudit(r, operator, "ops.customer.assign_plan", "customer", r.PathValue("customerId"), "failed", map[string]string{"error": err.Error(), "planCode": req.PlanCode})
		writeError(w, err)
		return
	}
	s.recordOpsAudit(r, operator, "ops.customer.assign_plan", "customer", customer.CustomerID, "succeeded", map[string]string{"planCode": req.PlanCode, "renewalId": renewal.RenewalID})
	writeJSON(w, http.StatusOK, map[string]any{"customer": customer, "renewal": renewal})
}

func (s *Server) opsListPlans(w http.ResponseWriter, r *http.Request) {
	if _, err := s.requireOperator(r); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListPlans()})
}

func (s *Server) opsUpsertPlan(w http.ResponseWriter, r *http.Request) {
	if _, err := s.requireOperator(r); err != nil {
		writeError(w, err)
		return
	}
	var req OpsPlan
	if !decodeJSON(w, r, &req) {
		return
	}
	if code := r.PathValue("planCode"); code != "" {
		req.Code = code
	}
	plan, err := s.store.UpsertPlan(req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, plan)
}

func (s *Server) opsListProducts(w http.ResponseWriter, r *http.Request) {
	if _, err := s.requireOperator(r); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListProducts()})
}

func (s *Server) opsCreateProduct(w http.ResponseWriter, r *http.Request) {
	operator, err := s.requireOperator(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var req Product
	if !decodeJSON(w, r, &req) {
		return
	}
	product, err := s.store.UpsertProduct(req)
	if err != nil {
		s.recordOpsAudit(r, operator, "ops.product.create", "product", "", "failed", map[string]string{"error": err.Error(), "planCode": req.PlanCode})
		writeError(w, err)
		return
	}
	s.recordOpsAudit(r, operator, "ops.product.create", "product", product.ProductID, "succeeded", map[string]string{"planCode": product.PlanCode, "status": product.Status})
	writeJSON(w, http.StatusCreated, product)
}

func (s *Server) opsUpdateProduct(w http.ResponseWriter, r *http.Request) {
	operator, err := s.requireOperator(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var req Product
	if !decodeJSON(w, r, &req) {
		return
	}
	req.ProductID = r.PathValue("productId")
	product, err := s.store.UpsertProduct(req)
	if err != nil {
		s.recordOpsAudit(r, operator, "ops.product.update", "product", req.ProductID, "failed", map[string]string{"error": err.Error(), "planCode": req.PlanCode})
		writeError(w, err)
		return
	}
	s.recordOpsAudit(r, operator, "ops.product.update", "product", product.ProductID, "succeeded", map[string]string{"planCode": product.PlanCode, "status": product.Status})
	writeJSON(w, http.StatusOK, product)
}

func (s *Server) opsListOrders(w http.ResponseWriter, r *http.Request) {
	if _, err := s.requireOperator(r); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListOrders()})
}

func (s *Server) opsCreateOrder(w http.ResponseWriter, r *http.Request) {
	operator, err := s.requireOperator(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var req Order
	if !decodeJSON(w, r, &req) {
		return
	}
	order, err := s.store.UpsertOrder(req)
	if err != nil {
		s.recordOpsAudit(r, operator, "ops.order.create", "order", "", "failed", map[string]string{"error": err.Error(), "customerEmail": req.CustomerEmail})
		writeError(w, err)
		return
	}
	s.recordOpsAudit(r, operator, "ops.order.create", "order", order.OrderID, "succeeded", map[string]string{"customerId": order.CustomerID, "productId": order.ProductID, "payStatus": order.PayStatus, "provisionStatus": order.ProvisionStatus})
	writeJSON(w, http.StatusCreated, order)
}

func (s *Server) opsUpdateOrder(w http.ResponseWriter, r *http.Request) {
	operator, err := s.requireOperator(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var req Order
	if !decodeJSON(w, r, &req) {
		return
	}
	req.OrderID = r.PathValue("orderId")
	order, err := s.store.UpsertOrder(req)
	if err != nil {
		s.recordOpsAudit(r, operator, "ops.order.update", "order", req.OrderID, "failed", map[string]string{"error": err.Error(), "customerEmail": req.CustomerEmail})
		writeError(w, err)
		return
	}
	s.recordOpsAudit(r, operator, "ops.order.update", "order", order.OrderID, "succeeded", map[string]string{"customerId": order.CustomerID, "productId": order.ProductID, "payStatus": order.PayStatus, "provisionStatus": order.ProvisionStatus})
	writeJSON(w, http.StatusOK, order)
}

func (s *Server) opsListRenewals(w http.ResponseWriter, r *http.Request) {
	if _, err := s.requireOperator(r); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListRenewals()})
}

func (s *Server) opsUpdateRenewal(w http.ResponseWriter, r *http.Request) {
	operator, err := s.requireOperator(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var req Renewal
	if !decodeJSON(w, r, &req) {
		return
	}
	req.RenewalID = r.PathValue("renewalId")
	renewal, err := s.store.UpdateRenewal(req)
	if err != nil {
		s.recordOpsAudit(r, operator, "ops.renewal.update", "renewal", req.RenewalID, "failed", map[string]string{"error": err.Error(), "customerEmail": req.CustomerEmail, "planCode": req.PlanCode})
		writeError(w, err)
		return
	}
	s.recordOpsAudit(r, operator, "ops.renewal.update", "renewal", renewal.RenewalID, "succeeded", map[string]string{"customerId": renewal.CustomerID, "planCode": renewal.PlanCode, "period": renewal.Period, "source": renewal.Source})
	writeJSON(w, http.StatusOK, renewal)
}

func (s *Server) requireOperator(r *http.Request) (OperatorUser, error) {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if auth == "" {
		return OperatorUser{}, errUnauthorized
	}
	return s.store.OperatorByToken(auth)
}
