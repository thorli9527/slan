package httpapi

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/service"
	"github.com/slan/server/server-biz/internal/util"
)

// registerNetworkRoutes registers logical network, subnet, and attachment APIs.
func registerNetworkRoutes(protected *gin.RouterGroup, deps routerDeps) {
	// GET /plan
	//
	// 返回当前账号的套餐/能力状态（需要鉴权）。
	//
	// 用途：
	// - 客户端在创建网络、邀请成员、启用高级能力前，可先检查计划状态并做 UI gating
	//
	// 响应：200 PlanStatus
	protected.GET("/plan", respondWithJSON(http.StatusOK, func(c *gin.Context) (dto.PlanStatus, error) {
		rc := currentRouteContext(c)
		return deps.Network.PlanStatus(rc.user())
	}))

	networks := protected.Group("/networks")

	// GET /networks/home
	//
	// 获取网络首页聚合视图（需要鉴权）。
	//
	// 用途：
	// - 返回“当前用户的 owned network / 已加入网络”等便于客户端快速决定默认 network
	//
	// 响应：200 NetworkHome
	networks.GET("/home", respondWithJSON(http.StatusOK, func(c *gin.Context) (dto.NetworkHome, error) {
		rc := currentRouteContext(c)
		return deps.Network.Home(rc.user())
	}))

	// GET /networks
	//
	// 列出当前用户可见的网络列表（需要鉴权）。
	//
	// 响应：200 ItemsResponse<Network>
	networks.GET("", respondWithItems(func(c *gin.Context) ([]dto.Network, error) {
		rc := currentRouteContext(c)
		return deps.Network.List(rc.user())
	}))

	// POST /networks
	//
	// 创建一个新网络（需要鉴权）。
	//
	// 请求：CreateNetworkRequest
	// - name/cidr/bindDeviceId 等字段以 DTO 为准
	//
	// 响应：201 Network(networkId, defaultSubnetId, ...)
	//
	// 典型错误：
	// - 400 INVALID_ARGUMENT：参数不合法（例如 CIDR）
	networks.POST("", respondWithBody(http.StatusCreated, func(c *gin.Context, req dto.CreateNetworkRequest) (dto.Network, error) {
		rc := currentRouteContext(c)
		return deps.Network.Create(rc.user(), req)
	}))

	// POST /networks/join-by-key
	//
	// 通过邀请码加入网络（需要鉴权）。
	//
	// 请求：JoinNetworkByKeyRequest(joinKey, deviceId, ...)
	// 响应：200 NetworkJoinResult(member, attachment, networkMap?, routes?, dns?)
	//
	// 说明：
	// - 该接口负责建立 member 关系，并为默认子网分配 attachment/virtual IP（必要时）
	networks.POST("/join-by-key", respondWithBody(http.StatusOK, func(c *gin.Context, req dto.JoinNetworkByKeyRequest) (dto.NetworkJoinResult, error) {
		rc := currentRouteContext(c)
		return deps.Network.JoinByKey(rc.user(), req)
	}))

	// PUT /networks/:networkId
	//
	// 更新网络基础属性（需要鉴权）。
	//
	// 请求：UpdateNetworkRequest
	// 响应：200 Network
	//
	// 典型错误：
	// - 403 FORBIDDEN：非网络 owner 或无权限
	networks.PUT("/:networkId", respondWithBody(http.StatusOK, func(c *gin.Context, req dto.UpdateNetworkRequest) (dto.Network, error) {
		rc := currentRouteContext(c)
		return deps.Network.Update(rc.user(), rc.networkID(c), req)
	}))

	// PUT /networks/:networkId/join-key
	//
	// 更新网络邀请码（需要鉴权）。
	//
	// 请求：UpdateNetworkJoinKeyRequest
	// 响应：200 NetworkDetail（包含最新 joinKey 等）
	networks.PUT("/:networkId/join-key", respondWithBody(http.StatusOK, func(c *gin.Context, req dto.UpdateNetworkJoinKeyRequest) (dto.NetworkDetail, error) {
		rc := currentRouteContext(c)
		return deps.Network.UpdateJoinKey(rc.user(), rc.networkID(c), req)
	}))

	// PUT /networks/:networkId/dns
	//
	// 更新网络 DNS 配置（需要鉴权）。
	//
	// 请求：UpdateNetworkDNSRequest
	// 响应：200 NetworkDetail
	networks.PUT("/:networkId/dns", respondWithBody(http.StatusOK, func(c *gin.Context, req dto.UpdateNetworkDNSRequest) (dto.NetworkDetail, error) {
		rc := currentRouteContext(c)
		return deps.Network.UpdateDNS(rc.user(), rc.networkID(c), req)
	}))

	// POST /networks/:networkId/switch
	//
	// 切换当前用户的“活跃网络”（需要鉴权）。
	//
	// 用途：
	// - 客户端在多网络之间切换选择；服务端会更新用户 activeNetworkId，并返回该 network 下的 member/attachment 视图
	//
	// 请求：SwitchNetworkRequest（通常包含 deviceId）
	// 响应：200 NetworkJoinResult
	networks.POST("/:networkId/switch", respondWithBody(http.StatusOK, func(c *gin.Context, req dto.SwitchNetworkRequest) (dto.NetworkJoinResult, error) {
		rc := currentRouteContext(c)
		return deps.Network.Switch(rc.user(), rc.networkID(c), req)
	}))

	// GET /networks/:networkId
	//
	// 获取网络详情（需要鉴权）。
	//
	// 响应：200 NetworkDetail(subnets, members, dns, routes, relayRegions, policy...)
	networks.GET("/:networkId", respondWithJSON(http.StatusOK, func(c *gin.Context) (dto.NetworkDetail, error) {
		rc := currentRouteContext(c)
		return deps.Network.Get(rc.user(), rc.networkID(c))
	}))

	// POST /networks/:networkId/join
	//
	// 将指定 deviceId 加入目标网络（需要鉴权）。
	//
	// 说明：
	// - 常用于“已知 networkId”的加入场景（例如 owner email / join key 兑换后得到 networkId）
	// - 负责创建 member + 默认子网 attachment，并分配 virtual IP（必要时）
	//
	// 请求：JoinNetworkRequest{deviceId}
	// 响应：200 NetworkJoinResult
	networks.POST("/:networkId/join", respondWithBody(http.StatusOK, func(c *gin.Context, req dto.JoinNetworkRequest) (dto.NetworkJoinResult, error) {
		rc := currentRouteContext(c)
		return deps.Network.Join(rc.user(), rc.networkID(c), req)
	}))

	// POST /networks/:networkId/activate
	//
	// 激活网络（需要鉴权）。
	//
	// 用途：
	// - 表达“用户希望在本机启用该 network 的隧道/路由/DNS”
	// - 服务端会确保 device 在该网络具备 active member + active attachment + virtual IP，并返回客户端落地隧道所需数据
	//
	// 请求：JoinNetworkRequest{deviceId}
	// 响应：200 NetworkJoinResult（包含 attachment.virtualIp、dns、routes、relay candidates 等）
	networks.POST("/:networkId/activate", respondWithBody(http.StatusOK, func(c *gin.Context, req dto.JoinNetworkRequest) (dto.NetworkJoinResult, error) {
		rc := currentRouteContext(c)
		return deps.Network.Activate(rc.user(), rc.networkID(c), req)
	}))

	// POST /networks/:networkId/relay-candidates
	//
	// 为当前设备返回 relay 候选列表（需要鉴权）。
	//
	// 用途：
	// - 客户端进行 relay 区域选择、质量探测或回退策略时使用
	//
	// 请求：JoinNetworkRequest{deviceId}
	// 响应：200 NetworkMap（其中包含 relayRegions / relayCandidateCountries 等字段）
	networks.POST("/:networkId/relay-candidates", respondWithBody(http.StatusOK, func(c *gin.Context, req dto.JoinNetworkRequest) (dto.NetworkMap, error) {
		rc := currentRouteContext(c)
		return deps.Network.RelayCandidates(rc.user(), rc.networkID(c), req)
	}))

	// POST /networks/:networkId/deactivate
	//
	// 取消激活网络（需要鉴权）。
	//
	// 用途：
	// - 客户端在本机关闭隧道后调用，通知控制面收敛 device 的 networkOnline/tunnelUp 等状态
	//
	// 请求：DeactivateNetworkRequest{deviceId,...}
	// 响应：200 {"status":"deactivated"}
	networks.POST("/:networkId/deactivate", respondWithBodyStatus(http.StatusOK, gin.H{"status": "deactivated"}, func(c *gin.Context, req dto.DeactivateNetworkRequest) error {
		rc := currentRouteContext(c)
		return deps.Network.Deactivate(rc.user(), rc.networkID(c), req)
	}))

	// GET /networks/:networkId/members
	//
	// 获取网络成员列表（需要鉴权）。
	//
	// 响应：200 ItemsResponse<NetworkMember>
	networks.GET("/:networkId/members", respondWithItems(func(c *gin.Context) ([]dto.NetworkMember, error) {
		rc := currentRouteContext(c)
		return deps.Network.ListMembers(rc.user(), rc.networkID(c))
	}))

	// POST /networks/:networkId/invitations
	//
	// 邀请成员加入网络（需要鉴权）。
	//
	// 请求：InviteNetworkMemberRequest（通常包含被邀请邮箱/角色等）
	// 响应：201 NetworkMember
	networks.POST("/:networkId/invitations", respondWithBody(http.StatusCreated, func(c *gin.Context, req dto.InviteNetworkMemberRequest) (dto.NetworkMember, error) {
		rc := currentRouteContext(c)
		return deps.Network.InviteMember(rc.user(), rc.networkID(c), req)
	}))

	// PUT /networks/:networkId/members/:memberId/status
	//
	// 更新成员状态（需要鉴权）。
	//
	// 用途：
	// - owner/管理员禁用或恢复成员等
	//
	// 请求：UpdateNetworkMemberStatusRequest
	// 响应：200 NetworkMember
	networks.PUT("/:networkId/members/:memberId/status", respondWithBody(http.StatusOK, func(c *gin.Context, req dto.UpdateNetworkMemberStatusRequest) (dto.NetworkMember, error) {
		rc := currentRouteContext(c)
		return deps.Network.UpdateMemberStatus(rc.user(), rc.networkID(c), rc.memberID(c), req)
	}))

	// GET /networks/:networkId/assignments
	//
	// 获取网络分配视图（需要鉴权）。
	//
	// 说明：
	// - assignments 是成员 + attachment 的聚合视图，常用于控制台展示每台设备的 virtual IP / remark / status 等
	//
	// 响应：200 ItemsResponse<NetworkAssignment>
	networks.GET("/:networkId/assignments", respondWithItems(func(c *gin.Context) ([]dto.NetworkAssignment, error) {
		rc := currentRouteContext(c)
		return deps.Network.ListAssignments(rc.user(), rc.networkID(c))
	}))

	networks.POST("/:networkId/devices/:deviceId/messages", respondWithBody(http.StatusAccepted, func(c *gin.Context, req dto.ClientMessageRequest) (dto.ClientMessageResponse, error) {
		rc := currentRouteContext(c)
		networkID := rc.networkID(c)
		targetDeviceID := strings.TrimSpace(c.Param("deviceId"))
		if targetDeviceID == "" || strings.TrimSpace(req.Body) == "" {
			return dto.ClientMessageResponse{}, service.ErrInvalidArgument
		}
		assignments, err := deps.Network.ListAssignments(rc.user(), networkID)
		if err != nil {
			return dto.ClientMessageResponse{}, err
		}
		fromDeviceID := strings.TrimSpace(req.FromDeviceID)
		if err := authorizeClientMessage(assignments, rc.user(), targetDeviceID, fromDeviceID); err != nil {
			return dto.ClientMessageResponse{}, err
		}
		nowMs := time.Now().UnixMilli()
		messageID := util.NewID("client-msg")
		payload := clientMessagePayload(messageID, networkID, fromDeviceID, targetDeviceID, req.Body, req.Metadata, nowMs)
		if err := publishControlMQTTEnvelope(deps, targetDeviceID, "client_message", "", payload); err != nil {
			return dto.ClientMessageResponse{}, err
		}
		return dto.ClientMessageResponse{
			MessageID:      messageID,
			NetworkID:      networkID,
			FromDeviceID:   fromDeviceID,
			TargetDeviceID: targetDeviceID,
			SentAtMs:       nowMs,
		}, nil
	}))

	// GET /networks/:networkId/quality?hours={1|2|12|24}
	//
	// 获取网络质量统计（需要鉴权）。
	//
	// 用途：
	// - 控制台/诊断面板展示 RTT/丢包/路径健康等聚合
	//
	// 响应：200 OpsNetworkQuality
	networks.GET("/:networkId/quality", respondWithJSON(http.StatusOK, func(c *gin.Context) (dto.OpsNetworkQuality, error) {
		rc := currentRouteContext(c)
		return deps.Network.NetworkQuality(rc.user(), rc.networkID(c), networkQualityHours(c))
	}))

	networks.GET("/:networkId/quality/templates", respondWithItems(func(c *gin.Context) ([]dto.RelayPolicyTemplate, error) {
		rc := currentRouteContext(c)
		return deps.Network.ListRelayPolicyTemplates(rc.user(), rc.networkID(c))
	}))

	networks.POST("/:networkId/quality/templates", respondWithBody(http.StatusCreated, func(c *gin.Context, req dto.RelayPolicyTemplateRequest) (dto.RelayPolicyTemplate, error) {
		rc := currentRouteContext(c)
		return deps.Network.CreateRelayPolicyTemplate(rc.user(), rc.networkID(c), req)
	}))

	networks.PUT("/:networkId/quality/templates/:templateId", respondWithBody(http.StatusOK, func(c *gin.Context, req dto.RelayPolicyTemplateRequest) (dto.RelayPolicyTemplate, error) {
		rc := currentRouteContext(c)
		return deps.Network.UpdateRelayPolicyTemplate(rc.user(), rc.networkID(c), c.Param("templateId"), req)
	}))

	networks.DELETE("/:networkId/quality/templates/:templateId", respondWithStatus(http.StatusOK, gin.H{"ok": true}, func(c *gin.Context) error {
		rc := currentRouteContext(c)
		return deps.Network.DeleteRelayPolicyTemplate(rc.user(), rc.networkID(c), c.Param("templateId"))
	}))

	// POST /networks/:networkId/quality/relay-policy
	//
	// 发布 relay 数据面策略（需要鉴权，且要求 ownedByCurrentUser）。
	//
	// 用途：
	// - Ops/控制台手动下发某 network 的 relay 数据面策略，影响客户端回退/选路
	//
	// 请求：OpsRelayDataPlanePolicyRequest（networkId 会在路由层补齐）
	// 响应：201 OpsRelayDataPlanePolicyResponse
	networks.POST("/:networkId/quality/relay-policy", respondWithBody(http.StatusCreated, func(c *gin.Context, req dto.OpsRelayDataPlanePolicyRequest) (dto.OpsRelayDataPlanePolicyResponse, error) {
		rc := currentRouteContext(c)
		detail, err := deps.Network.Get(rc.user(), rc.networkID(c))
		if err != nil {
			return dto.OpsRelayDataPlanePolicyResponse{}, err
		}
		if !detail.OwnedByCurrentUser {
			return dto.OpsRelayDataPlanePolicyResponse{}, service.ErrForbidden
		}
		req.NetworkID = rc.networkID(c)
		resp, err := publishOpsRelayDataPlanePolicy(deps, req)
		if err != nil {
			return dto.OpsRelayDataPlanePolicyResponse{}, err
		}
		if err := deps.Network.RecordRelayPolicyDispatch(rc.user(), rc.networkID(c), req, resp); err != nil {
			return dto.OpsRelayDataPlanePolicyResponse{}, err
		}
		return resp, nil
	}))

	// GET /networks/:networkId/subnets
	//
	// 获取网络子网列表（需要鉴权）。
	//
	// 响应：200 ItemsResponse<Subnet>
	networks.GET("/:networkId/subnets", respondWithItems(func(c *gin.Context) ([]dto.Subnet, error) {
		rc := currentRouteContext(c)
		return deps.Network.ListSubnets(rc.user(), rc.networkID(c))
	}))

	// PUT /networks/:networkId/attachments/:attachmentId/ip
	//
	// 手动更新 attachment 虚拟 IP（需要鉴权）。
	//
	// 请求：UpdateAttachmentIPRequest(virtualIp 或相关字段以 DTO 为准)
	// 响应：200 SubnetAttachment
	networks.PUT("/:networkId/attachments/:attachmentId/ip", respondWithBody(http.StatusOK, func(c *gin.Context, req dto.UpdateAttachmentIPRequest) (dto.SubnetAttachment, error) {
		rc := currentRouteContext(c)
		return deps.Network.UpdateAttachmentIP(rc.user(), rc.networkID(c), rc.attachmentID(c), req)
	}))

	// PUT /networks/:networkId/attachments/:attachmentId/remark
	//
	// 更新设备在网络内的备注/别名（需要鉴权）。
	//
	// 权限：
	// - 网络 owner，或 attachment 背后的 device owner
	//
	// 请求：UpdateAttachmentRemarkRequest(remark)
	// 响应：200 NetworkAssignment
	networks.PUT("/:networkId/attachments/:attachmentId/remark", respondWithBody(http.StatusOK, func(c *gin.Context, req dto.UpdateAttachmentRemarkRequest) (dto.NetworkAssignment, error) {
		rc := currentRouteContext(c)
		return deps.Network.UpdateAttachmentRemark(rc.user(), rc.networkID(c), rc.attachmentID(c), req)
	}))

	// PUT /networks/:networkId/attachments/:attachmentId/status
	//
	// 更新 attachment 状态（需要鉴权）。
	//
	// 用途：
	// - 禁用某设备在该子网的挂载、控制可见性等（具体语义以 DTO 与 service 层实现为准）
	//
	// 请求：UpdateAttachmentStatusRequest
	// 响应：200 NetworkAssignment
	networks.PUT("/:networkId/attachments/:attachmentId/status", respondWithBody(http.StatusOK, func(c *gin.Context, req dto.UpdateAttachmentStatusRequest) (dto.NetworkAssignment, error) {
		rc := currentRouteContext(c)
		return deps.Network.UpdateAttachmentStatus(rc.user(), rc.networkID(c), rc.attachmentID(c), req)
	}))

}

