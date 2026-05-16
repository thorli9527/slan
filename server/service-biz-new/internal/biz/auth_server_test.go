package biz

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestConsoleLoginHTTPFlowConsumesServerIssuedKeyOnce(t *testing.T) {
	server := NewServer()
	auth, _, err := server.store.RegisterUser("console-http@example.com", "secret", "Console HTTP")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	handler := server.Routes()

	var keyResponse struct {
		LoginKey string `json:"loginKey"`
		UserID   string `json:"userId"`
		DeviceID string `json:"deviceId"`
		Status   string `json:"status"`
	}
	postJSON(t, handler, "/api/auth/console-login-keys", "Bearer "+auth.Session.Token, map[string]any{
		"deviceId": "mac-http-1",
	}, http.StatusCreated, &keyResponse)
	if keyResponse.LoginKey == "" || keyResponse.UserID != auth.User.UserID || keyResponse.DeviceID != "mac-http-1" || keyResponse.Status != "unused" {
		t.Fatalf("unexpected console login key response: %+v", keyResponse)
	}

	var loginResponse struct {
		Auth AuthResponse `json:"auth"`
	}
	postJSON(t, handler, "/api/auth/console-login", "", map[string]any{
		"loginKey": keyResponse.LoginKey,
	}, http.StatusOK, &loginResponse)
	if loginResponse.Auth.User.UserID != auth.User.UserID || loginResponse.Auth.Session.Token == "" {
		t.Fatalf("unexpected console login response: %+v", loginResponse)
	}

	postJSON(t, handler, "/api/auth/console-login", "", map[string]any{
		"loginKey": keyResponse.LoginKey,
	}, http.StatusNotFound, nil)
}

func TestConsoleLoginHTTPRejectsInvalidCredentials(t *testing.T) {
	server := NewServer()
	handler := server.Routes()

	postJSON(t, handler, "/api/auth/console-login-keys", "Bearer bad-token", map[string]any{
		"deviceId": "mac-http-1",
	}, http.StatusNotFound, nil)
	postJSON(t, handler, "/api/auth/console-login", "", map[string]any{
		"loginKey": "not-a-real-key",
	}, http.StatusNotFound, nil)
}

func postJSON(t *testing.T, handler http.Handler, path, authorization string, body any, want int, out any) {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != want {
		t.Fatalf("POST %s status=%d want=%d body=%s", path, rec.Code, want, rec.Body.String())
	}
	if out != nil {
		if err := json.NewDecoder(rec.Body).Decode(out); err != nil {
			t.Fatalf("decode response: %v body=%s", err, rec.Body.String())
		}
	}
}
