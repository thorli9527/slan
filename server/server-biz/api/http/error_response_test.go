package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/service"
)

func TestWriteErrorMapsServiceErrors(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{name: "invalid argument", err: service.ErrInvalidArgument, status: http.StatusBadRequest, code: errorCodeInvalidArgument},
		{name: "unauthorized", err: service.ErrUnauthorized, status: http.StatusUnauthorized, code: errorCodeUnauthorized},
		{name: "forbidden", err: service.ErrForbidden, status: http.StatusForbidden, code: errorCodeForbidden},
		{name: "not found", err: service.ErrNotFound, status: http.StatusNotFound, code: errorCodeNotFound},
		{name: "conflict", err: service.ErrConflict, status: http.StatusConflict, code: errorCodeConflict},
		{name: "internal", err: errors.New("boom"), status: http.StatusInternalServerError, code: errorCodeInternal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			router := gin.New()
			router.GET("/error", func(c *gin.Context) {
				writeError(c, tt.err)
			})

			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/error", nil))

			if rec.Code != tt.status {
				t.Fatalf("expected status %d, got %d body=%s", tt.status, rec.Code, rec.Body.String())
			}
			resp := decodeErrorResponse(t, rec)
			if resp.Code != tt.code {
				t.Fatalf("expected code %s, got %+v", tt.code, resp)
			}
		})
	}
}

func TestPublicErrorHandlersReturnJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	installPublicErrorHandlers(router)
	router.GET("/ok", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/missing", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}
	if resp := decodeErrorResponse(t, rec); resp.Code != errorCodeNotFound {
		t.Fatalf("expected NOT_FOUND, got %+v", resp)
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/ok", strings.NewReader(`{}`)))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d body=%s", rec.Code, rec.Body.String())
	}
	if resp := decodeErrorResponse(t, rec); resp.Code != errorCodeInvalidArgument {
		t.Fatalf("expected INVALID_ARGUMENT, got %+v", resp)
	}
}

func TestControlMessageAckDisabledUsesErrorResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/control/messages/:messageId/ack", func(c *gin.Context) {
		writeErrorResponse(c, http.StatusNotImplemented, errorCodeNotImplemented, "message delivery disabled")
	})

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/control/messages/msg-1/ack", strings.NewReader(`{}`)))

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("expected 501, got %d body=%s", rec.Code, rec.Body.String())
	}
	if resp := decodeErrorResponse(t, rec); resp.Code != errorCodeNotImplemented {
		t.Fatalf("expected NOT_IMPLEMENTED, got %+v", resp)
	}
}

func decodeErrorResponse(t *testing.T, rec *httptest.ResponseRecorder) dto.ErrorResponse {
	t.Helper()
	var resp dto.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode error response: %v body=%s", err, rec.Body.String())
	}
	return resp
}
