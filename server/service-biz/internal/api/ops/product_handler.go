package ops

import (
	"net/http"

	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type ProductHandler struct {
	OpsCatalogProducts servicepkg.OpsCatalogProductUseCase
}

func (h ProductHandler) Routes() []serviceapi.Route {
	return withOptAliases([]serviceapi.Route{
		serviceapi.NewRoute(http.MethodGet, "/api/ops/products", h.OpsListProducts),
		serviceapi.NewRoute(http.MethodPost, "/api/ops/products", h.OpsCreateProduct),
		serviceapi.NewRoute(http.MethodPatch, "/api/ops/products/{productId}", h.OpsUpdateProduct),
	})
}

func (h ProductHandler) OpsListProducts(w http.ResponseWriter, r *http.Request) {
	items, err := h.OpsCatalogProducts.ListProducts(r.Context())
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteItems(w, serviceapi.MapPayloads(items, productPayload))
}

func (h ProductHandler) OpsCreateProduct(w http.ResponseWriter, r *http.Request) {
	var req createProductRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.toInput()
	item, err := h.OpsCatalogProducts.CreateProduct(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusCreated, productPayload(item))
}

func (h ProductHandler) OpsUpdateProduct(w http.ResponseWriter, r *http.Request) {
	var req updateProductRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.toInput()
	serviceapi.SetIfEmpty(&input.ProductID, requestProductID(r))
	item, err := h.OpsCatalogProducts.UpdateProduct(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, productPayload(item))
}
