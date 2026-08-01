package ops

import (
	"net/http"

	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type AuthHandler struct {
	OpsAuthSessions      servicepkg.OpsAuthSessionUseCase
	OpsOperatorPasswords servicepkg.OpsOperatorPasswordUseCase
}

func (h AuthHandler) Routes() []serviceapi.Route {
	return withOptAliases([]serviceapi.Route{
		serviceapi.NewRoute(http.MethodPost, "/api/ops/auth/login", h.OpsLogin),
		serviceapi.NewRoute(http.MethodPatch, "/api/ops/auth/password", h.OpsChangePassword),
	})
}

func (h AuthHandler) OpsLogin(w http.ResponseWriter, r *http.Request) {
	var req opsLoginRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.toInput()
	view, err := h.OpsAuthSessions.Login(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteEnvelope(w, http.StatusOK, "auth", authPayload(view))
}

func (h AuthHandler) OpsChangePassword(w http.ResponseWriter, r *http.Request) {
	var req opsChangePasswordRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.toInput()
	input.OperatorID = serviceapi.AuthenticatedOperatorID(r.Context())
	item, err := h.OpsOperatorPasswords.ChangePassword(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, operatorPayload(item))
}
