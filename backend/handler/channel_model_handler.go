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

type ChannelModelHandler struct {
	service *service.ChannelModelService
}

func NewChannelModelHandler(channelModelService *service.ChannelModelService) *ChannelModelHandler {
	return &ChannelModelHandler{service: channelModelService}
}

func (h *ChannelModelHandler) List(c *gin.Context) {
	channelID, err := parsePositiveID(c.Param("id"))
	if err != nil {
		model.Fail(c, http.StatusBadRequest, "无效的渠道 ID")
		return
	}
	items, err := h.service.ListUserCatalog(currentTenantID(c), channelID)
	if err != nil {
		model.Fail(c, http.StatusInternalServerError, "读取模型失败")
		return
	}
	model.OK(c, gin.H{"models": items})
}

func (h *ChannelModelHandler) ListAdmin(c *gin.Context) {
	channelID, err := parsePositiveID(c.Param("id"))
	if err != nil {
		model.Fail(c, http.StatusBadRequest, "无效的渠道 ID")
		return
	}
	items, err := h.service.List(channelID, false)
	if err != nil {
		model.Fail(c, http.StatusInternalServerError, "读取模型失败")
		return
	}
	model.OK(c, gin.H{"models": items})
}

func (h *ChannelModelHandler) Update(c *gin.Context) {
	modelID, err := parsePositiveID(c.Param("modelId"))
	if err != nil {
		model.Fail(c, http.StatusBadRequest, "无效的模型 ID")
		return
	}
	var input model.UpdateChannelModelInput
	if err := c.ShouldBindJSON(&input); err != nil {
		model.Fail(c, http.StatusBadRequest, "无效的请求参数")
		return
	}
	item, err := h.service.Update(modelID, input)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			model.Fail(c, http.StatusNotFound, "模型不存在")
			return
		}
		model.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	model.OK(c, item)
}

func (h *ChannelModelHandler) Sync(c *gin.Context) {
	channelID, err := parsePositiveID(c.Param("id"))
	if err != nil {
		model.Fail(c, http.StatusBadRequest, "无效的渠道 ID")
		return
	}
	if err := h.service.Sync(channelID); err != nil {
		model.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	model.OK(c, gin.H{"synced": true})
}

func parsePositiveID(value string) (uint, error) {
	id, err := strconv.ParseUint(value, 10, 64)
	if err != nil || id == 0 {
		return 0, errors.New("invalid id")
	}
	return uint(id), nil
}

func currentTenantID(c *gin.Context) uint {
	claims := c.MustGet("claims").(*service.Claims)
	return claims.TenantID
}
