package biz

import "strings"

// DeviceService 承载设备注册、会话绑定、续租和可见性相关业务实现。
type DeviceService struct {
	store BusinessStore
}

func (s DeviceService) RegisterDevice(accessToken string, req RegisterDeviceRequest) (Device, NetworkDevice, error) {
	if strings.TrimSpace(accessToken) == "" {
		return Device{}, NetworkDevice{}, errUnauthorized
	}
	normalized, err := normalizeDeviceIdentity(req.DeviceIdentityRequest)
	if err != nil {
		return Device{}, NetworkDevice{}, err
	}
	auth, err := s.store.AuthByToken(accessToken)
	if err != nil {
		return Device{}, NetworkDevice{}, err
	}
	return s.store.RegisterDevice(auth.User.UserID, normalized.DeviceID, normalized.Name, normalized.Platform, normalized.OSName, normalized.OSVersion, normalized.Alias, normalized.PublicKey)
}

func (s DeviceService) RenewDevice(accessToken, deviceID string, req RenewDeviceRequest) (Device, []NetworkConfig, error) {
	if strings.TrimSpace(accessToken) == "" {
		return Device{}, nil, errUnauthorized
	}
	auth, err := s.store.AuthByToken(accessToken)
	if err != nil {
		return Device{}, nil, err
	}
	return s.store.RenewDevice(deviceID, auth.User.UserID, req.NetworkEnabled, req.RxBytesTotal, req.TxBytesTotal)
}

func (s DeviceService) BootstrapDeviceSession(req DeviceSessionBootstrapRequest) (Device, DeviceSession, []NetworkConfig, error) {
	normalized, err := normalizeDeviceIdentity(req.DeviceIdentityRequest)
	if err != nil {
		return Device{}, DeviceSession{}, nil, err
	}
	return s.store.BootstrapDeviceSession(req.SessionKey, normalized.DeviceID, normalized.Name, normalized.Platform, normalized.OSName, normalized.OSVersion, normalized.Alias, normalized.PublicKey)
}

func (s DeviceService) BindDeviceSession(accessToken string, req DeviceSessionBindRequest) (Device, DeviceSession, []NetworkConfig, error) {
	normalized, err := normalizeDeviceIdentity(req)
	if err != nil {
		return Device{}, DeviceSession{}, nil, err
	}
	return s.store.BindDeviceSession(accessToken, normalized.DeviceID, normalized.Name, normalized.Platform, normalized.OSName, normalized.OSVersion, normalized.Alias, normalized.PublicKey)
}

func (s DeviceService) RenewDeviceSession(deviceToken string, req DeviceRuntimeCountersRequest) (Device, DeviceSession, []NetworkConfig, error) {
	return s.store.RenewDeviceSession(deviceToken, req.NetworkEnabled, req.RxBytesTotal, req.TxBytesTotal)
}

func (s DeviceService) ListDevices(userID string) []Device {
	return s.store.ListDevices(userID)
}

func (s DeviceService) ListVisibleDevices(userID string) []Device {
	return s.store.ListVisibleDevices(userID)
}

func (s DeviceService) UpdateAlias(deviceID string, req UpdateDeviceAliasRequest) (Device, error) {
	return s.store.UpdateDeviceAlias(deviceID, req.ActorUserID, req.Alias)
}

func (s DeviceService) RemoveVisibleDevice(deviceID, actorUserID string) error {
	return s.store.RemoveVisibleDevice(deviceID, actorUserID)
}

func (s DeviceService) NetworkConfigsForDevice(deviceID string) ([]NetworkConfig, error) {
	return s.store.NetworkConfigsForDevice(deviceID)
}
