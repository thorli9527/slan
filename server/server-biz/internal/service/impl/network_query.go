package impl

import (
	"context"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
)

func (s dbNetworkService) Home(userID string) (dto.NetworkHome, error) {
	ctx := context.Background()
	var home dto.NetworkHome

	if owned, err := s.state.pg.GetOwnedNetworkByUser(ctx, userID); err == nil {
		record := owned.ToDTO()
		home.OwnedNetwork = &record
		home.HasNetwork = true
	} else if !repo.IsNotFound(err) {
		return dto.NetworkHome{}, err
	}

	items, err := s.List(userID)
	if err != nil {
		return dto.NetworkHome{}, err
	}
	if len(items) > 0 {
		record := items[0]
		home.ActiveNetwork = &record
		home.HasNetwork = true
	}
	return home, nil
}

func (s dbNetworkService) List(userID string) ([]dto.Network, error) {
	records, err := s.state.pg.ListVisibleNetworksByUser(context.Background(), userID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.Network, 0, len(records))
	for _, record := range records {
		out = append(out, record.ToDTO())
	}
	return out, nil
}

func (s dbNetworkService) Get(userID, networkID string) (dto.NetworkDetail, error) {
	ctx := context.Background()
	record, err := s.state.pg.GetNetworkByID(ctx, networkID)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.NetworkDetail{}, ErrNotFound
		}
		return dto.NetworkDetail{}, err
	}
	if err := s.state.ensureNetworkAccess(ctx, userID, networkID); err != nil {
		return dto.NetworkDetail{}, err
	}
	members, err := s.state.pg.ListMembersByNetwork(ctx, networkID)
	if err != nil {
		return dto.NetworkDetail{}, err
	}
	return dto.NetworkDetail{
		Network:            record.ToDTO(),
		OwnedByCurrentUser: record.OwnerUserID == userID,
		DNS:                record.DNSConfig(),
		JoinKey:            visibleJoinKey(record, userID),
		Subnets:            nil,
		Members:            members,
	}, nil
}

func (s dbNetworkService) ListMembers(userID, networkID string) ([]dto.NetworkMember, error) {
	ctx := context.Background()
	if err := s.state.ensureNetworkAccess(ctx, userID, networkID); err != nil {
		return nil, err
	}
	return s.state.pg.ListMembersByNetwork(ctx, networkID)
}

func (s dbNetworkService) ListSubnets(userID, networkID string) ([]dto.Subnet, error) {
	ctx := context.Background()
	if err := s.state.ensureNetworkAccess(ctx, userID, networkID); err != nil {
		return nil, err
	}
	return s.state.pg.ListSubnetsByNetwork(ctx, networkID)
}
