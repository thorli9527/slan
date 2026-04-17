package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
)

func (s dbNetworkService) Create(userID string, req dto.CreateNetworkRequest) (dto.Network, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return dto.Network{}, fmt.Errorf("%w: name is required", ErrInvalidArgument)
	}
	networkID := newID("net")
	subnetID := newID("subnet")
	defaultSubnet, err := newSubnet(networkID, subnetID, "default", req.CIDR, "", "", "", true)
	if err != nil {
		return dto.Network{}, err
	}
	network := dto.Network{
		NetworkID:         networkID,
		Name:              name,
		Description:       strings.TrimSpace(req.Description),
		DefaultSubnetID:   defaultSubnet.SubnetID,
		DefaultSubnetCIDR: defaultSubnet.CIDR,
	}
	if err := s.state.pg.CreateNetworkWithDefaultSubnet(context.Background(), userID, network, defaultSubnet); err != nil {
		return dto.Network{}, err
	}
	return network, nil
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
	subnets, err := s.state.pg.ListSubnetsByNetwork(ctx, networkID)
	if err != nil {
		return dto.NetworkDetail{}, err
	}
	members, err := s.state.pg.ListMembersByNetwork(ctx, networkID)
	if err != nil {
		return dto.NetworkDetail{}, err
	}
	return dto.NetworkDetail{Network: record.ToDTO(), Subnets: subnets, Members: members}, nil
}

func (s dbNetworkService) Join(userID, networkID string, req dto.JoinNetworkRequest) (dto.NetworkJoinResult, error) {
	if strings.TrimSpace(req.DeviceID) == "" {
		return dto.NetworkJoinResult{}, fmt.Errorf("%w: deviceId is required", ErrInvalidArgument)
	}
	ctx := context.Background()
	record, err := s.state.pg.GetNetworkByID(ctx, networkID)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.NetworkJoinResult{}, ErrNotFound
		}
		return dto.NetworkJoinResult{}, err
	}
	if err := s.state.ensureDeviceOwner(ctx, userID, req.DeviceID); err != nil {
		return dto.NetworkJoinResult{}, err
	}
	if err := s.state.ensureNetworkAccess(ctx, userID, networkID); err != nil && record.OwnerUserID != userID {
		return dto.NetworkJoinResult{}, err
	}

	member, err := s.state.ensureMember(ctx, networkID, req.DeviceID, "member")
	if err != nil {
		return dto.NetworkJoinResult{}, err
	}
	attachment, err := s.state.ensureAttachment(ctx, networkID, record.DefaultSubnetID, req.DeviceID)
	if err != nil {
		return dto.NetworkJoinResult{}, err
	}
	return dto.NetworkJoinResult{Member: member, Attachment: attachment}, nil
}

func (s dbNetworkService) ListMembers(userID, networkID string) ([]dto.NetworkMember, error) {
	ctx := context.Background()
	if err := s.state.ensureNetworkAccess(ctx, userID, networkID); err != nil {
		return nil, err
	}
	return s.state.pg.ListMembersByNetwork(ctx, networkID)
}

func (s dbNetworkService) CreateSubnet(userID, networkID string, req dto.CreateSubnetRequest) (dto.Subnet, error) {
	if strings.TrimSpace(req.Name) == "" {
		return dto.Subnet{}, fmt.Errorf("%w: name is required", ErrInvalidArgument)
	}
	ctx := context.Background()
	record, err := s.state.pg.GetNetworkByID(ctx, networkID)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.Subnet{}, ErrNotFound
		}
		return dto.Subnet{}, err
	}
	if record.OwnerUserID != userID {
		return dto.Subnet{}, ErrForbidden
	}
	if _, err := s.state.pg.FindSubnetByName(ctx, networkID, req.Name); err == nil {
		return dto.Subnet{}, fmt.Errorf("%w: subnet name already exists", ErrConflict)
	} else if !repo.IsNotFound(err) {
		return dto.Subnet{}, err
	}
	subnet, err := newSubnet(networkID, newID("subnet"), req.Name, req.CIDR, req.GatewayIP, req.AllocationStartIP, req.AllocationEndIP, false)
	if err != nil {
		return dto.Subnet{}, err
	}
	if err := s.state.pg.CreateSubnet(ctx, subnet); err != nil {
		return dto.Subnet{}, err
	}
	return subnet, nil
}

func (s dbNetworkService) ListSubnets(userID, networkID string) ([]dto.Subnet, error) {
	ctx := context.Background()
	if err := s.state.ensureNetworkAccess(ctx, userID, networkID); err != nil {
		return nil, err
	}
	return s.state.pg.ListSubnetsByNetwork(ctx, networkID)
}

func (s dbNetworkService) AttachDevice(userID, networkID, subnetID string, req dto.AttachDeviceRequest) (dto.SubnetAttachment, error) {
	if strings.TrimSpace(req.DeviceID) == "" {
		return dto.SubnetAttachment{}, fmt.Errorf("%w: deviceId is required", ErrInvalidArgument)
	}
	ctx := context.Background()
	if err := s.state.ensureNetworkAccess(ctx, userID, networkID); err != nil {
		return dto.SubnetAttachment{}, err
	}
	if err := s.state.ensureDeviceOwner(ctx, userID, req.DeviceID); err != nil {
		return dto.SubnetAttachment{}, err
	}
	subnet, err := s.state.pg.GetSubnetByID(ctx, subnetID)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.SubnetAttachment{}, ErrNotFound
		}
		return dto.SubnetAttachment{}, err
	}
	if subnet.NetworkID != networkID {
		return dto.SubnetAttachment{}, ErrNotFound
	}
	if _, err := s.state.pg.GetMemberByNetworkDevice(ctx, networkID, req.DeviceID); err != nil {
		if repo.IsNotFound(err) {
			return dto.SubnetAttachment{}, fmt.Errorf("%w: device must join network first", ErrForbidden)
		}
		return dto.SubnetAttachment{}, err
	}
	return s.state.ensureAttachment(ctx, networkID, subnetID, req.DeviceID)
}
