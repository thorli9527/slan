package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/slan/server/server-biz/api/dto"
)

func registerIceRoutes(protected *gin.RouterGroup, deps routerDeps) {
	protected.GET("/client/ice-servers", respondWithJSON(http.StatusOK, func(c *gin.Context) (dto.ClientIceServersResponse, error) {
		return deps.Ice.ListIceServers(c.Query("region"), positiveQueryInt(c, "limit", 5))
	}))

	protected.POST("/peers/:peerId/candidates", respondWithBody(http.StatusOK, func(c *gin.Context, req dto.ReportCandidatesRequest) (dto.ReportCandidatesResponse, error) {
		rc := currentRouteContext(c)
		return deps.Ice.ReportCandidates(rc.user(), c.Param("peerId"), req)
	}))

	protected.POST("/punch-plans", respondWithBody(http.StatusCreated, func(c *gin.Context, req dto.CreatePunchPlanRequest) (dto.PunchPlan, error) {
		rc := currentRouteContext(c)
		return deps.Ice.CreatePunchPlan(rc.user(), req)
	}))

	protected.POST("/punch-sessions/:sessionId/result", respondWithBodyStatus(http.StatusOK, gin.H{"status": "ok"}, func(c *gin.Context, req dto.ReportPunchResultRequest) error {
		rc := currentRouteContext(c)
		return deps.Ice.ReportPunchResult(rc.user(), c.Param("sessionId"), req)
	}))
}
