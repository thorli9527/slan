package service

import (
	"testing"

	"github.com/slan/service-biz/internal/model"
)

func TestNextManagedNetworkNameUsesHighestOwnerSequence(t *testing.T) {
	items := []model.Network{
		{Name: "network-01"},
		{Name: "custom"},
		{Name: "NETWORK-09"},
	}
	if got := nextManagedNetworkName(items); got != "network-10" {
		t.Fatalf("nextManagedNetworkName() = %q, want network-10", got)
	}
}
