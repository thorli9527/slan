package app

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

const validInstallerLogBody = `{
  "installationId":"82f07687-9773-4948-a22f-0d19b43f77d9",
  "platform":"windows",
  "version":"0.1.0",
  "capturedAt":123,
  "stage":"runtime.verify",
  "error":"adapter not ready",
  "files":{"installer.log":"setup detail"}
}`

func TestInstallerLogUploadStoresAnonymousBundle(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SLAN_CLIENT_LOG_DIR", dir)
	handler := InstallerLogHandler{Limiter: newDeviceRequestLimiter(installerLogIdentityLimit, installerLogIPLimit)}
	request := httptest.NewRequest(http.MethodPost, "/api/app/diagnostics/installer", bytes.NewBufferString(validInstallerLogBody))
	request.RemoteAddr = "203.0.113.10:43210"
	recorder := httptest.NewRecorder()

	handler.Upload(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected HTTP 201, got %d body=%s", recorder.Code, recorder.Body.String())
	}
	entries, err := os.ReadDir(filepath.Join(dir, "installer-82f07687-9773-4948-a22f-0d19b43f77d9"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected one stored bundle, got %d", len(entries))
	}
}

func TestInstallerLogUploadRejectsInvalidPlatform(t *testing.T) {
	handler := InstallerLogHandler{}
	body := bytes.ReplaceAll([]byte(validInstallerLogBody), []byte(`"windows"`), []byte(`"linux"`))
	request := httptest.NewRequest(http.MethodPost, "/api/app/diagnostics/installer", bytes.NewReader(body))
	recorder := httptest.NewRecorder()

	handler.Upload(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected HTTP 400, got %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestInstallerLogUploadRateLimitsInstallationIdentity(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SLAN_CLIENT_LOG_DIR", dir)
	handler := InstallerLogHandler{Limiter: newDeviceRequestLimiter(1, installerLogIPLimit)}
	for attempt := 0; attempt < 2; attempt++ {
		request := httptest.NewRequest(http.MethodPost, "/api/app/diagnostics/installer", bytes.NewBufferString(validInstallerLogBody))
		request.RemoteAddr = "203.0.113.10:43210"
		recorder := httptest.NewRecorder()
		handler.Upload(recorder, request)
		want := http.StatusCreated
		if attempt == 1 {
			want = http.StatusTooManyRequests
		}
		if recorder.Code != want {
			t.Fatalf("attempt %d: expected HTTP %d, got %d body=%s", attempt, want, recorder.Code, recorder.Body.String())
		}
	}
}
