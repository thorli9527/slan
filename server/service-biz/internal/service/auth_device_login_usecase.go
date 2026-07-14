package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/pkg/mqttkit"
)

func (s AuthDeviceLoginPrepareService) PrepareDeviceLoginDevice(ctx context.Context, input PrepareDeviceLoginDeviceInput) (PrepareDeviceLoginDeviceView, error) {
	input = normalizePrepareDeviceLoginDeviceInput(input)
	if input.Name == "" || input.Platform == "" {
		return PrepareDeviceLoginDeviceView{}, ErrInvalidArgument
	}
	if input.UserID != "" {
		if _, err := requireAuthUser(ctx, s.Users, input.UserID); err != nil {
			return PrepareDeviceLoginDeviceView{}, err
		}
	}
	deviceID := input.DeviceID
	if deviceID == "" {
		deviceID = newAuthDeviceID(s.Devices)
	}
	code, err := randomHex(4)
	if err != nil {
		return PrepareDeviceLoginDeviceView{}, err
	}
	now := authNow(s.Now)
	item := model.DeviceLoginDevice{
		DeviceID:      deviceID,
		UserID:        input.UserID,
		Name:          input.Name,
		Platform:      input.Platform,
		Alias:         input.Alias,
		OSName:        input.OSName,
		OSVersion:     input.OSVersion,
		PublicKey:     input.PublicKey,
		DeviceVersion: input.DeviceVersion,
		CountryCode:   input.CountryCode,
		VerifyCode:    strings.ToUpper(code[:8]),
		Status:        "pending",
		ExpiresAt:     now.Add(15 * time.Minute).Unix(),
		CreatedAt:     now.Unix(),
		UpdatedAt:     now.Unix(),
	}
	if err := s.Devices.SaveDeviceLoginDevice(ctx, item); err != nil {
		return PrepareDeviceLoginDeviceView{}, err
	}
	return PrepareDeviceLoginDeviceView{
		Device:     deviceLoginDeviceView(item),
		Credential: mqttkit.CredentialForDevice(s.MQTT, item.DeviceID, now),
	}, nil
}

