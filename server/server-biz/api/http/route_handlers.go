package httpapi

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func respondWithJSON[T any](status int, call func(*gin.Context) (T, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		resp, err := call(c)
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(status, resp)
	}
}

func respondWithBody[Req any, Resp any](status int, call func(*gin.Context, Req) (Resp, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req Req
		if !bindJSON(c, &req) {
			return
		}
		resp, err := call(c, req)
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(status, resp)
	}
}

func respondWithItems[T any](call func(*gin.Context) ([]T, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		items, err := call(c)
		if err != nil {
			writeError(c, err)
			return
		}
		writeItems(c, items)
	}
}

func writeItems[T any](c *gin.Context, items []T) {
	page, pageSize, paged := paginationFromQuery(c)
	total := len(items)
	if paged {
		start := (page - 1) * pageSize
		if start >= total {
			items = []T{}
		} else {
			end := start + pageSize
			if end > total {
				end = total
			}
			items = items[start:end]
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"items":    items,
		"page":     page,
		"pageSize": pageSize,
		"total":    total,
	})
}

func paginationFromQuery(c *gin.Context) (int, int, bool) {
	_, hasPage := c.GetQuery("page")
	_, hasPageSize := c.GetQuery("pageSize")
	if !hasPage && !hasPageSize {
		return 1, 0, false
	}
	page := positiveQueryInt(c, "page", 1)
	pageSize := positiveQueryInt(c, "pageSize", 20)
	if pageSize > 200 {
		pageSize = 200
	}
	return page, pageSize, true
}

func positiveQueryInt(c *gin.Context, key string, fallback int) int {
	value, err := strconv.Atoi(c.Query(key))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func int64Query(c *gin.Context, key string, fallback int64) int64 {
	value, err := strconv.ParseInt(c.Query(key), 10, 64)
	if err != nil {
		return fallback
	}
	return value
}

func respondWithStatus(status int, body gin.H, call func(*gin.Context) error) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := call(c); err != nil {
			writeError(c, err)
			return
		}
		c.JSON(status, body)
	}
}

func respondWithBodyStatus[Req any](status int, body gin.H, call func(*gin.Context, Req) error) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req Req
		if !bindJSON(c, &req) {
			return
		}
		if err := call(c, req); err != nil {
			writeError(c, err)
			return
		}
		c.JSON(status, body)
	}
}
