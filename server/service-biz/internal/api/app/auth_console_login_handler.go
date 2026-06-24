package app

import (
	"net/http"

	serviceapi "github.com/slan/service-biz/internal/api"
	authpayload "github.com/slan/service-biz/internal/api/authpayload"
	authrequest "github.com/slan/service-biz/internal/api/authrequest"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type AuthConsoleLoginHandler struct {
	ConsoleLoginUseCase servicepkg.AuthConsoleLoginUseCase
}

func (h AuthConsoleLoginHandler) Routes() []serviceapi.Route {
	return []serviceapi.Route{
		serviceapi.NewRoute(http.MethodPost, "/api/auth/console-login", h.ConsoleLogin),
	}
}

func (h AuthConsoleLoginHandler) ConsoleLogin(w http.ResponseWriter, r *http.Request) {
	var req authrequest.ConsoleLogin
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	view, err := h.ConsoleLoginUseCase.ConsoleLogin(r.Context(), req.ToInput())
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteEnvelope(w, http.StatusOK, "auth", authpayload.Session(view))
}
