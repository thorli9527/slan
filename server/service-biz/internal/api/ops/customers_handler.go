package ops

import (
	"net/http"

	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type CustomerHandler struct {
	OpsCustomers servicepkg.OpsCustomerUseCase
}

func (h CustomerHandler) Routes() []serviceapi.Route {
	return withOptAliases([]serviceapi.Route{
		serviceapi.NewRoute(http.MethodGet, "/api/ops/customers", h.OpsListCustomers),
		serviceapi.NewRoute(http.MethodPost, "/api/ops/customers", h.OpsCreateCustomer),
		serviceapi.NewRoute(http.MethodPatch, "/api/ops/customers/{customerId}", h.OpsUpdateCustomer),
	})
}

func (h CustomerHandler) OpsCreateCustomer(w http.ResponseWriter, r *http.Request) {
	var req updateCustomerRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	item, err := h.OpsCustomers.CreateCustomer(r.Context(), req.toCreateInput())
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusCreated, customerPayload(item))
}

func (h CustomerHandler) OpsListCustomers(w http.ResponseWriter, r *http.Request) {
	items, err := h.OpsCustomers.ListCustomers(r.Context())
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteItems(w, serviceapi.MapPayloads(items, customerPayload))
}

func (h CustomerHandler) OpsUpdateCustomer(w http.ResponseWriter, r *http.Request) {
	var req updateCustomerRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.toInput()
	serviceapi.SetIfEmpty(&input.CustomerID, requestCustomerID(r))
	item, err := h.OpsCustomers.UpdateCustomer(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, customerPayload(item))
}
