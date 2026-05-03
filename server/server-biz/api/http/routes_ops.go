package httpapi

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/configs"
	controlmsg "github.com/slan/server/server-biz/internal/controlmsg"
	"github.com/slan/server/server-biz/internal/service"
	"github.com/slan/server/server-biz/internal/util"
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
	ops.GET("/plan-config", authorizeOpsMenu(deps, "ops.settings"), func(c *gin.Context) {
		resp, err := deps.Ops.PlanConfig()
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusOK, resp)
	})
	ops.PUT("/plan-config", authorizeOpsMenu(deps, "ops.settings"), func(c *gin.Context) {
		var req dto.UpdatePlanConfigRequest
		if !bindJSON(c, &req) {
			return
		}
		resp, err := deps.Ops.UpdatePlanConfig(req)
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusOK, resp)
	})
	ops.PUT("/users/:userId/plan", authorizeOpsMenu(deps, "ops.users"), func(c *gin.Context) {
		var req dto.UpdatePlanConfigRequest
		if !bindJSON(c, &req) {
			return
		}
		resp, err := deps.Ops.UpdateUserPlanOverride(c.Param("userId"), req)
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusOK, resp)
	})
	ops.DELETE("/users/:userId/plan", authorizeOpsMenu(deps, "ops.users"), func(c *gin.Context) {
		if err := deps.Ops.DeleteUserPlanOverride(c.Param("userId")); err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
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
	// GET /network-quality 返回用户设备实时网络质量样本。
	ops.GET("/network-quality", authorizeOpsMenu(deps, "ops.quality"), func(c *gin.Context) {
		resp, err := deps.Ops.NetworkQuality()
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusOK, resp)
	})
	// POST /network-quality/relay-policy 向网络内在线客户端下发 relay MTU/payload 策略。
	ops.POST("/network-quality/relay-policy", authorizeOpsMenu(deps, "ops.quality"), func(c *gin.Context) {
		var req dto.OpsRelayDataPlanePolicyRequest
		if !bindJSON(c, &req) {
			return
		}
		resp, err := publishOpsRelayDataPlanePolicy(deps, req)
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusCreated, resp)
	})
}