func (s AuthDeviceLoginCompleteService) CompleteDeviceLoginDevice(ctx context.Context, input CompleteDeviceLoginDeviceInput) (CompleteDeviceLoginDeviceView, error) {
	input = normalizeCompleteDeviceLoginDeviceInput(input)
	if input.DeviceID == "" || input.AccessToken == "" {
		return CompleteDeviceLoginDeviceView{}, ErrInvalidArgument
	}
	nowTime := authNow(s.Now)
	userSession, err := requireActiveUserSession(ctx, s.Sessions, input.AccessToken, nowTime.Unix())
	if err != nil {
		return CompleteDeviceLoginDeviceView{}, err
	}
	user, err := requireAuthUser(ctx, s.Users, userSession.UserID)
	if err != nil {
		return CompleteDeviceLoginDeviceView{}, err
	}
	if input.UserID != "" && input.UserID != user.UserID {
		return CompleteDeviceLoginDeviceView{}, ErrUnauthorized
	}
	input.UserID = user.UserID
	item, ok, err := s.Devices.GetDeviceLoginDevice(ctx, input.DeviceID)
	if err != nil {
		return CompleteDeviceLoginDeviceView{}, err
	}
	if !ok {
		return CompleteDeviceLoginDeviceView{}, ErrNotFound
	}
	if item.Status != "pending" || item.ExpiresAt < authNow(s.Now).Unix() {
		return CompleteDeviceLoginDeviceView{}, ErrConflict
	}
	if input.UserID != "" {
		item.UserID = input.UserID
	}
	if input.Name != "" {
		item.Name = input.Name
	}
	if input.Platform != "" {
		item.Platform = input.Platform
	}
	if input.Alias != "" {
		item.Alias = input.Alias
	}
	if input.OSName != "" {
		item.OSName = input.OSName
	}
	if input.OSVersion != "" {
		item.OSVersion = input.OSVersion
	}
	if input.PublicKey != "" {
		item.PublicKey = input.PublicKey
	}
	if input.DeviceVersion != "" {
		item.DeviceVersion = input.DeviceVersion
	}
	if input.CountryCode != "" {
		item.CountryCode = input.CountryCode
	}
	if item.UserID == "" {
		return CompleteDeviceLoginDeviceView{}, ErrInvalidArgument
	}
	now := nowTime.Unix()
	device := model.Device{
		DeviceID:      item.DeviceID,
		OwnerID:       item.UserID,
		Name:          item.Name,
		Platform:      item.Platform,
		Alias:         item.Alias,
		OSName:        item.OSName,
		OSVersion:     item.OSVersion,
		PublicKey:     item.PublicKey,
		DeviceVersion: item.DeviceVersion,
		CountryCode:   item.CountryCode,
		Status:        "active",
		LastSeenAt:    now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if existing, ok, err := s.Devices.GetDevice(ctx, item.DeviceID); err != nil {
		return CompleteDeviceLoginDeviceView{}, err
	} else if ok {
		device = existing
		device.OwnerID = item.UserID
		device.Name = item.Name
		device.Platform = item.Platform
		device.Alias = item.Alias
		device.OSName = item.OSName
		device.OSVersion = item.OSVersion
		device.PublicKey = item.PublicKey
		device.DeviceVersion = item.DeviceVersion
		device.CountryCode = item.CountryCode
		device.Status = "active"
		device.LastSeenAt = now
		device.UpdatedAt = now
	}
	if strings.TrimSpace(device.VirtualIP) == "" {
		device.VirtualIP = allocatedDeviceVirtualIP(newDeviceVirtualIPID(s.Devices))
	}
	if err := s.Devices.SaveDevice(ctx, device); err != nil {
		return CompleteDeviceLoginDeviceView{}, err
	}
	item.Status = "completed"
	item.UpdatedAt = now
	if err := s.Devices.SaveDeviceLoginDevice(ctx, item); err != nil {
		return CompleteDeviceLoginDeviceView{}, err
	}
	networks, err := s.Networks.ListNetworksByDevice(ctx, device.DeviceID)
	if err != nil {
		return CompleteDeviceLoginDeviceView{}, err
	}
	activeNetworkID := ""
	if len(networks) > 0 {
		activeNetworkID = networks[0].NetworkID
	}
	event := DeviceControlEnvelope{
		Type:      "device_user_login_succeeded",
		MessageID: fmt.Sprintf("devlogin%d%s", nowTime.UnixMilli(), device.DeviceID),
		Payload: map[string]any{
			"accessToken":     userSession.AccessToken,
			"refreshToken":    userSession.RefreshToken,
			"userId":          user.UserID,
			"userLabel":       user.Email,
			"deviceId":        device.DeviceID,
			"activeNetworkId": activeNetworkID,
			"virtualIp":       device.VirtualIP,
			"expiresIn":       max(userSession.ExpiresAt-now, 0),
		},
	}
	if s.DevicePublisher != nil {
		if err := s.DevicePublisher.PublishDeviceControl(ctx, device.DeviceID, event); err != nil {
			return CompleteDeviceLoginDeviceView{}, err
		}
	}
	if activeNetworkID != "" {
		member, ok, err := s.Networks.GetNetworkDevice(ctx, activeNetworkID, device.DeviceID)
		if err != nil {
			return CompleteDeviceLoginDeviceView{}, err
		}
		if ok {
			version, err := bumpNetworkConfigVersion(ctx, s.Networks, s.EventPublisher, s.Now, activeNetworkID, "device_login_member_attached")
			if err != nil {
				return CompleteDeviceLoginDeviceView{}, err
			}
			if err := publishNetworkMemberChanged(ctx, s.EventPublisher, s.Now, activeNetworkID, device.DeviceID, "added", member, version.Version, version.Reason); err != nil {
				return CompleteDeviceLoginDeviceView{}, err
			}
			if err := publishNetworkSnapshot(ctx, s.Users, s.Devices, s.Networks, nil, s.EventPublisher, s.Now, activeNetworkID, version.Version, version.Reason); err != nil {
				return CompleteDeviceLoginDeviceView{}, err
			}
		}
	}
	return completedDeviceLoginView(item), nil
}
