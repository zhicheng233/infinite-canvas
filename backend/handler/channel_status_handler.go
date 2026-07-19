package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"infinite-canvas-server/model"
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
		model.Fail(c, 500, err.Error())
		return
	}

	model.OK(c, status)
}
