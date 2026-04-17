package device

import "github.com/slan/server/server-biz/api/dto"

// Service 定义设备注册和用户维度设备查询能力。
type Service interface {
	// Register 将当前安装实例绑定到指定用户账号。
	Register(userID string, req dto.RegisterDeviceRequest) (dto.Device, error)
	// ListByUser 返回该用户拥有的全部设备。
	ListByUser(userID string) ([]dto.Device, error)
}
