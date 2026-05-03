package impl

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
)

func (s *dbState) upsertTrustedDeviceNetworkState(ctx context.Context, deviceID, networkID string, req dto.DeviceNetworkStateRequest) error {
	if _, err := s.pg.GetDeviceByID(ctx, deviceID); err != nil {
		if repo.IsNotFound(err) {
			return ErrNotFound
		}
		return err
	}
	if _, err := s.requireActiveNetworkMember(ctx, networkID, deviceID, ErrForbidden, "device"); err != nil {
		return err
	}
	attachmentActive, attachmentVirtualIP, err := s.networkAttachmentRuntimeState(ctx, networkID, deviceID)
	if err != nil {
		return err
	}
	now := reportedDeviceNetworkStateTime(req, time.Now())
	networkOnline, tunnelUp, lastProbeOK, virtualIP := normalizeDeviceNetworkRuntimeState(
		req,
		attachmentActive,
		attachmentVirtualIP,
	)
	state := repo.DeviceNetworkState{
		DeviceID:         deviceID,
		NetworkID:        networkID,
		ControlReachable: req.ControlReachable,
		NetworkOnline:    networkOnline,
		TunnelUp:         tunnelUp,
		LastProbeOK:      lastProbeOK,
		VirtualIP:        virtualIP,
		LastSeenAt:       now,
		UpdatedAt:        time.Now().Unix(),
	}
	if err := s.tokens.StoreDeviceNetworkState(ctx, state, 2*deviceNetworkStateFreshnessWindow); err != nil {
		log.Printf("redis device network state store failed device=%s network=%s err=%v", deviceID, networkID, err)
	}
	return s.pg.UpsertDeviceNetworkState(ctx, state)
}

func reportedDeviceNetworkStateTime(req dto.DeviceNetworkStateRequest, fallback time.Time) int64 {
	if req.ReportedAt > 0 {
		return req.ReportedAt
	}
	return fallback.Unix()
}

func normalizeDeviceNetworkRuntimeState(req dto.DeviceNetworkStateRequest, attachmentActive bool, attachmentVirtualIP string) (bool, bool, bool, string) {
	if !attachmentActive {
		return false, false, false, ""
	}
	virtualIP := strings.TrimSpace(req.VirtualIP)
	if virtualIP == "" {
		virtualIP = strings.TrimSpace(attachmentVirtualIP)
	}
	return req.NetworkOnline, req.TunnelUp, req.LastProbeOK, virtualIP
}

func (s *dbState) networkAttachmentRuntimeState(ctx context.Context, networkID, deviceID string) (bool, string, error) {
	attachments, err := s.pg.ListAttachmentsByDevice(ctx, deviceID)
	if err != nil {
		return false, "", err
	}
	for _, attachment := range attachments {
		if attachment.NetworkID != networkID {
			continue
		}
		if attachment.Status == "active" && strings.TrimSpace(attachment.VirtualIP) != "" {
			return true, strings.TrimSpace(attachment.VirtualIP), nil
		}
		return false, "", nil
	}
	return false, "", nil
}

func (s *dbState) markDeviceNetworkAttachmentDisabled(ctx context.Context, networkID, deviceID string) error {
	state, err := s.pg.GetDeviceNetworkState(ctx, deviceID, networkID)
	if err != nil {
		if !repo.IsNotFound(err) {
			return err
		}
		state = repo.DeviceNetworkState{
			DeviceID:  deviceID,
			NetworkID: networkID,
		}
	}
	state.NetworkOnline = false
	state.TunnelUp = false
	state.LastProbeOK = false
	state.VirtualIP = ""
	state.UpdatedAt = time.Now().Unix()
	if deviceNetworkStateIsFresh(state, time.Now()) {
		if err := s.tokens.StoreDeviceNetworkState(ctx, state, 2*deviceNetworkStateFreshnessWindow); err != nil {
			log.Printf("redis device network state store failed device=%s network=%s err=%v", deviceID, networkID, err)
		}
	}
	return s.pg.UpsertDeviceNetworkState(ctx, state)
}
