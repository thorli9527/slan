package service

import (
	"context"
	"errors"
	"strings"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

func (s OpsManagedDeviceService) ListDevices(ctx context.Context) ([]OpsManagedDeviceView, error) {
	items, err := s.Inventory.ListAllDevices(ctx)
	if err != nil {
		return nil, err
	}
	views := make([]OpsManagedDeviceView, 0, len(items))
	for _, item := range items {
		view, err := buildManagedDeviceView(ctx, s.Networks, item)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}

func (s OpsManagedDeviceService) CreateDevice(ctx context.Context, input CreateOpsDeviceInput) (OpsManagedDeviceView, error) {
	input.Name, input.Alias, input.Platform = strings.TrimSpace(input.Name), strings.TrimSpace(input.Alias), strings.TrimSpace(input.Platform)
	if input.Name == "" {
		return OpsManagedDeviceView{}, ErrInvalidArgument
	}
	now := opsNow(s.Now).Unix()
	item := model.Device{DeviceID: generatedID(s.NewDeviceID, "dev"), Name: input.Name, Alias: input.Alias, Platform: input.Platform,
		OSName: strings.TrimSpace(input.OSName), OSVersion: strings.TrimSpace(input.OSVersion), PublicKey: strings.TrimSpace(input.PublicKey), Status: "active", CreatedAt: now, UpdatedAt: now}
	if err := s.Devices.SaveDevice(ctx, item); err != nil {
		return OpsManagedDeviceView{}, err
	}
	return buildManagedDeviceView(ctx, s.Networks, item)
}

func (s OpsManagedDeviceService) UpdateDevice(ctx context.Context, input UpdateDeviceInput) (OpsManagedDeviceView, error) {
	input = normalizeUpdateDeviceInput(input)
	if input.DeviceID == "" || (input.Status != "" && !validManagedDeviceStatus(input.Status)) {
		return OpsManagedDeviceView{}, ErrInvalidArgument
	}
	item, err := requireOpsDevice(ctx, s.Devices, input.DeviceID)
	if err != nil {
		return OpsManagedDeviceView{}, err
	}
	requestedVirtualIP := input.VirtualIP
	if requestedVirtualIP != "" {
		if !managedDeviceVirtualIP(requestedVirtualIP) {
			return OpsManagedDeviceView{}, invalidArgumentError("设备 IP 不在可分配地址范围内")
		}
		if s.Inventory == nil {
			return OpsManagedDeviceView{}, ErrNotImplemented
		}
		items, err := s.Inventory.ListAllDevices(ctx)
		if err != nil {
			return OpsManagedDeviceView{}, err
		}
		for _, existing := range items {
			if existing.DeviceID != item.DeviceID && strings.TrimSpace(existing.VirtualIP) == requestedVirtualIP {
				return OpsManagedDeviceView{}, conflictError("设备 IP 已被使用")
			}
		}
	}
	previousStatus := item.Status
	previousVirtualIP := strings.TrimSpace(item.VirtualIP)
	now := opsNow(s.Now).Unix()
	input.VirtualIP = ""
	item = applyUpdateDeviceInput(item, input, now)
	if err := s.Devices.SaveDevice(ctx, item); err != nil {
		return OpsManagedDeviceView{}, err
	}
	if requestedVirtualIP != "" && requestedVirtualIP != previousVirtualIP {
		updater, ok := s.Devices.(repository.DeviceVirtualIPRepository)
		if !ok {
			return OpsManagedDeviceView{}, ErrNotImplemented
		}
		if err := updater.UpdateDeviceVirtualIP(ctx, item.DeviceID, requestedVirtualIP, now); err != nil {
			if errors.Is(err, repository.ErrDeviceVirtualIPConflict) {
				return OpsManagedDeviceView{}, conflictError("设备 IP 已被使用")
			}
			return OpsManagedDeviceView{}, err
		}
		item.VirtualIP = requestedVirtualIP
		if err := s.publishManagedDeviceIPChange(ctx, item.DeviceID); err != nil {
			return OpsManagedDeviceView{}, err
		}
		s.recordManagedDeviceAudit(ctx, "update_ip", item.DeviceID, now)
	}
	if previousStatus == "active" && item.Status == "disabled" {
		sessions, err := s.Devices.ListDeviceSessionsByDeviceID(ctx, item.DeviceID)
		if err != nil {
			return OpsManagedDeviceView{}, err
		}
		for _, session := range sessions {
			if err := s.Devices.DeleteDeviceSessionByAccessToken(ctx, session.AccessToken); err != nil {
				return OpsManagedDeviceView{}, err
			}
		}
	}
	if previousStatus != item.Status {
		action := "disable"
		if item.Status == "active" {
			action = "enable"
		}
		s.recordManagedDeviceAudit(ctx, action, item.DeviceID, now)
	}
	return buildManagedDeviceView(ctx, s.Networks, item)
}

func (s OpsManagedDeviceService) publishManagedDeviceIPChange(ctx context.Context, deviceID string) error {
	networks, err := s.Networks.ListNetworksByDevice(ctx, deviceID)
	if err != nil {
		return err
	}
	for _, network := range networks {
		version, err := bumpNetworkConfigVersion(ctx, s.Networks, s.EventPublisher, s.Now, network.NetworkID, "device_ip_updated")
		if err != nil {
			return err
		}
		if err := publishNetworkSnapshot(ctx, s.Devices, s.Networks, nil, s.EventPublisher, s.Now, network.NetworkID, version.Version, version.Reason); err != nil {
			return err
		}
	}
	return nil
}

func (s OpsManagedDeviceService) DeleteDevice(ctx context.Context, deviceID string) error {
	return s.Devices.DeleteDevice(ctx, normalizeManagedDeviceID(deviceID))
}

func validManagedDeviceStatus(status string) bool {
	return status == "active" || status == "disabled"
}

func (s OpsManagedDeviceService) recordManagedDeviceAudit(ctx context.Context, action, deviceID string, now int64) {
	if s.Audit == nil {
		return
	}
	eventID, err := randomHex(16)
	if err != nil {
		return
	}
	_ = s.Audit.SaveAuditEvent(ctx, model.AuditEvent{
		EventID: "audit_" + eventID, ActorType: "operator", ActorID: AuthenticatedOperatorID(ctx),
		Action: action, ResourceType: "device", ResourceID: deviceID, Status: "success", CreatedAt: now,
	})
}
