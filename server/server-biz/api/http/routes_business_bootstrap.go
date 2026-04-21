package httpapi

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/slan/server/server-biz/api/dto"
)

// registerBootstrapRoutes 注册客户端启动、控制会话和 relay ticket 相关入口。
func registerBootstrapRoutes(protected *gin.RouterGroup, deps routerDeps) {
	control := protected.Group("/control")
	// POST /control/sessions 为指定节点创建控制面会话。
	control.POST("/sessions", respondWithBody(http.StatusCreated, func(c *gin.Context, req dto.CreateControlSessionRequest) (dto.ControlSessionResponse, error) {
		rc := currentRouteContext(c)
		return deps.Bootstrap.CreateControlSession(rc.user(), req)
	}))
	control.POST("/messages/:messageId/ack", func(c *gin.Context) {
		rc := currentRouteContext(c)
		if deps.MessageDelivery == nil {
			c.JSON(http.StatusNotImplemented, gin.H{"error": "message delivery disabled"})
			return
		}
		if err := deps.MessageDelivery.Ack(rc.user(), rc.messageID(c), time.Now().UnixMilli()); err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// POST /bootstrap 返回客户端启动所需的完整运行时配置。
	protected.POST("/bootstrap", respondWithBody(http.StatusOK, func(c *gin.Context, req dto.BootstrapRequest) (dto.BootstrapResponse, error) {
		rc := currentRouteContext(c)
		return deps.Bootstrap.Bootstrap(rc.user(), req)
	}))
	// POST /relay/tickets 为节点对申请 relay 回退票据。
	protected.POST("/relay/tickets", respondWithBody(http.StatusCreated, func(c *gin.Context, req dto.RelayTicketRequest) (dto.RelayTicket, error) {
		rc := currentRouteContext(c)
		return deps.Bootstrap.IssueRelayTicket(rc.user(), req)
	}))
}
