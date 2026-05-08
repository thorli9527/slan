package httpapi

import (
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/mqttauth"
	"github.com/slan/server/server-biz/internal/service"
)

// registerAccessRoutes 注册无需业务鉴权即可访问的账号入口。
func registerAccessRoutes(api *gin.RouterGroup, deps routerDeps) {
	auth := api.Group("/auth")
	// POST /auth/register
	//
	// 账号注册（无需鉴权）。
	//
	// 请求：RegisterRequest
	// - email: 用户邮箱（服务端会做规范化：trim + toLower）
	// - password: 明文密码（当前校验：长度 >= 8）
	//
	// 响应：201 AuthResponse
	// - accessToken / refreshToken：后续访问受保护接口使用
	//
	// 典型错误：
	// - 403 FORBIDDEN：配置禁止注册（Auth.AllowRegistration=false）
	// - 409 CONFLICT：邮箱已存在
	// - 400 INVALID_ARGUMENT：参数缺失或格式不合法
	auth.POST("/register", respondWithBody(http.StatusCreated, func(c *gin.Context, req dto.RegisterRequest) (dto.AuthResponse, error) {
		if !deps.Config.Auth.AllowRegistration {
			return dto.AuthResponse{}, service.ErrForbidden
		}
		return deps.Auth.Register(req)
	}))
	// POST /auth/login
	//
	// 账号登录（无需鉴权）。
	//
	// 请求：LoginRequest
	// - email / password
	//
	// 响应：200 AuthResponse(accessToken, refreshToken)
	//
	// 典型错误：
	// - 401 UNAUTHORIZED：邮箱或密码错误
	// - 400 INVALID_ARGUMENT：参数缺失
	auth.POST("/login", respondWithBody(http.StatusOK, func(c *gin.Context, req dto.LoginRequest) (dto.AuthResponse, error) {
		return deps.Auth.Login(req)
	}))
	// POST /auth/refresh
	//
	// 使用 refreshToken 换取新的 accessToken（无需鉴权）。
	//
	// 请求：RefreshTokenRequest(refreshToken)
	// 响应：200 AuthResponse(accessToken, refreshToken)
	//
	// 典型错误：
	// - 401 UNAUTHORIZED：refreshToken 失效/过期
	// - 400 INVALID_ARGUMENT：参数缺失
	auth.POST("/refresh", respondWithBody(http.StatusOK, func(c *gin.Context, req dto.RefreshTokenRequest) (dto.AuthResponse, error) {
		return deps.Auth.Refresh(req)
	}))
	// POST /auth/console-login
	//
	// Web Console 使用的一次性登录 key 兑换 accessToken（无需鉴权）。
	//
	// 请求：ConsumeConsoleLoginKeyRequest(loginKey, ...)
	// 响应：200 AuthResponse(accessToken, refreshToken)
	//
	// 典型错误：
	// - 401 UNAUTHORIZED：key 不存在/已消费/过期
	auth.POST("/console-login", respondWithBody(http.StatusOK, func(c *gin.Context, req dto.ConsumeConsoleLoginKeyRequest) (dto.AuthResponse, error) {
		return deps.Auth.ConsumeConsoleLoginKey(req)
	}))
	// GET /auth/callback-status/:callbackId
	//
	// 浏览器登录回调状态查询（无需鉴权）。
	//
	// 用途：
	// - 桌面端/本地客户端在浏览器完成登录后，通过轮询该接口获取回调载荷并完成本地会话落盘。
	//
	// 请求：
	// - path: callbackId
	//
	// 响应：200 AuthCallbackStatusResponse
	auth.GET("/callback-status/:callbackId", respondWithJSON(http.StatusOK, func(c *gin.Context) (dto.AuthCallbackStatusResponse, error) {
		rc := currentRouteContext(c)
		return deps.Auth.GetCallbackStatus(rc.callbackID(c))
	}))
	// POST /auth/callback-status/:callbackId/complete
	//
	// 浏览器侧“登录完成”回调写入（无需鉴权）。
	//
	// 用途：
	// - Web 登录页/Console 在用户完成认证后，将 AuthPayload 写入 server-biz
	// - 客户端再轮询 GET /auth/callback-status/:callbackId 拉取
	//
	// 请求：
	// - path: callbackId
	// - body: CompleteAuthCallbackRequest（包含 token / userId 等回调载荷）
	//
	// 响应：200 {"status":"ok"}
	auth.POST("/callback-status/:callbackId/complete", respondWithBodyStatus(http.StatusOK, gin.H{"status": "ok"}, func(c *gin.Context, req dto.CompleteAuthCallbackRequest) error {
		rc := currentRouteContext(c)
		callbackID := rc.callbackID(c)
		return deps.Auth.CompleteCallback(callbackID, req)
	}))
	// POST /mqtt/bifromq/auth
	//
	// BifroMQ Auth Provider 回调（无需业务鉴权）。
	//
	// 用途：
	// - Broker 在客户端 CONNECT 时调用，校验 MQTT 凭证有效性
	//
	// 请求（由 BifroMQ 提供，字段以其契约为准）：
	// - clientId / username / password
	//
	// 响应：
	// - 200 BifroMQAuthResponse{ok:{tenantId,userId,attrs...}}：允许连接
	// - 403 BifroMQAuthResponse{reject:"NotAuthorized"}：拒绝连接
	//
	// 副作用：
	// - 当 principal=device 时，会把设备标记为 controlReachable=true（用于设备在线态观测）
	api.POST("/mqtt/bifromq/auth", func(c *gin.Context) {
		var req map[string]any
		if !bindJSON(c, &req) {
			return
		}
		clientID := stringValue(req, "clientId")
		username := stringValue(req, "username")
		password := stringValue(req, "password")
		response, ok := validateBifroMQCredential(deps, clientID, username, password)
		if !ok {
			c.JSON(http.StatusForbidden, dto.BifroMQAuthResponse{Reject: "NotAuthorized"})
			return
		}
		if response.Principal == "device" && deps.Device != nil {
			_ = deps.Device.MarkMQTTReachable(response.DeviceID)
		}
		userID := response.DeviceID
		if response.Principal == "server" {
			userID = "server-biz-subscriber"
		}
		c.JSON(http.StatusOK, dto.BifroMQAuthResponse{
			OK: &dto.BifroMQAuthOK{
				TenantID: "slan",
				UserID:   userID,
				Attrs: map[string]string{
					"principal": response.Principal,
					"deviceId":  response.DeviceID,
				},
			},
		})
	})
	// POST /mqtt/bifromq/check
	//
	// BifroMQ ACL Provider 回调（无需业务鉴权）。
	//
	// 用途：
	// - Broker 在 SUBSCRIBE / PUBLISH 等动作发生时调用，校验 topic 访问权限
	//
	// 核心规则（见 mqttauth.AllowTopicAccess）：
	// - device principal：
	//   - 允许 SUBSCRIBE {topicPrefix}/{deviceId}/control/down
	//   - 允许 PUBLISH   {topicPrefix}/{deviceId}/control/up
	//   - 允许 PUBLISH   {topicPrefix}/{deviceId}/networks/{networkId}/state
	// - server principal：
	//   - 允许 SUBSCRIBE {topicPrefix}/+/control/up 等过滤（用于 server-biz 订阅上行控制消息）
	//   - 允许 PUBLISH   {topicPrefix}/{deviceId}/control/down（下行控制消息）
	//
	// 响应：200 "true"/"false"
	api.POST("/mqtt/bifromq/check", func(c *gin.Context) {
		var req map[string]any
		if !bindJSON(c, &req) {
			return
		}
		userID := strings.TrimSpace(c.GetHeader("user_id"))
		if userID == "" {
			userID = strings.TrimSpace(c.GetHeader("userId"))
		}
		principal := strings.TrimSpace(c.GetHeader("principal"))
		deviceID := strings.TrimSpace(c.GetHeader("deviceId"))
		if principal == "" {
			if userID == "server-biz-subscriber" {
				principal = "server"
			} else if userID != "" {
				principal = "device"
				deviceID = userID
			}
		}
		topic, subscribe := mqttActionTopic(req)
		allowed := mqttActionConnect(req) || mqttauth.AllowTopicAccess(deps.Config.MQTT, principal, deviceID, topic, subscribe)
		if !allowed {
			log.Printf("bifromq mqtt check denied userID=%s principal=%s deviceID=%s topic=%s subscribe=%v req=%v", userID, principal, deviceID, topic, subscribe, req)
		}
		c.String(http.StatusOK, "%t", allowed)
	})
}

