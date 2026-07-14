package service

import "testing"

func TestWireStableRelayCandidateIgnoresRuntimeCandidateOrder(t *testing.T) {
	candidates := []RelayCandidateView{
		{EndpointID: "derp-b", Transport: relayTransportDerpTLS, Address: "host:29122"},
		{EndpointID: "derp-a", Transport: relayTransportDerpTLS, Address: "host:29120"},
	}
	reversed := []RelayCandidateView{candidates[1], candidates[0]}
	seed := stableRelaySessionSeed("network-1", "node-a", "node-b")

	forward, ok := wireStableRelayCandidate(candidates, seed)
	if !ok {
		t.Fatal("expected forward relay candidate")
	}
	reverse, ok := wireStableRelayCandidate(reversed, seed)
	if !ok {
		t.Fatal("expected reverse relay candidate")
	}
	if forward.EndpointID != reverse.EndpointID {
		t.Fatalf("expected stable endpoint across candidate order, got forward=%q reverse=%q", forward.EndpointID, reverse.EndpointID)
	}
}
