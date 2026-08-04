package ops

import (
	"errors"
	"net/http"
	"strings"
	"time"

	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type AuthHandler struct {
	OpsAuthSessions      servicepkg.OpsAuthSessionUseCase
	OpsOperatorPasswords servicepkg.OpsOperatorPasswordUseCase
	LoginLimiter         *opsLoginLimiter
}

func (h AuthHandler) Routes() []serviceapi.Route {
	return withOptAliases([]serviceapi.Route{
		serviceapi.NewRoute(http.MethodPost, "/api/ops/auth/login", h.OpsLogin),
		serviceapi.NewRoute(http.MethodPost, "/api/ops/auth/logout", h.OpsLogout),
		serviceapi.NewRoute(http.MethodPatch, "/api/ops/auth/password", h.OpsChangePassword),
	})
}

func (h AuthHandler) OpsLogout(w http.ResponseWriter, r *http.Request) {
	if err := h.OpsAuthSessions.Logout(r.Context(), bearerToken(r)); err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteNoContent(w)
}

func (h AuthHandler) OpsLogin(w http.ResponseWriter, r *http.Request) {
	var req opsLoginRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.toInput()
	remoteIP := serviceapi.RemoteIP(r)
	input.RemoteIP = remoteIP
	account := strings.ToLower(strings.TrimSpace(input.Email))
	if h.LoginLimiter != nil && !h.LoginLimiter.Allow(remoteIP, account, time.Now()) {
		serviceapi.WriteError(w, servicepkg.ErrRateLimited)
		return
	}
	view, err := h.OpsAuthSessions.Login(r.Context(), input)
	if err != nil {
		if h.LoginLimiter != nil && errors.Is(err, servicepkg.ErrUnauthorized) {
			h.LoginLimiter.RecordFailure(remoteIP, account, time.Now())
		}
		serviceapi.WriteError(w, err)
		return
	}
	if h.LoginLimiter != nil {
		h.LoginLimiter.RecordSuccess(remoteIP, account)
	}
	serviceapi.WriteEnvelope(w, http.StatusOK, "auth", authPayload(view))
}

func (h AuthHandler) OpsChangePassword(w http.ResponseWriter, r *http.Request) {
	var req opsChangePasswordRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.toInput()
	input.OperatorID = servicepkg.AuthenticatedOperatorID(r.Context())
	item, err := h.OpsOperatorPasswords.ChangePassword(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, operatorPayload(item))
}
