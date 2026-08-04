package mqtt

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	servicepkg "github.com/slan/service-biz/internal/service"
)

type mqttHandlerTestUseCase struct{}

func (mqttHandlerTestUseCase) Authenticate(context.Context, servicepkg.MQTTAuthInput) (servicepkg.MQTTAuthView, error) {
	return servicepkg.MQTTAuthView{}, nil
}

func (mqttHandlerTestUseCase) CheckACL(context.Context, servicepkg.MQTTCheckInput) (bool, error) {
	return false, nil
}

func (mqttHandlerTestUseCase) ReportEndpoint(context.Context, servicepkg.MQTTEndpointReportInput) (bool, error) {
	return false, nil
}

func (mqttHandlerTestUseCase) ReportPathHealth(context.Context, servicepkg.MQTTPathHealthReportInput) error {
	return nil
}

func TestCheckDoesNotLogRawPayloadOrHeaders(t *testing.T) {
	var logs bytes.Buffer
	previousOutput := log.Writer()
	previousFlags := log.Flags()
	previousPrefix := log.Prefix()
	log.SetOutput(&logs)
	log.SetFlags(0)
	log.SetPrefix("")
	t.Cleanup(func() {
		log.SetOutput(previousOutput)
		log.SetFlags(previousFlags)
		log.SetPrefix(previousPrefix)
	})

	const bodySecret = "mqtt-password-must-not-be-logged"
	const headerSecret = "bearer-token-must-not-be-logged"
	request := httptest.NewRequest(http.MethodPost, "/mqtt/bifromq/check", strings.NewReader(`{"password":"`+bodySecret+`","unknown":"value"}`))
	request.Header.Set("Authorization", "Bearer "+headerSecret)
	response := httptest.NewRecorder()
	(WebhookHandler{MQTT: mqttHandlerTestUseCase{}}).Check(response, request)

	if strings.Contains(logs.String(), bodySecret) || strings.Contains(logs.String(), headerSecret) {
		t.Fatalf("sensitive MQTT request data leaked to logs: %s", logs.String())
	}
	if !strings.Contains(logs.String(), "unresolved fields") {
		t.Fatalf("expected sanitized diagnostic log, got %q", logs.String())
	}
}

func TestAuthRateLimitsRepeatedIdentityFailures(t *testing.T) {
	handler := WebhookHandler{MQTT: mqttHandlerTestUseCase{}, AuthLimiter: newMQTTAuthLimiter()}
	for attempt := 0; attempt < mqttAuthIdentityFailureLimit; attempt++ {
		request := httptest.NewRequest(http.MethodPost, "/mqtt/bifromq/auth", strings.NewReader(
			`{"clientId":"client-1","username":"device-user","password":"wrong"}`,
		))
		request.RemoteAddr = "10.0.0.1:1883"
		response := httptest.NewRecorder()
		handler.Auth(response, request)
		if response.Code != http.StatusForbidden {
			t.Fatalf("attempt %d status = %d, want %d", attempt+1, response.Code, http.StatusForbidden)
		}
	}
	request := httptest.NewRequest(http.MethodPost, "/mqtt/bifromq/auth", strings.NewReader(
		`{"clientId":"client-1","username":"device-user","password":"wrong"}`,
	))
	request.RemoteAddr = "10.0.0.2:1883"
	response := httptest.NewRecorder()
	handler.Auth(response, request)
	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("rate limited status = %d, want %d", response.Code, http.StatusTooManyRequests)
	}
}

func TestWebhookTokenProtectsAllMQTTRoutes(t *testing.T) {
	handler := WebhookHandler{MQTT: mqttHandlerTestUseCase{}, Token: "mqtt-webhook-secret"}
	for _, route := range handler.Routes() {
		request := httptest.NewRequest(route.Method, route.Path, strings.NewReader(`{}`))
		response := httptest.NewRecorder()
		route.Handler(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("%s status = %d, want %d", route.Key(), response.Code, http.StatusUnauthorized)
		}
	}
	request := httptest.NewRequest(http.MethodPost, "/mqtt/bifromq/auth", strings.NewReader(`{}`))
	request.Header.Set("X-Slan-MQTT-Webhook-Token", "mqtt-webhook-secret")
	response := httptest.NewRecorder()
	handler.Auth(response, request)
	if response.Code == http.StatusUnauthorized {
		t.Fatal("valid webhook token was rejected")
	}
}
