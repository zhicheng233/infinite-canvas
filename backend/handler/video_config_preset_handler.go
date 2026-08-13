package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"infinite-canvas-server/model"
	"infinite-canvas-server/service"
)

type VideoConfigPresetHandler struct {
	service *service.VideoConfigPresetService
}

func NewVideoConfigPresetHandler(presetService *service.VideoConfigPresetService) *VideoConfigPresetHandler {
	return &VideoConfigPresetHandler{service: presetService}
}

func (h *VideoConfigPresetHandler) List(c *gin.Context) {
	items, err := h.service.List(currentTenantID(c))
	if err != nil {
		model.Fail(c, http.StatusInternalServerError, "读取视频配置预设失败")
		return
	}
	model.OK(c, items)
}

func (h *VideoConfigPresetHandler) Create(c *gin.Context) {
	var input model.CreateVideoConfigPresetInput
	if err := c.ShouldBindJSON(&input); err != nil {
		model.Fail(c, http.StatusBadRequest, "无效的请求参数")
		return
	}
	item, err := h.service.Create(currentTenantID(c), input)
	if err != nil {
		if errors.Is(err, service.ErrVideoConfigPresetNameConflict) {
			model.Fail(c, http.StatusConflict, err.Error())
			return
		}
		model.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	model.OK(c, item)
}

func (h *VideoConfigPresetHandler) Delete(c *gin.Context) {
	presetID, err := parsePositiveID(c.Param("presetId"))
	if err != nil {
		model.Fail(c, http.StatusBadRequest, "无效的预设 ID")
		return
	}
	if err := h.service.Delete(currentTenantID(c), presetID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			model.Fail(c, http.StatusNotFound, "视频配置预设不存在")
			return
		}
		model.Fail(c, http.StatusInternalServerError, "删除视频配置预设失败")
		return
	}
	model.OK(c, gin.H{"deleted": true})
}
