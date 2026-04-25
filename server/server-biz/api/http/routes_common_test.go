package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestLimitRequestBody_RejectsOversizedJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(limitRequestBody(maxHTTPJSONBodyBytes))
	router.POST("/echo", func(c *gin.Context) {
		var req map[string]string
		if !bindJSON(c, &req) {
			return
		}
		c.JSON(http.StatusOK, req)
	})

	payload := `{"payload":"` + strings.Repeat("a", maxHTTPJSONBodyBytes) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413 for oversized body, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "REQUEST_TOO_LARGE") {
		t.Fatalf("expected REQUEST_TOO_LARGE response, got %s", rec.Body.String())
	}
}

func TestLimitRequestBody_AllowsSmallJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(limitRequestBody(maxHTTPJSONBodyBytes))
	router.POST("/echo", func(c *gin.Context) {
		var req map[string]string
		if !bindJSON(c, &req) {
			return
		}
		c.JSON(http.StatusOK, req)
	})

	req := httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader(`{"payload":"ok"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for small body, got %d body=%s", rec.Code, rec.Body.String())
	}
}
