package app

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	servicepkg "github.com/slan/service-biz/internal/service"
)

func TestDeviceLogUploadAuthenticatesAndStoresBundle(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SLAN_CLIENT_LOG_DIR", dir)
	handler := DeviceLogHandler{DeviceSessions: deviceSessionAuthTestUseCase{
		session: servicepkg.DeviceSessionView{DeviceID: "device-a"},
	}}
	request := httptest.NewRequest(http.MethodPost, "/api/app/devices/device-a/logs", bytes.NewBufferString(`{
		"deviceId":"device-a","platform":"windows","capturedAt":123,"files":{"service.log":"ready"}
	}`))
	request.SetPathValue("deviceId", "device-a")
	request.Header.Set("Authorization", "Bearer device-token")
	recorder := httptest.NewRecorder()

	handler.Upload(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected HTTP 201, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	entries, err := os.ReadDir(filepath.Join(dir, "device-a"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected one stored bundle, got %d", len(entries))
	}
}

func TestDeviceLogUploadRejectsDifferentDevice(t *testing.T) {
	handler := DeviceLogHandler{DeviceSessions: deviceSessionAuthTestUseCase{
		session: servicepkg.DeviceSessionView{DeviceID: "device-a"},
	}}
	request := httptest.NewRequest(http.MethodPost, "/api/app/devices/device-b/logs", bytes.NewBufferString(`{
		"deviceId":"device-b","platform":"windows","capturedAt":123,"files":{"service.log":"ready"}
	}`))
	request.SetPathValue("deviceId", "device-b")
	recorder := httptest.NewRecorder()

	handler.Upload(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected HTTP 401, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}
