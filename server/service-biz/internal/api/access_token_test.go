package api

import (
	"net/http/httptest"
	"testing"
)

func TestAccessTokenFromRequestOnlyAcceptsBearerHeader(t *testing.T) {
	request := httptest.NewRequest("GET", "/api/app/runtime/endpoints?accessToken=query-secret", nil)
	request.Header.Set("X-Access-Token", "header-secret")
	if token := AccessTokenFromRequest(request); token != "" {
		t.Fatalf("non-bearer token accepted: %q", token)
	}

	request.Header.Set("Authorization", "Bearer bearer-secret")
	if token := AccessTokenFromRequest(request); token != "bearer-secret" {
		t.Fatalf("bearer token = %q, want bearer-secret", token)
	}
}
