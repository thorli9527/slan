package impl

import (
	"context"
	"strings"
	"time"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
)

const (
	defaultMaxActiveDevices = 5
	defaultRelayIngressKbps = 512
	defaultRelayEgressKbps  = 512
	defaultUDPIngressKbps   = 0
	defaultUDPEgressKbps    = 0
	globalPlanConfigID      = "global"
)

func (s dbNetworkService) PlanStatus(userID string) (dto.PlanStatus, error) {
	ctx := context.Background()
	if _, err := s.state.pg.GetUserByID(ctx, strings.TrimSpace(userID)); err != nil {
		if repo.IsNotFound(err) {
			return dto.PlanStatus{}, ErrNotFound
		}
		return dto.PlanStatus{}, err
	}
	return s.state.planStatusForUser(ctx, userID), nil
}

func fixedDeviceLimit() int {
	return defaultMaxActiveDevices
}

func defaultPlanConfig() dto.PlanConfig {
	return dto.PlanConfig{
		MaxActiveDevices: defaultMaxActiveDevices,
		RelayIngressKbps: defaultRelayIngressKbps,
		RelayEgressKbps:  defaultRelayEgressKbps,
		UDPIngressKbps:   defaultUDPIngressKbps,
		UDPEgressKbps:    defaultUDPEgressKbps,
	}
}

func sanitizePlanConfig(input dto.PlanConfig) dto.PlanConfig {
	if input.MaxActiveDevices <= 0 {
		input.MaxActiveDevices = defaultMaxActiveDevices
	}
	if input.RelayIngressKbps < 0 {
		input.RelayIngressKbps = 0
	}
	if input.RelayEgressKbps < 0 {
		input.RelayEgressKbps = 0
	}
	if input.UDPIngressKbps < 0 {
		input.UDPIngressKbps = 0
	}
	if input.UDPEgressKbps < 0 {
		input.UDPEgressKbps = 0
	}
	return input
}

func (s *dbState) globalPlanConfig(ctx context.Context) dto.PlanConfig {
	record, err := s.pg.GetPlanConfig(ctx, globalPlanConfigID)
	if err != nil {
		return defaultPlanConfig()
	}
	return sanitizePlanConfig(dto.PlanConfig{
		MaxActiveDevices: record.MaxActiveDevices,
		RelayIngressKbps: record.RelayIngressKbps,
		RelayEgressKbps:  record.RelayEgressKbps,
		UDPIngressKbps:   record.UDPIngressKbps,
		UDPEgressKbps:    record.UDPEgressKbps,
	})
}

func (s *dbState) effectivePlanConfig(ctx context.Context, userID string) dto.PlanConfig {
	plan := s.globalPlanConfig(ctx)
	override, err := s.pg.GetUserPlanOverride(ctx, strings.TrimSpace(userID))
	if err != nil {
		return plan
	}
	if override.MaxActiveDevices > 0 {
		plan.MaxActiveDevices = override.MaxActiveDevices
	}
	if override.RelayIngressKbps > 0 {
		plan.RelayIngressKbps = override.RelayIngressKbps
	}
	if override.RelayEgressKbps > 0 {
		plan.RelayEgressKbps = override.RelayEgressKbps
	}
	if override.UDPIngressKbps > 0 {
		plan.UDPIngressKbps = override.UDPIngressKbps
	}
	if override.UDPEgressKbps > 0 {
		plan.UDPEgressKbps = override.UDPEgressKbps
	}
	return sanitizePlanConfig(plan)
}

func (s *dbState) planStatusForUser(ctx context.Context, userID string) dto.PlanStatus {
	plan := s.effectivePlanConfig(ctx, userID)
	return planStatusFromConfig(plan)
}

func planStatusFromConfig(plan dto.PlanConfig) dto.PlanStatus {
	relayLimit := plan.RelayIngressKbps
	if plan.RelayEgressKbps > relayLimit {
		relayLimit = plan.RelayEgressKbps
	}
	return dto.PlanStatus{
		PlanName:                "free",
		FreeDeviceLimit:         plan.MaxActiveDevices,
		MaxActiveDevices:        plan.MaxActiveDevices,
		RelayBandwidthLimitKbps: relayLimit,
		RelayIngressKbps:        plan.RelayIngressKbps,
		RelayEgressKbps:         plan.RelayEgressKbps,
		UDPIngressKbps:          plan.UDPIngressKbps,
		UDPEgressKbps:           plan.UDPEgressKbps,
		P2PUnlimited:            plan.UDPIngressKbps <= 0 && plan.UDPEgressKbps <= 0,
		DNSAvailable:            true,
	}
}

func repoPlanConfigFromDTO(configID string, input dto.PlanConfig) repo.PlanConfig {
	plan := sanitizePlanConfig(input)
	return repo.PlanConfig{
		ConfigID:         configID,
		MaxActiveDevices: plan.MaxActiveDevices,
		RelayIngressKbps: plan.RelayIngressKbps,
		RelayEgressKbps:  plan.RelayEgressKbps,
		UDPIngressKbps:   plan.UDPIngressKbps,
		UDPEgressKbps:    plan.UDPEgressKbps,
		UpdatedAt:        time.Now().Unix(),
	}
}

func repoUserPlanOverrideFromDTO(userID string, input dto.PlanConfig) repo.UserPlanOverride {
	plan := sanitizePlanConfig(input)
	return repo.UserPlanOverride{
		UserID:           strings.TrimSpace(userID),
		MaxActiveDevices: plan.MaxActiveDevices,
		RelayIngressKbps: plan.RelayIngressKbps,
		RelayEgressKbps:  plan.RelayEgressKbps,
		UDPIngressKbps:   plan.UDPIngressKbps,
		UDPEgressKbps:    plan.UDPEgressKbps,
		UpdatedAt:        time.Now().Unix(),
	}
}
