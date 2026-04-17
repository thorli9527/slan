package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/infra"
	"github.com/slan/server/server-biz/internal/service"
)

// userIDContextKey 是 gin.Context 中保存当前认证用户 ID 的键。
const userIDContextKey = "userId"

// NewRouter 构建 server-biz 的 HTTP 路由。
//
// 路由分为两层：
// 1. 无鉴权路由：注册、登录、健康检查
// 2. 鉴权路由：设备、网络、bootstrap、relay ticket
func NewRouter(cfg infra.Config, services *service.Services) *gin.Engine {
	if services == nil {
		panic("http router requires explicit services")
	}
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	registerControlWS(router, cfg.WS.Path, services)

	router.GET("/healthz", healthz)

	api := router.Group("")
	{
		auth := api.Group("/auth")
		auth.POST("/register", func(c *gin.Context) {
			var req dto.RegisterRequest
			if !bindJSON(c, &req) {
				return
			}
			resp, err := services.Auth.Register(req)
			if err != nil {
				writeError(c, err)
				return
			}
			c.JSON(http.StatusCreated, resp)
		})
		auth.POST("/login", func(c *gin.Context) {
			var req dto.LoginRequest
			if !bindJSON(c, &req) {
				return
			}
			resp, err := services.Auth.Login(req)
			if err != nil {
				writeError(c, err)
				return
			}
			c.JSON(http.StatusOK, resp)
		})

		protected := api.Group("")
		protected.Use(authenticate(services))
		{
			devices := protected.Group("/devices")
			devices.POST("/register", func(c *gin.Context) {
				var req dto.RegisterDeviceRequest
				if !bindJSON(c, &req) {
					return
				}
				resp, err := services.Device.Register(userID(c), req)
				if err != nil {
					writeError(c, err)
					return
				}
				c.JSON(http.StatusCreated, resp)
			})
			devices.GET("", func(c *gin.Context) {
				items, err := services.Device.ListByUser(userID(c))
				if err != nil {
					writeError(c, err)
					return
				}
				c.JSON(http.StatusOK, gin.H{"items": items})
			})

			nodes := protected.Group("/nodes")
			nodes.POST("/register", func(c *gin.Context) {
				var req dto.RegisterNodeRequest
				if !bindJSON(c, &req) {
					return
				}
				resp, err := services.Node.Register(userID(c), req)
				if err != nil {
					writeError(c, err)
					return
				}
				c.JSON(http.StatusCreated, resp)
			})

			control := protected.Group("/control")
			control.POST("/sessions", func(c *gin.Context) {
				var req dto.CreateControlSessionRequest
				if !bindJSON(c, &req) {
					return
				}
				resp, err := services.Bootstrap.CreateControlSession(userID(c), req)
				if err != nil {
					writeError(c, err)
					return
				}
				c.JSON(http.StatusCreated, resp)
			})

			networks := protected.Group("/networks")
			networks.GET("", func(c *gin.Context) {
				items, err := services.Network.List(userID(c))
				if err != nil {
					writeError(c, err)
					return
				}
				c.JSON(http.StatusOK, gin.H{"items": items})
			})
			networks.POST("", func(c *gin.Context) {
				var req dto.CreateNetworkRequest
				if !bindJSON(c, &req) {
					return
				}
				resp, err := services.Network.Create(userID(c), req)
				if err != nil {
					writeError(c, err)
					return
				}
				c.JSON(http.StatusCreated, resp)
			})
			networks.GET("/:networkId", func(c *gin.Context) {
				resp, err := services.Network.Get(userID(c), c.Param("networkId"))
				if err != nil {
					writeError(c, err)
					return
				}
				c.JSON(http.StatusOK, resp)
			})
			networks.POST("/:networkId/join", func(c *gin.Context) {
				var req dto.JoinNetworkRequest
				if !bindJSON(c, &req) {
					return
				}
				resp, err := services.Network.Join(userID(c), c.Param("networkId"), req)
				if err != nil {
					writeError(c, err)
					return
				}
				c.JSON(http.StatusOK, resp)
			})
			networks.GET("/:networkId/members", func(c *gin.Context) {
				items, err := services.Network.ListMembers(userID(c), c.Param("networkId"))
				if err != nil {
					writeError(c, err)
					return
				}
				c.JSON(http.StatusOK, gin.H{"items": items})
			})
			networks.GET("/:networkId/subnets", func(c *gin.Context) {
				items, err := services.Network.ListSubnets(userID(c), c.Param("networkId"))
				if err != nil {
					writeError(c, err)
					return
				}
				c.JSON(http.StatusOK, gin.H{"items": items})
			})
			networks.POST("/:networkId/subnets", func(c *gin.Context) {
				var req dto.CreateSubnetRequest
				if !bindJSON(c, &req) {
					return
				}
				resp, err := services.Network.CreateSubnet(userID(c), c.Param("networkId"), req)
				if err != nil {
					writeError(c, err)
					return
				}
				c.JSON(http.StatusCreated, resp)
			})
			networks.POST("/:networkId/subnets/:subnetId/attachments", func(c *gin.Context) {
				var req dto.AttachDeviceRequest
				if !bindJSON(c, &req) {
					return
				}
				resp, err := services.Network.AttachDevice(userID(c), c.Param("networkId"), c.Param("subnetId"), req)
				if err != nil {
					writeError(c, err)
					return
				}
				c.JSON(http.StatusCreated, resp)
			})

			protected.POST("/bootstrap", func(c *gin.Context) {
				var req dto.BootstrapRequest
				if !bindJSON(c, &req) {
					return
				}
				resp, err := services.Bootstrap.Bootstrap(userID(c), req)
				if err != nil {
					writeError(c, err)
					return
				}
				c.JSON(http.StatusOK, resp)
			})
			protected.POST("/relay/tickets", func(c *gin.Context) {
				var req dto.RelayTicketRequest
				if !bindJSON(c, &req) {
					return
				}
				resp, err := services.Bootstrap.IssueRelayTicket(userID(c), req)
				if err != nil {
					writeError(c, err)
					return
				}
				c.JSON(http.StatusCreated, resp)
			})
		}
	}

	return router
}

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
func authenticate(services *service.Services) gin.HandlerFunc {
	return func(c *gin.Context) {
		authz := strings.TrimSpace(c.GetHeader("Authorization"))
		token := strings.TrimPrefix(authz, "Bearer ")
		if token == authz || token == "" {
			writeError(c, service.ErrUnauthorized)
			c.Abort()
			return
		}

		userID, err := services.Tokens.Authenticate(token)
		if err != nil {
			writeError(c, err)
			c.Abort()
			return
		}

		c.Set(userIDContextKey, userID)
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
