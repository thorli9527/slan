package web

import (
	"net/http"

	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type UserAliasHandler struct {
	AuthAlias servicepkg.AuthAliasUseCase
}

func (h UserAliasHandler) Routes() []serviceapi.Route {
	return []serviceapi.Route{
		serviceapi.NewRoute(http.MethodGet, "/api/users/{userId}/user-aliases", h.ListUserAliases),
		serviceapi.NewRoute(http.MethodGet, "/api/user-aliases", h.ListUserAliases),
		serviceapi.NewRoute(http.MethodPatch, "/api/user-aliases", h.UpsertUserAlias),
	}
}

func (h UserAliasHandler) ListUserAliases(w http.ResponseWriter, r *http.Request) {
	items, err := h.AuthAlias.ListUserAliases(r.Context(), requestUserID(r))
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteItems(w, serviceapi.MapPayloads(items, userAliasPayload))
}

func (h UserAliasHandler) UpsertUserAlias(w http.ResponseWriter, r *http.Request) {
	var req upsertUserAliasRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	input := req.toInput()
	setUserAndActor(r, &input.UserID, &input.ActorUserID)
	item, err := h.AuthAlias.UpsertUserAlias(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, userAliasPayload(item))
}
