package punch

import "time"

// Repository 定义打洞服务 UDP/HTTP 入口依赖的状态边界。
// Store 是当前默认的内存实现，后续可替换为持久化或分布式实现。
type Repository interface {
	// PutEndpoint 写入或刷新某个网络节点的最新端点观测值。
	PutEndpoint(endpoint Endpoint) Endpoint
	// Endpoint 查询某个网络节点的有效端点，过期记录不应暴露给调用方。
	Endpoint(networkID, nodeID string) (Endpoint, bool)
	// CreateSession 创建一次临时直连协商会话，并在端点已知时快照双方地址。
	CreateSession(networkID, requesterNodeID, peerNodeID string, ttl time.Duration) ConnectSession
	// Session 按会话 ID 查询有效的直连协商会话。
	Session(sessionID string) (ConnectSession, bool)
	// Stats 返回管理接口使用的轻量运行统计。
	Stats() Stats
}

var _ Repository = (*Store)(nil)
