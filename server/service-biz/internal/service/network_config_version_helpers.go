package service

import (
	"context"
	"strings"
	"time"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

func bumpNetworkConfigVersion(
	ctx context.Context,
	networks repository.NetworkRepository,
	eventPublisher NetworkEventPublisher,
	nowFn func() time.Time,
	networkID string,
	reason string,
) (model.NetworkConfigVersion, error) {
	networkID = strings.TrimSpace(networkID)
	reason = strings.TrimSpace(reason)
	if networkID == "" {
		return model.NetworkConfigVersion{}, ErrInvalidArgument
	}
	now := currentTime(nowFn)
	current, ok, err := networks.GetNetworkVersion(ctx, networkID)
	if err != nil {
		return model.NetworkConfigVersion{}, err
	}
	next := model.NetworkConfigVersion{
		NetworkID: networkID,
		Version:   1,
		Reason:    reason,
		CreatedAt: now.Unix(),
		UpdatedAt: now.Unix(),
	}
	if ok {
		next = current
		if next.CreatedAt <= 0 {
			next.CreatedAt = now.Unix()
		}
		next.Version += 1
		next.Reason = reason
		next.UpdatedAt = now.Unix()
	}
	if err := networks.SaveNetworkVersion(ctx, next); err != nil {
		return model.NetworkConfigVersion{}, err
	}
	if eventPublisher != nil {
		if err := publishNetworkEvent(
			ctx,
			eventPublisher,
			NetworkEventConfigChanged,
			networkID,
			uint64(next.Version),
			now.UnixMilli(),
			NetworkEventConfigChangedPayload{
				Network: NetworkEventNetworkView{
					NetworkID: networkID,
					UpdatedAt: now.Unix(),
				},
			},
		); err != nil {
			return model.NetworkConfigVersion{}, err
		}
	}
	return next, nil
}
