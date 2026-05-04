package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/slan/server/server-biz/api/dto"
)

func registerBootstrapRoutes(protected *gin.RouterGroup, deps routerDeps) {
	control := protected.Group("/control")
	// POST /control/sessions
	//
	// 仅刷新控制会话（需要鉴权）。
	//
	// 用途：
	// - 客户端已有 device/node/network 上下文时，仅需要刷新 control-plane sessionToken/wsUrl/networkMap
	// - 相比 POST /bootstrap 返回内容更聚焦在“控制通道会话”
	//
	// 请求：CreateControlSessionRequest
	// 响应：201 ControlSessionResponse(sessionToken, wsUrl, networkMap, ...)
	control.POST("/sessions", respondWithBody(http.StatusCreated, func(c *gin.Context, req dto.CreateControlSessionRequest) (dto.ControlSessionResponse, error) {
		rc := currentRouteContext(c)
		return deps.Bootstrap.CreateControlSession(rc.user(), req)
	}))

	// POST /bootstrap
	//
	// 客户端启动载荷获取（需要鉴权）。
	//
	// 用途：
	// - 统一返回客户端运行所需的关键配置：device/attachments、networks、controlPlane、stunServers、relay/derpMap、初始 networkMap
	// - 推荐的新客户端主流程入口（减少单独拼装依赖）
	//
	// 请求：BootstrapRequest(nodeId, networkId, ...)
	// 响应：200 BootstrapResponse（详见 dto/types_business_control.go: BootstrapResponse）
	//
	// 典型错误：
	// - 403 FORBIDDEN：node 不归属当前用户，或 device 不是 network active member，或没有 active attachment/virtual IP
	// - 404 NOT_FOUND：nodeId/networkId 不存在
	protected.POST("/bootstrap", respondWithBody(http.StatusOK, func(c *gin.Context, req dto.BootstrapRequest) (dto.BootstrapResponse, error) {
		rc := currentRouteContext(c)
		return deps.Bootstrap.Bootstrap(rc.user(), req)
	}))
	// POST /relay/tickets
	//
	// 签发 relay/DERP 回退票据（需要鉴权）。
	//
	// 用途：
	// - 当客户端 P2P 直连失败时，向控制面申请短时效票据
	// - 客户端再用该票据接入 server-relay 完成数据转发
	//
	// 请求：RelayTicketRequest(networkId, srcNodeId, dstNodeId, reason, relayRegionId, derpClusterId...)
	// 响应：201 RelayTicket(relayUrl, expiresAt, sessionKey, signature, ...)
	//
	// 典型错误：
	// - 403 FORBIDDEN：src/dst 节点不在同一 network 或不具备访问关系
	protected.POST("/relay/tickets", respondWithBody(http.StatusCreated, func(c *gin.Context, req dto.RelayTicketRequest) (dto.RelayTicket, error) {
		rc := currentRouteContext(c)
		return deps.Bootstrap.IssueRelayTicket(rc.user(), req)
	}))
}
