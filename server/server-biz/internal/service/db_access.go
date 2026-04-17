package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
)

func (s *dbState) issueAuthResponse(ctx context.Context, userID string) (dto.AuthResponse, error) {
	accessToken := opaqueToken("access", userID)
	refreshToken := opaqueToken("refresh", userID)
	if err := s.tokens.StoreAccessToken(ctx, accessToken, userID, time.Hour); err != nil {
		return dto.AuthResponse{}, err
	}
	if err := s.tokens.StoreRefreshToken(ctx, refreshToken, userID, 24*time.Hour); err != nil {
		return dto.AuthResponse{}, err
	}
	return dto.AuthResponse{
		UserID:       userID,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    3600,
	}, nil
}

func (s *dbState) ensureDeviceOwner(ctx context.Context, userID, deviceID string) error {
	record, err := s.pg.GetDeviceByID(ctx, deviceID)
	if err != nil {
		if repo.IsNotFound(err) {
			return ErrNotFound
		}
		return err
	}
	if record.UserID != userID {
		return ErrForbidden
	}
	return nil
}

func (s *dbState) ensureNetworkAccess(ctx context.Context, userID, networkID string) error {
	record, err := s.pg.GetNetworkByID(ctx, networkID)
	if err != nil {
		if repo.IsNotFound(err) {
			return ErrNotFound
		}
		return err
	}
	if record.OwnerUserID == userID {
		return nil
	}
	devices, err := s.pg.ListDevicesByUser(ctx, userID)
	if err != nil {
		return err
	}
	for _, device := range devices {
		if _, err := s.pg.GetMemberByNetworkDevice(ctx, networkID, device.DeviceID); err == nil {
			return nil
		} else if !repo.IsNotFound(err) {
			return err
		}
	}
	return ErrForbidden
}

func (s *dbState) ensureMember(ctx context.Context, networkID, deviceID, role string) (dto.NetworkMember, error) {
	member, err := s.pg.GetMemberByNetworkDevice(ctx, networkID, deviceID)
	if err == nil {
		return member, nil
	}
	if !repo.IsNotFound(err) {
		return dto.NetworkMember{}, err
	}
	member = dto.NetworkMember{
		MemberID:  newID("member"),
		NetworkID: networkID,
		DeviceID:  deviceID,
		Role:      role,
		Status:    "active",
	}
	return member, s.pg.CreateMember(ctx, member)
}

func (s *dbState) ensureAttachment(ctx context.Context, networkID, subnetID, deviceID string) (dto.SubnetAttachment, error) {
	attachment, err := s.pg.GetAttachmentBySubnetDevice(ctx, subnetID, deviceID)
	if err == nil {
		return attachment, nil
	}
	if !repo.IsNotFound(err) {
		return dto.SubnetAttachment{}, err
	}
	subnet, err := s.pg.GetSubnetByID(ctx, subnetID)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.SubnetAttachment{}, ErrNotFound
		}
		return dto.SubnetAttachment{}, err
	}
	virtualIP, err := s.allocateIP(ctx, subnet)
	if err != nil {
		return dto.SubnetAttachment{}, err
	}
	attachment = dto.SubnetAttachment{
		AttachmentID: newID("att"),
		NetworkID:    networkID,
		SubnetID:     subnetID,
		DeviceID:     deviceID,
		VirtualIP:    virtualIP,
		Status:       "active",
	}
	return attachment, s.pg.CreateAttachment(ctx, attachment)
}

func (s *dbState) allocateIP(ctx context.Context, subnet dto.Subnet) (string, error) {
	attachments, err := s.pg.ListAttachmentsBySubnet(ctx, subnet.SubnetID)
	if err != nil {
		return "", err
	}
	prefix, start, end, err := subnetRange(subnet.CIDR, subnet.GatewayIP, subnet.AllocationStartIP, subnet.AllocationEndIP)
	if err != nil {
		return "", err
	}
	used := make(map[string]struct{}, len(attachments))
	for _, attachment := range attachments {
		used[attachment.VirtualIP] = struct{}{}
	}
	for candidate := start; candidate <= end; candidate++ {
		ip := uint32ToAddr(candidate).String()
		if _, exists := used[ip]; exists {
			continue
		}
		if !prefix.Contains(uint32ToAddr(candidate)) {
			continue
		}
		return ip, nil
	}
	return "", fmt.Errorf("%w: subnet is exhausted", ErrConflict)
}

func (s *dbState) requireNodeSession(ctx context.Context, userID, nodeID, networkID string) (repo.Node, error) {
	if strings.TrimSpace(nodeID) == "" || strings.TrimSpace(networkID) == "" {
		return repo.Node{}, fmt.Errorf("%w: nodeId and networkId are required", ErrInvalidArgument)
	}
	node, err := s.pg.GetNodeByID(ctx, nodeID)
	if err != nil {
		if repo.IsNotFound(err) {
			return repo.Node{}, ErrNotFound
		}
		return repo.Node{}, err
	}
	if node.UserID != userID {
		return repo.Node{}, ErrForbidden
	}
	if err := s.ensureNetworkAccess(ctx, userID, networkID); err != nil {
		return repo.Node{}, err
	}
	if _, err := s.pg.GetMemberByNetworkDevice(ctx, networkID, node.DeviceID); err != nil {
		if repo.IsNotFound(err) {
			return repo.Node{}, fmt.Errorf("%w: node device is not a network member", ErrForbidden)
		}
		return repo.Node{}, err
	}
	return node, nil
}

func (s *dbState) deviceNetworkIDs(ctx context.Context, deviceID string) ([]string, error) {
	attachments, err := s.pg.ListAttachmentsByDevice(ctx, deviceID)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{})
	var out []string
	for _, attachment := range attachments {
		if _, ok := seen[attachment.NetworkID]; ok {
			continue
		}
		seen[attachment.NetworkID] = struct{}{}
		out = append(out, attachment.NetworkID)
	}
	return out, nil
}
