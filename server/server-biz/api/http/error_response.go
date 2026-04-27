package httpapi

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/service"
)

const (
	errorCodeInternal        = "INTERNAL"
	errorCodeInvalidArgument = "INVALID_ARGUMENT"
	errorCodeUnauthorized    = "UNAUTHORIZED"
	errorCodeForbidden       = "FORBIDDEN"
	errorCodeNotFound        = "NOT_FOUND"
	errorCodeConflict        = "CONFLICT"
	errorCodeRequestTooLarge = "REQUEST_TOO_LARGE"
	errorCodeRateLimited     = "RATE_LIMITED"
	errorCodeNotImplemented  = "NOT_IMPLEMENTED"
)

func writeErrorResponse(c *gin.Context, status int, code, message string) {
	c.JSON(status, dto.ErrorResponse{
		Code:    code,
		Message: message,
	})
}

func writeError(c *gin.Context, err error) {
	status := http.StatusInternalServerError
	code := errorCodeInternal
	switch {
	case errors.Is(err, service.ErrInvalidArgument):
		status = http.StatusBadRequest
		code = errorCodeInvalidArgument
	case errors.Is(err, service.ErrUnauthorized):
		status = http.StatusUnauthorized
		code = errorCodeUnauthorized
	case errors.Is(err, service.ErrForbidden):
		status = http.StatusForbidden
		code = errorCodeForbidden
	case errors.Is(err, service.ErrNotFound):
		status = http.StatusNotFound
		code = errorCodeNotFound
	case errors.Is(err, service.ErrConflict):
		status = http.StatusConflict
		code = errorCodeConflict
	case errors.Is(err, service.ErrNotImplemented):
		status = http.StatusNotImplemented
		code = errorCodeNotImplemented
	}

	writeErrorResponse(c, status, code, err.Error())
}

func installPublicErrorHandlers(router *gin.Engine) {
	router.HandleMethodNotAllowed = true
	router.NoRoute(func(c *gin.Context) {
		writeErrorResponse(c, http.StatusNotFound, errorCodeNotFound, "route not found")
	})
	router.NoMethod(func(c *gin.Context) {
		writeErrorResponse(c, http.StatusMethodNotAllowed, errorCodeInvalidArgument, "method not allowed")
	})
}
