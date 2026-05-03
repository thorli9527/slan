package impl

import (
	"context"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/service"
)

type dbBootstrapService struct{ state *dbState }

var _ service.Bootstrap = dbBootstrapService{}

// CreateControlSession allocates a fresh control-plane token for an existing
// node and returns the initial network map snapshot.
func (s dbBootstrapService) CreateControlSession(userID string, req dto.CreateControlSessionRequest) (dto.ControlSessionResponse, error) {
	if err := s.requireBootstrapRequest(req.NodeID, req.NetworkID); err != nil {
		return dto.ControlSessionResponse{}, err
	}

	ctx := context.Background()
	session, err := s.requireBootstrapSession(ctx, userID, req.NodeID, req.NetworkID)
	if err != nil {
		return dto.ControlSessionResponse{}, err
	}
	runtime, err := s.createRuntimeSession(ctx, userID, session)
	if err != nil {
		return dto.ControlSessionResponse{}, err
	}
	return dto.ControlSessionResponse{
		ControlSessionID: runtime.controlSessionID,
		SessionToken:     runtime.sessionToken,
		ControlPlane:     s.state.controlPlaneConfig(),
		NetworkMap:       runtime.networkMap,
	}, nil
}

// Bootstrap returns the full startup payload needed by a client after login,
// including device view, visible networks, relay config and control-plane data.
func (s dbBootstrapService) Bootstrap(userID string, req dto.BootstrapRequest) (dto.BootstrapResponse, error) {
	if err := s.requireBootstrapRequest(req.NodeID, req.NetworkID); err != nil {
		return dto.BootstrapResponse{}, err
	}

	ctx := context.Background()
	session, err := s.requireBootstrapSession(ctx, userID, req.NodeID, req.NetworkID)
	if err != nil {
		return dto.BootstrapResponse{}, err
	}
	device, err := s.buildBootstrapDevice(ctx, userID, session.node.DeviceID)
	if err != nil {
		return dto.BootstrapResponse{}, err
	}
	networks, err := s.buildVisibleNetworkDetails(ctx, userID)
	if err != nil {
		return dto.BootstrapResponse{}, err
	}
	runtime, err := s.createRuntimeSession(ctx, userID, session)
	if err != nil {
		return dto.BootstrapResponse{}, err
	}
	if err := s.state.pg.UpdateUserActiveNetwork(ctx, userID, req.NetworkID); err != nil {
		return dto.BootstrapResponse{}, err
	}

	return dto.BootstrapResponse{
		ControlSessionID: runtime.controlSessionID,
		SessionToken:     runtime.sessionToken,
		Device:           device,
		Networks:         networks,
		ControlPlane:     s.state.controlPlaneConfig(),
		STUNServers:      append([]string(nil), s.state.cfg.Bootstrap.STUNServers...),
		Relay:            s.state.relayConfig(),
		DerpMap:          s.state.derpMap(),
		NetworkMap:       runtime.networkMap,
	}, nil
}
