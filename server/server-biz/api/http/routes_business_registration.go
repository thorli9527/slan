package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/slan/server/server-biz/api/dto"
)

// registerRegistrationRoutes 注册设备与节点生命周期入口。
func registerRegistrationRoutes(protected *gin.RouterGroup, deps routerDeps) {
	devices := protected.Group("/devices")
	// POST /devices/register 为当前用户注册一台设备。
	devices.POST("/register", respondWithBody(http.StatusCreated, func(c *gin.Context, req dto.RegisterDeviceRequest) (dto.Device, error) {
		rc := currentRouteContext(c)
		return deps.Device.Register(rc.user(), req)
	}))
	// GET /devices 返回当前用户拥有的设备列表。
	devices.GET("", respondWithItems(func(c *gin.Context) ([]dto.Device, error) {
		rc := currentRouteContext(c)
		return deps.Device.ListByUser(rc.user())
	}))

	nodes := protected.Group("/nodes")
	// POST /nodes/register 为当前用户某台设备注册一个通信节点。
	nodes.POST("/register", respondWithBody(http.StatusCreated, func(c *gin.Context, req dto.RegisterNodeRequest) (dto.Node, error) {
		rc := currentRouteContext(c)
		return deps.Node.Register(rc.user(), req)
	}))
}
