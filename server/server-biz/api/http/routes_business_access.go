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
	auth.POST("/register", respondWithBody(http.StatusCreated, func(c *gin.Context, req dto.RegisterRequest) (dto.AuthResponse, error) {
		return deps.Auth.Register(req)
	}))
	// POST /auth/login 校验账号并返回新的访问令牌。
	auth.POST("/login", respondWithBody(http.StatusOK, func(c *gin.Context, req dto.LoginRequest) (dto.AuthResponse, error) {
		return deps.Auth.Login(req)
	}))
	auth.POST("/refresh", respondWithBody(http.StatusOK, func(c *gin.Context, req dto.RefreshTokenRequest) (dto.AuthResponse, error) {
		return deps.Auth.Refresh(req)
	}))
	auth.GET("/callback-status/:callbackId", respondWithJSON(http.StatusOK, func(c *gin.Context) (dto.AuthCallbackStatusResponse, error) {
		rc := currentRouteContext(c)
		return deps.Auth.GetCallbackStatus(rc.callbackID(c))
	}))
	auth.POST("/callback-status/:callbackId/complete", respondWithBodyStatus(http.StatusOK, gin.H{"status": "ok"}, func(c *gin.Context, req dto.CompleteAuthCallbackRequest) error {
		rc := currentRouteContext(c)
		callbackID := rc.callbackID(c)
		if err := deps.Auth.CompleteCallback(callbackID, req); err != nil {
			return err
		}
		defaultAuthCallbackWSHub.broadcastReady(callbackID, req)
		return nil
	}))
	auth.POST("/callback-status/:callbackId/ack", respondWithStatus(http.StatusOK, gin.H{"status": "ok"}, func(c *gin.Context) error {
		rc := currentRouteContext(c)
		return deps.Auth.MarkCallbackReceived(rc.callbackID(c))
	}))
}
