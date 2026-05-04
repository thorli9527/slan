package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/slan/server/server-biz/api/dto"
)

// registerBusinessRoutes 装配对外客户可见的全部业务 HTTP 路由。
//
// 路由划分与 service 层职责保持一致：
// - access
// - registration
// - network
// - bootstrap/control
func registerBusinessRoutes(api *gin.RouterGroup, deps routerDeps) {
	// 对外业务接口总入口（Public Router）。
	//
	// 约定：
	// - 除明确标注为“无需鉴权”的接口外，其余接口都要求在 Header 中携带：
	//   - Authorization: Bearer <accessToken>
	// - 鉴权失败统一返回：401 UNAUTHORIZED（错误码见 error_response.go / service.ErrUnauthorized）
	// - JSON 请求体大小限制为 1 MiB（见 routes_common.go: limitRequestBody）
	//
	// 分组说明：
	// - /system：不需要登录即可读取的公共配置（用于 UI 决定是否展示注册入口等）
	// - /auth：账号注册/登录/刷新、浏览器登录回调状态、MQTT 鉴权回调等
	// - /devices /nodes：设备与节点注册、运行态上报
	// - /networks：网络/子网/挂载/成员关系/激活编排
	// - /bootstrap /control：客户端启动载荷、控制会话刷新、relay ticket
	api.GET("/system/public-config", respondWithJSON(http.StatusOK, func(c *gin.Context) (dto.PublicSystemConfig, error) {
		return dto.PublicSystemConfig{AllowRegistration: deps.Config.Auth.AllowRegistration}, nil
	}))
	registerAccessRoutes(api, deps)

	protected := api.Group("")
	protected.Use(authenticate(deps))

	registerProtectedAccessRoutes(protected, deps)
	registerRegistrationRoutes(protected, deps)
	registerNetworkRoutes(protected, deps)
	registerBootstrapRoutes(protected, deps)
}
