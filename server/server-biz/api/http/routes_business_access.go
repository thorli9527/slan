package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/slan/server/server-biz/api/dto"
)

// registerAccessRoutes 注册无需业务鉴权即可访问的账号入口。
func registerAccessRoutes(api *gin.RouterGroup, deps routerDeps) {
	auth := api.Group("/auth")
	// POST /auth/register 创建终端用户账号。
	auth.POST("/register", func(c *gin.Context) {
		var req dto.RegisterRequest
		if !bindJSON(c, &req) {
			return
		}
		resp, err := deps.Auth.Register(req)
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusCreated, resp)
	})
	// POST /auth/login 校验账号并返回新的访问令牌。
	auth.POST("/login", func(c *gin.Context) {
		var req dto.LoginRequest
		if !bindJSON(c, &req) {
			return
		}
		resp, err := deps.Auth.Login(req)
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusOK, resp)
	})
}
