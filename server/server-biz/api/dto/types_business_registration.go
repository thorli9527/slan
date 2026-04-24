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
	// OwnerEmail 是设备所属用户邮箱。
	OwnerEmail string `json:"ownerEmail,omitempty"`
	// Platform 是设备操作系统。
	Platform string `json:"platform"`
	// MachineID 是本地持久化机器标识。
	MachineID string `json:"machineId,omitempty"`
	// Status 是设备在线状态的粗粒度快照。
	Status string `json:"status"`
	// CurrentVirtualIP 是当前活动网络下分配给设备的虚拟 IP。
	CurrentVirtualIP string `json:"currentVirtualIp,omitempty"`
	// LinkStatus 是当前链路状态，例如 connected / disconnected / online / offline。
	LinkStatus string `json:"linkStatus,omitempty"`
	// ConnectivityProtocol 是当前链路走的协议或路径，例如 direct / relay / derp。
	ConnectivityProtocol string `json:"connectivityProtocol,omitempty"`
	// JoinedAt 是当前活动网络下加入时间，使用 unix 秒时间戳。
	JoinedAt int64 `json:"joinedAt,omitempty"`
	// MembershipStatus 是设备在当前网络中的加入状态，例如 pending / active。
	MembershipStatus string `json:"membershipStatus,omitempty"`
	// NetworkRole 是设备在当前网络中的角色，例如 owner / member。
	NetworkRole string `json:"networkRole,omitempty"`
	// CreatedAt 是设备创建时间，使用 unix 秒时间戳。
	CreatedAt int64 `json:"createdAt,omitempty"`
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
