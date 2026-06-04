package biz

// DeviceService 承载设备注册、会话绑定、续租和可见性相关业务实现。
type DeviceService struct {
	store BusinessStore
}

func (s DeviceService) RegisterDevice(req RegisterDeviceRequest) (Device, NetworkDevice, error) {
	return s.store.RegisterDevice(req.UserID, req.DeviceID, req.Name, req.Platform, req.OSName, req.OSVersion, req.Alias, req.PublicKey)
}

func (s DeviceService) RenewDevice(deviceID string, req RenewDeviceRequest) (Device, []NetworkConfig, error) {
	return s.store.RenewDevice(deviceID, req.UserID, req.NetworkEnabled, req.RxBytesTotal, req.TxBytesTotal)
}

func (s DeviceService) BootstrapDeviceSession(req DeviceSessionBootstrapRequest) (Device, DeviceSession, []NetworkConfig, error) {
	return s.store.BootstrapDeviceSession(req.SessionKey, req.DeviceID, req.Name, req.Platform, req.OSName, req.OSVersion, req.Alias, req.PublicKey)
}

func (s DeviceService) BindDeviceSession(accessToken string, req DeviceSessionBindRequest) (Device, DeviceSession, []NetworkConfig, error) {
	return s.store.BindDeviceSession(accessToken, req.DeviceID, req.Name, req.Platform, req.OSName, req.OSVersion, req.Alias, req.PublicKey)
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
