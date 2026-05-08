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
	sourceDeviceID := strings.TrimSpace(req.SourceDeviceID)
	peerDeviceID := strings.TrimSpace(req.PeerDeviceID)
	targets, targetErr := resolveOpsRelayPolicyTargets(deps, networkID, req.TargetDeviceIDs, sourceDeviceID, peerDeviceID)
	if targetErr != nil {
		return dto.OpsRelayDataPlanePolicyResponse{}, targetErr
	}
	scope := normalizeOpsRelayPolicyScope(req.Scope, sourceDeviceID, peerDeviceID, len(targets.explicit) > 0)
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
		TargetDeviceIDs:          targets.values,
		SourceDeviceID:           sourceDeviceID,
		PeerDeviceID:             peerDeviceID,
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
	published := 0
	skipped := 0
	publishedTargets := make([]string, 0, len(targets.values))
	for _, deviceID := range targets.values {
		if err := publishControlMQTTEnvelope(deps, deviceID, "relay_data_plane_policy", "", policy); err != nil {
			skipped++
			continue
		}
		publishedTargets = append(publishedTargets, deviceID)
		published++
	}
	return dto.OpsRelayDataPlanePolicyResponse{
		PolicyID:                 policyID,
		TemplateID:               strings.TrimSpace(req.TemplateID),
		TemplateName:             strings.TrimSpace(req.TemplateName),
		NetworkID:                networkID,
		Scope:                    scope,
		TargetDeviceIDs:          sortedPublishedOpsRelayPolicyTargets(publishedTargets),
		SourceDeviceID:           sourceDeviceID,
		PeerDeviceID:             peerDeviceID,
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

type opsRelayPolicyTargets struct {
	values   []string
	explicit map[string]bool
}

func resolveOpsRelayPolicyTargets(deps routerDeps, networkID string, requestTargets []string, sourceDeviceID, peerDeviceID string) (opsRelayPolicyTargets, error) {
	explicit := normalizeOpsRelayPolicyTargets(requestTargets)
	if explicit == nil {
		explicit = make(map[string]bool, 2)
	}
	addOpsRelayPolicyTarget(explicit, sourceDeviceID)
	addOpsRelayPolicyTarget(explicit, peerDeviceID)
	if len(explicit) > 0 {
		return opsRelayPolicyTargets{values: sortedOpsRelayPolicyTargets(explicit), explicit: explicit}, nil
	}
	if deps.Network != nil {
		deviceIDs, err := deps.Network.ListEnabledNetworkDeviceIDs(networkID)
		if err != nil {
			return opsRelayPolicyTargets{}, err
		}
		if len(deviceIDs) > 0 {
			enabledTargets := normalizeOpsRelayPolicyTargets(deviceIDs)
			return opsRelayPolicyTargets{values: sortedOpsRelayPolicyTargets(enabledTargets)}, nil
		}
	}
	sessions, err := deps.ControlChannel.ActiveSessions(networkID, "")
	if err != nil {
		return opsRelayPolicyTargets{}, err
	}
	sessionTargets := make(map[string]bool, len(sessions))
	for _, session := range sessions {
		addOpsRelayPolicyTarget(sessionTargets, session.DeviceID)
	}
	return opsRelayPolicyTargets{values: sortedOpsRelayPolicyTargets(sessionTargets)}, nil
}

func sortedPublishedOpsRelayPolicyTargets(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		deviceID := strings.TrimSpace(value)
		if deviceID == "" || seen[deviceID] {
			continue
		}
		seen[deviceID] = true
		out = append(out, deviceID)
	}
	sort.Strings(out)
	return out
}

func normalizeOpsRelayPolicyTargets(values []string) map[string]bool {
	out := make(map[string]bool, len(values))
	for _, value := range values {
		addOpsRelayPolicyTarget(out, value)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func addOpsRelayPolicyTarget(targets map[string]bool, deviceID string) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return
	}
	targets[deviceID] = true
}

func normalizeOpsRelayPolicyScope(scope, sourceDeviceID, peerDeviceID string, hasExplicitTargets bool) string {
	value := strings.TrimSpace(scope)
	if strings.TrimSpace(sourceDeviceID) != "" && strings.TrimSpace(peerDeviceID) != "" {
		return "peer_pair"
	}
	if hasExplicitTargets {
		if value == "device" || value == "device_override" {
			return value
		}
		return "device_override"
	}
	switch value {
	case "global", "region", "network", "peer_pair":
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
