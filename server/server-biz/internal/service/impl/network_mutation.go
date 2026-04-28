package impl

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
	"github.com/slan/server/server-biz/internal/util"
)

func (s dbNetworkService) Create(userID string, req dto.CreateNetworkRequest) (dto.Network, error) {
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
		if _, err := s.state.ensureMember(ctx, networkID, bindDeviceID, "owner"); err != nil {
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
	if err := s.state.reassignDefaultSubnetLeasePool(ctx, record.NetworkID, subnet); err != nil {
		return dto.Network{}, err
	}
	if err := s.state.pg.UpdateNetworkMetadata(ctx, record.NetworkID, name, description, subnet.CIDR); err != nil {
		return dto.Network{}, err
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

func (s dbNetworkService) UpdateAttachmentIP(userID, networkID, attachmentID string, req dto.UpdateAttachmentIPRequest) (dto.SubnetAttachment, error) {
	ctx := context.Background()
	record, err := s.state.pg.GetNetworkByID(ctx, networkID)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.SubnetAttachment{}, ErrNotFound
		}
		return dto.SubnetAttachment{}, err
	}
	if record.OwnerUserID != userID {
		return dto.SubnetAttachment{}, ErrForbidden
	}
	attachment, err := s.state.pg.GetAttachmentByID(ctx, attachmentID)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.SubnetAttachment{}, ErrNotFound
		}
		return dto.SubnetAttachment{}, err
	}
	if attachment.NetworkID != networkID {
		return dto.SubnetAttachment{}, ErrNotFound
	}
	member, err := s.state.pg.GetMemberByNetworkDevice(ctx, networkID, attachment.DeviceID)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.SubnetAttachment{}, ErrNotFound
		}
		return dto.SubnetAttachment{}, err
	}
	if member.Role == "owner" {
		subnet, err := s.state.pg.GetSubnetByID(ctx, attachment.SubnetID)
		if err != nil {
			if repo.IsNotFound(err) {
				return dto.SubnetAttachment{}, ErrNotFound
			}
			return dto.SubnetAttachment{}, err
		}
		preferred, ok, err := s.state.preferredOwnerIP(ctx, subnet)
		if err != nil {
			return dto.SubnetAttachment{}, err
		}
		if !ok {
			prefix, start, _, rangeErr := subnetRange(subnet.CIDR, subnet.GatewayIP, subnet.AllocationStartIP, subnet.AllocationEndIP)
			if rangeErr != nil {
				return dto.SubnetAttachment{}, rangeErr
			}
			if prefix.Contains(uint32ToAddr(start)) {
				preferred = uint32ToAddr(start).String()
			}
		}
		if preferred != "" && strings.TrimSpace(req.VirtualIP) != preferred {
			return dto.SubnetAttachment{}, fmt.Errorf("%w: owner device virtual ip is fixed to %s", ErrConflict, preferred)
		}
	}
	reserved, err := s.Reserve(networkID, attachment.SubnetID, attachment.AttachmentID, attachment.DeviceID, req.VirtualIP)
	if err != nil {
		return dto.SubnetAttachment{}, err
	}
	if err := s.state.pg.UpdateAttachmentVirtualIP(ctx, attachment.AttachmentID, reserved); err != nil {
		return dto.SubnetAttachment{}, err
	}
	attachment.VirtualIP = reserved
	s.state.publishDeviceIPReassigned(networkID, attachment.DeviceID, attachment.AttachmentID, reserved)
	return attachment, nil
}

func (s dbNetworkService) UpdateAttachmentRemark(userID, networkID, attachmentID string, req dto.UpdateAttachmentRemarkRequest) (dto.NetworkAssignment, error) {
	ctx := context.Background()
	record, err := s.state.pg.GetNetworkByID(ctx, networkID)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.NetworkAssignment{}, ErrNotFound
		}
		return dto.NetworkAssignment{}, err
	}
	attachment, err := s.state.pg.GetAttachmentByID(ctx, attachmentID)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.NetworkAssignment{}, ErrNotFound
		}
		return dto.NetworkAssignment{}, err
	}
	if attachment.NetworkID != networkID {
		return dto.NetworkAssignment{}, ErrNotFound
	}
	if record.OwnerUserID != userID {
		if err := s.state.ensureDeviceOwner(ctx, userID, attachment.DeviceID); err != nil {
			return dto.NetworkAssignment{}, err
		}
	}
	remark := strings.TrimSpace(req.Remark)
	if err := s.state.pg.UpdateAttachmentRemark(ctx, attachmentID, remark); err != nil {
		return dto.NetworkAssignment{}, err
	}
	assignments, err := s.state.pg.ListAssignmentsByNetwork(ctx, networkID)
	if err != nil {
		return dto.NetworkAssignment{}, err
	}
	for _, item := range assignments {
		if item.AttachmentID == attachmentID {
			return item, nil
		}
	}
	return dto.NetworkAssignment{}, ErrNotFound
}

func sanitizeValues(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		value := strings.TrimSpace(item)
		if value == "" {
			continue
		}
		out = append(out, value)
	}
	return out
}

func sanitizeDNSWildcards(items []string) ([]string, error) {
	out := make([]string, 0, len(items))
	for _, item := range items {
		value := strings.ToLower(strings.TrimSpace(item))
		if value == "" {
			continue
		}
		host, ip, ok := strings.Cut(value, "=")
		if !ok {
			return nil, fmt.Errorf("%w: dns wildcard must use pattern=ip", ErrInvalidArgument)
		}
		host = normalizeDNSWildcardHost(host)
		ip = strings.TrimSpace(ip)
		if !isAllowedDNSWildcardHost(host) {
			return nil, fmt.Errorf("%w: dns wildcard only supports *.xx.com or *.*.xx.com style domains", ErrInvalidArgument)
		}
		if parsed := net.ParseIP(ip); parsed == nil || parsed.To4() == nil {
			return nil, fmt.Errorf("%w: dns wildcard target must be an IPv4 address", ErrInvalidArgument)
		}
		out = append(out, host+"="+ip)
	}
	return out, nil
}

func normalizeDNSWildcardHost(host string) string {
	host = strings.Trim(strings.ToLower(strings.TrimSpace(host)), ".")
	if strings.HasPrefix(host, "*") && !strings.HasPrefix(host, "*.") {
		host = "*." + strings.TrimPrefix(host, "*")
	}
	return host
}

func isAllowedDNSWildcardHost(host string) bool {
	if strings.Contains(host, "..") {
		return false
	}
	labels := strings.Split(host, ".")
	wildcardCount := 0
	for wildcardCount < len(labels) && labels[wildcardCount] == "*" {
		wildcardCount++
	}
	if wildcardCount == 0 || wildcardCount == len(labels) {
		return false
	}
	for _, label := range labels[wildcardCount:] {
		if label == "*" || !isDNSLabel(label) {
			return false
		}
	}
	return len(labels)-wildcardCount >= 2
}

func isDNSLabel(label string) bool {
	if label == "" || len(label) > 63 {
		return false
	}
	if label[0] == '-' || label[len(label)-1] == '-' {
		return false
	}
	for _, ch := range label {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-' {
			continue
		}
		return false
	}
	return true
}

func hasCreateNetworkDHCPOptions(req dto.CreateNetworkRequest) bool {
	return strings.TrimSpace(req.AllocationStartIP) != "" ||
		strings.TrimSpace(req.AllocationEndIP) != ""
}
