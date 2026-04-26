package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/slan/server/server-biz/api/dto"
	controlmsg "github.com/slan/server/server-biz/internal/controlmsg"
)

// registerNetworkRoutes 注册逻辑网络、子网和挂载关系相关入口。
func registerNetworkRoutes(protected *gin.RouterGroup, deps routerDeps) {
	networks := protected.Group("/networks")
	// GET /networks/home 返回当前用户的活动网络和自有网络摘要。
	networks.GET("/home", respondWithJSON(http.StatusOK, func(c *gin.Context) (dto.NetworkHome, error) {
		rc := currentRouteContext(c)
		return deps.Network.Home(rc.user())
	}))
	// GET /networks 返回当前用户可见的网络列表。
	networks.GET("", respondWithItems(func(c *gin.Context) ([]dto.Network, error) {
		rc := currentRouteContext(c)
		return deps.Network.List(rc.user())
	}))
	// POST /networks 创建一个新的逻辑网络。
	networks.POST("", respondWithBody(http.StatusCreated, func(c *gin.Context, req dto.CreateNetworkRequest) (dto.Network, error) {
		rc := currentRouteContext(c)
		return deps.Network.Create(rc.user(), req)
	}))
	// POST /networks/join-by-owner-email 通过宿主邮箱定位其网络并加入。
	networks.POST("/join-by-owner-email", respondWithBody(http.StatusOK, func(c *gin.Context, req dto.JoinNetworkByOwnerEmailRequest) (dto.NetworkJoinByOwnerEmailResult, error) {
		rc := currentRouteContext(c)
		return deps.Network.JoinByOwnerEmail(rc.user(), req)
	}))
	// POST /networks/join-by-key 通过加入 key 接入目标网络。
	networks.POST("/join-by-key", respondWithBody(http.StatusOK, func(c *gin.Context, req dto.JoinNetworkByKeyRequest) (dto.NetworkJoinResult, error) {
		rc := currentRouteContext(c)
		return deps.Network.JoinByKey(rc.user(), req)
	}))
	// PUT /networks/:networkId 修改当前用户自有网络的默认网段。
	networks.PUT("/:networkId", respondWithBody(http.StatusOK, func(c *gin.Context, req dto.UpdateNetworkRequest) (dto.Network, error) {
		rc := currentRouteContext(c)
		return deps.Network.Update(rc.user(), rc.networkID(c), req)
	}))
	// PUT /networks/:networkId/join-key 设置或清空网络加入 key。
	networks.PUT("/:networkId/join-key", respondWithBody(http.StatusOK, func(c *gin.Context, req dto.UpdateNetworkJoinKeyRequest) (dto.NetworkDetail, error) {
		rc := currentRouteContext(c)
		return deps.Network.UpdateJoinKey(rc.user(), rc.networkID(c), req)
	}))
	// PUT /networks/:networkId/dns 修改网络级 DNS 配置。
	networks.PUT("/:networkId/dns", respondWithBody(http.StatusOK, func(c *gin.Context, req dto.UpdateNetworkDNSRequest) (dto.NetworkDetail, error) {
		rc := currentRouteContext(c)
		return deps.Network.UpdateDNS(rc.user(), rc.networkID(c), req)
	}))
	// POST /networks/:networkId/switch 显式切换当前活动网络。
	networks.POST("/:networkId/switch", respondWithBody(http.StatusOK, func(c *gin.Context, req dto.SwitchNetworkRequest) (dto.NetworkJoinResult, error) {
		rc := currentRouteContext(c)
		return deps.Network.Switch(rc.user(), rc.networkID(c), req)
	}))
	// GET /networks/:networkId 返回网络详情，包括子网和成员视图。
	networks.GET("/:networkId", respondWithJSON(http.StatusOK, func(c *gin.Context) (dto.NetworkDetail, error) {
		rc := currentRouteContext(c)
		return deps.Network.Get(rc.user(), rc.networkID(c))
	}))
	// POST /networks/:networkId/join 将某个设备加入目标网络。
	networks.POST("/:networkId/join", respondWithBody(http.StatusOK, func(c *gin.Context, req dto.JoinNetworkRequest) (dto.NetworkJoinResult, error) {
		rc := currentRouteContext(c)
		return deps.Network.Join(rc.user(), rc.networkID(c), req)
	}))
	// POST /networks/:networkId/activate 为设备建立本地接入并分配虚拟 IP。
	networks.POST("/:networkId/activate", respondWithBody(http.StatusOK, func(c *gin.Context, req dto.JoinNetworkRequest) (dto.NetworkJoinResult, error) {
		rc := currentRouteContext(c)
		return deps.Network.Activate(rc.user(), rc.networkID(c), req)
	}))
	// POST /networks/:networkId/deactivate 释放设备当前网络接入和虚拟 IP。
	networks.POST("/:networkId/deactivate", respondWithBodyStatus(http.StatusOK, gin.H{"status": "deactivated"}, func(c *gin.Context, req dto.DeactivateNetworkRequest) error {
		rc := currentRouteContext(c)
		return deps.Network.Deactivate(rc.user(), rc.networkID(c), req)
	}))
	// GET /networks/:networkId/members 返回网络成员设备列表。
	networks.GET("/:networkId/members", respondWithItems(func(c *gin.Context) ([]dto.NetworkMember, error) {
		rc := currentRouteContext(c)
		return deps.Network.ListMembers(rc.user(), rc.networkID(c))
	}))
	// PUT /networks/:networkId/members/:memberId/status 审批或拒绝加入申请。
	networks.PUT("/:networkId/members/:memberId/status", respondWithBody(http.StatusOK, func(c *gin.Context, req dto.UpdateNetworkMemberStatusRequest) (dto.NetworkMember, error) {
		rc := currentRouteContext(c)
		return deps.Network.UpdateMemberStatus(rc.user(), rc.networkID(c), rc.memberID(c), req)
	}))
	// GET /networks/:networkId/assignments 返回网络内设备与虚拟 IP 绑定关系。
	networks.GET("/:networkId/assignments", respondWithItems(func(c *gin.Context) ([]dto.NetworkAssignment, error) {
		rc := currentRouteContext(c)
		return deps.Network.ListAssignments(rc.user(), rc.networkID(c))
	}))
	// GET /networks/:networkId/subnets 返回网络下的全部子网。
	networks.GET("/:networkId/subnets", respondWithItems(func(c *gin.Context) ([]dto.Subnet, error) {
		rc := currentRouteContext(c)
		return deps.Network.ListSubnets(rc.user(), rc.networkID(c))
	}))
	// POST /networks/:networkId/subnets 在网络内创建新子网。
	networks.POST("/:networkId/subnets", respondWithBody(http.StatusCreated, func(c *gin.Context, req dto.CreateSubnetRequest) (dto.Subnet, error) {
		rc := currentRouteContext(c)
		return deps.Network.CreateSubnet(rc.user(), rc.networkID(c), req)
	}))
	// POST /networks/:networkId/subnets/:subnetId/attachments 将设备挂载到指定子网。
	networks.POST("/:networkId/subnets/:subnetId/attachments", respondWithBody(http.StatusCreated, func(c *gin.Context, req dto.AttachDeviceRequest) (dto.SubnetAttachment, error) {
		rc := currentRouteContext(c)
		return deps.Network.AttachDevice(rc.user(), rc.networkID(c), rc.subnetID(c), req)
	}))
	// PUT /networks/:networkId/attachments/:attachmentId/ip 允许 owner 调整虚拟 IP。
	networks.PUT("/:networkId/attachments/:attachmentId/ip", respondWithBody(http.StatusOK, func(c *gin.Context, req dto.UpdateAttachmentIPRequest) (dto.SubnetAttachment, error) {
		rc := currentRouteContext(c)
		updated, err := deps.Network.UpdateAttachmentIP(rc.user(), rc.networkID(c), rc.attachmentID(c), req)
		if err != nil {
			return dto.SubnetAttachment{}, err
		}
		broadcastDeviceIPReassigned(deps, rc.networkID(c), controlmsg.DeviceIPReassigned{
			NetworkID:    rc.networkID(c),
			DeviceID:     updated.DeviceID,
			AttachmentID: updated.AttachmentID,
			VirtualIP:    updated.VirtualIP,
			Reason:       "attachment virtual ip updated",
		})
		return updated, nil
	}))
	// PUT /networks/:networkId/attachments/:attachmentId/remark 允许 owner 或设备所有者维护设备备注。
	networks.PUT("/:networkId/attachments/:attachmentId/remark", respondWithBody(http.StatusOK, func(c *gin.Context, req dto.UpdateAttachmentRemarkRequest) (dto.NetworkAssignment, error) {
		rc := currentRouteContext(c)
		return deps.Network.UpdateAttachmentRemark(rc.user(), rc.networkID(c), rc.attachmentID(c), req)
	}))
}
