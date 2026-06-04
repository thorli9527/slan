package biz

import (
	"net/http"
	"strings"
)

func (s *Server) registerOpsRoutes(mux *http.ServeMux) {
	registerAPIRoutes(mux, []apiRoute{
		route(http.MethodPost, "/api/ops/auth/login", s.opsLogin),
		route(http.MethodPatch, "/api/ops/auth/password", s.opsChangePassword),
		route(http.MethodGet, "/api/ops/dashboard", s.opsDashboard),
		route(http.MethodGet, "/api/ops/audit-events", s.opsListAuditEvents),
		route(http.MethodGet, "/api/ops/operators", s.opsListOperators),
		route(http.MethodPost, "/api/ops/operators", s.opsCreateOperator),
		route(http.MethodPatch, "/api/ops/operators/{operatorId}", s.opsUpdateOperator),
		route(http.MethodPost, "/api/ops/operators/{operatorId}/password", s.opsSetOperatorPassword),
		route(http.MethodGet, "/api/ops/relay-nodes", s.opsListRelayNodes),
		route(http.MethodPost, "/api/ops/relay-nodes", s.opsCreateRelayNode),
		route(http.MethodPatch, "/api/ops/relay-nodes/{nodeId}", s.opsUpdateRelayNode),
		route(http.MethodPatch, "/api/ops/relay-nodes/{nodeId}/status", s.opsUpdateRelayNodeStatus),
		route(http.MethodDelete, "/api/ops/relay-nodes/{nodeId}", s.opsDeleteRelayNode),
		route(http.MethodGet, "/api/ops/punch-nodes", s.opsListPunchNodes),
		route(http.MethodPost, "/api/ops/punch-nodes", s.opsCreatePunchNode),
		route(http.MethodPatch, "/api/ops/punch-nodes/{nodeId}", s.opsUpdatePunchNode),
		route(http.MethodPatch, "/api/ops/punch-nodes/{nodeId}/status", s.opsUpdatePunchNodeStatus),
		route(http.MethodDelete, "/api/ops/punch-nodes/{nodeId}", s.opsDeletePunchNode),
		route(http.MethodGet, "/api/ops/customers", s.opsListCustomers),
		route(http.MethodPatch, "/api/ops/customers/{customerId}", s.opsUpdateCustomer),
		route(http.MethodPost, "/api/ops/customers/{customerId}/assign-plan", s.opsAssignCustomerPlan),
		route(http.MethodGet, "/api/ops/devices", s.opsListDevices),
		route(http.MethodPatch, "/api/ops/devices/{deviceId}", s.opsUpdateDevice),
		route(http.MethodDelete, "/api/ops/devices/{deviceId}", s.opsDeleteDevice),
		route(http.MethodGet, "/api/ops/client-downloads", s.opsListClientDownloads),
		route(http.MethodPost, "/api/ops/client-downloads", s.opsUploadClientDownload),
		route(http.MethodDelete, "/api/ops/client-downloads/{downloadId}", s.opsDeleteClientDownload),
		route(http.MethodGet, "/api/ops/plans", s.opsListPlans),
		route(http.MethodPost, "/api/ops/plans", s.opsUpsertPlan),
		route(http.MethodPatch, "/api/ops/plans/{planCode}", s.opsUpsertPlan),
		route(http.MethodGet, "/api/ops/products", s.opsListProducts),
		route(http.MethodPost, "/api/ops/products", s.opsCreateProduct),
		route(http.MethodPatch, "/api/ops/products/{productId}", s.opsUpdateProduct),
		route(http.MethodGet, "/api/ops/orders", s.opsListOrders),
		route(http.MethodPost, "/api/ops/orders", s.opsCreateOrder),
		route(http.MethodPatch, "/api/ops/orders/{orderId}", s.opsUpdateOrder),
		route(http.MethodGet, "/api/ops/renewals", s.opsListRenewals),
		route(http.MethodPatch, "/api/ops/renewals/{renewalId}", s.opsUpdateRenewal),
	})
}

func (s *Server) requireOperator(r *http.Request) (OperatorUser, error) {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if auth == "" {
		return OperatorUser{}, errUnauthorized
	}
	return s.services.Ops.OperatorByToken(auth)
}
