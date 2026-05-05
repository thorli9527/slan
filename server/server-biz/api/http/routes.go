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
	// Ice 提供 ICE Server、candidate 和 punch plan 控制面能力。
	Ice service.Ice
	// Wire 提供给 server-wire 的内部授权与拓扑只读能力。
	Wire service.Wire
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
	ice service.Ice,
	wire service.Wire,
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
		Ice:             ice,
		Wire:            wire,
		Ops:             ops,
	}
}

// NewPublicRouter 构建对外客户使用的 HTTP 路由。
func NewPublicRouter(cfg configs.Config, deps routerDeps) *gin.Engine {
	deps.Config = cfg
	if deps.Auth == nil || deps.Device == nil || deps.Network == nil || deps.Node == nil || deps.Bootstrap == nil || deps.Tokens == nil || deps.ControlChannel == nil || deps.Ice == nil || deps.Wire == nil {
		panic("http public router requires explicit services")
	}
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery(), allowCORS())
	installPublicErrorHandlers(router)
	router.Use(limitRequestBody(maxHTTPJSONBodyBytes))
	startControlSync(deps)
	startControlMQTT(deps)
	startRelayHeartbeatMQTT(deps)
	startRelayDataPlanePolicyController(deps)

	// GET /healthz
	//
	// 健康检查（无需鉴权）。
	//
	// 用途：
	// - 供负载均衡器、K8S、探活脚本判断进程存活
	router.GET("/healthz", healthz)
	// GET /debug/vars
	//
	// expvar 运行时指标输出（无需鉴权）。
	//
	// 注意：
	// - 当前对外开放，生产环境如有安全要求应在网关层做访问控制
	router.GET("/debug/vars", gin.WrapH(expvar.Handler()))

	api := router.Group("")
	registerBusinessRoutes(api, deps)

	return router
}

// NewOpsRouter 构建运营管理 HTTP 路由。
func NewOpsRouter(cfg configs.Config, deps routerDeps) *gin.Engine {
	deps.Config = cfg
	if deps.Ops == nil || deps.Ice == nil {
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
