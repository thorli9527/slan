package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/slan/server/server-biz/api/dto"
)

func registerBootstrapRoutes(protected *gin.RouterGroup, deps routerDeps) {
	control := protected.Group("/control")
	control.POST("/sessions", respondWithBody(http.StatusCreated, func(c *gin.Context, req dto.CreateControlSessionRequest) (dto.ControlSessionResponse, error) {
		rc := currentRouteContext(c)
		return deps.Bootstrap.CreateControlSession(rc.user(), req)
	}))

	protected.POST("/bootstrap", respondWithBody(http.StatusOK, func(c *gin.Context, req dto.BootstrapRequest) (dto.BootstrapResponse, error) {
		rc := currentRouteContext(c)
		return deps.Bootstrap.Bootstrap(rc.user(), req)
	}))
	protected.POST("/relay/tickets", respondWithBody(http.StatusCreated, func(c *gin.Context, req dto.RelayTicketRequest) (dto.RelayTicket, error) {
		rc := currentRouteContext(c)
		return deps.Bootstrap.IssueRelayTicket(rc.user(), req)
	}))
}