func publishOpsRelayDataPlanePolicy(deps routerDeps, req dto.OpsRelayDataPlanePolicyRequest) (dto.OpsRelayDataPlanePolicyResponse, error) {
	networkID := strings.TrimSpace(req.NetworkID)
	if networkID == "" || req.RelayMtu < 576 || req.RelayMtu > 1500 || req.MaxFramePayload < 512 || req.MaxFramePayload > 1400 || req.MaxFramePayload >= req.RelayMtu {
		return dto.OpsRelayDataPlanePolicyResponse{}, service.ErrInvalidArgument
	}
	if req.RecommendationLevel != nil && *req.RecommendationLevel > 11 {
		return dto.OpsRelayDataPlanePolicyResponse{}, service.ErrInvalidArgument
	}
	if req.ExecutionLevel != nil && *req.ExecutionLevel > 7 {
		return dto.OpsRelayDataPlanePolicyResponse{}, service.ErrInvalidArgument
	}
	preferredPathTypes, ok := normalizeOpsPreferredPathTypes(req.PreferredPathTypes)
	if !ok {
		return dto.OpsRelayDataPlanePolicyResponse{}, service.ErrInvalidArgument
	}
	version := req.Version
	if version == 0 {
		version = 1
	}
	if version != 1 {
		return dto.OpsRelayDataPlanePolicyResponse{}, service.ErrInvalidArgument
	}
	if deps.ControlChannel == nil {
		return dto.OpsRelayDataPlanePolicyResponse{}, service.ErrNotImplemented
	}
	targets := normalizeOpsRelayPolicyTargets(req.TargetDeviceIDs)
	scope := normalizeOpsRelayPolicyScope(req.Scope, len(targets) > 0)
	policyID := strings.TrimSpace(req.PolicyID)
	if policyID == "" {
		policyID = util.NewID("relay-policy")
	}
	ttlMS := req.TTLMS
	if ttlMS == 0 {
		ttlMS = uint64(time.Hour / time.Millisecond)
	}
	nowMS := uint64(time.Now().UnixMilli())
	policy := controlmsg.RelayDataPlanePolicy{
		PolicyID:            policyID,
		Version:             version,
		Scope:               scope,
		NetworkID:           networkID,
		TargetDeviceIDs:     sortedOpsRelayPolicyTargets(targets),
		PathType:            normalizeOpsRelayPolicyPathType(req.PathType),
		PreferredPathTypes:  preferredPathTypes,
		RecommendationLevel: req.RecommendationLevel,
		ExecutionLevel:      req.ExecutionLevel,
		RelayMtu:            req.RelayMtu,
		MaxFramePayload:     req.MaxFramePayload,
		Reason:              strings.TrimSpace(req.Reason),
		TTLMS:               ttlMS,
		EffectiveMS:         req.EffectiveMS,
		UpdatedAtMS:         nowMS,
	}
	sessions, err := deps.ControlChannel.ActiveSessions(networkID, "")
	if err != nil {
		return dto.OpsRelayDataPlanePolicyResponse{}, err
	}
	published := 0
	skipped := 0
	matchedTargets := make(map[string]bool, len(targets))
	for _, session := range sessions {
		deviceID := strings.TrimSpace(session.DeviceID)
		if deviceID == "" {
			skipped++
			continue
		}
		if len(targets) > 0 && !targets[deviceID] {
			continue
		}
		if len(targets) > 0 {
			matchedTargets[deviceID] = true
		}
		if err := publishControlMQTTEnvelope(deps, deviceID, "relay_data_plane_policy", "", policy); err != nil {
			skipped++
			continue
		}
		published++
	}
	if len(targets) > 0 {
		for target := range targets {
			if !matchedTargets[target] {
				skipped++
			}
		}
	}
	return dto.OpsRelayDataPlanePolicyResponse{
		PolicyID:           policyID,
		NetworkID:          networkID,
		Scope:              scope,
		TargetDeviceIDs:    sortedOpsRelayPolicyTargets(targets),
		PathType:           normalizeOpsRelayPolicyPathType(req.PathType),
		PreferredPathTypes: preferredPathTypes,
		Published:          published,
		Skipped:            skipped,
		RelayMtu:           req.RelayMtu,
		MaxFramePayload:    req.MaxFramePayload,
		Reason:             strings.TrimSpace(req.Reason),
	}, nil
}

func normalizeOpsRelayPolicyPathType(value string) string {
	value = strings.TrimSpace(value)
	switch value {
	case "", "any":
		return ""
	case "direct", "p2p", "relay", "direct_udp", "relay_udp", "relay_tcp", "relay_http3", "relay_tls":
		return value
	default:
		return ""
	}
}

func normalizeOpsPreferredPathTypes(values []string) ([]string, bool) {
	if len(values) == 0 {
		return nil, true
	}
	out := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		pathType := strings.TrimSpace(value)
		switch pathType {
		case "direct_udp", "relay_udp", "relay_tcp", "relay_http3", "relay_tls":
		default:
			return nil, false
		}
		if !seen[pathType] {
			seen[pathType] = true
			out = append(out, pathType)
		}
	}
	return out, true
}

func normalizeOpsRelayPolicyTargets(values []string) map[string]bool {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]bool, len(values))
	for _, value := range values {
		deviceID := strings.TrimSpace(value)
		if deviceID != "" {
			out[deviceID] = true
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeOpsRelayPolicyScope(scope string, hasTargets bool) string {
	value := strings.TrimSpace(scope)
	if hasTargets {
		if value == "device" || value == "device_override" {
			return value
		}
		return "device_override"
	}
	switch value {
	case "global", "region", "network":
		return value
	}
	return "network"
}

func sortedOpsRelayPolicyTargets(values map[string]bool) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
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
