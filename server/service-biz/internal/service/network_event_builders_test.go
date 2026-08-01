package service

import "testing"

func TestNetworkEventIDSeparatesSameMillisecondMemberChanges(t *testing.T) {
	first := newNetworkEventEnvelope(
		NetworkEventMemberRemoved,
		"network-1",
		7,
		1_700_000_000_123,
		NetworkEventMemberRemovedPayload{DeviceID: "device-a"},
	)
	second := newNetworkEventEnvelope(
		NetworkEventMemberRemoved,
		"network-1",
		7,
		1_700_000_000_123,
		NetworkEventMemberRemovedPayload{DeviceID: "device-b"},
	)
	if first.EventID == second.EventID {
		t.Fatalf("distinct member removals share event ID %q", first.EventID)
	}
	if duplicate := newNetworkEventEnvelope(
		NetworkEventMemberRemoved,
		"network-1",
		7,
		1_700_000_000_123,
		NetworkEventMemberRemovedPayload{DeviceID: "device-a"},
	); duplicate.EventID != first.EventID {
		t.Fatalf("identical event identity is not stable: got %q, want %q", duplicate.EventID, first.EventID)
	}
	if len(first.EventID) != len("netevt-")+32 {
		t.Fatalf("unexpected fixed event ID length: %q", first.EventID)
	}
}
