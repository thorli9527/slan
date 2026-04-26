package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParsePublicGoRoutes(t *testing.T) {
	dir := t.TempDir()
	writeRouteFixtures(t, dir)

	routes, err := parsePublicGoRoutes(dir)
	if err != nil {
		t.Fatalf("parse public routes: %v", err)
	}

	for _, route := range []routeSpec{
		{Method: "GET", Path: "/healthz"},
		{Method: "POST", Path: "/auth/register"},
		{Method: "PUT", Path: "/networks/{networkId}/dns"},
		{Method: "POST", Path: "/mqtt/auth/check"},
		{Method: "POST", Path: "/mqtt/bifromq/auth"},
		{Method: "POST", Path: "/mqtt/bifromq/check"},
		{Method: "PUT", Path: "/devices/{deviceId}/networks/{networkId}/state"},
		{Method: "GET", Path: "/auth/ws/{callbackId}"},
		{Method: "GET", Path: "/control/ws"},
	} {
		if _, ok := routes[route]; !ok {
			t.Fatalf("expected route %s, got %#v", route.String(), routes)
		}
	}
}

func TestCheckHTTPRoutesDetectsDrift(t *testing.T) {
	dir := t.TempDir()
	routesDir := filepath.Join(dir, "routes")
	if err := os.Mkdir(routesDir, 0o755); err != nil {
		t.Fatalf("mkdir routes: %v", err)
	}
	writeRouteFixtures(t, routesDir)

	openAPIPath := filepath.Join(dir, "phase1.yaml")
	writeFile(t, openAPIPath, `openapi: 3.1.0
paths:
  /healthz:
    get: {}
  /auth/register:
    post: {}
  /mqtt/auth/check:
    post: {}
  /mqtt/bifromq/auth:
    post: {}
  /mqtt/bifromq/check:
    post: {}
  /devices/{deviceId}/networks/{networkId}/state:
    put: {}
  /networks/{networkId}/dns:
    put: {}
  /auth/ws/{callbackId}:
    get: {}
components:
  schemas: {}
`)

	err := checkHTTPRoutes(routesDir, openAPIPath)
	if err == nil {
		t.Fatal("expected route drift error")
	}
	if !strings.Contains(err.Error(), "GET /control/ws") {
		t.Fatalf("expected missing control websocket route, got %v", err)
	}
}

func writeRouteFixtures(t *testing.T, dir string) {
	t.Helper()
	files := map[string]string{
		"routes.go": `package httpapi

func routes(router interface{}) {
	router.GET("/healthz", healthz)
}`,
		"routes_business_access.go": `package httpapi

func routes(api interface{}) {
	auth := api.Group("/auth")
	auth.POST("/register", handler)
	api.POST("/mqtt/auth/check", handler)
	api.POST("/mqtt/bifromq/auth", handler)
	api.POST("/mqtt/bifromq/check", handler)
}`,
		"routes_business_registration.go": `package httpapi

func routes(protected interface{}) {
	devices := protected.Group("/devices")
	devices.PUT("/:deviceId/networks/:networkId/state", handler)
}
`,
		"routes_business_network.go": `package httpapi

func routes(protected interface{}) {
	networks := protected.Group("/networks")
	networks.PUT("/:networkId/dns", handler)
}`,
		"routes_business_bootstrap.go": `package httpapi

func routes(protected interface{}) {
}`,
		"auth_callback_ws.go": `package httpapi

func routes(router interface{}) {
	router.GET("/auth/ws/:callbackId", handler)
}`,
		"control_ws_sync.go": `package httpapi

func routes(router interface{}, path string) {
	router.GET(path, handler)
}`,
	}

	for name, content := range files {
		writeFile(t, filepath.Join(dir, name), content)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
