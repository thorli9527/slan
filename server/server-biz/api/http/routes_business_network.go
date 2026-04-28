package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/slan/server/server-biz/api/dto"
)

// registerNetworkRoutes registers logical network, subnet, and attachment APIs.
func registerNetworkRoutes(protected *gin.RouterGroup, deps routerDeps) {
	protected.GET("/plan", respondWithJSON(http.StatusOK, func(c *gin.Context) (dto.PlanStatus, error) {
		rc := currentRouteContext(c)
		return deps.Network.PlanStatus(rc.user())
	}))

	networks := protected.Group("/networks")

	networks.GET("/home", respondWithJSON(http.StatusOK, func(c *gin.Context) (dto.NetworkHome, error) {
		rc := currentRouteContext(c)
		return deps.Network.Home(rc.user())
	}))

	networks.GET("", respondWithItems(func(c *gin.Context) ([]dto.Network, error) {
		rc := currentRouteContext(c)
		return deps.Network.List(rc.user())
	}))

	networks.POST("", respondWithBody(http.StatusCreated, func(c *gin.Context, req dto.CreateNetworkRequest) (dto.Network, error) {
		rc := currentRouteContext(c)
		return deps.Network.Create(rc.user(), req)
	}))

	networks.POST("/join-by-key", respondWithBody(http.StatusOK, func(c *gin.Context, req dto.JoinNetworkByKeyRequest) (dto.NetworkJoinResult, error) {
		rc := currentRouteContext(c)
		return deps.Network.JoinByKey(rc.user(), req)
	}))

	networks.PUT("/:networkId", respondWithBody(http.StatusOK, func(c *gin.Context, req dto.UpdateNetworkRequest) (dto.Network, error) {
		rc := currentRouteContext(c)
		return deps.Network.Update(rc.user(), rc.networkID(c), req)
	}))

	networks.PUT("/:networkId/join-key", respondWithBody(http.StatusOK, func(c *gin.Context, req dto.UpdateNetworkJoinKeyRequest) (dto.NetworkDetail, error) {
		rc := currentRouteContext(c)
		return deps.Network.UpdateJoinKey(rc.user(), rc.networkID(c), req)
	}))

	networks.PUT("/:networkId/dns", respondWithBody(http.StatusOK, func(c *gin.Context, req dto.UpdateNetworkDNSRequest) (dto.NetworkDetail, error) {
		rc := currentRouteContext(c)
		return deps.Network.UpdateDNS(rc.user(), rc.networkID(c), req)
	}))

	networks.POST("/:networkId/switch", respondWithBody(http.StatusOK, func(c *gin.Context, req dto.SwitchNetworkRequest) (dto.NetworkJoinResult, error) {
		rc := currentRouteContext(c)
		return deps.Network.Switch(rc.user(), rc.networkID(c), req)
	}))

	networks.GET("/:networkId", respondWithJSON(http.StatusOK, func(c *gin.Context) (dto.NetworkDetail, error) {
		rc := currentRouteContext(c)
		return deps.Network.Get(rc.user(), rc.networkID(c))
	}))

	networks.POST("/:networkId/join", respondWithBody(http.StatusOK, func(c *gin.Context, req dto.JoinNetworkRequest) (dto.NetworkJoinResult, error) {
		rc := currentRouteContext(c)
		return deps.Network.Join(rc.user(), rc.networkID(c), req)
	}))

	networks.POST("/:networkId/activate", respondWithBody(http.StatusOK, func(c *gin.Context, req dto.JoinNetworkRequest) (dto.NetworkJoinResult, error) {
		rc := currentRouteContext(c)
		return deps.Network.Activate(rc.user(), rc.networkID(c), req)
	}))

	networks.POST("/:networkId/deactivate", respondWithBodyStatus(http.StatusOK, gin.H{"status": "deactivated"}, func(c *gin.Context, req dto.DeactivateNetworkRequest) error {
		rc := currentRouteContext(c)
		return deps.Network.Deactivate(rc.user(), rc.networkID(c), req)
	}))

	networks.GET("/:networkId/members", respondWithItems(func(c *gin.Context) ([]dto.NetworkMember, error) {
		rc := currentRouteContext(c)
		return deps.Network.ListMembers(rc.user(), rc.networkID(c))
	}))

	networks.POST("/:networkId/invitations", respondWithBody(http.StatusCreated, func(c *gin.Context, req dto.InviteNetworkMemberRequest) (dto.NetworkMember, error) {
		rc := currentRouteContext(c)
		return deps.Network.InviteMember(rc.user(), rc.networkID(c), req)
	}))

	networks.PUT("/:networkId/members/:memberId/status", respondWithBody(http.StatusOK, func(c *gin.Context, req dto.UpdateNetworkMemberStatusRequest) (dto.NetworkMember, error) {
		rc := currentRouteContext(c)
		return deps.Network.UpdateMemberStatus(rc.user(), rc.networkID(c), rc.memberID(c), req)
	}))

	networks.GET("/:networkId/assignments", respondWithItems(func(c *gin.Context) ([]dto.NetworkAssignment, error) {
		rc := currentRouteContext(c)
		return deps.Network.ListAssignments(rc.user(), rc.networkID(c))
	}))

	networks.GET("/:networkId/subnets", respondWithItems(func(c *gin.Context) ([]dto.Subnet, error) {
		rc := currentRouteContext(c)
		return deps.Network.ListSubnets(rc.user(), rc.networkID(c))
	}))

	networks.PUT("/:networkId/attachments/:attachmentId/ip", respondWithBody(http.StatusOK, func(c *gin.Context, req dto.UpdateAttachmentIPRequest) (dto.SubnetAttachment, error) {
		rc := currentRouteContext(c)
		return deps.Network.UpdateAttachmentIP(rc.user(), rc.networkID(c), rc.attachmentID(c), req)
	}))

	networks.PUT("/:networkId/attachments/:attachmentId/remark", respondWithBody(http.StatusOK, func(c *gin.Context, req dto.UpdateAttachmentRemarkRequest) (dto.NetworkAssignment, error) {
		rc := currentRouteContext(c)
		return deps.Network.UpdateAttachmentRemark(rc.user(), rc.networkID(c), rc.attachmentID(c), req)
	}))

	networks.PUT("/:networkId/attachments/:attachmentId/status", respondWithBody(http.StatusOK, func(c *gin.Context, req dto.UpdateAttachmentStatusRequest) (dto.NetworkAssignment, error) {
		rc := currentRouteContext(c)
		return deps.Network.UpdateAttachmentStatus(rc.user(), rc.networkID(c), rc.attachmentID(c), req)
	}))

}
