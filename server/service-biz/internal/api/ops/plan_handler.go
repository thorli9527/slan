package ops

import (
	"net/http"

	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type PlanHandler struct {
	OpsCatalogPlans servicepkg.OpsCatalogPlanUseCase
}

func (h PlanHandler) Routes() []serviceapi.Route {
	return withOptAliases([]serviceapi.Route{
		serviceapi.NewRoute(http.MethodGet, "/api/ops/plans", h.OpsListPlans),
		serviceapi.NewRoute(http.MethodPost, "/api/ops/plans", h.OpsUpsertPlan),
		serviceapi.NewRoute(http.MethodPatch, "/api/ops/plans/{planCode}", h.OpsUpsertPlan),
	})
}

func (h PlanHandler) OpsListPlans(w http.ResponseWriter, r *http.Request) {
	items, err := h.OpsCatalogPlans.ListPlans(r.Context())
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteItems(w, serviceapi.MapPayloads(items, planPayload))
}

func (h PlanHandler) OpsUpsertPlan(w http.ResponseWriter, r *http.Request) {
	var req upsertPlanRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.toInput()
	serviceapi.SetIfEmpty(&input.PlanCode, requestPlanCode(r))
	item, err := h.OpsCatalogPlans.UpsertPlan(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, planPayload(item))
}
