package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
)

func (s dbBootstrapService) CreateControlSession(userID string, req dto.CreateControlSessionRequest) (dto.ControlSessionResponse, error) {
	ctx := context.Background()
	node, err := s.state.requireNodeSession(ctx, userID, req.NodeID, req.NetworkID)
	if err != nil {
		return dto.ControlSessionResponse{}, err
	}
	controlSessionID := newID("ctrl")
	sessionToken := opaqueToken("control", controlSessionID)
	if err := s.state.pg.CreateControlSession(ctx, repo.ControlSession{
		ControlSessionID: controlSessionID,
		UserID:           userID,
		DeviceID:         node.DeviceID,
		NodeID:           node.NodeID,
		NetworkID:        req.NetworkID,
		SessionToken:     sessionToken,
	}); err != nil {
		return dto.ControlSessionResponse{}, err
	}
	if err := s.state.tokens.StoreControlSessionToken(ctx, sessionToken, userID, 24*time.Hour); err != nil {
		return dto.ControlSessionResponse{}, err
	}
	return dto.ControlSessionResponse{
		ControlSessionID: controlSessionID,
		SessionToken:     sessionToken,
		ControlPlane:     s.state.controlPlaneConfig(),
		NetworkMap:       s.state.buildNetworkMap(ctx, userID, node.ToDTO(nil), req.NetworkID),
	}, nil
}

func (s dbBootstrapService) Bootstrap(userID string, req dto.BootstrapRequest) (dto.BootstrapResponse, error) {
	ctx := context.Background()
	node, err := s.state.requireNodeSession(ctx, userID, req.NodeID, req.NetworkID)
	if err != nil {
		return dto.BootstrapResponse{}, err
	}
	deviceRecord, err := s.state.pg.GetDeviceByID(ctx, node.DeviceID)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.BootstrapResponse{}, ErrNotFound
		}
		return dto.BootstrapResponse{}, err
	}
	attachments, err := s.state.pg.ListAttachmentsByDevice(ctx, node.DeviceID)
	if err != nil {
		return dto.BootstrapResponse{}, err
	}

	seen := make(map[string]struct{})
	var networks []dto.NetworkDetail
	for _, attachment := range attachments {
		if _, ok := seen[attachment.NetworkID]; ok {
			continue
		}
		seen[attachment.NetworkID] = struct{}{}
		detail, err := dbNetworkService{state: s.state}.Get(userID, attachment.NetworkID)
		if err != nil {
			continue
		}
		networks = append(networks, detail)
	}

	deviceNetworkIDs, _ := s.state.deviceNetworkIDs(ctx, deviceRecord.DeviceID)
	device := deviceRecord.ToDTO(deviceNetworkIDs)
	return dto.BootstrapResponse{
		ControlSessionID: newID("ctrl"),
		Device: dto.DeviceBootstrap{
			Device:      device,
			Attachments: attachments,
		},
		Networks:     networks,
		ControlPlane: s.state.controlPlaneConfig(),
		STUNServers:  append([]string(nil), s.state.cfg.Bootstrap.STUNServers...),
		Relay:        s.state.relayConfig(),
		DerpMap:      s.state.derpMap(),
		NetworkMap:   s.state.buildNetworkMap(ctx, userID, node.ToDTO(nil), req.NetworkID),
	}, nil
}

func (s dbBootstrapService) IssueRelayTicket(userID string, req dto.RelayTicketRequest) (dto.RelayTicket, error) {
	return s.state.issueRelayTicket(context.Background(), userID, req)
}

func (s *dbState) issueRelayTicket(ctx context.Context, userID string, req dto.RelayTicketRequest) (dto.RelayTicket, error) {
	if strings.TrimSpace(req.NetworkID) == "" || strings.TrimSpace(req.SrcNodeID) == "" || strings.TrimSpace(req.DstNodeID) == "" || strings.TrimSpace(req.Reason) == "" {
		return dto.RelayTicket{}, fmt.Errorf("%w: networkId, srcNodeId, dstNodeId, and reason are required", ErrInvalidArgument)
	}
	srcNode, err := s.pg.GetNodeByID(ctx, req.SrcNodeID)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.RelayTicket{}, ErrNotFound
		}
		return dto.RelayTicket{}, err
	}
	if srcNode.UserID != userID {
		return dto.RelayTicket{}, ErrForbidden
	}
	if err := s.ensureNetworkAccess(ctx, userID, req.NetworkID); err != nil {
		return dto.RelayTicket{}, err
	}
	dstNode, err := s.pg.GetNodeByID(ctx, req.DstNodeID)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.RelayTicket{}, ErrNotFound
		}
		return dto.RelayTicket{}, err
	}
	if _, err := s.pg.GetMemberByNetworkDevice(ctx, req.NetworkID, srcNode.DeviceID); err != nil {
		if repo.IsNotFound(err) {
			return dto.RelayTicket{}, fmt.Errorf("%w: source node device is not a network member", ErrForbidden)
		}
		return dto.RelayTicket{}, err
	}
	if _, err := s.pg.GetMemberByNetworkDevice(ctx, req.NetworkID, dstNode.DeviceID); err != nil {
		if repo.IsNotFound(err) {
			return dto.RelayTicket{}, fmt.Errorf("%w: destination node device is not a network member", ErrNotFound)
		}
		return dto.RelayTicket{}, err
	}
	ticketID := newID("ticket")
	return dto.RelayTicket{
		TicketID:           ticketID,
		NetworkID:          req.NetworkID,
		SessionID:          newID("session"),
		SrcNodeID:          req.SrcNodeID,
		DstNodeID:          req.DstNodeID,
		DerpClusterID:      strings.TrimSpace(req.DerpClusterID),
		AllowedDerpNodeIDs: append([]string(nil), req.PreferredDerpNodeIDs...),
		RelayURL:           "udp://" + s.cfg.Relay.UDPEndpoint,
		ExpiresAt:          time.Now().Add(10 * time.Minute).UTC().Format(time.RFC3339),
		SessionKey:         base64.RawURLEncoding.EncodeToString([]byte(ticketID + ":" + req.SrcNodeID + ":" + req.DstNodeID)),
		Signature:          base64.RawURLEncoding.EncodeToString([]byte("sig:" + ticketID)),
	}, nil
}
