package impl

import (
	"context"
	"strings"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
)

const (
	fixedMaxActiveDevices          = 2
	defaultRelayBandwidthLimitKbps = 512
)

func (s dbNetworkService) PlanStatus(userID string) (dto.PlanStatus, error) {
	ctx := context.Background()
	if _, err := s.state.pg.GetUserByID(ctx, strings.TrimSpace(userID)); err != nil {
		if repo.IsNotFound(err) {
			return dto.PlanStatus{}, ErrNotFound
		}
		return dto.PlanStatus{}, err
	}
	return fixedPlanStatus(), nil
}

func fixedDeviceLimit() int {
	return fixedMaxActiveDevices
}

func fixedPlanStatus() dto.PlanStatus {
	return dto.PlanStatus{
		PlanName:                "free",
		FreeDeviceLimit:         fixedMaxActiveDevices,
		MaxActiveDevices:        fixedMaxActiveDevices,
		RelayBandwidthLimitKbps: defaultRelayBandwidthLimitKbps,
		P2PUnlimited:            true,
		DNSAvailable:            true,
	}
}