func registerProtectedAccessRoutes(api *gin.RouterGroup, deps routerDeps) {
	auth := api.Group("/auth")
	// POST /auth/console-login-key
	//
	// 生成一次性 Console 登录 key（需要鉴权）。
	//
	// 用途：
	// - 已登录的客户端生成一个短时效 key，供 Web Console 兑换成 accessToken
	//
	// 请求：CreateConsoleLoginKeyRequest(deviceId 可选)
	// 响应：201 ConsoleLoginKeyResponse(loginKey)
	auth.POST("/console-login-key", respondWithBody(http.StatusCreated, func(c *gin.Context, req dto.CreateConsoleLoginKeyRequest) (dto.ConsoleLoginKeyResponse, error) {
		rc := currentRouteContext(c)
		return deps.Auth.CreateConsoleLoginKey(rc.user(), req)
	}))
	// PUT /auth/password
	//
	// 修改当前账号密码（需要鉴权）。
	//
	// 请求：ChangePasswordRequest(oldPassword, newPassword)
	// 响应：200 {"status":"ok"}
	//
	// 典型错误：
	// - 401 UNAUTHORIZED：旧密码不匹配
	// - 400 INVALID_ARGUMENT：参数缺失或新密码不满足规则
	auth.PUT("/password", respondWithBodyStatus(http.StatusOK, gin.H{"status": "ok"}, func(c *gin.Context, req dto.ChangePasswordRequest) error {
		rc := currentRouteContext(c)
		return deps.Auth.ChangePassword(rc.user(), req)
	}))
}

