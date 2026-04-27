package httpapi

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/configs"
	"github.com/slan/server/server-biz/internal/service"
)

// registerOpsRoutes 注册运营管理入口使用的只读和 RBAC 管理路由。
func registerOpsRoutes(api *gin.RouterGroup, cfg configs.Config, deps routerDeps) {
	ops := api.Group("")

	// POST /login 使用管理员登录名和密码换取 ops access token。
	opsLoginHandlers := append(opsLoginRateLimitHandlers(cfg.Ops.LoginRateLimit), func(c *gin.Context) {
		var req dto.OpsLoginRequest
		if !bindJSON(c, &req) {
			return
		}
		resp, err := deps.Ops.LoginAdmin(req, c.ClientIP())
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusOK, resp)
	})
	ops.POST("/login", opsLoginHandlers...)

	ops.Use(authenticateOps(cfg, deps))

	ops.POST("/logout", func(c *gin.Context) {
		if err := deps.Ops.LogoutAdmin(bearerToken(c)); err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	ops.PUT("/me/password", func(c *gin.Context) {
		adminID := opsAdminID(c)
		if adminID == "" {
			writeError(c, service.ErrUnauthorized)
			return
		}
		var req dto.ChangeAdminPasswordRequest
		if !bindJSON(c, &req) {
			return
		}
		if err := deps.Ops.ChangeOwnAdminPassword(adminID, req); err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// GET /overview 返回运营首页摘要。
	ops.GET("/overview", authorizeOpsMenu(deps, "ops.overview"), func(c *gin.Context) {
		resp, err := deps.Ops.Overview()
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusOK, resp)
	})
	// GET /users 返回用户聚合视图。
	ops.GET("/users", authorizeOpsMenu(deps, "ops.users"), func(c *gin.Context) {
		items, err := deps.Ops.ListUsers()
		if err != nil {
			writeError(c, err)
			return
		}
		writeItems(c, items)
	})
	// GET /devices 返回设备聚合视图。
	ops.GET("/devices", authorizeOpsMenu(deps, "ops.devices"), func(c *gin.Context) {
		items, err := deps.Ops.ListDevices()
		if err != nil {
			writeError(c, err)
			return
		}
		writeItems(c, items)
	})
	// GET /admins 返回管理员扩展资料列表。
	ops.GET("/admins", authorizeOpsMenu(deps, "ops.admins"), func(c *gin.Context) {
		items, err := deps.Ops.ListAdmins()
		if err != nil {
			writeError(c, err)
			return
		}
		writeItems(c, items)
	})
	// POST /admins 创建或更新管理员扩展资料。
	ops.POST("/admins", authorizeOpsMenu(deps, "ops.admins"), func(c *gin.Context) {
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
	// PUT /admins/:adminId/password 修改管理员登录密码。
	ops.PUT("/admins/:adminId/password", authorizeOpsMenu(deps, "ops.admins"), func(c *gin.Context) {
		var req dto.ChangeAdminPasswordRequest
		if !bindJSON(c, &req) {
			return
		}
		if err := deps.Ops.ChangeAdminPassword(c.Param("adminId"), req); err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	// POST /admins/:adminId/unlock 清除管理员锁定状态。
	ops.POST("/admins/:adminId/unlock", authorizeOpsMenu(deps, "ops.admins"), func(c *gin.Context) {
		if err := deps.Ops.UnlockAdmin(c.Param("adminId")); err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	// GET /roles 返回角色及其绑定菜单视图。
	ops.GET("/roles", authorizeOpsMenu(deps, "ops.roles"), func(c *gin.Context) {
		items, err := deps.Ops.ListRoles()
		if err != nil {
			writeError(c, err)
			return
		}
		writeItems(c, items)
	})
	// POST /roles 创建角色。
	ops.POST("/roles", authorizeOpsMenu(deps, "ops.roles"), func(c *gin.Context) {
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
	ops.PUT("/users/:userId/roles", authorizeOpsMenu(deps, "ops.roles"), func(c *gin.Context) {
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
	ops.GET("/menus", authorizeOpsMenu(deps, "ops.menus"), func(c *gin.Context) {
		items, err := deps.Ops.ListMenus()
		if err != nil {
			writeError(c, err)
			return
		}
		writeItems(c, items)
	})
	// POST /menus 创建功能菜单。
	ops.POST("/menus", authorizeOpsMenu(deps, "ops.menus"), func(c *gin.Context) {
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
	ops.PUT("/roles/:roleId/menus", authorizeOpsMenu(deps, "ops.roles"), func(c *gin.Context) {
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
	ops.GET("/relays", authorizeOpsMenu(deps, "ops.relays"), func(c *gin.Context) {
		resp, err := deps.Ops.RelayTopology()
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusOK, resp)
	})
	ops.GET("/products", authorizeOpsMenu(deps, "ops.products"), func(c *gin.Context) {
		items, err := deps.Ops.ListProducts()
		if err != nil {
			writeError(c, err)
			return
		}
		writeItems(c, items)
	})
	ops.POST("/products", authorizeOpsMenu(deps, "ops.products"), func(c *gin.Context) {
		var req dto.UpsertProductRequest
		if !bindJSON(c, &req) {
			return
		}
		resp, err := deps.Ops.UpsertProduct(req)
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusCreated, resp)
	})
	ops.GET("/orders", authorizeOpsMenu(deps, "ops.orders"), func(c *gin.Context) {
		items, err := deps.Ops.ListPurchaseOrders()
		if err != nil {
			writeError(c, err)
			return
		}
		writeItems(c, items)
	})
	ops.POST("/orders/paid", authorizeOpsMenu(deps, "ops.orders"), func(c *gin.Context) {
		var req dto.OpsCreatePaidOrderRequest
		if !bindJSON(c, &req) {
			return
		}
		resp, err := deps.Ops.CreatePaidPurchaseOrder(req)
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusCreated, resp)
	})
	ops.PUT("/orders/:orderId/status", authorizeOpsMenu(deps, "ops.orders"), func(c *gin.Context) {
		var req dto.UpdatePurchaseOrderStatusRequest
		if !bindJSON(c, &req) {
			return
		}
		resp, err := deps.Ops.UpdatePurchaseOrderStatus(c.Param("orderId"), req)
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusOK, resp)
	})
}

func opsLoginRateLimitHandlers(cfg configs.OpsLoginRateLimitConfig) []gin.HandlerFunc {
	if cfg.Disabled {
		return nil
	}
	window := time.Duration(cfg.WindowSeconds) * time.Second
	if window <= 0 {
		window = defaultOpsLoginRateLimitWindow
	}
	ipLimit := cfg.IPLimit
	if ipLimit <= 0 {
		ipLimit = defaultOpsLoginIPLimit
	}
	ipLoginNameLimit := cfg.IPLoginNameLimit
	if ipLoginNameLimit <= 0 {
		ipLoginNameLimit = defaultOpsLoginIPLoginNameLimit
	}
	loginNameLimit := cfg.LoginNameLimit
	if loginNameLimit <= 0 {
		loginNameLimit = defaultOpsLoginLoginNameLimit
	}
	return []gin.HandlerFunc{
		rateLimitByIP(ipLimit, window),
		rateLimitByIPAndJSONField(ipLoginNameLimit, window, "loginName"),
		rateLimitByJSONField(loginNameLimit, window, "loginName"),
	}
}
