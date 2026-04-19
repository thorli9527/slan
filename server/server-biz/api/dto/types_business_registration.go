package dto

// RegisterDeviceRequest 用于将当前客户端安装实例绑定到用户。
type RegisterDeviceRequest struct {
	// Name 是用户可见的设备名称。
	Name string `json:"name"`
	// Platform 是客户端操作系统，例如 macos、windows 或 linux。
	Platform string `json:"platform"`
	// MachineID 是本地持久化的机器标识。
	MachineID string `json:"machineId"`
	// PublicKey 是设备隧道使用的公钥。
	PublicKey string `json:"publicKey"`
}

// Device 描述设备实体本身。
// 注意：虚拟 IP 不再存储在设备上，IP 归属于子网挂载关系。
type Device struct {
	// DeviceID 是由控制面管理的唯一设备标识。
	DeviceID string `json:"deviceId"`
	// Name 是当前用户可见的设备名称。
	Name string `json:"name"`
	// Platform 是设备操作系统。
	Platform string `json:"platform"`
	// Status 是设备在线状态的粗粒度快照。
	Status string `json:"status"`
	// PublicKey 是用于建立隧道时对外公布的公钥。
	PublicKey string `json:"publicKey,omitempty"`
	// NetworkIDs 列出当前与设备关联的网络 ID。
	NetworkIDs []string `json:"networkIds,omitempty"`
}

// RegisterNodeRequest 用于注册节点身份。
type RegisterNodeRequest struct {
	// DeviceID 是业务设备 ID。
	DeviceID string `json:"deviceId"`
	// NodeID 是通信节点 ID。
	NodeID string `json:"nodeId"`
	// NodePublicKey 是节点公钥。
	NodePublicKey string `json:"nodePublicKey"`
	// Capabilities 是节点能力声明，例如 relay、exit-node、subnet-router。
	Capabilities []string `json:"capabilities,omitempty"`
}

// Node 描述一个可参与组网的通信节点。
type Node struct {
	// NodeID 是节点唯一标识。
	NodeID string `json:"nodeId"`
	// DeviceID 是关联的业务设备 ID。
	DeviceID string `json:"deviceId"`
	// NodePublicKey 是节点公钥。
	NodePublicKey string `json:"nodePublicKey"`
	// NetworkIDs 是当前节点可见或已加入的网络列表。
	NetworkIDs []string `json:"networkIds,omitempty"`
	// Capabilities 是节点能力集合。
	Capabilities []string `json:"capabilities,omitempty"`
}
