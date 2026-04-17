package node

import "github.com/slan/server/server-biz/api/dto"

// Service 定义节点注册能力。
type Service interface {
	// Register 将业务设备绑定为一个可参与控制面和数据面的通信节点。
	Register(userID string, req dto.RegisterNodeRequest) (dto.Node, error)
}
