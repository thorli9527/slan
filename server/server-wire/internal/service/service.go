package service

import (
	"context"
	"errors"

	"github.com/slan/server/server-wire/internal/model"
	"github.com/slan/server/server-wire/internal/store"
)

// Service 聚合 server-wire 的业务逻辑：peer 注册、路径健康更新、路径规划和票据签发。
type Service struct {
	store store.Store
	biz   BizAuthorizer
}

var (
	ErrNoHealthyRelayNodes = errors.New("relay has no schedulable enabled healthy nodes")
	ErrNoHealthyDerpNodes  = errors.New("derp has no schedulable enabled healthy nodes")
)

// BizAuthorizer 定义 server-wire 向 service-biz 查询授权、拓扑和节点调度数据的边界。
type BizAuthorizer interface {
	// Enabled 表示该授权源是否启用。
	Enabled() bool
	// PeerAuthz 查询 peer 是否允许接入及其网络身份。
	PeerAuthz(ctx context.Context, peerID string) (model.PeerAuthzView, error)
	// PeerRuntimeConfig 查询 peer 的运行配置。
	PeerRuntimeConfig(ctx context.Context, peerID string) (model.PeerRuntimeConfigView, error)
	// NetworkTopology 查询虚拟网络拓扑。
	NetworkTopology(ctx context.Context, networkID string) (model.NetworkTopologyView, error)
	// DerpMap 查询 DERP 节点地图。
	DerpMap(ctx context.Context) (model.DerpMap, error)
	// RelayNodes 查询可调度 UDP relay 节点。
	RelayNodes(ctx context.Context) ([]model.RelayNode, error)
}

// New 使用指定 Store 创建不连接 biz 的 server-wire 服务。
func New(st store.Store) *Service {
	return &Service{store: st}
}

// NewWithBiz 使用指定 Store 和 biz 授权源创建 server-wire 服务。
func NewWithBiz(st store.Store, biz BizAuthorizer) *Service {
	return &Service{store: st, biz: biz}
}
