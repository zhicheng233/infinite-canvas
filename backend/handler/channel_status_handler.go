package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"infinite-canvas-server/service"
)

type ChannelStatusHandler struct {
	channelStatusService *service.ChannelStatusService
}

func NewChannelStatusHandler(channelStatusService *service.ChannelStatusService) *ChannelStatusHandler {
	return &ChannelStatusHandler{channelStatusService: channelStatusService}
}

func (h *ChannelStatusHandler) GetChannelStatus(c *gin.Context) {
	days := 7
	if daysParam := c.Query("days"); daysParam != "" {
		if parsed, err := strconv.Atoi(daysParam); err == nil && parsed > 0 {
			days = parsed
		}
	}

	status, err := h.channelStatusService.GetChannelStatus(0, days)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, status)
}
