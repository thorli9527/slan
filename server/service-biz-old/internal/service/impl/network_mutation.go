package impl

import (
	"context"
	"fmt"
	"strings"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
	"github.com/slan/server/server-biz/internal/util"
)

func (s dbNetworkService) Create(userID string, req dto.CreateNetworkRequest) (dto.Network, error) {
	req = normalizeCreateNetworkDefaults(req)
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return dto.Network{}, fmt.Errorf("%w: name is required", ErrInvalidArgument)
	}
	ctx := context.Background()
	bindDeviceID := strings.TrimSpace(req.BindDeviceID)
	if owned, err := s.state.pg.GetOwnedNetworkByUser(ctx, userID); err == nil {
		return dto.Network{}, fmt.Errorf("%w: user already owns network %s", ErrConflict, owned.NetworkID)
	} else if !repo.IsNotFound(err) {
		return dto.Network{}, err
	}
	if bindDeviceID != "" {
		if err := s.state.ensureDeviceOwner(ctx, userID, bindDeviceID); err != nil {
			return dto.Network{}, err
		}
	}
	user, err := s.state.pg.GetUserByID(ctx, userID)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.Network{}, ErrUnauthorized
		}
		return dto.Network{}, err
	}
	networkID := util.NewID("net")
	subnets, err := defaultSubnetsForNetwork(networkID, createNetworkCIDR(req.CIDR), func() string {
		return util.NewID("subnet")
	})
	if err != nil {
		return dto.Network{}, err
	}
	if hasCreateNetworkDHCPOptions(req) {
		subnets[0], err = newSubnet(
			networkID,
			subnets[0].SubnetID,
			subnets[0].Name,
			subnets[0].CIDR,
			"",
			strings.TrimSpace(req.AllocationStartIP),
			strings.TrimSpace(req.AllocationEndIP),
			true,
		)
		if err != nil {
			return dto.Network{}, err
		}
		subnets[0].Remark = "Default network for shared access and DHCP allocation"
	}
	defaultSubnet := subnets[0]
	network := dto.Network{
		NetworkID:         networkID,
		Name:              name,
		Description:       strings.TrimSpace(req.Description),
		DefaultSubnetID:   defaultSubnet.SubnetID,
		DefaultSubnetCIDR: defaultSubnet.CIDR,
	}
	if err := s.state.pg.CreateNetworkWithSubnets(ctx, userID, network, subnets); err != nil {
		return dto.Network{}, err
	}
	if strings.TrimSpace(user.ActiveNetworkID) == "" {
		if err := s.state.pg.UpdateUserActiveNetwork(ctx, userID, networkID); err != nil {
			return dto.Network{}, err
		}
		s.state.publishActiveNetworkEnabled(userID, networkID, "first network created")
	}
	if bindDeviceID != "" {
		if _, err := s.state.ensureMember(ctx, networkID, bindDeviceID, userID, "owner"); err != nil {
			return dto.Network{}, err
		}
	}
	return network, nil
}

func (s dbNetworkService) Update(userID, networkID string, req dto.UpdateNetworkRequest) (dto.Network, error) {
	ctx := context.Background()
	record, err := s.state.pg.GetNetworkByID(ctx, networkID)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.Network{}, ErrNotFound
		}
		return dto.Network{}, err
	}
	if record.OwnerUserID != userID {
		return dto.Network{}, ErrForbidden
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = record.Name
	}
	description := strings.TrimSpace(req.Description)
	if description == "" {
		description = record.Description
	}
	cidr := strings.TrimSpace(req.CIDR)
	if cidr == "" {
		return dto.Network{}, fmt.Errorf("%w: cidr is required", ErrInvalidArgument)
	}

	subnet, err := newSubnet(
		record.NetworkID,
		record.DefaultSubnetID,
		"default",
		cidr,
		"",
		strings.TrimSpace(req.AllocationStartIP),
		strings.TrimSpace(req.AllocationEndIP),
		true,
	)
	if err != nil {
		return dto.Network{}, err
	}
	changes, err := s.state.reassignDefaultSubnetLeasePool(ctx, record.NetworkID, subnet)
	if err != nil {
		return dto.Network{}, err
	}
	if err := s.state.pg.UpdateNetworkMetadata(ctx, record.NetworkID, name, description, subnet.CIDR); err != nil {
		return dto.Network{}, err
	}
	for _, change := range changes {
		s.state.publishDeviceIPReassigned(record.NetworkID, change.deviceID, change.attachmentID, change.virtualIP)
	}
	s.state.publishNetworkRestartRequired(record.NetworkID, subnet.CIDR)
	return dto.Network{
		NetworkID:         record.NetworkID,
		Name:              name,
		Description:       description,
		DefaultSubnetID:   record.DefaultSubnetID,
		DefaultSubnetCIDR: subnet.CIDR,
	}, nil
}

func (s dbNetworkService) UpdateDNS(userID, networkID string, req dto.UpdateNetworkDNSRequest) (dto.NetworkDetail, error) {
	ctx := context.Background()
	record, err := s.state.pg.GetNetworkByID(ctx, networkID)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.NetworkDetail{}, ErrNotFound
		}
		return dto.NetworkDetail{}, err
	}
	if record.OwnerUserID != userID {
		return dto.NetworkDetail{}, ErrForbidden
	}
	dns := dto.DNSConfig{
		Servers:       sanitizeValues(req.Servers),
		SearchDomains: sanitizeValues(req.SearchDomains),
	}
	wildcards, err := sanitizeDNSWildcards(req.Wildcards)
	if err != nil {
		return dto.NetworkDetail{}, err
	}
	dns.Wildcards = wildcards
	if err := s.state.pg.UpdateNetworkDNS(ctx, networkID, dns); err != nil {
		return dto.NetworkDetail{}, err
	}
	return s.Get(userID, networkID)
}

func (s dbNetworkService) UpdateJoinKey(userID, networkID string, req dto.UpdateNetworkJoinKeyRequest) (dto.NetworkDetail, error) {
	ctx := context.Background()
	record, err := s.state.pg.GetNetworkByID(ctx, networkID)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.NetworkDetail{}, ErrNotFound
		}
		return dto.NetworkDetail{}, err
	}
	if record.OwnerUserID != userID {
		return dto.NetworkDetail{}, ErrForbidden
	}
	joinKey := strings.TrimSpace(req.JoinKey)
	if joinKey == "" {
		joinKey = util.RandomHex(16)
	}
	if len(joinKey) != 32 {
		return dto.NetworkDetail{}, fmt.Errorf("%w: joinKey must be 32 characters", ErrInvalidArgument)
	}
	if err := s.state.pg.UpdateNetworkJoinKey(ctx, networkID, joinKey); err != nil {
		return dto.NetworkDetail{}, err
	}
	return s.Get(userID, networkID)
}
