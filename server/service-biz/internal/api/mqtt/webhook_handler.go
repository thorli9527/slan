package mqtt

import (
	"fmt"
	"log"
	"net/http"

	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type WebhookHandler struct {
	MQTT servicepkg.MQTTUseCase
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
		WebhookHandler{MQTT: useCase}.Routes(),
	)
}

func (h WebhookHandler) Auth(w http.ResponseWriter, r *http.Request) {
	req, err := decodeAuthRequest(r)
	if err != nil {
		serviceapi.WriteError(w, servicepkg.ErrInvalidArgument)
		return
	}
	input := req.input()
	view, err := h.MQTT.Authenticate(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	log.Printf(
		"mqtt webhook auth clientId=%s username=%s allowed=%t principal=%s deviceId=%s userId=%s mqtt5=%t",
		input.ClientID,
		input.Username,
		view.Allowed,
		view.Principal,
		view.DeviceID,
		view.UserID,
		input.IsV5,
	)
	if !view.Allowed {
		serviceapi.WriteJSON(w, http.StatusForbidden, authRejectedPayload(input.IsV5))
		return
	}
	serviceapi.WriteJSON(w, http.StatusOK, authAcceptedPayload(input.IsV5, authAllowedPayload(view)))
}

func (h WebhookHandler) Check(w http.ResponseWriter, r *http.Request) {
	req, err := decodeCheckRequest(r)
	if err != nil {
		serviceapi.WriteError(w, servicepkg.ErrInvalidArgument)
		return
	}
	input := req.input(r)
	if input.ClientID == "" && input.Username == "" && !input.Connect {
		log.Printf(
			"mqtt webhook check unresolved fields raw=%#v headers=%#v",
			req.Values,
			r.Header,
		)
	}
	allowed, err := h.MQTT.CheckACL(r.Context(), input)
	if err != nil {
		serviceapi.WriteError(w, err)
		return
	}
	log.Printf(
		"mqtt webhook check principal=%s deviceId=%s userId=%s clientId=%s username=%s topic=%s subscribe=%t connect=%t allowed=%t",
		input.Principal,
		input.DeviceID,
		input.UserID,
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
