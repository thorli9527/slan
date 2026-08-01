package app

import (
	"testing"

	servicepkg "github.com/slan/service-biz/internal/service"
)

func TestOpsResourcesReuseUnifiedNetworkUseCases(t *testing.T) {
	core := &servicepkg.NetworkCoreService{}
	invites := &servicepkg.NetworkInviteService{}
	dns := &servicepkg.NetworkDNSService{}
	access := &servicepkg.NetworkAccessService{}
	groups := &servicepkg.DeviceGroupService{}

	routes := newRouteUseCases(UseCases{
		Devices: DeviceUseCases{GroupManagement: groups},
		Network: NetworkUseCases{
			CoreAccess:       core,
			InviteManagement: invites,
			DNSManagement:    dns,
			AccessManagement: access,
		},
	})
	deps := opsRouteDependencies(routes)

	if deps.NetworkCore != core || deps.NetworkInvite != invites || deps.NetworkDNS != dns || deps.NetworkAccess != access || deps.DeviceGroup != groups {
		t.Fatal("ops resources must reuse the unified network use cases that own MQTT event publication")
	}
}