func networkQualityHours(c *gin.Context) int {
	value, err := strconv.Atoi(c.DefaultQuery("hours", "1"))
	if err != nil || value <= 0 {
		return 1
	}
	switch {
	case value <= 1:
		return 1
	case value <= 2:
		return 2
	case value <= 12:
		return 12
	default:
		return 24
	}
}

func authorizeClientMessage(assignments []dto.NetworkAssignment, userID, targetDeviceID, fromDeviceID string) error {
	var targetFound bool
	var fromFound bool
	for _, assignment := range assignments {
		if assignment.DeviceID == targetDeviceID && strings.EqualFold(assignment.Status, "active") {
			targetFound = true
		}
		if fromDeviceID != "" && assignment.DeviceID == fromDeviceID && assignment.UserID == userID && strings.EqualFold(assignment.Status, "active") {
			fromFound = true
		}
	}
	if !targetFound || (fromDeviceID != "" && !fromFound) {
		return service.ErrForbidden
	}
	return nil
}

func clientMessagePayload(messageID, networkID, fromDeviceID, targetDeviceID, body string, metadata map[string]any, sentAtMs int64) map[string]any {
	return map[string]any{
		"messageId":      messageID,
		"networkId":      networkID,
		"fromDeviceId":   fromDeviceID,
		"targetDeviceId": targetDeviceID,
		"body":           body,
		"metadata":       metadata,
		"sentAtMs":       sentAtMs,
	}
}
