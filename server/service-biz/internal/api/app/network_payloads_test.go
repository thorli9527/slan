package app

import (
	"testing"

	servicepkg "github.com/slan/service-biz/internal/service"
)

func TestNetworkResolvedConfigPayloadUsesStoredConfigVersion(t *testing.T) {
	payload := networkResolvedConfigPayload(servicepkg.NetworkResolvedConfigView{
		Config: servicepkg.NetworkConfigView{
			Network: servicepkg.NetworkView{
				NetworkID: "net-1",
				Name:      "Default",
				UpdatedAt: 123,
			},
			ConfigVersion: 456,
			DeviceID:      "dev-1",
			NodeID:        "node-dev-1",
			GlobalIP:      "10.0.0.2",
			PrefixLen:     24,
		},
	})

	got, _ := payload["configVersion"].(int64)
	if got != 456 {
		t.Fatalf("expected configVersion=456, got %v", payload["configVersion"])
	}
}
