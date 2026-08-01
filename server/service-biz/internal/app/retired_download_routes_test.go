package app

import (
	"strings"
	"testing"
)

func TestPublicRoutesDoNotPublishRetiredClientDownloads(t *testing.T) {
	for _, route := range newRouteCatalog(RouteUseCases{}).allRoutes() {
		if route.Path == "/api/client-downloads" ||
			strings.HasSuffix(route.Path, "/client-downloads") ||
			strings.HasPrefix(route.Path, "/downloads/") {
			t.Fatalf("retired client download route is still published: %s", route.Key())
		}
	}
}
