package ops

import (
	"strings"
	"testing"
)

func TestOpsRoutesOnlyExposeServerNodeManagement(t *testing.T) {
	for _, route := range Routes(RouteDependencies{}) {
		key := route.Key()
		if strings.Contains(key, "/relay-nodes") || strings.Contains(key, "/punch-nodes") {
			t.Fatalf("legacy standalone node route is still exposed: %s", key)
		}
	}
}
