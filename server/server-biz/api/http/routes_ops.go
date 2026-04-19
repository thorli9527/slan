package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/configs"
)

// registerOpsRoutes 注册运营管理入口使用的只读和 RBAC 管理路由。
func registerOpsRoutes(api *gin.RouterGroup, cfg configs.Config, deps routerDeps) {
	ops := api.Group("")
	ops.Use(authenticateOps(cfg))

	// GET /overview 返回运营首页摘要。
	ops.GET("/overview", func(c *gin.Context) {
		resp, err := deps.Ops.Overview()
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusOK, resp)
	})
	// GET /users 返回用户聚合视图。
	ops.GET("/users", func(c *gin.Context) {
		items, err := deps.Ops.ListUsers()
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": items})
	})
	// GET /devices 返回设备聚合视图。
	ops.GET("/devices", func(c *gin.Context) {
		items, err := deps.Ops.ListDevices()
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": items})
	})
	// GET /admins 返回管理员扩展资料列表。
	ops.GET("/admins", func(c *gin.Context) {
		items, err := deps.Ops.ListAdmins()
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": items})
	})
	// POST /admins 创建或更新管理员扩展资料。
	ops.POST("/admins", func(c *gin.Context) {
		var req dto.UpsertAdminInfoRequest
		if !bindJSON(c, &req) {
			return
		}
		resp, err := deps.Ops.UpsertAdminInfo(req)
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusCreated, resp)
	})
	// GET /roles 返回角色及其绑定菜单视图。
	ops.GET("/roles", func(c *gin.Context) {
		items, err := deps.Ops.ListRoles()
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": items})
	})
	// POST /roles 创建角色。
	ops.POST("/roles", func(c *gin.Context) {
		var req dto.CreateRoleRequest
		if !bindJSON(c, &req) {
			return
		}
		resp, err := deps.Ops.CreateRole(req)
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusCreated, resp)
	})
	// PUT /users/:userId/roles 重设用户角色绑定。
	ops.PUT("/users/:userId/roles", func(c *gin.Context) {
		var req dto.AssignUserRolesRequest
		if !bindJSON(c, &req) {
			return
		}
		if err := deps.Ops.AssignUserRoles(c.Param("userId"), req); err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	// GET /menus 返回功能菜单列表。
	ops.GET("/menus", func(c *gin.Context) {
		items, err := deps.Ops.ListMenus()
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"items": items})
	})
	// POST /menus 创建功能菜单。
	ops.POST("/menus", func(c *gin.Context) {
		var req dto.CreateMenuRequest
		if !bindJSON(c, &req) {
			return
		}
		resp, err := deps.Ops.CreateMenu(req)
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusCreated, resp)
	})
	// PUT /roles/:roleId/menus 重设角色菜单绑定。
	ops.PUT("/roles/:roleId/menus", func(c *gin.Context) {
		var req dto.AssignRoleMenusRequest
		if !bindJSON(c, &req) {
			return
		}
		if err := deps.Ops.AssignRoleMenus(c.Param("roleId"), req); err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	// GET /relays 返回 relay 拓扑与健康摘要。
	ops.GET("/relays", func(c *gin.Context) {
		resp, err := deps.Ops.RelayTopology()
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusOK, resp)
	})
}
