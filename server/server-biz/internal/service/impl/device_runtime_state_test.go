package impl

import (
	"testing"

	"github.com/slan/server/server-biz/api/dto"
)

func TestApplyRuntimeAssignmentSummaryAlignsDerivedStates(t *testing.T) {
	item := dto.NetworkAssignment{
		Status:                  "active",
		VirtualIP:               "10.0.0.2",
		RuntimeStateFresh:       true,
		RuntimeControlReachable: true,
		RuntimeNetworkOnline:    true,
		RuntimeTunnelUp:         true,
		RuntimeVirtualIP:        "10.0.0.2",
	}

	applyRuntimeAssignmentSummary(&item)

	if !item.RuntimeHeartbeatOnline {
		t.Fatal("expected heartbeat to be online")
	}
	if !item.RuntimeIPApplied {
		t.Fatal("expected runtime IP to be applied")
	}
	if !item.RuntimeNetworkEnabled {
		t.Fatal("expected network to be enabled")
	}
	if item.RuntimeDeviceDisabled {
		t.Fatal("expected device to be enabled")
	}
}

func TestApplyRuntimeAssignmentSummaryRequiresMatchingRuntimeIP(t *testing.T) {
	item := dto.NetworkAssignment{
		Status:                  "active",
		VirtualIP:               "10.0.0.2",
		RuntimeStateFresh:       true,
		RuntimeControlReachable: true,
		RuntimeNetworkOnline:    true,
		RuntimeTunnelUp:         true,
		RuntimeVirtualIP:        "10.0.0.5",
	}

	applyRuntimeAssignmentSummary(&item)

	if !item.RuntimeHeartbeatOnline {
		t.Fatal("expected heartbeat to remain online")
	}
	if item.RuntimeIPApplied {
		t.Fatal("expected runtime IP mismatch to be visible")
	}
	if item.RuntimeNetworkEnabled {
		t.Fatal("expected network enabled to wait for matching runtime IP")
	}
}

func TestApplyRuntimeAssignmentSummaryTreatsDisabledDeviceOffline(t *testing.T) {
	item := dto.NetworkAssignment{
		Status:                  "disabled",
		VirtualIP:               "10.0.0.2",
		RuntimeStateFresh:       true,
		RuntimeControlReachable: true,
		RuntimeNetworkOnline:    true,
		RuntimeTunnelUp:         true,
		RuntimeVirtualIP:        "10.0.0.2",
	}

	applyRuntimeAssignmentSummary(&item)

	if !item.RuntimeDeviceDisabled {
		t.Fatal("expected disabled attachment to mark device disabled")
	}
	if item.RuntimeHeartbeatOnline {
		t.Fatal("expected disabled device to be excluded from heartbeat online")
	}
	if item.RuntimeNetworkEnabled {
		t.Fatal("expected disabled device to be excluded from network enabled")
	}
}