func stringValue(values map[string]any, key string) string {
	value, ok := values[key]
	if !ok {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return strings.TrimSpace(toString(typed))
	}
}

func validateBifroMQCredential(deps routerDeps, clientID, username, password string) (dto.MQTTCredentialAuthResult, bool) {
	if response, ok := mqttauth.ValidateCredential(deps.Config.MQTT, clientID, username, password); ok {
		return response, true
	}
	decoded, err := base64.StdEncoding.DecodeString(password)
	if err != nil {
		return dto.MQTTCredentialAuthResult{}, false
	}
	return mqttauth.ValidateCredential(deps.Config.MQTT, clientID, username, string(decoded))
}

func toString(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func mqttActionTopic(req map[string]any) (topic string, subscribe bool) {
	if topic := stringValue(req, "topic"); topic != "" {
		return topic, false
	}
	if topic := stringValue(req, "topicFilter"); topic != "" {
		return topic, true
	}
	for key, value := range req {
		nested, ok := value.(map[string]any)
		if !ok {
			continue
		}
		switch strings.ToLower(key) {
		case "pub", "publish":
			if topic := stringValue(nested, "topic"); topic != "" {
				return topic, false
			}
		case "sub", "subscribe":
			if topic := stringValue(nested, "topicFilter"); topic != "" {
				return topic, true
			}
			if topic := stringValue(nested, "topic"); topic != "" {
				return topic, true
			}
		}
	}
	return "", false
}

func mqttActionConnect(req map[string]any) bool {
	_, ok := req["conn"]
	return ok
}
