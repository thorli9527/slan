package service

import "github.com/slan/server/server-biz/api/dto"

// Device defines device registration and user-scoped device management.
type Device interface {
	Register(userID string, req dto.RegisterDeviceRequest) (dto.Device, error)
	ListByUser(userID string) ([]dto.Device, error)
	SetDeviceNetworkState(userID, deviceID, networkID string, req dto.DeviceNetworkStateRequest) (dto.DeviceNetworkState, error)
	MarkMQTTReachable(deviceID string) error
}

// Node defines node registration.
type Node interface {
	Register(userID string, req dto.RegisterNodeRequest) (dto.Node, error)
}
