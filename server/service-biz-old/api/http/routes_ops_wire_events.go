package httpapi

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/slan/server/server-biz/api/dto"
)

func wireNodeEventQueryFromRequest(c *gin.Context, defaultPageSize int) dto.WireNodeEventQuery {
	pageSize := positiveQueryInt(c, "pageSize", defaultPageSize)
	if pageSize > 200 {
		pageSize = 200
	}
	return dto.WireNodeEventQuery{
		NodeKind:      strings.TrimSpace(strings.ToLower(c.Query("nodeKind"))),
		RegionID:      strings.TrimSpace(c.Query("regionId")),
		NodeID:        strings.TrimSpace(c.Query("nodeId")),
		EventType:     strings.TrimSpace(strings.ToLower(c.Query("eventType"))),
		CreatedFromMs: int64Query(c, "createdFromMs", 0),
		CreatedToMs:   int64Query(c, "createdToMs", 0),
		Page:          positiveQueryInt(c, "page", 1),
		PageSize:      pageSize,
	}
}
