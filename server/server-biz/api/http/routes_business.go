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
