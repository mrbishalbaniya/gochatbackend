package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/pulse/chat-service/internal/utils"
)

func (h *Handler) Metrics(c *gin.Context) {
	promhttp.Handler().ServeHTTP(c.Writer, c.Request)
}

func (h *Handler) Ready(c *gin.Context) {
	utils.JSON(c, 200, gin.H{"ready": true})
}
