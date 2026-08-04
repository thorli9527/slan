package ops

import (
	"net/http"
	"strconv"

	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type OverviewHandler struct {
	OpsDashboardReader servicepkg.OpsDashboardUseCase
	OpsAudit           servicepkg.OpsAuditUseCase
}

func (h OverviewHandler) Routes() []serviceapi.Route {
	return withOptAliases([]serviceapi.Route{
		serviceapi.NewRoute(http.MethodGet, "/api/ops/dashboard", h.OpsDashboard),
		serviceapi.NewRoute(http.MethodGet, "/api/ops/audit-events", h.OpsListAuditEvents),
	})
}

func (h OverviewHandler) OpsDashboard(w http.ResponseWriter, r *http.Request) {
	view, err := h.OpsDashboardReader.Dashboard(r.Context())
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, dashboardPayload(view))
}

func (h OverviewHandler) OpsListAuditEvents(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := h.OpsAudit.ListAuditEvents(r.Context(), limit)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteItems(w, serviceapi.MapPayloads(items, auditEventPayload))
}
