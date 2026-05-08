package httpapi

import (
	"log"
	"strconv"
	"sync"
	"time"

	controlmsg "github.com/slan/server/server-biz/internal/controlmsg"
	"github.com/slan/server/server-biz/internal/util"
)

// connectPlanThrottle tracks connect-plan retry backoff in this process.
type connectPlanThrottle struct {
	mu     sync.Mutex
	states map[string]connectPlanRetryState
}

type connectPlanRetryState struct {
	failures      int
	nextAllowedAt time.Time
}

// peerCandidateWindow deduplicates peer-candidate delivery in this process.
type peerCandidateWindow struct {
	mu      sync.Mutex
	expires map[string]time.Time
}

func newConnectPlanThrottle() *connectPlanThrottle {
	return &connectPlanThrottle{
		states: make(map[string]connectPlanRetryState),
	}
}

func newPeerCandidateWindow() *peerCandidateWindow {
	return &peerCandidateWindow{
		expires: make(map[string]time.Time),
	}
}

func (t *connectPlanThrottle) allow(networkID, nodeID, peerNodeID string) bool {
	key := connectPlanPairKey(networkID, nodeID, peerNodeID)
	now := time.Now()

	t.mu.Lock()
	defer t.mu.Unlock()

	state := t.states[key]
	if now.Before(state.nextAllowedAt) {
		return false
	}

	state.failures++
	state.nextAllowedAt = now.Add(connectPlanBackoff(state.failures))
	t.states[key] = state
	return true
}

func (t *connectPlanThrottle) reset(networkID, nodeID, peerNodeID string) {
	key := connectPlanPairKey(networkID, nodeID, peerNodeID)
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.states, key)
}

func connectPlanPairKey(networkID, nodeID, peerNodeID string) string {
	if nodeID > peerNodeID {
		nodeID, peerNodeID = peerNodeID, nodeID
	}
	return networkID + "|" + nodeID + "|" + peerNodeID
}

func connectPlanBackoff(failures int) time.Duration {
	switch {
	case failures <= 1:
		return 0
	case failures == 2:
		return 2 * time.Second
	case failures == 3:
		return 5 * time.Second
	case failures == 4:
		return 10 * time.Second
	default:
		return 20 * time.Second
	}
}

func (w *peerCandidateWindow) acquire(networkID, sourceNodeID, targetNodeID string, candidate controlmsg.PeerCandidate, ttl time.Duration) bool {
	if ttl <= 0 {
		ttl = 10 * time.Second
	}
	key := peerCandidateKey(networkID, sourceNodeID, targetNodeID, candidate)
	now := time.Now()

	w.mu.Lock()
	defer w.mu.Unlock()

	for existingKey, expiresAt := range w.expires {
		if !expiresAt.After(now) {
			delete(w.expires, existingKey)
		}
	}
	if expiresAt, ok := w.expires[key]; ok && expiresAt.After(now) {
		return false
	}
	w.expires[key] = now.Add(ttl)
	return true
}

func peerCandidateKey(networkID, sourceNodeID, targetNodeID string, candidate controlmsg.PeerCandidate) string {
	return networkID + "|" + sourceNodeID + "|" + targetNodeID + "|" + candidate.CandidateType + "|" + candidate.Endpoint + "|" + strconv.Itoa(candidate.Priority)
}

func allowConnectPlanRetry(deps routerDeps, networkID, nodeID, peerNodeID string) bool {
	if deps.ControlSync != nil {
		allowed, err := deps.ControlSync.AcquireConnectPlanRetry(networkID, nodeID, peerNodeID)
		if err == nil {
			if allowed {
				metricAdd("connect_plan_retry_allowed_total", 1)
			} else {
				metricAdd("connect_plan_retry_dropped_total", 1)
			}
			return allowed
		}
		log.Printf("control-mqtt connect-plan retry sync fallback network=%s node=%s peer=%s err=%v", networkID, nodeID, peerNodeID, err)
		metricAdd("connect_plan_retry_sync_error_total", 1)
	}
	allowed := defaultConnectPlanThrottle.allow(networkID, nodeID, peerNodeID)
	if allowed {
		metricAdd("connect_plan_retry_allowed_total", 1)
	} else {
		metricAdd("connect_plan_retry_dropped_total", 1)
	}
	return allowed
}

func resetConnectPlanRetry(deps routerDeps, networkID, nodeID, peerNodeID string) {
	defaultConnectPlanThrottle.reset(networkID, nodeID, peerNodeID)
	metricAdd("connect_plan_retry_reset_total", 1)
	if deps.ControlSync != nil {
		_ = deps.ControlSync.ResetConnectPlanRetry(networkID, nodeID, peerNodeID)
	}
}

func allowPeerCandidateDelivery(deps routerDeps, networkID, sourceNodeID, targetNodeID string, candidate controlmsg.PeerCandidate) bool {
	const candidateTTL = 10 * time.Second
	if deps.ControlSync != nil {
		allowed, err := deps.ControlSync.AcquirePeerCandidateDelivery(networkID, sourceNodeID, targetNodeID, candidate, candidateTTL)
		if err == nil {
			if allowed {
				metricAdd("peer_candidate_delivery_allowed_total", 1)
			} else {
				metricAdd("peer_candidate_delivery_dropped_total", 1)
			}
			return allowed
		}
		log.Printf("control-mqtt peer-candidate sync fallback network=%s source=%s target=%s endpoint=%s err=%v", networkID, sourceNodeID, targetNodeID, candidate.Endpoint, err)
		metricAdd("peer_candidate_delivery_sync_error_total", 1)
	}
	allowed := defaultPeerCandidateWindow.acquire(networkID, sourceNodeID, targetNodeID, candidate, candidateTTL)
	if allowed {
		metricAdd("peer_candidate_delivery_allowed_total", 1)
	} else {
		metricAdd("peer_candidate_delivery_dropped_total", 1)
	}
	return allowed
}

func forwardPeerCandidate(deps routerDeps, session controlSession, candidate controlmsg.PeerCandidate) {
	if !allowPeerCandidateDelivery(deps, session.networkID, session.nodeID, candidate.PeerNodeID, candidate) {
		return
	}
	metricAdd("peer_candidate_forward_total", 1)
	sendPeerCandidateToNode(deps, session.networkID, candidate.PeerNodeID, candidate)
	if deps.ControlSync != nil {
		rev, _ := deps.ControlSync.CurrentRevision(session.networkID)
		_ = deps.ControlSync.Publish(controlmsg.ControlSyncEvent{
			InstanceID:   controlmsgInstanceID,
			Type:         "peer_candidate",
			NetworkID:    session.networkID,
			SourceNodeID: session.nodeID,
			TargetNodeID: candidate.PeerNodeID,
			Revision:     rev,
			Candidate:    util.Ptr(candidate),
		})
		metricAdd("sync_event_published_total", 1)
	}
}
