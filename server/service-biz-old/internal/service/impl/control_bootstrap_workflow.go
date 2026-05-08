package impl

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
	"github.com/slan/server/server-biz/internal/util"
)

type bootstrapSessionContext struct {
	node      dto.Node
	networkID string
}

type bootstrapRuntimeSession struct {
	controlSessionID string
	sessionToken     string
	networkMap       dto.NetworkMap
}

func (s dbBootstrapService) requireBootstrapRequest(nodeID, networkID string) error {
	if strings.TrimSpace(nodeID) == "" || strings.TrimSpace(networkID) == "" {
		return fmt.Errorf("%w: nodeId and networkId are required", ErrInvalidArgument)
	}
	return nil
}

func (s dbBootstrapService) requireBootstrapSession(ctx context.Context, userID, nodeID, networkID string) (bootstrapSessionContext, error) {
	node, err := s.state.requireNodeSession(ctx, userID, nodeID, networkID)
	if err != nil {
		return bootstrapSessionContext{}, err
	}
	return bootstrapSessionContext{
		node:      node.ToDTO(nil),
		networkID: networkID,
	}, nil
}

func (s dbBootstrapService) createRuntimeSession(ctx context.Context, userID string, session bootstrapSessionContext) (bootstrapRuntimeSession, error) {
	controlSessionID, sessionToken, err := s.state.createStoredControlSession(ctx, userID, session.node.DeviceID, session.node.NodeID, session.networkID)
	if err != nil {
		return bootstrapRuntimeSession{}, err
	}
	return bootstrapRuntimeSession{
		controlSessionID: controlSessionID,
		sessionToken:     sessionToken,
		networkMap:       s.state.buildNetworkMap(ctx, userID, session.node, session.networkID),
	}, nil
}

// createStoredControlSession persists a control session in the database and
// stores its token in the token backend.
func (s *dbState) createStoredControlSession(ctx context.Context, userID, deviceID, nodeID, networkID string) (string, string, error) {
	controlSessionID := util.NewID("ctrl")
	sessionToken := util.OpaqueToken("control", controlSessionID)
	now := time.Now().Unix()

	if err := s.pg.CreateControlSession(ctx, repo.ControlSession{
		ControlSessionID: controlSessionID,
		UserID:           userID,
		DeviceID:         deviceID,
		NodeID:           nodeID,
		NetworkID:        networkID,
		SessionToken:     sessionToken,
		ConnectedAt:      now,
		LastSeenAt:       now,
	}); err != nil {
		return "", "", err
	}
	if s.tokens == nil {
		return controlSessionID, sessionToken, nil
	}
	if err := s.tokens.StoreControlSessionToken(ctx, sessionToken, userID, 24*time.Hour); err != nil {
		return "", "", err
	}
	return controlSessionID, sessionToken, nil
}

func (s dbBootstrapService) buildBootstrapDevice(ctx context.Context, userID, deviceID string) (dto.DeviceBootstrap, error) {
	deviceRecord, err := s.state.pg.GetDeviceByID(ctx, deviceID)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.DeviceBootstrap{}, ErrNotFound
		}
		return dto.DeviceBootstrap{}, err
	}
	attachments, err := s.state.pg.ListAttachmentsByDevice(ctx, deviceID)
	if err != nil {
		return dto.DeviceBootstrap{}, err
	}
	attachments, err = s.state.ensureDeviceAttachmentsVirtualIPs(ctx, attachments)
	if err != nil {
		return dto.DeviceBootstrap{}, err
	}
	deviceNetworkIDs, _ := s.state.activeDeviceNetworkIDs(ctx, deviceRecord.DeviceID)
	device := deviceRecord.ToDTO(deviceNetworkIDs)
	device.MQTT = s.state.buildDeviceMQTTCredential(deviceRecord)
	return dto.DeviceBootstrap{
		Device:      device,
		Attachments: attachments,
	}, nil
}

func (s dbBootstrapService) buildVisibleNetworkDetails(ctx context.Context, userID string) ([]dto.NetworkDetail, error) {
	visibleNetworks, err := s.state.pg.ListVisibleNetworksByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	networks := make([]dto.NetworkDetail, 0, len(visibleNetworks))
	for _, network := range visibleNetworks {
		detail, err := dbNetworkService{state: s.state}.Get(userID, network.NetworkID)
		if err != nil {
			continue
		}
		networks = append(networks, detail)
	}
	return networks, nil
}
