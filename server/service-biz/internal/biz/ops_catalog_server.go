package biz

import "net/http"

func (s *Server) opsAssignCustomerPlan(w http.ResponseWriter, r *http.Request) {
	operator, err := s.requireOperator(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var req OpsAssignCustomerPlanRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	customer, renewal, err := s.services.Ops.AssignCustomerPlan(r.PathValue("customerId"), req, operator.Email)
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
	writeJSON(w, http.StatusOK, map[string]any{"items": s.services.Ops.ListPlans()})
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
	plan, err := s.services.Ops.UpsertPlan(req)
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
	writeJSON(w, http.StatusOK, map[string]any{"items": s.services.Ops.ListProducts()})
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
	product, err := s.services.Ops.UpsertProduct(req)
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
	product, err := s.services.Ops.UpsertProduct(req)
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
	writeJSON(w, http.StatusOK, map[string]any{"items": s.services.Ops.ListOrders()})
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
	order, err := s.services.Ops.UpsertOrder(req)
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
	order, err := s.services.Ops.UpsertOrder(req)
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
	writeJSON(w, http.StatusOK, map[string]any{"items": s.services.Ops.ListRenewals()})
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
	renewal, err := s.services.Ops.UpdateRenewal(req)
	if err != nil {
		s.recordOpsAudit(r, operator, "ops.renewal.update", "renewal", req.RenewalID, "failed", map[string]string{"error": err.Error(), "customerEmail": req.CustomerEmail, "planCode": req.PlanCode})
		writeError(w, err)
		return
	}
	s.recordOpsAudit(r, operator, "ops.renewal.update", "renewal", renewal.RenewalID, "succeeded", map[string]string{"customerId": renewal.CustomerID, "planCode": renewal.PlanCode, "period": renewal.Period, "source": renewal.Source})
	writeJSON(w, http.StatusOK, renewal)
}
