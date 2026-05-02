package httpapi

import (
	"expvar"

	"github.com/gin-gonic/gin"
	"github.com/slan/server/server-biz/configs"
	"github.com/slan/server/server-biz/internal/service"
)

// routerDeps 聚合路由层需要显式注入的全部 service 依赖。
type routerDeps struct {
	Config configs.Config
	// Auth 提供注册和登录能力。
	Auth service.Auth
	// Device 提供设备注册和查询能力。
	Device service.Device
	// Network 提供网络、子网和挂载编排能力。
	Network service.Network
	// Node 提供节点注册能力。
	Node service.Node
	// Bootstrap 提供启动配置和 relay ticket 能力。
	Bootstrap service.Bootstrap
	// Tokens 提供 Bearer Token 校验能力。
	Tokens service.TokenVerifier
	// ControlChannel 提供控制面握手、地图和状态上报能力。
	ControlChannel service.ControlChannel
	// ControlSync 提供跨实例控制事件同步能力。
	ControlSync service.ControlSync
	// MessageDelivery 提供下行消息投递、重试和归档能力。
	MessageDelivery service.MessageDelivery
	// Ops 提供运营管理视图和 RBAC 管理能力。
	Ops service.Ops
}

func NewRouterDeps(
	auth service.Auth,
	device service.Device,
	network service.Network,
	node service.Node,
	bootstrap service.Bootstrap,
	tokens service.TokenVerifier,
	controlChannel service.ControlChannel,
	controlSync service.ControlSync,
	messageDelivery service.MessageDelivery,
	ops service.Ops,
) routerDeps {
	return routerDeps{
		Auth:            auth,
		Device:          device,
		Network:         network,
		Node:            node,
		Bootstrap:       bootstrap,
		Tokens:          tokens,
		ControlChannel:  controlChannel,
		ControlSync:     controlSync,
		MessageDelivery: messageDelivery,
		Ops:             ops,
	}
}

// NewPublicRouter 构建对外客户使用的 HTTP 路由。
func NewPublicRouter(cfg configs.Config, deps routerDeps) *gin.Engine {
	deps.Config = cfg
	if deps.Auth == nil || deps.Device == nil || deps.Network == nil || deps.Node == nil || deps.Bootstrap == nil || deps.Tokens == nil || deps.ControlChannel == nil {
		panic("http public router requires explicit services")
	}
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery(), allowCORS())
	installPublicErrorHandlers(router)
	router.Use(limitRequestBody(maxHTTPJSONBodyBytes))
	startControlSync(deps)
	startControlMQTT(deps)
	startRelayHeartbeatMQTT(deps)

	router.GET("/healthz", healthz)
	router.GET("/debug/vars", gin.WrapH(expvar.Handler()))

	api := router.Group("")
	registerBusinessRoutes(api, deps)

	return router
}

// NewOpsRouter 构建运营管理 HTTP 路由。
func NewOpsRouter(cfg configs.Config, deps routerDeps) *gin.Engine {
	deps.Config = cfg
	if deps.Ops == nil {
		panic("http ops router requires ops service")
	}
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery(), allowCORS())
	router.Use(limitRequestBody(maxHTTPJSONBodyBytes))

	router.GET("/healthz", healthz)
	router.GET("/debug/vars", gin.WrapH(expvar.Handler()))

	api := router.Group("")
	registerOpsRoutes(api, cfg, deps)

	return router
}
