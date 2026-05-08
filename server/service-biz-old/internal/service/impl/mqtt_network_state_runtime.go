package impl

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/slan/server/server-biz/api/dto"
	controlmsg "github.com/slan/server/server-biz/internal/controlmsg"
	"github.com/slan/server/server-biz/internal/repo"
	"github.com/slan/server/server-biz/internal/util"
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
	wasOnline := false
	if previous, err := s.pg.GetDeviceNetworkState(ctx, deviceID, networkID); err == nil {
		wasOnline = previous.NetworkOnline && deviceNetworkStateIsFresh(previous, time.Now())
	}
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
	s.storeDeviceNetworkRuntime(ctx, state)
	if err := s.pg.UpsertDeviceNetworkState(ctx, state); err != nil {
		return err
	}
	if wasOnline != networkOnline {
		s.publishDeviceNetworkPresence("", networkID, deviceID, virtualIP, networkOnline, "device network runtime state changed")
	}
	return s.upsertRelayPolicyExecutionFromNetworkState(ctx, deviceID, networkID, req)
}

func (s *dbState) refreshEnabledNetworkMember(ctx context.Context, networkID, deviceID, virtualIP string, now time.Time) {
	state := repo.DeviceNetworkState{
		DeviceID:         deviceID,
		NetworkID:        networkID,
		ControlReachable: true,
		NetworkOnline:    true,
		TunnelUp:         true,
		LastProbeOK:      true,
		VirtualIP:        strings.TrimSpace(virtualIP),
		LastSeenAt:       now.Unix(),
		UpdatedAt:        now.Unix(),
	}
	s.storeEnabledNetworkMember(ctx, state)
}

func (s *dbState) storeDeviceNetworkRuntime(ctx context.Context, state repo.DeviceNetworkState) {
	if err := s.tokens.StoreDeviceNetworkState(ctx, state, 2*deviceNetworkStateFreshnessWindow); err != nil {
		log.Printf("redis device network state store failed device=%s network=%s err=%v", state.DeviceID, state.NetworkID, err)
	}
	if state.NetworkOnline {
		s.storeEnabledNetworkMember(ctx, state)
		return
	}
	s.deleteEnabledNetworkMember(ctx, state.DeviceID, state.NetworkID)
}

func (s *dbState) storeEnabledNetworkMember(ctx context.Context, state repo.DeviceNetworkState) {
	if err := s.tokens.StoreEnabledNetworkMember(ctx, state, deviceNetworkStateFreshnessWindow); err != nil {
		log.Printf("redis enabled network member store failed device=%s network=%s err=%v", state.DeviceID, state.NetworkID, err)
	}
}

func (s *dbState) deleteEnabledNetworkMember(ctx context.Context, deviceID, networkID string) {
	if err := s.tokens.DeleteEnabledNetworkMember(ctx, deviceID, networkID); err != nil {
		log.Printf("redis enabled network member delete failed device=%s network=%s err=%v", deviceID, networkID, err)
	}
}

func (s *dbState) enabledDevicePresenceSnapshot(ctx context.Context, networkID string) []controlmsg.DeviceNetworkPresenceIP {
	states, err := s.tokens.ListEnabledNetworkMembers(ctx, networkID)
	if err != nil {
		log.Printf("redis enabled network member list failed network=%s err=%v", networkID, err)
		return nil
	}
	out := make([]controlmsg.DeviceNetworkPresenceIP, 0, len(states))
	for _, state := range states {
		if state.NetworkID != networkID {
			continue
		}
		out = append(out, controlmsg.DeviceNetworkPresenceIP{
			DeviceID:  state.DeviceID,
			VirtualIP: state.VirtualIP,
			LastSeen:  state.LastSeenAt,
		})
	}
	return out
}

func (s *dbState) upsertRelayPolicyExecutionFromNetworkState(ctx context.Context, deviceID, networkID string, req dto.DeviceNetworkStateRequest) error {
	policyID := strings.TrimSpace(req.RelayPolicyID)
	if policyID == "" {
		return nil
	}
	reportedAtMs := req.RelayPolicyAppliedAtMs
	if reportedAtMs == 0 {
		if req.ReportedAt > 0 {
			reportedAtMs = uint64(req.ReportedAt) * 1000
		} else {
			reportedAtMs = uint64(time.Now().UnixMilli())
		}
	}
	templateID := ""
	templateName := ""
	if existing, err := s.pg.GetRelayPolicyExecution(ctx, networkID, deviceID, policyID); err == nil {
		templateID = existing.TemplateID
		templateName = existing.TemplateName
	}
	return s.pg.UpsertRelayPolicyExecution(ctx, repo.RelayPolicyExecution{
		ExecutionID:       util.NewID("relay-exec"),
		NetworkID:         networkID,
		DeviceID:          deviceID,
		PolicyID:          policyID,
		TemplateID:        templateID,
		TemplateName:      templateName,
		Scope:             strings.TrimSpace(req.RelayPolicyScope),
		RelayMtu:          req.RelayMtu,
		MaxFramePayload:   req.MaxFramePayload,
		Applied:           req.RelayPolicyApplied,
		Reason:            strings.TrimSpace(req.RelayPolicyReason),
		PolicyUpdatedAtMs: req.RelayPolicyUpdatedAtMs,
		ReportedAtMs:      reportedAtMs,
		UpdatedAt:         time.Now().Unix(),
	})
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
