package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/slan/server/server-biz/api/dto"
)

func registerInternalWireRoutes(api *gin.RouterGroup, deps routerDeps) {
	internal := api.Group("/internal/wire")
	internal.Use(authenticateInternalWire(deps.Config))

	internal.GET("/peers/:peerId/authz", respondWithJSON(http.StatusOK, func(c *gin.Context) (dto.WirePeerAuthzView, error) {
		return deps.Wire.PeerAuthz(c.Param("peerId"))
	}))
	internal.GET("/peers/:peerId/runtime-config", respondWithJSON(http.StatusOK, func(c *gin.Context) (dto.WirePeerRuntimeConfigView, error) {
		return deps.Wire.PeerRuntimeConfig(c.Param("peerId"))
	}))
	internal.GET("/networks/:networkId/topology", respondWithJSON(http.StatusOK, func(c *gin.Context) (dto.WireNetworkTopologyView, error) {
		return deps.Wire.NetworkTopology(c.Param("networkId"))
	}))
	internal.GET("/derp-map", respondWithJSON(http.StatusOK, func(c *gin.Context) (dto.WireDerpMapView, error) {
		return deps.Wire.DerpMap()
	}))
	internal.GET("/admin/derp-nodes", respondWithItems(func(c *gin.Context) ([]dto.WireDerpNodeRecord, error) {
		return deps.Wire.ListDerpNodes()
	}))
	internal.PUT("/admin/derp-nodes", respondWithBody(http.StatusOK, func(c *gin.Context, req dto.UpsertWireDerpNodeRequest) (dto.WireDerpNodeRecord, error) {
		return deps.Wire.UpsertDerpNode(req)
	}))
	internal.POST("/admin/derp-nodes/:regionId/:nodeId/heartbeat", respondWithBody(http.StatusOK, func(c *gin.Context, req dto.WireNodeHeartbeatRequest) (dto.WireDerpNodeRecord, error) {
		return deps.Wire.UpdateDerpNodeHealth(c.Param("regionId"), c.Param("nodeId"), req)
	}))
	internal.PATCH("/admin/derp-nodes/:regionId/:nodeId/status", respondWithBody(http.StatusOK, func(c *gin.Context, req dto.UpdateWireNodeStatusRequest) (dto.WireDerpNodeRecord, error) {
		return deps.Wire.UpdateDerpNodeStatus(c.Param("regionId"), c.Param("nodeId"), req)
	}))
	internal.GET("/admin/relay-nodes", respondWithItems(func(c *gin.Context) ([]dto.WireRelayNodeRecord, error) {
		return deps.Wire.ListRelayNodes()
	}))
	internal.PUT("/admin/relay-nodes", respondWithBody(http.StatusOK, func(c *gin.Context, req dto.UpsertWireRelayNodeRequest) (dto.WireRelayNodeRecord, error) {
		return deps.Wire.UpsertRelayNode(req)
	}))
	internal.POST("/admin/relay-nodes/:regionId/:nodeId/heartbeat", respondWithBody(http.StatusOK, func(c *gin.Context, req dto.WireNodeHeartbeatRequest) (dto.WireRelayNodeRecord, error) {
		return deps.Wire.UpdateRelayNodeHealth(c.Param("regionId"), c.Param("nodeId"), req)
	}))
	internal.PATCH("/admin/relay-nodes/:regionId/:nodeId/status", respondWithBody(http.StatusOK, func(c *gin.Context, req dto.UpdateWireNodeStatusRequest) (dto.WireRelayNodeRecord, error) {
		return deps.Wire.UpdateRelayNodeStatus(c.Param("regionId"), c.Param("nodeId"), req)
	}))
}
