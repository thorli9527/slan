package service

import (
	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/auth"
	"github.com/slan/server/server-biz/internal/control"
	"github.com/slan/server/server-biz/internal/device"
	"github.com/slan/server/server-biz/internal/network"
	"github.com/slan/server/server-biz/internal/node"
	controlws "github.com/slan/server/server-biz/internal/ws"
)

// TokenVerifier 抽象了控制面对访问令牌的校验能力。
type TokenVerifier interface {
	// Authenticate 校验访问令牌并返回对应用户 ID。
	Authenticate(accessToken string) (string, error)
}

// ControlChannel 抽象了控制通道建立和网络地图读取能力。
type ControlChannel interface {
	// Handshake 校验 node hello 并返回会话确认和初始网络地图。
	Handshake(hello controlws.NodeHello) (controlws.NodeHelloAck, dto.NetworkMap, error)
	// NetworkMap 返回指定网络的当前完整网络地图。
	NetworkMap(userID, nodeID, networkID string) (dto.NetworkMap, error)
	// ReportEndpoints 持久化节点上报端点并返回更新后的网络地图。
	ReportEndpoints(userID string, report controlws.EndpointReport) (dto.NetworkMap, error)
	// ReportConnectionState 持久化节点连接状态。
	ReportConnectionState(userID, nodeID string, state controlws.ConnectionState) error
	// Disconnect 持久化断开状态并回写设备在线状态。
	Disconnect(userID, nodeID string, notice controlws.DisconnectNotice) error
	// CloseSession 在控制通道异常断开时清理在线状态。
	CloseSession(userID, nodeID, networkID string) error
	// PeerSnapshot 返回网络内指定 peer 的当前快照。
	PeerSnapshot(userID, nodeID, networkID, peerNodeID string) (dto.Peer, error)
	// ConnectPlan 返回当前节点面向指定 peer 的连接计划。
	ConnectPlan(userID, nodeID, networkID, peerNodeID string) (controlws.ConnectPlan, error)
}

// ControlSync 抽象了多实例控制通道事件同步。
type ControlSync interface {
	// Publish 发布控制通道同步事件。
	Publish(event controlws.ControlSyncEvent) error
	// Subscribe 订阅控制通道同步事件。
	Subscribe(handler func(controlws.ControlSyncEvent)) error
	// NextRevision 递增并返回网络修订号。
	NextRevision(networkID string) (uint64, error)
	// CurrentRevision 返回当前网络修订号。
	CurrentRevision(networkID string) (uint64, error)
}

// Services 聚合了 HTTP 路由层所需的全部服务接口。
//
// 这样路由层只依赖统一容器，不需要分别感知具体实现来源。
type Services struct {
	// Auth 处理用户注册和登录。
	Auth auth.Service
	// Device 处理设备注册和查询。
	Device device.Service
	// Network 处理网络、子网、成员关系和挂载关系。
	Network network.Service
	// Node 处理节点注册。
	Node node.Service
	// Bootstrap 提供启动配置和中继回退票据。
	Bootstrap control.BootstrapService
	// Tokens 负责校验受保护接口使用的 Bearer Token。
	Tokens TokenVerifier
	// ControlChannel 负责 WebSocket 控制通道的建链与地图读取。
	ControlChannel ControlChannel
	// ControlSync 负责多实例间的控制通道事件同步。
	ControlSync ControlSync
}
