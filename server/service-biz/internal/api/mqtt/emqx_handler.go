package mqtt

import (
	"log"
	"net/http"
	"time"

	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

// EMQX HTTP authentication and authorization adapter. The request body format
// matches the BifroMQ webhook shape (clientId/username/password, action/topic),
// so decoding is shared; only the response payloads follow EMQX conventions:
// allow -> HTTP 200 {"result":"allow"}, deny -> HTTP 403 {"result":"deny"}.

func (h WebhookHandler) EmqxAuth(w http.ResponseWriter, r *http.Request) {
	if !h.authorizeWebhook(w, r) {
		return
	}
	req, err := decodeAuthRequest(r)
	if err != nil {
		serviceapi.WriteJSON(w, http.StatusBadRequest, emqxResultPayload("deny"))
		return
	}
	input := req.input()
	source := mqttAuthSource(r)
	identity := mqttAuthIdentity(input.ClientID, input.Username)
	if h.AuthLimiter != nil && !h.AuthLimiter.Allow(source, identity, time.Now()) {
		serviceapi.WriteError(w, servicepkg.ErrRateLimited)
		return
	}
	view, err := h.MQTT.Authenticate(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	log.Printf(
		"mqtt emqx auth clientId=%s username=%s allowed=%t principal=%s deviceId=%s identityId=%s mqtt5=%t",
		input.ClientID,
		input.Username,
		view.Allowed,
		view.Principal,
		view.DeviceID,
		view.IdentityID,
		input.IsV5,
	)
	if !view.Allowed {
		if h.AuthLimiter != nil {
			h.AuthLimiter.RecordFailure(source, identity, time.Now())
		}
		serviceapi.WriteJSON(w, http.StatusForbidden, emqxResultPayload("deny"))
		return
	}
	if h.AuthLimiter != nil {
		h.AuthLimiter.RecordSuccess(identity)
	}
	payload := emqxResultPayload("allow")
	if view.Principal == "server" {
		payload["is_superuser"] = true
	}
	serviceapi.WriteJSON(w, http.StatusOK, payload)
}

func (h WebhookHandler) EmqxCheck(w http.ResponseWriter, r *http.Request) {
	if !h.authorizeWebhook(w, r) {
		return
	}
	req, err := decodeCheckRequest(r)
	if err != nil {
		serviceapi.WriteJSON(w, http.StatusBadRequest, emqxResultPayload("deny"))
		return
	}
	input := req.input(r)
	allowed, err := h.MQTT.CheckACL(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	log.Printf(
		"mqtt emqx check principal=%s deviceId=%s identityId=%s clientId=%s username=%s topic=%s subscribe=%t connect=%t allowed=%t",
		input.Principal,
		input.DeviceID,
		input.IdentityID,
		input.ClientID,
		input.Username,
		input.Topic,
		input.Subscribe,
		input.Connect,
		allowed,
	)
	if !allowed {
		serviceapi.WriteJSON(w, http.StatusForbidden, emqxResultPayload("deny"))
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, emqxResultPayload("allow"))
}

func emqxResultPayload(result string) map[string]any {
	return map[string]any{"result": result}
}
