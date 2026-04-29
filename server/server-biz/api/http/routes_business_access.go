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
	// POST /auth/register 创建终端用户账号。
	auth.POST("/register", respondWithBody(http.StatusCreated, func(c *gin.Context, req dto.RegisterRequest) (dto.AuthResponse, error) {
		if !deps.Config.Auth.AllowRegistration {
			return dto.AuthResponse{}, service.ErrForbidden
		}
		return deps.Auth.Register(req)
	}))
	// POST /auth/login 校验账号并返回新的访问令牌。
	auth.POST("/login", respondWithBody(http.StatusOK, func(c *gin.Context, req dto.LoginRequest) (dto.AuthResponse, error) {
		return deps.Auth.Login(req)
	}))
	auth.POST("/refresh", respondWithBody(http.StatusOK, func(c *gin.Context, req dto.RefreshTokenRequest) (dto.AuthResponse, error) {
		return deps.Auth.Refresh(req)
	}))
	auth.POST("/console-login", respondWithBody(http.StatusOK, func(c *gin.Context, req dto.ConsumeConsoleLoginKeyRequest) (dto.AuthResponse, error) {
		return deps.Auth.ConsumeConsoleLoginKey(req)
	}))
	auth.GET("/callback-status/:callbackId", respondWithJSON(http.StatusOK, func(c *gin.Context) (dto.AuthCallbackStatusResponse, error) {
		rc := currentRouteContext(c)
		return deps.Auth.GetCallbackStatus(rc.callbackID(c))
	}))
	auth.POST("/callback-status/:callbackId/complete", respondWithBodyStatus(http.StatusOK, gin.H{"status": "ok"}, func(c *gin.Context, req dto.CompleteAuthCallbackRequest) error {
		rc := currentRouteContext(c)
		callbackID := rc.callbackID(c)
		return deps.Auth.CompleteCallback(callbackID, req)
	}))
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
	auth.POST("/console-login-key", respondWithBody(http.StatusCreated, func(c *gin.Context, req dto.CreateConsoleLoginKeyRequest) (dto.ConsoleLoginKeyResponse, error) {
		rc := currentRouteContext(c)
		return deps.Auth.CreateConsoleLoginKey(rc.user(), req)
	}))
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
