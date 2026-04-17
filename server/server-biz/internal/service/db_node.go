package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
)

func (s dbNodeService) Register(userID string, req dto.RegisterNodeRequest) (dto.Node, error) {
	if strings.TrimSpace(req.DeviceID) == "" || strings.TrimSpace(req.NodeID) == "" || strings.TrimSpace(req.NodePublicKey) == "" {
		return dto.Node{}, fmt.Errorf("%w: deviceId, nodeId, and nodePublicKey are required", ErrInvalidArgument)
	}
	ctx := context.Background()
	if err := s.state.ensureDeviceOwner(ctx, userID, req.DeviceID); err != nil {
		return dto.Node{}, err
	}
	node := dto.Node{
		NodeID:        req.NodeID,
		DeviceID:      req.DeviceID,
		NodePublicKey: req.NodePublicKey,
		Capabilities:  append([]string(nil), req.Capabilities...),
	}
	node.NetworkIDs, _ = s.state.deviceNetworkIDs(ctx, req.DeviceID)
	if err := s.state.pg.UpsertNode(ctx, repo.Node{
		NodeID:        node.NodeID,
		UserID:        userID,
		DeviceID:      node.DeviceID,
		NodePublicKey: node.NodePublicKey,
		Capabilities:  node.Capabilities,
	}); err != nil {
		return dto.Node{}, err
	}
	return node, nil
}
