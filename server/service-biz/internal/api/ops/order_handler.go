package ops

import (
	"net/http"

	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type OrderHandler struct {
	OpsCatalogOrders servicepkg.OpsCatalogOrderUseCase
}

func (h OrderHandler) Routes() []serviceapi.Route {
	return withOptAliases([]serviceapi.Route{
		serviceapi.NewRoute(http.MethodGet, "/api/ops/orders", h.OpsListOrders),
		serviceapi.NewRoute(http.MethodPost, "/api/ops/orders", h.OpsCreateOrder),
		serviceapi.NewRoute(http.MethodPatch, "/api/ops/orders/{orderId}", h.OpsUpdateOrder),
		serviceapi.NewRoute(http.MethodGet, "/api/ops/renewals", h.OpsListRenewals),
		serviceapi.NewRoute(http.MethodPatch, "/api/ops/renewals/{renewalId}", h.OpsUpdateRenewal),
	})
}

func (h OrderHandler) OpsListOrders(w http.ResponseWriter, r *http.Request) {
	items, err := h.OpsCatalogOrders.ListOrders(r.Context())
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteItems(w, serviceapi.MapPayloads(items, orderPayload))
}

func (h OrderHandler) OpsCreateOrder(w http.ResponseWriter, r *http.Request) {
	var req createOrderRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.toInput()
	item, err := h.OpsCatalogOrders.CreateOrder(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusCreated, orderPayload(item))
}

func (h OrderHandler) OpsUpdateOrder(w http.ResponseWriter, r *http.Request) {
	var req updateOrderRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.toInput()
	serviceapi.SetIfEmpty(&input.OrderID, requestOrderID(r))
	item, err := h.OpsCatalogOrders.UpdateOrder(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, orderPayload(item))
}

func (h OrderHandler) OpsListRenewals(w http.ResponseWriter, r *http.Request) {
	items, err := h.OpsCatalogOrders.ListRenewals(r.Context())
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteItems(w, serviceapi.MapPayloads(items, renewalPayload))
}

func (h OrderHandler) OpsUpdateRenewal(w http.ResponseWriter, r *http.Request) {
	var req updateRenewalRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.toInput()
	serviceapi.SetIfEmpty(&input.RenewalID, requestRenewalID(r))
	item, err := h.OpsCatalogOrders.UpdateRenewal(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, renewalPayload(item))
}
