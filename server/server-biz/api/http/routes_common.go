package httpapi

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/slan/server/server-biz/configs"
	"github.com/slan/server/server-biz/internal/service"
)

const userIDContextKey = "userId"
const opsAdminIDContextKey = "opsAdminId"
const opsStaticTokenContextKey = "opsStaticToken"

const maxHTTPJSONBodyBytes = 1 << 20

func healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func limitRequestBody(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body != nil && maxBytes > 0 {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}
		c.Next()
	}
}

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

func bindJSON(c *gin.Context, out any) bool {
	if err := c.ShouldBindJSON(out); err != nil {
		if strings.Contains(err.Error(), "http: request body too large") {
			writeErrorResponse(c, http.StatusRequestEntityTooLarge, errorCodeRequestTooLarge, "request body exceeds 1 MiB limit")
			return false
		}
		writeErrorResponse(c, http.StatusBadRequest, errorCodeInvalidArgument, err.Error())
		return false
	}
	return true
}

func userID(c *gin.Context) string {
	value, _ := c.Get(userIDContextKey)
	userID, _ := value.(string)
	return userID
}
