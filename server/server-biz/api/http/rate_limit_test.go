package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestRateLimitByIPRejectsExcessRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/login", rateLimitByIP(2, time.Minute), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	for i := 0; i < 2; i++ {
		rec := performRateLimitedRequest(router, "/login")
		if rec.Code != http.StatusOK {
			t.Fatalf("expected request %d to pass, got %d body=%s", i+1, rec.Code, rec.Body.String())
		}
	}

	rec := performRateLimitedRequest(router, "/login")
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after limit, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "RATE_LIMITED") {
		t.Fatalf("expected RATE_LIMITED response, got %s", rec.Body.String())
	}
}

func TestRateLimitByIPSeparatesRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
	router.POST("/login", rateLimitByIP(1, time.Minute), handler)
	router.POST("/register", rateLimitByIP(1, time.Minute), handler)

	if rec := performRateLimitedRequest(router, "/login"); rec.Code != http.StatusOK {
		t.Fatalf("expected first login to pass, got %d", rec.Code)
	}
	if rec := performRateLimitedRequest(router, "/register"); rec.Code != http.StatusOK {
		t.Fatalf("expected first register to pass, got %d", rec.Code)
	}
	if rec := performRateLimitedRequest(router, "/login"); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected second login to be limited, got %d", rec.Code)
	}
}

func TestRequestRateLimiterAllowsNextWindow(t *testing.T) {
	limiter := newRequestRateLimiter(1, time.Minute)
	now := time.Unix(100, 0)

	if !limiter.allow("127.0.0.1:/login", now) {
		t.Fatal("expected first request to pass")
	}
	if limiter.allow("127.0.0.1:/login", now.Add(10*time.Second)) {
		t.Fatal("expected second request in window to be limited")
	}
	if !limiter.allow("127.0.0.1:/login", now.Add(time.Minute)) {
		t.Fatal("expected request in next window to pass")
	}
}

func performRateLimitedRequest(router http.Handler, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "192.0.2.10:12345"
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}
