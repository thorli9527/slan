package httpapi

import "github.com/gin-gonic/gin"

// registerBusinessRoutes 装配对外客户可见的全部业务 HTTP 路由。
//
// 路由划分与 service 层职责保持一致：
// - access
// - registration
// - network
// - bootstrap/control
func registerBusinessRoutes(api *gin.RouterGroup, deps routerDeps) {
	registerAccessRoutes(api, deps)

	protected := api.Group("")
	protected.Use(authenticate(deps))

	registerProtectedAccessRoutes(protected, deps)
	registerRegistrationRoutes(protected, deps)
	registerNetworkRoutes(protected, deps)
	registerCommerceRoutes(protected, deps)
	registerBootstrapRoutes(protected, deps)
}
