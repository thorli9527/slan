package httpapi

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/configs"
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
	if rec.Header().Get("Retry-After") == "" {
		t.Fatalf("expected Retry-After header on rate limited response")
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

func TestRequestRateLimiterReportsRetryAfter(t *testing.T) {
	limiter := newRequestRateLimiter(1, time.Minute)
	now := time.Unix(100, 0)

	if decision := limiter.check("127.0.0.1:/login", now); !decision.allowed {
		t.Fatalf("expected first request to pass, got %+v", decision)
	}
	decision := limiter.check("127.0.0.1:/login", now.Add(10*time.Second))
	if decision.allowed {
		t.Fatalf("expected second request to be limited")
	}
	if decision.retryAfter != 50*time.Second {
		t.Fatalf("expected retry after 50s, got %s", decision.retryAfter)
	}
}

func TestCeilSeconds(t *testing.T) {
	tests := []struct {
		name  string
		value time.Duration
		want  int
	}{
		{name: "zero", value: 0, want: 0},
		{name: "sub_second", value: time.Millisecond, want: 1},
		{name: "exact_second", value: time.Second, want: 1},
		{name: "partial_second", value: time.Second + time.Nanosecond, want: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ceilSeconds(tt.value); got != tt.want {
				t.Fatalf("expected %d, got %d", tt.want, got)
			}
		})
	}
}

func TestRateLimitJSONFieldKeyDoesNotExposeRawValue(t *testing.T) {
	key := rateLimitJSONFieldKey("email", "user@example.com")
	if !strings.HasPrefix(key, "email=") {
		t.Fatalf("expected field prefix, got %q", key)
	}
	if strings.Contains(key, "user@example.com") {
		t.Fatalf("expected key to avoid raw field value, got %q", key)
	}
	if key != rateLimitJSONFieldKey("email", "user@example.com") {
		t.Fatalf("expected stable key for same field value")
	}
	if key == rateLimitJSONFieldKey("email", "other@example.com") {
		t.Fatalf("expected different field values to produce different keys")
	}
}

func TestOpsLoginRateLimitHandlersDisabled(t *testing.T) {
	handlers := opsLoginRateLimitHandlers(configs.OpsLoginRateLimitConfig{Disabled: true})
	if len(handlers) != 0 {
		t.Fatalf("expected disabled ops login rate limit to return no handlers, got %d", len(handlers))
	}
}

func TestOpsLoginRateLimitHandlersZeroConfigUsesDefaults(t *testing.T) {
	handlers := opsLoginRateLimitHandlers(configs.OpsLoginRateLimitConfig{})
	if len(handlers) != 3 {
		t.Fatalf("expected zero ops login rate limit config to use default handlers, got %d", len(handlers))
	}
}

func TestOpsLoginRateLimitHandlersEnabled(t *testing.T) {
	handlers := opsLoginRateLimitHandlers(configs.OpsLoginRateLimitConfig{
		WindowSeconds:    30,
		IPLimit:          2,
		IPLoginNameLimit: 1,
		LoginNameLimit:   3,
	})
	if len(handlers) != 3 {
		t.Fatalf("expected enabled ops login rate limit to return 3 handlers, got %d", len(handlers))
	}
}

