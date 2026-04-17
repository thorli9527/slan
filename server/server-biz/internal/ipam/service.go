package ipam

// Allocator 定义子网范围内的虚拟 IP 分配能力。
// 注意：IP 分配绑定的是子网挂载关系，而不是设备实体本身。
type Allocator interface {
	// Allocate 分配子网内下一个可用虚拟 IP。
	Allocate(networkID, subnetID, attachmentID, deviceID string) (string, error)
	// Reserve 在请求静态保留时分配指定虚拟 IP。
	Reserve(networkID, subnetID, attachmentID, deviceID, ip string) (string, error)
	// Release 释放指定挂载关系占用的 IP。
	Release(attachmentID string) error
}
