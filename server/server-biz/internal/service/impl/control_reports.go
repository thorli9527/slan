package impl

import (
	"context"
	"fmt"
	"strings"

	"github.com/slan/server/server-biz/api/dto"
	controlmsg "github.com/slan/server/server-biz/internal/controlmsg"
	"github.com/slan/server/server-biz/internal/repo"
	"github.com/slan/server/server-biz/internal/util"
)

// ReportEndpoints replaces the node's advertised endpoints and NAT observation
// for the target network, then returns an updated network map.
func (s dbControlChannelService) ReportEndpoints(userID string, report controlmsg.EndpointReport) (dto.NetworkMap, error) {
	if strings.TrimSpace(report.NodeID) == "" || strings.TrimSpace(report.NetworkID) == "" {
		return dto.NetworkMap{}, fmt.Errorf("%w: nodeId and networkId are required", ErrInvalidArgument)
	}

	ctx := context.Background()
	node, err := s.state.requireNodeSession(ctx, userID, report.NodeID, report.NetworkID)
	if err != nil {
		return dto.NetworkMap{}, err
	}

	endpoints := make([]repo.NodeEndpoint, 0, len(report.Endpoints))
	for _, endpoint := range report.Endpoints {
		if strings.TrimSpace(endpoint.Type) == "" || strings.TrimSpace(endpoint.Address) == "" {
			return dto.NetworkMap{}, fmt.Errorf("%w: endpoint type and address are required", ErrInvalidArgument)
		}
		endpoints = append(endpoints, repo.NodeEndpoint{
			EndpointID: util.NewID("ep"),
			Type:       endpoint.Type,
			Address:    endpoint.Address,
			UpdatedAt:  endpoint.UpdatedAt,
		})
	}
	if err := s.state.pg.ReplaceNodeEndpoints(ctx, node.NodeID, report.NetworkID, report.NatType, endpoints); err != nil {
		return dto.NetworkMap{}, err
	}
	if err := s.state.pg.UpdateDeviceStatus(ctx, node.DeviceID, "reachable"); err != nil {
		return dto.NetworkMap{}, err
	}
	return s.state.buildNetworkMap(ctx, userID, node.ToDTO(nil), report.NetworkID), nil
}
