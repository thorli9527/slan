package impl

import (
	"context"
	"time"
)

const controlSessionFreshnessWindow = 45 * time.Second
const deviceNetworkStateFreshnessWindow = 45 * time.Second
const deviceBoundWebSessionFreshnessWindow = 2 * time.Minute
const nodeEndpointFreshnessWindow = 2 * time.Minute
const nodeConnectionStateFreshnessWindow = 2 * time.Minute
const nodePathHealthFreshnessWindow = 30 * time.Minute
const relayNodeHeartbeatFreshnessWindow = 2 * time.Minute
const controlStateCleanupInterval = 1 * time.Minute

// endpointCutoffUnix returns the oldest endpoint timestamp still trusted by the
// control plane.
func endpointCutoffUnix(now time.Time) int64 {
	return now.Add(-nodeEndpointFreshnessWindow).Unix()
}

// connectionStateCutoffUnix returns the oldest connection-state timestamp still
// considered for planning.
func connectionStateCutoffUnix(now time.Time) int64 {
	return now.Add(-nodeConnectionStateFreshnessWindow).Unix()
}

// pathHealthCutoffUnix returns the oldest path-health sample still considered
// useful for ranking.
func pathHealthCutoffUnix(now time.Time) int64 {
	return now.Add(-nodePathHealthFreshnessWindow).Unix()
}

// controlSessionCutoffUnix returns the oldest last-seen timestamp still treated
// as an active control session.
func controlSessionCutoffUnix(now time.Time) int64 {
	return now.Add(-controlSessionFreshnessWindow).Unix()
}

func deviceNetworkStateCutoffUnix(now time.Time) int64 {
	return now.Add(-deviceNetworkStateFreshnessWindow).Unix()
}

// startControlStateCleanupLoop starts the background janitor that periodically
// trims transient control-plane state.
func (s *dbState) startControlStateCleanupLoop() {
	go func() {
		ticker := time.NewTicker(controlStateCleanupInterval)
		defer ticker.Stop()

		for range ticker.C {
			s.cleanupExpiredControlPlaneState(context.Background(), time.Now())
		}
	}()
}

// cleanupExpiredControlPlaneState performs one cleanup pass over transient
// control-plane rows and in-memory relay ticket cache.
func (s *dbState) cleanupExpiredControlPlaneState(ctx context.Context, now time.Time) {
	_ = s.pg.MarkDevicesOfflineWithoutFreshControlSession(ctx, controlSessionCutoffUnix(now))
	_ = s.pg.MarkStaleDeviceNetworkStatesOffline(ctx, deviceNetworkStateCutoffUnix(now), now.Unix())
	_ = s.pg.DeleteControlSessionsBefore(ctx, controlSessionCutoffUnix(now))
	_ = s.pg.DeleteNodeEndpointsBeforeAll(ctx, endpointCutoffUnix(now))
	_ = s.pg.DeleteNodeConnectionStatesBeforeAll(ctx, connectionStateCutoffUnix(now))
	_ = s.pg.DeleteNodePathHealthBeforeAll(ctx, pathHealthCutoffUnix(now))
	_ = s.pg.DeleteNodePathHealthSamplesBeforeAll(ctx, uint64(now.Add(-7*24*time.Hour).UnixMilli()))
	_ = s.pg.DeleteRelayNodeHeartbeatsBefore(ctx, now.Add(-24*time.Hour).Unix())
	s.pruneExpiredRelayTickets(now)
}
