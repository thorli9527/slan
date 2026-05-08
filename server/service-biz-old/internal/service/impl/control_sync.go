package impl

import (
	"context"
	"time"

	controlmsg "github.com/slan/server/server-biz/internal/controlmsg"
)

// Publish emits a control-sync event into the shared pub/sub channel.
func (s dbControlSyncService) Publish(event controlmsg.ControlSyncEvent) error {
	return s.state.tokens.PublishControlSyncEvent(context.Background(), event)
}

// Subscribe registers a callback that receives control-sync events published by
// any biz instance.
func (s dbControlSyncService) Subscribe(handler func(controlmsg.ControlSyncEvent)) error {
	return s.state.tokens.SubscribeControlSyncEvents(context.Background(), handler)
}

// NextRevision increments and returns the current network revision counter.
func (s dbControlSyncService) NextRevision(networkID string) (uint64, error) {
	return s.state.tokens.NextNetworkRevision(context.Background(), networkID)
}

// CurrentRevision reads the current network revision without mutating it.
func (s dbControlSyncService) CurrentRevision(networkID string) (uint64, error) {
	return s.state.tokens.CurrentNetworkRevision(context.Background(), networkID)
}

// AcquireConnectPlanRetry provides a retry window so the same pair is not
// continuously re-planned.
func (s dbControlSyncService) AcquireConnectPlanRetry(networkID, nodeID, peerNodeID string) (bool, error) {
	return s.state.tokens.AcquireConnectPlanRetry(context.Background(), networkID, nodeID, peerNodeID)
}

// ResetConnectPlanRetry clears the retry gate after successful delivery or
// when the pending retry is no longer relevant.
func (s dbControlSyncService) ResetConnectPlanRetry(networkID, nodeID, peerNodeID string) error {
	return s.state.tokens.ResetConnectPlanRetry(context.Background(), networkID, nodeID, peerNodeID)
}

// AcquirePeerCandidateDelivery de-duplicates repeated candidate forwarding for
// a short TTL window.
func (s dbControlSyncService) AcquirePeerCandidateDelivery(networkID, sourceNodeID, targetNodeID string, candidate controlmsg.PeerCandidate, ttl time.Duration) (bool, error) {
	return s.state.tokens.AcquirePeerCandidateDelivery(context.Background(), networkID, sourceNodeID, targetNodeID, candidate, ttl)
}
