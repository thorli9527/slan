package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/slan/server/server-biz/api/dto"
)

func registerCommerceRoutes(api *gin.RouterGroup, deps routerDeps) {
	api.GET("/products", func(c *gin.Context) {
		rc := currentRouteContext(c)
		items, err := deps.Network.ListPurchaseProducts(rc.user())
		if err != nil {
			writeError(c, err)
			return
		}
		writeItems(c, items)
	})

	api.GET("/entitlements/:productCode", func(c *gin.Context) {
		rc := currentRouteContext(c)
		entitlement, err := deps.Network.GetProductEntitlement(rc.user(), c.Param("productCode"))
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusOK, entitlement)
	})

	api.GET("/orders", func(c *gin.Context) {
		rc := currentRouteContext(c)
		items, err := deps.Network.ListPurchaseOrders(rc.user())
		if err != nil {
			writeError(c, err)
			return
		}
		writeItems(c, items)
	})

	api.POST("/orders", func(c *gin.Context) {
		var req dto.CreatePurchaseOrderRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			writeErrorResponse(c, http.StatusBadRequest, errorCodeInvalidArgument, err.Error())
			return
		}
		rc := currentRouteContext(c)
		order, err := deps.Network.CreatePurchaseOrder(rc.user(), req)
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusCreated, order)
	})
}
