package mqtt

import (
	"crypto/subtle"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type WebhookHandler struct {
	MQTT        servicepkg.MQTTUseCase
	AuthLimiter *mqttAuthLimiter
	Token       string
}

func (h WebhookHandler) Routes() []serviceapi.Route {
	return []serviceapi.Route{
		serviceapi.NewRoute(http.MethodPost, "/mqtt/bifromq/auth", h.Auth),
		serviceapi.NewRoute(http.MethodPost, "/mqtt/bifromq/check", h.Check),
		serviceapi.NewRoute(http.MethodPost, "/mqtt/bifromq/endpoint-report", h.EndpointReport),
		serviceapi.NewRoute(http.MethodPost, "/mqtt/bifromq/path-health-report", h.PathHealthReport),
		serviceapi.NewRoute(http.MethodPost, "/mqtt/device/endpoint-report", h.EndpointReport),
		serviceapi.NewRoute(http.MethodPost, "/mqtt/device/path-health-report", h.PathHealthReport),
	}
}

func Routes(useCase servicepkg.MQTTUseCase) []serviceapi.Route {
	return serviceapi.CombineRoutes(
		WebhookHandler{
			MQTT: useCase, AuthLimiter: newMQTTAuthLimiter(), Token: strings.TrimSpace(os.Getenv("SLAN_MQTT_WEBHOOK_TOKEN")),
		}.Routes(),
	)
}

func (h WebhookHandler) Auth(w http.ResponseWriter, r *http.Request) {
	if !h.authorizeWebhook(w, r) {
		return
	}
	req, err := decodeAuthRequest(r)
	if err != nil {
		serviceapi.WriteError(w, servicepkg.ErrInvalidArgument)
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
		"mqtt webhook auth clientId=%s username=%s allowed=%t principal=%s deviceId=%s identityId=%s mqtt5=%t",
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
		serviceapi.WriteJSON(w, http.StatusForbidden, authRejectedPayload(input.IsV5))
		return
	}
	if h.AuthLimiter != nil {
		h.AuthLimiter.RecordSuccess(identity)
	}
	serviceapi.WriteJSON(w, http.StatusOK, authAcceptedPayload(input.IsV5, authAllowedPayload(view)))
}

func (h WebhookHandler) Check(w http.ResponseWriter, r *http.Request) {
	if !h.authorizeWebhook(w, r) {
		return
	}
	req, err := decodeCheckRequest(r)
	if err != nil {
		serviceapi.WriteError(w, servicepkg.ErrInvalidArgument)
		return
	}
	input := req.input(r)
	if input.ClientID == "" && input.Username == "" && !input.Connect {
		log.Printf(
			"mqtt webhook check unresolved fields remoteAddr=%s contentLength=%d valueCount=%d",
			r.RemoteAddr,
			r.ContentLength,
			len(req.Values),
		)
	}
	allowed, err := h.MQTT.CheckACL(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	log.Printf(
		"mqtt webhook check principal=%s deviceId=%s identityId=%s clientId=%s username=%s topic=%s subscribe=%t connect=%t allowed=%t",
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
	w.Header().Set("Content-Type", "text/plain")
	_, _ = fmt.Fprint(w, allowed)
}

func (h WebhookHandler) EndpointReport(w http.ResponseWriter, r *http.Request) {
	if !h.authorizeWebhook(w, r) {
		return
	}
	req, err := decodeEndpointReportRequest(r)
	if err != nil {
		serviceapi.WriteError(w, servicepkg.ErrInvalidArgument)
		return
	}
	input := req.input()
	log.Printf(
		"mqtt endpoint report request networkId=%s deviceId=%s nodeId=%s natType=%s endpoints=%#v",
		input.NetworkID,
		input.DeviceID,
		input.NodeID,
		input.NATType,
		input.Endpoints,
	)
	changed, err := h.MQTT.ReportEndpoint(r.Context(), input)
	if err != nil {
		log.Printf(
			"mqtt endpoint report failed networkId=%s deviceId=%s nodeId=%s err=%v",
			input.NetworkID,
			input.DeviceID,
			input.NodeID,
			err,
		)
		serviceapi.WriteError(w, err)
		return
	}
	log.Printf(
		"mqtt endpoint report applied networkId=%s deviceId=%s nodeId=%s changed=%t",
		input.NetworkID,
		input.DeviceID,
		input.NodeID,
		changed,
	)
	serviceapi.WriteJSON(w, http.StatusOK, reportEndpointPayload(changed))
}

func (h WebhookHandler) PathHealthReport(w http.ResponseWriter, r *http.Request) {
	if !h.authorizeWebhook(w, r) {
		return
	}
	req, err := decodePathHealthReportRequest(r)
	if err != nil {
		serviceapi.WriteError(w, servicepkg.ErrInvalidArgument)
		return
	}
	if err := h.MQTT.ReportPathHealth(r.Context(), req.input()); err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	serviceapi.WriteJSON(w, http.StatusAccepted, acceptedPayload())
}

func (h WebhookHandler) authorizeWebhook(w http.ResponseWriter, r *http.Request) bool {
	expected := strings.TrimSpace(h.Token)
	if expected == "" {
		return true
	}
	provided := strings.TrimSpace(r.Header.Get("X-Slan-MQTT-Webhook-Token"))
	if len(provided) != len(expected) || subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) != 1 {
		serviceapi.WriteError(w, servicepkg.ErrUnauthorized)
		return false
	}
	return true
}
