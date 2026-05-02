package impl

import (
	"testing"

	"github.com/slan/server/server-biz/api/dto"
)

func TestNormalizeCreateNetworkDefaultsTreatsLegacyTenDotDefaultsAsEmpty(t *testing.T) {
	req := normalizeCreateNetworkDefaults(dto.CreateNetworkRequest{
		CIDR:              "10.0.0.0/22",
		AllocationStartIP: "10.0.0.2",
		AllocationEndIP:   "10.0.0.254",
	})

	if req.CIDR != "" {
		t.Fatalf("expected legacy cidr to be cleared, got %q", req.CIDR)
	}
	if req.AllocationStartIP != "" {
		t.Fatalf("expected legacy allocation start to be cleared, got %q", req.AllocationStartIP)
	}
	if req.AllocationEndIP != "" {
		t.Fatalf("expected legacy allocation end to be cleared, got %q", req.AllocationEndIP)
	}
}

func TestNormalizeCreateNetworkDefaultsKeepsExplicitNonLegacyValues(t *testing.T) {
	req := normalizeCreateNetworkDefaults(dto.CreateNetworkRequest{
		CIDR:              "172.20.0.0/24",
		AllocationStartIP: "172.20.0.20",
		AllocationEndIP:   "172.20.0.200",
	})

	if req.CIDR != "172.20.0.0/24" {
		t.Fatalf("expected cidr to be preserved, got %q", req.CIDR)
	}
	if req.AllocationStartIP != "172.20.0.20" {
		t.Fatalf("expected allocation start to be preserved, got %q", req.AllocationStartIP)
	}
	if req.AllocationEndIP != "172.20.0.200" {
		t.Fatalf("expected allocation end to be preserved, got %q", req.AllocationEndIP)
	}
}
