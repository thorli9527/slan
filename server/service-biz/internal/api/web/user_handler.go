package web

import (
	"net/http"

	serviceapi "github.com/slan/service-biz/internal/api"
	authrequest "github.com/slan/service-biz/internal/api/authrequest"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type UserHandler struct {
	UserAccounts     servicepkg.AuthUserAccountUseCase
	UserEntitlements servicepkg.AuthUserEntitlementUseCase
}

func (h UserHandler) Routes() []serviceapi.Route {
	return []serviceapi.Route{
		serviceapi.NewRoute(http.MethodGet, "/api/users", h.ListUsers),
		serviceapi.NewRoute(http.MethodGet, "/api/users/{userId}/entitlement", h.UserEntitlement),
		serviceapi.NewRoute(http.MethodPatch, "/api/users/{userId}/password", h.ChangeUserPassword),
	}
}

func (h UserHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	items, err := h.UserAccounts.ListUsers(r.Context())
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteItems(w, serviceapi.MapPayloads(items, userPayload))
}

func (h UserHandler) UserEntitlement(w http.ResponseWriter, r *http.Request) {
	view, err := h.UserEntitlements.UserEntitlement(r.Context(), requestUserID(r))
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, userEntitlementPayload(view))
}

func (h UserHandler) ChangeUserPassword(w http.ResponseWriter, r *http.Request) {
	var req authrequest.ChangeUserPassword
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.ToInput()
	setUserAndActor(r, &input.UserID, &input.ActorUserID)
	item, err := h.UserAccounts.ChangeUserPassword(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, changedUserPasswordPayload(item))
}
