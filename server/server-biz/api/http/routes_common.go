package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/configs"
	"github.com/slan/server/server-biz/internal/service"
)

// userIDContextKey 是 gin.Context 中保存当前认证用户 ID 的键。
const userIDContextKey = "userId"
const opsAdminIDContextKey = "opsAdminId"
const opsStaticTokenContextKey = "opsStaticToken"

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

// authenticateOps 校验独立 ops HTTP 实例使用的静态访问令牌。
func authenticateOps(cfg configs.Config, deps routerDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		authz := strings.TrimSpace(c.GetHeader("Authorization"))
		token := strings.TrimPrefix(authz, "Bearer ")
		if token == authz || token == "" {
			writeError(c, service.ErrUnauthorized)
			c.Abort()
			return
		}
		if token == cfg.Ops.AccessToken {
			c.Set(opsStaticTokenContextKey, true)
			c.Next()
			return
		}
		if deps.Ops == nil {
			writeError(c, service.ErrUnauthorized)
			c.Abort()
			return
		}
		adminID, err := deps.Ops.AuthenticateAdminToken(token)
		if err != nil {
			writeError(c, err)
			c.Abort()
			return
		}
		c.Set(opsAdminIDContextKey, adminID)
		c.Next()
	}
}

// authorizeOpsMenu 校验当前 ops 请求是否具备目标菜单码的访问权限。
func authorizeOpsMenu(deps routerDeps, menuCode string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if staticToken, _ := c.Get(opsStaticTokenContextKey); staticToken == true {
			c.Next()
			return
		}
		if deps.Ops == nil {
			writeError(c, service.ErrUnauthorized)
			c.Abort()
			return
		}
		value, _ := c.Get(opsAdminIDContextKey)
		adminID, _ := value.(string)
		if adminID == "" {
			writeError(c, service.ErrUnauthorized)
			c.Abort()
			return
		}
		if err := deps.Ops.AuthorizeAdminMenu(adminID, menuCode); err != nil {
			writeError(c, err)
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
