package ops

import (
	"net/http"

	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type UserHandler struct {
	OpsUsers servicepkg.OpsUserUseCase
}

func (h UserHandler) Routes() []serviceapi.Route {
	return withOptAliases([]serviceapi.Route{
		serviceapi.NewRoute(http.MethodGet, "/api/ops/users", h.OpsListUsers),
		serviceapi.NewRoute(http.MethodPost, "/api/ops/users", h.OpsCreateUser),
		serviceapi.NewRoute(http.MethodPatch, "/api/ops/users/{userId}", h.OpsUpdateUser),
		serviceapi.NewRoute(http.MethodPatch, "/api/ops/users/{userId}/password", h.OpsSetUserPassword),
	})
}

func (h UserHandler) OpsCreateUser(w http.ResponseWriter, r *http.Request) {
	var req createUserRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	item, err := h.OpsUsers.CreateUser(r.Context(), req.toInput())
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusCreated, userPayload(item))
}

func (h UserHandler) OpsListUsers(w http.ResponseWriter, r *http.Request) {
	items, err := h.OpsUsers.ListUsers(r.Context())
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteItems(w, serviceapi.MapPayloads(items, userPayload))
}

func (h UserHandler) OpsSetUserPassword(w http.ResponseWriter, r *http.Request) {
	var req setUserPasswordRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	item, err := h.OpsUsers.SetUserPassword(r.Context(), servicepkg.SetUserPasswordInput{UserID: requestUserID(r), Password: req.Password})
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, userPayload(item))
}

func (h UserHandler) OpsUpdateUser(w http.ResponseWriter, r *http.Request) {
	var req updateUserRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.toInput()
	serviceapi.SetIfEmpty(&input.UserID, requestUserID(r))
	item, err := h.OpsUsers.UpdateUser(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, userPayload(item))
}
