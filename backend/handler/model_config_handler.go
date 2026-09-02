package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"infinite-canvas-server/model"
	"infinite-canvas-server/service"
)

type ModelConfigHandler struct {
	service *service.ModelConfigService
}

func NewModelConfigHandler(modelConfigService *service.ModelConfigService) *ModelConfigHandler {
	return &ModelConfigHandler{service: modelConfigService}
}

func (h *ModelConfigHandler) ListChannels(c *gin.Context) {
	items, err := h.service.ListChannels(currentTenantID(c))
	if err != nil {
		model.Fail(c, http.StatusInternalServerError, "读取渠道失败")
		return
	}
	model.OK(c, gin.H{"channels": items})
}

func (h *ModelConfigHandler) CreateChannel(c *gin.Context) {
	var input model.SaveChannelInput
	if c.ShouldBindJSON(&input) != nil {
		model.Fail(c, http.StatusBadRequest, "无效的请求参数")
		return
	}
	item, err := h.service.CreateChannel(currentTenantID(c), c.GetUint("user_id"), input)
	if err != nil {
		model.Fail(c, http.StatusBadRequest, err.Error())
		return
	}
	model.OK(c, item)
}

func (h *ModelConfigHandler) UpdateChannel(c *gin.Context) {
	id, ok := modelServiceID(c, "channelId", "渠道")
	if !ok {
		return
	}
	var input model.SaveChannelInput
	if c.ShouldBindJSON(&input) != nil {
		model.Fail(c, http.StatusBadRequest, "无效的请求参数")
		return
	}
	item, err := h.service.UpdateChannel(currentTenantID(c), c.GetUint("user_id"), id, input)
	if err != nil {
		writeModelServiceError(c, err)
		return
	}
	model.OK(c, item)
}

func (h *ModelConfigHandler) ListModels(c *gin.Context) {
	channelID, _ := strconv.ParseUint(strings.TrimSpace(c.Query("channel_id")), 10, 64)
	filter := service.ModelConfigFilter{
		ChannelID:       uint(channelID),
		Capability:      strings.TrimSpace(c.Query("capability")),
		Status:          strings.TrimSpace(c.Query("status")),
		Search:          strings.TrimSpace(c.Query("search")),
		IncludeArchived: c.Query("include_archived") == "true",
	}
	items, err := h.service.ListModels(currentTenantID(c), filter)
	if err != nil {
		model.Fail(c, http.StatusInternalServerError, "读取模型目录失败")
		return
	}
	model.OK(c, gin.H{"models": items})
}

func (h *ModelConfigHandler) GetModel(c *gin.Context) {
	id, ok := modelServiceID(c, "modelId", "模型")
	if !ok {
		return
	}
	item, err := h.service.GetModel(currentTenantID(c), id)
	if err != nil {
		writeModelServiceError(c, err)
		return
	}
	model.OK(c, item)
}

func (h *ModelConfigHandler) UpdateModel(c *gin.Context) {
	id, ok := modelServiceID(c, "modelId", "模型")
	if !ok {
		return
	}
	var input model.UpdateModelConfigInput
	if c.ShouldBindJSON(&input) != nil {
		model.Fail(c, http.StatusBadRequest, "无效的请求参数")
		return
	}
	item, err := h.service.UpdateModel(currentTenantID(c), c.GetUint("user_id"), id, input)
	if err != nil {
		writeModelServiceError(c, err)
		return
	}
	model.OK(c, item)
}

func (h *ModelConfigHandler) TestModel(c *gin.Context) {
	id, ok := modelServiceID(c, "modelId", "模型")
	if !ok {
		return
	}
	var input model.ModelTestDraftInput
	if c.ShouldBindJSON(&input) != nil {
		model.Fail(c, http.StatusBadRequest, "无效的请求参数")
		return
	}
	result, err := h.service.TestModel(currentTenantID(c), c.GetUint("user_id"), id, input)
	if err != nil {
		writeModelServiceError(c, err)
		return
	}
	model.OK(c, result)
}

func (h *ModelConfigHandler) PreviewChannelDefaults(c *gin.Context) {
	h.handleChannelDefaults(c, true)
}

