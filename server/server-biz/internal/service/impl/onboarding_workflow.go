package impl

import (
	"context"
	"strings"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
)

const (
	defaultOwnedNetworkName = "My Network"
	defaultOwnedNetworkCIDR = "10.0.0.0/16"
)

func (s *dbState) ensureOwnedNetwork(ctx context.Context, userID string) (dto.Network, error) {
	if existing, err := s.pg.GetOwnedNetworkByUser(ctx, userID); err == nil {
		return existing.ToDTO(), nil
	} else if !repo.IsNotFound(err) {
		return dto.Network{}, err
	}
	return dbNetworkService{state: s}.Create(userID, dto.CreateNetworkRequest{
		Name: defaultOwnedNetworkName,
		CIDR: defaultOwnedNetworkCIDR,
	})
}

func (s *dbState) ensureDeviceProvisionedInActiveNetwork(ctx context.Context, userID, deviceID string) error {
	if strings.TrimSpace(deviceID) == "" {
		return nil
	}
	user, err := s.pg.GetUserByID(ctx, userID)
	if err != nil {
		if repo.IsNotFound(err) {
			return ErrUnauthorized
		}
		return err
	}
	networkID := strings.TrimSpace(user.ActiveNetworkID)
	if networkID == "" {
		owned, err := s.ensureOwnedNetwork(ctx, userID)
		if err != nil {
			return err
		}
		networkID = owned.NetworkID
	}
	record, err := s.pg.GetNetworkByID(ctx, networkID)
	if err != nil {
		if repo.IsNotFound(err) {
			return ErrNotFound
		}
		return err
	}
	role, err := s.membershipRole(ctx, record.NetworkID, deviceID, record.OwnerUserID, userID)
	if err != nil {
		return err
	}
	if _, err := s.ensureMember(ctx, record.NetworkID, deviceID, role); err != nil {
		return err
	}
	attachment, err := s.ensureAttachment(ctx, record.NetworkID, record.DefaultSubnetID, deviceID)
	if err != nil {
		return err
	}
	s.publishActiveNetworkEnabled(userID, record.NetworkID, "device registered into active network")
	s.publishDeviceIPReassigned(record.NetworkID, deviceID, attachment.AttachmentID, attachment.VirtualIP)
	return nil
}
