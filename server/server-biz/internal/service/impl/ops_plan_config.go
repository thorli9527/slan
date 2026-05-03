package impl

import (
	"context"
	"strings"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
)

func (s dbOpsService) PlanConfig() (dto.PlanConfig, error) {
	return s.state.globalPlanConfig(context.Background()), nil
}

func (s dbOpsService) UpdatePlanConfig(req dto.UpdatePlanConfigRequest) (dto.PlanConfig, error) {
	ctx := context.Background()
	record := repoPlanConfigFromDTO(globalPlanConfigID, dto.PlanConfig(req))
	if err := s.state.pg.UpsertPlanConfig(ctx, record); err != nil {
		return dto.PlanConfig{}, err
	}
	return s.state.globalPlanConfig(ctx), nil
}

func (s dbOpsService) UpdateUserPlanOverride(userID string, req dto.UpdatePlanConfigRequest) (dto.UserPlanOverride, error) {
	ctx := context.Background()
	userID = strings.TrimSpace(userID)
	if _, err := s.state.pg.GetUserByID(ctx, userID); err != nil {
		if repo.IsNotFound(err) {
			return dto.UserPlanOverride{}, ErrNotFound
		}
		return dto.UserPlanOverride{}, err
	}
	record := repoUserPlanOverrideFromDTO(userID, dto.PlanConfig(req))
	if err := s.state.pg.UpsertUserPlanOverride(ctx, record); err != nil {
		return dto.UserPlanOverride{}, err
	}
	return dto.UserPlanOverride{
		UserID: userID,
		PlanConfig: dto.PlanConfig{
			MaxActiveDevices: record.MaxActiveDevices,
			RelayIngressKbps: record.RelayIngressKbps,
			RelayEgressKbps:  record.RelayEgressKbps,
			UDPIngressKbps:   record.UDPIngressKbps,
			UDPEgressKbps:    record.UDPEgressKbps,
		},
	}, nil
}

func (s dbOpsService) DeleteUserPlanOverride(userID string) error {
	return s.state.pg.DeleteUserPlanOverride(context.Background(), strings.TrimSpace(userID))
}
