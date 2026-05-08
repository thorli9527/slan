package impl

import (
	"context"
	"strings"
	"time"

	controlmsg "github.com/slan/server/server-biz/internal/controlmsg"
)

func (s *dbState) publishNetworkRestartRequired(networkID, cidr string) {
	if strings.TrimSpace(networkID) == "" || s.tokens == nil {
		return
	}

	ctx := context.Background()
	revision, err := s.tokens.NextNetworkRevision(ctx, networkID)
	if err != nil || revision == 0 {
		revision = 1
	}
	_ = s.tokens.PublishControlSyncEvent(ctx, controlmsg.ControlSyncEvent{
		Type:      "network_restart_required",
		NetworkID: networkID,
		Revision:  revision,
		Restart: &controlmsg.NetworkRestartRequired{
			NetworkID:         networkID,
			Revision:          revision,
			Reason:            "network configuration changed, tunnel restart required",
			DefaultSubnetCIDR: cidr,
		},
	})
}

func (s *dbState) publishDeviceIPReassigned(networkID, deviceID, attachmentID, virtualIP string) {
	if strings.TrimSpace(networkID) == "" || strings.TrimSpace(deviceID) == "" || s.tokens == nil {
		return
	}

	_ = s.tokens.PublishControlSyncEvent(context.Background(), controlmsg.ControlSyncEvent{
		Type:      "device_ip_reassigned",
		NetworkID: networkID,
		DeviceIP: &controlmsg.DeviceIPReassigned{
			NetworkID:    networkID,
			DeviceID:     deviceID,
			AttachmentID: attachmentID,
			VirtualIP:    virtualIP,
			Reason:       "attachment virtual ip updated",
		},
	})
}

func (s *dbState) publishDeviceNetworkDisabled(networkID, deviceID, attachmentID, reason string) {
	if strings.TrimSpace(networkID) == "" || strings.TrimSpace(deviceID) == "" || s.tokens == nil {
		return
	}
	if strings.TrimSpace(reason) == "" {
		reason = "attachment disabled by network owner"
	}

	_ = s.tokens.PublishControlSyncEvent(context.Background(), controlmsg.ControlSyncEvent{
		Type:      controlmsg.DeviceNetworkDisabledEvent,
		NetworkID: networkID,
		DeviceDisabled: &controlmsg.DeviceNetworkDisabled{
			NetworkID:    networkID,
			DeviceID:     deviceID,
			AttachmentID: attachmentID,
			Reason:       reason,
		},
		DevicePresence: s.deviceNetworkPresence(context.Background(), "", networkID, deviceID, "", false, reason),
	})
}

func (s *dbState) publishActiveNetworkEnabled(userID, networkID, reason string) {
	if strings.TrimSpace(userID) == "" || strings.TrimSpace(networkID) == "" || s.tokens == nil {
		return
	}

	_ = s.tokens.PublishControlSyncEvent(context.Background(), controlmsg.ControlSyncEvent{
		Type:         "active_network_enabled",
		NetworkID:    networkID,
		TargetUserID: userID,
		ActiveNetwork: &controlmsg.ActiveNetworkEnabled{
			UserID:    userID,
			NetworkID: networkID,
			Reason:    reason,
		},
	})
}

func (s *dbState) publishDeviceNetworkPresence(userID, networkID, deviceID, virtualIP string, online bool, reason string) {
	if strings.TrimSpace(networkID) == "" || strings.TrimSpace(deviceID) == "" || s.tokens == nil {
		return
	}
	if strings.TrimSpace(reason) == "" {
		if online {
			reason = "device network enabled"
		} else {
			reason = "device network disabled"
		}
	}
	eventType := controlmsg.DeviceNetworkDisabledEvent
	if online {
		eventType = controlmsg.DeviceNetworkEnabledEvent
	}
	_ = s.tokens.PublishControlSyncEvent(context.Background(), controlmsg.ControlSyncEvent{
		Type:           eventType,
		NetworkID:      networkID,
		DevicePresence: s.deviceNetworkPresence(context.Background(), userID, networkID, deviceID, virtualIP, online, reason),
	})
}

func (s *dbState) publishDeviceNetworkExpired(networkID, deviceID, virtualIP string) {
	if strings.TrimSpace(networkID) == "" || strings.TrimSpace(deviceID) == "" || s.tokens == nil {
		return
	}
	reason := "enabled network member heartbeat expired"
	_ = s.tokens.PublishControlSyncEvent(context.Background(), controlmsg.ControlSyncEvent{
		Type:           controlmsg.DeviceNetworkExpiredEvent,
		NetworkID:      networkID,
		DevicePresence: s.deviceNetworkPresence(context.Background(), "", networkID, deviceID, virtualIP, false, reason),
	})
}

func (s *dbState) deviceNetworkPresence(ctx context.Context, userID, networkID, deviceID, virtualIP string, online bool, reason string) *controlmsg.DeviceNetworkPresence {
	return &controlmsg.DeviceNetworkPresence{
		NetworkID:     networkID,
		DeviceID:      deviceID,
		UserID:        userID,
		Online:        online,
		VirtualIP:     strings.TrimSpace(virtualIP),
		ChangedAt:     time.Now().Unix(),
		Reason:        strings.TrimSpace(reason),
		OnlineDevices: s.enabledDevicePresenceSnapshot(ctx, networkID),
	}
}

func (s *dbState) publishPeerRemove(networkID, nodeID string) {
	if strings.TrimSpace(networkID) == "" || strings.TrimSpace(nodeID) == "" || s.tokens == nil {
		return
	}

	ctx := context.Background()
	revision, err := s.tokens.NextNetworkRevision(ctx, networkID)
	if err != nil || revision == 0 {
		revision = 1
	}
	_ = s.tokens.PublishControlSyncEvent(ctx, controlmsg.ControlSyncEvent{
		Type:         "peer_remove",
		NetworkID:    networkID,
		SourceNodeID: nodeID,
		Revision:     revision,
		PeerNodeID:   nodeID,
	})
}
