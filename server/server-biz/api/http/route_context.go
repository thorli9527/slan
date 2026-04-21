package httpapi

import "github.com/gin-gonic/gin"

type routeContext struct {
	userID string
}

func currentRouteContext(c *gin.Context) routeContext {
	return routeContext{userID: userID(c)}
}

func (ctx routeContext) user() string {
	return ctx.userID
}

func (ctx routeContext) networkID(c *gin.Context) string {
	return c.Param("networkId")
}

func (ctx routeContext) subnetID(c *gin.Context) string {
	return c.Param("subnetId")
}

func (ctx routeContext) attachmentID(c *gin.Context) string {
	return c.Param("attachmentId")
}

func (ctx routeContext) messageID(c *gin.Context) string {
	return c.Param("messageId")
}

func (ctx routeContext) callbackID(c *gin.Context) string {
	return c.Param("callbackId")
}