func TestRateLimitByIPAndJSONFieldSeparatesValues(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/login", rateLimitByIPAndJSONField(1, time.Minute, "email"), func(c *gin.Context) {
		var payload map[string]string
		if err := c.ShouldBindJSON(&payload); err != nil {
			t.Fatalf("bind body after limiter: %v", err)
		}
		c.JSON(http.StatusOK, gin.H{"email": payload["email"]})
	})

	if rec := performRateLimitedJSONRequest(router, "/login", `{"email":"A@Example.com"}`); rec.Code != http.StatusOK {
		t.Fatalf("expected first email to pass, got %d body=%s", rec.Code, rec.Body.String())
	}
	if rec := performRateLimitedJSONRequest(router, "/login", `{"email":"b@example.com"}`); rec.Code != http.StatusOK {
		t.Fatalf("expected different email to pass, got %d body=%s", rec.Code, rec.Body.String())
	}
	if rec := performRateLimitedJSONRequest(router, "/login", `{"email":" a@example.com "}`); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected normalized duplicate email to be limited, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestRateLimitByJSONFieldLimitsAcrossIPs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/login", rateLimitByJSONField(1, time.Minute, "email"), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	if rec := performRateLimitedJSONRequestFromIP(router, "/login", `{"email":"user@example.com"}`, "192.0.2.10:12345"); rec.Code != http.StatusOK {
		t.Fatalf("expected first request to pass, got %d body=%s", rec.Code, rec.Body.String())
	}
	if rec := performRateLimitedJSONRequestFromIP(router, "/login", `{"email":"user@example.com"}`, "192.0.2.11:12345"); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected second request for same email to be limited across ips, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestRateLimitByJSONFieldSkipsMissingField(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/login", rateLimitByJSONField(1, time.Minute, "email"), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	if rec := performRateLimitedJSONRequest(router, "/login", `{"password":"one"}`); rec.Code != http.StatusOK {
		t.Fatalf("expected request without email to pass, got %d body=%s", rec.Code, rec.Body.String())
	}
	if rec := performRateLimitedJSONRequest(router, "/login", `{"password":"two"}`); rec.Code != http.StatusOK {
		t.Fatalf("expected second request without email to pass, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestRateLimitByIPAndJSONFieldFallsBackToIPWhenFieldMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/login", rateLimitByIPAndJSONField(1, time.Minute, "email"), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	if rec := performRateLimitedJSONRequest(router, "/login", `{"password":"one"}`); rec.Code != http.StatusOK {
		t.Fatalf("expected first request without email to pass, got %d body=%s", rec.Code, rec.Body.String())
	}
	if rec := performRateLimitedJSONRequest(router, "/login", `{"password":"two"}`); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected missing field to fall back to ip route bucket, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestLoginRateLimitChainPreservesBodyForHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(limitRequestBody(maxHTTPJSONBodyBytes))
	router.POST(
		"/login",
		rateLimitByIP(defaultOpsLoginIPLimit, defaultOpsLoginRateLimitWindow),
		rateLimitByIPAndJSONField(defaultOpsLoginIPLoginNameLimit, defaultOpsLoginRateLimitWindow, "loginName"),
		rateLimitByJSONField(defaultOpsLoginLoginNameLimit, defaultOpsLoginRateLimitWindow, "loginName"),
		func(c *gin.Context) {
			var payload map[string]string
			if !bindJSON(c, &payload) {
				return
			}
			c.JSON(http.StatusOK, gin.H{"loginName": payload["loginName"]})
		},
	)

	rec := performRateLimitedJSONRequest(router, "/login", `{"loginName":"ops-admin","password":"StrongPass-2026"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected chained limiters to preserve body, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "ops-admin") {
		t.Fatalf("expected handler to receive original body, got %s", rec.Body.String())
	}
}

func TestLoginRateLimitChainLetsInvalidJSONReachBinder(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(limitRequestBody(maxHTTPJSONBodyBytes))
	router.POST(
		"/login",
		rateLimitByIP(defaultOpsLoginIPLimit, defaultOpsLoginRateLimitWindow),
		rateLimitByIPAndJSONField(defaultOpsLoginIPLoginNameLimit, defaultOpsLoginRateLimitWindow, "loginName"),
		rateLimitByJSONField(defaultOpsLoginLoginNameLimit, defaultOpsLoginRateLimitWindow, "loginName"),
		func(c *gin.Context) {
			var payload map[string]string
			if !bindJSON(c, &payload) {
				return
			}
			c.JSON(http.StatusOK, gin.H{"loginName": payload["loginName"]})
		},
	)

	rec := performRateLimitedJSONRequest(router, "/login", `{"loginName":`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid JSON to reach binder, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "INVALID_ARGUMENT") {
		t.Fatalf("expected INVALID_ARGUMENT response, got %s", rec.Body.String())
	}
}

func TestRequestJSONFieldCachesParsedValue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"email":"first@example.com"}`))

	fieldValue, ok := requestJSONField(c, "email")
	if !ok || fieldValue != "first@example.com" {
		t.Fatalf("expected first parsed field, got value=%q ok=%t", fieldValue, ok)
	}

	c.Request.Body = io.NopCloser(strings.NewReader(`{"email":"second@example.com"}`))
	fieldValue, ok = requestJSONField(c, "email")
	if !ok || fieldValue != "first@example.com" {
		t.Fatalf("expected cached field value, got value=%q ok=%t", fieldValue, ok)
	}
}

func TestRequestJSONFieldAllowsEmptyBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(""))

	fieldValue, ok := requestJSONField(c, "email")
	if !ok || fieldValue != "" {
		t.Fatalf("expected empty body to pass with empty field, got value=%q ok=%t", fieldValue, ok)
	}
}

