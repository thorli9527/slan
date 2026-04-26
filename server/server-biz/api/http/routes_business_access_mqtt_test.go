package httpapi

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/configs"
	"github.com/slan/server/server-biz/internal/mqttauth"
)

type fakeMQTTDeviceService struct {
	marked []string
}

func (s *fakeMQTTDeviceService) Register(string, dto.RegisterDeviceRequest) (dto.Device, error) {
	return dto.Device{}, nil
}

func (s *fakeMQTTDeviceService) ListByUser(string) ([]dto.Device, error) {
	return nil, nil
}

func (s *fakeMQTTDeviceService) SetDeviceNetworkState(string, string, string, dto.DeviceNetworkStateRequest) (dto.DeviceNetworkState, error) {
	return dto.DeviceNetworkState{}, nil
}

func (s *fakeMQTTDeviceService) MarkMQTTReachable(deviceID string) error {
	s.marked = append(s.marked, deviceID)
	return nil
}

func TestBifroMQAuthAcceptsBase64Password(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := configs.DefaultConfig()
	cfg.MQTT.Enabled = true
	device := &fakeMQTTDeviceService{}
	router := mqttAccessTestRouter(cfg, device)
	credential := mqttauth.DeviceCredential(cfg.MQTT, "dev-1", "machine-1", time.Now())

	rec := postJSON(router, "/mqtt/bifromq/auth", map[string]any{
		"clientId": credential.ClientID,
		"username": credential.Username,
		"password": base64.StdEncoding.EncodeToString([]byte(credential.Password)),
	}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected auth to pass, got %d body=%s", rec.Code, rec.Body.String())
	}
	var response dto.BifroMQAuthResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.OK == nil || response.OK.UserID != "dev-1" || response.OK.Attrs["principal"] != "device" {
		t.Fatalf("unexpected auth response: %+v", response)
	}
	if len(device.marked) != 1 || device.marked[0] != "dev-1" {
		t.Fatalf("expected mqtt reachability mark for device, got %+v", device.marked)
	}
}

func TestBifroMQAuthRejectsInvalidCredential(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := configs.DefaultConfig()
	cfg.MQTT.Enabled = true
	router := mqttAccessTestRouter(cfg, &fakeMQTTDeviceService{})

	rec := postJSON(router, "/mqtt/bifromq/auth", map[string]any{
		"clientId": "slan-dev-1",
		"username": "slan/dev-1",
		"password": "bad",
	}, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected auth reject, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestBifroMQCheckAllowsConnectAndOwnPublishOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := configs.DefaultConfig()
	cfg.MQTT.Enabled = true
	router := mqttAccessTestRouter(cfg, &fakeMQTTDeviceService{})

	headers := map[string]string{"user_id": "dev-1"}
	if rec := postJSON(router, "/mqtt/bifromq/check", map[string]any{"conn": map[string]any{}}, headers); rec.Code != http.StatusOK || rec.Body.String() != "true" {
		t.Fatalf("expected connect check to pass, got %d body=%s", rec.Code, rec.Body.String())
	}
	if rec := postJSON(router, "/mqtt/bifromq/check", map[string]any{"pub": map[string]any{"topic": "slan/devices/dev-1/networks/net-1/state"}}, headers); rec.Code != http.StatusOK || rec.Body.String() != "true" {
		t.Fatalf("expected own publish check to pass, got %d body=%s", rec.Code, rec.Body.String())
	}
	if rec := postJSON(router, "/mqtt/bifromq/check", map[string]any{"pub": map[string]any{"topic": "slan/devices/dev-2/networks/net-1/state"}}, headers); rec.Code != http.StatusOK || rec.Body.String() != "false" {
		t.Fatalf("expected cross-device publish check to fail, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestBifroMQCheckAllowsServerStateSubscriptionOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := configs.DefaultConfig()
	cfg.MQTT.Enabled = true
	router := mqttAccessTestRouter(cfg, &fakeMQTTDeviceService{})

	headers := map[string]string{"user_id": "server-biz-subscriber"}
	if rec := postJSON(router, "/mqtt/bifromq/check", map[string]any{"sub": map[string]any{"topicFilter": "slan/devices/+/networks/+/state"}}, headers); rec.Code != http.StatusOK || rec.Body.String() != "true" {
		t.Fatalf("expected server state subscribe to pass, got %d body=%s", rec.Code, rec.Body.String())
	}
	if rec := postJSON(router, "/mqtt/bifromq/check", map[string]any{"sub": map[string]any{"topicFilter": "slan/devices/#"}}, headers); rec.Code != http.StatusOK || rec.Body.String() != "false" {
		t.Fatalf("expected broad server subscribe to fail, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func mqttAccessTestRouter(cfg configs.Config, device *fakeMQTTDeviceService) *gin.Engine {
	router := gin.New()
	router.Use(limitRequestBody(maxHTTPJSONBodyBytes))
	registerAccessRoutes(router.Group(""), routerDeps{Config: cfg, Auth: fakeAuthService{}, Device: device})
	return router
}

func postJSON(router http.Handler, path string, body map[string]any, headers map[string]string) *httptest.ResponseRecorder {
	payload, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}
