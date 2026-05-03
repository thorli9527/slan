package httpapi

import (
	"sort"
	"strings"
	"time"

	"github.com/slan/server/server-biz/api/dto"
	controlmsg "github.com/slan/server/server-biz/internal/controlmsg"
	"github.com/slan/server/server-biz/internal/netpath"
	"github.com/slan/server/server-biz/internal/service"
	"github.com/slan/server/server-biz/internal/util"
)

func publishOpsRelayDataPlanePolicy(deps routerDeps, req dto.OpsRelayDataPlanePolicyRequest) (dto.OpsRelayDataPlanePolicyResponse, error) {
	networkID := strings.TrimSpace(req.NetworkID)
	if networkID == "" || req.RelayMtu < 576 || req.RelayMtu > 1500 || req.MaxFramePayload < 512 || req.MaxFramePayload > 1400 || req.MaxFramePayload >= req.RelayMtu {
		return dto.OpsRelayDataPlanePolicyResponse{}, service.ErrInvalidArgument
	}
	if req.RecommendationLevel != nil && *req.RecommendationLevel > 11 {
		return dto.OpsRelayDataPlanePolicyResponse{}, service.ErrInvalidArgument
	}
	if req.ExecutionLevel != nil && *req.ExecutionLevel > 7 {
		return dto.OpsRelayDataPlanePolicyResponse{}, service.ErrInvalidArgument
	}
	if req.ProbeIntervalMS != 0 && (req.ProbeIntervalMS < 1000 || req.ProbeIntervalMS > 300000) {
		return dto.OpsRelayDataPlanePolicyResponse{}, service.ErrInvalidArgument
	}
	if req.FailoverAfterMS != 0 && (req.FailoverAfterMS < 1000 || req.FailoverAfterMS > 600000) {
		return dto.OpsRelayDataPlanePolicyResponse{}, service.ErrInvalidArgument
	}
	if req.UpgradeSuccesses != 0 && (req.UpgradeSuccesses < 1 || req.UpgradeSuccesses > 10) {
		return dto.OpsRelayDataPlanePolicyResponse{}, service.ErrInvalidArgument
	}
	if req.FailedPathCooldownProbes != 0 && (req.FailedPathCooldownProbes < 1 || req.FailedPathCooldownProbes > 20) {
		return dto.OpsRelayDataPlanePolicyResponse{}, service.ErrInvalidArgument
	}
	preferredPathTypes, ok := netpath.NormalizePreferredPathTypes(req.PreferredPathTypes)
	if !ok {
		return dto.OpsRelayDataPlanePolicyResponse{}, service.ErrInvalidArgument
	}
	version := req.Version
	if version == 0 {
		version = 1
	}
	if version != 1 {
		return dto.OpsRelayDataPlanePolicyResponse{}, service.ErrInvalidArgument
	}
	if deps.ControlChannel == nil {
		return dto.OpsRelayDataPlanePolicyResponse{}, service.ErrNotImplemented
	}
	targets := normalizeOpsRelayPolicyTargets(req.TargetDeviceIDs)
	scope := normalizeOpsRelayPolicyScope(req.Scope, len(targets) > 0)
	policyID := strings.TrimSpace(req.PolicyID)
	if policyID == "" {
		policyID = util.NewID("relay-policy")
	}
	ttlMS := req.TTLMS
	if ttlMS == 0 {
		ttlMS = uint64(time.Hour / time.Millisecond)
	}
	nowMS := uint64(time.Now().UnixMilli())
	pathType := netpath.NormalizePolicyPathType(req.PathType)
	policy := controlmsg.RelayDataPlanePolicy{
		PolicyID:                 policyID,
		Version:                  version,
		Scope:                    scope,
		NetworkID:                networkID,
		TargetDeviceIDs:          sortedOpsRelayPolicyTargets(targets),
		PathType:                 pathType,
		PreferredPathTypes:       preferredPathTypes,
		ProbeIntervalMS:          req.ProbeIntervalMS,
		FailoverAfterMS:          req.FailoverAfterMS,
		UpgradeSuccesses:         req.UpgradeSuccesses,
		FailedPathCooldownProbes: req.FailedPathCooldownProbes,
		RecommendationLevel:      req.RecommendationLevel,
		ExecutionLevel:           req.ExecutionLevel,
		RelayMtu:                 req.RelayMtu,
		MaxFramePayload:          req.MaxFramePayload,
		Reason:                   strings.TrimSpace(req.Reason),
		TTLMS:                    ttlMS,
		EffectiveMS:              req.EffectiveMS,
		UpdatedAtMS:              nowMS,
	}
	sessions, err := deps.ControlChannel.ActiveSessions(networkID, "")
	if err != nil {
		return dto.OpsRelayDataPlanePolicyResponse{}, err
	}
	published := 0
	skipped := 0
	matchedTargets := make(map[string]bool, len(targets))
	for _, session := range sessions {
		deviceID := strings.TrimSpace(session.DeviceID)
		if deviceID == "" {
			skipped++
			continue
		}
		if len(targets) > 0 && !targets[deviceID] {
			continue
		}
		if len(targets) > 0 {
			matchedTargets[deviceID] = true
		}
		if err := publishControlMQTTEnvelope(deps, deviceID, "relay_data_plane_policy", "", policy); err != nil {
			skipped++
			continue
		}
		published++
	}
	if len(targets) > 0 {
		for target := range targets {
			if !matchedTargets[target] {
				skipped++
			}
		}
	}
	return dto.OpsRelayDataPlanePolicyResponse{
		PolicyID:                 policyID,
		NetworkID:                networkID,
		Scope:                    scope,
		TargetDeviceIDs:          sortedOpsRelayPolicyTargets(targets),
		PathType:                 pathType,
		PreferredPathTypes:       preferredPathTypes,
		ProbeIntervalMS:          req.ProbeIntervalMS,
		FailoverAfterMS:          req.FailoverAfterMS,
		UpgradeSuccesses:         req.UpgradeSuccesses,
		FailedPathCooldownProbes: req.FailedPathCooldownProbes,
		Published:                published,
		Skipped:                  skipped,
		RelayMtu:                 req.RelayMtu,
		MaxFramePayload:          req.MaxFramePayload,
		Reason:                   strings.TrimSpace(req.Reason),
	}, nil
}

func normalizeOpsRelayPolicyTargets(values []string) map[string]bool {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]bool, len(values))
	for _, value := range values {
		deviceID := strings.TrimSpace(value)
		if deviceID != "" {
			out[deviceID] = true
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeOpsRelayPolicyScope(scope string, hasTargets bool) string {
	value := strings.TrimSpace(scope)
	if hasTargets {
		if value == "device" || value == "device_override" {
			return value
		}
		return "device_override"
	}
	switch value {
	case "global", "region", "network":
		return value
	}
	return "network"
}

func sortedOpsRelayPolicyTargets(values map[string]bool) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
