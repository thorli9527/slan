package service

import "github.com/slan/server/server-biz/api/dto"

// Device 定义设备注册和用户维度设备查询能力。
type Device interface {
	// Register 为指定用户注册一台新设备，并返回设备视图。
	Register(userID string, req dto.RegisterDeviceRequest) (dto.Device, error)
	// ListByUser 返回当前用户拥有的全部设备。
	ListByUser(userID string) ([]dto.Device, error)
}

// Node 定义节点注册能力。
type Node interface {
	// Register 为指定用户的设备注册一个逻辑节点，并返回节点视图。
	Register(userID string, req dto.RegisterNodeRequest) (dto.Node, error)
}
