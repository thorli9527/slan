package impl

import (
	"context"
	"strings"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
)

func (s dbNetworkService) ListAssignments(userID, networkID string) ([]dto.NetworkAssignment, error) {
	ctx := context.Background()
	record, err := s.state.pg.GetNetworkByID(ctx, networkID)
	if err != nil {
		if repo.IsNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if record.OwnerUserID != userID {
		return nil, ErrForbidden
	}
	assignments, err := s.state.pg.ListAssignmentsByNetwork(ctx, networkID)
	if err != nil {
		return nil, err
	}
	repaired, err := s.ensureAssignmentVirtualIPs(ctx, assignments)
	if err != nil {
		return nil, err
	}
	if repaired {
		assignments, err = s.state.pg.ListAssignmentsByNetwork(ctx, networkID)
		if err != nil {
			return nil, err
		}
	}
	s.state.applyLiveDeviceNetworkStates(ctx, assignments)
	return assignments, nil
}

func (s dbNetworkService) ensureAssignmentVirtualIPs(ctx context.Context, assignments []dto.NetworkAssignment) (bool, error) {
	repaired := false
	for _, assignment := range assignments {
		if strings.TrimSpace(assignment.VirtualIP) != "" {
			continue
		}
		attachment, err := s.state.pg.GetAttachmentByID(ctx, assignment.AttachmentID)
		if err != nil {
			return false, err
		}
		_, err = s.state.pg.GetMemberByNetworkDevice(ctx, assignment.NetworkID, assignment.DeviceID)
		if err != nil {
			return false, err
		}
		if _, err := s.state.ensureAttachmentVirtualIP(ctx, attachment); err != nil {
			return false, err
		}
		repaired = true
	}
	return repaired, nil
}
