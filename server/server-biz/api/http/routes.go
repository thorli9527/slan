package httpapi

import (
	"errors"
	"expvar"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/configs"
	"github.com/slan/server/server-biz/internal/service"
)

// routerDeps 聚合路由层需要显式注入的全部 service 依赖。
type routerDeps struct {
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
	ops service.Ops,
) routerDeps {
	return routerDeps{
		Auth:           auth,
		Device:         device,
		Network:        network,
		Node:           node,
		Bootstrap:      bootstrap,
		Tokens:         tokens,
		ControlChannel: controlChannel,
		ControlSync:    controlSync,
		Ops:            ops,
	}
}

// userIDContextKey 是 gin.Context 中保存当前认证用户 ID 的键。
const userIDContextKey = "userId"

// NewPublicRouter 构建对外客户使用的 HTTP 路由。
func NewPublicRouter(cfg configs.Config, deps routerDeps) *gin.Engine {
	if deps.Auth == nil || deps.Device == nil || deps.Network == nil || deps.Node == nil || deps.Bootstrap == nil || deps.Tokens == nil || deps.ControlChannel == nil {
		panic("http public router requires explicit services")
	}
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	registerControlWS(router, cfg.WS.Path, deps)

	router.GET("/healthz", healthz)
	router.GET("/debug/vars", gin.WrapH(expvar.Handler()))

	api := router.Group("")
	registerBusinessRoutes(api, deps)

	return router
}

// NewOpsRouter 构建运营管理 HTTP 路由。
func NewOpsRouter(cfg configs.Config, deps routerDeps) *gin.Engine {
	if deps.Ops == nil {
		panic("http ops router requires ops service")
	}
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	router.GET("/healthz", healthz)
	router.GET("/debug/vars", gin.WrapH(expvar.Handler()))

	api := router.Group("")
	registerOpsRoutes(api, cfg, deps)

	return router
}

// NewRouter 保留为兼容入口，当前返回 public router。
func NewRouter(cfg configs.Config, deps routerDeps) *gin.Engine {
	return NewPublicRouter(cfg, deps)
}

// healthz 返回轻量级就绪探针响应。
func healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// authenticate 是受保护接口使用的 Bearer Token 鉴权中间件。
//
// 处理逻辑：
// 1. 读取 Authorization 头
// 2. 解析 Bearer Token
// 3. 调用 Tokens 服务校验
// 4. 将 userID 写入 gin context 供后续 handler 使用
func authenticate(deps routerDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		authz := strings.TrimSpace(c.GetHeader("Authorization"))
		token := strings.TrimPrefix(authz, "Bearer ")
		if token == authz || token == "" {
			writeError(c, service.ErrUnauthorized)
			c.Abort()
			return
		}

		userID, err := deps.Tokens.Authenticate(token)
		if err != nil {
			writeError(c, err)
			c.Abort()
			return
		}

		c.Set(userIDContextKey, userID)
		c.Next()
	}
}

func authenticateOps(cfg configs.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		authz := strings.TrimSpace(c.GetHeader("Authorization"))
		token := strings.TrimPrefix(authz, "Bearer ")
		if token == authz || token == "" || token != cfg.Ops.AccessToken {
			writeError(c, service.ErrUnauthorized)
			c.Abort()
			return
		}
		c.Next()
	}
}

// bindJSON 负责统一绑定并校验 JSON 请求体。
//
// 当绑定失败时，直接按控制面约定输出 INVALID_ARGUMENT 错误。
func bindJSON(c *gin.Context, out any) bool {
	if err := c.ShouldBindJSON(out); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{
			Code:    "INVALID_ARGUMENT",
			Message: err.Error(),
		})
		return false
	}
	return true
}

// userID 从 gin context 中取出当前认证用户 ID。
func userID(c *gin.Context) string {
	value, _ := c.Get(userIDContextKey)
	userID, _ := value.(string)
	return userID
}

// writeError 将领域错误统一映射为 HTTP 状态码和标准错误码。
//
// 这样路由层无需在每个 handler 中重复写错误分发逻辑。
func writeError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	code := "INTERNAL"
	switch {
	case errors.Is(err, service.ErrInvalidArgument):
		status = http.StatusBadRequest
		code = "INVALID_ARGUMENT"
	case errors.Is(err, service.ErrUnauthorized):
		status = http.StatusUnauthorized
		code = "UNAUTHORIZED"
	case errors.Is(err, service.ErrForbidden):
		status = http.StatusForbidden
		code = "FORBIDDEN"
	case errors.Is(err, service.ErrNotFound):
		status = http.StatusNotFound
		code = "NOT_FOUND"
	case errors.Is(err, service.ErrConflict):
		status = http.StatusConflict
		code = "CONFLICT"
	}

	c.JSON(status, dto.ErrorResponse{
		Code:    code,
		Message: err.Error(),
	})
}
