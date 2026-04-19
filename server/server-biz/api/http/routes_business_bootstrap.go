package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/slan/server/server-biz/api/dto"
)

// registerBootstrapRoutes 注册客户端启动、控制会话和 relay ticket 相关入口。
func registerBootstrapRoutes(protected *gin.RouterGroup, deps routerDeps) {
	control := protected.Group("/control")
	// POST /control/sessions 为指定节点创建控制面会话。
	control.POST("/sessions", func(c *gin.Context) {
		var req dto.CreateControlSessionRequest
		if !bindJSON(c, &req) {
			return
		}
		resp, err := deps.Bootstrap.CreateControlSession(userID(c), req)
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusCreated, resp)
	})

	// POST /bootstrap 返回客户端启动所需的完整运行时配置。
	protected.POST("/bootstrap", func(c *gin.Context) {
		var req dto.BootstrapRequest
		if !bindJSON(c, &req) {
			return
		}
		resp, err := deps.Bootstrap.Bootstrap(userID(c), req)
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusOK, resp)
	})
	// POST /relay/tickets 为节点对申请 relay 回退票据。
	protected.POST("/relay/tickets", func(c *gin.Context) {
		var req dto.RelayTicketRequest
		if !bindJSON(c, &req) {
			return
		}
		resp, err := deps.Bootstrap.IssueRelayTicket(userID(c), req)
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusCreated, resp)
	})
}
