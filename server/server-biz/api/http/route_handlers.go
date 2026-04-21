package httpapi

import (
	"net/http"

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
		c.JSON(http.StatusOK, gin.H{"items": items})
	}
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
