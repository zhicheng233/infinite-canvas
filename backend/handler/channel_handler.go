package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"infinite-canvas-server/model"
	"infinite-canvas-server/service"
)

type ChannelHandler struct {
	channelService *service.ChannelService
}

func NewChannelHandler(channelService *service.ChannelService) *ChannelHandler {
	return &ChannelHandler{channelService: channelService}
}

func (h *ChannelHandler) List(c *gin.Context) {
	channels, err := h.channelService.ListEnabled()
	if err != nil {
		model.Fail(c, http.StatusInternalServerError, "读取渠道失败")
		return
	}
	model.OK(c, gin.H{"channels": channels})
}

func (h *ChannelHandler) ListAdmin(c *gin.Context) {
	channels, err := h.channelService.ListAll()
	if err != nil {
		model.Fail(c, http.StatusInternalServerError, "读取渠道失败")
		return
	}
	model.OK(c, gin.H{"channels": channels})
}

func (h *ChannelHandler) Create(c *gin.Context) {
	var input model.SaveChannelInput
	if err := c.ShouldBindJSON(&input); err != nil {
		model.Fail(c, http.StatusBadRequest, "无效的请求参数")
		return
	}
	channel, err := h.channelService.Create(input)
	if err != nil {
		model.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	model.OK(c, channel)
}

func (h *ChannelHandler) Update(c *gin.Context) {
	id, ok := parseChannelID(c)
	if !ok {
		return
	}
	var input model.SaveChannelInput
	if err := c.ShouldBindJSON(&input); err != nil {
		model.Fail(c, http.StatusBadRequest, "无效的请求参数")
		return
	}
	channel, err := h.channelService.Update(id, input)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			model.Fail(c, http.StatusNotFound, "渠道不存在")
			return
		}
		model.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	model.OK(c, channel)
}

func (h *ChannelHandler) Disable(c *gin.Context) {
	id, ok := parseChannelID(c)
	if !ok {
		return
	}
	if err := h.channelService.Disable(id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			model.Fail(c, http.StatusNotFound, "渠道不存在")
			return
		}
		model.Fail(c, http.StatusInternalServerError, "禁用渠道失败")
		return
	}
	model.OK(c, gin.H{"disabled": true})
}

func (h *ChannelHandler) Enable(c *gin.Context) {
	id, ok := parseChannelID(c)
	if !ok {
		return
	}
	if err := h.channelService.Enable(id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			model.Fail(c, http.StatusNotFound, "渠道不存在")
			return
		}
		model.Fail(c, http.StatusInternalServerError, "启用渠道失败")
		return
	}
	model.OK(c, gin.H{"enabled": true})
}

func parseChannelID(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		model.Fail(c, http.StatusBadRequest, "无效的渠道 ID")
		return 0, false
	}
	return uint(id), true
}
