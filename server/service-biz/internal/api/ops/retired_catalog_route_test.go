package ops

import (
	"strings"
	"testing"
)

func TestRoutesDoNotPublishRetiredOperationsModules(t *testing.T) {
	retiredPrefixes := []string{
		"/api/ops/plans",
		"/api/ops/products",
		"/api/ops/orders",
		"/api/ops/renewals",
		"/api/ops/client-downloads",
		"/api/ops/users/{userId}/assign-plan",
	}
	for _, route := range Routes(RouteDependencies{}) {
		for _, prefix := range retiredPrefixes {
			if strings.HasPrefix(route.Path, prefix) {
				t.Fatalf("retired operations route is still published: %s", route.Key())
			}
		}
	}
}
