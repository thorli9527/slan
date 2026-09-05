package mqtt

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	servicepkg "github.com/slan/service-biz/internal/service"
)

type emqxHandlerTestUseCase struct {
	authView  servicepkg.MQTTAuthView
	aclResult bool
}

func (u emqxHandlerTestUseCase) Authenticate(context.Context, servicepkg.MQTTAuthInput) (servicepkg.MQTTAuthView, error) {
	return u.authView, nil
}

func (u emqxHandlerTestUseCase) CheckACL(context.Context, servicepkg.MQTTCheckInput) (bool, error) {
	return u.aclResult, nil
}

func (emqxHandlerTestUseCase) ReportEndpoint(context.Context, servicepkg.MQTTEndpointReportInput) (bool, error) {
	return false, nil
}

func (emqxHandlerTestUseCase) ReportPathHealth(context.Context, servicepkg.MQTTPathHealthReportInput) error {
	return nil
}

func decodeEmqxResult(t *testing.T, response *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode emqx payload: %v body=%q", err, response.Body.String())
	}
	return payload
}

func TestEmqxAuthAllowsDevicePrincipal(t *testing.T) {
	t.Parallel()
	handler := WebhookHandler{MQTT: emqxHandlerTestUseCase{authView: servicepkg.MQTTAuthView{
		Allowed:    true,
		Principal:  "device",
		DeviceID:   "dev-1",
		IdentityID: "dev-1",
	}}}
	request := httptest.NewRequest(http.MethodPost, "/mqtt/emqx/auth", strings.NewReader(
		`{"clientid":"node-dev-1","username":"device:dev-1:1","password":"secret","peerhost":"10.0.0.5"}`,
	))
	response := httptest.NewRecorder()
	handler.EmqxAuth(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body=%q", response.Code, response.Body.String())
	}
	payload := decodeEmqxResult(t, response)
	if payload["result"] != "allow" {
		t.Fatalf("result = %#v", payload["result"])
	}
	if _, exists := payload["is_superuser"]; exists {
		t.Fatalf("device principal must not be superuser: %#v", payload)
	}
}

func TestEmqxAuthAllowsServerPrincipalAsSuperuser(t *testing.T) {
	t.Parallel()
	handler := WebhookHandler{MQTT: emqxHandlerTestUseCase{authView: servicepkg.MQTTAuthView{
		Allowed:    true,
		Principal:  "server",
		IdentityID: "server",
	}}}
	request := httptest.NewRequest(http.MethodPost, "/mqtt/emqx/auth", strings.NewReader(
		`{"clientid":"slan-server","username":"server:slan:1","password":"secret"}`,
	))
	response := httptest.NewRecorder()
	handler.EmqxAuth(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	payload := decodeEmqxResult(t, response)
	if payload["result"] != "allow" || payload["is_superuser"] != true {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestEmqxAuthDeniesRejectedCredential(t *testing.T) {
	t.Parallel()
	handler := WebhookHandler{MQTT: emqxHandlerTestUseCase{}, AuthLimiter: newMQTTAuthLimiter()}
	request := httptest.NewRequest(http.MethodPost, "/mqtt/emqx/auth", strings.NewReader(
		`{"clientid":"node-dev-1","username":"device:dev-1:1","password":"wrong"}`,
	))
	response := httptest.NewRecorder()
	handler.EmqxAuth(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d", response.Code)
	}
	payload := decodeEmqxResult(t, response)
	if payload["result"] != "deny" {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestEmqxAuthRejectsInvalidBody(t *testing.T) {
	t.Parallel()
	handler := WebhookHandler{MQTT: emqxHandlerTestUseCase{}}
	request := httptest.NewRequest(http.MethodPost, "/mqtt/emqx/auth", strings.NewReader(`{not-json`))
	response := httptest.NewRecorder()
	handler.EmqxAuth(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", response.Code)
	}
	payload := decodeEmqxResult(t, response)
	if payload["result"] != "deny" {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestEmqxCheckAllowsSubscribedTopic(t *testing.T) {
	t.Parallel()
	handler := WebhookHandler{MQTT: emqxHandlerTestUseCase{aclResult: true}}
	request := httptest.NewRequest(http.MethodPost, "/mqtt/emqx/check", strings.NewReader(
		`{"clientid":"node-dev-1","username":"device:dev-1:1","action":"subscribe","topic":"slan/devices/dev-1/control/down"}`,
	))
	response := httptest.NewRecorder()
	handler.EmqxCheck(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	payload := decodeEmqxResult(t, response)
	if payload["result"] != "allow" {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestEmqxCheckDeniesPublishViolation(t *testing.T) {
	t.Parallel()
	handler := WebhookHandler{MQTT: emqxHandlerTestUseCase{aclResult: false}}
	request := httptest.NewRequest(http.MethodPost, "/mqtt/emqx/check", strings.NewReader(
		`{"clientid":"node-dev-1","username":"device:dev-1:1","action":"publish","topic":"slan/devices/dev-2/control/up"}`,
	))
	response := httptest.NewRecorder()
	handler.EmqxCheck(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d", response.Code)
	}
	payload := decodeEmqxResult(t, response)
	if payload["result"] != "deny" {
		t.Fatalf("payload = %#v", payload)
	}
}
