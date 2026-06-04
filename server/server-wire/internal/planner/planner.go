package planner

import (
	"sort"
	"time"

	"github.com/slan/server/server-wire/internal/model"
)

const pathProbeFreshnessMs = int64(2 * time.Minute / time.Millisecond)

func BuildPlan(req model.PathPlanRequest) model.PathPlan {
	paths := scorePaths(req.Peer)
	fallback := make([]model.PathKind, 0, len(paths))
	for i := range paths {
		fallback = append(fallback, paths[i].Path)
	}

	preferred := model.PathRelayUDP
	if len(paths) > 0 {
		preferred = paths[0].Path
		paths[0].Primary = true
	}

	return model.PathPlan{
		PreferredPath:      preferred,
		FallbackOrder:      fallback,
		ScoredPaths:        paths,
		Keepalive:          keepalivePlan(req.Peer, preferred),
		MTU:                mtuPlan(req.Peer, preferred, paths),
		Roaming:            roamingPlan(req.Peer),
		RelayTicket:        relayTicketPlan(req.Peer),
		FastReselection:    req.Peer.AllowFastReselection && shouldFastReselect(req.Peer, preferred),
		IPv6Preferred:      req.Peer.PreferIPv6,
		LANDirectPreferred: req.Peer.PreferLAN,
	}
}

func scorePaths(peer model.PeerSnapshot) []model.ScoredPath {
	out := make([]model.ScoredPath, 0, 5)
	for _, probe := range peer.Probes {
		if !supported(peer, probe.Path) || !probe.Reachable || !freshPathProbe(probe) {
			continue
		}
		score := basePriority(peer, probe.Path)
		score += probe.RTTMs
		score += probe.JitterMs
		score += probe.LossPPM / 1000
		score += probe.ConsecutiveFails * 200
		if peer.ActivePath == probe.Path {
			score -= 25
		}
		if peer.EndpointChanged && peer.ActivePath == probe.Path {
			score += 150
		}
		out = append(out, model.ScoredPath{
			Path:   probe.Path,
			Score:  score,
			Reason: reason(peer, probe.Path),
			MTU:    probe.MTU,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score == out[j].Score {
			return out[i].Path < out[j].Path
		}
		return out[i].Score < out[j].Score
	})
	return out
}

func freshPathProbe(probe model.PathProbe) bool {
	if probe.ObservedAt <= 0 {
		return true
	}
	observedAt := probe.ObservedAt
	if observedAt < 1_000_000_000_000 {
		observedAt *= 1000
	}
	now := time.Now().UnixMilli()
	if observedAt > now {
		return observedAt-now <= int64(10*time.Second/time.Millisecond)
	}
	return now-observedAt <= pathProbeFreshnessMs
}

func supported(peer model.PeerSnapshot, path model.PathKind) bool {
	switch path {
	case model.PathLANUDP:
		return peer.SupportsLANDirect
	case model.PathIPv6UDP:
		return peer.SupportsIPv6Direct
	case model.PathDirectUDP:
		return peer.SupportsDirectUDP
	case model.PathRelayUDP:
		return peer.SupportsRelayUDP
	case model.PathDerpTCP443:
		return peer.SupportsDerpTCPTLS443
	default:
		return false
	}
}

func basePriority(peer model.PeerSnapshot, path model.PathKind) int {
	switch path {
	case model.PathLANUDP:
		return 0
	case model.PathIPv6UDP:
		if peer.PreferIPv6 {
			return 30
		}
		return 60
	case model.PathDirectUDP:
		if peer.PreferLAN || peer.PreferIPv6 {
			return 90
		}
		return 70
	case model.PathRelayUDP:
		return 500
	case model.PathDerpTCP443:
		return 900 + derpPenalty(peer)
	default:
		return 1000
	}
}

func reason(peer model.PeerSnapshot, path model.PathKind) string {
	switch path {
	case model.PathLANUDP:
		return "lan direct preferred"
	case model.PathIPv6UDP:
		if peer.PreferIPv6 {
			return "ipv6 direct preferred"
		}
		return "ipv6 direct available"
	case model.PathDirectUDP:
		return "direct udp fallback"
	case model.PathRelayUDP:
		return "relay udp fallback"
	case model.PathDerpTCP443:
		return "derp tcp tls 443 final fallback"
	default:
		return "unknown"
	}
}

func keepalivePlan(peer model.PeerSnapshot, preferred model.PathKind) model.KeepalivePlan {
	interval := peer.KeepaliveIntervalSecs
	if interval <= 0 {
		switch preferred {
		case model.PathLANUDP:
			interval = 0
		case model.PathIPv6UDP:
			interval = 15
		case model.PathDirectUDP:
			interval = 15
		case model.PathRelayUDP:
			interval = 10
		case model.PathDerpTCP443:
			interval = 15
		default:
			interval = 15
		}
	}
	mode := "per_peer"
	if interval == 0 {
		mode = "idle"
	}
	return model.KeepalivePlan{IntervalSecs: interval, Mode: mode}
}

func mtuPlan(peer model.PeerSnapshot, preferred model.PathKind, scored []model.ScoredPath) model.MtuPlan {
	probeRequired := peer.RequireMtuRefresh || peer.EndpointChanged
	target := 1280
	for _, path := range scored {
		if path.Path == preferred && path.MTU > 0 {
			target = path.MTU
			break
		}
	}
	if preferred == model.PathLANUDP && target < 1420 {
		target = 1420
	}
	if preferred == model.PathRelayUDP && target > 1280 {
		target = 1280
	}
	if preferred == model.PathDerpTCP443 && target > 1240 {
		target = 1240
	}
	return model.MtuPlan{
		ProbeRequired: probeRequired,
		TargetMTU:     target,
	}
}

func roamingPlan(peer model.PeerSnapshot) model.RoamingPlan {
	if !peer.AllowEndpointRoaming || !peer.EndpointChanged {
		return model.RoamingPlan{Apply: false}
	}
	return model.RoamingPlan{
		Apply: true,
		Mode:  "update_endpoint_and_reprobe",
	}
}

func relayTicketPlan(peer model.PeerSnapshot) model.RelayTicketPlan {
	if !peer.AllowRelayTicketRenewal || !peer.RelayTicket.Present {
		return model.RelayTicketPlan{}
	}
	const renewWindowMs int64 = 60_000
	return model.RelayTicketPlan{
		RenewRequired: peer.RelayTicket.ExpiresInMs > 0 && peer.RelayTicket.ExpiresInMs <= renewWindowMs,
		RenewWindowMs: renewWindowMs,
	}
}

func shouldFastReselect(peer model.PeerSnapshot, preferred model.PathKind) bool {
	if peer.ActivePath == "" || peer.ActivePath == preferred {
		return false
	}
	if peer.EndpointChanged {
		return true
	}
	return peer.RecentPathDowngrades > 0
}

func derpPenalty(peer model.PeerSnapshot) int {
	if len(peer.DerpHealth) == 0 {
		return 0
	}
	best := 10_000
	reachable := false
	for _, sample := range peer.DerpHealth {
		if !sample.Reachable {
			continue
		}
		reachable = true
		score := sample.RTTMs
		if score < best {
			best = score
		}
	}
	if !reachable {
		return 5_000
	}
	return best
}
