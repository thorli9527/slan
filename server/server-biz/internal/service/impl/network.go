package impl

import (
	"context"
	"fmt"
	"strings"

	"github.com/slan/server/server-biz/internal/repo"
	"github.com/slan/server/server-biz/internal/service"
)

const (
	defaultControlHeartbeatSeconds = 15
	defaultTunnelMTU               = 1280
)

type dbNetworkService struct{ state *dbState }

var _ service.Network = dbNetworkService{}
var _ service.Allocator = dbNetworkService{}

func (s dbNetworkService) Allocate(networkID, subnetID, attachmentID, deviceID string) (string, error) {
	ctx := context.Background()
	subnet, err := s.state.pg.GetSubnetByID(ctx, subnetID)
	if err != nil {
		if repo.IsNotFound(err) {
			return "", ErrNotFound
		}
		return "", err
	}
	if subnet.NetworkID != networkID {
		return "", ErrNotFound
	}
	return s.state.allocateIP(ctx, subnet)
}

func (s dbNetworkService) Reserve(networkID, subnetID, attachmentID, deviceID, ip string) (string, error) {
	ctx := context.Background()
	subnet, err := s.state.pg.GetSubnetByID(ctx, subnetID)
	if err != nil {
		if repo.IsNotFound(err) {
			return "", ErrNotFound
		}
		return "", err
	}
	if subnet.NetworkID != networkID {
		return "", ErrNotFound
	}
	if strings.TrimSpace(ip) == "" {
		return "", fmt.Errorf("%w: ip is required", ErrInvalidArgument)
	}
	attachments, err := s.state.pg.ListAttachmentsBySubnet(ctx, subnetID)
	if err != nil {
		return "", err
	}
	for _, attachment := range attachments {
		if attachment.AttachmentID != attachmentID && attachment.VirtualIP == ip {
			return "", fmt.Errorf("%w: virtual ip already allocated", ErrConflict)
		}
	}
	return ip, nil
}

func (s dbNetworkService) Release(attachmentID string) error {
	if strings.TrimSpace(attachmentID) == "" {
		return fmt.Errorf("%w: attachmentId is required", ErrInvalidArgument)
	}
	return s.state.pg.ClearAttachmentVirtualIP(context.Background(), attachmentID)
}