func (h *ModelConfigHandler) UpdateChannelDefaults(c *gin.Context) {
	h.handleChannelDefaults(c, false)
}

func (h *ModelConfigHandler) handleChannelDefaults(c *gin.Context, preview bool) {
	id, ok := modelServiceID(c, "channelId", "渠道")
	if !ok {
		return
	}
	var input model.UpdateChannelDefaultsInput
	if c.ShouldBindJSON(&input) != nil {
		model.Fail(c, http.StatusBadRequest, "无效的请求参数")
		return
	}
	if preview {
		result, err := h.service.PreviewChannelDefaults(currentTenantID(c), id, input)
		if err != nil {
			writeModelServiceError(c, err)
			return
		}
		model.OK(c, result)
		return
	}
	if err := h.service.UpdateChannelDefaults(currentTenantID(c), c.GetUint("user_id"), id, input); err != nil {
		writeModelServiceError(c, err)
		return
	}
	model.OK(c, gin.H{"saved": true})
}

func (h *ModelConfigHandler) SyncChannel(c *gin.Context) {
	id, ok := modelServiceID(c, "channelId", "渠道")
	if !ok {
		return
	}
	result, err := h.service.SyncChannel(currentTenantID(c), c.GetUint("user_id"), id)
	if err != nil {
		writeModelServiceError(c, err)
		return
	}
	model.OK(c, result)
}

func (h *ModelConfigHandler) ArchiveChannel(c *gin.Context) { h.setChannelArchived(c, true) }
func (h *ModelConfigHandler) RestoreChannel(c *gin.Context) { h.setChannelArchived(c, false) }

func (h *ModelConfigHandler) setChannelArchived(c *gin.Context, archived bool) {
	id, ok := modelServiceID(c, "channelId", "渠道")
	if !ok {
		return
	}
	if err := h.service.ArchiveChannel(currentTenantID(c), c.GetUint("user_id"), id, archived); err != nil {
		writeModelServiceError(c, err)
		return
	}
	model.OK(c, gin.H{"archived": archived})
}

func (h *ModelConfigHandler) ArchiveModel(c *gin.Context) { h.setModelArchived(c, true) }
func (h *ModelConfigHandler) RestoreModel(c *gin.Context) { h.setModelArchived(c, false) }

func (h *ModelConfigHandler) setModelArchived(c *gin.Context, archived bool) {
	id, ok := modelServiceID(c, "modelId", "模型")
	if !ok {
		return
	}
	if err := h.service.ArchiveModel(currentTenantID(c), c.GetUint("user_id"), id, archived); err != nil {
		writeModelServiceError(c, err)
		return
	}
	model.OK(c, gin.H{"archived": archived})
}

func (h *ModelConfigHandler) SaveDefaultPricing(c *gin.Context) {
	id, ok := modelServiceID(c, "catalogModelId", "目录模型")
	if !ok {
		return
	}
	var input model.SaveModelPricingInput
	if c.ShouldBindJSON(&input) != nil {
		model.Fail(c, http.StatusBadRequest, "无效的请求参数")
		return
	}
	if err := h.service.SaveDefaultPricing(currentTenantID(c), c.GetUint("user_id"), id, input); err != nil {
		writeModelServiceError(c, err)
		return
	}
	model.OK(c, gin.H{"saved": true})
}

func modelServiceID(c *gin.Context, param, label string) (uint, bool) {
	id, err := parsePositiveID(c.Param(param))
	if err != nil {
		model.Fail(c, http.StatusBadRequest, "无效的"+label+" ID")
		return 0, false
	}
	return id, true
}

func writeModelServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		model.Fail(c, http.StatusNotFound, "配置不存在")
	case errors.Is(err, service.ErrModelConfigRevisionConflict):
		model.Fail(c, http.StatusConflict, err.Error())
	case errors.Is(err, service.ErrChannelConfigRevisionConflict):
		model.Fail(c, http.StatusConflict, err.Error())
	default:
		model.Fail(c, http.StatusBadRequest, err.Error())
	}
}
