package app

import (
	"net/http"
	"strings"

	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type NetworkInviteHandler struct {
	Invites      servicepkg.NetworkInviteUseCase
	AuthSessions servicepkg.AuthUserSessionUseCase
}

func (h NetworkInviteHandler) Routes() []serviceapi.Route {
	return []serviceapi.Route{
		serviceapi.NewRoute(http.MethodPost, "/api/app/network-invites", h.Create),
		serviceapi.NewRoute(http.MethodPost, "/api/app/network-invites/accept", h.Accept),
	}
}

type createNetworkInviteRequest struct {
	TTLSeconds int64 `json:"ttlSeconds"`
}

func (h NetworkInviteHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createNetworkInviteRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	userID, ok := h.authenticatedUserID(w, r)
	if !ok {
		return
	}
	item, err := h.Invites.CreateDeviceInvite(r.Context(), servicepkg.CreateDeviceInviteInput{
		InviterUserID: userID,
		TTLSeconds:    req.TTLSeconds,
	})
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusCreated, item)
}

type acceptNetworkInviteRequest struct {
	InviteCode string `json:"inviteCode"`
	DeviceID   string `json:"deviceId"`
	Alias      string `json:"alias"`
}

func (h NetworkInviteHandler) Accept(w http.ResponseWriter, r *http.Request) {
	var req acceptNetworkInviteRequest
	if !serviceapi.DecodeJSONOrError(w, r, &req) {
		return
	}
	userID, ok := h.authenticatedUserID(w, r)
	if !ok {
		return
	}
	item, err := h.Invites.AcceptDeviceInvite(r.Context(), servicepkg.AcceptDeviceInviteInput{
		InviteCode:  strings.TrimSpace(req.InviteCode),
		DeviceID:    strings.TrimSpace(req.DeviceID),
		UserID:      userID,
		ActorUserID: userID,
		Alias:       strings.TrimSpace(req.Alias),
	})
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteEnvelope(w, http.StatusOK, "invite", item)
}

func (h NetworkInviteHandler) authenticatedUserID(w http.ResponseWriter, r *http.Request) (string, bool) {
	session, err := h.AuthSessions.GetUserSession(r.Context(), serviceapi.AccessTokenFromRequest(r))
	if err != nil {
		serviceapi.WriteError(w, err)
		return "", false
	}
	return session.User.UserID, true
}
