package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	servicepkg "github.com/slan/service-biz/internal/service"
)

func TestWriteErrorSeparatesAuthenticationAndAuthorization(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{name: "invalid authentication", err: servicepkg.ErrUnauthorized, status: http.StatusUnauthorized},
		{name: "authenticated but forbidden", err: servicepkg.ErrForbidden, status: http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			WriteError(recorder, tt.err)
			if recorder.Code != tt.status {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.status)
			}
		})
	}
}
