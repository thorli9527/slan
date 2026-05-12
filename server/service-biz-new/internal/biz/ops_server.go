package biz

import (
	"net/http"
	"strings"
)

func (s *Server) registerOpsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/ops/auth/login", s.opsLogin)
	mux.HandleFunc("PATCH /api/ops/auth/password", s.opsChangePassword)
	mux.HandleFunc("GET /api/ops/dashboard", s.opsDashboard)
	mux.HandleFunc("GET /api/ops/operators", s.opsListOperators)
	mux.HandleFunc("POST /api/ops/operators", s.opsCreateOperator)
	mux.HandleFunc("PATCH /api/ops/operators/{operatorId}", s.opsUpdateOperator)
	mux.HandleFunc("POST /api/ops/operators/{operatorId}/password", s.opsSetOperatorPassword)
	mux.HandleFunc("GET /api/ops/relay-nodes", s.opsListRelayNodes)
	mux.HandleFunc("POST /api/ops/relay-nodes", s.opsCreateRelayNode)
	mux.HandleFunc("PATCH /api/ops/relay-nodes/{nodeId}", s.opsUpdateRelayNode)
	mux.HandleFunc("GET /api/ops/customers", s.opsListCustomers)
	mux.HandleFunc("POST /api/ops/customers/{customerId}/assign-plan", s.opsAssignCustomerPlan)
	mux.HandleFunc("GET /api/ops/devices", s.opsListDevices)
	mux.HandleFunc("PATCH /api/ops/devices/{deviceId}", s.opsUpdateDevice)
	mux.HandleFunc("DELETE /api/ops/devices/{deviceId}", s.opsDeleteDevice)
	mux.HandleFunc("GET /api/ops/plans", s.opsListPlans)
	mux.HandleFunc("POST /api/ops/plans", s.opsUpsertPlan)
	mux.HandleFunc("PATCH /api/ops/plans/{planCode}", s.opsUpsertPlan)
	mux.HandleFunc("GET /api/ops/products", s.opsListProducts)
	mux.HandleFunc("POST /api/ops/products", s.opsCreateProduct)
	mux.HandleFunc("PATCH /api/ops/products/{productId}", s.opsUpdateProduct)
	mux.HandleFunc("GET /api/ops/orders", s.opsListOrders)
	mux.HandleFunc("POST /api/ops/orders", s.opsCreateOrder)
	mux.HandleFunc("GET /api/ops/renewals", s.opsListRenewals)
}

func (s *Server) opsLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	auth, err := s.store.LoginOperator(req.Email, req.Password)
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

func (s *Server) opsListDevices(w http.ResponseWriter, r *http.Request) {
	if _, err := s.requireOperator(r); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListOpsDevices()})
}

func (s *Server) opsUpdateDevice(w http.ResponseWriter, r *http.Request) {
	if _, err := s.requireOperator(r); err != nil {
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
	device, err := s.store.UpdateOpsDevice(r.PathValue("deviceId"), req.Alias, req.Status, req.Enabled)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, device)
}

func (s *Server) opsDeleteDevice(w http.ResponseWriter, r *http.Request) {
	if _, err := s.requireOperator(r); err != nil {
		writeError(w, err)
		return
	}
	if err := s.store.DeleteOpsDevice(r.PathValue("deviceId")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
		writeError(w, err)
		return
	}
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
	if _, err := s.requireOperator(r); err != nil {
		writeError(w, err)
		return
	}
	var req Product
	if !decodeJSON(w, r, &req) {
		return
	}
	product, err := s.store.UpsertProduct(req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, product)
}

func (s *Server) opsUpdateProduct(w http.ResponseWriter, r *http.Request) {
	if _, err := s.requireOperator(r); err != nil {
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
		writeError(w, err)
		return
	}
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
	if _, err := s.requireOperator(r); err != nil {
		writeError(w, err)
		return
	}
	var req Order
	if !decodeJSON(w, r, &req) {
		return
	}
	order, err := s.store.UpsertOrder(req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, order)
}

func (s *Server) opsListRenewals(w http.ResponseWriter, r *http.Request) {
	if _, err := s.requireOperator(r); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": s.store.ListRenewals()})
}

func (s *Server) requireOperator(r *http.Request) (OperatorUser, error) {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if auth == "" {
		return OperatorUser{}, errUnauthorized
	}
	return s.store.OperatorByToken(auth)
}
