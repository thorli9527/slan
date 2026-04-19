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
	devices.POST("/register", func(c *gin.Context) {
		var req dto.RegisterDeviceRequest
		if !bindJSON(c, &req) {
			return
		}
		resp, err := deps.Device.Register(userID(c), req)
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusCreated, resp)
	})
	// GET /devices 返回当前用户拥有的设备列表。
	devices.GET("", func(c *gin.Context) {
		items, err := deps.Device.ListByUser(userID(c))
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": items})
	})

	nodes := protected.Group("/nodes")
	// POST /nodes/register 为当前用户某台设备注册一个通信节点。
	nodes.POST("/register", func(c *gin.Context) {
		var req dto.RegisterNodeRequest
		if !bindJSON(c, &req) {
			return
		}
		resp, err := deps.Node.Register(userID(c), req)
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusCreated, resp)
	})
}