func TestRateLimitJSONFieldRejectsOversizedBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/login", rateLimitByIPAndJSONField(1, time.Minute, "email"), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	payload, err := json.Marshal(map[string]string{
		"email": strings.Repeat("a", maxHTTPJSONBodyBytes),
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	rec := performRateLimitedJSONRequest(router, "/login", string(payload))
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected oversized body to be rejected, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAccessLoginRouteIsNotRateLimited(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(limitRequestBody(maxHTTPJSONBodyBytes))
	registerAccessRoutes(router.Group(""), routerDeps{Auth: fakeAuthService{}})

	for i := 0; i < defaultOpsLoginIPLimit+2; i++ {
		rec := performRateLimitedJSONRequest(router, "/auth/login", `{"email":"user@example.com","password":"StrongPass-2026"}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected public auth login request %d to pass without ops limiter, got %d body=%s", i+1, rec.Code, rec.Body.String())
		}
	}
}

func TestOpsLoginRouteUsesConfiguredRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := configs.DefaultConfig()
	cfg.Ops.LoginRateLimit = configs.OpsLoginRateLimitConfig{
		WindowSeconds:    60,
		IPLimit:          1,
		IPLoginNameLimit: 10,
		LoginNameLimit:   10,
	}
	router := gin.New()
	router.Use(limitRequestBody(maxHTTPJSONBodyBytes))
	registerOpsRoutes(router.Group(""), cfg, routerDeps{Ops: fakeOpsService{}})

	body := `{"loginName":"ops-admin","password":"StrongPass-2026"}`
	if rec := performRateLimitedJSONRequest(router, "/login", body); rec.Code != http.StatusOK {
		t.Fatalf("expected first ops login to pass, got %d body=%s", rec.Code, rec.Body.String())
	}
	if rec := performRateLimitedJSONRequest(router, "/login", body); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected configured ops login limiter to reject second request, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestOpsLoginRouteCanDisableRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := configs.DefaultConfig()
	cfg.Ops.LoginRateLimit = configs.OpsLoginRateLimitConfig{
		Disabled:         true,
		WindowSeconds:    60,
		IPLimit:          1,
		IPLoginNameLimit: 1,
		LoginNameLimit:   1,
	}
	router := gin.New()
	router.Use(limitRequestBody(maxHTTPJSONBodyBytes))
	registerOpsRoutes(router.Group(""), cfg, routerDeps{Ops: fakeOpsService{}})

	body := `{"loginName":"ops-admin","password":"StrongPass-2026"}`
	for i := 0; i < 2; i++ {
		rec := performRateLimitedJSONRequest(router, "/login", body)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected disabled ops login limiter to pass request %d, got %d body=%s", i+1, rec.Code, rec.Body.String())
		}
	}
}

func performRateLimitedRequest(router http.Handler, path string) *httptest.ResponseRecorder {
	return performRateLimitedJSONRequest(router, path, `{}`)
}

func performRateLimitedJSONRequest(router http.Handler, path, body string) *httptest.ResponseRecorder {
	return performRateLimitedJSONRequestFromIP(router, path, body, "192.0.2.10:12345")
}

func performRateLimitedJSONRequestFromIP(router http.Handler, path, body, remoteAddr string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = remoteAddr
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

type fakeAuthService struct{}

func (fakeAuthService) Register(req dto.RegisterRequest) (dto.AuthResponse, error) {
	return dto.AuthResponse{UserID: "user-1", AccessToken: "access", RefreshToken: "refresh", ExpiresIn: 3600}, nil
}

func (fakeAuthService) Login(req dto.LoginRequest) (dto.AuthResponse, error) {
	return dto.AuthResponse{UserID: "user-1", AccessToken: "access", RefreshToken: "refresh", ExpiresIn: 3600}, nil
}

func (fakeAuthService) Refresh(req dto.RefreshTokenRequest) (dto.AuthResponse, error) {
	return dto.AuthResponse{UserID: "user-1", AccessToken: "access", RefreshToken: "refresh", ExpiresIn: 3600}, nil
}

func (fakeAuthService) ChangePassword(userID string, req dto.ChangePasswordRequest) error {
	return nil
}

func (fakeAuthService) GetCallbackStatus(callbackID string) (dto.AuthCallbackStatusResponse, error) {
	return dto.AuthCallbackStatusResponse{CallbackID: callbackID}, nil
}

func (fakeAuthService) CompleteCallback(callbackID string, req dto.CompleteAuthCallbackRequest) error {
	return nil
}

type fakeOpsService struct{}

func (fakeOpsService) LoginAdmin(req dto.OpsLoginRequest, remoteIP string) (dto.OpsLoginResponse, error) {
	return dto.OpsLoginResponse{
		AdminID:     "admin-1",
		UserID:      "user-1",
		LoginName:   req.LoginName,
		DisplayName: "Ops Admin",
		AccessToken: "ops-access",
		ExpiresIn:   28800,
	}, nil
}

func (fakeOpsService) AuthenticateAdminToken(accessToken string) (string, error) {
	return "admin-1", nil
}

func (fakeOpsService) AuthorizeAdminMenu(adminID string, menuCode string) error {
	return nil
}

func (fakeOpsService) Overview() (dto.OpsOverview, error) {
	return dto.OpsOverview{}, nil
}

func (fakeOpsService) ListUsers() ([]dto.OpsUser, error) {
	return nil, nil
}

func (fakeOpsService) ListDevices() ([]dto.OpsDevice, error) {
	return nil, nil
}

func (fakeOpsService) RelayTopology() (dto.OpsRelayTopology, error) {
	return dto.OpsRelayTopology{}, nil
}

func (fakeOpsService) ListAdmins() ([]dto.OpsAdminInfo, error) {
	return nil, nil
}

func (fakeOpsService) UpsertAdminInfo(req dto.UpsertAdminInfoRequest) (dto.OpsAdminInfo, error) {
	return dto.OpsAdminInfo{}, nil
}

func (fakeOpsService) ChangeAdminPassword(adminID string, req dto.ChangeAdminPasswordRequest) error {
	return nil
}

func (fakeOpsService) ChangeOwnAdminPassword(adminID string, req dto.ChangeAdminPasswordRequest) error {
	return nil
}

func (fakeOpsService) LogoutAdmin(accessToken string) error {
	return nil
}

func (fakeOpsService) UnlockAdmin(adminID string) error {
	return nil
}

func (fakeOpsService) ListRoles() ([]dto.OpsRole, error) {
	return nil, nil
}

func (fakeOpsService) CreateRole(req dto.CreateRoleRequest) (dto.OpsRole, error) {
	return dto.OpsRole{}, nil
}

func (fakeOpsService) AssignUserRoles(userID string, req dto.AssignUserRolesRequest) error {
	return nil
}

func (fakeOpsService) ListMenus() ([]dto.OpsMenu, error) {
	return nil, nil
}

func (fakeOpsService) CreateMenu(req dto.CreateMenuRequest) (dto.OpsMenu, error) {
	return dto.OpsMenu{}, nil
}

func (fakeOpsService) AssignRoleMenus(roleID string, req dto.AssignRoleMenusRequest) error {
	return nil
}

func (fakeOpsService) ListProducts() ([]dto.OpsProduct, error) {
	return nil, nil
}

func (fakeOpsService) UpsertProduct(req dto.UpsertProductRequest) (dto.OpsProduct, error) {
	return dto.OpsProduct{}, nil
}

func (fakeOpsService) ListPurchaseOrders() ([]dto.OpsPurchaseOrder, error) {
	return nil, nil
}

func (fakeOpsService) CreatePaidPurchaseOrder(req dto.OpsCreatePaidOrderRequest) (dto.OpsPurchaseOrder, error) {
	return dto.OpsPurchaseOrder{}, nil
}

func (fakeOpsService) UpdatePurchaseOrderStatus(orderID string, req dto.UpdatePurchaseOrderStatusRequest) (dto.OpsPurchaseOrder, error) {
	return dto.OpsPurchaseOrder{}, nil
}
