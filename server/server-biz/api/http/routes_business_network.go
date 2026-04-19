package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/slan/server/server-biz/api/dto"
)

// registerNetworkRoutes 注册逻辑网络、子网和挂载关系相关入口。
func registerNetworkRoutes(protected *gin.RouterGroup, deps routerDeps) {
	networks := protected.Group("/networks")
	// GET /networks 返回当前用户可见的网络列表。
	networks.GET("", func(c *gin.Context) {
		items, err := deps.Network.List(userID(c))
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": items})
	})
	// POST /networks 创建一个新的逻辑网络。
	networks.POST("", func(c *gin.Context) {
		var req dto.CreateNetworkRequest
		if !bindJSON(c, &req) {
			return
		}
		resp, err := deps.Network.Create(userID(c), req)
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusCreated, resp)
	})
	// GET /networks/:networkId 返回网络详情，包括子网和成员视图。
	networks.GET("/:networkId", func(c *gin.Context) {
		resp, err := deps.Network.Get(userID(c), c.Param("networkId"))
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusOK, resp)
	})
	// POST /networks/:networkId/join 将某个设备加入目标网络。
	networks.POST("/:networkId/join", func(c *gin.Context) {
		var req dto.JoinNetworkRequest
		if !bindJSON(c, &req) {
			return
		}
		resp, err := deps.Network.Join(userID(c), c.Param("networkId"), req)
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusOK, resp)
	})
	// GET /networks/:networkId/members 返回网络成员设备列表。
	networks.GET("/:networkId/members", func(c *gin.Context) {
		items, err := deps.Network.ListMembers(userID(c), c.Param("networkId"))
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": items})
	})
	// GET /networks/:networkId/subnets 返回网络下的全部子网。
	networks.GET("/:networkId/subnets", func(c *gin.Context) {
		items, err := deps.Network.ListSubnets(userID(c), c.Param("networkId"))
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": items})
	})
	// POST /networks/:networkId/subnets 在网络内创建新子网。
	networks.POST("/:networkId/subnets", func(c *gin.Context) {
		var req dto.CreateSubnetRequest
		if !bindJSON(c, &req) {
			return
		}
		resp, err := deps.Network.CreateSubnet(userID(c), c.Param("networkId"), req)
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusCreated, resp)
	})
	// POST /networks/:networkId/subnets/:subnetId/attachments 将设备挂载到指定子网。
	networks.POST("/:networkId/subnets/:subnetId/attachments", func(c *gin.Context) {
		var req dto.AttachDeviceRequest
		if !bindJSON(c, &req) {
			return
		}
		resp, err := deps.Network.AttachDevice(userID(c), c.Param("networkId"), c.Param("subnetId"), req)
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusCreated, resp)
	})
}
