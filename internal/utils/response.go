package utils

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Envelope struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *APIError   `json:"error,omitempty"`
	Meta    interface{} `json:"meta,omitempty"`
}

func JSON(c *gin.Context, status int, data interface{}) {
	c.JSON(status, Envelope{Success: true, Data: data})
}

func JSONMeta(c *gin.Context, status int, data, meta interface{}) {
	c.JSON(status, Envelope{Success: true, Data: data, Meta: meta})
}

func Fail(c *gin.Context, status int, code, message string) {
	c.JSON(status, Envelope{
		Success: false,
		Error:   &APIError{Code: code, Message: message},
	})
}

func BadRequest(c *gin.Context, message string) {
	Fail(c, http.StatusBadRequest, "bad_request", message)
}

func Unauthorized(c *gin.Context, message string) {
	Fail(c, http.StatusUnauthorized, "unauthorized", message)
}

func Forbidden(c *gin.Context, message string) {
	Fail(c, http.StatusForbidden, "forbidden", message)
}

func NotFound(c *gin.Context, message string) {
	Fail(c, http.StatusNotFound, "not_found", message)
}

func Conflict(c *gin.Context, message string) {
	Fail(c, http.StatusConflict, "conflict", message)
}

func Internal(c *gin.Context, message string) {
	Fail(c, http.StatusInternalServerError, "internal_error", message)
}
