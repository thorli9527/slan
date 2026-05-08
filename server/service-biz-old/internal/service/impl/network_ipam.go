package impl

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
)

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

func (s *dbState) ensureAttachmentVirtualIP(ctx context.Context, attachment dto.SubnetAttachment) (string, error) {
	virtualIP := strings.TrimSpace(attachment.VirtualIP)
	if virtualIP != "" {
		return virtualIP, nil
	}
	subnet, err := s.pg.GetSubnetByID(ctx, attachment.SubnetID)
	if err != nil {
		if repo.IsNotFound(err) {
			return "", ErrNotFound
		}
		return "", err
	}
	virtualIP, err = s.allocateIP(ctx, subnet)
	if err != nil {
		return "", err
	}
	if err := s.pg.UpdateAttachmentVirtualIP(ctx, attachment.AttachmentID, virtualIP); err != nil {
		return "", err
	}
	return virtualIP, nil
}

func (s *dbState) ensureDeviceAttachmentsVirtualIPs(ctx context.Context, attachments []dto.SubnetAttachment) ([]dto.SubnetAttachment, error) {
	out := append([]dto.SubnetAttachment(nil), attachments...)
	for index, attachment := range out {
		if strings.TrimSpace(attachment.VirtualIP) != "" {
			continue
		}
		_, err := s.pg.GetMemberByNetworkDevice(ctx, attachment.NetworkID, attachment.DeviceID)
		if err != nil {
			if repo.IsNotFound(err) {
				continue
			}
			return nil, err
		}
		virtualIP, err := s.ensureAttachmentVirtualIP(ctx, attachment)
		if err != nil {
			return nil, err
		}
		out[index].VirtualIP = virtualIP
	}
	return out, nil
}

type attachmentIPChange struct {
	attachmentID string
	deviceID     string
	virtualIP    string
}

func (s *dbState) reassignDefaultSubnetLeasePool(ctx context.Context, networkID string, subnet dto.Subnet) ([]attachmentIPChange, error) {
	attachments, err := s.pg.ListAttachmentsBySubnet(ctx, subnet.SubnetID)
	if err != nil {
		return nil, err
	}
	prefix, start, end, err := subnetRange(subnet.CIDR, subnet.GatewayIP, subnet.AllocationStartIP, subnet.AllocationEndIP)
	if err != nil {
		return nil, err
	}
	if len(attachments) > int(end-start+1) {
		return nil, fmt.Errorf("%w: subnet is too small for current members", ErrConflict)
	}

	sort.Slice(attachments, func(i, j int) bool {
		return attachments[i].AttachmentID < attachments[j].AttachmentID
	})
	changes := make([]attachmentIPChange, 0, len(attachments))
	for index, attachment := range attachments {
		candidate := start + uint32(index)
		if candidate > end || !prefix.Contains(uint32ToAddr(candidate)) {
			return nil, fmt.Errorf("%w: subnet is exhausted", ErrConflict)
		}
		virtualIP := uint32ToAddr(candidate).String()
		if err := s.pg.UpdateAttachmentVirtualIP(ctx, attachment.AttachmentID, virtualIP); err != nil {
			return nil, err
		}
		changes = append(changes, attachmentIPChange{
			attachmentID: attachment.AttachmentID,
			deviceID:     attachment.DeviceID,
			virtualIP:    virtualIP,
		})
	}
	if err := s.pg.UpdateSubnetRange(ctx, subnet); err != nil {
		return nil, err
	}
	return changes, nil
}
