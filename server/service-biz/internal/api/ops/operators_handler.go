package ops

import (
	"net/http"

	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type OperatorHandler struct {
	OpsOperators         servicepkg.OpsOperatorUseCase
	OpsOperatorPasswords servicepkg.OpsOperatorPasswordUseCase
}

func (h OperatorHandler) Routes() []serviceapi.Route {
	return withOptAliases([]serviceapi.Route{
		serviceapi.NewRoute(http.MethodGet, "/api/ops/operators", h.OpsListOperators),
		serviceapi.NewRoute(http.MethodPost, "/api/ops/operators", h.OpsCreateOperator),
		serviceapi.NewRoute(http.MethodPatch, "/api/ops/operators/{operatorId}", h.OpsUpdateOperator),
		serviceapi.NewRoute(http.MethodPost, "/api/ops/operators/{operatorId}/password", h.OpsSetOperatorPassword),
	})
}

func (h OperatorHandler) OpsListOperators(w http.ResponseWriter, r *http.Request) {
	items, err := h.OpsOperators.ListOperators(r.Context())
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteItems(w, serviceapi.MapPayloads(items, operatorPayload))
}

func (h OperatorHandler) OpsCreateOperator(w http.ResponseWriter, r *http.Request) {
	var req createOperatorRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.toInput()
	item, err := h.OpsOperators.CreateOperator(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusCreated, operatorPayload(item))
}

func (h OperatorHandler) OpsUpdateOperator(w http.ResponseWriter, r *http.Request) {
	var req updateOperatorRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.toInput()
	serviceapi.SetIfEmpty(&input.OperatorID, requestOperatorID(r))
	item, err := h.OpsOperators.UpdateOperator(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, operatorPayload(item))
}

func (h OperatorHandler) OpsSetOperatorPassword(w http.ResponseWriter, r *http.Request) {
	var req setOperatorPasswordRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.toInput()
	serviceapi.SetIfEmpty(&input.OperatorID, requestOperatorID(r))
	item, err := h.OpsOperatorPasswords.SetOperatorPassword(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, operatorPayload(item))
}
