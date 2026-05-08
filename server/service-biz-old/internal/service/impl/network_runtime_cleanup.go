package impl

import (
	"context"
	"strings"

	"github.com/slan/server/server-biz/api/dto"
)

func (s *dbState) markDeviceNetworkRuntimeOffline(ctx context.Context, networkID, deviceID string) error {
	return s.upsertTrustedDeviceNetworkState(ctx, deviceID, networkID, dto.DeviceNetworkStateRequest{
		ControlReachable: true,
		NetworkOnline:    false,
		TunnelUp:         false,
		LastProbeOK:      false,
	})
}

func (s *dbState) cleanupDeactivatedNetworkDevice(ctx context.Context, networkID, deviceID string) error {
	if err := s.cleanupNetworkDeviceRuntime(ctx, networkID, deviceID); err != nil {
		return err
	}
	return s.pg.DeleteAttachmentsByDeviceInNetwork(ctx, deviceID, networkID)
}

func (s *dbState) cleanupNetworkDeviceRuntime(ctx context.Context, networkID, deviceID string) error {
	nodes, err := s.pg.ListNodesByDevice(ctx, deviceID)
	if err != nil {
		return err
	}
	if sessions, err := s.pg.ListControlSessionsByDeviceInNetwork(ctx, deviceID, networkID); err != nil {
		return err
	} else {
		s.deleteControlSessionTokens(ctx, sessions)
	}
	if err := s.pg.DeleteControlSessionsByDeviceInNetwork(ctx, deviceID, networkID); err != nil {
		return err
	}
	_ = s.tokens.DeleteDeviceNetworkState(ctx, deviceID, networkID)
	s.deleteEnabledNetworkMember(ctx, deviceID, networkID)
	for _, node := range nodes {
		s.publishPeerRemove(networkID, node.NodeID)
	}
	return nil
}

func (s *dbState) clearUserActiveNetworkIfNoAttachments(ctx context.Context, userID, networkID string) error {
	user, err := s.pg.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(user.ActiveNetworkID) != networkID {
		return nil
	}
	devices, err := s.pg.ListDevicesByUser(ctx, userID)
	if err != nil {
		return err
	}
	for _, device := range devices {
		attachments, err := s.pg.ListAttachmentsByDevice(ctx, device.DeviceID)
		if err != nil {
			return err
		}
		for _, attachment := range attachments {
			if attachment.NetworkID == networkID && attachment.Status == "active" && attachment.VirtualIP != "" {
				return nil
			}
		}
	}
	return s.pg.UpdateUserActiveNetwork(ctx, userID, "")
}
