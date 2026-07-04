package app

import (
	"net/http"

	serviceapi "github.com/slan/service-biz/internal/api"
	authrequest "github.com/slan/service-biz/internal/api/authrequest"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type AuthConsoleHandler struct {
	ConsoleKeys servicepkg.AuthConsoleKeyUseCase
}

func (h AuthConsoleHandler) Routes() []serviceapi.Route {
	return []serviceapi.Route{
		serviceapi.NewRoute(http.MethodPost, "/api/app/auth/console-login-keys", h.CreateConsoleLoginKey),
	}
}

func (h AuthConsoleHandler) CreateConsoleLoginKey(w http.ResponseWriter, r *http.Request) {
	var req authrequest.CreateConsoleLoginKey
	if !serviceapi.DecodeJSONIfPresentOrError(w, r, &req) {
		return
	}
	input := req.ToInput()
	serviceapi.SetIfEmpty(&input.UserID, requestUserID(r))
	item, err := h.ConsoleKeys.CreateConsoleLoginKey(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusCreated, appConsoleLoginKeyPayload(item))
}
