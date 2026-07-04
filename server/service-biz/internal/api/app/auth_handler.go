package app

import (
	"net/http"

	serviceapi "github.com/slan/service-biz/internal/api"
	authrequest "github.com/slan/service-biz/internal/api/authrequest"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type AuthHandler struct {
	AuthRegistration servicepkg.AuthUserRegistrationUseCase
	AuthSessions     servicepkg.AuthUserSessionUseCase
	NetworkCore      servicepkg.NetworkCoreUseCase
}

func (h AuthHandler) Routes() []serviceapi.Route {
	return []serviceapi.Route{
		serviceapi.NewRoute(http.MethodPost, "/api/app/auth/register", h.RegisterUser),
		serviceapi.NewRoute(http.MethodPost, "/api/app/auth/login", h.LoginUser),
		serviceapi.NewRoute(http.MethodPost, "/api/app/auth/renew", h.RenewUserSession),
		serviceapi.NewRoute(http.MethodPost, "/api/app/auth/logout", h.LogoutUser),
	}
}

func (h AuthHandler) RegisterUser(w http.ResponseWriter, r *http.Request) {
	var req authrequest.RegisterUser
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	view, err := h.AuthRegistration.RegisterUser(r.Context(), req.ToInput())
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusCreated, appAuthSessionPayload(r.Context(), h.NetworkCore, view))
}

func (h AuthHandler) LoginUser(w http.ResponseWriter, r *http.Request) {
	var req authrequest.LoginUser
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	view, err := h.AuthSessions.LoginUser(r.Context(), req.ToInput())
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, appAuthSessionPayload(r.Context(), h.NetworkCore, view))
}

func (h AuthHandler) RenewUserSession(w http.ResponseWriter, r *http.Request) {
	var req authrequest.RenewUserSession
	if !serviceapi.DecodeJSONIfPresentOrError(w, r, &req) {
		return
	}
	view, err := h.AuthSessions.RenewUserSession(r.Context(), serviceapi.AccessTokenFromRequest(r), req.ToInput())
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, appAuthSessionPayload(r.Context(), h.NetworkCore, view))
}

func (h AuthHandler) LogoutUser(w http.ResponseWriter, r *http.Request) {
	var req authrequest.LogoutUser
	if !serviceapi.DecodeJSONIfPresentOrError(w, r, &req) {
		return
	}
	if err := h.AuthSessions.LogoutUser(r.Context(), serviceapi.AccessTokenFromRequest(r), req.ToInput()); err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteOK(w)
}
