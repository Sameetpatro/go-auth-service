package response

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/sameetpatro/go-qr-auth/internal/dto"
)

func JSON(c *gin.Context, status int, data interface{}) {
	c.JSON(status, data)
}

func Success(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusOK, dto.SuccessResponse{Message: message, Data: data})
}

func Created(c *gin.Context, message string, data interface{}) {
	c.JSON(http.StatusCreated, dto.SuccessResponse{Message: message, Data: data})
}

func Error(c *gin.Context, status int, err string, message string) {
	c.JSON(status, dto.ErrorResponse{Error: err, Message: message})
	c.Abort()
}

func BadRequest(c *gin.Context, message string) {
	Error(c, http.StatusBadRequest, "bad_request", message)
}

func Unauthorized(c *gin.Context, message string) {
	Error(c, http.StatusUnauthorized, "unauthorized", message)
}

func Forbidden(c *gin.Context, message string) {
	Error(c, http.StatusForbidden, "forbidden", message)
}

func NotFound(c *gin.Context, message string) {
	Error(c, http.StatusNotFound, "not_found", message)
}

func InternalError(c *gin.Context, message string) {
	Error(c, http.StatusInternalServerError, "internal_error", message)
}
