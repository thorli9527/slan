package biz

// MQTTControlService 承载 MQTT 控制消息投递、重试和网络广播所需的业务查询。
type MQTTControlService struct {
	store BusinessStore
}

func (s MQTTControlService) Device(deviceID string) (Device, error) {
	return s.store.GetDevice(deviceID)
}

func (s MQTTControlService) HasActiveNetworkDevice(networkID, deviceID string) bool {
	return s.store.HasActiveNetworkDevice(networkID, deviceID)
}

func (s MQTTControlService) NextNetworkConfigVersion(networkID string, now int64) int64 {
	return s.store.NextNetworkConfigVersion(networkID, now)
}

func (s MQTTControlService) ListActiveNetworkDeviceIDs(networkID string) []string {
	return s.store.ListActiveNetworkDeviceIDs(networkID)
}

func (s MQTTControlService) NetworkConfig(networkID, deviceID string) (NetworkConfig, error) {
	return s.store.NetworkConfig(networkID, deviceID)
}

func (s MQTTControlService) ListNetworkDevices(networkID string) []NetworkDevice {
	return s.store.ListNetworkDevices(networkID)
}

func (s MQTTControlService) ReportDeviceRuntime(deviceID string, networkEnabled bool, rxBytesTotal, txBytesTotal uint64) DeviceRuntimeReportResult {
	return s.store.ReportDeviceRuntime(deviceID, networkEnabled, rxBytesTotal, txBytesTotal)
}

func (s MQTTControlService) ReportDeviceEndpoint(networkID, deviceID string, endpoints []DeviceEndpoint) (bool, error) {
	return s.store.ReportDeviceEndpoint(networkID, deviceID, endpoints)
}

func (s MQTTControlService) PrepareDelivery(deviceID, deliveryID, messageType, action string, payload any, createdAt, expiresAt int64) (MQTTControlDelivery, error) {
	return s.store.PrepareMQTTControlDelivery(deviceID, deliveryID, messageType, action, payload, createdAt, expiresAt)
}

func (s MQTTControlService) DeliveryForDevice(deviceID, deliveryID string) (MQTTControlDelivery, error) {
	return s.store.GetMQTTControlDeliveryForDevice(deviceID, deliveryID)
}

func (s MQTTControlService) RecordAck(deviceID, deliveryID, taskID, action, status, errorText string, processedAtMs int64, now int64) (MQTTControlDelivery, error) {
	return s.store.RecordMQTTControlAck(deviceID, deliveryID, taskID, action, status, errorText, processedAtMs, now)
}

func (s MQTTControlService) RecordPublishResult(deviceID, deliveryID string, published bool, errorText string, now int64) (MQTTControlDelivery, error) {
	return s.store.RecordMQTTControlPublishResult(deviceID, deliveryID, published, errorText, now)
}

func (s MQTTControlService) ExpireDeliveries(now int64) int {
	return s.store.ExpireMQTTControlDeliveries(now)
}

func (s MQTTControlService) RetryableDeliveries(now int64, limit int) []MQTTControlDelivery {
	return s.store.ListRetryableMQTTControlDeliveries(now, limit)
}

func (s MQTTControlService) ClaimRetry(deviceID, deliveryID string, previousUpdatedAt, now int64) (bool, error) {
	return s.store.ClaimMQTTControlDeliveryRetry(deviceID, deliveryID, previousUpdatedAt, now)
}
